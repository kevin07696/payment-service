# API Design and Dataflow Documentation

## Overview

This document provides comprehensive design and dataflow documentation for all APIs and RPCs in the payment service.

## API Integration Points

The payment service integrates with EPX gateway through three main API channels:

### 1. Browser Post API (Customer-Facing Forms)

**Purpose**: Secure credit card data collection directly from customers

**Flow**: Customer → EPX Hosted Form → EPX Gateway → Server Post Callback → Your Server

**Use Cases**:
- Credit card payments (one-time or save for future)
- PCI-compliant card data collection
- Customer checkout flows

**Integration Methods**:

#### Option A: HTML Form Redirect (Traditional, Recommended)
```html
<form method="POST" action="https://epx.gateway.com/browserpost">
  <input type="hidden" name="MERCHANT_ID" value="your_merchant_id">
  <input type="hidden" name="TRAN_TYPE" value="CCE2">
  <input type="hidden" name="TRAN_AMT" value="29.99">
  <input type="hidden" name="CALLBACK_URL" value="https://yourserver.com/callback">
  <!-- EPX-hosted page collects card details -->
  <button type="submit">Pay Now</button>
</form>
```

**Pros**:
- No JavaScript required
- Card data never touches your server (PCI Level 1 compliant)
- Works on all browsers
- EPX handles all validation

**Cons**:
- Page redirect (leaves your site temporarily)
- Limited styling control

#### Option B: Iframe Embed (Better UX)
```html
<iframe
  src="https://epx.gateway.com/browserpost/form?merchant_id=..."
  width="500"
  height="400"
  style="border: none;">
</iframe>

<script>
  // Listen for EPX completion message
  window.addEventListener('message', function(event) {
    if (event.origin === 'https://epx.gateway.com') {
      // EPX sends result via postMessage
      const result = event.data;
      // Redirect or update UI
    }
  });
</script>
```

**Pros**:
- Customer stays on your page
- Seamless UX
- Still PCI compliant (iframe isolation)

**Cons**:
- Requires JavaScript
- iframe styling limitations

#### Option C: EPX.js SDK with JSON (Recommended - PCI Compliant)

**Modern JavaScript approach using JSON communication while maintaining PCI compliance.**

**Complete Implementation Example**:

```html
<!DOCTYPE html>
<html>
<head>
  <title>Payment Form</title>
  <style>
    /* Style the containers for EPX-hosted fields */
    .card-field {
      border: 1px solid #ccc;
      padding: 10px;
      border-radius: 4px;
      margin: 10px 0;
    }
    .card-field.error {
      border-color: red;
    }
  </style>
</head>
<body>
  <form id="payment-form">
    <h2>Payment Information</h2>

    <!-- EPX-hosted field containers (iframes injected here) -->
    <div id="card-number" class="card-field"></div>
    <div style="display: flex; gap: 10px;">
      <div id="card-expiry" class="card-field" style="flex: 1;"></div>
      <div id="card-cvv" class="card-field" style="flex: 1;"></div>
    </div>

    <!-- Billing information (your inputs, not PCI sensitive) -->
    <input type="text" id="billing-zip" placeholder="Billing ZIP">

    <!-- Transaction type selector -->
    <select id="tran-type">
      <option value="CCE2">Pay Now (Sale)</option>
      <option value="CCE1">Authorize Only</option>
      <option value="CCE8">Save Card (No Charge)</option>
    </select>

    <input type="number" id="amount" placeholder="Amount" step="0.01">

    <button type="submit">Submit Payment</button>
    <div id="error-message" style="color: red;"></div>
  </form>

  <!-- Load EPX.js SDK -->
  <script src="https://epx.gateway.com/epx.js"></script>

  <script>
    // Initialize EPX SDK
    const epx = EPXGateway.init({
      merchantId: 'your_merchant_id',
      environment: 'staging', // or 'production'
      apiKey: 'your_public_api_key' // Public key, safe to expose
    });

    // Create hosted fields (EPX-controlled secure iframes)
    // Card data NEVER enters your JavaScript - stays in EPX iframes
    const cardNumber = epx.fields.create('cardNumber', {
      placeholder: '1234 5678 9012 3456',
      style: {
        fontSize: '16px',
        fontFamily: 'Arial, sans-serif'
      }
    });

    const cardExpiry = epx.fields.create('cardExpiry', {
      placeholder: 'MM/YY'
    });

    const cardCvv = epx.fields.create('cardCvv', {
      placeholder: 'CVV'
    });

    // Mount fields to your DOM (injects secure iframes)
    cardNumber.mount('#card-number');
    cardExpiry.mount('#card-expiry');
    cardCvv.mount('#card-cvv');

    // Field validation events
    cardNumber.on('change', (event) => {
      if (event.error) {
        document.getElementById('card-number').classList.add('error');
      } else {
        document.getElementById('card-number').classList.remove('error');
      }
    });

    // Handle form submission
    document.getElementById('payment-form').addEventListener('submit', async (e) => {
      e.preventDefault();

      const tranType = document.getElementById('tran-type').value;
      const amount = document.getElementById('amount').value;
      const billingZip = document.getElementById('billing-zip').value;

      try {
        // Tokenize card data (happens in EPX iframe, returns JSON)
        // Your JavaScript NEVER sees raw card data - PCI compliant!
        const result = await epx.tokenize({
          tranType: tranType,        // CCE2, CCE1, or CCE8
          amount: amount,             // Transaction amount
          billingZip: billingZip,     // Billing ZIP for AVS check

          // Optional: Additional transaction data
          metadata: {
            customerId: 'customer_123',
            orderId: 'order_456'
          },

          // Your server callback URL (EPX sends result here)
          callbackUrl: 'https://yourserver.com/api/epx/callback',

          // Optional: Client-side success/error handlers
          onSuccess: (response) => {
            // response is JSON from EPX
            console.log('Tokenization successful:', response);
            // response.authGuid - contains BRIC token
            // response.tranNbr - EPX transaction number
            // response.status - 'approved' or 'declined'
          },

          onError: (error) => {
            console.error('Tokenization failed:', error);
            document.getElementById('error-message').textContent = error.message;
          }
        });

        // EPX returns JSON response
        console.log('EPX Response:', result);
        /*
        {
          "success": true,
          "authGuid": "0V703LH1HDL006J74W1",
          "tranNbr": "1234567890",
          "authResp": "00",
          "authCode": "123456",
          "cardType": "V",
          "lastFour": "1111",
          "expMonth": "12",
          "expYear": "2025",
          "message": "Approved"
        }
        */

        // Show success to user
        if (result.success) {
          alert('Payment successful!');

          // Optionally send to your server for additional processing
          await fetch('/api/payment/confirm', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              authGuid: result.authGuid,
              tranNbr: result.tranNbr,
              // Do NOT send raw card data - not in response anyway
            })
          });
        }

      } catch (error) {
        console.error('Payment error:', error);
        document.getElementById('error-message').textContent =
          'Payment failed. Please check your card details.';
      }
    });
  </script>
</body>
</html>
```

**JSON Dataflow (PCI Compliant)**:

```
┌──────────────────────────────────────────────────────────┐
│ Customer Browser - Your Website                          │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Your HTML/CSS                  EPX Secure Iframes       │
│  ┌─────────────────┐           ┌──────────────────┐     │
│  │ Labels, Buttons │           │ [Card Number]    │ ←EPX│
│  │ Billing ZIP     │           │ [Expiry] [CVV]   │ ←EPX│
│  │ Amount          │           └──────────────────┘     │
│  └─────────────────┘                    │               │
│         │                               │               │
│         │ Customer enters card data     │               │
│         │ Card data NEVER leaves iframe │               │
│         │ (PCI compliant boundary)      │               │
│         │                               │               │
│         ▼                               ▼               │
│  ┌──────────────────────────────────────────────┐       │
│  │ Your JavaScript                              │       │
│  │ - Calls epx.tokenize({ ... })               │       │
│  │ - Sends: tranType, amount, billingZip       │       │
│  │ - NEVER receives raw card data              │       │
│  └──────────────────┬───────────────────────────┘       │
└─────────────────────┼───────────────────────────────────┘
                      │
                      │ HTTPS POST (JSON)
                      │ { tranType: "CCE2", amount: "29.99", ... }
                      │ Card data encrypted in iframe → EPX
                      ▼
┌──────────────────────────────────────────────────────────┐
│ EPX Gateway (epx.gateway.com)                            │
├──────────────────────────────────────────────────────────┤
│ 1. Receive encrypted card data from iframe              │
│ 2. Validate card (Luhn check, expiry, CVV)              │
│ 3. Perform transaction:                                 │
│    - CCE2: Sale (charge immediately)                     │
│    - CCE1: Auth (hold funds)                             │
│    - CCE8: Storage BRIC (save card, $0 verify)           │
│ 4. Generate BRIC token                                  │
│ 5. Create transaction record                            │
└──────────┬───────────────────────────┬───────────────────┘
           │                           │
           │ Response (JSON)           │ Callback (JSON)
           ▼                           ▼
┌────────────────────────┐   ┌──────────────────────────────┐
│ Customer Browser       │   │ Your Server                  │
│ (Client-side Response) │   │ POST /api/epx/callback       │
├────────────────────────┤   ├──────────────────────────────┤
│ JSON Response:         │   │ JSON Payload:                │
│ {                      │   │ {                            │
│   success: true,       │   │   tranNbr: "1234567890",     │
│   authGuid: "0V703L..", │   │   authGuid: "0V703LH1...",  │
│   authResp: "00",      │   │   authResp: "00",            │
│   authCode: "123456",  │   │   authCode: "123456",        │
│   cardType: "V",       │   │   cardType: "V",             │
│   lastFour: "1111",    │   │   lastFour: "1111",          │
│   message: "Approved"  │   │   expMonth: "12",            │
│ }                      │   │   expYear: "2025",           │
│                        │   │   tranType: "CCE2",          │
│ Show success to user   │   │   amount: "29.99",           │
│                        │   │   epxMac: "<signature>"      │
│                        │   │ }                            │
└────────────────────────┘   │                              │
                             │ Process based on tranType    │
                             └──────────────────────────────┘
```

**Why This Is PCI Compliant**:

1. **Card Data Isolation**
   - Card input fields are EPX-hosted iframes
   - Your JavaScript cannot access card data
   - Card data encrypted in transit to EPX
   - You never store or process raw card numbers

2. **Token-Based**
   - EPX returns BRIC token (not card data)
   - Your server only stores tokens
   - Tokens are useless outside EPX ecosystem

3. **Scope Reduction**
   - Card data never enters your DOM
   - Card data never in your server logs
   - Reduces PCI compliance scope to SAQ A

**Pros**:
- Best UX (no redirect, seamless)
- Full styling control (CSS for containers)
- Modern JSON API (easier to work with)
- PCI compliant (SAQ A level)
- Real-time validation feedback
- Client and server callbacks

**Cons**:
- Requires JavaScript
- Requires EPX.js SDK
- More complex than HTML form

**Recommended Approach**: Use **EPX.js SDK with JSON** for production. It provides the best balance of UX, security, and developer experience while maintaining PCI compliance.

### 2. Server Post API (Backend Processing)

**Purpose**: Backend-to-backend payment processing

**Flow**: Your Server → EPX Gateway → EPX Response (synchronous)

**Use Cases**:
- ACH transactions (debit, credit, void)
- Card-on-file transactions (using Storage BRIC)
- Refunds, voids, captures
- BRIC Storage creation

**Integration**:
```http
POST https://epx.gateway.com/serverpost
Content-Type: application/x-www-form-urlencoded

MERCHANT_ID=your_merchant_id
&TRAN_TYPE=CCE2
&AUTH_GUID=0V703LH1HDL006J74W1
&TRAN_AMT=29.99
&EPX_MAC=<hmac_signature>
```

### 3. Server Post Callbacks (EPX → Your Server)

**Purpose**: EPX sends transaction results to your server

**Flow**: EPX Gateway → Your Callback URL → Your Server Response

**Use Cases**:
- Browser Post completion (after customer submits card)
- Async transaction notifications
- ACH return notifications (days later)
- Settlement confirmations

**Integration**:
```http
POST https://yourserver.com/epx/callback
Content-Type: application/json

{
  "tranNbr": "1234567890",
  "authGuid": "0V703LH1HDL006J74W1",
  "authResp": "00",
  "authCode": "123456",
  "cardType": "V",
  "lastFour": "1111",
  "expMonth": "12",
  "expYear": "2025",
  "tranType": "CCE2",
  "amount": "29.99",
  "epxMac": "<hmac_signature>"
}
```

## EPX Callback Handler Implementation

This callback handler processes three transaction types from EPX Browser Post:
- **CCE8**: Storage BRIC creation (save card, no charge)
- **CCE2**: Sale (immediate payment)
- **CCE1**: Authorization (hold funds)

### Complete Callback Handler Dataflow

```
┌─────────────────────────────────────────────────────────┐
│ EPX Gateway                                             │
│ (After customer submits card via Browser Post)          │
└──────────┬──────────────────────────────────────────────┘
           │
           │ POST /api/epx/callback (JSON)
           │ {
           │   "tranNbr": "1234567890",
           │   "authGuid": "0V703LH1HDL006J74W1",
           │   "authResp": "00",
           │   "tranType": "CCE8",  ← Storage, Sale, or Auth
           │   "epxMac": "<signature>"
           │ }
           ▼
┌─────────────────────────────────────────────────────────┐
│ Your Server: /api/epx/callback                          │
├─────────────────────────────────────────────────────────┤
│ Step 1: Verify EPX Signature (Security)                │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ const receivedMac = request.body.epxMac;            │ │
│ │ const computedMac = hmacSHA256(                     │ │
│ │   request.body,                                     │ │
│ │   EPX_MAC_SECRET                                    │ │
│ │ );                                                  │ │
│ │                                                     │ │
│ │ if (receivedMac !== computedMac) {                 │ │
│ │   return 401 Unauthorized;                         │ │
│ │ }                                                   │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ Step 2: Check Transaction Status                       │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ if (authResp !== "00") {                           │ │
│ │   // Declined transaction                          │ │
│ │   logDeclinedTransaction();                        │ │
│ │   return 200 OK; // Acknowledge receipt            │ │
│ │ }                                                   │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ Step 3: Route by Transaction Type                      │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ switch (tranType) {                                │ │
│ │   case "CCE8": handleStorageBRIC();    break;     │ │
│ │   case "CCE2": handleSale();           break;     │ │
│ │   case "CCE1": handleAuth();           break;     │ │
│ │   default:     return 400 Bad Request;            │ │
│ │ }                                                   │ │
│ └─────────────────────────────────────────────────────┘ │
└──────────┬──────────────────────────────────────────────┘
           │
           ├─────── CCE8: Storage BRIC ──────────────────┐
           │                                             │
           ▼                                             │
┌─────────────────────────────────────────────────────────┤
│ handleStorageBRIC()                                     │
├─────────────────────────────────────────────────────────┤
│ 1. Extract BRIC and card metadata                      │
│    - authGuid: "0V703L..." (Storage BRIC)              │
│    - cardType: "V" → "visa"                            │
│    - lastFour: "1111"                                  │
│    - expMonth: 12, expYear: 2025                       │
│                                                         │
│ 2. Lookup customer from session/metadata               │
│    - customerId from JWT token or session              │
│    - merchantId from authenticated context             │
│                                                         │
│ 3. Call PaymentMethodService.SavePaymentMethod()       │
│    ┌───────────────────────────────────────────────┐   │
│    │ SavePaymentMethodRequest {                    │   │
│    │   merchant_id: merchantId                     │   │
│    │   customer_id: customerId                     │   │
│    │   payment_token: authGuid ← Storage BRIC      │   │
│    │   payment_type: CREDIT_CARD                   │   │
│    │   last_four: "1111"                           │   │
│    │   card_brand: "visa"                          │   │
│    │   card_exp_month: 12                          │   │
│    │   card_exp_year: 2025                         │   │
│    │   is_default: true/false                      │   │
│    │   idempotency_key: tranNbr                    │   │
│    │ }                                             │   │
│    └────────────┬──────────────────────────────────┘   │
│                 ▼                                       │
│    ┌───────────────────────────────────────────────┐   │
│    │ Database: customer_payment_methods            │   │
│    │ INSERT INTO customer_payment_methods:         │   │
│    │ - bric: "0V703L..." (Storage BRIC)            │   │
│    │ - payment_type: "credit_card"                 │   │
│    │ - last_four: "1111"                           │   │
│    │ - is_verified: true                           │   │
│    │ ON CONFLICT DO NOTHING (idempotency)          │   │
│    └───────────────────────────────────────────────┘   │
│                                                         │
│ 4. Return 200 OK to EPX                                │
│    - EPX stops retrying callback                       │
│                                                         │
│ 5. Notify customer (optional)                          │
│    - Email: "Card ending in 1111 saved"                │
│    - Push notification                                 │
└─────────────────────────────────────────────────────────┘

           ├─────── CCE2: Sale ──────────────────────────┐
           │                                             │
           ▼                                             │
┌─────────────────────────────────────────────────────────┤
│ handleSale()                                            │
├─────────────────────────────────────────────────────────┤
│ 1. Extract transaction data                            │
│    - authGuid: BRIC token (Financial BRIC)             │
│    - amount: "29.99"                                   │
│    - authCode: "123456" (bank authorization)           │
│                                                         │
│ 2. Lookup order/customer from tranNbr or metadata      │
│    - Query pending order by tranNbr                    │
│    - Get customerId, merchantId                        │
│                                                         │
│ 3. Create transaction record                           │
│    ┌───────────────────────────────────────────────┐   │
│    │ Database: transactions                        │   │
│    │ INSERT INTO transactions:                     │   │
│    │ - id: UUID (from tranNbr)                     │   │
│    │ - merchant_id: merchantId                     │   │
│    │ - customer_id: customerId                     │   │
│    │ - type: "CHARGE"                              │   │
│    │ - amount_cents: 2999                          │   │
│    │ - payment_method_type: "credit_card"          │   │
│    │ - tran_nbr: tranNbr                           │   │
│    │ - auth_guid: authGuid (Financial BRIC)        │   │
│    │ - auth_resp: "00" (approved)                  │   │
│    │ - auth_code: "123456"                         │   │
│    │ - auth_card_type: "V"                         │   │
│    │ - processed_at: NOW()                         │   │
│    │ ON CONFLICT DO NOTHING (idempotency)          │   │
│    └───────────────────────────────────────────────┘   │
│                                                         │
│ 4. Update order status                                 │
│    UPDATE orders SET status = 'paid'                   │
│    WHERE tran_nbr = tranNbr                            │
│                                                         │
│ 5. Optional: Save card for future (if customer opted) │
│    if (saveCard) {                                     │
│      // Convert Financial BRIC → Storage BRIC         │
│      PaymentMethodService.ConvertFinancialBRICToStorage│
│    }                                                    │
│                                                         │
│ 6. Return 200 OK to EPX                                │
│                                                         │
│ 7. Notify customer                                     │
│    - Email receipt                                     │
│    - Update UI (via WebSocket if customer online)      │
└─────────────────────────────────────────────────────────┘

           ├─────── CCE1: Authorization ─────────────────┐
           │                                             │
           ▼                                             │
┌─────────────────────────────────────────────────────────┤
│ handleAuth()                                            │
├─────────────────────────────────────────────────────────┤
│ 1. Extract authorization data                          │
│    - authGuid: BRIC token (Financial BRIC)             │
│    - amount: "100.00" (hold amount)                    │
│    - authCode: "789012"                                │
│                                                         │
│ 2. Create authorization transaction                    │
│    ┌───────────────────────────────────────────────┐   │
│    │ Database: transactions                        │   │
│    │ INSERT INTO transactions:                     │   │
│    │ - id: UUID                                    │   │
│    │ - type: "AUTH"                                │   │
│    │ - amount_cents: 10000                         │   │
│    │ - auth_guid: authGuid (Financial BRIC)        │   │
│    │ - auth_resp: "00"                             │   │
│    │ - auth_code: "789012"                         │   │
│    │ - parent_transaction_id: NULL (standalone)    │   │
│    │ ON CONFLICT DO NOTHING                        │   │
│    └───────────────────────────────────────────────┘   │
│                                                         │
│ 3. Update reservation/booking                          │
│    - Mark reservation as "authorized"                  │
│    - Store transaction_id for future capture          │
│    - Set expiry (7-10 days for capture)                │
│                                                         │
│ 4. Return 200 OK to EPX                                │
│                                                         │
│ 5. Notify customer                                     │
│    - "Authorization successful"                        │
│    - "Hold placed on card ending in 1111"              │
│                                                         │
│ 6. Schedule auto-capture or void                       │
│    - Set timer for capture (hotel check-out)           │
│    - Or void if not needed (rental canceled)           │
└─────────────────────────────────────────────────────────┘
```

### Implementation Code

```go
// POST /api/epx/callback
func HandleEPXCallback(w http.ResponseWriter, r *http.Request) {
    // Step 1: Parse JSON body
    var callback EPXCallbackPayload
    if err := json.NewDecoder(r.Body).Decode(&callback); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Step 2: Verify EPX signature (CRITICAL for security)
    if !verifyEPXSignature(callback, r.Header.Get("X-EPX-Signature")) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // Step 3: Check transaction status
    if callback.AuthResp != "00" {
        // Transaction declined - log and acknowledge
        log.Printf("Transaction declined: %s - %s", callback.TranNbr, callback.Message)
        w.WriteHeader(http.StatusOK)
        return
    }

    // Step 4: Route by transaction type
    switch callback.TranType {
    case "CCE8": // Storage BRIC creation
        handleStorageBRIC(callback)

    case "CCE2": // Sale
        handleSale(callback)

    case "CCE1": // Authorization
        handleAuth(callback)

    default:
        http.Error(w, "Unsupported transaction type", http.StatusBadRequest)
        return
    }

    // Step 5: Acknowledge receipt (important!)
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "received",
        "tranNbr": callback.TranNbr,
    })
}

// Handle Storage BRIC (Save Card)
func handleStorageBRIC(callback EPXCallbackPayload) error {
    // Extract customer context (from session, JWT, or metadata)
    customerId := getCustomerIdFromContext(callback.Metadata)
    merchantId := getMerchantIdFromContext()

    // Call PaymentMethodService
    _, err := paymentMethodClient.SavePaymentMethod(context.Background(), &paymentmethodv1.SavePaymentMethodRequest{
        MerchantId:    merchantId,
        CustomerId:    customerId,
        PaymentToken:  callback.AuthGuid, // Storage BRIC
        PaymentType:   paymentmethodv1.PaymentMethodType_PAYMENT_METHOD_TYPE_CREDIT_CARD,
        LastFour:      callback.LastFour,
        CardBrand:     mapCardType(callback.CardType), // "V" → "visa"
        CardExpMonth:  int32(callback.ExpMonth),
        CardExpYear:   int32(callback.ExpYear),
        IsDefault:     callback.Metadata["setAsDefault"] == "true",
        IdempotencyKey: callback.TranNbr, // Use tranNbr for idempotency
    })

    if err != nil {
        log.Printf("Failed to save payment method: %v", err)
        return err
    }

    // Notify customer (async)
    go notifyCustomer(customerId, "Card saved successfully")

    return nil
}

// Handle Sale (Immediate Payment)
func handleSale(callback EPXCallbackPayload) error {
    // Lookup order/booking from tranNbr or metadata
    order := getOrderByTranNbr(callback.TranNbr)
    if order == nil {
        return fmt.Errorf("order not found for tranNbr: %s", callback.TranNbr)
    }

    // Create transaction record
    amountCents := parseDecimalToCents(callback.Amount)
    tx, err := dbClient.CreateTransaction(context.Background(), &sqlc.CreateTransactionParams{
        ID:                uuid.MustParse(callback.TranNbr), // Or generate new UUID
        MerchantID:        order.MerchantID,
        CustomerID:        pgtype.UUID{Bytes: order.CustomerID, Valid: true},
        AmountCents:       amountCents,
        Currency:          "USD",
        Type:              "CHARGE",
        PaymentMethodType: "credit_card",
        TranNbr:           pgtype.Text{String: callback.TranNbr, Valid: true},
        AuthGuid:          pgtype.Text{String: callback.AuthGuid, Valid: true},
        AuthResp:          pgtype.Text{String: callback.AuthResp, Valid: true},
        AuthCode:          pgtype.Text{String: callback.AuthCode, Valid: true},
        AuthCardType:      pgtype.Text{String: callback.CardType, Valid: true},
        ProcessedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
        // status is GENERATED column (auto "approved")
    })

    if err != nil {
        log.Printf("Failed to create transaction: %v", err)
        return err
    }

    // Update order status
    updateOrderStatus(order.ID, "paid", tx.ID)

    // Optional: Save card for future if customer opted in
    if callback.Metadata["saveCard"] == "true" {
        // Convert Financial BRIC → Storage BRIC
        convertToStorageBRIC(callback, order)
    }

    // Send receipt email
    go sendReceiptEmail(order.CustomerID, tx)

    return nil
}

// Handle Authorization (Hold Funds)
func handleAuth(callback EPXCallbackPayload) error {
    // Similar to handleSale but type = "AUTH"
    // Store for future capture
    // Set expiry timer (7-10 days)

    reservation := getReservationByTranNbr(callback.TranNbr)

    amountCents := parseDecimalToCents(callback.Amount)
    tx, err := dbClient.CreateTransaction(context.Background(), &sqlc.CreateTransactionParams{
        ID:                uuid.New(),
        MerchantID:        reservation.MerchantID,
        CustomerID:        pgtype.UUID{Bytes: reservation.CustomerID, Valid: true},
        AmountCents:       amountCents,
        Type:              "AUTH",
        PaymentMethodType: "credit_card",
        TranNbr:           pgtype.Text{String: callback.TranNbr, Valid: true},
        AuthGuid:          pgtype.Text{String: callback.AuthGuid, Valid: true},
        AuthResp:          pgtype.Text{String: callback.AuthResp, Valid: true},
        AuthCode:          pgtype.Text{String: callback.AuthCode, Valid: true},
        ProcessedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
    })

    if err != nil {
        return err
    }

    // Update reservation
    updateReservationStatus(reservation.ID, "authorized", tx.ID)

    // Schedule auto-capture or void
    scheduleAuthExpiry(tx.ID, time.Now().Add(7*24*time.Hour))

    return nil
}

// Helper: Verify EPX HMAC signature
func verifyEPXSignature(payload EPXCallbackPayload, signature string) bool {
    // Reconstruct signature from payload
    data := fmt.Sprintf("%s|%s|%s|%s",
        payload.TranNbr,
        payload.AuthGuid,
        payload.AuthResp,
        payload.Amount,
    )

    mac := hmac.New(sha256.New, []byte(os.Getenv("EPX_MAC_SECRET")))
    mac.Write([]byte(data))
    expectedMAC := hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(expectedMAC), []byte(signature))
}

// Types
type EPXCallbackPayload struct {
    TranNbr   string            `json:"tranNbr"`
    AuthGuid  string            `json:"authGuid"`
    AuthResp  string            `json:"authResp"`
    AuthCode  string            `json:"authCode"`
    TranType  string            `json:"tranType"` // CCE8, CCE2, CCE1
    Amount    string            `json:"amount"`
    CardType  string            `json:"cardType"`
    LastFour  string            `json:"lastFour"`
    ExpMonth  int               `json:"expMonth"`
    ExpYear   int               `json:"expYear"`
    Message   string            `json:"message"`
    Metadata  map[string]string `json:"metadata"`
}
```

### Key Implementation Points

1. **Security First**
   - Always verify EPX signature before processing
   - Use HMAC-SHA256 with secret key
   - Reject callbacks with invalid signatures

2. **Idempotency**
   - EPX may retry callbacks
   - Use `tranNbr` as idempotency key
   - Database `ON CONFLICT DO NOTHING`

3. **Acknowledge Receipt**
   - Return 200 OK immediately
   - EPX stops retrying on 200
   - Non-200 triggers retries (up to 24 hours)

4. **Error Handling**
   - Log declined transactions (authResp != "00")
   - Don't return errors for declined (acknowledge receipt)
   - Only return errors for system failures

5. **Async Processing**
   - Don't block callback response
   - Send emails/notifications asynchronously
   - Update orders in background if slow

---

## Service Architecture

The payment service is organized into two main areas:

1. **PaymentService** - Transaction operations (charges, refunds, voids)
2. **PaymentMethodService** - Payment method storage and management

---

## PaymentService RPCs

### 1. Authorize

**Purpose**: Hold funds on a payment method without capturing them immediately. Used for pre-authorization scenarios (hotel reservations, rental cars, etc.).

**Input**: `AuthorizeRequest`
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier (nullable for guest)
- `amount` - Decimal string (e.g., "29.99")
- `currency` - ISO 4217 code (e.g., "USD")
- `payment_method` - Either `payment_method_id` (Storage BRIC) OR `payment_token` (Financial BRIC)
- `idempotency_key` - Prevents duplicate authorizations
- `metadata` - Optional key-value pairs

**Output**: `PaymentResponse`
- `transaction_id` - UUID of created transaction
- `group_id` - Links related transactions (for future capture/void)
- `amount`, `currency`, `status`, `type`
- `is_approved` - Boolean approval status
- `authorization_code` - Bank auth code
- `message` - Human-readable response
- `card` - Card display info (brand, last 4)
- `created_at` - Timestamp

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ AuthorizeRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.Authorize()                              │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request (amount > 0, payment method exists) │
│ 2. Generate transaction UUID from idempotency_key       │
│ 3. Determine BRIC source:                               │
│    - payment_method_id → Query DB for Storage BRIC      │
│    - payment_token → Use Financial BRIC directly        │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST to server post endpoint with:                      │
│ - TRAN_TYPE: "CCE1" (Credit Card Auth)                  │
│ - AUTH_GUID: Storage BRIC or Financial BRIC             │
│ - TRAN_AMT: Amount to authorize                         │
│ - TRAN_NBR: Deterministic 10-digit from UUID            │
│ - EPX_MAC: HMAC signature                               │
└──────┬──────────────────────────────────────────────────┘
       │ EPX Response (synchronous)
       ▼
┌─────────────────────────────────────────────────────────┐
│ Process EPX Response                                    │
├─────────────────────────────────────────────────────────┤
│ 1. Parse EPX response fields:                           │
│    - auth_resp ("00" = approved, else declined)         │
│    - auth_code (bank authorization code)                │
│    - auth_guid (new BRIC for this transaction)          │
│    - auth_card_type (V/M/A/D)                           │
│ 2. Map to internal status:                              │
│    - auth_resp = "00" → status = "approved"             │
│    - auth_resp ≠ "00" → status = "declined"             │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create Transaction Record                     │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO transactions:                               │
│ - id: UUID (from idempotency_key)                       │
│ - merchant_id, customer_id                              │
│ - amount_cents: Parse decimal to cents                  │
│ - currency: "USD"                                       │
│ - type: "AUTH"                                          │
│ - payment_method_type: "credit_card"                    │
│ - payment_method_id: If using Storage BRIC              │
│ - tran_nbr: EPX TRAN_NBR                                │
│ - auth_guid: EPX BRIC from response                     │
│ - auth_resp: EPX response code                          │
│ - auth_code: Bank authorization code                    │
│ - auth_card_type: Card brand code                       │
│ - parent_transaction_id: NULL (standalone)              │
│ - processed_at: CURRENT_TIMESTAMP                       │
│ - status: GENERATED ("approved" or "declined")          │
│                                                         │
│ ON CONFLICT (id) DO NOTHING (idempotency)               │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Update Payment Method (if applicable)                   │
├─────────────────────────────────────────────────────────┤
│ If payment_method_id provided:                          │
│ - UPDATE customer_payment_methods                       │
│   SET last_used_at = CURRENT_TIMESTAMP                  │
│   WHERE id = payment_method_id                          │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Return PaymentResponse                                  │
├─────────────────────────────────────────────────────────┤
│ - transaction_id: Created transaction UUID              │
│ - group_id: Same as transaction_id (parent)             │
│ - status: "approved" or "declined"                      │
│ - is_approved: true/false                               │
│ - authorization_code: Bank auth code                    │
│ - message: Mapped from EPX auth_resp                    │
│ - card: { brand, last_four } from metadata              │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Authorization holds funds for 7-10 days (card network dependent)
- Must be captured within hold period or funds auto-release
- Can be voided before capture to release immediately
- Each authorization gets unique auth_guid (BRIC) for future operations
- Idempotency prevents duplicate auths if retry occurs

**Error Scenarios**:
- Invalid payment method → Return error before EPX call
- Insufficient funds → EPX returns auth_resp ≠ "00", status = "declined"
- Duplicate idempotency_key → Return existing transaction (ON CONFLICT)
- Network error → Transaction status = "failed", no auth_resp

---

### 2. Capture

**Purpose**: Capture previously authorized funds. Completes a two-step payment flow.

**Input**: `CaptureRequest`
- `transaction_id` - UUID of authorization transaction
- `amount` - Optional: partial capture amount (default: full auth amount)
- `idempotency_key` - Prevents duplicate captures

**Output**: `PaymentResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ CaptureRequest(transaction_id)
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.Capture()                                │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request                                     │
│ 2. Generate new transaction UUID from idempotency_key   │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Authorization Transaction               │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM transactions WHERE id = transaction_id    │
│                                                         │
│ Validations:                                            │
│ - Transaction exists                                    │
│ - type = "AUTH"                                         │
│ - status = "approved"                                   │
│ - Not already captured (no child CAPTURE transactions)  │
│ - Merchant owns transaction                             │
└──────┬──────────────────────────────────────────────────┘
       │ auth_guid from AUTH transaction
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST to server post endpoint with:                      │
│ - TRAN_TYPE: "CCE4" (Credit Card Capture)               │
│ - AUTH_GUID: BRIC from AUTH transaction                 │
│ - TRAN_AMT: Amount to capture (≤ auth amount)           │
│ - TRAN_NBR: New deterministic 10-digit from new UUID    │
│ - ORIG_TRAN_NBR: Original AUTH TRAN_NBR                 │
│ - EPX_MAC: HMAC signature                               │
└──────┬──────────────────────────────────────────────────┘
       │ EPX Response
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create CAPTURE Transaction                    │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO transactions:                               │
│ - id: New UUID (from capture idempotency_key)           │
│ - parent_transaction_id: Original AUTH transaction ID   │
│ - type: "CAPTURE"                                       │
│ - amount_cents: Capture amount                          │
│ - auth_guid: New BRIC from EPX response                 │
│ - auth_resp, auth_code: From EPX                        │
│ - All other fields from AUTH transaction                │
│                                                         │
│ Transaction Chain: AUTH → CAPTURE                       │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Can only capture approved AUTH transactions
- Capture amount ≤ authorization amount
- Partial captures allowed (hotel incidentals, gas pumps)
- Creates new transaction linked via parent_transaction_id
- Each capture gets new auth_guid for potential refund
- AUTH transaction remains immutable

**Error Scenarios**:
- AUTH not found → Error
- AUTH not approved → Error
- Already captured → Error (check for existing CAPTURE child)
- Capture amount > auth amount → Error
- Network error → status = "failed"

---

### 3. Sale

**Purpose**: Combined authorization + capture in single operation. Used for immediate payments (e-commerce checkout, retail POS).

**Input**: `SaleRequest`
- Same fields as `AuthorizeRequest`

**Output**: `PaymentResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ SaleRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.Sale()                                   │
├─────────────────────────────────────────────────────────┤
│ Similar to Authorize flow                               │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST with:                                              │
│ - TRAN_TYPE: "CCE2" (Credit Card Sale)                  │
│ - AUTH_GUID: Storage BRIC or Financial BRIC             │
│ - TRAN_AMT: Full amount                                 │
│ - Funds captured immediately                            │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create CHARGE Transaction                     │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO transactions:                               │
│ - type: "CHARGE" (not "SALE" - using proto enum name)   │
│ - parent_transaction_id: NULL (standalone)              │
│ - All fields similar to AUTH                            │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Single-step payment (no separate capture needed)
- Funds settled immediately (within 1-3 business days)
- Can be refunded later using group_id
- Cannot be captured (already captured)
- Can be voided same-day before settlement

**Error Scenarios**:
- Same as Authorize

---

### 4. Void

**Purpose**: Cancel a transaction before settlement (same-day cancellation). Prevents funds from being transferred.

**Input**: `VoidRequest`
- `group_id` - Transaction group to void (parent transaction ID)
- `idempotency_key` - Prevents duplicate voids

**Output**: `PaymentResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ VoidRequest(group_id)
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.Void()                                   │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request                                     │
│ 2. Generate void transaction UUID                       │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Original Transaction                    │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM transactions WHERE id = group_id          │
│                                                         │
│ Validations:                                            │
│ - Transaction exists                                    │
│ - type IN ("AUTH", "CHARGE", "CAPTURE")                 │
│ - status = "approved"                                   │
│ - Same-day (created_at is today)                        │
│ - Not already voided (no VOID child)                    │
│ - Merchant owns transaction                             │
└──────┬──────────────────────────────────────────────────┘
       │ auth_guid from original transaction
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST with:                                              │
│ - TRAN_TYPE: "CCEX" (Credit Card Void)                  │
│ - AUTH_GUID: BRIC from original transaction             │
│ - TRAN_NBR: New deterministic 10-digit                  │
│ - ORIG_TRAN_NBR: Original transaction TRAN_NBR          │
│ - EPX_MAC: HMAC signature                               │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create VOID Transaction                       │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO transactions:                               │
│ - id: New UUID                                          │
│ - parent_transaction_id: Original transaction ID        │
│ - type: "VOID"                                          │
│ - amount_cents: $0 (voids don't transfer funds)         │
│ - auth_guid: From EPX response                          │
│                                                         │
│ Transaction Chain: AUTH → VOID or CHARGE → VOID         │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Only works same-day before settlement (typically before 11:59 PM merchant time)
- Immediately releases held funds (AUTH) or cancels charge (CHARGE)
- After settlement cutoff, must use Refund instead
- Voids don't transfer funds (amount = $0)
- Original transaction remains immutable

**Error Scenarios**:
- Transaction not found → Error
- Transaction not approved → Error
- Already voided → Error
- Not same-day → Error, must use Refund
- After settlement → EPX may reject, use Refund

---

### 5. Refund

**Purpose**: Return funds to customer after settlement. Used for returns, disputes, goodwill refunds.

**Input**: `RefundRequest`
- `group_id` - Transaction group to refund
- `amount` - Optional: partial refund amount
- `reason` - Reason for refund ("customer_request", "duplicate", etc.)
- `idempotency_key` - Prevents duplicate refunds

**Output**: `PaymentResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ RefundRequest(group_id)
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.Refund()                                 │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request                                     │
│ 2. Generate refund transaction UUID                     │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Original Transaction Chain              │
├─────────────────────────────────────────────────────────┤
│ WITH RECURSIVE: Get full transaction chain              │
│ - Find root transaction (CHARGE or CAPTURE)             │
│ - Get all REFUND children                               │
│                                                         │
│ Validations:                                            │
│ - Transaction exists and approved                       │
│ - Has settled (not same-day, or use Void)               │
│ - Calculate total refunded amount                       │
│ - Refund amount ≤ (original - total_refunded)           │
│ - Merchant owns transaction                             │
└──────┬──────────────────────────────────────────────────┘
       │ auth_guid from CHARGE/CAPTURE
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST with:                                              │
│ - TRAN_TYPE: "CCE3" (Credit Card Refund)                │
│ - AUTH_GUID: BRIC from CHARGE/CAPTURE                   │
│ - TRAN_AMT: Refund amount                               │
│ - TRAN_NBR: New deterministic 10-digit                  │
│ - ORIG_TRAN_NBR: Original CHARGE/CAPTURE TRAN_NBR       │
│ - EPX_MAC: HMAC signature                               │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create REFUND Transaction                     │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO transactions:                               │
│ - id: New UUID                                          │
│ - parent_transaction_id: CHARGE/CAPTURE transaction ID  │
│ - type: "REFUND"                                        │
│ - amount_cents: Refund amount                           │
│ - auth_guid: New BRIC from EPX                          │
│ - metadata: { reason: "customer_request" }              │
│                                                         │
│ Transaction Chain: CHARGE → REFUND(s)                   │
│ Multiple refunds allowed (partial refunds)              │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Only works after settlement (typically next day+)
- Partial refunds allowed (multiple REFUND children)
- Total refunds cannot exceed original charge amount
- Funds returned to customer's original payment method
- Takes 5-10 business days to appear on customer's statement
- Each refund gets new auth_guid
- Original transaction remains immutable

**Error Scenarios**:
- Transaction not found → Error
- Transaction not settled → Error, use Void
- Refund amount > remaining balance → Error
- Payment method no longer valid → EPX may decline
- Network error → status = "failed"

---

### 6. ACHDebit

**Purpose**: Pull funds from customer's bank account. Used for recurring payments, invoices, subscriptions.

**IMPORTANT**: Requires verified payment method. All ACH operations require pre-note verification (1-3 business days) before first use.

**Input**: `ACHDebitRequest`
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier (optional)
- `payment_method_id` - **Required** (ACH Storage BRIC, must be verified)
- `amount` - Decimal string
- `currency` - ISO 4217 code
- `idempotency_key` - Prevents duplicate debits
- `metadata` - Optional (invoice_id, etc.)

**Output**: `PaymentResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ ACHDebitRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.ACHDebit()                               │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request                                     │
│ 2. Generate transaction UUID from idempotency_key       │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Payment Method                          │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM customer_payment_methods                  │
│ WHERE id = payment_method_id                            │
│                                                         │
│ CRITICAL Validations:                                   │
│ - Payment method exists                                 │
│ - payment_type = "ach"                                  │
│ - is_verified = true ⚠️ REQUIRED                        │
│   → If false: Return error "Verification pending"      │
│   → Customer must wait 1-3 days after StoreACHAccount  │
│ - is_active = true                                      │
│ - Merchant owns payment method                          │
│ - Customer ID matches (if provided)                     │
└──────┬──────────────────────────────────────────────────┘
       │ bric (ACH Storage BRIC)
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST with:                                              │
│ - TRAN_TYPE: "CKC2" (ACH Checking Debit)                │
│   or "CKS2" (ACH Savings Debit)                         │
│ - AUTH_GUID: ACH Storage BRIC                           │
│ - TRAN_AMT: Debit amount                                │
│ - TRAN_NBR: Deterministic 10-digit from UUID            │
│ - STD_ENTRY_CLASS: "PPD", "CCD", "WEB", or "TEL"        │
│ - RECV_NAME: Account holder name                        │
│ - EPX_MAC: HMAC signature                               │
│                                                         │
│ Note: EPX submits to ACH network (not real-time)        │
└──────┬──────────────────────────────────────────────────┘
       │ EPX Response (accepted for processing)
       ▼
┌─────────────────────────────────────────────────────────┐
│ Process EPX Response                                    │
├─────────────────────────────────────────────────────────┤
│ ACH is different from credit cards:                     │
│ - auth_resp = "00" means "accepted for processing"      │
│ - NOT "funds captured" (takes 3-5 business days)        │
│ - Returns can occur up to 60 days later                 │
│ - status = "approved" means EPX accepted, not settled   │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create DEBIT Transaction                      │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO transactions:                               │
│ - id: UUID                                              │
│ - type: "DEBIT"                                         │
│ - payment_method_type: "ach"                            │
│ - payment_method_id: ACH payment method UUID            │
│ - amount_cents: Debit amount                            │
│ - auth_guid: ACH Storage BRIC (same as input)           │
│ - auth_resp: "00" (accepted)                            │
│ - parent_transaction_id: NULL (standalone)              │
│ - processed_at: CURRENT_TIMESTAMP                       │
│ - metadata: { std_entry_class, receiver_name }          │
│                                                         │
│ Note: Transaction record created immediately            │
│ Actual settlement happens 3-5 days later                │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Update Payment Method                                   │
├─────────────────────────────────────────────────────────┤
│ UPDATE customer_payment_methods                         │
│ SET last_used_at = CURRENT_TIMESTAMP                    │
│ WHERE id = payment_method_id                            │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Settlement Timeline**:
```
Day 0 (Today):     ACHDebit RPC call → EPX accepts
Day 1-2:           EPX submits to ACH network
Day 3-5:           Funds debited from customer account
Day 4-6:           Funds settle to merchant account
Up to Day 60:      Customer can dispute (return/reversal)
```

**Key Business Rules**:
- **Must use verified Storage BRIC** (payment_method_id required)
- **Verification requirement**: is_verified must be true
  - If false: Reject with error "ACH account verification pending. Please wait 1-3 business days."
  - Verification happens via pre-note after StoreACHAccount
- Returns can happen up to 60 days after debit
- Common return codes: R01 (insufficient funds), R03 (no account), R10 (customer advises not authorized)
- Must monitor EPX Server Post callbacks for return notifications
- NACHA rules require proper authorization from customer
- Recommended: Set per-transaction limits and daily limits

**Error Scenarios**:
- Payment method not verified → Error "Verification pending (1-3 business days)"
  - Guide customer to verified payment method or alternative
  - Show verification status in UI
- Payment method inactive → Error
- Payment method doesn't exist → Error
- Insufficient customer authorization → Compliance violation
- Return (R01, R03, R10) → Receive Server Post callback days later
- Unauthorized returns → May lose merchant account

---

### 7. ACHCredit

**Purpose**: Send funds to customer's bank account. Used for refunds, payouts, disbursements.

**IMPORTANT**: Requires verified payment method. All ACH operations require pre-note verification (1-3 business days) before first use.

**Input**: `ACHCreditRequest`
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier (optional)
- `payment_method_id` - **Required** (ACH Storage BRIC, must be verified)
- `amount` - Decimal string
- `currency` - ISO 4217 code
- `reason` - Purpose of credit ("refund", "payout", "disbursement")
- `idempotency_key` - Prevents duplicate credits
- `metadata` - Optional

**Output**: `PaymentResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ ACHCreditRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.ACHCredit()                              │
├─────────────────────────────────────────────────────────┤
│ Similar to ACHDebit validation flow                     │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST with:                                              │
│ - TRAN_TYPE: "CKC3" (ACH Checking Credit)               │
│   or "CKS3" (ACH Savings Credit)                        │
│ - AUTH_GUID: ACH Storage BRIC                           │
│ - TRAN_AMT: Credit amount                               │
│ - STD_ENTRY_CLASS: Usually "PPD" or "CCD"               │
│ - RECV_NAME: Account holder name                        │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create Transaction                            │
├─────────────────────────────────────────────────────────┤
│ - type: Could be "REFUND" (if refunding ACH debit)      │
│   or custom "ACH_CREDIT" (if new payout)                │
│ - payment_method_type: "ach"                            │
│ - metadata: { reason: "refund" }                        │
│                                                         │
│ Note: If refund, set parent_transaction_id              │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Settlement timeline: 3-5 business days (same as debit)
- Can be returned (R01, R03, etc.) up to 60 days
- Common use cases:
  - Refund ACH debit (overdraft reversal)
  - Payout to gig worker
  - Merchant disbursement
- Must comply with NACHA rules for credits

**Error Scenarios**:
- Same as ACHDebit
- Return codes can still occur (account closed, incorrect account)

---

### 8. ACHVoid

**Purpose**: Cancel ACH transaction same-day before submission to ACH network.

**Input**: `ACHVoidRequest`
- `transaction_id` - Original ACH transaction to void
- `idempotency_key` - Prevents duplicate voids

**Output**: `PaymentResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ ACHVoidRequest(transaction_id)
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.ACHVoid()                                │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request                                     │
│ 2. Generate void transaction UUID                       │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Original ACH Transaction                │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM transactions WHERE id = transaction_id    │
│                                                         │
│ Validations:                                            │
│ - Transaction exists                                    │
│ - type = "DEBIT" and payment_method_type = "ach"        │
│ - status = "approved"                                   │
│ - Same-day (before ACH submission cutoff ~5 PM ET)      │
│ - Not already voided                                    │
│ - Merchant owns transaction                             │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API                                     │
├─────────────────────────────────────────────────────────┤
│ POST with:                                              │
│ - TRAN_TYPE: "CKCX" or "CKSX" (ACH Void)                │
│ - AUTH_GUID: ACH Storage BRIC                           │
│ - TRAN_NBR: New deterministic ID                        │
│ - ORIG_TRAN_NBR: Original ACH TRAN_NBR                  │
│                                                         │
│ EPX cancels before submitting to ACH network            │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create VOID Transaction                       │
├─────────────────────────────────────────────────────────┤
│ - type: "VOID"                                          │
│ - parent_transaction_id: Original DEBIT transaction ID  │
│ - payment_method_type: "ach"                            │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Only works same-day before ACH submission (typically before 5 PM ET)
- After submission, cannot void (funds in ACH network)
- If already submitted, must wait for debit to settle then issue credit
- Prevents unnecessary returns and return fees

**Error Scenarios**:
- Not same-day → Error
- Already submitted to ACH → EPX may reject
- After cutoff → Use ACHCredit for reversal

---

### 9. GetTransaction

**Purpose**: Retrieve detailed information about a specific transaction.

**Input**: `GetTransactionRequest`
- `transaction_id` - UUID of transaction

**Output**: `Transaction`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ GetTransactionRequest(transaction_id)
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.GetTransaction()                         │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Transaction                             │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM transactions WHERE id = transaction_id    │
│                                                         │
│ Returns full transaction record with all fields         │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Map to Transaction Proto                                │
├─────────────────────────────────────────────────────────┤
│ - Convert amount_cents to decimal string                │
│ - Map status enum                                       │
│ - Map type enum                                         │
│ - Extract card info from metadata                       │
│ - Format timestamps                                     │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Returns complete transaction details
- Includes EPX response data (auth_code, auth_resp)
- Never exposes full card/account numbers (only last 4)
- Includes parent_transaction_id for linked transactions

---

### 10. ListTransactions

**Purpose**: Query transactions with filters and pagination.

**Input**: `ListTransactionsRequest`
- `merchant_id` - Required filter
- `customer_id` - Optional filter
- `group_id` - Optional: get all related transactions
- `status` - Optional filter (approved, declined)
- `limit` - Default 100
- `offset` - For pagination

**Output**: `ListTransactionsResponse`
- `transactions` - Array of Transaction
- `total_count` - Total matching records

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ ListTransactionsRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentService.ListTransactions()                       │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query with Filters                            │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM transactions                              │
│ WHERE                                                   │
│   merchant_id = ? AND                                   │
│   (customer_id IS NULL OR customer_id = ?) AND          │
│   (parent_transaction_id IS NULL OR                     │
│    parent_transaction_id = ?) AND                       │
│   (status IS NULL OR status = ?)                        │
│ ORDER BY created_at DESC                                │
│ LIMIT ? OFFSET ?                                        │
│                                                         │
│ SELECT COUNT(*) for total_count                         │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Always scoped to merchant_id (multi-tenancy)
- Supports pagination for large result sets
- Ordered by created_at DESC (newest first)
- Can filter by group_id to get transaction chains

---

## PaymentMethodService RPCs

### Credit Card Storage: Complete Browser Post Flow

Before diving into individual RPCs, let's understand how credit card storage works end-to-end using Browser Post API.

#### Scenario: Customer Saves Card During Checkout

**Flow Overview**:
```
1. Customer: Enters card details on EPX-hosted form
2. Browser Post: Submits to EPX gateway
3. EPX Gateway: Creates Storage BRIC (CCE8) + Account Verification
4. EPX Callback: Sends result to your server
5. Your Server: Saves payment method in database
```

**Complete Dataflow**:

```
┌─────────────────┐
│   Customer      │
│   (Browser)     │
└────────┬────────┘
         │
         │ 1. Clicks "Save Card" button
         ▼
┌─────────────────────────────────────────────────────────┐
│ Your Frontend (HTML Form or EPX.js)                     │
├─────────────────────────────────────────────────────────┤
│ Option A: HTML Form                                     │
│ <form method="POST"                                     │
│   action="https://epx.gateway.com/browserpost">         │
│   <input name="MERCHANT_ID" value="merchant_123">       │
│   <input name="TRAN_TYPE" value="CCE8">  ← Storage BRIC │
│   <input name="TRAN_AMT" value="0.00">   ← Verify only  │
│   <input name="CALLBACK_URL"                            │
│     value="https://yourserver.com/epx/callback">        │
│   <input name="RETURN_URL"                              │
│     value="https://yoursite.com/payment/success">       │
│   <button>Save Card</button>                            │
│ </form>                                                 │
│                                                         │
│ Customer is redirected to EPX-hosted page               │
└──────────┬──────────────────────────────────────────────┘
           │
           │ 2. Customer enters card details on EPX page
           ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Hosted Payment Page                                 │
├─────────────────────────────────────────────────────────┤
│ Card Number: [4111 1111 1111 1111]                      │
│ Expiry:      [12/25]                                    │
│ CVV:         [123]                                      │
│ Billing Zip: [12345]                                    │
│                                                         │
│ [Submit] button → POST to EPX Gateway                   │
└──────────┬──────────────────────────────────────────────┘
           │
           │ 3. Submit card details
           ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Gateway Processing                                  │
├─────────────────────────────────────────────────────────┤
│ 1. Validate card number (Luhn check)                    │
│ 2. Perform $0.00 Account Verification (CCE0):           │
│    - Validate card is active                            │
│    - Validate billing address (AVS check)               │
│    - Does NOT charge customer                           │
│ 3. Create Storage BRIC (CCE8):                          │
│    - Generate permanent token                           │
│    - No expiration (unlike Financial BRIC)              │
│    - Reusable for future charges                        │
│ 4. Return response                                      │
└──────────┬──────────────────────────────────────────────┘
           │
           │ 4. EPX sends two responses
           │
           ├─────────────────────────────────────┐
           │                                     │
           ▼                                     ▼
┌──────────────────────┐           ┌──────────────────────┐
│ Browser Redirect     │           │ Server Post Callback │
│ (Customer sees)      │           │ (Background)         │
├──────────────────────┤           ├──────────────────────┤
│ EPX redirects to:    │           │ POST to CALLBACK_URL │
│ RETURN_URL           │           │ (your server)        │
│ + query params       │           │                      │
│                      │           │ Form fields:         │
│ ?status=approved     │           │ TRAN_NBR=1234567890  │
│ &message=Success     │           │ AUTH_GUID=0V703L...  │
│                      │           │ AUTH_RESP=00         │
│ Customer sees        │           │ CARD_TYPE=V          │
│ success page         │           │ LAST_FOUR=1111       │
│                      │           │ EXP_MONTH=12         │
│                      │           │ EXP_YEAR=2025        │
│                      │           │ EPX_MAC=<signature>  │
└──────────────────────┘           └───────┬──────────────┘
                                           │
                                           │ 5. Your server receives callback
                                           ▼
                           ┌─────────────────────────────────────────┐
                           │ Your Server: EPX Callback Handler       │
                           ├─────────────────────────────────────────┤
                           │ POST /epx/callback                       │
                           │                                         │
                           │ 1. Verify EPX_MAC signature (security)  │
                           │ 2. Check AUTH_RESP = "00" (approved)    │
                           │ 3. Extract Storage BRIC from AUTH_GUID  │
                           │ 4. Extract card metadata:                │
                           │    - CARD_TYPE → "visa"                 │
                           │    - LAST_FOUR → "1111"                 │
                           │    - EXP_MONTH → 12                     │
                           │    - EXP_YEAR → 2025                    │
                           └──────┬──────────────────────────────────┘
                                  │
                                  │ 6. Call PaymentMethodService
                                  ▼
                           ┌─────────────────────────────────────────┐
                           │ PaymentMethodService.SavePaymentMethod()│
                           ├─────────────────────────────────────────┤
                           │ SavePaymentMethodRequest {              │
                           │   merchant_id: "merchant_123"           │
                           │   customer_id: "customer_456"           │
                           │   payment_token: "0V703L..." ← BRIC     │
                           │   payment_type: CREDIT_CARD             │
                           │   last_four: "1111"                     │
                           │   card_brand: "visa"                    │
                           │   card_exp_month: 12                    │
                           │   card_exp_year: 2025                   │
                           │   is_default: true                      │
                           │   idempotency_key: "save_pm_<uuid>"     │
                           │ }                                       │
                           └──────┬──────────────────────────────────┘
                                  │
                                  ▼
                           ┌─────────────────────────────────────────┐
                           │ Database: Create Payment Method          │
                           ├─────────────────────────────────────────┤
                           │ INSERT INTO customer_payment_methods:   │
                           │ - id: UUID                              │
                           │ - merchant_id: "merchant_123"           │
                           │ - customer_id: "customer_456"           │
                           │ - bric: "0V703L..." ← Storage BRIC      │
                           │ - payment_type: "credit_card"           │
                           │ - last_four: "1111"                     │
                           │ - card_brand: "visa"                    │
                           │ - card_exp_month: 12                    │
                           │ - card_exp_year: 2025                   │
                           │ - is_default: true                      │
                           │ - is_active: true                       │
                           │ - is_verified: true ← CC verified       │
                           │ - created_at: NOW()                     │
                           │                                         │
                           │ ✅ Card saved! Ready for future charges │
                           └──────┬──────────────────────────────────┘
                                  │
                                  │ 7. Return success
                                  ▼
                           ┌─────────────────────────────────────────┐
                           │ Callback Response to EPX                │
                           ├─────────────────────────────────────────┤
                           │ HTTP 200 OK                             │
                           │                                         │
                           │ EPX marks callback as delivered         │
                           └─────────────────────────────────────────┘
```

**Key Points**:

1. **Card Data Never Touches Your Server**
   - Customer enters card on EPX-hosted page
   - Keeps you PCI Level 1 compliant
   - EPX handles all card validation

2. **Storage BRIC Created Immediately (CCE8)**
   - EPX creates permanent Storage BRIC
   - No 13-month expiration
   - Ready for future charges

3. **Account Verification ($0.00 Auth)**
   - EPX performs CCE0 verification automatically
   - Validates card is active and billing address
   - Customer never sees charge (immediate void)
   - Required for card-on-file compliance

4. **Dual Response Pattern**
   - Browser redirect (customer sees success)
   - Server callback (your backend processes BRIC)
   - Callback is source of truth (contains BRIC)

5. **Idempotency**
   - Callback may arrive multiple times (EPX retries)
   - Use idempotency_key to prevent duplicates
   - Database UNIQUE constraint on (merchant_id, customer_id, bric)

**Alternative: Direct Storage BRIC via Browser Post (No Payment)**

For "Add Card" flows without making a payment:

```html
<form method="POST" action="https://epx.gateway.com/browserpost">
  <input type="hidden" name="TRAN_TYPE" value="CCE8">  ← Storage only
  <input type="hidden" name="TRAN_AMT" value="0.00">   ← No charge
  <input type="hidden" name="SAVE_CARD" value="1">     ← Save flag
  <button>Add Payment Method</button>
</form>
```

EPX will:
1. Collect card details
2. Perform Account Verification (CCE0)
3. Create Storage BRIC (CCE8)
4. Send callback with BRIC
5. No payment made, just card saved

---

### 1. GetPaymentForm

**Purpose**: Generate a payment form token and configuration for Browser Post integration. Returns JSON response with form parameters and payment session details.

**Method**: REST GET (JSON response)

**Input**: Query parameters
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier (optional for guest checkout)
- `amount` - Transaction amount (e.g., "29.99")
- `transaction_type` - "auth", "sale", or "storage"
- `save_payment_method` - Boolean flag to save card (optional)

**Output**: JSON response
```json
{
  "form_token": "ft_abc123...",
  "epx_url": "https://epx.gateway.com/browserpost",
  "merchant_id": "merchant_123",
  "transaction_type": "CCE2",
  "amount": "29.99",
  "callback_url": "https://yourserver.com/api/payment/callback",
  "return_url": "https://yoursite.com/payment/success",
  "session_expires_at": "2025-11-19T15:30:00Z"
}
```

**Use Case**: Frontend requests payment form configuration, then renders EPX Browser Post form or EPX.js integration.

---

### 2. BrowserPostCallback

**Purpose**: Handle Browser Post callbacks from EPX gateway for credit card transactions. Processes auth-only, sale, and storage BRIC responses.

**Method**: REST POST (handles EPX callbacks)

**Input**: Form data from EPX
- `TRAN_NBR` - EPX transaction number
- `AUTH_GUID` - Storage BRIC token
- `AUTH_RESP` - Response code ("00" = approved)
- `TRAN_TYPE` - Transaction type (CCE1/CCE2/CCE8)
- `TRAN_AMT` - Transaction amount
- `CARD_TYPE` - Card type (V/M/D/A)
- `LAST_FOUR` - Last 4 digits
- `EXP_MONTH` / `EXP_YEAR` - Card expiry
- `EPX_MAC` - HMAC signature for verification
- Additional AVS/CVV result fields

**Processing Logic**:

```
┌─────────────────────────────────────────────────────────┐
│ POST /api/payment/callback (from EPX)                   │
├─────────────────────────────────────────────────────────┤
│ 1. Verify EPX_MAC signature (security)                  │
│ 2. Check AUTH_RESP code                                 │
│ 3. Route by TRAN_TYPE:                                  │
│    - CCE1 (Auth Only) → Create auth transaction         │
│    - CCE2 (Sale) → Create sale transaction              │
│    - CCE8 (Storage) → Save payment method only          │
└─────────────────────────────────────────────────────────┘
```

**CCE1 (Auth Only)**:
- Creates transaction record with `type: "auth"`
- Stores auth_guid (Financial BRIC, 13-month expiry)
- Status: "authorized" (pending capture)
- Use case: Hotel reservations, pre-orders

**CCE2 (Sale)**:
- Creates transaction record with `type: "sale"`
- Stores auth_guid (Financial BRIC)
- Status: "approved" (funds captured)
- Use case: E-commerce checkout
- If `save_payment_method` flag: Convert to Storage BRIC

**CCE8 (Storage)**:
- Saves payment method to `customer_payment_methods` table
- Stores auth_guid (Storage BRIC, never expires)
- No transaction created (just card storage)
- Use case: "Add payment method" without charging

**Output**: HTTP 200 OK (acknowledges receipt to EPX)

---

### 3. StoreACHAccount

**Purpose**: Create ACH Storage BRIC from raw bank account details and send pre-note for verification. Backend receives sensitive account details directly (different security model than credit cards).

**CRITICAL**: This operation initiates verification but returns immediately with `is_verified=false`. The account CANNOT be used for ACHDebit/ACHCredit until verification completes (1-3 business days). Use `VerifyACHAccount` to check status or implement webhook for automatic verification updates.

**Input**: `StoreACHAccountRequest`
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier
- Raw account details:
  - `account_number` - Bank account number
  - `routing_number` - 9-digit ABA routing number
  - `account_holder_name` - Name on account
  - `account_type` - CHECKING or SAVINGS
  - `std_entry_class` - PPD, CCD, WEB, or TEL
- Billing information (optional but recommended)
- Display metadata (bank_name, nickname)
- `is_default` - Mark as default
- `idempotency_key` - Prevents duplicates

**Output**: `PaymentMethodResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
│  (Backend)  │ ← Note: Backend receives raw account details
└──────┬──────┘   (different from credit card Browser Post)
       │ StoreACHAccountRequest
       │ (raw account + routing numbers)
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.StoreACHAccount()                  │
├─────────────────────────────────────────────────────────┤
│ 1. Validate routing number (checksum, valid bank)       │
│ 2. Validate account number format                       │
│ 3. Generate payment method UUID                         │
│ 4. Generate pre-note transaction UUID                   │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API - Step 1: Pre-Note                  │
├─────────────────────────────────────────────────────────┤
│ POST with raw account details:                          │
│ - TRAN_TYPE: "CKC0" (Checking Pre-Note Debit)           │
│   or "CKS0" (Savings Pre-Note Debit)                    │
│ - ACCOUNT_NUMBER: Raw account number                    │
│ - ROUTING_NUMBER: 9-digit routing                       │
│ - TRAN_AMT: "0.00" ($0 verification)                    │
│ - STD_ENTRY_CLASS: "PPD", "CCD", "WEB", or "TEL"        │
│ - RECV_NAME: Account holder name                        │
│ - Billing address (if provided)                         │
│                                                         │
│ EPX Response:                                           │
│ - auth_resp: "00" (accepted for ACH submission)         │
│ - auth_guid: Pre-note BRIC (temporary)                  │
│ - NACHA rules: Must wait 1-3 business days             │
└──────┬──────────────────────────────────────────────────┘
       │ Pre-note BRIC
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create Pre-Note Transaction                   │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO transactions:                               │
│ - id: Pre-note transaction UUID                         │
│ - type: "PRE_NOTE"                                      │
│ - payment_method_type: "ach"                            │
│ - amount_cents: 0 ($0.00 verification)                  │
│ - auth_guid: Pre-note BRIC from EPX                     │
│ - auth_resp: "00"                                       │
│ - metadata: { receiver_name, std_entry_class }          │
│                                                         │
│ Track pre-note for monitoring returns                   │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API - Step 2: BRIC Storage              │
├─────────────────────────────────────────────────────────┤
│ POST with raw account details:                          │
│ - TRAN_TYPE: "CKC8" or "CKS8" (ACH BRIC Storage)        │
│ - ACCOUNT_NUMBER: Raw account number                    │
│ - ROUTING_NUMBER: 9-digit routing                       │
│ - STD_ENTRY_CLASS: Same as pre-note                     │
│ - RECV_NAME: Account holder name                        │
│                                                         │
│ EPX Response:                                           │
│ - auth_guid: Storage BRIC (never expires)               │
│ - This BRIC used for all future debits/credits          │
└──────┬──────────────────────────────────────────────────┘
       │ Storage BRIC
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Create Payment Method                         │
├─────────────────────────────────────────────────────────┤
│ INSERT INTO customer_payment_methods:                   │
│ - id: Payment method UUID                               │
│ - bric: ACH Storage BRIC                                │
│ - payment_type: "ach"                                   │
│ - last_four: Last 4 of account number                   │
│ - bank_name: User-provided (e.g., "Chase")              │
│ - account_type: "checking" or "savings"                 │
│ - is_verified: false (pending pre-note)                 │
│ - is_active: true                                       │
│ - is_default: per request                               │
│                                                         │
│ Note: RAW ACCOUNT DETAILS NEVER STORED                  │
│ Only Storage BRIC and display metadata saved            │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Return Response (Pending Verification)                  │
├─────────────────────────────────────────────────────────┤
│ PaymentMethodResponse:                                  │
│ - payment_method_id: New UUID                           │
│ - is_verified: false                                    │
│ - message: "Pre-note sent. Verify in 1-3 business days" │
│                                                         │
│ Client should:                                          │
│ - Show "Pending verification" in UI                     │
│ - Poll or wait for webhook                              │
│ - Cannot use for debits until verified                  │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘

  ⏰ Wait 1-3 business days

┌─────────────────────────────────────────────────────────┐
│ EPX Server Post Callback (Pre-Note Result)              │
├─────────────────────────────────────────────────────────┤
│ If no return (success):                                 │
│ - Pre-note cleared                                      │
│ - Account verified                                      │
│                                                         │
│ If return code received (R01, R03, R10):                │
│ - Invalid account or unauthorized                       │
│ - Must disable payment method                           │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Update Verification Status                    │
├─────────────────────────────────────────────────────────┤
│ If success:                                             │
│   UPDATE customer_payment_methods                       │
│   SET is_verified = true                                │
│   WHERE id = payment_method_id                          │
│                                                         │
│ If failed:                                              │
│   UPDATE customer_payment_methods                       │
│   SET is_active = false, is_verified = false            │
│   WHERE id = payment_method_id                          │
│                                                         │
│   Notify customer: account verification failed          │
└─────────────────────────────────────────────────────────┘
```

**Pre-Note Verification Timeline**:
```
Day 0 (Today):     StoreACHAccount → EPX sends pre-note
Day 1-2:           Pre-note travels through ACH network
Day 3:             If no return → Verified ✅
                   If return (R01, R03) → Failed ❌
```

**NACHA Pre-Note Requirements**:
- $0.00 ACH transaction to test account validity
- Must wait 1-3 business days before first debit
- Common return codes:
  - R01: Insufficient funds (should still verify account exists)
  - R03: No account/unable to locate account
  - R10: Customer advises not authorized
- Pre-note success = account exists and can receive ACH
- Does NOT verify customer authorization (separate requirement)

**Key Business Rules**:
- **Backend receives raw account details** (unlike CC Browser Post)
- Must validate routing number checksum before EPX call
- Must comply with PCI-DSS (even though ACH, not cards)
- Raw account details NEVER stored in database
- Only Storage BRIC and metadata stored
- Cannot use for debits until is_verified = true
- Pre-note is automatic (not separate RPC)
- Must monitor Server Post callbacks for pre-note results
- Failed pre-note → Deactivate payment method

**Security Considerations**:
- Raw account details in-flight (TLS required)
- Log scrubbing (never log account/routing numbers)
- Audit trail (who added account, when)
- Customer authorization records (NACHA compliance)
- Data retention policies (delete raw data after BRIC creation)

**Error Scenarios**:
- Invalid routing number → Error before EPX call
- Invalid account number format → Error
- Pre-note return (R03) → Callback days later, deactivate method
- Duplicate account → ON CONFLICT on BRIC

---

### 4. VerifyACHAccount

**Purpose**: Check verification status of ACH payment method. Does NOT trigger a new pre-note (pre-note only sent during StoreACHAccount). Used for polling or customer support.

**Input**: `VerifyACHAccountRequest`
- `payment_method_id` - ACH payment method to check
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier

**Output**: `VerifyACHAccountResponse`
- `payment_method_id` - UUID
- `transaction_id` - Pre-note transaction ID (if exists)
- `status` - "pending", "verified", "failed"
- `message` - Status message
  - "pending": "Verification in progress. Please wait 1-3 business days."
  - "verified": "Account verified and ready to use."
  - "failed": "Verification failed. Reason: Invalid routing number" (or other error)

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ VerifyACHAccountRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.VerifyACHAccount()                 │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request                                     │
│ 2. Query payment method                                 │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Payment Method                          │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM customer_payment_methods                  │
│ WHERE id = payment_method_id                            │
│                                                         │
│ Validations:                                            │
│ - Payment method exists                                 │
│ - payment_type = "ach"                                  │
│ - Merchant and customer match                           │
│ - is_active = true                                      │
└──────┬──────────────────────────────────────────────────┘
       │ Storage BRIC
       ▼
┌─────────────────────────────────────────────────────────┐
│ EPX Server Post API - Send Pre-Note                     │
├─────────────────────────────────────────────────────────┤
│ Note: Cannot use Storage BRIC for pre-note              │
│ Must use original account details (not available)       │
│                                                         │
│ Alternative Implementation:                             │
│ - If we stored encrypted account details               │
│ - Decrypt and send pre-note                             │
│                                                         │
│ OR:                                                     │
│ - Pre-note only during StoreACHAccount                  │
│ - This RPC just checks existing pre-note status         │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Return Verification Status                              │
├─────────────────────────────────────────────────────────┤
│ - If is_verified = true: "verified"                     │
│ - If pending pre-note: "pending"                        │
│ - If pre-note failed: "failed"                          │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Implementation**: This RPC is **read-only** and checks verification status. Pre-note is automatically sent during `StoreACHAccount`. This RPC does NOT trigger a new pre-note.

**Use Cases**:
1. **Polling**: Frontend polls every few hours to check if verification completed
2. **Customer Support**: Check why account isn't verified yet
3. **Debugging**: See verification status and any error messages

**Alternative to Polling**: Implement EPX webhook `/api/epx/webhook/prenote` that automatically updates `is_verified=true` when pre-note clears. This is more efficient than polling.

---

### 5. GetPaymentMethod

**Purpose**: Retrieve a specific saved payment method.

**Input**: `GetPaymentMethodRequest`
- `payment_method_id` - UUID

**Output**: `PaymentMethod`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ GetPaymentMethodRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.GetPaymentMethod()                 │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Payment Method                          │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM customer_payment_methods                  │
│ WHERE id = payment_method_id AND deleted_at IS NULL     │
│                                                         │
│ Returns:                                                │
│ - All payment method fields                             │
│ - NEVER returns bric (internal only)                    │
│ - Only last_four for display                            │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Map to PaymentMethod Proto                              │
├─────────────────────────────────────────────────────────┤
│ - Omit bric field (security)                            │
│ - Include display metadata only                         │
│ - Format timestamps                                     │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Never exposes BRIC token in response
- Only returns display metadata (last 4, brand, etc.)
- Filters out soft-deleted records

---

### 6. ListPaymentMethods

**Purpose**: List all saved payment methods for a customer with optional filters.

**Input**: `ListPaymentMethodsRequest`
- `merchant_id` - Required
- `customer_id` - Required
- `payment_type` - Optional filter (CREDIT_CARD or ACH)
- `is_active` - Optional filter

**Output**: `ListPaymentMethodsResponse`
- `payment_methods` - Array of PaymentMethod

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ ListPaymentMethodsRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.ListPaymentMethods()               │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Query Payment Methods                         │
├─────────────────────────────────────────────────────────┤
│ SELECT * FROM customer_payment_methods                  │
│ WHERE                                                   │
│   merchant_id = ? AND                                   │
│   customer_id = ? AND                                   │
│   deleted_at IS NULL AND                                │
│   (payment_type IS NULL OR payment_type = ?) AND        │
│   (is_active IS NULL OR is_active = ?)                  │
│ ORDER BY is_default DESC, created_at DESC               │
│                                                         │
│ Note: Default payment method appears first              │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Ordered by default first, then newest
- Never exposes BRICs
- Filters out deleted payment methods
- Can filter by type (show only cards or only ACH)

---

### 7. UpdatePaymentMethodStatus

**Purpose**: Activate or deactivate a payment method without deleting it.

**Input**: `UpdatePaymentMethodStatusRequest`
- `payment_method_id` - UUID
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier
- `is_active` - true to activate, false to deactivate

**Output**: `PaymentMethodResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ UpdatePaymentMethodStatusRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.UpdatePaymentMethodStatus()        │
├─────────────────────────────────────────────────────────┤
│ 1. Validate ownership (merchant, customer)              │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Update Status                                 │
├─────────────────────────────────────────────────────────┤
│ UPDATE customer_payment_methods                         │
│ SET is_active = ?, updated_at = CURRENT_TIMESTAMP       │
│ WHERE id = ? AND merchant_id = ? AND customer_id = ?    │
│   AND deleted_at IS NULL                                │
│ RETURNING *                                             │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Deactivating prevents use in transactions
- Does not delete payment method (reversible)
- If deactivating default payment method, should prompt to set new default

**Use Cases**:
- Temporary card hold/freeze
- Expired credit card
- Failed ACH verification
- Customer-requested suspension

---

### 8. DeletePaymentMethod

**Purpose**: Soft delete a payment method (sets deleted_at timestamp).

**Input**: `DeletePaymentMethodRequest`
- `payment_method_id` - UUID
- `idempotency_key` - Prevents duplicate deletes

**Output**: `DeletePaymentMethodResponse`
- `success` - Boolean
- `message` - Confirmation message

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ DeletePaymentMethodRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.DeletePaymentMethod()              │
├─────────────────────────────────────────────────────────┤
│ 1. Validate request                                     │
│ 2. Check for active subscriptions                       │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Check Active Subscriptions                    │
├─────────────────────────────────────────────────────────┤
│ SELECT COUNT(*) FROM subscriptions                      │
│ WHERE payment_method_id = ?                             │
│   AND status = 'active'                                 │
│   AND deleted_at IS NULL                                │
│                                                         │
│ If count > 0:                                           │
│   Error: "Cannot delete payment method with active      │
│           subscriptions. Cancel subscriptions first."   │
└──────┬──────────────────────────────────────────────────┘
       │ No active subscriptions
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Soft Delete                                   │
├─────────────────────────────────────────────────────────┤
│ UPDATE customer_payment_methods                         │
│ SET deleted_at = CURRENT_TIMESTAMP,                     │
│     updated_at = CURRENT_TIMESTAMP                      │
│ WHERE id = ? AND deleted_at IS NULL                     │
│                                                         │
│ Note: BRIC remains in database for audit/reconciliation │
│ Cannot be undeleted (create new if needed)              │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Soft delete (sets timestamp, doesn't remove row)
- Prevents deletion if active subscriptions exist
- Past transactions remain linked (audit trail)
- BRIC token retained for reconciliation
- Cannot be undeleted (customer must add new payment method)

**Error Scenarios**:
- Active subscriptions → Error, must cancel first
- Already deleted → Idempotent, return success

---

### 9. SetDefaultPaymentMethod

**Purpose**: Mark a payment method as the customer's default.

**Input**: `SetDefaultPaymentMethodRequest`
- `payment_method_id` - UUID
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier

**Output**: `PaymentMethodResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ SetDefaultPaymentMethodRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.SetDefaultPaymentMethod()          │
├─────────────────────────────────────────────────────────┤
│ 1. Validate ownership                                   │
│ 2. Validate payment method is active                    │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Transaction (Atomic Update)                   │
├─────────────────────────────────────────────────────────┤
│ BEGIN TRANSACTION;                                      │
│                                                         │
│ -- Step 1: Unset all defaults for customer              │
│ UPDATE customer_payment_methods                         │
│ SET is_default = false, updated_at = CURRENT_TIMESTAMP  │
│ WHERE merchant_id = ? AND customer_id = ?               │
│   AND deleted_at IS NULL;                               │
│                                                         │
│ -- Step 2: Set new default                              │
│ UPDATE customer_payment_methods                         │
│ SET is_default = true, updated_at = CURRENT_TIMESTAMP   │
│ WHERE id = ? AND deleted_at IS NULL                     │
│ RETURNING *;                                            │
│                                                         │
│ COMMIT;                                                 │
│                                                         │
│ Ensures only one default per customer (atomic)          │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- Only one default payment method per customer
- Must be active payment method
- Must be verified (especially for ACH)
- Atomic operation (transaction ensures consistency)

**Use Cases**:
- Customer changes preferred payment method
- Subscription renewals use default payment method
- One-click checkout uses default

---

### 10. UpdatePaymentMethod

**Purpose**: Update payment method metadata only (billing info, nickname). Does NOT support changing account/routing numbers or card numbers.

**Input**: `UpdatePaymentMethodRequest`
- `payment_method_id` - UUID
- `merchant_id` - Multi-tenant identifier
- `customer_id` - Customer identifier
- Optional updates:
  - `billing_name` - Updated cardholder/account holder name
  - `billing_address`, `billing_city`, `billing_state`, `billing_zip`
  - `nickname` - User-friendly label ("Primary checking", "Business card")
  - `is_default` - Change default status
- `idempotency_key` - Prevents duplicate updates

**Output**: `PaymentMethodResponse`

**Dataflow**:

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ UpdatePaymentMethodRequest
       ▼
┌─────────────────────────────────────────────────────────┐
│ PaymentMethodService.UpdatePaymentMethod()              │
├─────────────────────────────────────────────────────────┤
│ 1. Validate ownership                                   │
│ 2. Build update fields (only non-null values)           │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ Database: Update Metadata Only                          │
├─────────────────────────────────────────────────────────┤
│ UPDATE customer_payment_methods                         │
│ SET                                                     │
│   billing_name = COALESCE(?, billing_name),             │
│   billing_address = COALESCE(?, billing_address),       │
│   billing_city = COALESCE(?, billing_city),             │
│   billing_state = COALESCE(?, billing_state),           │
│   billing_zip = COALESCE(?, billing_zip),               │
│   nickname = COALESCE(?, nickname),                     │
│   updated_at = CURRENT_TIMESTAMP                        │
│ WHERE id = ? AND merchant_id = ? AND customer_id = ?    │
│   AND deleted_at IS NULL                                │
│ RETURNING *;                                            │
│                                                         │
│ Note: BRIC, account numbers, routing numbers IMMUTABLE  │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────────────────────────┐
│ If is_default changed: Handle Default Update            │
├─────────────────────────────────────────────────────────┤
│ Same logic as SetDefaultPaymentMethod                   │
└──────┬──────────────────────────────────────────────────┘
       │
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

**Key Business Rules**:
- **Metadata only** - cannot change account/routing/card numbers
- To change account details: Delete old, create new payment method
- BRIC token is immutable
- Billing info updates useful for address changes
- Nickname helpful for UI ("My business card", "Emergency backup")

**What CANNOT be Updated**:
- BRIC token
- Account number
- Routing number
- Card number
- Payment type (credit_card ↔ ACH)
- Last four digits

**What CAN be Updated**:
- Billing name
- Billing address
- Nickname
- Default status
- Card expiration date (optional, for display only)

---

## Summary Tables

### Payment Operations

| RPC | Purpose | Payment Method | EPX TRAN_TYPE | Settlement |
|-----|---------|---------------|---------------|------------|
| Authorize | Hold funds | Storage/Financial BRIC | CCE1 | 7-10 days hold |
| Capture | Complete auth | Uses AUTH BRIC | CCE4 | 1-3 business days |
| Sale | Immediate charge | Storage/Financial BRIC | CCE2 | 1-3 business days |
| Void | Cancel same-day | Uses original BRIC | CCEX | Immediate |
| Refund | Return after settlement | Uses CHARGE BRIC | CCE3 | 5-10 business days |
| ACHDebit | Pull from bank | **Verified** Storage BRIC (required) | CKC2/CKS2 | 3-5 business days |
| ACHCredit | Send to bank | **Verified** Storage BRIC (required) | CKC3/CKS3 | 3-5 business days |
| ACHVoid | Cancel same-day ACH | Uses original BRIC | CKCX/CKSX | Before submission |

### Payment Method Operations

| RPC | Purpose | EPX Interaction | Database Operation |
|-----|---------|-----------------|-------------------|
| GetPaymentForm | Generate payment form config | None (returns JSON) | None (stateless) |
| BrowserPostCallback | Handle EPX callbacks | Browser Post result | INSERT/UPDATE transaction or payment method |
| StoreACHAccount | Save bank account + pre-note | CKC0 (Pre-note) + CKC8 (Storage) | INSERT (is_verified=false) |
| VerifyACHAccount | Check verification status | None (read-only) | SELECT/UPDATE |
| GetPaymentMethod | Retrieve one | None | SELECT |
| ListPaymentMethods | Retrieve many | None | SELECT |
| UpdatePaymentMethodStatus | Activate/deactivate | None | UPDATE |
| DeletePaymentMethod | Soft delete | None | UPDATE (soft delete) |
| SetDefaultPaymentMethod | Change default | None | UPDATE (atomic) |
| UpdatePaymentMethod | Update metadata | None | UPDATE |

### Transaction Relationships

```
Standalone Transactions (parent_transaction_id = NULL):
- AUTH
- CHARGE (sale)
- DEBIT (ACH)
- STORAGE (BRIC creation)

Child Transactions (parent_transaction_id = parent ID):
- CAPTURE → AUTH
- VOID → AUTH or CHARGE
- REFUND → CHARGE or CAPTURE
- ACH VOID → DEBIT
```

### BRIC Types and Usage

| BRIC Type | Expiry | Created By | Used For |
|-----------|--------|------------|----------|
| Financial BRIC | 13 months | Browser Post (CCE2/CCE1) | One-time or short-term use |
| Storage BRIC | Never | CCE8, CKC8 | Saved payment methods, subscriptions |
| Pre-note BRIC | Temporary | CKC0/CKS0 | ACH verification only |

---

## Error Handling Patterns

All RPCs follow these error patterns:

1. **Validation Errors** (before EPX call)
   - Invalid input format
   - Missing required fields
   - Business rule violations
   - Return gRPC error immediately

2. **Authorization Errors**
   - Merchant doesn't own resource
   - Customer mismatch
   - Return gRPC PERMISSION_DENIED

3. **EPX Gateway Errors**
   - Network timeout
   - Invalid EPX response
   - Create transaction with status = "failed"
   - Return error to client

4. **EPX Declined Transactions**
   - auth_resp ≠ "00"
   - Create transaction with status = "declined"
   - Return PaymentResponse with is_approved = false

5. **Database Errors**
   - Constraint violations
   - Connection errors
   - Return gRPC INTERNAL error

6. **Idempotency**
   - Duplicate idempotency_key
   - Return existing transaction (ON CONFLICT DO NOTHING)

---

## Compliance and Security

### PCI DSS Compliance
- Never store full card numbers (only last 4)
- Never store CVV/CVV2
- Never log card numbers
- Use TLS for all communications
- Tokenization via EPX Browser Post

### NACHA Compliance (ACH)
- Pre-note verification required
- Customer authorization records
- Return monitoring and handling
- Proper entry class codes
- Record retention (2 years minimum)

### Multi-Tenancy
- All operations scoped to merchant_id
- Customers scoped to merchant_id + customer_id
- Prevent cross-merchant access
- Audit logging for all operations

---

## ACH Verification Requirements

### Verified-Only Architecture

**All ACH operations (ACHDebit, ACHCredit) require verified payment methods.**

#### Why Verification is Required

1. **NACHA Compliance**: Pre-note verification required for recurring debits
2. **Risk Management**: Prevents returns from invalid accounts
3. **Better UX**: Clear verification status shown to customers
4. **Audit Trail**: All ACH transactions linked to verified accounts
5. **No Surprises**: Account validated before first real transaction

#### Verification Flow Timeline

```
Day 0 (Today):
  └─ StoreACHAccount RPC
     ├─ Creates payment method (is_verified=false)
     ├─ Sends CKC0 pre-note ($0.00)
     └─ Returns immediately

Day 1-2:
  └─ Pre-note travels through ACH network

Day 3:
  └─ Pre-note clears (or returns with error)
     ├─ Success: VerifyACHAccount updates is_verified=true
     └─ Failure: Account remains is_verified=false

Day 3+:
  └─ Account ready for use
     └─ ACHDebit/ACHCredit now allowed
```

#### Customer Experience

**Adding ACH Account:**
1. Customer enters bank details
2. System: "Account added! Verification in progress (1-3 business days)"
3. Email/notification sent
4. Customer waits
5. Email: "Account verified! Ready to use."

**Attempting Unverified Payment:**
1. Customer selects pending ACH account
2. System: "This account is still being verified. Please use another payment method or wait 1-3 business days."
3. Show alternative: verified ACH accounts or credit cards
4. Display verification status: "Pending verification (added 1 day ago)"

#### Implementation Checklist

**Backend Validation:**
- ✅ Check `is_verified=true` before ACHDebit/ACHCredit
- ✅ Return clear error message if `is_verified=false`
- ✅ Log verification failures for monitoring
- ✅ Implement webhook for auto-verification (optional but recommended)

**Frontend Display:**
- ✅ Show verification status on payment methods list
- ✅ Disable unverified ACH accounts in payment selector
- ✅ Display "Pending verification" badge
- ✅ Show estimated verification completion date
- ✅ Send email/notification when verified

**Error Messages:**
```json
{
  "error": {
    "code": "PAYMENT_METHOD_NOT_VERIFIED",
    "message": "ACH account verification pending. Please wait 1-3 business days or use another payment method.",
    "details": {
      "payment_method_id": "pm_123",
      "verification_status": "pending",
      "added_at": "2025-01-19T10:00:00Z",
      "estimated_verification": "2025-01-22T10:00:00Z"
    }
  }
}
```

#### Database Schema

```sql
CREATE TABLE customer_payment_methods (
  id UUID PRIMARY KEY,
  payment_type VARCHAR(20), -- 'credit_card' or 'ach'
  is_verified BOOLEAN DEFAULT false, -- CRITICAL for ACH
  is_active BOOLEAN DEFAULT true,
  -- ... other fields

  CONSTRAINT ach_verified CHECK (
    payment_type != 'ach' OR
    (payment_type = 'ach' AND is_verified IS NOT NULL)
  )
);

-- Query for usable ACH payment methods
SELECT * FROM customer_payment_methods
WHERE payment_type = 'ach'
  AND is_verified = true  -- REQUIRED
  AND is_active = true
  AND deleted_at IS NULL;
```

#### Alternative: Micro-Deposits (Not Recommended)

Some processors use micro-deposit verification:
1. Send 2 small deposits ($0.32, $0.45)
2. Customer confirms amounts
3. Account verified

**We don't use this because:**
- Pre-note is NACHA standard
- Micro-deposits take longer (2-3 days + customer action)
- Higher failure rate (customer forgets to verify)
- EPX supports pre-note natively

