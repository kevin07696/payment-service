# TODO Implementation Plan - UPDATED

**Date:** 2025-11-20
**Status:** Ready to Implement
**Estimated Time:** 5-6 hours

## Summary of User Decisions

1. ✅ **ACH Storage:** Use Server Post API (not Browser Post)
2. ✅ **ACH Verification:** Required - Pre-Note → Verify → Enable
3. ✅ **Admin:** Convert to CLI tool (simpler than JWT auth)
4. ✅ **EPX Metadata:** Read from supplementals
5. ✅ **Test 216:** Remove it

---

## Phase 1: Clean Stale TODOs (30 minutes)

### 1.1 Remove Stale Browser Post STORAGE TODOs

**Files:**
- `tests/integration/payment_method/payment_method_test.go`

**Actions:**
1. **Tests 1-5** (Lines 18, 91, 129, 168, 269) - Unskip and update:
   ```go
   // REMOVE t.Skip() lines

   // ADD JWT token generation helper first
   jwtToken := testutil.GenerateTestJWT(t, cfg, "test-merchant-staging")

   // UPDATE to use TokenizeAndSaveCardViaBrowserPost
   paymentMethodID, err := testutil.TokenizeAndSaveCardViaBrowserPost(
       t, cfg, client, jwtToken,
       "test-merchant-staging",
       "test-customer-001",
       testutil.TestVisaCard,
       cfg.CallbackBaseURL,
   )
   ```

2. **Test 6** (Line 216) - **REMOVE ENTIRE TEST**
   - References non-existent `StorePaymentMethod` RPC
   - No longer needed

**Files:**
- `tests/integration/payment/browser_post_workflow_test.go:224`
  ```diff
  - // TODO: Implement SALE with stored BRIC when STORAGE endpoint is available
  + // SALE with stored BRIC using Browser Post STORAGE
  ```

- `tests/integration/testutil/tokenization.go:392`
  ```diff
  - // TODO: Update to use Browser Post STORAGE flow
    return "", fmt.Errorf("TokenizeAndSaveCard deprecated...")
  ```

- `tests/integration/fixtures/epx_brics.go:24, 37`
  ```diff
  - // TODO: Replace with real BRIC from EPX sandbox
  + // NOTE: Mock BRIC for testing - can replace with real sandbox BRIC if needed
  ```

---

## Phase 2: Implement ACH Storage with Server Post (3 hours)

### 2.1 Understanding: ACH GUID/BRIC Flow

**From EPX Documentation:**

1. **Pre-Note Debit (CKC0)** - Verify account can be debited ($0.00)
   - Sends routing number + account number
   - EPX returns **GUID/BRIC**
   - This triggers verification process at bank

2. **Store GUID/BRIC** - Save in payment_methods table
   - Status: `pending_verification`
   - Cannot be used for payments yet

3. **Verify Account** - After micro-deposits confirm
   - Update status: `pending_verification` → `active`
   - Now can use for payments

4. **Future Transactions** - Use stored GUID/BRIC
   - Sale/Debit (CKC2) with ORIG_AUTH_GUID
   - No need to send account number again

### 2.2 Server Post Adapter Extension (1 hour)

**File:** `internal/adapters/epx/server_post_adapter.go`

**Add Method:**
```go
// SubmitACHPreNote sends Pre-Note Debit (CKC0) to validate ACH account
// Returns GUID/BRIC that can be stored for future transactions
func (a *ServerPostAdapter) SubmitACHPreNote(ctx context.Context, req *ports.ACHPreNoteRequest) (*ports.ACHPreNoteResponse, error) {
    logger := a.logger.With(
        zap.String("merchant_number", req.MerchantNumber),
        zap.String("transaction_type", "CKC0"), // Pre-Note Debit for Checking
    )

    // Build XML request for Pre-Note Debit
    xmlReq := fmt.Sprintf(`
<DETAIL CUST_NBR="%s" MERCH_NBR="%s" DBA_NBR="%s" TERMINAL_NBR="%s">
    <TRAN_TYPE>%s</TRAN_TYPE>
    <ACCOUNT_NBR>%s</ACCOUNT_NBR>
    <ROUTING_NBR>%s</ROUTING_NBR>
    <BATCH_ID>%s</BATCH_ID>
    <TRAN_NBR>%s</TRAN_NBR>
    <AMOUNT>0.00</AMOUNT>
    <FIRST_NAME>%s</FIRST_NAME>
    <LAST_NAME>%s</LAST_NAME>
    <ADDRESS>%s</ADDRESS>
    <CITY>%s</CITY>
    <STATE>%s</STATE>
    <ZIP_CODE>%s</ZIP_CODE>
    <STD_ENTRY_CLASS>%s</STD_ENTRY_CLASS>
    <RECV_NAME>%s</RECV_NAME>
</DETAIL>`,
        req.CustomerNumber,
        req.MerchantNumber,
        req.DBANumber,
        req.TerminalNumber,
        req.TransactionType, // "CKC0" for checking, "CKS0" for savings
        req.AccountNumber,
        req.RoutingNumber,
        req.BatchID,
        req.TransactionNumber,
        req.FirstName,
        req.LastName,
        req.BillingAddress.Street,
        req.BillingAddress.City,
        req.BillingAddress.State,
        req.BillingAddress.PostalCode,
        req.StandardEntryClass, // "PPD" for personal, "CCD" for business
        req.ReceiverName,
    )

    // Submit to EPX Server Post endpoint
    resp, err := a.httpClient.Post(
        a.serverPostURL,
        "application/xml",
        strings.NewReader(xmlReq),
    )
    if err != nil {
        logger.Error("Failed to submit ACH pre-note", zap.Error(err))
        return nil, fmt.Errorf("failed to submit ACH pre-note: %w", err)
    }
    defer resp.Body.Close()

    // Parse XML response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    // Extract GUID/BRIC from response
    var epxResp struct {
        GUID   string `xml:"GUID"`
        Status string `xml:"STATUS"`
        Error  string `xml:"ERROR"`
    }
    if err := xml.Unmarshal(body, &epxResp); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    if epxResp.Status != "APPROVED" {
        return nil, fmt.Errorf("pre-note declined: %s", epxResp.Error)
    }

    return &ports.ACHPreNoteResponse{
        GUID:              epxResp.GUID,
        Status:            "pending_verification",
        TransactionNumber: req.TransactionNumber,
    }, nil
}

// SubmitACHDebit sends Sale/Debit (CKC2) using stored GUID/BRIC
func (a *ServerPostAdapter) SubmitACHDebit(ctx context.Context, req *ports.ACHDebitRequest) (*ports.ACHDebitResponse, error) {
    // Build XML with ORIG_AUTH_GUID instead of account details
    xmlReq := fmt.Sprintf(`
<DETAIL CUST_NBR="%s" MERCH_NBR="%s" DBA_NBR="%s" TERMINAL_NBR="%s">
    <TRAN_TYPE>CKC2</TRAN_TYPE>
    <BATCH_ID>%s</BATCH_ID>
    <TRAN_NBR>%s</TRAN_NBR>
    <AMOUNT>%s</AMOUNT>
    <ORIG_AUTH_GUID>%s</ORIG_AUTH_GUID>
    <FIRST_NAME>%s</FIRST_NAME>
    <LAST_NAME>%s</LAST_NAME>
    <STD_ENTRY_CLASS>%s</STD_ENTRY_CLASS>
</DETAIL>`,
        req.CustomerNumber,
        req.MerchantNumber,
        req.DBANumber,
        req.TerminalNumber,
        req.BatchID,
        req.TransactionNumber,
        req.Amount,
        req.StorageGUID, // Use stored GUID instead of account number
        req.FirstName,
        req.LastName,
        req.StandardEntryClass,
    )

    // Submit and parse response (similar to pre-note)
    // ...
}
```

**Add to ports interface:**
```go
// internal/adapters/ports/payment_gateway.go

type ACHPreNoteRequest struct {
    MerchantNumber      string
    DBANumber          string
    TerminalNumber     string
    CustomerNumber     string
    TransactionType    string // "CKC0" or "CKS0"
    AccountNumber      string
    RoutingNumber      string
    BatchID           string
    TransactionNumber string
    FirstName         string
    LastName          string
    BillingAddress    Address
    StandardEntryClass string // "PPD" or "CCD"
    ReceiverName      string
}

type ACHPreNoteResponse struct {
    GUID              string
    Status            string // "pending_verification"
    TransactionNumber string
}
```

### 2.3 Payment Method Service (45 minutes)

**File:** `internal/services/payment_method/payment_method_service.go`

**Add Method:**
```go
func (s *paymentMethodService) StoreACHAccount(ctx context.Context, req *ports.StoreACHAccountRequest) (*domain.PaymentMethod, error) {
    // 1. Validate request
    if req.RoutingNumber == "" || req.AccountNumber == "" {
        return nil, domain.ErrInvalidPaymentMethod
    }

    // 2. Submit Pre-Note to EPX (CKC0)
    preNoteResp, err := s.serverPostAdapter.SubmitACHPreNote(ctx, &ports.ACHPreNoteRequest{
        MerchantNumber:      req.MerchantCredentials.MerchantNumber,
        DBANumber:          req.MerchantCredentials.DBANumber,
        TerminalNumber:     req.MerchantCredentials.TerminalNumber,
        CustomerNumber:     req.MerchantCredentials.CustomerNumber,
        TransactionType:    req.AccountType == "SAVINGS" ? "CKS0" : "CKC0",
        AccountNumber:      req.AccountNumber,
        RoutingNumber:      req.RoutingNumber,
        BatchID:           time.Now().Format("20060102"),
        TransactionNumber: generateTransactionNumber(),
        FirstName:         req.NameOnAccount,
        LastName:          "",
        BillingAddress:    req.BillingAddress,
        StandardEntryClass: "PPD", // Personal accounts
        ReceiverName:      req.NameOnAccount,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to submit pre-note: %w", err)
    }

    // 3. Store payment method with status=pending_verification
    paymentMethod, err := s.db.Queries().CreatePaymentMethod(ctx, sqlc.CreatePaymentMethodParams{
        ID:         uuid.New(),
        MerchantID: req.MerchantID,
        CustomerID: req.CustomerID,
        Type:       "ach",
        Token:      pgtype.Text{String: preNoteResp.GUID, Valid: true}, // Store GUID/BRIC
        Last4:      req.AccountNumber[len(req.AccountNumber)-4:],
        Status:     "pending_verification", // NOT active yet
        Metadata: map[string]interface{}{
            "account_type":   req.AccountType,
            "name_on_account": req.NameOnAccount,
            "routing_number_last4": req.RoutingNumber[len(req.RoutingNumber)-4:],
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to store payment method: %w", err)
    }

    return &domain.PaymentMethod{
        ID:         paymentMethod.ID.String(),
        MerchantID: paymentMethod.MerchantID,
        CustomerID: paymentMethod.CustomerID,
        Type:       "ach",
        Status:     "pending_verification",
        Last4:      paymentMethod.Last4,
        CreatedAt:  paymentMethod.CreatedAt.Time,
    }, nil
}
```

### 2.4 ConnectRPC Handler (30 minutes)

**File:** `internal/handlers/payment_method/payment_method_handler_connect.go`

**Update StoreACHAccount:**
```go
func (h *ConnectHandler) StoreACHAccount(
    ctx context.Context,
    req *connect.Request[paymentmethodv1.StoreACHAccountRequest],
) (*connect.Response[paymentmethodv1.PaymentMethodResponse], error) {

    // Validate
    if req.Msg.MerchantId == "" || req.Msg.CustomerId == "" {
        return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("merchant_id and customer_id required"))
    }
    if req.Msg.RoutingNumber == "" || req.Msg.AccountNumber == "" {
        return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("account details required"))
    }

    // Get merchant credentials
    merchant, err := h.dbAdapter.Queries().GetMerchantByID(ctx, req.Msg.MerchantId)
    if err != nil {
        return nil, connect.NewError(connect.CodeNotFound, errors.New("merchant not found"))
    }

    // Call service to store ACH account (submits pre-note)
    paymentMethod, err := h.paymentMethodService.StoreACHAccount(ctx, &ports.StoreACHAccountRequest{
        MerchantID:     req.Msg.MerchantId,
        CustomerID:     req.Msg.CustomerId,
        RoutingNumber:  req.Msg.RoutingNumber,
        AccountNumber:  req.Msg.AccountNumber,
        AccountType:    req.Msg.AccountType, // "CHECKING" or "SAVINGS"
        NameOnAccount:  req.Msg.NameOnAccount,
        BillingAddress: convertBillingAddress(req.Msg.BillingInfo),
        MerchantCredentials: ports.MerchantCredentials{
            MerchantNumber: merchant.MerchantNumber,
            DBANumber:      merchant.DBANumber,
            TerminalNumber: merchant.TerminalNumber,
            CustomerNumber: merchant.CustomerNumber,
        },
    })
    if err != nil {
        return nil, handleServiceErrorConnect(err)
    }

    return connect.NewResponse(&paymentmethodv1.PaymentMethodResponse{
        Id:         paymentMethod.ID,
        MerchantId: paymentMethod.MerchantID,
        CustomerId: paymentMethod.CustomerID,
        Type:       "ach",
        Status:     "pending_verification", // Waiting for micro-deposit verification
        Last4:      paymentMethod.Last4,
        CreatedAt:  timestamppb.New(paymentMethod.CreatedAt),
    }), nil
}
```

### 2.5 Test Utilities (30 minutes)

**File:** `tests/integration/testutil/tokenization.go`

**Update stub to call StoreACHAccount:**
```go
func TokenizeAndSaveACH(cfg *Config, client *Client, merchantID, customerID string, achAccount TestACH) (string, error) {
    // Generate JWT token for auth
    jwtToken := GenerateTestJWT(cfg, merchantID)

    // Call StoreACHAccount via ConnectRPC
    connectClient := NewClient("http://localhost:8080")
    connectClient.SetHeader("Authorization", "Bearer "+jwtToken)
    defer connectClient.ClearHeaders()

    resp, err := connectClient.DoConnectRPC("payment_method.v1.PaymentMethodService", "StoreACHAccount", map[string]interface{}{
        "merchant_id":    merchantID,
        "customer_id":    customerID,
        "routing_number": achAccount.RoutingNumber,
        "account_number": achAccount.AccountNumber,
        "account_type":   achAccount.AccountType,
        "name_on_account": achAccount.NameOnAccount,
        "billing_info": map[string]string{
            "street":      "123 Test St",
            "city":        "Testville",
            "state":       "DE",
            "postal_code": "19703",
        },
    })
    if err != nil {
        return "", fmt.Errorf("StoreACHAccount failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("StoreACHAccount failed: %s", string(body))
    }

    var result map[string]interface{}
    if err := DecodeResponse(resp, &result); err != nil {
        return "", err
    }

    paymentMethodID, ok := result["id"].(string)
    if !ok {
        return "", errors.New("payment method ID not found in response")
    }

    return paymentMethodID, nil
}
```

### 2.6 Unskip Integration Tests (Now ACH works!)

**Tests to unskip:** 11 total
- `tests/integration/cron/ach_verification_cron_test.go:20, 109, 184` (3)
- `tests/integration/payment/payment_ach_verification_test.go:24, 53, 102, 159, 216` (5)
- `tests/integration/payment_method/payment_method_test.go:56, 61` (2)

**Remove t.Skip() and update TODO comments:**
```go
// WAS:
t.Skip("TODO: Update to use StoreACHAccount RPC once implemented")

// NOW:
// StoreACHAccount creates payment method with pending_verification status
// Tests verify the full pre-note → verification → active workflow
```

---

## Phase 3: Create Admin CLI Tool (1.5 hours)

### 3.1 Why CLI > API for Admin

**Benefits:**
- ✅ No JWT authentication complexity
- ✅ Direct database access (simpler)
- ✅ Secure (runs on server, not exposed)
- ✅ Perfect for ops tasks

**CLI Commands:**
```bash
./admin create-service --service-id=my-app --name="My App" --env=production
./admin rotate-key --service-id=my-app --reason="Scheduled rotation"
./admin deactivate-service --service-id=my-app --reason="Service deprecated"
./admin list-services
```

### 3.2 Create CLI Tool

**File:** `cmd/admin/main.go` (NEW)

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/kevin07696/payment-service/internal/db/sqlc"
    "github.com/kevin07696/payment-service/internal/handlers/admin"
    "github.com/spf13/cobra"
    "go.uber.org/zap"
)

var (
    dbURL string
    logger *zap.Logger
)

func main() {
    var err error
    logger, err = zap.NewProduction()
    if err != nil {
        log.Fatal(err)
    }
    defer logger.Sync()

    rootCmd := &cobra.Command{
        Use:   "admin",
        Short: "Payment Service Admin CLI",
    }

    rootCmd.PersistentFlags().StringVar(&dbURL, "db-url", os.Getenv("DATABASE_URL"), "Database URL")

    rootCmd.AddCommand(createServiceCmd())
    rootCmd.AddCommand(rotateKeyCmd())
    rootCmd.AddCommand(deactivateServiceCmd())
    rootCmd.AddCommand(listServicesCmd())

    if err := rootCmd.Execute(); err != nil {
        logger.Fatal("Failed to execute command", zap.Error(err))
    }
}

func getDB() *pgxpool.Pool {
    pool, err := pgxpool.New(context.Background(), dbURL)
    if err != nil {
        logger.Fatal("Failed to connect to database", zap.Error(err))
    }
    return pool
}

func createServiceCmd() *cobra.Command {
    var (
        serviceID         string
        serviceName       string
        environment       string
        requestsPerSecond int32
        burstLimit        int32
    )

    cmd := &cobra.Command{
        Use:   "create-service",
        Short: "Create a new service with auto-generated keypair",
        Run: func(cmd *cobra.Command, args []string) {
            db := getDB()
            defer db.Close()

            handler := admin.NewServiceHandler(sqlc.New(db))

            // Create service
            resp, err := handler.CreateService(context.Background(), &connect.Request[adminv1.CreateServiceRequest]{
                Msg: &adminv1.CreateServiceRequest{
                    ServiceId:         serviceID,
                    ServiceName:       serviceName,
                    Environment:       environment,
                    RequestsPerSecond: requestsPerSecond,
                    BurstLimit:        burstLimit,
                },
            })
            if err != nil {
                logger.Fatal("Failed to create service", zap.Error(err))
            }

            fmt.Printf("Service Created Successfully!\n")
            fmt.Printf("Service ID: %s\n", resp.Msg.Service.ServiceId)
            fmt.Printf("Fingerprint: %s\n", resp.Msg.Service.PublicKeyFingerprint)
            fmt.Printf("\n⚠️  SAVE THIS PRIVATE KEY - IT WILL NOT BE SHOWN AGAIN:\n\n%s\n", resp.Msg.PrivateKey)

            // Audit log to file
            logAudit("service.created", serviceID, "cli-user", "")
        },
    }

    cmd.Flags().StringVar(&serviceID, "service-id", "", "Service ID (required)")
    cmd.Flags().StringVar(&serviceName, "name", "", "Service name (required)")
    cmd.Flags().StringVar(&environment, "env", "production", "Environment (production/staging/development)")
    cmd.Flags().Int32Var(&requestsPerSecond, "rps", 100, "Requests per second limit")
    cmd.Flags().Int32Var(&burstLimit, "burst", 200, "Burst limit")

    cmd.MarkFlagRequired("service-id")
    cmd.MarkFlagRequired("name")

    return cmd
}

func logAudit(action, serviceID, adminID, reason string) {
    logger.Info("AUDIT",
        zap.String("action", action),
        zap.String("service_id", serviceID),
        zap.String("admin_id", adminID),
        zap.String("reason", reason),
    )
}

// ... similar for rotateKeyCmd, deactivateServiceCmd, listServicesCmd
```

### 3.3 Remove Audit TODOs (Now handled by CLI logging)

**Files:** `internal/handlers/admin/service_handler.go`

```diff
- // TODO: Get admin ID from auth context
- CreatedBy: pgtype.UUID{Valid: false},
+ // Admin operations now via CLI - admin ID tracked in audit logs
+ CreatedBy: pgtype.UUID{Valid: false},

- // TODO: Audit log the service creation
+ // Audit logging handled by CLI tool

- // TODO: Audit log the key rotation with reason
+ // Audit logging handled by CLI tool

- // TODO: Audit log deactivation with reason
+ // Audit logging handled by CLI tool
```

**Result:** Remove 4 admin audit TODOs ✅

---

## Phase 4: Other Valid TODOs (1 hour)

### 4.1 UpdatePaymentMethod Metadata (20 min)

**Implementation:** See previous plan - straightforward metadata update

### 4.2 Extract Payment Metadata from EPX (20 min)

**Read supplementals for card type fields, then implement extraction**

### 4.3 Fix Service Count Query (10 min)

**Add CountActiveServices query to sqlc**

### 4.4 Remove Test 216 (1 min)

```bash
# Just delete the test
```

### 4.5 Security Tests (30 min)

**Implement replay attack and IP whitelist tests**

---

## Phase 5: Testing (1 hour)

### 5.1 Integration Tests
```bash
# ACH tests should now pass!
go test -tags=integration ./tests/integration/payment/payment_ach_verification_test.go -v

# All integration tests
go test -tags=integration ./tests/integration/... -v
```

### 5.2 QA Checks
```bash
go vet ./...
staticcheck ./...
go build ./...
```

---

## Summary of Changes

| Task | TODOs Resolved | Time |
|------|---------------|------|
| Clean stale TODOs | 11 | 30 min |
| Implement ACH Storage | 13 | 3 hours |
| Create Admin CLI | 4 | 1.5 hours |
| Other features | 4 | 1 hour |
| **TOTAL** | **32** | **6 hours** |

**Remaining TODO:** 1 (database integration test - low priority)

---

**Ready to start? Let me know and I'll begin with Phase 1!**
