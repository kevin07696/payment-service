package mocks

import (
	"context"
	"sync"

	"github.com/kevin07696/payment-service/internal/domain/ports"
	"github.com/shopspring/decimal"
)

// MockRecurringGateway is a mock implementation of RecurringBillingGateway for testing
type MockRecurringGateway struct {
	mu sync.Mutex

	// Responses to return
	createResponse *ports.SubscriptionResult
	createError    error
	updateResponse *ports.SubscriptionResult
	updateError    error
	cancelResponse *ports.SubscriptionResult
	cancelError    error
	pauseResponse  *ports.SubscriptionResult
	pauseError     error
	resumeResponse *ports.SubscriptionResult
	resumeError    error
	getResponse    *ports.SubscriptionResult
	getError       error
	listResponse   []*ports.SubscriptionResult
	listError      error
	chargeResponse *ports.PaymentResult
	chargeError    error

	// Call tracking
	CreateCalls int
	UpdateCalls int
	CancelCalls int
	PauseCalls  int
	ResumeCalls int
	GetCalls    int
	ListCalls   int
	ChargeCalls int

	// Last request received
	LastCreateReq *ports.SubscriptionRequest
	LastUpdateReq *ports.UpdateSubscriptionRequest
	LastUpdateID  string
	LastCancelID  string
	LastPauseID   string
	LastResumeID  string
	LastGetID     string
	LastListCustID string
	LastChargePaymentMethodID string
	LastChargeAmount          string
}

// NewMockRecurringGateway creates a new mock recurring billing gateway
func NewMockRecurringGateway() *MockRecurringGateway {
	return &MockRecurringGateway{}
}

// SetCreateResponse sets the response to return from CreateSubscription
func (m *MockRecurringGateway) SetCreateResponse(result *ports.SubscriptionResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createResponse = result
	m.createError = err
}

// SetUpdateResponse sets the response to return from UpdateSubscription
func (m *MockRecurringGateway) SetUpdateResponse(result *ports.SubscriptionResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateResponse = result
	m.updateError = err
}

// SetCancelResponse sets the response to return from CancelSubscription
func (m *MockRecurringGateway) SetCancelResponse(result *ports.SubscriptionResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelResponse = result
	m.cancelError = err
}

// SetPauseResponse sets the response to return from PauseSubscription
func (m *MockRecurringGateway) SetPauseResponse(result *ports.SubscriptionResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pauseResponse = result
	m.pauseError = err
}

// SetResumeResponse sets the response to return from ResumeSubscription
func (m *MockRecurringGateway) SetResumeResponse(result *ports.SubscriptionResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resumeResponse = result
	m.resumeError = err
}

// SetChargeResponse sets the response to return from ChargePaymentMethod
func (m *MockRecurringGateway) SetChargeResponse(result *ports.PaymentResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chargeResponse = result
	m.chargeError = err
}

// CreateSubscription implements RecurringBillingGateway.CreateSubscription
func (m *MockRecurringGateway) CreateSubscription(ctx context.Context, req *ports.SubscriptionRequest) (*ports.SubscriptionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateCalls++
	m.LastCreateReq = req
	return m.createResponse, m.createError
}

// UpdateSubscription implements RecurringBillingGateway.UpdateSubscription
func (m *MockRecurringGateway) UpdateSubscription(ctx context.Context, subscriptionID string, req *ports.UpdateSubscriptionRequest) (*ports.SubscriptionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateCalls++
	m.LastUpdateID = subscriptionID
	m.LastUpdateReq = req
	return m.updateResponse, m.updateError
}

// CancelSubscription implements RecurringBillingGateway.CancelSubscription
func (m *MockRecurringGateway) CancelSubscription(ctx context.Context, subscriptionID string, immediate bool) (*ports.SubscriptionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CancelCalls++
	m.LastCancelID = subscriptionID
	return m.cancelResponse, m.cancelError
}

// PauseSubscription implements RecurringBillingGateway.PauseSubscription
func (m *MockRecurringGateway) PauseSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PauseCalls++
	m.LastPauseID = subscriptionID
	return m.pauseResponse, m.pauseError
}

// ResumeSubscription implements RecurringBillingGateway.ResumeSubscription
func (m *MockRecurringGateway) ResumeSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ResumeCalls++
	m.LastResumeID = subscriptionID
	return m.resumeResponse, m.resumeError
}

// GetSubscription implements RecurringBillingGateway.GetSubscription
func (m *MockRecurringGateway) GetSubscription(ctx context.Context, subscriptionID string) (*ports.SubscriptionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetCalls++
	m.LastGetID = subscriptionID
	return m.getResponse, m.getError
}

// ListSubscriptions implements RecurringBillingGateway.ListSubscriptions
func (m *MockRecurringGateway) ListSubscriptions(ctx context.Context, customerID string) ([]*ports.SubscriptionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ListCalls++
	m.LastListCustID = customerID
	return m.listResponse, m.listError
}

// ChargePaymentMethod implements RecurringBillingGateway.ChargePaymentMethod
func (m *MockRecurringGateway) ChargePaymentMethod(ctx context.Context, paymentMethodID string, amount decimal.Decimal) (*ports.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChargeCalls++
	m.LastChargePaymentMethodID = paymentMethodID
	m.LastChargeAmount = amount.String()
	return m.chargeResponse, m.chargeError
}

// Reset resets all mock state
func (m *MockRecurringGateway) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createResponse = nil
	m.createError = nil
	m.updateResponse = nil
	m.updateError = nil
	m.cancelResponse = nil
	m.cancelError = nil
	m.pauseResponse = nil
	m.pauseError = nil
	m.resumeResponse = nil
	m.resumeError = nil
	m.getResponse = nil
	m.getError = nil
	m.listResponse = nil
	m.listError = nil
	m.chargeResponse = nil
	m.chargeError = nil
	m.CreateCalls = 0
	m.UpdateCalls = 0
	m.CancelCalls = 0
	m.PauseCalls = 0
	m.ResumeCalls = 0
	m.GetCalls = 0
	m.ListCalls = 0
	m.ChargeCalls = 0
	m.LastCreateReq = nil
	m.LastUpdateReq = nil
	m.LastUpdateID = ""
	m.LastCancelID = ""
	m.LastPauseID = ""
	m.LastResumeID = ""
	m.LastGetID = ""
	m.LastListCustID = ""
	m.LastChargePaymentMethodID = ""
	m.LastChargeAmount = ""
}
