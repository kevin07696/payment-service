package payment_method

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/util"
	"go.uber.org/zap"
)

// paymentMethodService implements the PaymentMethodService port
type paymentMethodService struct {
	queries       sqlc.Querier
	txManager     database.TransactionManager
	browserPost   adapterports.BrowserPostAdapter
	serverPost    adapterports.ServerPostAdapter
	bricStorage   adapterports.BRICStorageAdapter
	secretManager adapterports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewPaymentMethodService creates a new payment method service
func NewPaymentMethodService(
	queries sqlc.Querier,
	txManager database.TransactionManager,
	browserPost adapterports.BrowserPostAdapter,
	serverPost adapterports.ServerPostAdapter,
	bricStorage adapterports.BRICStorageAdapter,
	secretManager adapterports.SecretManagerAdapter,
	logger *zap.Logger,
) ports.PaymentMethodService {
	return &paymentMethodService{
		queries:       queries,
		txManager:     txManager,
		browserPost:   browserPost,
		serverPost:    serverPost,
		bricStorage:   bricStorage,
		secretManager: secretManager,
		logger:        logger,
	}
}

// GetPaymentMethod retrieves a specific payment method
func (s *paymentMethodService) GetPaymentMethod(ctx context.Context, paymentMethodID string) (*domain.PaymentMethod, error) {
	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	dbPM, err := s.queries.GetPaymentMethodByID(ctx, pmID)
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
func (s *paymentMethodService) ListPaymentMethods(ctx context.Context, merchantID, customerID string) ([]*domain.PaymentMethod, error) {
	// Parse merchant ID
	mid, err := uuid.Parse(merchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	// Parse customer ID
	cid, err := uuid.Parse(customerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer_id format: %w", err)
	}

	params := sqlc.ListPaymentMethodsByCustomerParams{
		MerchantID: mid,
		CustomerID: cid,
	}

	dbPMs, err := s.queries.ListPaymentMethodsByCustomer(ctx, params)
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
func (s *paymentMethodService) UpdatePaymentMethodStatus(ctx context.Context, paymentMethodID, merchantID, customerID string, isActive bool) (*domain.PaymentMethod, error) {
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

	// Parse merchant ID
	mid, err := uuid.Parse(merchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	// Parse customer ID
	cid, err := uuid.Parse(customerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer_id format: %w", err)
	}

	// Verify payment method exists and belongs to customer
	pm, err := s.queries.GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, fmt.Errorf("payment method not found: %w", err)
	}

	if pm.MerchantID != mid || pm.CustomerID != cid {
		return nil, fmt.Errorf("payment method does not belong to customer")
	}

	// Update status
	if isActive {
		err = s.queries.ActivatePaymentMethod(ctx, pmID)
	} else {
		err = s.queries.DeactivatePaymentMethod(ctx, pmID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update payment method status: %w", err)
	}

	// Fetch updated payment method
	updated, err := s.queries.GetPaymentMethodByID(ctx, pmID)
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
	err = s.queries.DeletePaymentMethod(ctx, pmID)
	if err != nil {
		return fmt.Errorf("failed to delete payment method: %w", err)
	}

	s.logger.Info("Payment method deleted (soft delete)",
		zap.String("payment_method_id", paymentMethodID),
	)

	return nil
}

// SetDefaultPaymentMethod marks a payment method as default
func (s *paymentMethodService) SetDefaultPaymentMethod(ctx context.Context, paymentMethodID, merchantID, customerID string) (*domain.PaymentMethod, error) {
	s.logger.Info("Setting default payment method",
		zap.String("payment_method_id", paymentMethodID),
		zap.String("customer_id", customerID),
	)

	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	// Parse merchant ID
	mid, err := uuid.Parse(merchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	// Parse customer ID
	cid, err := uuid.Parse(customerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer_id format: %w", err)
	}

	// Verify payment method exists and belongs to customer
	pm, err := s.queries.GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, fmt.Errorf("payment method not found: %w", err)
	}

	if pm.MerchantID != mid || pm.CustomerID != cid {
		return nil, fmt.Errorf("payment method does not belong to customer")
	}

	if !pm.IsActive.Valid || !pm.IsActive.Bool {
		return nil, fmt.Errorf("cannot set inactive payment method as default")
	}

	var paymentMethod *domain.PaymentMethod
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Unset all defaults for this customer
		err := q.SetPaymentMethodAsDefault(ctx, sqlc.SetPaymentMethodAsDefaultParams{
			MerchantID: mid,
			CustomerID: cid,
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

// StoreACHAccount stores ACH account with pre-note verification
// Sends Pre-Note Debit (CKC0/CKS0) to EPX, stores GUID/BRIC with status=pending_verification
func (s *paymentMethodService) StoreACHAccount(ctx context.Context, req *ports.StoreACHAccountRequest) (*domain.PaymentMethod, error) {
	s.logger.Info("Storing ACH account with pre-note verification",
		zap.String("merchant_id", req.MerchantID),
		zap.String("customer_id", req.CustomerID),
		zap.String("account_type", req.AccountType),
	)

	// Validate merchant ID
	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	// Validate customer ID
	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer_id format: %w", err)
	}

	// Validate account type
	if req.AccountType != "CHECKING" && req.AccountType != "SAVINGS" {
		return nil, fmt.Errorf("account_type must be CHECKING or SAVINGS")
	}

	// Get merchant credentials
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merchant: %w", err)
	}

	if !merchant.IsActive {
		return nil, fmt.Errorf("merchant is not active")
	}

	// Get MAC secret for EPX authentication
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Determine transaction type based on account type
	var tranType adapterports.TransactionType
	if req.AccountType == "CHECKING" {
		tranType = adapterports.TransactionTypeACHPreNoteDebit // CKC0
	} else {
		tranType = adapterports.TransactionTypeACHSavingsPreNoteDebit // CKS0
	}

	// Parse idempotency key as UUID for transaction IDs
	// This ensures idempotency - same idempotency_key = same TRAN_NBR
	idempotencyUUID, err := uuid.Parse(req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("invalid idempotency_key format (must be UUID): %w", err)
	}

	// Convert UUID to EPX-compatible TRAN_NBR (max 10 digits, numeric only)
	tranNbr := util.UUIDToEPXTranNbr(idempotencyUUID)
	tranGroup := idempotencyUUID.String()

	// Build Server Post request for Pre-Note Debit
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         merchant.CustNbr,
		MerchNbr:        merchant.MerchNbr,
		DBAnbr:          merchant.DbaNbr,
		TerminalNbr:     merchant.TerminalNbr,
		TransactionType: tranType,
		Amount:          "0.00", // Pre-note is $0
		PaymentType:     adapterports.PaymentMethodTypeACH,
		TranNbr:         tranNbr,
		TranGroup:       tranGroup,
		CustomerID:      req.CustomerID,
		AccountNumber:   &req.AccountNumber,
		RoutingNumber:   &req.RoutingNumber,
		FirstName:       &req.FirstName,
		LastName:        &req.LastName,
		Address:         &req.Address,
		City:            &req.City,
		State:           &req.State,
		ZipCode:         &req.ZipCode,
	}

	// Send Pre-Note transaction to EPX
	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		s.logger.Error("EPX ACH pre-note failed", zap.Error(err))
		return nil, fmt.Errorf("failed to send ACH pre-note: %w", err)
	}

	if !epxResp.IsApproved {
		return nil, fmt.Errorf("ACH pre-note was declined: %s", epxResp.AuthRespText)
	}

	if epxResp.AuthGUID == "" {
		return nil, fmt.Errorf("EPX did not return AUTH_GUID for ACH account")
	}

	// Extract last four digits of account number
	lastFour := req.AccountNumber
	if len(lastFour) > 4 {
		lastFour = lastFour[len(lastFour)-4:]
	}

	// Create payment method in database with status=pending_verification
	var paymentMethod *domain.PaymentMethod
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		pmID := uuid.New()

		// Parse transaction number to UUID for prenote_transaction_id
		// Note: TranNbr is the transaction number we sent to EPX, which is a UUID string
		tranID, err := uuid.Parse(epxResp.TranNbr)
		if err != nil {
			s.logger.Warn("Failed to parse transaction number as UUID, storing without prenote_transaction_id",
				zap.String("tran_nbr", epxResp.TranNbr),
				zap.Error(err),
			)
		}

		params := sqlc.CreatePaymentMethodParams{
			ID:                 pmID,
			MerchantID:         merchantID,
			CustomerID:         customerID,
			PaymentType:        string(domain.PaymentMethodTypeACH),
			Bric:               epxResp.AuthGUID,
			LastFour:           lastFour,
			BankName:           pgtype.Text{Valid: false}, // Bank name not provided in request
			AccountType:        pgtype.Text{String: strings.ToLower(req.AccountType), Valid: true}, // Database expects lowercase
			IsDefault:          pgtype.Bool{Bool: false, Valid: true},
			IsActive:           pgtype.Bool{Bool: false, Valid: true}, // Not active until verified
			IsVerified:         pgtype.Bool{Bool: false, Valid: true},
			VerificationStatus: pgtype.Text{String: "pending", Valid: true},
		}

		// Add prenote transaction ID if we parsed it successfully
		if err == nil {
			params.PrenoteTransactionID = pgtype.UUID{Bytes: tranID, Valid: true}
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

	s.logger.Info("ACH account stored with pending verification",
		zap.String("payment_method_id", paymentMethod.ID),
		zap.String("bric", epxResp.AuthGUID),
		zap.String("tran_nbr", epxResp.TranNbr),
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
	pm, err := s.queries.GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return fmt.Errorf("payment method not found: %w", err)
	}

	// Parse merchant ID
	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return fmt.Errorf("invalid merchant_id format: %w", err)
	}

	// Parse customer ID
	cid, err := uuid.Parse(req.CustomerID)
	if err != nil {
		return fmt.Errorf("invalid customer_id format: %w", err)
	}

	// Verify ownership
	if pm.MerchantID != merchantID || pm.CustomerID != cid {
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

	// Get merchant credentials
	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return fmt.Errorf("failed to get merchant: %w", err)
	}

	if !merchant.IsActive {
		return fmt.Errorf("merchant is not active")
	}

	// Get MAC secret
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Send pre-note transaction through EPX
	epxReq := &adapterports.ServerPostRequest{
		CustNbr:         merchant.CustNbr,
		MerchNbr:        merchant.MerchNbr,
		DBAnbr:          merchant.DbaNbr,
		TerminalNbr:     merchant.TerminalNbr,
		TransactionType: adapterports.TransactionTypeACHPreNoteDebit,
		Amount:          "0.00", // Pre-note is $0
		PaymentType:     adapterports.PaymentMethodTypeACH,
		AuthGUID:        pm.Bric,
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
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
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
		MerchantID:   dbPM.MerchantID.String(),
		CustomerID:   dbPM.CustomerID.String(),
		PaymentType:  domain.PaymentMethodType(dbPM.PaymentType),
		PaymentToken: dbPM.Bric,
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

	// ACH Verification fields (from migration 009)
	if dbPM.VerificationStatus.Valid {
		pm.VerificationStatus = &dbPM.VerificationStatus.String
	}

	if dbPM.PrenoteTransactionID.Valid {
		prenoteID := uuid.UUID(dbPM.PrenoteTransactionID.Bytes).String()
		pm.PreNoteTransactionID = &prenoteID
	}

	if dbPM.VerifiedAt.Valid {
		pm.VerifiedAt = &dbPM.VerifiedAt.Time
	}

	if dbPM.VerificationFailureReason.Valid {
		pm.VerificationFailureReason = &dbPM.VerificationFailureReason.String
	}

	// ReturnCount is NOT NULL DEFAULT 0, so always present
	returnCount := int(dbPM.ReturnCount)
	pm.ReturnCount = &returnCount

	if dbPM.DeactivationReason.Valid {
		pm.DeactivationReason = &dbPM.DeactivationReason.String
	}

	if dbPM.DeactivatedAt.Valid {
		pm.DeactivatedAt = &dbPM.DeactivatedAt.Time
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
