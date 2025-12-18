package payment

import (
	"github.com/kevin07696/payment-service/internal/domain"
)

// GroupState represents the computed state of a transaction group
// Computed by replaying all transactions in chronological order (WAL-style)
type GroupState struct {
	// Active AUTH transaction (nil if voided or captured fully)
	ActiveAuthID     *string
	ActiveAuthAmount int64  // Amount in cents
	ActiveAuthBRIC   string // auth_guid from AUTH transaction

	// CAPTURE state
	CapturedAmount int64  // Amount in cents
	CaptureBRIC    string // Most recent CAPTURE's auth_guid (for REFUND)

	// REFUND state
	RefundedAmount int64 // Amount in cents

	// VOID state
	IsAuthVoided bool // True if AUTH was voided

	// Current BRIC for operations
	// - For CAPTURE: use ActiveAuthBRIC
	// - For VOID: use ActiveAuthBRIC
	// - For REFUND: use CaptureBRIC (if captured) or ActiveAuthBRIC (if SALE)
	CurrentBRIC string
}

// ComputeGroupState analyzes transaction history to determine current state
// Transactions MUST be ordered by created_at ASC (chronological order)
// This implements Write-Ahead Log (WAL) style state computation
func ComputeGroupState(transactions []*domain.Transaction) *GroupState {
	state := &GroupState{
		CapturedAmount: 0,
		RefundedAmount: 0,
	}

	for _, tx := range transactions {
		// Skip declined/failed transactions
		if tx.Status != domain.TransactionStatusApproved {
			continue
		}

		switch tx.Type {
		case domain.TransactionTypeAuth:
			// New AUTH replaces previous AUTH
			// This handles re-auth scenarios where customer adjusts order amount
			state.ActiveAuthID = &tx.ID
			state.ActiveAuthAmount = tx.AmountCents
			state.ActiveAuthBRIC = tx.AuthGUID
			state.CurrentBRIC = tx.AuthGUID
			state.IsAuthVoided = false
			// Reset capture/refund state (new auth starts fresh)
			state.CapturedAmount = 0
			state.RefundedAmount = 0
			state.CaptureBRIC = ""

		case domain.TransactionTypeSale:
			// SALE = AUTH + CAPTURE in one step
			// Treat as both AUTH and CAPTURE
			state.ActiveAuthID = &tx.ID
			state.ActiveAuthAmount = tx.AmountCents
			state.ActiveAuthBRIC = tx.AuthGUID
			state.CapturedAmount = tx.AmountCents // Already captured
			state.CurrentBRIC = tx.AuthGUID
			state.IsAuthVoided = false

		case domain.TransactionTypeCapture:
			// CAPTURE consumes part/all of AUTH
			state.CapturedAmount = state.CapturedAmount + tx.AmountCents
			state.CaptureBRIC = tx.AuthGUID // EPX returns new BRIC for CAPTURE
			state.CurrentBRIC = tx.AuthGUID // Use CAPTURE's BRIC for follow-up ops

		case domain.TransactionTypeVoid:
			// VOID cancels AUTH or reverses CAPTURE
			// Check metadata to see what was voided
			if tx.Metadata != nil {
				if origTxType, ok := tx.Metadata["original_transaction_type"].(string); ok {
					switch origTxType {
					case "auth":
						// VOID of AUTH - cancels the authorization hold
						state.IsAuthVoided = true
						state.ActiveAuthID = nil
						state.ActiveAuthBRIC = ""

					case "capture":
						// VOID of CAPTURE - reverses the capture (same-day only)
						state.CapturedAmount = state.CapturedAmount - tx.AmountCents
					}
				}
			}

		case domain.TransactionTypeRefund:
			// REFUND returns money to customer (post-settlement)
			state.RefundedAmount = state.RefundedAmount + tx.AmountCents
		}
	}

	return state
}

// CanCapture checks if a CAPTURE operation is allowed
func (s *GroupState) CanCapture(captureAmountCents int64) (bool, string) {
	// Check if AUTH was voided
	if s.IsAuthVoided {
		return false, "authorization was voided"
	}

	// Check if there's an active AUTH
	if s.ActiveAuthID == nil {
		return false, "no active authorization found"
	}

	// Check if capture amount exceeds remaining authorized amount
	remaining := s.ActiveAuthAmount - s.CapturedAmount
	if captureAmountCents > remaining {
		return false, "capture amount exceeds remaining authorized amount"
	}

	return true, ""
}

// CanVoid checks if a VOID operation is allowed
func (s *GroupState) CanVoid() (bool, string) {
	// Can void if there's an active (non-voided) AUTH
	if s.IsAuthVoided {
		return false, "authorization already voided"
	}

	if s.ActiveAuthID == nil {
		return false, "no active authorization to void"
	}

	return true, ""
}

// CanRefund checks if a REFUND operation is allowed
func (s *GroupState) CanRefund(refundAmountCents int64) (bool, string) {
	// Can only refund captured amounts
	if s.CapturedAmount == 0 {
		return false, "no captured amount to refund"
	}

	// Check if refund exceeds captured amount (minus already refunded)
	remaining := s.CapturedAmount - s.RefundedAmount
	if refundAmountCents > remaining {
		return false, "refund amount exceeds remaining refundable amount"
	}

	return true, ""
}

// GetBRICForOperation returns the correct BRIC to use for a given operation type
func (s *GroupState) GetBRICForOperation(opType domain.TransactionType) string {
	switch opType {
	case domain.TransactionTypeCapture, domain.TransactionTypeVoid:
		// CAPTURE and VOID use AUTH's BRIC
		return s.ActiveAuthBRIC

	case domain.TransactionTypeRefund:
		// REFUND uses CAPTURE's BRIC if available, otherwise AUTH's BRIC (SALE case)
		if s.CaptureBRIC != "" {
			return s.CaptureBRIC
		}
		return s.ActiveAuthBRIC

	default:
		return s.CurrentBRIC
	}
}
