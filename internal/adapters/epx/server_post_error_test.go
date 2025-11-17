package epx

import (
	"errors"
	"testing"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateRequest_ErrorCases tests validation error handling
func TestValidateRequest_ErrorCases(t *testing.T) {
	adapter := newTestAdapter(t)

	tests := []struct {
		name        string
		request     *ports.ServerPostRequest
		expectedErr string
	}{
		{
			name: "missing customer number",
			request: &ports.ServerPostRequest{
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			expectedErr: "cust_nbr is required",
		},
		{
			name: "missing merchant number",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			expectedErr: "merch_nbr is required",
		},
		{
			name: "missing DBA number",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			expectedErr: "dba_nbr is required",
		},
		{
			name: "missing terminal number",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			expectedErr: "terminal_nbr is required",
		},
		{
			name: "missing tran_nbr",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
			},
			expectedErr: "tran_nbr is required",
		},
		{
			name: "missing amount for sale",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				TranNbr:         "12345",
			},
			expectedErr: "amount is required",
		},
		{
			name: "invalid amount format",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "invalid",
				TranNbr:         "12345",
			},
			expectedErr: "amount must be numeric",
		},
		{
			name: "invalid transaction type",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: "INVALID",
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			expectedErr: "invalid transaction type",
		},
		{
			name: "missing OriginalAuthGUID for refund",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeRefund,
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			expectedErr: "original_auth_guid is required for",
		},
		{
			name: "missing OriginalAuthGUID for capture",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeCapture,
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			expectedErr: "original_auth_guid is required for",
		},
		{
			name: "missing OriginalAuthGUID for void",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeVoid,
				Amount:          "0.00", // VOID transactions may use $0.00
				TranNbr:         "12345",
			},
			expectedErr: "original_auth_guid is required for",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.validateRequest(tt.request)
			require.Error(t, err, "Should return validation error")
			assert.Contains(t, err.Error(), tt.expectedErr, "Error message should match")
		})
	}
}

// TestParseResponse_ErrorCases tests response parsing error handling
func TestParseXMLResponse_ErrorCases(t *testing.T) {
	adapter := newTestAdapter(t)

	// Mock request for parseXMLResponse
	mockReq := &ports.ServerPostRequest{
		CustNbr:         "9001",
		MerchNbr:        "900300",
		DBAnbr:          "2",
		TerminalNbr:     "77",
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         "12345",
	}

	tests := []struct {
		name        string
		responseXML string
		expectedErr string
	}{
		{
			name:        "empty response",
			responseXML: "",
			expectedErr: "EOF",
		},
		{
			name:        "invalid XML",
			responseXML: "not valid xml",
			expectedErr: "failed to unmarshal XML",
		},
		{
			name: "missing AUTH_GUID",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_RESP">00</FIELD>
		<FIELD KEY="TRAN_NBR">12345</FIELD>
	</FIELDS>
</RESPONSE>`,
			expectedErr: "AUTH_GUID is missing",
		},
		{
			name: "missing AUTH_RESP",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
	<FIELDS>
		<FIELD KEY="AUTH_GUID">test-guid-123</FIELD>
		<FIELD KEY="TRAN_NBR">12345</FIELD>
	</FIELDS>
</RESPONSE>`,
			expectedErr: "AUTH_RESP is missing",
		},
		{
			name: "malformed XML structure",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
	<FIELD KEY="TRAN_NBR">12345
</RESPONSE>`,
			expectedErr: "failed to unmarshal XML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := adapter.parseXMLResponse([]byte(tt.responseXML), mockReq)

			if tt.expectedErr != "" {
				require.Error(t, err, "Should return parsing error")
				assert.Contains(t, err.Error(), tt.expectedErr, "Error message should match")
			} else {
				require.NoError(t, err, "Should parse successfully")
				assert.NotNil(t, resp, "Response should not be nil")
			}
		})
	}
}

// TestIsRetryable tests retry logic for different error scenarios
func TestIsRetryable(t *testing.T) {
	adapter := newTestAdapter(t)

	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "nil error should not retry",
			err:         nil,
			shouldRetry: false,
		},
		{
			name:        "network timeout should retry",
			err:         errors.New("i/o timeout"),
			shouldRetry: true,
		},
		{
			name:        "connection refused should retry",
			err:         errors.New("connection refused"),
			shouldRetry: true,
		},
		{
			name:        "connection reset should retry",
			err:         errors.New("connection reset by peer"),
			shouldRetry: true,
		},
		{
			name:        "temporary network error should retry",
			err:         errors.New("temporary failure in name resolution"),
			shouldRetry: true,
		},
		{
			name:        "validation error should not retry",
			err:         errors.New("invalid request"),
			shouldRetry: false,
		},
		{
			name:        "authentication error should not retry",
			err:         errors.New("authentication failed"),
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.isRetryable(tt.err)
			assert.Equal(t, tt.shouldRetry, result, "Retry decision should match expectation")
		})
	}
}

// TestBuildFormData_AllTransactionTypes tests form data for different transaction types
func TestBuildFormData_AllTransactionTypes(t *testing.T) {
	adapter := newTestAdapter(t)

	accountNumber := "4111111111111111"
	expirationDate := "1225"

	tests := []struct {
		name            string
		transactionType ports.TransactionType
		request         *ports.ServerPostRequest
		requiredFields  []string
	}{
		{
			name:            "SALE transaction",
			transactionType: ports.TransactionTypeSale,
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
				AccountNumber:   &accountNumber,
				ExpirationDate:  &expirationDate,
			},
			requiredFields: []string{"CUST_NBR", "MERCH_NBR", "DBA_NBR", "TERMINAL_NBR", "TRAN_TYPE", "AMOUNT", "TRAN_NBR"},
		},
		{
			name:            "AUTH transaction",
			transactionType: ports.TransactionTypeAuthOnly,
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeAuthOnly,
				Amount:          "10.00",
				TranNbr:         "12345",
				AccountNumber:   &accountNumber,
				ExpirationDate:  &expirationDate,
			},
			requiredFields: []string{"CUST_NBR", "MERCH_NBR", "DBA_NBR", "TERMINAL_NBR", "TRAN_TYPE", "AMOUNT", "TRAN_NBR"},
		},
		{
			name:            "REFUND transaction with OriginalAuthGUID",
			transactionType: ports.TransactionTypeRefund,
			request: &ports.ServerPostRequest{
				CustNbr:          "9001",
				MerchNbr:         "900300",
				DBAnbr:           "2",
				TerminalNbr:      "77",
				TransactionType:  ports.TransactionTypeRefund,
				Amount:           "5.00",
				TranNbr:          "12345",
				OriginalAuthGUID: "test-orig-auth-guid",
			},
			requiredFields: []string{"CUST_NBR", "MERCH_NBR", "DBA_NBR", "TERMINAL_NBR", "TRAN_TYPE", "AMOUNT", "TRAN_NBR", "ORIG_AUTH_GUID"},
		},
		{
			name:            "CAPTURE transaction with OriginalAuthGUID",
			transactionType: ports.TransactionTypeCapture,
			request: &ports.ServerPostRequest{
				CustNbr:          "9001",
				MerchNbr:         "900300",
				DBAnbr:           "2",
				TerminalNbr:      "77",
				TransactionType:  ports.TransactionTypeCapture,
				Amount:           "10.00",
				TranNbr:          "12345",
				OriginalAuthGUID: "auth-guid-to-capture",
			},
			requiredFields: []string{"CUST_NBR", "MERCH_NBR", "DBA_NBR", "TERMINAL_NBR", "TRAN_TYPE", "AMOUNT", "TRAN_NBR", "ORIG_AUTH_GUID"},
		},
		{
			name:            "VOID transaction with OriginalAuthGUID",
			transactionType: ports.TransactionTypeVoid,
			request: &ports.ServerPostRequest{
				CustNbr:          "9001",
				MerchNbr:         "900300",
				DBAnbr:           "2",
				TerminalNbr:      "77",
				TransactionType:  ports.TransactionTypeVoid,
				TranNbr:          "12345",
				OriginalAuthGUID: "auth-guid-to-void",
			},
			requiredFields: []string{"CUST_NBR", "MERCH_NBR", "DBA_NBR", "TERMINAL_NBR", "TRAN_TYPE", "TRAN_NBR", "ORIG_AUTH_GUID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formData := adapter.buildFormData(tt.request)
			require.NotNil(t, formData, "Form data should not be nil")

			// Verify required fields are present
			for _, field := range tt.requiredFields {
				values, exists := formData[field]
				assert.True(t, exists, "Field %s should exist in form data", field)
				assert.NotEmpty(t, values, "Field %s should have a value", field)
			}
		})
	}
}
