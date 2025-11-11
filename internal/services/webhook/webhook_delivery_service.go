package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/db/sqlc"
)

// DatabaseAdapter defines the interface for database operations
type DatabaseAdapter interface {
	Queries() sqlc.Querier
}

// WebhookDeliveryService handles webhook delivery to merchant endpoints
type WebhookDeliveryService struct {
	db         DatabaseAdapter
	httpClient *http.Client
	logger     *zap.Logger
}

// WebhookEvent represents an event to be sent via webhook
type WebhookEvent struct {
	EventType string                 `json:"event_type"`
	AgentID   string                 `json:"agent_id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewWebhookDeliveryService creates a new webhook delivery service
func NewWebhookDeliveryService(db DatabaseAdapter, httpClient *http.Client, logger *zap.Logger) *WebhookDeliveryService {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	return &WebhookDeliveryService{
		db:         db,
		httpClient: httpClient,
		logger:     logger,
	}
}

// DeliverEvent delivers a webhook event to all subscribed endpoints
func (s *WebhookDeliveryService) DeliverEvent(ctx context.Context, event *WebhookEvent) error {
	s.logger.Info("Delivering webhook event",
		zap.String("event_type", event.EventType),
		zap.String("agent_id", event.AgentID),
	)

	// Find active webhook subscriptions for this event type
	subscriptions, err := s.db.Queries().ListActiveWebhooksByEvent(ctx, sqlc.ListActiveWebhooksByEventParams{
		AgentID:   event.AgentID,
		EventType: event.EventType,
	})

	if err != nil {
		s.logger.Error("Failed to fetch webhook subscriptions",
			zap.Error(err),
			zap.String("agent_id", event.AgentID),
			zap.String("event_type", event.EventType),
		)
		return fmt.Errorf("fetch webhook subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		s.logger.Debug("No active webhook subscriptions found",
			zap.String("agent_id", event.AgentID),
			zap.String("event_type", event.EventType),
		)
		return nil
	}

	// Deliver to each subscription
	for _, subscription := range subscriptions {
		if err := s.deliverToSubscription(ctx, subscription, event); err != nil {
			s.logger.Error("Failed to deliver webhook",
				zap.Error(err),
				zap.String("subscription_id", subscription.ID.String()),
				zap.String("webhook_url", subscription.WebhookUrl),
			)
			// Continue to next subscription even if one fails
			continue
		}
	}

	return nil
}

// deliverToSubscription delivers an event to a single webhook subscription
func (s *WebhookDeliveryService) deliverToSubscription(
	ctx context.Context,
	subscription sqlc.WebhookSubscription,
	event *WebhookEvent,
) error {
	// Serialize event payload
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	// Generate signature
	signature := s.generateSignature(payload, subscription.Secret)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", subscription.WebhookUrl, bytes.NewReader(payload))
	if err != nil {
		return s.recordDeliveryFailure(ctx, subscription.ID, event.EventType, payload, 0, fmt.Sprintf("create request: %v", err))
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event-Type", event.EventType)
	req.Header.Set("X-Webhook-Timestamp", event.Timestamp.Format(time.RFC3339))

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return s.recordDeliveryFailure(ctx, subscription.ID, event.EventType, payload, 0, fmt.Sprintf("send request: %v", err))
	}
	defer resp.Body.Close()

	// Read response body (for logging)
	body, _ := io.ReadAll(resp.Body)

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success
		return s.recordDeliverySuccess(ctx, subscription.ID, event.EventType, payload, resp.StatusCode)
	}

	// Failed delivery
	errorMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
	return s.recordDeliveryFailure(ctx, subscription.ID, event.EventType, payload, resp.StatusCode, errorMsg)
}

// generateSignature creates HMAC-SHA256 signature of the payload
func (s *WebhookDeliveryService) generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// recordDeliverySuccess records a successful webhook delivery
func (s *WebhookDeliveryService) recordDeliverySuccess(
	ctx context.Context,
	subscriptionID uuid.UUID,
	eventType string,
	payload []byte,
	httpStatusCode int,
) error {
	_, err := s.db.Queries().CreateWebhookDelivery(ctx, sqlc.CreateWebhookDeliveryParams{
		SubscriptionID: subscriptionID,
		EventType:      eventType,
		Payload:        payload,
		Status:         "success",
		HttpStatusCode: pgtype.Int4{Int32: int32(httpStatusCode), Valid: true},
		ErrorMessage:   pgtype.Text{Valid: false},
		Attempts:       1,
		NextRetryAt:    pgtype.Timestamptz{Valid: false},
	})

	if err != nil {
		s.logger.Error("Failed to record webhook delivery success",
			zap.Error(err),
			zap.String("subscription_id", subscriptionID.String()),
		)
		return err
	}

	s.logger.Info("Webhook delivered successfully",
		zap.String("subscription_id", subscriptionID.String()),
		zap.String("event_type", eventType),
		zap.Int("http_status", httpStatusCode),
	)

	return nil
}

// recordDeliveryFailure records a failed webhook delivery
func (s *WebhookDeliveryService) recordDeliveryFailure(
	ctx context.Context,
	subscriptionID uuid.UUID,
	eventType string,
	payload []byte,
	httpStatusCode int,
	errorMessage string,
) error {
	// Calculate next retry time (exponential backoff)
	nextRetry := time.Now().Add(5 * time.Minute) // First retry after 5 minutes

	_, err := s.db.Queries().CreateWebhookDelivery(ctx, sqlc.CreateWebhookDeliveryParams{
		SubscriptionID: subscriptionID,
		EventType:      eventType,
		Payload:        payload,
		Status:         "pending", // Will be retried
		HttpStatusCode: pgtype.Int4{Int32: int32(httpStatusCode), Valid: httpStatusCode > 0},
		ErrorMessage:   pgtype.Text{String: errorMessage, Valid: true},
		Attempts:       1,
		NextRetryAt:    pgtype.Timestamptz{Time: nextRetry, Valid: true},
	})

	if err != nil {
		s.logger.Error("Failed to record webhook delivery failure",
			zap.Error(err),
			zap.String("subscription_id", subscriptionID.String()),
		)
		return err
	}

	s.logger.Warn("Webhook delivery failed, scheduled for retry",
		zap.String("subscription_id", subscriptionID.String()),
		zap.String("event_type", eventType),
		zap.Int("http_status", httpStatusCode),
		zap.String("error", errorMessage),
		zap.Time("next_retry", nextRetry),
	)

	return fmt.Errorf("webhook delivery failed: %s", errorMessage)
}

// RetryFailedDeliveries retries pending webhook deliveries
func (s *WebhookDeliveryService) RetryFailedDeliveries(ctx context.Context, maxRetries int) (int, error) {
	s.logger.Info("Starting webhook delivery retry process", zap.Int("max_retries", maxRetries))

	deliveries, err := s.db.Queries().ListPendingWebhookDeliveries(ctx, 100) // Process up to 100 at a time
	if err != nil {
		return 0, fmt.Errorf("fetch pending deliveries: %w", err)
	}

	retried := 0
	for _, delivery := range deliveries {
		// Skip if max retries exceeded
		if int(delivery.Attempts) >= maxRetries {
			// Mark as failed
			_, _ = s.db.Queries().UpdateWebhookDeliveryStatus(ctx, sqlc.UpdateWebhookDeliveryStatusParams{
				ID:             delivery.ID,
				Status:         "failed",
				HttpStatusCode: delivery.HttpStatusCode,
				ErrorMessage:   pgtype.Text{String: "max retries exceeded", Valid: true},
				Attempts:       delivery.Attempts,
				NextRetryAt:    pgtype.Timestamptz{Valid: false},
				DeliveredAt:    pgtype.Timestamptz{Valid: false},
			})
			continue
		}

		// Get subscription
		subscription, err := s.db.Queries().GetWebhookSubscription(ctx, delivery.SubscriptionID)
		if err != nil {
			s.logger.Error("Failed to get subscription for retry",
				zap.Error(err),
				zap.String("delivery_id", delivery.ID.String()),
			)
			continue
		}

		// Unmarshal event from payload
		var event WebhookEvent
		if err := json.Unmarshal(delivery.Payload, &event); err != nil {
			s.logger.Error("Failed to unmarshal event payload",
				zap.Error(err),
				zap.String("delivery_id", delivery.ID.String()),
			)
			continue
		}

		// Retry delivery
		if err := s.deliverToSubscription(ctx, subscription, &event); err != nil {
			// Update attempts count and next retry time
			nextRetry := time.Now().Add(time.Duration(delivery.Attempts+1) * 10 * time.Minute)
			_, _ = s.db.Queries().UpdateWebhookDeliveryStatus(ctx, sqlc.UpdateWebhookDeliveryStatusParams{
				ID:             delivery.ID,
				Status:         "pending",
				HttpStatusCode: delivery.HttpStatusCode,
				ErrorMessage:   delivery.ErrorMessage,
				Attempts:       delivery.Attempts + 1,
				NextRetryAt:    pgtype.Timestamptz{Time: nextRetry, Valid: true},
				DeliveredAt:    pgtype.Timestamptz{Valid: false},
			})
		} else {
			// Success - update delivery record
			_, _ = s.db.Queries().UpdateWebhookDeliveryStatus(ctx, sqlc.UpdateWebhookDeliveryStatusParams{
				ID:             delivery.ID,
				Status:         "success",
				HttpStatusCode: pgtype.Int4{Int32: 200, Valid: true},
				ErrorMessage:   pgtype.Text{Valid: false},
				Attempts:       delivery.Attempts + 1,
				NextRetryAt:    pgtype.Timestamptz{Valid: false},
				DeliveredAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
			retried++
		}
	}

	s.logger.Info("Webhook retry process completed",
		zap.Int("total_pending", len(deliveries)),
		zap.Int("retried", retried),
	)

	return retried, nil
}
