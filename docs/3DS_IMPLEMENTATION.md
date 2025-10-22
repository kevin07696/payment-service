# 3D Secure (3DS) Implementation Guide

## Overview

**3D Secure (3DS)** is an additional authentication layer for credit/debit card payments that significantly reduces fraud and shifts liability from merchants to card issuers.

**Status:** 🚧 **BLOCKED - Awaiting North Gateway 3DS Documentation**

### Investigation Findings (2025-10-21):

❌ **Browser Post API does NOT mention 3DS support**
- Reviewed official Browser Post API Integration Guide
- No 3DS fields in TAC request
- No 3DS fields in payment form (ACCOUNT_NBR, CVV2, EXP_DATE only)
- No 3DS fields in response (no ECI, CAVV, XID, liability shift, etc.)
- No 3DS endpoints documented

⚠️ **Next Steps Required:**
1. Contact North/EPX support to confirm if they support 3DS at all
2. If yes, ask which API/product provides 3DS (Browser Post doesn't appear to)
3. Request 3DS-specific API documentation
4. Determine if 3DS requires different integration method (hosted page, different API, etc.)

**Implementation blocked until North provides 3DS documentation.**

## What is 3D Secure?

### Card Network Implementations

| Network | 3DS Name | Version |
|---------|----------|---------|
| Visa | Visa Secure | 3DS 2.0 |
| Mastercard | Mastercard Identity Check | 3DS 2.0 |
| American Express | American Express SafeKey | 3DS 2.0 |
| Discover | ProtectBuy | 3DS 2.0 |

### 3DS 2.0 vs 3DS 1.0

**3DS 1.0 (Legacy):**
- Full-page redirect to bank
- Always requires user interaction
- Poor mobile experience
- High cart abandonment (~20-30%)

**3DS 2.0 (Modern - Recommended):**
- Seamless in-app/in-browser experience
- Risk-based authentication (frictionless when low risk)
- Better mobile support
- Lower cart abandonment (~5-10%)
- Shares more data with issuer for better fraud detection

## Why Implement 3DS?

### Benefits

**1. Fraud Reduction**
- 70-90% reduction in card-not-present fraud
- Issuer validates cardholder identity

**2. Liability Shift**
- Chargebacks become issuer's responsibility (not yours)
- Exception: If merchant doesn't attempt 3DS when available

**3. Regulatory Compliance**
- **Required in Europe:** Strong Customer Authentication (SCA) under PSD2
- **Optional in US:** But increasingly required by issuers for high-value transactions

**4. Higher Authorization Rates**
- Some issuers decline transactions without 3DS
- 3DS can increase approval rates by 5-15%

### Drawbacks

**1. Implementation Complexity**
- Requires frontend redirect handling
- Challenge flow management
- Fallback handling

**2. Potential Cart Abandonment**
- Users may abandon if authentication fails
- Older 3DS 1.0 has high abandonment

**3. Latency**
- Adds 2-5 seconds to transaction (user authentication)

## Architecture Flow

### 3DS 2.0 Flow

```
┌──────────────┐
│   Customer   │
│   (Browser)  │
└──────┬───────┘
       │ 1. Enters card details
       │
       v
┌──────────────────────────┐
│  Your Frontend           │
│  (North JavaScript SDK)  │
└──────────┬───────────────┘
           │ 2. Tokenize card + request 3DS
           │
           v
┌──────────────────────────┐
│  North Gateway           │
│  (Tokenization + 3DS)    │
└──────────┬───────────────┘
           │ 3. Initiate 3DS authentication
           │
           v
┌──────────────────────────┐
│  Card Issuer             │
│  (Risk Assessment)       │
└──────────┬───────────────┘
           │
           ├──► 4a. Low Risk → Frictionless (no challenge)
           │                   Returns authentication token
           │
           └──► 4b. High Risk → Challenge required
                               Returns challenge URL

                               ┌──────────────────┐
                               │ Customer Browser │
                               │ (Redirect to     │
                               │  bank page)      │
                               └────────┬─────────┘
                                        │ 5. User enters password/
                                        │    biometric/SMS code
                                        v
                               ┌──────────────────┐
                               │  Card Issuer     │
                               │  (Validates)     │
                               └────────┬─────────┘
                                        │ 6. Redirect back with
                                        │    authentication result
                                        v
┌──────────────────────────────────────────────┐
│  Your Frontend                                │
│  (Receives authentication result)            │
└──────────────────┬───────────────────────────┘
                   │ 7. Send authenticated token to backend
                   │
                   v
┌──────────────────────────────────────────────┐
│  Your Backend                                 │
│  (Process payment with 3DS authentication)   │
└──────────────────┬───────────────────────────┘
                   │ 8. Authorize payment
                   │
                   v
┌──────────────────────────────────────────────┐
│  North Gateway                                │
│  (Process authenticated payment)             │
└───────────────────────────────────────────────┘
```

## Implementation

### Step 1: Frontend - Initiate 3DS

Update the Browser Post tokenization to request 3DS:

```javascript
// Enhanced tokenization with 3DS
NorthBrowserPost.tokenizeWith3DS({
    cardNumber: cardNumber,
    expMonth: expMonth,
    expYear: expYear,
    cvv: cvv,
    zipCode: zipCode,

    // 3DS parameters
    threeDSecure: {
        enabled: true,
        version: '2.0',  // Prefer 3DS 2.0

        // Transaction details (helps with risk assessment)
        amount: '99.99',
        currency: 'USD',

        // Customer details (required for 3DS 2.0)
        billingAddress: {
            firstName: 'John',
            lastName: 'Doe',
            street1: '123 Main St',
            city: 'New York',
            state: 'NY',
            postalCode: '12345',
            country: 'US'
        },
        email: 'john@example.com',
        phone: '+15551234567'
    }
}, function(response) {
    if (response.success) {
        if (response.threeDSecure.required) {
            // 3DS challenge required - redirect user
            handle3DSChallenge(response);
        } else {
            // Frictionless or 3DS not required
            processPayment(response.token);
        }
    } else {
        handleError(response.error);
    }
});
```

### Step 2: Frontend - Handle 3DS Challenge

```javascript
function handle3DSChallenge(response) {
    const challengeUrl = response.threeDSecure.challengeUrl;
    const sessionId = response.threeDSecure.sessionId;

    // Option A: Redirect (full page)
    window.location.href = challengeUrl;

    // Option B: iframe (better UX)
    const iframe = document.createElement('iframe');
    iframe.src = challengeUrl;
    iframe.width = '400';
    iframe.height = '600';
    iframe.style.border = 'none';

    document.getElementById('3ds-container').appendChild(iframe);

    // Listen for completion message
    window.addEventListener('message', function(event) {
        if (event.origin === 'https://secure.epxuap.com') {
            const result = event.data;

            if (result.status === 'authenticated') {
                // User completed authentication
                processPayment(response.token, result.authenticationId);
            } else if (result.status === 'failed') {
                // Authentication failed
                handleAuthenticationFailure(result);
            } else if (result.status === 'cancelled') {
                // User cancelled
                handleCancellation();
            }
        }
    });
}
```

### Step 3: Frontend - Process Authenticated Payment

```javascript
async function processPayment(bricToken, authenticationId) {
    try {
        const response = await fetch('/api/payment/authorize', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': 'Bearer ' + userJwt
            },
            body: JSON.stringify({
                merchantId: 'MERCH-001',
                customerId: 'CUST-12345',
                amount: '99.99',
                currency: 'USD',
                token: bricToken,
                capture: true,

                // Include 3DS authentication data
                threeDSecure: {
                    authenticationId: authenticationId,
                    version: '2.0',
                    eci: '05',  // Electronic Commerce Indicator
                    cavv: 'AAABCSIIAAAAAAACcwgAAAAAAAA=',  // From 3DS response
                    xid: 'MDAwMDAwMDAwMDAwMDAwMzIyNzY='  // Transaction ID
                },

                billingInfo: { /* ... */ },
                idempotencyKey: generateIdempotencyKey()
            })
        });

        const result = await response.json();

        if (result.status === 'TRANSACTION_STATUS_CAPTURED') {
            // Success!
            window.location.href = '/success';
        } else {
            handlePaymentFailure(result);
        }

    } catch (error) {
        handleError(error);
    }
}
```

### Step 4: Backend - Update Proto Definition

Add 3DS fields to payment proto:

```protobuf
// api/proto/payment/v1/payment.proto

message AuthorizeRequest {
    string merchant_id = 1;
    string customer_id = 2;
    string amount = 3;
    string currency = 4;
    string token = 5;
    bool capture = 6;
    BillingInfo billing_info = 7;
    string idempotency_key = 8;
    map<string, string> metadata = 9;

    // 3D Secure authentication data
    ThreeDSecureData three_d_secure = 10;
}

message ThreeDSecureData {
    string authentication_id = 1;     // 3DS session ID
    string version = 2;                // "2.0" or "1.0"
    string eci = 3;                    // Electronic Commerce Indicator
    string cavv = 4;                   // Cardholder Authentication Verification Value
    string xid = 5;                    // Transaction identifier
    string directory_server_transaction_id = 6;  // 3DS 2.0 only
}

message AuthorizeResponse {
    // ... existing fields ...

    // 3DS authentication result
    ThreeDSecureResult three_d_secure_result = 11;
}

message ThreeDSecureResult {
    bool authenticated = 1;
    string liability_shift = 2;  // "merchant" or "issuer"
    string status = 3;            // "Y" (authenticated), "N" (failed), "A" (attempted)
}
```

### Step 5: Backend - Update BrowserPostAdapter

```go
// internal/adapters/north/browser_post_adapter.go

func (a *BrowserPostAdapter) Authorize(ctx context.Context, req *ports.AuthorizeRequest) (*ports.PaymentResult, error) {
    // ... existing validation ...

    parts := strings.Split(a.config.EPIId, "-")
    if len(parts) != 4 {
        return nil, pkgerrors.NewPaymentError("CONFIG_ERROR", "Invalid EPI-Id format", pkgerrors.CategoryInvalidRequest, false)
    }

    formData := url.Values{}
    formData.Set("CUST_NBR", parts[0])
    formData.Set("MERCH_NBR", parts[1])
    formData.Set("DBA_NBR", parts[2])
    formData.Set("TERMINAL_NBR", parts[3])
    formData.Set("BRIC", req.Token)
    formData.Set("AMOUNT", fmt.Sprintf("%.2f", req.Amount.InexactFloat64()))
    formData.Set("CURRENCY", req.Currency)

    // Add 3DS data if present
    if req.ThreeDSecure != nil {
        formData.Set("3DS_VERSION", req.ThreeDSecure.Version)
        formData.Set("3DS_ECI", req.ThreeDSecure.ECI)
        formData.Set("3DS_CAVV", req.ThreeDSecure.CAVV)
        formData.Set("3DS_XID", req.ThreeDSecure.XID)

        if req.ThreeDSecure.Version == "2.0" {
            formData.Set("3DS_DS_TRANS_ID", req.ThreeDSecure.DirectoryServerTransactionID)
        }
    }

    tranType := "A"
    if req.Capture {
        tranType = "S"
    }
    formData.Set("TRAN_TYPE", tranType)

    // ... rest of authorization logic ...

    // Parse 3DS result from response
    threeDSResult := &ports.ThreeDSecureResult{
        Authenticated: a.getFieldValue(resp, "3DS_AUTH") == "Y",
        LiabilityShift: a.getFieldValue(resp, "3DS_LIABILITY"),
        Status: a.getFieldValue(resp, "3DS_STATUS"),
    }

    return &ports.PaymentResult{
        TransactionID:        transactionID,
        GatewayTransactionID: transactionID,
        Amount:               req.Amount,
        Status:               status,
        ResponseCode:         responseCode,
        Message:              responseText,
        AuthCode:             authCode,
        ThreeDSecureResult:   threeDSResult,
        Timestamp:            time.Now(),
    }, nil
}
```

### Step 6: Backend - Update Domain Ports

```go
// internal/domain/ports/payment_gateway.go

type AuthorizeRequest struct {
    MerchantID   string
    CustomerID   string
    Amount       decimal.Decimal
    Currency     string
    Token        string
    Capture      bool
    BillingInfo  BillingInfo
    ThreeDSecure *ThreeDSecureData  // Optional 3DS data
}

type ThreeDSecureData struct {
    AuthenticationID              string
    Version                       string  // "2.0" or "1.0"
    ECI                          string  // Electronic Commerce Indicator
    CAVV                         string  // Cardholder Authentication Verification Value
    XID                          string  // Transaction identifier
    DirectoryServerTransactionID string  // 3DS 2.0 only
}

type PaymentResult struct {
    TransactionID        string
    GatewayTransactionID string
    Amount               decimal.Decimal
    Status               models.TransactionStatus
    ResponseCode         string
    Message              string
    AuthCode             string
    ThreeDSecureResult   *ThreeDSecureResult  // 3DS authentication result
    Timestamp            time.Time
}

type ThreeDSecureResult struct {
    Authenticated  bool
    LiabilityShift string  // "merchant" or "issuer"
    Status         string  // "Y" (authenticated), "N" (failed), "A" (attempted)
}
```

## Testing 3DS

### Test Cards (Standard)

Most gateways provide test cards for 3DS testing:

| Card Number | 3DS Behavior |
|-------------|--------------|
| 4000 0000 0000 3220 | 3DS 2.0 - Frictionless (no challenge) |
| 4000 0000 0000 3238 | 3DS 2.0 - Challenge required |
| 4000 0000 0000 3246 | 3DS 2.0 - Authentication failed |
| 4000 0000 0000 3253 | 3DS 1.0 - Challenge required |

**Test Authentication:**
- Any password/code typically works in test mode
- Check North documentation for specific test values

### Testing Scenarios

**1. Frictionless Flow (Low Risk)**
```javascript
// User enters card
// 3DS assessment: Low risk
// No challenge required
// Payment proceeds immediately
```

**2. Challenge Flow (High Risk)**
```javascript
// User enters card
// 3DS assessment: High risk
// User redirected to bank page
// User enters password/biometric
// Returns to merchant site
// Payment proceeds
```

**3. Authentication Failed**
```javascript
// User enters card
// 3DS challenge required
// User enters wrong password
// Authentication fails
// Payment declined
```

**4. Challenge Timeout**
```javascript
// User enters card
// 3DS challenge presented
// User doesn't complete (closes window)
// Timeout after 5 minutes
// Payment abandoned
```

## Error Handling

### 3DS-Specific Errors

```javascript
function handle3DSError(error) {
    const errorCodes = {
        '3DS_AUTH_FAILED': 'Authentication failed. Please try again or use a different card.',
        '3DS_TIMEOUT': 'Authentication timed out. Please try again.',
        '3DS_CANCELLED': 'Authentication was cancelled.',
        '3DS_NOT_ENROLLED': 'This card is not enrolled in 3D Secure.',
        '3DS_SYSTEM_ERROR': 'Authentication service unavailable. Please try again later.',
        '3DS_INVALID_CARD': 'This card cannot be authenticated.'
    };

    return errorCodes[error.code] || 'Authentication failed. Please try again.';
}
```

### Fallback Strategy

```javascript
async function processPaymentWith3DS(cardData) {
    try {
        // Attempt 3DS authentication
        const threeDSResult = await authenticate3DS(cardData);

        if (threeDSResult.authenticated) {
            // Proceed with authenticated payment
            return await authorizePayment(cardData.token, threeDSResult);
        } else {
            // Authentication failed
            throw new Error('3DS_AUTH_FAILED');
        }

    } catch (error) {
        if (error.code === '3DS_NOT_ENROLLED') {
            // Card not enrolled in 3DS - proceed without it
            console.warn('Card not enrolled in 3DS, proceeding without authentication');
            return await authorizePayment(cardData.token, null);
        } else {
            // Other 3DS errors - fail the payment
            throw error;
        }
    }
}
```

## Configuration

### Environment Variables

```bash
# Enable/disable 3DS
ENABLE_3DS=true

# 3DS version preference
3DS_VERSION=2.0

# Challenge preference
3DS_CHALLENGE_PREFERENCE=no_preference  # or: no_challenge, challenge_preferred

# Minimum amount for 3DS (cents)
3DS_MIN_AMOUNT=5000  # $50.00 and above require 3DS
```

### Dynamic 3DS Rules

```go
// internal/services/payment/3ds_rules.go

func shouldRequire3DS(req *AuthorizeRequest) bool {
    // High-value transactions
    if req.Amount.GreaterThan(decimal.NewFromFloat(50.00)) {
        return true
    }

    // International cards
    if req.BillingInfo.Country != "US" {
        return true
    }

    // First transaction for customer
    if isFirstTransaction(req.CustomerID) {
        return true
    }

    // High-risk countries
    if isHighRiskCountry(req.BillingInfo.Country) {
        return true
    }

    return false
}
```

## Monitoring

### Metrics to Track

```go
// 3DS success rate
payment_3ds_attempts_total{result="authenticated"} 850
payment_3ds_attempts_total{result="failed"} 50
payment_3ds_attempts_total{result="not_enrolled"} 100

// 3DS challenge rate
payment_3ds_challenge_required_total 200
payment_3ds_frictionless_total 800

// Cart abandonment during 3DS
payment_3ds_abandoned_total 30

// Liability shift
payment_liability_shift_total{shift_to="issuer"} 850
payment_liability_shift_total{shift_to="merchant"} 150
```

## Next Steps

1. **Contact North Payment Gateway**
   - Confirm 3DS support (version 2.0 preferred)
   - Request 3DS API documentation
   - Get test credentials

2. **Update Proto Files**
   - Add 3DS fields to payment.proto
   - Regenerate Go code: `make proto`

3. **Implement Backend**
   - Update BrowserPostAdapter
   - Update domain ports
   - Add 3DS validation

4. **Implement Frontend**
   - Update tokenization to request 3DS
   - Handle challenge flow (iframe or redirect)
   - Handle authentication results

5. **Test Thoroughly**
   - Test frictionless flow
   - Test challenge flow
   - Test error cases
   - Test on mobile devices

6. **Deploy**
   - Start with test environment
   - Monitor 3DS success rates
   - Gradually roll out to production

## Resources

- **EMVCo 3DS Spec:** https://www.emvco.com/emv-technologies/3d-secure/
- **PSD2 SCA Requirements:** https://ec.europa.eu/info/law/payment-services-psd-2-directive-eu-2015-2366_en
- **Visa Secure:** https://usa.visa.com/pay-with-visa/featured-technologies/visa-secure.html
- **Mastercard Identity Check:** https://www.mastercard.us/en-us/merchants/safety-security/identity-check.html

## Status

**Current Status:** 🚧 Awaiting North Gateway confirmation

**Next Action:** Contact North to confirm 3DS support and get API documentation

**Priority:** Medium-High (required for Europe, beneficial for US)
