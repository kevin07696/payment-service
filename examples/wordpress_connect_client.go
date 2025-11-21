//go:build ignore

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
	"go.uber.org/zap"
)

// WordPressConnectClient handles payment API calls with JWT authentication using ConnectRPC
type WordPressConnectClient struct {
	serviceID   string
	privateKey  interface{}
	apiEndpoint string
	logger      *zap.Logger

	// Token management
	tokenCache  map[string]*cachedToken // merchantID -> token
	tokenMutex  sync.RWMutex
	tokenExpiry time.Duration

	// ConnectRPC client
	paymentClient paymentv1connect.PaymentServiceClient
	httpClient    *http.Client
}

// cachedToken stores a token with its expiry
type cachedToken struct {
	token     string
	expiresAt time.Time
}

// NewWordPressConnectClient creates a new ConnectRPC client
func NewWordPressConnectClient(serviceID string, privateKeyPEM []byte, apiEndpoint string) (*WordPressConnectClient, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	logger, _ := zap.NewProduction()

	// Create HTTP client with custom transport
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // For development only
			},
		},
	}

	// Create ConnectRPC client
	paymentClient := paymentv1connect.NewPaymentServiceClient(
		httpClient,
		apiEndpoint,
		connect.WithInterceptors(&authInterceptor{
			getToken: nil, // Will be set after struct creation
		}),
	)

	client := &WordPressConnectClient{
		serviceID:     serviceID,
		privateKey:    privateKey,
		apiEndpoint:   apiEndpoint,
		logger:        logger,
		tokenCache:    make(map[string]*cachedToken),
		tokenExpiry:   5 * time.Minute,
		paymentClient: paymentClient,
		httpClient:    httpClient,
	}

	// Set the getToken function for the interceptor
	if interceptors := httpClient.Transport.(*http.Transport); interceptors != nil {
		// The interceptor will call our getOrCreateToken method
		// We'll handle this differently - see below
	}

	return client, nil
}

// authInterceptor adds JWT token to ConnectRPC requests
type authInterceptor struct {
	getToken func(string) (string, error)
}

func (ai *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Extract merchant ID from request if available
		merchantID := extractMerchantID(req)
		if merchantID != "" && ai.getToken != nil {
			token, err := ai.getToken(merchantID)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			req.Header().Set("Authorization", "Bearer "+token)
		}
		req.Header().Set("X-Request-ID", uuid.New().String())
		return next(ctx, req)
	}
}

func (ai *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // Not implemented for this example
}

func (ai *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next // Not implemented for this example
}

// extractMerchantID attempts to extract merchant ID from the request
func extractMerchantID(req connect.AnyRequest) string {
	// This is a simplified version - in production you'd use reflection or type assertions
	// to extract the merchant_id field from the actual request message
	return ""
}

// getOrCreateToken gets a cached token or creates a new one
func (c *WordPressConnectClient) getOrCreateToken(merchantID string, scopes []string) (string, error) {
	// Check cache first
	c.tokenMutex.RLock()
	cached, exists := c.tokenCache[merchantID]
	c.tokenMutex.RUnlock()

	// If token exists and is still valid (with 30 second buffer), use it
	if exists && time.Now().Add(30*time.Second).Before(cached.expiresAt) {
		c.logger.Debug("Using cached token",
			zap.String("merchant_id", merchantID),
			zap.Time("expires_at", cached.expiresAt))
		return cached.token, nil
	}

	// Generate new token
	c.logger.Info("Generating new JWT token",
		zap.String("merchant_id", merchantID),
		zap.Strings("scopes", scopes))

	token, expiresAt, err := c.generateToken(merchantID, scopes)
	if err != nil {
		return "", err
	}

	// Cache the new token
	c.tokenMutex.Lock()
	c.tokenCache[merchantID] = &cachedToken{
		token:     token,
		expiresAt: expiresAt,
	}
	c.tokenMutex.Unlock()

	return token, nil
}

// generateToken creates a new JWT token
func (c *WordPressConnectClient) generateToken(merchantID string, scopes []string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(c.tokenExpiry)

	claims := jwt.MapClaims{
		"iss":         c.serviceID,         // Issuer (WordPress plugin)
		"sub":         merchantID,          // Subject (merchant)
		"merchant_id": merchantID,          // Merchant ID claim
		"iat":         now.Unix(),          // Issued at
		"exp":         expiresAt.Unix(),    // Expires at
		"jti":         uuid.New().String(), // Unique token ID
		"scopes":      scopes,              // Permissions
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// Sale creates a payment transaction (authorize + capture) using ConnectRPC
func (c *WordPressConnectClient) Sale(ctx context.Context, merchantID string, amount string, currency string, paymentToken string) (*paymentv1.PaymentResponse, error) {
	// Get or create token with required scopes
	token, err := c.getOrCreateToken(merchantID, []string{"payment:create"})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Create ConnectRPC request
	req := connect.NewRequest(&paymentv1.SaleRequest{
		MerchantId:     merchantID,
		Amount:         amount,
		Currency:       currency,
		PaymentMethod:  &paymentv1.SaleRequest_PaymentToken{PaymentToken: paymentToken},
		IdempotencyKey: uuid.New().String(),
		Metadata: map[string]string{
			"source":   "wordpress-plugin",
			"order_id": uuid.New().String(),
		},
	})

	// Add authentication header
	req.Header().Set("Authorization", "Bearer "+token)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Make the ConnectRPC call
	resp, err := c.paymentClient.Sale(ctx, req)
	if err != nil {
		// Handle authentication errors
		if connectErr, ok := err.(*connect.Error); ok {
			if connectErr.Code() == connect.CodeUnauthenticated {
				// Clear token cache and suggest retry
				c.clearTokenCache()
				return nil, fmt.Errorf("authentication failed, please retry")
			}
		}
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	return resp.Msg, nil
}

// RefundTransaction refunds a payment transaction
func (c *WordPressConnectClient) RefundTransaction(ctx context.Context, merchantID string, groupID string, amount string) (*paymentv1.PaymentResponse, error) {
	// Get or create token with refund scope
	token, err := c.getOrCreateToken(merchantID, []string{"payment:refund"})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Create refund request
	req := connect.NewRequest(&paymentv1.RefundRequest{
		GroupId:        groupID,
		Amount:         amount,
		Reason:         "Customer requested refund",
		IdempotencyKey: uuid.New().String(),
	})

	// Add authentication header
	req.Header().Set("Authorization", "Bearer "+token)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Make the ConnectRPC call
	resp, err := c.paymentClient.Refund(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("refund failed: %w", err)
	}

	return resp.Msg, nil
}

// GetTransaction retrieves transaction details
func (c *WordPressConnectClient) GetTransaction(ctx context.Context, merchantID string, transactionID string) (*paymentv1.Transaction, error) {
	// Get or create token with read scope
	token, err := c.getOrCreateToken(merchantID, []string{"payment:read"})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Create request
	req := connect.NewRequest(&paymentv1.GetTransactionRequest{
		TransactionId: transactionID,
	})

	// Add authentication header
	req.Header().Set("Authorization", "Bearer "+token)
	req.Header().Set("X-Request-ID", uuid.New().String())

	// Make the ConnectRPC call
	resp, err := c.paymentClient.GetTransaction(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get transaction failed: %w", err)
	}

	return resp.Msg, nil
}

// clearTokenCache clears all cached tokens
func (c *WordPressConnectClient) clearTokenCache() {
	c.tokenMutex.Lock()
	c.tokenCache = make(map[string]*cachedToken)
	c.tokenMutex.Unlock()
	c.logger.Info("Token cache cleared")
}

// StartTokenRefresher starts a background goroutine to clean expired tokens
func (c *WordPressConnectClient) StartTokenRefresher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.cleanExpiredTokens()
			}
		}
	}()
}

// cleanExpiredTokens removes expired tokens from cache
func (c *WordPressConnectClient) cleanExpiredTokens() {
	c.tokenMutex.Lock()
	defer c.tokenMutex.Unlock()

	now := time.Now()
	for merchantID, cached := range c.tokenCache {
		if now.After(cached.expiresAt) {
			delete(c.tokenCache, merchantID)
			c.logger.Debug("Removed expired token",
				zap.String("merchant_id", merchantID))
		}
	}
}

// Example usage
func main() {
	fmt.Println("WordPress ConnectRPC Payment Client")
	fmt.Println("====================================")
	fmt.Println()

	// Load WordPress service credentials
	credData, err := os.ReadFile("service_wordpress-plugin_credentials.json")
	if err != nil {
		fmt.Printf("Error loading credentials: %v\n", err)
		return
	}

	var creds struct {
		ServiceID  string `json:"service_id"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(credData, &creds); err != nil {
		fmt.Printf("Error parsing credentials: %v\n", err)
		return
	}

	// Create client
	client, err := NewWordPressConnectClient(
		creds.ServiceID,
		[]byte(creds.PrivateKey),
		"http://localhost:8080", // Your ConnectRPC server endpoint
	)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}

	// Start token refresher
	ctx := context.Background()
	client.StartTokenRefresher(ctx)

	// Example: Create a payment
	merchantID := "1a20fff8-2cec-48e5-af49-87e501652913" // ACME Corp

	fmt.Println("Creating payment transaction...")
	// In production, you'd get this token from your payment form or tokenization service
	// For testing, you might use a test token or create one via your payment gateway
	paymentToken := "test-token-12345"

	response, err := client.Sale(
		ctx,
		merchantID,
		"99.99",
		"USD",
		paymentToken,
	)
	if err != nil {
		fmt.Printf("Payment failed: %v\n", err)
		return
	}

	fmt.Printf("Payment successful!\n")
	fmt.Printf("  Transaction ID: %s\n", response.TransactionId)
	fmt.Printf("  Group ID: %s\n", response.GroupId)
	fmt.Printf("  Status: %s\n", response.Status)
	fmt.Printf("  Amount: %s %s\n", response.Amount, response.Currency)
	fmt.Printf("  Created: %s\n", response.CreatedAt.AsTime().Format(time.RFC3339))

	// Example: Get transaction details
	fmt.Println("\nRetrieving transaction details...")
	txDetails, err := client.GetTransaction(ctx, merchantID, response.TransactionId)
	if err != nil {
		fmt.Printf("Failed to get transaction: %v\n", err)
		return
	}

	fmt.Printf("Transaction details retrieved:\n")
	fmt.Printf("  Status: %s\n", txDetails.Status)
	fmt.Printf("  Type: %s\n", txDetails.Type)
	fmt.Printf("  Group ID: %s\n", txDetails.GroupId)

	// Example: Refund (if needed)
	/*
		fmt.Println("\nRefunding 50.00...")
		refundResp, err := client.RefundTransaction(ctx, merchantID, response.GroupId, "50.00")
		if err != nil {
			fmt.Printf("Refund failed: %v\n", err)
			return
		}
		fmt.Printf("Refund successful! New status: %s\n", refundResp.Status)
	*/

	fmt.Println("\n✓ WordPress ConnectRPC client successfully tested!")
	fmt.Println("\nKey features demonstrated:")
	fmt.Println("  • JWT token generation with RSA signing")
	fmt.Println("  • Token caching per merchant")
	fmt.Println("  • Automatic token reuse until expiry")
	fmt.Println("  • ConnectRPC protocol compliance")
	fmt.Println("  • Proper error handling")
}
