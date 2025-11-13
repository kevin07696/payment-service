package merchant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// merchantService implements the MerchantService port
type merchantService struct {
	db            *database.PostgreSQLAdapter
	secretManager adapterports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewMerchantService creates a new merchant service
func NewMerchantService(
	db *database.PostgreSQLAdapter,
	secretManager adapterports.SecretManagerAdapter,
	logger *zap.Logger,
) ports.MerchantService {
	return &merchantService{
		db:            db,
		secretManager: secretManager,
		logger:        logger,
	}
}

// RegisterMerchant adds a new merchant to the system
func (s *merchantService) RegisterMerchant(ctx context.Context, req *ports.RegisterMerchantRequest) (*domain.Merchant, error) {
	s.logger.Info("Registering new merchant",
		zap.String("merchant_id", req.AgentID),
		zap.String("environment", string(req.Environment)),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getMerchantByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing merchant",
				zap.String("merchant_id", existing.AgentID),
			)
			return existing, nil
		}
	}

	// Validate merchant_id is unique
	exists, err := s.db.Queries().MerchantExistsBySlug(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to check merchant existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("merchant_id already exists")
	}

	// Validate EPX credentials are provided
	if req.CustNbr == "" || req.MerchNbr == "" || req.DBAnbr == "" || req.TerminalNbr == "" {
		return nil, fmt.Errorf("all EPX credentials (cust_nbr, merch_nbr, dba_nbr, terminal_nbr) are required")
	}

	// Validate MAC secret is provided
	if req.MACSecret == "" {
		return nil, fmt.Errorf("mac_secret is required")
	}

	// Generate MAC secret path
	macSecretPath := fmt.Sprintf("payment-service/merchants/%s/mac", req.AgentID)

	var merchant *domain.Merchant
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Store MAC secret in secret manager
		_, err := s.secretManager.PutSecret(ctx, macSecretPath, req.MACSecret, nil)
		if err != nil {
			return fmt.Errorf("failed to store MAC secret: %w", err)
		}

		// Create merchant in database
		params := sqlc.CreateMerchantParams{
			ID:            uuid.New(),
			Slug:          req.AgentID,
			CustNbr:       req.CustNbr,
			MerchNbr:      req.MerchNbr,
			DbaNbr:        req.DBAnbr,
			TerminalNbr:   req.TerminalNbr,
			MacSecretPath: macSecretPath,
			Environment:   string(req.Environment),
			IsActive:      true,
			Name:          req.MerchantName,
		}

		dbMerchant, err := q.CreateMerchant(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create merchant: %w", err)
		}

		merchant = sqlcMerchantToDomain(&dbMerchant)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Merchant registered successfully",
		zap.String("merchant_id", merchant.AgentID),
		zap.String("environment", string(merchant.Environment)),
	)

	return merchant, nil
}

// GetMerchant retrieves merchant credentials (internal use only)
func (s *merchantService) GetMerchant(ctx context.Context, agentID string) (*domain.Merchant, error) {
	dbMerchant, err := s.db.Queries().GetMerchantBySlug(ctx, agentID)
	if err != nil {
		s.logger.Debug("Merchant not found",
			zap.String("merchant_id", agentID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	return sqlcMerchantToDomain(&dbMerchant), nil
}

// ListMerchants lists all registered merchants
func (s *merchantService) ListMerchants(ctx context.Context, environment *domain.Environment, isActive *bool, limit, offset int) ([]*domain.Merchant, int, error) {
	var envStr pgtype.Text
	if environment != nil {
		envStr = pgtype.Text{String: string(*environment), Valid: true}
	}

	var activeFlag pgtype.Bool
	if isActive != nil {
		activeFlag = pgtype.Bool{Bool: *isActive, Valid: true}
	}

	params := sqlc.ListMerchantsParams{
		Environment: envStr,
		IsActive:    activeFlag,
		LimitVal:    int32(limit),
		OffsetVal:   int32(offset),
	}

	dbMerchants, err := s.db.Queries().ListMerchants(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list merchants: %w", err)
	}

	countParams := sqlc.CountMerchantsParams{
		Environment: envStr,
		IsActive:    activeFlag,
	}

	count, err := s.db.Queries().CountMerchants(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count merchants: %w", err)
	}

	merchants := make([]*domain.Merchant, len(dbMerchants))
	for i, dbMerchant := range dbMerchants {
		merchants[i] = sqlcMerchantToDomain(&dbMerchant)
	}

	return merchants, int(count), nil
}

// UpdateMerchant updates merchant credentials
func (s *merchantService) UpdateMerchant(ctx context.Context, req *ports.UpdateMerchantRequest) (*domain.Merchant, error) {
	s.logger.Info("Updating merchant",
		zap.String("merchant_id", req.AgentID),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getMerchantByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
	}

	// Get existing merchant
	existing, err := s.db.Queries().GetMerchantBySlug(ctx, req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	var merchant *domain.Merchant
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// If MAC secret is being rotated, update it in secret manager
		if req.MACSecret != nil {
			_, err := s.secretManager.PutSecret(ctx, existing.MacSecretPath, *req.MACSecret, nil)
			if err != nil {
				return fmt.Errorf("failed to update MAC secret: %w", err)
			}
		}

		// Build update params with defaults from existing
		params := sqlc.UpdateMerchantParams{
			ID:          existing.ID,
			CustNbr:     valueOrDefault(req.CustNbr, existing.CustNbr),
			MerchNbr:    valueOrDefault(req.MerchNbr, existing.MerchNbr),
			DbaNbr:      valueOrDefault(req.DBAnbr, existing.DbaNbr),
			TerminalNbr: valueOrDefault(req.TerminalNbr, existing.TerminalNbr),
			Environment: valueOrEnvironment(req.Environment, existing.Environment),
			Name:        valueOrDefault(req.MerchantName, existing.Name),
		}

		dbMerchant, err := q.UpdateMerchant(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to update merchant: %w", err)
		}

		merchant = sqlcMerchantToDomain(&dbMerchant)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Merchant updated successfully",
		zap.String("merchant_id", merchant.AgentID),
	)

	return merchant, nil
}

// DeactivateMerchant deactivates a merchant
func (s *merchantService) DeactivateMerchant(ctx context.Context, agentID, reason string) error {
	s.logger.Info("Deactivating merchant",
		zap.String("merchant_id", agentID),
		zap.String("reason", reason),
	)

	// Get merchant to retrieve UUID
	merchant, err := s.db.Queries().GetMerchantBySlug(ctx, agentID)
	if err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	err = s.db.Queries().DeactivateMerchant(ctx, merchant.ID)
	if err != nil {
		return fmt.Errorf("failed to deactivate merchant: %w", err)
	}

	s.logger.Info("Merchant deactivated successfully",
		zap.String("merchant_id", agentID),
	)

	return nil
}

// RotateMerchantMAC rotates MAC secret in secret manager
func (s *merchantService) RotateMerchantMAC(ctx context.Context, req *ports.RotateMerchantMACRequest) error {
	s.logger.Info("Rotating MAC secret",
		zap.String("merchant_id", req.AgentID),
	)

	// Get merchant to retrieve MAC secret path
	merchant, err := s.db.Queries().GetMerchantBySlug(ctx, req.AgentID)
	if err != nil {
		return fmt.Errorf("merchant not found: %w", err)
	}

	if !merchant.IsActive {
		return fmt.Errorf("cannot rotate MAC for inactive merchant")
	}

	// Update MAC secret in secret manager
	_, err = s.secretManager.PutSecret(ctx, merchant.MacSecretPath, req.NewMACSecret, nil)
	if err != nil {
		return fmt.Errorf("failed to rotate MAC secret: %w", err)
	}

	s.logger.Info("MAC secret rotated successfully",
		zap.String("merchant_id", req.AgentID),
	)

	return nil
}

// getMerchantByIdempotencyKey retrieves a merchant by idempotency key
func (s *merchantService) getMerchantByIdempotencyKey(ctx context.Context, key string) (*domain.Merchant, error) {
	// Note: This would require adding idempotency_key to merchants table
	// For now, returning not found error
	return nil, fmt.Errorf("merchant not found")
}

// Helper functions

func sqlcMerchantToDomain(dbMerchant *sqlc.Merchant) *domain.Merchant {
	return &domain.Merchant{
		ID:            dbMerchant.ID.String(),
		AgentID:       dbMerchant.Slug,
		CustNbr:       dbMerchant.CustNbr,
		MerchNbr:      dbMerchant.MerchNbr,
		DBAnbr:        dbMerchant.DbaNbr,
		TerminalNbr:   dbMerchant.TerminalNbr,
		MACSecretPath: dbMerchant.MacSecretPath,
		Environment:   domain.Environment(dbMerchant.Environment),
		IsActive:      dbMerchant.IsActive,
		CreatedAt:     dbMerchant.CreatedAt.Time,
		UpdatedAt:     dbMerchant.UpdatedAt.Time,
	}
}

func valueOrDefault(value *string, defaultValue string) string {
	if value != nil {
		return *value
	}
	return defaultValue
}

func valueOrEnvironment(value *domain.Environment, defaultValue string) string {
	if value != nil {
		return string(*value)
	}
	return defaultValue
}
