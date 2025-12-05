package payment_method

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/services/authorization"
	"github.com/kevin07696/payment-service/internal/util"
	"github.com/kevin07696/payment-service/pkg/observability"
	"go.uber.org/zap"
)

// paymentMethodService implements the PaymentMethodService port
type paymentMethodService struct {
	cache               *PaymentMethodCache
	queries             sqlc.Querier
	txManager           database.TransactionManager
	browserPost         ports.BrowserPostAdapter
	serverPost          ports.ServerPostAdapter
	secretManager       ports.SecretManagerAdapter
	merchantAuthService *authorization.MerchantAuthorizationService
	logger              *zap.Logger
}

// NewPaymentMethodService creates a new payment method service
func NewPaymentMethodService(
	queries sqlc.Querier,
	txManager database.TransactionManager,
	browserPost ports.BrowserPostAdapter,
	serverPost ports.ServerPostAdapter,
	secretManager ports.SecretManagerAdapter,
	cache *PaymentMethodCache,
	logger *zap.Logger,
) ports.PaymentMethodService {
	// Create service-merchant access checker for authorization
	accessChecker := authorization.NewSQLCServiceMerchantAccessChecker(queries)

	// Create merchant authorization service with access checker
	merchantAuthService := authorization.NewMerchantAuthorizationService(logger, accessChecker)

	return &paymentMethodService{
		cache:               cache,
		queries:             queries,
		txManager:           txManager,
		browserPost:         browserPost,
		serverPost:          serverPost,
		secretManager:       secretManager,
		merchantAuthService: merchantAuthService,
		logger:              logger,
	}
}

// GetPaymentMethod retrieves a specific payment method
// Uses cache for performance (60% faster lookups)
func (s *paymentMethodService) GetPaymentMethod(ctx context.Context, paymentMethodID string) (*domain.PaymentMethod, error) {
	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "payment_method_id")
	}

	// Use cache instead of direct DB query
	pm, err := s.cache.Get(ctx, pmID)
	if err != nil {
		s.logger.Debug("Payment method not found",
			zap.String("payment_method_id", paymentMethodID),
			zap.Error(err),
		)
		return nil, domain.ErrPMNotFound
	}

	return pm, nil
}

// ListPaymentMethods lists all payment methods for a customer
func (s *paymentMethodService) ListPaymentMethods(ctx context.Context, merchantID, customerID string) ([]*domain.PaymentMethod, error) {
	// Resolve and validate merchant access
	resolvedMerchantID, err := s.merchantAuthService.ResolveMerchantID(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	// Parse merchant ID
	mid, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	params := sqlc.ListPaymentMethodsByCustomerParams{
		MerchantID: mid,
		CustomerID: customerID,
	}

	// Trace database query
	ctx, span := observability.StartDBSpan(ctx, "ListPaymentMethodsByCustomer", "payment_methods")
	defer span.End()

	dbPMs, err := s.queries.ListPaymentMethodsByCustomer(ctx, params)
	observability.EndDBSpan(span, err)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "list_payment_methods")
	}

	observability.AddDBResultAttributes(span, int64(len(dbPMs)))

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

	// Resolve and validate merchant access
	resolvedMerchantID, err := s.merchantAuthService.ResolveMerchantID(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "payment_method_id")
	}

	// Parse merchant ID
	mid, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	// Verify payment method exists and belongs to customer (use cache)
	pm, err := s.cache.Get(ctx, pmID)
	if err != nil {
		return nil, domain.ErrPMNotFound
	}

	pmMerchantID, _ := uuid.Parse(pm.MerchantID)
	if pmMerchantID != mid || pm.CustomerID != customerID {
		return nil, domain.ErrPMNotBelongToCustomer
	}

	// Update status
	if isActive {
		err = s.queries.ActivatePaymentMethod(ctx, sqlc.ActivatePaymentMethodParams{
			ID:           pmID,
			StatusReason: pgtype.Text{String: domain.StatusReasonManualRevoke, Valid: true},
		})
	} else {
		err = s.queries.RevokePaymentMethod(ctx, sqlc.RevokePaymentMethodParams{
			ID:           pmID,
			StatusReason: pgtype.Text{String: domain.StatusReasonManualRevoke, Valid: true},
		})
	}

	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "update_payment_method_status")
	}

	// Invalidate cache since we updated the payment method
	s.cache.Invalidate(pmID)

	// Fetch updated payment method from DB (cache is stale)
	updated, err := s.queries.GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_payment_method")
	}

	s.logger.Info("Payment method status updated",
		zap.String("payment_method_id", paymentMethodID),
		zap.Bool("is_active", isActive),
	)

	return sqlcPaymentMethodToDomain(&updated), nil
}

// DeletePaymentMethod hard deletes a payment method
// FK RESTRICT on transactions will prevent deletion if transactions exist
// FK SET NULL on subscriptions will nullify payment_method_id
func (s *paymentMethodService) DeletePaymentMethod(ctx context.Context, paymentMethodID string) error {
	s.logger.Info("Deleting payment method",
		zap.String("payment_method_id", paymentMethodID),
	)

	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return domain.ErrValidationInvalidUUID.WithDetail("field", "payment_method_id")
	}

	// Hard delete (FK RESTRICT will fail if transactions exist)
	err = s.queries.HardDeletePaymentMethod(ctx, pmID)
	if err != nil {
		return domain.ErrDatabaseError.WithDetail("operation", "delete_payment_method")
	}

	// Invalidate cache since we deleted the payment method
	s.cache.Invalidate(pmID)

	s.logger.Info("Payment method deleted (hard delete)",
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

	// Resolve and validate merchant access
	resolvedMerchantID, err := s.merchantAuthService.ResolveMerchantID(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	pmID, err := uuid.Parse(paymentMethodID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "payment_method_id")
	}

	// Parse merchant ID
	mid, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	// Verify payment method exists and belongs to customer (use cache)
	pm, err := s.cache.Get(ctx, pmID)
	if err != nil {
		return nil, domain.ErrPMNotFound
	}

	pmMerchantID, _ := uuid.Parse(pm.MerchantID)
	if pmMerchantID != mid || pm.CustomerID != customerID {
		return nil, domain.ErrPMNotBelongToCustomer
	}

	if !pm.IsActive() {
		return nil, domain.ErrPMInactive
	}

	var paymentMethod *domain.PaymentMethod
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Unset all defaults for this customer
		err := q.SetPaymentMethodAsDefault(ctx, sqlc.SetPaymentMethodAsDefaultParams{
			MerchantID: mid,
			CustomerID: customerID,
		})
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "unset_defaults")
		}

		// Set this one as default
		err = q.MarkPaymentMethodAsDefault(ctx, pmID)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "set_default")
		}

		// Fetch updated payment method (within transaction)
		updated, err := q.GetPaymentMethodByID(ctx, pmID)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "get_payment_method")
		}

		paymentMethod = sqlcPaymentMethodToDomain(&updated)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Invalidate cache for all payment methods for this customer
	// (multiple payment methods may have been updated - default flag)
	s.cache.InvalidateByCustomer(customerID)

	s.logger.Info("Default payment method set",
		zap.String("payment_method_id", paymentMethod.ID),
	)

	return paymentMethod, nil
}

// Helper functions

func sqlcPaymentMethodToDomain(dbPM *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
	pm := &domain.PaymentMethod{
		ID:           dbPM.ID.String(),
		MerchantID:   dbPM.MerchantID.String(),
		CustomerID:   dbPM.CustomerID,
		PaymentType:  domain.PaymentMethodType(dbPM.PaymentType),
		PaymentToken: dbPM.Bric,
		LastFour:     dbPM.LastFour,
		IsDefault:    dbPM.IsDefault.Bool,
		Status:       domain.PaymentMethodStatus(dbPM.Status),
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

	// Status reason and timestamp
	if dbPM.StatusReason.Valid {
		pm.StatusReason = &dbPM.StatusReason.String
	}

	if dbPM.StatusChangedAt.Valid {
		pm.StatusChangedAt = &dbPM.StatusChangedAt.Time
	}

	// ACH prenote fields
	if dbPM.PrenoteStatus.Valid {
		pm.PrenoteStatus = &dbPM.PrenoteStatus.String
	}

	if dbPM.PrenoteAttempts.Valid {
		attempts := int(dbPM.PrenoteAttempts.Int32)
		pm.PrenoteAttempts = &attempts
	}

	if dbPM.VerifiedAt.Valid {
		pm.VerifiedAt = &dbPM.VerifiedAt.Time
	}

	// ReturnCount is NOT NULL DEFAULT 0, so always present
	returnCount := int(dbPM.ReturnCount)
	pm.ReturnCount = &returnCount

	return pm
}

// SaveCreditCardFromCallback saves a credit card payment method from Browser Post storage callback
func (s *paymentMethodService) SaveCreditCardFromCallback(ctx context.Context, req *ports.SaveCreditCardFromCallbackRequest) (*domain.PaymentMethod, error) {
	s.logger.Info("Saving credit card from Browser Post callback",
		zap.String("merchant_id", req.MerchantID),
		zap.String("customer_id", req.CustomerID),
	)

	if req.CustomerID == "" {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "customer_id")
	}
	if req.BRIC == "" {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "bric")
	}

	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	lastFour := domain.ExtractLastFour(req.MaskedAccountNbr)
	if lastFour == "" {
		return nil, domain.ErrPMInvalid.WithDetail("reason", "unable to extract last four digits")
	}

	expDate := domain.ParseExpirationDateMMYY(req.ExpirationDate)
	cardBrand := domain.CardBrandFromEPXCode(req.CardTypeCode)

	pmID := uuid.New()
	dbPM, err := s.queries.CreatePaymentMethod(ctx, sqlc.CreatePaymentMethodParams{
		ID:          pmID,
		MerchantID:  merchantID,
		CustomerID:  req.CustomerID,
		Bric:        req.BRIC,
		PaymentType: string(domain.PaymentMethodTypeCreditCard),
		LastFour:    lastFour,
		CardBrand: func() pgtype.Text {
			if cardBrand.IsKnown() {
				return pgtype.Text{String: cardBrand.String(), Valid: true}
			}
			return pgtype.Text{}
		}(),
		CardExpMonth: func() pgtype.Int4 {
			if expDate != nil {
				return pgtype.Int4{Int32: int32(expDate.Month), Valid: true}
			}
			return pgtype.Int4{}
		}(),
		CardExpYear: func() pgtype.Int4 {
			if expDate != nil {
				return pgtype.Int4{Int32: int32(expDate.Year), Valid: true}
			}
			return pgtype.Int4{}
		}(),
		BankName:      pgtype.Text{},
		AccountType:   pgtype.Text{},
		IsDefault:     pgtype.Bool{Bool: false, Valid: true},
		Status:        string(domain.PaymentMethodStatusActive), // Credit cards are immediately active
		PrenoteStatus: pgtype.Text{String: "not_required", Valid: true}, // Credit cards don't need prenote
	})
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "create_payment_method")
	}

	s.logger.Info("Credit card payment method saved",
		zap.String("payment_method_id", pmID.String()),
		zap.String("customer_id", req.CustomerID),
		zap.String("last_four", lastFour),
	)

	return sqlcPaymentMethodToDomain(&dbPM), nil
}

// SaveACHFromCallback saves an ACH payment method from Browser Post storage callback
// Note: This only saves the payment method. Call SendPrenote separately.
func (s *paymentMethodService) SaveACHFromCallback(ctx context.Context, req *ports.SaveACHFromCallbackRequest) (*domain.PaymentMethod, error) {
	s.logger.Info("Saving ACH account from Browser Post callback",
		zap.String("merchant_id", req.MerchantID),
		zap.String("customer_id", req.CustomerID),
		zap.String("transaction_type", string(req.TransactionType)),
	)

	if req.CustomerID == "" {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "customer_id")
	}
	if req.BRIC == "" {
		return nil, domain.ErrValidationMissingField.WithDetail("field", "bric")
	}

	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	accountType := "checking"
	if !req.TransactionType.IsCheckingAccount() {
		accountType = "savings"
	}

	lastFour := domain.ExtractLastFour(req.MaskedAccountNbr)
	if lastFour == "" {
		return nil, domain.ErrPMInvalid.WithDetail("reason", "unable to extract last four digits")
	}

	pmID := uuid.New()
	dbPM, err := s.queries.CreatePaymentMethod(ctx, sqlc.CreatePaymentMethodParams{
		ID:                 pmID,
		MerchantID:         merchantID,
		CustomerID:         req.CustomerID,
		Bric:               req.BRIC,
		PaymentType:        string(domain.PaymentMethodTypeACH),
		LastFour:           lastFour,
		CardBrand:          pgtype.Text{},
		CardExpMonth:       pgtype.Int4{},
		CardExpYear:        pgtype.Int4{},
		BankName:      pgtype.Text{},
		AccountType:   pgtype.Text{String: accountType, Valid: true},
		IsDefault:     pgtype.Bool{Bool: false, Valid: true},
		Status:        string(domain.PaymentMethodStatusPending), // ACH starts pending verification
		PrenoteStatus: pgtype.Text{String: "pending", Valid: true}, // Prenote needs to be sent
	})
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "create_payment_method")
	}

	s.logger.Info("ACH payment method created (unverified)",
		zap.String("payment_method_id", pmID.String()),
		zap.String("customer_id", req.CustomerID),
		zap.String("account_type", accountType),
	)

	return sqlcPaymentMethodToDomain(&dbPM), nil
}

// SendPrenote sends a prenote transaction for ACH verification
func (s *paymentMethodService) SendPrenote(ctx context.Context, req *ports.SendPrenoteRequest) error {
	s.logger.Info("Sending ACH prenote",
		zap.String("payment_method_id", req.PaymentMethodID),
		zap.String("account_type", req.AccountType),
	)

	// Input validation (P1-6)
	if req.BRIC == "" {
		return domain.ErrValidationMissingField.WithDetail("field", "bric")
	}
	if req.MerchantID == "" {
		return domain.ErrMerchantRequired
	}
	if req.PaymentMethodID == "" {
		return domain.ErrValidationMissingField.WithDetail("field", "payment_method_id")
	}
	if req.CustomerID == "" {
		return domain.ErrValidationMissingField.WithDetail("field", "customer_id")
	}
	if req.AccountType == "" {
		return domain.ErrValidationMissingField.WithDetail("field", "account_type")
	}

	// Validate account type using domain constant (P0-3 + P1-6)
	accountType := domain.ACHAccountType(strings.ToLower(req.AccountType))
	if !accountType.IsValid() {
		return domain.ErrPMInvalidType.WithDetail("account_type", req.AccountType)
	}

	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	pmID, err := uuid.Parse(req.PaymentMethodID)
	if err != nil {
		return domain.ErrValidationInvalidUUID.WithDetail("field", "payment_method_id")
	}

	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return domain.ErrMerchantNotFoundTyped
	}

	// Merchant active status validation (P1-5)
	if !merchant.IsActive {
		return domain.ErrMerchantInactiveTyped
	}

	// Case-insensitive account type check using domain constant (P0-3)
	prenoteType := ports.TransactionTypeACHPreNoteDebit
	if accountType == domain.ACHAccountTypeSavings {
		prenoteType = ports.TransactionTypeACHSavingsPreNoteDebit
	}

	prenoteID := uuid.New()
	prenoteTranNbr := util.UUIDToEPXTranNbr(prenoteID)

	cardEntryMethod := "Z"
	stdEntryClass := "WEB"
	industryType := "E"

	prenoteReq := &ports.ServerPostRequest{
		CustNbr:          merchant.CustNbr,
		MerchNbr:         merchant.MerchNbr,
		DBAnbr:           merchant.DbaNbr,
		TerminalNbr:      merchant.TerminalNbr,
		TransactionType:  prenoteType,
		Amount:           "0.00",
		TranNbr:          prenoteTranNbr,
		TranGroup:        "PRENOTE",
		OriginalAuthGUID: req.BRIC, // Use OriginalAuthGUID for stored BRIC reference
		PaymentType:      ports.PaymentMethodTypeACH,
		CardEntryMethod:  &cardEntryMethod,
		StdEntryClass:    &stdEntryClass,
		IndustryType:     &industryType,
	}

	prenoteResp, err := s.serverPost.ProcessTransaction(ctx, prenoteReq)
	if err != nil {
		return domain.ErrGatewayError.WithDetail("operation", "send_prenote")
	}

	// Validate EPX response code (P0-2)
	if prenoteResp.AuthResp != "00" {
		return domain.ErrGatewayDeclined.WithDetail("auth_resp", prenoteResp.AuthResp).WithDetail("auth_resp_text", prenoteResp.AuthRespText)
	}

	// Wrap DB operations in transaction for atomicity (P0-1)
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Create prenote transaction record (linked via payment_method_id FK)
		_, txErr := q.CreateTransaction(ctx, sqlc.CreateTransactionParams{
			ID:                  prenoteID,
			MerchantID:          merchantID,
			CustomerID:          pgtype.Text{String: req.CustomerID, Valid: true},
			AmountCents:         0,
			Currency:            "USD",
			Type:                "PRENOTE",
			PaymentMethodType:   "ach",
			PaymentMethodID:     pgtype.UUID{Bytes: pmID, Valid: true},
			TranNbr:             pgtype.Text{String: prenoteTranNbr, Valid: true},
			AuthGuid:            pgtype.Text{String: prenoteResp.AuthGUID, Valid: true},
			AuthResp:            pgtype.Text{String: prenoteResp.AuthResp, Valid: true},
			AuthCode:            pgtype.Text{String: prenoteResp.AuthCode, Valid: true},
			AuthCardType:        pgtype.Text{},
			Metadata:            []byte("{}"),
			ParentTransactionID: pgtype.UUID{},
			ProcessedAt:         pgtype.Timestamptz{},
		})
		if txErr != nil {
			s.logger.Error("Failed to create prenote transaction record", zap.Error(txErr))
			return domain.ErrDatabaseError.WithDetail("operation", "create_prenote_transaction")
		}

		// Update prenote status to 'sent' on success
		txErr = q.UpdatePrenoteStatusSuccess(ctx, pmID)
		if txErr != nil {
			s.logger.Error("Failed to update prenote status", zap.Error(txErr))
			return domain.ErrDatabaseError.WithDetail("operation", "update_prenote_status")
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Invalidate cache after status update (P0-4)
	s.cache.Invalidate(pmID)

	s.logger.Info("ACH prenote sent successfully",
		zap.String("payment_method_id", req.PaymentMethodID),
		zap.String("prenote_id", prenoteID.String()),
		zap.String("auth_resp", prenoteResp.AuthResp),
	)

	return nil
}

