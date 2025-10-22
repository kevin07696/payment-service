# Frontend Integration Guide - Payment Microservice

## Overview

This guide explains how to integrate your frontend application with the payment microservice using **PCI-compliant tokenization**. The backend **NEVER** receives raw card data - all card tokenization happens directly between the user's browser and North Payment Gateway.

## Architecture Flow

```
┌─────────────┐
│   Browser   │
│   (User)    │
└──────┬──────┘
       │
       │ 1. User enters card details (4111-1111-1111-1111)
       │
       ├──────────────────────────────────────────────┐
       │                                              │
       │ 2. JavaScript SDK tokenizes card             │
       │    (posts directly to North, bypasses our    │
       │     backend - PCI compliance!)               │
       │                                              │
       v                                              │
┌──────────────────┐                                 │
│ North Gateway    │                                 │
│ (Tokenization)   │                                 │
└────────┬─────────┘                                 │
         │                                            │
         │ 3. Returns BRIC token                     │
         │    "tok_abc123xyz..."                     │
         │                                            │
         └──────────────────────────────────────────>│
                                                      │
                                                      │ 4. Send token to backend
                                                      │    POST /api/payment/authorize
                                                      │    { token: "tok_abc123..." }
                                                      │
                                                      v
                                              ┌───────────────┐
                                              │ Our Backend   │
                                              │ (gRPC Server) │
                                              └───────┬───────┘
                                                      │
                                                      │ 5. Process payment with token
                                                      │    (Backend never sees card!)
                                                      │
                                                      v
                                              ┌──────────────────┐
                                              │ North Gateway    │
                                              │ (Payment)        │
                                              └──────────────────┘
```

## Step 1: Include North Browser Post SDK

Add the North JavaScript SDK to your HTML:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Payment Form</title>
    <!-- Include North Browser Post SDK -->
    <script src="https://secure.epxuap.com/browserpost.js"></script>
</head>
<body>
    <!-- Your payment form here -->
</body>
</html>
```

**Note:** Check with North for the actual SDK URL - this is a placeholder.

## Step 2: Create Payment Form (Card Details)

Create a form for collecting card details. **IMPORTANT:** Do NOT submit this form to your backend!

```html
<form id="payment-form">
    <div>
        <label for="card-number">Card Number</label>
        <input
            type="text"
            id="card-number"
            placeholder="4111 1111 1111 1111"
            maxlength="19"
        />
    </div>

    <div>
        <label for="exp-month">Expiration Month</label>
        <input
            type="text"
            id="exp-month"
            placeholder="MM"
            maxlength="2"
        />
    </div>

    <div>
        <label for="exp-year">Expiration Year</label>
        <input
            type="text"
            id="exp-year"
            placeholder="YYYY"
            maxlength="4"
        />
    </div>

    <div>
        <label for="cvv">CVV</label>
        <input
            type="text"
            id="cvv"
            placeholder="123"
            maxlength="4"
        />
    </div>

    <div>
        <label for="zip">ZIP Code</label>
        <input
            type="text"
            id="zip"
            placeholder="12345"
            maxlength="10"
        />
    </div>

    <button type="submit">Pay Now</button>
</form>

<div id="error-message" style="color: red; display: none;"></div>
```

## Step 3: Tokenize Card (JavaScript)

When the user submits the form, tokenize the card **before** sending to your backend:

```javascript
document.getElementById('payment-form').addEventListener('submit', function(e) {
    e.preventDefault(); // Don't submit form normally!

    const errorDiv = document.getElementById('error-message');
    errorDiv.style.display = 'none';

    // Collect card data
    const cardData = {
        cardNumber: document.getElementById('card-number').value.replace(/\s/g, ''),
        expMonth: document.getElementById('exp-month').value,
        expYear: document.getElementById('exp-year').value,
        cvv: document.getElementById('cvv').value,
        zipCode: document.getElementById('zip').value
    };

    // Validate card data client-side
    if (!validateCardData(cardData)) {
        errorDiv.textContent = 'Please check your card details';
        errorDiv.style.display = 'block';
        return;
    }

    // Tokenize card using North SDK
    // This posts DIRECTLY to North (bypasses your backend)
    NorthBrowserPost.tokenize(cardData, function(response) {
        if (response.success && response.token) {
            // Success! We have a BRIC token
            const bricToken = response.token; // e.g., "tok_abc123xyz..."

            // Now send the TOKEN to your backend (NOT the card number!)
            processPaymentWithToken(bricToken, cardData.zipCode);
        } else {
            // Tokenization failed
            errorDiv.textContent = response.error || 'Failed to process card';
            errorDiv.style.display = 'block';
        }
    });
});

function validateCardData(data) {
    // Basic validation - enhance as needed
    return data.cardNumber.length >= 13 &&
           data.expMonth.length === 2 &&
           data.expYear.length === 4 &&
           data.cvv.length >= 3 &&
           data.zipCode.length >= 5;
}
```

## Step 4: Send Token to Your Backend

Once you have the BRIC token, send it to your backend API:

```javascript
async function processPaymentWithToken(bricToken, zipCode) {
    try {
        const response = await fetch('http://localhost:8080/api/payment/authorize', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': 'Bearer YOUR_JWT_TOKEN' // Your auth token
            },
            body: JSON.stringify({
                merchantId: 'MERCH-001',        // Your merchant ID
                customerId: 'CUST-12345',       // Customer ID from your system
                amount: '99.99',                // Amount to charge
                currency: 'USD',
                token: bricToken,               // BRIC token (NOT card number!)
                capture: true,                  // true = immediate capture, false = auth only
                billingInfo: {
                    firstName: 'John',
                    lastName: 'Doe',
                    email: 'john@example.com',
                    phone: '555-1234',
                    address: {
                        street1: '123 Main St',
                        city: 'New York',
                        state: 'NY',
                        postalCode: zipCode,
                        country: 'US'
                    }
                },
                idempotencyKey: generateIdempotencyKey(), // Prevent duplicate charges
                metadata: {
                    orderId: 'ORDER-123',
                    description: 'Premium Subscription'
                }
            })
        });

        if (!response.ok) {
            throw new Error('Payment failed');
        }

        const result = await response.json();

        if (result.status === 'TRANSACTION_STATUS_CAPTURED') {
            // Payment successful!
            window.location.href = '/success?transactionId=' + result.id;
        } else {
            // Payment failed
            document.getElementById('error-message').textContent =
                result.message || 'Payment declined';
            document.getElementById('error-message').style.display = 'block';
        }

    } catch (error) {
        console.error('Payment error:', error);
        document.getElementById('error-message').textContent =
            'An error occurred. Please try again.';
        document.getElementById('error-message').style.display = 'block';
    }
}

function generateIdempotencyKey() {
    // Generate unique key for this payment attempt
    return 'idem_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
}
```

## Step 5: Handle Response

Your backend returns a gRPC response (converted to JSON by your API gateway):

```json
{
  "id": "txn-abc123",
  "merchantId": "MERCH-001",
  "customerId": "CUST-12345",
  "amount": "99.99",
  "currency": "USD",
  "status": "TRANSACTION_STATUS_CAPTURED",
  "type": "TRANSACTION_TYPE_SALE",
  "paymentMethodType": "PAYMENT_METHOD_TYPE_CREDIT_CARD",
  "responseCode": "00",
  "message": "Approved",
  "createdAt": "2025-10-20T12:34:56Z"
}
```

## Complete React Example

```jsx
import React, { useState } from 'react';

function PaymentForm() {
    const [cardData, setCardData] = useState({
        cardNumber: '',
        expMonth: '',
        expYear: '',
        cvv: '',
        zipCode: ''
    });
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setLoading(true);

        try {
            // Tokenize card with North SDK
            const tokenResponse = await new Promise((resolve, reject) => {
                window.NorthBrowserPost.tokenize(cardData, (response) => {
                    if (response.success) {
                        resolve(response);
                    } else {
                        reject(response.error);
                    }
                });
            });

            const bricToken = tokenResponse.token;

            // Send token to backend
            const response = await fetch('/api/payment/authorize', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${localStorage.getItem('jwt')}`
                },
                body: JSON.stringify({
                    merchantId: process.env.REACT_APP_MERCHANT_ID,
                    customerId: getUserId(), // Your function
                    amount: getCartTotal(), // Your function
                    currency: 'USD',
                    token: bricToken,
                    capture: true,
                    billingInfo: {
                        // ... collect from form
                        address: {
                            postalCode: cardData.zipCode
                        }
                    },
                    idempotencyKey: `idem_${Date.now()}_${Math.random()}`
                })
            });

            if (!response.ok) {
                throw new Error('Payment failed');
            }

            const result = await response.json();

            if (result.status === 'TRANSACTION_STATUS_CAPTURED') {
                // Success - redirect to success page
                window.location.href = '/success';
            } else {
                setError(result.message || 'Payment declined');
            }

        } catch (err) {
            setError(err.message || 'An error occurred');
        } finally {
            setLoading(false);
        }
    };

    return (
        <form onSubmit={handleSubmit}>
            <input
                type="text"
                placeholder="Card Number"
                value={cardData.cardNumber}
                onChange={(e) => setCardData({...cardData, cardNumber: e.target.value})}
            />
            {/* ... other fields ... */}

            {error && <div className="error">{error}</div>}

            <button type="submit" disabled={loading}>
                {loading ? 'Processing...' : 'Pay Now'}
            </button>
        </form>
    );
}
```

## API Endpoints

### Authorize Payment

**Endpoint:** `POST /api/payment/authorize`

**Request:**
```json
{
  "merchantId": "MERCH-001",
  "customerId": "CUST-12345",
  "amount": "99.99",
  "currency": "USD",
  "token": "tok_abc123xyz...",
  "capture": true,
  "billingInfo": {
    "firstName": "John",
    "lastName": "Doe",
    "email": "john@example.com",
    "phone": "555-1234",
    "address": {
      "street1": "123 Main St",
      "city": "New York",
      "state": "NY",
      "postalCode": "12345",
      "country": "US"
    }
  },
  "idempotencyKey": "idem_unique_key_123",
  "metadata": {
    "orderId": "ORDER-123"
  }
}
```

**Response (Success):**
```json
{
  "id": "txn-abc123",
  "status": "TRANSACTION_STATUS_CAPTURED",
  "amount": "99.99",
  "responseCode": "00",
  "message": "Approved",
  "authCode": "123456",
  "createdAt": "2025-10-20T12:34:56Z"
}
```

**Response (Declined):**
```json
{
  "id": "txn-abc123",
  "status": "TRANSACTION_STATUS_FAILED",
  "amount": "99.99",
  "responseCode": "51",
  "message": "Insufficient funds",
  "createdAt": "2025-10-20T12:34:56Z"
}
```

### Capture Previously Authorized Payment

**Endpoint:** `POST /api/payment/capture`

```json
{
  "transactionId": "txn-abc123",
  "amount": "99.99"
}
```

### Void Transaction

**Endpoint:** `POST /api/payment/void`

```json
{
  "transactionId": "txn-abc123"
}
```

### Refund Transaction

**Endpoint:** `POST /api/payment/refund`

```json
{
  "transactionId": "txn-abc123",
  "amount": "99.99",
  "reason": "Customer requested refund"
}
```

### Get Transaction

**Endpoint:** `GET /api/payment/transaction/{transactionId}`

## Subscription Payments

### Create Subscription

**Endpoint:** `POST /api/subscription/create`

```json
{
  "merchantId": "MERCH-001",
  "customerId": "CUST-12345",
  "amount": "29.99",
  "currency": "USD",
  "frequency": "BILLING_FREQUENCY_MONTHLY",
  "paymentMethodToken": "tok_abc123xyz...",
  "startDate": "2025-10-20T00:00:00Z",
  "maxRetries": 3,
  "failureOption": "FAILURE_OPTION_PAUSE",
  "idempotencyKey": "idem_sub_123",
  "metadata": {
    "planName": "Premium Monthly"
  }
}
```

**Response:**
```json
{
  "id": "sub-xyz789",
  "status": "SUBSCRIPTION_STATUS_ACTIVE",
  "amount": "29.99",
  "frequency": "BILLING_FREQUENCY_MONTHLY",
  "nextBillingDate": "2025-11-20T00:00:00Z",
  "createdAt": "2025-10-20T12:34:56Z"
}
```

## Testing with North Test Cards

Use these test card numbers (from North documentation):

| Card Number          | Result                |
|---------------------|-----------------------|
| 4111 1111 1111 1111 | Approved              |
| 4000 0000 0000 0002 | Declined (generic)    |
| 4000 0000 0000 0051 | Insufficient funds    |
| 4000 0000 0000 0119 | Processing error      |
| 4000 0000 0000 9995 | CVV mismatch          |

**Test CVV:** Any 3-4 digit number
**Test Expiration:** Any future date
**Test ZIP:** Any 5-digit ZIP code

## Security Best Practices

### 1. NEVER Send Raw Card Data to Your Backend

```javascript
// ❌ WRONG - Security violation!
fetch('/api/payment', {
    body: JSON.stringify({
        cardNumber: '4111111111111111',  // NO!
        cvv: '123'                        // NO!
    })
});

// ✅ CORRECT - PCI compliant
NorthBrowserPost.tokenize(cardData, function(response) {
    fetch('/api/payment', {
        body: JSON.stringify({
            token: response.token  // YES! Only send token
        })
    });
});
```

### 2. Use HTTPS Only

All API calls must use HTTPS (TLS 1.2+):
```javascript
// ❌ WRONG
fetch('http://api.example.com/payment', ...)

// ✅ CORRECT
fetch('https://api.example.com/payment', ...)
```

### 3. Implement Idempotency

Always send an idempotency key to prevent duplicate charges:

```javascript
const idempotencyKey = 'idem_' + orderId + '_' + Date.now();
```

### 4. Validate Input Client-Side

```javascript
function validateCard(cardNumber) {
    // Luhn algorithm
    let sum = 0;
    let isEven = false;

    for (let i = cardNumber.length - 1; i >= 0; i--) {
        let digit = parseInt(cardNumber[i]);

        if (isEven) {
            digit *= 2;
            if (digit > 9) digit -= 9;
        }

        sum += digit;
        isEven = !isEven;
    }

    return sum % 10 === 0;
}
```

### 5. Don't Store Card Data Locally

```javascript
// ❌ WRONG - Never store card data!
localStorage.setItem('cardNumber', cardNumber);
sessionStorage.setItem('cvv', cvv);

// ✅ CORRECT - Only store the token if needed
localStorage.setItem('paymentToken', bricToken);
```

## Error Handling

Common errors and how to handle them:

```javascript
function handlePaymentError(error) {
    const errorMessages = {
        '51': 'Insufficient funds. Please use a different card.',
        '54': 'Card expired. Please check the expiration date.',
        '82': 'CVV mismatch. Please check the security code.',
        '05': 'Card declined. Please contact your bank.',
        '59': 'Suspected fraud. Please contact your bank.',
        '96': 'System error. Please try again later.'
    };

    return errorMessages[error.responseCode] ||
           error.message ||
           'Payment failed. Please try again.';
}
```

## Rate Limiting

The API has rate limits to prevent abuse:

- **Payment operations:** 100 requests/minute per merchant
- **Query operations:** 1000 requests/minute per merchant

Handle rate limit errors:

```javascript
if (response.status === 429) {
    const retryAfter = response.headers.get('Retry-After');
    setTimeout(() => retryPayment(), retryAfter * 1000);
}
```

## Webhooks (Future)

In the future, you can subscribe to payment webhooks for async notifications:

```json
POST https://your-domain.com/webhooks/payment
{
  "event": "payment.succeeded",
  "transactionId": "txn-abc123",
  "amount": "99.99",
  "timestamp": "2025-10-20T12:34:56Z"
}
```

## Support

For integration support:

- **API Documentation:** See `/docs/API.md`
- **North SDK Issues:** Contact North support
- **Backend Issues:** Contact your backend team

## Appendix: Full HTML Example

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Payment Example</title>
    <script src="https://secure.epxuap.com/browserpost.js"></script>
    <style>
        .payment-form { max-width: 400px; margin: 50px auto; }
        .form-group { margin-bottom: 15px; }
        label { display: block; margin-bottom: 5px; }
        input { width: 100%; padding: 8px; }
        button { width: 100%; padding: 10px; background: #007bff; color: white; border: none; cursor: pointer; }
        .error { color: red; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="payment-form">
        <h2>Payment</h2>
        <form id="payment-form">
            <div class="form-group">
                <label>Card Number</label>
                <input type="text" id="card-number" placeholder="4111 1111 1111 1111" required>
            </div>
            <div class="form-group">
                <label>Expiration (MM/YYYY)</label>
                <input type="text" id="exp-month" placeholder="12" maxlength="2" required>
                <input type="text" id="exp-year" placeholder="2025" maxlength="4" required>
            </div>
            <div class="form-group">
                <label>CVV</label>
                <input type="text" id="cvv" placeholder="123" maxlength="4" required>
            </div>
            <div class="form-group">
                <label>ZIP Code</label>
                <input type="text" id="zip" placeholder="12345" required>
            </div>
            <button type="submit">Pay $99.99</button>
        </form>
        <div id="error" class="error" style="display:none;"></div>
    </div>

    <script>
        document.getElementById('payment-form').addEventListener('submit', async function(e) {
            e.preventDefault();

            const errorDiv = document.getElementById('error');
            errorDiv.style.display = 'none';

            const cardData = {
                cardNumber: document.getElementById('card-number').value.replace(/\s/g, ''),
                expMonth: document.getElementById('exp-month').value,
                expYear: document.getElementById('exp-year').value,
                cvv: document.getElementById('cvv').value,
                zipCode: document.getElementById('zip').value
            };

            // Tokenize with North
            NorthBrowserPost.tokenize(cardData, async function(response) {
                if (!response.success || !response.token) {
                    errorDiv.textContent = 'Failed to process card details';
                    errorDiv.style.display = 'block';
                    return;
                }

                // Send token to backend
                try {
                    const result = await fetch('http://localhost:8080/api/payment/authorize', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            merchantId: 'MERCH-001',
                            customerId: 'CUST-12345',
                            amount: '99.99',
                            currency: 'USD',
                            token: response.token,
                            capture: true,
                            billingInfo: {
                                address: {
                                    postalCode: cardData.zipCode
                                }
                            },
                            idempotencyKey: 'idem_' + Date.now()
                        })
                    });

                    const data = await result.json();

                    if (data.status === 'TRANSACTION_STATUS_CAPTURED') {
                        alert('Payment successful! Transaction ID: ' + data.id);
                    } else {
                        errorDiv.textContent = data.message || 'Payment declined';
                        errorDiv.style.display = 'block';
                    }
                } catch (err) {
                    errorDiv.textContent = 'Network error. Please try again.';
                    errorDiv.style.display = 'block';
                }
            });
        });
    </script>
</body>
</html>
```
