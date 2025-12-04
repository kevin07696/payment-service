// Package seed provides database seeding for development/sandbox environments.
// Auto-seeds a default merchant using EPX sandbox credentials from environment.
package seed

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/ports"
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
			return err
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

