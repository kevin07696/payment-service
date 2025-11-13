package payment

import (
	"testing"

	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create approved transaction
func makeTransaction(id string, txType domain.TransactionType, amount string, authGUID string) *domain.Transaction {
	amt, _ := decimal.NewFromString(amount)
	status := domain.TransactionStatusApproved
	return &domain.Transaction{
		ID:       id,
		Type:     txType,
		Amount:   amt,
		Status:   status,
		AuthGUID: authGUID,
		Metadata: make(map[string]interface{}),
	}
}

// Helper to create declined transaction
func makeDeclinedTransaction(id string, txType domain.TransactionType, amount string) *domain.Transaction {
	amt, _ := decimal.NewFromString(amount)
	status := domain.TransactionStatusDeclined
	return &domain.Transaction{
		ID:     id,
		Type:   txType,
		Amount: amt,
		Status: status,
	}
}

// TestComputeGroupState_EmptyTransactions tests empty transaction list
func TestComputeGroupState_EmptyTransactions(t *testing.T) {
	state := ComputeGroupState([]*domain.Transaction{})

	assert.Nil(t, state.ActiveAuthID)
	assert.True(t, state.CapturedAmount.IsZero())
	assert.True(t, state.RefundedAmount.IsZero())
	assert.False(t, state.IsAuthVoided)
	assert.Empty(t, state.ActiveAuthBRIC)
}

// TestComputeGroupState_SingleAuth tests single AUTH transaction
func TestComputeGroupState_SingleAuth(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth1", *state.ActiveAuthID)
	assert.Equal(t, "100.00", state.ActiveAuthAmount.StringFixed(2))
	assert.Equal(t, "bric_auth1", state.ActiveAuthBRIC)
	assert.True(t, state.CapturedAmount.IsZero())
	assert.False(t, state.IsAuthVoided)
}

// TestComputeGroupState_Sale tests SALE transaction (auth+capture in one)
func TestComputeGroupState_Sale(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("sale1", domain.TransactionTypeSale, "100.00", "bric_sale1"),
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "sale1", *state.ActiveAuthID)
	assert.Equal(t, "100.00", state.ActiveAuthAmount.StringFixed(2))
	assert.Equal(t, "100.00", state.CapturedAmount.StringFixed(2)) // SALE captures immediately
	assert.Equal(t, "bric_sale1", state.ActiveAuthBRIC)
	assert.False(t, state.IsAuthVoided)
}

// TestComputeGroupState_AuthThenCapture tests AUTH → CAPTURE flow
func TestComputeGroupState_AuthThenCapture(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, "100.00", "bric_capture1"),
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth1", *state.ActiveAuthID)
	assert.Equal(t, "100.00", state.CapturedAmount.StringFixed(2))
	assert.Equal(t, "bric_capture1", state.CaptureBRIC)
	assert.Equal(t, "bric_capture1", state.CurrentBRIC) // Uses CAPTURE's BRIC
}

// TestComputeGroupState_PartialCapture tests partial CAPTURE
func TestComputeGroupState_PartialCapture(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, "60.00", "bric_capture1"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, "100.00", state.ActiveAuthAmount.StringFixed(2))
	assert.Equal(t, "60.00", state.CapturedAmount.StringFixed(2))
}

// TestComputeGroupState_MultiplePartialCaptures tests multiple partial CAPTUREs
func TestComputeGroupState_MultiplePartialCaptures(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, "30.00", "bric_capture1"),
		makeTransaction("capture2", domain.TransactionTypeCapture, "40.00", "bric_capture2"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, "100.00", state.ActiveAuthAmount.StringFixed(2))
	assert.Equal(t, "70.00", state.CapturedAmount.StringFixed(2)) // 30 + 40
	assert.Equal(t, "bric_capture2", state.CaptureBRIC)     // Most recent CAPTURE's BRIC
}

// TestComputeGroupState_ReAuth tests re-authorization (new AUTH resets state)
func TestComputeGroupState_ReAuth(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, "50.00", "bric_capture1"),
		makeTransaction("auth2", domain.TransactionTypeAuth, "150.00", "bric_auth2"), // Re-auth
	}

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID)
	assert.Equal(t, "auth2", *state.ActiveAuthID)
	assert.Equal(t, "150.00", state.ActiveAuthAmount.StringFixed(2))
	assert.True(t, state.CapturedAmount.IsZero()) // Resets on new AUTH
	assert.Equal(t, "bric_auth2", state.ActiveAuthBRIC)
}

// TestComputeGroupState_VoidAuth tests VOID of AUTH
func TestComputeGroupState_VoidAuth(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeTransaction("void1", domain.TransactionTypeVoid, "100.00", "bric_void1"),
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
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, "100.00", "bric_capture1"),
		makeTransaction("void1", domain.TransactionTypeVoid, "100.00", "bric_void1"),
	}
	txs[2].Metadata["original_transaction_type"] = "capture"

	state := ComputeGroupState(txs)

	require.NotNil(t, state.ActiveAuthID) // AUTH still active
	assert.Equal(t, "auth1", *state.ActiveAuthID)
	assert.True(t, state.CapturedAmount.IsZero()) // CAPTURE was voided
	assert.False(t, state.IsAuthVoided)
}

// TestComputeGroupState_Refund tests REFUND
func TestComputeGroupState_Refund(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("sale1", domain.TransactionTypeSale, "100.00", "bric_sale1"),
		makeTransaction("refund1", domain.TransactionTypeRefund, "30.00", "bric_refund1"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, "100.00", state.CapturedAmount.StringFixed(2))
	assert.Equal(t, "30.00", state.RefundedAmount.StringFixed(2))
}

// TestComputeGroupState_MultipleRefunds tests multiple partial REFUNDs
func TestComputeGroupState_MultipleRefunds(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("sale1", domain.TransactionTypeSale, "100.00", "bric_sale1"),
		makeTransaction("refund1", domain.TransactionTypeRefund, "25.00", "bric_refund1"),
		makeTransaction("refund2", domain.TransactionTypeRefund, "25.00", "bric_refund2"),
		makeTransaction("refund3", domain.TransactionTypeRefund, "25.00", "bric_refund3"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, "100.00", state.CapturedAmount.StringFixed(2))
	assert.Equal(t, "75.00", state.RefundedAmount.StringFixed(2)) // 25 + 25 + 25
}

// TestComputeGroupState_DeclinedTransactionsIgnored tests that declined transactions don't affect state
func TestComputeGroupState_DeclinedTransactionsIgnored(t *testing.T) {
	txs := []*domain.Transaction{
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeDeclinedTransaction("capture1", domain.TransactionTypeCapture, "50.00"), // Declined
		makeTransaction("capture2", domain.TransactionTypeCapture, "60.00", "bric_capture2"),
	}

	state := ComputeGroupState(txs)

	assert.Equal(t, "60.00", state.CapturedAmount.StringFixed(2)) // Only approved CAPTURE counts
}

// TestCanCapture_Success tests successful CAPTURE validation
func TestCanCapture_Success(t *testing.T) {
	tests := []struct {
		name           string
		authAmount     string
		capturedAmount string
		captureAmount  string
		shouldAllow    bool
	}{
		{"full capture", "100.00", "0", "100.00", true},
		{"partial capture", "100.00", "0", "60.00", true},
		{"remaining capture", "100.00", "60.00", "40.00", true},
		{"exceed auth", "100.00", "0", "100.01", false},
		{"exceed remaining", "100.00", "60.00", "40.01", false},
		{"capture after full", "100.00", "100.00", "0.01", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authAmt, _ := decimal.NewFromString(tt.authAmount)
			capturedAmt, _ := decimal.NewFromString(tt.capturedAmount)
			captureAmt, _ := decimal.NewFromString(tt.captureAmount)

			state := &GroupState{
				ActiveAuthID:     stringPtr("auth1"),
				ActiveAuthAmount: authAmt,
				CapturedAmount:   capturedAmt,
				IsAuthVoided:     false,
			}

			canCapture, reason := state.CanCapture(captureAmt)
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
		CapturedAmount: decimal.Zero,
	}

	captureAmt, _ := decimal.NewFromString("50.00")
	canCapture, reason := state.CanCapture(captureAmt)

	assert.False(t, canCapture)
	assert.Equal(t, "no active authorization found", reason)
}

// TestCanCapture_VoidedAuth tests CAPTURE validation after AUTH was voided
func TestCanCapture_VoidedAuth(t *testing.T) {
	state := &GroupState{
		ActiveAuthID: stringPtr("auth1"),
		IsAuthVoided: true,
	}

	captureAmt, _ := decimal.NewFromString("50.00")
	canCapture, reason := state.CanCapture(captureAmt)

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
		name           string
		capturedAmount string
		refundedAmount string
		refundAmount   string
		shouldAllow    bool
	}{
		{"full refund", "100.00", "0", "100.00", true},
		{"partial refund", "100.00", "0", "60.00", true},
		{"remaining refund", "100.00", "60.00", "40.00", true},
		{"exceed captured", "100.00", "0", "100.01", false},
		{"exceed remaining", "100.00", "60.00", "40.01", false},
		{"refund after full", "100.00", "100.00", "0.01", false},
		{"multiple partial refunds", "100.00", "75.00", "25.00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedAmt, _ := decimal.NewFromString(tt.capturedAmount)
			refundedAmt, _ := decimal.NewFromString(tt.refundedAmount)
			refundAmt, _ := decimal.NewFromString(tt.refundAmount)

			state := &GroupState{
				CapturedAmount: capturedAmt,
				RefundedAmount: refundedAmt,
			}

			canRefund, reason := state.CanRefund(refundAmt)
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
		CapturedAmount: decimal.Zero,
		RefundedAmount: decimal.Zero,
	}

	refundAmt, _ := decimal.NewFromString("50.00")
	canRefund, reason := state.CanRefund(refundAmt)

	assert.False(t, canRefund)
	assert.Equal(t, "no captured amount to refund", reason)
}

// TestGetBRICForOperation tests BRIC selection for different operation types
func TestGetBRICForOperation(t *testing.T) {
	tests := []struct {
		name          string
		state         *GroupState
		operation     domain.TransactionType
		expectedBRIC  string
		description   string
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
		makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
		makeTransaction("capture1", domain.TransactionTypeCapture, "60.00", "bric_capture1"),
		makeTransaction("capture2", domain.TransactionTypeCapture, "40.00", "bric_capture2"),
		makeTransaction("refund1", domain.TransactionTypeRefund, "30.00", "bric_refund1"),
		makeTransaction("refund2", domain.TransactionTypeRefund, "20.00", "bric_refund2"),
	}

	state := ComputeGroupState(txs)

	// Verify state
	assert.Equal(t, "100.00", state.ActiveAuthAmount.StringFixed(2))
	assert.Equal(t, "100.00", state.CapturedAmount.StringFixed(2)) // 60 + 40
	assert.Equal(t, "50.00", state.RefundedAmount.StringFixed(2))  // 30 + 20

	// Should allow REFUND of remaining $50
	refundAmt50, _ := decimal.NewFromString("50.00")
	canRefund, reason := state.CanRefund(refundAmt50)
	assert.True(t, canRefund, reason)

	// Should block REFUND of $50.01
	refundAmt5001, _ := decimal.NewFromString("50.01")
	canRefund, reason = state.CanRefund(refundAmt5001)
	assert.False(t, canRefund)
	assert.Contains(t, reason, "exceeds remaining refundable amount")

	// Should not allow CAPTURE (already fully captured)
	captureAmt001, _ := decimal.NewFromString("0.01")
	canCapture, reason := state.CanCapture(captureAmt001)
	assert.False(t, canCapture)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
