package testutil

import (
	"database/sql"
	"fmt"
)

// MarkACHAsVerified simulates the cron job marking an ACH account as verified
// This allows testing payments without waiting 3 days for verification
func MarkACHAsVerified(db *sql.DB, paymentMethodID string) error {
	_, err := db.Exec(`
		UPDATE customer_payment_methods
		SET verification_status = 'verified',
		    is_verified = true,
		    verified_at = NOW()
		WHERE id = $1
	`, paymentMethodID)

	if err != nil {
		return fmt.Errorf("failed to mark ACH as verified: %w", err)
	}

	return nil
}

// MarkACHAsFailed simulates receiving an ACH return code (e.g., R03 - No Account)
// This allows testing failed verification scenarios
func MarkACHAsFailed(db *sql.DB, paymentMethodID string, returnCode string) error {
	_, err := db.Exec(`
		UPDATE customer_payment_methods
		SET verification_status = 'failed',
		    is_active = false,
		    verification_failure_reason = $2,
		    deactivated_at = NOW(),
		    deactivation_reason = 'ach_return_code'
		WHERE id = $1
	`, paymentMethodID, returnCode)

	if err != nil {
		return fmt.Errorf("failed to mark ACH as failed: %w", err)
	}

	return nil
}

// GetACHVerificationStatus retrieves the current verification status of an ACH payment method
func GetACHVerificationStatus(db *sql.DB, paymentMethodID string) (string, bool, error) {
	var verificationStatus string
	var isVerified bool

	err := db.QueryRow(`
		SELECT verification_status, is_verified
		FROM customer_payment_methods
		WHERE id = $1
	`, paymentMethodID).Scan(&verificationStatus, &isVerified)

	if err != nil {
		return "", false, fmt.Errorf("failed to get verification status: %w", err)
	}

	return verificationStatus, isVerified, nil
}

// SimulateDaysPassed updates the created_at timestamp to simulate time passing
// This allows testing the cron job logic without waiting
func SimulateDaysPassed(db *sql.DB, paymentMethodID string, days int) error {
	_, err := db.Exec(`
		UPDATE customer_payment_methods
		SET created_at = NOW() - INTERVAL '1 day' * $2
		WHERE id = $1
	`, paymentMethodID, days)

	if err != nil {
		return fmt.Errorf("failed to simulate days passed: %w", err)
	}

	return nil
}
