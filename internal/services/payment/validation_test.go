package payment

import (
	"testing"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

// TestTransactionAmountEdgeCases tests edge cases in amount calculations
func TestTransactionAmountEdgeCases(t *testing.T) {
	tests := []struct {
		name                string
		transactions        []*domain.Transaction
		expectedAuthCents   int64
		expectedCapCents    int64
		expectedRefundCents int64
	}{
		{
			name: "zero amounts",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, 0, "bric1"),
			},
			expectedAuthCents:   0,
			expectedCapCents:    0,
			expectedRefundCents: 0,
		},
		{
			name: "very small amounts",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, 1, "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, 1, "bric2"),
			},
			expectedAuthCents:   1,
			expectedCapCents:    1,
			expectedRefundCents: 0,
		},
		{
			name: "large amounts",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, 99999999, "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, 99999999, "bric2"),
			},
			expectedAuthCents:   99999999,
			expectedCapCents:    99999999,
			expectedRefundCents: 0,
		},
		{
			name: "multiple captures with rounding",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, 3333, "bric2"),
				makeTransaction("cap2", domain.TransactionTypeCapture, 3333, "bric3"),
				makeTransaction("cap3", domain.TransactionTypeCapture, 3334, "bric4"),
			},
			expectedAuthCents:   10000,
			expectedCapCents:    10000, // 3333 + 3333 + 3334
			expectedRefundCents: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ComputeGroupState(tt.transactions)

			assert.Equal(t, tt.expectedAuthCents, state.ActiveAuthAmount)
			assert.Equal(t, tt.expectedCapCents, state.CapturedAmount)
			assert.Equal(t, tt.expectedRefundCents, state.RefundedAmount)
		})
	}
}

// TestCaptureValidation_TableDriven tests CAPTURE validation with comprehensive scenarios
func TestCaptureValidation_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		authAmountCents     int64
		capturedSoFarCents  int64
		captureAmountCents  int64
		isVoided            bool
		expectAllow         bool
		expectReason        string
	}{
		// Valid scenarios
		{
			name:               "full capture of AUTH",
			authAmountCents:    10000,
			capturedSoFarCents: 0,
			captureAmountCents: 10000,
			isVoided:           false,
			expectAllow:        true,
		},
		{
			name:               "first partial capture",
			authAmountCents:    10000,
			capturedSoFarCents: 0,
			captureAmountCents: 6000,
			isVoided:           false,
			expectAllow:        true,
		},
		{
			name:               "second partial capture",
			authAmountCents:    10000,
			capturedSoFarCents: 6000,
			captureAmountCents: 4000,
			isVoided:           false,
			expectAllow:        true,
		},
		{
			name:               "capture exact remaining",
			authAmountCents:    10000,
			capturedSoFarCents: 9999,
			captureAmountCents: 1,
			isVoided:           false,
			expectAllow:        true,
		},

		// Invalid scenarios
		{
			name:               "exceed auth by 1 cent",
			authAmountCents:    10000,
			capturedSoFarCents: 0,
			captureAmountCents: 10001,
			isVoided:           false,
			expectAllow:        false,
			expectReason:       "exceeds remaining authorized amount",
		},
		{
			name:               "exceed remaining after partial",
			authAmountCents:    10000,
			capturedSoFarCents: 6000,
			captureAmountCents: 4001,
			isVoided:           false,
			expectAllow:        false,
			expectReason:       "exceeds remaining authorized amount",
		},
		{
			name:               "capture after full capture",
			authAmountCents:    10000,
			capturedSoFarCents: 10000,
			captureAmountCents: 1,
			isVoided:           false,
			expectAllow:        false,
			expectReason:       "exceeds remaining authorized amount",
		},
		{
			name:               "capture voided AUTH",
			authAmountCents:    10000,
			capturedSoFarCents: 0,
			captureAmountCents: 5000,
			isVoided:           true,
			expectAllow:        false,
			expectReason:       "authorization was voided",
		},
		{
			name:               "large amount exceed",
			authAmountCents:    99999999,
			capturedSoFarCents: 0,
			captureAmountCents: 100000000,
			isVoided:           false,
			expectAllow:        false,
			expectReason:       "exceeds remaining authorized amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authID := "auth1"
			state := &GroupState{
				ActiveAuthID:     &authID,
				ActiveAuthAmount: tt.authAmountCents,
				CapturedAmount:   tt.capturedSoFarCents,
				IsAuthVoided:     tt.isVoided,
			}

			canCapture, reason := state.CanCapture(tt.captureAmountCents)

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
		name               string
		capturedCents      int64
		refundedSoFarCents int64
		refundAmountCents  int64
		expectAllow        bool
		expectReason       string
	}{
		// Valid scenarios
		{
			name:               "full refund",
			capturedCents:      10000,
			refundedSoFarCents: 0,
			refundAmountCents:  10000,
			expectAllow:        true,
		},
		{
			name:               "first partial refund",
			capturedCents:      10000,
			refundedSoFarCents: 0,
			refundAmountCents:  6000,
			expectAllow:        true,
		},
		{
			name:               "second partial refund",
			capturedCents:      10000,
			refundedSoFarCents: 6000,
			refundAmountCents:  4000,
			expectAllow:        true,
		},
		{
			name:               "refund exact remaining",
			capturedCents:      10000,
			refundedSoFarCents: 9999,
			refundAmountCents:  1,
			expectAllow:        true,
		},
		{
			name:               "multiple small refunds",
			capturedCents:      10000,
			refundedSoFarCents: 7500,
			refundAmountCents:  2500,
			expectAllow:        true,
		},

		// Invalid scenarios
		{
			name:               "exceed captured by 1 cent",
			capturedCents:      10000,
			refundedSoFarCents: 0,
			refundAmountCents:  10001,
			expectAllow:        false,
			expectReason:       "exceeds remaining refundable amount",
		},
		{
			name:               "exceed remaining after partial",
			capturedCents:      10000,
			refundedSoFarCents: 6000,
			refundAmountCents:  4001,
			expectAllow:        false,
			expectReason:       "exceeds remaining refundable amount",
		},
		{
			name:               "refund after full refund",
			capturedCents:      10000,
			refundedSoFarCents: 10000,
			refundAmountCents:  1,
			expectAllow:        false,
			expectReason:       "exceeds remaining refundable amount",
		},
		{
			name:               "refund without capture",
			capturedCents:      0,
			refundedSoFarCents: 0,
			refundAmountCents:  5000,
			expectAllow:        false,
			expectReason:       "no captured amount to refund",
		},
		{
			name:               "large amount exceed",
			capturedCents:      99999999,
			refundedSoFarCents: 0,
			refundAmountCents:  100000000,
			expectAllow:        false,
			expectReason:       "exceeds remaining refundable amount",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &GroupState{
				CapturedAmount: tt.capturedCents,
				RefundedAmount: tt.refundedSoFarCents,
			}

			canRefund, reason := state.CanRefund(tt.refundAmountCents)

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
				makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, 10000, "bric2"),
				makeTransaction("ref1", domain.TransactionTypeRefund, 3000, "bric3"),
				makeTransaction("ref2", domain.TransactionTypeRefund, 2000, "bric4"),
			},
			validate: func(t *testing.T, state *GroupState) {
				assert.Equal(t, int64(10000), state.CapturedAmount)
				assert.Equal(t, int64(5000), state.RefundedAmount)

				// Can refund remaining $50 (5000 cents)
				canRefund, _ := state.CanRefund(5000)
				assert.True(t, canRefund)

				// Cannot refund $50.01 (5001 cents)
				canRefund, _ = state.CanRefund(5001)
				assert.False(t, canRefund)
			},
		},
		{
			name: "AUTH → PARTIAL CAPTURE → VOID AUTH",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, 6000, "bric2"),
				makeVoidTransactionWithType("void1", 10000, "auth"),
			},
			validate: func(t *testing.T, state *GroupState) {
				assert.True(t, state.IsAuthVoided)
				assert.Nil(t, state.ActiveAuthID)

				// Cannot capture after void
				canCapture, _ := state.CanCapture(4000)
				assert.False(t, canCapture)
			},
		},
		{
			name: "SALE → REFUND (simpler than AUTH+CAPTURE)",
			transactions: []*domain.Transaction{
				makeTransaction("sale1", domain.TransactionTypeSale, 10000, "bric1"),
				makeTransaction("ref1", domain.TransactionTypeRefund, 10000, "bric2"),
			},
			validate: func(t *testing.T, state *GroupState) {
				assert.Equal(t, int64(10000), state.CapturedAmount)
				assert.Equal(t, int64(10000), state.RefundedAmount)

				// Cannot refund more
				canRefund, reason := state.CanRefund(1)
				assert.False(t, canRefund)
				assert.Contains(t, reason, "exceeds remaining refundable amount")
			},
		},
		{
			name: "RE-AUTH scenario",
			transactions: []*domain.Transaction{
				makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric1"),
				makeTransaction("cap1", domain.TransactionTypeCapture, 5000, "bric2"),
				makeTransaction("auth2", domain.TransactionTypeAuth, 15000, "bric3"), // Customer increased order
			},
			validate: func(t *testing.T, state *GroupState) {
				// New AUTH resets state
				assert.Equal(t, int64(15000), state.ActiveAuthAmount)
				assert.Equal(t, int64(0), state.CapturedAmount)
				assert.Equal(t, int64(0), state.RefundedAmount)

				// Can capture full $150 (15000 cents)
				canCapture, _ := state.CanCapture(15000)
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
func makeVoidTransactionWithType(id string, amountCents int64, origType string) *domain.Transaction {
	tx := makeTransaction(id, domain.TransactionTypeVoid, amountCents, "bric_void")
	tx.Metadata["original_transaction_type"] = origType
	return tx
}
