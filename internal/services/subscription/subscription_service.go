package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/services/authorization"
	"github.com/kevin07696/payment-service/internal/util"
	"github.com/kevin07696/payment-service/pkg/observability"
	"github.com/kevin07696/payment-service/pkg/timeutil"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// BillingRetryConfig holds configuration for billing retry backoff
type BillingRetryConfig struct {
	BaseDelaySecs   int     // Base delay in seconds (default: 300 = 5 min)
	MaxDelaySecs    int     // Max delay in seconds (default: 7200 = 2 hr)
	Multiplier      float64 // Backoff multiplier (default: 2.0)
	Jitter          float64 // Jitter factor 0.0-1.0 (default: 0.1 = ±10%)
	// RSWindowDays: Window for retrying a failed recurring billing transaction with ACI_EXT=RS.
	// COMPLIANCE: Visa/MC/Discover require RS retries within 30 days of original decline.
	RSWindowDays int
	DefaultACHClass string  // Default STD_ENTRY_CLASS for ACH (default: "WEB")
}

// DefaultBillingRetryConfig returns sensible defaults for subscription billing retries
// Reads from environment variables:
// - RS_WINDOW_DAYS: ACI_EXT=RS window in days (default: 30)
// - DEFAULT_ACH_CLASS: Default STD_ENTRY_CLASS for ACH (default: "WEB")
func DefaultBillingRetryConfig() BillingRetryConfig {
	rsWindowDays := 30 // 30 days per Visa/Mastercard/Discover rules
	if v := os.Getenv("RS_WINDOW_DAYS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			rsWindowDays = parsed
		}
	}

	defaultACHClass := "WEB" // Internet-initiated (default for e-commerce)
	if v := os.Getenv("DEFAULT_ACH_CLASS"); v != "" {
		defaultACHClass = v
	}

	return BillingRetryConfig{
		BaseDelaySecs:   300,            // 5 minutes
		MaxDelaySecs:    7200,           // 2 hours
		Multiplier:      2.0,
		Jitter:          0.1,            // ±10%
		RSWindowDays:    rsWindowDays,
		DefaultACHClass: defaultACHClass,
	}
}

// subscriptionService implements the SubscriptionService port
type subscriptionService struct {
	queries             sqlc.Querier
	txManager           database.TransactionManager
	serverPost          ports.ServerPostAdapter
	secretManager       ports.SecretManagerAdapter
	merchantAuthService *authorization.MerchantAuthorizationService
	webhookService      ports.WebhookService
	logger              *zap.Logger
	retryConfig         BillingRetryConfig
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(
	queries sqlc.Querier,
	txManager database.TransactionManager,
	serverPost ports.ServerPostAdapter,
	secretManager ports.SecretManagerAdapter,
	webhookService ports.WebhookService,
	logger *zap.Logger,
	retryConfig *BillingRetryConfig,
) ports.SubscriptionService {
	// Create service-merchant access checker for authorization
	accessChecker := authorization.NewSQLCServiceMerchantAccessChecker(queries)

	// Create merchant authorization service with access checker
	merchantAuthService := authorization.NewMerchantAuthorizationService(logger, accessChecker)

	// Use default config if not provided
	cfg := DefaultBillingRetryConfig()
	if retryConfig != nil {
		if retryConfig.BaseDelaySecs > 0 {
			cfg.BaseDelaySecs = retryConfig.BaseDelaySecs
		}
		if retryConfig.MaxDelaySecs > 0 {
			cfg.MaxDelaySecs = retryConfig.MaxDelaySecs
		}
		if retryConfig.Multiplier > 0 {
			cfg.Multiplier = retryConfig.Multiplier
		}
		if retryConfig.Jitter > 0 {
			cfg.Jitter = retryConfig.Jitter
		}
		if retryConfig.RSWindowDays > 0 {
			cfg.RSWindowDays = retryConfig.RSWindowDays
		}
		if retryConfig.DefaultACHClass != "" {
			cfg.DefaultACHClass = retryConfig.DefaultACHClass
		}
	}

	return &subscriptionService{
		queries:             queries,
		txManager:           txManager,
		serverPost:          serverPost,
		secretManager:       secretManager,
		merchantAuthService: merchantAuthService,
		webhookService:      webhookService,
		logger:              logger,
		retryConfig:         cfg,
	}
}

// CreateSubscription creates a new recurring billing subscription
func (s *subscriptionService) CreateSubscription(ctx context.Context, req *ports.CreateSubscriptionRequest) (*domain.Subscription, error) {
	s.logger.Info("Creating subscription",
		zap.String("merchant_id", req.MerchantID),
		zap.String("customer_id", req.CustomerID),
		zap.Int64("amount_cents", req.AmountCents),
	)

	// Resolve and validate merchant access
	resolvedMerchantID, err := s.merchantAuthService.ResolveMerchantID(ctx, req.MerchantID)
	if err != nil {
		return nil, err
	}

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getSubscriptionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			s.logger.Info("Idempotent request, returning existing subscription",
				zap.String("subscription_id", existing.ID),
			)
			return existing, nil
		}
	}

	// Parse and validate payment method ID
	pmID, err := uuid.Parse(req.PaymentMethodID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	// Verify payment method exists and belongs to customer
	pm, err := s.queries.GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, domain.ErrPMNotFound
	}

	if pm.MerchantID.String() != resolvedMerchantID || pm.CustomerID != req.CustomerID {
		return nil, domain.ErrPMNotBelongToCustomer
	}

	if pm.Status != string(domain.PaymentMethodStatusActive) {
		return nil, domain.ErrPMInactive
	}

	// Validate amount
	if req.AmountCents <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	// Calculate next billing date
	nextBillingDate := calculateNextBillingDate(req.StartDate, req.IntervalValue, req.IntervalUnit)

	// Parse merchant ID
	merchantID, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	// Create subscription in database
	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Marshal metadata
		metadataJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			s.logger.Warn("Failed to marshal metadata", zap.Error(err))
			metadataJSON = []byte("{}")
		}

		params := sqlc.CreateSubscriptionParams{
			ID:                    uuid.New(),
			MerchantID:            merchantID,
			CustomerID:            req.CustomerID,
			AmountCents:           req.AmountCents,
			Currency:              req.Currency,
			IntervalValue:         int32(req.IntervalValue),
			IntervalUnit:          string(req.IntervalUnit),
			Status:                string(domain.SubscriptionStatusActive),
			PaymentMethodID:       pmID,
			NextBillingDate:       pgtype.Date{Time: nextBillingDate, Valid: true},
			FailureRetryCount:     0,
			MaxRetries:            int32(req.MaxRetries),
			GatewaySubscriptionID: pgtype.Text{Valid: false}, // EPX doesn't use gateway subscription IDs
			Metadata:              metadataJSON,
		}

		dbSub, err := q.CreateSubscription(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "create_subscription")
		}

		subscription = sqlcSubscriptionToDomain(&dbSub)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Subscription created",
		zap.String("subscription_id", subscription.ID),
		zap.Time("next_billing_date", subscription.NextBillingDate),
	)

	return subscription, nil
}

// Rest of methods will be added in next part...

// UpdateSubscription updates subscription properties
func (s *subscriptionService) UpdateSubscription(ctx context.Context, req *ports.UpdateSubscriptionRequest) (*domain.Subscription, error) {
	s.logger.Info("Updating subscription",
		zap.String("subscription_id", req.SubscriptionID),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getSubscriptionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
	}

	// Parse subscription ID
	subID, err := uuid.Parse(req.SubscriptionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	// Get existing subscription
	existing, err := s.queries.GetSubscriptionByID(ctx, subID)
	if err != nil {
		return nil, domain.ErrSubscriptionNotFound
	}

	// Ensure subscription is active or past_due
	if existing.Status != string(domain.SubscriptionStatusActive) &&
		existing.Status != string(domain.SubscriptionStatusPastDue) {
		return nil, domain.ErrSubscriptionNotActive
	}

	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Build update params
		params := sqlc.UpdateSubscriptionParams{
			ID: subID,
		}

		// Update amount if provided
		if req.AmountCents != nil {
			if *req.AmountCents <= 0 {
				return domain.ErrInvalidAmount
			}
			params.AmountCents = *req.AmountCents
		} else {
			params.AmountCents = existing.AmountCents
		}

		// Update interval if provided
		if req.IntervalValue != nil {
			params.IntervalValue = int32(*req.IntervalValue)
		} else {
			params.IntervalValue = existing.IntervalValue
		}

		if req.IntervalUnit != nil {
			params.IntervalUnit = string(*req.IntervalUnit)
		} else {
			params.IntervalUnit = existing.IntervalUnit
		}

		// Update payment method if provided
		if req.PaymentMethodID != nil {
			pmID, err := uuid.Parse(*req.PaymentMethodID)
			if err != nil {
				return domain.ErrValidationInvalidUUID
			}

			// Verify payment method exists and belongs to customer
			pm, err := q.GetPaymentMethodByID(ctx, pmID)
			if err != nil {
				return domain.ErrPMNotFound
			}

			if pm.MerchantID.String() != existing.MerchantID.String() || pm.CustomerID != existing.CustomerID {
				return domain.ErrPMNotBelongToCustomer
			}

			if pm.Status != string(domain.PaymentMethodStatusActive) {
				return domain.ErrPMInactive
			}

			params.PaymentMethodID = pmID
		} else {
			params.PaymentMethodID = existing.PaymentMethodID
		}

		dbSub, err := q.UpdateSubscription(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError
		}

		subscription = sqlcSubscriptionToDomain(&dbSub)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Subscription updated",
		zap.String("subscription_id", subscription.ID),
	)

	return subscription, nil
}

// CancelSubscription cancels an active subscription
func (s *subscriptionService) CancelSubscription(ctx context.Context, req *ports.CancelSubscriptionRequest) (*domain.Subscription, error) {
	s.logger.Info("Canceling subscription",
		zap.String("subscription_id", req.SubscriptionID),
		zap.Bool("cancel_at_period_end", req.CancelAtPeriodEnd),
	)

	// Check idempotency
	if req.IdempotencyKey != nil {
		existing, err := s.getSubscriptionByIdempotencyKey(ctx, *req.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
	}

	// Parse subscription ID
	subID, err := uuid.Parse(req.SubscriptionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Get existing subscription
		existing, err := q.GetSubscriptionByID(ctx, subID)
		if err != nil {
			return domain.ErrSubscriptionNotFound
		}

		// Check if already cancelled
		if existing.Status == string(domain.SubscriptionStatusCancelled) {
			subscription = sqlcSubscriptionToDomain(&existing)
			return nil
		}

		var newStatus string
		var cancelledAt pgtype.Timestamptz

		if req.CancelAtPeriodEnd {
			// Mark for cancellation at period end
			newStatus = string(domain.SubscriptionStatusActive)
			cancelledAt = pgtype.Timestamptz{Valid: false}
		} else {
			// Cancel immediately
			newStatus = string(domain.SubscriptionStatusCancelled)
			cancelledAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		}

		params := sqlc.CancelSubscriptionParams{
			ID:         subID,
			Status:     newStatus,
			CanceledAt: cancelledAt,
		}

		dbSub, err := q.CancelSubscription(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError
		}

		subscription = sqlcSubscriptionToDomain(&dbSub)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Subscription canceled",
		zap.String("subscription_id", subscription.ID),
		zap.String("status", string(subscription.Status)),
	)

	return subscription, nil
}

// PauseSubscription pauses an active subscription
func (s *subscriptionService) PauseSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error) {
	s.logger.Info("Pausing subscription",
		zap.String("subscription_id", subscriptionID),
	)

	// Parse subscription ID
	subID, err := uuid.Parse(subscriptionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Get existing subscription
		existing, err := q.GetSubscriptionByID(ctx, subID)
		if err != nil {
			return domain.ErrSubscriptionNotFound
		}

		// Can only pause active subscriptions
		if existing.Status != string(domain.SubscriptionStatusActive) {
			return domain.ErrSubscriptionNotActive
		}

		params := sqlc.UpdateSubscriptionStatusParams{
			ID:     subID,
			Status: string(domain.SubscriptionStatusPaused),
		}

		dbSub, err := q.UpdateSubscriptionStatus(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError
		}

		subscription = sqlcSubscriptionToDomain(&dbSub)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Subscription paused",
		zap.String("subscription_id", subscription.ID),
	)

	return subscription, nil
}

// ResumeSubscription resumes a paused subscription
func (s *subscriptionService) ResumeSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error) {
	s.logger.Info("Resuming subscription",
		zap.String("subscription_id", subscriptionID),
	)

	// Parse subscription ID
	subID, err := uuid.Parse(subscriptionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Get existing subscription
		existing, err := q.GetSubscriptionByID(ctx, subID)
		if err != nil {
			return domain.ErrSubscriptionNotFound
		}

		// Can only resume paused subscriptions
		if existing.Status != string(domain.SubscriptionStatusPaused) {
			return domain.ErrSubscriptionNotActive
		}

		params := sqlc.UpdateSubscriptionStatusParams{
			ID:     subID,
			Status: string(domain.SubscriptionStatusActive),
		}

		dbSub, err := q.UpdateSubscriptionStatus(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError
		}

		subscription = sqlcSubscriptionToDomain(&dbSub)
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Subscription resumed",
		zap.String("subscription_id", subscription.ID),
	)

	return subscription, nil
}

// GetSubscription retrieves subscription details
func (s *subscriptionService) GetSubscription(ctx context.Context, subscriptionID string) (*domain.Subscription, error) {
	subID, err := uuid.Parse(subscriptionID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	// Trace database query
	ctx, span := observability.StartDBSpan(ctx, "GetSubscriptionByID", "subscriptions")
	defer span.End()

	dbSub, err := s.queries.GetSubscriptionByID(ctx, subID)
	observability.EndDBSpan(span, err)
	if err != nil {
		s.logger.Debug("Subscription not found",
			zap.String("subscription_id", subscriptionID),
			zap.Error(err),
		)
		return nil, domain.ErrSubscriptionNotFound
	}

	return sqlcSubscriptionToDomain(&dbSub), nil
}

// ListSubscriptions lists subscriptions with optional filters
func (s *subscriptionService) ListSubscriptions(ctx context.Context, merchantID, customerID string) ([]*domain.Subscription, error) {
	// Resolve and validate merchant access
	resolvedMerchantID, err := s.merchantAuthService.ResolveMerchantID(ctx, merchantID)
	if err != nil {
		return nil, err
	}

	// Parse merchant ID
	merchantUUID, err := uuid.Parse(resolvedMerchantID)
	if err != nil {
		return nil, domain.ErrValidationInvalidUUID
	}

	params := sqlc.ListSubscriptionsByCustomerParams{
		MerchantID: merchantUUID,
		CustomerID: customerID,
	}

	// Trace database query
	ctx, span := observability.StartDBSpan(ctx, "ListSubscriptionsByCustomer", "subscriptions")
	defer span.End()

	dbSubs, err := s.queries.ListSubscriptionsByCustomer(ctx, params)
	observability.EndDBSpan(span, err)
	if err != nil {
		return nil, domain.ErrDatabaseError
	}

	observability.AddDBResultAttributes(span, int64(len(dbSubs)))

	subscriptions := make([]*domain.Subscription, len(dbSubs))
	for i, dbSub := range dbSubs {
		subscriptions[i] = sqlcSubscriptionToDomain(&dbSub)
	}

	return subscriptions, nil
}

// ProcessDueBilling processes subscriptions due for billing (cron/admin)
func (s *subscriptionService) ProcessDueBilling(ctx context.Context, asOfDate time.Time, batchSize int) (processed, success, failed int, errors []error) {
	s.logger.Info("Processing due billing",
		zap.Time("as_of_date", asOfDate),
		zap.Int("batch_size", batchSize),
	)

	// Get subscriptions due for billing
	params := sqlc.ListSubscriptionsDueForBillingParams{
		NextBillingDate: pgtype.Date{Time: asOfDate, Valid: true},
		LimitVal:        int32(batchSize),
	}

	dueSubs, err := s.queries.ListSubscriptionsDueForBilling(ctx, params)
	if err != nil {
		s.logger.Error("Failed to list due subscriptions", zap.Error(err))
		return 0, 0, 0, []error{err}
	}

	processed = len(dueSubs)
	s.logger.Info("Found subscriptions due for billing",
		zap.Int("count", processed),
	)

	// Process each subscription
	for _, sub := range dueSubs {
		if err := s.processSubscriptionBilling(ctx, &sub); err != nil {
			failed++
			errors = append(errors, fmt.Errorf("subscription %s: %w", sub.ID.String(), err))
			s.logger.Error("Failed to process subscription billing",
				zap.String("subscription_id", sub.ID.String()),
				zap.Error(err),
			)
		} else {
			success++
			s.logger.Info("Successfully processed subscription billing",
				zap.String("subscription_id", sub.ID.String()),
			)
		}
	}

	s.logger.Info("Billing processing completed",
		zap.Int("processed", processed),
		zap.Int("success", success),
		zap.Int("failed", failed),
	)

	return processed, success, failed, errors
}

// processSubscriptionBilling handles billing for a single subscription
func (s *subscriptionService) processSubscriptionBilling(ctx context.Context, sub *sqlc.Subscription) error {
	// Generate deterministic transaction ID for this billing cycle (idempotency)
	// Format: subscription_id + next_billing_date = ensures one charge per billing period
	idempotencyKey := fmt.Sprintf("%s-%s", sub.ID.String(), sub.NextBillingDate.Time.Format("2006-01-02"))
	txID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(idempotencyKey))

	// Check if we already processed this billing cycle
	existingTx, err := s.queries.GetTransactionByID(ctx, txID)
	if err == nil && existingTx.ID == txID {
		// Already charged successfully, just update subscription
		s.logger.Info("Billing already processed for this cycle, skipping",
			zap.String("subscription_id", sub.ID.String()),
			zap.String("transaction_id", txID.String()),
		)
		return s.updateNextBillingDate(ctx, sub)
	}

	// Get merchant credentials
	merchantID, err := uuid.Parse(sub.MerchantID.String())
	if err != nil {
		return domain.ErrValidationInvalidUUID.WithDetail("field", "merchant_id")
	}

	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return domain.ErrMerchantNotFoundTyped
	}

	if !merchant.IsActive {
		return domain.ErrMerchantInactiveTyped
	}

	// Get payment method
	pm, err := s.queries.GetPaymentMethodByID(ctx, sub.PaymentMethodID)
	if err != nil {
		return domain.ErrPMNotFound
	}

	if pm.Status != string(domain.PaymentMethodStatusActive) {
		return domain.ErrPMInactive
	}

	// Get MAC secret for EPX request signing
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return domain.ErrMerchantCredentialFailed
	}

	// Prepare EPX request - convert cents back to decimal string
	amount := decimal.NewFromInt(sub.AmountCents).Div(decimal.NewFromInt(100))

	// Generate deterministic TRAN_NBR from transaction ID using FNV-1a hash
	// This ensures the same transaction ID always produces the same TRAN_NBR (idempotency)
	tranNbr := util.UUIDToEPXTranNbr(txID)

	// Recurring billing requires specific EPX fields:
	// - OriginalAuthGUID: the Storage BRIC (not AuthGUID)
	// - ACIExt: "RB" (Recurring Billing) or "RS" (Resubmission for retries)
	// - CardEntryMethod: "Z" (stored credential/token)
	//
	// ACI_EXT values per EPX Card on File specs:
	// - RB: First billing attempt (Recurring Billing)
	// - RS: Retry of previously declined transaction (Resubmission)
	//
	// RS Window Rule (Visa/Mastercard/Discover):
	// RS can only be used within the configured window (default 30 days) of the original decline.
	// After the window expires, use RB (treating it as a new billing attempt).
	var aciExt string
	if sub.FailureRetryCount > 0 {
		// Check if within RS window (configurable, default 30 days)
		rsWindowValid := false
		if sub.LastBillingErrorAt.Valid {
			daysSinceFailure := time.Since(sub.LastBillingErrorAt.Time).Hours() / 24
			rsWindowValid = daysSinceFailure <= float64(s.retryConfig.RSWindowDays)
		}

		if rsWindowValid {
			// Within 30-day window - use RS (Resubmission)
			aciExt = "RS"
			s.logger.Info("Using ACI_EXT=RS for payment retry (within 30-day window)",
				zap.String("subscription_id", sub.ID.String()),
				zap.Int32("retry_count", sub.FailureRetryCount),
				zap.Time("last_error_at", sub.LastBillingErrorAt.Time),
			)
		} else {
			// Beyond 30-day RS window - use RB (new billing attempt)
			aciExt = "RB"
			s.logger.Warn("RS 30-day window expired, using RB instead",
				zap.String("subscription_id", sub.ID.String()),
				zap.Int32("retry_count", sub.FailureRetryCount),
			)
		}
	} else {
		// First attempt for this billing cycle
		aciExt = "RB" // Recurring Billing
	}
	cardEntryMethod := "Z"
	industryType := "E" // E-commerce

	epxReq := &ports.ServerPostRequest{
		CustNbr:          merchant.CustNbr,
		MerchNbr:         merchant.MerchNbr,
		DBAnbr:           merchant.DbaNbr,
		TerminalNbr:      merchant.TerminalNbr,
		TransactionType:  ports.TransactionTypeSale,
		Amount:           amount.String(),
		PaymentType:      ports.PaymentMethodType(pm.PaymentType),
		OriginalAuthGUID: pm.Bric, // Use OriginalAuthGUID for stored BRIC
		TranNbr:          tranNbr,
		TranGroup:        uuid.New().String(),
		CustomerID:       sub.CustomerID,
		ACIExt:           &aciExt,          // "RB" = Recurring Billing
		CardEntryMethod:  &cardEntryMethod, // "Z" = stored credential
		IndustryType:     &industryType,    // "E" = E-commerce
	}

	// For ACH subscription billing, automatically use PPD (Prearranged Payment and Deposit)
	// PPD is the correct SEC code for recurring/prearranged consumer ACH transactions
	if pm.PaymentType == string(domain.PaymentMethodTypeACH) {
		stdEntryClass := "PPD"
		epxReq.StdEntryClass = &stdEntryClass
	}

	// Process transaction through EPX
	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		// Handle billing failure
		return s.handleBillingFailure(ctx, sub, err)
	}

	if !epxResp.IsApproved {
		// Handle declined transaction
		return s.handleBillingFailure(ctx, sub, domain.ErrGatewayDeclined.WithDetail("auth_resp_text", epxResp.AuthRespText))
	}

	// Save transaction and update subscription
	return s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Create transaction record with deterministic ID
		// Note: Status is auto-generated by database based on auth_resp
		// auth_guid (BRIC) is stored directly in the transaction
		pmIDStr := pm.ID.String()

		// Build metadata with subscription info and EPX display fields
		metadata := map[string]interface{}{
			"subscription_id": sub.ID.String(),
			"auth_resp_text":  epxResp.AuthRespText,
			"auth_avs":        epxResp.AuthAVS,
			"auth_cvv2":       epxResp.AuthCVV2,
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return domain.ErrInternalError.WithDetail("operation", "marshal_metadata")
		}

		txParams := sqlc.CreateTransactionParams{
			ID:                  txID, // Use deterministic ID for idempotency
			MerchantID:          sub.MerchantID,
			CustomerID:          pgtype.Text{String: sub.CustomerID, Valid: true},
			AmountCents:         sub.AmountCents,
			Currency:            sub.Currency,
			Type:                string(domain.TransactionTypeSale),
			PaymentMethodType:   pm.PaymentType,
			PaymentMethodID:     toNullableUUID(&pmIDStr),
			SubscriptionID:      pgtype.UUID{Bytes: sub.ID, Valid: true},   // Link to subscription
			TranNbr:             pgtype.Text{String: tranNbr, Valid: true}, // Store deterministic TRAN_NBR
			AuthGuid:            toNullableText(&epxResp.AuthGUID),         // Store BRIC token
			AuthResp:            pgtype.Text{String: epxResp.AuthResp, Valid: true},
			AuthCode:            toNullableText(&epxResp.AuthCode),
			AuthCardType:        toNullableText(&epxResp.AuthCardType),
			Metadata:            metadataJSON,
			ParentTransactionID: pgtype.UUID{}, // NULL for first transaction
			ProcessedAt:         pgtype.Timestamptz{},
		}

		_, err = q.CreateTransaction(ctx, txParams)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "create_transaction")
		}

		// Calculate next billing date
		nextBillingDate := calculateNextBillingDate(
			sub.NextBillingDate.Time,
			int(sub.IntervalValue),
			domain.IntervalUnit(sub.IntervalUnit),
		)

		// Update subscription with new billing date and reset failure count
		updateParams := sqlc.UpdateSubscriptionBillingParams{
			ID:                sub.ID,
			NextBillingDate:   pgtype.Date{Time: nextBillingDate, Valid: true},
			FailureRetryCount: 0,
			Status:            string(domain.SubscriptionStatusActive),
		}

		_, err = q.UpdateSubscriptionBilling(ctx, updateParams)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "update_subscription_billing")
		}

		// Clear any retry backoff on successful billing
		err = q.ClearBillingRetryBackoff(ctx, sub.ID)
		if err != nil {
			s.logger.Warn("Failed to clear billing retry backoff (non-fatal)",
				zap.String("subscription_id", sub.ID.String()),
				zap.Error(err),
			)
			// Non-fatal - billing succeeded, just couldn't clear backoff tracking
		}

		return nil
	})
}

// updateNextBillingDate updates the subscription's next billing date
func (s *subscriptionService) updateNextBillingDate(ctx context.Context, sub *sqlc.Subscription) error {
	return s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Calculate next billing date
		nextBillingDate := calculateNextBillingDate(
			sub.NextBillingDate.Time,
			int(sub.IntervalValue),
			domain.IntervalUnit(sub.IntervalUnit),
		)

		// Update subscription with new billing date and reset failure count
		updateParams := sqlc.UpdateSubscriptionBillingParams{
			ID:                sub.ID,
			NextBillingDate:   pgtype.Date{Time: nextBillingDate, Valid: true},
			FailureRetryCount: 0,
			Status:            string(domain.SubscriptionStatusActive),
		}

		_, err := q.UpdateSubscriptionBilling(ctx, updateParams)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "update_subscription_billing")
		}

		return nil
	})
}

// handleBillingFailure handles a failed billing attempt with conditional backoff
// Transient errors (network issues) trigger exponential backoff
// Permanent errors (card declined) just increment retry count without backoff
func (s *subscriptionService) handleBillingFailure(ctx context.Context, sub *sqlc.Subscription, billingErr error) error {
	return s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		newRetryCount := sub.FailureRetryCount + 1
		isTransient := isTransientBillingError(billingErr)
		errorMsg := truncateErrorMessage(billingErr.Error(), 500)

		if newRetryCount >= sub.MaxRetries {
			// Max retries reached - mark as past_due (uses IncrementSubscriptionFailureCount for status change)
			s.logger.Warn("Subscription billing failed - max retries reached",
				zap.String("subscription_id", sub.ID.String()),
				zap.Int32("retry_count", newRetryCount),
				zap.Bool("transient_error", isTransient),
				zap.Error(billingErr),
			)

			params := sqlc.IncrementSubscriptionFailureCountParams{
				ID:                sub.ID,
				FailureRetryCount: newRetryCount,
				Status:            string(domain.SubscriptionStatusPastDue),
			}
			_, err := q.IncrementSubscriptionFailureCount(ctx, params)
			if err != nil {
				return domain.ErrDatabaseError.WithDetail("operation", "update_failure_count")
			}
			return billingErr
		}

		// Still have retries remaining
		if isTransient {
			// Transient error - schedule retry with exponential backoff
			nextRetryAt := s.calculateRetryDelay(int(sub.FailureRetryCount))
			s.logger.Warn("Subscription billing failed (transient) - scheduling retry with backoff",
				zap.String("subscription_id", sub.ID.String()),
				zap.Int32("retry_count", newRetryCount),
				zap.Int32("max_retries", sub.MaxRetries),
				zap.Time("next_retry_at", nextRetryAt),
				zap.Error(billingErr),
			)

			err := q.SetBillingRetryBackoff(ctx, sqlc.SetBillingRetryBackoffParams{
				ID:                 sub.ID,
				NextBillingRetryAt: pgtype.Timestamptz{Time: nextRetryAt, Valid: true},
				LastBillingError:   pgtype.Text{String: errorMsg, Valid: true},
			})
			if err != nil {
				return domain.ErrDatabaseError.WithDetail("operation", "set_billing_retry_backoff")
			}
		} else {
			// Permanent error - just record failure, no backoff (will retry on next cron run)
			s.logger.Warn("Subscription billing failed (permanent) - will retry on next cron run",
				zap.String("subscription_id", sub.ID.String()),
				zap.Int32("retry_count", newRetryCount),
				zap.Int32("max_retries", sub.MaxRetries),
				zap.Error(billingErr),
			)

			err := q.RecordBillingFailure(ctx, sqlc.RecordBillingFailureParams{
				ID:               sub.ID,
				LastBillingError: pgtype.Text{String: errorMsg, Valid: true},
			})
			if err != nil {
				return domain.ErrDatabaseError.WithDetail("operation", "record_billing_failure")
			}
		}

		return billingErr
	})
}

// isTransientBillingError checks if the billing error is transient (network issues)
// vs permanent (card declined, insufficient funds, etc.)
// Transient errors warrant exponential backoff; permanent errors do not
func isTransientBillingError(err error) bool {
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
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "gateway") ||
		strings.Contains(errStr, "econnrefused") ||
		strings.Contains(errStr, "econnreset")
}

// calculateRetryDelay calculates the next retry time using exponential backoff with jitter
func (s *subscriptionService) calculateRetryDelay(attempt int) time.Time {
	if attempt < 0 {
		attempt = 0
	}

	baseDelay := float64(s.retryConfig.BaseDelaySecs)
	maxDelay := float64(s.retryConfig.MaxDelaySecs)

	// Calculate exponential delay: BaseDelay * (Multiplier ^ attempt)
	delay := baseDelay * math.Pow(s.retryConfig.Multiplier, float64(attempt))

	// Cap at MaxDelay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter: delay ± (delay * jitter)
	jitterAmount := delay * s.retryConfig.Jitter
	jitter := (rand.Float64()*2 - 1) * jitterAmount

	finalDelay := time.Duration(delay+jitter) * time.Second

	// Ensure minimum delay
	if finalDelay < time.Duration(s.retryConfig.BaseDelaySecs)*time.Second/2 {
		finalDelay = time.Duration(s.retryConfig.BaseDelaySecs) * time.Second
	}

	return timeutil.Now().Add(finalDelay)
}

// truncateErrorMessage truncates error message to max length
func truncateErrorMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen-3] + "..."
}

// getSubscriptionByIdempotencyKey retrieves a subscription by idempotency key
func (s *subscriptionService) getSubscriptionByIdempotencyKey(ctx context.Context, key string) (*domain.Subscription, error) {
	// Note: This would require a separate SQL query if we want to support idempotency for subscriptions
	// For now, returning not found error
	return nil, domain.ErrSubscriptionNotFound
}

// Advisory lock ID for preventing concurrent expired past_due processing
// Using 'pastdue' as hex: 0x70617374647565
const expiredPastDueLockID int64 = 0x70617374647565

// ProcessExpiredPastDue auto-cancels subscriptions whose grace period has expired.
// It queries for past_due subscriptions where past_due_since + grace_period_days < now,
// then cancels them with reason "grace_period_expired".
// Uses advisory locking to prevent concurrent execution.
func (s *subscriptionService) ProcessExpiredPastDue(ctx context.Context, batchSize int) *ports.ExpiredPastDueResult {
	result := &ports.ExpiredPastDueResult{}

	s.logger.Info("Processing expired past_due subscriptions",
		zap.Int("batch_size", batchSize),
	)

	// Try to acquire advisory lock to prevent concurrent runs
	acquired, err := s.queries.TryAdvisoryLock(ctx, expiredPastDueLockID)
	if err != nil {
		s.logger.Error("Failed to acquire advisory lock", zap.Error(err))
		result.Errors = append(result.Errors, domain.ErrDatabaseError.WithDetail("operation", "acquire_advisory_lock"))
		return result
	}
	if !acquired {
		s.logger.Info("Another process is already handling expired past_due subscriptions, skipping")
		return result
	}
	defer func() {
		if _, unlockErr := s.queries.AdvisoryUnlock(ctx, expiredPastDueLockID); unlockErr != nil {
			s.logger.Error("Failed to release advisory lock", zap.Error(unlockErr))
		}
	}()

	// Get subscriptions where grace period has expired
	expiredSubs, err := s.queries.ListExpiredPastDueSubscriptions(ctx, int32(batchSize))
	if err != nil {
		s.logger.Error("Failed to list expired past_due subscriptions", zap.Error(err))
		result.Errors = append(result.Errors, domain.ErrDatabaseError.WithDetail("operation", "list_expired_subscriptions"))
		return result
	}

	result.Processed = len(expiredSubs)

	if len(expiredSubs) == 0 {
		s.logger.Info("No expired past_due subscriptions found")
		return result
	}

	s.logger.Info("Found expired past_due subscriptions",
		zap.Int("count", len(expiredSubs)),
	)

	// Cancel each expired subscription
	for _, sub := range expiredSubs {
		if err := s.cancelExpiredSubscription(ctx, &sub); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Errorf("failed to cancel subscription %s (merchant=%s): %w",
				sub.ID.String(), sub.MerchantID.String(), err))
			s.logger.Error("Failed to cancel expired subscription",
				zap.String("subscription_id", sub.ID.String()),
				zap.String("merchant_id", sub.MerchantID.String()),
				zap.String("customer_id", sub.CustomerID),
				zap.Error(err),
			)
		} else {
			result.Cancelled++
			s.logger.Info("Cancelled expired past_due subscription",
				zap.String("subscription_id", sub.ID.String()),
				zap.String("merchant_id", sub.MerchantID.String()),
				zap.String("customer_id", sub.CustomerID),
				zap.Time("past_due_since", sub.PastDueSince.Time),
				zap.Int32("grace_period_days", sub.GracePeriodDays),
			)
		}
	}

	s.logger.Info("Expired past_due processing completed",
		zap.Int("processed", result.Processed),
		zap.Int("cancelled", result.Cancelled),
		zap.Int("failed", result.Failed),
	)

	return result
}

// cancelExpiredSubscription cancels a subscription due to grace period expiration.
// Uses row-level locking (SELECT FOR UPDATE) to prevent race conditions.
func (s *subscriptionService) cancelExpiredSubscription(ctx context.Context, sub *sqlc.Subscription) error {
	return s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Lock the row and re-fetch to ensure subscription state hasn't changed
		current, err := q.GetSubscriptionByIDForUpdate(ctx, sub.ID)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "lock_subscription")
		}

		// Validate subscription is still in past_due status
		if current.Status != string(domain.SubscriptionStatusPastDue) {
			s.logger.Info("Subscription status changed, skipping cancellation",
				zap.String("subscription_id", sub.ID.String()),
				zap.String("current_status", current.Status),
			)
			return nil // Not an error, just skip
		}

		// Validate subscription is already cancelled
		if current.Status == string(domain.SubscriptionStatusCancelled) {
			s.logger.Info("Subscription already cancelled, skipping",
				zap.String("subscription_id", sub.ID.String()),
			)
			return nil
		}

		// Double-check grace period hasn't been extended
		if current.PastDueSince.Valid {
			gracePeriodEnd := current.PastDueSince.Time.AddDate(0, 0, int(current.GracePeriodDays))
			if time.Now().Before(gracePeriodEnd) {
				s.logger.Info("Grace period extended, skipping cancellation",
					zap.String("subscription_id", sub.ID.String()),
					zap.Time("new_grace_period_end", gracePeriodEnd),
				)
				return nil
			}
		}

		params := sqlc.CancelSubscriptionWithReasonParams{
			ID:                 sub.ID,
			CancellationReason: pgtype.Text{String: string(domain.CancellationReasonGracePeriodExpired), Valid: true},
		}

		_, err = q.CancelSubscriptionWithReason(ctx, params)
		if err != nil {
			return domain.ErrDatabaseError.WithDetail("operation", "cancel_subscription")
		}

		// Send webhook notification for subscription.cancelled event
		if s.webhookService != nil {
			if webhookErr := s.webhookService.SendSubscriptionCancelled(ctx, sub.ID, sub.MerchantID.String(), string(domain.CancellationReasonGracePeriodExpired)); webhookErr != nil {
				// Log but don't fail - webhook delivery failure shouldn't block subscription cancellation
				s.logger.Warn("Failed to send subscription cancelled webhook",
					zap.Error(webhookErr),
					zap.String("subscription_id", sub.ID.String()),
					zap.String("merchant_id", sub.MerchantID.String()),
				)
			}
		}

		return nil
	})
}

// Helper functions

func calculateNextBillingDate(currentDate time.Time, intervalValue int, intervalUnit domain.IntervalUnit) time.Time {
	switch intervalUnit {
	case domain.IntervalUnitDay:
		return currentDate.AddDate(0, 0, intervalValue)
	case domain.IntervalUnitWeek:
		return currentDate.AddDate(0, 0, intervalValue*7)
	case domain.IntervalUnitMonth:
		return currentDate.AddDate(0, intervalValue, 0)
	case domain.IntervalUnitYear:
		return currentDate.AddDate(intervalValue, 0, 0)
	default:
		return currentDate.AddDate(0, 1, 0) // Default to monthly
	}
}

func sqlcSubscriptionToDomain(dbSub *sqlc.Subscription) *domain.Subscription {
	sub := &domain.Subscription{
		ID:                dbSub.ID.String(),
		MerchantID:        dbSub.MerchantID.String(),
		CustomerID:        dbSub.CustomerID,
		AmountCents:       dbSub.AmountCents,
		Currency:          dbSub.Currency,
		IntervalValue:     int(dbSub.IntervalValue),
		IntervalUnit:      domain.IntervalUnit(dbSub.IntervalUnit),
		Status:            domain.SubscriptionStatus(dbSub.Status),
		PaymentMethodID:   dbSub.PaymentMethodID.String(),
		NextBillingDate:   dbSub.NextBillingDate.Time,
		FailureRetryCount: int(dbSub.FailureRetryCount),
		MaxRetries:        int(dbSub.MaxRetries),
		GracePeriodDays:   int(dbSub.GracePeriodDays),
		CreatedAt:         dbSub.CreatedAt,
		UpdatedAt:         dbSub.UpdatedAt,
	}

	if dbSub.CancelledAt.Valid {
		sub.CancelledAt = &dbSub.CancelledAt.Time
	}

	if dbSub.PastDueSince.Valid {
		sub.PastDueSince = &dbSub.PastDueSince.Time
	}

	if dbSub.GatewaySubscriptionID.Valid {
		sub.GatewaySubscriptionID = &dbSub.GatewaySubscriptionID.String
	}

	if dbSub.CancellationReason.Valid {
		sub.CancellationReason = &dbSub.CancellationReason.String
	}

	if len(dbSub.Metadata) > 0 {
		if err := json.Unmarshal(dbSub.Metadata, &sub.Metadata); err != nil {
			// Metadata unmarshal failed - set to nil
			sub.Metadata = nil
		}
	}

	return sub
}

func toNullableText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func toNullableUUID(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{Valid: false}
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}
