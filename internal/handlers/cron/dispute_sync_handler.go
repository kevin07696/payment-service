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
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/services/webhook"
	"go.uber.org/zap"
)

// DisputeSyncHandler handles cron job endpoints for dispute synchronization
type DisputeSyncHandler struct {
	merchantReporting adapterports.MerchantReportingAdapter
	db                *database.PostgreSQLAdapter
	webhookService    *webhook.WebhookDeliveryService
	logger            *zap.Logger
	cronSecret        string
}

// NewDisputeSyncHandler creates a new dispute sync cron handler
func NewDisputeSyncHandler(
	merchantReporting adapterports.MerchantReportingAdapter,
	db *database.PostgreSQLAdapter,
	webhookService *webhook.WebhookDeliveryService,
	logger *zap.Logger,
	cronSecret string,
) *DisputeSyncHandler {
	return &DisputeSyncHandler{
		merchantReporting: merchantReporting,
		db:                db,
		webhookService:    webhookService,
		logger:            logger,
		cronSecret:        cronSecret,
	}
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

	// Authenticate the request
	if !h.authenticateRequest(r) {
		h.logger.Warn("Unauthorized cron request",
			zap.String("remote_addr", r.RemoteAddr),
		)
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
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

	ctx := context.Background()

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
	searchReq := &adapterports.DisputeSearchRequest{
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
func (h *DisputeSyncHandler) upsertChargeback(ctx context.Context, agentID string, dispute *adapterports.Dispute) (isNew bool, err error) {
	// Check if chargeback already exists
	existing, err := h.db.Queries().GetChargebackByCaseNumber(ctx, sqlc.GetChargebackByCaseNumberParams{
		AgentID:    agentID,
		CaseNumber: dispute.CaseNumber,
	})

	if err != nil {
		// Chargeback doesn't exist - create new one
		return true, h.createChargeback(ctx, agentID, dispute)
	}

	// Chargeback exists - update it
	return false, h.updateChargeback(ctx, &existing, dispute)
}

// createChargeback creates a new chargeback record and triggers webhook
func (h *DisputeSyncHandler) createChargeback(ctx context.Context, agentID string, dispute *adapterports.Dispute) error {
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

	// Find matching transaction by transaction number
	var groupID pgtype.UUID
	if dispute.TransactionNumber != "" {
		// Try to find the transaction group
		// This would require a query to find transaction by auth response or other identifiers
		// For now, we'll leave it NULL and allow manual linking later
		groupID = pgtype.UUID{Valid: false}
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
		GroupID:           groupID,
		AgentID:           agentID,
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
		return err
	}

	// Trigger webhook notification for new chargeback
	if h.webhookService != nil {
		h.triggerChargebackWebhook(ctx, agentID, "chargeback.created", &chargeback)
	}

	return nil
}

// updateChargeback updates an existing chargeback record and triggers webhook
func (h *DisputeSyncHandler) updateChargeback(ctx context.Context, existing *sqlc.Chargeback, dispute *adapterports.Dispute) error {
	// Parse dates
	disputeDate, _ := time.Parse("2006-01-02", dispute.DisputeDate)
	chargebackDate, _ := time.Parse("2006-01-02", dispute.ChargebackDate)

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
		return err
	}

	// Trigger webhook notification for updated chargeback
	if h.webhookService != nil {
		h.triggerChargebackWebhook(ctx, existing.AgentID, "chargeback.updated", &chargeback)
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

// authenticateRequest verifies the cron request is authorized
func (h *DisputeSyncHandler) authenticateRequest(r *http.Request) bool {
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
func (h *DisputeSyncHandler) triggerChargebackWebhook(ctx context.Context, agentID, eventType string, chargeback *sqlc.Chargeback) {
	// Build webhook event data
	eventData := map[string]interface{}{
		"chargeback_id":      chargeback.ID.String(),
		"case_number":        chargeback.CaseNumber,
		"status":             chargeback.Status,
		"amount":             chargeback.ChargebackAmount,
		"currency":           chargeback.Currency,
		"reason_code":        chargeback.ReasonCode,
		"reason_description": chargeback.ReasonDescription.String,
		"dispute_date":       chargeback.DisputeDate.Format("2006-01-02"),
		"chargeback_date":    chargeback.ChargebackDate.Format("2006-01-02"),
	}

	if chargeback.GroupID.Valid {
		eventData["transaction_id"] = chargeback.GroupID.Bytes
	}

	if chargeback.CustomerID.Valid {
		eventData["customer_id"] = chargeback.CustomerID.String
	}

	event := &webhook.WebhookEvent{
		EventType: eventType,
		AgentID:   agentID,
		Data:      eventData,
		Timestamp: time.Now(),
	}

	// Deliver webhook asynchronously (don't block cron job)
	go func() {
		if err := h.webhookService.DeliverEvent(context.Background(), event); err != nil {
			h.logger.Error("Failed to deliver chargeback webhook",
				zap.String("event_type", eventType),
				zap.String("merchant_id", agentID),
				zap.String("case_number", chargeback.CaseNumber),
				zap.Error(err),
			)
		}
	}()
}
