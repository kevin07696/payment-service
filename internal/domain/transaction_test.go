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
