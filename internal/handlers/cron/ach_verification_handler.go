package cron

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ACHVerificationHandler handles cron job endpoints for ACH verification
type ACHVerificationHandler struct {
	db         *sql.DB
	logger     *zap.Logger
	cronSecret string // Secret token for authenticating cron requests
}

// NewACHVerificationHandler creates a new ACH verification cron handler
func NewACHVerificationHandler(
	db *sql.DB,
	logger *zap.Logger,
	cronSecret string,
) *ACHVerificationHandler {
	return &ACHVerificationHandler{
		db:         db,
		logger:     logger,
		cronSecret: cronSecret,
	}
}

// VerifyACHRequest represents the request body for ACH verification
type VerifyACHRequest struct {
	VerificationDays *int `json:"verification_days"` // Optional: days to wait before verifying, defaults to 3
	BatchSize        *int `json:"batch_size"`        // Optional: defaults to 100
}

// VerifyACHResponse represents the response from ACH verification
type VerifyACHResponse struct {
	Success     bool     `json:"success"`
	Verified    int      `json:"verified"`
	Skipped     int      `json:"skipped"`
	Errors      []string `json:"errors,omitempty"`
	ProcessedAt string   `json:"processed_at"`
}

// VerifyACH handles the POST /cron/verify-ach endpoint
// This endpoint is called by Cloud Scheduler to verify ACH accounts after pre-note period (3 days)
// ACH pre-note (CKC0) must be sent when saving an ACH account, then wait 3 business days
// before considering the account verified and allowing transactions (per EPX guidelines)
func (h *ACHVerificationHandler) VerifyACH(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("ACH verification cron job triggered",
		zap.String("method", r.Method),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
	)

	// Verify request method
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// Authenticate the request
	if !h.authenticateRequest(r) {
		h.logger.Warn("Unauthorized cron request",
			zap.String("remote_addr", r.RemoteAddr),
		)
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Parse request body (optional parameters)
	var req VerifyACHRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Failed to parse request body",
				zap.Error(err),
			)
			// Continue with defaults if parsing fails
		}
	}

	// Determine verification days (default 3)
	verificationDays := 3
	if req.VerificationDays != nil {
		if *req.VerificationDays < 1 || *req.VerificationDays > 30 {
			h.respondError(w, http.StatusBadRequest, "verification_days must be between 1 and 30")
			return
		}
		verificationDays = *req.VerificationDays
	}

	// Determine batch size
	batchSize := 100
	if req.BatchSize != nil {
		if *req.BatchSize < 1 || *req.BatchSize > 1000 {
			h.respondError(w, http.StatusBadRequest, "batch_size must be between 1 and 1000")
			return
		}
		batchSize = *req.BatchSize
	}

	// Process ACH verification
	ctx := context.Background()
	verified, skipped, errs := h.processACHVerification(ctx, verificationDays, batchSize)

	// Build response
	resp := VerifyACHResponse{
		Success:     len(errs) == 0,
		Verified:    verified,
		Skipped:     skipped,
		ProcessedAt: time.Now().Format(time.RFC3339),
	}

	if len(errs) > 0 {
		resp.Errors = make([]string, len(errs))
		for i, err := range errs {
			resp.Errors[i] = err.Error()
		}
	}

	h.logger.Info("ACH verification completed",
		zap.Int("verified", verified),
		zap.Int("skipped", skipped),
		zap.Int("errors", len(errs)),
	)

	// Respond with JSON
	w.Header().Set("Content-Type", "application/json")
	if resp.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusPartialContent) // 206 indicates partial success
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// processACHVerification finds and verifies pending ACH accounts that are past the verification period
func (h *ACHVerificationHandler) processACHVerification(ctx context.Context, verificationDays int, batchSize int) (verified int, skipped int, errs []error) {
	// Find ACH payment methods pending verification that are older than verificationDays
	// Note: We use calendar days (not business days) for simplicity
	// In production, you might want to calculate business days excluding weekends/holidays
	cutoffDate := time.Now().AddDate(0, 0, -verificationDays)

	query := `
		SELECT id, merchant_id, customer_id, payment_type
		FROM customer_payment_methods
		WHERE payment_type = 'ach'
		  AND verification_status = 'pending'
		  AND created_at <= $1
		  AND deleted_at IS NULL
		ORDER BY created_at ASC
		LIMIT $2
	`

	rows, err := h.db.QueryContext(ctx, query, cutoffDate, batchSize)
	if err != nil {
		h.logger.Error("Failed to query pending ACH accounts",
			zap.Error(err),
			zap.Time("cutoff_date", cutoffDate),
		)
		return 0, 0, []error{fmt.Errorf("failed to query pending accounts: %w", err)}
	}
	defer rows.Close()

	var paymentMethodIDs []string
	for rows.Next() {
		var id, merchantID, customerID, paymentType string
		if err := rows.Scan(&id, &merchantID, &customerID, &paymentType); err != nil {
			h.logger.Error("Failed to scan payment method",
				zap.Error(err),
			)
			errs = append(errs, fmt.Errorf("failed to scan row: %w", err))
			continue
		}

		paymentMethodIDs = append(paymentMethodIDs, id)
	}

	if err := rows.Err(); err != nil {
		h.logger.Error("Error iterating rows", zap.Error(err))
		errs = append(errs, fmt.Errorf("error iterating rows: %w", err))
	}

	h.logger.Info("Found ACH accounts eligible for verification",
		zap.Int("count", len(paymentMethodIDs)),
		zap.Time("cutoff_date", cutoffDate),
		zap.Int("verification_days", verificationDays),
	)

	// Update each payment method to verified status and activate it
	updateQuery := `
		UPDATE customer_payment_methods
		SET verification_status = 'verified',
		    is_verified = true,
		    is_active = true,
		    verified_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND verification_status = 'pending'
		  AND payment_type = 'ach'
	`

	for _, paymentMethodID := range paymentMethodIDs {
		result, err := h.db.ExecContext(ctx, updateQuery, paymentMethodID)
		if err != nil {
			h.logger.Error("Failed to verify ACH account",
				zap.String("payment_method_id", paymentMethodID),
				zap.Error(err),
			)
			errs = append(errs, fmt.Errorf("failed to verify %s: %w", paymentMethodID, err))
			continue
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			h.logger.Warn("Failed to get rows affected",
				zap.String("payment_method_id", paymentMethodID),
				zap.Error(err),
			)
		}

		if rowsAffected == 0 {
			h.logger.Warn("No rows updated - payment method may have been modified",
				zap.String("payment_method_id", paymentMethodID),
			)
			skipped++
		} else {
			h.logger.Info("ACH account verified",
				zap.String("payment_method_id", paymentMethodID),
			)
			verified++
		}
	}

	return verified, skipped, errs
}

// authenticateRequest verifies the cron request is authorized
func (h *ACHVerificationHandler) authenticateRequest(r *http.Request) bool {
	// Check X-Cron-Secret header
	cronSecret := r.Header.Get("X-Cron-Secret")
	if cronSecret != "" && cronSecret == h.cronSecret {
		return true
	}

	// Check Authorization header (Bearer token)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "Bearer "+h.cronSecret {
		return true
	}

	// Check for Google Cloud Scheduler OIDC token (for production)
	// In production, you would verify the OIDC token here
	// For now, we'll accept requests from Cloud Scheduler's IP ranges
	// or rely on the X-Cron-Secret header

	// Check query parameter (less secure, for development only)
	querySecret := r.URL.Query().Get("secret")
	if querySecret != "" && querySecret == h.cronSecret {
		h.logger.Warn("Using query parameter authentication (insecure)",
			zap.String("remote_addr", r.RemoteAddr),
		)
		return true
	}

	return false
}

// respondError sends an error response
func (h *ACHVerificationHandler) respondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := map[string]interface{}{
		"success": false,
		"error":   message,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode error response", zap.Error(err))
	}
}

// HealthCheck handles GET /cron/ach/health for monitoring
func (h *ACHVerificationHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(resp)
}

// Stats handles GET /cron/ach/stats for monitoring ACH verification statistics
func (h *ACHVerificationHandler) Stats(w http.ResponseWriter, r *http.Request) {
	// Authenticate the request
	if !h.authenticateRequest(r) {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Query ACH verification statistics
	var stats struct {
		TotalACH    int `json:"total_ach"`
		Pending     int `json:"pending"`
		Verified    int `json:"verified"`
		Failed      int `json:"failed"`
		EligibleNow int `json:"eligible_now"` // Pending accounts eligible for verification
	}

	// Total ACH accounts
	err := h.db.QueryRow(`
		SELECT COUNT(*) FROM customer_payment_methods
		WHERE payment_type = 'ach' AND deleted_at IS NULL
	`).Scan(&stats.TotalACH)
	if err != nil && err != sql.ErrNoRows {
		h.logger.Error("Failed to query total ACH", zap.Error(err))
	}

	// Pending verification
	err = h.db.QueryRow(`
		SELECT COUNT(*) FROM customer_payment_methods
		WHERE payment_type = 'ach'
		  AND verification_status = 'pending'
		  AND deleted_at IS NULL
	`).Scan(&stats.Pending)
	if err != nil && err != sql.ErrNoRows {
		h.logger.Error("Failed to query pending ACH", zap.Error(err))
	}

	// Verified
	err = h.db.QueryRow(`
		SELECT COUNT(*) FROM customer_payment_methods
		WHERE payment_type = 'ach'
		  AND verification_status = 'verified'
		  AND deleted_at IS NULL
	`).Scan(&stats.Verified)
	if err != nil && err != sql.ErrNoRows {
		h.logger.Error("Failed to query verified ACH", zap.Error(err))
	}

	// Failed
	err = h.db.QueryRow(`
		SELECT COUNT(*) FROM customer_payment_methods
		WHERE payment_type = 'ach'
		  AND verification_status = 'failed'
		  AND deleted_at IS NULL
	`).Scan(&stats.Failed)
	if err != nil && err != sql.ErrNoRows {
		h.logger.Error("Failed to query failed ACH", zap.Error(err))
	}

	// Eligible for verification (pending > 3 days)
	cutoffDate := time.Now().AddDate(0, 0, -3)
	err = h.db.QueryRow(`
		SELECT COUNT(*) FROM customer_payment_methods
		WHERE payment_type = 'ach'
		  AND verification_status = 'pending'
		  AND created_at <= $1
		  AND deleted_at IS NULL
	`, cutoffDate).Scan(&stats.EligibleNow)
	if err != nil && err != sql.ErrNoRows {
		h.logger.Error("Failed to query eligible ACH", zap.Error(err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"success":   true,
		"stats":     stats,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(resp)
}
