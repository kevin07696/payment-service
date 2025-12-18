package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSubscription_IsActive tests active status check
func TestSubscription_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   SubscriptionStatus
		expected bool
	}{
		{"active status returns true", SubscriptionStatusActive, true},
		{"paused status returns false", SubscriptionStatusPaused, false},
		{"cancelled status returns false", SubscriptionStatusCancelled, false},
		{"past_due status returns false", SubscriptionStatusPastDue, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{Status: tt.status}
			assert.Equal(t, tt.expected, sub.IsActive())
		})
	}
}

// TestSubscription_IsCancelled tests cancelled status check
func TestSubscription_IsCancelled(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		status      SubscriptionStatus
		cancelledAt *time.Time
		expected    bool
	}{
		{"cancelled status with timestamp", SubscriptionStatusCancelled, &now, true},
		{"cancelled status without timestamp", SubscriptionStatusCancelled, nil, true},
		{"active status with cancelled timestamp", SubscriptionStatusActive, &now, true},
		{"active status without cancelled timestamp", SubscriptionStatusActive, nil, false},
		{"paused status without cancelled timestamp", SubscriptionStatusPaused, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				Status:      tt.status,
				CancelledAt: tt.cancelledAt,
			}
			assert.Equal(t, tt.expected, sub.IsCancelled())
		})
	}
}

// TestSubscription_CanBeBilled tests billing eligibility
func TestSubscription_CanBeBilled(t *testing.T) {
	pastDate := time.Now().Add(-24 * time.Hour)
	futureDate := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name            string
		status          SubscriptionStatus
		nextBillingDate time.Time
		expected        bool
	}{
		{"active subscription past due date", SubscriptionStatusActive, pastDate, true},
		{"active subscription future date", SubscriptionStatusActive, futureDate, false},
		{"paused subscription past due date", SubscriptionStatusPaused, pastDate, false},
		{"cancelled subscription past due date", SubscriptionStatusCancelled, pastDate, false},
		{"past_due subscription past due date", SubscriptionStatusPastDue, pastDate, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				Status:          tt.status,
				NextBillingDate: tt.nextBillingDate,
			}
			assert.Equal(t, tt.expected, sub.CanBeBilled())
		})
	}
}

// TestSubscription_ShouldRetry tests retry logic
func TestSubscription_ShouldRetry(t *testing.T) {
	tests := []struct {
		name              string
		failureRetryCount int
		maxRetries        int
		expected          bool
	}{
		{"no retries yet", 0, 3, true},
		{"one retry of three", 1, 3, true},
		{"at max retries", 3, 3, false},
		{"exceeded max retries", 4, 3, false},
		{"max retries is zero", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				FailureRetryCount: tt.failureRetryCount,
				MaxRetries:        tt.maxRetries,
			}
			assert.Equal(t, tt.expected, sub.ShouldRetry())
		})
	}
}

// TestSubscription_IncrementRetryCount tests retry counter increment
func TestSubscription_IncrementRetryCount(t *testing.T) {
	tests := []struct {
		name               string
		initialRetryCount  int
		maxRetries         int
		expectedRetryCount int
		expectedStatus     SubscriptionStatus
	}{
		{
			name:               "first retry increments counter",
			initialRetryCount:  0,
			maxRetries:         3,
			expectedRetryCount: 1,
			expectedStatus:     SubscriptionStatusActive,
		},
		{
			name:               "reaching max retries changes status to past_due",
			initialRetryCount:  2,
			maxRetries:         3,
			expectedRetryCount: 3,
			expectedStatus:     SubscriptionStatusPastDue,
		},
		{
			name:               "exceeding max retries keeps past_due status",
			initialRetryCount:  3,
			maxRetries:         3,
			expectedRetryCount: 4,
			expectedStatus:     SubscriptionStatusPastDue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				Status:            SubscriptionStatusActive,
				FailureRetryCount: tt.initialRetryCount,
				MaxRetries:        tt.maxRetries,
			}

			sub.IncrementRetryCount()

			assert.Equal(t, tt.expectedRetryCount, sub.FailureRetryCount)
			assert.Equal(t, tt.expectedStatus, sub.Status)
		})
	}
}

// TestSubscription_ResetRetryCount tests retry counter reset
func TestSubscription_ResetRetryCount(t *testing.T) {
	sub := &Subscription{
		FailureRetryCount: 5,
		MaxRetries:        3,
	}

	sub.ResetRetryCount()

	assert.Equal(t, 0, sub.FailureRetryCount, "Retry count should reset to 0")
}

// TestSubscription_CalculateNextBillingDate tests billing date calculation
func TestSubscription_CalculateNextBillingDate(t *testing.T) {
	baseDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		intervalValue int
		intervalUnit  IntervalUnit
		expected      time.Time
	}{
		{
			name:          "daily interval",
			intervalValue: 7,
			intervalUnit:  IntervalUnitDay,
			expected:      time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "weekly interval",
			intervalValue: 2,
			intervalUnit:  IntervalUnitWeek,
			expected:      time.Date(2025, 1, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "monthly interval",
			intervalValue: 1,
			intervalUnit:  IntervalUnitMonth,
			expected:      time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "quarterly interval",
			intervalValue: 3,
			intervalUnit:  IntervalUnitMonth,
			expected:      time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "yearly interval",
			intervalValue: 1,
			intervalUnit:  IntervalUnitYear,
			expected:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:          "invalid interval defaults to monthly",
			intervalValue: 1,
			intervalUnit:  IntervalUnit("invalid"),
			expected:      time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				NextBillingDate: baseDate,
				IntervalValue:   tt.intervalValue,
				IntervalUnit:    tt.intervalUnit,
			}

			result := sub.CalculateNextBillingDate()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSubscription_GetIntervalDescription tests interval description formatting
func TestSubscription_GetIntervalDescription(t *testing.T) {
	tests := []struct {
		name          string
		intervalValue int
		intervalUnit  IntervalUnit
		expected      string
	}{
		{"singular day", 1, IntervalUnitDay, "day"},
		{"plural days", 7, IntervalUnitDay, "7 days"},
		{"singular week", 1, IntervalUnitWeek, "week"},
		{"plural weeks", 2, IntervalUnitWeek, "2 weeks"},
		{"singular month", 1, IntervalUnitMonth, "month"},
		{"plural months", 3, IntervalUnitMonth, "3 months"},
		{"singular year", 1, IntervalUnitYear, "year"},
		{"plural years", 2, IntervalUnitYear, "2 years"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				IntervalValue: tt.intervalValue,
				IntervalUnit:  tt.intervalUnit,
			}

			result := sub.GetIntervalDescription()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSubscription_BusinessLogic_CompleteWorkflow tests realistic subscription lifecycle
func TestSubscription_BusinessLogic_CompleteWorkflow(t *testing.T) {
	t.Run("successful billing cycle", func(t *testing.T) {
		sub := &Subscription{
			Status:            SubscriptionStatusActive,
			NextBillingDate:   time.Now().Add(-1 * time.Hour), // Past due
			IntervalValue:     1,
			IntervalUnit:      IntervalUnitMonth,
			FailureRetryCount: 0,
			MaxRetries:        3,
		}

		// Check eligibility
		assert.True(t, sub.CanBeBilled(), "Should be eligible for billing")
		assert.True(t, sub.IsActive(), "Should be active")

		// Calculate next billing
		nextDate := sub.CalculateNextBillingDate()
		assert.True(t, nextDate.After(time.Now()), "Next date should be in future")
	})

	t.Run("failed billing with retries", func(t *testing.T) {
		sub := &Subscription{
			Status:            SubscriptionStatusActive,
			FailureRetryCount: 0,
			MaxRetries:        3,
		}

		// First failure
		assert.True(t, sub.ShouldRetry())
		sub.IncrementRetryCount()
		assert.Equal(t, 1, sub.FailureRetryCount)
		assert.Equal(t, SubscriptionStatusActive, sub.Status)

		// Second failure
		assert.True(t, sub.ShouldRetry())
		sub.IncrementRetryCount()
		assert.Equal(t, 2, sub.FailureRetryCount)
		assert.Equal(t, SubscriptionStatusActive, sub.Status)

		// Third failure - reaches max retries
		assert.True(t, sub.ShouldRetry())
		sub.IncrementRetryCount()
		assert.Equal(t, 3, sub.FailureRetryCount)
		assert.Equal(t, SubscriptionStatusPastDue, sub.Status)

		// Fourth failure - no more retries
		assert.False(t, sub.ShouldRetry())
	})

	t.Run("recovery after failure", func(t *testing.T) {
		sub := &Subscription{
			Status:            SubscriptionStatusActive,
			FailureRetryCount: 2,
			MaxRetries:        3,
		}

		// Billing succeeds
		sub.ResetRetryCount()

		assert.Equal(t, 0, sub.FailureRetryCount)
		assert.True(t, sub.ShouldRetry(), "Should be eligible for retries again")
	})

	t.Run("cancelled subscription cannot be billed", func(t *testing.T) {
		now := time.Now()
		sub := &Subscription{
			Status:          SubscriptionStatusCancelled,
			CancelledAt:     &now,
			NextBillingDate: time.Now().Add(-24 * time.Hour),
		}

		assert.True(t, sub.IsCancelled())
		assert.False(t, sub.IsActive())
		assert.False(t, sub.CanBeBilled())
	})
}
