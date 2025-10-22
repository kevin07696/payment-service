package north

import (
	"testing"

	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestGetCreditCardResponseCode(t *testing.T) {
	tests := []struct {
		name               string
		code               string
		wantIsApproved     bool
		wantIsDeclined     bool
		wantIsRetriable    bool
		wantCategory       pkgerrors.ErrorCategory
		wantUserActionReq  bool
	}{
		{
			name:            "approval code 00",
			code:            "00",
			wantIsApproved:  true,
			wantIsDeclined:  false,
			wantIsRetriable: false,
			wantCategory:    pkgerrors.CategoryApproved,
		},
		{
			name:            "insufficient funds 51",
			code:            "51",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: true,
			wantCategory:    pkgerrors.CategoryInsufficientFunds,
			wantUserActionReq: true,
		},
		{
			name:            "expired card 54",
			code:            "54",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: true,
			wantCategory:    pkgerrors.CategoryExpiredCard,
			wantUserActionReq: true,
		},
		{
			name:            "CVV error 82",
			code:            "82",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: true,
			wantCategory:    pkgerrors.CategoryInvalidCard,
			wantUserActionReq: true,
		},
		{
			name:            "fraud 59",
			code:            "59",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: false,
			wantCategory:    pkgerrors.CategoryFraud,
			wantUserActionReq: true,
		},
		{
			name:            "system error 96",
			code:            "96",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: true,
			wantCategory:    pkgerrors.CategorySystemError,
		},
		{
			name:            "unknown code defaults to declined",
			code:            "999",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: false,
			wantCategory:    pkgerrors.CategoryDeclined,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetCreditCardResponseCode(tt.code)

			assert.Equal(t, tt.code, info.Code, "Code should match")
			assert.Equal(t, tt.wantIsApproved, info.IsApproved, "IsApproved mismatch")
			assert.Equal(t, tt.wantIsDeclined, info.IsDeclined, "IsDeclined mismatch")
			assert.Equal(t, tt.wantIsRetriable, info.IsRetriable, "IsRetriable mismatch")
			assert.Equal(t, tt.wantCategory, info.Category, "Category mismatch")

			if tt.wantUserActionReq {
				assert.True(t, info.RequiresUserAction, "Should require user action")
			}

			// All response codes should have user message
			assert.NotEmpty(t, info.UserMessage, "UserMessage should not be empty")
		})
	}
}

func TestGetACHResponseCode(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		wantIsApproved  bool
		wantIsDeclined  bool
		wantIsRetriable bool
		wantCategory    pkgerrors.ErrorCategory
	}{
		{
			name:            "approval code 00",
			code:            "00",
			wantIsApproved:  true,
			wantIsDeclined:  false,
			wantIsRetriable: false,
			wantCategory:    pkgerrors.CategoryApproved,
		},
		{
			name:            "invalid account 14",
			code:            "14",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: false,
			wantCategory:    pkgerrors.CategoryInvalidCard,
		},
		{
			name:            "invalid routing 78",
			code:            "78",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: false,
			wantCategory:    pkgerrors.CategoryInvalidCard,
		},
		{
			name:            "system error 96",
			code:            "96",
			wantIsApproved:  false,
			wantIsDeclined:  true,
			wantIsRetriable: true,
			wantCategory:    pkgerrors.CategorySystemError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetACHResponseCode(tt.code)

			assert.Equal(t, tt.code, info.Code)
			assert.Equal(t, tt.wantIsApproved, info.IsApproved)
			assert.Equal(t, tt.wantIsDeclined, info.IsDeclined)
			assert.Equal(t, tt.wantIsRetriable, info.IsRetriable)
			assert.Equal(t, tt.wantCategory, info.Category)
			assert.NotEmpty(t, info.UserMessage)
		})
	}
}

func TestResponseCodeInfo_ToPaymentError(t *testing.T) {
	info := ResponseCodeInfo{
		Code:        "51",
		Display:     "INSUFF FUNDS",
		Description: "Insufficient funds",
		IsRetriable: true,
		Category:    pkgerrors.CategoryInsufficientFunds,
		UserMessage: "Insufficient funds. Please use a different payment method.",
	}

	gatewayMsg := "DECLINED - Insufficient Funds Available"
	err := info.ToPaymentError(gatewayMsg)

	assert.Equal(t, "51", err.Code)
	assert.Equal(t, info.UserMessage, err.Message)
	assert.Equal(t, gatewayMsg, err.GatewayMessage)
	assert.True(t, err.IsRetriable)
	assert.Equal(t, pkgerrors.CategoryInsufficientFunds, err.Category)
	assert.NotNil(t, err.Details)
	assert.Equal(t, "INSUFF FUNDS", err.Details["display"])
	assert.Equal(t, "Insufficient funds", err.Details["description"])
}

func TestCreditCardResponseCode_Coverage(t *testing.T) {
	// Test that all important codes are covered
	importantCodes := []string{"00", "05", "14", "41", "43", "51", "54", "59", "82", "91", "96"}

	for _, code := range importantCodes {
		t.Run("code_"+code, func(t *testing.T) {
			info := GetCreditCardResponseCode(code)
			assert.Equal(t, code, info.Code)
			assert.NotEmpty(t, info.Display, "Display should be set")
			assert.NotEmpty(t, info.UserMessage, "UserMessage should be set")
			assert.NotEmpty(t, info.Category, "Category should be set")
		})
	}
}

func TestACHResponseCode_Coverage(t *testing.T) {
	// Test that all important ACH codes are covered
	importantCodes := []string{"00", "03", "14", "52", "53", "78", "96"}

	for _, code := range importantCodes {
		t.Run("code_"+code, func(t *testing.T) {
			info := GetACHResponseCode(code)
			assert.Equal(t, code, info.Code)
			assert.NotEmpty(t, info.Display)
			assert.NotEmpty(t, info.UserMessage)
			assert.NotEmpty(t, info.Category)
		})
	}
}

func TestRetryLogic(t *testing.T) {
	// Test that retry logic is correctly set for different scenarios
	t.Run("system errors should be retriable", func(t *testing.T) {
		systemErrorCodes := []string{"91", "96"}
		for _, code := range systemErrorCodes {
			info := GetCreditCardResponseCode(code)
			assert.True(t, info.IsRetriable, "System error %s should be retriable", code)
			assert.Equal(t, pkgerrors.CategorySystemError, info.Category)
		}
	})

	t.Run("user errors should require action but may be retriable", func(t *testing.T) {
		userErrorCodes := []string{"51", "54", "82"}
		for _, code := range userErrorCodes {
			info := GetCreditCardResponseCode(code)
			assert.True(t, info.RequiresUserAction, "Code %s should require user action", code)
			assert.True(t, info.IsRetriable, "User fixable error %s should be retriable", code)
		}
	})

	t.Run("fraud codes should not be retriable", func(t *testing.T) {
		fraudCodes := []string{"41", "43", "59"}
		for _, code := range fraudCodes {
			info := GetCreditCardResponseCode(code)
			assert.False(t, info.IsRetriable, "Fraud code %s should not be retriable", code)
			assert.Equal(t, pkgerrors.CategoryFraud, info.Category)
		}
	})
}
