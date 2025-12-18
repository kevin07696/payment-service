// Package handlers provides shared utilities for API handlers.
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

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

	// ============================================================
	// Transaction Errors
	// ============================================================
	case errors.Is(err, domain.ErrTxnNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("transaction not found"))
	case errors.Is(err, domain.ErrTxnCannotBeVoided):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be voided"))
	case errors.Is(err, domain.ErrTxnCannotBeCaptured):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be captured"))
	case errors.Is(err, domain.ErrTxnCannotBeRefunded):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be refunded"))
	case errors.Is(err, domain.ErrTxnDeclined):
		return connect.NewError(connect.CodeAborted, errors.New("transaction was declined"))

	// ============================================================
	// Payment Method Errors
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

	// ============================================================
	// Subscription Errors
	// ============================================================
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("subscription not found"))
	case errors.Is(err, domain.ErrSubscriptionNotActive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("subscription is not active"))
	case errors.Is(err, domain.ErrSubscriptionAlreadyCancelled):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("subscription is already cancelled"))
	case errors.Is(err, domain.ErrSubscriptionInvalidInterval):
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

// HTTPErrorResponse contains the sanitized error response for HTTP handlers.
// Details field is intentionally omitted to prevent leaking internal information.
type HTTPErrorResponse struct {
	Message    string
	StatusCode int
}

// HandleServiceErrorHTTP maps domain errors to HTTP status codes and sanitized messages.
// This is the centralized error handler for Browser POST and other HTTP handlers.
// It ensures internal error details are never exposed to clients.
//
// Security: Always logs full error server-side, returns generic message to client.
func HandleServiceErrorHTTP(err error, logger *zap.Logger) HTTPErrorResponse {
	if err == nil {
		return HTTPErrorResponse{Message: "", StatusCode: http.StatusOK}
	}

	// Log full error details server-side for debugging
	if logger != nil {
		logger.Error("HTTP handler error",
			zap.Error(err),
			zap.String("error_type", "http_handler_error"),
		)
	}

	switch {
	// ============================================================
	// Authentication & Authorization Errors
	// ============================================================
	case errors.Is(err, domain.ErrAuthMerchantMismatch):
		return HTTPErrorResponse{Message: "Access denied: merchant mismatch", StatusCode: http.StatusForbidden}
	case errors.Is(err, domain.ErrAuthAccessDenied):
		return HTTPErrorResponse{Message: "Access denied", StatusCode: http.StatusForbidden}

	// ============================================================
	// Merchant Errors
	// ============================================================
	case errors.Is(err, domain.ErrMerchantNotFoundTyped):
		return HTTPErrorResponse{Message: "Merchant not found", StatusCode: http.StatusNotFound}
	case errors.Is(err, domain.ErrMerchantInactiveTyped):
		return HTTPErrorResponse{Message: "Merchant is inactive", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrMerchantAlreadyExists):
		return HTTPErrorResponse{Message: "Merchant already exists", StatusCode: http.StatusConflict}
	case errors.Is(err, domain.ErrMerchantInvalidEnv):
		return HTTPErrorResponse{Message: "Invalid environment", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrMerchantRequired):
		return HTTPErrorResponse{Message: "Merchant ID is required", StatusCode: http.StatusBadRequest}

	// ============================================================
	// Transaction Errors
	// ============================================================
	case errors.Is(err, domain.ErrTxnNotFound):
		return HTTPErrorResponse{Message: "Transaction not found", StatusCode: http.StatusNotFound}
	case errors.Is(err, domain.ErrTxnCannotBeVoided):
		return HTTPErrorResponse{Message: "Transaction cannot be voided", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrTxnCannotBeCaptured):
		return HTTPErrorResponse{Message: "Transaction cannot be captured", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrTxnCannotBeRefunded):
		return HTTPErrorResponse{Message: "Transaction cannot be refunded", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrTxnDeclined):
		return HTTPErrorResponse{Message: "Transaction was declined", StatusCode: http.StatusPaymentRequired}

	// ============================================================
	// Payment Method Errors
	// ============================================================
	case errors.Is(err, domain.ErrPMNotFound):
		return HTTPErrorResponse{Message: "Payment method not found", StatusCode: http.StatusNotFound}
	case errors.Is(err, domain.ErrPMExpired):
		return HTTPErrorResponse{Message: "Payment method is expired", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrPMNotVerified):
		return HTTPErrorResponse{Message: "ACH payment method is not verified", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrPMInactive):
		return HTTPErrorResponse{Message: "Payment method is inactive", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrPMInvalidType):
		return HTTPErrorResponse{Message: "Invalid payment method type", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrPMNotBelongToCustomer):
		return HTTPErrorResponse{Message: "Payment method does not belong to customer", StatusCode: http.StatusForbidden}

	// ============================================================
	// Subscription Errors
	// ============================================================
	case errors.Is(err, domain.ErrSubscriptionNotFound):
		return HTTPErrorResponse{Message: "Subscription not found", StatusCode: http.StatusNotFound}
	case errors.Is(err, domain.ErrSubscriptionNotActive):
		return HTTPErrorResponse{Message: "Subscription is not active", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrSubscriptionAlreadyCancelled):
		return HTTPErrorResponse{Message: "Subscription is already cancelled", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrSubscriptionInvalidInterval):
		return HTTPErrorResponse{Message: "Invalid billing interval", StatusCode: http.StatusBadRequest}

	// ============================================================
	// Validation Errors
	// ============================================================
	case errors.Is(err, domain.ErrValidationInvalidUUID):
		return HTTPErrorResponse{Message: "Invalid UUID format", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrInvalidAmount):
		return HTTPErrorResponse{Message: "Invalid amount", StatusCode: http.StatusBadRequest}
	case errors.Is(err, domain.ErrInvalidCurrency):
		return HTTPErrorResponse{Message: "Invalid currency", StatusCode: http.StatusBadRequest}

	// ============================================================
	// Idempotency Errors
	// ============================================================
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		return HTTPErrorResponse{Message: "Duplicate request", StatusCode: http.StatusConflict}

	// ============================================================
	// Database Errors
	// ============================================================
	case errors.Is(err, domain.ErrDatabaseError):
		return HTTPErrorResponse{Message: "A database error occurred", StatusCode: http.StatusInternalServerError}
	case errors.Is(err, sql.ErrNoRows):
		return HTTPErrorResponse{Message: "Resource not found", StatusCode: http.StatusNotFound}

	// ============================================================
	// Context Errors
	// ============================================================
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return HTTPErrorResponse{Message: "Request timed out", StatusCode: http.StatusGatewayTimeout}

	// ============================================================
	// Default: Internal Server Error
	// ============================================================
	default:
		return HTTPErrorResponse{Message: "An error occurred", StatusCode: http.StatusInternalServerError}
	}
}
