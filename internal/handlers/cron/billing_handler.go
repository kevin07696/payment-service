package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/kevin07696/payment-service/internal/services/ports"
	"go.uber.org/zap"
)

// BillingHandler handles cron job endpoints for subscription billing
type BillingHandler struct {
	subscriptionService ports.SubscriptionService
	logger              *zap.Logger
	cronSecret          string // Secret token for authenticating cron requests
}

// NewBillingHandler creates a new billing cron handler
func NewBillingHandler(
	subscriptionService ports.SubscriptionService,
	logger *zap.Logger,
	cronSecret string,
) *BillingHandler {
	return &BillingHandler{
		subscriptionService: subscriptionService,
		logger:              logger,
		cronSecret:          cronSecret,
	}
}

// ProcessBillingRequest represents the request body for manual billing processing
type ProcessBillingRequest struct {
	AsOfDate  *string `json:"as_of_date"` // Optional: ISO date string, defaults to today
	BatchSize *int    `json:"batch_size"` // Optional: defaults to 100
}

// ProcessBillingResponse represents the response from billing processing
type ProcessBillingResponse struct {
	Success      bool     `json:"success"`
	Processed    int      `json:"processed"`
	SuccessCount int      `json:"success_count"`
	FailureCount int      `json:"failure_count"`
	Errors       []string `json:"errors,omitempty"`
	ProcessedAt  string   `json:"processed_at"`
}

// ProcessBilling handles the POST /cron/process-billing endpoint
// This endpoint is called by Cloud Scheduler to process subscription billing
func (h *BillingHandler) ProcessBilling(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Billing cron job triggered",
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
	var req ProcessBillingRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Failed to parse request body",
				zap.Error(err),
			)
			// Continue with defaults if parsing fails
		}
	}

	// Determine as-of date
	asOfDate := time.Now()
	if req.AsOfDate != nil {
		parsed, err := time.Parse("2006-01-02", *req.AsOfDate)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid as_of_date format: %v", err))
			return
		}
		asOfDate = parsed
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

	// Process billing
	ctx := context.Background()
	processed, success, failed, errs := h.subscriptionService.ProcessDueBilling(ctx, asOfDate, batchSize)

	// Build response
	resp := ProcessBillingResponse{
		Success:      failed == 0,
		Processed:    processed,
		SuccessCount: success,
		FailureCount: failed,
		ProcessedAt:  time.Now().Format(time.RFC3339),
	}

	if len(errs) > 0 {
		resp.Errors = make([]string, len(errs))
		for i, err := range errs {
			resp.Errors[i] = err.Error()
		}
	}

	h.logger.Info("Billing processing completed",
		zap.Int("processed", processed),
		zap.Int("success", success),
		zap.Int("failed", failed),
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

// authenticateRequest verifies the cron request is authorized
func (h *BillingHandler) authenticateRequest(r *http.Request) bool {
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
func (h *BillingHandler) respondError(w http.ResponseWriter, statusCode int, message string) {
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

// HealthCheck handles GET /cron/health for monitoring
func (h *BillingHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(resp)
}

// Stats handles GET /cron/stats for monitoring billing statistics
func (h *BillingHandler) Stats(w http.ResponseWriter, r *http.Request) {
	// Authenticate the request
	if !h.authenticateRequest(r) {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Parse query parameters
	days := 7
	if daysParam := r.URL.Query().Get("days"); daysParam != "" {
		if parsed, err := strconv.Atoi(daysParam); err == nil && parsed > 0 && parsed <= 90 {
			days = parsed
		}
	}

	// In a real implementation, you would query the database for stats
	// For now, return placeholder data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"success": true,
		"period":  fmt.Sprintf("last_%d_days", days),
		"stats": map[string]interface{}{
			"total_subscriptions":    0,
			"active_subscriptions":   0,
			"paused_subscriptions":   0,
			"past_due_subscriptions": 0,
			"billing_runs":           0,
			"total_processed":        0,
			"total_succeeded":        0,
			"total_failed":           0,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(resp)
}
