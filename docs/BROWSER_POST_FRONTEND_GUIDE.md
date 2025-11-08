# EPX Browser Post Frontend Implementation Guide

## What is Browser Post?

Browser Post is EPX's **PCI-compliant** payment method where:
- Card data is submitted **directly from the user's browser to EPX**
- Card data **never touches your backend server**
- You remain PCI compliant without handling sensitive card data
- EPX processes the payment and redirects back to your callback URL

---

## How Browser Post Works - Complete Flow

```
┌─────────────┐         ┌──────────────┐         ┌─────────┐         ┌──────────────┐
│   User      │         │   Your       │         │   EPX   │         │   Your       │
│   Browser   │         │   Backend    │         │         │         │   Callback   │
└─────────────┘         └──────────────┘         └─────────┘         └──────────────┘
      │                        │                       │                      │
      │  1. Load payment page  │                       │                      │
      │───────────────────────>│                       │                      │
      │                        │                       │                      │
      │  2. Generate TAC token │                       │                      │
      │                        │ (optional, depends on │                      │
      │                        │  EPX setup)           │                      │
      │                        │                       │                      │
      │  3. Return form data   │                       │                      │
      │<───────────────────────│                       │                      │
      │    (TAC, credentials,  │                       │                      │
      │     hidden fields)     │                       │                      │
      │                        │                       │                      │
      │  4. User enters card   │                       │                      │
      │     and clicks Pay     │                       │                      │
      │                        │                       │                      │
      │  5. POST to EPX        │                       │                      │
      │────────────────────────────────────────────────>                      │
      │    (card data + TAC +  │                       │                      │
      │     transaction info)  │                       │                      │
      │                        │                       │                      │
      │                        │  6. EPX processes     │                      │
      │                        │     payment           │                      │
      │                        │                       │                      │
      │  7. EPX redirects      │                       │                      │
      │     with results       │                       │                      │
      │<───────────────────────────────────────────────│                      │
      │    (POST to REDIRECT_URL)                      │                      │
      │                                                 │                      │
      │  8. Callback receives results                  │                      │
      │────────────────────────────────────────────────────────────────────────>
      │                                                 │                      │
      │                                                 │  9. Store in DB,    │
      │                                                 │     send webhooks   │
      │                                                 │                      │
      │  10. Display confirmation                       │                      │
      │<────────────────────────────────────────────────────────────────────────│
```

---

## Step 1: Backend - Generate Form Data

Your backend needs to generate the form data that will be used in the HTML form.

### Option A: With TAC Token (More Secure)

```go
// internal/handlers/payment/browser_post_handler.go
func (h *BrowserPostHandler) GeneratePaymentForm(ctx context.Context, req *PaymentFormRequest) (*BrowserPostFormData, error) {
    // 1. Generate unique transaction number
    tranNbr := fmt.Sprintf("%d", time.Now().Unix() % 100000)

    // 2. Generate TAC token (if required by your EPX setup)
    // Note: TAC generation method depends on your EPX configuration
    // Some merchants use Key Exchange API, others have different methods
    tacToken, err := h.generateTAC(ctx, &TACRequest{
        Amount:      req.Amount,
        TranNbr:     tranNbr,
        RedirectURL: "https://yourdomain.com/api/v1/payments/callback",
    })
    if err != nil {
        return nil, err
    }

    // 3. Build form data using adapter
    formData := h.browserPostAdapter.BuildFormData(&ports.BrowserPostRequest{
        TAC:         tacToken,
        Amount:      req.Amount,
        TranNbr:     tranNbr,
        TranGroup:   tranNbr,
        TranCode:    "SALE",
        RedirectURL: "https://yourdomain.com/api/v1/payments/callback",
        CustNbr:     h.config.EPX.CustNbr,
        MerchNbr:    h.config.EPX.MerchNbr,
        DBAnbr:      h.config.EPX.DBAnbr,
        TerminalNbr: h.config.EPX.TerminalNbr,
    })

    return formData, nil
}
```

### Option B: Without TAC Token (Simpler)

Some EPX configurations don't require TAC tokens:

```go
func (h *BrowserPostHandler) GeneratePaymentForm(ctx context.Context, req *PaymentFormRequest) (*BrowserPostFormData, error) {
    tranNbr := fmt.Sprintf("%d", time.Now().Unix() % 100000)

    formData := &BrowserPostFormData{
        PostURL:     "https://secure.epxuap.com/browserpost", // Sandbox
        Amount:      req.Amount,
        TranNbr:     tranNbr,
        TranGroup:   tranNbr,
        TranCode:    "SALE",
        IndustryType: "E", // E-commerce
        CardEntMeth: "E",  // E-commerce entry
        RedirectURL: "https://yourdomain.com/api/v1/payments/callback",
        CustNbr:     "9001",
        MerchNbr:    "900300",
        DBAnbr:      "2",
        TerminalNbr: "77",
    }

    return formData, nil
}
```

---

## Step 2: Backend - API Endpoint

Create an endpoint to serve the form data to your frontend:

```go
// GET /api/v1/payments/browser-post/form?amount=99.99
func (h *BrowserPostHandler) GetPaymentForm(w http.ResponseWriter, r *http.Request) {
    amount := r.URL.Query().Get("amount")
    if amount == "" {
        http.Error(w, "amount is required", http.StatusBadRequest)
        return
    }

    formData, err := h.GeneratePaymentForm(r.Context(), &PaymentFormRequest{
        Amount: amount,
    })
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(formData)
}
```

---

## Step 3: Frontend - HTML Form

### React Example

```tsx
// PaymentForm.tsx
import React, { useEffect, useState } from 'react';

interface BrowserPostFormData {
  postURL: string;
  tac?: string;
  amount: string;
  tranNbr: string;
  tranGroup: string;
  tranCode: string;
  industryType: string;
  cardEntMeth: string;
  redirectURL: string;
  custNbr: string;
  merchNbr: string;
  dBAnbr: string;
  terminalNbr: string;
}

export default function PaymentForm({ amount }: { amount: string }) {
  const [formData, setFormData] = useState<BrowserPostFormData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Fetch form data from backend
    fetch(`/api/v1/payments/browser-post/form?amount=${amount}`)
      .then(res => res.json())
      .then(data => {
        setFormData(data);
        setLoading(false);
      })
      .catch(err => {
        console.error('Failed to load payment form:', err);
        setLoading(false);
      });
  }, [amount]);

  if (loading) return <div>Loading payment form...</div>;
  if (!formData) return <div>Failed to load payment form</div>;

  return (
    <div className="payment-form">
      <h2>Payment Details</h2>

      <form method="POST" action={formData.postURL}>
        {/* Hidden EPX Fields */}
        {formData.tac && <input type="hidden" name="TAC" value={formData.tac} />}
        <input type="hidden" name="CUST_NBR" value={formData.custNbr} />
        <input type="hidden" name="MERCH_NBR" value={formData.merchNbr} />
        <input type="hidden" name="DBA_NBR" value={formData.dBAnbr} />
        <input type="hidden" name="TERMINAL_NBR" value={formData.terminalNbr} />
        <input type="hidden" name="TRAN_CODE" value={formData.tranCode} />
        <input type="hidden" name="TRAN_NBR" value={formData.tranNbr} />
        <input type="hidden" name="TRAN_GROUP" value={formData.tranGroup} />
        <input type="hidden" name="AMOUNT" value={formData.amount} />
        <input type="hidden" name="INDUSTRY_TYPE" value={formData.industryType} />
        <input type="hidden" name="CARD_ENT_METH" value={formData.cardEntMeth} />
        <input type="hidden" name="REDIRECT_URL" value={formData.redirectURL} />

        {/* Card Details - User Input */}
        <div className="form-group">
          <label htmlFor="card_nbr">Card Number</label>
          <input
            type="text"
            id="card_nbr"
            name="CARD_NBR"
            placeholder="4111111111111111"
            maxLength={16}
            required
          />
        </div>

        <div className="form-row">
          <div className="form-group">
            <label htmlFor="exp_month">Exp Month</label>
            <input
              type="text"
              id="exp_month"
              name="EXP_MONTH"
              placeholder="12"
              maxLength={2}
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="exp_year">Exp Year</label>
            <input
              type="text"
              id="exp_year"
              name="EXP_YEAR"
              placeholder="2025"
              maxLength={4}
              required
            />
          </div>

          <div className="form-group">
            <label htmlFor="cvv">CVV</label>
            <input
              type="text"
              id="cvv"
              name="CVV"
              placeholder="123"
              maxLength={4}
              required
            />
          </div>
        </div>

        <div className="form-group">
          <label htmlFor="first_name">First Name</label>
          <input type="text" id="first_name" name="FIRST_NAME" required />
        </div>

        <div className="form-group">
          <label htmlFor="last_name">Last Name</label>
          <input type="text" id="last_name" name="LAST_NAME" required />
        </div>

        <div className="form-group">
          <label htmlFor="zip_code">Zip Code</label>
          <input type="text" id="zip_code" name="ZIP_CODE" maxLength={10} />
        </div>

        <button type="submit" className="pay-button">
          Pay ${formData.amount}
        </button>
      </form>

      <div className="test-cards">
        <p><strong>Test Cards:</strong></p>
        <ul>
          <li>Visa: 4111111111111111</li>
          <li>Mastercard: 5499740000000057</li>
          <li>CVV: Any 3 digits (123)</li>
          <li>Expiry: Any future date (12/2025)</li>
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
        .payment-form { max-width: 500px; margin: 50px auto; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; font-weight: bold; }
        input { width: 100%; padding: 10px; border: 1px solid #ddd; }
        button { width: 100%; padding: 15px; background: #4CAF50; color: white; }
    </style>
</head>
<body>
    <div class="payment-form" id="paymentForm">Loading...</div>

    <script>
        // Fetch form data from backend
        fetch('/api/v1/payments/browser-post/form?amount=99.99')
            .then(res => res.json())
            .then(data => {
                document.getElementById('paymentForm').innerHTML = `
                    <h2>Payment: $${data.amount}</h2>
                    <form method="POST" action="${data.postURL}">
                        <!-- Hidden Fields -->
                        ${data.tac ? `<input type="hidden" name="TAC" value="${data.tac}">` : ''}
                        <input type="hidden" name="CUST_NBR" value="${data.custNbr}">
                        <input type="hidden" name="MERCH_NBR" value="${data.merchNbr}">
                        <input type="hidden" name="DBA_NBR" value="${data.dBAnbr}">
                        <input type="hidden" name="TERMINAL_NBR" value="${data.terminalNbr}">
                        <input type="hidden" name="TRAN_CODE" value="${data.tranCode}">
                        <input type="hidden" name="TRAN_NBR" value="${data.tranNbr}">
                        <input type="hidden" name="TRAN_GROUP" value="${data.tranGroup}">
                        <input type="hidden" name="AMOUNT" value="${data.amount}">
                        <input type="hidden" name="INDUSTRY_TYPE" value="${data.industryType}">
                        <input type="hidden" name="CARD_ENT_METH" value="${data.cardEntMeth}">
                        <input type="hidden" name="REDIRECT_URL" value="${data.redirectURL}">

                        <!-- Card Details -->
                        <div class="form-group">
                            <label>Card Number</label>
                            <input type="text" name="CARD_NBR" placeholder="4111111111111111" maxlength="16" required>
                        </div>

                        <div class="form-group">
                            <label>Expiry Month (MM)</label>
                            <input type="text" name="EXP_MONTH" placeholder="12" maxlength="2" required>
                        </div>

                        <div class="form-group">
                            <label>Expiry Year (YYYY)</label>
                            <input type="text" name="EXP_YEAR" placeholder="2025" maxlength="4" required>
                        </div>

                        <div class="form-group">
                            <label>CVV</label>
                            <input type="text" name="CVV" placeholder="123" maxlength="4" required>
                        </div>

                        <div class="form-group">
                            <label>First Name</label>
                            <input type="text" name="FIRST_NAME" required>
                        </div>

                        <div class="form-group">
                            <label>Last Name</label>
                            <input type="text" name="LAST_NAME" required>
                        </div>

                        <button type="submit">Pay $${data.amount}</button>
                    </form>
                `;
            })
            .catch(err => {
                document.getElementById('paymentForm').innerHTML =
                    '<p>Error loading payment form. Please try again.</p>';
            });
    </script>
</body>
</html>
```

---

## Step 4: Backend - Handle Callback

EPX will POST the payment results to your `REDIRECT_URL`:

```go
// POST /api/v1/payments/browser-post/callback
func (h *BrowserPostHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    // 1. Parse form data from EPX
    if err := r.ParseForm(); err != nil {
        http.Error(w, "Invalid form data", http.StatusBadRequest)
        return
    }

    // 2. Extract response fields
    response := &ports.BrowserPostResponse{
        AuthGUID:     r.FormValue("AUTH_GUID"),
        AuthResp:     r.FormValue("AUTH_RESP"),
        AuthCode:     r.FormValue("AUTH_CODE"),
        Amount:       r.FormValue("AMOUNT"),
        TranNbr:      r.FormValue("TRAN_NBR"),
        IsApproved:   r.FormValue("AUTH_RESP") == "00",
        RespMsg:      r.FormValue("RESP_MSG"),
        // ... extract other fields
    }

    // 3. Verify HMAC signature (security!)
    receivedMAC := r.FormValue("MAC")
    if !h.verifyHMAC(response, receivedMAC) {
        h.logger.Error("Invalid HMAC signature")
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // 4. Store transaction in database
    if err := h.paymentService.StoreTransaction(r.Context(), response); err != nil {
        h.logger.Error("Failed to store transaction", zap.Error(err))
        http.Error(w, "Internal error", http.StatusInternalServerError)
        return
    }

    // 5. Redirect user to success/failure page
    if response.IsApproved {
        http.Redirect(w, r, "/payment/success?txn="+response.AuthGUID, http.StatusSeeOther)
    } else {
        http.Redirect(w, r, "/payment/failed?reason="+response.RespMsg, http.StatusSeeOther)
    }
}
```

---

## Required Form Fields

### Hidden Fields (From Backend)

| Field | Description | Example |
|-------|-------------|---------|
| `TAC` | Terminal Authorization Code (if required) | Generated token |
| `CUST_NBR` | Customer number | `9001` |
| `MERCH_NBR` | Merchant number | `900300` |
| `DBA_NBR` | DBA number | `2` |
| `TERMINAL_NBR` | Terminal number | `77` |
| `TRAN_CODE` | Transaction code | `SALE` |
| `TRAN_NBR` | Unique transaction number | `12345` |
| `TRAN_GROUP` | Transaction group | `SALE` |
| `AMOUNT` | Transaction amount | `99.99` |
| `INDUSTRY_TYPE` | Industry type | `E` (ecommerce) |
| `CARD_ENT_METH` | Card entry method | `E` (ecommerce) |
| `REDIRECT_URL` | Callback URL | `https://yourdomain.com/callback` |

### User Input Fields

| Field | Description | Example |
|-------|-------------|---------|
| `CARD_NBR` | Card number | `4111111111111111` |
| `EXP_MONTH` | Expiry month | `12` |
| `EXP_YEAR` | Expiry year | `2025` |
| `CVV` | Card security code | `123` |
| `FIRST_NAME` | Cardholder first name | `John` |
| `LAST_NAME` | Cardholder last name | `Doe` |
| `ZIP_CODE` | Billing zip code (optional) | `10001` |

---

## EPX Endpoints

### Sandbox (Testing)

```
https://secure.epxuap.com/browserpost
```

### Production

```
https://secure.epxnow.com/browserpost
```

---

## Security Best Practices

### 1. HMAC Verification

**Always verify the HMAC signature** in the callback to ensure the response came from EPX:

```go
func (h *BrowserPostHandler) verifyHMAC(response *BrowserPostResponse, receivedMAC string) bool {
    // Concatenate fields in specific order
    message := response.TranNbr + response.Amount + response.AuthGUID + response.AuthResp

    // Calculate HMAC-SHA256
    mac := hmac.New(sha256.New, []byte(h.config.EPX.MAC))
    mac.Write([]byte(message))
    expectedMAC := hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(receivedMAC), []byte(expectedMAC))
}
```

### 2. Use HTTPS

- Always use HTTPS for your callback URL
- EPX requires HTTPS in production

### 3. Validate Amounts

```go
// Verify amount matches what you expected
expectedAmount := getExpectedAmount(response.TranNbr)
if response.Amount != expectedAmount {
    h.logger.Error("Amount mismatch",
        zap.String("expected", expectedAmount),
        zap.String("received", response.Amount))
    return errors.New("amount mismatch")
}
```

### 4. Idempotency

```go
// Check if transaction already processed
existing, err := h.repo.GetTransactionByTranNbr(ctx, response.TranNbr)
if err == nil && existing != nil {
    h.logger.Warn("Duplicate callback received", zap.String("tran_nbr", response.TranNbr))
    // Return success but don't reprocess
    return nil
}
```

---

## Testing

### Test Card Numbers

**Approved:**
- Visa: `4111111111111111`
- Mastercard: `5499740000000057`
- Amex: `378282246310005`

**Declined:**
- Visa: `4000000000000002`

**Test CVV:** Any 3 digits (e.g., `123`)
**Test Expiry:** Any future date (e.g., `12/2025`)

### Quick Test

1. Open the test HTML file:

```bash
firefox test_browser_post.html
```

2. Use test card: `4111111111111111`
3. Submit form
4. EPX processes and redirects to your callback
5. Verify transaction stored in your database

---

## Common Issues

### Issue 1: "Invalid TAC"

**Cause:** TAC token expired (4 hour TTL) or not generated correctly
**Solution:** Generate a fresh TAC token before rendering the form

### Issue 2: "Invalid REDIRECT_URL"

**Cause:** Callback URL not whitelisted with EPX
**Solution:** Contact EPX to whitelist your callback URL

### Issue 3: Callback not receiving data

**Cause:** CORS or HTTPS issues
**Solution:** Ensure your callback endpoint accepts POST requests and uses HTTPS in production

### Issue 4: HMAC verification failing

**Cause:** Wrong MAC key or incorrect field concatenation
**Solution:** Verify your MAC key with EPX and check field order in HMAC calculation

---

## Summary

**Browser Post Flow:**

1. ✅ Backend generates form data (with or without TAC)
2. ✅ Frontend renders HTML form with hidden fields
3. ✅ User enters card details and submits
4. ✅ Form POSTs directly to EPX (card data never hits your server)
5. ✅ EPX processes payment
6. ✅ EPX redirects to your callback with results
7. ✅ Backend verifies HMAC, stores transaction, redirects user

**Key Benefits:**

- 🔒 PCI compliant (card data never touches your server)
- ⚡ Simple integration (just an HTML form)
- 🛡️ Secure with HMAC verification
- 💳 Supports all major card brands

**Next Steps:**

- Implement backend form data generation
- Create frontend payment form
- Set up callback handler with HMAC verification
- Test with sandbox credentials
- Move to production with production credentials
