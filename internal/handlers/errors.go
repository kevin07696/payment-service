// Package handlers provides shared utilities for API handlers.
package handlers

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/domain"
)

// HandleServiceErrorConnect maps domain errors to Connect error codes.
// This is the centralized error handler for all ConnectRPC handlers.
// It ensures consistent error responses across all API endpoints.
func HandleServiceErrorConnect(err error, logger *zap.Logger) error {
	if err == nil {
		return nil
	}

	switch {
	// ============================================================
	// Authentication & Authorization Errors
	// ============================================================
	case errors.Is(err, domain.ErrAuthMerchantMismatch):
		return connect.NewError(connect.CodePermissionDenied, errors.New("access denied: merchant mismatch"))
	case errors.Is(err, domain.ErrAuthAccessDenied):
		return connect.NewError(connect.CodePermissionDenied, errors.New("access denied"))

	// ============================================================
	// Merchant Errors - New DomainError instances
	// ============================================================
	case errors.Is(err, domain.ErrMerchantNotFoundTyped):
		return connect.NewError(connect.CodeNotFound, errors.New("merchant not found"))
	case errors.Is(err, domain.ErrMerchantInactiveTyped):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("merchant is inactive"))
	case errors.Is(err, domain.ErrMerchantAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("merchant already exists"))
	case errors.Is(err, domain.ErrMerchantInvalidEnv):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid environment"))
	case errors.Is(err, domain.ErrMerchantRequired):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))

	// Merchant Errors - Legacy sentinel errors (backward compatibility)
	case errors.Is(err, domain.ErrMerchantNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("merchant not found"))
	case errors.Is(err, domain.ErrMerchantInactive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("merchant is inactive"))
	case errors.Is(err, domain.ErrInvalidEnvironment):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid environment"))

	// ============================================================
	// Transaction Errors
	// ============================================================
	case errors.Is(err, domain.ErrTxnNotFound), errors.Is(err, domain.ErrTransactionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("transaction not found"))
	case errors.Is(err, domain.ErrTxnCannotBeVoided), errors.Is(err, domain.ErrTransactionCannotBeVoided):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be voided"))
	case errors.Is(err, domain.ErrTxnCannotBeCaptured), errors.Is(err, domain.ErrTransactionCannotBeCaptured):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be captured"))
	case errors.Is(err, domain.ErrTxnCannotBeRefunded), errors.Is(err, domain.ErrTransactionCannotBeRefunded):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be refunded"))
	case errors.Is(err, domain.ErrTxnDeclined), errors.Is(err, domain.ErrTransactionDeclined):
		return connect.NewError(connect.CodeAborted, errors.New("transaction was declined"))

	// ============================================================
	// Payment Method Errors - New DomainError instances
	// ============================================================
	case errors.Is(err, domain.ErrPMNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("payment method not found"))
	case errors.Is(err, domain.ErrPMExpired):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is expired"))
	case errors.Is(err, domain.ErrPMNotVerified):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("ACH payment method is not verified"))
	case errors.Is(err, domain.ErrPMInactive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is inactive"))
	case errors.Is(err, domain.ErrPMInvalidType):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid payment method type"))
	case errors.Is(err, domain.ErrPMNotBelongToCustomer):
		return connect.NewError(connect.CodePermissionDenied, errors.New("payment method does not belong to customer"))

	// Payment Method Errors - Legacy sentinel errors (backward compatibility)
	case errors.Is(err, domain.ErrPaymentMethodNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("payment method not found"))
	case errors.Is(err, domain.ErrPaymentMethodExpired):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is expired"))
	case errors.Is(err, domain.ErrPaymentMethodNotVerified):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("ACH payment method must be verified before use"))
	case errors.Is(err, domain.ErrPaymentMethodInactive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("payment method is not active"))
	case errors.Is(err, domain.ErrInvalidPaymentMethodType):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid payment method type"))

	// ============================================================
	// Subscription Errors
	// ============================================================
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("subscription not found"))
	case errors.Is(err, domain.ErrSubscriptionNotActive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("subscription is not active"))
	case errors.Is(err, domain.ErrSubscriptionAlreadyCancelled):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("subscription is already cancelled"))
	case errors.Is(err, domain.ErrInvalidBillingInterval):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid billing interval"))

	// ============================================================
	// Validation Errors
	// ============================================================
	case errors.Is(err, domain.ErrValidationInvalidUUID):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid UUID format"))
	case errors.Is(err, domain.ErrInvalidAmount):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid amount"))
	case errors.Is(err, domain.ErrInvalidCurrency):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid currency"))

	// ============================================================
	// Idempotency Errors
	// ============================================================
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("duplicate idempotency key"))

	// ============================================================
	// Database Errors
	// ============================================================
	case errors.Is(err, domain.ErrDatabaseError):
		return connect.NewError(connect.CodeInternal, errors.New("database error"))
	case errors.Is(err, sql.ErrNoRows):
		return connect.NewError(connect.CodeNotFound, errors.New("resource not found"))

	// ============================================================
	// Context Errors
	// ============================================================
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeCanceled, errors.New("request canceled"))

	// ============================================================
	// Default: Internal Server Error
	// ============================================================
	default:
		// Log internal errors but don't expose details to client
		if logger != nil {
			logger.Error("Internal server error in Connect handler",
				zap.Error(err),
				zap.String("error_type", "unhandled_error"),
			)
		}
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
}
