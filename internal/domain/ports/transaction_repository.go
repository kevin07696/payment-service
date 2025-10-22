package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/domain/models"
)

// TransactionRepository defines the interface for transaction persistence
type TransactionRepository interface {
	// Create creates a new transaction
	Create(ctx context.Context, tx DBTX, transaction *models.Transaction) error

	// GetByID retrieves a transaction by its ID
	GetByID(ctx context.Context, db DBTX, id uuid.UUID) (*models.Transaction, error)

	// GetByIdempotencyKey retrieves a transaction by its idempotency key
	GetByIdempotencyKey(ctx context.Context, db DBTX, key string) (*models.Transaction, error)

	// GetByGatewayTransactionID retrieves a transaction by gateway transaction ID
	// Used for linking chargebacks/disputes to original transactions
	GetByGatewayTransactionID(ctx context.Context, db DBTX, gatewayTxnID string) (*models.Transaction, error)

	// UpdateStatus updates the status of a transaction
	UpdateStatus(ctx context.Context, tx DBTX, id uuid.UUID, status models.TransactionStatus, gatewayTxnID, responseCode, responseMessage *string) error

	// ListByMerchant lists transactions for a merchant with pagination
	ListByMerchant(ctx context.Context, db DBTX, merchantID string, limit, offset int32) ([]*models.Transaction, error)

	// ListByCustomer lists transactions for a customer with pagination
	ListByCustomer(ctx context.Context, db DBTX, merchantID, customerID string, limit, offset int32) ([]*models.Transaction, error)
}
