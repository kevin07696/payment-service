package fixtures

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// MerchantBuilder provides fluent API for building test merchants.
type MerchantBuilder struct {
	merchant *sqlc.Merchant
}

// NewMerchant creates a new merchant builder with sensible defaults.
func NewMerchant() *MerchantBuilder {
	return &MerchantBuilder{
		merchant: &sqlc.Merchant{
			ID:            uuid.New(),
			Slug:          "test-merchant",
			CustNbr:       "123456",
			MerchNbr:      "789012",
			DbaNbr:        "001",
			TerminalNbr:   "001",
			MacSecretPath: "secrets/test-merchant/mac",
			Environment:   "sandbox",
			IsActive:      true,
			Name:          "Test Merchant",
			CreatedAt: pgtype.Timestamp{
				Time:  time.Now(),
				Valid: true,
			},
			UpdatedAt: pgtype.Timestamp{
				Time:  time.Now(),
				Valid: true,
			},
		},
	}
}

func (b *MerchantBuilder) WithID(id uuid.UUID) *MerchantBuilder {
	b.merchant.ID = id
	return b
}

func (b *MerchantBuilder) WithSlug(slug string) *MerchantBuilder {
	b.merchant.Slug = slug
	return b
}

func (b *MerchantBuilder) WithCustNbr(custNbr string) *MerchantBuilder {
	b.merchant.CustNbr = custNbr
	return b
}

func (b *MerchantBuilder) WithMerchNbr(merchNbr string) *MerchantBuilder {
	b.merchant.MerchNbr = merchNbr
	return b
}

func (b *MerchantBuilder) WithDbaNbr(dbaNbr string) *MerchantBuilder {
	b.merchant.DbaNbr = dbaNbr
	return b
}

func (b *MerchantBuilder) WithTerminalNbr(terminalNbr string) *MerchantBuilder {
	b.merchant.TerminalNbr = terminalNbr
	return b
}

func (b *MerchantBuilder) WithMacSecretPath(path string) *MerchantBuilder {
	b.merchant.MacSecretPath = path
	return b
}

func (b *MerchantBuilder) WithEnvironment(env string) *MerchantBuilder {
	b.merchant.Environment = env
	return b
}

func (b *MerchantBuilder) Active() *MerchantBuilder {
	b.merchant.IsActive = true
	return b
}

func (b *MerchantBuilder) Inactive() *MerchantBuilder {
	b.merchant.IsActive = false
	return b
}

func (b *MerchantBuilder) WithName(name string) *MerchantBuilder {
	b.merchant.Name = name
	return b
}

func (b *MerchantBuilder) WithStatus(status string) *MerchantBuilder {
	b.merchant.Status = pgtype.Text{String: status, Valid: true}
	return b
}

func (b *MerchantBuilder) WithTier(tier string) *MerchantBuilder {
	b.merchant.Tier = pgtype.Text{String: tier, Valid: true}
	return b
}

func (b *MerchantBuilder) Build() sqlc.Merchant {
	return *b.merchant
}

// Convenience functions for common merchant scenarios

// ActiveMerchant creates an active merchant with given ID and slug.
func ActiveMerchant(id uuid.UUID, slug string) sqlc.Merchant {
	return NewMerchant().
		WithID(id).
		WithSlug(slug).
		Active().
		Build()
}

// InactiveMerchant creates an inactive merchant with given ID and slug.
func InactiveMerchant(id uuid.UUID, slug string) sqlc.Merchant {
	return NewMerchant().
		WithID(id).
		WithSlug(slug).
		Inactive().
		Build()
}
