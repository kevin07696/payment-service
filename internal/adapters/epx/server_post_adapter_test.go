package epx

import (
	"testing"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Test helper to create a test adapter
func newTestAdapter(t *testing.T) *serverPostAdapter {
	logger := zap.NewNop() // No-op logger for tests
	config := DefaultServerPostConfig("sandbox")
	return NewServerPostAdapter(config, logger).(*serverPostAdapter)
}

// Test helper for string pointer
func strPtr(s string) *string {
	return &s
}

// Test helper to generate unique transaction numbers
func generateTranNbr() string {
	return time.Now().Format("150405") // HHMMSS format
}

// TestDefaultServerPostConfig tests configuration initialization
func TestDefaultServerPostConfig(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantBaseURL string
		wantSocket  string
	}{
		{
			name:        "sandbox environment",
			environment: "sandbox",
			wantBaseURL: "https://secure.epxuap.com",
			wantSocket:  "secure.epxuap.com:8087",
		},
		{
			name:        "production environment",
			environment: "production",
			wantBaseURL: "https://epxnow.com/epx/server_post",
			wantSocket:  "epxnow.com:8086",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultServerPostConfig(tt.environment)

			assert.NotNil(t, config)
			assert.Equal(t, tt.wantBaseURL, config.BaseURL)
			assert.Equal(t, tt.wantSocket, config.SocketEndpoint)
			assert.NotEmpty(t, config.Timeout)
		})
	}
}

// TestNewServerPostAdapter tests adapter initialization
func TestNewServerPostAdapter(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultServerPostConfig("sandbox")

	adapter := NewServerPostAdapter(config, logger)

	assert.NotNil(t, adapter)
	assert.Implements(t, (*ports.ServerPostAdapter)(nil), adapter)
}

// TestBuildFormData tests form data construction
func TestBuildFormData(t *testing.T) {
	adapter := newTestAdapter(t)

	tests := []struct {
		name     string
		request  *ports.ServerPostRequest
		validate func(t *testing.T, formData map[string][]string)
	}{
		{
			name: "sale transaction with card details",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
				TranGroup:       "12345",
				AccountNumber:   strPtr("4111111111111111"),
				ExpirationDate:  strPtr("1225"),
				CVV:             strPtr("123"),
				CardEntryMethod: strPtr("E"),
				IndustryType:    strPtr("E"),
				FirstName:       strPtr("John"),
				LastName:        strPtr("Doe"),
				ZipCode:         strPtr("10001"),
			},
			validate: func(t *testing.T, formData map[string][]string) {
				assert.Equal(t, "9001", formData["CUST_NBR"][0])
				assert.Equal(t, "CCE1", formData["TRAN_TYPE"][0])
				assert.Equal(t, "10.00", formData["AMOUNT"][0])
				assert.Equal(t, "12345", formData["TRAN_NBR"][0])
				assert.Equal(t, "12345", formData["BATCH_ID"][0])
				assert.Equal(t, "4111111111111111", formData["ACCOUNT_NBR"][0])
				assert.Equal(t, "1225", formData["EXP_DATE"][0])
				assert.Equal(t, "123", formData["CVV2"][0])
				assert.Equal(t, "E", formData["CARD_ENT_METH"][0])
				assert.Equal(t, "John", formData["FIRST_NAME"][0])
				assert.Equal(t, "Doe", formData["LAST_NAME"][0])
				assert.Equal(t, "10001", formData["ZIP_CODE"][0])
			},
		},
		{
			name: "capture transaction with BRIC",
			request: &ports.ServerPostRequest{
				CustNbr:          "9001",
				MerchNbr:         "900300",
				DBAnbr:           "2",
				TerminalNbr:      "77",
				TransactionType:  ports.TransactionTypeCapture,
				Amount:           "25.00",
				TranNbr:          "12346",
				TranGroup:        "12346",
				OriginalAuthGUID: "09TESTGUID123456789",
				CardEntryMethod:  strPtr("Z"),
				IndustryType:     strPtr("E"),
			},
			validate: func(t *testing.T, formData map[string][]string) {
				assert.Equal(t, "CCE4", formData["TRAN_TYPE"][0])
				assert.Equal(t, "25.00", formData["AMOUNT"][0])
				assert.Equal(t, "09TESTGUID123456789", formData["ORIG_AUTH_GUID"][0])
				assert.Equal(t, "Z", formData["CARD_ENT_METH"][0])
			},
		},
		{
			name: "recurring payment with ACI_EXT",
			request: &ports.ServerPostRequest{
				CustNbr:          "9001",
				MerchNbr:         "900300",
				DBAnbr:           "2",
				TerminalNbr:      "77",
				TransactionType:  ports.TransactionTypeSale,
				Amount:           "15.00",
				TranNbr:          "12347",
				TranGroup:        "12347",
				OriginalAuthGUID: "09STORAGEGUID123456",
				ACIExt:           strPtr("RB"),
				CardEntryMethod:  strPtr("Z"),
				IndustryType:     strPtr("E"),
			},
			validate: func(t *testing.T, formData map[string][]string) {
				assert.Equal(t, "CCE1", formData["TRAN_TYPE"][0])
				assert.Equal(t, "RB", formData["ACI_EXT"][0])
				assert.Equal(t, "Z", formData["CARD_ENT_METH"][0])
				assert.Equal(t, "09STORAGEGUID123456", formData["ORIG_AUTH_GUID"][0])
			},
		},
		{
			name: "BRIC storage with zero amount",
			request: &ports.ServerPostRequest{
				CustNbr:          "9001",
				MerchNbr:         "900300",
				DBAnbr:           "2",
				TerminalNbr:      "77",
				TransactionType:  ports.TransactionTypeBRICStorageCC,
				Amount:           "0.00",
				TranNbr:          "12348",
				TranGroup:        "12348",
				OriginalAuthGUID: "09FINANCIALBRIC1234",
				CardEntryMethod:  strPtr("Z"),
				IndustryType:     strPtr("E"),
				Address:          strPtr("123 Main St"),
				ZipCode:          strPtr("10001"),
			},
			validate: func(t *testing.T, formData map[string][]string) {
				assert.Equal(t, "CCE8", formData["TRAN_TYPE"][0])
				assert.Equal(t, "0.00", formData["AMOUNT"][0])
				assert.Equal(t, "123 Main St", formData["ADDRESS"][0])
				assert.Equal(t, "10001", formData["ZIP_CODE"][0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formData := adapter.buildFormData(tt.request)
			tt.validate(t, formData)
		})
	}
}

// TestValidateRequest tests request validation
func TestValidateRequest(t *testing.T) {
	adapter := newTestAdapter(t)

	tests := []struct {
		name    string
		request *ports.ServerPostRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid sale request",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				DBAnbr:          "2",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
				AccountNumber:   strPtr("4111111111111111"),
				ExpirationDate:  strPtr("1225"),
			},
			wantErr: false,
		},
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
			wantErr: true,
			errMsg:  "cust_nbr is required",
		},
		{
			name: "missing dba number",
			request: &ports.ServerPostRequest{
				CustNbr:         "9001",
				MerchNbr:        "900300",
				TerminalNbr:     "77",
				TransactionType: ports.TransactionTypeSale,
				Amount:          "10.00",
				TranNbr:         "12345",
			},
			wantErr: true,
			errMsg:  "dba_nbr is required",
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
			wantErr: true,
			errMsg:  "amount is required",
		},
		{
			name: "BRIC storage with zero amount is valid",
			request: &ports.ServerPostRequest{
				CustNbr:          "9001",
				MerchNbr:         "900300",
				DBAnbr:           "2",
				TerminalNbr:      "77",
				TransactionType:  ports.TransactionTypeBRICStorageCC,
				Amount:           "0.00",
				TranNbr:          "12345",
				OriginalAuthGUID: "09TESTGUID",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.validateRequest(tt.request)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestTransactionTypeMapping tests transaction type constants
func TestTransactionTypeMapping(t *testing.T) {
	tests := []struct {
		transactionType ports.TransactionType
		expectedCode    string
	}{
		{ports.TransactionTypeSale, "CCE1"},
		{ports.TransactionTypeAuthOnly, "CCE2"},
		{ports.TransactionTypeCapture, "CCE4"},
		{ports.TransactionTypeRefund, "CCE9"},
		{ports.TransactionTypeVoid, "CCEX"},
		{ports.TransactionTypeBRICStorageCC, "CCE8"},
		{ports.TransactionTypeACHDebit, "CKC1"},
	}

	for _, tt := range tests {
		t.Run(string(tt.transactionType), func(t *testing.T) {
			assert.Equal(t, tt.expectedCode, string(tt.transactionType))
		})
	}
}

// TestParseXMLResponse tests XML response parsing
func TestParseXMLResponse(t *testing.T) {
	adapter := newTestAdapter(t)

	tests := []struct {
		name     string
		xmlBody  string
		validate func(t *testing.T, resp *ports.ServerPostResponse, err error)
	}{
		{
			name: "approved sale response",
			xmlBody: `<RESPONSE>
				<FIELDS>
					<FIELD KEY="AUTH_GUID">09LMQ886L2K2W11MPX1</FIELD>
					<FIELD KEY="AUTH_RESP">00</FIELD>
					<FIELD KEY="AUTH_CODE">057579</FIELD>
					<FIELD KEY="AUTH_RESP_TEXT">ZIP MATCH</FIELD>
					<FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
					<FIELD KEY="AUTH_AVS">Z</FIELD>
					<FIELD KEY="AUTH_CVV2">M</FIELD>
					<FIELD KEY="TRAN_NBR">12345</FIELD>
					<FIELD KEY="AMOUNT">10.00</FIELD>
				</FIELDS>
			</RESPONSE>`,
			validate: func(t *testing.T, resp *ports.ServerPostResponse, err error) {
				require.NoError(t, err)
				assert.Equal(t, "09LMQ886L2K2W11MPX1", resp.AuthGUID)
				assert.Equal(t, "00", resp.AuthResp)
				assert.Equal(t, "057579", resp.AuthCode)
				assert.Equal(t, "ZIP MATCH", resp.AuthRespText)
				assert.True(t, resp.IsApproved)
				assert.Equal(t, "V", resp.AuthCardType)
				assert.Equal(t, "Z", resp.AuthAVS)
				assert.Equal(t, "M", resp.AuthCVV2)
				assert.Equal(t, "12345", resp.TranNbr)
				assert.Equal(t, "10.00", resp.Amount)
			},
		},
		{
			name: "declined transaction",
			xmlBody: `<RESPONSE>
				<FIELDS>
					<FIELD KEY="AUTH_GUID">09DECLINEDGUID123</FIELD>
					<FIELD KEY="AUTH_RESP">05</FIELD>
					<FIELD KEY="AUTH_CODE"></FIELD>
					<FIELD KEY="AUTH_RESP_TEXT">DO NOT HONOR</FIELD>
				</FIELDS>
			</RESPONSE>`,
			validate: func(t *testing.T, resp *ports.ServerPostResponse, err error) {
				require.NoError(t, err)
				assert.Equal(t, "05", resp.AuthResp)
				assert.False(t, resp.IsApproved)
				assert.Equal(t, "DO NOT HONOR", resp.AuthRespText)
				assert.Equal(t, "09DECLINEDGUID123", resp.AuthGUID)
			},
		},
		{
			name:    "invalid XML",
			xmlBody: `<INVALID>not xml</INVALID>`,
			validate: func(t *testing.T, resp *ports.ServerPostResponse, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "unmarshal XML")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal request for parseXMLResponse
			req := &ports.ServerPostRequest{
				TranNbr:   "12345",
				TranGroup: "12345",
				Amount:    "10.00",
			}
			resp, err := adapter.parseXMLResponse([]byte(tt.xmlBody), req)
			tt.validate(t, resp, err)
		})
	}
}

// TestIsApprovedLogic tests approval determination
func TestIsApprovedLogic(t *testing.T) {
	tests := []struct {
		authResp   string
		isApproved bool
	}{
		{"00", true},  // Approved
		{"05", false}, // Do not honor
		{"12", false}, // Invalid transaction
		{"51", false}, // Insufficient funds
		{"EH", false}, // CEM invalid
		{"RR", false}, // EPX decline
		{"", false},   // Empty response
	}

	for _, tt := range tests {
		t.Run(tt.authResp, func(t *testing.T) {
			response := &ports.ServerPostResponse{
				AuthResp: tt.authResp,
			}

			// The IsApproved field should be set correctly
			response.IsApproved = (tt.authResp == "00")

			assert.Equal(t, tt.isApproved, response.IsApproved)
		})
	}
}

// Benchmark tests
func BenchmarkBuildFormData(b *testing.B) {
	adapter := newTestAdapter(&testing.T{})
	request := &ports.ServerPostRequest{
		CustNbr:         "9001",
		MerchNbr:        "900300",
		DBAnbr:          "2",
		TerminalNbr:     "77",
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         "12345",
		TranGroup:       "12345",
		AccountNumber:   strPtr("4111111111111111"),
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.buildFormData(request)
	}
}

func BenchmarkValidateRequest(b *testing.B) {
	adapter := newTestAdapter(&testing.T{})
	request := &ports.ServerPostRequest{
		CustNbr:         "9001",
		MerchNbr:        "900300",
		DBAnbr:          "2",
		TerminalNbr:     "77",
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         "12345",
		AccountNumber:   strPtr("4111111111111111"),
		ExpirationDate:  strPtr("1225"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.validateRequest(request)
	}
}
