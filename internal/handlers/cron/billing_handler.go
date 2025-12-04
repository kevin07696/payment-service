package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/pkg/timeutil"
	"go.uber.org/zap"
)

const (
	// defaultCronJobTimeout is the default maximum time allowed for cron job execution
	// when no timeout is configured via environment variable.
	defaultCronJobTimeout = 5 * time.Minute
)

// BillingHandlerConfig holds configuration for the billing cron handler
type BillingHandlerConfig struct {
	CronSecret       string
	JobTimeoutSecs   int
	DefaultBatchSize int
	MaxBatchSize     int
}

// BillingHandler handles cron job endpoints for subscription billing
type BillingHandler struct {
	subscriptionService ports.SubscriptionService
	queries             sqlc.Querier
	logger              *zap.Logger
	cronSecret          string        // Secret token for authenticating cron requests
	jobTimeout          time.Duration // Maximum time allowed for cron job execution
	defaultBatchSize    int           // Default batch size for processing
	maxBatchSize        int           // Maximum allowed batch size
}

// NewBillingHandler creates a new billing cron handler
func NewBillingHandler(
	subscriptionService ports.SubscriptionService,
	queries sqlc.Querier,
	logger *zap.Logger,
	cfg BillingHandlerConfig,
) *BillingHandler {
	timeout := defaultCronJobTimeout
	if cfg.JobTimeoutSecs > 0 {
		timeout = time.Duration(cfg.JobTimeoutSecs) * time.Second
	}

	defaultBatch := 100
	if cfg.DefaultBatchSize > 0 {
		defaultBatch = cfg.DefaultBatchSize
	}

	maxBatch := 1000
	if cfg.MaxBatchSize > 0 {
		maxBatch = cfg.MaxBatchSize
	}

	return &BillingHandler{
		subscriptionService: subscriptionService,
		queries:             queries,
		logger:              logger,
		cronSecret:          cfg.CronSecret,
		jobTimeout:          timeout,
		defaultBatchSize:    defaultBatch,
		maxBatchSize:        maxBatch,
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
	asOfDate := timeutil.Now()
	if req.AsOfDate != nil {
		parsed, err := time.Parse("2006-01-02", *req.AsOfDate)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid as_of_date format: %v", err))
			return
		}
		asOfDate = timeutil.ToUTC(parsed)
	}

	// Determine batch size
	batchSize := h.defaultBatchSize
	if req.BatchSize != nil {
		if *req.BatchSize < 1 || *req.BatchSize > h.maxBatchSize {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("batch_size must be between 1 and %d", h.maxBatchSize))
			return
		}
		batchSize = *req.BatchSize
	}

	// Process billing with timeout to prevent runaway jobs
	ctx, cancel := context.WithTimeout(context.Background(), h.jobTimeout)
	defer cancel()

	processed, success, failed, errs := h.subscriptionService.ProcessDueBilling(ctx, asOfDate, batchSize)

	// Build response
	resp := ProcessBillingResponse{
		Success:      failed == 0,
		Processed:    processed,
		SuccessCount: success,
		FailureCount: failed,
		ProcessedAt:  timeutil.Now().Format(time.RFC3339),
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

// ProcessExpiredPastDueRequest represents the request body for expired past_due processing
type ProcessExpiredPastDueRequest struct {
	BatchSize *int `json:"batch_size"` // Optional: defaults to 100
}

// ProcessExpiredPastDueResponse represents the response from expired past_due processing
type ProcessExpiredPastDueResponse struct {
	Success     bool     `json:"success"`
	Processed   int      `json:"processed"`
	Cancelled   int      `json:"cancelled"`
	Failed      int      `json:"failed"`
	Errors      []string `json:"errors,omitempty"`
	ProcessedAt string   `json:"processed_at"`
}

// ProcessExpiredPastDue handles the POST /cron/process-expired-past-due endpoint
// This endpoint auto-cancels subscriptions whose grace period has expired
func (h *BillingHandler) ProcessExpiredPastDue(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Expired past_due cron job triggered",
		zap.String("method", r.Method),
		zap.String("remote_addr", r.RemoteAddr),
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
	var req ProcessExpiredPastDueRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Failed to parse request body", zap.Error(err))
			// Continue with defaults if parsing fails
		}
	}

	// Determine batch size
	batchSize := h.defaultBatchSize
	if req.BatchSize != nil {
		if *req.BatchSize < 1 || *req.BatchSize > h.maxBatchSize {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("batch_size must be between 1 and %d", h.maxBatchSize))
			return
		}
		batchSize = *req.BatchSize
	}

	// Process expired past_due subscriptions with timeout to prevent runaway jobs
	ctx, cancel := context.WithTimeout(context.Background(), h.jobTimeout)
	defer cancel()

	result := h.subscriptionService.ProcessExpiredPastDue(ctx, batchSize)

	// Build response
	resp := ProcessExpiredPastDueResponse{
		Success:     result.Failed == 0,
		Processed:   result.Processed,
		Cancelled:   result.Cancelled,
		Failed:      result.Failed,
		ProcessedAt: timeutil.Now().Format(time.RFC3339),
	}

	if len(result.Errors) > 0 {
		resp.Errors = make([]string, len(result.Errors))
		for i, err := range result.Errors {
			resp.Errors[i] = err.Error()
		}
	}

	h.logger.Info("Expired past_due processing completed",
		zap.Int("processed", result.Processed),
		zap.Int("cancelled", result.Cancelled),
		zap.Int("failed", result.Failed),
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
		"time":   timeutil.Now().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// Stats handles GET /cron/stats for monitoring billing statistics
func (h *BillingHandler) Stats(w http.ResponseWriter, r *http.Request) {
	// Authenticate the request
	if !h.authenticateRequest(r) {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query subscription counts by status
	var totalCount, activeCount, pausedCount, pastDueCount, cancelledCount int64

	// Total subscriptions (no status filter)
	total, err := h.queries.CountSubscriptions(ctx, sqlc.CountSubscriptionsParams{
		MerchantID: pgtype.UUID{Valid: false},
		CustomerID: pgtype.Text{Valid: false},
		Status:     pgtype.Text{Valid: false},
	})
	if err != nil {
		h.logger.Error("Failed to count total subscriptions", zap.Error(err))
	} else {
		totalCount = total
	}

	// Active subscriptions
	active, err := h.queries.CountSubscriptions(ctx, sqlc.CountSubscriptionsParams{
		MerchantID: pgtype.UUID{Valid: false},
		CustomerID: pgtype.Text{Valid: false},
		Status:     pgtype.Text{String: "active", Valid: true},
	})
	if err != nil {
		h.logger.Error("Failed to count active subscriptions", zap.Error(err))
	} else {
		activeCount = active
	}

	// Paused subscriptions
	paused, err := h.queries.CountSubscriptions(ctx, sqlc.CountSubscriptionsParams{
		MerchantID: pgtype.UUID{Valid: false},
		CustomerID: pgtype.Text{Valid: false},
		Status:     pgtype.Text{String: "paused", Valid: true},
	})
	if err != nil {
		h.logger.Error("Failed to count paused subscriptions", zap.Error(err))
	} else {
		pausedCount = paused
	}

	// Past due subscriptions
	pastDue, err := h.queries.CountSubscriptions(ctx, sqlc.CountSubscriptionsParams{
		MerchantID: pgtype.UUID{Valid: false},
		CustomerID: pgtype.Text{Valid: false},
		Status:     pgtype.Text{String: "past_due", Valid: true},
	})
	if err != nil {
		h.logger.Error("Failed to count past_due subscriptions", zap.Error(err))
	} else {
		pastDueCount = pastDue
	}

	// Cancelled subscriptions
	cancelled, err := h.queries.CountSubscriptions(ctx, sqlc.CountSubscriptionsParams{
		MerchantID: pgtype.UUID{Valid: false},
		CustomerID: pgtype.Text{Valid: false},
		Status:     pgtype.Text{String: "cancelled", Valid: true},
	})
	if err != nil {
		h.logger.Error("Failed to count cancelled subscriptions", zap.Error(err))
	} else {
		cancelledCount = cancelled
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"success": true,
		"stats": map[string]interface{}{
			"total_subscriptions":     totalCount,
			"active_subscriptions":    activeCount,
			"paused_subscriptions":    pausedCount,
			"past_due_subscriptions":  pastDueCount,
			"cancelled_subscriptions": cancelledCount,
		},
		"timestamp": timeutil.Now().Format(time.RFC3339),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
