package ports

import (
	"context"

	"github.com/kevin07696/payment-service/internal/domain/models"
)

// ChargebackRepository defines the interface for chargeback data persistence
type ChargebackRepository interface {
	// Create creates a new chargeback record
	Create(ctx context.Context, tx DBTX, chargeback *models.Chargeback) error

	// GetByID retrieves a chargeback by its ID
	GetByID(ctx context.Context, db DBTX, id string) (*models.Chargeback, error)

	// GetByChargebackID retrieves a chargeback by the gateway's chargeback ID
	GetByChargebackID(ctx context.Context, db DBTX, chargebackID string) (*models.Chargeback, error)

	// GetByTransactionID retrieves all chargebacks for a specific transaction
	GetByTransactionID(ctx context.Context, db DBTX, transactionID string) ([]*models.Chargeback, error)

	// ListByMerchant retrieves chargebacks for a merchant with pagination
	ListByMerchant(ctx context.Context, db DBTX, merchantID string, limit, offset int32) ([]*models.Chargeback, error)

	// ListByCustomer retrieves chargebacks for a customer with pagination
	ListByCustomer(ctx context.Context, db DBTX, merchantID, customerID string, limit, offset int32) ([]*models.Chargeback, error)

	// ListByStatus retrieves chargebacks by status
	ListByStatus(ctx context.Context, db DBTX, merchantID string, status models.ChargebackStatus, limit, offset int32) ([]*models.Chargeback, error)

	// ListPendingResponses retrieves all chargebacks that need a response
	// (status = pending and respond_by_date is approaching or past)
	ListPendingResponses(ctx context.Context, db DBTX) ([]*models.Chargeback, error)

	// Update updates an existing chargeback record
	Update(ctx context.Context, tx DBTX, chargeback *models.Chargeback) error

	// UpdateStatus updates the status and outcome of a chargeback
	UpdateStatus(ctx context.Context, tx DBTX, id string, status models.ChargebackStatus, outcome *models.ChargebackOutcome) error
}
