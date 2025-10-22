package north

import (
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
)

// ResponseCodeInfo contains detailed information about a response code
type ResponseCodeInfo struct {
	Code               string
	Display            string
	Description        string
	IsApproved         bool
	IsDeclined         bool
	IsRetriable        bool
	RequiresUserAction bool
	Category           pkgerrors.ErrorCategory
	UserMessage        string
}

// Response code map for credit card transactions
var creditCardResponseCodes = map[string]ResponseCodeInfo{
	// Approval
	"00": {
		Code:        "00",
		Display:     "APPROVAL",
		Description: "Transaction approved",
		IsApproved:  true,
		Category:    pkgerrors.CategoryApproved,
		UserMessage: "Payment successful",
	},

	// Insufficient Funds
	"51": {
		Code:               "51",
		Display:            "INSUFF FUNDS",
		Description:        "Insufficient funds in account",
		IsDeclined:         true,
		IsRetriable:        true,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryInsufficientFunds,
		UserMessage:        "Insufficient funds. Please use a different payment method or add funds to your account.",
	},

	// Expired Card
	"54": {
		Code:               "54",
		Display:            "EXP CARD",
		Description:        "Expired card",
		IsDeclined:         true,
		IsRetriable:        true,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryExpiredCard,
		UserMessage:        "Your card has expired. Please use a different payment method.",
	},

	// CVV Error
	"82": {
		Code:               "82",
		Display:            "CVV ERROR",
		Description:        "CVV verification failed",
		IsDeclined:         true,
		IsRetriable:        true,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryInvalidCard,
		UserMessage:        "Incorrect CVV. Please check the security code on your card.",
	},

	// Invalid Card
	"14": {
		Code:               "14",
		Display:            "INVALID ACCT",
		Description:        "Invalid card number",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryInvalidCard,
		UserMessage:        "Invalid card number. Please check your card details.",
	},

	// Fraud/Security
	"59": {
		Code:               "59",
		Display:            "SUSPECTED FRAUD",
		Description:        "Suspected fraud",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryFraud,
		UserMessage:        "Transaction declined for security reasons. Please contact your bank.",
	},

	// System Errors (retriable)
	"91": {
		Code:        "91",
		Display:     "TIMEOUT",
		Description: "Issuer or switch timeout",
		IsDeclined:  true,
		IsRetriable: true,
		Category:    pkgerrors.CategorySystemError,
		UserMessage: "Transaction timeout. Please try again.",
	},
	"96": {
		Code:        "96",
		Display:     "SYSTEM ERROR",
		Description: "System malfunction",
		IsDeclined:  true,
		IsRetriable: true,
		Category:    pkgerrors.CategorySystemError,
		UserMessage: "System error. Please try again in a few moments.",
	},

	// Decline (generic)
	"05": {
		Code:               "05",
		Display:            "DECLINE",
		Description:        "Do not honor",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryDeclined,
		UserMessage:        "Transaction declined by your bank. Please contact your bank or use a different payment method.",
	},

	// Lost/Stolen Card
	"41": {
		Code:               "41",
		Display:            "LOST CARD",
		Description:        "Lost card, pick up",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryFraud,
		UserMessage:        "Card reported as lost. Please contact your bank.",
	},
	"43": {
		Code:               "43",
		Display:            "STOLEN CARD",
		Description:        "Stolen card, pick up",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryFraud,
		UserMessage:        "Card reported as stolen. Please contact your bank.",
	},
}

// ACH response codes
var achResponseCodes = map[string]ResponseCodeInfo{
	"00": {
		Code:        "00",
		Display:     "ACCEPTED",
		Description: "Transaction accepted",
		IsApproved:  true,
		Category:    pkgerrors.CategoryApproved,
		UserMessage: "Payment successful",
	},
	"03": {
		Code:               "03",
		Display:            "INVALID MERCHANT",
		Description:        "Invalid merchant or service provider",
		IsDeclined:         true,
		IsRetriable:        false,
		Category:           pkgerrors.CategoryInvalidRequest,
		UserMessage:        "Merchant configuration error. Please contact support.",
	},
	"14": {
		Code:               "14",
		Display:            "INVALID ACCT NBR",
		Description:        "Invalid account number",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryInvalidCard,
		UserMessage:        "Invalid account number. Please check your bank account details.",
	},
	"52": {
		Code:               "52",
		Display:            "NO CHECK ACCOUNT",
		Description:        "No checking account",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryInvalidCard,
		UserMessage:        "No checking account found. Please verify your account details.",
	},
	"53": {
		Code:               "53",
		Display:            "NO SAVE ACCOUNT",
		Description:        "No savings account",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryInvalidCard,
		UserMessage:        "No savings account found. Please verify your account details.",
	},
	"78": {
		Code:               "78",
		Display:            "INVALID RTN NBR",
		Description:        "Invalid routing number",
		IsDeclined:         true,
		IsRetriable:        false,
		RequiresUserAction: true,
		Category:           pkgerrors.CategoryInvalidCard,
		UserMessage:        "Invalid routing number. Please check your bank routing number.",
	},
	"96": {
		Code:        "96",
		Display:     "SYSTEM ERROR",
		Description: "System error",
		IsDeclined:  true,
		IsRetriable: true,
		Category:    pkgerrors.CategorySystemError,
		UserMessage: "System error. Please try again in a few moments.",
	},
}

// GetCreditCardResponseCode retrieves response code information for credit card transactions
func GetCreditCardResponseCode(code string) ResponseCodeInfo {
	if info, exists := creditCardResponseCodes[code]; exists {
		return info
	}
	// Default for unknown codes
	return ResponseCodeInfo{
		Code:        code,
		Display:     "UNKNOWN",
		Description: "Unknown response code",
		IsDeclined:  true,
		IsRetriable: false,
		Category:    pkgerrors.CategoryDeclined,
		UserMessage: "Transaction declined. Please try a different payment method or contact support.",
	}
}

// GetACHResponseCode retrieves response code information for ACH transactions
func GetACHResponseCode(code string) ResponseCodeInfo {
	if info, exists := achResponseCodes[code]; exists {
		return info
	}
	// Default for unknown codes
	return ResponseCodeInfo{
		Code:        code,
		Display:     "UNKNOWN",
		Description: "Unknown response code",
		IsDeclined:  true,
		IsRetriable: false,
		Category:    pkgerrors.CategoryDeclined,
		UserMessage: "Transaction declined. Please try a different payment method or contact support.",
	}
}

// ToPaymentError converts a response code to a PaymentError
func (r ResponseCodeInfo) ToPaymentError(gatewayMessage string) *pkgerrors.PaymentError {
	return &pkgerrors.PaymentError{
		Code:           r.Code,
		Message:        r.UserMessage,
		GatewayMessage: gatewayMessage,
		IsRetriable:    r.IsRetriable,
		Category:       r.Category,
		Details:        map[string]interface{}{"display": r.Display, "description": r.Description},
	}
}
