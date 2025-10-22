package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
)

// TransactionRepository implements ports.TransactionRepository using SQLC
type TransactionRepository struct {
	queries *sqlc.Queries
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(db ports.DBPort) *TransactionRepository {
	return &TransactionRepository{
		queries: sqlc.New(db.GetDB()),
	}
}

// Create creates a new transaction
func (r *TransactionRepository) Create(ctx context.Context, tx ports.DBTX, transaction *models.Transaction) error {
	var q *sqlc.Queries
	if tx != nil {
		q = sqlc.New(tx)
	} else {
		q = r.queries
	}

	// Parse UUID from string
	txID, err := uuid.Parse(transaction.ID)
	if err != nil {
		return fmt.Errorf("invalid transaction ID: %w", err)
	}

	// Convert metadata map to JSON bytes
	var metadataBytes []byte
	if transaction.Metadata != nil {
		metadataBytes, err = json.Marshal(transaction.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	} else {
		metadataBytes = []byte("{}")
	}

	// Convert amount to pgtype.Numeric
	amount := pgtype.Numeric{}
	if err := amount.Scan(transaction.Amount.String()); err != nil {
		return fmt.Errorf("convert amount: %w", err)
	}

	err = q.CreateTransaction(ctx, sqlc.CreateTransactionParams{
		ID:                     txID,
		MerchantID:             transaction.MerchantID,
		CustomerID:             nullText(transaction.CustomerID),
		Amount:                 amount,
		Currency:               transaction.Currency,
		Status:                 string(transaction.Status),
		Type:                   string(transaction.Type),
		PaymentMethodType:      string(transaction.PaymentMethodType),
		PaymentMethodToken:     nullText(transaction.PaymentMethodToken),
		GatewayTransactionID:   nullText(transaction.GatewayTransactionID),
		GatewayResponseCode:    nullText(transaction.GatewayResponseCode),
		GatewayResponseMessage: nullText(transaction.GatewayResponseMsg),
		IdempotencyKey:         nullText(transaction.IdempotencyKey),
		Metadata:               metadataBytes,
	})

	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	return nil
}

// GetByID retrieves a transaction by its ID
func (r *TransactionRepository) GetByID(ctx context.Context, db ports.DBTX, id uuid.UUID) (*models.Transaction, error) {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	} else {
		q = r.queries
	}

	row, err := q.GetTransactionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get transaction by id: %w", err)
	}

	return r.toDomainModel(row)
}

// GetByIdempotencyKey retrieves a transaction by its idempotency key
func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, db ports.DBTX, key string) (*models.Transaction, error) {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	} else {
		q = r.queries
	}

	row, err := q.GetTransactionByIdempotencyKey(ctx, nullText(key))
	if err != nil {
		return nil, fmt.Errorf("get transaction by idempotency key: %w", err)
	}

	return r.toDomainModel(row)
}

// UpdateStatus updates the status of a transaction
func (r *TransactionRepository) UpdateStatus(ctx context.Context, tx ports.DBTX, id uuid.UUID, status models.TransactionStatus, gatewayTxnID, responseCode, responseMessage *string) error {
	var q *sqlc.Queries
	if tx != nil {
		q = sqlc.New(tx)
	} else {
		q = r.queries
	}

	var nullGatewayTxnID pgtype.Text
	if gatewayTxnID != nil {
		nullGatewayTxnID = pgtype.Text{String: *gatewayTxnID, Valid: true}
	}

	var nullResponseCode pgtype.Text
	if responseCode != nil {
		nullResponseCode = pgtype.Text{String: *responseCode, Valid: true}
	}

	var nullResponseMessage pgtype.Text
	if responseMessage != nil {
		nullResponseMessage = pgtype.Text{String: *responseMessage, Valid: true}
	}

	err := q.UpdateTransactionStatus(ctx, sqlc.UpdateTransactionStatusParams{
		ID:                     id,
		Status:                 string(status),
		GatewayTransactionID:   nullGatewayTxnID,
		GatewayResponseCode:    nullResponseCode,
		GatewayResponseMessage: nullResponseMessage,
	})

	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}

	return nil
}

// ListByMerchant lists transactions for a merchant with pagination
func (r *TransactionRepository) ListByMerchant(ctx context.Context, db ports.DBTX, merchantID string, limit, offset int32) ([]*models.Transaction, error) {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	} else {
		q = r.queries
	}

	rows, err := q.ListTransactionsByMerchant(ctx, sqlc.ListTransactionsByMerchantParams{
		MerchantID: merchantID,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list transactions by merchant: %w", err)
	}

	transactions := make([]*models.Transaction, len(rows))
	for i, row := range rows {
		tx, err := r.toDomainModel(row)
		if err != nil {
			return nil, fmt.Errorf("convert transaction %d: %w", i, err)
		}
		transactions[i] = tx
	}

	return transactions, nil
}

// ListByCustomer lists transactions for a customer with pagination
func (r *TransactionRepository) ListByCustomer(ctx context.Context, db ports.DBTX, merchantID, customerID string, limit, offset int32) ([]*models.Transaction, error) {
	var q *sqlc.Queries
	if db != nil {
		q = sqlc.New(db)
	} else {
		q = r.queries
	}

	rows, err := q.ListTransactionsByCustomer(ctx, sqlc.ListTransactionsByCustomerParams{
		MerchantID: merchantID,
		CustomerID: nullText(customerID),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list transactions by customer: %w", err)
	}

	transactions := make([]*models.Transaction, len(rows))
	for i, row := range rows {
		tx, err := r.toDomainModel(row)
		if err != nil {
			return nil, fmt.Errorf("convert transaction %d: %w", i, err)
		}
		transactions[i] = tx
	}

	return transactions, nil
}

// toDomainModel converts a SQLC transaction to a domain model
func (r *TransactionRepository) toDomainModel(row sqlc.Transaction) (*models.Transaction, error) {
	// Convert metadata bytes to map
	var metadata map[string]string
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	// Convert amount from pgtype.Numeric to decimal.Decimal
	amount, err := pgNumericToDecimal(row.Amount)
	if err != nil {
		return nil, fmt.Errorf("convert amount: %w", err)
	}

	return &models.Transaction{
		ID:                   row.ID.String(),
		MerchantID:           row.MerchantID,
		CustomerID:           row.CustomerID.String,
		Amount:               amount,
		Currency:             row.Currency,
		Status:               models.TransactionStatus(row.Status),
		Type:                 models.TransactionType(row.Type),
		PaymentMethodType:    models.PaymentMethodType(row.PaymentMethodType),
		PaymentMethodToken:   row.PaymentMethodToken.String,
		GatewayTransactionID: row.GatewayTransactionID.String,
		GatewayResponseCode:  row.GatewayResponseCode.String,
		GatewayResponseMsg:   row.GatewayResponseMessage.String,
		IdempotencyKey:       row.IdempotencyKey.String,
		Metadata:             metadata,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

