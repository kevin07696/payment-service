# Single Bank Transaction (ACH) - Server Post API Dataflow

**Date**: 2025-11-03
**Transaction Type**: One-time ACH debit from bank account
**API**: EPX Server Post API
**Use Case**: Direct server-to-server bank account transaction

---

## Overview

This document describes the complete dataflow for processing a single ACH (Automated Clearing House) bank transaction using the EPX Server Post API. This is a server-to-server integration where the merchant backend communicates directly with EPX.

**Important**: This flow assumes the merchant has collected bank account information securely and is responsible for PCI/PII compliance when handling sensitive banking data.

---

## Complete Transaction Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│              SINGLE ACH TRANSACTION - SERVER POST API               │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Customer  │         │   Merchant  │         │     EPX     │
│  Interface  │         │   Backend   │         │   Gateway   │
└──────┬──────┘         └──────┬──────┘         └──────┬──────┘
       │                       │                       │
       │  1. Provide Bank      │                       │
       │     Account Info      │                       │
       ├──────────────────────>│                       │
       │                       │                       │
       │                       │  2. Build ACH Request │
       │                       │     (ServerPostAdapter)
       │                       │                       │
       │                       │  3. POST to EPX       │
       │                       │     (HTTPS:443 or     │
       │                       │      XML Socket:8086) │
       │                       ├──────────────────────>│
       │                       │                       │
       │                       │                       │  4. Validate
       │                       │                       │     Request
       │                       │                       │
       │                       │                       │  5. Process
       │                       │                       │     ACH
       │                       │                       │
       │                       │  6. XML Response      │
       │                       │     with AUTH_GUID    │
       │                       │<──────────────────────┤
       │                       │                       │
       │                       │  7. Parse Response    │
       │                       │     Store Transaction │
       │                       │                       │
       │  8. Confirmation      │                       │
       │<──────────────────────┤                       │
       │                       │                       │
```

---

## Detailed Step-by-Step Flow

### Step 1: Customer Provides Bank Account Information

**Actor**: Customer via Merchant Interface (web form, mobile app, agent system)
**Action**: User provides checking or savings account details

**Collected Data**:
- **Routing Number**: `021000021` (9 digits)
- **Account Number**: `1234567890`
- **Account Type**: `Checking` or `Savings`
- **Account Holder Name**: `John Doe`
- **Amount**: `$150.00`
- **Purpose**: `Invoice #INV-2025-045`

**Security Note**: Merchant is responsible for securely transmitting and storing this data

**Result**: Merchant backend receives bank account information

---

### Step 2: Merchant Backend Builds ACH Request

**Actor**: Merchant Backend
**Component**: ServerPostAdapter
**File**: `internal/adapters/epx/server_post_adapter.go` (adapter implementation)
**Port Interface**: `internal/adapters/ports/server_post.go`

**Method**: `ProcessTransaction(ctx, ServerPostRequest)`

**Build ServerPostRequest struct**:
```go
request := &ports.ServerPostRequest{
    // Agent credentials (required)
    CustNbr:     "123456",
    MerchNbr:    "789012",
    DBAnbr:      "1",
    TerminalNbr: "1",

    // Transaction details (required)
    TransactionType: ports.TransactionTypeSale, // "S" = Sale (debit)
    Amount:          "150.00",
    PaymentType:     ports.PaymentMethodTypeACH, // "ach"

    // Bank account details (instead of card)
    // Note: For initial transaction with full account info
    // (In production, these would be encrypted or tokenized)

    // Transaction identification
    TranNbr:   "ACH-2025-001",
    TranGroup: "550e8400-e29b-41d4-a716-446655440000", // Group ID

    // Customer info (optional but recommended)
    CustomerID: "CUST-12345",

    // Metadata
    Metadata: map[string]string{
        "invoice_number": "INV-2025-045",
        "description":    "Monthly subscription payment",
    },
}
```

**Format Request** (HTTPS POST format):
```
CUST_NBR=123456
&MERCH_NBR=789012
&DBA_NBR=1
&TERMINAL_NBR=1
&TRAN_TYPE=ACE1
&AMOUNT=150.00
&ACCOUNT_NBR=1234567890
&ROUTING_NBR=021000021
&ACCOUNT_TYPE=C
&NAME=John Doe
&BATCH_ID=20251103
&TRAN_NBR=ACH-2025-001
&CARD_ENT_METH=M
```

**OR Format as XML** (Socket method):
```xml
<DETAIL CUST_NBR="123456" MERCH_NBR="789012" DBA_NBR="1" TERMINAL_NBR="1">
    <TRAN_TYPE>ACE1</TRAN_TYPE>
    <AMOUNT>150.00</AMOUNT>
    <ACCOUNT_NBR>1234567890</ACCOUNT_NBR>
    <ROUTING_NBR>021000021</ROUTING_NBR>
    <ACCOUNT_TYPE>C</ACCOUNT_TYPE>
    <NAME>John Doe</NAME>
    <BATCH_ID>20251103</BATCH_ID>
    <TRAN_NBR>ACH-2025-001</TRAN_NBR>
    <CARD_ENT_METH>M</CARD_ENT_METH>
</DETAIL>
```

**Transaction Type Codes for ACH**:
- `ACE1`: ACH Ecommerce Sale (debit from customer account)
- `ACC1`: ACH Credit (credit to customer account)
- `ACP1`: ACH Pre-Note (verify account before real transaction)

**Result**: Request formatted and ready to send to EPX

---

### Step 3: POST Request to EPX Server Post API

**Actor**: Merchant Backend → EPX Gateway
**Component**: ServerPostAdapter HTTP client

**Connection Options**:

**Option A: HTTPS POST (Recommended for single transactions)**
- **URL**: `https://epx.com/server_post` (production)
- **URL**: `https://test.epx.com/server_post` (sandbox)
- **Port**: 443
- **Method**: POST
- **Content-Type**: `application/x-www-form-urlencoded`
- **Connection**: Opens new connection for each transaction
- **Timeout**: 30 seconds (configurable)

**Option B: XML Secure Socket (For high-volume batch processing)**
- **URL**: `ssl://epx.com`
- **Port**: 8086
- **Method**: TCP socket with SSL/TLS
- **Format**: XML
- **Connection**: Persistent (stays open for 30 seconds)
- **Use Case**: Multiple transactions in rapid succession

**For this example, using HTTPS POST**:

```go
func (a *serverPostAdapter) ProcessTransaction(ctx context.Context, req *ports.ServerPostRequest) (*ports.ServerPostResponse, error) {
    // Build form data
    formData := buildFormData(req)

    // Create HTTP request
    httpReq, err := http.NewRequestWithContext(
        ctx,
        "POST",
        a.config.URL,
        strings.NewReader(formData),
    )
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    // Send request
    resp, err := a.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("send request: %w", err)
    }
    defer resp.Body.Close()

    // Read response
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    // Parse XML response
    return parseXMLResponse(body)
}
```

**Result**: Request sent to EPX, waiting for response

---

### Step 4: EPX Validates Request

**Actor**: EPX Server Post API
**Process**:

1. **Authenticate Merchant Credentials**:
   - Verify CUST_NBR, MERCH_NBR, DBA_NBR, TERMINAL_NBR
   - Check merchant account status (active, not suspended)
   - Verify merchant authorized for ACH processing

2. **Validate Request Format**:
   - Check all required fields present
   - Validate field formats (regex patterns)
   - Verify TRAN_TYPE is valid for merchant
   - Check AMOUNT format (decimal, max 2 decimal places)

3. **Validate Bank Account Info**:
   - Routing number is 9 digits
   - Routing number exists in ABA database
   - Account number format valid
   - Account type is 'C' (checking) or 'S' (savings)

**Result**: Request validated, ready for processing

---

### Step 5: EPX Processes ACH Transaction

**Actor**: EPX Payment Gateway
**Process**:

1. **Submit to ACH Network**:
   - Format NACHA file entry
   - Submit to Federal Reserve ACH system
   - Generate ACH trace number

2. **Generate BRIC Token**:
   - Create AUTH_GUID (Financial BRIC)
   - Token represents this bank account
   - Lifetime: 13-24 months
   - Can be used for future recurring payments

3. **Record Transaction**:
   - Store in EPX transaction database
   - Assign batch ID
   - Record for settlement processing

**ACH Processing Timeline**:
- **Submitted**: Today (2025-11-03)
- **Settlement**: 1-3 business days
- **Funds Available**: Depends on merchant's bank

**Output**:
```
AUTH_GUID: "ACH-BRIC-09MBFZ3PV6BTRETVQK2"
AUTH_RESP: "00" (accepted for processing)
AUTH_CODE: "ACH001084" (ACH trace number)
AUTH_RESP_TEXT: "ACH ACCEPTED"
```

**Note**: AUTH_RESP "00" means ACH was **accepted for processing**, NOT that funds have cleared. Actual settlement takes 1-3 days.

---

### Step 6: EPX Returns XML Response

**Actor**: EPX Gateway → Merchant Backend
**Format**: XML
**Timing**: ~1 second after request

**XML Response**:
```xml
<RESPONSE>
    <FIELDS>
        <FIELD KEY="MSG_VERSION">003</FIELD>
        <FIELD KEY="CUST_NBR">123456</FIELD>
        <FIELD KEY="MERCH_NBR">789012</FIELD>
        <FIELD KEY="DBA_NBR">1</FIELD>
        <FIELD KEY="TERMINAL_NBR">1</FIELD>
        <FIELD KEY="TRAN_TYPE">ACE1</FIELD>
        <FIELD KEY="BATCH_ID">20251103</FIELD>
        <FIELD KEY="TRAN_NBR">ACH-2025-001</FIELD>
        <FIELD KEY="LOCAL_DATE">110325</FIELD>
        <FIELD KEY="LOCAL_TIME">143022</FIELD>
        <FIELD KEY="AUTH_GUID">ACH-BRIC-09MBFZ3PV6BTRETVQK2</FIELD>
        <FIELD KEY="AUTH_RESP">00</FIELD>
        <FIELD KEY="AUTH_CODE">ACH001084</FIELD>
        <FIELD KEY="AUTH_RESP_TEXT">ACH ACCEPTED</FIELD>
        <FIELD KEY="AMOUNT">150.00</FIELD>
        <FIELD KEY="ACCOUNT_TYPE">C</FIELD>
        <FIELD KEY="AUTH_TRAN_DATE_GMT">11/03/2025 02:30:22 PM</FIELD>
    </FIELDS>
</RESPONSE>
```

**Key Differences from Credit Card Response**:
- ❌ No `AUTH_CARD_TYPE` (not a card)
- ❌ No `AUTH_AVS` (no address verification for ACH)
- ❌ No `AUTH_CVV2` (no CVV for bank accounts)
- ✅ `ACCOUNT_TYPE` included (C = Checking, S = Savings)
- ✅ `AUTH_RESP` "00" = accepted for processing (not immediate approval)

**Result**: Merchant receives ACH acceptance confirmation

---

### Step 7: Merchant Parses Response and Stores Transaction

**Actor**: Merchant Backend
**Component**: ServerPostAdapter + Payment Service
**Files**:
- `internal/adapters/epx/server_post_adapter.go`
- `internal/services/payment/payment_service.go`

**Parse XML Response**:
```go
func parseXMLResponse(xmlData []byte) (*ports.ServerPostResponse, error) {
    type XMLResponse struct {
        Fields []struct {
            Key   string `xml:"KEY,attr"`
            Value string `xml:",chardata"`
        } `xml:"FIELDS>FIELD"`
    }

    var resp XMLResponse
    if err := xml.Unmarshal(xmlData, &resp); err != nil {
        return nil, fmt.Errorf("parse xml: %w", err)
    }

    // Extract fields into ServerPostResponse struct
    result := &ports.ServerPostResponse{
        RawXML: string(xmlData),
    }

    for _, field := range resp.Fields {
        switch field.Key {
        case "AUTH_GUID":
            result.AuthGUID = field.Value
        case "AUTH_RESP":
            result.AuthResp = field.Value
            result.IsApproved = (field.Value == "00")
        case "AUTH_CODE":
            result.AuthCode = field.Value
        case "AUTH_RESP_TEXT":
            result.AuthRespText = field.Value
        case "TRAN_NBR":
            result.TranNbr = field.Value
        case "AMOUNT":
            result.Amount = field.Value
        // ... other fields
        }
    }

    return result, nil
}
```

**Store in Database**:
```sql
INSERT INTO transactions (
    id,
    group_id,
    agent_id,
    customer_id,           -- 'CUST-12345'
    amount,
    currency,
    status,                -- 'pending' (waiting for settlement)
    type,
    payment_method_type,   -- 'ach'
    payment_method_id,     -- NULL (not saved yet)
    auth_guid,             -- ACH BRIC token
    auth_resp,
    auth_code,
    auth_resp_text,
    auth_card_type,        -- NULL (ACH has no card type)
    auth_avs,              -- NULL (no AVS for ACH)
    auth_cvv2,             -- NULL (no CVV for ACH)
    idempotency_key,
    metadata,
    created_at,
    updated_at
) VALUES (
    '770e8400-e29b-41d4-a716-446655440000',
    '550e8400-e29b-41d4-a716-446655440000',
    'AGT-123456-789012-1-1',
    'CUST-12345',
    150.00,
    'USD',
    'pending',                              -- ACH is pending settlement
    'charge',
    'ach',
    NULL,
    'ACH-BRIC-09MBFZ3PV6BTRETVQK2',        -- Financial BRIC for bank account
    '00',
    'ACH001084',
    'ACH ACCEPTED',
    NULL,                                   -- No card type
    NULL,                                   -- No AVS
    NULL,                                   -- No CVV
    'ACH-2025-001',
    '{"invoice_number": "INV-2025-045", "description": "Monthly subscription payment"}',
    NOW(),
    NOW()
);
```

**Why Store AUTH_GUID for ACH?**
1. **Recurring Payments**: Use BRIC for future monthly charges
2. **Refunds**: Can credit customer account using BRIC
3. **Account Verification**: BRIC represents verified bank account
4. **Saved Payment Methods**: Convert to Storage BRIC for customer's saved accounts
5. **Faster Future Transactions**: No need to collect bank info again

**Result**: ACH transaction stored with pending status

---

### Step 8: Merchant Confirms to Customer

**Actor**: Merchant Backend → Customer Interface
**Action**: Display confirmation message

**Confirmation Message**:
```
✓ Payment Processing

Your bank account payment has been submitted for processing.

Amount: $150.00
Account: Checking account ending in 7890
Transaction ID: 770e8400-e29b-41d4-a716-446655440000
Reference: ACH-2025-001
Status: Pending

Settlement Timeline:
- Submitted: November 3, 2025
- Expected Settlement: November 6-8, 2025 (1-3 business days)
- Funds will be debited from your account after settlement

You will receive a confirmation email once the payment has cleared.
```

**Result**: Customer is informed of ACH processing status

---

## ACH vs Credit Card Comparison

| Feature | Credit Card (Browser Post) | ACH (Server Post) |
|---------|---------------------------|-------------------|
| **API** | Browser Post API | Server Post API |
| **PCI Scope** | Minimal (card never touches backend) | Merchant responsibility |
| **Flow** | Browser → EPX → Backend | Backend ↔ EPX (direct) |
| **Settlement** | Real-time authorization | 1-3 business days |
| **Verification** | AVS, CVV, instant approval | No instant verification |
| **Response Time** | ~1 second (immediate) | ~1 second (acceptance only) |
| **Status** | Approved/Declined immediately | Accepted → Pending → Settled |
| **BRIC Lifetime** | 13-24 months | 13-24 months |
| **Use Cases** | One-time purchases, cards | Recurring, subscriptions, invoices |
| **Fees** | Higher (2-3% typically) | Lower (flat fee, typically < $1) |
| **Reversals** | Chargebacks (60-120 days) | Returns (60 days) |
| **Failure Mode** | Declined immediately | May bounce after submission |

---

## Using ACH BRIC for Recurring Payments

After the first ACH transaction, the merchant receives a Financial BRIC token that can be used for recurring payments without collecting bank account info again.

### Step 1: Save Payment Method (Optional)

**Convert Financial BRIC to Storage BRIC**:
- Call EPX BRIC Storage API
- One-time conversion fee
- Storage BRIC never expires

**Store in customer_payment_methods**:
```sql
INSERT INTO customer_payment_methods (
    id,
    customer_id,
    agent_id,
    payment_type,
    bric_token,           -- Storage BRIC
    account_type,         -- 'checking' or 'savings'
    last_four,            -- '7890'
    routing_number_last_four, -- '0021'
    account_holder_name,
    is_default,
    created_at,
    updated_at
) VALUES (
    UUID(),
    'CUST-12345',
    'AGT-123456-789012-1-1',
    'ach',
    'STORAGE-ACH-BRIC-TOKEN',
    'checking',
    '7890',
    '0021',
    'John Doe',
    true,
    NOW(),
    NOW()
);
```

### Step 2: Process Recurring Payment with BRIC

**Next month's charge** (no bank account info needed):
```go
request := &ports.ServerPostRequest{
    // Agent credentials
    CustNbr:     "123456",
    MerchNbr:    "789012",
    DBAnbr:      "1",
    TerminalNbr: "1",

    // Transaction details
    TransactionType: ports.TransactionTypeSale,
    Amount:          "150.00",
    PaymentType:     ports.PaymentMethodTypeACH,

    // Use BRIC token instead of bank account info
    AuthGUID: "STORAGE-ACH-BRIC-TOKEN",  // ← Saved payment method

    // Transaction identification
    TranNbr:   "ACH-2025-002",
    TranGroup: "550e8400-e29b-41d4-a716-446655440000",

    // Customer info
    CustomerID: "CUST-12345",

    // Metadata
    Metadata: map[string]string{
        "invoice_number": "INV-2025-046",
        "description":    "Monthly subscription payment",
        "recurring":      "true",
    },
}

// Send to EPX
response, err := serverPostAdapter.ProcessTransaction(ctx, request)
```

**HTTPS POST with BRIC**:
```
CUST_NBR=123456
&MERCH_NBR=789012
&DBA_NBR=1
&TERMINAL_NBR=1
&TRAN_TYPE=ACE1
&AMOUNT=150.00
&BRIC=STORAGE-ACH-BRIC-TOKEN
&BATCH_ID=20251203
&TRAN_NBR=ACH-2025-002
```

**Benefits**:
- ✅ No bank account info needed
- ✅ Faster processing
- ✅ Lower PCI compliance burden
- ✅ Better customer experience
- ✅ Reduced data storage liability

---

## Implementation Status

### ✅ Completed (Server Post Adapter)
- ServerPostAdapter port interface defined
- ServerPostRequest/Response structures
- Transaction type constants (Sale, Refund, Void, PreNote)
- Payment method type constants (CreditCard, ACH)
- ProcessTransaction method signature
- ProcessTransactionViaSocket method signature
- ValidateToken method signature

### ⏳ Pending Implementation
- Actual HTTP client implementation
- XML parsing logic
- XML socket connection handling
- Error handling and retries
- ACH-specific validation
- BRIC token validation ($0.00 auth)
- Convert Financial BRIC to Storage BRIC
- Link to customer_payment_methods table
- Recurring payment scheduling
- ACH status polling (for settlement confirmation)

---

## Security & Compliance

### Bank Account Data Handling
- ⚠️ Merchant must securely transmit bank account info to EPX
- ⚠️ Merchant responsible for PCI DSS if storing account data
- ✅ Recommended: Use BRIC tokens, never store raw account numbers
- ✅ Encrypt account data in transit (HTTPS/TLS)
- ✅ Encrypt account data at rest if temporary storage needed

### ACH Compliance
- ✅ NACHA (National Automated Clearing House Association) rules apply
- ✅ Customer authorization required (written or recorded)
- ✅ Pre-notification (pre-note) recommended for new accounts
- ✅ Return/reversal handling required (60-day window)
- ✅ Customer notification before each debit (for recurring)

### Data Protection
- ✅ HTTPS required for all communications
- ✅ SSL certificate validation mandatory
- ✅ Connection timeout (30 seconds)
- ✅ Idempotency via TRAN_NBR
- ✅ BRIC tokens reduce data exposure

---

## Testing Checklist

### Unit Tests
- ⏳ ServerPostAdapter.ProcessTransaction() - TODO
- ⏳ ServerPostAdapter.ProcessTransactionViaSocket() - TODO
- ⏳ ServerPostAdapter.ValidateToken() - TODO
- ⏳ XML request building - TODO
- ⏳ XML response parsing - TODO

### Integration Tests
- ⏳ End-to-end ACH transaction with test account - TODO
- ⏳ BRIC token generation and reuse - TODO
- ⏳ Error handling (invalid routing number, declined) - TODO
- ⏳ Timeout handling - TODO

### ACH-Specific Tests
- ⏳ Pre-note verification - TODO
- ⏳ ACH return handling - TODO
- ⏳ Settlement status polling - TODO
- ⏳ Recurring payment with BRIC - TODO

---

## Common ACH Response Codes

| Code | Description | Action |
|------|-------------|--------|
| 00 | Accepted | Transaction submitted successfully |
| 05 | Declined | Bank account invalid or closed |
| 12 | Invalid Transaction | Check request format |
| 13 | Invalid Amount | Amount exceeds limits or format wrong |
| 19 | Re-enter | Temporary issue, retry |
| 51 | Insufficient Funds | Customer has insufficient balance |
| 52 | No Account | Account number not found |
| 53 | Invalid Account | Account type mismatch or closed |

**Note**: "00" means accepted for processing, not guaranteed settlement. Monitor for returns over next 3-5 days.

---

## File Structure

### Required Files
```
internal/
├── adapters/
│   ├── epx/
│   │   └── server_post_adapter.go     # ← Implementation needed
│   └── ports/
│       └── server_post.go              # ✅ Port interface exists
├── services/
│   └── payment/
│       └── payment_service.go          # Uses ServerPostAdapter
└── handlers/
    └── payment/
        └── payment_handler.go          # gRPC handlers
```

---

**Document Version**: 1.0
**Last Updated**: 2025-11-03
**Status**: ✅ ACTIVE - Pending Implementation
