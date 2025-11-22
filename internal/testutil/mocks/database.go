// Package mocks provides shared mock implementations for testing.
// This eliminates ~300 lines of duplicated mock code across test files.
package mocks

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/stretchr/testify/mock"
)

// MockQuerier provides a full mock implementation of sqlc.Querier.
// Only methods used in tests need full implementation.
// Unused methods return sensible zero values.
type MockQuerier struct {
	mock.Mock
}

// Frequently used methods - full mock implementation

func (m *MockQuerier) GetMerchantByID(ctx context.Context, id uuid.UUID) (sqlc.Merchant, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) GetMerchantBySlug(ctx context.Context, slug string) (sqlc.Merchant, error) {
	args := m.Called(ctx, slug)
	return args.Get(0).(sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) CreateTransaction(ctx context.Context, arg sqlc.CreateTransactionParams) (sqlc.Transaction, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) GetTransactionByID(ctx context.Context, id uuid.UUID) (sqlc.Transaction, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) GetTransactionByTranNbr(ctx context.Context, tranNbr pgtype.Text) (sqlc.Transaction, error) {
	args := m.Called(ctx, tranNbr)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) UpdateTransactionFromEPXResponse(ctx context.Context, arg sqlc.UpdateTransactionFromEPXResponseParams) (sqlc.Transaction, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) CreatePaymentMethod(ctx context.Context, arg sqlc.CreatePaymentMethodParams) (sqlc.CustomerPaymentMethod, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.CustomerPaymentMethod), args.Error(1)
}

func (m *MockQuerier) GetPaymentMethodByID(ctx context.Context, id uuid.UUID) (sqlc.CustomerPaymentMethod, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.CustomerPaymentMethod), args.Error(1)
}

func (m *MockQuerier) ListPaymentMethods(ctx context.Context, arg sqlc.ListPaymentMethodsParams) ([]sqlc.CustomerPaymentMethod, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.CustomerPaymentMethod), args.Error(1)
}

func (m *MockQuerier) ListPaymentMethodsByCustomer(ctx context.Context, arg sqlc.ListPaymentMethodsByCustomerParams) ([]sqlc.CustomerPaymentMethod, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.CustomerPaymentMethod), args.Error(1)
}

func (m *MockQuerier) GetDefaultPaymentMethod(ctx context.Context, arg sqlc.GetDefaultPaymentMethodParams) (sqlc.CustomerPaymentMethod, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.CustomerPaymentMethod), args.Error(1)
}

func (m *MockQuerier) MarkPaymentMethodAsDefault(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) SetPaymentMethodAsDefault(ctx context.Context, arg sqlc.SetPaymentMethodAsDefaultParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) MarkPaymentMethodVerified(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) MarkPaymentMethodUsed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) DeletePaymentMethod(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) ActivatePaymentMethod(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) DeactivatePaymentMethod(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) CreateSubscription(ctx context.Context, arg sqlc.CreateSubscriptionParams) (sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (sqlc.Subscription, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) UpdateSubscription(ctx context.Context, arg sqlc.UpdateSubscriptionParams) (sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) UpdateSubscriptionStatus(ctx context.Context, arg sqlc.UpdateSubscriptionStatusParams) (sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) UpdateSubscriptionBilling(ctx context.Context, arg sqlc.UpdateSubscriptionBillingParams) (sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) CancelSubscription(ctx context.Context, arg sqlc.CancelSubscriptionParams) (sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) ListSubscriptions(ctx context.Context, arg sqlc.ListSubscriptionsParams) ([]sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) ListSubscriptionsByCustomer(ctx context.Context, arg sqlc.ListSubscriptionsByCustomerParams) ([]sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) ListSubscriptionsDueForBilling(ctx context.Context, arg sqlc.ListSubscriptionsDueForBillingParams) ([]sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) ListDueSubscriptions(ctx context.Context, arg sqlc.ListDueSubscriptionsParams) ([]sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) IncrementSubscriptionFailureCount(ctx context.Context, arg sqlc.IncrementSubscriptionFailureCountParams) (sqlc.Subscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Subscription), args.Error(1)
}

func (m *MockQuerier) IncrementSubscriptionRetryCount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) ResetSubscriptionRetryCount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) UpdateNextBillingDate(ctx context.Context, arg sqlc.UpdateNextBillingDateParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetServiceByID(ctx context.Context, id uuid.UUID) (sqlc.Service, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Service), args.Error(1)
}

func (m *MockQuerier) GetServiceByServiceID(ctx context.Context, serviceID string) (sqlc.Service, error) {
	args := m.Called(ctx, serviceID)
	return args.Get(0).(sqlc.Service), args.Error(1)
}

func (m *MockQuerier) CreateService(ctx context.Context, arg sqlc.CreateServiceParams) (sqlc.Service, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Service), args.Error(1)
}

func (m *MockQuerier) UpdateService(ctx context.Context, arg sqlc.UpdateServiceParams) (sqlc.Service, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Service), args.Error(1)
}

func (m *MockQuerier) ListServices(ctx context.Context, arg sqlc.ListServicesParams) ([]sqlc.Service, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Service), args.Error(1)
}

func (m *MockQuerier) ActivateService(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) DeactivateService(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) RotateServiceKey(ctx context.Context, arg sqlc.RotateServiceKeyParams) (sqlc.Service, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Service), args.Error(1)
}

func (m *MockQuerier) GrantServiceAccess(ctx context.Context, arg sqlc.GrantServiceAccessParams) (sqlc.ServiceMerchant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.ServiceMerchant), args.Error(1)
}

func (m *MockQuerier) RevokeServiceAccess(ctx context.Context, arg sqlc.RevokeServiceAccessParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetServiceMerchantAccess(ctx context.Context, arg sqlc.GetServiceMerchantAccessParams) (sqlc.ServiceMerchant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.ServiceMerchant), args.Error(1)
}

func (m *MockQuerier) UpdateServiceScopes(ctx context.Context, arg sqlc.UpdateServiceScopesParams) (sqlc.ServiceMerchant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.ServiceMerchant), args.Error(1)
}

func (m *MockQuerier) ListServiceMerchants(ctx context.Context, serviceID uuid.UUID) ([]sqlc.ListServiceMerchantsRow, error) {
	args := m.Called(ctx, serviceID)
	return args.Get(0).([]sqlc.ListServiceMerchantsRow), args.Error(1)
}

func (m *MockQuerier) ListMerchantServices(ctx context.Context, merchantID uuid.UUID) ([]sqlc.ListMerchantServicesRow, error) {
	args := m.Called(ctx, merchantID)
	return args.Get(0).([]sqlc.ListMerchantServicesRow), args.Error(1)
}

func (m *MockQuerier) CheckServiceHasScope(ctx context.Context, arg sqlc.CheckServiceHasScopeParams) (bool, error) {
	args := m.Called(ctx, arg)
	return args.Bool(0), args.Error(1)
}

func (m *MockQuerier) CreateAuditLog(ctx context.Context, arg sqlc.CreateAuditLogParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetAuditLogsByActor(ctx context.Context, arg sqlc.GetAuditLogsByActorParams) ([]sqlc.AuditLog, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.AuditLog), args.Error(1)
}

func (m *MockQuerier) GetAuditLogsByEntity(ctx context.Context, arg sqlc.GetAuditLogsByEntityParams) ([]sqlc.AuditLog, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.AuditLog), args.Error(1)
}

func (m *MockQuerier) ListAuditLogs(ctx context.Context, arg sqlc.ListAuditLogsParams) ([]sqlc.AuditLog, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.AuditLog), args.Error(1)
}

func (m *MockQuerier) CountAuditLogs(ctx context.Context, arg sqlc.CountAuditLogsParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

// Stub methods - return zero values for less frequently used methods

func (m *MockQuerier) ActivateAdmin(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) ActivateMerchant(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) AddEvidenceFile(ctx context.Context, arg sqlc.AddEvidenceFileParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) CountChargebacks(ctx context.Context, arg sqlc.CountChargebacksParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountMerchants(ctx context.Context, arg sqlc.CountMerchantsParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountSubscriptions(ctx context.Context, arg sqlc.CountSubscriptionsParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountTransactions(ctx context.Context, arg sqlc.CountTransactionsParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CreateAdmin(ctx context.Context, arg sqlc.CreateAdminParams) (sqlc.Admin, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Admin), args.Error(1)
}

func (m *MockQuerier) CreateAdminSession(ctx context.Context, arg sqlc.CreateAdminSessionParams) (sqlc.AdminSession, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.AdminSession), args.Error(1)
}

func (m *MockQuerier) CreateChargeback(ctx context.Context, arg sqlc.CreateChargebackParams) (sqlc.Chargeback, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQuerier) CreateMerchant(ctx context.Context, arg sqlc.CreateMerchantParams) (sqlc.Merchant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) CreateWebhookDelivery(ctx context.Context, arg sqlc.CreateWebhookDeliveryParams) (sqlc.WebhookDelivery, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.WebhookDelivery), args.Error(1)
}

func (m *MockQuerier) CreateWebhookSubscription(ctx context.Context, arg sqlc.CreateWebhookSubscriptionParams) (sqlc.WebhookSubscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.WebhookSubscription), args.Error(1)
}

func (m *MockQuerier) DeactivateAdmin(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) DeactivateMerchant(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) DeleteAdminSession(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuerier) DeleteExpiredAdminSessions(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQuerier) DeleteWebhookSubscription(ctx context.Context, arg sqlc.DeleteWebhookSubscriptionParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetAdminByEmail(ctx context.Context, email string) (sqlc.Admin, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(sqlc.Admin), args.Error(1)
}

func (m *MockQuerier) GetAdminByID(ctx context.Context, id uuid.UUID) (sqlc.Admin, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Admin), args.Error(1)
}

func (m *MockQuerier) GetAdminSession(ctx context.Context, id uuid.UUID) (sqlc.AdminSession, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.AdminSession), args.Error(1)
}

func (m *MockQuerier) GetChargebackByCaseNumber(ctx context.Context, arg sqlc.GetChargebackByCaseNumberParams) (sqlc.Chargeback, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQuerier) GetChargebackByTransactionID(ctx context.Context, transactionID uuid.UUID) (sqlc.Chargeback, error) {
	args := m.Called(ctx, transactionID)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQuerier) GetChargebackByID(ctx context.Context, id uuid.UUID) (sqlc.Chargeback, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQuerier) GetTransactionTree(ctx context.Context, parentTransactionID uuid.UUID) ([]sqlc.GetTransactionTreeRow, error) {
	args := m.Called(ctx, parentTransactionID)
	return args.Get(0).([]sqlc.GetTransactionTreeRow), args.Error(1)
}

func (m *MockQuerier) GetWebhookDeliveryHistory(ctx context.Context, arg sqlc.GetWebhookDeliveryHistoryParams) ([]sqlc.WebhookDelivery, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.WebhookDelivery), args.Error(1)
}

func (m *MockQuerier) GetWebhookSubscription(ctx context.Context, id uuid.UUID) (sqlc.WebhookSubscription, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.WebhookSubscription), args.Error(1)
}

func (m *MockQuerier) ListActiveMerchants(ctx context.Context) ([]sqlc.Merchant, error) {
	args := m.Called(ctx)
	return args.Get(0).([]sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) ListActiveWebhooksByEvent(ctx context.Context, arg sqlc.ListActiveWebhooksByEventParams) ([]sqlc.WebhookSubscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.WebhookSubscription), args.Error(1)
}

func (m *MockQuerier) ListAdmins(ctx context.Context, arg sqlc.ListAdminsParams) ([]sqlc.Admin, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Admin), args.Error(1)
}

func (m *MockQuerier) ListChargebacks(ctx context.Context, arg sqlc.ListChargebacksParams) ([]sqlc.Chargeback, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Chargeback), args.Error(1)
}

func (m *MockQuerier) ListMerchants(ctx context.Context, arg sqlc.ListMerchantsParams) ([]sqlc.Merchant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) ListPendingWebhookDeliveries(ctx context.Context, limitVal int32) ([]sqlc.WebhookDelivery, error) {
	args := m.Called(ctx, limitVal)
	return args.Get(0).([]sqlc.WebhookDelivery), args.Error(1)
}

func (m *MockQuerier) ListTransactions(ctx context.Context, arg sqlc.ListTransactionsParams) ([]sqlc.Transaction, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.Transaction), args.Error(1)
}

func (m *MockQuerier) ListWebhookSubscriptions(ctx context.Context, arg sqlc.ListWebhookSubscriptionsParams) ([]sqlc.WebhookSubscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]sqlc.WebhookSubscription), args.Error(1)
}

func (m *MockQuerier) MarkChargebackResolved(ctx context.Context, arg sqlc.MarkChargebackResolvedParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) MerchantExists(ctx context.Context, id uuid.UUID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockQuerier) MerchantExistsBySlug(ctx context.Context, slug string) (bool, error) {
	args := m.Called(ctx, slug)
	return args.Bool(0), args.Error(1)
}

func (m *MockQuerier) UpdateAdminPassword(ctx context.Context, arg sqlc.UpdateAdminPasswordParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateAdminRole(ctx context.Context, arg sqlc.UpdateAdminRoleParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateChargeback(ctx context.Context, arg sqlc.UpdateChargebackParams) (sqlc.Chargeback, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQuerier) UpdateChargebackNotes(ctx context.Context, arg sqlc.UpdateChargebackNotesParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateChargebackResponse(ctx context.Context, arg sqlc.UpdateChargebackResponseParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateChargebackStatus(ctx context.Context, arg sqlc.UpdateChargebackStatusParams) (sqlc.Chargeback, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Chargeback), args.Error(1)
}

func (m *MockQuerier) UpdateMerchant(ctx context.Context, arg sqlc.UpdateMerchantParams) (sqlc.Merchant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.Merchant), args.Error(1)
}

func (m *MockQuerier) UpdateMerchantMACPath(ctx context.Context, arg sqlc.UpdateMerchantMACPathParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) UpdateWebhookDeliveryStatus(ctx context.Context, arg sqlc.UpdateWebhookDeliveryStatusParams) (sqlc.WebhookDelivery, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.WebhookDelivery), args.Error(1)
}

func (m *MockQuerier) UpdateWebhookSubscription(ctx context.Context, arg sqlc.UpdateWebhookSubscriptionParams) (sqlc.WebhookSubscription, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.WebhookSubscription), args.Error(1)
}

// ACH Verification Management Methods

func (m *MockQuerier) DeactivatePaymentMethodWithReason(ctx context.Context, arg sqlc.DeactivatePaymentMethodWithReasonParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetPendingACHVerifications(ctx context.Context, arg sqlc.GetPendingACHVerificationsParams) ([]sqlc.CustomerPaymentMethod, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]sqlc.CustomerPaymentMethod), args.Error(1)
}

func (m *MockQuerier) UpdateVerificationStatus(ctx context.Context, arg sqlc.UpdateVerificationStatusParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) IncrementReturnCount(ctx context.Context, arg sqlc.IncrementReturnCountParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetPaymentMethodByPreNoteTransaction(ctx context.Context, prenoteTransactionID pgtype.UUID) (sqlc.CustomerPaymentMethod, error) {
	args := m.Called(ctx, prenoteTransactionID)
	return args.Get(0).(sqlc.CustomerPaymentMethod), args.Error(1)
}

func (m *MockQuerier) MarkVerificationFailed(ctx context.Context, arg sqlc.MarkVerificationFailedParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

// JWT Blacklist Methods

func (m *MockQuerier) IsJWTBlacklisted(ctx context.Context, jti string) (bool, error) {
	args := m.Called(ctx, jti)
	return args.Bool(0), args.Error(1)
}

func (m *MockQuerier) BlacklistJWT(ctx context.Context, arg sqlc.BlacklistJWTParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) CleanupExpiredBlacklist(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// EPX IP Whitelist Methods

func (m *MockQuerier) ListActiveIPWhitelist(ctx context.Context) ([]netip.Addr, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]netip.Addr), args.Error(1)
}

func (m *MockQuerier) AddIPToWhitelist(ctx context.Context, arg sqlc.AddIPToWhitelistParams) (sqlc.EpxIpWhitelist, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.EpxIpWhitelist), args.Error(1)
}

func (m *MockQuerier) RemoveIPFromWhitelist(ctx context.Context, ipAddress netip.Addr) error {
	args := m.Called(ctx, ipAddress)
	return args.Error(0)
}

func (m *MockQuerier) GetIPWhitelistEntry(ctx context.Context, ipAddress netip.Addr) (sqlc.EpxIpWhitelist, error) {
	args := m.Called(ctx, ipAddress)
	return args.Get(0).(sqlc.EpxIpWhitelist), args.Error(1)
}

// Service Authentication Methods

func (m *MockQuerier) ListActiveServicePublicKeys(ctx context.Context) ([]sqlc.ListActiveServicePublicKeysRow, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]sqlc.ListActiveServicePublicKeysRow), args.Error(1)
}

func (m *MockQuerier) GetServiceRateLimit(ctx context.Context, serviceID string) (pgtype.Int4, error) {
	args := m.Called(ctx, serviceID)
	return args.Get(0).(pgtype.Int4), args.Error(1)
}

func (m *MockQuerier) CheckServiceMerchantAccessByID(ctx context.Context, arg sqlc.CheckServiceMerchantAccessByIDParams) (bool, error) {
	args := m.Called(ctx, arg)
	return args.Bool(0), args.Error(1)
}

func (m *MockQuerier) CheckServiceMerchantAccessBySlug(ctx context.Context, arg sqlc.CheckServiceMerchantAccessBySlugParams) (bool, error) {
	args := m.Called(ctx, arg)
	return args.Bool(0), args.Error(1)
}

// ACH Statistics Methods

func (m *MockQuerier) CountTotalACH(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountPendingACH(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountVerifiedACH(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountFailedACH(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) CountEligibleACH(ctx context.Context, cutoffDate time.Time) (int64, error) {
	args := m.Called(ctx, cutoffDate)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) FindEligibleACHForVerification(ctx context.Context, arg sqlc.FindEligibleACHForVerificationParams) ([]sqlc.FindEligibleACHForVerificationRow, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]sqlc.FindEligibleACHForVerificationRow), args.Error(1)
}

func (m *MockQuerier) VerifyACHPaymentMethod(ctx context.Context, id uuid.UUID) (pgconn.CommandTag, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

// Rate Limit Methods

func (m *MockQuerier) ConsumeRateLimitToken(ctx context.Context, arg sqlc.ConsumeRateLimitTokenParams) (int32, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int32), args.Error(1)
}

func (m *MockQuerier) RefillRateLimitBucket(ctx context.Context, arg sqlc.RefillRateLimitBucketParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockQuerier) GetRateLimitBucket(ctx context.Context, bucketKey string) (sqlc.RateLimitBucket, error) {
	args := m.Called(ctx, bucketKey)
	return args.Get(0).(sqlc.RateLimitBucket), args.Error(1)
}

func (m *MockQuerier) CleanupOldRateLimitBuckets(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockQuerier) DeleteOldAuditLogs(ctx context.Context, cutoffDate pgtype.Timestamp) (pgconn.CommandTag, error) {
	args := m.Called(ctx, cutoffDate)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

// Ensure MockQuerier implements sqlc.Querier
var _ sqlc.Querier = (*MockQuerier)(nil)
