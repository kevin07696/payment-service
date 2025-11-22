package cron

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/pkg/timeutil"
	"go.uber.org/zap"
)

// AuditCleanupHandler handles cron job for cleaning up old audit logs
type AuditCleanupHandler struct {
	queries    sqlc.Querier
	logger     *zap.Logger
	cronSecret string // Secret token for authenticating cron requests
}

// NewAuditCleanupHandler creates a new audit cleanup cron handler
func NewAuditCleanupHandler(
	queries sqlc.Querier,
	logger *zap.Logger,
	cronSecret string,
) *AuditCleanupHandler {
	return &AuditCleanupHandler{
		queries:    queries,
		logger:     logger,
		cronSecret: cronSecret,
	}
}

// CleanupRequest represents the request body for audit log cleanup
type CleanupRequest struct {
	RetentionDays *int `json:"retention_days"` // Optional: defaults to 90 days
}

// CleanupResponse represents the response from audit log cleanup
type CleanupResponse struct {
	Success     bool   `json:"success"`
	DeletedRows int64  `json:"deleted_rows"`
	CutoffDate  string `json:"cutoff_date"`
	ProcessedAt string `json:"processed_at"`
}

// CleanupAuditLogs handles the POST /cron/cleanup-audit-logs endpoint
// This endpoint is called by Cloud Scheduler to clean up old audit logs
// Default retention period: 90 days (configurable)
func (h *AuditCleanupHandler) CleanupAuditLogs(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Audit cleanup cron job triggered",
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
	var req CleanupRequest
	retentionDays := 90 // Default retention: 90 days (PCI DSS recommends 90-365 days)

	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Failed to parse request body, using defaults",
				zap.Error(err),
			)
		} else if req.RetentionDays != nil && *req.RetentionDays > 0 {
			retentionDays = *req.RetentionDays
		}
	}

	// Calculate cutoff date (retention period ago from now)
	cutoffDate := timeutil.Now().AddDate(0, 0, -retentionDays)

	h.logger.Info("Starting audit log cleanup",
		zap.Int("retention_days", retentionDays),
		zap.Time("cutoff_date", cutoffDate),
	)

	// Execute cleanup with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Convert time.Time to pgtype.Timestamp
	pgCutoffDate := pgtype.Timestamp{
		Time:  cutoffDate,
		Valid: true,
	}

	result, err := h.queries.DeleteOldAuditLogs(ctx, pgCutoffDate)
	if err != nil {
		h.logger.Error("Failed to delete old audit logs",
			zap.Error(err),
			zap.Time("cutoff_date", cutoffDate),
		)
		h.respondError(w, http.StatusInternalServerError, "cleanup failed")
		return
	}

	deletedRows := result.RowsAffected()

	h.logger.Info("Audit log cleanup completed successfully",
		zap.Int64("deleted_rows", deletedRows),
		zap.Int("retention_days", retentionDays),
		zap.Time("cutoff_date", cutoffDate),
	)

	// Return success response
	h.respondSuccess(w, CleanupResponse{
		Success:     true,
		DeletedRows: deletedRows,
		CutoffDate:  cutoffDate.Format(time.RFC3339),
		ProcessedAt: timeutil.Now().Format(time.RFC3339),
	})
}

// HealthCheck returns the health status of the audit cleanup handler
func (h *AuditCleanupHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "audit-cleanup-cron",
		"time":    timeutil.Now().Format(time.RFC3339),
	})
}

// Stats returns statistics about audit log cleanup
func (h *AuditCleanupHandler) Stats(w http.ResponseWriter, r *http.Request) {
	// Authenticate the request
	if !h.authenticateRequest(r) {
		h.logger.Warn("Unauthorized stats request",
			zap.String("remote_addr", r.RemoteAddr),
		)
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get total audit log count
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := h.queries.CountAuditLogs(ctx, sqlc.CountAuditLogsParams{})
	if err != nil {
		h.logger.Error("Failed to count audit logs",
			zap.Error(err),
		)
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve stats")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_audit_logs":     count,
		"default_retention":    90,
		"retention_unit":       "days",
		"last_check":           timeutil.Now().Format(time.RFC3339),
		"recommended_schedule": "daily at 2 AM UTC",
	})
}

// Helper methods

func (h *AuditCleanupHandler) authenticateRequest(r *http.Request) bool {
	// Check Authorization header for Bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	expectedToken := "Bearer " + h.cronSecret
	return authHeader == expectedToken
}

func (h *AuditCleanupHandler) respondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

func (h *AuditCleanupHandler) respondSuccess(w http.ResponseWriter, data CleanupResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}
