# North Payment Gateway API Guide

## Quick Answer: Which API for What?

| Use Case | API to Use | Adapter | Reason |
|----------|-----------|---------|--------|
| **Immediate one-time payment** (checkout) | Browser Post API | BrowserPostAdapter | ✅ PCI compliant, no storage |
| **One-time charge** (stored method) | Recurring Billing API | RecurringBillingAdapter | ✅ Charge stored payment method |
| **Recurring subscription** | Recurring Billing API | RecurringBillingAdapter | ✅ Automatic billing schedule |
| **Raw card processing** | Custom Pay API | CustomPayAdapter | ❌ Avoid (PCI risk) |
| **Bank transfer** | Pay-by-Bank API | ACHAdapter | ✅ For ACH |

**IMPORTANT:** Recurring Billing API = "Stored Payment Methods API"
- Can be used for BOTH one-time charges AND recurring subscriptions
- Creates stored payment methods that can be charged on-demand or on a schedule

## The APIs Explained

### 1. Browser Post API ✅ (One-Time Payments)

**Current implementation:** `BrowserPostAdapter`

**What it does:**
- Processes one-time payments with BRIC tokens
- Frontend tokenizes card → backend processes with token
- PCI compliant (backend never sees raw card data)

**Best for:**
- Single purchases
- One-time charges
- Auth/capture flows
- Refunds and voids

**Operations:**
```
POST /sale              - Authorize or sale
POST /sale/{id}/capture - Capture authorized payment
POST /void/{id}         - Void transaction
POST /refund/{id}       - Refund transaction
```

**Request example:**
```
BRIC=tok_abc123
AMOUNT=99.99
TRAN_TYPE=S  (S=sale, A=auth only)
```

**When to use:**
- ✅ Customer checking out with a product
- ✅ One-time service payment
- ✅ Donation
- ✅ Ad-hoc charge

**When NOT to use:**
- ❌ Recurring monthly subscription (use Recurring Billing instead)
- ❌ Automatic billing schedule (use Recurring Billing instead)

---

### 2. Recurring Billing API ✅ (Stored Payment Methods + Subscriptions)

**Current implementation:** `RecurringBillingAdapter`

**What it does:**
- Stores payment methods (customer vault)
- Charges stored payment methods one-time (on-demand)
- Creates automatic recurring billing schedules
- Manages subscription lifecycle (pause, resume, cancel)

**Best for:**
- Storing customer payment methods securely
- One-time charges to stored payment methods
- Monthly subscriptions with automatic billing
- Recurring donations
- Membership fees

**Operations:**
```
POST /subscription              - Create subscription (stores payment method + billing schedule)
PUT /subscription/{id}          - Update subscription
POST /subscription/cancel       - Cancel subscription
POST /subscription/pause        - Pause billing
POST /subscription/resume       - Resume billing
POST /chargepaymentmethod       - One-time charge to stored payment method (independent from subscription)
```

**Request example (Create Subscription):**
```json
{
  "paymentMethod": {
    "previousPayment": {
      "bric": "tok_abc123",
      "paymentType": "CreditCard"
    }
  },
  "subscriptionData": {
    "amount": 29.99,
    "frequency": "monthly",
    "billingDate": "2025-10-20",
    "retries": 3
  }
}
```

**Request example (One-Time Charge to Stored Method):**
```json
{
  "PaymentMethodID": 12345,
  "Amount": 99.99
}
```

**When to use:**
- ✅ Store customer payment method for future use
- ✅ One-time charge to previously stored payment method
- ✅ Monthly subscription (SaaS, gym, Netflix-style)
- ✅ Weekly billing
- ✅ Annual renewal
- ✅ Installment plans

**When NOT to use:**
- ❌ Immediate checkout without storing payment method (use Browser Post instead)

---

### 3. Custom Pay API ❌ (Avoid - PCI Risk)

**Implementation exists:** `CustomPayAdapter` (not currently used)

**What it does:**
- Processes payments with raw card data (JSON)
- Backend receives card number directly

**Why we DON'T use it:**
- ❌ PCI compliance risk (backend handles raw card data)
- ❌ Requires SAQ-D compliance (most extensive)
- ❌ Security liability

**We replaced it with:** Browser Post API (tokenized)

---

### 4. ACH / Pay-by-Bank API ✅ (Bank Transfers)

**Current implementation:** `ACHAdapter`

**What it does:**
- Processes bank account transfers (checking/savings)
- Lower cost than cards (~$0.25 vs ~2.9%)

**Best for:**
- Large transactions (lower % fee)
- B2B payments
- Rent/utilities
- Payroll

---

## Key Architectural Decision

### Our Current Setup (Correct ✅)

```go
// cmd/server/main.go

// ONE-TIME PAYMENTS → Browser Post Adapter
creditCardGateway := north.NewBrowserPostAdapter(
    authConfig,
    cfg.Gateway.BaseURL,
    httpClient,
    logger,
)

// RECURRING SUBSCRIPTIONS → Recurring Billing Adapter
recurringGateway := north.NewRecurringBillingAdapter(
    authConfig,
    cfg.Gateway.BaseURL,
    httpClient,
    logger,
)

// ONE-TIME PAYMENTS use Browser Post
paymentService := payment.NewService(
    dbPort,
    txRepo,
    creditCardGateway,  // Browser Post for one-time
    logger,
)

// SUBSCRIPTIONS use Recurring Billing
subscriptionService := subscription.NewService(
    dbPort,
    subRepo,
    paymentService,
    recurringGateway,  // Recurring Billing for subscriptions
    logger,
)
```

**Why this is correct:**
- One-time payments → Browser Post → PCI compliant, tokenized
- Recurring payments → Recurring Billing → Automatic scheduling

---

## Browser Post vs Recurring Billing: When to Use Each

**Key Understanding:** Recurring Billing API is for **stored payment methods**, not just subscriptions.

### Use Browser Post When:
- ✅ Customer is checking out immediately (one-time purchase)
- ✅ You DON'T need to store the payment method
- ✅ Customer won't be charged again in the future
- ✅ Simple, immediate payment flow

**Example:** Customer buys a product for $99.99 → done

### Use Recurring Billing When:
- ✅ You need to STORE the payment method for future use
- ✅ Customer will be charged again (either on-demand OR on a schedule)
- ✅ You want North to securely manage payment methods (customer vault)

**Two Scenarios:**

**Scenario A - Store + Charge On-Demand:**
1. Customer signs up → Create subscription (stores payment method)
2. Later, charge customer $50 → Use `/chargepaymentmethod`
3. Later again, charge customer $75 → Use `/chargepaymentmethod`
4. No automatic billing - YOU control when to charge

**Scenario B - Store + Automatic Billing:**
1. Customer subscribes to $29.99/month plan → Create subscription
2. North automatically charges $29.99 every month
3. You can also charge one-time fees → Use `/chargepaymentmethod`

**Analogy:**
```
Browser Post         = Pay cash at a store (one-time, no record)
Recurring Billing    = Credit card on file (can charge anytime, or set up auto-billing)
```

---

## Decision Matrix

### Scenario 1: Customer buys a product for $99.99

**Question:** Which API?

**Answer:** Browser Post API

**Flow:**
```
1. Frontend tokenizes card → BRIC token
2. Frontend sends token to backend
3. Backend calls Browser Post: POST /sale
4. Payment processed once
5. Done ✅
```

**Code:**
```go
result, err := creditCardGateway.Authorize(ctx, &ports.AuthorizeRequest{
    Token:    "tok_abc123",
    Amount:   decimal.NewFromFloat(99.99),
    Capture:  true,  // Immediate charge
})
```

---

### Scenario 2: Customer subscribes to $29.99/month service

**Question:** Which API?

**Answer:** Recurring Billing API

**Flow:**
```
1. Frontend tokenizes card → BRIC token
2. Frontend sends token to backend
3. Backend calls Recurring Billing: POST /subscription
4. North creates automatic billing schedule
5. North charges customer automatically every month
6. Done ✅
```

**Code:**
```go
result, err := recurringGateway.CreateSubscription(ctx, &ports.SubscriptionRequest{
    PaymentToken: "tok_abc123",
    Amount:       decimal.NewFromFloat(29.99),
    Frequency:    models.FrequencyMonthly,
    StartDate:    time.Now(),
})
```

---

### Scenario 3: Customer wants to be charged $50 every Friday

**Question:** Which API?

**Answer:** **Option A (Recommended):** Recurring Billing API with automatic billing
- Set frequency to weekly
- Let North handle automatic charging

**Option B:** Recurring Billing API with manual charging
- Create subscription to store payment method
- Run a cron job every Friday
- Charge using `/chargepaymentmethod` endpoint

**Which to choose?**
- Use Option A if amount is always $50 and schedule is always Friday ✅
- Use Option B if you need custom logic (e.g., skip holidays, variable amounts)

---

### Scenario 4: Customer wants 12 monthly payments of $100

**Question:** Which API?

**Answer:** Recurring Billing API

**Special feature:**
```json
{
  "subscriptionData": {
    "amount": 100.00,
    "frequency": "monthly",
    "numberOfPayments": 12  // Auto-cancel after 12 payments
  }
}
```

North automatically:
- Charges 12 times
- Cancels subscription after 12th payment
- You don't have to track manually

---

## Special Cases

### Case 1: Variable Recurring Amount

**Scenario:** Charge customer monthly, but amount varies (utility bill style)

**API to use:** Recurring Billing API with `/chargepaymentmethod`

**Why:**
- North securely stores payment method (customer vault)
- You control when and how much to charge
- No PCI burden on your system

**Implementation:**
```go
// 1. Create subscription to store payment method (can set numberOfPayments to 0 or use a dummy amount)
// 2. Run monthly cron job
// 3. Calculate amount for this month
// 4. Charge using /chargepaymentmethod endpoint

func (s *Service) ProcessMonthlyBilling() {
    customers := s.getCustomersWithStoredPaymentMethods()

    for _, customer := range customers {
        amount := s.calculateMonthlyAmount(customer)  // Variable amount

        // Use ChargePaymentMethod instead of Browser Post
        result, err := s.recurringGateway.ChargePaymentMethod(ctx,
            customer.PaymentMethodID,
            amount,
        )
    }
}
```

---

### Case 2: Free Trial then Subscription

**Scenario:** 14-day free trial, then $29.99/month

**API to use:** Recurring Billing API

**Implementation:**
```go
// Create subscription with future start date
result, err := recurringGateway.CreateSubscription(ctx, &ports.SubscriptionRequest{
    PaymentToken: "tok_abc123",
    Amount:       decimal.NewFromFloat(29.99),
    Frequency:    models.FrequencyMonthly,
    StartDate:    time.Now().AddDate(0, 0, 14),  // Start in 14 days
})
```

North won't charge until start date.

---

### Case 3: Pay-As-You-Go with Stored Card

**Scenario:** Customer adds card, you charge variable amounts on-demand

**API to use:** Recurring Billing API with `/chargepaymentmethod`

**Why:**
- North securely stores payment method (no PCI burden)
- Amount varies
- Timing varies
- Perfect for on-demand charging

**Implementation:**
```go
// 1. First time: Create subscription to store payment method
func (s *Service) AddPaymentMethod(token string) {
    // Create subscription to store payment method
    // Set numberOfPayments to 0 (infinite) with a future start date to avoid automatic charges
    result, _ := s.recurringGateway.CreateSubscription(ctx, &ports.SubscriptionRequest{
        PaymentToken: token,
        Amount:       decimal.Zero,  // Dummy amount, we won't use auto-billing
        Frequency:    models.FrequencyMonthly,
        StartDate:    time.Now().AddDate(100, 0, 0),  // Far future date
    })

    // Store the payment method ID
    s.savePaymentMethodID(customerID, result.GatewaySubscriptionID)
}

// 2. Future charges: Use stored payment method
func (s *Service) ChargeStoredCard(customerID string, amount decimal.Decimal) {
    paymentMethodID := s.getPaymentMethodID(customerID)

    // Charge using /chargepaymentmethod endpoint
    result, _ := s.recurringGateway.ChargePaymentMethod(ctx,
        paymentMethodID,
        amount,
    )
}
```

---

## Summary Table

| Payment Type | API | Why |
|--------------|-----|-----|
| **Immediate one-time purchase** | Browser Post | Single charge, no storage needed |
| **One-time charge (stored method)** | Recurring Billing (`/chargepaymentmethod`) | Charge stored payment method once |
| **Fixed recurring** | Recurring Billing (`/subscription`) | Auto-billing schedule |
| **Variable recurring** | Recurring Billing (`/chargepaymentmethod`) | Stored method + flexible amounts |
| **Installments (fixed)** | Recurring Billing | Use `numberOfPayments` |
| **Installments (custom)** | Recurring Billing (`/chargepaymentmethod`) | Manual control with stored method |
| **Free trial → subscription** | Recurring Billing | Future start date |
| **Pay-as-you-go** | Recurring Billing (`/chargepaymentmethod`) | On-demand charging of stored method |

---

## Best Practices

### 1. Payment Method Storage

**Browser Post (No Storage):**
- BRIC tokens are typically single-use
- Each charge requires fresh tokenization
- No payment method storage
- Best for immediate checkout

**Recurring Billing (Stored Methods):**
- North stores payment methods securely
- Returns `customerId` and `paymentMethodId`
- Can charge multiple times without re-tokenization
- Best for repeat customers

**Recommendation:** Use Recurring Billing API if you need to charge customers more than once

### 2. PCI Compliance

**Browser Post (Tokenized):**
- ✅ Backend never sees card data
- ✅ Reduced PCI scope (SAQ-A)
- ✅ Lower compliance burden

**Custom Pay (Raw Card):**
- ❌ Backend handles card data
- ❌ Full PCI scope (SAQ-D)
- ❌ Higher compliance burden

**Recurring Billing (Tokenized):**
- ✅ Backend never sees card data
- ✅ Reduced PCI scope
- ✅ North manages recurring charges

### 3. Error Handling

**Recurring Billing:**
- Subscription creation might succeed but first charge fails
- Check both subscription status AND charge status
- Implement retry logic for failed recurring charges

**Browser Post:**
- Immediate feedback (success or failure)
- Easier to handle errors in real-time

---

## Migration Guide

### If you're currently using Custom Pay for one-time payments:

**From:**
```go
creditCardGateway := north.NewCustomPayAdapter(...)
```

**To:**
```go
creditCardGateway := north.NewBrowserPostAdapter(...)
```

**Changes needed:**
1. Frontend: Add North JavaScript SDK for tokenization
2. Frontend: Tokenize cards before sending to backend
3. Backend: Accept BRIC tokens instead of raw card data
4. Testing: Use tokenized test cards

**We already did this!** ✅ (cmd/server/main.go:209)

---

## FAQ

**Q: Can I use Recurring Billing API for one-time payments?**
A: Yes! Use the `/chargepaymentmethod` endpoint to charge a stored payment method one-time.

**Q: What if I want to charge a customer multiple times, but not on a schedule?**
A: Create a subscription to store the payment method, then use `/chargepaymentmethod` to charge on-demand.

**Q: Do I need both adapters?**
A: Yes! Browser Post for immediate checkout (no storage), Recurring Billing for stored payment methods (one-time OR recurring).

**Q: Can I switch a customer from one-time to recurring?**
A: If you already created a subscription (for payment method storage), you can update it to enable automatic billing. If you used Browser Post, create a new subscription with a fresh BRIC token.

**Q: What about refunds for recurring charges?**
A: Use Browser Post API refund endpoint with the transaction ID from the charge.

**Q: Should I store BRIC tokens in my database?**
A: No. Use Recurring Billing API to let North store payment methods securely (better PCI compliance).

---

## Next Steps

1. **Implement Payment Method Storage**
   - Use Recurring Billing API to store customer payment methods
   - Store `customerId` and `paymentMethodId` in your database
   - Use `/chargepaymentmethod` for on-demand charges

2. **Document Your Use Cases**
   - List all payment scenarios in your app
   - Map each to the correct API (Browser Post vs Recurring Billing)
   - Update frontend integration guide

3. **Test All Flows**
   - Immediate checkout with Browser Post
   - Store payment method with Recurring Billing
   - One-time charge to stored method (`/chargepaymentmethod`)
   - Recurring subscription with automatic billing

---

## Current Architecture (Correct ✅)

```
Immediate One-Time Payment (No Storage):
Frontend → Tokenize Card → BRIC Token → Browser Post API → Charge

Store Payment Method + One-Time Charge:
Frontend → Tokenize Card → BRIC Token → Recurring Billing API (CreateSubscription) → Store PaymentMethodID
Backend → Charge On-Demand → /chargepaymentmethod → One-Time Charge

Store Payment Method + Recurring Subscription:
Frontend → Tokenize Card → BRIC Token → Recurring Billing API (CreateSubscription) → Auto-Billing Schedule
Backend → Optional One-Time Charges → /chargepaymentmethod → Additional Charges
```

**Key Insight:** Recurring Billing API = Stored Payment Methods (for both one-time AND recurring use)

## Resources

- North Browser Post API Documentation
- North Recurring Billing API Documentation
- PCI DSS Compliance Guide
- Frontend Integration Guide (docs/FRONTEND_INTEGRATION.md)
