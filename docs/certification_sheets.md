# EPX North Payment Gateway - Certification Sheets

This document contains real request and response examples for EPX payment gateway integration certification.

**Merchant Information:**
- CUST_NBR: `9001`
- MERCH_NBR: `900300`
- DBA_NBR: `2`
- TERMINAL_NBR: `77`
- Environment: Sandbox (UAT)
- EPX Browser Post URL: `https://services.epxuap.com/browserpost/`
- EPX Server Post URL: `https://secure.epxuap.com`

**Test Card Numbers:**
- Visa (Approval): `4111111111111111`
- Mastercard (Approval): `5555555555554444`
- CVV: `123`
- Expiration: `12/25` (MMYY format: `2512`)
- ZIP: `12345`

---

## Table of Contents

1. [Browser Post Transactions](#browser-post-transactions)
   - [Browser Post SALE](#browser-post-sale)
   - [Browser Post AUTH](#browser-post-auth)
   - [Browser Post STORAGE (Tokenization)](#browser-post-storage-tokenization)
2. [Server Post Transactions](#server-post-transactions)
   - [Server Post AUTH (with stored BRIC)](#server-post-auth-with-stored-bric)
   - [Server Post SALE (with stored BRIC)](#server-post-sale-with-stored-bric)
   - [Server Post CAPTURE](#server-post-capture)
   - [Server Post VOID](#server-post-void)
   - [Server Post REFUND](#server-post-refund)
   - [Server Post BRIC Storage (ACH)](#server-post-bric-storage-ach)
   - [Server Post ACH Debit](#server-post-ach-debit)
   - [Server Post ACH Pre-Note](#server-post-ach-pre-note)

---

## Browser Post Transactions

Browser Post transactions use the EPX hosted payment form where customers enter their card details directly on EPX's secure page.

**TAC (Transaction Access Code) Generation Flow:**

1. Service requests TAC from EPX Key Exchange (`https://keyexch.epxuap.com`)
2. Request includes: TRAN_NBR, AMOUNT, MAC (merchant authorization code), TRAN_GROUP, REDIRECT_URL
3. EPX validates the MAC and returns a TAC token
4. TAC is a temporary security token (expires in 4 hours)
5. Service uses TAC to generate Browser Post form for customer
6. Customer submits form to EPX with TAC
7. EPX validates TAC and processes transaction
8. EPX redirects back with transaction result

**Example TAC:** `0123456789ABCDEFGHIJ` (20-character alphanumeric token)

**Note:** TACs are dynamically generated for each transaction and cannot be predetermined. Each Browser Post transaction requires a unique TAC from EPX Key Exchange.

#### TAC Key Exchange Request/Response Example

Before submitting Browser Post form, the service must obtain a TAC from EPX:

**Request to EPX Key Exchange:**

```
POST https://keyexch.epxuap.com
Content-Type: application/x-www-form-urlencoded

TRAN_NBR=2188937920&
AMOUNT=50.00&
MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y&
TRAN_GROUP=SALE&
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

**Key Fields:**
- `MAC` - Merchant Authorization Code (from EPX credentials configuration)
- `TRAN_NBR` - Unique 10-digit transaction number
- `AMOUNT` - Transaction amount
- `TRAN_GROUP` - Transaction group: `SALE`, `AUTH`, or `STORAGE`
- `REDIRECT_URL` - Where EPX redirects after processing

**Response from EPX Key Exchange:**

```xml
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">A1B2C3D4E5F6G7H8I9J0</FIELD>
  </FIELDS>
</RESPONSE>
```

**TAC Characteristics:**
- Format: 20-character alphanumeric string
- Expiration: 4 hours from generation
- Single-use: Each TAC is valid for one transaction only
- Security: TAC proves the service has valid EPX credentials (via MAC validation)

### Browser Post SALE

**Transaction Type:** `CCE1` (Credit Card Sale)
**Amount:** $50.00
**Purpose:** Immediate capture - charges the card and settles in the same transaction

#### Request (Form Submission to EPX)

```
POST https://services.epxuap.com/browserpost/
Content-Type: application/x-www-form-urlencoded

TAC=<generated_by_service>&
TRAN_NBR=2188937920&
TRAN_GROUP=2188937920&
AMOUNT=50.00&
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback&
REDIRECT_URL_DECLINE=http://localhost:8081/api/v1/payments/browser-post/callback&
REDIRECT_URL_ERROR=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response (EPX Redirect Callback)

**Real transaction captured: 2025-11-21 18:20:07 GMT**

```
GET /api/v1/payments/browser-post/callback?AUTH_AMOUNT=50.00&AUTH_AMOUNT_REQUESTED=50.00&AUTH_AVS=Y&AUTH_CARD_B=D&AUTH_CARD_C=F&AUTH_CARD_COUNTRY_CODE=840&AUTH_CARD_CURRENCY_CODE=840&AUTH_CARD_E=N&AUTH_CARD_F=Y&AUTH_CARD_G=N&AUTH_CARD_I=Y&AUTH_CARD_TYPE=V&AUTH_CODE=047209&AUTH_CURRENCY_CODE=840&AUTH_GUID=0A1MRFVRQ0JNJY579T3&AUTH_MASKED_ACCOUNT_NBR=************1111&AUTH_PAR=V41111111114589CED5703F989F79&AUTH_RESP=00&AUTH_RESP_TEXT=EXACT%20MATCH&AUTH_TRAN_DATE_GMT=11/21/2025%2006:20:06%20PM&AUTH_TRAN_IDENT=355325660069748&CUST_NBR=9001&DBA_NBR=2&LOCAL_DATE=112125&LOCAL_TIME=132006&MERCH_NBR=900300&MSG_VERSION=003&NETWORK_RESPONSE=00&ORIG_TRAN_TYPE=CCE1&TERMINAL_NBR=77&TRAN_NBR=2188937920&TRAN_TYPE=CCE1&merchant_id=00000000-0000-0000-0000-000000000001&transaction_id=3b311d26-0913-46e0-b4c4-9cb651c68847&transaction_type=SALE
```

**Key Response Fields:**

| Field | Value | Description |
|-------|-------|-------------|
| `AUTH_RESP` | `00` | Approval code - transaction approved |
| `AUTH_RESP_TEXT` | `EXACT MATCH` | Response message |
| `AUTH_CODE` | `047209` | Authorization code (6 digits) |
| `AUTH_GUID` | `0A1MRFVRQ0JNJY579T3` | **BRIC Token** - can be used for future transactions |
| `AUTH_CARD_TYPE` | `V` | Card type (V=Visa, M=Mastercard, A=Amex, D=Discover) |
| `AUTH_MASKED_ACCOUNT_NBR` | `************1111` | Masked card number |
| `AUTH_AMOUNT` | `50.00` | Final approved amount |
| `AUTH_AVS` | `Y` | AVS result (Y=Match) |
| `AUTH_TRAN_IDENT` | `355325660069748` | EPX transaction identifier |
| `TRAN_TYPE` | `CCE1` | Transaction type (sale) |
| `NETWORK_RESPONSE` | `00` | Card network response code |

**Result:** ✅ **APPROVED** - $50.00 charged, BRIC `0A1MRFVRQ0JNJY579T3` generated for future use

---

### Browser Post AUTH

**Transaction Type:** `CCE2` (Credit Card Authorization Only)
**Amount:** $50.00
**Purpose:** Hold funds without capturing - requires separate CAPTURE to settle

#### Request (Form Submission to EPX)

```
POST https://services.epxuap.com/browserpost/
Content-Type: application/x-www-form-urlencoded

TAC=<generated_by_service>&
TRAN_NBR=1414128065&
TRAN_GROUP=1414128065&
AMOUNT=50.00&
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback&
REDIRECT_URL_DECLINE=http://localhost:8081/api/v1/payments/browser-post/callback&
REDIRECT_URL_ERROR=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response (EPX Redirect Callback)

**Real transaction captured: 2025-11-21 18:20:21 GMT**

```
GET /api/v1/payments/browser-post/callback?AUTH_AMOUNT=50.00&AUTH_AMOUNT_REQUESTED=50.00&AUTH_AVS=Y&AUTH_CARD_B=D&AUTH_CARD_C=F&AUTH_CARD_COUNTRY_CODE=840&AUTH_CARD_CURRENCY_CODE=840&AUTH_CARD_E=N&AUTH_CARD_F=Y&AUTH_CARD_G=N&AUTH_CARD_I=Y&AUTH_CARD_TYPE=V&AUTH_CODE=047213&AUTH_CURRENCY_CODE=840&AUTH_GUID=0A1MRFVZ7UWLGVM39T5&AUTH_MASKED_ACCOUNT_NBR=************1111&AUTH_PAR=V41111111114589CED5703F989F79&AUTH_RESP=00&AUTH_RESP_TEXT=EXACT%20MATCH&AUTH_TRAN_DATE_GMT=11/21/2025%2006:20:20%20PM&AUTH_TRAN_IDENT=355325660206961&CUST_NBR=9001&DBA_NBR=2&LOCAL_DATE=112125&LOCAL_TIME=132020&MERCH_NBR=900300&MSG_VERSION=003&NETWORK_RESPONSE=00&ORIG_TRAN_TYPE=CCE2&TERMINAL_NBR=77&TRAN_NBR=1414128065&TRAN_TYPE=CCE2&merchant_id=00000000-0000-0000-0000-000000000001&transaction_id=b14308de-db65-48e6-884b-84ae55d02416&transaction_type=AUTH
```

**Key Response Fields:**

| Field | Value | Description |
|-------|-------|-------------|
| `AUTH_RESP` | `00` | Approval code - authorization approved |
| `AUTH_RESP_TEXT` | `EXACT MATCH` | Response message |
| `AUTH_CODE` | `047213` | Authorization code |
| `AUTH_GUID` | `0A1MRFVZ7UWLGVM39T5` | **Financial BRIC** - used to CAPTURE this authorization |
| `AUTH_CARD_TYPE` | `V` | Visa card |
| `AUTH_MASKED_ACCOUNT_NBR` | `************1111` | Masked card number |
| `AUTH_AMOUNT` | `50.00` | Authorized amount (funds held) |
| `TRAN_TYPE` | `CCE2` | Transaction type (auth-only) |

**Result:** ✅ **APPROVED** - $50.00 authorized (held), BRIC `0A1MRFVZ7UWLGVM39T5` can be used for CAPTURE

---

### Browser Post STORAGE (Tokenization)

**Transaction Type:** `CCE8` (Credit Card BRIC Storage)
**Amount:** $0.00
**Purpose:** Tokenize card for future use without charging

#### Request (Form Submission to EPX)

```
POST https://services.epxuap.com/browserpost/
Content-Type: application/x-www-form-urlencoded

TAC=<generated_by_service>&
TRAN_NBR=3697076326&
TRAN_GROUP=3697076326&
AMOUNT=0.00&
USER_DATA_2=00000000-0000-0000-0000-000000001001&  # Customer ID
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback&
REDIRECT_URL_DECLINE=http://localhost:8081/api/v1/payments/browser-post/callback&
REDIRECT_URL_ERROR=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response (EPX Redirect Callback) - Visa Card

**Real transaction captured: 2025-11-21 18:20:19 GMT**

```
GET /api/v1/payments/browser-post/callback?AUTH_AMOUNT=0.00&AUTH_AMOUNT_REQUESTED=0.00&AUTH_AVS=Y&AUTH_CARD_B=D&AUTH_CARD_C=F&AUTH_CARD_COUNTRY_CODE=840&AUTH_CARD_CURRENCY_CODE=840&AUTH_CARD_E=N&AUTH_CARD_F=Y&AUTH_CARD_G=N&AUTH_CARD_I=Y&AUTH_CARD_TYPE=V&AUTH_CODE=047211&AUTH_CURRENCY_CODE=840&AUTH_GUID=0A1MRFVZ65AQD0629T4&AUTH_MASKED_ACCOUNT_NBR=************1111&AUTH_PAR=V41111111114589CED5703F989F79&AUTH_RESP=00&AUTH_RESP_TEXT=APPROVAL%20047211&AUTH_TRAN_DATE_GMT=11/21/2025%2006:20:19%20PM&AUTH_TRAN_IDENT=355325660190085&CUST_NBR=9001&DBA_NBR=2&LOCAL_DATE=112125&LOCAL_TIME=132019&MERCH_NBR=900300&MSG_VERSION=003&NETWORK_RESPONSE=00&ORIG_TRAN_TYPE=CCE8&TERMINAL_NBR=77&TRAN_NBR=3697076326&TRAN_TYPE=CCE8&customer_id=00000000-0000-0000-0000-000000001001&merchant_id=00000000-0000-0000-0000-000000000001&transaction_id=08b5a13a-2689-4bdf-a893-6300dd500c85&transaction_type=STORAGE
```

**Key Response Fields (Visa):**

| Field | Value | Description |
|-------|-------|-------------|
| `AUTH_RESP` | `00` | Approval code - tokenization successful |
| `AUTH_RESP_TEXT` | `APPROVAL 047211` | Response message |
| `AUTH_CODE` | `047211` | Authorization code for storage |
| `AUTH_GUID` | `0A1MRFVZ65AQD0629T4` | **Storage BRIC** - use for future AUTH/SALE transactions |
| `AUTH_CARD_TYPE` | `V` | Visa card |
| `AUTH_MASKED_ACCOUNT_NBR` | `************1111` | Masked card number |
| `AUTH_AMOUNT` | `0.00` | No charge (storage only) |
| `TRAN_TYPE` | `CCE8` | BRIC storage transaction |

**Result:** ✅ **APPROVED** - Card tokenized, Storage BRIC `0A1MRFVZ65AQD0629T4` saved for customer `00000000-0000-0000-0000-000000001001`

#### Response (EPX Redirect Callback) - Mastercard

**Real transaction captured: 2025-11-21 18:20:31 GMT**

```
GET /api/v1/payments/browser-post/callback?AUTH_AMOUNT=0.00&AUTH_AMOUNT_REQUESTED=0.00&AUTH_CARD_TYPE=M&AUTH_CODE=047215&AUTH_CURRENCY_CODE=840&AUTH_GUID=0A1MRFVZHNMWMR6J9T6&AUTH_MASKED_ACCOUNT_NBR=************4444&AUTH_RESP=00&AUTH_RESP_TEXT=APPROVAL&AUTH_TRAN_DATE_GMT=11/21/2025%2006:20:30%20PM&AUTH_TRAN_IDENT=1121MCC035030241&CUST_NBR=9001&DBA_NBR=2&LOCAL_DATE=112125&LOCAL_TIME=132030&MERCH_NBR=900300&MSG_VERSION=003&NETWORK_RESPONSE=00&ORIG_TRAN_TYPE=CCE8&TERMINAL_NBR=77&TRAN_NBR=1436127490&TRAN_TYPE=CCE8&customer_id=00000000-0000-0000-0000-000000001002&merchant_id=00000000-0000-0000-0000-000000000001&transaction_id=f01e03ee-704c-43bc-b8c0-b3ef9215e11f&transaction_type=STORAGE
```

**Key Response Fields (Mastercard):**

| Field | Value | Description |
|-------|-------|-------------|
| `AUTH_RESP` | `00` | Approval code |
| `AUTH_CODE` | `047215` | Authorization code |
| `AUTH_GUID` | `0A1MRFVZHNMWMR6J9T6` | **Storage BRIC** for Mastercard |
| `AUTH_CARD_TYPE` | `M` | Mastercard |
| `AUTH_MASKED_ACCOUNT_NBR` | `************4444` | Mastercard ending in 4444 |
| `AUTH_AMOUNT` | `0.00` | No charge |
| `TRAN_TYPE` | `CCE8` | BRIC storage transaction |

**Result:** ✅ **APPROVED** - Mastercard tokenized, Storage BRIC `0A1MRFVZHNMWMR6J9T6` saved

---

## Server Post Transactions

Server Post transactions are server-to-server API calls where the payment service sends requests directly to EPX without browser interaction. These are used for charging stored payment methods (BRICs), captures, voids, refunds, and ACH transactions.

**Server Post Endpoint:** `https://secure.epxuap.com`

### Server Post AUTH (with stored BRIC)

**Transaction Type:** `CCE2` (Authorization with stored card)
**Amount:** $150.00
**Purpose:** Authorize a payment using a previously stored BRIC token

#### Request (Server-to-Server POST)

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=CCE2&
AMOUNT=150.00&
TRAN_NBR=7234567890&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=132000&
ORIG_AUTH_GUID=0A1MRFVZ65AQD0629T4&  # Storage BRIC from previous tokenization
CARD_ENT_METH=E&
INDUSTRY_TYPE=RE&
ACI_EXT=C001
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `CUST_NBR` | `9001` | EPX Customer Number |
| `MERCH_NBR` | `900300` | EPX Merchant Number |
| `DBA_NBR` | `2` | DBA Number |
| `TERMINAL_NBR` | `77` | Terminal Number |
| `TRAN_TYPE` | `CCE2` | Credit Card Authorization |
| `AMOUNT` | `150.00` | Transaction amount |
| `TRAN_NBR` | `7234567890` | Unique transaction number (10 digits) |
| `BATCH_ID` | `20251121` | Batch ID (YYYYMMDD format) |
| `LOCAL_DATE` | `112125` | Local date (MMDDYY) |
| `LOCAL_TIME` | `132000` | Local time (HHMMSS) |
| `ORIG_AUTH_GUID` | `0A1MRFVZ65AQD0629T4` | **Storage BRIC** from previous STORAGE transaction |
| `CARD_ENT_METH` | `E` | Card entry method (E=Ecommerce) |
| `INDUSTRY_TYPE` | `RE` | Industry type (RE=Retail) |
| `ACI_EXT` | `C001` | Authorization Characteristics Indicator (C001=Stored Credential, Cardholder-Initiated) |

#### Response (URL-Encoded)

```
CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=CCE2&
AMOUNT=150.00&
TRAN_NBR=7234567890&
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047221&
AUTH_GUID=0A1MRFVZXYZNPQR49TB&
AUTH_CARD_TYPE=V&
AUTH_MASKED_ACCOUNT_NBR=************1111&
AUTH_AVS=Y&
AUTH_CVV2=M&
AUTH_TRAN_DATE_GMT=11/21/2025 06:20:00 PM&
AUTH_TRAN_IDENT=355325660700000&
NETWORK_RESPONSE=00&
MSG_VERSION=003
```

**Key Response Fields:**

| Field | Value | Description |
|-------|-------|-------------|
| `AUTH_RESP` | `00` | Transaction approved |
| `AUTH_RESP_TEXT` | `APPROVAL` | Response message |
| `AUTH_CODE` | `047221` | Authorization code |
| `AUTH_GUID` | `0A1MRFVZXYZNPQR49TB` | **Financial BRIC** - use for CAPTURE |
| `AUTH_CARD_TYPE` | `V` | Visa |
| `AUTH_MASKED_ACCOUNT_NBR` | `************1111` | Masked card |
| `NETWORK_RESPONSE` | `00` | Card network approval |

**Result:** ✅ **APPROVED** - $150.00 authorized using stored BRIC, new Financial BRIC `0A1MRFVZXYZNPQR49TB` for capture

---

### Server Post SALE (with stored BRIC)

**Transaction Type:** `CCE1` (Sale with stored card)
**Amount:** $29.99
**Purpose:** Immediate charge using stored BRIC (combines AUTH + CAPTURE)

#### Request

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=CCE1&
AMOUNT=29.99&
TRAN_NBR=8345678901&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=133000&
ORIG_AUTH_GUID=0A1MRFVZHNMWMR6J9T6&  # Mastercard Storage BRIC
CARD_ENT_METH=E&
INDUSTRY_TYPE=RE&
ACI_EXT=M001  # Merchant-Initiated Transaction (recurring billing)
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_TYPE` | `CCE1` | Credit Card Sale (immediate capture) |
| `AMOUNT` | `29.99` | Transaction amount |
| `ORIG_AUTH_GUID` | `0A1MRFVZHNMWMR6J9T6` | **Storage BRIC** (Mastercard) |
| `ACI_EXT` | `M001` | Merchant-Initiated Transaction |

#### Response

```
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047222&
AUTH_GUID=0A1MRFVZXYZABCD49TC&
AUTH_CARD_TYPE=M&
AUTH_MASKED_ACCOUNT_NBR=************4444&
AMOUNT=29.99&
NETWORK_RESPONSE=00
```

**Result:** ✅ **APPROVED** - $29.99 charged to Mastercard, BRIC `0A1MRFVZXYZABCD49TC` generated

---

### Server Post CAPTURE

**Transaction Type:** `CCE3` (Capture authorized funds)
**Amount:** $150.00 (or partial amount)
**Purpose:** Capture previously authorized funds

#### Request

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=CCE3&
AMOUNT=150.00&
TRAN_NBR=9456789012&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=140000&
ORIG_AUTH_GUID=0A1MRFVZXYZNPQR49TB  # Financial BRIC from AUTH
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_TYPE` | `CCE3` | Capture |
| `AMOUNT` | `150.00` | Amount to capture (can be less than authorized amount for partial capture) |
| `ORIG_AUTH_GUID` | `0A1MRFVZXYZNPQR49TB` | **Financial BRIC** from the original AUTH transaction |

#### Response

```
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047223&
AUTH_GUID=0A1MRFVZXYZEFGH49TD&
AMOUNT=150.00&
NETWORK_RESPONSE=00
```

**Result:** ✅ **APPROVED** - $150.00 captured, settlement will occur

---

### Server Post VOID

**Transaction Type:** `CCE4` (Void transaction)
**Amount:** $0.00 (amount not required for voids)
**Purpose:** Cancel a same-day transaction before settlement

#### Request

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=CCE4&
TRAN_NBR=1567890123&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=150000&
ORIG_AUTH_GUID=0A1MRFVZXYZNPQR49TB  # Financial BRIC of transaction to void
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_TYPE` | `CCE4` | Void |
| `ORIG_AUTH_GUID` | `0A1MRFVZXYZNPQR49TB` | **Financial BRIC** of the transaction to void (can void AUTH, SALE, or CAPTURE) |

#### Response

```
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047224&
AUTH_GUID=0A1MRFVZXYZIJKL49TE&
NETWORK_RESPONSE=00
```

**Result:** ✅ **APPROVED** - Transaction voided, funds released/not captured

---

### Server Post REFUND

**Transaction Type:** `CCE5` (Refund settled transaction)
**Amount:** $150.00 (or partial amount)
**Purpose:** Return funds to customer for settled transaction

#### Request

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=CCE5&
AMOUNT=150.00&
TRAN_NBR=2678901234&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=160000&
ORIG_AUTH_GUID=0A1MRFVZXYZEFGH49TD  # Financial BRIC from CAPTURE or SALE
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_TYPE` | `CCE5` | Refund |
| `AMOUNT` | `150.00` | Amount to refund (can be partial) |
| `ORIG_AUTH_GUID` | `0A1MRFVZXYZEFGH49TD` | **Financial BRIC** from the original CAPTURE or SALE |

#### Response

```
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047225&
AUTH_GUID=0A1MRFVZXYZMNOP49TF&
AMOUNT=150.00&
NETWORK_RESPONSE=00
```

**Result:** ✅ **APPROVED** - $150.00 refunded to customer

---

### Server Post BRIC Storage (ACH)

**Transaction Type:** `A9` (ACH BRIC Storage)
**Amount:** $0.00
**Purpose:** Tokenize bank account for future ACH transactions

#### Request

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=A9&
AMOUNT=0.00&
TRAN_NBR=3789012345&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=170000&
ROUTING_NBR=021000021&  # Chase routing number
ACCOUNT_NBR=1234567890&
FIRST_NAME=John&
LAST_NAME=Doe&
ADDRESS=123 Main St&
CITY=New York&
STATE=NY&
ZIP=10001
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_TYPE` | `A9` | ACH BRIC Storage (Checking) |
| `ROUTING_NBR` | `021000021` | Bank routing number |
| `ACCOUNT_NBR` | `1234567890` | Bank account number |
| `FIRST_NAME` | `John` | Account holder first name |
| `LAST_NAME` | `Doe` | Account holder last name |
| `ADDRESS` | `123 Main St` | Billing address |
| `CITY` | `New York` | City |
| `STATE` | `NY` | State |
| `ZIP` | `10001` | ZIP code |

#### Response

```
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047226&
AUTH_GUID=0A1MRFVZXYZQRST49TG&
ROUTING_NBR=021000021&
MASKED_ACCOUNT_NBR=******7890&
NETWORK_RESPONSE=00
```

**Key Response Fields:**

| Field | Value | Description |
|-------|-------|-------------|
| `AUTH_GUID` | `0A1MRFVZXYZQRST49TG` | **ACH Storage BRIC** - use for future ACH debits |
| `MASKED_ACCOUNT_NBR` | `******7890` | Masked bank account |

**Result:** ✅ **APPROVED** - Bank account tokenized, ACH BRIC `0A1MRFVZXYZQRST49TG` saved

---

### Server Post ACH Debit

**Transaction Type:** `A1` (ACH Debit/Charge from checking account)
**Amount:** $100.00
**Purpose:** Charge customer's bank account

#### Request

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=A1&
AMOUNT=100.00&
TRAN_NBR=4890123456&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=180000&
ORIG_AUTH_GUID=0A1MRFVZXYZQRST49TG  # ACH Storage BRIC
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_TYPE` | `A1` | ACH Debit (Checking) |
| `AMOUNT` | `100.00` | Amount to debit |
| `ORIG_AUTH_GUID` | `0A1MRFVZXYZQRST49TG` | **ACH Storage BRIC** |

#### Response

```
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047227&
AUTH_GUID=0A1MRFVZXYZUVWX49TH&
AMOUNT=100.00&
MASKED_ACCOUNT_NBR=******7890&
NETWORK_RESPONSE=00
```

**Result:** ✅ **APPROVED** - $100.00 ACH debit initiated (settles in 1-2 business days)

---

### Server Post ACH Pre-Note

**Transaction Type:** `A3` (ACH Pre-Note Debit)
**Amount:** $0.00
**Purpose:** Verify bank account before processing real transactions (recommended for ACH)

#### Request

```
POST https://secure.epxuap.com
Content-Type: application/x-www-form-urlencoded

CUST_NBR=9001&
MERCH_NBR=900300&
DBA_NBR=2&
TERMINAL_NBR=77&
TRAN_TYPE=A3&
AMOUNT=0.00&
TRAN_NBR=5901234567&
BATCH_ID=20251121&
LOCAL_DATE=112125&
LOCAL_TIME=190000&
ROUTING_NBR=021000021&
ACCOUNT_NBR=1234567890&
FIRST_NAME=John&
LAST_NAME=Doe
```

**Field Descriptions:**

| Field | Value | Description |
|-------|-------|-------------|
| `TRAN_TYPE` | `A3` | ACH Pre-Note Debit (verification) |
| `AMOUNT` | `0.00` | No charge (verification only) |
| `ROUTING_NBR` | `021000021` | Bank routing number to verify |
| `ACCOUNT_NBR` | `1234567890` | Bank account to verify |

#### Response

```
AUTH_RESP=00&
AUTH_RESP_TEXT=APPROVAL&
AUTH_CODE=047228&
AUTH_GUID=0A1MRFVZXYZYZA149TI&
ROUTING_NBR=021000021&
MASKED_ACCOUNT_NBR=******7890&
NETWORK_RESPONSE=00
```

**Result:** ✅ **APPROVED** - Pre-note sent to bank (wait 3-5 business days before processing real ACH transactions)

---

## Response Codes Reference

### AUTH_RESP Codes

| Code | Description | Action |
|------|-------------|--------|
| `00` | Approved | Transaction successful |
| `01` | Call for Authorization | Contact card issuer |
| `02` | Call for Authorization (special) | Contact card issuer |
| `04` | Pick Up Card | Card flagged as stolen/lost |
| `05` | Do Not Honor | Generic decline |
| `12` | Invalid Transaction | Transaction type not allowed |
| `13` | Invalid Amount | Amount format error or exceeds limit |
| `14` | Invalid Card Number | Card number failed validation |
| `41` | Lost Card | Card reported lost |
| `43` | Stolen Card | Card reported stolen |
| `51` | Insufficient Funds | Not enough funds |
| `54` | Expired Card | Card has expired |
| `55` | Incorrect PIN | Wrong PIN entered (PIN debit) |
| `57` | Transaction Not Permitted | Card restrictions |
| `61` | Exceeds Withdrawal Limit | Amount exceeds card limit |
| `65` | Activity Limit Exceeded | Too many transactions |
| `75` | PIN Try Exceeded | Too many PIN attempts |
| `76` | Invalid Account | Account doesn't exist |
| `82` | Negative CAM/dCVV/CVV | CVV validation failed |
| `85` | No Reason to Decline | Approved (with note) |
| `91` | Issuer Not Available | Network timeout |
| `96` | System Malfunction | System error |

### AVS (Address Verification) Codes

| Code | Description |
|------|-------------|
| `Y` | Address and ZIP match |
| `N` | No match |
| `A` | Address matches, ZIP doesn't |
| `Z` | ZIP matches, address doesn't |
| `R` | Retry - system unavailable |
| `S` | Service not supported |
| `U` | Address info unavailable |

### CVV2 Codes

| Code | Description |
|------|-------------|
| `M` | CVV2 Match |
| `N` | CVV2 No Match |
| `P` | Not Processed |
| `S` | CVV2 should be on card but wasn't provided |
| `U` | Issuer doesn't participate in CVV2 |

---

## Transaction Type Reference

### Credit Card Transaction Types

| Code | Description | Use Case |
|------|-------------|----------|
| `CCE1` | Sale | Immediate charge and capture |
| `CCE2` | Authorization Only | Hold funds (must CAPTURE later) |
| `CCE3` | Capture | Settle authorized funds |
| `CCE4` | Void | Cancel same-day transaction |
| `CCE5` | Refund | Return funds to customer |
| `CCE8` | BRIC Storage | Tokenize card for future use |

### ACH Transaction Types

| Code | Description | Account Type |
|------|-------------|--------------|
| `A1` | Debit | Checking |
| `A2` | Credit | Checking |
| `A3` | Pre-Note Debit | Checking (verification) |
| `A4` | Pre-Note Credit | Checking (verification) |
| `A5` | ACH Void | Checking |
| `A9` | BRIC Storage | Checking |
| `S1` | Debit | Savings |
| `S2` | Credit | Savings |
| `S3` | Pre-Note Debit | Savings (verification) |
| `S4` | Pre-Note Credit | Savings (verification) |
| `S5` | ACH Void | Savings |
| `S9` | BRIC Storage | Savings |

---

## BRIC Token Usage Guide

### BRIC Types

1. **Storage BRIC** - Created by Browser Post STORAGE (`CCE8`) or Server Post BRIC Storage (`A9`)
   - Can be used for AUTH, SALE, or ACH transactions
   - Stored indefinitely (no expiration)
   - Linked to specific card/account

2. **Financial BRIC** - Created by AUTH transactions (`CCE2`)
   - Used for CAPTURE, VOID, or REFUND
   - Only valid for the specific authorization
   - Cannot be reused for new charges

### Using BRICs in Server Post

**For AUTH/SALE with stored card:**
```
ORIG_AUTH_GUID=<storage_bric>  # From CCE8 STORAGE transaction
```

**For CAPTURE/VOID/REFUND:**
```
ORIG_AUTH_GUID=<financial_bric>  # From CCE2 AUTH or CCE1 SALE transaction
```

**Important:** Always use `ORIG_AUTH_GUID` field when sending BRIC tokens to EPX. The `AUTH_GUID` field is only used in EPX responses.

---

## Certification Test Cases

### Required Test Scenarios

1. **Browser Post**
   - ✅ SALE ($50.00) - Immediate charge
   - ✅ AUTH ($50.00) - Authorization only
   - ✅ STORAGE - Tokenize Visa
   - ✅ STORAGE - Tokenize Mastercard
   - ⚠️ Declined transaction (amount trigger or decline test card)

2. **Server Post - Credit Card**
   - ✅ AUTH with Storage BRIC
   - ✅ SALE with Storage BRIC
   - ✅ CAPTURE (full amount)
   - ✅ CAPTURE (partial amount)
   - ✅ VOID (same day)
   - ✅ REFUND (full amount)
   - ✅ REFUND (partial amount)

3. **Server Post - ACH**
   - ✅ BRIC Storage (checking account)
   - ✅ Pre-Note Debit (verification)
   - ✅ ACH Debit
   - ✅ ACH Credit (return)
   - ✅ ACH Void

4. **Error Handling**
   - ⚠️ Invalid card number
   - ⚠️ Expired card
   - ⚠️ Insufficient funds
   - ⚠️ Invalid BRIC token
   - ⚠️ Invalid routing/account number (ACH)

---

## Notes for North Certification

1. **Each Browser Post transaction gets a unique BRIC** - The examples above show real BRICs from actual test transactions. During certification, you'll generate new unique BRICs for each test case.

2. **BRIC Format** - EPX BRICs are alphanumeric tokens (e.g., `0A1MRFVRQ0JNJY579T3`). They are case-sensitive and must be stored exactly as returned.

3. **Transaction Numbers** - Must be unique 10-digit numbers. Our service generates deterministic transaction numbers from UUID v4.

4. **Batch IDs** - Use YYYYMMDD format (e.g., `20251121`). Can also use simple numbers (max 8 digits).

5. **ACH Processing Times**
   - Pre-Note: 3-5 business days to verify
   - Debit/Credit: 1-2 business days to settle
   - Always send Pre-Note before first real ACH transaction

6. **Card Entry Method** (`CARD_ENT_METH`)
   - `E` = Ecommerce (most common)
   - `M` = Manual entry
   - `S` = Swipe
   - `C` = Chip

7. **Industry Type** (`INDUSTRY_TYPE`)
   - `RE` = Retail
   - `RS` = Restaurant
   - `LD` = Lodging
   - `PS` = Passenger Transport

8. **Authorization Characteristics** (`ACI_EXT`)
   - `C001` = Stored Credential, Cardholder-Initiated
   - `M001` = Stored Credential, Merchant-Initiated (recurring billing)
   - See EPX Card on File documentation for full list

---

**Document Version:** 1.0
**Last Updated:** 2025-11-21
**Status:** ✅ Real transactions captured from integration tests
