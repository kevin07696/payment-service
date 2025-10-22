package mocks

import (
	"context"
	"sync"

	"github.com/kevin07696/payment-service/internal/domain/ports"
)

// MockCreditCardGateway is a mock implementation of CreditCardGateway for testing
type MockCreditCardGateway struct {
	mu sync.Mutex

	// Responses to return
	authorizeResponse *ports.PaymentResult
	authorizeError    error
	captureResponse   *ports.PaymentResult
	captureError      error
	voidResponse      *ports.PaymentResult
	voidError         error
	refundResponse    *ports.PaymentResult
	refundError       error
	verifyResponse    *ports.VerificationResult
	verifyError       error

	// Call tracking
	AuthorizeCalls int
	CaptureCalls   int
	VoidCalls      int
	RefundCalls    int
	VerifyCalls    int

	// Last request received
	LastAuthorizeReq *ports.AuthorizeRequest
	LastCaptureReq   *ports.CaptureRequest
	LastVoidTxID     string
	LastRefundReq    *ports.RefundRequest
	LastVerifyReq    *ports.VerifyAccountRequest
}

// NewMockCreditCardGateway creates a new mock credit card gateway
func NewMockCreditCardGateway() *MockCreditCardGateway {
	return &MockCreditCardGateway{}
}

// SetAuthorizeResponse sets the response to return from Authorize
func (m *MockCreditCardGateway) SetAuthorizeResponse(result *ports.PaymentResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authorizeResponse = result
	m.authorizeError = err
}

// SetCaptureResponse sets the response to return from Capture
func (m *MockCreditCardGateway) SetCaptureResponse(result *ports.PaymentResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.captureResponse = result
	m.captureError = err
}

// SetVoidResponse sets the response to return from Void
func (m *MockCreditCardGateway) SetVoidResponse(result *ports.PaymentResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.voidResponse = result
	m.voidError = err
}

// SetRefundResponse sets the response to return from Refund
func (m *MockCreditCardGateway) SetRefundResponse(result *ports.PaymentResult, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refundResponse = result
	m.refundError = err
}

// SetSaleResponse is a helper to set authorize response (for sale operations)
func (m *MockCreditCardGateway) SetSaleResponse(result *ports.PaymentResult, err error) {
	m.SetAuthorizeResponse(result, err)
}

// Authorize implements CreditCardGateway.Authorize
func (m *MockCreditCardGateway) Authorize(ctx context.Context, req *ports.AuthorizeRequest) (*ports.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AuthorizeCalls++
	m.LastAuthorizeReq = req
	return m.authorizeResponse, m.authorizeError
}

// Capture implements CreditCardGateway.Capture
func (m *MockCreditCardGateway) Capture(ctx context.Context, req *ports.CaptureRequest) (*ports.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CaptureCalls++
	m.LastCaptureReq = req
	return m.captureResponse, m.captureError
}

// Void implements CreditCardGateway.Void
func (m *MockCreditCardGateway) Void(ctx context.Context, transactionID string) (*ports.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.VoidCalls++
	m.LastVoidTxID = transactionID
	return m.voidResponse, m.voidError
}

// Refund implements CreditCardGateway.Refund
func (m *MockCreditCardGateway) Refund(ctx context.Context, req *ports.RefundRequest) (*ports.PaymentResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RefundCalls++
	m.LastRefundReq = req
	return m.refundResponse, m.refundError
}

// VerifyAccount implements CreditCardGateway.VerifyAccount
func (m *MockCreditCardGateway) VerifyAccount(ctx context.Context, req *ports.VerifyAccountRequest) (*ports.VerificationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.VerifyCalls++
	m.LastVerifyReq = req
	return m.verifyResponse, m.verifyError
}

// Reset resets all mock state
func (m *MockCreditCardGateway) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authorizeResponse = nil
	m.authorizeError = nil
	m.captureResponse = nil
	m.captureError = nil
	m.voidResponse = nil
	m.voidError = nil
	m.refundResponse = nil
	m.refundError = nil
	m.verifyResponse = nil
	m.verifyError = nil
	m.AuthorizeCalls = 0
	m.CaptureCalls = 0
	m.VoidCalls = 0
	m.RefundCalls = 0
	m.VerifyCalls = 0
	m.LastAuthorizeReq = nil
	m.LastCaptureReq = nil
	m.LastVoidTxID = ""
	m.LastRefundReq = nil
	m.LastVerifyReq = nil
}
