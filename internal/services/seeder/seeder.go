// Package seeder provides database seeding for development/sandbox environments.
// Auto-seeds a default merchant using EPX sandbox credentials from environment.
package seeder

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/pkg/crypto"
)

// Deterministic UUIDs for predictable testing (documented in API_SPECS.md)
const (
	// Service
	TestServiceID   = "test-pos-system"
	TestServiceName = "Test POS System"

	// Subscriptions (deterministic for copy-paste API testing)
	TestActiveSubscriptionID    = "66666666-6666-6666-6666-666666666666"
	TestPausedSubscriptionID    = "77777777-7777-7777-7777-777777777777"
	TestCancelledSubscriptionID = "88888888-8888-8888-8888-888888888888"

	// Chargebacks
	TestChargebackID = "99999999-9999-9999-9999-999999999999"

	// Customer IDs
	TestCustomerID = "test-customer-001"

	// Placeholder payment method ID (used for subscriptions until real one is created)
	PlaceholderPaymentMethodID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
)

// Config holds the seed configuration derived from EPX environment variables.
type Config struct {
	// Merchant identification (auto-generated for sandbox)
	MerchantID   string
	MerchantSlug string
	MerchantName string

	// EPX credentials (from EPX_* env vars)
	CustNbr     string
	MerchNbr    string
	DbaNbr      string
	TerminalNbr string

	// MAC (Merchant Authorization Code)
	MAC           string
	MacSecretPath string

	// Environment
	Environment string
}

// LoadConfig loads seed configuration from environment variables.
// Returns nil if environment is production (no auto-seeding in prod).
func LoadConfig() *Config {
	env := os.Getenv("ENVIRONMENT")

	// Never auto-seed in production
	if env == "production" {
		return nil
	}

	merchantSlug := os.Getenv("SANDBOX_MERCHANT_SLUG")

	return &Config{
		MerchantID:    os.Getenv("SANDBOX_MERCHANT_ID"),
		MerchantSlug:  merchantSlug,
		MerchantName:  os.Getenv("SANDBOX_MERCHANT_NAME"),
		CustNbr:       os.Getenv("EPX_CUST_NBR"),
		MerchNbr:      os.Getenv("EPX_MERCH_NBR"),
		DbaNbr:        os.Getenv("EPX_DBA_NBR"),
		TerminalNbr:   os.Getenv("EPX_TERMINAL_NBR"),
		MAC:           os.Getenv("EPX_SANDBOX_MAC"),
		MacSecretPath: fmt.Sprintf("payments/merchants/%s/mac", merchantSlug),
		Environment:   env,
	}
}

// Seeder handles database seeding using SQLC queries and secret manager.
type Seeder struct {
	queries       *sqlc.Queries
	secretManager ports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewSeeder creates a new Seeder instance.
func NewSeeder(queries *sqlc.Queries, secretManager ports.SecretManagerAdapter, logger *zap.Logger) *Seeder {
	return &Seeder{
		queries:       queries,
		secretManager: secretManager,
		logger:        logger,
	}
}

// SeedIfNeeded checks if seeding is needed and performs it.
// Only seeds in development/staging if no merchants exist.
func (s *Seeder) SeedIfNeeded(ctx context.Context) error {
	cfg := LoadConfig()
	if cfg == nil {
		s.logger.Debug("Auto-seeding disabled (production environment)")
		return nil
	}

	// Check if sandbox merchant already exists
	merchantID, err := uuid.Parse(cfg.MerchantID)
	if err != nil {
		return fmt.Errorf("invalid merchant ID: %w", err)
	}

	_, err = s.queries.GetMerchantByID(ctx, merchantID)
	if err == nil {
		s.logger.Debug("Sandbox merchant already exists, skipping seed",
			zap.String("merchant_id", cfg.MerchantID),
		)
		return nil
	}

	// Merchant doesn't exist, seed it
	return s.seedSandboxMerchant(ctx, cfg)
}

// seedSandboxMerchant creates the sandbox merchant with EPX credentials.
func (s *Seeder) seedSandboxMerchant(ctx context.Context, cfg *Config) error {
	// Validate required EPX credentials
	if cfg.CustNbr == "" || cfg.MerchNbr == "" {
		s.logger.Warn("EPX credentials not configured, skipping auto-seed",
			zap.Bool("has_cust_nbr", cfg.CustNbr != ""),
			zap.Bool("has_merch_nbr", cfg.MerchNbr != ""),
		)
		return nil
	}

	merchantID, err := uuid.Parse(cfg.MerchantID)
	if err != nil {
		return fmt.Errorf("invalid merchant ID: %w", err)
	}

	s.logger.Info("Auto-seeding sandbox merchant",
		zap.String("merchant_id", cfg.MerchantID),
		zap.String("slug", cfg.MerchantSlug),
		zap.String("environment", cfg.Environment),
	)

	// Store MAC in secret manager if provided
	if cfg.MAC != "" {
		if err := s.storeMAC(ctx, cfg); err != nil {
			return fmt.Errorf("storing MAC for merchant %s: %w", cfg.MerchantSlug, err)
		}
	}

	// Upsert merchant record
	merchant, err := s.queries.UpsertMerchant(ctx, sqlc.UpsertMerchantParams{
		ID:            merchantID,
		Slug:          cfg.MerchantSlug,
		Name:          cfg.MerchantName,
		CustNbr:       cfg.CustNbr,
		MerchNbr:      cfg.MerchNbr,
		DbaNbr:        cfg.DbaNbr,
		TerminalNbr:   cfg.TerminalNbr,
		MacSecretPath: cfg.MacSecretPath,
		Environment:   cfg.Environment,
		IsActive:      true,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert merchant: %w", err)
	}

	s.logger.Info("Sandbox merchant seeded successfully",
		zap.String("merchant_id", merchant.ID.String()),
		zap.String("slug", merchant.Slug),
		zap.String("name", merchant.Name),
		zap.Bool("mac_stored", cfg.MAC != ""),
	)

	return nil
}

// storeMAC stores the MAC in the secret manager.
func (s *Seeder) storeMAC(ctx context.Context, cfg *Config) error {
	s.logger.Info("Storing MAC in secret manager",
		zap.String("path", cfg.MacSecretPath),
	)

	metadata := map[string]string{
		"merchant_slug": cfg.MerchantSlug,
		"environment":   cfg.Environment,
		"seeded_by":     "auto-seed",
	}

	version, err := s.secretManager.PutSecret(ctx, cfg.MacSecretPath, cfg.MAC, metadata)
	if err != nil {
		return fmt.Errorf("failed to store MAC at %s: %w", cfg.MacSecretPath, err)
	}

	s.logger.Info("MAC stored successfully",
		zap.String("path", cfg.MacSecretPath),
		zap.String("version", version),
	)

	return nil
}

// SeedTestData seeds test data for API documentation and testing.
// Creates: service, service-merchant access, subscriptions, chargebacks.
// Idempotent - skips if data already exists.
func (s *Seeder) SeedTestData(ctx context.Context) error {
	cfg := LoadConfig()
	if cfg == nil {
		s.logger.Debug("Test data seeding disabled (production environment)")
		return nil
	}

	merchantID, err := uuid.Parse(cfg.MerchantID)
	if err != nil {
		s.logger.Debug("No valid merchant ID for test data seeding")
		return nil
	}

	s.logger.Info("Seeding test data for API documentation",
		zap.String("merchant_id", cfg.MerchantID),
	)

	// 1. Create test service
	serviceUUID, err := s.seedTestService(ctx, cfg.Environment)
	if err != nil {
		return fmt.Errorf("failed to seed test service: %w", err)
	}

	// 2. Grant service access to merchant
	if err := s.seedServiceAccess(ctx, serviceUUID, merchantID); err != nil {
		return fmt.Errorf("failed to seed service access: %w", err)
	}

	// 3. Seed subscriptions (with placeholder payment method)
	if err := s.seedTestSubscriptions(ctx, merchantID); err != nil {
		return fmt.Errorf("failed to seed test subscriptions: %w", err)
	}

	s.logger.Info("Test data seeded successfully")
	return nil
}

// seedTestService creates the test-pos-system service if it doesn't exist.
func (s *Seeder) seedTestService(ctx context.Context, environment string) (uuid.UUID, error) {
	// Check if service already exists
	existing, err := s.queries.GetServiceByServiceID(ctx, TestServiceID)
	if err == nil {
		s.logger.Debug("Test service already exists",
			zap.String("service_id", TestServiceID),
		)
		return existing.ID, nil
	}

	// Generate RSA keypair
	keypair, err := crypto.GenerateRSAKeyPair()
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to generate RSA keypair: %w", err)
	}

	serviceUUID := uuid.New()
	fingerprint := generateFingerprint([]byte(keypair.PublicKeyPEM))

	_, err = s.queries.CreateService(ctx, sqlc.CreateServiceParams{
		ID:                   serviceUUID,
		ServiceID:            TestServiceID,
		ServiceName:          TestServiceName,
		PublicKey:            keypair.PublicKeyPEM,
		PublicKeyFingerprint: fingerprint,
		Environment:          environment,
		RequestsPerSecond:    pgtype.Int4{Int32: 1000, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: 2000, Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		return uuid.Nil, err
	}

	// Save credentials file
	creds := map[string]interface{}{
		"service_id":   TestServiceID,
		"service_name": TestServiceName,
		"private_key":  keypair.PrivateKeyPEM,
		"environment":  environment,
		"created_at":   time.Now().UTC().Format(time.RFC3339),
	}

	credsJSON, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		s.logger.Warn("Failed to marshal service credentials",
			zap.String("service_id", TestServiceID),
			zap.Error(err))
	}
	credsFile := fmt.Sprintf("service_%s_credentials.json", TestServiceID)
	if err := os.WriteFile(credsFile, credsJSON, 0600); err != nil {
		s.logger.Warn("Failed to save service credentials file",
			zap.String("file", credsFile),
			zap.Error(err),
		)
	} else {
		s.logger.Info("Service credentials saved",
			zap.String("file", credsFile),
		)
	}

	s.logger.Info("Created test service",
		zap.String("service_id", TestServiceID),
		zap.String("uuid", serviceUUID.String()),
	)

	return serviceUUID, nil
}

// seedServiceAccess grants the test service access to the sandbox merchant.
func (s *Seeder) seedServiceAccess(ctx context.Context, serviceUUID, merchantID uuid.UUID) error {
	// Check if access already exists (ServiceID is the string identifier, not UUID)
	hasAccess, err := s.queries.CheckServiceMerchantAccessByID(ctx, sqlc.CheckServiceMerchantAccessByIDParams{
		ServiceID:  TestServiceID,
		MerchantID: merchantID,
	})
	if err != nil {
		s.logger.Debug("Error checking service merchant access, will attempt to grant",
			zap.String("service_id", TestServiceID),
			zap.Error(err))
	}
	if hasAccess {
		s.logger.Debug("Service already has merchant access",
			zap.String("service_id", TestServiceID),
		)
		return nil
	}

	scopes := []string{
		"payments:create", "payments:read", "payments:void", "payments:refund",
		"payment_methods:read", "payment_methods:create",
		"subscriptions:manage", "subscriptions:read",
	}

	_, err = s.queries.GrantServiceAccess(ctx, sqlc.GrantServiceAccessParams{
		ServiceID:  serviceUUID,
		MerchantID: merchantID,
		Scopes:     scopes,
		ExpiresAt:  pgtype.Timestamptz{}, // No expiry
	})
	if err != nil {
		return fmt.Errorf("granting service access to merchant %s: %w", merchantID.String(), err)
	}

	s.logger.Info("Granted service access to merchant",
		zap.String("service_id", TestServiceID),
		zap.String("merchant_id", merchantID.String()),
	)

	return nil
}

// seedTestSubscriptions creates test subscriptions for API documentation.
func (s *Seeder) seedTestSubscriptions(ctx context.Context, merchantID uuid.UUID) error {
	placeholderPM := uuid.MustParse(PlaceholderPaymentMethodID)
	nextBilling := time.Now().AddDate(0, 1, 0) // 1 month from now

	subscriptions := []struct {
		id       string
		status   string
		amount   int64
		interval string
	}{
		{TestActiveSubscriptionID, "active", 2999, "month"},
		{TestPausedSubscriptionID, "paused", 1999, "month"},
		{TestCancelledSubscriptionID, "cancelled", 999, "month"},
	}

	for _, sub := range subscriptions {
		subID := uuid.MustParse(sub.id)

		// Check if already exists
		_, err := s.queries.GetSubscriptionByID(ctx, subID)
		if err == nil {
			s.logger.Debug("Subscription already exists",
				zap.String("id", sub.id),
			)
			continue
		}

		metadata, err := json.Marshal(map[string]string{
			"plan_name": "Test Plan",
			"seeded":    "true",
		})
		if err != nil {
			s.logger.Warn("Failed to marshal subscription metadata",
				zap.String("id", sub.id),
				zap.Error(err))
			metadata = []byte("{}")
		}

		_, err = s.queries.CreateSubscription(ctx, sqlc.CreateSubscriptionParams{
			ID:              subID,
			MerchantID:      merchantID,
			CustomerID:      TestCustomerID,
			AmountCents:     sub.amount,
			Currency:        "USD",
			IntervalValue:   1,
			IntervalUnit:    sub.interval,
			Status:          sub.status,
			PaymentMethodID: placeholderPM,
			NextBillingDate: pgtype.Date{
				Time:  nextBilling,
				Valid: sub.status == "active",
			},
			FailureRetryCount:     0,
			MaxRetries:            3,
			GatewaySubscriptionID: pgtype.Text{},
			Metadata:              metadata,
		})
		if err != nil {
			s.logger.Warn("Failed to create test subscription",
				zap.String("id", sub.id),
				zap.Error(err),
			)
			continue
		}

		s.logger.Debug("Created test subscription",
			zap.String("id", sub.id),
			zap.String("status", sub.status),
		)
	}

	return nil
}

// generateFingerprint creates a SHA256 fingerprint of the public key.
func generateFingerprint(publicKeyPEM []byte) string {
	h := sha256.New()
	h.Write(publicKeyPEM)
	return fmt.Sprintf("SHA256:%x", h.Sum(nil))[:50]
}
