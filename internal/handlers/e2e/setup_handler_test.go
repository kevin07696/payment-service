package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/ports"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockSecretManager implements ports.SecretManagerAdapter for testing
type mockSecretManager struct {
	putSecretFunc func(ctx context.Context, path string, value string, metadata map[string]string) (string, error)
}

func (m *mockSecretManager) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	return &ports.Secret{Value: "test-secret"}, nil
}

func (m *mockSecretManager) GetSecretVersion(ctx context.Context, path string, version string) (*ports.Secret, error) {
	return &ports.Secret{Value: "test-secret"}, nil
}

func (m *mockSecretManager) PutSecret(ctx context.Context, path string, value string, metadata map[string]string) (string, error) {
	if m.putSecretFunc != nil {
		return m.putSecretFunc(ctx, path, value, metadata)
	}
	return "v1", nil
}

func (m *mockSecretManager) RotateSecret(ctx context.Context, path string, newValue string) (*ports.SecretRotationInfo, error) {
	return &ports.SecretRotationInfo{}, nil
}

func (m *mockSecretManager) DeleteSecret(ctx context.Context, path string) error {
	return nil
}

func TestHandler_Setup_ProductionBlocked(t *testing.T) {
	mockQ := new(mocks.MockQuerier)
	// Pass "production" via dependency injection
	handler := NewHandler(mockQ, &mockSecretManager{}, zap.NewNop(), "production")

	req := httptest.NewRequest(http.MethodPost, "/internal/e2e/setup", nil)
	w := httptest.NewRecorder()

	handler.Setup(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "not available in production")
}

func TestHandler_Setup_MethodNotAllowed(t *testing.T) {
	mockQ := new(mocks.MockQuerier)
	handler := NewHandler(mockQ, &mockSecretManager{}, zap.NewNop(), "development")

	// Test GET method
	req := httptest.NewRequest(http.MethodGet, "/internal/e2e/setup", nil)
	w := httptest.NewRecorder()

	handler.Setup(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandler_Setup_InvalidJSON(t *testing.T) {
	mockQ := new(mocks.MockQuerier)
	handler := NewHandler(mockQ, &mockSecretManager{}, zap.NewNop(), "development")

	req := httptest.NewRequest(http.MethodPost, "/internal/e2e/setup", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	handler.Setup(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request body")
}

func TestHandler_Setup_Success(t *testing.T) {
	mockQ := new(mocks.MockQuerier)
	merchantID := uuid.New()
	serviceID := uuid.New()

	// Setup expectations - use concrete return values, not functions
	mockQ.On("CreateMerchant", mock.Anything, mock.AnythingOfType("sqlc.CreateMerchantParams")).
		Return(sqlc.Merchant{ID: merchantID, Slug: "test-merchant", Name: "Test Merchant"}, nil)

	mockQ.On("CreateService", mock.Anything, mock.AnythingOfType("sqlc.CreateServiceParams")).
		Return(sqlc.Service{ID: serviceID, ServiceID: "test-service", ServiceName: "Test Service"}, nil)

	mockQ.On("GrantServiceAccess", mock.Anything, mock.AnythingOfType("sqlc.GrantServiceAccessParams")).
		Return(sqlc.ServiceMerchant{}, nil)

	handler := NewHandler(mockQ, &mockSecretManager{}, zap.NewNop(), "development")

	setupReq := SetupRequest{
		Merchant: MerchantRequest{
			ID:          merchantID.String(),
			Slug:        "test-merchant",
			Name:        "Test Merchant",
			CustNbr:     "9001",
			MerchNbr:    "900300",
			DbaNbr:      "2",
			TerminalNbr: "77",
			MacSecret:   "test-mac-secret",
			Environment: "test",
		},
		Service: ServiceRequest{
			ID:        serviceID.String(),
			ServiceID: "test-service",
			Name:      "Test Service",
			PublicKey: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
		},
	}

	body, _ := json.Marshal(setupReq)
	req := httptest.NewRequest(http.MethodPost, "/internal/e2e/setup", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.Setup(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SetupResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "test-merchant", resp.Merchant.Slug)
	assert.Equal(t, "test-service", resp.Service.ServiceID)

	mockQ.AssertExpectations(t)
}

func TestHandler_Cleanup_ProductionBlocked(t *testing.T) {
	mockQ := new(mocks.MockQuerier)
	// Pass "production" via dependency injection
	handler := NewHandler(mockQ, &mockSecretManager{}, zap.NewNop(), "production")

	req := httptest.NewRequest(http.MethodPost, "/internal/e2e/cleanup", nil)
	w := httptest.NewRecorder()

	handler.Cleanup(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandler_Cleanup_MethodNotAllowed(t *testing.T) {
	mockQ := new(mocks.MockQuerier)
	handler := NewHandler(mockQ, &mockSecretManager{}, zap.NewNop(), "development")

	req := httptest.NewRequest(http.MethodGet, "/internal/e2e/cleanup", nil)
	w := httptest.NewRecorder()

	handler.Cleanup(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandler_Cleanup_Success(t *testing.T) {
	mockQ := new(mocks.MockQuerier)

	// Setup expectations
	mockQ.On("RevokeServiceAccess", mock.Anything, mock.AnythingOfType("sqlc.RevokeServiceAccessParams")).Return(nil)
	mockQ.On("HardDeleteService", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
	mockQ.On("HardDeleteMerchant", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)

	handler := NewHandler(mockQ, &mockSecretManager{}, zap.NewNop(), "development")

	cleanupReq := CleanupRequest{
		MerchantID: uuid.New().String(),
		ServiceID:  uuid.New().String(),
	}

	body, _ := json.Marshal(cleanupReq)
	req := httptest.NewRequest(http.MethodPost, "/internal/e2e/cleanup", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	handler.Cleanup(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	mockQ.AssertExpectations(t)
}

func TestGenerateFingerprint(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantLen  int
		wantPref string
	}{
		{
			name:     "valid public key",
			input:    []byte("-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"),
			wantLen:  50,
			wantPref: "SHA256:",
		},
		{
			name:     "empty input",
			input:    []byte{},
			wantLen:  50,
			wantPref: "SHA256:",
		},
		{
			name:     "short input",
			input:    []byte("short"),
			wantLen:  50,
			wantPref: "SHA256:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateFingerprint(tt.input)

			assert.Len(t, result, tt.wantLen)
			assert.True(t, len(result) >= len(tt.wantPref))
			assert.Equal(t, tt.wantPref, result[:len(tt.wantPref)])
		})
	}
}

func TestGenerateFingerprint_Deterministic(t *testing.T) {
	input := []byte("test-public-key-content")

	result1 := generateFingerprint(input)
	result2 := generateFingerprint(input)
	result3 := generateFingerprint(input)

	assert.Equal(t, result1, result2, "Fingerprint should be deterministic")
	assert.Equal(t, result2, result3, "Fingerprint should be deterministic")
}

func TestGenerateFingerprint_UniqueForDifferentInputs(t *testing.T) {
	input1 := []byte("key-1")
	input2 := []byte("key-2")

	result1 := generateFingerprint(input1)
	result2 := generateFingerprint(input2)

	assert.NotEqual(t, result1, result2, "Different inputs should produce different fingerprints")
}
