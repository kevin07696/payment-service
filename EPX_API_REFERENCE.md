# EPX Server Post API Reference

**Version:** 1.0
**Last Updated:** November 6, 2025
**Environment:** Sandbox (https://secure.epxuap.com)
**Status:** ✅ All transaction types tested and verified

---

## Table of Contents

1. [Introduction](#introduction)
2. [Authentication](#authentication)
3. [Common Request Fields](#common-request-fields)
4. [Common Response Fields](#common-response-fields)
5. [Transaction Types](#transaction-types)
   - [Sale (CCE1)](#1-sale-cce1---authorization--capture)
   - [Authorization Only (CCE2)](#2-authorization-only-cce2)
   - [Capture (CCE4)](#3-capture-cce4)
   - [Refund (CCE9)](#4-refund-cce9)
   - [Void (CCEX)](#5-void-ccex)
   - [BRIC Storage (CCE8)](#6-bric-storage-cce8---tokenization)
   - [Recurring Payment](#7-recurring-payment---sale-with-storage-bric)
6. [Response Codes](#response-codes)
7. [Error Handling](#error-handling)
8. [Best Practices](#best-practices)

---

## Introduction

The EPX Server Post API is a server-to-server payment processing interface that allows merchants to:
- Process credit card transactions (authorization, capture, sale)
- Generate BRIC tokens for recurring payments
- Process refunds and voids
- Store payment methods securely for future use

**Key Features:**
- Real-time transaction processing
- PCI-compliant tokenization (BRIC)
- Support for recurring/installment payments
- AVS and CVV verification
- Card-on-file compliance

---

## Authentication

Every request requires these EPX credentials:

| Field | Description | Example | Required |
|-------|-------------|---------|----------|
| `CUST_NBR` | EPX Customer Number | `9001` | Yes |
| `MERCH_NBR` | EPX Merchant Number | `900300` | Yes |
| `DBA_NBR` | EPX DBA Number | `2` | Yes |
| `TERMINAL_NBR` | EPX Terminal Number | `77` | Yes |

**How to Obtain:**
Contact EPX/North American Bancard to receive your credentials via their Developer Portal.

---

## Common Request Fields

These fields are used across multiple transaction types:

### Required Fields

| Field | Type | Max Length | Description | Example |
|-------|------|------------|-------------|---------|
| `TRAN_TYPE` | String | 4 | Transaction type code | `CCE1` |
| `AMOUNT` | Decimal | 10 | Transaction amount (dollars.cents) | `10.00` |
| `TRAN_NBR` | String | 10 | Unique transaction number | `12345` |
| `BATCH_ID` | String | 10 | Batch/group identifier | `12345` |

**Important Notes:**
- `TRAN_NBR` must be unique per transaction
- Use timestamp modulo to generate: `time.Now().Unix()%100000`
- Keep to 5-10 digits to avoid "Invalid TRAN_NBR[LEN]" error

### Card Information Fields

| Field | Type | Max Length | Description | Example | Required |
|-------|------|------------|-------------|---------|----------|
| `ACCOUNT_NBR` | String | 16 | Credit card number | `4111111111111111` | Yes* |
| `EXP_DATE` | String | 4 | Expiration date (YYMM) | `1225` | Yes* |
| `CVV2` | String | 4 | Card verification value | `123` | Recommended |
| `CARD_ENT_METH` | String | 1 | Card entry method | `E` or `Z` | Yes |
| `INDUSTRY_TYPE` | String | 1 | Industry type | `E` | Yes |

*Required for new card transactions, not required when using BRIC tokens

**Card Entry Method Values:**
- `E` = E-commerce (card details provided)
- `Z` = BRIC/Token (using previously stored token)
- `6` = PAN Entry via Card on File

**Industry Type Values:**
- `E` = E-commerce
- `R` = Retail
- `T` = Restaurant

### Billing Information Fields

| Field | Type | Max Length | Description | Example | Required for AVS |
|-------|------|------------|-------------|---------|------------------|
| `FIRST_NAME` | String | 50 | Cardholder first name | `John` | Recommended |
| `LAST_NAME` | String | 50 | Cardholder last name | `Doe` | Recommended |
| `ADDRESS` | String | 100 | Street address | `123 Main St` | Yes |
| `CITY` | String | 50 | City name | `New York` | No |
| `STATE` | String | 2 | State code (2 letters) | `NY` | No |
| `ZIP_CODE` | String | 10 | ZIP/Postal code | `10001` | Yes |

**AVS (Address Verification System):**
Providing billing information enables AVS checks, which can:
- Reduce fraud risk
- Improve authorization rates
- Required for BRIC Storage (tokenization)

### BRIC Token Fields

| Field | Type | Max Length | Description | When to Use |
|-------|------|------------|-------------|-------------|
| `AUTH_GUID` | String | 20 | BRIC token for new transaction | When starting a transaction with a token |
| `ORIG_AUTH_GUID` | String | 20 | Original BRIC token reference | For capture, void, refund, recurring payments |

**Key Difference:**
- `AUTH_GUID` = "I want to use this token for THIS transaction"
- `ORIG_AUTH_GUID` = "I'm referencing a PREVIOUS transaction's token"

### Advanced Fields

| Field | Type | Max Length | Description | Values |
|-------|------|------------|-------------|--------|
| `ACI_EXT` | String | 2 | Authorization Characteristics Indicator | `RB`, `IP`, `CA`, etc. |
| `COF_PERIOD` | Integer | 2 | Card-on-file lifetime (months) | `0`, `13`, `18`, `24` |
| `ORIG_AUTH_AMOUNT` | Decimal | 10 | Original transaction amount | `100.00` |
| `ORIG_AUTH_TRAN_IDENT` | String | 15 | Network Transaction ID (NTID) | From previous response |

**ACI_EXT Values:**
- `RB` = Recurring Billing (subscriptions)
- `IP` = Installment Payment
- `CA` = Completion Advice
- `DS` = Delayed Card Sale
- `NS` = No Show Charge
- `RA` = Re-authorization

---

## Common Response Fields

Every EPX response contains these fields:

### Core Response Fields

| Field | Description | Example | Meaning |
|-------|-------------|---------|---------|
| `AUTH_GUID` | BRIC token (transaction identifier) | `09LMQ886L2K2W11MPX1` | Unique token for this transaction |
| `AUTH_RESP` | Authorization response code | `00` | `00` = Approved, others = Declined |
| `AUTH_CODE` | Bank authorization code | `057579` | Approval code from issuing bank |
| `AUTH_RESP_TEXT` | Human-readable response | `ZIP MATCH` | Description of response |

**AUTH_GUID (BRIC Token):**
- 19-character alphanumeric identifier
- Format: `09XXXXXXXXXXXXXXXXX`
- **Financial BRIC**: Valid 13 months (from regular transactions)
- **Storage BRIC**: Valid indefinitely (from CCE8 transactions)
- Use this token for future transactions with the same card

**AUTH_RESP Codes:**
- `00` = Approved ✅
- `05` = Do not honor (declined by bank)
- `12` = Invalid transaction (incorrect parameters)
- `14` = Invalid card number
- `51` = Insufficient funds
- `54` = Expired card
- `EH` = CEM Invalid (Card Entry Method error)
- `RR` = Declined by EPX (check AUTH_RESP_TEXT)

### Card Verification Fields

| Field | Description | Example | Values |
|-------|-------------|---------|--------|
| `AUTH_CARD_TYPE` | Card brand | `V` | V=Visa, M=Mastercard, A=Amex, D=Discover |
| `AUTH_AVS` | Address verification result | `Z` | See AVS codes below |
| `AUTH_CVV2` | CVV verification result | `M` | M=Match, N=No match, P=Not processed |

**AVS Codes:**
- `Y` = Address and ZIP match (best)
- `Z` = ZIP matches, address doesn't
- `A` = Address matches, ZIP doesn't
- `N` = No match (high fraud risk)
- `U` = Unavailable

**CVV Codes:**
- `M` = Match (valid CVV)
- `N` = No match (invalid CVV)
- `P` = Not processed
- `U` = Unavailable

### Transaction Echo Fields

| Field | Description | Example |
|-------|-------------|---------|
| `TRAN_NBR` | Echo back transaction number | `12345` |
| `BATCH_ID` | Echo back batch ID | `12345` |
| `AMOUNT` | Echo back amount | `10.00` |

### Additional Response Fields

| Field | Description | When Present |
|-------|-------------|--------------|
| `NTID` (Network Transaction ID) | Card network transaction identifier | BRIC Storage (CCE8) |
| `AUTH_TRAN_IDENT` | Same as NTID | Card-on-file transactions |

---

## Transaction Types

### 1. Sale (CCE1) - Authorization + Capture

**What it does:** Authorizes and captures funds in one step. Money is immediately earmarked for settlement.

**When to use:**
- E-commerce purchases
- Point of sale transactions
- When you want immediate payment

**Flow:**
1. Customer provides card details
2. EPX authorizes the transaction with the bank
3. If approved, funds are captured automatically
4. Transaction settles in the next batch close (usually nightly)

#### Request Example

```go
request := &ports.ServerPostRequest{
    // EPX Credentials
    CustNbr:     "9001",
    MerchNbr:    "900300",
    DBAnbr:      "2",
    TerminalNbr: "77",

    // Transaction Details
    TransactionType: ports.TransactionTypeSale, // "CCE1"
    Amount:          "10.00",
    TranNbr:         "12345",  // Unique transaction number
    TranGroup:       "12345",  // Maps to BATCH_ID

    // Payment Method (Card Details)
    PaymentType:     ports.PaymentMethodTypeCreditCard,
    AccountNumber:   strPtr("4111111111111111"),  // Visa test card
    ExpirationDate:  strPtr("1225"),              // December 2025
    CVV:             strPtr("123"),
    CardEntryMethod: strPtr("E"),                 // E-commerce
    IndustryType:    strPtr("E"),

    // Billing Information (for AVS)
    FirstName: strPtr("John"),
    LastName:  strPtr("Doe"),
    Address:   strPtr("123 Main St"),
    City:      strPtr("New York"),
    State:     strPtr("NY"),
    ZipCode:   strPtr("10001"),
}
```

#### HTTP Form Data (Raw)

```
CUST_NBR=9001
MERCH_NBR=900300
DBA_NBR=2
TERMINAL_NBR=77
TRAN_TYPE=CCE1
AMOUNT=10.00
TRAN_NBR=12345
BATCH_ID=12345
ACCOUNT_NBR=4111111111111111
EXP_DATE=1225
CVV2=123
CARD_ENT_METH=E
INDUSTRY_TYPE=E
FIRST_NAME=John
LAST_NAME=Doe
ADDRESS=123 Main St
CITY=New York
STATE=NY
ZIP_CODE=10001
```

#### Response Example

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="AUTH_GUID">09LMQ886L2K2W11MPX1</FIELD>
    <FIELD KEY="AUTH_RESP">00</FIELD>
    <FIELD KEY="AUTH_CODE">057579</FIELD>
    <FIELD KEY="AUTH_RESP_TEXT">ZIP MATCH</FIELD>
    <FIELD KEY="AUTH_CARD_TYPE">V</FIELD>
    <FIELD KEY="AUTH_AVS">Z</FIELD>
    <FIELD KEY="AUTH_CVV2">M</FIELD>
    <FIELD KEY="TRAN_NBR">12345</FIELD>
    <FIELD KEY="BATCH_ID">12345</FIELD>
    <FIELD KEY="AMOUNT">10.00</FIELD>
  </FIELDS>
</RESPONSE>
```

#### Response Parsed

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "09LMQ886L2K2W11MPX1",  // Save this for future transactions
    AuthResp:     "00",                    // Approved
    AuthCode:     "057579",                // Bank approval code
    AuthRespText: "ZIP MATCH",
    IsApproved:   true,                    // Derived from AuthResp == "00"

    // Verification Results
    AuthCardType: "V",                     // Visa
    AuthAVS:      "Z",                     // ZIP matches, address doesn't
    AuthCVV2:     "M",                     // CVV matches

    // Echo Back
    TranNbr:   "12345",
    TranGroup: "12345",
    Amount:    "10.00",

    ProcessedAt: time.Now(),               // Transaction timestamp
}
```

#### Field Explanations

**Request:**
- `TransactionType: "CCE1"` = Credit Card E-commerce Sale
- `Amount: "10.00"` = $10.00 USD (always use decimal format)
- `TranNbr` = Your internal transaction ID (must be unique)
- `AccountNumber` = Full 16-digit card number (PAN)
- `ExpirationDate: "1225"` = December 2025 (YYMM format)
- `CardEntryMethod: "E"` = E-commerce (card not present)

**Response:**
- `AUTH_GUID` = BRIC token valid for 13 months - save this!
- `AUTH_RESP: "00"` = Transaction approved by bank
- `AUTH_CODE` = Bank's approval code (for settlement reconciliation)
- `AUTH_AVS: "Z"` = ZIP code matched, street address didn't (acceptable)
- `AUTH_CVV2: "M"` = CVV matched (good security indicator)

---

### 2. Authorization Only (CCE2)

**What it does:** Authorizes funds but doesn't capture them. Holds the amount on the customer's card.

**When to use:**
- Pre-authorizations (hotels, car rentals)
- When final amount is unknown (gas pumps)
- When you need to validate card before shipping
- Two-step checkout processes

**Flow:**
1. Customer provides card details
2. EPX authorizes the transaction (funds held)
3. **No capture** - funds remain held until you capture or authorization expires (typically 7-30 days)
4. You must call Capture (CCE4) to actually receive the funds

#### Request Example

```go
request := &ports.ServerPostRequest{
    CustNbr:     "9001",
    MerchNbr:    "900300",
    DBAnbr:      "2",
    TerminalNbr: "77",

    TransactionType: ports.TransactionTypeAuthOnly, // "CCE2"
    Amount:          "25.00",
    TranNbr:         "12346",
    TranGroup:       "12346",

    PaymentType:     ports.PaymentMethodTypeCreditCard,
    AccountNumber:   strPtr("5499740000000057"),  // Mastercard test card
    ExpirationDate:  strPtr("1225"),
    CVV:             strPtr("123"),
    CardEntryMethod: strPtr("E"),
    IndustryType:    strPtr("E"),

    FirstName: strPtr("Jane"),
    LastName:  strPtr("Smith"),
    ZipCode:   strPtr("90210"),
}
```

#### Response Example

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "09LMQ886N9RB1P9JPX3",  // Save for later capture
    AuthResp:     "00",                    // Authorized (not captured)
    AuthCode:     "057580",
    AuthRespText: "APPROVAL",
    IsApproved:   true,

    AuthCardType: "M",                     // Mastercard

    TranNbr:   "12346",
    Amount:    "25.00",
}
```

#### Field Explanations

**Key Difference from Sale:**
- `TransactionType: "CCE2"` = Authorization Only (no capture)
- Funds are **held** on customer's card but **not transferred** to you
- You must call Capture (CCE4) within the authorization window

**What happens:**
1. Bank approves the authorization
2. $25.00 is "held" on customer's card (reduces available credit)
3. Money doesn't move yet
4. You have ~7-30 days to capture (depends on card network)
5. If you don't capture, the hold releases automatically

---

### 3. Capture (CCE4)

**What it does:** Captures (completes) a previously authorized transaction.

**When to use:**
- After Auth-Only (CCE2)
- When item ships
- When final amount is confirmed
- Within the authorization validity period

**Flow:**
1. Reference the original Auth-Only transaction by its `AUTH_GUID`
2. EPX captures the funds
3. Transaction moves to settlement

#### Request Example

```go
request := &ports.ServerPostRequest{
    CustNbr:     "9001",
    MerchNbr:    "900300",
    DBAnbr:      "2",
    TerminalNbr: "77",

    TransactionType:  ports.TransactionTypeCapture, // "CCE4"
    Amount:           "25.00",  // Can be less than original auth
    TranNbr:          "12347",
    TranGroup:        "12347",

    // Reference the Auth-Only transaction
    OriginalAuthGUID: "09LMQ886N9RB1P9JPX3",  // From Auth-Only response

    CardEntryMethod: strPtr("Z"),  // "Z" because using BRIC token
    IndustryType:    strPtr("E"),
}
```

#### Response Example

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "09LMQ886PJFUZWJ4PX7",  // New BRIC for the capture
    AuthResp:     "00",                    // Capture approved
    AuthCode:     "",                      // May be empty for captures
    AuthRespText: "APPROVAL",
    IsApproved:   true,

    AuthCardType: "M",

    TranNbr:   "12347",
    Amount:    "25.00",
}
```

#### Field Explanations

**Request:**
- `TransactionType: "CCE4"` = Capture previous authorization
- `OriginalAuthGUID` = The `AUTH_GUID` from the Auth-Only (CCE2) response
- `Amount` = Can be **equal or less** than original authorization (partial capture)
- `CardEntryMethod: "Z"` = Using BRIC token (not entering card again)

**Important Notes:**
- You **cannot** capture more than the original authorization amount
- Partial captures are allowed (e.g., authorize $100, capture $75)
- Once captured, you cannot void - you must refund instead
- Some banks allow multiple partial captures, others don't

**Example Use Case:**
```
1. Customer orders 2 items ($50 total)
2. You authorize $50 (CCE2)
3. Item 1 ships ($30) - Capture $30 (CCE4)
4. Item 2 out of stock - Authorization for remaining $20 expires
```

---

### 4. Refund (CCE9)

**What it does:** Returns money to the customer's card.

**When to use:**
- Product returns
- Order cancellations (after settlement)
- Billing disputes
- Overpayments

**Flow:**
1. Reference the original Sale transaction by its `AUTH_GUID`
2. EPX processes the refund
3. Funds return to customer's card (typically 3-5 business days)

#### Request Example

```go
request := &ports.ServerPostRequest{
    CustNbr:     "9001",
    MerchNbr:    "900300",
    DBAnbr:      "2",
    TerminalNbr: "77",

    TransactionType:  ports.TransactionTypeRefund, // "CCE9"
    Amount:           "5.00",   // Partial refund
    TranNbr:          "12348",
    TranGroup:        "12348",

    // Reference the original Sale transaction
    OriginalAuthGUID: "09LMQ886L2K2W11MPX1",  // From original sale

    CardEntryMethod: strPtr("Z"),  // Using BRIC reference
    IndustryType:    strPtr("E"),
}
```

#### Response Example

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "09LMQ886RV3TWKM3PXA",  // New BRIC for refund
    AuthResp:     "00",                    // Refund approved
    AuthCode:     "057581",
    AuthRespText: "ZIP MATCH",
    IsApproved:   true,

    AuthCardType: "V",
    AuthAVS:      "Z",

    TranNbr:   "12348",
    Amount:    "5.00",  // Refunded amount
}
```

#### Field Explanations

**Request:**
- `TransactionType: "CCE9"` = Credit Card E-commerce Refund
- `Amount: "5.00"` = Can be **partial** or **full** refund
- `OriginalAuthGUID` = The sale transaction you're refunding

**Refund Types:**
- **Full Refund**: Amount equals original sale amount
- **Partial Refund**: Amount less than original sale (e.g., restocking fee)
- **Multiple Partial Refunds**: Some processors allow multiple refunds up to original amount

**Important Notes:**
- Cannot refund more than original sale amount
- Refunds typically take 3-5 business days to appear on customer's statement
- Some banks charge refund fees
- Original sale must be settled (usually 1 day after sale)

**Void vs Refund:**
- **Void (CCEX)**: Cancel before settlement (same day, no money moved)
- **Refund (CCE9)**: Return money after settlement (next day+, money already moved)

---

### 5. Void (CCEX)

**What it does:** Cancels a transaction before it settles.

**When to use:**
- Order cancelled on same day
- Duplicate transaction
- Customer changed mind immediately
- Error correction before settlement

**Flow:**
1. Reference the original transaction by its `AUTH_GUID`
2. EPX voids the transaction
3. Transaction removed from settlement batch
4. **No money moves** (authorization released)

#### Request Example

```go
// First, create a sale to void
saleReq := &ports.ServerPostRequest{
    CustNbr:         "9001",
    MerchNbr:        "900300",
    DBAnbr:          "2",
    TerminalNbr:     "77",
    TransactionType: ports.TransactionTypeSale,
    Amount:          "1.00",
    TranNbr:         "12349",
    TranGroup:       "12349",
    AccountNumber:   strPtr("4111111111111111"),
    ExpirationDate:  strPtr("1225"),
    CVV:             strPtr("123"),
    CardEntryMethod: strPtr("E"),
    IndustryType:    strPtr("E"),
}
// Response: AUTH_GUID = "09LMQ8870L2BZPWYPXE"

// Then void it
voidReq := &ports.ServerPostRequest{
    CustNbr:     "9001",
    MerchNbr:    "900300",
    DBAnbr:      "2",
    TerminalNbr: "77",

    TransactionType:  ports.TransactionTypeVoid, // "CCEX"
    Amount:           "1.00",  // Must match original
    TranNbr:          "12350",
    TranGroup:        "12350",

    // Reference the transaction to void
    OriginalAuthGUID: "09LMQ8870L2BZPWYPXE",  // From sale

    CardEntryMethod: strPtr("Z"),
    IndustryType:    strPtr("E"),
}
```

#### Response Example

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "09LMQ8872ZVNWJRFPXK",  // New BRIC for void record
    AuthResp:     "00",                    // Void approved
    AuthCode:     "",                      // Typically empty for voids
    AuthRespText: "APPROVAL",
    IsApproved:   true,

    TranNbr:   "12350",
    Amount:    "1.00",
}
```

#### Field Explanations

**Request:**
- `TransactionType: "CCEX"` = Credit Card E-commerce Void
- `Amount` = Must match original transaction amount
- `OriginalAuthGUID` = The transaction you're voiding

**Critical Timing:**
- **Same Day Only**: Must void before settlement (usually before midnight)
- After settlement (next day), you must use Refund (CCE9) instead

**Void vs Refund Comparison:**

| Aspect | Void (CCEX) | Refund (CCE9) |
|--------|-------------|---------------|
| Timing | Same day, before settlement | After settlement |
| Money Movement | None (transaction cancelled) | Money returned to customer |
| Customer Impact | Authorization released immediately | 3-5 days to see credit |
| Fees | Usually none | May incur refund fees |
| Settlement | Removed from batch | Separate refund transaction |

**Best Practice:**
Always try Void first (same day), fall back to Refund if void fails.

---

### 6. BRIC Storage (CCE8) - Tokenization

**What it does:** Converts a Financial BRIC token to a Storage BRIC token that never expires.

**When to use:**
- Saving payment methods for future use
- Setting up recurring/subscription payments
- Card-on-file for quick checkout
- Customer requests "Save my card"

**Flow:**
1. Customer completes a sale (CCE1) - receives Financial BRIC (13-month expiry)
2. Customer clicks "Save payment method"
3. You call BRIC Storage (CCE8) with the Financial BRIC
4. EPX performs $0.00 Account Verification with card networks
5. Receives Storage BRIC (never expires) + Network Transaction ID

#### Request Example

```go
// After a successful sale that returned AUTH_GUID = "09LMQ886L2K2W11MPX1"

request := &ports.ServerPostRequest{
    CustNbr:     "9001",
    MerchNbr:    "900300",
    DBAnbr:      "2",
    TerminalNbr: "77",

    TransactionType: ports.TransactionTypeBRICStorageCC, // "CCE8"
    Amount:          "0.00",  // ALWAYS $0.00 for BRIC Storage
    TranNbr:         "12351",
    TranGroup:       "12351",

    // Reference the Financial BRIC to convert
    OriginalAuthGUID: "09LMQ886L2K2W11MPX1",  // From sale

    CardEntryMethod: strPtr("Z"),  // BRIC-based
    IndustryType:    strPtr("E"),

    // Billing info REQUIRED for Account Verification
    FirstName: strPtr("John"),
    LastName:  strPtr("Doe"),
    Address:   strPtr("123 Main Street"),
    City:      strPtr("New York"),
    State:     strPtr("NY"),
    ZipCode:   strPtr("10001"),
}
```

#### Response Example

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "09LMQ88756L8UDTUPXN",  // Storage BRIC - NEVER EXPIRES
    AuthResp:     "00",                    // Account Verification approved
    AuthCode:     "429745",                // Verification code
    AuthRespText: "APPROVAL",
    IsApproved:   true,

    AuthCardType: "V",

    // Network Transaction ID (for card-on-file compliance)
    NetworkTransactionID: strPtr("123456789012345"),  // NTID

    TranNbr:   "12351",
    Amount:    "0.00",  // $0.00 verification
}
```

#### Field Explanations

**Request:**
- `TransactionType: "CCE8"` = BRIC Storage (Credit Card)
- `Amount: "0.00"` = **MUST be $0.00** - triggers Account Verification
- `OriginalAuthGUID` = Financial BRIC from previous sale
- Billing information **REQUIRED** for AVS check

**What is Account Verification?**
- EPX sends a $0.00 authorization to the card networks
- Verifies the card is still valid
- Checks billing address (AVS)
- No charge to customer
- Returns Network Transaction ID (NTID) for compliance

**Response:**
- `AuthGUID` = **Storage BRIC** - save this in your database!
- `NetworkTransactionID` (NTID) = Required for card-on-file regulations

**Storage BRIC Benefits:**
1. **Never Expires**: Unlike Financial BRIC (13 months), Storage BRIC lasts forever
2. **Card Updates**: Some networks auto-update expired cards
3. **Compliance**: Meets PCI and card network storage requirements
4. **Recurring Ready**: Use for subscription/recurring payments

**Financial BRIC vs Storage BRIC:**

| Feature | Financial BRIC | Storage BRIC |
|---------|----------------|--------------|
| Created By | Regular transactions (CCE1, CCE2, etc.) | BRIC Storage (CCE8) |
| Lifetime | 13 months | Never expires |
| Use Case | Reference previous transaction | Store for future use |
| Account Verification | No | Yes ($0.00 auth) |
| Network Transaction ID | No | Yes (NTID) |
| Card Updates | No | Possible (network-dependent) |

**Best Practice Workflow:**
```
1. Customer completes purchase (CCE1) → Get Financial BRIC
2. Customer clicks "Save card" → Call BRIC Storage (CCE8)
3. Store Storage BRIC + NTID in database
4. Use Storage BRIC for future recurring payments
5. Replace with new Storage BRIC after each transaction (rolling token)
```

---

### 7. Recurring Payment - Sale with Storage BRIC

**What it does:** Charges a stored payment method for recurring/subscription payments.

**When to use:**
- Monthly subscriptions
- Recurring membership fees
- Installment payments
- Auto-renewal charges

**Flow:**
1. Customer has Storage BRIC from previous BRIC Storage (CCE8) transaction
2. Your system triggers recurring payment (e.g., monthly subscription)
3. Process sale using Storage BRIC + Recurring Billing indicator
4. Customer charged without re-entering card details

#### Request Example

```go
// Using Storage BRIC from previous BRIC Storage: "09LMQ88756L8UDTUPXN"

request := &ports.ServerPostRequest{
    CustNbr:     "9001",
    MerchNbr:    "900300",
    DBAnbr:      "2",
    TerminalNbr: "77",

    TransactionType:  ports.TransactionTypeSale, // "CCE1" - regular sale
    Amount:           "15.00",  // Subscription amount
    TranNbr:          "12352",
    TranGroup:        "12352",

    // Use Storage BRIC (CRITICAL: use OriginalAuthGUID, NOT AuthGUID)
    OriginalAuthGUID: "09LMQ88756L8UDTUPXN",  // Storage BRIC

    // Recurring Billing Indicator (REQUIRED)
    ACIExt: strPtr("RB"),  // "RB" = Recurring Billing

    CardEntryMethod: strPtr("Z"),  // BRIC/Token transaction
    IndustryType:    strPtr("E"),
}
```

#### Response Example

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "09LMQ8877XWJEBZRPXP",  // New BRIC for this payment
    AuthResp:     "00",                    // Payment approved
    AuthCode:     "057583",
    AuthRespText: "ADDRESS MATCH",
    IsApproved:   true,

    AuthCardType: "V",
    AuthAVS:      "A",  // Address matches

    TranNbr:   "12352",
    Amount:    "15.00",  // Amount charged
}
```

#### Field Explanations

**Request - Critical Fields:**
- `TransactionType: "CCE1"` = Regular sale (not a special recurring type)
- `OriginalAuthGUID` = Storage BRIC token ⚠️ **NOT `AuthGUID`**
- `ACIExt: "RB"` = **REQUIRED** - tells card networks this is recurring
- `CardEntryMethod: "Z"` = Using stored token

**Why OriginalAuthGUID vs AuthGUID?**
- `AuthGUID` = "Start a new transaction with this token"
- `OriginalAuthGUID` = "Reference this stored token for a new charge"

For recurring payments, you're saying "charge the card referenced by this Storage BRIC token", hence `OriginalAuthGUID`.

**ACI_EXT Field (Authorization Characteristics Indicator):**
- **Required by card networks** for card-on-file compliance
- Tells networks how to process the transaction
- Common values:
  - `RB` = Recurring Billing (subscriptions)
  - `IP` = Installment Payment (split payments)
  - `CA` = Completion Advice
  - `DS` = Delayed Sale

**Card Network Compliance:**
When you use `ACI_EXT=RB`, card networks:
1. Know this is a recurring charge
2. Apply different authorization rules
3. May have better approval rates
4. Ensure proper transaction coding

**Response:**
- `AuthGUID` = New BRIC for this payment (can save for next billing cycle)
- Standard approval response
- Check `IsApproved` to verify payment succeeded

**Best Practice Workflow:**
```
Month 1: Customer signs up
  → Process initial sale (CCE1)
  → Convert to Storage BRIC (CCE8)
  → Save Storage BRIC: "09LMQ88756L8UDTUPXN"

Month 2: Recurring payment due
  → Process sale with OriginalAuthGUID + ACIExt=RB
  → New BRIC returned: "09LMQ8877XWJEBZRPXP"
  → Update Storage BRIC (rolling token pattern)

Month 3: Next recurring payment
  → Use updated Storage BRIC from Month 2
  → Continue rolling pattern
```

**Common Errors:**

| Error | Cause | Solution |
|-------|-------|----------|
| `EH - CEM INVALID` | Used `AuthGUID` instead of `OriginalAuthGUID` | Use `OriginalAuthGUID` field |
| `EH - CEM INVALID` | Missing `ACI_EXT=RB` | Add `ACIExt: "RB"` |
| `12 - INVALID TRANS` | Using Financial BRIC instead of Storage BRIC | Convert to Storage BRIC first (CCE8) |
| `54 - EXPIRED CARD` | Card expired since Storage BRIC created | Card needs updating by customer |

**Other ACI_EXT Values:**

```go
// Installment Payment (e.g., "Pay in 4")
ACIExt: strPtr("IP")

// Delayed Sale (hotel, car rental after checkout)
ACIExt: strPtr("DS")

// No Show Charge (hotel)
ACIExt: strPtr("NS")

// Re-authorization (extending hold)
ACIExt: strPtr("RA")
```

---

## Response Codes

### AUTH_RESP Codes

| Code | Description | Action |
|------|-------------|--------|
| `00` | Approved | Transaction successful - proceed |
| `05` | Do not honor | Soft decline - customer should contact bank |
| `12` | Invalid transaction | Check request parameters |
| `14` | Invalid card number | Card number format incorrect |
| `41` | Lost card | Card reported lost - do not retry |
| `43` | Stolen card | Card reported stolen - do not retry |
| `51` | Insufficient funds | Customer has insufficient balance |
| `54` | Expired card | Card past expiration date |
| `61` | Exceeds withdrawal limit | Amount exceeds card limit |
| `65` | Exceeds withdrawal frequency | Too many transactions |
| `EH` | CEM Invalid | Card Entry Method error (check CARD_ENT_METH) |
| `RR` | Declined by EPX | Check AUTH_RESP_TEXT for reason |

### AVS Codes (Address Verification)

| Code | Description | Fraud Risk |
|------|-------------|------------|
| `Y` | Address and ZIP match | Low ✅ |
| `Z` | ZIP matches, address doesn't | Medium ⚠️ |
| `A` | Address matches, ZIP doesn't | Medium ⚠️ |
| `N` | No match | High ⚠️⚠️⚠️ |
| `U` | Unavailable | Unknown |
| `R` | Retry/system unavailable | Try again |
| `S` | Service not supported | N/A |

**Best Practice:**
- Approve: `Y`, `Z`, `A` (most transactions)
- Review: `N` (high fraud risk)
- Retry: `R`, `U`

### CVV Codes (Card Verification Value)

| Code | Description | Fraud Risk |
|------|-------------|------------|
| `M` | Match | Low ✅ |
| `N` | No match | High ⚠️⚠️⚠️ |
| `P` | Not processed | N/A |
| `S` | Should be on card but not indicated | Unknown |
| `U` | Unavailable | Unknown |

**Best Practice:**
- Approve: `M`
- Decline/Review: `N` (card security code incorrect)

---

## Error Handling

### Common Errors and Solutions

#### 1. Invalid TRAN_NBR[LEN]

**Error:** Transaction number too long

**Solution:**
```go
// ❌ Wrong - full timestamp
TranNbr: fmt.Sprintf("%d", time.Now().Unix())  // 1699234567 (10 digits)

// ✅ Correct - modulo to 5 digits
TranNbr: fmt.Sprintf("%d", time.Now().Unix()%100000)  // 34567 (5 digits)
```

Keep `TRAN_NBR` to 5-10 digits maximum.

#### 2. AUTH_RESP: RR - Invalid TRAN_TYPE

**Error:** Transaction type not recognized

**Solution:**
```go
// ❌ Wrong - old codes
TransactionType: "S"   // Old format

// ✅ Correct - EPX codes
TransactionType: "CCE1"  // Sale
TransactionType: "CCE2"  // Auth Only
TransactionType: "CCE4"  // Capture
TransactionType: "CCE9"  // Refund
TransactionType: "CCEX"  // Void
TransactionType: "CCE8"  // BRIC Storage
```

#### 3. AUTH_RESP: EH - CEM INVALID

**Error:** Card Entry Method invalid for transaction type

**Solution for Recurring Payments:**
```go
// ❌ Wrong - using AuthGUID
AuthGUID: storageAuthGUID,
CardEntryMethod: strPtr("Z"),

// ✅ Correct - using OriginalAuthGUID + ACI_EXT
OriginalAuthGUID: storageAuthGUID,
ACIExt:           strPtr("RB"),  // Recurring Billing
CardEntryMethod:  strPtr("Z"),
```

#### 4. AUTH_RESP: RR - Invalid AMOUNT

**Error:** Amount format incorrect

**Solution:**
```go
// ❌ Wrong
Amount: "10"      // Missing decimals
Amount: "$10.00"  // Currency symbol
Amount: "10.0"    // Only one decimal

// ✅ Correct
Amount: "10.00"   // Two decimal places
Amount: "0.00"    // For BRIC Storage
```

#### 5. AUTH_GUID Missing from Response

**Error:** Cannot parse response XML

**Solution:**
The issue is XML parsing. EPX uses:
```xml
<FIELD KEY="AUTH_GUID">value</FIELD>
```

Not direct tags like:
```xml
<AUTH_GUID>value</AUTH_GUID>
```

Use proper XML parsing with `EPXField` struct.

### Error Response Example

```go
response := &ports.ServerPostResponse{
    AuthGUID:     "",
    AuthResp:     "12",                // Invalid transaction
    AuthCode:     "",
    AuthRespText: "INVALID TRAN_TYPE", // Error description
    IsApproved:   false,
}
```

**How to Handle:**
```go
response, err := adapter.ProcessTransaction(ctx, request)
if err != nil {
    // Network or configuration error
    log.Printf("Transaction failed: %v", err)
    return err
}

if !response.IsApproved {
    // Transaction declined by bank/gateway
    log.Printf("Transaction declined: %s - %s",
        response.AuthResp,
        response.AuthRespText)

    // Check specific decline reason
    switch response.AuthResp {
    case "05":
        // Soft decline - ask customer to contact bank
    case "51":
        // Insufficient funds - suggest different payment method
    case "54":
        // Expired card - ask for updated card details
    case "EH":
        // Technical error - check request parameters
    }
}
```

---

## Best Practices

### 1. BRIC Token Management

**Rolling Token Pattern (Recommended):**
```go
// After each transaction, update stored token
func processRecurringPayment(customerID string, oldBRIC string, amount string) error {
    response, err := processTransaction(oldBRIC, amount)
    if err != nil {
        return err
    }

    // Save new BRIC for next billing cycle
    newBRIC := response.AuthGUID
    updateCustomerBRIC(customerID, newBRIC)

    return nil
}
```

**Why Rolling Tokens?**
- New BRIC after each transaction extends 13-month validity
- Reduces risk of expired tokens
- Card networks may auto-update card details

### 2. Transaction Uniqueness

**Generate Unique TRAN_NBR:**
```go
// Option 1: Timestamp-based (5 digits)
tranNbr := fmt.Sprintf("%d", time.Now().Unix()%100000)

// Option 2: UUID-based (truncated)
tranNbr := strings.Replace(uuid.New().String(), "-", "", -1)[:10]

// Option 3: Database sequence
tranNbr := fmt.Sprintf("%d", getNextTransactionID())
```

**Store Transaction History:**
```sql
CREATE TABLE transactions (
    id UUID PRIMARY KEY,
    tran_nbr VARCHAR(10) UNIQUE NOT NULL,
    auth_guid VARCHAR(20),
    customer_id VARCHAR(255),
    amount DECIMAL(10,2),
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW()
);
```

### 3. Idempotency

**Prevent Duplicate Transactions:**
```go
func processPayment(idempotencyKey string, request *PaymentRequest) (*Response, error) {
    // Check if already processed
    existing := getTransactionByIdempotencyKey(idempotencyKey)
    if existing != nil {
        return existing, nil  // Return cached response
    }

    // Process new transaction
    response, err := adapter.ProcessTransaction(ctx, request)
    if err != nil {
        return nil, err
    }

    // Cache response with idempotency key
    saveTransactionWithIdempotencyKey(idempotencyKey, response)

    return response, nil
}
```

### 4. Error Handling Strategy

**Retry Logic:**
```go
func processWithRetry(request *ServerPostRequest, maxRetries int) (*Response, error) {
    var lastErr error

    for attempt := 0; attempt < maxRetries; attempt++ {
        response, err := adapter.ProcessTransaction(ctx, request)

        if err == nil && response.IsApproved {
            return response, nil  // Success
        }

        // Don't retry hard declines
        if response != nil && shouldNotRetry(response.AuthResp) {
            return response, fmt.Errorf("declined: %s", response.AuthRespText)
        }

        lastErr = err
        time.Sleep(time.Second * time.Duration(attempt+1))  // Exponential backoff
    }

    return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func shouldNotRetry(authResp string) bool {
    // Don't retry these decline codes
    noRetry := []string{
        "41",  // Lost card
        "43",  // Stolen card
        "14",  // Invalid card number
        "54",  // Expired card
    }

    for _, code := range noRetry {
        if authResp == code {
            return true
        }
    }
    return false
}
```

### 5. Logging and Monitoring

**Comprehensive Logging:**
```go
func logTransaction(request *ServerPostRequest, response *ServerPostResponse, err error) {
    logger.Info("Transaction processed",
        zap.String("tran_type", string(request.TransactionType)),
        zap.String("tran_nbr", request.TranNbr),
        zap.String("amount", request.Amount),
        zap.String("auth_guid", response.AuthGUID),
        zap.String("auth_resp", response.AuthResp),
        zap.Bool("approved", response.IsApproved),
        zap.Error(err),
        zap.Duration("duration", time.Since(startTime)),
    )
}
```

**Monitor Key Metrics:**
- Transaction success rate
- Average response time
- Decline reasons distribution
- AVS/CVV mismatch rates

### 6. Security Best Practices

**PCI Compliance:**
```go
// ❌ NEVER log full card numbers
log.Printf("Processing card: %s", cardNumber)

// ✅ Log masked card numbers
func maskCard(cardNumber string) string {
    if len(cardNumber) < 10 {
        return "****"
    }
    return "****" + cardNumber[len(cardNumber)-4:]
}
log.Printf("Processing card: %s", maskCard(cardNumber))
```

**Store Only BRIC Tokens:**
```go
// ❌ NEVER store card details
type Customer struct {
    CardNumber     string  // DON'T DO THIS
    CVV            string  // DON'T DO THIS
    ExpirationDate string
}

// ✅ Store only BRIC tokens
type Customer struct {
    StorageBRIC           string
    NetworkTransactionID  string
    CardLast4             string  // For display only
    CardBrand             string  // V, M, A, D
    CardExpirationMonth   int
    CardExpirationYear    int
}
```

### 7. Testing

**Test Card Numbers:**
```go
// Visa
"4111111111111111"  // Approved
"4012888888881881"  // Approved

// Mastercard
"5499740000000057"  // Approved
"5555555555554444"  // Approved

// Amex
"378282246310005"   // Approved

// Discover
"6011000990139424"  // Approved

// Declined for testing
"4000000000000002"  // Declined - Do not honor
```

**Test Workflow:**
```bash
# 1. Run unit tests
go test ./internal/adapters/epx

# 2. Run integration tests (live EPX sandbox)
go test -tags=integration -v ./internal/adapters/epx

# 3. Test Browser Post (manual testing)
firefox test_browser_post.html
```

See [TESTING.md](TESTING.md) for complete testing guide.

---

## Complete Code Example

Here's a complete example processing a customer's first purchase and setting up recurring billing:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/kevin07696/payment-service/internal/adapters/epx"
    "github.com/kevin07696/payment-service/internal/adapters/ports"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewDevelopment()
    defer logger.Sync()

    // Initialize EPX adapter
    config := epx.DefaultServerPostConfig("sandbox")
    adapter := epx.NewServerPostAdapter(config, logger)
    ctx := context.Background()

    strPtr := func(s string) *string { return &s }

    // Step 1: Process initial sale
    fmt.Println("Step 1: Processing initial sale...")
    saleReq := &ports.ServerPostRequest{
        CustNbr:         "9001",
        MerchNbr:        "900300",
        DBAnbr:          "2",
        TerminalNbr:     "77",
        TransactionType: ports.TransactionTypeSale,
        Amount:          "29.99",
        TranNbr:         fmt.Sprintf("%d", time.Now().Unix()%100000),
        TranGroup:       fmt.Sprintf("%d", time.Now().Unix()%100000),
        PaymentType:     ports.PaymentMethodTypeCreditCard,
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

    saleResp, err := adapter.ProcessTransaction(ctx, saleReq)
    if err != nil || !saleResp.IsApproved {
        fmt.Printf("Sale failed: %v\n", err)
        return
    }

    fmt.Printf("✅ Sale approved: $29.99\n")
    fmt.Printf("   AUTH_GUID: %s\n", saleResp.AuthGUID)
    fmt.Printf("   AUTH_CODE: %s\n", saleResp.AuthCode)

    financialBRIC := saleResp.AuthGUID

    // Step 2: Convert to Storage BRIC for recurring billing
    time.Sleep(2 * time.Second)
    fmt.Println("\nStep 2: Converting to Storage BRIC...")

    bricReq := &ports.ServerPostRequest{
        CustNbr:          "9001",
        MerchNbr:         "900300",
        DBAnbr:           "2",
        TerminalNbr:      "77",
        TransactionType:  ports.TransactionTypeBRICStorageCC,
        Amount:           "0.00",  // Account Verification
        TranNbr:          fmt.Sprintf("%d", time.Now().Unix()%100000),
        TranGroup:        fmt.Sprintf("%d", time.Now().Unix()%100000),
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

    bricResp, err := adapter.ProcessTransaction(ctx, bricReq)
    if err != nil || !bricResp.IsApproved {
        fmt.Printf("BRIC Storage failed: %v\n", err)
        return
    }

    fmt.Printf("✅ Storage BRIC created\n")
    fmt.Printf("   Storage BRIC: %s\n", bricResp.AuthGUID)

    storageBRIC := bricResp.AuthGUID

    // Save to database (pseudocode)
    // saveCustomerPaymentMethod(customerID, storageBRIC, bricResp.NetworkTransactionID)

    // Step 3: Process recurring payment (e.g., next month)
    time.Sleep(2 * time.Second)
    fmt.Println("\nStep 3: Processing recurring payment...")

    recurringReq := &ports.ServerPostRequest{
        CustNbr:          "9001",
        MerchNbr:         "900300",
        DBAnbr:           "2",
        TerminalNbr:      "77",
        TransactionType:  ports.TransactionTypeSale,
        Amount:           "29.99",  // Monthly subscription
        TranNbr:          fmt.Sprintf("%d", time.Now().Unix()%100000),
        TranGroup:        fmt.Sprintf("%d", time.Now().Unix()%100000),
        OriginalAuthGUID: storageBRIC,  // Use Storage BRIC
        ACIExt:           strPtr("RB"),  // Recurring Billing
        CardEntryMethod:  strPtr("Z"),
        IndustryType:     strPtr("E"),
    }

    recurringResp, err := adapter.ProcessTransaction(ctx, recurringReq)
    if err != nil || !recurringResp.IsApproved {
        fmt.Printf("Recurring payment failed: %v\n", err)
        return
    }

    fmt.Printf("✅ Recurring payment approved: $29.99\n")
    fmt.Printf("   AUTH_GUID: %s\n", recurringResp.AuthGUID)
    fmt.Printf("   AUTH_CODE: %s\n", recurringResp.AuthCode)

    // Update stored BRIC for next billing cycle (rolling token)
    newStorageBRIC := recurringResp.AuthGUID
    // updateCustomerPaymentMethod(customerID, newStorageBRIC)

    fmt.Println("\n🎉 Complete workflow successful!")
    fmt.Println("Customer can now be billed monthly using the stored payment method.")
}
```

---

## Actual Test Output

This is the real output from running integration tests on **November 6, 2025**:

**Command:** `go test -tags=integration -v ./internal/adapters/epx`

```
╔════════════════════════════════════════════════════════════╗
║   EPX COMPREHENSIVE TRANSACTION TESTING SUITE             ║
╚════════════════════════════════════════════════════════════╝

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST 1: SALE (CCE1) - Auth + Capture
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-11-06T02:56:13.625-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCE1", "tran_nbr": "15773", "amount": "10.00"}
2025-11-06T02:56:13.903-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "277.805588ms", "body_length": 1399}
2025-11-06T02:56:13.903-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:13.903-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ886L2K2W11MPX1", "auth_resp": "00", "is_approved": true}
✅ Sale APPROVED
   AUTH_GUID: 09LMQ886L2K2W11MPX1
   AUTH_RESP: 00
   AUTH_CODE: 057579
   Response:  ZIP MATCH
   Card Type: V
   AVS:       Z
   CVV:       M

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST 2: AUTH-ONLY (CCE2) - Authorization without Capture
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-11-06T02:56:15.904-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCE2", "tran_nbr": "15775", "amount": "25.00"}
2025-11-06T02:56:16.154-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "249.407136ms", "body_length": 978}
2025-11-06T02:56:16.154-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:16.154-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ886N9RB1P9JPX3", "auth_resp": "00", "is_approved": true}
✅ Auth-Only APPROVED
   AUTH_GUID: 09LMQ886N9RB1P9JPX3
   AUTH_RESP: 00
   AUTH_CODE: 057580
   Response:  APPROVAL
   Card Type: M

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST 3: CAPTURE (CCE3) - Capture Previous Authorization
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-11-06T02:56:18.157-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCE4", "tran_nbr": "15778", "amount": "25.00"}
2025-11-06T02:56:18.622-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "464.81717ms", "body_length": 827}
2025-11-06T02:56:18.622-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:18.622-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ886PJFUZWJ4PX7", "auth_resp": "00", "is_approved": true}
✅ Capture APPROVED
   AUTH_GUID: 09LMQ886PJFUZWJ4PX7
   AUTH_RESP: 00
   AUTH_CODE:
   Response:  APPROVAL
   Card Type: M

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST 4: REFUND (CCE4) - Refund Original Sale
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-11-06T02:56:20.623-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCE9", "tran_nbr": "15780", "amount": "5.00"}
2025-11-06T02:56:23.396-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "2.773293261s", "body_length": 1365}
2025-11-06T02:56:23.396-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:23.397-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ886RV3TWKM3PXA", "auth_resp": "00", "is_approved": true}
✅ Refund APPROVED
   AUTH_GUID: 09LMQ886RV3TWKM3PXA
   AUTH_RESP: 00
   AUTH_CODE: 057581
   Response:  ZIP MATCH
   Card Type: V
   AVS:       Z

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST 5: VOID (CCE5) - Void Unsettled Transaction
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Creating a new sale to void...
2025-11-06T02:56:25.398-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCE1", "tran_nbr": "15785", "amount": "1.00"}
2025-11-06T02:56:25.817-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "419.324964ms", "body_length": 1372}
2025-11-06T02:56:25.817-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:25.817-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ8870L2BZPWYPXE", "auth_resp": "00", "is_approved": true}
✓ Sale created: AUTH_GUID = 09LMQ8870L2BZPWYPXE
2025-11-06T02:56:27.818-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCEX", "tran_nbr": "15787", "amount": "1.00"}
2025-11-06T02:56:28.080-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "262.261464ms", "body_length": 788}
2025-11-06T02:56:28.081-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:28.081-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ8872ZVNWJRFPXK", "auth_resp": "00", "is_approved": true}
✅ Void APPROVED
   AUTH_GUID: 09LMQ8872ZVNWJRFPXK
   AUTH_RESP: 00
   AUTH_CODE:
   Response:  APPROVAL

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST 6: BRIC STORAGE (CCE8) - Convert Financial BRIC to Storage BRIC
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Converting Financial BRIC to Storage BRIC for recurring payments...
2025-11-06T02:56:30.081-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCE8", "tran_nbr": "15790", "amount": "0.00"}
2025-11-06T02:56:30.483-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "401.825544ms", "body_length": 1222}
2025-11-06T02:56:30.483-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:30.483-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ88756L8UDTUPXN", "auth_resp": "00", "is_approved": true}
✅ BRIC Storage APPROVED
   AUTH_GUID: 09LMQ88756L8UDTUPXN
   AUTH_RESP: 00
   AUTH_CODE: 429745
   Response:  APPROVAL
   Card Type: V

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST 7: RECURRING PAYMENT - Sale with Storage BRIC
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
2025-11-06T02:56:32.483-0500	INFO	epx/server_post_adapter.go:108	Processing EPX Server Post transaction	{"transaction_type": "CCE1", "tran_nbr": "15792", "amount": "15.00"}
2025-11-06T02:56:32.924-0500	INFO	epx/server_post_adapter.go:163	Received Server Post response	{"status_code": 200, "elapsed": "440.975565ms", "body_length": 1371}
2025-11-06T02:56:32.924-0500	INFO	epx/server_post_adapter.go:501	Parsing as XML response
2025-11-06T02:56:32.924-0500	INFO	epx/server_post_adapter.go:179	Successfully processed Server Post transaction	{"auth_guid": "09LMQ8877XWJEBZRPXP", "auth_resp": "00", "is_approved": true}
✅ Recurring Payment APPROVED
   AUTH_GUID: 09LMQ8877XWJEBZRPXP
   AUTH_RESP: 00
   AUTH_CODE: 057583
   Response:  ADDRESS MATCH
   Card Type: V
   AVS:       A

╔════════════════════════════════════════════════════════════╗
║                    TEST SUMMARY                            ║
╚════════════════════════════════════════════════════════════╝

✅ All transaction types tested successfully!

Tested Transactions:
  1. ✓ Sale (CCE1) - Auth + Capture
  2. ✓ Auth-Only (CCE2)
  3. ✓ Capture (CCE3)
  4. ✓ Refund (CCE4)
  5. ✓ Void (CCE5)
  6. ✓ BRIC Storage (CCE8)
  7. ✓ Recurring Payment with Storage BRIC

Your EPX integration is fully operational! 🎉
```

### Key Observations from Test Output

**Performance Metrics:**
- Sale (CCE1): 277ms
- Auth-Only (CCE2): 249ms
- Capture (CCE4): 465ms
- Refund (CCE9): 2,773ms (2.7 seconds - longest)
- Void (CCEX): 262ms
- BRIC Storage (CCE8): 402ms
- Recurring Payment: 441ms

**Response Body Sizes:**
- Regular transactions: 1,371-1,399 bytes
- Auth-Only: 978 bytes (no capture data)
- Capture: 827 bytes
- Void: 788 bytes (minimal data)
- BRIC Storage: 1,222 bytes (includes NTID)

**All Transactions Received:**
- Status Code: 200 OK
- AUTH_RESP: 00 (Approved)
- Unique AUTH_GUID for each transaction
- Appropriate AUTH_CODE from bank

**Test Command:**
```bash
go test -tags=integration -v ./internal/adapters/epx
```

**Test Files:**
- Unit tests: `internal/adapters/epx/server_post_adapter_test.go`
- Integration tests: `internal/adapters/epx/integration_test.go`

---

## Summary

This API reference covers all 7 EPX Server Post transaction types:

1. **Sale (CCE1)** - Immediate authorization and capture
2. **Authorization Only (CCE2)** - Hold funds, capture later
3. **Capture (CCE4)** - Complete a previous authorization
4. **Refund (CCE9)** - Return money to customer
5. **Void (CCEX)** - Cancel same-day transaction
6. **BRIC Storage (CCE8)** - Create permanent payment token
7. **Recurring Payment** - Charge stored payment method

**Key Takeaways:**
- Always use unique `TRAN_NBR` (5-10 digits)
- Store Storage BRICs, not card details (PCI compliance)
- Use `OriginalAuthGUID` + `ACIExt=RB` for recurring payments
- Check `IsApproved` before processing orders
- Implement proper error handling and retry logic
- Log transactions for debugging and monitoring

**Test Successfully:**
All 7 transaction types have been tested and verified working in sandbox environment.

**Production Checklist:**
- [ ] Obtain production EPX credentials
- [ ] Update `EPX_BASE_URL` to production endpoint
- [ ] Configure HTTPS callback URLs
- [ ] Implement MAC signature validation
- [ ] Set up monitoring and alerting
- [ ] Configure production database with backups
- [ ] Conduct load testing
- [ ] Perform security audit
- [ ] Document rollback procedures

---

**Questions or Issues?**
- EPX Documentation: https://developer.north.com
- Support: info@theautobot.ai
- Test Environment: https://secure.epxuap.com
