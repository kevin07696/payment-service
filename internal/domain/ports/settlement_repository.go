package ports

import (
	"context"
	"time"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/shopspring/decimal"
)

// SettlementRepository defines the interface for settlement data persistence
type SettlementRepository interface {
	// Batch operations
	CreateBatch(ctx context.Context, tx DBTX, batch *models.SettlementBatch) error
	GetBatchByID(ctx context.Context, db DBTX, id string) (*models.SettlementBatch, error)
	GetBatchByBatchID(ctx context.Context, db DBTX, batchID string) (*models.SettlementBatch, error)
	ListBatchesByMerchant(ctx context.Context, db DBTX, merchantID string, limit, offset int32) ([]*models.SettlementBatch, error)
	ListBatchesByDate(ctx context.Context, db DBTX, merchantID string, startDate, endDate time.Time) ([]*models.SettlementBatch, error)
	ListBatchesByStatus(ctx context.Context, db DBTX, merchantID string, status models.SettlementStatus, limit, offset int32) ([]*models.SettlementBatch, error)
	UpdateBatch(ctx context.Context, tx DBTX, batch *models.SettlementBatch) error
	UpdateBatchStatus(ctx context.Context, tx DBTX, id string, status models.SettlementStatus, discrepancy *decimal.Decimal) error

	// Transaction operations
	CreateTransaction(ctx context.Context, tx DBTX, txn *models.SettlementTransaction) error
	GetTransactionByID(ctx context.Context, db DBTX, id string) (*models.SettlementTransaction, error)
	ListTransactionsByBatch(ctx context.Context, db DBTX, batchID string) ([]*models.SettlementTransaction, error)
	ListTransactionsByGatewayID(ctx context.Context, db DBTX, gatewayTransactionID string) ([]*models.SettlementTransaction, error)

	// Reconciliation helpers
	GetBatchSummary(ctx context.Context, db DBTX, batchID string) (*SettlementBatchSummary, error)
}

// SettlementBatchSummary provides aggregated data for a settlement batch
type SettlementBatchSummary struct {
	BatchID              string
	TotalTransactions    int32
	TotalSalesAmount     decimal.Decimal
	TotalRefundsAmount   decimal.Decimal
	TotalChargebacksAmount decimal.Decimal
	TotalFeesAmount      decimal.Decimal
	NetAmount            decimal.Decimal
}
