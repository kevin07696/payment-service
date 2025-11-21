package merchant

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/adapters/database"
	adapterports "github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services/ports"
	"github.com/kevin07696/payment-service/internal/testutil/mocks"
)

// MockTransactionManager implements database.TransactionManager for testing
type MockTransactionManager struct {
	mock.Mock
	querier *mocks.MockQuerier
}

var _ database.TransactionManager = (*MockTransactionManager)(nil)

func (m *MockTransactionManager) WithTx(ctx context.Context, fn func(sqlc.Querier) error) error {
	args := m.Called(ctx, fn)

	// Execute the transaction function with the mock querier
	if err := fn(m.querier); err != nil {
		return err
	}

	return args.Error(0)
}

// MockSecretManagerAdapter implements adapterports.SecretManagerAdapter for testing
type MockSecretManagerAdapter struct {
	mock.Mock
}

func (m *MockSecretManagerAdapter) GetSecret(ctx context.Context, path string) (*adapterports.Secret, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.Secret), args.Error(1)
}

func (m *MockSecretManagerAdapter) GetSecretVersion(ctx context.Context, path string, version string) (*adapterports.Secret, error) {
	args := m.Called(ctx, path, version)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.Secret), args.Error(1)
}

func (m *MockSecretManagerAdapter) PutSecret(ctx context.Context, path, value string, metadata map[string]string) (string, error) {
	args := m.Called(ctx, path, value, metadata)
	return args.String(0), args.Error(1)
}

func (m *MockSecretManagerAdapter) RotateSecret(ctx context.Context, path string, newValue string) (*adapterports.SecretRotationInfo, error) {
	args := m.Called(ctx, path, newValue)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*adapterports.SecretRotationInfo), args.Error(1)
}

func (m *MockSecretManagerAdapter) DeleteSecret(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

// Test setup helper
func setupMerchantService(t *testing.T) (*merchantService, *mocks.MockQuerier, *MockTransactionManager, *MockSecretManagerAdapter) {
	mockQuerier := new(mocks.MockQuerier)
	mockTxManager := new(MockTransactionManager)
	mockTxManager.querier = mockQuerier
	mockSecretManager := new(MockSecretManagerAdapter)
	logger := zap.NewNop()

	service := &merchantService{
		queries:       mockQuerier,
		txManager:     mockTxManager,
		secretManager: mockSecretManager,
		logger:        logger,
	}

	return service, mockQuerier, mockTxManager, mockSecretManager
}

// TestRegisterMerchant_Success tests successful merchant registration
func TestRegisterMerchant_Success(t *testing.T) {
	service, mockQuerier, mockTxManager, mockSecretManager := setupMerchantService(t)
	ctx := context.Background()

	req := &ports.RegisterMerchantRequest{
		AgentID:      "test-merchant",
		MerchantName: "Test Merchant",
		CustNbr:      "12345",
		MerchNbr:     "67890",
		DBAnbr:       "11111",
		TerminalNbr:  "22222",
		MACSecret:    "secret123",
		Environment:  domain.EnvironmentSandbox,
	}

	// Mock merchant doesn't exist check
	mockQuerier.On("MerchantExistsBySlug", ctx, req.AgentID).
		Return(false, nil)

	// Mock secret storage
	macSecretPath := "payment-service/merchants/test-merchant/mac"
	mockSecretManager.On("PutSecret", ctx, macSecretPath, req.MACSecret, mock.Anything).
		Return("", nil)

	// Mock merchant creation
	merchantID := uuid.New()
	expectedMerchant := sqlc.Merchant{
		ID:            merchantID,
		Slug:          req.AgentID,
		Name:          req.MerchantName,
		CustNbr:       req.CustNbr,
		MerchNbr:      req.MerchNbr,
		DbaNbr:        req.DBAnbr,
		TerminalNbr:   req.TerminalNbr,
		MacSecretPath: macSecretPath,
		Environment:   string(req.Environment),
		IsActive:      true,
	}

	mockQuerier.On("CreateMerchant", ctx, mock.AnythingOfType("sqlc.CreateMerchantParams")).
		Return(expectedMerchant, nil)

	// Mock transaction
	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	// Execute
	result, err := service.RegisterMerchant(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, req.AgentID, result.AgentID)
	assert.Equal(t, req.Environment, result.Environment)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
	mockSecretManager.AssertExpectations(t)
}

// TestRegisterMerchant_AlreadyExists tests duplicate merchant registration
func TestRegisterMerchant_AlreadyExists(t *testing.T) {
	service, mockQuerier, _, _ := setupMerchantService(t)
	ctx := context.Background()

	req := &ports.RegisterMerchantRequest{
		AgentID:      "existing-merchant",
		MerchantName: "Test Merchant",
		CustNbr:      "12345",
		MerchNbr:     "67890",
		DBAnbr:       "11111",
		TerminalNbr:  "22222",
		MACSecret:    "secret123",
		Environment:  domain.EnvironmentSandbox,
	}

	// Mock merchant exists
	mockQuerier.On("MerchantExistsBySlug", ctx, req.AgentID).
		Return(true, nil)

	// Execute
	result, err := service.RegisterMerchant(ctx, req)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "merchant_id already exists")

	mockQuerier.AssertExpectations(t)
}

// TestRegisterMerchant_MissingCredentials tests validation
func TestRegisterMerchant_MissingCredentials(t *testing.T) {
	service, mockQuerier, _, _ := setupMerchantService(t)
	ctx := context.Background()

	testCases := []struct {
		name          string
		req           *ports.RegisterMerchantRequest
		expectedError string
	}{
		{
			name: "Missing CustNbr",
			req: &ports.RegisterMerchantRequest{
				AgentID:      "test",
				MerchantName: "Test",
				CustNbr:      "", // Missing
				MerchNbr:     "67890",
				DBAnbr:       "11111",
				TerminalNbr:  "22222",
				MACSecret:    "secret",
				Environment:  domain.EnvironmentSandbox,
			},
			expectedError: "all EPX credentials",
		},
		{
			name: "Missing MACSecret",
			req: &ports.RegisterMerchantRequest{
				AgentID:      "test",
				MerchantName: "Test",
				CustNbr:      "12345",
				MerchNbr:     "67890",
				DBAnbr:       "11111",
				TerminalNbr:  "22222",
				MACSecret:    "", // Missing
				Environment:  domain.EnvironmentSandbox,
			},
			expectedError: "mac_secret is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock merchant doesn't exist
			mockQuerier.On("MerchantExistsBySlug", ctx, tc.req.AgentID).
				Return(false, nil).Maybe()

			result, err := service.RegisterMerchant(ctx, tc.req)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

// TestGetMerchant_Success tests successful merchant retrieval
func TestGetMerchant_Success(t *testing.T) {
	service, mockQuerier, _, _ := setupMerchantService(t)
	ctx := context.Background()

	agentID := "test-merchant"
	dbMerchant := sqlc.Merchant{
		ID:            uuid.New(),
		Slug:          agentID,
		Name:          "Test Merchant",
		CustNbr:       "12345",
		MerchNbr:      "67890",
		DbaNbr:        "11111",
		TerminalNbr:   "22222",
		MacSecretPath: "path/to/secret",
		Environment:   "staging",
		IsActive:      true,
	}

	mockQuerier.On("GetMerchantBySlug", ctx, agentID).
		Return(dbMerchant, nil)

	result, err := service.GetMerchant(ctx, agentID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, agentID, result.AgentID)

	mockQuerier.AssertExpectations(t)
}

// TestGetMerchant_NotFound tests merchant not found error
func TestGetMerchant_NotFound(t *testing.T) {
	service, mockQuerier, _, _ := setupMerchantService(t)
	ctx := context.Background()

	agentID := "nonexistent"

	mockQuerier.On("GetMerchantBySlug", ctx, agentID).
		Return(sqlc.Merchant{}, fmt.Errorf("not found"))

	result, err := service.GetMerchant(ctx, agentID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "merchant not found")

	mockQuerier.AssertExpectations(t)
}

// TestListMerchants_Success tests successful merchant listing
func TestListMerchants_Success(t *testing.T) {
	service, mockQuerier, _, _ := setupMerchantService(t)
	ctx := context.Background()

	env := domain.EnvironmentSandbox
	active := true

	dbMerchants := []sqlc.Merchant{
		{
			ID:          uuid.New(),
			Slug:        "merchant-1",
			Name:        "Merchant 1",
			Environment: "sandbox",
			IsActive:    true,
		},
		{
			ID:          uuid.New(),
			Slug:        "merchant-2",
			Name:        "Merchant 2",
			Environment: "sandbox",
			IsActive:    true,
		},
	}

	mockQuerier.On("ListMerchants", ctx, mock.MatchedBy(func(params sqlc.ListMerchantsParams) bool {
		return params.Environment.Valid && params.Environment.String == "sandbox" &&
			params.IsActive.Valid && params.IsActive.Bool == true
	})).Return(dbMerchants, nil)

	mockQuerier.On("CountMerchants", ctx, mock.MatchedBy(func(params sqlc.CountMerchantsParams) bool {
		return params.Environment.Valid && params.Environment.String == "sandbox"
	})).Return(int64(2), nil)

	result, count, err := service.ListMerchants(ctx, &env, &active, 10, 0)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, count)

	mockQuerier.AssertExpectations(t)
}

// TestUpdateMerchant_Success tests successful merchant update
func TestUpdateMerchant_Success(t *testing.T) {
	service, mockQuerier, mockTxManager, _ := setupMerchantService(t)
	ctx := context.Background()

	newCustNbr := "99999"
	req := &ports.UpdateMerchantRequest{
		AgentID: "test-merchant",
		CustNbr: &newCustNbr,
	}

	existingMerchant := sqlc.Merchant{
		ID:            uuid.New(),
		Slug:          "test-merchant",
		CustNbr:       "12345",
		MerchNbr:      "67890",
		DbaNbr:        "11111",
		TerminalNbr:   "22222",
		MacSecretPath: "path/to/secret",
		Environment:   "staging",
		IsActive:      true,
	}

	updatedMerchant := existingMerchant
	updatedMerchant.CustNbr = newCustNbr

	mockQuerier.On("GetMerchantBySlug", ctx, req.AgentID).
		Return(existingMerchant, nil)

	mockQuerier.On("UpdateMerchant", ctx, mock.MatchedBy(func(params sqlc.UpdateMerchantParams) bool {
		return params.CustNbr == newCustNbr
	})).Return(updatedMerchant, nil)

	mockTxManager.On("WithTx", ctx, mock.AnythingOfType("func(sqlc.Querier) error")).
		Return(nil)

	result, err := service.UpdateMerchant(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newCustNbr, result.CustNbr)

	mockQuerier.AssertExpectations(t)
	mockTxManager.AssertExpectations(t)
}

// TestDeactivateMerchant_Success tests successful merchant deactivation
func TestDeactivateMerchant_Success(t *testing.T) {
	service, mockQuerier, _, _ := setupMerchantService(t)
	ctx := context.Background()

	agentID := "test-merchant"
	merchant := sqlc.Merchant{
		ID:          uuid.New(),
		Slug:        agentID,
		Environment: "staging",
		IsActive:    true,
	}

	mockQuerier.On("GetMerchantBySlug", ctx, agentID).
		Return(merchant, nil)

	mockQuerier.On("DeactivateMerchant", ctx, merchant.ID).
		Return(nil)

	err := service.DeactivateMerchant(ctx, agentID, "testing")

	require.NoError(t, err)

	mockQuerier.AssertExpectations(t)
}

// TestRotateMerchantMAC_Success tests successful MAC rotation
func TestRotateMerchantMAC_Success(t *testing.T) {
	service, mockQuerier, _, mockSecretManager := setupMerchantService(t)
	ctx := context.Background()

	req := &ports.RotateMerchantMACRequest{
		AgentID:      "test-merchant",
		NewMACSecret: "new-secret-123",
	}

	merchant := sqlc.Merchant{
		ID:            uuid.New(),
		Slug:          req.AgentID,
		MacSecretPath: "payment-service/merchants/test-merchant/mac",
		IsActive:      true,
	}

	mockQuerier.On("GetMerchantBySlug", ctx, req.AgentID).
		Return(merchant, nil)

	mockSecretManager.On("PutSecret", ctx, merchant.MacSecretPath, req.NewMACSecret, mock.Anything).
		Return("", nil)

	err := service.RotateMerchantMAC(ctx, req)

	require.NoError(t, err)

	mockQuerier.AssertExpectations(t)
	mockSecretManager.AssertExpectations(t)
}

// TestRotateMerchantMAC_InactiveMerchant tests rotation fails for inactive merchant
func TestRotateMerchantMAC_InactiveMerchant(t *testing.T) {
	service, mockQuerier, _, _ := setupMerchantService(t)
	ctx := context.Background()

	req := &ports.RotateMerchantMACRequest{
		AgentID:      "test-merchant",
		NewMACSecret: "new-secret-123",
	}

	merchant := sqlc.Merchant{
		ID:            uuid.New(),
		Slug:          req.AgentID,
		MacSecretPath: "payment-service/merchants/test-merchant/mac",
		IsActive:      false, // Inactive
	}

	mockQuerier.On("GetMerchantBySlug", ctx, req.AgentID).
		Return(merchant, nil)

	err := service.RotateMerchantMAC(ctx, req)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot rotate MAC for inactive merchant")

	mockQuerier.AssertExpectations(t)
}
