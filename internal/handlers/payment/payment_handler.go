package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
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
		zap.String("merchant_id", req.MerchantId),
		zap.Int64("amount_cents", req.AmountCents),
	)

	// Validate request
	if err := validateAuthorizeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Convert to service request
	serviceReq := &ports.AuthorizeRequest{
		MerchantID:  req.MerchantId,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		Metadata:    convertMetadata(req.Metadata),
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

	if req.AmountCents > 0 {
		serviceReq.AmountCents = &req.AmountCents
	}

	if req.IdempotencyKey != "" {
		serviceReq.IdempotencyKey = &req.IdempotencyKey
	}

	h.logger.Info("Calling capture service", zap.String("transaction_id", req.TransactionId))
	tx, err := h.service.Capture(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Capture service error",
			zap.Error(err),
			zap.String("transaction_id", req.TransactionId),
		)
		return nil, handleServiceError(err)
	}
	h.logger.Info("Capture service succeeded", zap.String("transaction_id", req.TransactionId))

	return transactionToPaymentResponse(tx), nil
}

// Sale combines authorize and capture in one operation
func (h *Handler) Sale(ctx context.Context, req *paymentv1.SaleRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("Sale request received",
		zap.String("merchant_id", req.MerchantId),
		zap.Int64("amount_cents", req.AmountCents),
	)

	// Validate request
	if err := validateSaleRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	serviceReq := &ports.SaleRequest{
		MerchantID:  req.MerchantId,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		Metadata:    convertMetadata(req.Metadata),
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
		ParentTransactionID: req.TransactionId,
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
		ParentTransactionID: req.TransactionId,
		Reason:              req.Reason,
	}

	if req.AmountCents > 0 {
		serviceReq.AmountCents = &req.AmountCents
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
	if req.MerchantId == "" {
		return nil, status.Error(codes.InvalidArgument, "merchant_id is required")
	}

	// Build filter parameters from request
	filters := &ports.ListTransactionsFilters{
		MerchantID: &req.MerchantId,
		Limit:      int(req.Limit),
		Offset:     int(req.Offset),
	}

	// Add optional filters
	if req.CustomerId != "" {
		filters.CustomerID = &req.CustomerId
	}
	if req.ParentTransactionId != "" {
		filters.ParentTransactionID = &req.ParentTransactionId
	}
	if req.Status != paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		statusStr := protoStatusToDomain(req.Status)
		filters.Status = &statusStr
	}

	txs, totalCount, err := h.service.ListTransactions(ctx, filters)
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
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
	}
	if req.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be greater than 0")
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
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
	}
	if req.AmountCents <= 0 {
		return fmt.Errorf("amount_cents must be greater than 0")
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
		TransactionId:       tx.ID,
		ParentTransactionId: stringPtrToString(tx.ParentTransactionID),
		AmountCents:         tx.AmountCents,
		Currency:            string(tx.Currency),
		Status:              transactionStatusToProto(tx.Status),
		Type:                transactionTypeToProto(tx.Type),
		IsApproved:          tx.IsApproved(),
		AuthorizationCode:   stringPtrToString(tx.AuthCode),
		Message:             stringPtrToString(tx.AuthRespText),
		Card:                extractCardInfo(tx),
		CreatedAt:           timestamppb.New(tx.CreatedAt),
	}
}

// extractCardInfo converts gateway-specific card data to clean CardInfo
func extractCardInfo(tx *domain.Transaction) *paymentv1.CardInfo {
	if tx.AuthCardType == nil {
		return nil
	}

	// Convert EPX card type codes to clean brand names
	brand := epxCardTypeToBrand(*tx.AuthCardType)
	lastFour := extractLastFour(tx)

	if brand == "" && lastFour == "" {
		return nil
	}

	return &paymentv1.CardInfo{
		Brand:    brand,
		LastFour: lastFour,
	}
}

// epxCardTypeToBrand converts EPX card type codes to clean brand names
func epxCardTypeToBrand(epxCode string) string {
	switch epxCode {
	case "V":
		return "visa"
	case "M":
		return "mastercard"
	case "A":
		return "amex"
	case "D":
		return "discover"
	default:
		return ""
	}
}

// extractLastFour extracts last 4 digits from transaction metadata or linked payment method
func extractLastFour(tx *domain.Transaction) string {
	// Check transaction metadata for last_four (from gateway response)
	if tx.Metadata != nil {
		if lastFour, ok := tx.Metadata["last_four"].(string); ok && lastFour != "" {
			return lastFour
		}
		// Check for AUTH_MASKED_ACCOUNT_NBR (EPX field)
		if maskedAcct, ok := tx.Metadata["AUTH_MASKED_ACCOUNT_NBR"].(string); ok && len(maskedAcct) >= 4 {
			return maskedAcct[len(maskedAcct)-4:]
		}
		// Check for CARD_NBR (EPX field)
		if cardNbr, ok := tx.Metadata["CARD_NBR"].(string); ok && len(cardNbr) >= 4 {
			return cardNbr[len(cardNbr)-4:]
		}
	}

	// Note: If payment_method_id is present, caller should fetch the payment method separately
	// to get last_four. We don't fetch it here to avoid N+1 query issues.
	return ""
}

func transactionToProto(tx *domain.Transaction) *paymentv1.Transaction {
	proto := &paymentv1.Transaction{
		Id:                  tx.ID,
		ParentTransactionId: stringPtrToString(tx.ParentTransactionID),
		MerchantId:          tx.MerchantID,
		CustomerId:          stringPtrToString(tx.CustomerID),
		AmountCents:         tx.AmountCents,
		Currency:            string(tx.Currency),
		Status:              transactionStatusToProto(tx.Status),
		Type:                transactionTypeToProto(tx.Type),
		PaymentMethodType:   paymentMethodTypeToProto(tx.PaymentMethodType),
		AuthorizationCode:   stringPtrToString(tx.AuthCode),
		Message:             stringPtrToString(tx.AuthRespText),
		Card:                extractCardInfo(tx),
		IdempotencyKey:      stringPtrToString(tx.IdempotencyKey),
		CreatedAt:           timestamppb.New(tx.CreatedAt),
		UpdatedAt:           timestamppb.New(tx.UpdatedAt),
	}

	if tx.PaymentMethodID != nil {
		proto.PaymentMethodId = *tx.PaymentMethodID
	}

	return proto
}

func transactionStatusToProto(status domain.TransactionStatus) paymentv1.TransactionStatus {
	switch status {
	case domain.TransactionStatusApproved:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_APPROVED
	case domain.TransactionStatusDeclined:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_DECLINED
	default:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

func protoStatusToDomain(status paymentv1.TransactionStatus) string {
	switch status {
	case paymentv1.TransactionStatus_TRANSACTION_STATUS_APPROVED:
		return string(domain.TransactionStatusApproved)
	case paymentv1.TransactionStatus_TRANSACTION_STATUS_DECLINED:
		return string(domain.TransactionStatusDeclined)
	default:
		return ""
	}
}

func transactionTypeToProto(txType domain.TransactionType) paymentv1.TransactionType {
	switch txType {
	case domain.TransactionTypeAuth:
		return paymentv1.TransactionType_TRANSACTION_TYPE_AUTH
	case domain.TransactionTypeCapture:
		return paymentv1.TransactionType_TRANSACTION_TYPE_CAPTURE
	case domain.TransactionTypeSale:
		return paymentv1.TransactionType_TRANSACTION_TYPE_CHARGE // Proto uses CHARGE for SALE
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
	case errors.Is(err, domain.ErrMerchantInactive):
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
