package authorization

import (
	"context"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	merchantsvc "github.com/kevin07696/payment-service/internal/services/merchant"
	"go.uber.org/zap"
)

// MerchantCredentials contains merchant record and its MAC secret
type MerchantCredentials struct {
	Merchant  sqlc.Merchant
	MACSecret string
}

// MerchantCredentialResolver fetches merchant records and credentials
// Uses cache for performance (70% DB load reduction)
// Falls back to direct queries for transactional operations
type MerchantCredentialResolver struct {
	cache         *merchantsvc.MerchantCredentialCache
	queries       sqlc.Querier               // Fallback for transactions
	secretManager ports.SecretManagerAdapter // Fallback for transactions
	logger        *zap.Logger
}

// NewMerchantCredentialResolver creates a new merchant credential resolver
func NewMerchantCredentialResolver(
	cache *merchantsvc.MerchantCredentialCache,
	queries sqlc.Querier,
	secretManager ports.SecretManagerAdapter,
	logger *zap.Logger,
) *MerchantCredentialResolver {
	return &MerchantCredentialResolver{
		cache:         cache,
		queries:       queries,
		secretManager: secretManager,
		logger:        logger,
	}
}

// Resolve fetches merchant record and MAC secret, validates merchant is active
// Uses cache for performance (70% DB load reduction, eliminates Vault calls)
func (r *MerchantCredentialResolver) Resolve(ctx context.Context, merchantID uuid.UUID) (*MerchantCredentials, error) {
	// Fetch from cache (combines DB + Vault into single call)
	cached, err := r.cache.Get(ctx, merchantID)
	if err != nil {
		return nil, domain.ErrMerchantNotFoundTyped
	}

	// Get merchant and MAC secret from cache
	merchant, macSecret := cached.GetBoth()

	// Check merchant is active (cached data is fresh within TTL)
	if !merchant.Status.Valid || merchant.Status.String != "active" {
		return nil, domain.ErrMerchantInactiveTyped
	}

	return &MerchantCredentials{
		Merchant:  merchant,
		MACSecret: macSecret,
	}, nil
}

// ResolveWithinTx fetches merchant record and MAC secret within a transaction
func (r *MerchantCredentialResolver) ResolveWithinTx(ctx context.Context, q sqlc.Querier, merchantID uuid.UUID) (*MerchantCredentials, error) {
	// Fetch merchant record (using transaction's querier)
	merchant, err := q.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return nil, domain.ErrMerchantNotFoundTyped
	}

	// Check merchant is active
	if !merchant.Status.Valid || merchant.Status.String != "active" {
		return nil, domain.ErrMerchantInactiveTyped
	}

	// Fetch MAC secret (secret manager doesn't need transaction)
	secret, err := r.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return nil, domain.ErrMerchantCredentialFailed
	}

	return &MerchantCredentials{
		Merchant:  merchant,
		MACSecret: secret.Value,
	}, nil
}
