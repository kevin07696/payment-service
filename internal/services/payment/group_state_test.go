package payment

import (
	"testing"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create approved transaction
func makeTransaction(id string, txType domain.TransactionType, amountCents int64, authGUID string) *domain.Transaction {
	status := domain.TransactionStatusApproved
	return &domain.Transaction{
		ID:          id,
		Type:        txType,
		AmountCents: amountCents,
		Status:      status,
		AuthGUID:    authGUID,
		Metadata:    make(map[string]interface{}),
	}
}

// Helper to create declined transaction
func makeDeclinedTransaction(id string, txType domain.TransactionType, amountCents int64) *domain.Transaction {
	status := domain.TransactionStatusDeclined
	return &domain.Transaction{
		ID:          id,
		Type:        txType,
		AmountCents: amountCents,
		Status:      status,
	}
}

// TestComputeGroupState_EmptyTransactions tests empty transaction list
func TestComputeGroupState_EmptyTransactions(t *testing.T) {
	state := ComputeGroupState([]*domain.Transaction{})

	assert.Nil(t, state.ActiveAuthID)
	assert.Equal(t, int64(0), state.CapturedAmount)
	assert.Equal(t, int64(0), state.RefundedAmount)
	assert.False(t, state.IsAuthVoided)
	assert.Empty(t, state.ActiveAuthBRIC)
}

// TestComputeGroupState_SingleAuth tests single AUTH transaction
func TestComputeGroupState_SingleAuth(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth1", *state.ActiveAuthID)
	assert.Equal(t, int64(10000), state.ActiveAuthAmount)
	assert.Equal(t, "bric_auth1", state.ActiveAuthBRIC)
	assert.Equal(t, int64(0), state.CapturedAmount)
	assert.False(t, state.IsAuthVoided)
}

// TestComputeGroupState_Sale tests SALE transaction (auth+capture in one)
func TestComputeGroupState_Sale(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("sale1", domain.TransactionTypeSale, 10000, "bric_sale1"),
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "sale1", *state.ActiveAuthID)
	assert.Equal(t, int64(10000), state.ActiveAuthAmount)
	assert.Equal(t, int64(10000), state.CapturedAmount) // SALE captures immediately
	assert.Equal(t, "bric_sale1", state.ActiveAuthBRIC)
	assert.False(t, state.IsAuthVoided)
}

// TestComputeGroupState_AuthThenCapture tests AUTH → CAPTURE flow
func TestComputeGroupState_AuthThenCapture(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, 10000, "bric_capture1"),
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth1", *state.ActiveAuthID)
	assert.Equal(t, int64(10000), state.CapturedAmount)
	assert.Equal(t, "bric_capture1", state.CaptureBRIC)
	assert.Equal(t, "bric_capture1", state.CurrentBRIC) // Uses CAPTURE's BRIC
}

// TestComputeGroupState_PartialCapture tests partial CAPTURE
func TestComputeGroupState_PartialCapture(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, 6000, "bric_capture1"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, int64(10000), state.ActiveAuthAmount)
	assert.Equal(t, int64(6000), state.CapturedAmount)
}

// TestComputeGroupState_MultiplePartialCaptures tests multiple partial CAPTUREs
func TestComputeGroupState_MultiplePartialCaptures(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, 3000, "bric_capture1"),
		makeTransaction("capture2", domain.TransactionTypeCapture, 4000, "bric_capture2"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, int64(10000), state.ActiveAuthAmount)
	assert.Equal(t, int64(7000), state.CapturedAmount)  // 30 + 40
	assert.Equal(t, "bric_capture2", state.CaptureBRIC) // Most recent CAPTURE's BRIC
}

// TestComputeGroupState_ReAuth tests re-authorization (new AUTH resets state)
func TestComputeGroupState_ReAuth(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, 5000, "bric_capture1"),
		makeTransaction("auth2", domain.TransactionTypeAuth, 15000, "bric_auth2"), // Re-auth
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth2", *state.ActiveAuthID)
	assert.Equal(t, int64(15000), state.ActiveAuthAmount)
	assert.Equal(t, int64(0), state.CapturedAmount) // Resets on new AUTH
	assert.Equal(t, "bric_auth2", state.ActiveAuthBRIC)
}

// TestComputeGroupState_VoidAuth tests VOID of AUTH
func TestComputeGroupState_VoidAuth(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("void1", domain.TransactionTypeVoid, 10000, "bric_void1"),
	}
	// Add metadata to VOID to indicate what was voided
	txs[1].Metadata["original_transaction_type"] = "auth"

	state := ComputeGroupState(txs)

	assert.Nil(t, state.ActiveAuthID)
	assert.True(t, state.IsAuthVoided)
	assert.Empty(t, state.ActiveAuthBRIC)
}

// TestComputeGroupState_VoidCapture tests VOID of CAPTURE (same-day reversal)
func TestComputeGroupState_VoidCapture(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, 10000, "bric_capture1"),
		makeTransaction("void1", domain.TransactionTypeVoid, 10000, "bric_void1"),
	}
	txs[2].Metadata["original_transaction_type"] = "capture"

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID) // AUTH still active
	assert.Equal(t, "auth1", *state.ActiveAuthID)
	assert.Equal(t, int64(0), state.CapturedAmount) // CAPTURE was voided
	assert.False(t, state.IsAuthVoided)
}

// TestComputeGroupState_Refund tests REFUND
func TestComputeGroupState_Refund(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("sale1", domain.TransactionTypeSale, 10000, "bric_sale1"),
		makeTransaction("refund1", domain.TransactionTypeRefund, 3000, "bric_refund1"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, int64(10000), state.CapturedAmount)
	assert.Equal(t, int64(3000), state.RefundedAmount)
}

// TestComputeGroupState_MultipleRefunds tests multiple partial REFUNDs
func TestComputeGroupState_MultipleRefunds(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("sale1", domain.TransactionTypeSale, 10000, "bric_sale1"),
		makeTransaction("refund1", domain.TransactionTypeRefund, 2500, "bric_refund1"),
		makeTransaction("refund2", domain.TransactionTypeRefund, 2500, "bric_refund2"),
		makeTransaction("refund3", domain.TransactionTypeRefund, 2500, "bric_refund3"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, int64(10000), state.CapturedAmount)
	assert.Equal(t, int64(7500), state.RefundedAmount) // 25 + 25 + 25
}

// TestComputeGroupState_DeclinedTransactionsIgnored tests that declined transactions don't affect state
func TestComputeGroupState_DeclinedTransactionsIgnored(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeDeclinedTransaction("capture1", domain.TransactionTypeCapture, 10000), // Declined
		makeTransaction("capture2", domain.TransactionTypeCapture, 6000, "bric_capture2"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, int64(6000), state.CapturedAmount) // Only approved CAPTURE counts
}

// TestCanCapture_Success tests successful CAPTURE validation
func TestCanCapture_Success(t *testing.T) {
	tests := []struct {
		name                string
		authAmountCents     int64
		capturedAmountCents int64
		captureAmountCents  int64
		shouldAllow         bool
	}{
		{"full capture", 10000, 0, 10000, true},
		{"partial capture", 10000, 0, 6000, true},
		{"remaining capture", 10000, 6000, 4000, true},
		{"exceed auth", 10000, 0, 10001, false},
		{"exceed remaining", 10000, 6000, 4001, false},
		{"capture after full", 10000, 10000, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &GroupState{
				ActiveAuthID:     stringPtr("auth1"),
				ActiveAuthAmount: tt.authAmountCents,
				CapturedAmount:   tt.capturedAmountCents,
				IsAuthVoided:     false,
			}

			canCapture, reason := state.CanCapture(tt.captureAmountCents)
			if tt.shouldAllow {
				assert.True(t, canCapture, "should allow capture: %s", reason)
				assert.Empty(t, reason)
			} else {
				assert.False(t, canCapture, "should block capture")
				assert.NotEmpty(t, reason)
			}
		})
	}
}

// TestCanCapture_NoActiveAuth tests CAPTURE validation without active AUTH
func TestCanCapture_NoActiveAuth(t *testing.T) {
	state := &GroupState{
		ActiveAuthID:   nil,
		CapturedAmount: 0,
	}

	canCapture, reason := state.CanCapture(5000) // $50.00

	assert.False(t, canCapture)
	assert.Equal(t, "no active authorization found", reason)
}

// TestCanCapture_VoidedAuth tests CAPTURE validation after AUTH was voided
func TestCanCapture_VoidedAuth(t *testing.T) {
	state := &GroupState{
		ActiveAuthID: stringPtr("auth1"),
		IsAuthVoided: true,
	}

	canCapture, reason := state.CanCapture(5000) // $50.00

	assert.False(t, canCapture)
	assert.Equal(t, "authorization was voided", reason)
}

// TestCanVoid_Success tests successful VOID validation
func TestCanVoid_Success(t *testing.T) {
	state := &GroupState{
		ActiveAuthID: stringPtr("auth1"),
		IsAuthVoided: false,
	}

	canVoid, reason := state.CanVoid()

	assert.True(t, canVoid)
	assert.Empty(t, reason)
}

// TestCanVoid_AlreadyVoided tests VOID validation when AUTH already voided
func TestCanVoid_AlreadyVoided(t *testing.T) {
	state := &GroupState{
		ActiveAuthID: stringPtr("auth1"),
		IsAuthVoided: true,
	}

	canVoid, reason := state.CanVoid()

	assert.False(t, canVoid)
	assert.Equal(t, "authorization already voided", reason)
}

// TestCanVoid_NoActiveAuth tests VOID validation without active AUTH
func TestCanVoid_NoActiveAuth(t *testing.T) {
	state := &GroupState{
		ActiveAuthID: nil,
		IsAuthVoided: false,
	}

	canVoid, reason := state.CanVoid()

	assert.False(t, canVoid)
	assert.Equal(t, "no active authorization to void", reason)
}

// TestCanRefund_Success tests successful REFUND validation
func TestCanRefund_Success(t *testing.T) {
	tests := []struct {
		name                string
		capturedAmountCents int64
		refundedAmountCents int64
		refundAmountCents   int64
		shouldAllow         bool
	}{
		{"full refund", 10000, 0, 10000, true},
		{"partial refund", 10000, 0, 6000, true},
		{"remaining refund", 10000, 6000, 4000, true},
		{"exceed captured", 10000, 0, 10001, false},
		{"exceed remaining", 10000, 6000, 4001, false},
		{"refund after full", 10000, 10000, 1, false},
		{"multiple partial refunds", 10000, 7500, 2500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &GroupState{
				CapturedAmount: tt.capturedAmountCents,
				RefundedAmount: tt.refundedAmountCents,
			}

			canRefund, reason := state.CanRefund(tt.refundAmountCents)
			if tt.shouldAllow {
				assert.True(t, canRefund, "should allow refund: %s", reason)
				assert.Empty(t, reason)
			} else {
				assert.False(t, canRefund, "should block refund")
				assert.NotEmpty(t, reason)
			}
		})
	}
}

// TestCanRefund_NoCapturedAmount tests REFUND validation without captured amount
func TestCanRefund_NoCapturedAmount(t *testing.T) {
	state := &GroupState{
		CapturedAmount: 0,
		RefundedAmount: 0,
	}

	canRefund, reason := state.CanRefund(5000) // $50.00 in cents

	assert.False(t, canRefund)
	assert.Equal(t, "no captured amount to refund", reason)
}

// TestGetBRICForOperation tests BRIC selection for different operation types
func TestGetBRICForOperation(t *testing.T) {
	tests := []struct {
		name         string
		state        *GroupState
		operation    domain.TransactionType
		expectedBRIC string
		description  string
	}{
		{
			name: "CAPTURE uses AUTH BRIC",
			state: &GroupState{
				ActiveAuthBRIC: "bric_auth",
				CaptureBRIC:    "bric_capture",
			},
			operation:    domain.TransactionTypeCapture,
			expectedBRIC: "bric_auth",
			description:  "CAPTURE should use AUTH's BRIC",
		},
		{
			name: "VOID uses AUTH BRIC",
			state: &GroupState{
				ActiveAuthBRIC: "bric_auth",
				CaptureBRIC:    "bric_capture",
			},
			operation:    domain.TransactionTypeVoid,
			expectedBRIC: "bric_auth",
			description:  "VOID should use AUTH's BRIC",
		},
		{
			name: "REFUND uses CAPTURE BRIC when available",
			state: &GroupState{
				ActiveAuthBRIC: "bric_auth",
				CaptureBRIC:    "bric_capture",
			},
			operation:    domain.TransactionTypeRefund,
			expectedBRIC: "bric_capture",
			description:  "REFUND should use CAPTURE's BRIC when available",
		},
		{
			name: "REFUND uses AUTH BRIC for SALE (no CAPTURE BRIC)",
			state: &GroupState{
				ActiveAuthBRIC: "bric_sale",
				CaptureBRIC:    "", // SALE doesn't have separate CAPTURE
			},
			operation:    domain.TransactionTypeRefund,
			expectedBRIC: "bric_sale",
			description:  "REFUND should use AUTH's BRIC for SALE transactions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bric := tt.state.GetBRICForOperation(tt.operation)
			assert.Equal(t, tt.expectedBRIC, bric, tt.description)
		})
	}
}

// TestComplexWorkflow tests a complex transaction workflow
func TestComplexWorkflow(t *testing.T) {
	// Scenario: AUTH → CAPTURE (partial) → CAPTURE (partial) → REFUND (partial) → REFUND (partial)
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, 6000, "bric_capture1"),
		makeTransaction("capture2", domain.TransactionTypeCapture, 4000, "bric_capture2"),
		makeTransaction("refund1", domain.TransactionTypeRefund, 3000, "bric_refund1"),
		makeTransaction("refund2", domain.TransactionTypeRefund, 2000, "bric_refund2"),
	}

	state := ComputeGroupState(txs)

	// Verify state
	assert.Equal(t, int64(10000), state.ActiveAuthAmount)
	assert.Equal(t, int64(10000), state.CapturedAmount) // 60 + 40
	assert.Equal(t, int64(5000), state.RefundedAmount)  // 30 + 20

	// Should allow REFUND of remaining $50 (5000 cents)
	canRefund, reason := state.CanRefund(5000)
	assert.True(t, canRefund, reason)

	// Should block REFUND of $50.01 (5001 cents)
	canRefund, reason = state.CanRefund(5001)
	assert.False(t, canRefund)
	assert.Contains(t, reason, "exceeds remaining refundable amount")

	// Should not allow CAPTURE (already fully captured)
	canCapture, _ := state.CanCapture(1)
	assert.False(t, canCapture)
}

// TestComputeGroupState_ReAuthWithChildTransactions tests re-auth scenario with child transactions
// Validates that auth1 → auth2 → capture2 → refund2 works correctly
// This test validates the SQL GetTransactionTree query behavior where second AUTH has first AUTH as parent
func TestComputeGroupState_ReAuthWithChildTransactions(t *testing.T) {
	// Scenario: Customer increases cart amount, requiring re-authorization
	// auth1 ($100) → auth2 ($150, parent=auth1) → capture2 ($150) → refund2 ($50)
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),          // $100 original auth
		makeTransaction("auth2", domain.TransactionTypeAuth, 15000, "bric_auth2"),          // $150 re-auth (parent=auth1)
		makeTransaction("capture2", domain.TransactionTypeCapture, 15000, "bric_capture2"), // Capture the re-auth
		makeTransaction("refund2", domain.TransactionTypeRefund, 5000, "bric_refund2"),     // Partial refund
	}

	state := ComputeGroupState(txs)

	// State should reflect auth2 (the most recent AUTH)
	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth2", *state.ActiveAuthID, "Should use second AUTH")
	assert.Equal(t, int64(15000), state.ActiveAuthAmount, "Should use $150 from auth2")
	assert.Equal(t, int64(15000), state.CapturedAmount, "Should have $150 captured")
	assert.Equal(t, int64(5000), state.RefundedAmount, "Should have $50 refunded")
	assert.Equal(t, "bric_capture2", state.CaptureBRIC, "Should use capture2's BRIC")
	assert.False(t, state.IsAuthVoided, "Auth should not be voided")

	// Validate remaining refundable amount
	remaining := state.CapturedAmount - state.RefundedAmount
	assert.Equal(t, int64(10000), remaining, "Should have $100 remaining for refund")

	// Should allow refund of remaining $100
	canRefund, reason := state.CanRefund(10000)
	assert.True(t, canRefund, reason)

	// Should not allow refund exceeding remaining
	canRefund, reason = state.CanRefund(10001)
	assert.False(t, canRefund)
	assert.Contains(t, reason, "exceeds remaining refundable amount")

	// Should not allow capture (already fully captured)
	canCapture, _ := state.CanCapture(1)
	assert.False(t, canCapture)
}

// TestComputeGroupState_ReAuthIgnoresFirstAuthCaptures tests that captures on old auth are ignored
// When re-auth happens, previous auth's captures should not count toward new auth
func TestComputeGroupState_ReAuthIgnoresFirstAuthCaptures(t *testing.T) {
	// auth1 ($100) → capture1 ($50) → auth2 ($150, parent=auth1)
	// After auth2, capture1 should not count in state
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, 10000, "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, 5000, "bric_capture1"), // Capture on auth1
		makeTransaction("auth2", domain.TransactionTypeAuth, 15000, "bric_auth2"),         // Re-auth resets state
	}

	state := ComputeGroupState(txs)

	// State should be reset by auth2
	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth2", *state.ActiveAuthID)
	assert.Equal(t, int64(15000), state.ActiveAuthAmount)
	assert.Equal(t, int64(0), state.CapturedAmount, "Captured amount should reset to 0 after re-auth")
	assert.Equal(t, int64(0), state.RefundedAmount, "Refunded amount should reset to 0 after re-auth")

	// Should allow capturing full $150 from auth2
	canCapture, reason := state.CanCapture(15000)
	assert.True(t, canCapture, reason)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
