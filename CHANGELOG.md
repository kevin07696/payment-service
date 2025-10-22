# Changelog

All notable changes to the payment-service project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added - Chargeback & Settlement Infrastructure (2025-10-21)

- **Database Migrations**:
  - `002_chargebacks.sql`: Complete chargeback tracking table with comprehensive fields
  - `003_settlements.sql`: Settlement batch and transaction tables for daily reconciliation
  - Full goose migration support with up/down migrations
  - Proper indexes for performance optimization
  - Check constraints for data integrity
  - Auto-updating timestamp triggers

- **Chargeback Domain Models** (internal/domain/models/chargeback.go):
  - `Chargeback`: Complete chargeback entity with all lifecycle fields
  - `ChargebackStatus`: Status enum (pending, responded, won, lost, accepted)
  - `ChargebackOutcome`: Outcome enum (reversed, upheld, partial)
  - `ChargebackCategory`: Category enum (fraud, authorization, processing_error, consumer_dispute)
  - Support for reason codes (10.4, 13.1, etc.), evidence files, response tracking
  - Timeline tracking: chargeback_date, received_date, respond_by_date, response_submitted_at, resolved_at

- **Settlement Domain Models** (internal/domain/models/settlement.go):
  - `SettlementBatch`: Batch-level settlement data with financial summaries
  - `SettlementTransaction`: Individual transaction settlement details
  - `SettlementStatus`: Status enum (pending, reconciled, discrepancy, completed)
  - Support for sales, refunds, chargebacks tracking
  - Interchange fee tracking (rate and amount)
  - Reconciliation with discrepancy detection

- **Repository Interfaces**:
  - `ChargebackRepository` (internal/domain/ports/chargeback_repository.go):
    - CRUD operations for chargebacks
    - Query by transaction, merchant, customer, status
    - List pending responses needing attention
    - Status and outcome updates
  - `SettlementRepository` (internal/domain/ports/settlement_repository.go):
    - Batch and transaction operations
    - Query by date range, merchant, status
    - Gateway transaction ID lookups
    - Reconciliation helper methods with summary calculations

- **Implementation Documentation**:
  - `docs/CHARGEBACK_MANAGEMENT.md`: Comprehensive guide on chargeback necessity, implementation phases, and best practices
  - `docs/IMPLEMENTATION_CHECKLIST.md`: Step-by-step checklist for integrating with North gateway
  - `docs/BUSINESS_REPORTING_API_ANALYSIS.md`: Analysis of Business Reporting API for chargebacks and settlements
  - `docs/DISPUTE_API_INTEGRATION.md`: **Complete integration guide for North's Dispute API** ✅
    - Dedicated API endpoint: GET /merchant/disputes/mid/search
    - Response format with all chargeback fields (caseNumber, reasonCode, status, etc.)
    - Field mapping to our database schema
    - Complete polling service implementation (DisputeAdapter, SyncService, scheduled job)
    - Scheduled sync job architecture
  - `docs/CHARGEBACK_SETTLEMENT_SUMMARY.md`: **Executive summary of implementation** ✅
    - Complete feature overview
    - What's built vs what's pending North response
    - Architecture decisions and rationale
    - Next steps checklist
  - `docs/SETTLEMENTS_VS_REFUNDS.md`: **Clarifies the difference between settlements and refunds** ✅
    - Settlements = When North deposits money to YOUR bank (accounting)
    - Refunds = When you return money to CUSTOMER (customer service)
    - Real-world examples and visual flows
    - Why both are important for different reasons
  - `docs/FEATURE_API_MAPPING.md`: **Complete feature inventory with North API mapping** ✅
    - All 23 features with implementation status
    - North API endpoints used for each feature
    - Authentication methods per API
    - Request/response formats
    - Data flow diagrams
    - 83% feature completion (19/23 implemented)
  - `docs/FEATURE_CHART.md`: **Chart/table format of all features and APIs** ✅
    - Scannable tables for quick reference
    - Feature-to-API mapping tables
    - Authentication summary table
    - Implementation status by category
    - Test coverage table
    - Questions for North support table
  - Includes reason code mapping, evidence requirements, and automated response strategies
  - Settlement report reconciliation procedures
  - **Decision**: Keep current ListTransactions implementation (database query) - faster, more reliable than API calls
  - **Decision**: Use Dispute API for chargeback tracking via hourly polling service ✅

### Technical Details

- **Chargeback Table Schema**:
  - Links to transactions table via foreign key
  - Stores gateway chargeback ID, amount, currency
  - Reason code and category tracking
  - Evidence files stored as JSONB array
  - Raw webhook data preservation for debugging
  - Indexes on transaction_id, merchant_id, status, respond_by_date

- **Settlement Tables Schema**:
  - `settlement_batches`: Summary-level data with totals and counts
  - `settlement_transactions`: Detail-level transaction data with fees
  - Cascade delete: removing batch removes all associated transactions
  - Indexes optimized for date-based queries and lookups
  - Support for discrepancy tracking and reconciliation status

- **Database Integrity**:
  - Check constraints on positive amounts
  - Foreign key relationships with transactions table
  - Nullable fields for optional data (outcome, evidence, etc.)
  - Auto-updating timestamps via existing trigger function

### Next Steps (Awaiting North Gateway Integration Details)

**Phase 1: Contact North Support** ⏳
- [ ] Send email to North support using template in `IMPLEMENTATION_CHECKLIST.md`
- [ ] Request Dispute API authentication details (HMAC/JWT/API Key?)
- [ ] Request complete list of `status` and `disputeType` enumeration values
- [ ] Request reason code mapping by card brand
- [ ] Request settlement report access method (API/SFTP/portal?)
- [ ] Request sample settlement file

**Phase 2: Implement Chargeback Sync** (After North Response)
- [ ] Implement `DisputeAdapter` with proper authentication
- [ ] Implement `TransactionRepository.GetByGatewayTransactionID()`
- [ ] Implement PostgreSQL query for gateway transaction ID lookup
- [ ] Implement `ChargebackRepository` (Create, Update, GetByChargebackID, etc.)
- [ ] Implement `SyncService` with polling logic
- [ ] Create scheduled job (hourly ticker)
- [ ] Set up alerting system (email/Slack/PagerDuty)
- [ ] Test with North sandbox environment
- [ ] Deploy to production with monitoring

**Phase 3: Implement Settlement Reconciliation** (After North Response)
- [ ] Implement settlement report parser (based on North's format)
- [ ] Implement `SettlementRepository` (CreateBatch, CreateTransaction, etc.)
- [ ] Implement import service
- [ ] Implement reconciliation logic (compare expected vs actual)
- [ ] Set up daily reconciliation job
- [ ] Configure discrepancy alerts
- [ ] Test with sample settlement files

---

### Added - One-Time Charging for Stored Payment Methods (2025-10-21)

- **ChargePaymentMethod() Implementation**:
  - Added `ChargePaymentMethod()` method to `RecurringBillingAdapter` (internal/adapters/north/recurring_billing_adapter.go:304-352)
  - Implements North's `/chargepaymentmethod` endpoint for one-time charges to stored payment methods
  - Independent from subscription billing - does not count toward subscription payments
  - Accepts `paymentMethodID` and `amount` parameters
  - Returns full `PaymentResult` with transaction details
  - Proper error handling and response code validation

- **Gateway Interface Extension**:
  - Updated `RecurringBillingGateway` interface (internal/domain/ports/subscription_gateway.go:68-70)
  - Added `ChargePaymentMethod()` method signature
  - Documented as independent from subscription payments

- **Architecture Documentation Updates**:
  - **Updated docs/NORTH_API_GUIDE.md**:
    - Corrected understanding: Recurring Billing API = "Stored Payment Methods API"
    - Added `/chargepaymentmethod` endpoint documentation
    - Updated "Browser Post vs Recurring Billing" decision guide
    - Updated all scenarios to use correct API (Special Cases, FAQ, Summary Table)
    - Fixed misinformation about token storage and API usage
    - Added comprehensive examples for on-demand charging

  - **Updated docs/ARCHITECTURE_DECISION.md**:
    - Marked investigation as RESOLVED (2025-10-21)
    - Documented North's three Recurring Billing API capabilities:
      1. Store payment methods (customer vault)
      2. One-time charging via `/chargepaymentmethod`
      3. Recurring subscriptions via `/subscription`
    - Added resolution summary with implementation details
    - Moved original investigation to collapsible details section

### Changed

- **Recurring Billing API Usage**:
  - Clarified that Recurring Billing API serves dual purpose:
    - Store payment methods securely (customer vault)
    - Enable both one-time AND recurring charges
  - Browser Post API remains for immediate checkout (no storage)
  - Recurring Billing API now recommended for all stored payment method scenarios

### Technical Details

- **API Endpoint**: `POST /chargepaymentmethod`
- **Request Format**:
  ```json
  {
    "PaymentMethodID": 12345,
    "Amount": 99.99
  }
  ```
- **Response Format**:
  ```json
  {
    "Date": "2025-10-21T10:30:00Z",
    "GUID": "txn_abc123",
    "Amount": 99.99,
    "Code": "00",
    "Text": "Approved",
    "Approval": "AUTH123",
    "Successful": true
  }
  ```

### Key Insights

- User correction was critical: "Recurring Billing also works for one time payment I think recurring means a stored payment method"
- North's naming is misleading - "Recurring Billing" implies subscriptions only, but it's actually a stored payment method vault
- This discovery enables pay-as-you-go, variable recurring billing, and on-demand charging without PCI burden
- Eliminates need to store BRIC tokens in our database

---

### Added - Comprehensive Documentation (2025-10-20)

- **Frontend Integration Guide** (docs/FRONTEND_INTEGRATION.md):
  - Complete guide for frontend developers implementing tokenized payments
  - Browser Post JavaScript SDK integration examples
  - PCI-compliant tokenization flow documentation
  - React, vanilla JavaScript, and HTML examples
  - API endpoint documentation with request/response examples
  - Security best practices and error handling
  - Testing with North test cards
  - **Sections**:
    - Architecture flow diagrams
    - Step-by-step tokenization implementation
    - Payment and subscription API examples
    - Complete HTML/React code examples
    - Error handling and validation
    - Security best practices (NEVER send raw card data)

- **Local Testing Setup Guide** (docs/LOCAL_TESTING_SETUP.md):
  - Comprehensive guide for backend developers
  - Docker Compose setup for test database
  - Integration testing procedures
  - gRPC endpoint testing with grpcurl
  - Database management and troubleshooting
  - Performance testing with ghz
  - CI/CD configuration examples
  - **Sections**:
    - Quick start guide
    - Detailed Docker setup
    - Running integration tests
    - Testing gRPC endpoints
    - Health check and metrics monitoring
    - Database queries and management
    - Troubleshooting common issues
    - Development workflow
    - Makefile commands reference

- **Documentation Index** (docs/README.md):
  - Central hub for all documentation
  - Quick links to guides organized by role (frontend, backend, DevOps)
  - Architecture diagrams
  - Testing strategy documentation
  - Deployment guides
  - Common tasks and workflows
  - Environment variables reference

- **3D Secure Implementation Guide** (docs/3DS_IMPLEMENTATION.md):
  - Comprehensive guide for implementing 3DS authentication for credit cards
  - **Status**: Pending North Gateway confirmation of 3DS support
  - Architecture flow diagrams for 3DS 1.0 and 3DS 2.0
  - Frontend integration with challenge flow handling
  - Backend implementation with proto definitions
  - Testing strategies and test cards
  - Error handling and fallback strategies
  - **Key Concept**: 3DS is for CREDIT CARDS only (not ACH)
  - Benefits: 70-90% fraud reduction, liability shift to issuer
  - Required in Europe (PSD2/SCA), optional but beneficial in US

- **ACH Bank Account Verification Guide** (docs/ACH_VERIFICATION.md):
  - Complete guide for implementing ACH bank account verification
  - **Status**: Ready to implement immediately
  - Three verification methods explained:
    1. Micro-deposits (2-3 days, low cost, traditional)
    2. Instant verification with Plaid (< 30 seconds, better UX)
    3. Account validation API (instant format check only)
  - Implementation examples for all three methods
  - Database schema, proto definitions, service layer code
  - Frontend integration examples
  - Security best practices (never store full account numbers)
  - Testing procedures and metrics tracking
  - **Key Concept**: ACH verification prevents returns and reduces fraud
  - Benefits: 80-90% reduction in invalid account returns

- **North API Selection Guide** (docs/NORTH_API_GUIDE.md):
  - Comprehensive guide explaining which North API to use for different payment scenarios
  - **Key Question Answered**: Should we use Recurring Billing API for one-time payments? NO
  - **Correct Architecture**:
    - Browser Post API → One-time tokenized payments (current setup ✅)
    - Recurring Billing API → Subscription management only
    - Custom Pay API → Avoid (PCI risk, uses raw card data)
    - ACH API → Bank transfers
  - Decision matrix for all payment scenarios
  - Special cases explained: variable recurring, pay-as-you-go, free trials
  - Token reuse considerations
  - Migration guide from Custom Pay to Browser Post
  - **Current implementation is correct** - don't mix APIs!

### Changed - Server Configuration for PCI Compliance (2025-10-20)

- **Server Payment Gateway Adapter** (cmd/server/main.go:209):
  - Changed from `CustomPayAdapter` to `BrowserPostAdapter` for PCI-compliant tokenization
  - **Why**: CustomPayAdapter expects raw card data (JSON), BrowserPostAdapter uses BRIC tokens
  - **Security Impact**: Backend now NEVER receives or processes raw card numbers
  - **Frontend Integration Required**:
    - Frontend must use North JavaScript SDK to tokenize cards
    - Frontend posts card data directly to North → receives BRIC token
    - Frontend sends BRIC token to backend API (not card numbers)
  - **PCI DSS Scope**: Dramatically reduced - backend is out of scope for card data handling
  - **Adapter Features**:
    - Accepts BRIC tokens only (not raw card data)
    - Form-encoded requests with HMAC-SHA256 authentication
    - XML response parsing
    - All payment operations: authorize, capture, void, refund, verify
  - **Testing**: 19 comprehensive tests, 89% coverage

### Fixed - Test Issues (2025-10-20)

- **Test Database Configuration**:
  - Changed test database port from 5433 to 5434 to avoid port conflicts
  - Updated `docker-compose.test.yml` to use port 5434
  - Updated `test/integration/testdb/setup.go` default port configuration

- **Payment Service Test Assertions**:
  - Fixed incorrect status expectations in authorization tests
  - Changed `Authorize_Success` test to expect `StatusAuthorized` instead of `StatusCaptured`
  - Fixed error assertion pattern to use `errors.As()` instead of type assertion for proper error unwrapping

- **Subscription Gateway Integration Bug** (internal/services/subscription/subscription_service.go:91-95):
  - **Root Cause**: Gateway subscription ID was set in memory but never persisted to database
  - **Fix 1 - Service Layer**: Added `s.subRepo.Update(ctx, tx, subscription)` call after receiving gateway subscription ID from CreateSubscription gateway call
  - **Fix 2 - SQL Query** (internal/db/queries/subscriptions.sql:22): Added `gateway_subscription_id = COALESCE(sqlc.narg(gateway_subscription_id), gateway_subscription_id)` to UpdateSubscription query
  - **Fix 3 - Repository Layer** (internal/adapters/postgres/subscription_repository.go:134): Added `GatewaySubscriptionID: nullText(subscription.GatewaySubscriptionID)` parameter to UpdateSubscription call
  - **Fix 4 - Unit Test** (subscription_service_test.go:260-261): Added mock expectation for `Update` call in CreateSubscription_Success test
  - **Impact**: Subscription service can now properly persist and retrieve gateway-managed subscription IDs

- **Test Results**: All 19 integration tests now passing (100% success rate):
  - ✅ Payment Service tests (7/7)
  - ✅ Transaction Repository tests (5/5)
  - ✅ Subscription Repository tests (4/4)
  - ✅ Subscription Service tests (6/6)

### Added - PostgreSQL Integration Tests

- **Integration Test Infrastructure** (test/integration/testdb):
  - `SetupTestDB`: Automated test database setup with connection pooling
  - `CleanDatabase`: Truncates all tables for fresh test state
  - `TeardownTestDB`: Proper cleanup and connection closing
  - In-memory migration execution without external dependencies
  - Environment-based configuration (TEST_DB_HOST, TEST_DB_PORT, etc.)
  - Automatic table creation with full schema (transactions, subscriptions, audit_logs)
  - Idempotent test setup for reliable CI/CD integration

- **Repository Integration Tests** (test/integration/repository_test.go):
  - **TransactionRepository tests**:
    - CreateAndGet: Full CRUD lifecycle with UUID and metadata
    - GetByIdempotencyKey: Idempotency key lookups
    - UpdateStatus: Transaction status transitions
    - ListByMerchant: Pagination with merchant filtering
    - ListByCustomer: Customer transaction history
  - **SubscriptionRepository tests**:
    - CreateAndGet: Subscription creation with billing schedules
    - Update: Amount, frequency, and status updates
    - ListByCustomer: Customer subscription queries
    - ListActiveSubscriptionsDueForBilling: Batch billing queries with date filtering
  - All tests use real PostgreSQL database
  - Automatic cleanup between test runs

- **Payment Service Integration Tests** (test/integration/payment_service_test.go):
  - **AuthorizeSale tests**:
    - Authorize_Success: Full authorization flow with database persistence
    - Authorize_IdempotencyCheck: Duplicate request handling
    - Sale_Success: Combined authorize + capture
    - Authorize_GatewayError: Error handling with payment error types
  - **CaptureVoidRefund tests**:
    - Capture_Success: Two-step payment capture with status updates
    - Void_Success: Transaction cancellation
    - Refund_Success: Full refund processing
  - Tests verify both gateway calls and database state
  - Mock gateway integration for controlled testing

- **Subscription Service Integration Tests** (test/integration/subscription_service_test.go):
  - **Lifecycle tests**:
    - CreateSubscription_WithGateway: Gateway-managed subscription creation
    - CreateSubscription_WithoutGateway: App-managed subscriptions
    - UpdateSubscription: Amount and frequency changes
    - CancelSubscription: Cancellation with timestamp tracking
  - **ProcessBilling tests**:
    - ProcessDueBilling_Success: Batch billing with transaction creation
    - Billing schedule updates
    - Failure retry count management
  - Tests verify full business logic with database transactions

- **Test Infrastructure**:
  - `docker-compose.test.yml`: Dedicated PostgreSQL container for testing
  - Makefile targets:
    - `make test-integration`: Run all integration tests
    - `make test-unit`: Run unit tests only (skip integration)
    - `make test-integration-cover`: Integration test coverage report
    - `make test-db-up`: Start test database
    - `make test-db-down`: Stop test database
    - `make test-db-logs`: View test database logs
  - Test database runs on port 5433 (separate from dev database)
  - Comprehensive README with setup instructions and troubleshooting

- **CI/CD Ready**:
  - `testing.Short()` support to skip integration tests in unit test runs
  - Environment variable configuration for different environments
  - Health checks and connection validation
  - Fast test execution with parallel test support

### Added - North Browser Post Adapter (Tokenized Payments)

- **BrowserPostAdapter** (internal/adapters/north/browser_post_adapter.go):
  - Complete implementation of CreditCardGateway using BRIC tokens
  - **PCI Compliance**: Backend operates ONLY with tokenized BRIC tokens, never touches raw card data
  - 5 operations fully implemented:
    - `Authorize`: Authorizes payment using BRIC token (auth-only or sale mode)
    - `Capture`: Captures previously authorized payment by transaction ID
    - `Void`: Voids authorized transaction before settlement
    - `Refund`: Refunds captured transaction with optional reason
    - `VerifyAccount`: Validates BRIC token with $0.00 verification
  - HMAC-SHA256 authentication for all API calls
  - XML-based request/response handling
  - Form-encoded HTTP requests to Browser Post API endpoints
  - Transaction type support: Authorization (A), Sale (S), Verification (V)
  - Comprehensive error handling with payment error types
  - Retry logic based on response codes
  - Logging support for all operations

- **Security Features**:
  - Frontend tokenization using North's JavaScript SDK (not included - client-side)
  - Backend receives only BRIC tokens from frontend
  - No raw card data ever stored or processed by backend
  - Reduces PCI DSS scope dramatically
  - Token-based refunds, voids, and captures

- **Request/Response Types**:
  - `BrowserPostResponse`: XML response parser with field extraction
  - `BrowserPostField`: Individual XML field representation
  - Support for billing info (ZIP code, address) for AVS
  - Transaction ID tracking for captures, voids, refunds
  - Refund reason tracking for audit purposes

- **Comprehensive Test Coverage** (browser_post_adapter_test.go):
  - 19 test cases covering all tokenized payment operations
  - Authorize: success (auth-only), success (sale mode), missing token, declined card
  - Capture: success, missing transaction ID
  - Void: success, missing transaction ID
  - Refund: success with reason, missing transaction ID
  - VerifyAccount: success, missing token, network error handling
  - Error handling: network errors, 5xx gateway errors, 4xx bad requests
  - HMAC signature verification
  - Dependency injection demonstration
  - All tests passing with httptest mock server
  - **Coverage**: North adapters overall coverage increased to 89.0%

- **Integration with Frontend**:
  - Frontend uses North Browser Post JavaScript SDK to tokenize cards
  - SDK posts card data directly to North (HTTPS), returns BRIC token
  - Frontend sends BRIC token to backend API
  - Backend uses BrowserPostAdapter with BRIC token for all operations
  - **Result**: Backend is PCI DSS compliant (reduced scope)

### Added - gRPC Handler Tests (API Layer)

- **Payment Handler Tests** (internal/api/grpc/payment/payment_handler_test.go):
  - 12 comprehensive test cases covering all payment operations
  - **Authorization Tests**:
    - Authorize_Success: Successful payment authorization with proto conversion
    - Authorize_MissingMerchantID: Validation of required merchant ID field
    - Authorize_InvalidAmount: Handling of invalid decimal amounts
    - Authorize_ServiceError: Proper gRPC error code mapping
  - **Transaction Operation Tests**:
    - Capture_Success: Successful capture of authorized payment
    - Capture_MissingTransactionID: Validation of transaction ID requirement
    - Sale_Success: Combined authorize + capture flow
    - Void_Success: Transaction cancellation
    - Refund_Success: Refund processing with amount validation
  - **Query Tests**:
    - GetTransaction_Success: Transaction retrieval with proper field mapping
    - GetTransaction_NotFound: NotFound error handling
    - ListTransactions_NotImplemented: Unimplemented RPC response
  - **Coverage**: 78.9% of statements
  - All tests passing (12/12)

- **Subscription Handler Tests** (internal/api/grpc/subscription/subscription_handler_test.go):
  - 15 comprehensive test cases covering subscription lifecycle
  - **Lifecycle Management Tests**:
    - CreateSubscription_Success: Creation with billing schedule and metadata
    - CreateSubscription_MissingMerchantID: Field validation
    - CreateSubscription_InvalidAmount: Decimal parsing validation
    - CreateSubscription_ServiceError: Error propagation
    - UpdateSubscription_Success: Amount and frequency updates
    - UpdateSubscription_MissingSubscriptionID: Required field validation
    - CancelSubscription_Success: Cancellation with timestamp tracking
    - PauseSubscription_Success: Subscription pause
    - ResumeSubscription_Success: Subscription resume
  - **Query Tests**:
    - GetSubscription_Success: Subscription retrieval with conversion
    - GetSubscription_NotFound: NotFound error handling
    - ListCustomerSubscriptions_Success: Customer subscription list
    - ListCustomerSubscriptions_MissingMerchantID: Field validation
  - **Batch Processing Tests**:
    - ProcessDueBilling_Success: Batch billing with error tracking
    - ProcessDueBilling_DefaultBatchSize: Batch size defaulting
  - **Coverage**: 81.2% of statements
  - All tests passing (15/15)

- **Test Patterns and Techniques**:
  - Mock-based unit testing with testify/mock framework
  - Decimal comparison using `decimal.Equal()` for floating-point safety
  - Time comparison using `time.Equal()` for timezone-safe assertions
  - Proto to domain model conversion testing
  - gRPC status code mapping validation
  - Request validation testing for all required fields
  - Service layer error propagation testing
  - Mock logger integration for observability testing

- **Fixed Issues During Implementation**:
  - Decimal comparison: Changed from string equality to `decimal.Equal()` for proper decimal comparison
  - Mock logger calls: Updated to expect variadic fields parameter with `mock.Anything`
  - Time timezone handling: Used `time.Equal()` matcher for protobuf time conversion
  - Nil pointer handling: Added required BillingInfo/Address structures to prevent panics

### Added - North ACH Adapter (Pay-by-Bank)

- **ACHAdapter** (internal/adapters/north/ach_adapter.go):
  - Complete implementation of ACHGateway interface
  - 3 operations fully implemented:
    - `ProcessPayment`: Processes ACH debit transactions for checking/savings accounts
    - `RefundPayment`: Processes ACH credit (refund) transactions
    - `VerifyBankAccount`: Validates bank account routing and account number format
  - Support for both checking and savings accounts (transaction types CKC2, CKS2, CKC3)
  - SEC code support: PPD, WEB, CCD, TEL, ARC
  - XML-based request/response handling
  - Form-encoded HTTP requests to North Pay-by-Bank API
  - Proper error handling with payment error types
  - Retry logic based on ACH response codes
  - Comprehensive logging for all operations

- **Request/Response Types**:
  - `ACHPaymentRequest`: Structured request with bank account details, billing info, SEC code
  - `BankAccountVerificationRequest`: Account validation request
  - `ACHResponse`: XML response parser with field extraction
  - Support for receiver name (required for CCD corporate transactions)
  - Masked account number handling for security/audit
  - Transaction type mapping based on account type and operation

- **Comprehensive Test Coverage** (ach_adapter_test.go):
  - 16 test cases covering all ACH operations
  - ProcessPayment: checking account success, savings account success
  - Validation: missing bank info, invalid routing number, invalid account number
  - SEC codes: WEB, CCD with receiver name
  - RefundPayment: successful ACH credit
  - VerifyBankAccount: valid account, invalid routing, missing account
  - Error handling: network errors, invalid EPI-Id format
  - XML response parsing with complex fields
  - Dependency injection demonstration
  - All tests passing with httptest mock server
  - **Individual method coverage**: ProcessPayment (100%), RefundPayment (88%), VerifyBankAccount (100%)

- **ACH Response Code Coverage**:
  - Code 00: Accepted/Approved
  - Code 03: Unable to locate account
  - Code 14: Invalid account number
  - Code 52: Insufficient funds
  - Code 53: Account not found
  - Code 78: Invalid routing number
  - Code 96: System error
  - All codes mapped to appropriate error categories and retry logic

### Added - North Recurring Billing Adapter

- **RecurringBillingAdapter** (internal/adapters/north/recurring_billing_adapter.go):
  - Complete implementation of RecurringBillingGateway interface
  - 7 operations fully implemented:
    - `CreateSubscription`: Creates new recurring subscription with BRIC token
    - `UpdateSubscription`: Updates amount, frequency, billing date, or payment method
    - `CancelSubscription`: Cancels subscription (immediate or at period end)
    - `PauseSubscription`: Temporarily pauses active subscription
    - `ResumeSubscription`: Resumes paused subscription with recalculated billing
    - `GetSubscription`: Retrieves subscription details from gateway
    - `ListSubscriptions`: Lists all subscriptions for a customer
  - HMAC-SHA256 authentication for all API calls
  - Proper error handling with payment error types
  - Retry logic based on response codes
  - Type conversion between domain models and North API formats
  - Logging support for all operations

- **Request/Response Types**:
  - `CreateSubscriptionRequest`: Structured API request with customer data, payment method, subscription details
  - `UpdateSubscriptionRequest`: Partial update support with optional fields
  - `SubscriptionResponse`: API response with subscription ID, status, next billing date
  - Support for BRIC token-based payment method references
  - Frequency mapping: Weekly, BiWeekly, Monthly, Yearly
  - Failure option mapping: Forward, Skip, Pause
  - Status mapping: Active, Paused, Cancelled, Expired

- **Comprehensive Test Coverage** (recurring_billing_adapter_test.go):
  - 14 test cases covering all operations
  - CreateSubscription: success, missing token validation, declined card handling
  - UpdateSubscription: success with partial updates
  - CancelSubscription: immediate cancellation
  - PauseSubscription: pause active subscription
  - ResumeSubscription: resume with next billing date
  - GetSubscription: retrieve subscription details
  - ListSubscriptions: list customer subscriptions
  - Network error handling with retries
  - Frequency mapping validation (4 cases)
  - Failure option mapping validation (3 cases)
  - Status mapping validation (4 cases)
  - Dependency injection demonstration
  - All tests passing with httptest mock server

- **Server Integration** (cmd/server/main.go):
  - RecurringBillingAdapter initialized in dependency injection
  - Shared HTTP client with 30-second timeout
  - Shared AuthConfig (EPIId and EPIKey) with CustomPay adapter
  - Injected into SubscriptionService for gateway-managed subscriptions
  - Replaces previous nil placeholder

### Added - Database Migrations with Goose

- **Migration Framework**:
  - Integrated Goose for SQL-based database migrations
  - Migration file with proper Goose annotations (`-- +goose Up`, `-- +goose Down`)
  - Rollback capability with down migrations
  - Clean separation of schema changes with version control

- **Migration Runner** (cmd/migrate/main.go):
  - Standalone migration binary for database management
  - Environment-based configuration (DB_HOST, DB_PORT, DB_USER, etc.)
  - Support for all Goose commands:
    - `up`: Apply all pending migrations
    - `down`: Rollback last migration
    - `status`: Show migration status
    - `version`: Show current database version
    - `create`: Create new migration file
  - PostgreSQL connection with pgx driver
  - Connection validation with ping check
  - Clear usage documentation

- **Docker Integration**:
  - Dockerfile builds both `payment-server` and `migrate` binaries
  - Separate migration service in docker-compose.yml
  - Migrations run automatically before server starts
  - Service dependency chain: postgres → migrate → payment-server
  - Migration service runs once and exits (`restart: no`)
  - Server waits for successful migration completion

- **Migration Schema** (internal/db/migrations/001_transactions.sql):
  - Transactions table with full audit trail
  - Subscriptions table for recurring billing
  - Audit logs table for PCI compliance
  - Performance-optimized indexes
  - Data integrity check constraints
  - Auto-updating timestamp triggers
  - Proper down migration for rollback

### Added - Observability (Prometheus Metrics & Health Checks)

- **Prometheus Metrics** (pkg/observability/metrics.go):
  - Automatic gRPC request tracking via interceptor
  - Metrics exposed on HTTP endpoint: `/metrics` (port 9090)
  - Three core metrics:
    - `grpc_requests_total`: Counter with labels (method, status)
    - `grpc_request_duration_seconds`: Histogram with method label
    - `grpc_requests_in_flight`: Gauge for concurrent requests
  - UnaryServerInterceptor for automatic metric collection
  - Standard Prometheus exposition format

- **Health Check System** (pkg/observability/health.go):
  - Comprehensive health check endpoint: `/health` (port 9090)
  - Database connectivity validation with 2-second timeout
  - JSON response with detailed component status
  - HTTP 503 status code when unhealthy
  - Health status includes:
    - Overall service status (healthy/unhealthy)
    - Individual component checks (database)
    - Timestamp of health check
  - Readiness probe: `/ready` (port 9090)

- **Metrics HTTP Server** (pkg/observability/server.go):
  - Dedicated HTTP server for observability (separate from gRPC)
  - Runs on configurable port (default: 9090)
  - Graceful shutdown support with 5-second timeout
  - Endpoints:
    - `/metrics`: Prometheus metrics
    - `/health`: Liveness probe
    - `/ready`: Readiness probe
  - Production-ready timeouts (read: 5s, write: 10s, idle: 15s)

- **Server Integration** (cmd/server/main.go):
  - Metrics server starts alongside gRPC server
  - Chained gRPC interceptors (metrics → logging)
  - Health checker with database pool integration
  - Graceful shutdown of both servers on SIGINT/SIGTERM
  - Startup logging with metrics/health URLs

- **Configuration**:
  - New `METRICS_PORT` environment variable (default: 9090)
  - Added to .env.example
  - Exposed in docker-compose.yml (port 9090)
  - Configurable via ServerConfig.MetricsPort

### Added - Docker Containerization

- **Dockerfile**:
  - Multi-stage build for optimized image size
  - Build stage: Uses golang:1.21-alpine
  - Runtime stage: Uses alpine:latest (minimal footprint)
  - CGO disabled for static binary compilation
  - CA certificates included for HTTPS support
  - Final image size optimized

- **docker-compose.yml**:
  - Complete stack definition with PostgreSQL
  - Service orchestration:
    - PostgreSQL 15 with persistent volume
    - Payment server with health checks
    - Automatic dependency management (waits for DB)
  - Network isolation between services
  - Volume mounts for database migrations
  - Environment variable configuration
  - Health checks for PostgreSQL
  - Restart policies configured

- **.dockerignore**:
  - Optimized Docker build context
  - Excludes unnecessary files (docs, tests, IDE configs)
  - Reduces build time and image size

- **Makefile**:
  - Common development tasks automated:
    - `make build` - Build server binary
    - `make test` - Run all tests
    - `make test-cover` - Generate coverage report
    - `make docker-build` - Build Docker image
    - `make docker-up` - Start all services (with migrations)
    - `make docker-down` - Stop all services
    - `make docker-logs` - View logs
    - `make docker-rebuild` - Rebuild and restart services
    - `make proto` - Generate protobuf code
    - `make sqlc` - Generate SQLC code
    - `make lint` - Run go vet
    - `make clean` - Clean build artifacts
  - Help system with `make help`

### Added - gRPC Server Application

- **Server Main Entrypoint** (cmd/server/main.go):
  - Complete gRPC server with dependency injection
  - Graceful shutdown handling with signal catching (SIGINT, SIGTERM)
  - Logging interceptor for all gRPC requests with duration tracking
  - Reflection service enabled for development tools (grpcurl, grpc_cli)
  - Health check support
  - Production-ready error handling

- **Configuration System** (internal/config):
  - Environment-based configuration loading
  - Support for all service components:
    - Server configuration (host, port)
    - Database configuration (PostgreSQL connection pooling)
    - Gateway configuration (North API credentials)
    - Logger configuration (level, development mode)
  - Validation of required configuration fields
  - Default values for optional settings
  - Example `.env.example` file provided

- **Dependency Injection**:
  - Clean initialization of all services in order:
    1. Logger (Zap with configurable level)
    2. Database connection pool (pgx with connection limits)
    3. Database executor and repositories
    4. Payment gateway adapters (North CustomPay)
    5. Business logic services (Payment, Subscription)
    6. gRPC handlers
  - All dependencies injected through interfaces
  - Easy to test and swap implementations

- **Binary Output**:
  - Compiles to single binary: `bin/payment-server`
  - Configured via environment variables
  - Ready for containerization (Docker)

### Added - gRPC API Layer

- **gRPC Protobuf Definitions** (api/proto):
  - **Payment API** (payment/v1/payment.proto):
    - 7 RPC methods: Authorize, Capture, Sale, Void, Refund, GetTransaction, ListTransactions
    - Complete request/response message types with validation
    - Enums for transaction status, type, and payment method type
    - BillingInfo and Address message types
  - **Subscription API** (subscription/v1/subscription.proto):
    - 8 RPC methods: CreateSubscription, UpdateSubscription, CancelSubscription, PauseSubscription, ResumeSubscription, GetSubscription, ListCustomerSubscriptions, ProcessDueBilling
    - Complete request/response message types with optional fields
    - Enums for billing frequency, subscription status, and failure option
    - Batch billing result types with error details
  - All proto files generated to Go code using protoc-gen-go and protoc-gen-go-grpc

- **gRPC Payment Handler** (internal/api/grpc/payment):
  - Implements PaymentService gRPC interface
  - Bridges protobuf messages to business logic Payment Service
  - Request validation with gRPC error codes
  - Type conversion between proto and domain models
  - Decimal amount handling (string representation in proto)
  - Billing info mapping from nested proto Address to flat domain model
  - Comprehensive logging for all operations

- **gRPC Subscription Handler** (internal/api/grpc/subscription):
  - Implements SubscriptionService gRPC interface
  - Bridges protobuf messages to business logic Subscription Service
  - Request validation with gRPC error codes
  - Type conversion with optional field handling
  - Enum mapping for frequency, status, and failure options
  - Timestamp conversion using timestamppb
  - Batch billing processing endpoint
  - Comprehensive logging for all operations

### Added - Subscription Service (Recurring Billing Business Logic)

- **Subscription Service** (internal/services/subscription):
  - Complete recurring billing orchestration with business logic
  - **Subscription lifecycle management**:
    - `CreateSubscription`: Creates new recurring billing subscription with calculated billing schedule
    - `UpdateSubscription`: Updates subscription properties (amount, frequency, payment method)
    - `CancelSubscription`: Cancels active subscription with optional gateway integration
    - `PauseSubscription`: Pauses active subscription
    - `ResumeSubscription`: Resumes paused subscription with recalculated billing date
    - `GetSubscription`: Retrieves subscription by ID
    - `ListCustomerSubscriptions`: Lists all subscriptions for a customer
  - **Batch billing processing**: `ProcessDueBilling` processes subscriptions due for billing
  - **Billing schedule calculation**: Automatic calculation of next billing date based on frequency
    - Weekly: 7 days
    - Bi-weekly: 14 days
    - Monthly: 1 month
    - Yearly: 1 year
  - **Failure handling with three strategies**:
    - `Forward`: Reschedule failed billing to next period, reset retry count
    - `Skip`: Skip current billing period, move to next, reset retry count
    - `Pause`: Pause subscription after max retries exceeded
  - **Retry mechanism**: Configurable max retries per subscription with failure count tracking
  - **Payment integration**: Uses Payment Service for actual charging (Sale operation)
  - **Gateway integration**: Optional recurring billing gateway support for gateway-managed subscriptions
  - **Idempotency for billing**: Uses subscription ID + billing date as idempotency key
  - **Transaction management**: All operations wrapped in database transactions
  - **Comprehensive logging**: Logs all subscription operations with structured fields

- **Subscription Service Port Interface** (internal/domain/ports):
  - `SubscriptionService`: Interface for subscription business logic
  - Request types: `ServiceCreateSubscriptionRequest`, `ServiceUpdateSubscriptionRequest`, `ServiceCancelSubscriptionRequest`
  - Response type: `ServiceSubscriptionResponse` with subscription details and status
  - `BillingBatchResult`: Tracks batch billing results with success/failure counts and error details
  - `BillingError`: Details about individual billing failures with retriability flag

- **Subscription Service Tests**:
  - 15 comprehensive unit tests covering all operations
  - Test coverage: **77.0%**
  - Tests include:
    - CreateSubscription: Success with gateway, without gateway
    - UpdateSubscription: Success, cancelled subscription error
    - CancelSubscription: Success, already cancelled error
    - PauseSubscription: Success, not active error
    - ResumeSubscription: Success, not paused error
    - ProcessDueBilling: Success with batch processing, failure handling
    - GetSubscription and ListCustomerSubscriptions
    - Billing schedule calculation for all frequencies

### Added - Payment Service (Business Logic Layer)

- **Payment Service** (internal/services/payment):
  - Complete payment orchestration with business logic
  - **Idempotency handling**: Prevents duplicate charges using idempotency keys
  - **Transaction management**: All operations wrapped in database transactions with automatic rollback
  - **Payment operations**:
    - `Authorize`: Authorizes payment without capturing (hold funds)
    - `Capture`: Captures previously authorized payment (full or partial)
    - `Sale`: Combined authorize + capture in one step
    - `Void`: Cancels authorized or captured transaction
    - `Refund`: Refunds captured transaction (full or partial)
  - **Transaction lifecycle tracking**: Creates child transactions for captures, voids, refunds
  - **Gateway integration**: Calls payment gateway with proper error handling
  - **Status management**: Updates both new and original transaction statuses
  - **Token-based**: Uses BRIC tokens for PCI compliance (no raw card data)
  - **Comprehensive logging**: Logs all operations with structured fields

- **Payment Service Port Interface** (internal/domain/ports):
  - `PaymentService`: Interface for payment business logic
  - Request types: `ServiceAuthorizeRequest`, `ServiceCaptureRequest`, `ServiceSaleRequest`, `ServiceVoidRequest`, `ServiceRefundRequest`
  - Response type: `PaymentResponse` with transaction details and approval status

### Added - Repository Layer (Database Access)

- **PostgreSQL Integration** (internal/adapters/postgres):
  - `DBExecutor`: Implements DBPort interface with transaction management
  - `WithTransaction()`: Write transactions with automatic rollback on error/panic
  - `WithReadOnlyTransaction()`: Optimized read-only transactions
  - Based on pgx/v5 for maximum performance

- **SQLC Type-Safe Database Code** (internal/db/sqlc):
  - Generated from SQL queries (no hand-written database code)
  - Type-safe query methods
  - Interface-based (`Querier`) for easy mocking
  - Automatic struct mapping from database rows

- **Database Schema** (internal/db/migrations):
  - `transactions` table: Stores all payment transactions
  - `subscriptions` table: Manages recurring billing
  - `audit_logs` table: PCI compliance audit trail
  - Indexes optimized for common query patterns
  - Check constraints for data integrity
  - Automatic timestamp updates via triggers

- **Database Queries** (internal/db/queries):
  - Transaction CRUD operations
  - Idempotency key lookups
  - Subscription management queries
  - Active subscription billing queries
  - Parameterized queries for safety

- **Repository Implementations** (internal/adapters/postgres):
  - `TransactionRepository`: Complete CRUD operations for transactions
    - Create with UUID parsing and metadata marshaling
    - GetByID and GetByIdempotencyKey for lookups
    - UpdateStatus for transaction state changes
    - ListByMerchant and ListByCustomer with pagination
    - Type conversion between pgtype and domain models
  - `SubscriptionRepository`: Complete CRUD operations for subscriptions
    - Create with billing schedule initialization
    - GetByID for single subscription retrieval
    - Update for modifying subscription properties
    - ListByCustomer for customer subscription history
    - ListActiveSubscriptionsDueForBilling for recurring billing job
    - Handles nullable CancelledAt timestamps
  - Helper functions for type conversion:
    - `nullText`: Converts empty strings to SQL NULL
    - `pgNumericToDecimal`: Converts pgtype.Numeric to decimal.Decimal

- **Port Interfaces** (internal/domain/ports):
  - `DBTX`: Interface for pool or transaction
  - `TransactionManager`: Transaction lifecycle management
  - `DBPort`: Combined database access interface
  - `TransactionRepository`: Interface for transaction persistence
  - `SubscriptionRepository`: Interface for subscription persistence
  - Enables testing without real database

### Added (Previous)
- **Project Structure**: Hexagonal architecture with ports/adapters pattern
  - `internal/domain/models`: Core domain entities (Transaction, Subscription, ACH models)
  - `internal/domain/ports`: Interface contracts for all dependencies
  - `internal/adapters/north`: North payment gateway implementations
  - `pkg/errors`: Custom error types with categories and retry logic
  - `pkg/security`: Security utilities and logger adapters
  - `test/mocks`: Mock implementations for testing

- **Port Interfaces** (internal/domain/ports):
  - `Logger`: Abstract logging interface for dependency injection
  - `HTTPClient`: Abstract HTTP client interface for testability
  - `CreditCardGateway`: Interface for credit card payment operations
  - `RecurringBillingGateway`: Interface for subscription management
  - `ACHGateway`: Interface for ACH/bank transfer operations

- **Domain Models** (internal/domain/models):
  - `Transaction`: Payment transaction entity with status tracking
  - `Subscription`: Recurring billing subscription entity
  - `ACHTransaction`: ACH-specific transaction with SEC codes
  - Payment method types and enumerations

- **North Payment Gateway Adapters** (internal/adapters/north):
  - **HMAC Authentication**: `CalculateSignature()` and `ValidateSignature()` for North API auth
  - **Response Code Mapper**: Comprehensive mapping of 40+ credit card and ACH response codes
    - User-friendly error messages
    - Retry logic based on error category
    - Support for Visa, Mastercard, Discover, Amex, and ACH codes
  - **Custom Pay Adapter**: Full implementation of North Custom Pay API
    - Authorize (with optional capture)
    - Capture
    - Void
    - Refund
    - Account verification (AVS)
    - BRIC token support
    - HMAC-SHA256 authentication
    - 85.7% test coverage

- **Security & Utilities** (pkg):
  - `PaymentError`: Structured error type with categories and retry flags
  - `ValidationError`: Input validation error type
  - `ZapLoggerAdapter`: Zap logger adapter implementing Logger port
  - Logger factory functions for development and production

- **Testing Infrastructure** (test/mocks):
  - `MockLogger`: Captures and verifies log calls
  - `MockHTTPClient`: Mocks HTTP requests and responses
  - Comprehensive test utilities for unit testing

- **Tests**:
  - HMAC authentication tests (100% coverage)
  - Response code mapper tests (covering all critical codes)
  - Custom Pay adapter tests:
    - Successful authorize, capture, void, refund operations
    - Error handling (insufficient funds, network errors, gateway errors)
    - Validation error handling
    - Dependency injection demonstrations
  - Overall adapter test coverage: **85.7%**

### Changed
- **Dependency Injection Pattern**: All adapters use constructor injection
  - Before: `NewAdapter(config, url, *zap.Logger)`
  - After: `NewAdapter(config, url, ports.HTTPClient, ports.Logger)`
  - Enables easy mocking and swapping of implementations

### Technical Details

#### Architecture Benefits
- **Testability**: All external dependencies (HTTP, logging) are mockable
- **Interchangeability**: Easy to swap logger implementations (Zap, custom, mock)
- **Flexibility**: Add features (circuit breaker, tracing) by wrapping interfaces
- **Team Velocity**: Multiple teams can work on different adapters simultaneously
- **Migration**: Switch payment gateways without changing business logic

#### Response Code Coverage
- **Credit Card Codes**: 00 (approval), 05, 14, 41, 43, 51, 54, 59, 82, 91, 96
- **ACH Codes**: 00 (accepted), 03, 14, 52, 53, 78, 96
- **Categories**: Approved, Declined, Insufficient Funds, Invalid Card, Expired Card, Fraud, System Error, Network Error

#### HMAC Authentication
- Algorithm: HMAC-SHA256
- Format: `signature = HMAC(concat(endpoint, payload), EPIKey)`
- Headers: `EPI-Id` (4-part merchant key), `EPI-Signature` (hex-encoded signature)
- Validation function for webhook signature verification

### Dependencies
- `github.com/shopspring/decimal`: Precise decimal arithmetic for money (PCI requirement)
- `github.com/stretchr/testify`: Testing assertions and mocks
- `go.uber.org/zap`: Structured logging (via adapter pattern)
- `github.com/jackc/pgx/v5`: PostgreSQL driver and connection pooling
- `github.com/pressly/goose/v3`: Database migration management
- `github.com/prometheus/client_golang`: Prometheus metrics collection
- `google.golang.org/grpc`: gRPC framework for API

### Documentation
- `SYSTEM_DESIGN.md`: Comprehensive system design document
- `docs/ARCHITECTURE_BENEFITS.md`: Ports & adapters architecture benefits and examples
- Code comments and examples throughout

### Next Steps
- [x] Implement payment service (business logic layer) - **COMPLETED**
- [x] Implement subscription service - **COMPLETED**
- [x] Add gRPC API layer - **COMPLETED**
- [x] Create gRPC server main entrypoint - **COMPLETED**
- [x] Add Docker containerization - **COMPLETED**
- [x] Create database migration runner - **COMPLETED**
- [x] Add Prometheus metrics - **COMPLETED**
- [x] Add health check endpoint - **COMPLETED**
- [x] Implement North Recurring Billing adapter with tests - **COMPLETED**
- [x] Integration tests with PostgreSQL database - **COMPLETED**
- [x] Implement ACH adapter with tests - **COMPLETED**
- [x] Implement Browser Post adapter with tests - **COMPLETED**
- [ ] Integration tests with North sandbox environment (requires API credentials)
- [ ] Add OpenTelemetry distributed tracing (optional)
- [ ] Add Kubernetes manifests (optional)

---

## Version History

### [0.1.0] - 2025-10-20
- Initial project setup
- Foundation layer: domain models, ports, and Custom Pay adapter
- Testing infrastructure with 85.7% adapter coverage
