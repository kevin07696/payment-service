package cron

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/pkg/timeutil"
	"go.uber.org/zap"
)

// RateLimitCleanupHandler handles cron job for cleaning up old rate limit buckets
type RateLimitCleanupHandler struct {
	queries sqlc.Querier
	logger  *zap.Logger
}

// NewRateLimitCleanupHandler creates a new rate limit cleanup cron handler
func NewRateLimitCleanupHandler(
	queries sqlc.Querier,
	logger *zap.Logger,
) *RateLimitCleanupHandler {
	return &RateLimitCleanupHandler{
		queries: queries,
		logger:  logger,
	}
}

// CleanupResponse represents the response from rate limit cleanup
type RateLimitCleanupResponse struct {
	Success     bool   `json:"success"`
	DeletedRows int64  `json:"deleted_rows"`
	ProcessedAt string `json:"processed_at"`
	Message     string `json:"message"`
}

// CleanupRateLimitBuckets handles the POST /cron/cleanup-rate-limits endpoint
// This endpoint is called by Cloud Scheduler every 5 minutes to clean up old rate limit buckets
// Keeps last 1 hour of data for analytics, deletes older entries
func (h *RateLimitCleanupHandler) CleanupRateLimitBuckets(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Rate limit cleanup cron job triggered",
		zap.String("method", r.Method),
		zap.String("remote_addr", r.RemoteAddr),
	)

	// Verify request method
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// Note: Authentication is handled by cronAuthMiddleware in main.go

	// Execute cleanup with timeout
	// Short timeout - UNLOGGED table deletes are fast
	// Using r.Context() allows graceful shutdown during deployment
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	h.logger.Info("Starting rate limit bucket cleanup (deletes entries > 1 hour old)")

	// Execute cleanup (query automatically filters by last_refill < NOW() - INTERVAL '1 hour')
	err := h.queries.CleanupOldRateLimitBuckets(ctx)
	if err != nil {
		h.logger.Error("Failed to cleanup old rate limit buckets",
			zap.Error(err),
		)
		h.respondError(w, http.StatusInternalServerError, "cleanup failed")
		return
	}

	h.logger.Info("Rate limit bucket cleanup completed successfully")

	// Return success response
	// Note: pgx doesn't return row count for DELETE without RETURNING clause
	// This is acceptable - cleanup is fire-and-forget
	h.respondSuccess(w, RateLimitCleanupResponse{
		Success:     true,
		DeletedRows: -1, // Unknown - not critical for cleanup operation
		ProcessedAt: timeutil.Now().Format(time.RFC3339),
		Message:     "Deleted rate limit buckets older than 1 hour",
	})
}

// HealthCheck returns the health status of the rate limit cleanup handler
func (h *RateLimitCleanupHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "rate-limit-cleanup-cron",
		"time":    timeutil.Now().Format(time.RFC3339),
	}); err != nil {
		h.logger.Error("Failed to encode health response", zap.Error(err))
	}
}

// Stats returns statistics about rate limit buckets
// Note: Authentication is handled by cronAuthMiddleware in main.go
func (h *RateLimitCleanupHandler) Stats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"table_type":           "UNLOGGED (2-3x faster writes, no WAL)",
		"cleanup_interval":     "5 minutes",
		"retention_period":     "1 hour",
		"last_check":           timeutil.Now().Format(time.RFC3339),
		"recommended_schedule": "every 5 minutes",
		"distributed":          true,
		"cache_level":          "L2 (PostgreSQL UNLOGGED)",
	}); err != nil {
		h.logger.Error("Failed to encode stats response", zap.Error(err))
	}
}

// Helper methods

func (h *RateLimitCleanupHandler) respondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	}); err != nil {
		h.logger.Error("Failed to encode error response", zap.Error(err))
	}
}

func (h *RateLimitCleanupHandler) respondSuccess(w http.ResponseWriter, data RateLimitCleanupResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode success response", zap.Error(err))
	}
}
