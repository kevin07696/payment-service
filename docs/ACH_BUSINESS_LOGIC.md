# ACH Payment Operations - Complete Business Logic Documentation

**Audience:** Backend developers implementing ACH payment handlers and service layer
**Topic:** Business flows, data operations, and state transitions for ACH payment operations
**Goal:** Provide complete understanding of ACH payment flows to enable service handler implementation

## Quick Overview

ACH (Automated Clearing House) enables bank account transfers with these characteristics:

| Feature | Details |
|---------|---------|
| **Settlement** | 1-3 business days (not real-time) |
| **Costs** | Flat fee ($0.50-$1.00) vs credit card % fees (2-3%) |
| **Use Cases** | Subscriptions, invoice payments, payouts |
| **Verification** | Pre-note required before recurring charges |
| **Returns** | Can be disputed within 2 business days after settlement |

---

## 1. StoreACHAccount Flow

### Purpose
Creates an ACH Storage BRIC (persistent token) by verifying the account via pre-note transaction.

### Complete Process

**Step 1: Validate Request**
- Account number is numeric and proper length (typically 10-12 digits)
- Routing number is valid ABA routing number (9 digits)
- Account holder name provided
- Account type: CHECKING or SAVINGS

**Step 2: Check Idempotency**
- Look up existing payment method by routing + last 4 digits
- If found with same merchant/customer: return existing method

**Step 3: Send Pre-Note Debit (CKC0)**
```xml
<DETAIL>
    <TRAN_TYPE>CKC0</TRAN_TYPE>
    <AMOUNT>0.00</AMOUNT>
    <ACCOUNT_NBR>1234567890</ACCOUNT_NBR>
    <ROUTING_NBR>021000021</ROUTING_NBR>
    <CARD_ENT_METH>X</CARD_ENT_METH>
    <RECEIVER_NAME>John Doe</RECEIVER_NAME>
    <STD_ENTRY_CLASS>PPD</STD_ENTRY_CLASS>
    <RECEIVER_TYPE_CODE>0</RECEIVER_TYPE_CODE>
</DETAIL>
```
- Receives: Financial BRIC from EPX
- AUTH_RESP must be "00" (accepted)

**Step 4: Convert Financial BRIC to Storage BRIC (CKC8)**
```xml
<DETAIL>
    <TRAN_TYPE>CKC8</TRAN_TYPE>
    <ORIG_AUTH_GUID>[Financial BRIC]</ORIG_AUTH_GUID>
    <CARD_ENT_METH>Z</CARD_ENT_METH>
</DETAIL>
```
- Receives: Storage BRIC (never expires)

**Step 5: Store in Database**
```sql
INSERT INTO customer_payment_methods (
    id, merchant_id, customer_id, bric, payment_type,
    last_four, bank_name, account_type, is_verified,
    created_at, updated_at
) VALUES (
    gen_random_uuid(), $1, $2, '[Storage BRIC]',
    'ach', '7890', 'Chase', 'checking', true,
    NOW(), NOW()
);
```

**Step 6: Return Payment Method**
```protobuf
message PaymentMethod {
    string id = 1;                  // UUID
    string payment_type = 2;        // "ach"
    string last_four = 3;           // "7890"
    string bank_name = 4;           // "Chase"
    string account_type = 5;        // "checking"
    bool is_verified = 6;           // true
}
```

### Key Points
- Pre-note appears as $0.00 on bank statement
- Account verification happens within 1-3 business days
- Storage BRIC never expires (unlike Financial BRIC with 13-month expiry)
- `is_verified = true` only after pre-note succeeds
- All ACH debits MUST use verified payment methods

---

## 2. ACHDebit Flow

### Purpose
Pull money from customer's bank account for payments/subscriptions.

### Process

**Step 1: Validate Request**
- Amount > 0
- Amount <= daily limit (typically $25,000)
- Payment method exists and is ACH
- Payment method is verified (`is_verified = true`)

**Step 2: Create Debit Transaction (status=pending)**
```sql
INSERT INTO transactions (
    id, merchant_id, customer_id, amount_cents, type,
    payment_method_type, payment_method_id, auth_resp,
    processed_at, created_at, updated_at
) VALUES (
    $1::uuid, $2::uuid, $3::uuid, 15000, 'DEBIT',
    'ach', $4::uuid, NULL, NULL, NOW(), NOW()
);
-- Status auto-generated as 'pending'
```

**Step 3: Send Debit to EPX (CKC2)**
```xml
<DETAIL>
    <TRAN_TYPE>CKC2</TRAN_TYPE>
    <AMOUNT>150.00</AMOUNT>
    <AUTH_GUID>[Storage BRIC]</AUTH_GUID>
    <CARD_ENT_METH>Z</CARD_ENT_METH>
    <INDUSTRY_TYPE>E</INDUSTRY_TYPE>
</DETAIL>
```
- Response: Financial BRIC, AUTH_RESP ('00' = accepted)

**Step 4: Update Transaction with Response**
```sql
UPDATE transactions SET
    auth_resp = $1,              -- '00' or error code
    auth_guid = $2,              -- Financial BRIC
    processed_at = NOW(),
    metadata = jsonb_build_object('auth_resp_text', $3)
WHERE id = $4;
-- Status auto-generated from auth_resp
```

**Step 5: Update Payment Method Last Used**
```sql
UPDATE customer_payment_methods SET last_used_at = NOW()
WHERE id = $1;
```

### Status Transitions
- Created: `pending` (auth_resp = NULL)
- Response "00": `approved` → Funds scheduled for settlement (1-3 days)
- Response other: `declined` → Transaction failed
- Failed before EPX: `failed` (processed_at set but auth_resp NULL)

### Settlement Timeline
```
Day 0: EPX accepts, returns "00" (ACH ACCEPTED)
Day 1-2: ACH network processes batch
Day 2-3: Customer's bank debits account
```

### Returns (Customer Disputes)
Customer can dispute within 2 business days of settlement:
- Return code "R01": Insufficient funds
- Return code "R02": Account closed
- Return code "R03": Routing number invalid
- Return code "R04": Stop payment requested
- Return code "R05": Unauthorized transaction

Application response:
1. Update transaction metadata with return code
2. Notify customer
3. Mark payment method with return count
4. After 2+ returns: disable payment method

---

## 3. ACHCredit Flow

### Purpose
Send money to customer's bank account (refunds, payouts, vendor payments).

### Process

**Step 1: Validate Request**
- Amount > 0
- Payment method exists and is ACH
- Payment method is verified

**Step 2: Create Credit Transaction**
```sql
-- For simple payout (no parent)
INSERT INTO transactions (
    id, parent_transaction_id, merchant_id, customer_id,
    amount_cents, type, payment_method_type,
    payment_method_id, created_at, updated_at
) VALUES (
    $1, NULL, $2, $3, $4, 'DEBIT', 'ach', $5, NOW(), NOW()
);

-- For refund (has parent)
INSERT INTO transactions (
    id, parent_transaction_id, merchant_id, customer_id,
    amount_cents, type, payment_method_type,
    payment_method_id, created_at, updated_at
) VALUES (
    $1, $6, $2, $3, $4, 'REFUND', 'ach', $5, NOW(), NOW()
);
```

**Step 3: Send Credit to EPX (CKC3)**
```xml
<DETAIL>
    <TRAN_TYPE>CKC3</TRAN_TYPE>
    <AMOUNT>50.00</AMOUNT>
    <AUTH_GUID>[Storage BRIC]</AUTH_GUID>
    <CARD_ENT_METH>Z</CARD_ENT_METH>
</DETAIL>
```

**Step 4: Update Transaction with Response**
```sql
UPDATE transactions SET
    auth_resp = $1,
    auth_guid = $2,
    processed_at = NOW()
WHERE id = $3;
```

### Status Transitions
Same as ACHDebit:
- pending → approved (auth_resp='00') → Customer receives funds in 1-3 days
- pending → declined (auth_resp != '00')

---

## 4. ACHVoid Flow

### Purpose
Cancel an unsettled ACH debit transaction before it settles.

**IMPORTANT:** Only works on same-day transactions before settlement. Once settled (typically next business day), must use ACHCredit instead.

### Process

**Step 1: Validate Request**
- Transaction exists
- Transaction type is DEBIT (can only void debits)
- Transaction not already voided
- Transaction is still pending or approved (not too old)

**Step 2: Create Void Transaction**
```sql
INSERT INTO transactions (
    id, parent_transaction_id, merchant_id, customer_id,
    amount_cents, type, payment_method_type,
    payment_method_id, created_at, updated_at
) VALUES (
    $1, $2::uuid,           -- parent_transaction_id
    $3, $4, $5,
    'VOID',                 -- Type must be VOID
    'ach', $6,
    NOW(), NOW()
);
-- Must have parent (database constraint enforces this)
```

**Step 3: Send Void to EPX (CKCX)**
```xml
<DETAIL>
    <TRAN_TYPE>CKCX</TRAN_TYPE>
    <AMOUNT>150.00</AMOUNT>         <!-- Must match original -->
    <ORIG_AUTH_GUID>[Debit BRIC]</ORIG_AUTH_GUID>
    <CARD_ENT_METH>Z</CARD_ENT_METH>
</DETAIL>
```

**Step 4: Update Void Transaction**
```sql
UPDATE transactions SET
    auth_resp = $1,              -- '00' = void successful
    auth_guid = $2,
    processed_at = NOW()
WHERE id = $3;
```

### Error Scenarios
- **TRANSACTION_ALREADY_SETTLED**: Amount debited to customer account → Too late, use ACHCredit
- **TRANSACTION_ALREADY_VOIDED**: Already cancelled
- **AMOUNT_MISMATCH**: Amount doesn't match original debit
- **WRONG_TRANSACTION_TYPE**: Not a DEBIT (can't void REFUND, VOID, etc.)

---

## 5. Pre-note Verification

### What is Pre-note?

Pre-note is a **$0.00 ACH transaction** that verifies an account exists before allowing recurring charges.

**NACHA Requirement:** Cannot debit a consumer account with recurring charges without prior authorization and account verification via pre-note.

### How StoreACHAccount Uses Pre-note

1. **Automatic:** Pre-note sent automatically in StoreACHAccount
2. **Verification:** EPX submits to ACH network
3. **Bank Check:** Bank verifies account exists (no funds transferred)
4. **Timeline:** Clears in 1-3 business days
5. **Result:** If auth_resp='00', account marked as verified

### Pre-note Response Codes

| Code | Meaning | Action |
|------|---------|--------|
| `00` | Account verified ✓ | Create Storage BRIC |
| `05` | Account closed/restricted | Return error, user must fix |
| `14` | Invalid routing number | Return validation error |
| `51` | Insufficient funds | Rare for $0.00 |
| `91` | Bank unavailable | Retry later |

### Database Validation

```sql
-- Can only use ACH for debit if is_verified = true
SELECT * FROM customer_payment_methods
WHERE id = $1
  AND payment_type = 'ach'
  AND is_verified = true
  AND is_active = true;
-- If no results found: account not verified, cannot debit
```

---

## 6. EPX Transaction Types Summary

### Checking Account Transactions

| Type | Purpose | Direction | Required Fields |
|------|---------|-----------|-----------------|
| **CKC0** | Pre-note Debit | Debit | Account, Routing, Name |
| **CKC1** | Pre-note Credit | Credit | Account, Routing, Name |
| **CKC2** | ACH Debit | Debit | Storage BRIC, Amount |
| **CKC3** | ACH Credit | Credit | Storage BRIC, Amount |
| **CKC8** | BRIC Storage | N/A | Financial BRIC |
| **CKCX** | Void | N/A | Financial BRIC, Amount |

### Savings Account Transactions
- CKS0 (Pre-note Debit), CKS1 (Pre-note Credit), CKS2 (Debit), CKS3 (Credit), CKSX (Void)

---

## 7. Standard Entry Class Codes

### Common Entry Classes

| Code | Use Case | Example | Authorization |
|------|----------|---------|-----------------|
| **PPD** | Personal/Prearranged | Monthly subscription | Written authorization |
| **CCD** | Corporate | Invoice payments | Corporate authorization |
| **WEB** | Internet-initiated | Online bill pay | Web form authorization |
| **TEL** | Telephone-initiated | Phone authorization | Verbal authorization |

**Important:** PPD and CCD have lower return rates (~0.5%). WEB and TEL higher (~1-2%).

---

## 8. Database Schema

### customer_payment_methods Table

```sql
CREATE TABLE customer_payment_methods (
    id UUID PRIMARY KEY,
    merchant_id UUID NOT NULL,
    customer_id UUID NOT NULL,
    bric TEXT NOT NULL,                 -- Storage BRIC token
    payment_type VARCHAR(20),           -- 'ach' or 'credit_card'
    last_four VARCHAR(4),               -- Last 4 of account
    bank_name VARCHAR(255),             -- 'Chase', 'Bank of America'
    account_type VARCHAR(20),           -- 'checking' or 'savings'
    is_verified BOOLEAN DEFAULT false,  -- Pre-note passed
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);
```

### transactions Table

```sql
CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    parent_transaction_id UUID,         -- REFUND/VOID reference
    merchant_id UUID NOT NULL,
    customer_id UUID,
    amount_cents BIGINT NOT NULL,       -- 15000 = $150.00
    currency VARCHAR(3),
    type VARCHAR(20),                   -- 'DEBIT', 'REFUND', 'VOID'
    payment_method_type VARCHAR(20),    -- 'ach'
    payment_method_id UUID,
    tran_nbr TEXT,                      -- EPX transaction number
    auth_resp VARCHAR(10),              -- '00' = approved
    auth_guid TEXT,                     -- Financial BRIC
    auth_code VARCHAR(50),
    status VARCHAR(20) GENERATED ALWAYS,-- 'pending', 'approved', 'declined'
    processed_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);
```

---

## 9. State Machine

### Transaction Type Hierarchy

```
STANDALONE (no parent)
  ├─ SALE (auth + capture)
  ├─ AUTH (auth only)
  ├─ DEBIT (ACH debit)
  └─ STORAGE (tokenization)

CHILD (must have parent)
  ├─ CAPTURE (parent = AUTH or SALE)
  ├─ REFUND (parent = SALE or CAPTURE)
  └─ VOID (parent = AUTH or SALE)
```

### ACH-Specific State Transitions

```
DEBIT created (pending)
    │
    ├─→ Sent to EPX
    │   │
    │   ├─→ auth_resp='00' → approved (funds settling in 1-3 days)
    │   │   │
    │   │   ├─→ Can VOID (if same-day before settlement)
    │   │   └─→ Can CREDIT (refund after settlement)
    │   │
    │   └─→ auth_resp≠'00' → declined (transaction failed)
    │
    └─→ Never sent (system crash)
        └─→ failed
```

---

## 10. Error Handling

### EPX Error Codes

| Code | Description | Retry | User Message |
|------|-------------|--------|-------------|
| `05` | Do not honor | No | "Account closed or restricted" |
| `14` | Invalid card | No | "Invalid routing number" |
| `51` | Insufficient funds | Yes | "Not enough funds, please try later" |
| `61` | Exceeds limit | Yes | "Daily limit exceeded, try tomorrow" |
| `91` | Issuer unavailable | Yes | "Bank temporarily unavailable, try again" |

### Idempotency Implementation

```sql
-- Transaction ID is primary key → prevents duplicates
INSERT INTO transactions (id, ...) VALUES ($1, ...);
-- If duplicate ID sent, INSERT fails → return existing transaction

-- Check idempotency BEFORE sending to EPX:
BEGIN;
  -- Try to insert
  INSERT INTO transactions (id, ...) VALUES ($1, ...);
  -- If succeeds, send to EPX
  -- If fails (duplicate), return existing transaction
COMMIT;
```

---

## 11. NACHA Compliance Checklist

### Requirements

- [ ] **Authorization Form**
  - Customer agrees to recurring charges
  - Specifies amount (fixed or variable)
  - Specifies frequency (monthly, weekly, etc.)
  - Specifies start date
  - Specifies cancellation terms
  - Keep for 5 years

- [ ] **Pre-note Verification**
  - Send CKC0/CKC1 before first recurring debit
  - Verify auth_resp='00'
  - Store `is_verified=true` in database

- [ ] **Record Keeping**
  - Transaction ID (TRAN_NBR)
  - Amount and date
  - Customer name and account
  - Keep for 3-5 years

- [ ] **Return Handling**
  - Monitor returns within 2 business days of settlement
  - Refund disputed amount within 10 business days
  - Don't retry after return
  - Track return rate (must stay < 0.5%)

- [ ] **Notification**
  - Notify customer 10 days before charge (best practice)
  - Provide receipt within 15 days
  - Provide monthly statement
  - Honor cancellation requests within 1 business day

---

## 12. Implementation Examples

### StoreACHAccount Service Handler

```go
func (s *PaymentMethodService) StoreACHAccount(ctx context.Context,
    req *ports.StoreACHAccountRequest) (*domain.PaymentMethod, error) {

    // 1. Validate
    if req.AccountNumber == "" || req.RoutingNumber == "" {
        return nil, ErrInvalidAccount
    }

    // 2. Check idempotency
    existing, _ := s.getByRoutingAndLastFour(ctx, req)
    if existing != nil {
        return existing, nil
    }

    // 3. Pre-note
    preNoteReq := &ports.ServerPostRequest{
        TransactionType: ports.TransactionTypeACHPreNoteDebit,
        Amount: "0.00",
        AccountNumber: &req.AccountNumber,
        RoutingNumber: &req.RoutingNumber,
        ReceiverName: &req.AccountHolderName,
        StdEntryClass: &req.StdEntryClass,
    }

    preNoteResp, err := s.serverPost.ProcessTransaction(ctx, preNoteReq)
    if err != nil || preNoteResp.AuthResp != "00" {
        return nil, fmt.Errorf("pre-note failed: %s", preNoteResp.AuthRespText)
    }

    // 4. Storage BRIC
    storageReq := &ports.ServerPostRequest{
        TransactionType: ports.TransactionTypeBRICStorageACH,
        Amount: "0.00",
        OriginalAuthGUID: preNoteResp.AuthGUID,
    }

    storageResp, err := s.serverPost.ProcessTransaction(ctx, storageReq)
    if err != nil {
        return nil, fmt.Errorf("storage creation failed: %w", err)
    }

    // 5. Store
    pm := &domain.PaymentMethod{
        ID: uuid.New().String(),
        MerchantID: req.MerchantID,
        CustomerID: req.CustomerID,
        PaymentToken: storageResp.AuthGUID,
        PaymentType: domain.PaymentMethodTypeACH,
        LastFour: req.AccountNumber[len(req.AccountNumber)-4:],
        BankName: req.BankName,
        AccountType: "checking",
        IsVerified: true,
    }

    err = s.db.CreatePaymentMethod(ctx, pm)
    if err != nil {
        return nil, fmt.Errorf("failed to store payment method: %w", err)
    }

    return pm, nil
}
```

### ACHDebit Service Handler

```go
func (s *PaymentService) ACHDebit(ctx context.Context,
    req *ports.ACHDebitRequest) (*domain.Transaction, error) {

    // 1. Get payment method
    pm, err := s.db.GetPaymentMethod(ctx, req.PaymentMethodID)
    if err != nil || pm.PaymentType != "ach" || !pm.IsVerified {
        return nil, ErrInvalidPaymentMethod
    }

    // 2. Create transaction
    tx := &domain.Transaction{
        ID: uuid.New().String(),
        MerchantID: req.MerchantID,
        CustomerID: req.CustomerID,
        AmountCents: parseAmount(req.Amount),
        Type: "DEBIT",
        PaymentMethodType: "ach",
        PaymentMethodID: req.PaymentMethodID,
    }

    err = s.db.CreateTransaction(ctx, tx)
    if err != nil {
        return nil, err
    }

    // 3. Send to EPX
    epxReq := &ports.ServerPostRequest{
        TransactionType: ports.TransactionTypeACHDebit,
        Amount: req.Amount,
        AuthGUID: pm.PaymentToken,
        CardEntryMethod: stringPtr("Z"),
    }

    epxResp, err := s.serverPost.ProcessTransaction(ctx, epxReq)
    if err != nil {
        // Store as failed
        s.db.UpdateTransaction(ctx, tx.ID, "failed", nil, nil)
        return nil, err
    }

    // 4. Update transaction
    tx.AuthResp = &epxResp.AuthResp
    tx.AuthGUID = &epxResp.AuthGUID
    tx.ProcessedAt = timePtr(time.Now())

    err = s.db.UpdateTransaction(ctx, tx.ID, "", tx.AuthGUID, tx.ProcessedAt)
    if err != nil {
        return nil, err
    }

    // 5. Update payment method
    s.db.UpdatePaymentMethodLastUsed(ctx, req.PaymentMethodID)

    return tx, nil
}
```

---

## Summary

### Quick Reference Table

| Operation | Input | EPX Type | Storage Update | Status Path |
|-----------|-------|----------|-----------------|------------|
| **StoreACHAccount** | Account details | CKC0 → CKC8 | Create with is_verified=true | Pre-note verification |
| **ACHDebit** | Payment method | CKC2 | Create DEBIT | pending → approved/declined |
| **ACHCredit** | Payment method | CKC3 | Create DEBIT/REFUND | pending → approved/declined |
| **ACHVoid** | Transaction ID | CKCX | Create VOID with parent | pending → approved/declined |

---

This comprehensive documentation covers all ACH payment flows, EPX API integration points, database operations, error handling, and NACHA compliance requirements.
