package payment

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
)

// ConnectHandler implements the Connect RPC PaymentServiceHandler interface
type ConnectHandler struct {
	service ports.PaymentService
	logger  *zap.Logger
}

// NewConnectHandler creates a new Connect RPC payment handler
func NewConnectHandler(service ports.PaymentService, logger *zap.Logger) *ConnectHandler {
	return &ConnectHandler{
		service: service,
		logger:  logger,
	}
}

// Authorize holds funds on a payment method without capturing
func (h *ConnectHandler) Authorize(
	ctx context.Context,
	req *connect.Request[paymentv1.AuthorizeRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
	msg := req.Msg

	h.logger.Info("Authorize request received",
		zap.String("merchant_id", msg.MerchantId),
		zap.Int64("amount_cents", msg.AmountCents),
	)

	// Validate request
	if err := validateAuthorizeRequest(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Convert to service request
	serviceReq := &ports.AuthorizeRequest{
		MerchantID:  msg.MerchantId,
		AmountCents: msg.AmountCents,
		Currency:    msg.Currency,
		Metadata:    convertMetadata(msg.Metadata),
	}

	if msg.CustomerId != "" {
		serviceReq.CustomerID = &msg.CustomerId
	}

	// Handle payment method oneof
	switch pm := msg.PaymentMethod.(type) {
	case *paymentv1.AuthorizeRequest_PaymentMethodId:
		serviceReq.PaymentMethodID = &pm.PaymentMethodId
	case *paymentv1.AuthorizeRequest_PaymentToken:
		serviceReq.PaymentToken = &pm.PaymentToken
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_method is required"))
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	// Call service
	tx, err := h.service.Authorize(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	// Convert to proto response and wrap in Connect response
	return connect.NewResponse(transactionToPaymentResponse(tx)), nil
}

// Capture completes a previously authorized payment
func (h *ConnectHandler) Capture(
	ctx context.Context,
	req *connect.Request[paymentv1.CaptureRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
	msg := req.Msg

	h.logger.Info("Capture request received",
		zap.String("transaction_id", msg.TransactionId),
	)

	if msg.TransactionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("transaction_id is required"))
	}

	serviceReq := &ports.CaptureRequest{
		TransactionID: msg.TransactionId,
	}

	if msg.AmountCents > 0 {
		serviceReq.AmountCents = &msg.AmountCents
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	h.logger.Info("Calling capture service", zap.String("transaction_id", msg.TransactionId))
	tx, err := h.service.Capture(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Capture service error",
			zap.Error(err),
			zap.String("transaction_id", msg.TransactionId),
		)
		return nil, handleServiceErrorConnect(err)
	}
	h.logger.Info("Capture service succeeded", zap.String("transaction_id", msg.TransactionId))

	return connect.NewResponse(transactionToPaymentResponse(tx)), nil
}

// Sale combines authorize and capture in one operation
func (h *ConnectHandler) Sale(
	ctx context.Context,
	req *connect.Request[paymentv1.SaleRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
	msg := req.Msg

	h.logger.Info("Sale request received",
		zap.String("merchant_id", msg.MerchantId),
		zap.Int64("amount_cents", msg.AmountCents),
	)

	// Validate request
	if err := validateSaleRequest(msg); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	serviceReq := &ports.SaleRequest{
		MerchantID:  msg.MerchantId,
		AmountCents: msg.AmountCents,
		Currency:    msg.Currency,
		Metadata:    convertMetadata(msg.Metadata),
	}

	if msg.CustomerId != "" {
		serviceReq.CustomerID = &msg.CustomerId
	}

	// Handle payment method oneof
	switch pm := msg.PaymentMethod.(type) {
	case *paymentv1.SaleRequest_PaymentMethodId:
		serviceReq.PaymentMethodID = &pm.PaymentMethodId
	case *paymentv1.SaleRequest_PaymentToken:
		serviceReq.PaymentToken = &pm.PaymentToken
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payment_method is required"))
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	tx, err := h.service.Sale(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Sale service error", zap.Error(err), zap.String("merchant_id", serviceReq.MerchantID))
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(transactionToPaymentResponse(tx)), nil
}

// Void cancels an authorized or captured payment
func (h *ConnectHandler) Void(
	ctx context.Context,
	req *connect.Request[paymentv1.VoidRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
	msg := req.Msg

	h.logger.Info("Void request received",
		zap.String("group_id", msg.GroupId),
	)

	if msg.GroupId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("group_id is required"))
	}

	serviceReq := &ports.VoidRequest{
		ParentTransactionID: msg.GroupId,
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	tx, err := h.service.Void(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(transactionToPaymentResponse(tx)), nil
}

// Refund returns funds to the customer
func (h *ConnectHandler) Refund(
	ctx context.Context,
	req *connect.Request[paymentv1.RefundRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
	msg := req.Msg

	h.logger.Info("Refund request received",
		zap.String("group_id", msg.GroupId),
	)

	if msg.GroupId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("group_id is required"))
	}

	serviceReq := &ports.RefundRequest{
		ParentTransactionID: msg.GroupId,
		Reason:              msg.Reason,
	}

	if msg.AmountCents > 0 {
		serviceReq.AmountCents = &msg.AmountCents
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	tx, err := h.service.Refund(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(transactionToPaymentResponse(tx)), nil
}

// GetTransaction retrieves transaction details
func (h *ConnectHandler) GetTransaction(
	ctx context.Context,
	req *connect.Request[paymentv1.GetTransactionRequest],
) (*connect.Response[paymentv1.Transaction], error) {
	msg := req.Msg

	if msg.TransactionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("transaction_id is required"))
	}

	tx, err := h.service.GetTransaction(ctx, msg.TransactionId)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	return connect.NewResponse(transactionToProto(tx)), nil
}

// ListTransactions lists transactions for a merchant or customer
func (h *ConnectHandler) ListTransactions(
	ctx context.Context,
	req *connect.Request[paymentv1.ListTransactionsRequest],
) (*connect.Response[paymentv1.ListTransactionsResponse], error) {
	msg := req.Msg

	if msg.MerchantId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id is required"))
	}

	// Build filter parameters from request
	filters := &ports.ListTransactionsFilters{
		MerchantID: &msg.MerchantId,
		Limit:      int(msg.Limit),
		Offset:     int(msg.Offset),
	}

	// Add optional filters
	if msg.CustomerId != "" {
		filters.CustomerID = &msg.CustomerId
	}
	if msg.GroupId != "" {
		filters.ParentTransactionID = &msg.GroupId
	}
	if msg.Status != paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		statusStr := protoStatusToDomain(msg.Status)
		filters.Status = &statusStr
	}

	txs, totalCount, err := h.service.ListTransactions(ctx, filters)
	if err != nil {
		return nil, handleServiceErrorConnect(err)
	}

	protoTxs := make([]*paymentv1.Transaction, len(txs))
	for i, tx := range txs {
		protoTxs[i] = transactionToProto(tx)
	}

	response := &paymentv1.ListTransactionsResponse{
		Transactions: protoTxs,
		TotalCount:   int32(totalCount),
	}

	return connect.NewResponse(response), nil
}

// handleServiceErrorConnect maps domain errors to Connect error codes
func handleServiceErrorConnect(err error) error {
	// Map domain errors to Connect status codes
	switch {
	case errors.Is(err, domain.ErrMerchantInactive):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("agent is inactive"))
	case errors.Is(err, domain.ErrPaymentMethodNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("payment method not found"))
	case errors.Is(err, domain.ErrTransactionCannotBeVoided):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be voided"))
	case errors.Is(err, domain.ErrTransactionCannotBeCaptured):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be captured"))
	case errors.Is(err, domain.ErrTransactionCannotBeRefunded):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("transaction cannot be refunded"))
	case errors.Is(err, domain.ErrTransactionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("transaction not found"))
	case errors.Is(err, domain.ErrTransactionDeclined):
		return connect.NewError(connect.CodeAborted, errors.New("transaction was declined"))
	case errors.Is(err, domain.ErrInvalidAmount):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid amount"))
	case errors.Is(err, domain.ErrInvalidCurrency):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid currency"))
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("duplicate idempotency key"))
	case errors.Is(err, sql.ErrNoRows):
		return connect.NewError(connect.CodeNotFound, errors.New("resource not found"))
	case err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		return connect.NewError(connect.CodeCanceled, errors.New("request canceled"))
	default:
		// Log internal errors but don't expose details to client
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
}
