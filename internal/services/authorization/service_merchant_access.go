package authorization

import (
	"context"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// SQLCServiceMerchantAccessChecker implements ServiceMerchantAccessChecker using sqlc queries
type SQLCServiceMerchantAccessChecker struct {
	queries sqlc.Querier
}

// NewSQLCServiceMerchantAccessChecker creates a new access checker
func NewSQLCServiceMerchantAccessChecker(queries sqlc.Querier) *SQLCServiceMerchantAccessChecker {
	return &SQLCServiceMerchantAccessChecker{
		queries: queries,
	}
}

// CheckServiceMerchantAccess checks if a service has access to a merchant
func (c *SQLCServiceMerchantAccessChecker) CheckServiceMerchantAccess(ctx context.Context, serviceID, merchantID string) (bool, error) {
	merchantUUID, err := uuid.Parse(merchantID)
	if err != nil {
		return false, err
	}

	return c.queries.CheckServiceMerchantAccessByID(ctx, sqlc.CheckServiceMerchantAccessByIDParams{
		ServiceID:  serviceID,
		MerchantID: merchantUUID,
	})
}
