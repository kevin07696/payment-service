package fixtures

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// TransactionBuilder provides fluent API for building test transactions.
type TransactionBuilder struct {
	transaction *sqlc.Transaction
}

// NewTransaction creates a new transaction builder with sensible defaults.
func NewTransaction() *TransactionBuilder {
	now := time.Now()
	return &TransactionBuilder{
		transaction: &sqlc.Transaction{
			ID:                uuid.New(),
			MerchantID:        uuid.New(),
			AmountCents:       10000, // $100.00
			Currency:          "USD",
			Type:              "sale",
			PaymentMethodType: "credit_card",
			Metadata:          []byte("{}"),
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}
}

func (b *TransactionBuilder) WithID(id uuid.UUID) *TransactionBuilder {
	b.transaction.ID = id
	return b
}

func (b *TransactionBuilder) WithParentTransactionID(parentID uuid.UUID) *TransactionBuilder {
	b.transaction.ParentTransactionID = pgtype.UUID{Bytes: parentID, Valid: true}
	return b
}

func (b *TransactionBuilder) WithMerchantID(merchantID uuid.UUID) *TransactionBuilder {
	b.transaction.MerchantID = merchantID
	return b
}

func (b *TransactionBuilder) WithCustomerID(customerID uuid.UUID) *TransactionBuilder {
	b.transaction.CustomerID = pgtype.UUID{Bytes: customerID, Valid: true}
	return b
}

func (b *TransactionBuilder) WithAmountCents(amountCents int64) *TransactionBuilder {
	b.transaction.AmountCents = amountCents
	return b
}

func (b *TransactionBuilder) WithCurrency(currency string) *TransactionBuilder {
	b.transaction.Currency = currency
	return b
}

func (b *TransactionBuilder) WithType(txType string) *TransactionBuilder {
	b.transaction.Type = txType
	return b
}

func (b *TransactionBuilder) Sale() *TransactionBuilder {
	b.transaction.Type = "sale"
	return b
}

func (b *TransactionBuilder) Auth() *TransactionBuilder {
	b.transaction.Type = "auth"
	return b
}

func (b *TransactionBuilder) Capture() *TransactionBuilder {
	b.transaction.Type = "capture"
	return b
}

func (b *TransactionBuilder) Refund() *TransactionBuilder {
	b.transaction.Type = "refund"
	return b
}

func (b *TransactionBuilder) Void() *TransactionBuilder {
	b.transaction.Type = "void"
	return b
}

func (b *TransactionBuilder) Storage() *TransactionBuilder {
	b.transaction.Type = "storage"
	return b
}

func (b *TransactionBuilder) Debit() *TransactionBuilder {
	b.transaction.Type = "debit"
	return b
}

func (b *TransactionBuilder) WithPaymentMethodType(pmType string) *TransactionBuilder {
	b.transaction.PaymentMethodType = pmType
	return b
}

func (b *TransactionBuilder) CreditCard() *TransactionBuilder {
	b.transaction.PaymentMethodType = "credit_card"
	return b
}

func (b *TransactionBuilder) ACH() *TransactionBuilder {
	b.transaction.PaymentMethodType = "ach"
	return b
}

func (b *TransactionBuilder) WithPaymentMethodID(paymentMethodID uuid.UUID) *TransactionBuilder {
	b.transaction.PaymentMethodID = pgtype.UUID{Bytes: paymentMethodID, Valid: true}
	return b
}

func (b *TransactionBuilder) WithSubscriptionID(subscriptionID uuid.UUID) *TransactionBuilder {
	b.transaction.SubscriptionID = pgtype.UUID{Bytes: subscriptionID, Valid: true}
	return b
}

func (b *TransactionBuilder) WithTranNbr(tranNbr string) *TransactionBuilder {
	b.transaction.TranNbr = pgtype.Text{String: tranNbr, Valid: true}
	return b
}

func (b *TransactionBuilder) WithAuthGuid(authGuid string) *TransactionBuilder {
	b.transaction.AuthGuid = pgtype.Text{String: authGuid, Valid: true}
	return b
}

func (b *TransactionBuilder) WithAuthResp(authResp string) *TransactionBuilder {
	b.transaction.AuthResp = pgtype.Text{String: authResp, Valid: true}
	return b
}

func (b *TransactionBuilder) WithAuthCode(authCode string) *TransactionBuilder {
	b.transaction.AuthCode = pgtype.Text{String: authCode, Valid: true}
	return b
}

func (b *TransactionBuilder) WithAuthCardType(cardType string) *TransactionBuilder {
	b.transaction.AuthCardType = pgtype.Text{String: cardType, Valid: true}
	return b
}

func (b *TransactionBuilder) WithStatus(status string) *TransactionBuilder {
	b.transaction.Status = pgtype.Text{String: status, Valid: true}
	return b
}

func (b *TransactionBuilder) Pending() *TransactionBuilder {
	b.transaction.Status = pgtype.Text{String: "pending", Valid: true}
	return b
}

func (b *TransactionBuilder) Approved() *TransactionBuilder {
	b.transaction.Status = pgtype.Text{String: "approved", Valid: true}
	b.transaction.AuthResp = pgtype.Text{String: "00", Valid: true}
	b.transaction.ProcessedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return b
}

func (b *TransactionBuilder) Declined() *TransactionBuilder {
	b.transaction.Status = pgtype.Text{String: "declined", Valid: true}
	b.transaction.AuthResp = pgtype.Text{String: "51", Valid: true}
	b.transaction.ProcessedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return b
}

func (b *TransactionBuilder) Failed() *TransactionBuilder {
	b.transaction.Status = pgtype.Text{String: "failed", Valid: true}
	return b
}

func (b *TransactionBuilder) WithProcessedAt(t time.Time) *TransactionBuilder {
	b.transaction.ProcessedAt = pgtype.Timestamptz{Time: t, Valid: true}
	return b
}

func (b *TransactionBuilder) Build() sqlc.Transaction {
	return *b.transaction
}

// Convenience functions for common transaction scenarios

// ApprovedSale creates an approved sale transaction.
func ApprovedSale(merchantID uuid.UUID, amountCents int64) sqlc.Transaction {
	return NewTransaction().
		WithMerchantID(merchantID).
		WithAmountCents(amountCents).
		Sale().
		CreditCard().
		Approved().
		Build()
}

// ApprovedAuth creates an approved auth transaction.
func ApprovedAuth(merchantID uuid.UUID, amountCents int64, authGuid string) sqlc.Transaction {
	return NewTransaction().
		WithMerchantID(merchantID).
		WithAmountCents(amountCents).
		Auth().
		CreditCard().
		WithAuthGuid(authGuid).
		Approved().
		Build()
}

// DeclinedSale creates a declined sale transaction.
func DeclinedSale(merchantID uuid.UUID, amountCents int64) sqlc.Transaction {
	return NewTransaction().
		WithMerchantID(merchantID).
		WithAmountCents(amountCents).
		Sale().
		CreditCard().
		Declined().
		Build()
}

// PendingTransaction creates a pending transaction.
func PendingTransaction(merchantID uuid.UUID, amountCents int64, txType string) sqlc.Transaction {
	return NewTransaction().
		WithMerchantID(merchantID).
		WithAmountCents(amountCents).
		WithType(txType).
		CreditCard().
		Pending().
		Build()
}

// CaptureTransaction creates a capture transaction linked to a parent auth.
func CaptureTransaction(merchantID uuid.UUID, parentID uuid.UUID, amountCents int64, authGuid string) sqlc.Transaction {
	return NewTransaction().
		WithMerchantID(merchantID).
		WithParentTransactionID(parentID).
		WithAmountCents(amountCents).
		Capture().
		CreditCard().
		WithAuthGuid(authGuid).
		Approved().
		Build()
}

// RefundTransaction creates a refund transaction linked to a parent sale/capture.
func RefundTransaction(merchantID uuid.UUID, parentID uuid.UUID, amountCents int64) sqlc.Transaction {
	return NewTransaction().
		WithMerchantID(merchantID).
		WithParentTransactionID(parentID).
		WithAmountCents(amountCents).
		Refund().
		CreditCard().
		Approved().
		Build()
}
