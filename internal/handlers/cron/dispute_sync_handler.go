package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/services/webhook"
	"go.uber.org/zap"
)

// webhookJob represents a webhook delivery job
type webhookJob struct {
	ctx        context.Context
	merchantID string
	eventType  string
	event      *webhook.WebhookEvent
}

// DisputeSyncHandler handles cron job endpoints for dispute synchronization
type DisputeSyncHandler struct {
	merchantReporting ports.MerchantReportingAdapter
	db                *database.PostgreSQLAdapter
	webhookService    *webhook.WebhookDeliveryService
	logger            *zap.Logger
	webhookJobs       chan webhookJob // Buffered channel for webhook delivery jobs
	stopCh            chan struct{}   // Channel to signal worker shutdown
	workerCount       int             // Number of webhook delivery workers
}

// NewDisputeSyncHandler creates a new dispute sync cron handler
func NewDisputeSyncHandler(
	merchantReporting ports.MerchantReportingAdapter,
	db *database.PostgreSQLAdapter,
	webhookService *webhook.WebhookDeliveryService,
	logger *zap.Logger,
) *DisputeSyncHandler {
	const workerCount = 5 // Max 5 concurrent webhook deliveries
	const queueSize = 100 // Max 100 pending webhooks in queue

	h := &DisputeSyncHandler{
		merchantReporting: merchantReporting,
		db:                db,
		webhookService:    webhookService,
		logger:            logger,
		webhookJobs:       make(chan webhookJob, queueSize),
		stopCh:            make(chan struct{}),
		workerCount:       workerCount,
	}

	// Start webhook delivery workers
	h.startWebhookWorkers()

	return h
}

// startWebhookWorkers starts fixed number of worker goroutines for webhook delivery
func (h *DisputeSyncHandler) startWebhookWorkers() {
	for i := 0; i < h.workerCount; i++ {
		go func(workerID int) {
			h.logger.Debug("Webhook delivery worker started", zap.Int("worker_id", workerID))

			for {
				select {
				case <-h.stopCh:
					h.logger.Info("Webhook delivery worker stopped", zap.Int("worker_id", workerID))
					return
				case job := <-h.webhookJobs:
					// Deliver webhook using internal event type
					if err := h.webhookService.DeliverInternalEvent(job.ctx, job.event); err != nil {
						h.logger.Error("Failed to deliver chargeback webhook",
							zap.Int("worker_id", workerID),
							zap.String("event_type", job.eventType),
							zap.String("merchant_id", job.merchantID),
							zap.Error(err),
						)
					}
				}
			}
		}(i)
	}

	h.logger.Info("Webhook delivery worker pool started", zap.Int("worker_count", h.workerCount))
}

// Shutdown gracefully stops webhook delivery workers
func (h *DisputeSyncHandler) Shutdown() {
	h.logger.Info("Shutting down dispute sync handler")
	close(h.stopCh)
	// Note: Pending jobs in the channel will be lost. Consider draining if needed.
}

// SyncDisputesRequest represents the request body for dispute sync
type SyncDisputesRequest struct {
	MerchantID *string `json:"merchant_id"` // Optional: sync for specific agent, otherwise sync all
	FromDate   *string `json:"from_date"`   // Optional: ISO date string
	ToDate     *string `json:"to_date"`     // Optional: ISO date string
	DaysBack   *int    `json:"days_back"`   // Optional: sync last N days, defaults to 7
}

// SyncDisputesResponse represents the response from dispute sync
type SyncDisputesResponse struct {
	Success            bool     `json:"success"`
	AgentsProcessed    int      `json:"agents_processed"`
	TotalDisputes      int      `json:"total_disputes"`
	NewChargebacks     int      `json:"new_chargebacks"`
	UpdatedChargebacks int      `json:"updated_chargebacks"`
	Errors             []string `json:"errors,omitempty"`
	ProcessedAt        string   `json:"processed_at"`
}

// SyncDisputes handles the POST /cron/sync-disputes endpoint
// This endpoint is called by Cloud Scheduler to sync disputes from North API
func (h *DisputeSyncHandler) SyncDisputes(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Dispute sync cron job triggered",
		zap.String("method", r.Method),
		zap.String("remote_addr", r.RemoteAddr),
	)

	// Verify request method
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "only POST method is allowed")
		return
	}

	// Parse request body
	var req SyncDisputesRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.logger.Warn("Failed to parse request body", zap.Error(err))
			// Continue with defaults
		}
	}

	// Determine date range
	var fromDate, toDate *time.Time
	daysBack := 7
	if req.DaysBack != nil && *req.DaysBack > 0 && *req.DaysBack <= 90 {
		daysBack = *req.DaysBack
	}

	if req.FromDate != nil {
		parsed, err := time.Parse("2006-01-02", *req.FromDate)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid from_date format: %v", err))
			return
		}
		fromDate = &parsed
	} else {
		// Default to last N days
		d := time.Now().AddDate(0, 0, -daysBack)
		fromDate = &d
	}

	if req.ToDate != nil {
		parsed, err := time.Parse("2006-01-02", *req.ToDate)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid to_date format: %v", err))
			return
		}
		toDate = &parsed
	}

	// Process dispute sync with request context and timeout
	// Using r.Context() allows graceful shutdown during deployment
	// 10 minute timeout for external API calls per merchant
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	// Get agents to sync
	var agents []sqlc.Merchant
	var err error

	if req.MerchantID != nil {
		// Sync specific agent
		agent, err := h.db.Queries().GetMerchantBySlug(ctx, *req.MerchantID)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("agent not found: %v", err))
			return
		}
		agents = []sqlc.Merchant{agent}
	} else {
		// Sync all active agents
		agents, err = h.db.Queries().ListActiveMerchants(ctx)
		if err != nil {
			h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list agents: %v", err))
			return
		}
	}

	// Process each agent
	resp := SyncDisputesResponse{
		Success:         true,
		AgentsProcessed: len(agents),
		ProcessedAt:     time.Now().Format(time.RFC3339),
	}

	for _, agent := range agents {
		newCount, updatedCount, err := h.syncAgentDisputes(ctx, &agent, fromDate, toDate)
		if err != nil {
			resp.Success = false
			resp.Errors = append(resp.Errors, fmt.Sprintf("agent %s: %v", agent.ID.String(), err))
			h.logger.Error("Failed to sync disputes for agent",
				zap.String("merchant_id", agent.ID.String()),
				zap.Error(err),
			)
			continue
		}

		resp.NewChargebacks += newCount
		resp.UpdatedChargebacks += updatedCount
		h.logger.Info("Synced disputes for agent",
			zap.String("merchant_id", agent.ID.String()),
			zap.Int("new", newCount),
			zap.Int("updated", updatedCount),
		)
	}

	// Respond with JSON
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

// syncAgentDisputes syncs disputes for a single agent
func (h *DisputeSyncHandler) syncAgentDisputes(ctx context.Context, agent *sqlc.Merchant, fromDate, toDate *time.Time) (newCount, updatedCount int, err error) {
	// Call North API to search disputes
	searchReq := &ports.DisputeSearchRequest{
		MerchantID: agent.ID.String(),
		FromDate:   fromDate,
		ToDate:     toDate,
	}

	searchResp, err := h.merchantReporting.SearchDisputes(ctx, searchReq)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to search disputes: %w", err)
	}

	h.logger.Info("Retrieved disputes from North API",
		zap.String("merchant_id", agent.ID.String()),
		zap.Int("total_disputes", searchResp.TotalDisputes),
	)

	// Process each dispute
	for _, dispute := range searchResp.Disputes {
		isNew, err := h.upsertChargeback(ctx, agent.ID.String(), dispute)
		if err != nil {
			h.logger.Error("Failed to upsert chargeback",
				zap.String("case_number", dispute.CaseNumber),
				zap.Error(err),
			)
			// Continue processing other disputes
			continue
		}

		if isNew {
			newCount++
		} else {
			updatedCount++
		}
	}

	return newCount, updatedCount, nil
}

// upsertChargeback inserts or updates a chargeback record
func (h *DisputeSyncHandler) upsertChargeback(ctx context.Context, merchantID string, dispute *ports.Dispute) (isNew bool, err error) {
	// Check if chargeback already exists
	existing, err := h.db.Queries().GetChargebackByCaseNumber(ctx, sqlc.GetChargebackByCaseNumberParams{
		MerchantID: merchantID,
		CaseNumber: dispute.CaseNumber,
	})

	if err != nil {
		// Chargeback doesn't exist - create new one
		return true, h.createChargeback(ctx, merchantID, dispute)
	}

	// Chargeback exists - update it
	return false, h.updateChargeback(ctx, &existing, dispute)
}

// createChargeback creates a new chargeback record and triggers webhook
func (h *DisputeSyncHandler) createChargeback(ctx context.Context, merchantID string, dispute *ports.Dispute) error {
	// Parse dates
	disputeDate, err := time.Parse("2006-01-02", dispute.DisputeDate)
	if err != nil {
		h.logger.Warn("Failed to parse dispute date", zap.String("date", dispute.DisputeDate))
		disputeDate = time.Now()
	}

	chargebackDate, err := time.Parse("2006-01-02", dispute.ChargebackDate)
	if err != nil {
		h.logger.Warn("Failed to parse chargeback date", zap.String("date", dispute.ChargebackDate))
		chargebackDate = time.Now()
	}

	// Link to the specific transaction being disputed
	// TransactionNumber from North API matches our tran_nbr field
	var transactionID uuid.UUID = uuid.Nil

	if dispute.TransactionNumber != "" {
		// Get transaction ID from tran_nbr
		tx, err := h.db.Queries().GetTransactionByTranNbr(ctx, pgtype.Text{
			String: dispute.TransactionNumber,
			Valid:  true,
		})
		if err != nil {
			h.logger.Warn("Could not find transaction for chargeback",
				zap.String("tran_nbr", dispute.TransactionNumber),
				zap.String("case_number", dispute.CaseNumber),
				zap.Error(err),
			)
		} else {
			transactionID = tx.ID
		}
	} else {
		h.logger.Warn("Dispute missing transaction number",
			zap.String("case_number", dispute.CaseNumber),
		)
	}

	// Marshal dispute as raw_data
	rawData, err := json.Marshal(dispute)
	if err != nil {
		h.logger.Warn("Failed to marshal dispute data", zap.Error(err))
		rawData = []byte("{}")
	}

	chargebackID := uuid.New()
	params := sqlc.CreateChargebackParams{
		ID:                chargebackID,
		TransactionID:     transactionID,
		MerchantID:        merchantID,
		CustomerID:        pgtype.Text{Valid: false}, // Not available from North API
		CaseNumber:        dispute.CaseNumber,
		DisputeDate:       disputeDate,
		ChargebackDate:    chargebackDate,
		ChargebackAmount:  fmt.Sprintf("%.2f", dispute.ChargebackAmount),
		Currency:          "USD", // Default to USD
		ReasonCode:        dispute.ReasonCode,
		ReasonDescription: pgtype.Text{String: dispute.ReasonDescription, Valid: dispute.ReasonDescription != ""},
		Status:            mapDisputeStatus(dispute.Status),
		RespondByDate:     pgtype.Date{Valid: false}, // Calculate from chargeback_date + grace period if needed
		EvidenceFiles:     []string{},                // Empty array for new chargebacks
		ResponseNotes:     pgtype.Text{Valid: false},
		InternalNotes:     pgtype.Text{Valid: false},
		RawData:           rawData,
	}

	chargeback, err := h.db.Queries().CreateChargeback(ctx, params)
	if err != nil {
		return fmt.Errorf("creating chargeback for case %s: %w", dispute.CaseNumber, err)
	}

	// Trigger webhook notification for new chargeback
	if h.webhookService != nil {
		h.triggerChargebackWebhook(ctx, merchantID, "chargeback.created", &chargeback)
	}

	return nil
}

// updateChargeback updates an existing chargeback record and triggers webhook
func (h *DisputeSyncHandler) updateChargeback(ctx context.Context, existing *sqlc.Chargeback, dispute *ports.Dispute) error {
	// Parse dates with fallback to existing values on error
	disputeDate, err := time.Parse("2006-01-02", dispute.DisputeDate)
	if err != nil {
		h.logger.Warn("Failed to parse dispute date in update, using existing",
			zap.String("date", dispute.DisputeDate),
			zap.Error(err))
		disputeDate = existing.DisputeDate
	}

	chargebackDate, err := time.Parse("2006-01-02", dispute.ChargebackDate)
	if err != nil {
		h.logger.Warn("Failed to parse chargeback date in update, using existing",
			zap.String("date", dispute.ChargebackDate),
			zap.Error(err))
		chargebackDate = existing.ChargebackDate
	}

	params := sqlc.UpdateChargebackStatusParams{
		ID:                existing.ID,
		Status:            mapDisputeStatus(dispute.Status),
		DisputeDate:       disputeDate,
		ChargebackDate:    chargebackDate,
		ChargebackAmount:  fmt.Sprintf("%.2f", dispute.ChargebackAmount),
		ReasonCode:        dispute.ReasonCode,
		ReasonDescription: pgtype.Text{String: dispute.ReasonDescription, Valid: dispute.ReasonDescription != ""},
	}

	chargeback, err := h.db.Queries().UpdateChargebackStatus(ctx, params)
	if err != nil {
		return fmt.Errorf("updating chargeback %s: %w", existing.ID.String(), err)
	}

	// Trigger webhook notification for updated chargeback
	if h.webhookService != nil {
		h.triggerChargebackWebhook(ctx, existing.MerchantID, "chargeback.updated", &chargeback)
	}

	return nil
}

// mapDisputeStatus maps North API status to our domain status
func mapDisputeStatus(northStatus string) string {
	switch northStatus {
	case "NEW":
		return "new"
	case "PENDING":
		return "pending"
	case "RESPONDED":
		return "responded"
	case "WON":
		return "won"
	case "LOST":
		return "lost"
	default:
		return "new"
	}
}

// respondError sends an error response
func (h *DisputeSyncHandler) respondError(w http.ResponseWriter, statusCode int, message string) {
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

// triggerChargebackWebhook sends a webhook notification for chargeback events
func (h *DisputeSyncHandler) triggerChargebackWebhook(ctx context.Context, merchantID, eventType string, chargeback *sqlc.Chargeback) {
	// Build webhook event data
	eventData := map[string]interface{}{
		"chargeback_id":      chargeback.ID.String(),
		"case_number":        chargeback.CaseNumber,
		"transaction_id":     chargeback.TransactionID.String(),
		"status":             chargeback.Status,
		"amount":             chargeback.ChargebackAmount,
		"currency":           chargeback.Currency,
		"reason_code":        chargeback.ReasonCode,
		"reason_description": chargeback.ReasonDescription.String,
		"dispute_date":       chargeback.DisputeDate.Format("2006-01-02"),
		"chargeback_date":    chargeback.ChargebackDate.Format("2006-01-02"),
	}

	if chargeback.CustomerID.Valid {
		eventData["customer_id"] = chargeback.CustomerID.String
	}

	event := &webhook.WebhookEvent{
		EventType:  eventType,
		MerchantID: merchantID,
		Data:       eventData,
		Timestamp:  time.Now(),
	}

	// Send webhook job to worker pool (non-blocking with select)
	// If queue is full, drop the webhook and log error
	job := webhookJob{
		ctx:        context.Background(),
		merchantID: merchantID,
		eventType:  eventType,
		event:      event,
	}

	select {
	case h.webhookJobs <- job:
		// Successfully queued for delivery
		h.logger.Debug("Webhook job queued",
			zap.String("event_type", eventType),
			zap.String("merchant_id", merchantID),
		)
	default:
		// Queue is full - drop webhook and log critical error
		h.logger.Error("Webhook queue full - dropping webhook",
			zap.String("event_type", eventType),
			zap.String("merchant_id", merchantID),
			zap.String("case_number", chargeback.CaseNumber),
			zap.Int("queue_capacity", cap(h.webhookJobs)),
			zap.String("recommendation", "Increase queue size or worker count"),
		)
	}
}
