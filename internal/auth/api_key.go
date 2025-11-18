package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// APIKeyGenerator handles API key generation and validation
type APIKeyGenerator struct {
	db         *sql.DB
	saltPrefix string
}

// NewAPIKeyGenerator creates a new API key generator
func NewAPIKeyGenerator(db *sql.DB, saltPrefix string) *APIKeyGenerator {
	return &APIKeyGenerator{
		db:         db,
		saltPrefix: saltPrefix,
	}
}

// APICredentials represents generated API credentials
type APICredentials struct {
	APIKey       string    `json:"api_key"`
	APISecret    string    `json:"api_secret"`
	MerchantID   string    `json:"merchant_id"`
	Environment  string    `json:"environment"`
	Description  string    `json:"description,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// GenerateAPIKey generates a new API key with a specific prefix
func GenerateAPIKey(prefix string) (string, error) {
	// Generate 24 random bytes
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Encode to base64 and clean up
	encoded := base64.URLEncoding.EncodeToString(randomBytes)
	encoded = strings.TrimRight(encoded, "=")

	// Add prefix
	apiKey := fmt.Sprintf("%s_%s", prefix, encoded)

	return apiKey, nil
}

// GenerateAPISecret generates a secure API secret
func GenerateAPISecret() (string, error) {
	// Generate 32 random bytes for higher security
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Encode to base64 and clean up
	encoded := base64.URLEncoding.EncodeToString(randomBytes)
	encoded = strings.TrimRight(encoded, "=")

	return encoded, nil
}

// GenerateCredentials generates a new set of API credentials for a merchant
func (g *APIKeyGenerator) GenerateCredentials(merchantID, environment, description string, expiryDays int) (*APICredentials, error) {
	// Determine prefix based on environment
	var prefix string
	switch environment {
	case "production":
		prefix = "pk_live"
	case "staging":
		prefix = "pk_test"
	case "development":
		prefix = "pk_dev"
	default:
		prefix = "pk"
	}

	// Generate API key and secret
	apiKey, err := GenerateAPIKey(prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	apiSecret, err := GenerateAPISecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API secret: %w", err)
	}

	// Calculate expiry
	var expiresAt *time.Time
	if expiryDays > 0 {
		exp := time.Now().AddDate(0, 0, expiryDays)
		expiresAt = &exp
	}

	// Hash the credentials for storage
	apiKeyHash := g.hashWithSalt(apiKey)
	apiSecretHash := g.hashWithSalt(apiSecret)
	apiKeyPrefix := g.extractPrefix(apiKey)

	// Store in database
	credentialID := uuid.New().String()
	query := `
		INSERT INTO merchant_credentials (
			id, merchant_id, api_key_prefix, api_key_hash, api_secret_hash,
			description, environment, expires_at, is_active, created_at, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), $10)
	`

	_, err = g.db.Exec(query,
		credentialID,
		merchantID,
		apiKeyPrefix,
		apiKeyHash,
		apiSecretHash,
		description,
		environment,
		expiresAt,
		true,
		"api_generation",
	)

	if err != nil {
		return nil, fmt.Errorf("failed to store credentials: %w", err)
	}

	credentials := &APICredentials{
		APIKey:      apiKey,
		APISecret:   apiSecret,
		MerchantID:  merchantID,
		Environment: environment,
		Description: description,
		CreatedAt:   time.Now(),
	}

	if expiresAt != nil {
		credentials.ExpiresAt = *expiresAt
	}

	return credentials, nil
}

// ValidateCredentials validates API key and secret
func (g *APIKeyGenerator) ValidateCredentials(apiKey, apiSecret string) (*MerchantInfo, error) {
	apiKeyHash := g.hashWithSalt(apiKey)
	apiSecretHash := g.hashWithSalt(apiSecret)

	var merchantID, merchantSlug, environment string
	var rateLimit int
	err := g.db.QueryRow(`
		SELECT
			mc.merchant_id,
			m.slug,
			mc.environment,
			m.requests_per_second
		FROM merchant_credentials mc
		JOIN merchants m ON mc.merchant_id = m.id
		WHERE mc.api_key_hash = $1
		AND mc.api_secret_hash = $2
		AND mc.is_active = true
		AND (mc.expires_at IS NULL OR mc.expires_at > NOW())
		AND m.status = 'active'
	`, apiKeyHash, apiSecretHash).Scan(&merchantID, &merchantSlug, &environment, &rateLimit)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, err
	}

	// Update last used timestamp
	go g.updateLastUsed(apiKeyHash)

	return &MerchantInfo{
		MerchantID:   merchantID,
		MerchantCode: merchantSlug, // Using slug as the merchant identifier
		Environment:  environment,
		RateLimit:    rateLimit,
	}, nil
}

// RotateCredentials rotates API credentials for a merchant
func (g *APIKeyGenerator) RotateCredentials(merchantID, oldAPIKey string, gracePeriodHours int) (*APICredentials, error) {
	// Verify old API key belongs to merchant
	oldAPIKeyHash := g.hashWithSalt(oldAPIKey)

	var oldCredentialID string
	var environment, description string
	err := g.db.QueryRow(`
		SELECT id, environment, description
		FROM merchant_credentials
		WHERE merchant_id = $1
		AND api_key_hash = $2
		AND is_active = true
	`, merchantID, oldAPIKeyHash).Scan(&oldCredentialID, &environment, &description)

	if err != nil {
		return nil, fmt.Errorf("invalid old credentials: %w", err)
	}

	// Generate new credentials
	newCreds, err := g.GenerateCredentials(
		merchantID,
		environment,
		fmt.Sprintf("Rotated from %s", description),
		0, // No expiry for rotated credentials
	)
	if err != nil {
		return nil, err
	}

	// Update old credentials with grace period
	if gracePeriodHours > 0 {
		expiresAt := time.Now().Add(time.Duration(gracePeriodHours) * time.Hour)
		_, err = g.db.Exec(`
			UPDATE merchant_credentials
			SET expires_at = $1,
				description = description || ' (rotated)'
			WHERE id = $2
		`, expiresAt, oldCredentialID)
	} else {
		// Immediately deactivate old credentials
		_, err = g.db.Exec(`
			UPDATE merchant_credentials
			SET is_active = false,
				description = description || ' (rotated)'
			WHERE id = $2
		`, oldCredentialID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update old credentials: %w", err)
	}

	return newCreds, nil
}

// RevokeCredentials revokes API credentials
func (g *APIKeyGenerator) RevokeCredentials(apiKey string) error {
	apiKeyHash := g.hashWithSalt(apiKey)

	result, err := g.db.Exec(`
		UPDATE merchant_credentials
		SET is_active = false,
			description = description || ' (revoked)'
		WHERE api_key_hash = $1
		AND is_active = true
	`, apiKeyHash)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("credentials not found or already revoked")
	}

	return nil
}

// ListMerchantCredentials lists all credentials for a merchant
func (g *APIKeyGenerator) ListMerchantCredentials(merchantID string) ([]*CredentialInfo, error) {
	rows, err := g.db.Query(`
		SELECT
			id,
			api_key_prefix,
			description,
			environment,
			last_used_at,
			expires_at,
			is_active,
			created_at
		FROM merchant_credentials
		WHERE merchant_id = $1
		ORDER BY created_at DESC
	`, merchantID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var credentials []*CredentialInfo
	for rows.Next() {
		var cred CredentialInfo
		var lastUsed, expiresAt sql.NullTime

		err := rows.Scan(
			&cred.ID,
			&cred.APIKeyPrefix,
			&cred.Description,
			&cred.Environment,
			&lastUsed,
			&expiresAt,
			&cred.IsActive,
			&cred.CreatedAt,
		)

		if err != nil {
			continue
		}

		if lastUsed.Valid {
			cred.LastUsedAt = &lastUsed.Time
		}
		if expiresAt.Valid {
			cred.ExpiresAt = &expiresAt.Time
		}

		credentials = append(credentials, &cred)
	}

	return credentials, nil
}

// hashWithSalt creates a salted hash of the input
func (g *APIKeyGenerator) hashWithSalt(input string) string {
	h := sha256.New()
	h.Write([]byte(g.saltPrefix + input))
	return hex.EncodeToString(h.Sum(nil))
}

// extractPrefix extracts the prefix from an API key (first 10 chars)
func (g *APIKeyGenerator) extractPrefix(apiKey string) string {
	if len(apiKey) > 10 {
		return apiKey[:10]
	}
	return apiKey
}

// updateLastUsed updates the last used timestamp
func (g *APIKeyGenerator) updateLastUsed(apiKeyHash string) {
	_, err := g.db.Exec(`
		UPDATE merchant_credentials
		SET last_used_at = NOW()
		WHERE api_key_hash = $1
	`, apiKeyHash)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to update last_used_at: %v\n", err)
	}
}

// MerchantInfo contains merchant information from credential validation
type MerchantInfo struct {
	MerchantID   string
	MerchantCode string
	Environment  string
	RateLimit    int
}

// CredentialInfo contains credential information
type CredentialInfo struct {
	ID           string
	APIKeyPrefix string
	Description  string
	Environment  string
	LastUsedAt   *time.Time
	ExpiresAt    *time.Time
	IsActive     bool
	CreatedAt    time.Time
}