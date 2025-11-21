package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/database"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/util"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// subscriptionService implements the SubscriptionService port
type subscriptionService struct {
	queries       sqlc.Querier
	txManager     database.TransactionManager
	serverPost    adapterports.ServerPostAdapter
	secretManager adapterports.SecretManagerAdapter
	logger        *zap.Logger
}

// NewSubscriptionService creates a new subscription service
func NewSubscriptionService(
	queries sqlc.Querier,
	txManager database.TransactionManager,
	serverPost adapterports.ServerPostAdapter,
	secretManager adapterports.SecretManagerAdapter,
	logger *zap.Logger,
) ports.SubscriptionService {
	return &subscriptionService{
		queries:       queries,
		txManager:     txManager,
		serverPost:    serverPost,
		secretManager: secretManager,
		logger:        logger,
	}
}

// CreateSubscription creates a new recurring billing subscription
func (s *subscriptionService) CreateSubscription(ctx context.Context, req *ports.CreateSubscriptionRequest) (*domain.Subscription, error) {
	s.logger.Info("Creating subscription",
		zap.String("merchant_id", req.MerchantID),
		zap.String("customer_id", req.CustomerID),
		zap.Int64("amount_cents", req.AmountCents),
	)

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
		return nil, fmt.Errorf("invalid payment_method_id format: %w", err)
	}

	// Verify payment method exists and belongs to customer
	pm, err := s.queries.GetPaymentMethodByID(ctx, pmID)
	if err != nil {
		return nil, fmt.Errorf("payment method not found: %w", err)
	}

	// Parse customer ID to UUID for comparison
	reqCustomerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer_id format: %w", err)
	}

	if pm.MerchantID.String() != req.MerchantID || pm.CustomerID != reqCustomerID {
		return nil, fmt.Errorf("payment method does not belong to customer")
	}

	if !pm.IsActive.Valid || !pm.IsActive.Bool {
		return nil, fmt.Errorf("payment method is not active")
	}

	// Validate amount
	if req.AmountCents <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}

	// Calculate next billing date
	nextBillingDate := calculateNextBillingDate(req.StartDate, req.IntervalValue, req.IntervalUnit)

	// Parse merchant ID
	merchantID, err := uuid.Parse(req.MerchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	// Parse customer ID
	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer_id format: %w", err)
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
			CustomerID:            customerID,
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
			return fmt.Errorf("failed to create subscription: %w", err)
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
		return nil, fmt.Errorf("invalid subscription_id format: %w", err)
	}

	// Get existing subscription
	existing, err := s.queries.GetSubscriptionByID(ctx, subID)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	// Ensure subscription is active or past_due
	if existing.Status != string(domain.SubscriptionStatusActive) &&
		existing.Status != string(domain.SubscriptionStatusPastDue) {
		return nil, fmt.Errorf("cannot update subscription in %s status", existing.Status)
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
				return fmt.Errorf("amount must be greater than zero")
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
				return fmt.Errorf("invalid payment_method_id format: %w", err)
			}

			// Verify payment method exists and belongs to customer
			pm, err := q.GetPaymentMethodByID(ctx, pmID)
			if err != nil {
				return fmt.Errorf("payment method not found: %w", err)
			}

			if pm.MerchantID.String() != existing.MerchantID.String() || pm.CustomerID != existing.CustomerID {
				return fmt.Errorf("payment method does not belong to customer")
			}

			if !pm.IsActive.Valid || !pm.IsActive.Bool {
				return fmt.Errorf("payment method is not active")
			}

			params.PaymentMethodID = pmID
		} else {
			params.PaymentMethodID = existing.PaymentMethodID
		}

		dbSub, err := q.UpdateSubscription(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to update subscription: %w", err)
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
		return nil, fmt.Errorf("invalid subscription_id format: %w", err)
	}

	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Get existing subscription
		existing, err := q.GetSubscriptionByID(ctx, subID)
		if err != nil {
			return fmt.Errorf("subscription not found: %w", err)
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
			return fmt.Errorf("failed to cancel subscription: %w", err)
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
		return nil, fmt.Errorf("invalid subscription_id format: %w", err)
	}

	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Get existing subscription
		existing, err := q.GetSubscriptionByID(ctx, subID)
		if err != nil {
			return fmt.Errorf("subscription not found: %w", err)
		}

		// Can only pause active subscriptions
		if existing.Status != string(domain.SubscriptionStatusActive) {
			return fmt.Errorf("cannot pause subscription in %s status", existing.Status)
		}

		params := sqlc.UpdateSubscriptionStatusParams{
			ID:     subID,
			Status: string(domain.SubscriptionStatusPaused),
		}

		dbSub, err := q.UpdateSubscriptionStatus(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to pause subscription: %w", err)
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
		return nil, fmt.Errorf("invalid subscription_id format: %w", err)
	}

	var subscription *domain.Subscription
	err = s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		// Get existing subscription
		existing, err := q.GetSubscriptionByID(ctx, subID)
		if err != nil {
			return fmt.Errorf("subscription not found: %w", err)
		}

		// Can only resume paused subscriptions
		if existing.Status != string(domain.SubscriptionStatusPaused) {
			return fmt.Errorf("cannot resume subscription in %s status", existing.Status)
		}

		params := sqlc.UpdateSubscriptionStatusParams{
			ID:     subID,
			Status: string(domain.SubscriptionStatusActive),
		}

		dbSub, err := q.UpdateSubscriptionStatus(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to resume subscription: %w", err)
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
		return nil, fmt.Errorf("invalid subscription_id format: %w", err)
	}

	dbSub, err := s.queries.GetSubscriptionByID(ctx, subID)
	if err != nil {
		s.logger.Debug("Subscription not found",
			zap.String("subscription_id", subscriptionID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("subscription not found: %w", err)
	}

	return sqlcSubscriptionToDomain(&dbSub), nil
}

// ListCustomerSubscriptions lists all subscriptions for a customer
func (s *subscriptionService) ListCustomerSubscriptions(ctx context.Context, merchantID, customerID string) ([]*domain.Subscription, error) {
	// Parse merchant ID
	merchantUUID, err := uuid.Parse(merchantID)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant_id format: %w", err)
	}

	params := sqlc.ListSubscriptionsByCustomerParams{
		MerchantID: merchantUUID,
		CustomerID: uuid.MustParse(customerID),
	}

	dbSubs, err := s.queries.ListSubscriptionsByCustomer(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions: %w", err)
	}

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
		return fmt.Errorf("invalid merchant_id: %w", err)
	}

	merchant, err := s.queries.GetMerchantByID(ctx, merchantID)
	if err != nil {
		return fmt.Errorf("failed to get merchant: %w", err)
	}

	if !merchant.IsActive {
		return fmt.Errorf("merchant is not active")
	}

	// Get payment method
	pm, err := s.queries.GetPaymentMethodByID(ctx, sub.PaymentMethodID)
	if err != nil {
		return fmt.Errorf("failed to get payment method: %w", err)
	}

	if !pm.IsActive.Valid || !pm.IsActive.Bool {
		return fmt.Errorf("payment method is not active")
	}

	// Get MAC secret for EPX request signing
	_, err = s.secretManager.GetSecret(ctx, merchant.MacSecretPath)
	if err != nil {
		return fmt.Errorf("failed to get MAC secret: %w", err)
	}

	// Prepare EPX request - convert cents back to decimal string
	amount := decimal.NewFromInt(sub.AmountCents).Div(decimal.NewFromInt(100))

	// Generate deterministic TRAN_NBR from transaction ID using FNV-1a hash
	// This ensures the same transaction ID always produces the same TRAN_NBR (idempotency)
	tranNbr := util.UUIDToEPXTranNbr(txID)

	// Recurring billing requires specific EPX fields:
	// - OriginalAuthGUID: the Storage BRIC (not AuthGUID)
	// - ACIExt: "RB" (Recurring Billing indicator)
	// - CardEntryMethod: "Z" (stored credential/token)
	aciExt := "RB"
	cardEntryMethod := "Z"
	industryType := "E" // E-commerce

	epxReq := &adapterports.ServerPostRequest{
		CustNbr:          merchant.CustNbr,
		MerchNbr:         merchant.MerchNbr,
		DBAnbr:           merchant.DbaNbr,
		TerminalNbr:      merchant.TerminalNbr,
		TransactionType:  adapterports.TransactionTypeSale,
		Amount:           amount.String(),
		PaymentType:      adapterports.PaymentMethodType(pm.PaymentType),
		OriginalAuthGUID: pm.Bric, // Use OriginalAuthGUID for stored BRIC
		TranNbr:          tranNbr,
		TranGroup:        uuid.New().String(),
		CustomerID:       sub.CustomerID.String(),
		ACIExt:           &aciExt,          // "RB" = Recurring Billing
		CardEntryMethod:  &cardEntryMethod, // "Z" = stored credential
		IndustryType:     &industryType,    // "E" = E-commerce
	}

	// Process transaction through EPX
	epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
	if err != nil {
		// Handle billing failure
		return s.handleBillingFailure(ctx, sub, err)
	}

	if !epxResp.IsApproved {
		// Handle declined transaction
		return s.handleBillingFailure(ctx, sub, fmt.Errorf("transaction declined: %s", epxResp.AuthRespText))
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
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		txParams := sqlc.CreateTransactionParams{
			ID:                  txID, // Use deterministic ID for idempotency
			MerchantID:          sub.MerchantID,
			CustomerID:          pgtype.UUID{Bytes: sub.CustomerID, Valid: true},
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
			return fmt.Errorf("failed to create transaction: %w", err)
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
			return fmt.Errorf("failed to update subscription: %w", err)
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
			return fmt.Errorf("failed to update subscription: %w", err)
		}

		return nil
	})
}

// handleBillingFailure handles a failed billing attempt
func (s *subscriptionService) handleBillingFailure(ctx context.Context, sub *sqlc.Subscription, billingErr error) error {
	return s.txManager.WithTx(ctx, func(q sqlc.Querier) error {
		newRetryCount := sub.FailureRetryCount + 1
		var newStatus string

		if newRetryCount >= sub.MaxRetries {
			// Max retries reached - mark as past_due
			newStatus = string(domain.SubscriptionStatusPastDue)
			s.logger.Warn("Subscription billing failed - max retries reached",
				zap.String("subscription_id", sub.ID.String()),
				zap.Int32("retry_count", newRetryCount),
				zap.Error(billingErr),
			)
		} else {
			// Still have retries remaining
			newStatus = string(domain.SubscriptionStatusActive)
			s.logger.Warn("Subscription billing failed - will retry",
				zap.String("subscription_id", sub.ID.String()),
				zap.Int32("retry_count", newRetryCount),
				zap.Int32("max_retries", sub.MaxRetries),
				zap.Error(billingErr),
			)
		}

		params := sqlc.IncrementSubscriptionFailureCountParams{
			ID:                sub.ID,
			FailureRetryCount: newRetryCount,
			Status:            newStatus,
		}

		_, err := q.IncrementSubscriptionFailureCount(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to update failure count: %w", err)
		}

		return billingErr
	})
}

// getSubscriptionByIdempotencyKey retrieves a subscription by idempotency key
func (s *subscriptionService) getSubscriptionByIdempotencyKey(ctx context.Context, key string) (*domain.Subscription, error) {
	// Note: This would require a separate SQL query if we want to support idempotency for subscriptions
	// For now, returning not found error
	return nil, fmt.Errorf("subscription not found")
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
		CustomerID:        dbSub.CustomerID.String(),
		AmountCents:       dbSub.AmountCents,
		Currency:          dbSub.Currency,
		IntervalValue:     int(dbSub.IntervalValue),
		IntervalUnit:      domain.IntervalUnit(dbSub.IntervalUnit),
		Status:            domain.SubscriptionStatus(dbSub.Status),
		PaymentMethodID:   dbSub.PaymentMethodID.String(),
		NextBillingDate:   dbSub.NextBillingDate.Time,
		FailureRetryCount: int(dbSub.FailureRetryCount),
		MaxRetries:        int(dbSub.MaxRetries),
		CreatedAt:         dbSub.CreatedAt,
		UpdatedAt:         dbSub.UpdatedAt,
	}

	if dbSub.CancelledAt.Valid {
		sub.CancelledAt = &dbSub.CancelledAt.Time
	}

	if dbSub.GatewaySubscriptionID.Valid {
		sub.GatewaySubscriptionID = &dbSub.GatewaySubscriptionID.String
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
