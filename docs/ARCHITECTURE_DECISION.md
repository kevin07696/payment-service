# Architecture Decision: Payment Method Storage & One-Time Charging

## Context

**Question:** Should we use Recurring Billing API for one-time payments?

**ANSWER (RESOLVED):** ✅ YES - Recurring Billing API supports BOTH:
1. One-time charges via `/chargepaymentmethod` endpoint
2. Recurring subscriptions via `/subscription` endpoint

**Key Insight:** "Recurring Billing" = "Stored Payment Methods API"

---

## ✅ RESOLUTION (2025-10-21)

### What We Discovered

North's Recurring Billing API provides **THREE capabilities**:

1. **Store Payment Methods** (Customer Vault)
   - Create subscription → Returns `customerId` and `paymentMethodId`
   - North securely stores payment method
   - No PCI burden on our system

2. **One-Time Charging** (`POST /chargepaymentmethod`)
   - Charge stored payment method on-demand
   - Variable amounts
   - **Independent from subscriptions** (does not count toward subscription payments)

3. **Recurring Subscriptions** (`POST /subscription`)
   - Automatic billing schedule
   - Fixed frequency (weekly, monthly, yearly)
   - Fixed amount per billing cycle

### Implementation Completed

✅ Added `ChargePaymentMethod()` to `RecurringBillingAdapter` (internal/adapters/north/recurring_billing_adapter.go:304-352)
✅ Updated `RecurringBillingGateway` interface (internal/domain/ports/subscription_gateway.go:68-70)
✅ Updated documentation with correct architecture

### Final Architecture

```
Browser Post API:
- Use for: Immediate checkout (no payment method storage)
- Example: Customer buys product → charge once → done

Recurring Billing API:
- Use for: Any scenario requiring stored payment methods
- Scenario A: Store method → charge on-demand (variable amounts/timing)
- Scenario B: Store method → automatic billing (fixed recurring charges)
```

---

## Investigation History (Resolved)

<details>
<summary>Click to view original investigation (no longer needed)</summary>

## Investigation Needed

### Scenario 1: North Recurring Billing = Subscriptions Only

**If North's API only supports:**
- Creating subscriptions with recurring schedules
- No "customer vault" or "charge on-demand" endpoints

**Then our current architecture is correct:**
```
One-time payments → Browser Post API
Recurring subscriptions → Recurring Billing API
```

### Scenario 2: North Recurring Billing = Customer Vault + Subscriptions

**If North's API supports:**
- ✅ Store payment method (customer vault)
- ✅ Charge stored method on-demand (one-time)
- ✅ Create subscriptions with stored method

**Then we should extend our architecture:**
```
Immediate one-time → Browser Post API
Stored method + on-demand → Recurring Billing (customer vault)
Stored method + auto-billing → Recurring Billing (subscription)
```

## Questions for North Support

### 1. Customer Vault
```
Q: Does the Recurring Billing API have endpoints to:
   - Store a payment method without creating a subscription?
   - Retrieve stored payment methods?
   - Charge a stored payment method on-demand (one-time)?

Expected endpoints:
   POST /customer
   GET /customer/{id}
   POST /customer/{id}/charge
   DELETE /customer/{id}
```

### 2. One-Time Charges
```
Q: Can we use Recurring Billing API for one-time payments?

Options:
   a) Create customer → charge on-demand
   b) Create subscription with numberOfPayments: 1
   c) Create subscription with frequency: "one-time"
   d) Not supported - use Browser Post instead
```

### 3. Token Reuse
```
Q: Can BRIC tokens from Browser Post be reused?

If YES:
   - Store token in our database
   - Reuse for multiple Browser Post charges
   - No need for customer vault

If NO:
   - Must re-tokenize for each payment
   - Customer vault becomes more important
   - Need recurring billing for stored methods
```

## Recommended Architecture (Pending Answers)

### Option A: If BRIC Tokens Are Reusable + No Customer Vault

```go
// One-time payment (immediate)
result := browserPostAdapter.Authorize(ctx, &AuthorizeRequest{
    Token: "tok_from_frontend",  // Fresh token from frontend
})

// One-time payment (stored method)
storedToken := db.GetCustomerToken(customerID)
result := browserPostAdapter.Authorize(ctx, &AuthorizeRequest{
    Token: storedToken,  // Reuse stored BRIC token
})

// Recurring subscription
subscription := recurringAdapter.CreateSubscription(ctx, &SubscriptionRequest{
    PaymentToken: "tok_from_frontend",
    Frequency: FrequencyMonthly,
})
```

**Pros:**
- Simple architecture
- Only 2 adapters needed
- Can store BRIC tokens for future use

**Cons:**
- Storing tokens in our database (security consideration)
- No built-in customer vault features

### Option B: If Customer Vault Exists

```go
// Store payment method
customer := recurringAdapter.CreateCustomer(ctx, &CustomerRequest{
    Token: "tok_from_frontend",
    Email: "customer@example.com",
})

// One-time charge (on-demand)
result := recurringAdapter.ChargeCustomer(ctx, &ChargeRequest{
    CustomerID: customer.ID,
    Amount: decimal.NewFromFloat(99.99),
})

// Recurring subscription
subscription := recurringAdapter.CreateSubscription(ctx, &SubscriptionRequest{
    CustomerID: customer.ID,  // Use stored method
    Amount: decimal.NewFromFloat(29.99),
    Frequency: FrequencyMonthly,
})
```

**Pros:**
- North manages payment method storage
- Built-in vault security
- Unified API for stored methods

**Cons:**
- More complex adapter
- Additional API endpoints to implement

### Option C: Hybrid (Most Likely)

```go
// Immediate one-time (checkout)
browserPostAdapter.Authorize(...)

// Store method + subscription
customer := recurringAdapter.CreateCustomer(...)
subscription := recurringAdapter.CreateSubscription(customer.ID, ...)

// On-demand charge (if supported)
recurringAdapter.ChargeCustomer(customer.ID, ...)
// OR if not supported:
browserPostAdapter.Authorize(customer.StoredToken, ...)
```

## Implementation Plan

### Step 1: Contact North Support ✅
**Questions to ask:**
1. Full Recurring Billing API documentation
2. Customer vault capabilities
3. BRIC token reuse policy
4. Best practices for one-time vs recurring

### Step 2: Review API Documentation ⏳
**Look for:**
- Customer/vault endpoints
- Charge/payment endpoints (separate from subscriptions)
- Token management
- API examples for different scenarios

### Step 3: Decide Architecture ⏳
Based on answers, choose:
- Option A (reusable tokens)
- Option B (customer vault)
- Option C (hybrid)

### Step 4: Extend Implementation (If Needed) ⏳

If customer vault exists, extend RecurringBillingAdapter:

```go
// internal/adapters/north/recurring_billing_adapter.go

// New methods to add:
func (a *RecurringBillingAdapter) CreateCustomer(ctx context.Context, req *ports.CustomerRequest) (*ports.CustomerResult, error) {
    endpoint := "/customer"
    // Store payment method without subscription
}

func (a *RecurringBillingAdapter) ChargeCustomer(ctx context.Context, customerID string, req *ports.ChargeRequest) (*ports.PaymentResult, error) {
    endpoint := fmt.Sprintf("/customer/%s/charge", customerID)
    // One-time charge to stored method
}

func (a *RecurringBillingAdapter) GetCustomer(ctx context.Context, customerID string) (*ports.CustomerResult, error) {
    endpoint := fmt.Sprintf("/customer/%s", customerID)
    // Get stored payment methods
}

func (a *RecurringBillingAdapter) DeleteCustomer(ctx context.Context, customerID string) error {
    endpoint := fmt.Sprintf("/customer/%s", customerID)
    // Remove stored payment method
}
```

## Security Considerations

### If Storing BRIC Tokens Ourselves
```go
// Encrypt tokens before storage
encrypted := encrypt(bricToken, encryptionKey)
db.SaveToken(customerID, encrypted)

// Decrypt when needed
bricToken := decrypt(encryptedToken, encryptionKey)
```

**Requirements:**
- AES-256-GCM encryption
- Secure key management (Vault/KMS)
- PCI DSS SAQ-D if storing card references
- Regular key rotation

### If Using Customer Vault
```go
// North stores the token
customer := recurringAdapter.CreateCustomer(token)
// We only store customer.ID (not sensitive)
db.SaveCustomerID(userID, customer.ID)

// Later, charge without exposing token
recurringAdapter.ChargeCustomer(customer.ID, amount)
```

**Benefits:**
- North handles token security
- Reduced PCI scope
- Built-in security features

## Current Status

**What we know:**
- ✅ Browser Post works for immediate one-time payments
- ✅ Recurring Billing works for subscriptions
- ❓ Unknown if Recurring Billing supports customer vault
- ❓ Unknown if BRIC tokens are reusable
- ❓ Unknown best practice for stored payment methods

**What we need:**
1. North API documentation review
2. Support team clarification
3. Architecture decision based on capabilities

## Temporary Recommendation

**Until we get answers from North:**

**Use current architecture:**
- Immediate one-time payments → Browser Post ✅
- Recurring subscriptions → Recurring Billing ✅

**Don't try to:**
- Use Recurring Billing for one-time (might work, might not)
- Store BRIC tokens (unknown if allowed)
- Implement customer vault (unknown if exists)

**Contact North for:**
1. Full Recurring Billing API documentation
2. Customer vault capabilities
3. BRIC token reuse policy
4. Recommended architecture for:
   - One-time payments
   - Stored payment methods
   - On-demand charging
   - Recurring subscriptions

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-10-20 | Use Browser Post for one-time | PCI compliant, immediate charging |
| 2025-10-20 | Use Recurring Billing for subscriptions | Automatic scheduling |
| 2025-10-20 | Question raised: customer vault? | User suggested recurring = stored method |
| Pending | Confirm with North | Need official documentation |

## Next Steps (ORIGINAL - NO LONGER NEEDED)

1. ~~**Immediate:** Contact North support with questions above~~ ✅ User provided API documentation
2. ~~**Short-term:** Review full API documentation~~ ✅ Reviewed and implemented
3. ~~**Medium-term:** Extend adapters if customer vault exists~~ ✅ ChargePaymentMethod() added
4. ~~**Long-term:** Implement optimal architecture based on findings~~ ✅ Architecture finalized

</details>

---

**Status:** ✅ RESOLVED

**Resolution Date:** 2025-10-21

**Final Decision:** Use Recurring Billing API for all stored payment method scenarios (both one-time AND recurring)

**Implementation:** ChargePaymentMethod() added to RecurringBillingAdapter
