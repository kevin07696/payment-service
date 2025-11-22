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
   - [Generate Browser Post Form](#generate-browser-post-form)
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
  -d "MAC=<MERCHANT_MAC_SECRET>" \
  -d "TRAN_GROUP=SALE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000028096
AMOUNT=50.00
MAC=<MERCHANT_MAC_SECRET>
TRAN_GROUP=SALE
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELD KEY="TAC">dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9uMTIzNDU2Nzg5MA==</FIELD>
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
  -d "MAC=<MERCHANT_MAC_SECRET>" \
  -d "TRAN_GROUP=AUTH" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000022329
AMOUNT=50.00
MAC=<MERCHANT_MAC_SECRET>
TRAN_GROUP=AUTH
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELD KEY="TAC">YXV0aHRhY3Rva2VuZm9yYXV0aG9yaXphdGlvbnRyYW5zYWN0aW9u</FIELD>
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
  -d "MAC=<MERCHANT_MAC_SECRET>" \
  -d "TRAN_GROUP=STORAGE" \
  -d "REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback"
```

#### Request Body

```
TRAN_NBR=1000029490
AMOUNT=0.00
MAC=<MERCHANT_MAC_SECRET>
TRAN_GROUP=STORAGE
REDIRECT_URL=http://localhost:8081/api/v1/payments/browser-post/callback
```

#### Response Body

```xml
<?xml version="1.0" encoding="UTF-8"?>
<RESPONSE>
  <FIELD KEY="TAC">c3RvcmFnZXRhY3Rva2VuZm9yY2FyZHRva2VuaXphdGlvbg==</FIELD>
</RESPONSE>
```

---

## Browser Post

### Generate Browser Post Form

#### Curl Command

```bash
curl -X GET "http://localhost:8081/api/v1/payments/browser-post/form" \
  -G \
  --data-urlencode "transaction_id=550e8400-e29b-41d4-a716-446655440000" \
  --data-urlencode "merchant_id=00000000-0000-0000-0000-000000000001" \
  --data-urlencode "amount=50.00" \
  --data-urlencode "transaction_type=SALE" \
  --data-urlencode "return_url=http://localhost:8082/payment-result"
```

#### Request Body

```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "merchant_id": "00000000-0000-0000-0000-000000000001",
  "amount": "50.00",
  "transaction_type": "SALE",
  "return_url": "http://localhost:8082/payment-result"
}
```

#### Response Body

```json
{
  "tac": "dGVzdHRhY3ZhbHVlZm9yc2FsZXRyYW5zYWN0aW9u...",
  "postURL": "https://epxuap.com",
  "epxTranNbr": "1000028096",
  "merchantCredentials": {
    "custNbr": "9001",
    "merchNbr": "900300",
    "dbaNbr": "2",
    "terminalNbr": "77"
  },
  "transactionId": "550e8400-e29b-41d4-a716-446655440000",
  "amount": "50.00",
  "transactionType": "SALE"
}
```

---

### Browser Post Callback

#### Curl Command

```bash
curl -X POST "http://localhost:8081/api/v1/payments/browser-post/callback" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "RESP_CODE=00" \
  -d "AUTH_CODE=052598" \
  -d "TRAN_TYPE=U" \
  -d "AMOUNT=50.00" \
  -d "BRIC=YmFzZTY0ZW5jb2RlZGJyaWN0b2tlbmhlcmU=" \
  -d "SIGNATURE=a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6" \
  -d "transaction_id=550e8400-e29b-41d4-a716-446655440000" \
  -d "merchant_id=00000000-0000-0000-0000-000000000001" \
  -d "transaction_type=SALE"
```

#### Request Body

```
RESP_CODE=00
AUTH_CODE=052598
TRAN_TYPE=U
AMOUNT=50.00
BRIC=YmFzZTY0ZW5jb2RlZGJyaWN0b2tlbmhlcmU=
SIGNATURE=a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
transaction_id=550e8400-e29b-41d4-a716-446655440000
merchant_id=00000000-0000-0000-0000-000000000001
transaction_type=SALE
```

#### Response Body

```
HTTP/1.1 302 Found
Location: http://localhost:8082/payment-result?status=approved&auth_code=052598
```

---

## Server Post

### Authorize

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Authorize" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
EOF
```

#### Request Body

```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
```

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_AUTHORIZED",
    "amountCents": 15000,
    "authorizationCode": "052585",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:30:00Z",
    "updatedAt": "2025-11-22T10:30:00Z"
  }
}
```

---

### Sale

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Sale" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
EOF
```

#### Request Body

```json
{
  "merchantId": "00000000-0000-0000-0000-000000000001",
  "customerId": "00000000-0000-0000-0000-000000001001",
  "amountCents": 15000,
  "paymentMethodId": "00000000-0000-0000-0000-000000002001"
}
```

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_APPROVED",
    "amountCents": 15000,
    "authorizationCode": "052587",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:31:00Z",
    "updatedAt": "2025-11-22T10:31:00Z"
  }
}
```

---

### Capture

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Capture" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001",
  "amountCents": 15000
}
EOF
```

#### Request Body

```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001",
  "amountCents": 15000
}
```

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_CAPTURED",
    "amountCents": 15000,
    "capturedAmountCents": 15000,
    "authorizationCode": "052589",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:30:00Z",
    "updatedAt": "2025-11-22T10:32:00Z"
  }
}
```

---

### Void

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Void" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001"
}
EOF
```

#### Request Body

```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440001"
}
```

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_VOIDED",
    "amountCents": 15000,
    "authorizationCode": "052591",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:30:00Z",
    "updatedAt": "2025-11-22T10:33:00Z"
  }
}
```

---

### Refund

#### Curl Command

```bash
curl -X POST "http://localhost:8080/payment.v1.PaymentService/Refund" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  --data-binary @- <<EOF
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440002",
  "amountCents": 5000,
  "reason": "Customer request"
}
EOF
```

#### Request Body

```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440002",
  "amountCents": 5000,
  "reason": "Customer request"
}
```

#### Response Body

```json
{
  "transaction": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "merchantId": "00000000-0000-0000-0000-000000000001",
    "customerId": "00000000-0000-0000-0000-000000001001",
    "status": "TRANSACTION_STATUS_REFUNDED",
    "amountCents": 15000,
    "refundedAmountCents": 5000,
    "authorizationCode": "052593",
    "gatewayResponse": "EXACT MATCH",
    "createdAt": "2025-11-22T10:31:00Z",
    "updatedAt": "2025-11-22T10:34:00Z"
  }
}
```
