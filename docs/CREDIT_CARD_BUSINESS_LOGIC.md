# Credit Card Payment Operations - Complete Business Logic Documentation

**Audience:** Backend developers implementing credit card payment handlers and service layer
**Topic:** Business flows, data operations, and state transitions for credit card payment operations
**Goal:** Provide complete understanding of credit card storage flows to enable service handler implementation

## Quick Overview

Credit card payment method storage enables recurring payments and subscriptions with these characteristics:

| Feature | Details |
|---------|---------|
| **PCI Compliance** | Browser Post keeps card data out of merchant scope |
| **Storage** | Financial BRIC (13 months) vs Storage BRIC (never expires) |
| **Verification** | $0.00 Account Verification required for card-on-file |
| **Card Networks** | Visa, Mastercard, Amex, Discover requirements |
| **Settlement** | Real-time authorization, 1-2 day settlement |

---

## 1. Browser Post Credit Card Flow

### Purpose
Customer enters card in EPX-hosted form, card data never touches merchant server (PCI compliant).

### Complete Process

**Step 1: Generate TAC (Terminal Authorization Code)**
```go
// Call EPX Key Exchange service
keyExchangeReq := &ports.KeyExchangeRequest{
    MAC: merchantMAC,
    TranNbr: transactionID,
    TranGroup: "SALE",
    Amount: "29.99",
    RedirectURL: "https://merchant.com/callback",
}

tacResp := keyExchange.GetTAC(ctx, keyExchangeReq)
// Returns: TAC token (encrypted, expires in 4 hours)
```

**Step 2: Create Pending Transaction**
```sql
INSERT INTO transactions (
    id, merchant_id, customer_id, amount_cents,
    type, payment_method_type, created_at
) VALUES (
    $transaction_id,    -- Frontend-generated UUID
    $merchant_id,
    $customer_id,
    2999,               -- $29.99
    'SALE',
    'credit_card',
    NOW()
);
-- Status auto-generated as 'pending' (auth_resp=NULL)
```

**Step 3: Return Form Configuration**
```json
{
  "tac": "FEU86Eo5/S1CZwZemrks...",
  "epx_post_url": "https://secure.epxuap.com/browserpost",
  "merchant_credentials": {
    "cust_nbr": "9001",
    "merch_nbr": "900300",
    "dba_nbr": "2",
    "terminal_nbr": "77"
  },
  "transaction": {
    "tran_code": "U",      // SALE
    "amount": "29.99",
    "tran_nbr": "12345",
    "industry_type": "E"
  },
  "redirect_url": "https://merchant.com/callback?transaction_id=xxx"
}
```

**Step 4: Customer Submits Form**
```html
<form action="https://secure.epxuap.com/browserpost" method="POST">
  <input type="hidden" name="TAC" value="FEU86Eo5..." />
  <input type="hidden" name="CUST_NBR" value="9001" />
  <input type="hidden" name="TRAN_CODE" value="U" />
  <input type="hidden" name="AMOUNT" value="29.99" />

  <!-- Customer enters these -->
  <input name="CARD_NUMBER" placeholder="4111 1111 1111 1111" />
  <input name="EXP_DATE" placeholder="12/25" />
  <input name="CVV2" placeholder="123" />
  <input name="ZIP_CODE" placeholder="10001" />

  <button type="submit">Pay</button>
</form>
```

**Step 5: EPX Processes Transaction**
- Validates TAC (ensures no tampering)
- Validates card details (CVV, expiration)
- Sends to card network for authorization
- Receives auth response from issuer

**Step 6: EPX Redirects to Callback**
```
POST https://merchant.com/callback?transaction_id=xxx

AUTH_GUID=09LMQAABBCCDD&          // Financial BRIC (13-month expiry)
AUTH_RESP=00&                      // Approved
AUTH_CODE=123456&                  // Bank authorization code
AUTH_RESP_TEXT=APPROVED&
AUTH_CARD_TYPE=V&                  // Visa
AUTH_AVS=Y&                        // Address verified
AUTH_CVV2=M&                       // CVV matched
AMOUNT=29.99&
TRAN_NBR=12345
```

**Step 7: Backend Updates Transaction**
```sql
UPDATE transactions SET
    auth_resp = '00',
    auth_guid = '09LMQAABBCCDD',
    auth_code = '123456',
    processed_at = NOW(),
    metadata = jsonb_build_object(
        'auth_resp_text', 'APPROVED',
        'card_type', 'V',
        'avs_result', 'Y',
        'cvv_result', 'M',
        'last_four', '1111'
    )
WHERE id = $transaction_id;
-- Status auto-generated as 'approved'
```

**Step 8: Optionally Save Payment Method**
If customer requested to save card (`USER_DATA_2='save_card'`):
- Convert Financial BRIC to Storage BRIC (see Section 3)
- Save to `customer_payment_methods` table

### Key Points
- Card data never seen by merchant server (PCI out of scope)
- TAC prevents form tampering
- POST-REDIRECT-GET prevents duplicate submissions
- Financial BRIC expires in 13 months
- Can convert to Storage BRIC for recurring use

---

## 2. StoreCreditCard Flow (Post-Payment Conversion)

### Purpose
Save customer's payment method after successful transaction for recurring use.

### Process

**Step 1: Check If Save Requested**
```go
// In Browser Post callback handler
shouldSave := response.UserData2 == "save_card"
if !shouldSave || !response.IsApproved {
    return // Don't save if not requested or declined
}
```

**Step 2: Convert Financial BRIC to Storage BRIC**
Call `ConvertFinancialBRICToStorageBRIC` (see Section 3 for details)

**Step 3: Save to Database**
```sql
INSERT INTO customer_payment_methods (
    id, merchant_id, customer_id, bric,
    payment_type, last_four, card_brand,
    card_exp_month, card_exp_year,
    is_verified, is_active, is_default,
    metadata, created_at, updated_at
) VALUES (
    gen_random_uuid(),
    $merchant_id,
    $customer_id,
    $storage_bric,          -- Never expires
    'credit_card',
    '1111',                 -- Last 4 digits
    'Visa',                 -- Card brand
    12, 2025,               -- Expiration
    true,                   -- Verified via Account Verification
    true,                   -- Active
    false,                  -- Not default
    jsonb_build_object(
        'ntid', $network_transaction_id,
        'avs_result', 'Y',
        'cvv_result', 'M'
    ),
    NOW(), NOW()
);
```

**Step 4: Return Payment Method**
```protobuf
message PaymentMethod {
    string id = 1;                  // UUID
    string payment_type = 2;        // "credit_card"
    string last_four = 3;           // "1111"
    string card_brand = 4;          // "Visa"
    int32 card_exp_month = 5;       // 12
    int32 card_exp_year = 6;        // 2025
    bool is_verified = 7;           // true
    bool is_active = 8;             // true
}
```

### Security Notes
- Never store full card number
- Never store CVV (prohibited by PCI DSS)
- Store only: last 4, brand, expiration, Storage BRIC
- Storage BRIC is tokenized and secure

---

## 3. ConvertFinancialBRICToStorageBRIC Flow

### Purpose
Convert a temporary Financial BRIC (from any transaction) to a permanent Storage BRIC.

### Process

**Step 1: Validate Financial BRIC**
```go
if financialBRIC == "" {
    return ErrFinancialBRICRequired
}

// Financial BRIC format: 19-20 characters alphanumeric
if !isValidBRICFormat(financialBRIC) {
    return ErrInvalidBRICFormat
}
```

**Step 2: Send BRIC Storage Request (CCE8)**
```xml
<DETAIL cust_nbr="9001" merch_nbr="900300" dba_nbr="2" terminal_nbr="77">
    <TRAN_TYPE>CCE8</TRAN_TYPE>
    <BATCH_ID>20250119</BATCH_ID>
    <TRAN_NBR>12346</TRAN_NBR>
    <ORIG_AUTH_GUID>09LMQAABBCCDD</ORIG_AUTH_GUID>  <!-- Financial BRIC -->
    <CARD_ENT_METH>Z</CARD_ENT_METH>                <!-- BRIC-based -->

    <!-- Required for Account Verification -->
    <ADDRESS>123 Main Street</ADDRESS>
    <CITY>New York</CITY>
    <STATE>NY</STATE>
    <ZIP_CODE>10001</ZIP_CODE>
    <FIRST_NAME>John</FIRST_NAME>
    <LAST_NAME>Doe</LAST_NAME>
</DETAIL>
```

**Step 3: EPX Performs Account Verification**
- EPX sends $0.00 authorization to card network (CCE0 internally)
- Issuer validates: Card active, address matches, CVV matches
- Issuer assigns Network Transaction ID (NTID)
- Issuer returns approval/decline

**Step 4: Parse Response**
```xml
<RESPONSE>
  <FIELD KEY="AUTH_GUID">09XYZSTORAGETOKEN</FIELD>  <!-- Storage BRIC -->
  <FIELD KEY="AUTH_RESP">00</FIELD>                 <!-- Approved -->
  <FIELD KEY="AUTH_RESP_TEXT">APPROVED</FIELD>
  <FIELD KEY="AUTH_AVS">Y</FIELD>                   <!-- Address match -->
  <FIELD KEY="AUTH_CVV2">M</FIELD>                  <!-- CVV match -->
  <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>             <!-- Visa -->
  <FIELD KEY="AUTH_TRAN_IDENT">1234567890123456</FIELD>  <!-- NTID -->
</RESPONSE>
```

**Step 5: Validate Response**
```go
if response.AuthResp != "00" && response.AuthResp != "85" {
    // Account Verification declined
    return fmt.Errorf("verification failed: %s", response.AuthRespText)
}

storageBRIC := response.AuthGUID
ntid := response.NetworkTransactionID
```

**Step 6: Return Storage BRIC**
- Storage BRIC: Never expires
- NTID: Required for card-on-file compliance
- AVS/CVV results: Stored for fraud analysis

### Important Notes
- Billing address REQUIRED for Account Verification
- AVS match improves approval rate
- NTID must be stored for recurring transactions
- Original Financial BRIC still valid (13 months)
- Storage BRIC is separate, independent token

---

## 4. Account Verification (CCE0) Flow

### Purpose
$0.00 authorization to verify card validity and obtain Network Transaction ID for card-on-file.

### When Used
- During Storage BRIC creation (CCE8 automatically triggers CCE0)
- Validating saved cards before recurring charge
- Annual verification for dormant cards

### Process

**EPX Request (Internal to CCE8)**
```xml
<DETAIL>
    <TRAN_TYPE>CCE0</TRAN_TYPE>
    <AMOUNT>0.00</AMOUNT>
    <ACCOUNT_NBR>4111111111111111</ACCOUNT_NBR>
    <EXP_DATE>1225</EXP_DATE>
    <CVV2>123</CVV2>
    <ADDRESS>123 Main St</ADDRESS>
    <ZIP_CODE>10001</ZIP_CODE>
    <CARD_ENT_METH>E</CARD_ENT_METH>
</DETAIL>
```

**Issuer Response**
```xml
<RESPONSE>
  <FIELD KEY="AUTH_RESP">00</FIELD>
  <FIELD KEY="AUTH_AVS">Y</FIELD>   <!-- Address match -->
  <FIELD KEY="AUTH_CVV2">M</FIELD>  <!-- CVV match -->
  <FIELD KEY="AUTH_TRAN_IDENT">1234567890123456</FIELD>  <!-- NTID -->
</RESPONSE>
```

### Response Codes

**AUTH_RESP:**
| Code | Meaning | Action |
|------|---------|--------|
| `00` | Approved | Create Storage BRIC |
| `85` | Not declined (treat as approval) | Create Storage BRIC |
| `05` | Do not honor | Card restricted/closed |
| `14` | Invalid card number | User must correct |
| `51` | Insufficient funds | Rare for $0.00 |

**AUTH_AVS (Address Verification):**
| Code | Meaning | Recommendation |
|------|---------|----------------|
| `Y` | Address and ZIP match | Accept |
| `Z` | ZIP matches only | Accept |
| `A` | Address matches only | Review |
| `N` | No match | Decline (fraud risk) |
| `U` | Unavailable | Accept with caution |

**AUTH_CVV2:**
| Code | Meaning | Recommendation |
|------|---------|----------------|
| `M` | Match | Accept |
| `N` | No match | Decline (fraud risk) |
| `P` | Not processed | Accept with caution |
| `U` | Unavailable | Accept with caution |

### Card-on-File Compliance
- NTID proves Account Verification was performed
- NTID must be provided in all recurring transactions
- NTID links to original authorization date
- Reduces chargebacks (issuer has record)

---

## 5. Direct Storage BRIC Creation (CCE8 from Raw Card)

### Purpose
Create Storage BRIC without initial transaction (customer adds card for future use).

### When to Use
- Subscription setup before first charge
- Wallet/card vault implementation
- Pre-authorized payment setup

### Process

**Step 1: Customer Provides Card Details**
```go
type StoreCreditCardRequest struct {
    MerchantID   string
    CustomerID   string
    CardNumber   string  // PCI-scoped
    ExpirationMM int     // 1-12
    ExpirationYY int     // 2025, 2026, etc.
    CVV          string  // 3-4 digits

    // Billing address
    FirstName string
    LastName  string
    Address   string
    City      string
    State     string
    ZipCode   string
}
```

**Step 2: Validate Card Details**
```go
// Luhn algorithm check
if !isValidCardNumber(req.CardNumber) {
    return ErrInvalidCardNumber
}

// Expiration check
if isExpired(req.ExpirationMM, req.ExpirationYY) {
    return ErrCardExpired
}

// CVV format check
if !isValidCVV(req.CVV, cardBrand) {
    return ErrInvalidCVV
}
```

**Step 3: Send BRIC Storage Request**
```xml
<DETAIL cust_nbr="9001" merch_nbr="900300" dba_nbr="2" terminal_nbr="77">
    <TRAN_TYPE>CCE8</TRAN_TYPE>
    <BATCH_ID>20250119</BATCH_ID>
    <TRAN_NBR>12347</TRAN_NBR>

    <!-- Raw card details -->
    <ACCOUNT_NBR>4111111111111111</ACCOUNT_NBR>
    <EXP_DATE>1225</EXP_DATE>
    <CVV2>123</CVV2>
    <CARD_ENT_METH>E</CARD_ENT_METH>  <!-- Ecommerce (not BRIC) -->

    <!-- Billing information -->
    <FIRST_NAME>John</FIRST_NAME>
    <LAST_NAME>Doe</LAST_NAME>
    <ADDRESS>123 Main Street</ADDRESS>
    <CITY>New York</CITY>
    <STATE>NY</STATE>
    <ZIP_CODE>10001</ZIP_CODE>
</DETAIL>
```

**Step 4: EPX Performs Account Verification**
- $0.00 auth sent to card network
- Issuer validates card and address
- NTID assigned

**Step 5: Clear Sensitive Data**
```go
// After EPX request, immediately clear
req.CardNumber = ""
req.CVV = ""

// Only store:
storageBRIC := response.AuthGUID
lastFour := cardNumber[len(cardNumber)-4:]
```

**Step 6: Save Payment Method**
```sql
INSERT INTO customer_payment_methods (
    id, merchant_id, customer_id, bric,
    payment_type, last_four, card_brand,
    card_exp_month, card_exp_year,
    is_verified, metadata
) VALUES (
    gen_random_uuid(),
    $merchant_id,
    $customer_id,
    $storage_bric,
    'credit_card',
    $last_four,
    $card_brand,
    $exp_month,
    $exp_year,
    true,
    jsonb_build_object('ntid', $ntid)
);
```

### PCI Compliance Notes
- Card number and CVV NEVER stored
- Card data only in memory during EPX request
- Use HTTPS/TLS 1.2+ for transmission
- Clear sensitive data immediately after use
- Only Storage BRIC persisted to database

---

## 6. PCI Compliance & Security

### PCI DSS Scope

**Out of Scope (Browser Post):**
- Card data in browser (EPX handles via hosted form)
- Card data in callback (only last 4 digits in metadata)
- Customer-entered card details never touch merchant server

**In Scope (Direct Storage BRIC):**
- Card data in memory during EPX request
- TLS/HTTPS transmission
- Immediate clearing of sensitive data
- No logging of card numbers or CVV

**Storage Requirements:**
- ✅ Store: Storage BRIC, last 4 digits, expiration, brand
- ❌ Never store: Full card number, CVV, track data

### Network Transaction ID (NTID)

**Purpose:**
- Proves Account Verification was performed
- Required for card-on-file transactions
- Reduces chargeback disputes
- Links to original authorization date

**Storage:**
```sql
UPDATE customer_payment_methods
SET metadata = metadata || jsonb_build_object('ntid', $ntid)
WHERE id = $payment_method_id;
```

**Usage in Recurring Transactions:**
```xml
<DETAIL>
    <TRAN_TYPE>CCE1</TRAN_TYPE>
    <AUTH_GUID>[Storage BRIC]</AUTH_GUID>
    <ORIG_AUTH_TRAN_IDENT>[NTID]</ORIG_AUTH_TRAN_IDENT>
    <ACI_EXT>RB</ACI_EXT>  <!-- Recurring Billing -->
</DETAIL>
```

### Card Network Requirements

**Visa:**
- ACI indicator: "RB" (Recurring Billing)
- Account Verification required
- NTID required in subsequent transactions
- COF_PERIOD specifies validity (months)

**Mastercard:**
- CUC (Card Unique Reference) assigned
- Account Verification required
- Mastercard Transaction Identifier required
- MOC indicator for card-on-file

**American Express:**
- Direct Agreement Merchant (DAM) codes
- Account Verification required
- Amex Reference Number in response
- Company ID required

**Discover:**
- Same as Visa/Mastercard
- Account Verification required
- Network Reference ID required

---

## 7. Browser Post vs Server Post Trade-offs

| Aspect | Browser Post | Server Post |
|--------|-------------|------------|
| **PCI Scope** | Out of scope ✅ | Full compliance required ❌ |
| **Card Data Flow** | Browser → EPX only | Merchant → EPX |
| **User Experience** | Hosted form (EPX UI) | Custom UI ✅ |
| **Implementation** | Simple (days) ✅ | Complex (weeks) |
| **Fraud Protection** | EPX handles ✅ | Merchant responsible |
| **Liability** | EPX bears risk | Merchant bears risk |
| **Customization** | Limited | Full control ✅ |
| **Recurring** | Via conversion | Direct Storage BRIC ✅ |

### When to Use Browser Post
- ✅ Want PCI compliance out-of-scope
- ✅ Quick integration timeline
- ✅ Accept UI limitations
- ✅ Lower development cost

### When to Use Server Post
- ✅ Need full UI control
- ✅ Have PCI compliance team
- ✅ Complex payment workflows
- ✅ Direct BRIC creation

---

## 8. Financial BRIC vs Storage BRIC

| Aspect | Financial BRIC | Storage BRIC |
|--------|----------------|--------------|
| **Source** | Transaction (CCE1/CCE2) | Conversion (CCE8) |
| **Lifetime** | 13 months | Never expires ✅ |
| **Verification** | Not required | Account Verification ✅ |
| **Use Cases** | One-time, refunds | Recurring, subscriptions |
| **Compliance** | AUTH_GUID only | NTID required ✅ |
| **Conversion** | Can convert → Storage | Cannot convert further |
| **Expiration** | Expires | Never expires |

### When to Use Each

**Financial BRIC:**
- One-time guest checkout
- Capture/void/refund original transaction
- Short-term use (< 13 months)

**Storage BRIC:**
- Recurring subscriptions
- Card-on-file payments
- Long-term customer relationships
- Compliance with card network rules

---

## 9. State Machine & Status Transitions

### Payment Method Lifecycle

```
Created (is_verified=false)
    ↓
Account Verification sent
    ├─→ Approved (AUTH_RESP='00')
    │   └─→ is_verified=true, is_active=true
    │       └─→ Can be used for payments
    │
    └─→ Declined (AUTH_RESP='05', '14')
        └─→ is_verified=false
            └─→ Cannot be used

Active (is_active=true)
    └─→ Can process transactions

Deactivated (is_active=false)
    └─→ Cannot process new transactions

Deleted (deleted_at != NULL)
    └─→ Soft deleted, not visible
```

### Transaction Hierarchy

```
Standalone
    ├─ SALE (CCE1)
    │   ├─→ REFUND (CCE9)
    │   └─→ VOID (CCEX)
    │
    └─ AUTH (CCE2)
        ├─→ CAPTURE (CCE4)
        ├─→ REFUND (CCE9)
        └─→ VOID (CCEX)

Recurring (using Storage BRIC)
    └─ SALE (CCE1 + NTID + ACI_EXT=RB)
```

---

## 10. Error Handling

### Retryable Errors

| Code | Description | Retry Strategy |
|------|-------------|----------------|
| `51` | Insufficient funds | Retry after 60s |
| `61` | Exceeds daily limit | Retry next day |
| `91` | Issuer unavailable | Exponential backoff |

### Non-Retryable Errors

| Code | Description | User Action |
|------|-------------|-------------|
| `05` | Do not honor | Use different card |
| `14` | Invalid card | Correct card number |
| `41` | Lost card | Card reported lost |
| `43` | Stolen card | Card reported stolen |

---

## 11. Idempotency & Duplicate Prevention

### Problem
User clicks Back/Refresh → Duplicate transaction submissions

### Solution: POST-REDIRECT-GET Pattern

```
1. User submits form → POST to EPX
2. EPX processes transaction
3. EPX redirects to EPX response page
4. Response page redirects to merchant callback
5. User clicks Back → Goes to response page (not POST)
6. No duplicate transaction
```

### Backend Implementation

```go
// Check if already processed
existing, _ := db.GetTransactionByID(ctx, transactionID)
if existing != nil {
    // Return cached result
    return redirectToService(w, existing)
}

// Process new transaction
// ...
```

---

## 12. Implementation Examples

### ConvertFinancialBRICToStorageBRIC Handler

```go
func (s *PaymentMethodService) ConvertFinancialBRICToStorageBRIC(
    ctx context.Context,
    req *ports.ConvertFinancialBRICRequest,
) (*domain.PaymentMethod, error) {

    // 1. Validate Financial BRIC
    if req.FinancialBRIC == "" {
        return nil, ErrFinancialBRICRequired
    }

    // 2. Build BRIC Storage request
    bricReq := &ports.BRICStorageRequest{
        CustNbr: merchant.CustNbr,
        MerchNbr: merchant.MerchNbr,
        DBAnbr: merchant.DBAnbr,
        TerminalNbr: merchant.TerminalNbr,
        PaymentType: ports.PaymentMethodTypeCreditCard,
        FinancialBRIC: &req.FinancialBRIC,
        FirstName: req.FirstName,
        LastName: req.LastName,
        Address: req.Address,
        City: req.City,
        State: req.State,
        ZipCode: req.ZipCode,
    }

    // 3. Call BRIC Storage API (CCE8)
    resp, err := s.bricStorage.ConvertFinancialBRICToStorage(ctx, bricReq)
    if err != nil {
        return nil, fmt.Errorf("BRIC conversion failed: %w", err)
    }

    if !resp.IsApproved {
        return nil, fmt.Errorf("account verification declined: %s", resp.AuthRespText)
    }

    // 4. Save to database
    pm := &domain.PaymentMethod{
        ID: uuid.New().String(),
        MerchantID: req.MerchantID,
        CustomerID: req.CustomerID,
        PaymentToken: resp.StorageBRIC,
        PaymentType: "credit_card",
        LastFour: req.LastFour,
        CardBrand: req.CardBrand,
        CardExpMonth: req.CardExpMonth,
        CardExpYear: req.CardExpYear,
        IsVerified: true,
        Metadata: map[string]interface{}{
            "ntid": resp.NetworkTransactionID,
            "avs_result": resp.AuthAVS,
            "cvv_result": resp.AuthCVV2,
        },
    }

    err = s.db.CreatePaymentMethod(ctx, pm)
    if err != nil {
        return nil, fmt.Errorf("failed to save payment method: %w", err)
    }

    return pm, nil
}
```

---

## Summary

### Quick Reference Table

| Operation | Input | EPX Type | Verification | Output |
|-----------|-------|----------|--------------|--------|
| **Browser Post** | Card in form | CCE1/CCE2 | Real-time auth | Financial BRIC |
| **StoreCreditCard** | Financial BRIC | CCE8 | Account Verification | Storage BRIC + NTID |
| **Direct BRIC** | Raw card details | CCE8 | Account Verification | Storage BRIC + NTID |

### Key Takeaways

1. **Browser Post keeps you PCI compliant** - Card data never touches your server
2. **Financial BRIC expires** - Convert to Storage BRIC for recurring use
3. **Account Verification required** - $0.00 auth obtains NTID for compliance
4. **NTID is mandatory** - Required for all card-on-file transactions
5. **Never store card numbers or CVV** - Only Storage BRIC + metadata

---

This comprehensive documentation covers all credit card payment method storage flows, from Browser Post integration to Storage BRIC creation, with detailed PCI compliance and card network requirements.
