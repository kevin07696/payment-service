//go:build integration
// +build integration

package epx

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// EPXIntegrationTestSuite is the test suite for EPX integration tests
type EPXIntegrationTestSuite struct {
	suite.Suite
	adapter         ports.ServerPostAdapter
	ctx             context.Context
	testCredentials *TestCredentials
}

// TestCredentials holds EPX sandbox credentials
type TestCredentials struct {
	CustNbr     string
	MerchNbr    string
	DBAnbr      string
	TerminalNbr string
}

// SetupSuite runs once before all tests
func (s *EPXIntegrationTestSuite) SetupSuite() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	require.NoError(s.T(), err)

	// Load test credentials from environment or use defaults
	s.testCredentials = &TestCredentials{
		CustNbr:     getEnv("EPX_TEST_CUST_NBR", "9001"),
		MerchNbr:    getEnv("EPX_TEST_MERCH_NBR", "900300"),
		DBAnbr:      getEnv("EPX_TEST_DBA_NBR", "2"),
		TerminalNbr: getEnv("EPX_TEST_TERMINAL_NBR", "77"),
	}

	// Initialize EPX adapter
	config := DefaultServerPostConfig("sandbox")
	s.adapter = NewServerPostAdapter(config, logger)

	s.ctx = context.Background()
}

// SetupTest runs before each test
func (s *EPXIntegrationTestSuite) SetupTest() {
	// Add delay between tests to avoid rate limiting
	time.Sleep(2 * time.Second)
}

// TestRunIntegrationSuite runs the integration test suite
func TestRunIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(EPXIntegrationTestSuite))
}

// Test 1: Sale (CCE1) - Authorization + Capture
func (s *EPXIntegrationTestSuite) TestSaleTransaction() {
	t := s.T()

	request := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		PaymentType:     ports.PaymentMethodTypeCreditCard,
		AccountNumber:   strPtr("4111111111111111"), // Visa test card
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		FirstName:       strPtr("John"),
		LastName:        strPtr("Doe"),
		Address:         strPtr("123 Main St"),
		City:            strPtr("New York"),
		State:           strPtr("NY"),
		ZipCode:         strPtr("10001"),
	}

	response, err := s.adapter.ProcessTransaction(s.ctx, request)

	// Assertions
	require.NoError(t, err, "Sale transaction should not return error")
	require.NotNil(t, response, "Response should not be nil")

	assert.True(t, response.IsApproved, "Transaction should be approved")
	assert.Equal(t, "00", response.AuthResp, "AUTH_RESP should be 00 (approved)")
	assert.NotEmpty(t, response.AuthGUID, "AUTH_GUID should be present")
	assert.NotEmpty(t, response.AuthCode, "AUTH_CODE should be present")
	assert.Equal(t, "V", response.AuthCardType, "Card type should be Visa")
	assert.NotEmpty(t, response.AuthAVS, "AVS response should be present")
	assert.NotEmpty(t, response.AuthCVV2, "CVV response should be present")

	t.Logf("✅ Sale approved: AUTH_GUID=%s, AUTH_CODE=%s", response.AuthGUID, response.AuthCode)
}

// Test 2: Authorization Only (CCE2)
func (s *EPXIntegrationTestSuite) TestAuthorizationOnly() {
	t := s.T()

	request := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeAuthOnly,
		Amount:          "25.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		PaymentType:     ports.PaymentMethodTypeCreditCard,
		AccountNumber:   strPtr("5499740000000057"), // Mastercard test card
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		FirstName:       strPtr("Jane"),
		LastName:        strPtr("Smith"),
		ZipCode:         strPtr("90210"),
	}

	response, err := s.adapter.ProcessTransaction(s.ctx, request)

	require.NoError(t, err)
	require.NotNil(t, response)

	assert.True(t, response.IsApproved)
	assert.Equal(t, "00", response.AuthResp)
	assert.NotEmpty(t, response.AuthGUID)
	assert.Equal(t, "M", response.AuthCardType, "Card type should be Mastercard")

	// Store AUTH_GUID for capture test
	s.Suite.T().Cleanup(func() {
		s.Suite.T().Logf("Auth-Only AUTH_GUID for capture: %s", response.AuthGUID)
	})

	t.Logf("✅ Auth-Only approved: AUTH_GUID=%s", response.AuthGUID)
}

// Test 3: Complete Auth-Capture Flow
func (s *EPXIntegrationTestSuite) TestAuthCaptureFlow() {
	t := s.T()

	// Step 1: Authorization
	authRequest := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeAuthOnly,
		Amount:          "50.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		AccountNumber:   strPtr("5499740000000057"),
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		ZipCode:         strPtr("90210"),
	}

	authResp, err := s.adapter.ProcessTransaction(s.ctx, authRequest)
	require.NoError(t, err)
	require.True(t, authResp.IsApproved, "Authorization should be approved")

	authGUID := authResp.AuthGUID
	t.Logf("✅ Authorization approved: AUTH_GUID=%s", authGUID)

	// Wait before capture
	time.Sleep(2 * time.Second)

	// Step 2: Capture
	captureRequest := &ports.ServerPostRequest{
		CustNbr:          s.testCredentials.CustNbr,
		MerchNbr:         s.testCredentials.MerchNbr,
		DBAnbr:           s.testCredentials.DBAnbr,
		TerminalNbr:      s.testCredentials.TerminalNbr,
		TransactionType:  ports.TransactionTypeCapture,
		Amount:           "50.00",
		TranNbr:          generateUniqueTranNbr(),
		TranGroup:        generateUniqueTranNbr(),
		OriginalAuthGUID: authGUID,
		CardEntryMethod:  strPtr("Z"), // Using BRIC token
		IndustryType:     strPtr("E"),
	}

	captureResp, err := s.adapter.ProcessTransaction(s.ctx, captureRequest)
	require.NoError(t, err)
	require.NotNil(t, captureResp)

	assert.True(t, captureResp.IsApproved, "Capture should be approved")
	assert.Equal(t, "00", captureResp.AuthResp)
	assert.NotEmpty(t, captureResp.AuthGUID)

	t.Logf("✅ Capture approved: AUTH_GUID=%s", captureResp.AuthGUID)
}

// Test 4: Sale and Refund Flow
func (s *EPXIntegrationTestSuite) TestSaleRefundFlow() {
	t := s.T()

	// Step 1: Create a sale
	saleRequest := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeSale,
		Amount:          "20.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		AccountNumber:   strPtr("4111111111111111"),
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		ZipCode:         strPtr("10001"),
	}

	saleResp, err := s.adapter.ProcessTransaction(s.ctx, saleRequest)
	require.NoError(t, err)
	require.True(t, saleResp.IsApproved)

	saleAuthGUID := saleResp.AuthGUID
	t.Logf("✅ Sale approved: AUTH_GUID=%s", saleAuthGUID)

	// Wait before refund
	time.Sleep(2 * time.Second)

	// Step 2: Partial refund
	refundRequest := &ports.ServerPostRequest{
		CustNbr:          s.testCredentials.CustNbr,
		MerchNbr:         s.testCredentials.MerchNbr,
		DBAnbr:           s.testCredentials.DBAnbr,
		TerminalNbr:      s.testCredentials.TerminalNbr,
		TransactionType:  ports.TransactionTypeRefund,
		Amount:           "5.00", // Partial refund
		TranNbr:          generateUniqueTranNbr(),
		TranGroup:        generateUniqueTranNbr(),
		OriginalAuthGUID: saleAuthGUID,
		CardEntryMethod:  strPtr("Z"),
		IndustryType:     strPtr("E"),
	}

	refundResp, err := s.adapter.ProcessTransaction(s.ctx, refundRequest)
	require.NoError(t, err)
	require.NotNil(t, refundResp)

	assert.True(t, refundResp.IsApproved, "Refund should be approved")
	assert.Equal(t, "00", refundResp.AuthResp)
	assert.NotEmpty(t, refundResp.AuthGUID)

	t.Logf("✅ Refund approved: AUTH_GUID=%s, Amount=$5.00", refundResp.AuthGUID)
}

// Test 5: Sale and Void Flow
func (s *EPXIntegrationTestSuite) TestSaleVoidFlow() {
	t := s.T()

	// Step 1: Create a sale to void
	saleRequest := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeSale,
		Amount:          "1.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		AccountNumber:   strPtr("4111111111111111"),
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		ZipCode:         strPtr("10001"),
	}

	saleResp, err := s.adapter.ProcessTransaction(s.ctx, saleRequest)
	require.NoError(t, err)
	require.True(t, saleResp.IsApproved)

	saleAuthGUID := saleResp.AuthGUID
	t.Logf("✅ Sale created: AUTH_GUID=%s", saleAuthGUID)

	// Wait before void
	time.Sleep(2 * time.Second)

	// Step 2: Void the sale
	voidRequest := &ports.ServerPostRequest{
		CustNbr:          s.testCredentials.CustNbr,
		MerchNbr:         s.testCredentials.MerchNbr,
		DBAnbr:           s.testCredentials.DBAnbr,
		TerminalNbr:      s.testCredentials.TerminalNbr,
		TransactionType:  ports.TransactionTypeVoid,
		Amount:           "1.00",
		TranNbr:          generateUniqueTranNbr(),
		TranGroup:        generateUniqueTranNbr(),
		OriginalAuthGUID: saleAuthGUID,
		CardEntryMethod:  strPtr("Z"),
		IndustryType:     strPtr("E"),
	}

	voidResp, err := s.adapter.ProcessTransaction(s.ctx, voidRequest)
	require.NoError(t, err)
	require.NotNil(t, voidResp)

	assert.True(t, voidResp.IsApproved, "Void should be approved")
	assert.Equal(t, "00", voidResp.AuthResp)

	t.Logf("✅ Void approved: AUTH_GUID=%s", voidResp.AuthGUID)
}

// Test 6: BRIC Storage (CCE8)
func (s *EPXIntegrationTestSuite) TestBRICStorage() {
	t := s.T()

	// Step 1: Create a sale to get Financial BRIC
	saleRequest := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		AccountNumber:   strPtr("4111111111111111"),
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		FirstName:       strPtr("John"),
		LastName:        strPtr("Doe"),
		Address:         strPtr("123 Main St"),
		City:            strPtr("New York"),
		State:           strPtr("NY"),
		ZipCode:         strPtr("10001"),
	}

	saleResp, err := s.adapter.ProcessTransaction(s.ctx, saleRequest)
	require.NoError(t, err)
	require.True(t, saleResp.IsApproved)

	financialBRIC := saleResp.AuthGUID
	t.Logf("✅ Sale approved, Financial BRIC: %s", financialBRIC)

	// Wait before BRIC Storage
	time.Sleep(2 * time.Second)

	// Step 2: Convert to Storage BRIC
	bricRequest := &ports.ServerPostRequest{
		CustNbr:          s.testCredentials.CustNbr,
		MerchNbr:         s.testCredentials.MerchNbr,
		DBAnbr:           s.testCredentials.DBAnbr,
		TerminalNbr:      s.testCredentials.TerminalNbr,
		TransactionType:  ports.TransactionTypeBRICStorageCC,
		Amount:           "0.00", // Account Verification
		TranNbr:          generateUniqueTranNbr(),
		TranGroup:        generateUniqueTranNbr(),
		OriginalAuthGUID: financialBRIC,
		CardEntryMethod:  strPtr("Z"),
		IndustryType:     strPtr("E"),
		FirstName:        strPtr("John"),
		LastName:         strPtr("Doe"),
		Address:          strPtr("123 Main St"),
		City:             strPtr("New York"),
		State:            strPtr("NY"),
		ZipCode:          strPtr("10001"),
	}

	bricResp, err := s.adapter.ProcessTransaction(s.ctx, bricRequest)
	require.NoError(t, err)
	require.NotNil(t, bricResp)

	assert.True(t, bricResp.IsApproved, "BRIC Storage should be approved")
	assert.Equal(t, "00", bricResp.AuthResp)
	assert.NotEmpty(t, bricResp.AuthGUID, "Storage BRIC should be present")
	// Note: EPX may return empty Amount for BRIC Storage (CCE8) transactions - this is expected

	storageBRIC := bricResp.AuthGUID
	t.Logf("✅ Storage BRIC created: %s", storageBRIC)
}

// Test 7: Complete Recurring Payment Flow
func (s *EPXIntegrationTestSuite) TestRecurringPaymentFlow() {
	t := s.T()

	// Step 1: Create initial sale
	saleRequest := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		AccountNumber:   strPtr("4111111111111111"),
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		FirstName:       strPtr("John"),
		LastName:        strPtr("Doe"),
		Address:         strPtr("123 Main St"),
		City:            strPtr("New York"),
		State:           strPtr("NY"),
		ZipCode:         strPtr("10001"),
	}

	saleResp, err := s.adapter.ProcessTransaction(s.ctx, saleRequest)
	require.NoError(t, err)
	require.True(t, saleResp.IsApproved)
	t.Logf("✅ Initial sale approved: %s", saleResp.AuthGUID)

	time.Sleep(2 * time.Second)

	// Step 2: Convert to Storage BRIC
	bricRequest := &ports.ServerPostRequest{
		CustNbr:          s.testCredentials.CustNbr,
		MerchNbr:         s.testCredentials.MerchNbr,
		DBAnbr:           s.testCredentials.DBAnbr,
		TerminalNbr:      s.testCredentials.TerminalNbr,
		TransactionType:  ports.TransactionTypeBRICStorageCC,
		Amount:           "0.00",
		TranNbr:          generateUniqueTranNbr(),
		TranGroup:        generateUniqueTranNbr(),
		OriginalAuthGUID: saleResp.AuthGUID,
		CardEntryMethod:  strPtr("Z"),
		IndustryType:     strPtr("E"),
		FirstName:        strPtr("John"),
		LastName:         strPtr("Doe"),
		Address:          strPtr("123 Main St"),
		City:             strPtr("New York"),
		State:            strPtr("NY"),
		ZipCode:          strPtr("10001"),
	}

	bricResp, err := s.adapter.ProcessTransaction(s.ctx, bricRequest)
	require.NoError(t, err)
	require.True(t, bricResp.IsApproved)

	storageBRIC := bricResp.AuthGUID
	t.Logf("✅ Storage BRIC created: %s", storageBRIC)

	time.Sleep(2 * time.Second)

	// Step 3: Process recurring payment using Storage BRIC
	recurringRequest := &ports.ServerPostRequest{
		CustNbr:          s.testCredentials.CustNbr,
		MerchNbr:         s.testCredentials.MerchNbr,
		DBAnbr:           s.testCredentials.DBAnbr,
		TerminalNbr:      s.testCredentials.TerminalNbr,
		TransactionType:  ports.TransactionTypeSale,
		Amount:           "15.00",
		TranNbr:          generateUniqueTranNbr(),
		TranGroup:        generateUniqueTranNbr(),
		OriginalAuthGUID: storageBRIC,    // Use Storage BRIC
		ACIExt:           strPtr("RB"),    // Recurring Billing indicator
		CardEntryMethod:  strPtr("Z"),     // BRIC token
		IndustryType:     strPtr("E"),
	}

	recurringResp, err := s.adapter.ProcessTransaction(s.ctx, recurringRequest)
	require.NoError(t, err)
	require.NotNil(t, recurringResp)

	assert.True(t, recurringResp.IsApproved, "Recurring payment should be approved")
	assert.Equal(t, "00", recurringResp.AuthResp)
	assert.NotEmpty(t, recurringResp.AuthGUID)

	t.Logf("✅ Recurring payment approved: AUTH_GUID=%s, Amount=$15.00", recurringResp.AuthGUID)
}

// Test 8: Error Handling - Invalid Card
func (s *EPXIntegrationTestSuite) TestErrorHandling_InvalidCard() {
	t := s.T()

	request := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		AccountNumber:   strPtr("1234567890123456"), // Invalid card
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
	}

	response, err := s.adapter.ProcessTransaction(s.ctx, request)

	// Should not return an error (network succeeded)
	require.NoError(t, err)
	require.NotNil(t, response)

	// But transaction should be declined
	assert.False(t, response.IsApproved, "Transaction should be declined")
	assert.NotEqual(t, "00", response.AuthResp, "AUTH_RESP should not be 00")
	assert.NotEmpty(t, response.AuthRespText, "Should have decline reason")

	t.Logf("⚠️ Expected decline: AUTH_RESP=%s, Reason=%s", response.AuthResp, response.AuthRespText)
}

// Test 9: Performance - Response Time
func (s *EPXIntegrationTestSuite) TestPerformance_ResponseTime() {
	t := s.T()

	request := &ports.ServerPostRequest{
		CustNbr:         s.testCredentials.CustNbr,
		MerchNbr:        s.testCredentials.MerchNbr,
		DBAnbr:          s.testCredentials.DBAnbr,
		TerminalNbr:     s.testCredentials.TerminalNbr,
		TransactionType: ports.TransactionTypeSale,
		Amount:          "10.00",
		TranNbr:         generateUniqueTranNbr(),
		TranGroup:       generateUniqueTranNbr(),
		AccountNumber:   strPtr("4111111111111111"),
		ExpirationDate:  strPtr("1225"),
		CVV:             strPtr("123"),
		CardEntryMethod: strPtr("E"),
		IndustryType:    strPtr("E"),
		ZipCode:         strPtr("10001"),
	}

	start := time.Now()
	response, err := s.adapter.ProcessTransaction(s.ctx, request)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.True(t, response.IsApproved)

	// EPX typically responds in < 1 second
	assert.Less(t, elapsed, 3*time.Second, "Response should be under 3 seconds")

	t.Logf("⏱️ Response time: %v", elapsed)
}

// Helper Functions

func generateUniqueTranNbr() string {
	return fmt.Sprintf("%d", time.Now().Unix()%100000)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
