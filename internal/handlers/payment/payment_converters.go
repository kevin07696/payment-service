package payment

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
	paymentmethodv1 "github.com/kevin07696/payment-service/proto/payment_method/v1"
)

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
		Status:              mapDomainStatusToProto(tx.Status),
		Type:                transactionTypeToProto(tx.Type),
		IsApproved:          tx.IsApproved(),
		AuthorizationCode:   stringPtrToString(tx.AuthCode),
		Message:             stringPtrToString(tx.AuthRespText),
		Card:                buildCardInfo(tx.AuthCardType, tx.Metadata),
		CreatedAt:           timestamppb.New(tx.CreatedAt),
	}
}

// buildCardInfo extracts card info from transaction fields
func buildCardInfo(authCardType *string, metadata map[string]interface{}) *paymentv1.CardInfo {
	if authCardType == nil {
		return nil
	}

	brand := domain.CardBrandFromEPXCode(*authCardType)
	lastFour := extractLastFourFromMetadata(metadata)

	if !brand.IsKnown() && lastFour == "" {
		return nil
	}

	return &paymentv1.CardInfo{
		Brand:    brand.String(),
		LastFour: lastFour,
	}
}

// extractLastFourFromMetadata extracts last 4 digits from transaction metadata
func extractLastFourFromMetadata(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}

	// Check for pre-extracted last_four
	if lastFour, ok := metadata["last_four"].(string); ok && lastFour != "" {
		return lastFour
	}

	// Check for AUTH_MASKED_ACCOUNT_NBR (EPX field)
	if maskedAcct, ok := metadata["AUTH_MASKED_ACCOUNT_NBR"].(string); ok {
		return domain.ExtractLastFour(maskedAcct)
	}

	// Check for AUTH_ACCOUNT_NBR (EPX response field per North Developer Browser Post API Guide)
	if accountNbr, ok := metadata["AUTH_ACCOUNT_NBR"].(string); ok {
		return domain.ExtractLastFour(accountNbr)
	}

	return ""
}

func transactionToProto(tx *domain.Transaction) *paymentv1.Transaction {
	proto := &paymentv1.Transaction{
		Id:                  tx.ID,
		ParentTransactionId: stringPtrToString(tx.ParentTransactionID),
		MerchantId:          tx.MerchantID,
		CustomerId:          stringPtrToString(tx.CustomerID),
		OrderId:             stringPtrToString(tx.OrderID),
		AmountCents:         tx.AmountCents,
		Currency:            string(tx.Currency),
		Status:              mapDomainStatusToProto(tx.Status),
		Type:                transactionTypeToProto(tx.Type),
		PaymentMethodType:   paymentMethodTypeToProto(tx.PaymentMethodType),
		AuthorizationCode:   stringPtrToString(tx.AuthCode),
		Message:             stringPtrToString(tx.AuthRespText),
		Card:                buildCardInfo(tx.AuthCardType, tx.Metadata),
		IdempotencyKey:      stringPtrToString(tx.IdempotencyKey),
		CreatedAt:           timestamppb.New(tx.CreatedAt),
		UpdatedAt:           timestamppb.New(tx.UpdatedAt),
		ProcessorReference:  tx.AuthGUID, // BRIC/AUTH_GUID for refunds/voids
	}

	if tx.PaymentMethodID != nil {
		proto.PaymentMethodId = *tx.PaymentMethodID
	}

	return proto
}

func mapDomainStatusToProto(status domain.TransactionStatus) paymentv1.TransactionStatus {
	switch status {
	case domain.TransactionStatusApproved:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_APPROVED
	case domain.TransactionStatusDeclined:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_DECLINED
	default:
		return paymentv1.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED
	}
}

func mapProtoStatusToDomain(status paymentv1.TransactionStatus) string {
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

func paymentMethodTypeToProto(pmType domain.PaymentMethodType) paymentmethodv1.PaymentMethodType {
	switch pmType {
	case domain.PaymentMethodTypeCreditCard:
		return paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD
	case domain.PaymentMethodTypeACH:
		return paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_ACH
	default:
		return paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_UNSPECIFIED
	}
}

func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

