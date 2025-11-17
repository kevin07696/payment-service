package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	serviceports "github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// =============================================================================
// MOCK IMPLEMENTATIONS - Demonstrates how ports enable easy testing
// =============================================================================

// MockDatabaseAdapter mocks the database adapter interface
type MockDatabaseAdapter struct {
	mock.Mock
}

func (m *MockDatabaseAdapter) Queries() sqlc.Querier {
	args := m.Called()
	return args.Get(0).(sqlc.Querier)
}

// MockQuerier mocks the sqlc.Querier interface
type MockQuerier struct {
	mock.Mock
}

func (m *MockQuerier) GetMerchantByID(ctx context.Context, id uuid.UUID) (sqlc.Merchant, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) CreateTransaction(ctx context.Context, params sqlc.CreateTransactionParams) (sqlc.Transaction, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) GetTransactionByID(ctx context.Context, id uuid.UUID) (sqlc.Transaction, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

// Add other sqlc.Querier methods as stubs (not used in tests but required for interface)
func (m *MockQuerier) GetMerchantBySlug(ctx context.Context, slug string) (sqlc.Merchant, error) {
	return sqlc.Merchant{}, nil
}
func (m *MockQuerier) MerchantExistsBySlug(ctx context.Context, slug string) (bool, error) {
	return false, nil
}
func (m *MockQuerier) CreateMerchant(ctx context.Context, params sqlc.CreateMerchantParams) (sqlc.Merchant, error) {
	return sqlc.Merchant{}, nil
}
func (m *MockQuerier) ActivateMerchant(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) DeactivateMerchant(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) UpdateMerchant(ctx context.Context, params sqlc.UpdateMerchantParams) (sqlc.Merchant, error) {
	return sqlc.Merchant{}, nil
}
func (m *MockQuerier) ListMerchants(ctx context.Context, params sqlc.ListMerchantsParams) ([]sqlc.Merchant, error) {
	return nil, nil
}
func (m *MockQuerier) CountMerchants(ctx context.Context, params sqlc.CountMerchantsParams) (int64, error) {
	return 0, nil
}
func (m *MockQuerier) MerchantExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return false, nil
}
func (m *MockQuerier) ListActiveMerchants(ctx context.Context) ([]sqlc.Merchant, error) {
	return nil, nil
}
func (m *MockQuerier) UpdateMerchantMACPath(ctx context.Context, params sqlc.UpdateMerchantMACPathParams) error {
	return nil
}

// Payment method stubs
func (m *MockQuerier) CreatePaymentMethod(ctx context.Context, params sqlc.CreatePaymentMethodParams) (sqlc.CustomerPaymentMethod, error) {
	return sqlc.CustomerPaymentMethod{}, nil
}
func (m *MockQuerier) GetPaymentMethodByID(ctx context.Context, id uuid.UUID) (sqlc.CustomerPaymentMethod, error) {
	return sqlc.CustomerPaymentMethod{}, nil
}
func (m *MockQuerier) ListPaymentMethods(ctx context.Context, params sqlc.ListPaymentMethodsParams) ([]sqlc.CustomerPaymentMethod, error) {
	return nil, nil
}
func (m *MockQuerier) ListPaymentMethodsByCustomer(ctx context.Context, params sqlc.ListPaymentMethodsByCustomerParams) ([]sqlc.CustomerPaymentMethod, error) {
	return nil, nil
}
func (m *MockQuerier) GetDefaultPaymentMethod(ctx context.Context, params sqlc.GetDefaultPaymentMethodParams) (sqlc.CustomerPaymentMethod, error) {
	return sqlc.CustomerPaymentMethod{}, nil
}
func (m *MockQuerier) ActivatePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) DeactivatePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) DeletePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) MarkPaymentMethodUsed(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) MarkPaymentMethodVerified(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) MarkPaymentMethodAsDefault(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) SetPaymentMethodAsDefault(ctx context.Context, params sqlc.SetPaymentMethodAsDefaultParams) error {
	return nil
}

// Transaction stubs
func (m *MockQuerier) GetTransactionByTranNbr(ctx context.Context, tranNbr pgtype.Text) (sqlc.Transaction, error) {
	args := m.Called(ctx, tranNbr)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}
func (m *MockQuerier) UpdateTransactionFromEPXResponse(ctx context.Context, params sqlc.UpdateTransactionFromEPXResponseParams) (sqlc.Transaction, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}
func (m *MockQuerier) GetTransactionsByGroupID(ctx context.Context, groupID uuid.UUID) ([]sqlc.Transaction, error) {
	return nil, nil
}
func (m *MockQuerier) ListTransactions(ctx context.Context, params sqlc.ListTransactionsParams) ([]sqlc.Transaction, error) {
	return nil, nil
}
func (m *MockQuerier) CountTransactions(ctx context.Context, params sqlc.CountTransactionsParams) (int64, error) {
	return 0, nil
}

// Subscription stubs
func (m *MockQuerier) CreateSubscription(ctx context.Context, params sqlc.CreateSubscriptionParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) ListSubscriptions(ctx context.Context, params sqlc.ListSubscriptionsParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListSubscriptionsByCustomer(ctx context.Context, params sqlc.ListSubscriptionsByCustomerParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListSubscriptionsDueForBilling(ctx context.Context, params sqlc.ListSubscriptionsDueForBillingParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListDueSubscriptions(ctx context.Context, params sqlc.ListDueSubscriptionsParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) CountSubscriptions(ctx context.Context, params sqlc.CountSubscriptionsParams) (int64, error) {
	return 0, nil
}
func (m *MockQuerier) UpdateSubscription(ctx context.Context, params sqlc.UpdateSubscriptionParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) UpdateSubscriptionStatus(ctx context.Context, params sqlc.UpdateSubscriptionStatusParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) UpdateSubscriptionBilling(ctx context.Context, params sqlc.UpdateSubscriptionBillingParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) CancelSubscription(ctx context.Context, params sqlc.CancelSubscriptionParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) IncrementSubscriptionRetryCount(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) ResetSubscriptionRetryCount(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) IncrementSubscriptionFailureCount(ctx context.Context, params sqlc.IncrementSubscriptionFailureCountParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) UpdateNextBillingDate(ctx context.Context, params sqlc.UpdateNextBillingDateParams) error {
	return nil
}

// Chargeback stubs
func (m *MockQuerier) CreateChargeback(ctx context.Context, params sqlc.CreateChargebackParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) GetChargebackByID(ctx context.Context, id uuid.UUID) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) GetChargebackByCaseNumber(ctx context.Context, params sqlc.GetChargebackByCaseNumberParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) GetChargebackByGroupID(ctx context.Context, groupID pgtype.UUID) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) ListChargebacks(ctx context.Context, params sqlc.ListChargebacksParams) ([]sqlc.Chargeback, error) {
	return nil, nil
}
func (m *MockQuerier) CountChargebacks(ctx context.Context, params sqlc.CountChargebacksParams) (int64, error) {
	return 0, nil
}
func (m *MockQuerier) UpdateChargeback(ctx context.Context, params sqlc.UpdateChargebackParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) UpdateChargebackStatus(ctx context.Context, params sqlc.UpdateChargebackStatusParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) UpdateChargebackResponse(ctx context.Context, params sqlc.UpdateChargebackResponseParams) error {
	return nil
}
func (m *MockQuerier) UpdateChargebackNotes(ctx context.Context, params sqlc.UpdateChargebackNotesParams) error {
	return nil
}
func (m *MockQuerier) AddEvidenceFile(ctx context.Context, params sqlc.AddEvidenceFileParams) error {
	return nil
}
func (m *MockQuerier) MarkChargebackResolved(ctx context.Context, params sqlc.MarkChargebackResolvedParams) error {
	return nil
}

// Webhook stubs
func (m *MockQuerier) CreateWebhookSubscription(ctx context.Context, params sqlc.CreateWebhookSubscriptionParams) (sqlc.WebhookSubscription, error) {
	return sqlc.WebhookSubscription{}, nil
}
func (m *MockQuerier) GetWebhookSubscription(ctx context.Context, id uuid.UUID) (sqlc.WebhookSubscription, error) {
	return sqlc.WebhookSubscription{}, nil
}
func (m *MockQuerier) ListWebhookSubscriptions(ctx context.Context, params sqlc.ListWebhookSubscriptionsParams) ([]sqlc.WebhookSubscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListActiveWebhooksByEvent(ctx context.Context, params sqlc.ListActiveWebhooksByEventParams) ([]sqlc.WebhookSubscription, error) {
	return nil, nil
}
func (m *MockQuerier) UpdateWebhookSubscription(ctx context.Context, params sqlc.UpdateWebhookSubscriptionParams) (sqlc.WebhookSubscription, error) {
	return sqlc.WebhookSubscription{}, nil
}
func (m *MockQuerier) DeleteWebhookSubscription(ctx context.Context, params sqlc.DeleteWebhookSubscriptionParams) error {
	return nil
}
func (m *MockQuerier) CreateWebhookDelivery(ctx context.Context, params sqlc.CreateWebhookDeliveryParams) (sqlc.WebhookDelivery, error) {
	return sqlc.WebhookDelivery{}, nil
}
func (m *MockQuerier) GetWebhookDeliveryHistory(ctx context.Context, params sqlc.GetWebhookDeliveryHistoryParams) ([]sqlc.WebhookDelivery, error) {
	return nil, nil
}
func (m *MockQuerier) ListPendingWebhookDeliveries(ctx context.Context, limitVal int32) ([]sqlc.WebhookDelivery, error) {
	return nil, nil
}
func (m *MockQuerier) UpdateWebhookDeliveryStatus(ctx context.Context, params sqlc.UpdateWebhookDeliveryStatusParams) (sqlc.WebhookDelivery, error) {
	return sqlc.WebhookDelivery{}, nil
}

// MockBrowserPostAdapter mocks the Browser Post adapter
type MockBrowserPostAdapter struct {
	mock.Mock
}

func (m *MockBrowserPostAdapter) BuildFormData(tac, amount, tranNbr, tranGroup, redirectURL string) (*ports.BrowserPostFormData, error) {
	args := m.Called(tac, amount, tranNbr, tranGroup, redirectURL)
	return args.Get(0).(*ports.BrowserPostFormData), args.Error(1)
}

func (m *MockBrowserPostAdapter) ParseRedirectResponse(params map[string][]string) (*ports.BrowserPostResponse, error) {
	args := m.Called(params)
	return args.Get(0).(*ports.BrowserPostResponse), args.Error(1)
}

func (m *MockBrowserPostAdapter) ValidateResponseMAC(params map[string][]string, mac string) error {
	args := m.Called(params, mac)
	return args.Error(0)
}

// MockKeyExchangeAdapter mocks the Key Exchange adapter
type MockKeyExchangeAdapter struct {
	mock.Mock
}

func (m *MockKeyExchangeAdapter) GetTAC(ctx context.Context, req *ports.KeyExchangeRequest) (*ports.KeyExchangeResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*ports.KeyExchangeResponse), args.Error(1)
}

// MockSecretManager mocks the secret manager
type MockSecretManager struct {
	mock.Mock
}

func (m *MockSecretManager) GetSecret(ctx context.Context, path string) (*ports.Secret, error) {
	args := m.Called(ctx, path)
	return args.Get(0).(*ports.Secret), args.Error(1)
}

func (m *MockSecretManager) GetSecretVersion(ctx context.Context, path, version string) (*ports.Secret, error) {
	return nil, nil
}

func (m *MockSecretManager) PutSecret(ctx context.Context, path, value string, metadata map[string]string) (string, error) {
	return "", nil
}

func (m *MockSecretManager) RotateSecret(ctx context.Context, path, newValue string) (*ports.SecretRotationInfo, error) {
	return nil, nil
}

func (m *MockSecretManager) DeleteSecret(ctx context.Context, path string) error {
	return nil
}

// MockPaymentMethodService mocks the payment method service
type MockPaymentMethodService struct {
	mock.Mock
}

func (m *MockPaymentMethodService) ConvertFinancialBRICToStorageBRIC(ctx context.Context, req *serviceports.ConvertFinancialBRICRequest) (*domain.PaymentMethod, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentMethod), args.Error(1)
}

// =============================================================================
// TEST HELPER FUNCTIONS
// =============================================================================

func setupTestHandler() (*BrowserPostCallbackHandler, *MockDatabaseAdapter, *MockQuerier, *MockBrowserPostAdapter, *MockKeyExchangeAdapter, *MockSecretManager, *MockPaymentMethodService) {
	mockDB := new(MockDatabaseAdapter)
	mockQuerier := new(MockQuerier)
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockKeyExchange := new(MockKeyExchangeAdapter)
	mockSecretMgr := new(MockSecretManager)
	mockPaymentMethod := new(MockPaymentMethodService)

	// Setup mockDB to return mockQuerier
	mockDB.On("Queries").Return(mockQuerier)

	logger := zap.NewNop()

	handler := NewBrowserPostCallbackHandler(
		mockDB,
		mockBrowserPost,
		mockKeyExchange,
		mockSecretMgr,
		mockPaymentMethod,
		logger,
		"https://secure.epxuap.com/browserpost",
		"http://localhost:8081",
	)

	return handler, mockDB, mockQuerier, mockBrowserPost, mockKeyExchange, mockSecretMgr, mockPaymentMethod
}

// =============================================================================
// GetPaymentForm Tests - Demonstrates ports enable easy unit testing
// =============================================================================

func TestGetPaymentForm_Success(t *testing.T) {
	handler, _, mockQuerier, _, mockKeyExchange, mockSecretMgr, _ := setupTestHandler()

	// Generate test UUIDs
	txID := uuid.New()
	merchantID := uuid.New()

	// Mock merchant lookup
	merchant := sqlc.Merchant{
		ID:            merchantID,
		Slug:          "test-merchant",
		CustNbr:       "9001",
		MerchNbr:      "900300",
		DbaNbr:        "2",
		TerminalNbr:   "77",
		MacSecretPath: "/secrets/test-merchant/mac",
		Name:          "Test Merchant",
		IsActive:      true,
		Environment:   "sandbox",
		CreatedAt:     pgtype.Timestamp{Time: time.Now(), Valid: true},
		UpdatedAt:     pgtype.Timestamp{Time: time.Now(), Valid: true},
	}
	mockQuerier.On("GetMerchantByID", mock.Anything, merchantID).Return(merchant, nil)

	// Mock transaction lookup (for idempotency check) - should not exist yet
	mockQuerier.On("GetTransactionByID", mock.Anything, txID).Return(sqlc.Transaction{}, fmt.Errorf("transaction not found"))

	// Mock transaction creation (pending transaction pattern)
	createdTx := sqlc.Transaction{
		ID:                txID,
		GroupID:           uuid.New(),
		MerchantID:        merchantID,
		Amount:            pgtype.Numeric{Int: nil, Exp: 0, Valid: true}, // Amount representation
		Currency:          "USD",
		Type:              "sale",
		PaymentMethodType: "credit_card",
		TranNbr:           pgtype.Text{String: "2466125485", Valid: true},
		AuthResp:          "", // Pending (empty string)
		Status:            pgtype.Text{String: "pending", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	mockQuerier.On("CreateTransaction", mock.Anything, mock.AnythingOfType("sqlc.CreateTransactionParams")).Return(createdTx, nil)

	// Mock secret manager
	secret := &ports.Secret{
		Value:     "test-mac-secret",
		Version:   "v1",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	mockSecretMgr.On("GetSecret", mock.Anything, "/secrets/test-merchant/mac").Return(secret, nil)

	// Mock key exchange
	keyExchangeResp := &ports.KeyExchangeResponse{
		TAC:       "test-tac-token-12345",
		ExpiresAt: time.Now().Add(4 * time.Hour),
		TranNbr:   txID.String(),
		TranGroup: uuid.New().String(),
	}
	mockKeyExchange.On("GetTAC", mock.Anything, mock.MatchedBy(func(req *ports.KeyExchangeRequest) bool {
		return req.MerchantID == merchantID.String() &&
			req.Amount == "99.99" &&
			req.TranNbr != "" &&
			req.RedirectURL != "" &&
			req.TranGroup == "SALE"
	})).Return(keyExchangeResp, nil)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=99.99&transaction_type=SALE&return_url=https://example.com/receipt",
		txID.String(),
		merchantID.String(),
	), nil)
	w := httptest.NewRecorder()

	// Execute
	handler.GetPaymentForm(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Parse response
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err, "Should parse JSON response")

	// Verify response fields
	assert.Equal(t, "https://secure.epxuap.com/browserpost", response["postURL"])
	assert.Equal(t, "test-tac-token-12345", response["tac"])
	assert.Equal(t, "9001", response["custNbr"])
	assert.Equal(t, "900300", response["merchNbr"])
	assert.Equal(t, "2", response["dbaName"]) // Changed from dbaNbr to dbaName
	assert.Equal(t, "77", response["terminalNbr"])
	assert.Equal(t, txID.String(), response["transactionId"]) // Changed from tranNbr to transactionId
	assert.NotEmpty(t, response["epxTranNbr"])                // EPX numeric transaction number
	assert.Equal(t, merchantID.String(), response["merchantId"])
	assert.Equal(t, "Test Merchant", response["merchantName"])

	// Verify all mocks were called correctly
	mockQuerier.AssertExpectations(t)
	mockKeyExchange.AssertExpectations(t)
	mockSecretMgr.AssertExpectations(t)
}

func TestGetPaymentForm_MissingTransactionID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?merchant_id=123&amount=99.99", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "transaction_id parameter is required")
}

func TestGetPaymentForm_InvalidTransactionID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?transaction_id=invalid-uuid&merchant_id=123&amount=99.99", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid transaction_id format")
}

func TestGetPaymentForm_MissingMerchantID(t *testing.T) {
	handler, _, _, _, _, _, _ := setupTestHandler()

	txID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/payments/browser-post/form?transaction_id=%s&amount=99.99", txID.String()), nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "merchant_id parameter is required")
}

func TestGetPaymentForm_MissingAmount(t *testing.T) {
	handler, _, _, _, _, _, _ := setupTestHandler()

	txID := uuid.New()
	merchantID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s",
		txID.String(),
		merchantID.String(),
	), nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "amount parameter is required")
}

func TestGetPaymentForm_InvalidAmount(t *testing.T) {
	handler, _, _, _, _, _, _ := setupTestHandler()

	txID := uuid.New()
	merchantID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=invalid",
		txID.String(),
		merchantID.String(),
	), nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "amount must be a valid number")
}

func TestGetPaymentForm_InvalidTransactionType(t *testing.T) {
	handler, _, _, _, _, _, _ := setupTestHandler()

	txID := uuid.New()
	merchantID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=99.99&transaction_type=INVALID",
		txID.String(),
		merchantID.String(),
	), nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "transaction_type must be SALE or AUTH")
}

func TestGetPaymentForm_DefaultTransactionType(t *testing.T) {
	handler, _, mockQuerier, _, mockKeyExchange, mockSecretMgr, _ := setupTestHandler()

	txID := uuid.New()
	merchantID := uuid.New()

	// Setup mocks
	merchant := sqlc.Merchant{
		ID:            merchantID,
		CustNbr:       "9001",
		MerchNbr:      "900300",
		DbaNbr:        "2",
		TerminalNbr:   "77",
		MacSecretPath: "/secrets/test-merchant/mac",
		Name:          "Test Merchant",
		IsActive:      true,
	}
	mockQuerier.On("GetMerchantByID", mock.Anything, merchantID).Return(merchant, nil)

	// Mock transaction lookup (for idempotency check) - should not exist yet
	mockQuerier.On("GetTransactionByID", mock.Anything, txID).Return(sqlc.Transaction{}, fmt.Errorf("transaction not found"))

	// Mock transaction creation (pending transaction pattern)
	createdTx := sqlc.Transaction{
		ID:                txID,
		GroupID:           uuid.New(),
		MerchantID:        merchantID,
		Amount:            pgtype.Numeric{Int: nil, Exp: 0, Valid: true},
		Currency:          "USD",
		Type:              "sale",
		PaymentMethodType: "credit_card",
		TranNbr:           pgtype.Text{String: txID.String(), Valid: true},
		AuthResp:          "", // Pending (empty string)
		Status:            pgtype.Text{String: "pending", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	mockQuerier.On("CreateTransaction", mock.Anything, mock.AnythingOfType("sqlc.CreateTransactionParams")).Return(createdTx, nil)

	secret := &ports.Secret{Value: "test-mac-secret", Version: "v1", CreatedAt: time.Now().Format(time.RFC3339)}
	mockSecretMgr.On("GetSecret", mock.Anything, "/secrets/test-merchant/mac").Return(secret, nil)

	keyExchangeResp := &ports.KeyExchangeResponse{
		TAC:       "test-tac",
		ExpiresAt: time.Now().Add(4 * time.Hour),
		TranNbr:   txID.String(),
		TranGroup: uuid.New().String(),
	}
	mockKeyExchange.On("GetTAC", mock.Anything, mock.Anything).Return(keyExchangeResp, nil)

	// Request without transaction_type (should default to SALE)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=99.99&return_url=https://example.com/receipt",
		txID.String(),
		merchantID.String(),
	), nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	// Verify request succeeded (transaction_type defaults to SALE internally)
	assert.NotEmpty(t, response["tac"], "Should receive TAC")
	assert.Equal(t, txID.String(), response["transactionId"], "Should receive transaction ID")
}

func TestGetPaymentForm_MethodNotAllowed(t *testing.T) {
	handler, _, _, _, _, _, _ := setupTestHandler()

	// POST request instead of GET
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/browser-post/form", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// =============================================================================
// HandleCallback Tests - Demonstrates ports enable easy integration testing
// =============================================================================

func TestHandleCallback_Success(t *testing.T) {
	handler, _, mockQuerier, mockBrowserPost, _, _, _ := setupTestHandler()

	// Setup form data (simulating EPX callback)
	txID := uuid.New()
	formParams := make(map[string][]string)
	formParams["TRAN_NBR"] = []string{txID.String()}
	formParams["AUTH_RESP"] = []string{"00"}
	formParams["AUTH_CODE"] = []string{"123456"}
	formParams["AUTH_GUID"] = []string{"TEST-BRIC-TOKEN-123"}
	formParams["AUTH_CARD_TYPE"] = []string{"Visa"}
	formParams["AMOUNT"] = []string{"99.99"}

	// Mock Browser Post adapter parsing
	merchantID := uuid.New()

	epxResponse := &ports.BrowserPostResponse{
		TranNbr:      txID.String(),
		AuthResp:     "00",
		AuthCode:     "123456",
		AuthGUID:     "TEST-BRIC-TOKEN-123",
		AuthCardType: "Visa",
		Amount:       "99.99",
		IsApproved:   true,
		RawParams: map[string]string{
			"transaction_id": txID.String(),
			"merchant_id":    merchantID.String(),
		},
	}
	mockBrowserPost.On("ParseRedirectResponse", mock.Anything).Return(epxResponse, nil)

	// Mock transaction update with EPX response
	// UpdateTransactionFromEPXResponse does the lookup AND update in a single operation
	groupID := uuid.New()
	updatedTransaction := sqlc.Transaction{
		ID:                txID,
		MerchantID:        merchantID,
		GroupID:           groupID,
		Amount:            pgtype.Numeric{Int: nil, Exp: 0, Valid: true},
		Currency:          "USD",
		Type:              "sale",
		AuthResp:          "00", // Updated with EPX response
		AuthCode:          pgtype.Text{String: "123456", Valid: true},
		AuthGuid:          pgtype.Text{String: "TEST-BRIC-TOKEN-123", Valid: true},
		PaymentMethodType: "credit_card",
		TranNbr:           pgtype.Text{String: txID.String(), Valid: true},
		Status:            pgtype.Text{String: "completed", Valid: true},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	mockQuerier.On("UpdateTransactionFromEPXResponse", mock.Anything, mock.AnythingOfType("sqlc.UpdateTransactionFromEPXResponseParams")).Return(updatedTransaction, nil)

	// Create test request with form data in body (EPX uses self-posting form = POST with form data)
	formData := url.Values{}
	formData.Set("TRAN_NBR", txID.String())
	formData.Set("AUTH_RESP", "00")
	formData.Set("AUTH_CODE", "123456")
	formData.Set("AUTH_GUID", "TEST-BRIC-TOKEN-123")
	formData.Set("AUTH_CARD_TYPE", "Visa")
	formData.Set("AMOUNT", "99.99")

	reqURL := "/api/v1/payments/browser-post/callback"
	req := httptest.NewRequest(http.MethodPost, reqURL, strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Execute
	handler.HandleCallback(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code, "Expected 200 OK")
	assert.Contains(t, w.Body.String(), "Payment Successful", "Should show success message")

	// Verify mocks
	mockBrowserPost.AssertExpectations(t)
	mockQuerier.AssertExpectations(t)
}

// =============================================================================
// SUMMARY: How Ports Architecture Enables Easy Testing
// =============================================================================
//
// Benefits Demonstrated:
// 1. **Easy Mocking**: All dependencies (DB, adapters, services) are interfaces
// 2. **Isolated Testing**: Test handler logic without real database/EPX calls
// 3. **Fast Tests**: No network calls, no database setup required
// 4. **Predictable**: Control exact behavior of dependencies with mocks
// 5. **Maintainable**: Change adapter implementation without changing tests
//
// Without ports, we would need:
// - Real PostgreSQL database for every test
// - Real EPX sandbox API calls (slow, flaky, costs money)
// - Complex test data setup and teardown
// - Tests would be slow (seconds instead of milliseconds)
// - Tests would fail when EPX sandbox is down
//
// With ports, tests run in milliseconds with full control!
// =============================================================================
