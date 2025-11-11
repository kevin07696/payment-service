# Browser POST Frontend Integration Guide

**Last Updated**: 2025-11-11
**Pattern**: PENDING→UPDATE transaction lifecycle
**For**: Frontend developers integrating Payment Service

---

## Quick Start

Browser POST enables PCI-compliant **credit card payments** where card data flows directly from browser to EPX, bypassing your backend entirely.

> **Payment Method Support**: Browser POST supports **credit cards only**. For ACH payments, use Server POST API.

**3-Step Integration**:
1. Backend calls Payment Service `GetPaymentForm` API → receives form config + transaction IDs
2. Frontend renders HTML form with received config + user input fields
3. User submits → EPX processes → Payment Service updates transaction → redirects to your return_url

**Complete technical details**: See [Browser POST Dataflow](./BROWSER_POST_DATAFLOW.md)

---

## Step 1: Backend Calls Payment Service

### API Request

**Endpoint**: `GET /api/v1/payments/browser-post/form`

```http
GET /api/v1/payments/browser-post/form?amount=99.99&return_url=https://pos.example.com/payment-complete&agent_id=merchant-123
```

**Required Parameters**:
- `amount`: Transaction amount (e.g., "99.99")
- `return_url`: Where to redirect browser after payment completes
- `agent_id`: Merchant identifier (optional, defaults to EPX custNbr)

### API Response

```json
{
  "transactionId": "054edaac-3770-4222-ab50-e09b41051cc4",
  "groupId": "9b3d3df9-e37b-47ca-83f8-106b51b0ff50",
  "postURL": "https://secure.epxuap.com/browserpost",
  "amount": "99.99",
  "tranNbr": "45062844883",
  "tranGroup": "SALE",
  "tranCode": "SALE",
  "industryType": "E",
  "cardEntMeth": "E",
  "redirectURL": "http://payment-service:8081/api/v1/payments/browser-post/callback",
  "userData1": "return_url=https://pos.example.com/payment-complete",
  "custNbr": "9001",
  "merchNbr": "900300",
  "dBAnbr": "2",
  "terminalNbr": "77",
  "merchantName": "Payment Service"
}
```

**Critical**: `transactionId` and `groupId` represent PENDING transaction already created. Store `groupId` to link order→payment.

### Backend Implementation Example

```go
func (s *POSService) InitiatePayment(orderID string, amount string) (*FormConfig, error) {
    // 1. Call Payment Service
    resp, err := http.Get(fmt.Sprintf(
        "https://payment-service/api/v1/payments/browser-post/form?amount=%s&return_url=%s&agent_id=%s",
        amount,
        url.QueryEscape("https://pos.example.com/payment-complete"),
        s.AgentID,
    ))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var formConfig FormConfig
    json.NewDecoder(resp.Body).Decode(&formConfig)

    // 2. Store group_id with order
    s.DB.Exec("UPDATE orders SET payment_group_id = $1, payment_status = 'PENDING' WHERE id = $2",
        formConfig.GroupID, orderID)

    // 3. Return config to frontend
    return &formConfig, nil
}
```

---

## Step 2: Frontend Renders Payment Form

### React Example

```tsx
import React, { useEffect, useState } from 'react';

interface FormConfig {
  transactionId: string;
  groupId: string;
  postURL: string;
  amount: string;
  tranNbr: string;
  tranGroup: string;
  tranCode: string;
  industryType: string;
  cardEntMeth: string;
  redirectURL: string;
  userData1: string;
  custNbr: string;
  merchNbr: string;
  dBAnbr: string;
  terminalNbr: string;
}

export default function PaymentForm({ orderID, amount }: { orderID: string; amount: string }) {
  const [config, setConfig] = useState<FormConfig | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Fetch form config from POS backend
    fetch(`/api/orders/${orderID}/payment-form?amount=${amount}`)
      .then(res => res.json())
      .then(data => {
        setConfig(data);
        setLoading(false);
      })
      .catch(err => {
        console.error('Failed to load payment form:', err);
        setLoading(false);
      });
  }, [orderID, amount]);

  if (loading) return <div>Loading payment form...</div>;
  if (!config) return <div>Failed to load payment form</div>;

  return (
    <div className="payment-form">
      <h2>Pay ${config.amount}</h2>
      <p className="info">Transaction ID: {config.transactionId}</p>

      <form method="POST" action={config.postURL}>
        {/* Hidden EPX fields - from Payment Service */}
        <input type="hidden" name="CUST_NBR" value={config.custNbr} />
        <input type="hidden" name="MERCH_NBR" value={config.merchNbr} />
        <input type="hidden" name="DBA_NBR" value={config.dBAnbr} />
        <input type="hidden" name="TERMINAL_NBR" value={config.terminalNbr} />
        <input type="hidden" name="TRAN_CODE" value={config.tranCode} />
        <input type="hidden" name="TRAN_NBR" value={config.tranNbr} />
        <input type="hidden" name="TRAN_GROUP" value={config.tranGroup} />
        <input type="hidden" name="AMOUNT" value={config.amount} />
        <input type="hidden" name="INDUSTRY_TYPE" value={config.industryType} />
        <input type="hidden" name="CARD_ENT_METH" value={config.cardEntMeth} />
        <input type="hidden" name="REDIRECT_URL" value={config.redirectURL} />
        <input type="hidden" name="USER_DATA_1" value={config.userData1} />

        {/* Card details - user input */}
        <div className="form-group">
          <label>Card Number</label>
          <input type="text" name="CARD_NBR" placeholder="4111111111111111" maxLength={16} required />
        </div>

        <div className="form-row">
          <div className="form-group">
            <label>Exp Month (MM)</label>
            <input type="text" name="EXP_MONTH" placeholder="12" maxLength={2} required />
          </div>
          <div className="form-group">
            <label>Exp Year (YYYY)</label>
            <input type="text" name="EXP_YEAR" placeholder="2025" maxLength={4} required />
          </div>
          <div className="form-group">
            <label>CVV</label>
            <input type="text" name="CVV" placeholder="123" maxLength={4} required />
          </div>
        </div>

        <div className="form-group">
          <label>First Name</label>
          <input type="text" name="FIRST_NAME" required />
        </div>

        <div className="form-group">
          <label>Last Name</label>
          <input type="text" name="LAST_NAME" required />
        </div>

        <div className="form-group">
          <label>Zip Code</label>
          <input type="text" name="ZIP_CODE" maxLength={10} />
        </div>

        <button type="submit" className="pay-button">
          Pay ${config.amount}
        </button>
      </form>

      <div className="test-cards">
        <p><strong>Test Cards (Sandbox):</strong></p>
        <ul>
          <li>Visa: 4111111111111111</li>
          <li>Mastercard: 5499740000000057</li>
          <li>CVV: 123, Expiry: 12/2025</li>
        </ul>
      </div>
    </div>
  );
}
```

### Vanilla JavaScript Example

```html
<!DOCTYPE html>
<html>
<head>
    <title>Payment</title>
    <style>
        .payment-form { max-width: 500px; margin: 50px auto; font-family: sans-serif; }
        .form-group { margin-bottom: 15px; }
        .form-row { display: flex; gap: 10px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input { width: 100%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; }
        button { width: 100%; padding: 15px; background: #4CAF50; color: white; border: none; border-radius: 4px; font-size: 16px; cursor: pointer; }
        button:hover { background: #45a049; }
    </style>
</head>
<body>
    <div class="payment-form" id="paymentForm">Loading...</div>

    <script>
        // Extract orderID from URL or pass it in
        const orderID = 'ORDER-123';
        const amount = '99.99';

        // Fetch form config from POS backend
        fetch(`/api/orders/${orderID}/payment-form?amount=${amount}`)
            .then(res => res.json())
            .then(config => {
                // Render form with fetched config
                document.getElementById('paymentForm').innerHTML = `
                    <h2>Pay $${config.amount}</h2>
                    <p style="color: #666; font-size: 14px;">Transaction: ${config.transactionId}</p>

                    <form method="POST" action="${config.postURL}">
                        <!-- Hidden EPX fields -->
                        <input type="hidden" name="CUST_NBR" value="${config.custNbr}">
                        <input type="hidden" name="MERCH_NBR" value="${config.merchNbr}">
                        <input type="hidden" name="DBA_NBR" value="${config.dBAnbr}">
                        <input type="hidden" name="TERMINAL_NBR" value="${config.terminalNbr}">
                        <input type="hidden" name="TRAN_CODE" value="${config.tranCode}">
                        <input type="hidden" name="TRAN_NBR" value="${config.tranNbr}">
                        <input type="hidden" name="TRAN_GROUP" value="${config.tranGroup}">
                        <input type="hidden" name="AMOUNT" value="${config.amount}">
                        <input type="hidden" name="INDUSTRY_TYPE" value="${config.industryType}">
                        <input type="hidden" name="CARD_ENT_METH" value="${config.cardEntMeth}">
                        <input type="hidden" name="REDIRECT_URL" value="${config.redirectURL}">
                        <input type="hidden" name="USER_DATA_1" value="${config.userData1}">

                        <!-- Card input fields -->
                        <div class="form-group">
                            <label>Card Number</label>
                            <input type="text" name="CARD_NBR" placeholder="4111111111111111" maxlength="16" required>
                        </div>

                        <div class="form-row">
                            <div class="form-group">
                                <label>Month</label>
                                <input type="text" name="EXP_MONTH" placeholder="12" maxlength="2" required>
                            </div>
                            <div class="form-group">
                                <label>Year</label>
                                <input type="text" name="EXP_YEAR" placeholder="2025" maxlength="4" required>
                            </div>
                            <div class="form-group">
                                <label>CVV</label>
                                <input type="text" name="CVV" placeholder="123" maxlength="4" required>
                            </div>
                        </div>

                        <div class="form-group">
                            <label>First Name</label>
                            <input type="text" name="FIRST_NAME" required>
                        </div>

                        <div class="form-group">
                            <label>Last Name</label>
                            <input type="text" name="LAST_NAME" required>
                        </div>

                        <button type="submit">Pay $${config.amount}</button>
                    </form>

                    <p style="margin-top: 20px; font-size: 12px; color: #666;">
                        <strong>Test Card:</strong> 4111111111111111, CVV: 123, Exp: 12/2025
                    </p>
                `;
            })
            .catch(err => {
                document.getElementById('paymentForm').innerHTML =
                    '<p style="color: red;">Error loading payment form. Please try again.</p>';
                console.error(err);
            });
    </script>
</body>
</html>
```

---

## Step 3: Handle Payment Completion

After EPX processes payment, Payment Service redirects browser back to your `return_url` with transaction data.

### Return URL Query Parameters

```
https://pos.example.com/payment-complete?groupId=9b3d3df9-e37b-47ca-83f8-106b51b0ff50&transactionId=054edaac-3770-4222-ab50-e09b41051cc4&status=completed&amount=99.99&cardType=VISA&authCode=OK1234
```

**Parameters**:
- `groupId`: Payment group ID (use to look up order)
- `transactionId`: Specific transaction ID
- `status`: "completed" or "failed"
- `amount`: Transaction amount
- `cardType`: Card brand (VISA, MASTERCARD, etc.)
- `authCode`: Authorization code from EPX

### Backend Handler Example

```go
func (s *POSService) HandlePaymentComplete(w http.ResponseWriter, r *http.Request) {
    groupID := r.URL.Query().Get("groupId")
    status := r.URL.Query().Get("status")

    // Look up order by payment_group_id
    var order Order
    s.DB.QueryRow("SELECT * FROM orders WHERE payment_group_id = $1", groupID).Scan(&order)

    if status == "completed" {
        // Mark order as paid
        s.DB.Exec("UPDATE orders SET payment_status = 'PAID' WHERE id = $1", order.ID)

        // Render success page with complete receipt
        s.RenderReceipt(w, &order, r.URL.Query())
    } else {
        // Payment failed - show error and allow retry
        s.RenderPaymentFailed(w, &order, r.URL.Query().Get("authRespText"))
    }
}
```

---

## Required Form Fields Reference

### Hidden Fields (From Payment Service)

| Field | Source | Example | Required |
|-------|--------|---------|----------|
| `CUST_NBR` | config.custNbr | `9001` | ✅ |
| `MERCH_NBR` | config.merchNbr | `900300` | ✅ |
| `DBA_NBR` | config.dBAnbr | `2` | ✅ |
| `TERMINAL_NBR` | config.terminalNbr | `77` | ✅ |
| `TRAN_CODE` | config.tranCode | `SALE` | ✅ |
| `TRAN_NBR` | config.tranNbr | `45062844883` | ✅ |
| `TRAN_GROUP` | config.tranGroup | `SALE` | ✅ |
| `AMOUNT` | config.amount | `99.99` | ✅ |
| `INDUSTRY_TYPE` | config.industryType | `E` | ✅ |
| `CARD_ENT_METH` | config.cardEntMeth | `E` | ✅ |
| `REDIRECT_URL` | config.redirectURL | Payment Service callback | ✅ |
| `USER_DATA_1` | config.userData1 | return_url state | ✅ |

### User Input Fields

| Field | Description | Example | Required |
|-------|-------------|---------|----------|
| `CARD_NBR` | Card number | `4111111111111111` | ✅ |
| `EXP_MONTH` | Expiry month (MM) | `12` | ✅ |
| `EXP_YEAR` | Expiry year (YYYY) | `2025` | ✅ |
| `CVV` | Security code | `123` | ✅ |
| `FIRST_NAME` | Cardholder first name | `John` | ✅ |
| `LAST_NAME` | Cardholder last name | `Doe` | ✅ |
| `ZIP_CODE` | Billing zip | `10001` | ❌ (optional) |

---

## Testing

### Test Credentials (Sandbox)

```
EPX Endpoint: https://secure.epxuap.com/browserpost
Payment Service: http://localhost:8081
```

### Test Cards

**Approved**:
- Visa: `4111111111111111`
- Mastercard: `5499740000000057`
- Amex: `378282246310005`

**Declined**:
- Visa: `4000000000000002`

**All cards**: CVV: `123`, Expiry: any future date (e.g., `12/2025`)

### Test Flow

1. Call `/api/v1/payments/browser-post/form?amount=1.00&return_url=http://localhost:3000/complete&agent_id=test`
2. Render form with returned config
3. Enter test card: `4111111111111111`, CVV: `123`, Exp: `12/2025`
4. Submit form → redirects to EPX
5. EPX processes → redirects to Payment Service callback
6. Payment Service updates transaction → redirects to your return_url
7. Verify query parameters include groupId, transactionId, status

---

## Common Issues

### Issue: "amount parameter is required"
**Cause**: Missing `amount` in GetPaymentForm request
**Fix**: Include `?amount=99.99` in API call

### Issue: "return_url parameter is required"
**Cause**: Missing `return_url` in GetPaymentForm request
**Fix**: Include `&return_url=https://pos.example.com/complete` in API call

### Issue: Form submits but redirects to error page
**Cause**: REDIRECT_URL not whitelisted with EPX
**Fix**: Contact EPX to whitelist Payment Service callback URL

### Issue: Payment completes but no redirect back to POS
**Cause**: Invalid return_url or Payment Service cannot reach it
**Fix**: Ensure return_url is publicly accessible HTTPS URL

### Issue: "Transaction not found" after callback
**Cause**: TRAN_NBR mismatch or transaction not created in Step 1
**Fix**: Verify GetPaymentForm was called and returned transactionId

---

## Security Notes

### PCI Compliance
✅ **Card data never touches your backend** - Forms POST directly to EPX
✅ **Payment Service never sees card data** - Only receives tokenized response
✅ **Your backend remains PCI-compliant** - No card data storage required

### HTTPS Requirements
- **Development**: HTTP localhost allowed
- **Staging/Production**: HTTPS required for return_url

### Best Practices
1. Always validate amount matches expected value when handling return_url
2. Store groupId with order immediately after GetPaymentForm
3. Mark order PENDING after GetPaymentForm (before payment completes)
4. Handle both completed and failed statuses at return_url
5. Show user-friendly error messages for declined payments

---

## Summary

**3-Step Integration**:
1. **Backend**: Call Payment Service GetPaymentForm → Store groupId with order
2. **Frontend**: Render HTML form with config → User enters card → Submit to EPX
3. **Completion**: Browser redirects to your return_url → Render receipt

**Key Points**:
- Transaction created in PENDING state at Step 1 (audit trail)
- Card data flows: Browser → EPX (never touches your backend)
- Payment Service updates PENDING→COMPLETED/FAILED at callback
- Your backend renders complete receipt (has order context)

**Architecture**: Payment Service = Gateway Integration ONLY

---

**For Complete Technical Details**: See [Browser POST Dataflow](./BROWSER_POST_DATAFLOW.md)

**Last Updated**: 2025-11-11
