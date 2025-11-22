# EPX API Reference

**Target Audience:** Developers understanding EPX payment gateway integration
**Topic:** EPX Server Post and Browser Post APIs used in payment-service
**Goal:** Reference for EPX transaction types, fields, and response codes we actually use

---

## Overview

This service integrates with EPX payment gateway using two methods:

1. **Server Post API** - Direct server-to-server for payment operations
2. **Browser Post API** - PCI-compliant hosted form for card entry

**EPX Environments:**
- **Sandbox:** `https://secure.epxuap.com`
- **Production:** `https://secure.epxnow.com`

---

## Server Post API

### Authentication (Required in Every Request)

| Field | Example | Notes |
|-------|---------|-------|
| `CUST_NBR` | `9001` | Customer number from EPX |
| `MERCH_NBR` | `900300` | Merchant number from EPX |
| `DBA_NBR` | `2` | DBA number from EPX |
| `TERMINAL_NBR` | `77` | Terminal number from EPX |

### Credit Card Transactions

| Transaction | Code | Use Case | Requires ORIG_AUTH_GUID |
|-------------|------|----------|-------------------------|
| **Sale** | CCE1 | Immediate payment (auth + capture) | No |
| **Authorization** | CCE2 | Hold funds for capture later | No |
| **Capture** | CCE4 | Capture previously authorized funds | Yes (auth GUID) |
| **Refund** | CCE9 | Return money to customer | Yes (sale/capture GUID) |
| **Void** | CCEX | Cancel unsettled transaction | Yes |
| **Reversal** | CCE7 | Void + release authorization | Yes |
| **BRIC Storage** | CCE8 | Tokenize card for recurring payments | Yes (Financial BRIC) or card details |

### ACH Checking Account Transactions

| Transaction | Code | Use Case | Requires Fields |
|-------------|------|----------|-----------------|
| **Pre-Note Debit** | CKC0 | Verify account before debit | Account, Routing, Name |
| **Pre-Note Credit** | CKC1 | Verify account before credit | Account, Routing, Name |
| **Debit/Sale** | CKC2 | Pull funds from account | Storage BRIC, Amount |
| **Credit/Refund** | CKC3 | Send funds to account | Storage BRIC, Amount |
| **BRIC Storage** | CKC8 | Tokenize account for recurring | Account, Routing or Financial BRIC |
| **Void** | CKCX | Cancel unsettled ACH transaction | Original GUID, Amount |

### ACH Savings Account Transactions

| Transaction | Code | Use Case |
|-------------|------|----------|
| **Pre-Note Debit** | CKS0 | Verify savings account |
| **Pre-Note Credit** | CKS1 | Verify savings account |
| **Debit/Sale** | CKS2 | Pull funds from savings |
| **Credit/Refund** | CKS3 | Send funds to savings |
| **Void** | CKSX | Cancel savings transaction |

---

## Browser Post API

### Purpose
PCI-compliant credit card entry - card data goes directly from customer's browser to EPX, never touching our server.

### Supported Operations
- **Sale** (TRAN_CODE=U) - Immediate payment
- **Auth Only** (TRAN_CODE=A) - Authorization for later capture

### Flow
1. Backend requests TAC (Temporary Access Code) from EPX Key Exchange API
2. Backend creates pending transaction in database
3. Frontend displays EPX-hosted form with TAC
4. Customer enters card details directly on EPX form
5. EPX processes transaction and redirects to callback URL
6. Backend receives Financial BRIC in callback
7. Backend converts Financial BRIC → Storage BRIC via CCE8 (if saving card)

### TAC Authentication
- TAC expires in 4 hours
- Cryptographically signed by EPX
- Prevents form tampering
- One-time use per transaction

---

## Common Request Fields

### Required in All Server Post Transactions

| Field | Format | Example | Notes |
|-------|--------|---------|-------|
| `TRAN_TYPE` | String(4) | `CCE1` | Transaction code from tables above |
| `AMOUNT` | Decimal | `10.00` | Dollars.cents (required except voids) |
| `TRAN_NBR` | String(5-10) | `12345` | Unique transaction identifier |
| `BATCH_ID` | String | `20250122` | Batch identifier (usually YYYYMMDD) |

### Credit Card Fields (for non-BRIC transactions)

| Field | Format | Example | Required |
|-------|--------|---------|----------|
| `ACCOUNT_NBR` | String(13-16) | `4111111111111111` | Yes* |
| `EXP_DATE` | MMYY | `1225` | Yes* |
| `CVV2` | String(3-4) | `123` | Recommended |
| `ZIP_CODE` | String | `10001` | For AVS verification |
| `CARD_ENT_METH` | String(1) | `E` | E=Ecommerce, Z=BRIC |

*Not required if using `AUTH_GUID` (BRIC token)

### ACH Fields (for non-BRIC transactions)

| Field | Format | Example | Required |
|-------|--------|---------|----------|
| `ACCOUNT_NBR` | String(10-12) | `1234567890` | Yes* |
| `ROUTING_NBR` | String(9) | `021000021` | Yes* |
| `RECEIVER_NAME` | String | `John Doe` | Yes* |
| `STD_ENTRY_CLASS` | String(3) | `PPD` | Yes |
| `RECEIVER_TYPE_CODE` | String(1) | `0` | Yes |

*Not required if using `AUTH_GUID` (BRIC token)

### BRIC Token Fields

| Field | Format | Example | Notes |
|-------|--------|---------|-------|
| `AUTH_GUID` | String(19-20) | `09LMQAABBCCDD` | Storage BRIC for recurring |
| `ORIG_AUTH_GUID` | String(19-20) | `09XYZFINANCIAL` | Financial BRIC to convert or void |
| `CARD_ENT_METH` | String(1) | `Z` | Must be 'Z' when using BRIC |

---

## Response Fields

### Standard Response Fields

| Field | Description | Example |
|-------|-------------|---------|
| `AUTH_RESP` | Authorization response code | `00` (approved) |
| `AUTH_RESP_TEXT` | Human-readable response | `APPROVED` |
| `AUTH_CODE` | Bank authorization code | `123456` |
| `AUTH_GUID` | BRIC token (Financial or Storage) | `09LMQAABBCCDD` |
| `TRAN_NBR` | Transaction number echoed back | `12345` |

### Common Response Codes

| Code | Description | Retry? | Action |
|------|-------------|--------|--------|
| `00` | **Approved** | N/A | Success |
| `85` | **Not Declined** (treat as approval) | N/A | Success |
| `05` | Do not honor | No | Card restricted/closed |
| `14` | Invalid card number | No | Fix card number |
| `51` | Insufficient funds | Yes (1 min) | Retry later |
| `61` | Exceeds limit | Yes (24h) | Retry next day |
| `91` | Issuer unavailable | Yes (exp backoff) | Bank temporarily down |

### AVS Response Codes

| Code | Meaning | Accept? |
|------|---------|---------|
| `Y` | Address and ZIP match | ✅ Yes |
| `Z` | ZIP matches only | ✅ Yes |
| `A` | Address matches only | ⚠️ Review |
| `N` | No match | ❌ Decline |
| `U` | Unavailable | ⚠️ Caution |

### CVV2 Response Codes

| Code | Meaning | Accept? |
|------|---------|---------|
| `M` | Match | ✅ Yes |
| `N` | No match | ❌ Decline |
| `P` | Not processed | ⚠️ Caution |
| `U` | Unavailable | ⚠️ Caution |

---

## BRIC Tokens

### Financial BRIC
- **Created by:** Any transaction (Sale, Auth, etc.)
- **Lifetime:** 13 months
- **Use case:** Refunds, voids, one-time capture
- **Format:** 19-20 character alphanumeric

### Storage BRIC
- **Created by:** CCE8 (Credit Card) or CKC8 (ACH)
- **Lifetime:** Never expires
- **Use case:** Recurring payments, subscriptions
- **Requires:** Account Verification ($0.00 auth for cards, Pre-Note for ACH)
- **Format:** 19-20 character alphanumeric

### Converting Financial → Storage BRIC

**Credit Card (CCE8):**
```
TRAN_TYPE=CCE8
ORIG_AUTH_GUID=[Financial BRIC from previous transaction]
CARD_ENT_METH=Z
ADDRESS, CITY, STATE, ZIP_CODE (required for Account Verification)
FIRST_NAME, LAST_NAME
```

**ACH (CKC8):**
```
TRAN_TYPE=CKC8
ORIG_AUTH_GUID=[Financial BRIC from CKC0 pre-note]
CARD_ENT_METH=Z
```

---

## Standard Entry Class Codes (ACH)

| Code | Description | Use Case |
|------|-------------|----------|
| `PPD` | Prearranged Payment/Deposit | Consumer recurring (subscriptions) |
| `CCD` | Corporate Credit/Debit | B2B payments |
| `WEB` | Internet-Initiated | Online bill pay |
| `TEL` | Telephone-Initiated | Phone authorizations |

**Recommendation:** Use PPD for consumer subscriptions (lower return rate ~0.5% vs WEB ~1-2%)

---

## Card Entry Methods

| Code | Description | Use Case |
|------|-------------|----------|
| `E` | Ecommerce | Manual card entry (Server Post with raw card data) |
| `Z` | BRIC-based | Using tokenized payment method (AUTH_GUID) |
| `X` | Browser Post | Customer enters card on EPX-hosted form |

---

## Account Verification

### Credit Cards (CCE8)
- $0.00 authorization sent to card network
- Validates: Card active, address matches, CVV matches
- Returns: Network Transaction ID (NTID)
- **NTID required** for card-on-file compliance (all recurring transactions)

### ACH (CKC0/CKC1)
- $0.00 pre-note transaction
- Validates: Account exists and can accept debits/credits
- Clears in 1-3 business days
- **NACHA requirement:** Must verify before recurring ACH debits

---

## Idempotency

- Use unique `TRAN_NBR` for each request
- EPX does not provide built-in idempotency
- **Our implementation:** Store transaction ID before EPX request, check for duplicates

---

## Testing

### Test Credit Cards (Sandbox Only)

| Card Number | Brand | Result |
|-------------|-------|--------|
| `4111111111111111` | Visa | Approved (00) |
| `5499740000000057` | Mastercard | Approved (00) |
| `340000000000009` | Amex | Approved (00) |
| `4000300011112220` | Visa | Declined (05) |

### Test ACH Accounts (Sandbox Only)

| Account | Routing | Result |
|---------|---------|--------|
| Any valid 10-12 digits | `021000021` | Approved |
| Any | `000000000` | Declined |

---

## Chargeback Management (North API)

**Note:** We integrate with North Payment Solutions for chargeback/dispute management.

### Read-Only Operations
- `SearchDisputes` - List/filter chargebacks
- `GetChargeback` - Retrieve chargeback details

### Response via Portal
- Merchants must respond to chargebacks through North's web portal
- North API does not provide write operations for dispute responses

### Chargeback Statuses
- `new` - Just received
- `pending` - Under review
- `responded` - Merchant submitted response
- `won` - Merchant prevailed
- `lost` - Chargeback upheld
- `accepted` - Merchant accepted liability

---

## Error Handling Best Practices

### Retry Strategy

**Retryable (with backoff):**
- `51` - Insufficient funds (retry after 60s)
- `61` - Exceeds limit (retry next day)
- `91` - Issuer unavailable (exponential backoff: 2s, 4s, 8s, 16s)

**Non-Retryable:**
- `05` - Do not honor (card restricted)
- `14` - Invalid card (fix data)
- `41`/`43` - Lost/stolen card
- Any `00`/`85` - Already approved

### Timeout Recommendations
- Connection timeout: 10 seconds
- Read timeout: 30 seconds
- Max retries: 3 (for retryable errors only)

---

## Related Documentation

- **[Payment CLI](PAYMENT_CLI.md)** - Setting up merchants and credentials
- **[Browser Post Form Setup](BROWSER_POST_FORM_SETUP.md)** - PCI-compliant integration
- **[API Specs](API_SPECS.md)** - ConnectRPC/gRPC payment service APIs
- **EPX Official Docs** (in supplemental-resources/):
  - EPX API - Server Post.pdf
  - EPX API - Browser Post.pdf
  - EPX Data Dictionary.pdf
  - EPX Reference - BRICs.pdf

---

**Last Updated:** 2025-11-22
**Based on:** Payment Service codebase analysis
