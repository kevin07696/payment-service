package authorization

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"go.uber.org/zap"
)

// MerchantCredentials contains merchant record and its MAC secret
type MerchantCredentials struct {
	Merchant  sqlc.Merchant
	MACSecret string
}

// MerchantCredentialResolver fetches merchant records and credentials
type MerchantCredentialResolver struct {
	queries       sqlc.Querier
	secretManager adapterports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewMerchantCredentialResolver creates a new merchant credential resolver
func NewMerchantCredentialResolver(
	queries sqlc.Querier,
	secretManager adapterports.SecretManagerAdapter,
	logger *zap.Logger,
) *MerchantCredentialResolver {
	return &MerchantCredentialResolver{
		queries:       queries,
		secretManager: secretManager,
		logger:        logger,
	}
}

// Resolve fetches merchant record and MAC secret, validates merchant is active
func (r *MerchantCredentialResolver) Resolve(ctx context.Context, merchantID uuid.UUID) (*MerchantCredentials, error) {
	// Fetch merchant record
	merchant, err := r.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	// Check merchant is active
	if !merchant.Status.Valid || merchant.Status.String != "active" {
		return nil, domain.ErrMerchantInactive
	}

	// Fetch MAC secret
	secret, err := r.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	return &MerchantCredentials{
		Merchant:  merchant,
		MACSecret: secret.Value,
	}, nil
}

// ResolveWithinTx fetches merchant record and MAC secret within a transaction
func (r *MerchantCredentialResolver) ResolveWithinTx(ctx context.Context, q sqlc.Querier, merchantID uuid.UUID) (*MerchantCredentials, error) {
	// Fetch merchant record (using transaction's querier)
	merchant, err := q.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, fmt.Errorf("merchant not found: %w", err)
	}

	// Check merchant is active
	if !merchant.Status.Valid || merchant.Status.String != "active" {
		return nil, domain.ErrMerchantInactive
	}

	// Fetch MAC secret (secret manager doesn't need transaction)
	secret, err := r.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAC secret: %w", err)
	}

	return &MerchantCredentials{
		Merchant:  merchant,
		MACSecret: secret.Value,
	}, nil
}
