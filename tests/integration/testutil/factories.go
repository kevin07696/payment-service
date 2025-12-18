package testutil

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kevin07696/payment-service/internal/db/sqlc"
	"github.com/stretchr/testify/require"
)

// TestFactory provides factory methods for creating test data using SQLC
// Thread-safe counters ensure unique identifiers across concurrent tests
type TestFactory struct {
	queries *sqlc.Queries
	cfg     *Config
	counter int64
}

// NewFactory creates a new test factory with SQLC queries
func NewFactory(t *testing.T) *TestFactory {
	t.Helper()
	cfg, err := LoadConfig()
	require.NoError(t, err, "Failed to load config for factory")

	pool := GetPgxPool(t)
	return &TestFactory{
		queries: sqlc.New(pool),
		cfg:     cfg,
		counter: 0,
	}
}

// nextID generates a unique sequential ID for test data
func (f *TestFactory) nextID() int64 {
	return atomic.AddInt64(&f.counter, 1)
}

// =============================================================================
// Merchant Factory - Creates isolated test merchants
// =============================================================================

// CreatedMerchant represents a merchant created by the factory
type CreatedMerchant struct {
	ID            uuid.UUID
	Slug          string
	Name          string
	CustNbr       string
	MerchNbr      string
	DbaNbr        string
	TerminalNbr   string
	MacSecretPath string
	Environment   string
	IsActive      bool
}

// MerchantBuilder builds test merchants with fluent API
type MerchantBuilder struct {
	factory   *TestFactory
	t         *testing.T
	name      string
	active    bool
	macSecret string // Optional: if set, stores in secret manager dynamically
}

// NewMerchant starts building a new merchant
func (f *TestFactory) NewMerchant(t *testing.T) *MerchantBuilder {
	return &MerchantBuilder{
		factory: f,
		t:       t,
		name:    fmt.Sprintf("Test Merchant %d", f.nextID()),
		active:  true,
	}
}

// WithName sets a custom merchant name
func (b *MerchantBuilder) WithName(name string) *MerchantBuilder {
	b.name = name
	return b
}

// WithMacSecret sets a custom MAC secret for this merchant.
// The secret will be stored in the mock secret manager at a unique path.
// This enables true test isolation where each merchant has its own MAC secret.
func (b *MerchantBuilder) WithMacSecret(secret string) *MerchantBuilder {
	b.macSecret = secret
	return b
}

// Inactive creates an inactive merchant
func (b *MerchantBuilder) Inactive() *MerchantBuilder {
	b.active = false
	return b
}

// Create inserts the merchant using SQLC and returns the created merchant
func (b *MerchantBuilder) Create() *CreatedMerchant {
	b.t.Helper()

	merchantID := uuid.New()
	slug := fmt.Sprintf("test-merchant-%d", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
	defer cancel()

	// Determine MAC secret path
	// If a custom MAC secret is provided, store it dynamically in the secret manager
	// Otherwise, use the default staging path (for backwards compatibility)
	var macSecretPath string
	if b.macSecret != "" {
		// Store MAC secret in mock secret manager for test isolation
		macSecretPath = StoreTestMacSecret(b.t, merchantID.String(), b.macSecret)
	} else {
		// Fall back to default staging path for tests that don't need custom MAC
		macSecretPath = "epx/staging/mac_secret"
	}

	// Use SQLC CreateMerchant query
	dbMerchant, err := b.factory.queries.CreateMerchant(ctx, sqlc.CreateMerchantParams{
		ID:            merchantID,
		Slug:          slug,
		Name:          b.name,
		CustNbr:       b.factory.cfg.EPXCustNbr,
		MerchNbr:      b.factory.cfg.EPXMerchNbr,
		DbaNbr:        b.factory.cfg.EPXDBANbr,
		TerminalNbr:   b.factory.cfg.EPXTerminalNbr,
		MacSecretPath: macSecretPath,
		Environment:   "staging",
		IsActive:      b.active,
	})
	require.NoError(b.t, err, "Failed to create test merchant via SQLC")

	merchant := &CreatedMerchant{
		ID:            dbMerchant.ID,
		Slug:          dbMerchant.Slug,
		Name:          dbMerchant.Name,
		CustNbr:       dbMerchant.CustNbr,
		MerchNbr:      dbMerchant.MerchNbr,
		DbaNbr:        dbMerchant.DbaNbr,
		TerminalNbr:   dbMerchant.TerminalNbr,
		MacSecretPath: dbMerchant.MacSecretPath,
		Environment:   dbMerchant.Environment,
		IsActive:      dbMerchant.IsActive,
	}

	// Register cleanup using SQLC
	b.t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = b.factory.queries.HardDeleteMerchant(cleanupCtx, merchant.ID)
	})

	b.t.Logf("Created test merchant: %s (%s)", merchant.Slug, merchant.ID)
	return merchant
}

// =============================================================================
// Service Factory - Creates isolated test services with RSA keypairs
// =============================================================================

// CreatedService represents a service created by the factory
type CreatedService struct {
	ID            uuid.UUID
	ServiceID     string
	ServiceName   string
	PublicKeyPEM  string
	PrivateKeyPEM string
	Fingerprint   string
	Environment   string
}

// ServiceBuilder builds test services with fluent API
type ServiceBuilder struct {
	factory           *TestFactory
	t                 *testing.T
	environment       string
	requestsPerMinute int32 // Rate limit (requests per minute bucket)
}

// NewService starts building a new service
func (f *TestFactory) NewService(t *testing.T) *ServiceBuilder {
	return &ServiceBuilder{
		factory:           f,
		t:                 t,
		environment:       "staging",
		requestsPerMinute: 1000, // Default high limit
	}
}

// WithEnvironment sets the service environment
func (b *ServiceBuilder) WithEnvironment(env string) *ServiceBuilder {
	b.environment = env
	return b
}

// WithRateLimit sets the rate limit (requests per minute)
func (b *ServiceBuilder) WithRateLimit(requestsPerMinute int32) *ServiceBuilder {
	b.requestsPerMinute = requestsPerMinute
	return b
}

// Create inserts the service using SQLC and returns it with keypair for JWT signing
func (b *ServiceBuilder) Create() *CreatedService {
	b.t.Helper()

	// Generate RSA keypair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(b.t, err, "Failed to generate RSA key")

	// Encode private key
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(b.t, err, "Failed to marshal public key")
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Generate fingerprint
	h := sha256.New()
	h.Write(publicKeyPEM)
	fingerprint := fmt.Sprintf("SHA256:%x", h.Sum(nil))[:50]

	serviceID := uuid.New()
	serviceIDStr := fmt.Sprintf("test-service-%d", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
	defer cancel()

	// Use SQLC CreateService query with pgtype for nullable fields
	dbService, err := b.factory.queries.CreateService(ctx, sqlc.CreateServiceParams{
		ID:                   serviceID,
		ServiceID:            serviceIDStr,
		ServiceName:          fmt.Sprintf("Test Service %d", b.factory.nextID()),
		PublicKey:            string(publicKeyPEM),
		PublicKeyFingerprint: fingerprint,
		Environment:          b.environment,
		RequestsPerSecond:    pgtype.Int4{Int32: b.requestsPerMinute, Valid: true},
		BurstLimit:           pgtype.Int4{Int32: b.requestsPerMinute * 2, Valid: true},
		IsActive:             pgtype.Bool{Bool: true, Valid: true},
	})
	require.NoError(b.t, err, "Failed to create test service via SQLC")

	service := &CreatedService{
		ID:            dbService.ID,
		ServiceID:     dbService.ServiceID,
		ServiceName:   dbService.ServiceName,
		PublicKeyPEM:  dbService.PublicKey,
		PrivateKeyPEM: string(privateKeyPEM),
		Fingerprint:   dbService.PublicKeyFingerprint,
		Environment:   dbService.Environment,
	}

	// Register cleanup using SQLC
	b.t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = b.factory.queries.HardDeleteService(cleanupCtx, service.ID)
	})

	b.t.Logf("Created test service: %s (%s)", service.ServiceID, service.ID)
	return service
}

// =============================================================================
// Service-Merchant Access Factory
// =============================================================================

// GrantAccess grants a service access to a merchant with all payment scopes using SQLC
func (f *TestFactory) GrantAccess(t *testing.T, serviceID, merchantID uuid.UUID) {
	t.Helper()

	scopes := []string{
		"payments:create", "payments:read", "payments:void", "payments:refund",
		"payment_methods:read", "payment_methods:create",
		"subscriptions:manage", "subscriptions:read",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
	defer cancel()

	// Use SQLC GrantServiceAccess query
	_, err := f.queries.GrantServiceAccess(ctx, sqlc.GrantServiceAccessParams{
		ServiceID:  serviceID,
		MerchantID: merchantID,
		Scopes:     scopes,
		ExpiresAt:  pgtype.Timestamptz{Valid: false}, // No expiration
	})
	require.NoError(t, err, "Failed to grant service access via SQLC")

	// Register cleanup using SQLC
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = f.queries.RevokeServiceAccess(cleanupCtx, sqlc.RevokeServiceAccessParams{
			ServiceID:  serviceID,
			MerchantID: merchantID,
		})
	})

	t.Logf("Granted service %s access to merchant %s", serviceID, merchantID)
}

// =============================================================================
// Convenience Function: Create Merchant + Service + Access in one call
// =============================================================================

// TestContext holds all the created entities for a test
type TestContext struct {
	Merchant *CreatedMerchant
	Service  *CreatedService
}

// CreateTestContext creates a merchant, service, and grants access - all isolated per test
func (f *TestFactory) CreateTestContext(t *testing.T) *TestContext {
	t.Helper()

	merchant := f.NewMerchant(t).Create()
	service := f.NewService(t).Create()
	f.GrantAccess(t, service.ID, merchant.ID)

	return &TestContext{
		Merchant: merchant,
		Service:  service,
	}
}

// CreateTestContextWithRateLimit creates merchant + service with custom rate limit
func (f *TestFactory) CreateTestContextWithRateLimit(t *testing.T, requestsPerMinute int32) *TestContext {
	t.Helper()

	merchant := f.NewMerchant(t).Create()
	service := f.NewService(t).WithRateLimit(requestsPerMinute).Create()
	f.GrantAccess(t, service.ID, merchant.ID)

	return &TestContext{
		Merchant: merchant,
		Service:  service,
	}
}

// =============================================================================
// Transaction Factory
// =============================================================================

// TransactionBuilder builds test transactions with fluent API
type TransactionBuilder struct {
	factory       *TestFactory
	t             *testing.T
	merchantID    string // REQUIRED: must be set via WithMerchant() or ForMerchant()
	customerID    string
	amountCents   int64
	currency      string
	txType        string
	paymentMethod string
	authGUID      string
	authResp      *string // NULL = pending/failed, "00" = approved, other = declined
	authCode      string
	orderID       *string // Groups related transactions (replaced group_id)
	parentID      *string // parent_transaction_id for CAPTURE/REFUND/VOID
	processedAt   bool    // true = NOW(), false = NULL (for status generation)
}

// NewTransaction starts building a new transaction
// NOTE: You MUST call WithMerchant() before Create() - no default merchant ID
// Default creates an 'approved' transaction (auth_resp='00', processed_at=NOW())
func (f *TestFactory) NewTransaction(t *testing.T) *TransactionBuilder {
	authResp := "00" // Default to approved
	return &TransactionBuilder{
		factory:       f,
		t:             t,
		merchantID:    "", // Must be explicitly set
		customerID:    uuid.New().String(),
		amountCents:   DefaultAmountCents,
		currency:      DefaultCurrency,
		txType:        "SALE",
		paymentMethod: "credit_card",
		authGUID:      fmt.Sprintf("BRIC-%s", uuid.New().String()),
		authResp:      &authResp,
		authCode:      fmt.Sprintf("AUTH%d", f.nextID()),
		processedAt:   true, // Default to processed (for approved status)
	}
}

// ForMerchant creates a transaction builder pre-configured for a merchant
// This is the preferred way to create transactions with proper isolation
func (f *TestFactory) ForMerchant(merchantID uuid.UUID) *MerchantScope {
	return &MerchantScope{
		factory:    f,
		merchantID: merchantID.String(),
	}
}

// MerchantScope provides factory methods scoped to a specific merchant
type MerchantScope struct {
	factory    *TestFactory
	merchantID string
}

// NewTransaction creates a transaction builder for this merchant
func (ms *MerchantScope) NewTransaction(t *testing.T) *TransactionBuilder {
	builder := ms.factory.NewTransaction(t)
	builder.merchantID = ms.merchantID
	return builder
}

// NewPaymentMethod creates a payment method builder for this merchant
func (ms *MerchantScope) NewPaymentMethod(t *testing.T) *PaymentMethodBuilder {
	builder := ms.factory.NewPaymentMethod(t)
	builder.merchantID = ms.merchantID
	return builder
}

// NewSubscription creates a subscription builder for this merchant
func (ms *MerchantScope) NewSubscription(t *testing.T) *SubscriptionBuilder {
	builder := ms.factory.NewSubscription(t)
	builder.merchantID = ms.merchantID
	return builder
}

// NewChargeback creates a chargeback builder for this merchant
func (ms *MerchantScope) NewChargeback(t *testing.T) *ChargebackBuilder {
	builder := ms.factory.NewChargeback(t)
	builder.merchantSlug = ms.merchantID // Will need to look up slug
	return builder
}

// WithMerchant sets the merchant ID
func (b *TransactionBuilder) WithMerchant(merchantID string) *TransactionBuilder {
	b.merchantID = merchantID
	return b
}

// WithCustomer sets the customer ID
func (b *TransactionBuilder) WithCustomer(customerID string) *TransactionBuilder {
	b.customerID = customerID
	return b
}

// WithAmount sets the amount in cents
func (b *TransactionBuilder) WithAmount(cents int64) *TransactionBuilder {
	b.amountCents = cents
	return b
}

// WithType sets the transaction type (SALE, AUTH, CAPTURE, REFUND, VOID)
func (b *TransactionBuilder) WithType(txType string) *TransactionBuilder {
	b.txType = txType
	return b
}

// Pending creates a pending transaction (auth_resp=NULL, processed_at=NULL)
// Status 'pending' is GENERATED when both auth_resp and processed_at are NULL
func (b *TransactionBuilder) Pending() *TransactionBuilder {
	b.authResp = nil
	b.processedAt = false
	return b
}

// Failed creates a failed transaction (auth_resp=NULL, processed_at=NOW())
// Status 'failed' is GENERATED when auth_resp is NULL but processed_at is set
func (b *TransactionBuilder) Failed() *TransactionBuilder {
	b.authResp = nil
	b.processedAt = true
	return b
}

// WithParent sets the parent transaction ID (for CAPTURE, REFUND, VOID)
// Required for CAPTURE, REFUND, VOID types per schema constraint
func (b *TransactionBuilder) WithParent(parentID string) *TransactionBuilder {
	b.parentID = &parentID
	return b
}

// WithOrder sets the order ID for grouping related transactions
// Replaces the deprecated group_id column
func (b *TransactionBuilder) WithOrder(orderID string) *TransactionBuilder {
	b.orderID = &orderID
	return b
}

// Declined creates a declined transaction (auth_resp='05', processed_at=NOW())
// Status 'declined' is GENERATED when auth_resp is NOT '00' and processed_at is set
func (b *TransactionBuilder) Declined() *TransactionBuilder {
	authResp := "05"
	b.authResp = &authResp
	b.processedAt = true
	return b
}

// Create inserts the transaction and returns its ID
// NOTE: Transactions use raw SQL as SQLC transaction queries are more complex
// Status is GENERATED from auth_resp and processed_at:
//   - pending: auth_resp=NULL, processed_at=NULL
//   - failed: auth_resp=NULL, processed_at=NOW()
//   - approved: auth_resp='00', processed_at=NOW()
//   - declined: auth_resp!='00', processed_at=NOW()
func (b *TransactionBuilder) Create() string {
	b.t.Helper()

	if b.merchantID == "" {
		b.t.Fatal("Transaction requires a merchant ID - use WithMerchant() or ForMerchant()")
	}

	// Validate parent_transaction_id constraint
	// SALE, AUTH, STORAGE, DEBIT must have NULL parent
	// CAPTURE, REFUND, VOID must have parent
	standaloneTypes := map[string]bool{"SALE": true, "AUTH": true, "STORAGE": true, "DEBIT": true}
	childTypes := map[string]bool{"CAPTURE": true, "REFUND": true, "VOID": true}

	if standaloneTypes[b.txType] && b.parentID != nil {
		b.t.Fatalf("Transaction type %s must NOT have a parent_transaction_id", b.txType)
	}
	if childTypes[b.txType] && b.parentID == nil {
		b.t.Fatalf("Transaction type %s REQUIRES a parent_transaction_id - use WithParent()", b.txType)
	}

	id := uuid.New().String()
	tranNbr := fmt.Sprintf("TXN-%d", b.factory.nextID())

	pool := GetPgxPool(b.t)
	ctx := context.Background()

	// Build query dynamically based on whether processed_at should be NULL or NOW()
	var err error
	if b.processedAt {
		_, err = pool.Exec(ctx, `
			INSERT INTO transactions (
				id, merchant_id, customer_id,
				amount_cents, currency, type, payment_method_type,
				tran_nbr, auth_guid, auth_resp, auth_code,
				order_id, parent_transaction_id,
				processed_at, created_at, updated_at
			) VALUES (
				$1::uuid, $2::uuid, $3::uuid,
				$4, $5, $6, $7,
				$8, $9, $10, $11,
				$12, $13,
				NOW(), NOW(), NOW()
			)
		`, id, b.merchantID, b.customerID,
			b.amountCents, b.currency, b.txType, b.paymentMethod,
			tranNbr, b.authGUID, b.authResp, b.authCode,
			b.orderID, b.parentID)
	} else {
		_, err = pool.Exec(ctx, `
			INSERT INTO transactions (
				id, merchant_id, customer_id,
				amount_cents, currency, type, payment_method_type,
				tran_nbr, auth_guid, auth_resp, auth_code,
				order_id, parent_transaction_id,
				created_at, updated_at
			) VALUES (
				$1::uuid, $2::uuid, $3::uuid,
				$4, $5, $6, $7,
				$8, $9, $10, $11,
				$12, $13,
				NOW(), NOW()
			)
		`, id, b.merchantID, b.customerID,
			b.amountCents, b.currency, b.txType, b.paymentMethod,
			tranNbr, b.authGUID, b.authResp, b.authCode,
			b.orderID, b.parentID)
	}

	require.NoError(b.t, err, "Failed to create test transaction")

	// Determine expected status for logging
	var expectedStatus string
	if b.authResp == nil && !b.processedAt {
		expectedStatus = "pending"
	} else if b.authResp == nil && b.processedAt {
		expectedStatus = "failed"
	} else if *b.authResp == "00" {
		expectedStatus = "approved"
	} else {
		expectedStatus = "declined"
	}

	b.t.Logf("Created test transaction: %s (type=%s, expected_status=%s, amount=%d)", id, b.txType, expectedStatus, b.amountCents)

	return id
}

// =============================================================================
// Subscription Factory
// =============================================================================

// SubscriptionBuilder builds test subscriptions with fluent API
type SubscriptionBuilder struct {
	factory         *TestFactory
	t               *testing.T
	merchantID      string
	customerID      string
	paymentMethodID string
	amountCents     int64
	currency        string
	intervalValue   int
	intervalUnit    string
	status          string
	nextBillingDate time.Time
	maxRetries      int
}

// NewSubscription starts building a new subscription
// NOTE: You MUST call WithMerchant() before Create() - no default merchant ID
func (f *TestFactory) NewSubscription(t *testing.T) *SubscriptionBuilder {
	return &SubscriptionBuilder{
		factory:         f,
		t:               t,
		merchantID:      "", // Must be explicitly set
		customerID:      uuid.New().String(),
		amountCents:     DefaultSubscriptionAmount,
		currency:        DefaultCurrency,
		intervalValue:   1,
		intervalUnit:    "month",
		status:          "active",
		nextBillingDate: time.Now().Add(30 * 24 * time.Hour),
		maxRetries:      3,
	}
}

// WithMerchant sets the merchant ID
func (b *SubscriptionBuilder) WithMerchant(merchantID string) *SubscriptionBuilder {
	b.merchantID = merchantID
	return b
}

// WithCustomer sets the customer ID
func (b *SubscriptionBuilder) WithCustomer(customerID string) *SubscriptionBuilder {
	b.customerID = customerID
	return b
}

// WithPaymentMethod sets the payment method ID
func (b *SubscriptionBuilder) WithPaymentMethod(pmID string) *SubscriptionBuilder {
	b.paymentMethodID = pmID
	return b
}

// WithAmount sets the amount in cents
func (b *SubscriptionBuilder) WithAmount(cents int64) *SubscriptionBuilder {
	b.amountCents = cents
	return b
}

// WithInterval sets the billing interval (e.g., 1, "month")
func (b *SubscriptionBuilder) WithInterval(value int, unit string) *SubscriptionBuilder {
	b.intervalValue = value
	b.intervalUnit = unit
	return b
}

// WithStatus sets the subscription status
func (b *SubscriptionBuilder) WithStatus(status string) *SubscriptionBuilder {
	b.status = status
	return b
}

// DueNow sets next billing date to past (due for billing)
func (b *SubscriptionBuilder) DueNow() *SubscriptionBuilder {
	b.nextBillingDate = time.Now().Add(-24 * time.Hour)
	return b
}

// Create inserts the subscription and returns its ID
func (b *SubscriptionBuilder) Create() string {
	b.t.Helper()

	if b.merchantID == "" {
		b.t.Fatal("Subscription requires a merchant ID - use WithMerchant() or ForMerchant()")
	}
	if b.paymentMethodID == "" {
		b.t.Fatal("Subscription requires a payment method ID - use WithPaymentMethod()")
	}

	id := uuid.New().String()
	pool := GetPgxPool(b.t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO subscriptions (
			id, merchant_id, customer_id, payment_method_id,
			amount_cents, currency, interval_value, interval_unit,
			status, next_billing_date, max_retries,
			created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, $3, $4::uuid,
			$5, $6, $7, $8,
			$9, $10, $11,
			NOW(), NOW()
		)
	`, id, b.merchantID, b.customerID, b.paymentMethodID,
		b.amountCents, b.currency, b.intervalValue, b.intervalUnit,
		b.status, b.nextBillingDate, b.maxRetries)

	require.NoError(b.t, err, "Failed to create test subscription")
	b.t.Logf("Created test subscription: %s (amount=%d, status=%s)", id, b.amountCents, b.status)

	return id
}

// =============================================================================
// PaymentMethod Factory
// =============================================================================

// PaymentMethodBuilder builds test payment methods with fluent API
type PaymentMethodBuilder struct {
	factory      *TestFactory
	t            *testing.T
	merchantID   string
	customerID   string
	paymentType  string
	status       string
	lastFour     string
	cardBrand    string
	paymentToken string
	expMonth     int
	expYear      int
}

// NewPaymentMethod starts building a new payment method
// NOTE: You MUST call WithMerchant() before Create() - no default merchant ID
func (f *TestFactory) NewPaymentMethod(t *testing.T) *PaymentMethodBuilder {
	return &PaymentMethodBuilder{
		factory:      f,
		t:            t,
		merchantID:   "", // Must be explicitly set
		customerID:   uuid.New().String(),
		paymentType:  "credit_card",
		status:       "active",
		lastFour:     "4242",
		cardBrand:    "Visa",
		paymentToken: fmt.Sprintf("tok_%s", uuid.New().String()),
		expMonth:     12,
		expYear:      time.Now().Year() + 2,
	}
}

// WithMerchant sets the merchant ID
func (b *PaymentMethodBuilder) WithMerchant(merchantID string) *PaymentMethodBuilder {
	b.merchantID = merchantID
	return b
}

// WithCustomer sets the customer ID
func (b *PaymentMethodBuilder) WithCustomer(customerID string) *PaymentMethodBuilder {
	b.customerID = customerID
	return b
}

// WithType sets the payment type (credit_card, ach)
func (b *PaymentMethodBuilder) WithType(paymentType string) *PaymentMethodBuilder {
	b.paymentType = paymentType
	return b
}

// AsACH creates an ACH payment method
func (b *PaymentMethodBuilder) AsACH() *PaymentMethodBuilder {
	b.paymentType = "ach"
	b.cardBrand = ""
	b.expMonth = 0
	b.expYear = 0
	return b
}

// WithStatus sets the payment method status
func (b *PaymentMethodBuilder) WithStatus(status string) *PaymentMethodBuilder {
	b.status = status
	return b
}

// Expired creates an expired payment method
func (b *PaymentMethodBuilder) Expired() *PaymentMethodBuilder {
	b.status = "expired"
	b.expYear = time.Now().Year() - 1
	return b
}

// Create inserts the payment method and returns its ID
func (b *PaymentMethodBuilder) Create() string {
	b.t.Helper()

	if b.merchantID == "" {
		b.t.Fatal("PaymentMethod requires a merchant ID - use WithMerchant() or ForMerchant()")
	}

	id := uuid.New().String()
	pool := GetPgxPool(b.t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO payment_methods (
			id, merchant_id, customer_id,
			payment_type, status, last_four, card_brand, payment_token,
			card_exp_month, card_exp_year,
			created_at, updated_at
		) VALUES (
			$1::uuid, $2::uuid, $3,
			$4, $5, $6, $7, $8,
			$9, $10,
			NOW(), NOW()
		)
	`, id, b.merchantID, b.customerID,
		b.paymentType, b.status, b.lastFour, b.cardBrand, b.paymentToken,
		b.expMonth, b.expYear)

	require.NoError(b.t, err, "Failed to create test payment method")
	b.t.Logf("Created test payment method: %s (type=%s, status=%s)", id, b.paymentType, b.status)

	return id
}

// =============================================================================
// Chargeback Factory
// =============================================================================

// ChargebackBuilder builds test chargebacks with fluent API
type ChargebackBuilder struct {
	factory           *TestFactory
	t                 *testing.T
	transactionID     string
	merchantSlug      string
	customerID        string
	caseNumber        string
	status            string
	amount            string
	currency          string
	reasonCode        string
	reasonDescription string
}

// NewChargeback starts building a new chargeback
// NOTE: You MUST call WithTransaction() before Create()
func (f *TestFactory) NewChargeback(t *testing.T) *ChargebackBuilder {
	return &ChargebackBuilder{
		factory:           f,
		t:                 t,
		transactionID:     "", // Must be explicitly set
		merchantSlug:      "", // Must be explicitly set
		customerID:        uuid.New().String(),
		caseNumber:        fmt.Sprintf("CB-%d", time.Now().UnixNano()),
		status:            "new",
		amount:            "50.00",
		currency:          "USD",
		reasonCode:        "P22",
		reasonDescription: "Cardholder disputes quality of goods or services",
	}
}

// WithTransaction sets the transaction ID and merchant slug
func (b *ChargebackBuilder) WithTransaction(txnID, merchantSlug string) *ChargebackBuilder {
	b.transactionID = txnID
	b.merchantSlug = merchantSlug
	return b
}

// WithStatus sets the chargeback status (new, pending, responded, won, lost)
func (b *ChargebackBuilder) WithStatus(status string) *ChargebackBuilder {
	b.status = status
	return b
}

// WithAmount sets the chargeback amount
func (b *ChargebackBuilder) WithAmount(amount string) *ChargebackBuilder {
	b.amount = amount
	return b
}

// WithReason sets the reason code and description
func (b *ChargebackBuilder) WithReason(code, description string) *ChargebackBuilder {
	b.reasonCode = code
	b.reasonDescription = description
	return b
}

// Create inserts the chargeback and returns its case number
func (b *ChargebackBuilder) Create() string {
	b.t.Helper()

	if b.transactionID == "" {
		b.t.Fatal("Chargeback requires a transaction ID - use WithTransaction()")
	}
	if b.merchantSlug == "" {
		b.t.Fatal("Chargeback requires a merchant slug - use WithTransaction()")
	}

	pool := GetPgxPool(b.t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DBInsertTimeout)*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, `
		INSERT INTO chargebacks (
			transaction_id, merchant_id, customer_id,
			case_number, dispute_date, chargeback_date,
			chargeback_amount, currency, reason_code, reason_description,
			status, raw_data
		) VALUES (
			$1::uuid, $2, $3,
			$4, NOW() - INTERVAL '5 days', NOW() - INTERVAL '3 days',
			$5, $6, $7, $8,
			$9, '{"source": "factory", "test": true}'::jsonb
		)`,
		b.transactionID, b.merchantSlug, b.customerID,
		b.caseNumber,
		b.amount, b.currency, b.reasonCode, b.reasonDescription,
		b.status,
	)
	require.NoError(b.t, err, "Failed to create test chargeback")

	// Register cleanup
	b.t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM chargebacks WHERE case_number = $1", b.caseNumber)
	})

	b.t.Logf("Created test chargeback: %s (status=%s, amount=%s)", b.caseNumber, b.status, b.amount)
	return b.caseNumber
}

// =============================================================================
// Cleanup Helpers
// =============================================================================

// NOTE: Most cleanup is now handled automatically by t.Cleanup() registered
// in each Create() method. These helpers are for manual cleanup when needed.

// CleanupSubscription removes a specific subscription
func (f *TestFactory) CleanupSubscription(t *testing.T, id string) {
	t.Helper()
	pool := GetPgxPool(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, "DELETE FROM subscriptions WHERE id = $1::uuid", id)
	})
}

// CleanupPaymentMethod removes a specific payment method
func (f *TestFactory) CleanupPaymentMethod(t *testing.T, id string) {
	t.Helper()
	pool := GetPgxPool(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, "DELETE FROM payment_methods WHERE id = $1::uuid", id)
	})
}

// CleanupTransaction removes a specific transaction
func (f *TestFactory) CleanupTransaction(t *testing.T, id string) {
	t.Helper()
	pool := GetPgxPool(t)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, "DELETE FROM transactions WHERE id = $1::uuid", id)
	})
}
