package fixtures

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// PaymentMethodBuilder provides fluent API for building test payment methods.
type PaymentMethodBuilder struct {
	paymentMethod *sqlc.CustomerPaymentMethod
}

// NewPaymentMethod creates a new payment method builder with sensible defaults.
func NewPaymentMethod() *PaymentMethodBuilder {
	now := time.Now()
	return &PaymentMethodBuilder{
		paymentMethod: &sqlc.CustomerPaymentMethod{
			ID:           uuid.New(),
			MerchantID:   uuid.New(),
			CustomerID:   uuid.New(),
			Bric:         "bric_test_" + uuid.New().String()[:8],
			PaymentType:  "credit_card",
			LastFour:     "4242",
			CardBrand:    pgtype.Text{String: "visa", Valid: true},
			CardExpMonth: pgtype.Int4{Int32: 12, Valid: true},
			CardExpYear:  pgtype.Int4{Int32: 2025, Valid: true},
			IsDefault:    pgtype.Bool{Bool: false, Valid: true},
			IsActive:     pgtype.Bool{Bool: true, Valid: true},
			IsVerified:   pgtype.Bool{Bool: false, Valid: true},
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
}

func (b *PaymentMethodBuilder) WithID(id uuid.UUID) *PaymentMethodBuilder {
	b.paymentMethod.ID = id
	return b
}

func (b *PaymentMethodBuilder) WithMerchantID(merchantID uuid.UUID) *PaymentMethodBuilder {
	b.paymentMethod.MerchantID = merchantID
	return b
}

func (b *PaymentMethodBuilder) WithCustomerID(customerID uuid.UUID) *PaymentMethodBuilder {
	b.paymentMethod.CustomerID = customerID
	return b
}

func (b *PaymentMethodBuilder) WithBric(bric string) *PaymentMethodBuilder {
	b.paymentMethod.Bric = bric
	return b
}

func (b *PaymentMethodBuilder) WithPaymentType(paymentType string) *PaymentMethodBuilder {
	b.paymentMethod.PaymentType = paymentType
	return b
}

func (b *PaymentMethodBuilder) CreditCard() *PaymentMethodBuilder {
	b.paymentMethod.PaymentType = "credit_card"
	return b
}

func (b *PaymentMethodBuilder) ACH() *PaymentMethodBuilder {
	b.paymentMethod.PaymentType = "ach"
	// Clear credit card fields
	b.paymentMethod.CardBrand = pgtype.Text{}
	b.paymentMethod.CardExpMonth = pgtype.Int4{}
	b.paymentMethod.CardExpYear = pgtype.Int4{}
	// Set ACH fields
	b.paymentMethod.BankName = pgtype.Text{String: "Test Bank", Valid: true}
	b.paymentMethod.AccountType = pgtype.Text{String: "checking", Valid: true}
	return b
}

func (b *PaymentMethodBuilder) WithLastFour(lastFour string) *PaymentMethodBuilder {
	b.paymentMethod.LastFour = lastFour
	return b
}

func (b *PaymentMethodBuilder) WithCardBrand(brand string) *PaymentMethodBuilder {
	b.paymentMethod.CardBrand = pgtype.Text{String: brand, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) WithCardExpiration(month, year int32) *PaymentMethodBuilder {
	b.paymentMethod.CardExpMonth = pgtype.Int4{Int32: month, Valid: true}
	b.paymentMethod.CardExpYear = pgtype.Int4{Int32: year, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) WithBankName(bankName string) *PaymentMethodBuilder {
	b.paymentMethod.BankName = pgtype.Text{String: bankName, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) WithAccountType(accountType string) *PaymentMethodBuilder {
	b.paymentMethod.AccountType = pgtype.Text{String: accountType, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) Default() *PaymentMethodBuilder {
	b.paymentMethod.IsDefault = pgtype.Bool{Bool: true, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) NotDefault() *PaymentMethodBuilder {
	b.paymentMethod.IsDefault = pgtype.Bool{Bool: false, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) Active() *PaymentMethodBuilder {
	b.paymentMethod.IsActive = pgtype.Bool{Bool: true, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) Inactive() *PaymentMethodBuilder {
	b.paymentMethod.IsActive = pgtype.Bool{Bool: false, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) Verified() *PaymentMethodBuilder {
	b.paymentMethod.IsVerified = pgtype.Bool{Bool: true, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) NotVerified() *PaymentMethodBuilder {
	b.paymentMethod.IsVerified = pgtype.Bool{Bool: false, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) WithLastUsedAt(t time.Time) *PaymentMethodBuilder {
	b.paymentMethod.LastUsedAt = pgtype.Timestamptz{Time: t, Valid: true}
	return b
}

func (b *PaymentMethodBuilder) Build() sqlc.CustomerPaymentMethod {
	return *b.paymentMethod
}

// Convenience functions for common payment method scenarios

// VisaCard creates a verified Visa credit card payment method.
func VisaCard(merchantID, customerID uuid.UUID, bric string) sqlc.CustomerPaymentMethod {
	return NewPaymentMethod().
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithBric(bric).
		CreditCard().
		WithCardBrand("visa").
		WithLastFour("4242").
		Verified().
		Active().
		Build()
}

// DefaultVisaCard creates a default Visa credit card payment method.
func DefaultVisaCard(merchantID, customerID uuid.UUID, bric string) sqlc.CustomerPaymentMethod {
	return NewPaymentMethod().
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithBric(bric).
		CreditCard().
		WithCardBrand("visa").
		WithLastFour("4242").
		Verified().
		Active().
		Default().
		Build()
}

// CheckingAccount creates a verified checking account payment method.
func CheckingAccount(merchantID, customerID uuid.UUID, bric string) sqlc.CustomerPaymentMethod {
	return NewPaymentMethod().
		WithMerchantID(merchantID).
		WithCustomerID(customerID).
		WithBric(bric).
		ACH().
		WithLastFour("6789").
		WithBankName("Test Bank").
		WithAccountType("checking").
		Verified().
		Active().
		Build()
}
