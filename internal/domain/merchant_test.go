package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test merchants with default values
func newTestMerchant() *Merchant {
	return &Merchant{
		ID:            "test-id-123",
		AgentID:       "agent-456",
		CustNbr:       "12345",
		MerchNbr:      "67890",
		DBAnbr:        "11111",
		TerminalNbr:   "22222",
		MACSecretPath: "payment-service/merchants/test-id-123/mac",
		Environment:   EnvironmentSandbox,
		IsActive:      true,
		Metadata:      map[string]interface{}{"name": "Test Merchant"},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// TestMerchant_IsSandbox tests the IsSandbox method with various environments
func TestMerchant_IsSandbox(t *testing.T) {
	tests := []struct {
		name        string
		environment Environment
		expected    bool
	}{
		{
			name:        "sandbox_environment_returns_true",
			environment: EnvironmentSandbox,
			expected:    true,
		},
		{
			name:        "production_environment_returns_false",
			environment: EnvironmentProduction,
			expected:    false,
		},
		{
			name:        "empty_environment_returns_false",
			environment: "",
			expected:    false,
		},
		{
			name:        "invalid_environment_returns_false",
			environment: "invalid",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Merchant{Environment: tt.environment}
			result := m.IsSandbox()
			assert.Equal(t, tt.expected, result, "IsSandbox() should return %v for environment '%s'", tt.expected, tt.environment)
		})
	}
}

// TestMerchant_IsProduction tests the IsProduction method with various environments
func TestMerchant_IsProduction(t *testing.T) {
	tests := []struct {
		name        string
		environment Environment
		expected    bool
	}{
		{
			name:        "production_environment_returns_true",
			environment: EnvironmentProduction,
			expected:    true,
		},
		{
			name:        "sandbox_environment_returns_false",
			environment: EnvironmentSandbox,
			expected:    false,
		},
		{
			name:        "empty_environment_returns_false",
			environment: "",
			expected:    false,
		},
		{
			name:        "invalid_environment_returns_false",
			environment: "invalid",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Merchant{Environment: tt.environment}
			result := m.IsProduction()
			assert.Equal(t, tt.expected, result, "IsProduction() should return %v for environment '%s'", tt.expected, tt.environment)
		})
	}
}

// TestMerchant_CanProcessTransactions tests the business rule that only active merchants can process transactions
func TestMerchant_CanProcessTransactions(t *testing.T) {
	tests := []struct {
		name     string
		isActive bool
		expected bool
	}{
		{
			name:     "active_merchant_can_process_transactions",
			isActive: true,
			expected: true,
		},
		{
			name:     "inactive_merchant_cannot_process_transactions",
			isActive: false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Merchant{IsActive: tt.isActive}
			result := m.CanProcessTransactions()
			assert.Equal(t, tt.expected, result, "CanProcessTransactions() should return %v for IsActive=%v", tt.expected, tt.isActive)
		})
	}
}

// TestMerchant_CanProcessTransactions_WithFullContext tests transaction processing with real merchant data
func TestMerchant_CanProcessTransactions_WithFullContext(t *testing.T) {
	t.Run("active_sandbox_merchant_can_process", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = true
		m.Environment = EnvironmentSandbox

		assert.True(t, m.CanProcessTransactions(), "Active sandbox merchant should be able to process transactions")
	})

	t.Run("active_production_merchant_can_process", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = true
		m.Environment = EnvironmentProduction

		assert.True(t, m.CanProcessTransactions(), "Active production merchant should be able to process transactions")
	})

	t.Run("inactive_merchant_cannot_process_regardless_of_environment", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = false
		m.Environment = EnvironmentProduction

		assert.False(t, m.CanProcessTransactions(), "Inactive merchant should not be able to process transactions regardless of environment")
	})
}

// TestMerchant_GetMACSecretPath tests the MAC secret path retrieval
func TestMerchant_GetMACSecretPath(t *testing.T) {
	tests := []struct {
		name          string
		macSecretPath string
		expected      string
	}{
		{
			name:          "returns_configured_mac_secret_path",
			macSecretPath: "payment-service/merchants/merchant-123/mac",
			expected:      "payment-service/merchants/merchant-123/mac",
		},
		{
			name:          "returns_empty_string_when_not_configured",
			macSecretPath: "",
			expected:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Merchant{MACSecretPath: tt.macSecretPath}
			result := m.GetMACSecretPath()
			assert.Equal(t, tt.expected, result, "GetMACSecretPath() should return '%s'", tt.expected)
		})
	}
}

// TestMerchant_GetMACSecretPath_FormatValidation tests MAC secret path format
func TestMerchant_GetMACSecretPath_FormatValidation(t *testing.T) {
	t.Run("standard_format_path", func(t *testing.T) {
		m := newTestMerchant()
		path := m.GetMACSecretPath()

		assert.NotEmpty(t, path, "MAC secret path should not be empty")
		assert.Contains(t, path, "payment-service/merchants", "Path should follow standard format")
		assert.Contains(t, path, "/mac", "Path should end with /mac")
	})
}

// TestMerchant_Deactivate tests the merchant deactivation business logic
func TestMerchant_Deactivate(t *testing.T) {
	t.Run("sets_is_active_to_false", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = true

		m.Deactivate()

		assert.False(t, m.IsActive, "Deactivate() should set IsActive to false")
	})

	t.Run("updates_timestamp", func(t *testing.T) {
		m := newTestMerchant()
		originalUpdatedAt := m.UpdatedAt

		// Sleep to ensure timestamp changes
		time.Sleep(2 * time.Millisecond)

		m.Deactivate()

		assert.True(t, m.UpdatedAt.After(originalUpdatedAt), "Deactivate() should update the UpdatedAt timestamp")
	})

	t.Run("idempotent_deactivation", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = false
		m.UpdatedAt = time.Now()
		firstDeactivation := m.UpdatedAt

		time.Sleep(2 * time.Millisecond)

		m.Deactivate()

		assert.False(t, m.IsActive, "Multiple deactivations should keep IsActive false")
		assert.True(t, m.UpdatedAt.After(firstDeactivation), "UpdatedAt should still be updated even for already inactive merchant")
	})
}

// TestMerchant_Activate tests the merchant activation business logic
func TestMerchant_Activate(t *testing.T) {
	t.Run("sets_is_active_to_true", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = false

		m.Activate()

		assert.True(t, m.IsActive, "Activate() should set IsActive to true")
	})

	t.Run("updates_timestamp", func(t *testing.T) {
		m := newTestMerchant()
		originalUpdatedAt := m.UpdatedAt

		// Sleep to ensure timestamp changes
		time.Sleep(2 * time.Millisecond)

		m.Activate()

		assert.True(t, m.UpdatedAt.After(originalUpdatedAt), "Activate() should update the UpdatedAt timestamp")
	})

	t.Run("idempotent_activation", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = true
		m.UpdatedAt = time.Now()
		firstActivation := m.UpdatedAt

		time.Sleep(2 * time.Millisecond)

		m.Activate()

		assert.True(t, m.IsActive, "Multiple activations should keep IsActive true")
		assert.True(t, m.UpdatedAt.After(firstActivation), "UpdatedAt should still be updated even for already active merchant")
	})
}

// TestMerchant_StateTransitions tests complete state transition workflows
func TestMerchant_StateTransitions(t *testing.T) {
	t.Run("active_to_inactive_to_active_transition", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = true

		// Initially active
		assert.True(t, m.IsActive, "Merchant should start active")
		assert.True(t, m.CanProcessTransactions(), "Active merchant should be able to process transactions")

		// Deactivate
		time.Sleep(2 * time.Millisecond)
		beforeDeactivate := time.Now()
		m.Deactivate()

		assert.False(t, m.IsActive, "Merchant should be inactive after Deactivate()")
		assert.False(t, m.CanProcessTransactions(), "Inactive merchant should not be able to process transactions")
		assert.True(t, m.UpdatedAt.After(beforeDeactivate.Add(-2*time.Millisecond)), "UpdatedAt should be updated after deactivation")

		// Re-activate
		time.Sleep(2 * time.Millisecond)
		beforeActivate := time.Now()
		m.Activate()

		assert.True(t, m.IsActive, "Merchant should be active after Activate()")
		assert.True(t, m.CanProcessTransactions(), "Re-activated merchant should be able to process transactions")
		assert.True(t, m.UpdatedAt.After(beforeActivate.Add(-2*time.Millisecond)), "UpdatedAt should be updated after activation")
	})

	t.Run("inactive_to_active_transition", func(t *testing.T) {
		m := newTestMerchant()
		m.IsActive = false

		assert.False(t, m.CanProcessTransactions(), "Inactive merchant should not be able to process transactions")

		m.Activate()

		assert.True(t, m.IsActive, "Merchant should be active after Activate()")
		assert.True(t, m.CanProcessTransactions(), "Activated merchant should be able to process transactions")
	})

	t.Run("multiple_state_transitions_update_timestamps", func(t *testing.T) {
		m := newTestMerchant()
		timestamps := []time.Time{}

		// Record initial timestamp
		timestamps = append(timestamps, m.UpdatedAt)

		// Perform multiple state transitions
		for i := 0; i < 3; i++ {
			time.Sleep(2 * time.Millisecond)
			if i%2 == 0 {
				m.Deactivate()
			} else {
				m.Activate()
			}
			timestamps = append(timestamps, m.UpdatedAt)
		}

		// Verify timestamps are monotonically increasing
		for i := 1; i < len(timestamps); i++ {
			assert.True(t, timestamps[i].After(timestamps[i-1]),
				"Timestamp %d should be after timestamp %d", i, i-1)
		}
	})
}

// TestMerchant_CredentialValidation tests that credential fields are properly set
func TestMerchant_CredentialValidation(t *testing.T) {
	t.Run("all_epx_credentials_present", func(t *testing.T) {
		m := newTestMerchant()

		require.NotEmpty(t, m.CustNbr, "CustNbr should not be empty")
		require.NotEmpty(t, m.MerchNbr, "MerchNbr should not be empty")
		require.NotEmpty(t, m.DBAnbr, "DBAnbr should not be empty")
		require.NotEmpty(t, m.TerminalNbr, "TerminalNbr should not be empty")
		require.NotEmpty(t, m.MACSecretPath, "MACSecretPath should not be empty")
	})

	t.Run("merchant_with_empty_credentials", func(t *testing.T) {
		m := &Merchant{
			ID:          "test-id",
			AgentID:     "agent-id",
			Environment: EnvironmentSandbox,
			IsActive:    true,
		}

		assert.Empty(t, m.CustNbr, "CustNbr should be empty for newly created merchant")
		assert.Empty(t, m.MerchNbr, "MerchNbr should be empty for newly created merchant")
		assert.Empty(t, m.DBAnbr, "DBAnbr should be empty for newly created merchant")
		assert.Empty(t, m.TerminalNbr, "TerminalNbr should be empty for newly created merchant")
		assert.Empty(t, m.MACSecretPath, "MACSecretPath should be empty for newly created merchant")
	})

	t.Run("merchant_identity_fields", func(t *testing.T) {
		m := newTestMerchant()

		require.NotEmpty(t, m.ID, "ID should not be empty")
		require.NotEmpty(t, m.AgentID, "AgentID should not be empty")
		assert.NotEqual(t, m.ID, m.AgentID, "ID and AgentID should be different")
	})
}

// TestMerchant_EnvironmentValidation tests environment field validation
func TestMerchant_EnvironmentValidation(t *testing.T) {
	t.Run("valid_sandbox_environment", func(t *testing.T) {
		m := &Merchant{Environment: EnvironmentSandbox}

		assert.True(t, m.IsSandbox(), "Merchant with sandbox environment should return true for IsSandbox()")
		assert.False(t, m.IsProduction(), "Merchant with sandbox environment should return false for IsProduction()")
	})

	t.Run("valid_production_environment", func(t *testing.T) {
		m := &Merchant{Environment: EnvironmentProduction}

		assert.False(t, m.IsSandbox(), "Merchant with production environment should return false for IsSandbox()")
		assert.True(t, m.IsProduction(), "Merchant with production environment should return true for IsProduction()")
	})

	t.Run("invalid_environment_behavior", func(t *testing.T) {
		m := &Merchant{Environment: "invalid"}

		assert.False(t, m.IsSandbox(), "Merchant with invalid environment should return false for IsSandbox()")
		assert.False(t, m.IsProduction(), "Merchant with invalid environment should return false for IsProduction()")
	})
}

// TestMerchant_MetadataHandling tests metadata field functionality
func TestMerchant_MetadataHandling(t *testing.T) {
	t.Run("metadata_preserved_during_state_transitions", func(t *testing.T) {
		m := newTestMerchant()
		m.Metadata = map[string]interface{}{
			"business_name": "Test Business",
			"contact_email": "test@example.com",
			"tier":          "premium",
		}

		// Perform state transitions
		m.Deactivate()
		m.Activate()

		assert.Equal(t, "Test Business", m.Metadata["business_name"], "Metadata should be preserved during state transitions")
		assert.Equal(t, "test@example.com", m.Metadata["contact_email"], "Metadata should be preserved during state transitions")
		assert.Equal(t, "premium", m.Metadata["tier"], "Metadata should be preserved during state transitions")
	})

	t.Run("nil_metadata_does_not_cause_panic", func(t *testing.T) {
		m := &Merchant{
			ID:       "test-id",
			IsActive: true,
			Metadata: nil,
		}

		// These operations should not panic
		assert.NotPanics(t, func() {
			m.Deactivate()
			m.Activate()
			_ = m.CanProcessTransactions()
		}, "Operations should not panic with nil metadata")
	})
}

// TestMerchant_TimestampBehavior tests timestamp management
func TestMerchant_TimestampBehavior(t *testing.T) {
	t.Run("created_at_not_modified_by_state_transitions", func(t *testing.T) {
		m := newTestMerchant()
		createdAt := m.CreatedAt

		time.Sleep(2 * time.Millisecond)
		m.Deactivate()
		m.Activate()

		assert.Equal(t, createdAt, m.CreatedAt, "CreatedAt should not be modified by state transitions")
	})

	t.Run("updated_at_reflects_latest_operation", func(t *testing.T) {
		m := newTestMerchant()

		time.Sleep(2 * time.Millisecond)
		m.Deactivate()
		deactivatedAt := m.UpdatedAt

		time.Sleep(2 * time.Millisecond)
		m.Activate()
		activatedAt := m.UpdatedAt

		assert.True(t, activatedAt.After(deactivatedAt), "UpdatedAt should reflect the latest operation")
	})
}

// TestMerchant_NilPointerSafety tests nil pointer handling
func TestMerchant_NilPointerSafety(t *testing.T) {
	t.Run("methods_on_nil_merchant_panic", func(t *testing.T) {
		var m *Merchant

		assert.Panics(t, func() {
			_ = m.IsSandbox()
		}, "IsSandbox() should panic on nil merchant")

		assert.Panics(t, func() {
			_ = m.IsProduction()
		}, "IsProduction() should panic on nil merchant")

		assert.Panics(t, func() {
			_ = m.CanProcessTransactions()
		}, "CanProcessTransactions() should panic on nil merchant")

		assert.Panics(t, func() {
			_ = m.GetMACSecretPath()
		}, "GetMACSecretPath() should panic on nil merchant")

		assert.Panics(t, func() {
			m.Deactivate()
		}, "Deactivate() should panic on nil merchant")

		assert.Panics(t, func() {
			m.Activate()
		}, "Activate() should panic on nil merchant")
	})
}
