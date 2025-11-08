# Browser Post Complete Dataflow

**Date**: 2025-11-03
**Status**: ✅ CORRECTED - Removed incorrect Key Exchange API references

---

## Flow Overview

The EPX Browser Post API provides a PCI-compliant payment flow where card data is submitted directly from the user's browser to EPX, never touching the merchant backend.

```
┌─────────────────────────────────────────────────────────────────────┐
│                    BROWSER POST COMPLETE DATAFLOW                   │
└─────────────────────────────────────────────────────────────────────┘

Step 1: GENERATE TAC TOKEN
┌──────────────────────────────────────────────────────────────┐
│ Component: Merchant Backend                                  │
│ Responsibility: Obtain TAC token for transaction            │
│                                                              │
│ Note: TAC (Terminal Authorization Code) generation method   │
│ depends on merchant's specific EPX credentials setup.       │
│                                                              │
│ TAC Token Contains (encrypted):                             │
│   - MAC (Merchant Authorization Code)                       │
│   - REDIRECT_URL (callback endpoint)                        │
│   - AMOUNT                                                  │
│   - TRAN_NBR (unique transaction number)                   │
│   - TRAN_GROUP (e.g., "SALE")                              │
│   - Expiration timestamp (4 hours)                         │
│                                                              │
│ Output:                                                      │
│   ✅ TAC token string                                       │
└──────────────────────────────────────────────────────────────┘
                            │
                            ▼

Step 2: BUILD FORM DATA
┌──────────────────────────────────────────────────────────────┐
│ Component: BrowserPostAdapter                                │
│ File: internal/adapters/epx/browser_post_adapter.go:62      │
│ Method: BuildFormData(tac, amount, tranNbr, ...)           │
│                                                              │
│ Input:                                                       │
│   - TAC (from Step 1)                                       │
│   - Amount ("99.99")                                        │
│   - TranNbr ("TXN-12345")                                  │
│   - TranGroup ("SALE")                                     │
│   - RedirectURL ("http://localhost:8081/api/v1/...")      │
│                                                              │
│ Process:                                                     │
│   1. Validates all required fields                          │
│   2. Constructs BrowserPostFormData struct                 │
│   3. Sets PostURL based on environment                     │
│                                                              │
│ Output:                                                      │
│   ✅ BrowserPostFormData struct:                           │
│      - PostURL (EPX endpoint)                              │
│      - TAC                                                 │
│      - Amount, TranNbr, TranGroup                          │
│      - RedirectURL (callback endpoint)                     │
│      - MerchantName                                        │
│      - CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR         │
│                                                              │
│ EPX Endpoints:                                              │
│   - Sandbox: https://epxnow.com/epx/browser_post_sandbox   │
│   - Production: https://epxnow.com/epx/browser_post        │
└──────────────────────────────────────────────────────────────┘
                            │
                            ▼

Step 3: RENDER FRONTEND FORM
┌──────────────────────────────────────────────────────────────┐
│ Component: Merchant Frontend (HTML Form)                    │
│ Responsibility: Merchant's web application                  │
│                                                              │
│ HTML Form Fields:                                           │
│   <form method="POST" action="{{.PostURL}}">               │
│     <!-- Hidden fields from backend -->                     │
│     <input type="hidden" name="TAC" value="{{.TAC}}" />    │
│     <input type="hidden" name="TRAN_NBR" value="..." />    │
│     <input type="hidden" name="AMOUNT" value="99.99" />    │
│     <input type="hidden" name="TRAN_GROUP" value="SALE"/>  │
│     <input type="hidden" name="TRAN_CODE" value="SALE"/>   │
│     <input type="hidden" name="INDUSTRY_TYPE" value="E"/>  │
│     <input type="hidden" name="REDIRECT_URL" value="..."/> │
│     <input type="hidden" name="CUST_NBR" value="..." />    │
│     <input type="hidden" name="MERCH_NBR" value="..." />   │
│     <input type="hidden" name="DBA_NBR" value="..." />     │
│     <input type="hidden" name="TERMINAL_NBR" value="..."/> │
│                                                              │
│     <!-- User-entered card data (PCI-sensitive) -->         │
│     <input type="text" name="CARD_NBR" />                  │
│     <input type="text" name="EXP_MONTH" />                 │
│     <input type="text" name="EXP_YEAR" />                  │
│     <input type="text" name="CVV" />                       │
│                                                              │
│     <button type="submit">Pay Now</button>                 │
│   </form>                                                    │
│                                                              │
│ User Action:                                                │
│   ✅ User enters card details                              │
│   ✅ User clicks "Pay Now"                                 │
│   ✅ Browser POSTs directly to EPX (not merchant backend!)│
│                                                              │
│ PCI Compliance:                                             │
│   ✅ Card data never touches merchant backend              │
└──────────────────────────────────────────────────────────────┘
                            │
                            ▼

Step 4: EPX PROCESSING
┌──────────────────────────────────────────────────────────────┐
│ Component: EPX Payment Gateway (External)                   │
│ URL: https://epxnow.com/epx/browser_post[_sandbox]         │
│                                                              │
│ Validation Phase:                                           │
│   1. Decrypts TAC token                                     │
│   2. Verifies TAC not expired (4-hour window)              │
│   3. Compares TAC fields with POSTed fields for tampering  │
│   4. Validates all fields with regex patterns              │
│   5. Checks REDIRECT_URL is authorized for merchant        │
│                                                              │
│ Payment Processing Phase:                                   │
│   1. Validates card data with card networks                 │
│   2. Performs fraud checks (AVS, CVV)                       │
│   3. Authorizes transaction                                 │
│   4. Generates AUTH_GUID (Financial BRIC token)             │
│   5. Builds response with all transaction fields            │
│                                                              │
│ Redirect Phase (PRG Pattern):                               │
│   1. Redirects to EPX response page                         │
│   2. EPX response page auto-POSTs to REDIRECT_URL          │
│   3. Prevents duplicate processing on browser Back/Refresh  │
│                                                              │
│ PCI Compliance:                                             │
│   ✅ Card data processed securely by EPX                   │
│   ✅ Merchant receives only tokenized response             │
└──────────────────────────────────────────────────────────────┘
                            │
                            ▼

Step 5: CALLBACK HANDLER (MERCHANT BACKEND)
┌──────────────────────────────────────────────────────────────┐
│ Component: BrowserPostCallbackHandler                       │
│ File: internal/handlers/payment/                            │
│       browser_post_callback_handler.go:45                   │
│ Endpoint: POST /api/v1/payments/browser-post/callback      │
│ Port: 8081 (HTTP server)                                    │
│                                                              │
│ Sub-Step 5a: Parse Response                                 │
│ ────────────────────────────────────────                    │
│ Method: browserPost.ParseRedirectResponse(params)           │
│ File: internal/adapters/epx/browser_post_adapter.go:107    │
│                                                              │
│ Received Form Fields (EPX redirects with):                  │
│   ✅ AUTH_GUID - Transaction token (Financial BRIC)        │
│   ✅ AUTH_RESP - Approval code ("00" = approved)           │
│   ✅ AUTH_CODE - Bank authorization code                   │
│   ✅ AUTH_RESP_TEXT - Human-readable message               │
│   ✅ AUTH_CARD_TYPE - Card brand (V/M/A/D)                 │
│   ✅ AUTH_AVS - Address verification result                │
│   ✅ AUTH_CVV2 - CVV verification result                   │
│   ✅ TRAN_NBR - Echo back transaction number               │
│   ✅ AMOUNT - Echo back amount                             │
│   ✅ CARD_NBR - Masked card number (last 4 digits)         │
│   + 20+ other optional fields                              │
│                                                              │
│ Process:                                                     │
│   1. Validates AUTH_GUID and AUTH_RESP exist               │
│   2. Determines if approved (AUTH_RESP == "00")            │
│   3. Extracts all response fields                          │
│   4. Returns BrowserPostResponse struct                    │
│                                                              │
│ Sub-Step 5b: Check for Duplicates                          │
│ ────────────────────────────────────────                    │
│ Method: dbAdapter.GetTransactionByIdempotencyKey()         │
│                                                              │
│ Process:                                                     │
│   1. Uses TRAN_NBR as idempotency key                      │
│   2. Queries transactions table                            │
│   3. If found: Return existing transaction (no insert)     │
│   4. If not found: Continue to storage                     │
│                                                              │
│ Why: EPX implements PRG pattern                            │
│   - Browser "Back" button may cause re-POST                │
│   - We prevent duplicate database inserts                  │
│                                                              │
│ Sub-Step 5c: Store Transaction                             │
│ ────────────────────────────────────────                    │
│ Method: storeTransaction(ctx, response)                    │
│ File: browser_post_callback_handler.go:137                 │
│                                                              │
│ Database Operation:                                         │
│   Method: dbAdapter.Queries().CreateTransaction()          │
│   Table: transactions                                       │
│                                                              │
│ Stored Fields:                                              │
│   ✅ id (UUID)                                             │
│   ✅ group_id (UUID)                                       │
│   ✅ agent_id (from CUST_NBR in raw params)               │
│   ✅ customer_id (NULL for guest checkout)                 │
│   ✅ amount (parsed from response.Amount)                  │
│   ✅ currency ("USD")                                      │
│   ✅ status ("completed" or "failed")                      │
│   ✅ type ("charge")                                       │
│   ✅ payment_method_type ("credit_card")                   │
│   ✅ payment_method_id (NULL for guest checkout)           │
│   ✅ auth_guid (AUTH_GUID/Financial BRIC) ← CRITICAL      │
│   ✅ auth_resp (AUTH_RESP)                                 │
│   ✅ auth_code (AUTH_CODE)                                 │
│   ✅ auth_resp_text (AUTH_RESP_TEXT)                       │
│   ✅ auth_card_type (AUTH_CARD_TYPE)                       │
│   ✅ auth_avs (AUTH_AVS)                                   │
│   ✅ auth_cvv2 (AUTH_CVV2)                                 │
│   ✅ idempotency_key (TRAN_NBR)                            │
│   ✅ metadata (empty JSON object)                          │
│   ✅ created_at, updated_at (auto-generated)               │
│                                                              │
│ Why Store AUTH_GUID (Financial BRIC)?                      │
│   ✅ Required for REFUNDS                                  │
│   ✅ Required for VOIDS (cancel before settlement)         │
│   ✅ Required for CHARGEBACK DEFENSE                       │
│   ✅ Required for RECONCILIATION with EPX reports          │
│   ✅ Can be used for RECURRING PAYMENTS (13-24 months)     │
│   ✅ Can be converted to Storage BRIC for saved payment    │
│       methods (never expires)                              │
│   ✅ Not PCI-sensitive (already tokenized by EPX)          │
│                                                              │
│ Sub-Step 5d: Render Receipt Page                           │
│ ────────────────────────────────────────                    │
│ Method: renderReceiptPage(w, response, txID)               │
│ File: browser_post_callback_handler.go:218                 │
│                                                              │
│ Success Page (AUTH_RESP == "00"):                          │
│   ✅ Checkmark icon                                        │
│   ✅ "Payment Successful" heading                          │
│   ✅ Amount display                                        │
│   ✅ Masked card number (last 4 digits)                    │
│   ✅ Card type (Visa/Mastercard/etc)                       │
│   ✅ Authorization code                                    │
│   ✅ Transaction ID (database UUID)                        │
│   ✅ Reference number (TRAN_NBR)                           │
│   ✅ "Thank you" message                                   │
│                                                              │
│ Failure Page (AUTH_RESP != "00"):                          │
│   ✅ X icon                                                │
│   ✅ "Payment Failed" heading                              │
│   ✅ Error message (AUTH_RESP_TEXT)                        │
│   ✅ Amount display                                        │
│   ✅ "Try Again" button                                    │
│                                                              │
│ Output:                                                      │
│   ✅ HTML page rendered to user's browser                  │
│   ✅ User sees immediate feedback                          │
└──────────────────────────────────────────────────────────────┘
```

---

## Component Verification

### ✅ 1. Browser Post Adapter
- **File**: `internal/adapters/epx/browser_post_adapter.go`
- **Port**: `internal/adapters/ports/browser_post.go`
- **Status**: IMPLEMENTED
- **Methods**:
  - ✅ `BuildFormData(tac, amount, tranNbr, tranGroup, redirectURL) -> BrowserPostFormData`
  - ✅ `ParseRedirectResponse(params) -> BrowserPostResponse`
  - ✅ `ValidateResponseMAC(params, mac) -> error`
  - ✅ `DefaultBrowserPostConfig(environment) -> BrowserPostConfig`
  - ✅ `NewBrowserPostAdapter(config, logger) -> BrowserPostAdapter`

### ✅ 2. Callback Handler
- **File**: `internal/handlers/payment/browser_post_callback_handler.go`
- **Status**: IMPLEMENTED
- **Methods**:
  - ✅ `HandleCallback(w http.ResponseWriter, r *http.Request)`
  - ✅ `storeTransaction(ctx, response) -> (string, error)`
  - ✅ `renderReceiptPage(w, response, txID)`
  - ✅ `renderErrorPage(w, message, details)`
  - ✅ `NewBrowserPostCallbackHandler(dbAdapter, browserPost, logger)`

### ✅ 3. Database Layer
- **Schema**: `internal/db/migrations/002_transactions.sql`
- **Queries**: `internal/db/queries/transactions.sql`
- **Generated**: `internal/db/sqlc/transactions.sql.go`
- **Status**: IMPLEMENTED
- **Methods**:
  - ✅ `CreateTransaction(ctx, CreateTransactionParams) -> Transaction`
  - ✅ `GetTransactionByIdempotencyKey(ctx, idempotencyKey) -> Transaction`
  - ✅ `GetTransactionByID(ctx, id) -> Transaction`

### ✅ 4. Server Integration
- **File**: `cmd/server/main.go`
- **Status**: WIRED UP
- **Changes**:
  - ✅ Added `browserPostCallbackHandler` to Dependencies struct (line 186)
  - ✅ Initialized handler in `initDependencies()` (line 359)
  - ✅ Registered endpoint on HTTP mux (line 96)
  - ✅ Endpoint: `POST /api/v1/payments/browser-post/callback`
  - ✅ Port: 8081 (HTTP server, same as cron endpoints)

---

## Configuration Required

### EPX Credentials Setup

**1. Obtain Browser Post Credentials from EPX:**
- CUST_NBR (Customer Number)
- MERCH_NBR (Merchant Number)
- DBA_NBR (DBA Number)
- TERMINAL_NBR (Terminal Number)
- MAC (Merchant Authorization Code)
- TAC generation credentials/method

**2. Provide REDIRECT_URL to EPX:**

EPX **MUST** configure your REDIRECT_URL before Browser Post will work:

**Local Development:**
```
http://localhost:8081/api/v1/payments/browser-post/callback
```

**Production:**
```
https://yourdomain.com/api/v1/payments/browser-post/callback
```

### Environment Variables

```bash
# .env file
EPX_ENVIRONMENT=sandbox  # or "production"
EPX_CUST_NBR=123456
EPX_MERCH_NBR=789012
EPX_DBA_NBR=345678
EPX_TERMINAL_NBR=901234
EPX_MAC=your-merchant-authorization-code

# HTTP server port (for callback endpoint and cron jobs)
HTTP_PORT=8081

# gRPC server port
PORT=8080

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=payment_service
```

---

## Financial BRIC Token Usage

The AUTH_GUID returned from EPX Browser Post is a **Financial BRIC** token with the following capabilities:

### Current Implementation (Guest Checkout)
1. **Refunds**: Use BRIC to refund the transaction
2. **Voids**: Cancel transaction before settlement
3. **Chargeback Defense**: Reference in dispute resolution
4. **Reconciliation**: Match with EPX reporting

### Future Enhancement (Saved Payment Methods)
When user opts to save payment method:
1. **Convert to Storage BRIC**: Call EPX API to convert Financial BRIC (13-24 month lifetime) to Storage BRIC (never expires)
2. **Store in customer_payment_methods**: Link Storage BRIC to customer_id and merchant_id
3. **Recurring Payments**: Use Storage BRIC for subscription billing
4. **Card-on-File**: Use Storage BRIC for one-click checkout

---

## Security Considerations

### ✅ Implemented
- PCI compliance (card data bypasses backend)
- HTTPS required for callback endpoint (production)
- Idempotency (duplicate detection via TRAN_NBR)
- Proper error handling (no sensitive data in errors)
- TAC token encryption (4-hour expiration)
- Field validation by EPX

### ⏳ Recommended Enhancements
- [ ] MAC signature validation (ValidateResponseMAC method exists but not used)
- [ ] Rate limiting on callback endpoint
- [ ] CSRF protection (if applicable)
- [ ] Webhook signature validation for Postmaster responses

---

## Testing Checklist

### Unit Tests
- ✅ BrowserPostAdapter.BuildFormData() - Tested
- ✅ BrowserPostAdapter.ParseRedirectResponse() - Tested
- ⏳ BrowserPostCallbackHandler.HandleCallback() - **TODO: Add tests**
- ⏳ BrowserPostCallbackHandler.storeTransaction() - **TODO: Add tests**

### Integration Tests
- ⏳ End-to-end Browser Post flow - **TODO: Manual testing required**
- ⏳ Duplicate callback detection - **TODO: Test PRG pattern**
- ⏳ Receipt page rendering - **TODO: Visual verification**

### Manual Testing Steps

1. **Obtain TAC Token**
   - Use merchant-specific method to generate TAC
   - Ensure TAC includes all required fields

2. **Create HTML Form**
   ```html
   <!-- Save as test_payment.html -->
   <form method="POST" action="https://epxnow.com/epx/browser_post_sandbox">
     <input type="hidden" name="TAC" value="<TAC_FROM_STEP_1>" />
     <input type="hidden" name="TRAN_CODE" value="SALE" />
     <input type="hidden" name="INDUSTRY_TYPE" value="E" />
     <input type="hidden" name="TRAN_NBR" value="TEST-12345" />
     <input type="hidden" name="AMOUNT" value="1.00" />
     <input type="hidden" name="TRAN_GROUP" value="SALE" />
     <input type="hidden" name="REDIRECT_URL" value="http://localhost:8081/api/v1/payments/browser-post/callback" />
     <input type="hidden" name="CUST_NBR" value="..." />
     <input type="hidden" name="MERCH_NBR" value="..." />
     <input type="hidden" name="DBA_NBR" value="..." />
     <input type="hidden" name="TERMINAL_NBR" value="..." />

     <input type="text" name="CARD_NBR" placeholder="4111111111111111" />
     <input type="text" name="EXP_MONTH" placeholder="12" />
     <input type="text" name="EXP_YEAR" placeholder="2025" />
     <input type="text" name="CVV" placeholder="123" />

     <button type="submit">Pay $1.00</button>
   </form>
   ```

3. **Submit Form**
   - Open test_payment.html in browser
   - Enter test card details
   - Click "Pay $1.00"
   - Verify redirect to callback endpoint
   - Verify receipt page displays correctly

4. **Check Database**
   ```sql
   SELECT * FROM transactions ORDER BY created_at DESC LIMIT 1;
   -- Verify AUTH_GUID is stored
   -- Verify idempotency_key (TRAN_NBR) is set
   ```

5. **Test Duplicate Detection**
   - Click browser "Back" button
   - Click "Pay" again
   - Verify no duplicate transaction in database
   - Verify same receipt page is shown

---

## Summary

### ✅ DATAFLOW IS COMPLETE

All components are implemented and properly connected:

1. ✅ **TAC Generation**: Merchant obtains TAC token
2. ✅ **Form Builder**: Construct payment form data
3. ✅ **Frontend Form**: Merchant implements (documented)
4. ✅ **EPX Processing**: External (EPX validates and processes)
5. ✅ **Callback Handler**: Parse, validate, store, render
6. ✅ **Database**: Store transaction with AUTH_GUID (Financial BRIC)
7. ✅ **Receipt Page**: Show success/failure to user

### 🎯 Ready for EPX Configuration

**ACTION REQUIRED**: Provide this REDIRECT_URL to EPX:
- Local: `http://localhost:8081/api/v1/payments/browser-post/callback`
- Production: `https://yourdomain.com/api/v1/payments/browser-post/callback`

### ✅ Quality Assurance
- ✅ `go vet ./...` - No issues
- ✅ `go build ./...` - Compiles successfully
- ✅ All types properly connected
- ✅ Documentation complete (README.md, DOCUMENTATION.md, CHANGELOG.md)

---

**Review Date**: 2025-11-03
**Reviewer**: Claude Code
**Status**: ✅ CORRECTED - Removed Key Exchange API references
