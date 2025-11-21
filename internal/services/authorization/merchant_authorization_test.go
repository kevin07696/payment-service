package authorization

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/auth"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestResolveMerchantID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	service := NewMerchantAuthorizationService(logger)

	tests := []struct {
		name               string
		authInfo           *auth.AuthInfo
		requestedMerchantID string
		expectedMerchantID string
		expectError        bool
		errorContains      string
	}{
		{
			name: "no auth mode with requested merchant",
			authInfo: &auth.AuthInfo{
				Type: auth.AuthTypeNone,
			},
			requestedMerchantID: "merchant-123",
			expectedMerchantID:  "merchant-123",
			expectError:         false,
		},
		{
			name: "no auth mode without requested merchant",
			authInfo: &auth.AuthInfo{
				Type: auth.AuthTypeNone,
			},
			requestedMerchantID: "",
			expectError:         true,
			errorContains:       "merchant_id is required when auth is disabled",
		},
		{
			name: "JWT auth with merchant in context, no request merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-123",
			},
			requestedMerchantID: "",
			expectedMerchantID:  "merchant-123",
			expectError:         false,
		},
		{
			name: "JWT auth with merchant in context, matching request merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-123",
			},
			requestedMerchantID: "merchant-123",
			expectedMerchantID:  "merchant-123",
			expectError:         false,
		},
		{
			name: "JWT auth with merchant in context, mismatching request merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-123",
			},
			requestedMerchantID: "merchant-456",
			expectError:         true,
			errorContains:       "merchant_id mismatch",
		},
		{
			name: "JWT service auth with requested merchant",
			authInfo: &auth.AuthInfo{
				Type:      auth.AuthTypeJWT,
				ServiceID: "service-1",
			},
			requestedMerchantID: "merchant-123",
			expectedMerchantID:  "merchant-123",
			expectError:         false,
		},
		{
			name: "JWT service auth without requested merchant",
			authInfo: &auth.AuthInfo{
				Type:      auth.AuthTypeJWT,
				ServiceID: "service-1",
			},
			requestedMerchantID: "",
			expectError:         true,
			errorContains:       "unable to determine merchant_id",
		},
		{
			name: "API key auth with merchant in context",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeAPIKey,
				MerchantID: "merchant-123",
			},
			requestedMerchantID: "",
			expectedMerchantID:  "merchant-123",
			expectError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := auth.WithAuth(context.Background(), tt.authInfo)

			merchantID, err := service.ResolveMerchantID(ctx, tt.requestedMerchantID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMerchantID, merchantID)
			}
		})
	}
}

func TestValidateTransactionAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	service := NewMerchantAuthorizationService(logger)

	txID := uuid.New().String()
	tx := &domain.Transaction{
		ID:         txID,
		MerchantID: "merchant-123",
	}

	tests := []struct {
		name          string
		authInfo      *auth.AuthInfo
		expectError   bool
		errorContains string
	}{
		{
			name: "no auth mode allows access",
			authInfo: &auth.AuthInfo{
				Type: auth.AuthTypeNone,
			},
			expectError: false,
		},
		{
			name: "merchant auth with matching merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-123",
			},
			expectError: false,
		},
		{
			name: "merchant auth with different merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-456",
			},
			expectError:   true,
			errorContains: "access denied: transaction belongs to different merchant",
		},
		{
			name: "service auth allows access",
			authInfo: &auth.AuthInfo{
				Type:      auth.AuthTypeJWT,
				ServiceID: "service-1",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := auth.WithAuth(context.Background(), tt.authInfo)

			err := service.ValidateTransactionAccess(ctx, tx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCustomerAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	service := NewMerchantAuthorizationService(logger)

	tests := []struct {
		name          string
		authInfo      *auth.AuthInfo
		merchantID    string
		expectError   bool
		errorContains string
	}{
		{
			name: "no auth mode allows access",
			authInfo: &auth.AuthInfo{
				Type: auth.AuthTypeNone,
			},
			merchantID:  "merchant-123",
			expectError: false,
		},
		{
			name: "merchant auth with matching merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-123",
			},
			merchantID:  "merchant-123",
			expectError: false,
		},
		{
			name: "merchant auth with different merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-456",
			},
			merchantID:    "merchant-123",
			expectError:   true,
			errorContains: "access denied: customer belongs to different merchant",
		},
		{
			name: "service auth allows access",
			authInfo: &auth.AuthInfo{
				Type:      auth.AuthTypeJWT,
				ServiceID: "service-1",
			},
			merchantID:  "merchant-123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := auth.WithAuth(context.Background(), tt.authInfo)

			err := service.ValidateCustomerAccess(ctx, tt.merchantID, "customer-123")

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePaymentMethodAccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	service := NewMerchantAuthorizationService(logger)

	tests := []struct {
		name          string
		authInfo      *auth.AuthInfo
		merchantID    string
		expectError   bool
		errorContains string
	}{
		{
			name: "no auth mode allows access",
			authInfo: &auth.AuthInfo{
				Type: auth.AuthTypeNone,
			},
			merchantID:  "merchant-123",
			expectError: false,
		},
		{
			name: "merchant auth with matching merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-123",
			},
			merchantID:  "merchant-123",
			expectError: false,
		},
		{
			name: "merchant auth with different merchant",
			authInfo: &auth.AuthInfo{
				Type:       auth.AuthTypeJWT,
				MerchantID: "merchant-456",
			},
			merchantID:    "merchant-123",
			expectError:   true,
			errorContains: "access denied: payment method belongs to different merchant",
		},
		{
			name: "service auth allows access",
			authInfo: &auth.AuthInfo{
				Type:      auth.AuthTypeJWT,
				ServiceID: "service-1",
			},
			merchantID:  "merchant-123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := auth.WithAuth(context.Background(), tt.authInfo)

			err := service.ValidatePaymentMethodAccess(ctx, tt.merchantID, "pm-123")

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
