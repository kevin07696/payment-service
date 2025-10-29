package payment_method

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

// paymentMethodService implements the PaymentMethodService port
type paymentMethodService struct {
	db            *database.PostgreSQLAdapter
	browserPost   adapterports.BrowserPostAdapter
	serverPost    adapterports.ServerPostAdapter
	secretManager adapterports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewPaymentMethodService creates a new payment method service
func NewPaymentMethodService(
	db *database.PostgreSQLAdapter,
	browserPost adapterports.BrowserPostAdapter,
	serverPost adapterports.ServerPostAdapter,
	secretManager adapterports.SecretManagerAdapter,
	logger *zap.Logger,
) ports.PaymentMethodService {
	return &paymentMethodService{
		db:            db,
		browserPost:   browserPost,
		serverPost:    serverPost,
		secretManager: secretManager,
		logger:        logger,
	}
}

// SavePaymentMethod tokenizes and saves a payment method
func (s *paymentMethodService) SavePaymentMethod(ctx context.Context, req *ports.SavePaymentMethodRequest) (*domain.PaymentMethod, error) {
	s.logger.Info("Saving payment method",
		zap.String("agent_id", req.AgentID),
		zap.String("customer_id", req.CustomerID),
		zap.String("payment_type", string(req.PaymentType)),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getPaymentMethodByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing payment method",
				zap.String("payment_method_id", existing.ID),
			)
			return existing, nil
		}
	}

	// Validate payment token
	if req.PaymentToken == "" {
		return nil, fmt.Errorf("payment_token is required")
	}

	// Validate last four digits
	if len(req.LastFour) != 4 {
		return nil, fmt.Errorf("last_four must be exactly 4 digits")
	}

	// Type-specific validation
	if req.PaymentType == domain.PaymentMethodTypeCreditCard {
		if req.CardBrand == nil || req.CardExpMonth == nil || req.CardExpYear == nil {
			return nil, fmt.Errorf("card details (brand, exp_month, exp_year) are required for credit cards")
		}
	} else if req.PaymentType == domain.PaymentMethodTypeACH {
		if req.BankName == nil || req.AccountType == nil {
			return nil, fmt.Errorf("bank details (bank_name, account_type) are required for ACH")
		}
	}

	var paymentMethod *domain.PaymentMethod
	err := s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// If this is set as default, unset all other defaults first
		if req.IsDefault {
			err := q.SetPaymentMethodAsDefault(ctx, sqlc.SetPaymentMethodAsDefaultParams{
				AgentID:    req.AgentID,
				CustomerID: req.CustomerID,
			})
			if err != nil {
				s.logger.Warn("Failed to unset existing defaults", zap.Error(err))
			}
		}

		// Create payment method
		params := sqlc.CreatePaymentMethodParams{
			ID:           uuid.New(),
			AgentID:      req.AgentID,
			CustomerID:   req.CustomerID,
			PaymentType:  string(req.PaymentType),
			PaymentToken: req.PaymentToken,
			LastFour:     req.LastFour,
			CardBrand:    toNullableText(req.CardBrand),
			CardExpMonth: toNullableInt32(req.CardExpMonth),
			CardExpYear:  toNullableInt32(req.CardExpYear),
			BankName:     toNullableText(req.BankName),
			AccountType:  toNullableText(req.AccountType),
			IsDefault:    pgtype.Bool{Bool: req.IsDefault, Valid: true},
			IsActive:     pgtype.Bool{Bool: true, Valid: true},
			IsVerified:   pgtype.Bool{Bool: req.PaymentType == domain.PaymentMethodTypeCreditCard, Valid: true}, // Credit cards don't need verification
		}

		dbPM, err := q.CreatePaymentMethod(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create payment method: %w", err)
		}

		paymentMethod = sqlcPaymentMethodToDomain(&dbPM)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Payment method saved",
		zap.String("payment_method_id", paymentMethod.ID),
		zap.Bool("is_default", paymentMethod.IsDefault),
	)

	return paymentMethod, nil
}

// GetPaymentMethod retrieves a specific payment method
func (s *paymentMethodService) GetPaymentMethod(ctx context.Context, paymentMethodID string) (*domain.PaymentMethod, error) {
	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	dbPM, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		s.logger.Debug("Payment method not found",
			zap.String("payment_method_id", paymentMethodID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("payment method not found: %w", err)
	}

	return sqlcPaymentMethodToDomain(&dbPM), nil
}

// ListPaymentMethods lists all payment methods for a customer
func (s *paymentMethodService) ListPaymentMethods(ctx context.Context, agentID, customerID string) ([]*domain.PaymentMethod, error) {
	params := sqlc.ListPaymentMethodsByCustomerParams{
		AgentID:    agentID,
		CustomerID: customerID,
	}

	dbPMs, err := s.db.Queries().ListPaymentMethodsByCustomer(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list payment methods: %w", err)
	}

	paymentMethods := make([]*domain.PaymentMethod, len(dbPMs))
	for i, dbPM := range dbPMs {
		paymentMethods[i] = sqlcPaymentMethodToDomain(&dbPM)
	}

	return paymentMethods, nil
}

// UpdatePaymentMethodStatus updates the active status of a payment method
func (s *paymentMethodService) UpdatePaymentMethodStatus(ctx context.Context, paymentMethodID, agentID, customerID string, isActive bool) (*domain.PaymentMethod, error) {
	action := "deactivating"
	if isActive {
		action = "activating"
	}

	s.logger.Info("Updating payment method status",
		zap.String("payment_method_id", paymentMethodID),
		zap.String("action", action),
		zap.Bool("is_active", isActive),
	)

	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	// Verify payment method exists and belongs to customer
	pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, fmt.Errorf("payment method not found: %w", err)
	}

	if pm.AgentID != agentID || pm.CustomerID != customerID {
		return nil, fmt.Errorf("payment method does not belong to customer")
	}

	// Update status
	if isActive {
		err = s.db.Queries().ActivatePaymentMethod(ctx, pmID)
	} else {
		err = s.db.Queries().DeactivatePaymentMethod(ctx, pmID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update payment method status: %w", err)
	}

	// Fetch updated payment method
	updated, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated payment method: %w", err)
	}

	s.logger.Info("Payment method status updated",
		zap.String("payment_method_id", paymentMethodID),
		zap.Bool("is_active", isActive),
	)

	return sqlcPaymentMethodToDomain(&updated), nil
}

// DeletePaymentMethod soft deletes a payment method (sets deleted_at)
func (s *paymentMethodService) DeletePaymentMethod(ctx context.Context, paymentMethodID string) error {
	s.logger.Info("Deleting payment method",
		zap.String("payment_method_id", paymentMethodID),
	)

	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	// Soft delete (sets deleted_at timestamp)
	err = s.db.Queries().DeletePaymentMethod(ctx, pmID)
	if err != nil {
		return fmt.Errorf("failed to delete payment method: %w", err)
	}

	s.logger.Info("Payment method deleted (soft delete)",
		zap.String("payment_method_id", paymentMethodID),
	)

	return nil
}

// SetDefaultPaymentMethod marks a payment method as default
func (s *paymentMethodService) SetDefaultPaymentMethod(ctx context.Context, paymentMethodID, agentID, customerID string) (*domain.PaymentMethod, error) {
	s.logger.Info("Setting default payment method",
		zap.String("payment_method_id", paymentMethodID),
		zap.String("customer_id", customerID),
	)

	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	// Verify payment method exists and belongs to customer
	pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, fmt.Errorf("payment method not found: %w", err)
	}

	if pm.AgentID != agentID || pm.CustomerID != customerID {
		return nil, fmt.Errorf("payment method does not belong to customer")
	}

	if !pm.IsActive.Valid || !pm.IsActive.Bool {
		return nil, fmt.Errorf("cannot set inactive payment method as default")
	}

	var paymentMethod *domain.PaymentMethod
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		// Unset all defaults for this customer
		err := q.SetPaymentMethodAsDefault(ctx, sqlc.SetPaymentMethodAsDefaultParams{
			AgentID:    agentID,
			CustomerID: customerID,
		})
		if err != nil {
			return fmt.Errorf("failed to unset existing defaults: %w", err)
		}

		// Set this one as default
		err = q.MarkPaymentMethodAsDefault(ctx, pmID)
		if err != nil {
			return fmt.Errorf("failed to set as default: %w", err)
		}

		// Fetch updated payment method
		updated, err := q.GetPaymentMethodByID(ctx, pmID)
		if err != nil {
			return fmt.Errorf("failed to fetch updated payment method: %w", err)
		}

		paymentMethod = sqlcPaymentMethodToDomain(&updated)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Default payment method set",
		zap.String("payment_method_id", paymentMethod.ID),
	)

	return paymentMethod, nil
}

// VerifyACHAccount sends pre-note for ACH verification
func (s *paymentMethodService) VerifyACHAccount(ctx context.Context, req *ports.VerifyACHAccountRequest) error {
	s.logger.Info("Verifying ACH account",
		zap.String("payment_method_id", req.PaymentMethodID),
	)

	pmID, err := uuid.Parse(req.PaymentMethodID)
	if err != nil {
		return fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	// Get payment method
	pm, err := s.db.Queries().GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return fmt.Errorf("payment method not found: %w", err)
	}

	// Verify ownership
	if pm.AgentID != req.AgentID || pm.CustomerID != req.CustomerID {
		return fmt.Errorf("payment method does not belong to customer")
	}

	// Verify it's ACH
	if pm.PaymentType != string(domain.PaymentMethodTypeACH) {
		return fmt.Errorf("payment method is not ACH type")
	}

	// Verify it's not already verified
	if pm.IsVerified.Valid && pm.IsVerified.Bool {
		s.logger.Info("ACH account already verified",
			zap.String("payment_method_id", req.PaymentMethodID),
		)
		return nil
	}

	// Get agent credentials
	agent, err := s.db.Queries().GetAgentByAgentID(ctx, req.AgentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	if !agent.IsActive.Valid || !agent.IsActive.Bool {
		return fmt.Errorf("agent is not active")
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, agent.MacSecretPath)
	if err != nil {
		return fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Send pre-note transaction through EPX
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         agent.CustNbr,
		MerchNbr:        agent.MerchNbr,
		DBAnbr:          agent.DbaNbr,
		TerminalNbr:     agent.TerminalNbr,
		TransactionType: adapterports.TransactionTypePreNote,
		Amount:          "0.00", // Pre-note is $0
		PaymentType:     adapterports.PaymentMethodTypeACH,
		AuthGUID:        pm.PaymentToken,
		TranNbr:         uuid.New().String(),
		TranGroup:       uuid.New().String(),
		CustomerID:      req.CustomerID,
	}

	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX pre-note failed", zap.Error(err))
		return fmt.Errorf("failed to send pre-note: %w", err)
	}

	if !epxResp.IsApproved {
		return fmt.Errorf("pre-note was declined: %s", epxResp.AuthRespText)
	}

	// Mark as verified
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		err := q.MarkPaymentMethodVerified(ctx, pmID)
		if err != nil {
			return fmt.Errorf("failed to mark as verified: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	s.logger.Info("ACH account verified",
		zap.String("payment_method_id", req.PaymentMethodID),
	)

	return nil
}

// getPaymentMethodByIdempotencyKey retrieves a payment method by idempotency key
func (s *paymentMethodService) getPaymentMethodByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentMethod, error) {
	// Note: This would require adding idempotency_key to payment_methods table
	// For now, returning not found error
	return nil, fmt.Errorf("payment method not found")
}

// Helper functions

func sqlcPaymentMethodToDomain(dbPM *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
	pm := &domain.PaymentMethod{
		ID:           dbPM.ID.String(),
		AgentID:      dbPM.AgentID,
		CustomerID:   dbPM.CustomerID,
		PaymentType:  domain.PaymentMethodType(dbPM.PaymentType),
		PaymentToken: dbPM.PaymentToken,
		LastFour:     dbPM.LastFour,
		IsDefault:    dbPM.IsDefault.Bool,
		IsActive:     dbPM.IsActive.Bool,
		IsVerified:   dbPM.IsVerified.Bool,
		CreatedAt:    dbPM.CreatedAt,
		UpdatedAt:    dbPM.UpdatedAt,
	}

	if dbPM.CardBrand.Valid {
		pm.CardBrand = &dbPM.CardBrand.String
	}

	if dbPM.CardExpMonth.Valid {
		expMonth := int(dbPM.CardExpMonth.Int32)
		pm.CardExpMonth = &expMonth
	}

	if dbPM.CardExpYear.Valid {
		expYear := int(dbPM.CardExpYear.Int32)
		pm.CardExpYear = &expYear
	}

	if dbPM.BankName.Valid {
		pm.BankName = &dbPM.BankName.String
	}

	if dbPM.AccountType.Valid {
		pm.AccountType = &dbPM.AccountType.String
	}

	if dbPM.LastUsedAt.Valid {
		pm.LastUsedAt = &dbPM.LastUsedAt.Time
	}

	return pm
}

func toNullableText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func toNullableInt32(i *int) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: int32(*i), Valid: true}
}
