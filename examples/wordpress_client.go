//go:build ignore

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// WordPressPaymentClient handles payment API calls with JWT authentication
type WordPressPaymentClient struct {
	serviceID   string
	privateKey  interface{}
	apiEndpoint string
	logger      *zap.Logger

	// Token management
	tokenCache  map[string]*cachedToken // merchantID -> token
	tokenMutex  sync.RWMutex
	tokenExpiry time.Duration
}

// cachedToken stores a token with its expiry
type cachedToken struct {
	token     string
	expiresAt time.Time
}

// NewWordPressPaymentClient creates a new client
func NewWordPressPaymentClient(serviceID string, privateKeyPEM []byte, apiEndpoint string) (*WordPressPaymentClient, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	logger, _ := zap.NewProduction()

	return &WordPressPaymentClient{
		serviceID:   serviceID,
		privateKey:  privateKey,
		apiEndpoint: apiEndpoint,
		logger:      logger,
		tokenCache:  make(map[string]*cachedToken),
		tokenExpiry: 5 * time.Minute, // Short-lived tokens for security
	}, nil
}

// getOrCreateToken gets a cached token or creates a new one
func (c *WordPressPaymentClient) getOrCreateToken(merchantID string, scopes []string) (string, error) {
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
func (c *WordPressPaymentClient) generateToken(merchantID string, scopes []string) (string, time.Time, error) {
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

// CreatePayment creates a payment for a merchant
func (c *WordPressPaymentClient) CreatePayment(ctx context.Context, merchantID string, payment *PaymentRequest) (*PaymentResponse, error) {
	// Get or create token with required scopes
	token, err := c.getOrCreateToken(merchantID, []string{"payment:create"})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// Make API call with token
	return c.makeAuthenticatedRequest(ctx, token, "POST", "/payment.v1.PaymentService/CreateTransaction", payment)
}

// RefundPayment refunds a payment
func (c *WordPressPaymentClient) RefundPayment(ctx context.Context, merchantID string, transactionID string, amount float64) (*PaymentResponse, error) {
	// Get or create token with refund scope
	token, err := c.getOrCreateToken(merchantID, []string{"payment:refund"})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	refundRequest := &RefundRequest{
		TransactionID: transactionID,
		Amount:        amount,
	}

	return c.makeAuthenticatedRequest(ctx, token, "POST", "/payment.v1.PaymentService/RefundTransaction", refundRequest)
}

// makeAuthenticatedRequest makes an HTTP request with JWT authentication
func (c *WordPressPaymentClient) makeAuthenticatedRequest(ctx context.Context, token string, method string, path string, body interface{}) (*PaymentResponse, error) {
	// Serialize request body
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	// Create request
	url := c.apiEndpoint + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.New().String())

	// Execute request with retry logic
	resp, err := c.executeWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle response
	if resp.StatusCode == http.StatusUnauthorized {
		// Token might be expired, clear cache and retry once
		c.clearTokenCache()
		return nil, fmt.Errorf("authentication failed, please retry")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed: %s - %s", resp.Status, string(body))
	}

	// Parse response
	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &paymentResp, nil
}

// executeWithRetry executes HTTP request with exponential backoff retry
func (c *WordPressPaymentClient) executeWithRetry(req *http.Request) (*http.Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	maxRetries := 3
	backoff := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		resp, err := client.Do(req)
		if err != nil {
			if i == maxRetries-1 {
				return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, err)
			}
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		// Retry on 5xx errors
		if resp.StatusCode >= 500 && i < maxRetries-1 {
			resp.Body.Close()
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded")
}

// clearTokenCache clears all cached tokens
func (c *WordPressPaymentClient) clearTokenCache() {
	c.tokenMutex.Lock()
	c.tokenCache = make(map[string]*cachedToken)
	c.tokenMutex.Unlock()
	c.logger.Info("Token cache cleared")
}

// StartTokenRefresher starts a background goroutine to clean expired tokens
func (c *WordPressPaymentClient) StartTokenRefresher(ctx context.Context) {
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
func (c *WordPressPaymentClient) cleanExpiredTokens() {
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

// Request/Response types
type PaymentRequest struct {
	MerchantID    string  `json:"merchant_id"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Description   string  `json:"description"`
	CustomerEmail string  `json:"customer_email"`
	CardNumber    string  `json:"card_number"`
	ExpMonth      string  `json:"exp_month"`
	ExpYear       string  `json:"exp_year"`
	CVV           string  `json:"cvv"`
}

type RefundRequest struct {
	TransactionID string  `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Reason        string  `json:"reason"`
}

type PaymentResponse struct {
	TransactionID string    `json:"transaction_id"`
	Status        string    `json:"status"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
	Message       string    `json:"message,omitempty"`
}

// Example usage in WordPress plugin
func ExampleWordPressPlugin() {
	// Load credentials (in real plugin, from WordPress options/database)
	credentials := loadWordPressCredentials()

	// Create client
	client, err := NewWordPressPaymentClient(
		credentials.ServiceID,
		[]byte(credentials.PrivateKey),
		"http://localhost:8080", // Payment service endpoint
	)
	if err != nil {
		panic(err)
	}

	// Start token refresher
	ctx := context.Background()
	client.StartTokenRefresher(ctx)

	// Process payment for merchant
	merchantID := "1a20fff8-2cec-48e5-af49-87e501652913" // ACME Corp

	payment := &PaymentRequest{
		MerchantID:    merchantID,
		Amount:        99.99,
		Currency:      "USD",
		Description:   "WordPress Premium Plugin",
		CustomerEmail: "customer@example.com",
		CardNumber:    "4111111111111111",
		ExpMonth:      "12",
		ExpYear:       "2025",
		CVV:           "123",
	}

	// Make payment (token is handled automatically)
	response, err := client.CreatePayment(ctx, merchantID, payment)
	if err != nil {
		fmt.Printf("Payment failed: %v\n", err)
		return
	}

	fmt.Printf("Payment successful! Transaction ID: %s\n", response.TransactionID)

	// Later, if refund is needed
	refundResp, err := client.RefundPayment(ctx, merchantID, response.TransactionID, 50.00)
	if err != nil {
		fmt.Printf("Refund failed: %v\n", err)
		return
	}

	fmt.Printf("Refund successful! New status: %s\n", refundResp.Status)
}

// loadWordPressCredentials would load from WordPress database/options
func loadWordPressCredentials() struct {
	ServiceID  string
	PrivateKey string
} {
	// In real WordPress plugin:
	// - Store encrypted in wp_options table
	// - Or in separate secure storage
	// - Never expose private key to frontend

	return struct {
		ServiceID  string
		PrivateKey string
	}{
		ServiceID:  "wordpress-plugin",
		PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----",
	}
}

func main() {
	fmt.Println("WordPress Payment Client Example")
	fmt.Println("=================================")
	fmt.Println()
	fmt.Println("This client demonstrates:")
	fmt.Println("1. Automatic token generation")
	fmt.Println("2. Token caching and reuse")
	fmt.Println("3. Automatic token refresh before expiry")
	fmt.Println("4. Retry logic with exponential backoff")
	fmt.Println("5. Proper error handling")
	fmt.Println()
	fmt.Println("Key features:")
	fmt.Println("- Tokens are cached per merchant")
	fmt.Println("- Tokens are reused until 30 seconds before expiry")
	fmt.Println("- Expired tokens are automatically cleaned up")
	fmt.Println("- Failed requests are retried with backoff")
	fmt.Println("- 401 errors clear cache and suggest retry")
}
