package payment

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/converters"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
)

// transactionQueryService provides read-only transaction access.
// Separated from PaymentService for interface segregation and testability.
type transactionQueryService struct {
	queries             sqlc.Querier
	merchantAuthService ports.MerchantAuthorizationService
	logger              *zap.Logger
}

// NewTransactionQueryService creates a new transaction query service.
func NewTransactionQueryService(
	queries sqlc.Querier,
	merchantAuthService ports.MerchantAuthorizationService,
	logger *zap.Logger,
) ports.TransactionQueryService {
	return &transactionQueryService{
		queries:             queries,
		merchantAuthService: merchantAuthService,
		logger:              logger,
	}
}

// GetTransaction retrieves transaction details by ID.
func (s *transactionQueryService) GetTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
	txID, err := uuid.Parse(transactionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "transaction_id")
	}

	dbTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		s.logger.Debug("Transaction not found",
			zap.String("transaction_id", transactionID),
			zap.Error(err),
		)
		return nil, domain.ErrTxnNotFound
	}

	return sqlcTransactionToDomain(&dbTx), nil
}

// GetTransactionByIdempotencyKey retrieves a transaction by idempotency key.
// Note: idempotency_key IS the transaction ID (no separate column).
func (s *transactionQueryService) GetTransactionByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	// Idempotency key is the transaction ID (UUID)
	txID, err := uuid.Parse(key)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "idempotency_key")
	}

	dbTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err != nil {
		s.logger.Debug("Transaction not found by idempotency key",
			zap.String("idempotency_key", key),
			zap.Error(err),
		)
		return nil, domain.ErrTxnNotFound
	}

	return sqlcTransactionToDomain(&dbTx), nil
}

// ListTransactions lists transactions with filters.
func (s *transactionQueryService) ListTransactions(ctx context.Context, filters *ports.ListTransactionsFilters) ([]*domain.Transaction, int, error) {
	// MerchantID is required
	if filters.MerchantID == nil {
		return nil, 0, domain.ErrMerchantRequired
	}

	// Validate service has access to the merchant
	// Note: merchantAuthService may be nil in unit tests that don't inject it
	resolvedMerchantID := *filters.MerchantID
	if s.merchantAuthService != nil {
		var err error
		resolvedMerchantID, err = s.merchantAuthService.ResolveMerchantID(ctx, *filters.MerchantID)
		if err != nil {
			return nil, 0, err
		}
	}

	merchantID, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, 0, domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	// Set defaults
	limit := filters.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	params := sqlc.ListTransactionsParams{
		MerchantID:          merchantID,
		CustomerID:          converters.ToNullableText(filters.CustomerID),
		OrderID:             converters.ToNullableText(filters.OrderID),
		SubscriptionID:      converters.ToNullableUUID(filters.SubscriptionID),
		ParentTransactionID: converters.ToNullableUUID(filters.ParentTransactionID),
		Status:              converters.ToNullableText(filters.Status),
		Type:                converters.ToNullableText(filters.Type),
		PaymentMethodID:     converters.ToNullableUUID(filters.PaymentMethodID),
		LimitVal:            int32(limit),
		OffsetVal:           int32(offset),
	}

	dbTxs, err := s.queries.ListTransactions(ctx, params)
	if err != nil {
		return nil, 0, domain.ErrDatabaseError.WithDetail("operation", "list_transactions")
	}

	countParams := sqlc.CountTransactionsParams{
		MerchantID:          merchantID,
		CustomerID:          converters.ToNullableText(filters.CustomerID),
		OrderID:             converters.ToNullableText(filters.OrderID),
		SubscriptionID:      converters.ToNullableUUID(filters.SubscriptionID),
		ParentTransactionID: converters.ToNullableUUID(filters.ParentTransactionID),
		Status:              converters.ToNullableText(filters.Status),
		Type:                converters.ToNullableText(filters.Type),
		PaymentMethodID:     converters.ToNullableUUID(filters.PaymentMethodID),
	}

	count, err := s.queries.CountTransactions(ctx, countParams)
	if err != nil {
		return nil, 0, domain.ErrDatabaseError.WithDetail("operation", "count_transactions")
	}

	transactions := make([]*domain.Transaction, len(dbTxs))
	for i, dbTx := range dbTxs {
		transactions[i] = sqlcTransactionToDomain(&dbTx)
	}

	return transactions, int(count), nil
}

// GetTransactionsByGroup retrieves all transactions in a group (parent + children).
func (s *transactionQueryService) GetTransactionsByGroup(ctx context.Context, parentTransactionID string) ([]*domain.Transaction, error) {
	parentID, err := uuid.Parse(parentTransactionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID.WithDetail("field", "parent_transaction_id")
	}

	// Get transaction tree (includes parent + all descendants)
	groupTxs, err := s.queries.GetTransactionTree(ctx, parentID)
	if err != nil {
		return nil, domain.ErrDatabaseError.WithDetail("operation", "get_transaction_tree")
	}

	transactions := make([]*domain.Transaction, len(groupTxs))
	for i, tx := range groupTxs {
		sqlcTx := sqlc.Transaction(tx)
		transactions[i] = sqlcTransactionToDomain(&sqlcTx)
	}

	return transactions, nil
}
