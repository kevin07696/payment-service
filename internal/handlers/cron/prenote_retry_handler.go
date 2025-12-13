package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/epxutil"
	"github.com/kevin07696/payment-service/pkg/timeutil"
	"go.uber.org/zap"
)

// PrenoteRetryHandler handles cron job endpoints for prenote retries
type PrenoteRetryHandler struct {
	queries    sqlc.Querier
	serverPost ports.ServerPostAdapter
	logger     *zap.Logger
	cronSecret string
}

// NewPrenoteRetryHandler creates a new prenote retry cron handler
func NewPrenoteRetryHandler(
	queries sqlc.Querier,
	serverPost ports.ServerPostAdapter,
	logger *zap.Logger,
	cronSecret string,
) *PrenoteRetryHandler {
	return &PrenoteRetryHandler{
		queries:    queries,
		serverPost: serverPost,
		logger:     logger,
		cronSecret: cronSecret,
	}
}

// PrenoteRetryRequest represents the request body for prenote retry
type PrenoteRetryRequest struct {
	BatchSize   *int `json:"batch_size"`   // Optional: defaults to 50
	MaxAttempts *int `json:"max_attempts"` // Optional: defaults to 5
}

// PrenoteRetryResponse represents the response from prenote retry
type PrenoteRetryResponse struct {
	Success      bool     `json:"success"`
	Processed    int      `json:"processed"`
	Succeeded    int      `json:"succeeded"`
	Failed       int      `json:"failed"`
	MaxRetries   int      `json:"max_retries"`
	Errors       []string `json:"errors,omitempty"`
	ProcessedAt  string   `json:"processed_at"`
}

// MaxPrenoteAttempts is the maximum number of prenote retry attempts
const MaxPrenoteAttempts = 5

// RetryPrenotes handles the POST /cron/prenote-retry endpoint
// This endpoint is called by Cloud Scheduler every 5 minutes to retry failed prenotes
func (h *PrenoteRetryHandler) RetryPrenotes(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Prenote retry cron job triggered",
		zap.String("method", r.Method),
		zap.String("remote_addr", r.RemoteAddr),
	)

	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	if !h.authenticateRequest(r) {
		h.logger.Warn("Unauthorized cron request", zap.String("remote_addr", r.RemoteAddr))
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Parse request body
	var req PrenoteRetryRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Failed to parse request body", zap.Error(err))
		}
	}

	// Determine batch size (default 50)
	batchSize := 50
	if req.BatchSize != nil && *req.BatchSize >= 1 && *req.BatchSize <= 200 {
		batchSize = *req.BatchSize
	}

	// Determine max attempts (default 5)
	maxAttempts := MaxPrenoteAttempts
	if req.MaxAttempts != nil && *req.MaxAttempts >= 1 && *req.MaxAttempts <= 10 {
		maxAttempts = *req.MaxAttempts
	}

	// Process prenote retries with request context and timeout
	// Using r.Context() allows graceful shutdown during deployment
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	succeeded, failed, maxRetries, errs := h.processPrenoteRetries(ctx, batchSize, maxAttempts)

	resp := PrenoteRetryResponse{
		Success:     len(errs) == 0,
		Processed:   succeeded + failed + maxRetries,
		Succeeded:   succeeded,
		Failed:      failed,
		MaxRetries:  maxRetries,
		ProcessedAt: timeutil.Now().Format(time.RFC3339),
	}

	if len(errs) > 0 {
		resp.Errors = make([]string, len(errs))
		for i, err := range errs {
			resp.Errors[i] = err.Error()
		}
	}

	h.logger.Info("Prenote retry completed",
		zap.Int("succeeded", succeeded),
		zap.Int("failed", failed),
		zap.Int("max_retries", maxRetries),
		zap.Int("errors", len(errs)),
	)

	w.Header().Set("Content-Type", "application/json")
	if resp.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusPartialContent)
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// processPrenoteRetries finds and retries failed prenotes
func (h *PrenoteRetryHandler) processPrenoteRetries(ctx context.Context, batchSize, maxAttempts int) (succeeded, failed, maxRetries int, errs []error) {
	// Query ACH payment methods needing prenote retry
	paymentMethods, err := h.queries.GetACHNeedingPrenoteRetry(ctx, sqlc.GetACHNeedingPrenoteRetryParams{
		MaxAttempts: pgtype.Int4{Int32: int32(maxAttempts), Valid: true},
		BatchLimit:  int32(batchSize),
	})
	if err != nil {
		h.logger.Error("Failed to query payment methods for prenote retry", zap.Error(err))
		return 0, 0, 0, []error{fmt.Errorf("failed to query payment methods: %w", err)}
	}

	h.logger.Info("Found payment methods needing prenote retry",
		zap.Int("count", len(paymentMethods)),
	)

	for _, pm := range paymentMethods {
		result, processErr := h.processSinglePrenote(ctx, pm, maxAttempts)

		switch result {
		case prenoteResultSuccess:
			succeeded++
		case prenoteResultFailed:
			failed++
			if processErr != nil {
				errs = append(errs, processErr)
			}
		case prenoteResultMaxRetries:
			maxRetries++
		case prenoteResultPermanentFailure:
			maxRetries++ // Count as max retries since account is deactivated
			if processErr != nil {
				errs = append(errs, processErr)
			}
		}
	}

	return succeeded, failed, maxRetries, errs
}

type prenoteResult int

const (
	prenoteResultSuccess prenoteResult = iota
	prenoteResultFailed
	prenoteResultMaxRetries
	prenoteResultPermanentFailure
)

// processSinglePrenote processes a single prenote retry
func (h *PrenoteRetryHandler) processSinglePrenote(ctx context.Context, pm sqlc.GetACHNeedingPrenoteRetryRow, maxAttempts int) (prenoteResult, error) {
	pmID := pm.ID
	currentAttempts := int(pm.PrenoteAttempts.Int32)

	h.logger.Info("Processing prenote retry",
		zap.String("payment_method_id", pmID.String()),
		zap.Int("current_attempts", currentAttempts),
		zap.Int("max_attempts", maxAttempts),
	)

	// Determine prenote type based on account type
	prenoteType := ports.TransactionTypeACHPreNoteDebit
	if pm.AccountType.Valid && strings.ToLower(pm.AccountType.String) == "savings" {
		prenoteType = ports.TransactionTypeACHSavingsPreNoteDebit
	}

	// Build prenote request
	prenoteID := uuid.New()
	prenoteTranNbr := epxutil.UUIDToEPXTranNbr(prenoteID)

	cardEntryMethod := "Z"
	stdEntryClass := "WEB"
	industryType := "E"

	prenoteReq := &ports.ServerPostRequest{
		CustNbr:         pm.CustNbr,
		MerchNbr:        pm.MerchNbr,
		DBAnbr:          pm.DbaNbr,
		TerminalNbr:     pm.TerminalNbr,
		TransactionType: prenoteType,
		Amount:          "0.00",
		TranNbr:         prenoteTranNbr,
		TranGroup:       "PRENOTE",
		AuthGUID:        pm.Bric,
		PaymentType:     ports.PaymentMethodTypeACH,
		CardEntryMethod: &cardEntryMethod,
		StdEntryClass:   &stdEntryClass,
		IndustryType:    &industryType,
	}

	// Send prenote to EPX
	prenoteResp, err := h.serverPost.ProcessTransaction(ctx, prenoteReq)

	if err != nil {
		// Network/timeout error - check if transient
		if isTransientError(err) {
			return h.handleTransientFailure(ctx, pmID, currentAttempts, maxAttempts, err)
		}
		// Non-transient error - permanent failure
		return h.handlePermanentFailure(ctx, pmID, "request_error", err.Error())
	}

	// Check EPX response
	if prenoteResp.AuthResp != "00" {
		// EPX rejected - permanent failure (bad account, invalid BRIC, etc.)
		return h.handlePermanentFailure(ctx, pmID, prenoteResp.AuthResp, prenoteResp.AuthRespText)
	}

	// Success - create transaction record and update status
	return h.handleSuccess(ctx, pm, prenoteID, prenoteTranNbr, prenoteResp)
}

// handleSuccess handles successful prenote
func (h *PrenoteRetryHandler) handleSuccess(ctx context.Context, pm sqlc.GetACHNeedingPrenoteRetryRow, prenoteID uuid.UUID, prenoteTranNbr string, prenoteResp *ports.ServerPostResponse) (prenoteResult, error) {
	// Create prenote transaction record
	_, err := h.queries.CreateTransaction(ctx, sqlc.CreateTransactionParams{
		ID:                prenoteID,
		MerchantID:        pm.MerchantID,
		CustomerID:        pgtype.Text{String: pm.CustomerID, Valid: true},
		AmountCents:       0,
		Currency:          "USD",
		Type:              "PRENOTE",
		PaymentMethodType: "ach",
		PaymentMethodID:   pgtype.UUID{Bytes: pm.ID, Valid: true},
		TranNbr:           pgtype.Text{String: prenoteTranNbr, Valid: true},
		AuthGuid:          pgtype.Text{String: prenoteResp.AuthGUID, Valid: true},
		AuthResp:          pgtype.Text{String: prenoteResp.AuthResp, Valid: true},
		AuthCode:          pgtype.Text{String: prenoteResp.AuthCode, Valid: true},
		AuthCardType:      pgtype.Text{},
		Metadata:          []byte("{}"),
		ParentTransactionID: pgtype.UUID{},
		ProcessedAt:       pgtype.Timestamptz{},
	})
	if err != nil {
		h.logger.Error("Failed to create prenote transaction", zap.Error(err))
		return prenoteResultFailed, fmt.Errorf("failed to create transaction for %s: %w", pm.ID.String(), err)
	}

	// Update prenote status to success
	err = h.queries.UpdatePrenoteStatusSuccess(ctx, pm.ID)
	if err != nil {
		h.logger.Error("Failed to update prenote status", zap.Error(err))
		return prenoteResultFailed, fmt.Errorf("failed to update status for %s: %w", pm.ID.String(), err)
	}

	h.logger.Info("Prenote sent successfully",
		zap.String("payment_method_id", pm.ID.String()),
		zap.String("prenote_id", prenoteID.String()),
		zap.String("auth_resp", prenoteResp.AuthResp),
	)

	return prenoteResultSuccess, nil
}

// handleTransientFailure handles transient errors with backoff
func (h *PrenoteRetryHandler) handleTransientFailure(ctx context.Context, pmID uuid.UUID, currentAttempts, maxAttempts int, err error) (prenoteResult, error) {
	nextAttempt := currentAttempts + 1

	if nextAttempt >= maxAttempts {
		// Max retries exceeded
		h.logger.Warn("Prenote max retries exceeded",
			zap.String("payment_method_id", pmID.String()),
			zap.Int("attempts", nextAttempt),
		)

		updateErr := h.queries.UpdatePrenoteStatusMaxRetries(ctx, pmID)
		if updateErr != nil {
			h.logger.Error("Failed to update max retries status", zap.Error(updateErr))
			return prenoteResultMaxRetries, fmt.Errorf("failed to update max retries for %s: %w", pmID.String(), updateErr)
		}
		return prenoteResultMaxRetries, nil
	}

	// Schedule retry with backoff
	nextRetryAt := calculateNextRetryTime(nextAttempt)

	h.logger.Info("Scheduling prenote retry",
		zap.String("payment_method_id", pmID.String()),
		zap.Int("attempt", nextAttempt),
		zap.Time("next_retry_at", nextRetryAt),
		zap.Error(err),
	)

	updateErr := h.queries.UpdatePrenoteStatusFailed(ctx, sqlc.UpdatePrenoteStatusFailedParams{
		ID:              pmID,
		NextRetryAt:     pgtype.Timestamptz{Time: nextRetryAt, Valid: true},
	})
	if updateErr != nil {
		h.logger.Error("Failed to update failed status", zap.Error(updateErr))
		return prenoteResultFailed, fmt.Errorf("failed to schedule retry for %s: %w", pmID.String(), updateErr)
	}

	return prenoteResultFailed, nil
}

// handlePermanentFailure handles non-retryable errors
func (h *PrenoteRetryHandler) handlePermanentFailure(ctx context.Context, pmID uuid.UUID, errorCode, errorMessage string) (prenoteResult, error) {
	h.logger.Warn("Prenote permanent failure - deactivating payment method",
		zap.String("payment_method_id", pmID.String()),
		zap.String("error_code", errorCode),
		zap.String("error_message", errorMessage),
	)

	statusReason := fmt.Sprintf("prenote_error: %s - %s", errorCode, errorMessage)

	err := h.queries.UpdatePrenoteStatusPermanentFailure(ctx, sqlc.UpdatePrenoteStatusPermanentFailureParams{
		ID:           pmID,
		StatusReason: pgtype.Text{String: statusReason, Valid: true},
	})
	if err != nil {
		h.logger.Error("Failed to update permanent failure status", zap.Error(err))
		return prenoteResultPermanentFailure, fmt.Errorf("failed to deactivate %s: %w", pmID.String(), err)
	}

	return prenoteResultPermanentFailure, nil
}

// calculateNextRetryTime calculates the next retry time with exponential backoff and jitter
// Backoff schedule: 5min -> 15min -> 30min -> 1hr -> 2hr
func calculateNextRetryTime(attempts int) time.Time {
	delays := []time.Duration{
		5 * time.Minute,
		15 * time.Minute,
		30 * time.Minute,
		1 * time.Hour,
		2 * time.Hour,
	}

	idx := attempts - 1
	if idx >= len(delays) {
		idx = len(delays) - 1
	}
	if idx < 0 {
		idx = 0
	}

	delay := delays[idx]

	// Add jitter (±10%)
	jitterRange := float64(delay) * 0.1
	jitter := time.Duration(jitterRange * (rand.Float64()*2 - 1))

	return timeutil.Now().Add(delay + jitter)
}

// isTransientError checks if an error is transient (network issues) vs permanent
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "504")
}

// authenticateRequest verifies the cron request is authorized
func (h *PrenoteRetryHandler) authenticateRequest(r *http.Request) bool {
	cronSecret := r.Header.Get("X-Cron-Secret")
	if cronSecret != "" && cronSecret == h.cronSecret {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "Bearer "+h.cronSecret {
		return true
	}

	// Note: Google Cloud Scheduler OIDC token validation can be added here
	// for production environments that require additional security.
	// The X-Cron-Secret header is the primary authentication method.

	return false
}

// respondError sends an error response
func (h *PrenoteRetryHandler) respondError(w http.ResponseWriter, statusCode int, message string) {
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

// Stats handles GET /cron/prenote/stats for monitoring prenote retry statistics
func (h *PrenoteRetryHandler) Stats(w http.ResponseWriter, r *http.Request) {
	if !h.authenticateRequest(r) {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx := r.Context()

	// Get prenote status counts
	statusCounts, err := h.queries.CountPrenotesByStatus(ctx)
	if err != nil {
		h.logger.Error("Failed to query prenote stats", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to query stats")
		return
	}

	stats := make(map[string]int64)
	for _, sc := range statusCounts {
		if sc.PrenoteStatus.Valid {
			stats[sc.PrenoteStatus.String] = sc.Count
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := map[string]interface{}{
		"success":   true,
		"stats":     stats,
		"timestamp": timeutil.Now().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode stats response", zap.Error(err))
	}
}
