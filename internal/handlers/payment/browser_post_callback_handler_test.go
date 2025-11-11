package payment

import (
	"context"
	"encoding/json"
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
	"go.uber.org/zap/zaptest"
)

// MockQuerier mocks the sqlc.Querier interface
type MockQuerier struct {
	mock.Mock
}

func (m *MockQuerier) CreateTransaction(ctx context.Context, arg sqlc.CreateTransactionParams) (sqlc.Transaction, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) GetTransactionByIdempotencyKey(ctx context.Context, idempotencyKey pgtype.Text) (sqlc.Transaction, error) {
	args := m.Called(ctx, idempotencyKey)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) UpdateTransaction(ctx context.Context, arg sqlc.UpdateTransactionParams) (sqlc.Transaction, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

// Stub implementations for other Querier methods (not used in these tests)
func (m *MockQuerier) ActivateAgent(ctx context.Context, agentID string) error {
	return nil
}
func (m *MockQuerier) ActivatePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) AddEvidenceFile(ctx context.Context, arg sqlc.AddEvidenceFileParams) error {
	return nil
}
func (m *MockQuerier) AgentExists(ctx context.Context, agentID string) (bool, error) {
	return false, nil
}
func (m *MockQuerier) CancelSubscription(ctx context.Context, arg sqlc.CancelSubscriptionParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) CountAgents(ctx context.Context, arg sqlc.CountAgentsParams) (int64, error) {
	return 0, nil
}
func (m *MockQuerier) CountChargebacks(ctx context.Context, arg sqlc.CountChargebacksParams) (int64, error) {
	return 0, nil
}
func (m *MockQuerier) CountSubscriptions(ctx context.Context, arg sqlc.CountSubscriptionsParams) (int64, error) {
	return 0, nil
}
func (m *MockQuerier) CountTransactions(ctx context.Context, arg sqlc.CountTransactionsParams) (int64, error) {
	return 0, nil
}
func (m *MockQuerier) CreateAgent(ctx context.Context, arg sqlc.CreateAgentParams) (sqlc.AgentCredential, error) {
	return sqlc.AgentCredential{}, nil
}
func (m *MockQuerier) CreateChargeback(ctx context.Context, arg sqlc.CreateChargebackParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) CreatePaymentMethod(ctx context.Context, arg sqlc.CreatePaymentMethodParams) (sqlc.CustomerPaymentMethod, error) {
	return sqlc.CustomerPaymentMethod{}, nil
}
func (m *MockQuerier) CreateSubscription(ctx context.Context, arg sqlc.CreateSubscriptionParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) CreateWebhookDelivery(ctx context.Context, arg sqlc.CreateWebhookDeliveryParams) (sqlc.WebhookDelivery, error) {
	return sqlc.WebhookDelivery{}, nil
}
func (m *MockQuerier) CreateWebhookSubscription(ctx context.Context, arg sqlc.CreateWebhookSubscriptionParams) (sqlc.WebhookSubscription, error) {
	return sqlc.WebhookSubscription{}, nil
}
func (m *MockQuerier) DeactivateAgent(ctx context.Context, agentID string) error {
	return nil
}
func (m *MockQuerier) DeactivatePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) DeletePaymentMethod(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) DeleteWebhookSubscription(ctx context.Context, arg sqlc.DeleteWebhookSubscriptionParams) error {
	return nil
}
func (m *MockQuerier) GetAgentByAgentID(ctx context.Context, agentID string) (sqlc.AgentCredential, error) {
	return sqlc.AgentCredential{}, nil
}
func (m *MockQuerier) GetAgentByID(ctx context.Context, id uuid.UUID) (sqlc.AgentCredential, error) {
	return sqlc.AgentCredential{}, nil
}
func (m *MockQuerier) GetChargebackByCaseNumber(ctx context.Context, arg sqlc.GetChargebackByCaseNumberParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) GetChargebackByGroupID(ctx context.Context, groupID pgtype.UUID) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) GetChargebackByID(ctx context.Context, id uuid.UUID) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) GetDefaultPaymentMethod(ctx context.Context, arg sqlc.GetDefaultPaymentMethodParams) (sqlc.CustomerPaymentMethod, error) {
	return sqlc.CustomerPaymentMethod{}, nil
}
func (m *MockQuerier) GetPaymentMethodByID(ctx context.Context, id uuid.UUID) (sqlc.CustomerPaymentMethod, error) {
	return sqlc.CustomerPaymentMethod{}, nil
}
func (m *MockQuerier) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) GetTransactionByID(ctx context.Context, id uuid.UUID) (sqlc.Transaction, error) {
	return sqlc.Transaction{}, nil
}
func (m *MockQuerier) GetTransactionsByGroupID(ctx context.Context, groupID uuid.UUID) ([]sqlc.Transaction, error) {
	return nil, nil
}
func (m *MockQuerier) GetWebhookDeliveryHistory(ctx context.Context, arg sqlc.GetWebhookDeliveryHistoryParams) ([]sqlc.WebhookDelivery, error) {
	return nil, nil
}
func (m *MockQuerier) GetWebhookSubscription(ctx context.Context, id uuid.UUID) (sqlc.WebhookSubscription, error) {
	return sqlc.WebhookSubscription{}, nil
}
func (m *MockQuerier) IncrementSubscriptionFailureCount(ctx context.Context, arg sqlc.IncrementSubscriptionFailureCountParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) IncrementSubscriptionRetryCount(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) ListActiveAgents(ctx context.Context) ([]sqlc.AgentCredential, error) {
	return nil, nil
}
func (m *MockQuerier) ListActiveWebhooksByEvent(ctx context.Context, arg sqlc.ListActiveWebhooksByEventParams) ([]sqlc.WebhookSubscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListAgents(ctx context.Context, arg sqlc.ListAgentsParams) ([]sqlc.AgentCredential, error) {
	return nil, nil
}
func (m *MockQuerier) ListChargebacks(ctx context.Context, arg sqlc.ListChargebacksParams) ([]sqlc.Chargeback, error) {
	return nil, nil
}
func (m *MockQuerier) ListDueSubscriptions(ctx context.Context, arg sqlc.ListDueSubscriptionsParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListPaymentMethods(ctx context.Context, arg sqlc.ListPaymentMethodsParams) ([]sqlc.CustomerPaymentMethod, error) {
	return nil, nil
}
func (m *MockQuerier) ListPaymentMethodsByCustomer(ctx context.Context, arg sqlc.ListPaymentMethodsByCustomerParams) ([]sqlc.CustomerPaymentMethod, error) {
	return nil, nil
}
func (m *MockQuerier) ListPendingWebhookDeliveries(ctx context.Context, limitVal int32) ([]sqlc.WebhookDelivery, error) {
	return nil, nil
}
func (m *MockQuerier) ListSubscriptions(ctx context.Context, arg sqlc.ListSubscriptionsParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListSubscriptionsByCustomer(ctx context.Context, arg sqlc.ListSubscriptionsByCustomerParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListSubscriptionsDueForBilling(ctx context.Context, arg sqlc.ListSubscriptionsDueForBillingParams) ([]sqlc.Subscription, error) {
	return nil, nil
}
func (m *MockQuerier) ListTransactions(ctx context.Context, arg sqlc.ListTransactionsParams) ([]sqlc.Transaction, error) {
	return nil, nil
}
func (m *MockQuerier) ListWebhookSubscriptions(ctx context.Context, arg sqlc.ListWebhookSubscriptionsParams) ([]sqlc.WebhookSubscription, error) {
	return nil, nil
}
func (m *MockQuerier) MarkChargebackResolved(ctx context.Context, arg sqlc.MarkChargebackResolvedParams) error {
	return nil
}
func (m *MockQuerier) MarkPaymentMethodAsDefault(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) MarkPaymentMethodUsed(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) MarkPaymentMethodVerified(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) ResetSubscriptionRetryCount(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (m *MockQuerier) SetPaymentMethodAsDefault(ctx context.Context, arg sqlc.SetPaymentMethodAsDefaultParams) error {
	return nil
}
func (m *MockQuerier) UpdateAgent(ctx context.Context, arg sqlc.UpdateAgentParams) (sqlc.AgentCredential, error) {
	return sqlc.AgentCredential{}, nil
}
func (m *MockQuerier) UpdateAgentMACPath(ctx context.Context, arg sqlc.UpdateAgentMACPathParams) error {
	return nil
}
func (m *MockQuerier) UpdateChargeback(ctx context.Context, arg sqlc.UpdateChargebackParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) UpdateChargebackNotes(ctx context.Context, arg sqlc.UpdateChargebackNotesParams) error {
	return nil
}
func (m *MockQuerier) UpdateChargebackResponse(ctx context.Context, arg sqlc.UpdateChargebackResponseParams) error {
	return nil
}
func (m *MockQuerier) UpdateChargebackStatus(ctx context.Context, arg sqlc.UpdateChargebackStatusParams) (sqlc.Chargeback, error) {
	return sqlc.Chargeback{}, nil
}
func (m *MockQuerier) UpdateNextBillingDate(ctx context.Context, arg sqlc.UpdateNextBillingDateParams) error {
	return nil
}
func (m *MockQuerier) UpdateSubscription(ctx context.Context, arg sqlc.UpdateSubscriptionParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) UpdateSubscriptionBilling(ctx context.Context, arg sqlc.UpdateSubscriptionBillingParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) UpdateSubscriptionStatus(ctx context.Context, arg sqlc.UpdateSubscriptionStatusParams) (sqlc.Subscription, error) {
	return sqlc.Subscription{}, nil
}
func (m *MockQuerier) UpdateTransactionStatus(ctx context.Context, arg sqlc.UpdateTransactionStatusParams) error {
	return nil
}
func (m *MockQuerier) UpdateWebhookDeliveryStatus(ctx context.Context, arg sqlc.UpdateWebhookDeliveryStatusParams) (sqlc.WebhookDelivery, error) {
	return sqlc.WebhookDelivery{}, nil
}
func (m *MockQuerier) UpdateWebhookSubscription(ctx context.Context, arg sqlc.UpdateWebhookSubscriptionParams) (sqlc.WebhookSubscription, error) {
	return sqlc.WebhookSubscription{}, nil
}

// MockDatabaseAdapter mocks the DatabaseAdapter interface
type MockDatabaseAdapter struct {
	mock.Mock
	querier *MockQuerier
}

func (m *MockDatabaseAdapter) Queries() sqlc.Querier {
	return m.querier
}

// MockBrowserPostAdapter mocks ports.BrowserPostAdapter
type MockBrowserPostAdapter struct {
	mock.Mock
}

func (m *MockBrowserPostAdapter) BuildFormData(tac, amount, tranNbr, tranGroup, redirectURL string) (*ports.BrowserPostFormData, error) {
	args := m.Called(tac, amount, tranNbr, tranGroup, redirectURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BrowserPostFormData), args.Error(1)
}

func (m *MockBrowserPostAdapter) ParseRedirectResponse(params map[string][]string) (*ports.BrowserPostResponse, error) {
	args := m.Called(params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BrowserPostResponse), args.Error(1)
}

func (m *MockBrowserPostAdapter) ValidateResponseMAC(params map[string][]string, mac string) error {
	args := m.Called(params, mac)
	return args.Error(0)
}

// MockPaymentMethodService mocks PaymentMethodService
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

// TestGetPaymentForm_Success tests successful form generation with PENDING transaction creation
func TestGetPaymentForm_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockQuerier := new(MockQuerier)
	mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockPaymentMethodSvc := new(MockPaymentMethodService)

	handler := &BrowserPostCallbackHandler{
		dbAdapter:        mockDBAdapter,
		browserPost:      mockBrowserPost,
		paymentMethodSvc: mockPaymentMethodSvc,
		logger:           logger,
		epxPostURL:       "https://secure.epxuap.com/browserpost",
		epxCustNbr:       "9001",
		epxMerchNbr:      "900300",
		epxDBAnbr:        "2",
		epxTerminalNbr:   "77",
		callbackBaseURL:  "http://localhost:8081",
	}

	// Mock CreateTransaction to succeed
	mockQuerier.On("CreateTransaction", mock.Anything, mock.MatchedBy(func(arg sqlc.CreateTransactionParams) bool {
		// Verify PENDING status and correct fields
		return arg.Status == "pending" &&
			arg.Type == "charge" &&
			arg.Currency == "USD" &&
			arg.PaymentMethodType == "credit_card" &&
			arg.AgentID == "merchant-123"
	})).Return(sqlc.Transaction{
		ID:      uuid.New(),
		GroupID: uuid.New(),
		Status:  "pending",
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/complete&agent_id=merchant-123", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify response contains required fields
	assert.NotEmpty(t, response["transactionId"])
	assert.NotEmpty(t, response["groupId"])
	assert.Equal(t, "99.99", response["amount"])
	assert.Equal(t, "https://secure.epxuap.com/browserpost", response["postURL"])
	assert.Equal(t, "9001", response["custNbr"])
	assert.Equal(t, "900300", response["merchNbr"])
	assert.Equal(t, "2", response["dBAnbr"])
	assert.Equal(t, "77", response["terminalNbr"])
	assert.Equal(t, "http://localhost:8081/api/v1/payments/browser-post/callback", response["redirectURL"])

	// Verify userData1 contains return_url (state parameter pattern)
	userData1, ok := response["userData1"].(string)
	assert.True(t, ok)
	assert.Contains(t, userData1, "return_url=https://pos.example.com/complete")

	mockQuerier.AssertExpectations(t)
}

// TestGetPaymentForm_MissingRequiredParams tests validation of required parameters
func TestGetPaymentForm_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name          string
		queryParams   string
		expectedError string
		expectedCode  int
	}{
		{
			name:          "Missing amount",
			queryParams:   "?return_url=https://pos.example.com/complete&agent_id=merchant-123",
			expectedError: "amount parameter is required",
			expectedCode:  http.StatusBadRequest,
		},
		{
			name:          "Missing return_url",
			queryParams:   "?amount=99.99&agent_id=merchant-123",
			expectedError: "return_url parameter is required",
			expectedCode:  http.StatusBadRequest,
		},
		{
			name:          "Invalid amount format",
			queryParams:   "?amount=invalid&return_url=https://pos.example.com/complete&agent_id=merchant-123",
			expectedError: "amount must be a valid number",
			expectedCode:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			mockQuerier := new(MockQuerier)
			mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
			mockBrowserPost := new(MockBrowserPostAdapter)
			mockPaymentMethodSvc := new(MockPaymentMethodService)

			handler := &BrowserPostCallbackHandler{
				dbAdapter:        mockDBAdapter,
				browserPost:      mockBrowserPost,
				paymentMethodSvc: mockPaymentMethodSvc,
				logger:           logger,
				epxPostURL:       "https://secure.epxuap.com/browserpost",
				epxCustNbr:       "9001",
				epxMerchNbr:      "900300",
				epxDBAnbr:        "2",
				epxTerminalNbr:   "77",
				callbackBaseURL:  "http://localhost:8081",
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			handler.GetPaymentForm(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedError)
		})
	}
}

// TestGetPaymentForm_DefaultAgentID tests that agent_id defaults to epxCustNbr if not provided
func TestGetPaymentForm_DefaultAgentID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockQuerier := new(MockQuerier)
	mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockPaymentMethodSvc := new(MockPaymentMethodService)

	handler := &BrowserPostCallbackHandler{
		dbAdapter:        mockDBAdapter,
		browserPost:      mockBrowserPost,
		paymentMethodSvc: mockPaymentMethodSvc,
		logger:           logger,
		epxPostURL:       "https://secure.epxuap.com/browserpost",
		epxCustNbr:       "9001",
		epxMerchNbr:      "900300",
		epxDBAnbr:        "2",
		epxTerminalNbr:   "77",
		callbackBaseURL:  "http://localhost:8081",
	}

	// Verify CreateTransaction is called with default agent_id
	mockQuerier.On("CreateTransaction", mock.Anything, mock.MatchedBy(func(arg sqlc.CreateTransactionParams) bool {
		return arg.AgentID == "9001" // Should default to epxCustNbr
	})).Return(sqlc.Transaction{
		ID:      uuid.New(),
		GroupID: uuid.New(),
		Status:  "pending",
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/complete", nil)
	w := httptest.NewRecorder()

	handler.GetPaymentForm(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockQuerier.AssertExpectations(t)
}

// TestGetPaymentForm_MethodNotAllowed tests HTTP method validation
func TestGetPaymentForm_MethodNotAllowed(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockQuerier := new(MockQuerier)
	mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockPaymentMethodSvc := new(MockPaymentMethodService)

	handler := &BrowserPostCallbackHandler{
		dbAdapter:        mockDBAdapter,
		browserPost:      mockBrowserPost,
		paymentMethodSvc: mockPaymentMethodSvc,
		logger:           logger,
		epxPostURL:       "https://secure.epxuap.com/browserpost",
		epxCustNbr:       "9001",
		epxMerchNbr:      "900300",
		epxDBAnbr:        "2",
		epxTerminalNbr:   "77",
		callbackBaseURL:  "http://localhost:8081",
	}

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/complete", nil)
			w := httptest.NewRecorder()

			handler.GetPaymentForm(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

// TestHandleCallback_Success tests successful callback processing with transaction UPDATE
func TestHandleCallback_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockQuerier := new(MockQuerier)
	mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockPaymentMethodSvc := new(MockPaymentMethodService)

	handler := &BrowserPostCallbackHandler{
		dbAdapter:        mockDBAdapter,
		browserPost:      mockBrowserPost,
		paymentMethodSvc: mockPaymentMethodSvc,
		logger:           logger,
		epxPostURL:       "https://secure.epxuap.com/browserpost",
		epxCustNbr:       "9001",
		epxMerchNbr:      "900300",
		epxDBAnbr:        "2",
		epxTerminalNbr:   "77",
		callbackBaseURL:  "http://localhost:8081",
	}

	// Create a PENDING transaction
	txID := uuid.New()
	groupID := uuid.New()
	tranNbr := "1234567890"

	pendingTx := sqlc.Transaction{
		ID:      txID,
		GroupID: groupID,
		Status:  "pending",
		IdempotencyKey: pgtype.Text{
			String: tranNbr,
			Valid:  true,
		},
	}

	// Mock ParseRedirectResponse to return approved transaction
	mockBrowserPost.On("ParseRedirectResponse", mock.Anything).Return(&ports.BrowserPostResponse{
		IsApproved:   true,
		Amount:       "99.99",
		TranNbr:      tranNbr, // Set TranNbr field for idempotency lookup
		AuthGUID:     "test-guid-123",
		AuthResp:     "APPROVED",
		AuthCode:     "OK1234",
		AuthRespText: "Approved",
		AuthCardType: "VISA",
		AuthAVS:      "Y",
		AuthCVV2:     "M",
		RawParams: map[string]string{
			"TRAN_NBR":    tranNbr,
			"USER_DATA_1": "return_url=https://pos.example.com/complete",
		},
	}, nil)

	// Mock GetTransactionByIdempotencyKey to return PENDING transaction
	mockQuerier.On("GetTransactionByIdempotencyKey", mock.Anything, mock.MatchedBy(func(key pgtype.Text) bool {
		return key.String == tranNbr && key.Valid
	})).Return(pendingTx, nil)

	// Mock UpdateTransaction to verify it's called with completed status
	mockQuerier.On("UpdateTransaction", mock.Anything, mock.MatchedBy(func(arg sqlc.UpdateTransactionParams) bool {
		return arg.ID == txID &&
			arg.Status == "completed" &&
			arg.AuthGuid.String == "test-guid-123" &&
			arg.AuthCode.String == "OK1234"
	})).Return(sqlc.Transaction{
		ID:      txID,
		GroupID: groupID,
		Status:  "completed",
	}, nil)

	// Create POST request with form data
	formData := url.Values{}
	formData.Set("TRAN_NBR", tranNbr)
	formData.Set("USER_DATA_1", "return_url=https://pos.example.com/complete")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/browser-post/callback", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleCallback(w, req)

	// Verify redirect response
	body := w.Body.String()
	assert.Contains(t, body, "https://pos.example.com/complete") // Should redirect to return_url
	assert.Contains(t, body, txID.String())                      // Should include transaction_id
	assert.Contains(t, body, groupID.String())                   // Should include group_id
	assert.Contains(t, body, "completed")                        // Should include status

	mockBrowserPost.AssertExpectations(t)
	mockQuerier.AssertExpectations(t)
}

// TestHandleCallback_Failed tests callback with declined transaction
func TestHandleCallback_Failed(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockQuerier := new(MockQuerier)
	mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockPaymentMethodSvc := new(MockPaymentMethodService)

	handler := &BrowserPostCallbackHandler{
		dbAdapter:        mockDBAdapter,
		browserPost:      mockBrowserPost,
		paymentMethodSvc: mockPaymentMethodSvc,
		logger:           logger,
		epxPostURL:       "https://secure.epxuap.com/browserpost",
		epxCustNbr:       "9001",
		epxMerchNbr:      "900300",
		epxDBAnbr:        "2",
		epxTerminalNbr:   "77",
		callbackBaseURL:  "http://localhost:8081",
	}

	txID := uuid.New()
	groupID := uuid.New()
	tranNbr := "1234567890"

	pendingTx := sqlc.Transaction{
		ID:      txID,
		GroupID: groupID,
		Status:  "pending",
		IdempotencyKey: pgtype.Text{
			String: tranNbr,
			Valid:  true,
		},
	}

	// Mock ParseRedirectResponse to return declined transaction
	mockBrowserPost.On("ParseRedirectResponse", mock.Anything).Return(&ports.BrowserPostResponse{
		IsApproved:   false,
		Amount:       "99.99",
		TranNbr:      tranNbr, // Set TranNbr field for idempotency lookup
		AuthResp:     "DECLINED",
		AuthRespText: "Insufficient funds",
		RawParams: map[string]string{
			"TRAN_NBR":    tranNbr,
			"USER_DATA_1": "return_url=https://pos.example.com/complete",
		},
	}, nil)

	mockQuerier.On("GetTransactionByIdempotencyKey", mock.Anything, mock.Anything).Return(pendingTx, nil)

	// Verify UpdateTransaction is called with failed status
	mockQuerier.On("UpdateTransaction", mock.Anything, mock.MatchedBy(func(arg sqlc.UpdateTransactionParams) bool {
		return arg.ID == txID && arg.Status == "failed"
	})).Return(sqlc.Transaction{
		ID:      txID,
		GroupID: groupID,
		Status:  "failed",
	}, nil)

	formData := url.Values{}
	formData.Set("TRAN_NBR", tranNbr)
	formData.Set("USER_DATA_1", "return_url=https://pos.example.com/complete")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/browser-post/callback", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleCallback(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "failed")

	mockBrowserPost.AssertExpectations(t)
	mockQuerier.AssertExpectations(t)
}

// TestExtractReturnURL tests the extractReturnURL helper method
func TestExtractReturnURL(t *testing.T) {
	logger := zaptest.NewLogger(t)
	handler := &BrowserPostCallbackHandler{
		logger: logger,
	}

	tests := []struct {
		name        string
		rawParams   map[string]string
		expectedURL string
	}{
		{
			name: "Valid return_url",
			rawParams: map[string]string{
				"USER_DATA_1": "return_url=https://pos.example.com/complete",
			},
			expectedURL: "https://pos.example.com/complete",
		},
		{
			name: "Return URL with query params",
			rawParams: map[string]string{
				"USER_DATA_1": "return_url=https://pos.example.com/complete?order=123&session=abc",
			},
			expectedURL: "https://pos.example.com/complete?order=123&session=abc",
		},
		{
			name:        "Missing USER_DATA_1",
			rawParams:   map[string]string{},
			expectedURL: "",
		},
		{
			name: "Invalid format",
			rawParams: map[string]string{
				"USER_DATA_1": "invalid_format",
			},
			expectedURL: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.extractReturnURL(tt.rawParams)
			assert.Equal(t, tt.expectedURL, result)
		})
	}
}

// TestGetPaymentForm_UniqueTransactionNumbers verifies uniqueness across multiple requests
func TestGetPaymentForm_UniqueTransactionNumbers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockQuerier := new(MockQuerier)
	mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockPaymentMethodSvc := new(MockPaymentMethodService)

	handler := &BrowserPostCallbackHandler{
		dbAdapter:        mockDBAdapter,
		browserPost:      mockBrowserPost,
		paymentMethodSvc: mockPaymentMethodSvc,
		logger:           logger,
		epxPostURL:       "https://secure.epxuap.com/browserpost",
		epxCustNbr:       "9001",
		epxMerchNbr:      "900300",
		epxDBAnbr:        "2",
		epxTerminalNbr:   "77",
		callbackBaseURL:  "http://localhost:8081",
	}

	// Mock CreateTransaction to always succeed
	mockQuerier.On("CreateTransaction", mock.Anything, mock.Anything).Return(sqlc.Transaction{
		ID:      uuid.New(),
		GroupID: uuid.New(),
		Status:  "pending",
	}, nil)

	tranNbrs := make(map[string]bool)

	// Make 10 rapid requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/complete&agent_id=test", nil)
		w := httptest.NewRecorder()

		handler.GetPaymentForm(w, req)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		tranNbr := response["tranNbr"].(string)
		assert.False(t, tranNbrs[tranNbr], "Duplicate transaction number: %s", tranNbr)
		tranNbrs[tranNbr] = true

		// Small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	assert.Equal(t, 10, len(tranNbrs), "Should have 10 unique transaction numbers")
}

// BenchmarkGetPaymentForm benchmarks the GetPaymentForm handler
func BenchmarkGetPaymentForm(b *testing.B) {
	logger := zaptest.NewLogger(b)
	mockQuerier := new(MockQuerier)
	mockDBAdapter := &MockDatabaseAdapter{querier: mockQuerier}
	mockBrowserPost := new(MockBrowserPostAdapter)
	mockPaymentMethodSvc := new(MockPaymentMethodService)

	handler := &BrowserPostCallbackHandler{
		dbAdapter:        mockDBAdapter,
		browserPost:      mockBrowserPost,
		paymentMethodSvc: mockPaymentMethodSvc,
		logger:           logger,
		epxPostURL:       "https://secure.epxuap.com/browserpost",
		epxCustNbr:       "9001",
		epxMerchNbr:      "900300",
		epxDBAnbr:        "2",
		epxTerminalNbr:   "77",
		callbackBaseURL:  "http://localhost:8081",
	}

	mockQuerier.On("CreateTransaction", mock.Anything, mock.Anything).Return(sqlc.Transaction{
		ID:      uuid.New(),
		GroupID: uuid.New(),
		Status:  "pending",
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/complete&agent_id=test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.GetPaymentForm(w, req)
	}
}
