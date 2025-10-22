package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
)

// Service implements ports.PaymentService
type Service struct {
	db           ports.DBPort
	txRepo       ports.TransactionRepository
	gateway      ports.CreditCardGateway
	logger       ports.Logger
}

// NewService creates a new payment service
func NewService(
	db ports.DBPort,
	txRepo ports.TransactionRepository,
	gateway ports.CreditCardGateway,
	logger ports.Logger,
) *Service {
	return &Service{
		db:      db,
		txRepo:  txRepo,
		gateway: gateway,
		logger:  logger,
	}
}

// Authorize authorizes a payment without capturing funds
func (s *Service) Authorize(ctx context.Context, req ports.ServiceAuthorizeRequest) (*ports.PaymentResponse, error) {
	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.txRepo.GetByIdempotencyKey(ctx, nil, req.IdempotencyKey)
		if err == nil && existing != nil {
			s.logger.Info("returning existing transaction for idempotency key",
				ports.String("idempotency_key", req.IdempotencyKey),
				ports.String("transaction_id", existing.ID))
			return s.toPaymentResponse(existing), nil
		}
	}

	// Create transaction record
	transaction := &models.Transaction{
		ID:                uuid.New().String(),
		MerchantID:        req.MerchantID,
		CustomerID:        req.CustomerID,
		Amount:            req.Amount,
		Currency:          req.Currency,
		Status:            models.StatusPending,
		Type:              models.TypeAuthorization,
		PaymentMethodType: models.PaymentMethodCreditCard,
		IdempotencyKey:    req.IdempotencyKey,
		Metadata:          req.Metadata,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	var response *ports.PaymentResponse

	// Execute in database transaction
	err := s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Persist pending transaction
		if err := s.txRepo.Create(ctx, tx, transaction); err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		// Call gateway
		gatewayReq := &ports.AuthorizeRequest{
			Amount:         req.Amount,
			Currency:       req.Currency,
			Token:          req.Token,
			BillingInfo:    req.BillingInfo,
			Capture:        false, // Auth only
			IdempotencyKey: req.IdempotencyKey,
			Metadata:       req.Metadata,
		}

		gatewayResp, err := s.gateway.Authorize(ctx, gatewayReq)
		if err != nil {
			// Update transaction as failed
			responseCode := "ERROR"
			responseMsg := err.Error()
			_ = s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(transaction.ID),
				models.StatusFailed, nil, &responseCode, &responseMsg)

			return fmt.Errorf("gateway authorize: %w", err)
		}

		// Gateway returns status already determined
		status := gatewayResp.Status
		gatewayTxnID := gatewayResp.GatewayTransactionID
		responseCode := gatewayResp.ResponseCode
		responseMsg := gatewayResp.Message

		if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(transaction.ID),
			status, &gatewayTxnID, &responseCode, &responseMsg); err != nil {
			return fmt.Errorf("update transaction status: %w", err)
		}

		// Update in-memory transaction for response
		transaction.Status = status
		transaction.GatewayTransactionID = gatewayTxnID
		transaction.GatewayResponseCode = responseCode
		transaction.GatewayResponseMsg = responseMsg
		transaction.AVSResponse = gatewayResp.AVSResponse
		transaction.CVVResponse = gatewayResp.CVVResponse
		transaction.PaymentMethodToken = req.Token

		response = s.toPaymentResponse(transaction)
		response.IsApproved = (status == models.StatusAuthorized)
		response.IsDeclined = (status == models.StatusFailed)

		return nil
	})

	if err != nil {
		s.logger.Error("authorize failed",
			ports.String("transaction_id", transaction.ID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("authorize completed",
		ports.String("transaction_id", transaction.ID),
		ports.String("status", string(transaction.Status)),
		ports.String("gateway_response_code", transaction.GatewayResponseCode))

	return response, nil
}

// Capture captures a previously authorized payment
func (s *Service) Capture(ctx context.Context, req ports.ServiceCaptureRequest) (*ports.PaymentResponse, error) {
	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.txRepo.GetByIdempotencyKey(ctx, nil, req.IdempotencyKey)
		if err == nil && existing != nil {
			s.logger.Info("returning existing transaction for idempotency key",
				ports.String("idempotency_key", req.IdempotencyKey),
				ports.String("transaction_id", existing.ID))
			return s.toPaymentResponse(existing), nil
		}
	}

	// Get original transaction
	originalTxn, err := s.txRepo.GetByID(ctx, nil, uuid.MustParse(req.TransactionID))
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	// Validate transaction can be captured
	if originalTxn.Status != models.StatusAuthorized {
		return nil, fmt.Errorf("transaction %s is not in authorized state (current: %s)",
			req.TransactionID, originalTxn.Status)
	}

	// Determine capture amount
	captureAmount := originalTxn.Amount
	if req.Amount != nil {
		captureAmount = *req.Amount
		if captureAmount.GreaterThan(originalTxn.Amount) {
			return nil, fmt.Errorf("capture amount %s exceeds authorized amount %s",
				captureAmount, originalTxn.Amount)
		}
	}

	// Create capture transaction record
	captureTxn := &models.Transaction{
		ID:                    uuid.New().String(),
		MerchantID:            originalTxn.MerchantID,
		CustomerID:            originalTxn.CustomerID,
		Amount:                captureAmount,
		Currency:              originalTxn.Currency,
		Status:                models.StatusPending,
		Type:                  models.TypeCapture,
		PaymentMethodType:     originalTxn.PaymentMethodType,
		PaymentMethodToken:    originalTxn.PaymentMethodToken,
		GatewayTransactionID:  originalTxn.GatewayTransactionID,
		IdempotencyKey:        req.IdempotencyKey,
		Metadata:              map[string]string{"original_transaction_id": originalTxn.ID},
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	var response *ports.PaymentResponse

	// Execute in database transaction
	err = s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Persist capture transaction
		if err := s.txRepo.Create(ctx, tx, captureTxn); err != nil {
			return fmt.Errorf("create capture transaction: %w", err)
		}

		// Call gateway
		gatewayReq := &ports.CaptureRequest{
			TransactionID: originalTxn.GatewayTransactionID,
			Amount:        captureAmount,
		}

		gatewayResp, err := s.gateway.Capture(ctx, gatewayReq)
		if err != nil {
			responseCode := "ERROR"
			responseMsg := err.Error()
			_ = s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(captureTxn.ID),
				models.StatusFailed, nil, &responseCode, &responseMsg)
			return fmt.Errorf("gateway capture: %w", err)
		}

		// Gateway returns status already determined
		status := gatewayResp.Status
		gatewayTxnID := gatewayResp.GatewayTransactionID
		responseCode := gatewayResp.ResponseCode
		responseMsg := gatewayResp.Message

		if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(captureTxn.ID),
			status, &gatewayTxnID, &responseCode, &responseMsg); err != nil {
			return fmt.Errorf("update capture transaction: %w", err)
		}

		// Update original transaction status
		if status == models.StatusCaptured {
			if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(originalTxn.ID),
				models.StatusCaptured, nil, nil, nil); err != nil {
				return fmt.Errorf("update original transaction: %w", err)
			}
		}

		captureTxn.Status = status
		captureTxn.GatewayTransactionID = gatewayTxnID
		captureTxn.GatewayResponseCode = responseCode
		captureTxn.GatewayResponseMsg = responseMsg
		captureTxn.AVSResponse = gatewayResp.AVSResponse
		captureTxn.CVVResponse = gatewayResp.CVVResponse

		response = s.toPaymentResponse(captureTxn)
		response.IsApproved = (status == models.StatusCaptured)
		response.IsDeclined = (status == models.StatusFailed)

		return nil
	})

	if err != nil {
		s.logger.Error("capture failed",
			ports.String("capture_transaction_id", captureTxn.ID),
			ports.String("original_transaction_id", originalTxn.ID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("capture completed",
		ports.String("capture_transaction_id", captureTxn.ID),
		ports.String("status", string(captureTxn.Status)))

	return response, nil
}

// Sale performs authorization and capture in one step
func (s *Service) Sale(ctx context.Context, req ports.ServiceSaleRequest) (*ports.PaymentResponse, error) {
	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.txRepo.GetByIdempotencyKey(ctx, nil, req.IdempotencyKey)
		if err == nil && existing != nil {
			s.logger.Info("returning existing transaction for idempotency key",
				ports.String("idempotency_key", req.IdempotencyKey),
				ports.String("transaction_id", existing.ID))
			return s.toPaymentResponse(existing), nil
		}
	}

	// Create transaction record
	transaction := &models.Transaction{
		ID:                uuid.New().String(),
		MerchantID:        req.MerchantID,
		CustomerID:        req.CustomerID,
		Amount:            req.Amount,
		Currency:          req.Currency,
		Status:            models.StatusPending,
		Type:              models.TypeSale,
		PaymentMethodType: models.PaymentMethodCreditCard,
		IdempotencyKey:    req.IdempotencyKey,
		Metadata:          req.Metadata,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	var response *ports.PaymentResponse

	err := s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.txRepo.Create(ctx, tx, transaction); err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		// Call gateway - authorize with auto-capture
		gatewayReq := &ports.AuthorizeRequest{
			Amount:         req.Amount,
			Currency:       req.Currency,
			Token:          req.Token,
			BillingInfo:    req.BillingInfo,
			Capture:        true, // This is the key difference for Sale
			IdempotencyKey: req.IdempotencyKey,
			Metadata:       req.Metadata,
		}

		gatewayResp, err := s.gateway.Authorize(ctx, gatewayReq)
		if err != nil {
			responseCode := "ERROR"
			responseMsg := err.Error()
			_ = s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(transaction.ID),
				models.StatusFailed, nil, &responseCode, &responseMsg)
			return fmt.Errorf("gateway sale: %w", err)
		}

		// Gateway returns status already determined
		status := gatewayResp.Status
		gatewayTxnID := gatewayResp.GatewayTransactionID
		responseCode := gatewayResp.ResponseCode
		responseMsg := gatewayResp.Message

		if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(transaction.ID),
			status, &gatewayTxnID, &responseCode, &responseMsg); err != nil {
			return fmt.Errorf("update transaction status: %w", err)
		}

		transaction.Status = status
		transaction.GatewayTransactionID = gatewayTxnID
		transaction.GatewayResponseCode = responseCode
		transaction.GatewayResponseMsg = responseMsg
		transaction.AVSResponse = gatewayResp.AVSResponse
		transaction.CVVResponse = gatewayResp.CVVResponse
		transaction.PaymentMethodToken = req.Token

		response = s.toPaymentResponse(transaction)
		response.IsApproved = (status == models.StatusCaptured)
		response.IsDeclined = (status == models.StatusFailed)

		return nil
	})

	if err != nil {
		s.logger.Error("sale failed",
			ports.String("transaction_id", transaction.ID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("sale completed",
		ports.String("transaction_id", transaction.ID),
		ports.String("status", string(transaction.Status)))

	return response, nil
}

// Void voids a previously authorized or captured transaction
func (s *Service) Void(ctx context.Context, req ports.ServiceVoidRequest) (*ports.PaymentResponse, error) {
	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.txRepo.GetByIdempotencyKey(ctx, nil, req.IdempotencyKey)
		if err == nil && existing != nil {
			s.logger.Info("returning existing transaction for idempotency key",
				ports.String("idempotency_key", req.IdempotencyKey))
			return s.toPaymentResponse(existing), nil
		}
	}

	// Get original transaction
	originalTxn, err := s.txRepo.GetByID(ctx, nil, uuid.MustParse(req.TransactionID))
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	// Validate transaction can be voided
	if originalTxn.Status != models.StatusAuthorized && originalTxn.Status != models.StatusCaptured {
		return nil, fmt.Errorf("transaction %s cannot be voided (status: %s)",
			req.TransactionID, originalTxn.Status)
	}

	// Create void transaction
	voidTxn := &models.Transaction{
		ID:                   uuid.New().String(),
		MerchantID:           originalTxn.MerchantID,
		CustomerID:           originalTxn.CustomerID,
		Amount:               originalTxn.Amount,
		Currency:             originalTxn.Currency,
		Status:               models.StatusPending,
		Type:                 models.TypeVoid,
		PaymentMethodType:    originalTxn.PaymentMethodType,
		GatewayTransactionID: originalTxn.GatewayTransactionID,
		IdempotencyKey:       req.IdempotencyKey,
		Metadata:             map[string]string{"original_transaction_id": originalTxn.ID},
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	var response *ports.PaymentResponse

	err = s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.txRepo.Create(ctx, tx, voidTxn); err != nil {
			return fmt.Errorf("create void transaction: %w", err)
		}

		gatewayResp, err := s.gateway.Void(ctx, originalTxn.GatewayTransactionID)
		if err != nil {
			responseCode := "ERROR"
			responseMsg := err.Error()
			_ = s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(voidTxn.ID),
				models.StatusFailed, nil, &responseCode, &responseMsg)
			return fmt.Errorf("gateway void: %w", err)
		}

		// Gateway returns status already determined
		status := gatewayResp.Status
		gatewayTxnID := gatewayResp.GatewayTransactionID
		responseCode := gatewayResp.ResponseCode
		responseMsg := gatewayResp.Message

		if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(voidTxn.ID),
			status, &gatewayTxnID, &responseCode, &responseMsg); err != nil {
			return fmt.Errorf("update void transaction: %w", err)
		}

		// Update original transaction
		if status == models.StatusVoided {
			if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(originalTxn.ID),
				models.StatusVoided, nil, nil, nil); err != nil {
				return fmt.Errorf("update original transaction: %w", err)
			}
		}

		voidTxn.Status = status
		voidTxn.GatewayTransactionID = gatewayTxnID
		voidTxn.GatewayResponseCode = responseCode
		voidTxn.GatewayResponseMsg = responseMsg
		voidTxn.AVSResponse = gatewayResp.AVSResponse
		voidTxn.CVVResponse = gatewayResp.CVVResponse

		response = s.toPaymentResponse(voidTxn)
		response.IsApproved = (status == models.StatusVoided)

		return nil
	})

	if err != nil {
		s.logger.Error("void failed",
			ports.String("void_transaction_id", voidTxn.ID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("void completed",
		ports.String("void_transaction_id", voidTxn.ID))

	return response, nil
}

// Refund refunds a captured transaction
func (s *Service) Refund(ctx context.Context, req ports.ServiceRefundRequest) (*ports.PaymentResponse, error) {
	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.txRepo.GetByIdempotencyKey(ctx, nil, req.IdempotencyKey)
		if err == nil && existing != nil {
			s.logger.Info("returning existing transaction for idempotency key",
				ports.String("idempotency_key", req.IdempotencyKey))
			return s.toPaymentResponse(existing), nil
		}
	}

	// Get original transaction
	originalTxn, err := s.txRepo.GetByID(ctx, nil, uuid.MustParse(req.TransactionID))
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	// Validate transaction can be refunded
	if originalTxn.Status != models.StatusCaptured {
		return nil, fmt.Errorf("transaction %s is not captured (status: %s)",
			req.TransactionID, originalTxn.Status)
	}

	// Determine refund amount
	refundAmount := originalTxn.Amount
	if req.Amount != nil {
		refundAmount = *req.Amount
		if refundAmount.GreaterThan(originalTxn.Amount) {
			return nil, fmt.Errorf("refund amount %s exceeds captured amount %s",
				refundAmount, originalTxn.Amount)
		}
	}

	// Create refund transaction
	refundTxn := &models.Transaction{
		ID:                   uuid.New().String(),
		MerchantID:           originalTxn.MerchantID,
		CustomerID:           originalTxn.CustomerID,
		Amount:               refundAmount,
		Currency:             originalTxn.Currency,
		Status:               models.StatusPending,
		Type:                 models.TypeRefund,
		PaymentMethodType:    originalTxn.PaymentMethodType,
		GatewayTransactionID: originalTxn.GatewayTransactionID,
		IdempotencyKey:       req.IdempotencyKey,
		Metadata:             map[string]string{"original_transaction_id": originalTxn.ID, "reason": req.Reason},
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	var response *ports.PaymentResponse

	err = s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.txRepo.Create(ctx, tx, refundTxn); err != nil {
			return fmt.Errorf("create refund transaction: %w", err)
		}

		gatewayReq := &ports.RefundRequest{
			TransactionID: originalTxn.GatewayTransactionID,
			Amount:        refundAmount,
			Reason:        req.Reason,
		}

		gatewayResp, err := s.gateway.Refund(ctx, gatewayReq)
		if err != nil {
			responseCode := "ERROR"
			responseMsg := err.Error()
			_ = s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(refundTxn.ID),
				models.StatusFailed, nil, &responseCode, &responseMsg)
			return fmt.Errorf("gateway refund: %w", err)
		}

		// Gateway returns status already determined
		status := gatewayResp.Status
		gatewayTxnID := gatewayResp.GatewayTransactionID
		responseCode := gatewayResp.ResponseCode
		responseMsg := gatewayResp.Message

		if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(refundTxn.ID),
			status, &gatewayTxnID, &responseCode, &responseMsg); err != nil {
			return fmt.Errorf("update refund transaction: %w", err)
		}

		// Update original transaction
		if status == models.StatusRefunded {
			if err := s.txRepo.UpdateStatus(ctx, tx, uuid.MustParse(originalTxn.ID),
				models.StatusRefunded, nil, nil, nil); err != nil {
				return fmt.Errorf("update original transaction: %w", err)
			}
		}

		refundTxn.Status = status
		refundTxn.GatewayTransactionID = gatewayTxnID
		refundTxn.GatewayResponseCode = responseCode
		refundTxn.GatewayResponseMsg = responseMsg
		refundTxn.AVSResponse = gatewayResp.AVSResponse
		refundTxn.CVVResponse = gatewayResp.CVVResponse

		response = s.toPaymentResponse(refundTxn)
		response.IsApproved = (status == models.StatusRefunded)

		return nil
	})

	if err != nil {
		s.logger.Error("refund failed",
			ports.String("refund_transaction_id", refundTxn.ID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("refund completed",
		ports.String("refund_transaction_id", refundTxn.ID))

	return response, nil
}

// GetTransaction retrieves a transaction by ID
func (s *Service) GetTransaction(ctx context.Context, transactionID string) (*models.Transaction, error) {
	txn, err := s.txRepo.GetByID(ctx, nil, uuid.MustParse(transactionID))
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	return txn, nil
}

// GetTransactionByIdempotencyKey retrieves a transaction by idempotency key
func (s *Service) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error) {
	txn, err := s.txRepo.GetByIdempotencyKey(ctx, nil, key)
	if err != nil {
		return nil, fmt.Errorf("get transaction by idempotency key: %w", err)
	}
	return txn, nil
}

// ListTransactions lists transactions for a merchant or customer with pagination
func (s *Service) ListTransactions(ctx context.Context, req ports.ServiceListTransactionsRequest) (*ports.ServiceListTransactionsResponse, error) {
	// Validate request
	if req.MerchantID == "" {
		return nil, fmt.Errorf("merchant_id is required")
	}

	// Set default limit if not provided
	limit := req.Limit
	if limit == 0 {
		limit = 100 // Default to 100 transactions
	}
	if limit > 500 {
		limit = 500 // Maximum 500 transactions per request
	}

	var transactions []*models.Transaction
	var err error

	// List by customer or merchant
	if req.CustomerID != "" {
		// Filter by specific customer
		transactions, err = s.txRepo.ListByCustomer(ctx, nil, req.MerchantID, req.CustomerID, limit, req.Offset)
	} else {
		// List all transactions for merchant
		transactions, err = s.txRepo.ListByMerchant(ctx, nil, req.MerchantID, limit, req.Offset)
	}

	if err != nil {
		s.logger.Error("failed to list transactions",
			ports.String("merchant_id", req.MerchantID),
			ports.String("customer_id", req.CustomerID),
			ports.String("error", err.Error()))
		return nil, fmt.Errorf("list transactions: %w", err)
	}

	s.logger.Info("listed transactions",
		ports.String("merchant_id", req.MerchantID),
		ports.String("customer_id", req.CustomerID),
		ports.Int("count", len(transactions)))

	return &ports.ServiceListTransactionsResponse{
		Transactions: transactions,
		TotalCount:   int32(len(transactions)), // Note: This is count of returned items, not total in DB
	}, nil
}

// toPaymentResponse converts a transaction to a payment response
func (s *Service) toPaymentResponse(txn *models.Transaction) *ports.PaymentResponse {
	return &ports.PaymentResponse{
		TransactionID:        txn.ID,
		Status:               txn.Status,
		Amount:               txn.Amount,
		Currency:             txn.Currency,
		GatewayTransactionID: txn.GatewayTransactionID,
		GatewayResponseCode:  txn.GatewayResponseCode,
		GatewayResponseMsg:   txn.GatewayResponseMsg,
		AVSResponse:          txn.AVSResponse,
		CVVResponse:          txn.CVVResponse,
		CreatedAt:            txn.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            txn.UpdatedAt.Format(time.RFC3339),
	}
}
