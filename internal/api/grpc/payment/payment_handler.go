package payment

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	paymentv1 "github.com/kevin07696/payment-service/api/proto/payment/v1"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/shopspring/decimal"
)

// Handler implements the gRPC Payment Service
type Handler struct {
	paymentv1.UnimplementedPaymentServiceServer
	paymentService ports.PaymentService
	logger         ports.Logger
}

// NewHandler creates a new payment gRPC handler
func NewHandler(paymentService ports.PaymentService, logger ports.Logger) *Handler {
	return &Handler{
		paymentService: paymentService,
		logger:         logger,
	}
}

// Authorize authorizes a payment without capturing
func (h *Handler) Authorize(ctx context.Context, req *paymentv1.AuthorizeRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("gRPC Authorize request received",
		ports.String("merchant_id", req.MerchantId),
		ports.String("customer_id", req.CustomerId))

	// Validate request
	if err := validateAuthorizeRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Parse amount
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid amount: %v", err))
	}

	// Convert to service request
	serviceReq := ports.ServiceAuthorizeRequest{
		MerchantID: req.MerchantId,
		CustomerID: req.CustomerId,
		Amount:     amount,
		Currency:   req.Currency,
		Token:      req.Token,
		BillingInfo: models.BillingInfo{
			FirstName: req.BillingInfo.FirstName,
			LastName:  req.BillingInfo.LastName,
			Email:     req.BillingInfo.Email,
			Phone:     req.BillingInfo.Phone,
			Address:   req.BillingInfo.Address.Street1,
			City:      req.BillingInfo.Address.City,
			State:     req.BillingInfo.Address.State,
			ZipCode:   req.BillingInfo.Address.PostalCode,
			Country:   req.BillingInfo.Address.Country,
		},
		IdempotencyKey: req.IdempotencyKey,
		Metadata:       req.Metadata,
	}

	// Call business logic
	resp, err := h.paymentService.Authorize(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Authorize failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("authorization failed: %v", err))
	}

	return toProtoPaymentResponse(resp), nil
}

// Capture captures a previously authorized payment
func (h *Handler) Capture(ctx context.Context, req *paymentv1.CaptureRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("gRPC Capture request received",
		ports.String("transaction_id", req.TransactionId))

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	var amount *decimal.Decimal
	if req.Amount != "" {
		amt, err := decimal.NewFromString(req.Amount)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid amount: %v", err))
		}
		amount = &amt
	}

	serviceReq := ports.ServiceCaptureRequest{
		TransactionID:  req.TransactionId,
		Amount:         amount,
		IdempotencyKey: req.IdempotencyKey,
	}

	resp, err := h.paymentService.Capture(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Capture failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("capture failed: %v", err))
	}

	return toProtoPaymentResponse(resp), nil
}

// Sale combines authorize and capture in one operation
func (h *Handler) Sale(ctx context.Context, req *paymentv1.SaleRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("gRPC Sale request received",
		ports.String("merchant_id", req.MerchantId),
		ports.String("customer_id", req.CustomerId))

	if err := validateSaleRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid amount: %v", err))
	}

	serviceReq := ports.ServiceSaleRequest{
		MerchantID: req.MerchantId,
		CustomerID: req.CustomerId,
		Amount:     amount,
		Currency:   req.Currency,
		Token:      req.Token,
		BillingInfo: models.BillingInfo{
			FirstName: req.BillingInfo.FirstName,
			LastName:  req.BillingInfo.LastName,
			Email:     req.BillingInfo.Email,
			Phone:     req.BillingInfo.Phone,
			Address:   req.BillingInfo.Address.Street1,
			City:      req.BillingInfo.Address.City,
			State:     req.BillingInfo.Address.State,
			ZipCode:   req.BillingInfo.Address.PostalCode,
			Country:   req.BillingInfo.Address.Country,
		},
		IdempotencyKey: req.IdempotencyKey,
		Metadata:       req.Metadata,
	}

	resp, err := h.paymentService.Sale(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Sale failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("sale failed: %v", err))
	}

	return toProtoPaymentResponse(resp), nil
}

// Void cancels an authorized or captured payment
func (h *Handler) Void(ctx context.Context, req *paymentv1.VoidRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("gRPC Void request received",
		ports.String("transaction_id", req.TransactionId))

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	serviceReq := ports.ServiceVoidRequest{
		TransactionID:  req.TransactionId,
		IdempotencyKey: req.IdempotencyKey,
	}

	resp, err := h.paymentService.Void(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Void failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("void failed: %v", err))
	}

	return toProtoPaymentResponse(resp), nil
}

// Refund refunds a captured payment
func (h *Handler) Refund(ctx context.Context, req *paymentv1.RefundRequest) (*paymentv1.PaymentResponse, error) {
	h.logger.Info("gRPC Refund request received",
		ports.String("transaction_id", req.TransactionId))

	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	var amount *decimal.Decimal
	if req.Amount != "" {
		amt, err := decimal.NewFromString(req.Amount)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid amount: %v", err))
		}
		amount = &amt
	}

	serviceReq := ports.ServiceRefundRequest{
		TransactionID:  req.TransactionId,
		Amount:         amount,
		Reason:         req.Reason,
		IdempotencyKey: req.IdempotencyKey,
	}

	resp, err := h.paymentService.Refund(ctx, serviceReq)
	if err != nil {
		h.logger.Error("Refund failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("refund failed: %v", err))
	}

	return toProtoPaymentResponse(resp), nil
}

// GetTransaction retrieves transaction details
func (h *Handler) GetTransaction(ctx context.Context, req *paymentv1.GetTransactionRequest) (*paymentv1.Transaction, error) {
	if req.TransactionId == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}

	txn, err := h.paymentService.GetTransaction(ctx, req.TransactionId)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("transaction not found: %v", err))
	}

	return toProtoTransaction(txn), nil
}

// ListTransactions lists transactions for a merchant or customer
func (h *Handler) ListTransactions(ctx context.Context, req *paymentv1.ListTransactionsRequest) (*paymentv1.ListTransactionsResponse, error) {
	h.logger.Info("gRPC ListTransactions request received",
		ports.String("merchant_id", req.MerchantId),
		ports.String("customer_id", req.CustomerId))

	// Validate merchant_id is required
	if req.MerchantId == "" {
		return nil, status.Error(codes.InvalidArgument, "merchant_id is required")
	}

	// Convert to service request
	serviceReq := ports.ServiceListTransactionsRequest{
		MerchantID: req.MerchantId,
		CustomerID: req.CustomerId,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	// Call service
	resp, err := h.paymentService.ListTransactions(ctx, serviceReq)
	if err != nil {
		h.logger.Error("ListTransactions failed", ports.String("error", err.Error()))
		return nil, status.Error(codes.Internal, fmt.Sprintf("list transactions failed: %v", err))
	}

	// Convert transactions to proto
	protoTransactions := make([]*paymentv1.Transaction, len(resp.Transactions))
	for i, txn := range resp.Transactions {
		protoTransactions[i] = toProtoTransaction(txn)
	}

	return &paymentv1.ListTransactionsResponse{
		Transactions: protoTransactions,
		TotalCount:   resp.TotalCount,
	}, nil
}

// Validation helpers

func validateAuthorizeRequest(req *paymentv1.AuthorizeRequest) error {
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
	}
	if req.CustomerId == "" {
		return fmt.Errorf("customer_id is required")
	}
	if req.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.Token == "" {
		return fmt.Errorf("token is required")
	}
	return nil
}

func validateSaleRequest(req *paymentv1.SaleRequest) error {
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
	}
	if req.CustomerId == "" {
		return fmt.Errorf("customer_id is required")
	}
	if req.Amount == "" {
		return fmt.Errorf("amount is required")
	}
	if req.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if req.Token == "" {
		return fmt.Errorf("token is required")
	}
	return nil
}

// Conversion helpers

func toProtoPaymentResponse(resp *ports.PaymentResponse) *paymentv1.PaymentResponse {
	// Parse CreatedAt from string to time
	// For now, we'll leave it as empty since the service returns a string
	return &paymentv1.PaymentResponse{
		TransactionId:        resp.TransactionID,
		Amount:               resp.Amount.String(),
		Currency:             resp.Currency,
		Status:               toProtoTransactionStatus(resp.Status),
		GatewayTransactionId: resp.GatewayTransactionID,
		ResponseCode:         resp.GatewayResponseCode,
		ResponseMessage:      resp.GatewayResponseMsg,
		AvsResponse:          resp.AVSResponse,
		CvvResponse:          resp.CVVResponse,
		IsApproved:           resp.IsApproved,
		// Note: PaymentResponse doesn't include all fields, would need to fetch full transaction for complete details
	}
}

func toProtoTransaction(txn *models.Transaction) *paymentv1.Transaction {
	return &paymentv1.Transaction{
		Id:                   txn.ID,
		MerchantId:           txn.MerchantID,
		CustomerId:           txn.CustomerID,
		Amount:               txn.Amount.String(),
		Currency:             txn.Currency,
		Status:               toProtoTransactionStatus(txn.Status),
		Type:                 toProtoTransactionType(txn.Type),
		PaymentMethodType:    toProtoPaymentMethodType(txn.PaymentMethodType),
		GatewayTransactionId: txn.GatewayTransactionID,
		PaymentMethodToken:   txn.PaymentMethodToken,
		ResponseCode:         txn.GatewayResponseCode,
		ResponseMessage:      txn.GatewayResponseMsg,
		IdempotencyKey:       txn.IdempotencyKey,
		CreatedAt:            timestamppb.New(txn.CreatedAt),
		UpdatedAt:            timestamppb.New(txn.UpdatedAt),
		Metadata:             txn.Metadata,
	}
}

func toProtoTransactionStatus(status models.TransactionStatus) paymentv1.TransactionStatus {
	switch status {
	case models.StatusPending:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_PENDING
	case models.StatusAuthorized:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_AUTHORIZED
	case models.StatusCaptured:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_CAPTURED
	case models.StatusVoided:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_VOIDED
	case models.StatusRefunded:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_REFUNDED
	case models.StatusFailed:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_FAILED
	default:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

func toProtoTransactionType(txnType models.TransactionType) paymentv1.TransactionType {
	switch txnType {
	case models.TypeAuthorization:
		return paymentv1.TransactionType_TRANSACTION_TYPE_AUTHORIZATION
	case models.TypeCapture:
		return paymentv1.TransactionType_TRANSACTION_TYPE_CAPTURE
	case models.TypeSale:
		return paymentv1.TransactionType_TRANSACTION_TYPE_SALE
	case models.TypeVoid:
		return paymentv1.TransactionType_TRANSACTION_TYPE_VOID
	case models.TypeRefund:
		return paymentv1.TransactionType_TRANSACTION_TYPE_REFUND
	default:
		return paymentv1.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func toProtoPaymentMethodType(pmType models.PaymentMethodType) paymentv1.PaymentMethodType {
	switch pmType {
	case models.PaymentMethodCreditCard:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD
	case models.PaymentMethodACH:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH
	default:
		return paymentv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED
	}
}
