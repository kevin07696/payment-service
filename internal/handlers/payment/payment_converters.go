package payment

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	paymentv1 "github.com/kevin07696/payment-service/proto/payment/v1"
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
		Status:              mapDomainStatusToProto(tx.Status),
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
