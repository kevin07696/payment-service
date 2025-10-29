package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// Handler implements the gRPC PaymentServiceServer
type Handler struct {
	paymentv1.UnimplementedPaymentServiceServer
	service ports.PaymentService
	logger  *zap.Logger
}

// NewHandler creates a new payment handler
func NewHandler(service ports.PaymentService, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// Authorize holds funds on a payment method without capturing
func (h *Handler) Authorize(ctx context.Context, req *paymentv1.AuthorizeRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("Authorize request received",
		zap.String("agent_id", req.AgentId),
		zap.String("amount", req.Amount),
	)

	// Validate request
	if err := validateAuthorizeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Convert to service request
	serviceReq := &ports.AuthorizeRequest{
		AgentID:  req.AgentId,
		Amount:   req.Amount,
		Currency: req.Currency,
		Metadata: convertMetadata(req.Metadata),
	}

	if req.CustomerId != "" {
		serviceReq.CustomerID = &req.CustomerId
	}

	// Handle payment method oneof
	switch pm := req.PaymentMethod.(type) {
	case *paymentv1.AuthorizeRequest_PaymentMethodId:
		serviceReq.PaymentMethodID = &pm.PaymentMethodId
	case *paymentv1.AuthorizeRequest_PaymentToken:
		serviceReq.PaymentToken = &pm.PaymentToken
	default:
		return nil, status.Error(codes.InvalidArgument, "payment_method is required")
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	// Call service
	tx, err := h.service.Authorize(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	// Convert to proto response
	return transactionToPaymentResponse(tx), nil
}

// Capture completes a previously authorized payment
func (h *Handler) Capture(ctx context.Context, req *paymentv1.CaptureRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("Capture request received",
		zap.String("transaction_id", req.TransactionId),
	)

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	serviceReq := &ports.CaptureRequest{
		TransactionID: req.TransactionId,
	}

	if req.Amount != "" {
		serviceReq.Amount = &req.Amount
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	tx, err := h.service.Capture(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return transactionToPaymentResponse(tx), nil
}

// Sale combines authorize and capture in one operation
func (h *Handler) Sale(ctx context.Context, req *paymentv1.SaleRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("Sale request received",
		zap.String("agent_id", req.AgentId),
		zap.String("amount", req.Amount),
	)

	// Validate request
	if err := validateSaleRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	serviceReq := &ports.SaleRequest{
		AgentID:  req.AgentId,
		Amount:   req.Amount,
		Currency: req.Currency,
		Metadata: convertMetadata(req.Metadata),
	}

	if req.CustomerId != "" {
		serviceReq.CustomerID = &req.CustomerId
	}

	// Handle payment method oneof
	switch pm := req.PaymentMethod.(type) {
	case *paymentv1.SaleRequest_PaymentMethodId:
		serviceReq.PaymentMethodID = &pm.PaymentMethodId
	case *paymentv1.SaleRequest_PaymentToken:
		serviceReq.PaymentToken = &pm.PaymentToken
	default:
		return nil, status.Error(codes.InvalidArgument, "payment_method is required")
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	tx, err := h.service.Sale(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return transactionToPaymentResponse(tx), nil
}

// Void cancels an authorized or captured payment
func (h *Handler) Void(ctx context.Context, req *paymentv1.VoidRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("Void request received",
		zap.String("transaction_id", req.TransactionId),
	)

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	serviceReq := &ports.VoidRequest{
		TransactionID: req.TransactionId,
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	tx, err := h.service.Void(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return transactionToPaymentResponse(tx), nil
}

// Refund returns funds to the customer
func (h *Handler) Refund(ctx context.Context, req *paymentv1.RefundRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("Refund request received",
		zap.String("transaction_id", req.TransactionId),
	)

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	serviceReq := &ports.RefundRequest{
		TransactionID: req.TransactionId,
		Reason:        req.Reason,
	}

	if req.Amount != "" {
		serviceReq.Amount = &req.Amount
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	tx, err := h.service.Refund(ctx, serviceReq)
	if err != nil {
		return nil, handleServiceError(err)
	}

	return transactionToPaymentResponse(tx), nil
}

// GetTransaction retrieves transaction details
func (h *Handler) GetTransaction(ctx context.Context, req *paymentv1.GetTransactionRequest) (*paymentv1.Transaction, error) {
	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	tx, err := h.service.GetTransaction(ctx, req.TransactionId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "transaction not found")
		}
		return nil, status.Error(codes.Internal, "failed to get transaction")
	}

	return transactionToProto(tx), nil
}

// ListTransactions lists transactions for a merchant or customer
func (h *Handler) ListTransactions(ctx context.Context, req *paymentv1.ListTransactionsRequest) (*paymentv1.ListTransactionsResponse, error) {
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}

	// Handle group_id query
	if req.GroupId != "" {
		txs, err := h.service.GetTransactionsByGroup(ctx, req.GroupId)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to list transactions by group")
		}

		protoTxs := make([]*paymentv1.Transaction, len(txs))
		for i, tx := range txs {
			protoTxs[i] = transactionToProto(tx)
		}

		return &paymentv1.ListTransactionsResponse{
			Transactions: protoTxs,
			TotalCount:   int32(len(txs)),
		}, nil
	}

	// Default pagination
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.Offset)

	var customerID *string
	if req.CustomerId != "" {
		customerID = &req.CustomerId
	}

	txs, totalCount, err := h.service.ListTransactions(ctx, req.AgentId, customerID, limit, offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list transactions")
	}

	protoTxs := make([]*paymentv1.Transaction, len(txs))
	for i, tx := range txs {
		protoTxs[i] = transactionToProto(tx)
	}

	return &paymentv1.ListTransactionsResponse{
		Transactions: protoTxs,
		TotalCount:   int32(totalCount),
	}, nil
}

// Validation helpers

func validateAuthorizeRequest(req *paymentv1.AuthorizeRequest) error {
	if req.AgentId == "" {
		return fmt.Errorf("agent_id is required")
	}
	if req.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.PaymentMethod == nil {
		return fmt.Errorf("payment_method is required")
	}
	return nil
}

func validateSaleRequest(req *paymentv1.SaleRequest) error {
	if req.AgentId == "" {
		return fmt.Errorf("agent_id is required")
	}
	if req.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.PaymentMethod == nil {
		return fmt.Errorf("payment_method is required")
	}
	return nil
}

// Conversion helpers

func convertMetadata(meta map[string]string) map[string]interface{} {
	if meta == nil {
		return nil
	}
	result := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		result[k] = v
	}
	return result
}

func transactionToPaymentResponse(tx *domain.Transaction) *paymentv1.PaymentResponse {
	return &paymentv1.PaymentResponse{
		TransactionId:     tx.ID,
		GroupId:           tx.GroupID,
		AgentId:           tx.AgentID,
		CustomerId:        stringPtrToString(tx.CustomerID),
		Amount:            tx.Amount.String(),
		Currency:          string(tx.Currency),
		Status:            transactionStatusToProto(tx.Status),
		Type:              transactionTypeToProto(tx.Type),
		PaymentMethodType: paymentMethodTypeToProto(tx.PaymentMethodType),
		AuthGuid:          stringPtrToString(tx.AuthGUID),
		AuthResp:          stringPtrToString(tx.AuthResp),
		AuthCode:          stringPtrToString(tx.AuthCode),
		AuthRespText:      stringPtrToString(tx.AuthRespText),
		AuthCardType:      stringPtrToString(tx.AuthCardType),
		AuthAvs:           stringPtrToString(tx.AuthAVS),
		AuthCvv2:          stringPtrToString(tx.AuthCVV2),
		IsApproved:        tx.IsApproved(),
		CreatedAt:         timestamppb.New(tx.CreatedAt),
		Metadata:          convertMetadataToProto(tx.Metadata),
	}
}

func transactionToProto(tx *domain.Transaction) *paymentv1.Transaction {
	proto := &paymentv1.Transaction{
		Id:                tx.ID,
		GroupId:           tx.GroupID,
		AgentId:           tx.AgentID,
		CustomerId:        stringPtrToString(tx.CustomerID),
		Amount:            tx.Amount.String(),
		Currency:          string(tx.Currency),
		Status:            transactionStatusToProto(tx.Status),
		Type:              transactionTypeToProto(tx.Type),
		PaymentMethodType: paymentMethodTypeToProto(tx.PaymentMethodType),
		AuthGuid:          stringPtrToString(tx.AuthGUID),
		AuthResp:          stringPtrToString(tx.AuthResp),
		AuthCode:          stringPtrToString(tx.AuthCode),
		AuthRespText:      stringPtrToString(tx.AuthRespText),
		AuthCardType:      stringPtrToString(tx.AuthCardType),
		AuthAvs:           stringPtrToString(tx.AuthAVS),
		AuthCvv2:          stringPtrToString(tx.AuthCVV2),
		IdempotencyKey:    stringPtrToString(tx.IdempotencyKey),
		CreatedAt:         timestamppb.New(tx.CreatedAt),
		UpdatedAt:         timestamppb.New(tx.UpdatedAt),
		Metadata:          convertMetadataToProto(tx.Metadata),
	}

	if tx.PaymentMethodID != nil {
		proto.PaymentMethodId = *tx.PaymentMethodID
	}

	return proto
}

func transactionStatusToProto(status domain.TransactionStatus) paymentv1.TransactionStatus {
	switch status {
	case domain.TransactionStatusPending:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_PENDING
	case domain.TransactionStatusCompleted:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_COMPLETED
	case domain.TransactionStatusFailed:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_FAILED
	case domain.TransactionStatusRefunded:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_REFUNDED
	case domain.TransactionStatusVoided:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_VOIDED
	default:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

func transactionTypeToProto(txType domain.TransactionType) paymentv1.TransactionType {
	switch txType {
	case domain.TransactionTypeAuth:
		return paymentv1.TransactionType_TRANSACTION_TYPE_AUTH
	case domain.TransactionTypeCapture:
		return paymentv1.TransactionType_TRANSACTION_TYPE_CAPTURE
	case domain.TransactionTypeCharge:
		return paymentv1.TransactionType_TRANSACTION_TYPE_CHARGE
	case domain.TransactionTypeRefund:
		return paymentv1.TransactionType_TRANSACTION_TYPE_REFUND
	case domain.TransactionTypePreNote:
		return paymentv1.TransactionType_TRANSACTION_TYPE_PRE_NOTE
	default:
		return paymentv1.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func paymentMethodTypeToProto(pmType domain.PaymentMethodType) paymentv1.PaymentMethodType {
	switch pmType {
	case domain.PaymentMethodTypeCreditCard:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD
	case domain.PaymentMethodTypeACH:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH
	default:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED
	}
}

func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func convertMetadataToProto(meta map[string]interface{}) map[string]string {
	if meta == nil {
		return nil
	}
	result := make(map[string]string, len(meta))
	for k, v := range meta {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// Error handling

func handleServiceError(err error) error {
	// Map domain errors to gRPC status codes
	switch {
	case errors.Is(err, domain.ErrAgentInactive):
		return status.Error(codes.FailedPrecondition, "agent is inactive")
	case errors.Is(err, domain.ErrPaymentMethodNotFound):
		return status.Error(codes.NotFound, "payment method not found")
	case errors.Is(err, domain.ErrTransactionCannotBeVoided):
		return status.Error(codes.FailedPrecondition, "transaction cannot be voided")
	case errors.Is(err, domain.ErrTransactionCannotBeCaptured):
		return status.Error(codes.FailedPrecondition, "transaction cannot be captured")
	case errors.Is(err, domain.ErrTransactionCannotBeRefunded):
		return status.Error(codes.FailedPrecondition, "transaction cannot be refunded")
	case errors.Is(err, domain.ErrTransactionNotFound):
		return status.Error(codes.NotFound, "transaction not found")
	case errors.Is(err, domain.ErrTransactionDeclined):
		return status.Error(codes.Aborted, "transaction was declined")
	case errors.Is(err, domain.ErrInvalidAmount):
		return status.Error(codes.InvalidArgument, "invalid amount")
	case errors.Is(err, domain.ErrInvalidCurrency):
		return status.Error(codes.InvalidArgument, "invalid currency")
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		return status.Error(codes.AlreadyExists, "duplicate idempotency key")
	case errors.Is(err, sql.ErrNoRows):
		return status.Error(codes.NotFound, "resource not found")
	case err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)):
		return status.Error(codes.Canceled, "request canceled")
	default:
		// Log internal errors but don't expose details to client
		return status.Error(codes.Internal, "internal server error")
	}
}
