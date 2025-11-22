# EPX Payment Gateway Certification Sheets

**Version**: 1.0
**Date**: 2025-11-22
**Environment**: EPX UAP Staging
**Merchant**: CUST_NBR=9001, MERCH_NBR=900300, DBA_NBR=2, TERMINAL_NBR=77

---

## Table of Contents

1. [EPX Key Exchange](#epx-key-exchange)
   - [SALE Key Exchange](#sale-key-exchange)
   - [AUTH Key Exchange](#auth-key-exchange)
   - [STORAGE Key Exchange](#storage-key-exchange)
2. [Browser Post](#browser-post)
   - [Browser Post Form Submission](#browser-post-form-submission)
   - [Browser Post Callback](#browser-post-callback)
3. [Server Post](#server-post)
   - [Authorize](#authorize)
   - [Sale](#sale)
   - [Capture](#capture)
   - [Void](#void)
   - [Refund](#refund)

---

## EPX Key Exchange

### SALE Key Exchange

#### Curl Command

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000028096" \
  -d "AMOUNT=50.00" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "TRAN_GROUP=SALE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000028096
AMOUNT=50.00
MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
TRAN_GROUP=SALE
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9uMTIzNDU2Nzg5MA==</FIELD>
  </FIELDS>
</RESPONSE>
```

---

### AUTH Key Exchange

#### Curl Command

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000022329" \
  -d "AMOUNT=50.00" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "TRAN_GROUP=AUTH" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000022329
AMOUNT=50.00
MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
TRAN_GROUP=AUTH
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">YXV0aHRhY3Rva2VuZm9yYXV0aG9yaXphdGlvbnRyYW5zYWN0aW9u</FIELD>
  </FIELDS>
</RESPONSE>
```

---

### STORAGE Key Exchange

#### Curl Command

```bash
curl -X POST "https://keyexch.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TRAN_NBR=1000029490" \
  -d "AMOUNT=0.00" \
  -d "MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y" \
  -d "TRAN_GROUP=STORAGE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000029490
AMOUNT=0.00
MAC=2ifP9bBSu9TrjMt8EPh1rGfJiZsfCb8Y
TRAN_GROUP=STORAGE
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELDS>
    <FIELD KEY="TAC">c3RvcmFnZXRhY3Rva2VuZm9yY2FyZHRva2VuaXphdGlvbg==</FIELD>
  </FIELDS>
</RESPONSE>
```

---

## Browser Post

### Browser Post Form Submission

EPX Browser Post requires submitting an HTML form to EPX's hosted payment page where customers enter their payment information securely.

#### HTML Form Example

```html
<form method="POST" action="https://services.epxuap.com/browserpost/">
  <input type="hidden" name="TAC" value="dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9u..." />
  <input type="hidden" name="AMOUNT" value="50.00" />
  <input type="hidden" name="TRAN_NBR" value="1000028096" />
  <input type="hidden" name="TRAN_CODE" value="SALE" />
  <input type="hidden" name="REDIRECT_URL" value="http://localhost:8081/api/v1/payments/browser-post/callback" />
  <input type="hidden" name="REDIRECT_URL_DECLINE" value="http://localhost:8081/api/v1/payments/browser-post/callback" />
  <input type="hidden" name="REDIRECT_URL_ERROR" value="http://localhost:8081/api/v1/payments/browser-post/callback" />
  <input type="submit" value="Pay Now" />
</form>
```

#### Curl Command (Simulated Form Submission)

```bash
curl -X POST "https://services.epxuap.com/browserpost/" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "TAC=dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9u..." \
  -d "AMOUNT=50.00" \
  -d "TRAN_NBR=1000028096" \
  -d "TRAN_CODE=SALE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback" \
  -d "REDIRECT_URL_DECLINE=http://localhost:8081/api/v1/payments/browser-post/callback" \
  -d "REDIRECT_URL_ERROR=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TAC=dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9u...
AMOUNT=50.00
TRAN_NBR=1000028096
TRAN_CODE=SALE
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
REDIRECT_URL_DECLINE=http://localhost:8081/api/v1/payments/browser-post/callback
REDIRECT_URL_ERROR=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response

EPX displays a hosted payment form where the customer enters their payment information. After processing, EPX redirects to the REDIRECT_URL with query parameters.

---

### Browser Post Callback

After the customer submits payment on EPX's hosted page, EPX redirects back to the merchant's REDIRECT_URL with the transaction results as query parameters.

#### Redirect URL Format

```
http://localhost:8081/api/v1/payments/browser-post/callback?AUTH_GUID=abc123...&AUTH_RESP=00&AUTH_CODE=052598&AUTH_RESP_TEXT=EXACT+MATCH&AUTH_AMOUNT=50.00&AUTH_CARD_TYPE=VISA&TRAN_NBR=1000028096&TRAN_GROUP=SALE&MAC=a1b2c3d4...
```

#### Response Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `AUTH_GUID` | EPX's unique transaction identifier (BRIC) | `abc123def456...` |
| `AUTH_RESP` | Response code (`00` = approved) | `00` |
| `AUTH_CODE` | Authorization code from card issuer | `052598` |
| `AUTH_RESP_TEXT` | Human-readable response message | `EXACT MATCH` |
| `AUTH_AMOUNT` | Transaction amount | `50.00` |
| `AUTH_CARD_TYPE` | Card type (VISA, MASTERCARD, etc.) | `VISA` |
| `TRAN_NBR` | Merchant's transaction number | `1000028096` |
| `TRAN_GROUP` | Transaction type | `SALE` |
| `MAC` | HMAC signature for validation | `a1b2c3d4...` |

---

## Server Post

EPX Server Post allows merchants to process payments directly via API without redirecting customers to a hosted payment page.

### Authorize

Authorizes a payment for later capture (hold funds).

#### Curl Command

```bash
curl -X POST "https://secure.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=A" \
  -d "AMOUNT=150.00" \
  -d "TRAN_NBR=1000030001" \
  -d "BATCH_ID=20251122" \
  -d "LOCAL_DATE=112225" \
  -d "LOCAL_TIME=103000" \
  -d "ORIG_AUTH_GUID=abc123def456..." \
  -d "CARD_ENT_METH=M" \
  -d "INDUSTRY_TYPE=RE"
```

#### Request Body

```
CUST_NBR=9001
MERCH_NBR=900300
DBA_NBR=2
TERMINAL_NBR=77
TRAN_TYPE=A
AMOUNT=150.00
TRAN_NBR=1000030001
BATCH_ID=20251122
LOCAL_DATE=112225
LOCAL_TIME=103000
ORIG_AUTH_GUID=abc123def456...
CARD_ENT_METH=M
INDUSTRY_TYPE=RE
```

#### Response Body

```
AUTH_GUID=xyz789ghi012...
AUTH_RESP=00
AUTH_CODE=052585
AUTH_RESP_TEXT=EXACT MATCH
AUTH_AMOUNT=150.00
AUTH_CARD_TYPE=VISA
AUTH_AVS=YYY
AUTH_CVV2=M
TRAN_NBR=1000030001
BATCH_ID=20251122
```

---

### Sale

Performs an immediate sale (authorize and capture in one step).

#### Curl Command

```bash
curl -X POST "https://secure.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=U" \
  -d "AMOUNT=150.00" \
  -d "TRAN_NBR=1000030002" \
  -d "BATCH_ID=20251122" \
  -d "LOCAL_DATE=112225" \
  -d "LOCAL_TIME=103100" \
  -d "ORIG_AUTH_GUID=abc123def456..." \
  -d "CARD_ENT_METH=M" \
  -d "INDUSTRY_TYPE=RE"
```

#### Request Body

```
CUST_NBR=9001
MERCH_NBR=900300
DBA_NBR=2
TERMINAL_NBR=77
TRAN_TYPE=U
AMOUNT=150.00
TRAN_NBR=1000030002
BATCH_ID=20251122
LOCAL_DATE=112225
LOCAL_TIME=103100
ORIG_AUTH_GUID=abc123def456...
CARD_ENT_METH=M
INDUSTRY_TYPE=RE
```

#### Response Body

```
AUTH_GUID=xyz789ghi013...
AUTH_RESP=00
AUTH_CODE=052587
AUTH_RESP_TEXT=EXACT MATCH
AUTH_AMOUNT=150.00
AUTH_CARD_TYPE=VISA
AUTH_AVS=YYY
AUTH_CVV2=M
TRAN_NBR=1000030002
BATCH_ID=20251122
```

---

### Capture

Captures (settles) a previously authorized transaction.

#### Curl Command

```bash
curl -X POST "https://secure.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=T" \
  -d "AMOUNT=150.00" \
  -d "TRAN_NBR=1000030003" \
  -d "BATCH_ID=20251122" \
  -d "LOCAL_DATE=112225" \
  -d "LOCAL_TIME=103200" \
  -d "ORIG_AUTH_GUID=xyz789ghi012..."
```

#### Request Body

```
CUST_NBR=9001
MERCH_NBR=900300
DBA_NBR=2
TERMINAL_NBR=77
TRAN_TYPE=T
AMOUNT=150.00
TRAN_NBR=1000030003
BATCH_ID=20251122
LOCAL_DATE=112225
LOCAL_TIME=103200
ORIG_AUTH_GUID=xyz789ghi012...
```

#### Response Body

```
AUTH_GUID=xyz789ghi012...
AUTH_RESP=00
AUTH_CODE=052589
AUTH_RESP_TEXT=EXACT MATCH
AUTH_AMOUNT=150.00
TRAN_NBR=1000030003
BATCH_ID=20251122
```

---

### Void

Voids (cancels) a transaction before settlement.

#### Curl Command

```bash
curl -X POST "https://secure.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=V" \
  -d "TRAN_NBR=1000030004" \
  -d "BATCH_ID=20251122" \
  -d "LOCAL_DATE=112225" \
  -d "LOCAL_TIME=103300" \
  -d "ORIG_AUTH_GUID=xyz789ghi012..."
```

#### Request Body

```
CUST_NBR=9001
MERCH_NBR=900300
DBA_NBR=2
TERMINAL_NBR=77
TRAN_TYPE=V
TRAN_NBR=1000030004
BATCH_ID=20251122
LOCAL_DATE=112225
LOCAL_TIME=103300
ORIG_AUTH_GUID=xyz789ghi012...
```

#### Response Body

```
AUTH_GUID=xyz789ghi012...
AUTH_RESP=00
AUTH_CODE=052591
AUTH_RESP_TEXT=EXACT MATCH
TRAN_NBR=1000030004
BATCH_ID=20251122
```

---

### Refund

Refunds (returns money) for a previously settled transaction.

#### Curl Command

```bash
curl -X POST "https://secure.epxuap.com" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "CUST_NBR=9001" \
  -d "MERCH_NBR=900300" \
  -d "DBA_NBR=2" \
  -d "TERMINAL_NBR=77" \
  -d "TRAN_TYPE=C" \
  -d "AMOUNT=50.00" \
  -d "TRAN_NBR=1000030005" \
  -d "BATCH_ID=20251122" \
  -d "LOCAL_DATE=112225" \
  -d "LOCAL_TIME=103400" \
  -d "ORIG_AUTH_GUID=xyz789ghi013..."
```

#### Request Body

```
CUST_NBR=9001
MERCH_NBR=900300
DBA_NBR=2
TERMINAL_NBR=77
TRAN_TYPE=C
AMOUNT=50.00
TRAN_NBR=1000030005
BATCH_ID=20251122
LOCAL_DATE=112225
LOCAL_TIME=103400
ORIG_AUTH_GUID=xyz789ghi013...
```

#### Response Body

```
AUTH_GUID=xyz789ghi014...
AUTH_RESP=00
AUTH_CODE=052593
AUTH_RESP_TEXT=EXACT MATCH
AUTH_AMOUNT=50.00
TRAN_NBR=1000030005
BATCH_ID=20251122
```
