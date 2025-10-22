package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
)

// Service implements ports.SubscriptionService
type Service struct {
	db              ports.DBPort
	subRepo         ports.SubscriptionRepository
	paymentService  ports.PaymentService
	recurringGateway ports.RecurringBillingGateway
	logger          ports.Logger
}

// NewService creates a new subscription service
func NewService(
	db ports.DBPort,
	subRepo ports.SubscriptionRepository,
	paymentService ports.PaymentService,
	recurringGateway ports.RecurringBillingGateway,
	logger ports.Logger,
) *Service {
	return &Service{
		db:              db,
		subRepo:         subRepo,
		paymentService:  paymentService,
		recurringGateway: recurringGateway,
		logger:          logger,
	}
}

// CreateSubscription creates a new recurring billing subscription
func (s *Service) CreateSubscription(ctx context.Context, req ports.ServiceCreateSubscriptionRequest) (*ports.ServiceSubscriptionResponse, error) {
	// Calculate first billing date
	nextBillingDate := s.calculateNextBillingDate(req.StartDate, req.Frequency)

	subscription := &models.Subscription{
		ID:                 uuid.New().String(),
		MerchantID:         req.MerchantID,
		CustomerID:         req.CustomerID,
		Amount:             req.Amount,
		Currency:           req.Currency,
		Frequency:          req.Frequency,
		Status:             models.SubStatusActive,
		PaymentMethodToken: req.PaymentMethodToken,
		NextBillingDate:    nextBillingDate,
		FailureRetryCount:  0,
		MaxRetries:         req.MaxRetries,
		FailureOption:      req.FailureOption,
		Metadata:           req.Metadata,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	var response *ports.ServiceSubscriptionResponse

	err := s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Create subscription in database
		if err := s.subRepo.Create(ctx, tx, subscription); err != nil {
			return fmt.Errorf("create subscription: %w", err)
		}

		// Optionally create gateway subscription (for gateway-managed recurring billing)
		if s.recurringGateway != nil {
			gatewayReq := &ports.SubscriptionRequest{
				CustomerID:       req.CustomerID,
				Amount:           req.Amount,
				Currency:         req.Currency,
				Frequency:        req.Frequency,
				PaymentToken:     req.PaymentMethodToken,
				StartDate:        req.StartDate,
				MaxRetries:       req.MaxRetries,
				FailureOption:    req.FailureOption,
				Metadata:         req.Metadata,
			}

			gatewayResp, err := s.recurringGateway.CreateSubscription(ctx, gatewayReq)
			if err != nil {
				s.logger.Warn("gateway subscription creation failed, continuing with local subscription",
					ports.String("subscription_id", subscription.ID),
					ports.String("error", err.Error()))
			} else {
				subscription.GatewaySubscriptionID = gatewayResp.GatewaySubscriptionID
				// Update subscription with gateway ID
				if err := s.subRepo.Update(ctx, tx, subscription); err != nil {
					return fmt.Errorf("update subscription with gateway ID: %w", err)
				}
			}
		}

		response = s.toSubscriptionResponse(subscription)
		return nil
	})

	if err != nil {
		s.logger.Error("create subscription failed",
			ports.String("merchant_id", req.MerchantID),
			ports.String("customer_id", req.CustomerID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("subscription created",
		ports.String("subscription_id", subscription.ID),
		ports.String("customer_id", req.CustomerID),
		ports.String("frequency", string(req.Frequency)),
		ports.String("next_billing", nextBillingDate.Format(time.RFC3339)))

	return response, nil
}

// UpdateSubscription updates subscription properties
func (s *Service) UpdateSubscription(ctx context.Context, req ports.ServiceUpdateSubscriptionRequest) (*ports.ServiceSubscriptionResponse, error) {
	// Get existing subscription
	subID, err := uuid.Parse(req.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription ID: %w", err)
	}

	subscription, err := s.subRepo.GetByID(ctx, nil, subID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	// Validate subscription can be updated
	if subscription.Status == models.SubStatusCancelled {
		return nil, fmt.Errorf("cannot update cancelled subscription")
	}

	var response *ports.ServiceSubscriptionResponse

	err = s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Update fields if provided
		if req.Amount != nil {
			subscription.Amount = *req.Amount
		}
		if req.Frequency != nil {
			subscription.Frequency = *req.Frequency
			// Recalculate next billing date with new frequency
			subscription.NextBillingDate = s.calculateNextBillingDate(time.Now(), subscription.Frequency)
		}
		if req.PaymentMethodToken != nil {
			subscription.PaymentMethodToken = *req.PaymentMethodToken
		}

		subscription.UpdatedAt = time.Now()

		// Update in database
		if err := s.subRepo.Update(ctx, tx, subscription); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		// Update gateway subscription if exists
		if s.recurringGateway != nil && subscription.GatewaySubscriptionID != "" {
			updateReq := &ports.UpdateSubscriptionRequest{
				Amount:       req.Amount,
				Frequency:    req.Frequency,
				PaymentToken: req.PaymentMethodToken,
			}

			_, err := s.recurringGateway.UpdateSubscription(ctx, subscription.GatewaySubscriptionID, updateReq)
			if err != nil {
				s.logger.Warn("gateway subscription update failed",
					ports.String("subscription_id", subscription.ID),
					ports.String("error", err.Error()))
			}
		}

		response = s.toSubscriptionResponse(subscription)
		return nil
	})

	if err != nil {
		s.logger.Error("update subscription failed",
			ports.String("subscription_id", req.SubscriptionID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("subscription updated",
		ports.String("subscription_id", subscription.ID))

	return response, nil
}

// CancelSubscription cancels an active subscription
func (s *Service) CancelSubscription(ctx context.Context, req ports.ServiceCancelSubscriptionRequest) (*ports.ServiceSubscriptionResponse, error) {
	subID, err := uuid.Parse(req.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription ID: %w", err)
	}

	subscription, err := s.subRepo.GetByID(ctx, nil, subID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	// Validate subscription can be cancelled
	if subscription.Status == models.SubStatusCancelled {
		return nil, fmt.Errorf("subscription already cancelled")
	}

	var response *ports.ServiceSubscriptionResponse

	err = s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		now := time.Now()
		subscription.Status = models.SubStatusCancelled
		subscription.CancelledAt = &now
		subscription.UpdatedAt = now

		if err := s.subRepo.Update(ctx, tx, subscription); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		// Cancel gateway subscription if exists
		if s.recurringGateway != nil && subscription.GatewaySubscriptionID != "" {
			_, err := s.recurringGateway.CancelSubscription(ctx, subscription.GatewaySubscriptionID, true)
			if err != nil {
				s.logger.Warn("gateway subscription cancellation failed",
					ports.String("subscription_id", subscription.ID),
					ports.String("error", err.Error()))
			}
		}

		response = s.toSubscriptionResponse(subscription)
		return nil
	})

	if err != nil {
		s.logger.Error("cancel subscription failed",
			ports.String("subscription_id", req.SubscriptionID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("subscription cancelled",
		ports.String("subscription_id", subscription.ID),
		ports.String("reason", req.Reason))

	return response, nil
}

// PauseSubscription pauses an active subscription
func (s *Service) PauseSubscription(ctx context.Context, subscriptionID string) (*ports.ServiceSubscriptionResponse, error) {
	subID, err := uuid.Parse(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription ID: %w", err)
	}

	subscription, err := s.subRepo.GetByID(ctx, nil, subID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	if subscription.Status != models.SubStatusActive {
		return nil, fmt.Errorf("can only pause active subscriptions")
	}

	var response *ports.ServiceSubscriptionResponse

	err = s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		subscription.Status = models.SubStatusPaused
		subscription.UpdatedAt = time.Now()

		if err := s.subRepo.Update(ctx, tx, subscription); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		// Pause gateway subscription if exists
		if s.recurringGateway != nil && subscription.GatewaySubscriptionID != "" {
			_, err := s.recurringGateway.PauseSubscription(ctx, subscription.GatewaySubscriptionID)
			if err != nil {
				s.logger.Warn("gateway subscription pause failed",
					ports.String("subscription_id", subscription.ID),
					ports.String("error", err.Error()))
			}
		}

		response = s.toSubscriptionResponse(subscription)
		return nil
	})

	if err != nil {
		s.logger.Error("pause subscription failed",
			ports.String("subscription_id", subscriptionID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("subscription paused",
		ports.String("subscription_id", subscription.ID))

	return response, nil
}

// ResumeSubscription resumes a paused subscription
func (s *Service) ResumeSubscription(ctx context.Context, subscriptionID string) (*ports.ServiceSubscriptionResponse, error) {
	subID, err := uuid.Parse(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription ID: %w", err)
	}

	subscription, err := s.subRepo.GetByID(ctx, nil, subID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}

	if subscription.Status != models.SubStatusPaused {
		return nil, fmt.Errorf("can only resume paused subscriptions")
	}

	var response *ports.ServiceSubscriptionResponse

	err = s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		subscription.Status = models.SubStatusActive
		subscription.UpdatedAt = time.Now()
		// Recalculate next billing date from now
		subscription.NextBillingDate = s.calculateNextBillingDate(time.Now(), subscription.Frequency)

		if err := s.subRepo.Update(ctx, tx, subscription); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		// Resume gateway subscription if exists
		if s.recurringGateway != nil && subscription.GatewaySubscriptionID != "" {
			_, err := s.recurringGateway.ResumeSubscription(ctx, subscription.GatewaySubscriptionID)
			if err != nil {
				s.logger.Warn("gateway subscription resume failed",
					ports.String("subscription_id", subscription.ID),
					ports.String("error", err.Error()))
			}
		}

		response = s.toSubscriptionResponse(subscription)
		return nil
	})

	if err != nil {
		s.logger.Error("resume subscription failed",
			ports.String("subscription_id", subscriptionID),
			ports.String("error", err.Error()))
		return nil, err
	}

	s.logger.Info("subscription resumed",
		ports.String("subscription_id", subscription.ID),
		ports.String("next_billing", subscription.NextBillingDate.Format(time.RFC3339)))

	return response, nil
}

// GetSubscription retrieves a subscription by ID
func (s *Service) GetSubscription(ctx context.Context, subscriptionID string) (*models.Subscription, error) {
	subID, err := uuid.Parse(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("invalid subscription ID: %w", err)
	}

	return s.subRepo.GetByID(ctx, nil, subID)
}

// ListCustomerSubscriptions lists all subscriptions for a customer
func (s *Service) ListCustomerSubscriptions(ctx context.Context, merchantID, customerID string) ([]*models.Subscription, error) {
	return s.subRepo.ListByCustomer(ctx, nil, merchantID, customerID)
}

// ProcessDueBilling processes all subscriptions due for billing
func (s *Service) ProcessDueBilling(ctx context.Context, asOfDate time.Time, batchSize int) (*ports.BillingBatchResult, error) {
	result := &ports.BillingBatchResult{
		Errors: make([]ports.BillingError, 0),
	}

	// Get subscriptions due for billing
	subscriptions, err := s.subRepo.ListActiveSubscriptionsDueForBilling(ctx, nil, asOfDate, int32(batchSize))
	if err != nil {
		return nil, fmt.Errorf("list subscriptions due for billing: %w", err)
	}

	result.ProcessedCount = len(subscriptions)

	s.logger.Info("processing billing batch",
		ports.String("as_of_date", asOfDate.Format(time.RFC3339)),
		ports.String("count", fmt.Sprintf("%d", len(subscriptions))))

	// Process each subscription
	for _, sub := range subscriptions {
		err := s.processSingleBilling(ctx, sub)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, ports.BillingError{
				SubscriptionID: sub.ID,
				CustomerID:     sub.CustomerID,
				Error:          err.Error(),
				Retriable:      true,
			})
			s.logger.Error("billing failed for subscription",
				ports.String("subscription_id", sub.ID),
				ports.String("customer_id", sub.CustomerID),
				ports.String("error", err.Error()))
		} else {
			result.SuccessCount++
		}
	}

	s.logger.Info("billing batch completed",
		ports.String("processed", fmt.Sprintf("%d", result.ProcessedCount)),
		ports.String("success", fmt.Sprintf("%d", result.SuccessCount)),
		ports.String("failed", fmt.Sprintf("%d", result.FailedCount)))

	return result, nil
}

// processSingleBilling processes billing for a single subscription
func (s *Service) processSingleBilling(ctx context.Context, sub *models.Subscription) error {
	// Charge the customer using Payment Service
	saleReq := ports.ServiceSaleRequest{
		MerchantID: sub.MerchantID,
		CustomerID: sub.CustomerID,
		Amount:     sub.Amount,
		Currency:   sub.Currency,
		Token:      sub.PaymentMethodToken,
		BillingInfo: models.BillingInfo{}, // Subscription already has customer info
		IdempotencyKey: fmt.Sprintf("sub-%s-%s", sub.ID, sub.NextBillingDate.Format("2006-01-02")),
		Metadata: map[string]string{
			"subscription_id":   sub.ID,
			"billing_period":    sub.NextBillingDate.Format("2006-01-02"),
			"billing_frequency": string(sub.Frequency),
		},
	}

	paymentResp, err := s.paymentService.Sale(ctx, saleReq)
	if err != nil || (paymentResp != nil && !paymentResp.IsApproved) {
		// Update subscription failure state
		updateErr := s.handleBillingFailure(ctx, sub, err)
		if updateErr != nil {
			return fmt.Errorf("payment failed and failed to update subscription: %w", updateErr)
		}
		// Return the original payment error so ProcessDueBilling counts this as failed
		if err != nil {
			return fmt.Errorf("payment failed: %w", err)
		}
		return fmt.Errorf("payment declined")
	}

	// Billing successful - update subscription
	return s.handleBillingSuccess(ctx, sub)
}

// handleBillingSuccess updates subscription after successful billing
func (s *Service) handleBillingSuccess(ctx context.Context, sub *models.Subscription) error {
	return s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// Reset failure count
		sub.FailureRetryCount = 0
		// Calculate next billing date
		sub.NextBillingDate = s.calculateNextBillingDate(sub.NextBillingDate, sub.Frequency)
		sub.UpdatedAt = time.Now()

		return s.subRepo.Update(ctx, tx, sub)
	})
}

// handleBillingFailure handles subscription billing failures
func (s *Service) handleBillingFailure(ctx context.Context, sub *models.Subscription, billingErr error) error {
	return s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
		sub.FailureRetryCount++
		sub.UpdatedAt = time.Now()

		// Check if max retries exceeded
		if sub.FailureRetryCount >= sub.MaxRetries {
			switch sub.FailureOption {
			case models.FailureForward:
				// Move billing date forward
				sub.NextBillingDate = s.calculateNextBillingDate(sub.NextBillingDate, sub.Frequency)
				sub.FailureRetryCount = 0 // Reset for next period
			case models.FailureSkip:
				// Skip this billing cycle
				sub.NextBillingDate = s.calculateNextBillingDate(sub.NextBillingDate, sub.Frequency)
				sub.FailureRetryCount = 0
			case models.FailurePause:
				// Pause subscription
				sub.Status = models.SubStatusPaused
			}
		}

		return s.subRepo.Update(ctx, tx, sub)
	})
}

// calculateNextBillingDate calculates the next billing date based on frequency
func (s *Service) calculateNextBillingDate(fromDate time.Time, frequency models.BillingFrequency) time.Time {
	switch frequency {
	case models.FrequencyWeekly:
		return fromDate.AddDate(0, 0, 7)
	case models.FrequencyBiWeekly:
		return fromDate.AddDate(0, 0, 14)
	case models.FrequencyMonthly:
		return fromDate.AddDate(0, 1, 0)
	case models.FrequencyYearly:
		return fromDate.AddDate(1, 0, 0)
	default:
		return fromDate.AddDate(0, 1, 0) // Default to monthly
	}
}

// toSubscriptionResponse converts a subscription model to a response
func (s *Service) toSubscriptionResponse(sub *models.Subscription) *ports.ServiceSubscriptionResponse {
	return &ports.ServiceSubscriptionResponse{
		SubscriptionID:        sub.ID,
		MerchantID:            sub.MerchantID,
		CustomerID:            sub.CustomerID,
		Amount:                sub.Amount,
		Currency:              sub.Currency,
		Frequency:             sub.Frequency,
		Status:                sub.Status,
		PaymentMethodToken:    sub.PaymentMethodToken,
		NextBillingDate:       sub.NextBillingDate,
		GatewaySubscriptionID: sub.GatewaySubscriptionID,
		CreatedAt:             sub.CreatedAt,
		UpdatedAt:             sub.UpdatedAt,
		CancelledAt:           sub.CancelledAt,
	}
}
