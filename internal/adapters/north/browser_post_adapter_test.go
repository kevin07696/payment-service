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

func setupBrowserPostTest(t *testing.T, handler http.HandlerFunc) (*BrowserPostAdapter, *httptest.Server) {
	server := httptest.NewServer(handler)

	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}

	adapter := NewBrowserPostAdapter(config, server.URL, httpClient, logger)

	return adapter, server
}

func TestBrowserPostAdapter_Authorize_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.Equal(t, "7000-700010-1-1", r.Header.Get("EPI-Id"))
		assert.NotEmpty(t, r.Header.Get("EPI-Signature"))

		// Parse form data
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		assert.Equal(t, "7000", formData.Get("CUST_NBR"))
		assert.Equal(t, "700010", formData.Get("MERCH_NBR"))
		assert.Equal(t, "tok_bric_123456", formData.Get("BRIC"))
		assert.Equal(t, "100.00", formData.Get("AMOUNT"))
		assert.Equal(t, "A", formData.Get("TRAN_TYPE")) // Authorization only

		// Send XML success response
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_abc123</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123456",
		Capture:  false,
		BillingInfo: models.BillingInfo{
			ZipCode: "19801",
			Address: "123 Main St",
		},
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "txn_abc123", result.TransactionID)
	assert.Equal(t, models.StatusAuthorized, result.Status)
	assert.Equal(t, "00", result.ResponseCode)
	assert.Equal(t, "123456", result.AuthCode)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(100.00)))
}

func TestBrowserPostAdapter_Authorize_Sale(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		// Verify sale transaction type
		assert.Equal(t, "S", formData.Get("TRAN_TYPE")) // Sale (auth + capture)

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">789012</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_sale_456</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(50.00),
		Currency: "USD",
		Token:    "tok_bric_789",
		Capture:  true, // Sale mode
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, models.StatusCaptured, result.Status)
	assert.Equal(t, "txn_sale_456", result.TransactionID)
}

func TestBrowserPostAdapter_Authorize_MissingToken(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Should not make request with missing token")
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "", // Missing
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var validationErr *pkgerrors.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "token", validationErr.Field)
}

func TestBrowserPostAdapter_Authorize_DeclinedCard(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">05</FIELD>
		<FIELD KEY="RESP_TEXT">DECLINED</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_declined",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var paymentErr *pkgerrors.PaymentError
	require.ErrorAs(t, err, &paymentErr)
	assert.Equal(t, "05", paymentErr.Code)
	assert.Equal(t, pkgerrors.CategoryDeclined, paymentErr.Category)
}

func TestBrowserPostAdapter_Capture_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/sale/")
		assert.Contains(t, r.URL.Path, "/capture")

		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		assert.Equal(t, "txn_auth_123", formData.Get("TRANSACTION_ID"))
		assert.Equal(t, "75.50", formData.Get("AMOUNT"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">CAPTURED</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_auth_123</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.CaptureRequest{
		TransactionID: "txn_auth_123",
		Amount:        decimal.NewFromFloat(75.50),
	}

	result, err := adapter.Capture(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, models.StatusCaptured, result.Status)
	assert.Equal(t, "txn_auth_123", result.TransactionID)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(75.50)))
}

func TestBrowserPostAdapter_Capture_MissingTransactionID(t *testing.T) {
	adapter, server := setupBrowserPostTest(t, nil)
	defer server.Close()

	req := &ports.CaptureRequest{
		TransactionID: "",
		Amount:        decimal.NewFromFloat(100.00),
	}

	result, err := adapter.Capture(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var validationErr *pkgerrors.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "transaction_id", validationErr.Field)
}

func TestBrowserPostAdapter_Void_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/void/")
		assert.Contains(t, r.URL.Path, "txn_void_123")

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">VOIDED</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_void_123</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	result, err := adapter.Void(context.Background(), "txn_void_123")

	require.NoError(t, err)
	assert.Equal(t, models.StatusVoided, result.Status)
	assert.Equal(t, "txn_void_123", result.TransactionID)
}

func TestBrowserPostAdapter_Void_MissingTransactionID(t *testing.T) {
	adapter, server := setupBrowserPostTest(t, nil)
	defer server.Close()

	result, err := adapter.Void(context.Background(), "")

	require.Error(t, err)
	assert.Nil(t, result)

	var validationErr *pkgerrors.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "transaction_id", validationErr.Field)
}

func TestBrowserPostAdapter_Refund_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/refund/")
		assert.Contains(t, r.URL.Path, "txn_refund_789")

		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		assert.Equal(t, "txn_refund_789", formData.Get("TRANSACTION_ID"))
		assert.Equal(t, "50.00", formData.Get("AMOUNT"))
		assert.Equal(t, "Customer requested", formData.Get("REASON"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">REFUNDED</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_refund_new</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.RefundRequest{
		TransactionID: "txn_refund_789",
		Amount:        decimal.NewFromFloat(50.00),
		Reason:        "Customer requested",
	}

	result, err := adapter.Refund(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, models.StatusRefunded, result.Status)
	assert.True(t, result.Amount.Equal(decimal.NewFromFloat(50.00)))
}

func TestBrowserPostAdapter_Refund_MissingTransactionID(t *testing.T) {
	adapter, server := setupBrowserPostTest(t, nil)
	defer server.Close()

	req := &ports.RefundRequest{
		TransactionID: "",
		Amount:        decimal.NewFromFloat(100.00),
	}

	result, err := adapter.Refund(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var validationErr *pkgerrors.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "transaction_id", validationErr.Field)
}

func TestBrowserPostAdapter_VerifyAccount_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/verify")

		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		assert.Equal(t, "tok_verify_123", formData.Get("BRIC"))
		assert.Equal(t, "0.00", formData.Get("AMOUNT"))
		assert.Equal(t, "V", formData.Get("TRAN_TYPE"))
		assert.Equal(t, "19801", formData.Get("ZIP_CODE"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">85</FIELD>
		<FIELD KEY="RESP_TEXT">VERIFIED</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.VerifyAccountRequest{
		Token: "tok_verify_123",
		BillingInfo: models.BillingInfo{
			ZipCode: "19801",
		},
	}

	result, err := adapter.VerifyAccount(context.Background(), req)

	require.NoError(t, err)
	assert.True(t, result.Verified)
	assert.Equal(t, "85", result.ResponseCode)
}

func TestBrowserPostAdapter_VerifyAccount_MissingToken(t *testing.T) {
	adapter, server := setupBrowserPostTest(t, nil)
	defer server.Close()

	req := &ports.VerifyAccountRequest{
		Token: "",
	}

	result, err := adapter.VerifyAccount(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var validationErr *pkgerrors.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Equal(t, "token", validationErr.Field)
}

func TestBrowserPostAdapter_VerifyAccount_NetworkError(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}
	adapter := NewBrowserPostAdapter(config, "http://invalid-url:9999", httpClient, logger)

	req := &ports.VerifyAccountRequest{
		Token: "tok_test",
	}

	result, err := adapter.VerifyAccount(context.Background(), req)

	// Network errors should return unverified, not error
	require.NoError(t, err)
	assert.False(t, result.Verified)
	assert.Equal(t, "96", result.ResponseCode)
}

func TestBrowserPostAdapter_NetworkError(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-secret-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}
	adapter := NewBrowserPostAdapter(config, "http://invalid-url:9999", httpClient, logger)

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_test",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var paymentErr *pkgerrors.PaymentError
	require.ErrorAs(t, err, &paymentErr)
	assert.Equal(t, "NETWORK_ERROR", paymentErr.Code)
	assert.True(t, paymentErr.IsRetriable)
}

func TestBrowserPostAdapter_GatewayError_5xx(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_test",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var paymentErr *pkgerrors.PaymentError
	require.ErrorAs(t, err, &paymentErr)
	assert.Equal(t, "GATEWAY_ERROR", paymentErr.Code)
	assert.True(t, paymentErr.IsRetriable)
}

func TestBrowserPostAdapter_BadRequest_4xx(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_test",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var paymentErr *pkgerrors.PaymentError
	require.ErrorAs(t, err, &paymentErr)
	assert.Equal(t, "REQUEST_ERROR", paymentErr.Code)
	assert.False(t, paymentErr.IsRetriable)
}

func TestBrowserPostAdapter_DependencyInjection(t *testing.T) {
	config := AuthConfig{
		EPIId:  "7000-700010-1-1",
		EPIKey: "test-key",
	}

	mockLogger := mocks.NewMockLogger()
	mockHTTP := mocks.NewMockHTTPClient(func(req *http.Request) (*http.Response, error) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_mock_001</FIELD>
	</FIELDS>
</RESPONSE>`

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(xmlResp))),
		}, nil
	})

	adapter := NewBrowserPostAdapter(config, "http://test.com", mockHTTP, mockLogger)

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_test",
	}

	_, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Len(t, mockHTTP.Calls, 1)
	assert.Len(t, mockLogger.InfoCalls, 1)
	assert.Equal(t, "making request to North Browser Post API", mockLogger.InfoCalls[0].Message)
}

func TestBrowserPostAdapter_InvalidEPIIdFormat(t *testing.T) {
	config := AuthConfig{
		EPIId:  "invalid-format", // Missing parts
		EPIKey: "test-key",
	}

	logger := mocks.NewMockLogger()
	httpClient := &http.Client{}
	adapter := NewBrowserPostAdapter(config, "http://test.com", httpClient, logger)

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_test",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, result)

	var paymentErr *pkgerrors.PaymentError
	require.ErrorAs(t, err, &paymentErr)
	assert.Equal(t, "CONFIG_ERROR", paymentErr.Code)
	assert.Contains(t, strings.ToLower(paymentErr.Message), "invalid")
}

func TestBrowserPostAdapter_HMACSignature(t *testing.T) {
	var capturedSignature string

	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedSignature = r.Header.Get("EPI-Signature")
		assert.NotEmpty(t, capturedSignature)
		assert.Equal(t, "7000-700010-1-1", r.Header.Get("EPI-Id"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_001</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_test",
	}

	_, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.NotEmpty(t, capturedSignature)
	assert.Len(t, capturedSignature, 64) // SHA-256 hex digest is 64 characters
}

// AVS/CVV Tests

func TestBrowserPostAdapter_AVS_FullMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		formData, _ := url.ParseQuery(string(body))

		// Verify all billing fields are sent
		assert.Equal(t, "19801", formData.Get("ZIP_CODE"))
		assert.Equal(t, "123 Main St", formData.Get("ADDRESS"))
		assert.Equal(t, "Wilmington", formData.Get("CITY"))
		assert.Equal(t, "DE", formData.Get("STATE"))
		assert.Equal(t, "John", formData.Get("FIRST_NAME"))
		assert.Equal(t, "Doe", formData.Get("LAST_NAME"))

		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_avs_match</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
		BillingInfo: models.BillingInfo{
			FirstName: "John",
			LastName:  "Doe",
			Address:   "123 Main St",
			City:      "Wilmington",
			State:     "DE",
			ZipCode:   "19801",
		},
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse) // Full match
	assert.Equal(t, "M", result.CVVResponse) // CVV match
	assert.Equal(t, models.StatusAuthorized, result.Status)
}

func TestBrowserPostAdapter_AVS_NoMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_avs_nomatch</FIELD>
		<FIELD KEY="AUTH_CARD_K">N</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
		BillingInfo: models.BillingInfo{
			ZipCode: "12345",
			Address: "456 Wrong St",
		},
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "N", result.AVSResponse) // No match
	assert.Equal(t, "M", result.CVVResponse)
}

func TestBrowserPostAdapter_AVS_ZipOnlyMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_avs_zip</FIELD>
		<FIELD KEY="AUTH_CARD_K">Z</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
		BillingInfo: models.BillingInfo{
			ZipCode: "19801",
			Address: "Wrong Address",
		},
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Z", result.AVSResponse) // ZIP match only
	assert.Equal(t, "M", result.CVVResponse)
}

func TestBrowserPostAdapter_AVS_AddressOnlyMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_avs_addr</FIELD>
		<FIELD KEY="AUTH_CARD_K">A</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
		BillingInfo: models.BillingInfo{
			ZipCode: "00000",
			Address: "123 Main St",
		},
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "A", result.AVSResponse) // Address match only
	assert.Equal(t, "M", result.CVVResponse)
}

func TestBrowserPostAdapter_AVS_Unavailable(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_avs_unavail</FIELD>
		<FIELD KEY="AUTH_CARD_K">U</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "U", result.AVSResponse) // AVS unavailable
	assert.Equal(t, "M", result.CVVResponse)
}

func TestBrowserPostAdapter_AVS_Retry(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_avs_retry</FIELD>
		<FIELD KEY="AUTH_CARD_K">R</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "R", result.AVSResponse) // Retry
	assert.Equal(t, "M", result.CVVResponse)
}

func TestBrowserPostAdapter_CVV_Match(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_cvv_match</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse)
	assert.Equal(t, "M", result.CVVResponse) // CVV match
}

func TestBrowserPostAdapter_CVV_NoMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_cvv_nomatch</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">N</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse)
	assert.Equal(t, "N", result.CVVResponse) // CVV no match
}

func TestBrowserPostAdapter_CVV_NotProcessed(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_cvv_notproc</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">P</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse)
	assert.Equal(t, "P", result.CVVResponse) // CVV not processed
}

func TestBrowserPostAdapter_CVV_Unavailable(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_cvv_unavail</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">U</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse)
	assert.Equal(t, "U", result.CVVResponse) // CVV unavailable
}

func TestBrowserPostAdapter_Capture_WithAVSCVV(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">CAPTURED</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_cap_123</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.CaptureRequest{
		TransactionID: "txn_cap_123",
		Amount:        decimal.NewFromFloat(100.00),
	}

	result, err := adapter.Capture(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse)
	assert.Equal(t, "M", result.CVVResponse)
	assert.Equal(t, models.StatusCaptured, result.Status)
}

func TestBrowserPostAdapter_Void_WithAVSCVV(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">VOIDED</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_void_123</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	result, err := adapter.Void(context.Background(), "txn_void_123")

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse)
	assert.Equal(t, "M", result.CVVResponse)
	assert.Equal(t, models.StatusVoided, result.Status)
}

func TestBrowserPostAdapter_Refund_WithAVSCVV(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">REFUNDED</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_refund_new</FIELD>
		<FIELD KEY="AUTH_CARD_K">Y</FIELD>
		<FIELD KEY="AUTH_CARD_L">M</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.RefundRequest{
		TransactionID: "txn_refund_789",
		Amount:        decimal.NewFromFloat(50.00),
	}

	result, err := adapter.Refund(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Y", result.AVSResponse)
	assert.Equal(t, "M", result.CVVResponse)
	assert.Equal(t, models.StatusRefunded, result.Status)
}

func TestBrowserPostAdapter_MissingAVSCVV(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Response without AVS/CVV fields
		xmlResp := `<?xml version="1.0"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="RESP_CODE">00</FIELD>
		<FIELD KEY="RESP_TEXT">APPROVED</FIELD>
		<FIELD KEY="AUTH_CODE">123456</FIELD>
		<FIELD KEY="TRANSACTION_ID">txn_no_avs</FIELD>
	</FIELDS>
</RESPONSE>`

		w.Write([]byte(xmlResp))
	}

	adapter, server := setupBrowserPostTest(t, handler)
	defer server.Close()

	req := &ports.AuthorizeRequest{
		Amount:   decimal.NewFromFloat(100.00),
		Currency: "USD",
		Token:    "tok_bric_123",
	}

	result, err := adapter.Authorize(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "", result.AVSResponse) // Empty when not provided
	assert.Equal(t, "", result.CVVResponse) // Empty when not provided
	assert.Equal(t, models.StatusAuthorized, result.Status)
}
