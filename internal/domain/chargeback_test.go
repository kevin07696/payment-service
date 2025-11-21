package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChargeback_IsOpen tests open status check
func TestChargeback_IsOpen(t *testing.T) {
	tests := []struct {
		name     string
		status   ChargebackStatus
		expected bool
	}{
		{"new status is open", ChargebackStatusNew, true},
		{"pending status is open", ChargebackStatusPending, true},
		{"responded status is not open", ChargebackStatusResponded, false},
		{"won status is not open", ChargebackStatusWon, false},
		{"lost status is not open", ChargebackStatusLost, false},
		{"accepted status is not open", ChargebackStatusAccepted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Chargeback{Status: tt.status}
			assert.Equal(t, tt.expected, cb.IsOpen())
		})
	}
}

// TestChargeback_IsResolved tests resolved status check
func TestChargeback_IsResolved(t *testing.T) {
	tests := []struct {
		name     string
		status   ChargebackStatus
		expected bool
	}{
		{"won status is resolved", ChargebackStatusWon, true},
		{"lost status is resolved", ChargebackStatusLost, true},
		{"accepted status is resolved", ChargebackStatusAccepted, true},
		{"new status is not resolved", ChargebackStatusNew, false},
		{"pending status is not resolved", ChargebackStatusPending, false},
		{"responded status is not resolved", ChargebackStatusResponded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Chargeback{Status: tt.status}
			assert.Equal(t, tt.expected, cb.IsResolved())
		})
	}
}

// TestChargeback_CanRespond tests response eligibility
func TestChargeback_CanRespond(t *testing.T) {
	pastDate := time.Now().Add(-24 * time.Hour)
	futureDate := time.Now().Add(24 * time.Hour)
	responseTime := time.Now()

	tests := []struct {
		name                string
		status              ChargebackStatus
		respondByDate       *time.Time
		responseSubmittedAt *time.Time
		expected            bool
	}{
		{
			name:          "new chargeback with future deadline",
			status:        ChargebackStatusNew,
			respondByDate: &futureDate,
			expected:      true,
		},
		{
			name:          "pending chargeback with future deadline",
			status:        ChargebackStatusPending,
			respondByDate: &futureDate,
			expected:      true,
		},
		{
			name:          "new chargeback with past deadline",
			status:        ChargebackStatusNew,
			respondByDate: &pastDate,
			expected:      false,
		},
		{
			name:                "new chargeback already responded",
			status:              ChargebackStatusNew,
			respondByDate:       &futureDate,
			responseSubmittedAt: &responseTime,
			expected:            false,
		},
		{
			name:          "resolved chargeback cannot respond",
			status:        ChargebackStatusWon,
			respondByDate: &futureDate,
			expected:      false,
		},
		{
			name:          "new chargeback with no deadline",
			status:        ChargebackStatusNew,
			respondByDate: nil,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Chargeback{
				Status:              tt.status,
				RespondByDate:       tt.respondByDate,
				ResponseSubmittedAt: tt.responseSubmittedAt,
			}
			assert.Equal(t, tt.expected, cb.CanRespond())
		})
	}
}

// TestChargeback_IsOverdue tests overdue status check
func TestChargeback_IsOverdue(t *testing.T) {
	pastDate := time.Now().Add(-24 * time.Hour)
	futureDate := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name          string
		status        ChargebackStatus
		respondByDate *time.Time
		expected      bool
	}{
		{
			name:          "open chargeback past deadline",
			status:        ChargebackStatusNew,
			respondByDate: &pastDate,
			expected:      true,
		},
		{
			name:          "open chargeback before deadline",
			status:        ChargebackStatusNew,
			respondByDate: &futureDate,
			expected:      false,
		},
		{
			name:          "resolved chargeback not overdue",
			status:        ChargebackStatusWon,
			respondByDate: &pastDate,
			expected:      false,
		},
		{
			name:          "no deadline set",
			status:        ChargebackStatusNew,
			respondByDate: nil,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Chargeback{
				Status:        tt.status,
				RespondByDate: tt.respondByDate,
			}
			assert.Equal(t, tt.expected, cb.IsOverdue())
		})
	}
}

// TestChargeback_DaysUntilDeadline tests deadline calculation
func TestChargeback_DaysUntilDeadline(t *testing.T) {
	tests := []struct {
		name          string
		respondByDate *time.Time
		expected      int
	}{
		{
			name:          "no deadline",
			respondByDate: nil,
			expected:      0,
		},
		{
			name: "deadline tomorrow",
			respondByDate: func() *time.Time {
				d := time.Now().Add(25 * time.Hour)
				return &d
			}(),
			expected: 1,
		},
		{
			name: "deadline in 7 days",
			respondByDate: func() *time.Time {
				d := time.Now().Add((7 * 24 * time.Hour) + 1*time.Hour) // Add extra hour for rounding
				return &d
			}(),
			expected: 7,
		},
		{
			name: "deadline passed yesterday",
			respondByDate: func() *time.Time {
				d := time.Now().Add(-25 * time.Hour)
				return &d
			}(),
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Chargeback{
				RespondByDate: tt.respondByDate,
			}
			result := cb.DaysUntilDeadline()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestChargeback_MarkResponded tests response submission
func TestChargeback_MarkResponded(t *testing.T) {
	cb := &Chargeback{
		Status:              ChargebackStatusNew,
		ResponseSubmittedAt: nil,
	}

	beforeUpdate := cb.UpdatedAt

	// Small delay to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)

	cb.MarkResponded()

	assert.Equal(t, ChargebackStatusResponded, cb.Status)
	require.NotNil(t, cb.ResponseSubmittedAt)
	assert.True(t, cb.ResponseSubmittedAt.After(beforeUpdate))
	assert.True(t, cb.UpdatedAt.After(beforeUpdate))
}

// TestChargeback_MarkResolved tests resolution status setting
func TestChargeback_MarkResolved(t *testing.T) {
	tests := []struct {
		name        string
		status      ChargebackStatus
		expectError bool
	}{
		{"won is valid", ChargebackStatusWon, false},
		{"lost is valid", ChargebackStatusLost, false},
		{"accepted is valid", ChargebackStatusAccepted, false},
		{"new is invalid", ChargebackStatusNew, true},
		{"pending is invalid", ChargebackStatusPending, true},
		{"responded is invalid", ChargebackStatusResponded, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Chargeback{
				Status:     ChargebackStatusNew,
				ResolvedAt: nil,
			}

			beforeUpdate := cb.UpdatedAt
			time.Sleep(1 * time.Millisecond)

			err := cb.MarkResolved(tt.status)

			if tt.expectError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidChargebackStatus)
				assert.Nil(t, cb.ResolvedAt)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.status, cb.Status)
				require.NotNil(t, cb.ResolvedAt)
				assert.True(t, cb.ResolvedAt.After(beforeUpdate))
				assert.True(t, cb.UpdatedAt.After(beforeUpdate))
			}
		})
	}
}

// TestChargeback_GetCustomerID tests customer ID retrieval
func TestChargeback_GetCustomerID(t *testing.T) {
	customerID := "cust_123456"

	tests := []struct {
		name       string
		customerID *string
		expected   string
	}{
		{"returns customer ID when present", &customerID, "cust_123456"},
		{"returns empty string when nil", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &Chargeback{
				CustomerID: tt.customerID,
			}
			assert.Equal(t, tt.expected, cb.GetCustomerID())
		})
	}
}

// TestChargeback_BusinessLogic_CompleteWorkflow tests realistic chargeback lifecycle
func TestChargeback_BusinessLogic_CompleteWorkflow(t *testing.T) {
	t.Run("successful response and resolution", func(t *testing.T) {
		deadline := time.Now().Add(14 * 24 * time.Hour) // 14 days from now
		cb := &Chargeback{
			Status:        ChargebackStatusNew,
			RespondByDate: &deadline,
		}

		// Initially open and can respond
		assert.True(t, cb.IsOpen())
		assert.False(t, cb.IsResolved())
		assert.True(t, cb.CanRespond())
		assert.False(t, cb.IsOverdue())

		// Check deadline
		days := cb.DaysUntilDeadline()
		assert.True(t, days > 0, "Should have positive days until deadline")

		// Submit response
		cb.MarkResponded()
		assert.Equal(t, ChargebackStatusResponded, cb.Status)
		assert.False(t, cb.CanRespond(), "Cannot respond again")

		// Resolve as won
		err := cb.MarkResolved(ChargebackStatusWon)
		require.NoError(t, err)
		assert.True(t, cb.IsResolved())
		assert.False(t, cb.IsOpen())
	})

	t.Run("overdue chargeback", func(t *testing.T) {
		pastDeadline := time.Now().Add(-7 * 24 * time.Hour) // 7 days ago
		cb := &Chargeback{
			Status:        ChargebackStatusNew,
			RespondByDate: &pastDeadline,
		}

		assert.True(t, cb.IsOpen())
		assert.True(t, cb.IsOverdue())
		assert.False(t, cb.CanRespond(), "Cannot respond after deadline")

		days := cb.DaysUntilDeadline()
		assert.True(t, days < 0, "Should have negative days (overdue)")
	})

	t.Run("accepted without response", func(t *testing.T) {
		cb := &Chargeback{
			Status: ChargebackStatusNew,
		}

		// Accept without responding
		err := cb.MarkResolved(ChargebackStatusAccepted)
		require.NoError(t, err)
		assert.Equal(t, ChargebackStatusAccepted, cb.Status)
		assert.True(t, cb.IsResolved())
		assert.Nil(t, cb.ResponseSubmittedAt, "No response was submitted")
	})

	t.Run("invalid resolution attempt", func(t *testing.T) {
		cb := &Chargeback{
			Status: ChargebackStatusNew,
		}

		// Try to set invalid status
		err := cb.MarkResolved(ChargebackStatusPending)
		assert.Error(t, err)
		assert.Equal(t, ChargebackStatusNew, cb.Status, "Status should remain unchanged")
	})

	t.Run("customer ID handling", func(t *testing.T) {
		customerID := "cust_abc123"
		cbWithCustomer := &Chargeback{CustomerID: &customerID}
		cbWithoutCustomer := &Chargeback{CustomerID: nil}

		assert.Equal(t, "cust_abc123", cbWithCustomer.GetCustomerID())
		assert.Equal(t, "", cbWithoutCustomer.GetCustomerID())
	})
}

// TestChargeback_EdgeCases tests boundary conditions
func TestChargeback_EdgeCases(t *testing.T) {
	t.Run("multiple state transitions", func(t *testing.T) {
		cb := &Chargeback{
			Status: ChargebackStatusNew,
		}

		// New -> Responded
		cb.MarkResponded()
		assert.Equal(t, ChargebackStatusResponded, cb.Status)
		assert.False(t, cb.CanRespond())

		// Responded -> Won
		err := cb.MarkResolved(ChargebackStatusWon)
		require.NoError(t, err)
		assert.True(t, cb.IsResolved())
	})

	t.Run("already responded chargeback", func(t *testing.T) {
		responseTime := time.Now()
		cb := &Chargeback{
			Status:              ChargebackStatusNew,
			ResponseSubmittedAt: &responseTime,
		}

		assert.False(t, cb.CanRespond(), "Cannot respond twice")
	})

	t.Run("resolved chargebacks remain resolved", func(t *testing.T) {
		cb := &Chargeback{Status: ChargebackStatusWon}

		// Try to respond after resolution
		assert.False(t, cb.CanRespond())
		assert.False(t, cb.IsOpen())
		assert.True(t, cb.IsResolved())
	})
}
