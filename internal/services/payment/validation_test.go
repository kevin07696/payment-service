package payment

import (
	"testing"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestTransactionAmountEdgeCases tests edge cases in amount calculations
func TestTransactionAmountEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		transactions   []*domain.Transaction
		expectedAuth   string
		expectedCap    string
		expectedRefund string
	}{
		{
			name: "zero amounts",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, "0", "bric1"),
			},
			expectedAuth:   "0.00",
			expectedCap:    "0.00",
			expectedRefund: "0.00",
		},
		{
			name: "very small amounts",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, "0.01", "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, "0.01", "bric2"),
			},
			expectedAuth:   "0.01",
			expectedCap:    "0.01",
			expectedRefund: "0.00",
		},
		{
			name: "large amounts",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, "999999.99", "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, "999999.99", "bric2"),
			},
			expectedAuth:   "999999.99",
			expectedCap:    "999999.99",
			expectedRefund: "0.00",
		},
		{
			name: "multiple captures with rounding",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, "33.33", "bric2"),
				makeTransaction("cap2", domain.TransactionTypeCapture, "33.33", "bric3"),
				makeTransaction("cap3", domain.TransactionTypeCapture, "33.34", "bric4"),
			},
			expectedAuth:   "100.00",
			expectedCap:    "100.00", // 33.33 + 33.33 + 33.34
			expectedRefund: "0.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ComputeGroupState(tt.transactions)

			assert.Equal(t, tt.expectedAuth, state.ActiveAuthAmount.StringFixed(2))
			assert.Equal(t, tt.expectedCap, state.CapturedAmount.StringFixed(2))
			assert.Equal(t, tt.expectedRefund, state.RefundedAmount.StringFixed(2))
		})
	}
}

// TestCaptureValidation_TableDriven tests CAPTURE validation with comprehensive scenarios
func TestCaptureValidation_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		authAmount    string
		capturedSoFar string
		captureAmount string
		isVoided      bool
		expectAllow   bool
		expectReason  string
	}{
		// Valid scenarios
		{
			name:          "full capture of AUTH",
			authAmount:    "100.00",
			capturedSoFar: "0",
			captureAmount: "100.00",
			isVoided:      false,
			expectAllow:   true,
		},
		{
			name:          "first partial capture",
			authAmount:    "100.00",
			capturedSoFar: "0",
			captureAmount: "60.00",
			isVoided:      false,
			expectAllow:   true,
		},
		{
			name:          "second partial capture",
			authAmount:    "100.00",
			capturedSoFar: "60.00",
			captureAmount: "40.00",
			isVoided:      false,
			expectAllow:   true,
		},
		{
			name:          "capture exact remaining",
			authAmount:    "100.00",
			capturedSoFar: "99.99",
			captureAmount: "0.01",
			isVoided:      false,
			expectAllow:   true,
		},

		// Invalid scenarios
		{
			name:          "exceed auth by 1 cent",
			authAmount:    "100.00",
			capturedSoFar: "0",
			captureAmount: "100.01",
			isVoided:      false,
			expectAllow:   false,
			expectReason:  "exceeds remaining authorized amount",
		},
		{
			name:          "exceed remaining after partial",
			authAmount:    "100.00",
			capturedSoFar: "60.00",
			captureAmount: "40.01",
			isVoided:      false,
			expectAllow:   false,
			expectReason:  "exceeds remaining authorized amount",
		},
		{
			name:          "capture after full capture",
			authAmount:    "100.00",
			capturedSoFar: "100.00",
			captureAmount: "0.01",
			isVoided:      false,
			expectAllow:   false,
			expectReason:  "exceeds remaining authorized amount",
		},
		{
			name:          "capture voided AUTH",
			authAmount:    "100.00",
			capturedSoFar: "0",
			captureAmount: "50.00",
			isVoided:      true,
			expectAllow:   false,
			expectReason:  "authorization was voided",
		},
		{
			name:          "large amount exceed",
			authAmount:    "999999.99",
			capturedSoFar: "0",
			captureAmount: "1000000.00",
			isVoided:      false,
			expectAllow:   false,
			expectReason:  "exceeds remaining authorized amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authAmt, _ := decimal.NewFromString(tt.authAmount)
			capturedAmt, _ := decimal.NewFromString(tt.capturedSoFar)
			captureAmt, _ := decimal.NewFromString(tt.captureAmount)

			authID := "auth1"
			state := &GroupState{
				ActiveAuthID:     &authID,
				ActiveAuthAmount: authAmt,
				CapturedAmount:   capturedAmt,
				IsAuthVoided:     tt.isVoided,
			}

			canCapture, reason := state.CanCapture(captureAmt)

			assert.Equal(t, tt.expectAllow, canCapture,
				"Expected allow=%v, got allow=%v, reason=%s", tt.expectAllow, canCapture, reason)

			if !tt.expectAllow {
				assert.Contains(t, reason, tt.expectReason)
			}
		})
	}
}

// TestRefundValidation_TableDriven tests REFUND validation with comprehensive scenarios
func TestRefundValidation_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		captured      string
		refundedSoFar string
		refundAmount  string
		expectAllow   bool
		expectReason  string
	}{
		// Valid scenarios
		{
			name:          "full refund",
			captured:      "100.00",
			refundedSoFar: "0",
			refundAmount:  "100.00",
			expectAllow:   true,
		},
		{
			name:          "first partial refund",
			captured:      "100.00",
			refundedSoFar: "0",
			refundAmount:  "60.00",
			expectAllow:   true,
		},
		{
			name:          "second partial refund",
			captured:      "100.00",
			refundedSoFar: "60.00",
			refundAmount:  "40.00",
			expectAllow:   true,
		},
		{
			name:          "refund exact remaining",
			captured:      "100.00",
			refundedSoFar: "99.99",
			refundAmount:  "0.01",
			expectAllow:   true,
		},
		{
			name:          "multiple small refunds",
			captured:      "100.00",
			refundedSoFar: "75.00",
			refundAmount:  "25.00",
			expectAllow:   true,
		},

		// Invalid scenarios
		{
			name:          "exceed captured by 1 cent",
			captured:      "100.00",
			refundedSoFar: "0",
			refundAmount:  "100.01",
			expectAllow:   false,
			expectReason:  "exceeds remaining refundable amount",
		},
		{
			name:          "exceed remaining after partial",
			captured:      "100.00",
			refundedSoFar: "60.00",
			refundAmount:  "40.01",
			expectAllow:   false,
			expectReason:  "exceeds remaining refundable amount",
		},
		{
			name:          "refund after full refund",
			captured:      "100.00",
			refundedSoFar: "100.00",
			refundAmount:  "0.01",
			expectAllow:   false,
			expectReason:  "exceeds remaining refundable amount",
		},
		{
			name:          "refund without capture",
			captured:      "0",
			refundedSoFar: "0",
			refundAmount:  "50.00",
			expectAllow:   false,
			expectReason:  "no captured amount to refund",
		},
		{
			name:          "large amount exceed",
			captured:      "999999.99",
			refundedSoFar: "0",
			refundAmount:  "1000000.00",
			expectAllow:   false,
			expectReason:  "exceeds remaining refundable amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedAmt, _ := decimal.NewFromString(tt.captured)
			refundedAmt, _ := decimal.NewFromString(tt.refundedSoFar)
			refundAmt, _ := decimal.NewFromString(tt.refundAmount)

			state := &GroupState{
				CapturedAmount: capturedAmt,
				RefundedAmount: refundedAmt,
			}

			canRefund, reason := state.CanRefund(refundAmt)

			assert.Equal(t, tt.expectAllow, canRefund,
				"Expected allow=%v, got allow=%v, reason=%s", tt.expectAllow, canRefund, reason)

			if !tt.expectAllow {
				assert.Contains(t, reason, tt.expectReason)
			}
		})
	}
}

// TestVoidValidation_TableDriven tests VOID validation with comprehensive scenarios
func TestVoidValidation_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		hasActiveAuth bool
		isVoided      bool
		expectAllow   bool
		expectReason  string
	}{
		{
			name:          "valid void of active AUTH",
			hasActiveAuth: true,
			isVoided:      false,
			expectAllow:   true,
		},
		{
			name:          "cannot void already voided AUTH",
			hasActiveAuth: true,
			isVoided:      true,
			expectAllow:   false,
			expectReason:  "authorization already voided",
		},
		{
			name:          "cannot void without active AUTH",
			hasActiveAuth: false,
			isVoided:      false,
			expectAllow:   false,
			expectReason:  "no active authorization to void",
		},
		{
			name:          "cannot void when both conditions fail",
			hasActiveAuth: false,
			isVoided:      true,
			expectAllow:   false,
			expectReason:  "authorization already voided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &GroupState{
				IsAuthVoided: tt.isVoided,
			}

			if tt.hasActiveAuth {
				authID := "auth1"
				state.ActiveAuthID = &authID
			}

			canVoid, reason := state.CanVoid()

			assert.Equal(t, tt.expectAllow, canVoid,
				"Expected allow=%v, got allow=%v, reason=%s", tt.expectAllow, canVoid, reason)

			if !tt.expectAllow {
				assert.Contains(t, reason, tt.expectReason)
			}
		})
	}
}

// TestComplexTransactionSequences tests complex real-world scenarios
func TestComplexTransactionSequences(t *testing.T) {
	tests := []struct {
		name         string
		transactions []*domain.Transaction
		validate     func(t *testing.T, state *GroupState)
	}{
		{
			name: "AUTH → CAPTURE → REFUND → REFUND",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, "100.00", "bric2"),
				makeTransaction("ref1", domain.TransactionTypeRefund, "30.00", "bric3"),
				makeTransaction("ref2", domain.TransactionTypeRefund, "20.00", "bric4"),
			},
			validate: func(t *testing.T, state *GroupState) {
				assert.Equal(t, "100.00", state.CapturedAmount.StringFixed(2))
				assert.Equal(t, "50.00", state.RefundedAmount.StringFixed(2))

				// Can refund remaining $50
				refundAmt50, _ := decimal.NewFromString("50.00")
				canRefund, _ := state.CanRefund(refundAmt50)
				assert.True(t, canRefund)

				// Cannot refund $50.01
				refundAmt5001, _ := decimal.NewFromString("50.01")
				canRefund, _ = state.CanRefund(refundAmt5001)
				assert.False(t, canRefund)
			},
		},
		{
			name: "AUTH → PARTIAL CAPTURE → VOID AUTH",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, "60.00", "bric2"),
				makeVoidTransactionWithType("void1", "100.00", "auth"),
			},
			validate: func(t *testing.T, state *GroupState) {
				assert.True(t, state.IsAuthVoided)
				assert.Nil(t, state.ActiveAuthID)

				// Cannot capture after void
				captureAmt40, _ := decimal.NewFromString("40.00")
				canCapture, _ := state.CanCapture(captureAmt40)
				assert.False(t, canCapture)
			},
		},
		{
			name: "SALE → REFUND (simpler than AUTH+CAPTURE)",
			transactions: []*domain.Transaction{
				makeTransaction("sale1", domain.TransactionTypeSale, "100.00", "bric1"),
				makeTransaction("ref1", domain.TransactionTypeRefund, "100.00", "bric2"),
			},
			validate: func(t *testing.T, state *GroupState) {
				assert.Equal(t, "100.00", state.CapturedAmount.StringFixed(2))
				assert.Equal(t, "100.00", state.RefundedAmount.StringFixed(2))

				// Cannot refund more
				refundAmt001, _ := decimal.NewFromString("0.01")
				canRefund, reason := state.CanRefund(refundAmt001)
				assert.False(t, canRefund)
				assert.Contains(t, reason, "exceeds remaining refundable amount")
			},
		},
		{
			name: "RE-AUTH scenario",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, "50.00", "bric2"),
				makeTransaction("auth2", domain.TransactionTypeAuth, "150.00", "bric3"), // Customer increased order
			},
			validate: func(t *testing.T, state *GroupState) {
				// New AUTH resets state
				assert.Equal(t, "150.00", state.ActiveAuthAmount.StringFixed(2))
				assert.True(t, state.CapturedAmount.IsZero())
				assert.True(t, state.RefundedAmount.IsZero())

				// Can capture full $150
				captureAmt150, _ := decimal.NewFromString("150.00")
				canCapture, _ := state.CanCapture(captureAmt150)
				assert.True(t, canCapture)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ComputeGroupState(tt.transactions)
			tt.validate(t, state)
		})
	}
}

// Helper function to create VOID with metadata
func makeVoidTransactionWithType(id string, amount string, origType string) *domain.Transaction {
	tx := makeTransaction(id, domain.TransactionTypeVoid, amount, "bric_void")
	tx.Metadata["original_transaction_type"] = origType
	return tx
}
