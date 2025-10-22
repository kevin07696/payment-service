package north

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kevin07696/payment-service/internal/domain/models"
	"github.com/kevin07696/payment-service/internal/domain/ports"
	pkgerrors "github.com/kevin07696/payment-service/pkg/errors"
	"github.com/kevin07696/payment-service/test/mocks"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupACHTest(t *testing.T, handler http.HandlerFunc) (*ACHAdapter, *httptest.Server) {
	server := httptest.NewServer(handler)

	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}

	adapter := NewACHAdapter(config, server.URL, httpClient, logger)

	return adapter, server
}

func TestACHAdapter_ProcessPayment_Checking_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		// Parse form data
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		assert.Equal(t, "7000", formData.Get("CUST_NBR"))
		assert.Equal(t, "700010", formData.Get("MERCH_NBR"))
		assert.Equal(t, "1", formData.Get("DBA_NBR"))
		assert.Equal(t, "1", formData.Get("TERMINAL_NBR"))
		assert.Equal(t, "CKC2", formData.Get("TRAN_TYPE")) // Checking debit
		assert.Equal(t, "100.00", formData.Get("AMOUNT"))
		assert.Equal(t, "123456789", formData.Get("ROUTING_NBR"))
		assert.Equal(t, "987654321", formData.Get("ACCOUNT_NBR"))

		// Send XML success response
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">ACCEPTED</FIELD>
		<FIELD KEY="AUTH_GUID">ABC123XYZ</FIELD>
		<FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">******4321</FIELD>
		<FIELD KEY="AUTH_AMOUNT">100.00</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		Currency:      "USD",
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
		SECCode:       models.SECCodeWEB,
		BillingInfo: models.BillingInfo{
			FirstName: "John",
			LastName:  "Doe",
			Address:   "123 Main St",
			City:      "Wilmington",
			State:     "DE",
			ZipCode:   "12345",
		},
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "ABC123XYZ", result.GatewayTransactionID)
	assert.Equal(t, models.StatusCaptured, result.Status)
	assert.Equal(t, "00", result.ResponseCode)
	assert.Equal(t, "ACCEPTED", result.Message)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(100.00)))
}

func TestACHAdapter_ProcessPayment_Savings_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		// Verify savings account transaction type
		assert.Equal(t, "CKS2", formData.Get("TRAN_TYPE")) // Savings debit

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">ACCEPTED</FIELD>
		<FIELD KEY="AUTH_GUID">SAVINGS-GUID</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(50.00),
		AccountType:   models.AccountTypeSavings,
		RoutingNumber: "123456789",
		AccountNumber: "111222333",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "SAVINGS-GUID", result.GatewayTransactionID)
}

func TestACHAdapter_ProcessPayment_MissingBankInfo(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not make request with missing bank info")
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "", // Missing
		AccountNumber: "987654321",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	validationErr, ok := err.(*pkgerrors.ValidationError)
	require.True(t, ok)
	assert.Equal(t, "bank_account", validationErr.Field)
}

func TestACHAdapter_ProcessPayment_InvalidRouting(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">78</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">INVALID ROUTING NUMBER</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "999999999",
		AccountNumber: "987654321",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "78", paymentErr.Code)
	assert.Equal(t, pkgerrors.CategoryInvalidCard, paymentErr.Category)
}

func TestACHAdapter_ProcessPayment_InvalidAccount(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">14</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">INVALID ACCOUNT NUMBER</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "000000000",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "14", paymentErr.Code)
}

func TestACHAdapter_ProcessPayment_WithSECCode(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		// Verify SEC code was included
		assert.Equal(t, "WEB", formData.Get("STD_ENTRY_CLASS"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">ACCEPTED</FIELD>
		<FIELD KEY="AUTH_GUID">WEB-GUID</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
		SECCode:       models.SECCodeWEB,
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestACHAdapter_ProcessPayment_CorporateCCD(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		// Verify receiver name for CCD
		assert.Equal(t, "CCD", formData.Get("STD_ENTRY_CLASS"))
		assert.Equal(t, "Acme Corporation", formData.Get("RECV_NAME"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">ACCEPTED</FIELD>
		<FIELD KEY="AUTH_GUID">CCD-GUID</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(1000.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
		SECCode:       models.SECCodeCCD,
		ReceiverName:  "Acme Corporation",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "CCD-GUID", result.GatewayTransactionID)
}

func TestACHAdapter_RefundPayment_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		assert.Equal(t, "CKC3", formData.Get("TRAN_TYPE")) // Checking credit (refund)
		assert.Equal(t, "50.00", formData.Get("AMOUNT"))
		assert.Equal(t, "ORIG-GUID-123", formData.Get("ORIG_AUTH_GUID"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">REFUND ACCEPTED</FIELD>
		<FIELD KEY="AUTH_GUID">REFUND-GUID</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	result, err := adapter.RefundPayment(context.Background(), "ORIG-GUID-123", decimal.NewFromFloat(50.00))

	require.NoError(t, err)
	assert.Equal(t, "ORIG-GUID-123", result.TransactionID)
	assert.Equal(t, "REFUND-GUID", result.GatewayTransactionID)
	assert.Equal(t, models.StatusRefunded, result.Status)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(50.00)))
}

func TestACHAdapter_VerifyBankAccount_Valid(t *testing.T) {
	adapter, server := setupACHTest(t, nil)
	defer server.Close()

	req := &ports.BankAccountVerificationRequest{
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
		AccountType:   models.AccountTypeChecking,
	}

	result, err := adapter.VerifyBankAccount(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, result.Verified)
	assert.Equal(t, "00", result.ResponseCode)
}

func TestACHAdapter_VerifyBankAccount_InvalidRouting(t *testing.T) {
	adapter, server := setupACHTest(t, nil)
	defer server.Close()

	req := &ports.BankAccountVerificationRequest{
		RoutingNumber: "12345", // Too short
		AccountNumber: "987654321",
		AccountType:   models.AccountTypeChecking,
	}

	result, err := adapter.VerifyBankAccount(context.Background(), req)

	require.NoError(t, err)
	assert.False(t, result.Verified)
	assert.Equal(t, "78", result.ResponseCode)
}

func TestACHAdapter_VerifyBankAccount_MissingAccount(t *testing.T) {
	adapter, server := setupACHTest(t, nil)
	defer server.Close()

	req := &ports.BankAccountVerificationRequest{
		RoutingNumber: "123456789",
		AccountNumber: "", // Missing
		AccountType:   models.AccountTypeChecking,
	}

	result, err := adapter.VerifyBankAccount(context.Background(), req)

	require.NoError(t, err)
	assert.False(t, result.Verified)
	assert.Equal(t, "14", result.ResponseCode)
}

func TestACHAdapter_NetworkError(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}
	adapter := NewACHAdapter(config, "http://invalid-url:9999", httpClient, logger)

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "NETWORK_ERROR", paymentErr.Code)
	assert.True(t, paymentErr.IsRetriable)
}

func TestACHAdapter_DependencyInjection(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-key",
	}

	mockLogger := mocks.NewMockLogger()
	mockHTTP := mocks.NewMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">ACCEPTED</FIELD>
		<FIELD KEY="AUTH_GUID">MOCK-GUID</FIELD>
	</FIELDS>
</RESPONSE>`

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(xmlResp))),
		}, nil
	})

	adapter := NewACHAdapter(config, "http://test.com", mockHTTP, mockLogger)

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
	}

	_, err := adapter.ProcessPayment(context.Background(), req)

	require.NoError(t, err)
	assert.Len(t, mockHTTP.Calls, 1)
	assert.Len(t, mockLogger.InfoCalls, 1)
	assert.Equal(t, "making request to North ACH API", mockLogger.InfoCalls[0].Message)
}

func TestACHAdapter_XMLResponseParsing(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Test complex XML response with multiple fields
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="AUTH_RESP_TEXT">ACCEPTED 123456</FIELD>
		<FIELD KEY="AUTH_GUID">COMPLEX-GUID-789</FIELD>
		<FIELD KEY="AUTH_MASKED_ACCOUNT_NBR">******7890</FIELD>
		<FIELD KEY="AUTH_AMOUNT">250.00</FIELD>
		<FIELD KEY="LOCAL_DATE">102025</FIELD>
		<FIELD KEY="LOCAL_TIME">143022</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(xmlResp))
	}

	adapter, server := setupACHTest(t, handler)
	defer server.Close()

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(250.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "COMPLEX-GUID-789", result.GatewayTransactionID)
	assert.Equal(t, "ACCEPTED 123456", result.Message)
}

func TestACHAdapter_InvalidEPIIdFormat(t *testing.T) {
	config := AuthConfig{
		EPIId:  "invalid-format", // Missing parts
		EPIKey: "test-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}
	adapter := NewACHAdapter(config, "http://test.com", httpClient, logger)

	req := &ports.ACHPaymentRequest{
		Amount:        decimal.NewFromFloat(100.00),
		AccountType:   models.AccountTypeChecking,
		RoutingNumber: "123456789",
		AccountNumber: "987654321",
	}

	result, err := adapter.ProcessPayment(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	paymentErr, ok := err.(*pkgerrors.PaymentError)
	require.True(t, ok)
	assert.Equal(t, "CONFIG_ERROR", paymentErr.Code)
	assert.Contains(t, strings.ToLower(paymentErr.Message), "invalid")
}
