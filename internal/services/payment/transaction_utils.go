package payment

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
)

// Transaction utility functions for converting between database and domain models.
// These are pure functions with no side effects.

// sqlcTransactionToDomain converts a sqlc Transaction to a domain Transaction.
func sqlcTransactionToDomain(dbTx *sqlc.Transaction) *domain.Transaction {
	var parentTxID *string
	if dbTx.ParentTransactionID.Valid {
		id := uuid.UUID(dbTx.ParentTransactionID.Bytes).String()
		parentTxID = &id
	}

	var customerID *string
	if dbTx.CustomerID.Valid {
		customerID = &dbTx.CustomerID.String
	}

	var pmID *string
	if dbTx.PaymentMethodID.Valid {
		id := uuid.UUID(dbTx.PaymentMethodID.Bytes).String()
		pmID = &id
	}

	var subscriptionID *string
	if dbTx.SubscriptionID.Valid {
		id := uuid.UUID(dbTx.SubscriptionID.Bytes).String()
		subscriptionID = &id
	}

	var orderID *string
	if dbTx.OrderID.Valid {
		orderID = &dbTx.OrderID.String
	}

	tx := &domain.Transaction{
		ID:                  dbTx.ID.String(),
		ParentTransactionID: parentTxID,
		MerchantID:          dbTx.MerchantID.String(),
		CustomerID:          customerID,
		OrderID:             orderID,
		SubscriptionID:      subscriptionID,
		AmountCents:         dbTx.AmountCents,
		Currency:            dbTx.Currency,
		Type:                domain.TransactionType(dbTx.Type),
		PaymentMethodType:   domain.PaymentMethodType(dbTx.PaymentMethodType),
		PaymentMethodID:     pmID,
		CreatedAt:           dbTx.CreatedAt,
		UpdatedAt:           dbTx.UpdatedAt,
	}

	// Status is a GENERATED column in database (pgtype.Text)
	if dbTx.Status.Valid {
		tx.Status = domain.TransactionStatus(dbTx.Status.String)
	}

	// Note: auth_guid (BRIC) is stored in each transaction record
	if dbTx.AuthGuid.Valid {
		tx.AuthGUID = dbTx.AuthGuid.String
	}

	// AuthResp is pgtype.Text
	if dbTx.AuthResp.Valid {
		tx.AuthResp = &dbTx.AuthResp.String
	}
	if dbTx.AuthCode.Valid {
		tx.AuthCode = &dbTx.AuthCode.String
	}
	if dbTx.AuthCardType.Valid {
		tx.AuthCardType = &dbTx.AuthCardType.String
	}

	// Parse metadata JSONB and extract display-only fields
	if len(dbTx.Metadata) > 0 {
		if err := json.Unmarshal(dbTx.Metadata, &tx.Metadata); err != nil {
			// Log error but don't fail the entire operation
			// Metadata is supplementary information
			tx.Metadata = nil
		} else {
			// Extract display-only fields from metadata for API compatibility
			if authRespText, ok := tx.Metadata["auth_resp_text"].(string); ok {
				tx.AuthRespText = &authRespText
			}
			if authAvs, ok := tx.Metadata["auth_avs"].(string); ok {
				tx.AuthAVS = &authAvs
			}
			if authCvv2, ok := tx.Metadata["auth_cvv2"].(string); ok {
				tx.AuthCVV2 = &authCvv2
			}
		}
	}

	// Transaction ID is the idempotency key
	txID := dbTx.ID.String()
	tx.IdempotencyKey = &txID

	return tx
}

// sqlcPaymentMethodToDomain converts sqlc model to domain model.
func sqlcPaymentMethodToDomain(dbPM *sqlc.CustomerPaymentMethod) *domain.PaymentMethod {
	pm := &domain.PaymentMethod{
		ID:           dbPM.ID.String(),
		MerchantID:   dbPM.MerchantID.String(),
		CustomerID:   dbPM.CustomerID,
		PaymentType:  domain.PaymentMethodType(dbPM.PaymentType),
		PaymentToken: dbPM.Bric,
		LastFour:     dbPM.LastFour,
		IsDefault:    dbPM.IsDefault.Bool,
		Status:       domain.PaymentMethodStatus(dbPM.Status),
		CreatedAt:    dbPM.CreatedAt,
		UpdatedAt:    dbPM.UpdatedAt,
	}

	if dbPM.CardBrand.Valid {
		pm.CardBrand = &dbPM.CardBrand.String
	}

	if dbPM.CardExpMonth.Valid {
		expMonth := int(dbPM.CardExpMonth.Int32)
		pm.CardExpMonth = &expMonth
	}

	if dbPM.CardExpYear.Valid {
		expYear := int(dbPM.CardExpYear.Int32)
		pm.CardExpYear = &expYear
	}

	if dbPM.BankName.Valid {
		pm.BankName = &dbPM.BankName.String
	}

	if dbPM.AccountType.Valid {
		pm.AccountType = &dbPM.AccountType.String
	}

	if dbPM.LastUsedAt.Valid {
		pm.LastUsedAt = &dbPM.LastUsedAt.Time
	}

	if dbPM.PrenoteStatus.Valid {
		pm.PrenoteStatus = &dbPM.PrenoteStatus.String
	}

	if dbPM.PrenoteAttempts.Valid {
		attempts := int(dbPM.PrenoteAttempts.Int32)
		pm.PrenoteAttempts = &attempts
	}

	if dbPM.VerifiedAt.Valid {
		pm.VerifiedAt = &dbPM.VerifiedAt.Time
	}

	// ReturnCount is NOT NULL DEFAULT 0, so always present
	returnCount := int(dbPM.ReturnCount)
	pm.ReturnCount = &returnCount

	// Status metadata
	if dbPM.StatusReason.Valid {
		pm.StatusReason = &dbPM.StatusReason.String
	}

	if dbPM.StatusChangedAt.Valid {
		pm.StatusChangedAt = &dbPM.StatusChangedAt.Time
	}

	return pm
}

// stringOrEmpty returns the string value or empty string if nil.
func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// stringToUUIDPtr converts an optional string to a UUID pointer.
// Returns nil if string is empty or nil.
func stringToUUIDPtr(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &id
}

// centsToDecimalString converts cents (int64) to a decimal string for EPX API.
// Example: 1050 -> "10.50"
func centsToDecimalString(cents int64) string {
	d := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
	return d.StringFixed(2)
}

// formatCentsForLog formats cents (int64) as a dollar amount for logging.
// Example: 1050 -> "$10.50"
func formatCentsForLog(cents int64) string {
	return "$" + centsToDecimalString(cents)
}
