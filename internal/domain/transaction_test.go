package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function to create a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// TestTransaction_IsApproved tests the IsApproved method with various scenarios
func TestTransaction_IsApproved(t *testing.T) {
	tests := []struct {
		name     string
		authResp *string
		expected bool
	}{
		{
			name:     "approved_with_00_response_code",
			authResp: stringPtr("00"),
			expected: true,
		},
		{
			name:     "declined_with_05_response_code",
			authResp: stringPtr("05"),
			expected: false,
		},
		{
			name:     "declined_with_other_response_code",
			authResp: stringPtr("51"),
			expected: false,
		},
		{
			name:     "nil_auth_resp_not_approved",
			authResp: nil,
			expected: false,
		},
		{
			name:     "empty_string_auth_resp_not_approved",
			authResp: stringPtr(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				AuthResp: tt.authResp,
			}
			assert.Equal(t, tt.expected, tx.IsApproved(),
				"IsApproved() should return %v for auth_resp=%v", tt.expected, tt.authResp)
		})
	}
}

// TestTransaction_CanBeVoided tests the CanBeVoided method with various transaction types and statuses
func TestTransaction_CanBeVoided(t *testing.T) {
	tests := []struct {
		name     string
		txType   TransactionType
		status   TransactionStatus
		expected bool
	}{
		{
			name:     "approved_auth_can_be_voided",
			txType:   TransactionTypeAuth,
			status:   TransactionStatusApproved,
			expected: true,
		},
		{
			name:     "approved_sale_can_be_voided",
			txType:   TransactionTypeSale,
			status:   TransactionStatusApproved,
			expected: true,
		},
		{
			name:     "declined_auth_cannot_be_voided",
			txType:   TransactionTypeAuth,
			status:   TransactionStatusDeclined,
			expected: false,
		},
		{
			name:     "declined_sale_cannot_be_voided",
			txType:   TransactionTypeSale,
			status:   TransactionStatusDeclined,
			expected: false,
		},
		{
			name:     "approved_capture_cannot_be_voided",
			txType:   TransactionTypeCapture,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_refund_cannot_be_voided",
			txType:   TransactionTypeRefund,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_void_cannot_be_voided",
			txType:   TransactionTypeVoid,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_prenote_cannot_be_voided",
			txType:   TransactionTypePreNote,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_storage_cannot_be_voided",
			txType:   TransactionTypeStorage,
			status:   TransactionStatusApproved,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				Type:   tt.txType,
				Status: tt.status,
			}
			assert.Equal(t, tt.expected, tx.CanBeVoided(),
				"CanBeVoided() should return %v for type=%s status=%s", tt.expected, tt.txType, tt.status)
		})
	}
}

// TestTransaction_CanBeCaptured tests the CanBeCaptured method with various transaction types and statuses
func TestTransaction_CanBeCaptured(t *testing.T) {
	tests := []struct {
		name     string
		txType   TransactionType
		status   TransactionStatus
		expected bool
	}{
		{
			name:     "approved_auth_can_be_captured",
			txType:   TransactionTypeAuth,
			status:   TransactionStatusApproved,
			expected: true,
		},
		{
			name:     "declined_auth_cannot_be_captured",
			txType:   TransactionTypeAuth,
			status:   TransactionStatusDeclined,
			expected: false,
		},
		{
			name:     "approved_sale_cannot_be_captured",
			txType:   TransactionTypeSale,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_capture_cannot_be_captured_again",
			txType:   TransactionTypeCapture,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_refund_cannot_be_captured",
			txType:   TransactionTypeRefund,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_void_cannot_be_captured",
			txType:   TransactionTypeVoid,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_prenote_cannot_be_captured",
			txType:   TransactionTypePreNote,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_storage_cannot_be_captured",
			txType:   TransactionTypeStorage,
			status:   TransactionStatusApproved,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				Type:   tt.txType,
				Status: tt.status,
			}
			assert.Equal(t, tt.expected, tx.CanBeCaptured(),
				"CanBeCaptured() should return %v for type=%s status=%s", tt.expected, tt.txType, tt.status)
		})
	}
}

// TestTransaction_CanBeRefunded tests the CanBeRefunded method with various transaction types and statuses
func TestTransaction_CanBeRefunded(t *testing.T) {
	tests := []struct {
		name     string
		txType   TransactionType
		status   TransactionStatus
		expected bool
	}{
		{
			name:     "approved_sale_can_be_refunded",
			txType:   TransactionTypeSale,
			status:   TransactionStatusApproved,
			expected: true,
		},
		{
			name:     "approved_capture_can_be_refunded",
			txType:   TransactionTypeCapture,
			status:   TransactionStatusApproved,
			expected: true,
		},
		{
			name:     "declined_sale_cannot_be_refunded",
			txType:   TransactionTypeSale,
			status:   TransactionStatusDeclined,
			expected: false,
		},
		{
			name:     "declined_capture_cannot_be_refunded",
			txType:   TransactionTypeCapture,
			status:   TransactionStatusDeclined,
			expected: false,
		},
		{
			name:     "approved_auth_cannot_be_refunded",
			txType:   TransactionTypeAuth,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_refund_cannot_be_refunded_again",
			txType:   TransactionTypeRefund,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_void_cannot_be_refunded",
			txType:   TransactionTypeVoid,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_prenote_cannot_be_refunded",
			txType:   TransactionTypePreNote,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_storage_cannot_be_refunded",
			txType:   TransactionTypeStorage,
			status:   TransactionStatusApproved,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				Type:   tt.txType,
				Status: tt.status,
			}
			assert.Equal(t, tt.expected, tx.CanBeRefunded(),
				"CanBeRefunded() should return %v for type=%s status=%s", tt.expected, tt.txType, tt.status)
		})
	}
}

// TestTransaction_GetCustomerID tests the GetCustomerID method with various scenarios
func TestTransaction_GetCustomerID(t *testing.T) {
	tests := []struct {
		name       string
		customerID *string
		expected   string
	}{
		{
			name:       "returns_customer_id_when_present",
			customerID: stringPtr("cust_123456"),
			expected:   "cust_123456",
		},
		{
			name:       "returns_empty_string_when_nil",
			customerID: nil,
			expected:   "",
		},
		{
			name:       "returns_empty_string_when_customer_id_is_empty",
			customerID: stringPtr(""),
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				CustomerID: tt.customerID,
			}
			assert.Equal(t, tt.expected, tx.GetCustomerID(),
				"GetCustomerID() should return %q for customerID=%v", tt.expected, tt.customerID)
		})
	}
}

// TestTransaction_BusinessLogic_IntegrationScenarios tests realistic business scenarios
// combining multiple methods to ensure the business logic is coherent
func TestTransaction_BusinessLogic_IntegrationScenarios(t *testing.T) {
	t.Run("approved_auth_workflow", func(t *testing.T) {
		// An approved AUTH transaction should be captured or voided, but not refunded
		tx := &Transaction{
			Type:     TransactionTypeAuth,
			Status:   TransactionStatusApproved,
			AuthResp: stringPtr("00"),
		}

		assert.True(t, tx.IsApproved(), "approved AUTH should be approved")
		assert.True(t, tx.CanBeCaptured(), "approved AUTH should be capturable")
		assert.True(t, tx.CanBeVoided(), "approved AUTH should be voidable")
		assert.False(t, tx.CanBeRefunded(), "approved AUTH cannot be refunded")
	})

	t.Run("declined_auth_workflow", func(t *testing.T) {
		// A declined AUTH transaction should not allow any follow-up actions
		tx := &Transaction{
			Type:     TransactionTypeAuth,
			Status:   TransactionStatusDeclined,
			AuthResp: stringPtr("05"),
		}

		assert.False(t, tx.IsApproved(), "declined AUTH should not be approved")
		assert.False(t, tx.CanBeCaptured(), "declined AUTH cannot be captured")
		assert.False(t, tx.CanBeVoided(), "declined AUTH cannot be voided")
		assert.False(t, tx.CanBeRefunded(), "declined AUTH cannot be refunded")
	})

	t.Run("approved_sale_workflow", func(t *testing.T) {
		// An approved SALE transaction should be refunded or voided, but not captured
		tx := &Transaction{
			Type:     TransactionTypeSale,
			Status:   TransactionStatusApproved,
			AuthResp: stringPtr("00"),
		}

		assert.True(t, tx.IsApproved(), "approved SALE should be approved")
		assert.False(t, tx.CanBeCaptured(), "approved SALE cannot be captured (already captured)")
		assert.True(t, tx.CanBeVoided(), "approved SALE should be voidable")
		assert.True(t, tx.CanBeRefunded(), "approved SALE should be refundable")
	})

	t.Run("approved_capture_workflow", func(t *testing.T) {
		// An approved CAPTURE transaction should only be refunded
		tx := &Transaction{
			Type:     TransactionTypeCapture,
			Status:   TransactionStatusApproved,
			AuthResp: stringPtr("00"),
		}

		assert.True(t, tx.IsApproved(), "approved CAPTURE should be approved")
		assert.False(t, tx.CanBeCaptured(), "approved CAPTURE cannot be captured again")
		assert.False(t, tx.CanBeVoided(), "approved CAPTURE cannot be voided (use refund)")
		assert.True(t, tx.CanBeRefunded(), "approved CAPTURE should be refundable")
	})

	t.Run("guest_transaction_no_customer", func(t *testing.T) {
		// Guest transactions should have no customer ID
		tx := &Transaction{
			Type:       TransactionTypeSale,
			Status:     TransactionStatusApproved,
			CustomerID: nil,
		}

		assert.Equal(t, "", tx.GetCustomerID(), "guest transaction should return empty customer ID")
	})

	t.Run("customer_transaction_with_id", func(t *testing.T) {
		// Customer transactions should return the customer ID
		customerID := "cust_abc123"
		tx := &Transaction{
			Type:       TransactionTypeSale,
			Status:     TransactionStatusApproved,
			CustomerID: &customerID,
		}

		assert.Equal(t, customerID, tx.GetCustomerID(), "customer transaction should return customer ID")
	})
}

// TestParseRequestTransactionType tests parsing of transaction type strings
func TestParseRequestTransactionType(t *testing.T) {
	tests := []struct {
		input    string
		expected RequestTransactionType
	}{
		{"SALE", RequestTransactionTypeSale},
		{"sale", RequestTransactionTypeSale},
		{"Sale", RequestTransactionTypeSale},
		{"AUTH", RequestTransactionTypeAuth},
		{"auth", RequestTransactionTypeAuth},
		{"STORAGE", RequestTransactionTypeStorage},
		{"storage", RequestTransactionTypeStorage},
		{"ACH_STORAGE_C", RequestTransactionTypeACHStorageC},
		{"ach_storage_c", RequestTransactionTypeACHStorageC},
		{"ACH_STORAGE_S", RequestTransactionTypeACHStorageS},
		{"ach_storage_s", RequestTransactionTypeACHStorageS},
		{"", RequestTransactionTypeSale},        // default
		{"INVALID", RequestTransactionTypeSale}, // unknown defaults to SALE
		{"REFUND", RequestTransactionTypeSale},  // not a request type
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseRequestTransactionType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRequestTransactionType_ToTransactionType tests conversion to internal transaction type
func TestRequestTransactionType_ToTransactionType(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected TransactionType
	}{
		{RequestTransactionTypeSale, TransactionTypeSale},
		{RequestTransactionTypeAuth, TransactionTypeAuth},
		{RequestTransactionTypeStorage, TransactionTypeStorage},
		{RequestTransactionTypeACHStorageC, TransactionTypeStorage},
		{RequestTransactionTypeACHStorageS, TransactionTypeStorage},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := tt.input.ToTransactionType()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRequestTransactionType_IsACHStorage tests ACH storage detection
func TestRequestTransactionType_IsACHStorage(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected bool
	}{
		{RequestTransactionTypeACHStorageC, true},
		{RequestTransactionTypeACHStorageS, true},
		{RequestTransactionTypeStorage, false},
		{RequestTransactionTypeSale, false},
		{RequestTransactionTypeAuth, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input.IsACHStorage())
		})
	}
}

// TestRequestTransactionType_IsCheckingAccount tests checking account detection
func TestRequestTransactionType_IsCheckingAccount(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected bool
	}{
		{RequestTransactionTypeACHStorageC, true},
		{RequestTransactionTypeACHStorageS, false},
		{RequestTransactionTypeStorage, false},
		{RequestTransactionTypeSale, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input.IsCheckingAccount())
		})
	}
}

// TestRequestTransactionType_IsStorage tests storage transaction detection
func TestRequestTransactionType_IsStorage(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected bool
	}{
		{RequestTransactionTypeStorage, true},
		{RequestTransactionTypeACHStorageC, true},
		{RequestTransactionTypeACHStorageS, true},
		{RequestTransactionTypeSale, false},
		{RequestTransactionTypeAuth, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input.IsStorage())
		})
	}
}

// TestRequestTransactionType_ToPaymentMethodType tests payment method type derivation
func TestRequestTransactionType_ToPaymentMethodType(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected PaymentMethodType
	}{
		{RequestTransactionTypeACHStorageC, PaymentMethodTypeACH},
		{RequestTransactionTypeACHStorageS, PaymentMethodTypeACH},
		{RequestTransactionTypeStorage, PaymentMethodTypeCreditCard},
		{RequestTransactionTypeSale, PaymentMethodTypeCreditCard},
		{RequestTransactionTypeAuth, PaymentMethodTypeCreditCard},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := tt.input.ToPaymentMethodType()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRequestTransactionType_IsValid tests validation of transaction types
func TestRequestTransactionType_IsValid(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected bool
	}{
		{RequestTransactionTypeSale, true},
		{RequestTransactionTypeAuth, true},
		{RequestTransactionTypeStorage, true},
		{RequestTransactionTypeACHStorageC, true},
		{RequestTransactionTypeACHStorageS, true},
		{RequestTransactionType("INVALID"), false},
		{RequestTransactionType(""), false},
		{RequestTransactionType("REFUND"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.input.IsValid())
		})
	}
}

// TestRequestTransactionType_ToEPXTranCode tests EPX TRAN_CODE generation
func TestRequestTransactionType_ToEPXTranCode(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected string
	}{
		{RequestTransactionTypeSale, "SALE"},
		{RequestTransactionTypeAuth, "AUTH"},
		{RequestTransactionTypeStorage, "STORAGE"},
		{RequestTransactionTypeACHStorageC, "ACHSTORAGE_C"},
		{RequestTransactionTypeACHStorageS, "ACHSTORAGE_S"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := tt.input.ToEPXTranCode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRequestTransactionType_ToEPXTranGroup tests EPX TRAN_GROUP generation
func TestRequestTransactionType_ToEPXTranGroup(t *testing.T) {
	tests := []struct {
		input    RequestTransactionType
		expected string
	}{
		{RequestTransactionTypeSale, "SALE"},
		{RequestTransactionTypeAuth, "AUTH"},
		{RequestTransactionTypeStorage, "STORAGE"},
		{RequestTransactionTypeACHStorageC, "STORAGE"},
		{RequestTransactionTypeACHStorageS, "STORAGE"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := tt.input.ToEPXTranGroup()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTransaction_CanBeReversed tests the CanBeReversed method with various transaction types and statuses
// Reversal (CCE7) releases the authorization hold AND voids the transaction in one call
// Unlike Void which only stops the transaction, Reversal immediately releases funds to cardholder
func TestTransaction_CanBeReversed(t *testing.T) {
	tests := []struct {
		name     string
		txType   TransactionType
		status   TransactionStatus
		expected bool
	}{
		// Approved AUTH and SALE can be reversed
		{
			name:     "approved_auth_can_be_reversed",
			txType:   TransactionTypeAuth,
			status:   TransactionStatusApproved,
			expected: true,
		},
		{
			name:     "approved_sale_can_be_reversed",
			txType:   TransactionTypeSale,
			status:   TransactionStatusApproved,
			expected: true,
		},
		// Declined transactions cannot be reversed
		{
			name:     "declined_auth_cannot_be_reversed",
			txType:   TransactionTypeAuth,
			status:   TransactionStatusDeclined,
			expected: false,
		},
		{
			name:     "declined_sale_cannot_be_reversed",
			txType:   TransactionTypeSale,
			status:   TransactionStatusDeclined,
			expected: false,
		},
		// Other transaction types cannot be reversed
		{
			name:     "approved_capture_cannot_be_reversed",
			txType:   TransactionTypeCapture,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_refund_cannot_be_reversed",
			txType:   TransactionTypeRefund,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_void_cannot_be_reversed",
			txType:   TransactionTypeVoid,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_reversal_cannot_be_reversed_again",
			txType:   TransactionTypeReversal,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_prenote_cannot_be_reversed",
			txType:   TransactionTypePreNote,
			status:   TransactionStatusApproved,
			expected: false,
		},
		{
			name:     "approved_storage_cannot_be_reversed",
			txType:   TransactionTypeStorage,
			status:   TransactionStatusApproved,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Transaction{
				Type:   tt.txType,
				Status: tt.status,
			}
			assert.Equal(t, tt.expected, tx.CanBeReversed(),
				"CanBeReversed() should return %v for type=%s status=%s", tt.expected, tt.txType, tt.status)
		})
	}
}

// TestTransaction_ReversalWorkflow tests the business workflow for reversal transactions
func TestTransaction_ReversalWorkflow(t *testing.T) {
	t.Run("approved_auth_reversal_workflow", func(t *testing.T) {
		// An approved AUTH transaction should be reversible (CCE7 releases hold + voids)
		tx := &Transaction{
			Type:     TransactionTypeAuth,
			Status:   TransactionStatusApproved,
			AuthResp: stringPtr("00"),
		}

		assert.True(t, tx.IsApproved(), "approved AUTH should be approved")
		assert.True(t, tx.CanBeReversed(), "approved AUTH should be reversible (CCE7)")
		assert.True(t, tx.CanBeVoided(), "approved AUTH should also be voidable (CCEX fallback)")
		assert.True(t, tx.CanBeCaptured(), "approved AUTH should be capturable")
		assert.False(t, tx.CanBeRefunded(), "approved AUTH cannot be refunded")
	})

	t.Run("approved_sale_reversal_workflow", func(t *testing.T) {
		// An approved SALE transaction should be reversible
		tx := &Transaction{
			Type:     TransactionTypeSale,
			Status:   TransactionStatusApproved,
			AuthResp: stringPtr("00"),
		}

		assert.True(t, tx.IsApproved(), "approved SALE should be approved")
		assert.True(t, tx.CanBeReversed(), "approved SALE should be reversible (CCE7)")
		assert.True(t, tx.CanBeVoided(), "approved SALE should also be voidable (CCEX fallback)")
		assert.True(t, tx.CanBeRefunded(), "approved SALE should be refundable")
	})

	t.Run("reversal_vs_void_comparison", func(t *testing.T) {
		// Both auth and sale can be voided or reversed
		// Reversal (CCE7) is preferred because it releases funds immediately
		// Void (CCEX) is fallback if issuer declines reversal

		authTx := &Transaction{Type: TransactionTypeAuth, Status: TransactionStatusApproved}
		saleTx := &Transaction{Type: TransactionTypeSale, Status: TransactionStatusApproved}

		// Both can be reversed
		assert.True(t, authTx.CanBeReversed())
		assert.True(t, saleTx.CanBeReversed())

		// Both can also be voided (as fallback)
		assert.True(t, authTx.CanBeVoided())
		assert.True(t, saleTx.CanBeVoided())
	})
}

// TestTransaction_EdgeCases_NilPointerSafety tests nil pointer safety across all methods
func TestTransaction_EdgeCases_NilPointerSafety(t *testing.T) {
	t.Run("nil_transaction_fields_dont_panic", func(t *testing.T) {
		// Create a transaction with all nullable fields as nil
		tx := &Transaction{
			Type:   TransactionTypeAuth,
			Status: TransactionStatusApproved,
			// All pointer fields are nil
			AuthResp:   nil,
			CustomerID: nil,
		}

		// These methods should not panic even with nil pointers
		assert.NotPanics(t, func() {
			_ = tx.IsApproved()
		}, "IsApproved should not panic with nil AuthResp")

		assert.NotPanics(t, func() {
			_ = tx.GetCustomerID()
		}, "GetCustomerID should not panic with nil CustomerID")

		assert.NotPanics(t, func() {
			_ = tx.CanBeVoided()
		}, "CanBeVoided should not panic")

		assert.NotPanics(t, func() {
			_ = tx.CanBeCaptured()
		}, "CanBeCaptured should not panic")

		assert.NotPanics(t, func() {
			_ = tx.CanBeRefunded()
		}, "CanBeRefunded should not panic")
	})
}
