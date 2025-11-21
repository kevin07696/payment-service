package payment

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/converters"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/util"
	"go.uber.org/zap"
)

// CreatePendingTransactionParams contains parameters for creating a pending transaction
type CreatePendingTransactionParams struct {
	ID                  uuid.UUID
	ParentTransactionID *uuid.UUID // Optional: parent transaction ID for child transactions (CAPTURE, VOID, REFUND)
	MerchantID          uuid.UUID
	CustomerID          *string
	Amount              int64 // Amount in cents
	Currency            string
	Type                domain.TransactionType
	PaymentMethodType   domain.PaymentMethodType
	PaymentMethodID     *uuid.UUID
	Metadata            map[string]interface{}
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
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		createParams := sqlc.CreateTransactionParams{
			ID:                  txID,
			ParentTransactionID: converters.ToNullableUUIDFromUUID(params.ParentTransactionID),
			MerchantID:          params.MerchantID,
			CustomerID:          converters.ToNullableText(params.CustomerID),
			AmountCents:         params.Amount, // Amount is already in cents
			Currency:            params.Currency,
			Type:                string(params.Type),
			PaymentMethodType:   string(params.PaymentMethodType),
			PaymentMethodID:     converters.ToNullableUUIDFromUUID(params.PaymentMethodID),
			TranNbr: pgtype.Text{
				String: tranNbr,
				Valid:  true,
			},
			AuthGuid:     converters.ToNullableText(nil), // Will be set after EPX response
			AuthResp:     pgtype.Text{},       // Empty initially, updated after EPX
			AuthCode:     converters.ToNullableText(nil),
			AuthCardType: converters.ToNullableText(nil),
			Metadata:     metadataJSON,
			ProcessedAt:  pgtype.Timestamptz{},
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
func (s *paymentService) UpdateTransactionWithEPXResponse(ctx context.Context, tranNbr string, customerID, authGUID, authResp, authCode, authCardType *string, metadata map[string]interface{}) error {
	// Merge metadata
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		s.logger.Warn("Failed to marshal metadata", zap.Error(err))
		metadataJSON = []byte("{}")
	}

	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		_, err := q.UpdateTransactionFromEPXResponse(ctx, sqlc.UpdateTransactionFromEPXResponseParams{
			CustomerID: converters.ToNullableText(customerID),
			TranNbr: pgtype.Text{
				String: tranNbr,
				Valid:  true,
			},
			AuthGuid:     converters.ToNullableText(authGUID),
			AuthResp:     pgtype.Text{String: *authResp, Valid: true}, // Required
			AuthCode:     converters.ToNullableText(authCode),
			AuthCardType: converters.ToNullableText(authCardType),
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
