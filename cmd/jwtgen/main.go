package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/domain"
)

// ServiceCredentials represents the structure of the credentials JSON file
type ServiceCredentials struct {
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	Environment string `json:"environment"`
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
}

// TokenOutput represents the JSON output format
type TokenOutput struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	ServiceID string    `json:"service_id"`
	Scopes    []string  `json:"scopes"`
}

func main() {
	// Define flags
	credentials := flag.String("c", "", "Path to service credentials JSON file (required)")
	credentialsLong := flag.String("credentials", "", "Path to service credentials JSON file (required)")
	expires := flag.String("e", "1h", "Token expiry duration (e.g., 5m, 1h, 24h)")
	expiresLong := flag.String("expires", "1h", "Token expiry duration (e.g., 5m, 1h, 24h)")
	scopes := flag.String("s", "", "Comma-separated scopes (default: all payment scopes)")
	scopesLong := flag.String("scopes", "", "Comma-separated scopes (default: all payment scopes)")
	output := flag.String("o", "token", "Output format: token, json, curl")
	outputLong := flag.String("output", "token", "Output format: token, json, curl")
	decode := flag.Bool("decode", false, "Show decoded claims")
	help := flag.Bool("h", false, "Show help")
	helpLong := flag.Bool("help", false, "Show help")

	flag.Parse()

	// Handle help
	if *help || *helpLong {
		printHelp()
		os.Exit(0)
	}

	// Resolve flag values (short takes precedence if both provided)
	credFile := resolveString(*credentials, *credentialsLong)
	expiry := resolveString(*expires, *expiresLong)
	scopeStr := resolveString(*scopes, *scopesLong)
	outputFmt := resolveString(*output, *outputLong)

	// Validate required flags
	if credFile == "" {
		fmt.Fprintln(os.Stderr, "Error: credentials file is required (-c or --credentials)")
		fmt.Fprintln(os.Stderr, "Run 'jwtgen --help' for usage")
		os.Exit(1)
	}

	// Parse expiry duration
	expiryDuration, err := time.ParseDuration(expiry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid expiry duration '%s': %v\n", expiry, err)
		os.Exit(1)
	}

	// Load credentials
	creds, err := loadCredentials(credFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading credentials: %v\n", err)
		os.Exit(1)
	}

	// Parse scopes
	var tokenScopes []string
	if scopeStr != "" {
		tokenScopes = strings.Split(scopeStr, ",")
		for i := range tokenScopes {
			tokenScopes[i] = strings.TrimSpace(tokenScopes[i])
		}
	} else {
		tokenScopes = domain.AllPaymentScopes()
	}

	// Generate token
	token, err := generateToken(creds, tokenScopes, expiryDuration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating token: %v\n", err)
		os.Exit(1)
	}

	// Output based on format
	expiresAt := time.Now().Add(expiryDuration)

	switch outputFmt {
	case "json":
		outputJSON(token, expiresAt, creds.ServiceID, tokenScopes)
	case "curl":
		outputCurl(token)
	default:
		fmt.Println(token)
	}

	// Show decoded claims if requested
	if *decode {
		fmt.Fprintln(os.Stderr, "\n# Decoded Claims:")
		showDecodedClaims(token)
	}
}

func resolveString(short, long string) string {
	if short != "" {
		return short
	}
	return long
}

func loadCredentials(path string) (*ServiceCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var creds ServiceCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if creds.ServiceID == "" {
		return nil, fmt.Errorf("missing service_id in credentials file")
	}

	if creds.PrivateKey == "" {
		return nil, fmt.Errorf("missing private_key in credentials file")
	}

	return &creds, nil
}

func parsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Try PKCS1 format first
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return privateKey, nil
	}

	// Try PKCS8 format
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an RSA private key")
	}

	return rsaKey, nil
}

// generateToken creates a JWT token for service authentication
// NOTE: merchant_id is NOT included in the token
// The token identifies the SERVICE, and the merchant is specified per-request
func generateToken(creds *ServiceCredentials, scopes []string, expiry time.Duration) (string, error) {
	privateKey, err := parsePrivateKey(creds.PrivateKey)
	if err != nil {
		return "", err
	}

	now := time.Now()
	jti := uuid.New().String()

	claims := jwt.MapClaims{
		"iss":    creds.ServiceID,
		"sub":    creds.ServiceID,
		"scopes": scopes,
		"exp":    now.Add(expiry).Unix(),
		"iat":    now.Unix(),
		"nbf":    now.Unix(),
		"jti":    jti,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func outputJSON(token string, expiresAt time.Time, serviceID string, scopes []string) {
	output := TokenOutput{
		Token:     token,
		ExpiresAt: expiresAt,
		ServiceID: serviceID,
		Scopes:    scopes,
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(data))
}

func outputCurl(token string) {
	fmt.Printf(`# Example curl command:
# NOTE: merchant_id must be included in request body
curl -X POST http://localhost:8080/payment.v1.PaymentService/Sale \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer %s" \
  -d '{
    "merchant_id": "your-merchant-uuid",
    "amount_cents": 1000,
    "currency": "USD",
    "payment_token": "your-payment-token"
  }'
`, token)
}

func showDecodedClaims(tokenStr string) {
	// Parse without validation to show claims
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing token: %v\n", err)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		fmt.Fprintln(os.Stderr, "Error: invalid claims format")
		return
	}

	data, _ := json.MarshalIndent(claims, "", "  ")
	fmt.Fprintln(os.Stderr, string(data))
}

func printHelp() {
	fmt.Println(`jwtgen - Generate JWT tokens for payment service API authentication

USAGE:
  jwtgen -c <credentials.json> [options]

AUTHENTICATION MODEL:
  - Token identifies the SERVICE (via service credentials)
  - Request body specifies the MERCHANT (via merchant_id field)
  - Database validates service has access to the merchant per-request
  - One token works for all merchants the service has access to

REQUIRED:
  -c, --credentials    Path to service credentials JSON file

OPTIONS:
  -e, --expires        Token expiry duration (default: 1h)
                       Examples: 5m, 30m, 1h, 24h, 7d
  -s, --scopes         Comma-separated scopes (default: all payment scopes)
                       Available scopes:
                         payments:create     - Create payments (auth, capture, sale)
                         payments:read       - View transactions
                         payments:void       - Void payments
                         payments:refund     - Issue refunds
                         payment_methods:read   - View saved payment methods
                         payment_methods:create - Store payment methods
                         subscriptions:manage   - Manage subscriptions
                         subscriptions:read     - View subscriptions
                         *                      - All permissions
  -o, --output         Output format (default: token)
                         token  - Just the JWT string
                         json   - JSON with token + metadata
                         curl   - Ready-to-use curl command
      --decode         Show decoded claims (for verification)
  -h, --help           Show this help message

EXAMPLES:
  # Generate token with default scopes (1 hour expiry)
  jwtgen -c service_acme_credentials.json

  # Custom expiry and scopes
  jwtgen -c creds.json -e 30m -s "payments:create,payments:read"

  # Output as curl command (shows how to include merchant_id in request)
  jwtgen -c creds.json -o curl

  # Show decoded claims for verification
  jwtgen -c creds.json --decode

  # Pipe to clipboard (macOS)
  jwtgen -c creds.json | pbcopy

  # Pipe to clipboard (Linux with xclip)
  jwtgen -c creds.json | xclip -selection clipboard`)
}
