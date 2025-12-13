package payment

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/handlers"
	"github.com/kevin07696/payment-service/internal/ports"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
)

// ConnectHandler implements the Connect RPC PaymentServiceHandler interface
type ConnectHandler struct {
	commandService ports.PaymentService
	queryService   ports.TransactionQueryService
	logger         *zap.Logger
}

// NewConnectHandler creates a new Connect RPC payment handler
func NewConnectHandler(
	commandService ports.PaymentService,
	queryService ports.TransactionQueryService,
	logger *zap.Logger,
) *ConnectHandler {
	return &ConnectHandler{
		commandService: commandService,
		queryService:   queryService,
		logger:         logger,
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
	if msg.OrderId != "" {
		serviceReq.OrderID = &msg.OrderId
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
	tx, err := h.commandService.Authorize(ctx, serviceReq)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	// Convert to proto response with customer-friendly message
	resp := transactionToPaymentResponse(tx)
	resp.Message = "Payment authorized"
	return connect.NewResponse(resp), nil
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
	tx, err := h.commandService.Capture(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Capture service error",
			zap.Error(err),
			zap.String("transaction_id", msg.TransactionId),
		)
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}
	h.logger.Info("Capture service succeeded", zap.String("transaction_id", msg.TransactionId))

	resp := transactionToPaymentResponse(tx)
	resp.Message = "Payment captured"
	return connect.NewResponse(resp), nil
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
	if msg.OrderId != "" {
		serviceReq.OrderID = &msg.OrderId
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

	// StdEntryClass defaults to WEB internally for ACH transactions

	tx, err := h.commandService.Sale(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Sale service error", zap.Error(err), zap.String("merchant_id", serviceReq.MerchantID))
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	resp := transactionToPaymentResponse(tx)
	resp.Message = "Payment processed successfully"
	return connect.NewResponse(resp), nil
}

// Void cancels an authorized or captured payment
func (h *ConnectHandler) Void(
	ctx context.Context,
	req *connect.Request[paymentv1.VoidRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
	msg := req.Msg

	h.logger.Info("Void request received",
		zap.String("transaction_id", msg.TransactionId),
	)

	if msg.TransactionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("transaction_id is required"))
	}

	serviceReq := &ports.VoidRequest{
		ParentTransactionID: msg.TransactionId,
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	tx, err := h.commandService.Void(ctx, serviceReq)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	resp := transactionToPaymentResponse(tx)
	resp.Message = "Payment cancelled"
	return connect.NewResponse(resp), nil
}

// Refund returns funds to the customer
func (h *ConnectHandler) Refund(
	ctx context.Context,
	req *connect.Request[paymentv1.RefundRequest],
) (*connect.Response[paymentv1.PaymentResponse], error) {
	msg := req.Msg

	h.logger.Info("Refund request received",
		zap.String("transaction_id", msg.TransactionId),
	)

	if msg.TransactionId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("transaction_id is required"))
	}

	serviceReq := &ports.RefundRequest{
		ParentTransactionID: msg.TransactionId,
		Reason:              msg.Reason,
	}

	if msg.AmountCents > 0 {
		serviceReq.AmountCents = &msg.AmountCents
	}

	if msg.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &msg.IdempotencyKey
	}

	tx, err := h.commandService.Refund(ctx, serviceReq)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
	}

	resp := transactionToPaymentResponse(tx)
	resp.Message = "Refund processed"
	return connect.NewResponse(resp), nil
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

	tx, err := h.queryService.GetTransaction(ctx, msg.TransactionId)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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
	if msg.OrderId != "" {
		filters.OrderID = &msg.OrderId
	}
	if msg.ParentTransactionId != "" {
		filters.ParentTransactionID = &msg.ParentTransactionId
	}
	if msg.Status != paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		statusStr := mapProtoStatusToDomain(msg.Status)
		filters.Status = &statusStr
	}

	txs, totalCount, err := h.queryService.ListTransactions(ctx, filters)
	if err != nil {
		return nil, handlers.HandleServiceErrorConnect(err, h.logger)
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

// NOTE: ACH operations are handled through the standard payment methods:
// - ACH Debit: Use Sale() with ACH payment method (uses CKC2)
// - ACH Credit/Refund: Use Refund() with ACH payment method (uses CKC3)
// - ACH Void: Use Void() with ACH payment method (uses CKCX)
//
// The adapter layer (determineTransactionType) automatically translates
// Operation + PaymentMethod to the correct EPX transaction type.
// No separate ACH-specific endpoints are needed.

