package payment

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/util"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// CreatePendingTransactionParams contains parameters for creating a pending transaction
type CreatePendingTransactionParams struct {
	ID                uuid.UUID
	GroupID           *uuid.UUID // Optional: will auto-generate if nil
	MerchantID        uuid.UUID
	CustomerID        *string
	Amount            decimal.Decimal
	Currency          string
	Type              domain.TransactionType
	PaymentMethodType domain.PaymentMethodType
	PaymentMethodID   *uuid.UUID
	Metadata          map[string]interface{}
}

// CreatePendingTransaction creates a pending transaction record before calling EPX
// This establishes the transaction UUID and TRAN_NBR for idempotency
// Returns the transaction UUID and deterministic TRAN_NBR
func (s *paymentService) CreatePendingTransaction(ctx context.Context, params CreatePendingTransactionParams) (txID uuid.UUID, tranNbr string, err error) {
	// Use provided UUID or generate deterministically from idempotency key
	txID = params.ID

	// Generate deterministic TRAN_NBR from UUID
	tranNbr = util.UUIDToEPXTranNbr(txID)

	// Marshal metadata
	metadataJSON, err := json.Marshal(params.Metadata)
	if err != nil {
		s.logger.Warn("Failed to marshal metadata", zap.Error(err))
		metadataJSON = []byte("{}")
	}

	// Create pending transaction in database
	// auth_resp is set to empty string initially (will be updated after EPX response)
	// Using empty string allows the GENERATED status column to work
	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		createParams := sqlc.CreateTransactionParams{
			ID:                txID,
			GroupID:           params.GroupID,
			MerchantID:        params.MerchantID,
			CustomerID:        toNullableText(params.CustomerID),
			Amount:            toNumeric(params.Amount),
			Currency:          params.Currency,
			Type:              string(params.Type),
			PaymentMethodType: string(params.PaymentMethodType),
			PaymentMethodID:   toNullableUUIDFromUUID(params.PaymentMethodID),
			TranNbr: pgtype.Text{
				String: tranNbr,
				Valid:  true,
			},
			AuthGuid: toNullableText(nil), // Will be set after EPX response
			AuthResp: "",                   // Empty initially, updated after EPX
			AuthCode: toNullableText(nil),
			AuthCardType: toNullableText(nil),
			Metadata:     metadataJSON,
		}

		_, err := q.CreateTransaction(ctx, createParams)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return uuid.Nil, "", err
	}

	s.logger.Info("Created pending transaction",
		zap.String("transaction_id", txID.String()),
		zap.String("tran_nbr", tranNbr),
		zap.String("type", string(params.Type)),
	)

	return txID, tranNbr, nil
}

// UpdateTransactionWithEPXResponse updates a pending transaction with EPX response data
func (s *paymentService) UpdateTransactionWithEPXResponse(ctx context.Context, tranNbr string, authGUID, authResp, authCode, authCardType *string, metadata map[string]interface{}) error {
	// Merge metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		s.logger.Warn("Failed to marshal metadata", zap.Error(err))
		metadataJSON = []byte("{}")
	}

	err = s.db.WithTx(ctx, func(q *sqlc.Queries) error {
		_, err := q.UpdateTransactionFromEPXResponse(ctx, sqlc.UpdateTransactionFromEPXResponseParams{
			TranNbr: pgtype.Text{
				String: tranNbr,
				Valid:  true,
			},
			AuthGuid:     toNullableText(authGUID),
			AuthResp:     *authResp, // Required
			AuthCode:     toNullableText(authCode),
			AuthCardType: toNullableText(authCardType),
			Metadata:     metadataJSON,
		})
		return err
	})

	if err != nil {
		s.logger.Error("Failed to update transaction with EPX response",
			zap.String("tran_nbr", tranNbr),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info("Updated transaction with EPX response",
		zap.String("tran_nbr", tranNbr),
		zap.String("auth_resp", *authResp),
	)

	return nil
}
