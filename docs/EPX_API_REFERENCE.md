# EPX Server Post API Reference

**Sandbox:** `https://secure.epxuap.com`
**Production:** `https://secure.epxnow.com`

## Quick Reference

| Transaction | Code | Use Case | Auth | Capture | Requires ORIG_AUTH_GUID |
|-------------|------|----------|------|---------|-------------------------|
| Sale | CCE1 | Immediate payment | Yes | Yes | No |
| Authorization | CCE2 | Hold funds | Yes | No | No |
| Capture | CCE4 | Capture auth | No | Yes | Yes |
| Refund | CCE9 | Return money | No | No | Yes |
| Void | CCEX | Cancel unsettled | No | No | Yes |
| BRIC Storage | CCE8 | Tokenize for future | No | No | Yes (Financial BRIC) |
| Recurring | CCE1 | Subscription payment | Yes | Yes | Yes (Storage BRIC) |

## Authentication

Required in every request:

| Field | Example | Source |
|-------|---------|--------|
| `CUST_NBR` | `9001` | EPX support |
| `MERCH_NBR` | `900300` | EPX support |
| `DBA_NBR` | `2` | EPX support |
| `TERMINAL_NBR` | `77` | EPX support |

## Common Fields

### Required in All Transactions

| Field | Format | Example | Notes |
|-------|--------|---------|-------|
| `TRAN_TYPE` | String(4) | `CCE1` | Transaction code |
| `AMOUNT` | Decimal | `10.00` | Dollars.cents |
| `TRAN_NBR` | String(5-10) | `12345` | Unique per transaction |
| `BATCH_ID` | String | `12345` | Batch identifier |

**TRAN_NBR Generation:**
```go
tranNbr := fmt.Sprintf("%d", time.Now().Unix() % 100000)
```

### Card Information (New Card Transactions)

| Field | Format | Example | Required |
|-------|--------|---------|----------|
| `ACCOUNT_NBR` | String(16) | `4111111111111111` | Yes* |
| `EXP_DATE` | YYMM | `1225` | Yes* |
| `CVV2` | String(3-4) | `123` | Recommended |
| `CARD_ENT_METH` | E/Z/6 | `E` | Yes |
| `INDUSTRY_TYPE` | E/R/T | `E` | Yes |

*Not required when using BRIC token

**CARD_ENT_METH Values:**
- `E` = E-commerce (card details provided)
- `Z` = BRIC/Token
- `6` = PAN Entry via Card on File

### Billing Information (AVS)

| Field | Format | Example | AVS Required |
|-------|--------|---------|--------------|
| `FIRST_NAME` | String(50) | `John` | No |
| `LAST_NAME` | String(50) | `Doe` | No |
| `ADDRESS` | String(100) | `123 Main St` | Yes |
| `ZIP_CODE` | String(10) | `10001` | Yes |

AVS improves authorization rates and reduces fraud.

### BRIC Token Fields

| Field | Format | Use Case |
|-------|--------|----------|
| `AUTH_GUID` | String(20) | Use token FOR this transaction |
| `ORIG_AUTH_GUID` | String(20) | Reference PREVIOUS transaction |

### Advanced Fields

| Field | Format | Use Case | Values |
|-------|--------|----------|--------|
| `ACI_EXT` | String(2) | Card-on-file type | `RB`, `IP`, `CA` |
| `COF_PERIOD` | Integer | Token lifetime (months) | `0`, `13`, `18`, `24` |
| `ORIG_AUTH_AMOUNT` | Decimal | Original transaction amount | `100.00` |
| `ORIG_AUTH_TRAN_IDENT` | String(15) | Network Transaction ID | From previous response |

**ACI_EXT Values:**
- `RB` = Recurring Billing (subscriptions)
- `IP` = Installment Payment
- `CA` = Completion Advice

## Response Fields

### Core Response

| Field | Format | Description |
|-------|--------|-------------|
| `AUTH_GUID` | String | BRIC token (save for future use) |
| `AUTH_RESP` | String(2) | Response code (`00` = approved) |
| `AUTH_CODE` | String | Bank approval code |
| `AUTH_RESP_TEXT` | String | Response message |
| `AUTH_CARD_TYPE` | V/M/A/D | Card brand |
| `AUTH_AVS` | String(1) | AVS result code |
| `AUTH_CVV2` | M/N/P/U | CVV verification result |

### Network Fields

| Field | Description |
|-------|-------------|
| `AUTH_TRAN_IDENT` | Network Transaction ID (save for disputes) |
| `PROC_CODE` | Processing code |
| `SYS_TRACE_NUM` | System trace number |

## Transaction Types

### Sale (CCE1)

Authorization + capture in one step.

**Request:**
```go
request := &ports.ServerPostRequest{
    TransactionType: "CCE1",
    Amount:          "10.00",
    TranNbr:         "12345",
    AccountNumber:   strPtr("4111111111111111"),
    ExpirationDate:  strPtr("1225"),
    CVV:             strPtr("123"),
    CardEntryMethod: strPtr("E"),
    ZipCode:         strPtr("10001"),
}
```

**Response:**
- `AUTH_GUID`: Financial BRIC (13-month expiry)
- `AUTH_RESP`: `00` = Approved
- `AUTH_CODE`: Bank approval code

### Authorization Only (CCE2)

Hold funds without capturing.

**Request:**
```go
request := &ports.ServerPostRequest{
    TransactionType: "CCE2",
    Amount:          "25.00",
    TranNbr:         "12346",
    AccountNumber:   strPtr("4111111111111111"),
    ExpirationDate:  strPtr("1225"),
    CardEntryMethod: strPtr("E"),
}
```

**Capture later with CCE4 using ORIG_AUTH_GUID.**

**Expiry:** Typically 7-30 days depending on card network.

### Capture (CCE4)

Capture previously authorized funds.

**Request:**
```go
request := &ports.ServerPostRequest{
    TransactionType: "CCE4",
    Amount:          "25.00",           // Must match or be less than auth
    TranNbr:         "12347",
    OrigAuthGUID:    "09LMQ886...",     // From authorization response
}
```

**Notes:**
- Amount can be less than original (partial capture)
- Cannot exceed original authorization amount
- Must capture before authorization expires

### Refund (CCE9)

Return money to customer.

**Request:**
```go
request := &ports.ServerPostRequest{
    TransactionType: "CCE9",
    Amount:          "5.00",            // Refund amount
    TranNbr:         "12348",
    OrigAuthGUID:    "09LMQ886...",     // From original sale
    AccountNumber:   strPtr("4111111111111111"),
    ExpirationDate:  strPtr("1225"),
}
```

**Notes:**
- Can be partial or full refund
- May require card details depending on processor
- Refund appears on customer statement in 3-5 business days

### Void (CCEX)

Cancel unsettled transaction.

**Request:**
```go
request := &ports.ServerPostRequest{
    TransactionType: "CCEX",
    Amount:          "1.00",            // Original amount
    TranNbr:         "12349",
    OrigAuthGUID:    "09LMQ886...",     // From original transaction
}
```

**Notes:**
- Only works for unsettled transactions (same day)
- After settlement, use refund instead
- Amount must match original transaction

### BRIC Storage (CCE8)

Convert Financial BRIC to Storage BRIC for recurring payments.

**Request:**
```go
request := &ports.ServerPostRequest{
    TransactionType: "CCE8",
    Amount:          "0.00",            // Always $0 for tokenization
    TranNbr:         "12350",
    OrigAuthGUID:    "09LMQ886...",     // Financial BRIC from previous sale
    ACIExt:          strPtr("RB"),      // Recurring billing
    COFPeriod:       intPtr(0),         // Never expires
}
```

**Response:**
- `AUTH_GUID`: Storage BRIC (never expires)

**COF_PERIOD Values:**
- `0` = Never expires (recommended for subscriptions)
- `13` = 13 months
- `18` = 18 months
- `24` = 24 months

### Recurring Payment

Sale using Storage BRIC.

**Request:**
```go
request := &ports.ServerPostRequest{
    TransactionType:    "CCE1",
    Amount:             "15.00",
    TranNbr:            "12351",
    AuthGUID:           "09LMQ886...",  // Storage BRIC
    CardEntryMethod:    strPtr("Z"),    // Token entry method
    ACIExt:             strPtr("RB"),   // Recurring billing
    OrigAuthAmount:     "15.00",        // First transaction amount
    OrigAuthTranIdent:  "...",          // NTID from first transaction
}
```

**Notes:**
- Use `AUTH_GUID` (not `ORIG_AUTH_GUID`)
- Set `CARD_ENT_METH` to `Z`
- Include `ORIG_AUTH_TRAN_IDENT` from first transaction

## Response Codes

### AUTH_RESP Codes

| Code | Meaning | Action |
|------|---------|--------|
| `00` | Approved | Process transaction |
| `05` | Do not honor | Decline, retry different card |
| `14` | Invalid card | Card number invalid |
| `41` | Lost card | Decline, do not retry |
| `43` | Stolen card | Decline, do not retry |
| `51` | Insufficient funds | Decline, user needs to add funds |
| `54` | Expired card | Request updated expiration |
| `57` | Transaction not permitted | Card restricted |
| `61` | Exceeds withdrawal limit | User exceeded daily limit |
| `91` | Issuer unavailable | Retry later |

### AVS Codes

| Code | Meaning | Recommendation |
|------|---------|----------------|
| `Y` | Address and ZIP match | Accept |
| `Z` | ZIP matches, address no match | Accept |
| `A` | Address matches, ZIP no match | Review |
| `N` | Neither address nor ZIP match | Decline (fraud risk) |
| `U` | Unavailable | Accept with caution |

### CVV2 Codes

| Code | Meaning | Recommendation |
|------|---------|----------------|
| `M` | Match | Accept |
| `N` | No match | Decline (fraud risk) |
| `P` | Not processed | Accept with caution |
| `U` | Unavailable | Accept with caution |

## Error Handling

### HTTP Response Codes

| Code | Meaning | Action |
|------|---------|--------|
| 200 | Success | Parse XML response |
| 400 | Bad request | Check request formatting |
| 401 | Unauthorized | Verify credentials |
| 500 | Server error | Retry with exponential backoff |

### Common Errors

**"Invalid TRAN_NBR[LEN]"**
- TRAN_NBR too long
- Keep to 5-10 digits
- Use: `time.Now().Unix() % 100000`

**"Duplicate TRAN_NBR"**
- TRAN_NBR must be unique per transaction
- Generate new number for retries

**"Invalid AMOUNT format"**
- Must be decimal format: `10.00`
- No currency symbols
- Maximum 2 decimal places

**"Missing required field"**
- Check all required fields present
- Verify field names match API exactly

## Best Practices

### TRAN_NBR Generation

```go
// Generate unique 5-digit transaction number
tranNbr := fmt.Sprintf("%d", time.Now().Unix() % 100000)

// For high volume, add randomness
tranNbr := fmt.Sprintf("%d%d", time.Now().Unix() % 10000, rand.Intn(10))
```

### BRIC Token Lifecycle

```
1. Sale (CCE1)
   → Returns Financial BRIC (13-month expiry)

2. Convert to Storage BRIC (CCE8)
   → Returns Storage BRIC (never expires)

3. Use for Recurring Payments (CCE1 with AUTH_GUID)
   → Creates new Financial BRIC each time
```

### Idempotency

- Store `AUTH_GUID` immediately after successful transaction
- Use `TRAN_NBR` to prevent duplicate transactions
- Check for duplicate before processing

### Security

- Never log full card numbers
- Store only BRIC tokens, never raw card data
- Use HTTPS for all requests
- Validate CVV and AVS results
- Implement fraud detection rules

### Error Handling

```go
switch response.AuthResp {
case "00":
    // Approved - process order
    saveTransaction(response)
case "05", "51", "61":
    // Soft decline - prompt for different card
    return ErrPaymentDeclined
case "41", "43":
    // Hard decline - do not retry
    return ErrCardRestricted
case "91":
    // Temporary issue - retry with backoff
    return ErrTemporaryFailure
default:
    // Unknown error - review logs
    return ErrUnknownResponse
}
```

### Rate Limiting

- EPX sandbox: ~2 requests/second
- Production: Contact EPX for limits
- Implement exponential backoff for retries

## Testing

### Test Cards

| Card | Number | Exp | CVV | AVS ZIP | Expected |
|------|--------|-----|-----|---------|----------|
| Visa | 4111111111111111 | Any future | 123 | Any | Approved |
| Mastercard | 5499740000000057 | Any future | 123 | Any | Approved |
| Amex | 371449635398431 | Any future | 1234 | Any | Approved |
| Discover | 6011000991001201 | Any future | 123 | Any | Approved |
| Decline | 4000000000000002 | Any future | 123 | Any | Declined |

### Test Workflow

```bash
# 1. Sale
curl -X POST https://secure.epxuap.com/xml \
  -d "TRAN_TYPE=CCE1&AMOUNT=10.00&TRAN_NBR=12345..."

# 2. Convert to Storage BRIC
curl -X POST https://secure.epxuap.com/xml \
  -d "TRAN_TYPE=CCE8&AMOUNT=0.00&ORIG_AUTH_GUID=..."

# 3. Recurring payment
curl -X POST https://secure.epxuap.com/xml \
  -d "TRAN_TYPE=CCE1&AMOUNT=15.00&AUTH_GUID=..."
```

## References

- EPX Developer Portal: Contact EPX support
- Card network rules: Visa, Mastercard, Amex documentation
- PCI DSS compliance: https://pcisecuritystandards.org
- Implementation: `internal/adapters/epx/server_post_adapter.go`
