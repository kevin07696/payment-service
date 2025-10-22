# Settlements vs Refunds - Key Differences

## TL;DR

**Settlements** = When North deposits money into YOUR bank account (accounting/reconciliation)
**Refunds** = When you return money to a CUSTOMER (transaction reversal)

These are **completely different** business processes!

---

## What is a Settlement?

**Settlement** is the process where the payment processor (North) moves money from their holding account to your merchant bank account.

### Settlement Flow Example:

```
Day 1 (Monday):
  Customer pays: $100.00
  Transaction approved ✅
  You ship product
  Money sits in North's holding account

Day 2 (Tuesday):
  Money still in North's account
  Processing fees calculated: $2.90 (2.9%)

Day 3 (Wednesday):
  North deposits to your bank: $97.10  ← THIS IS SETTLEMENT
  Your bank shows: +$97.10
```

### What Settlement Reports Show:

```
Settlement Report - March 15, 2025
─────────────────────────────────────────
Sales (100 transactions):        $10,000.00
Refunds (5 transactions):          -$500.00
Chargebacks (2 transactions):      -$200.00
Processing Fees:                   -$290.00
─────────────────────────────────────────
NET DEPOSITED TO YOUR BANK:       $9,010.00
```

**Key Point**: This is about **when you actually receive money**, not about customer transactions.

---

## What is a Refund?

**Refund** is when you return money to a customer for a previously completed transaction.

### Refund Flow Example:

```
Day 1 (Monday):
  Customer pays: $100.00
  Transaction approved ✅
  You ship product

Day 5 (Friday):
  Customer returns product
  You issue refund: $100.00  ← THIS IS A REFUND
  North sends $100 back to customer's card

Your Account Impact:
  Revenue: -$100.00
  Fee charged: -$2.90 (you still pay the processing fee!)
  Total cost to you: -$102.90
```

### Refund API Call:

```go
// This is what YOU do in your code
resp, err := paymentService.Refund(ctx, ports.ServiceRefundRequest{
    TransactionID: "txn_123",
    Amount:        decimal.NewFromFloat(100.00),
    Reason:        "Customer returned product",
})
```

**Key Point**: This is a **business decision you make** to return money to a customer.

---

## Side-by-Side Comparison

| Aspect | Settlement | Refund |
|--------|-----------|--------|
| **What is it?** | Money moving FROM North TO your bank | Money moving FROM you TO customer |
| **Who initiates?** | North (automatic, scheduled) | You (manual decision) |
| **Frequency** | Daily (usually T+1, T+2, or T+3 days) | Whenever you choose |
| **Purpose** | Accounting reconciliation | Customer service / returns |
| **Your control** | ❌ No control over timing | ✅ You decide when/if to refund |
| **Affects customer?** | ❌ No (they already paid) | ✅ Yes (they get money back) |
| **API endpoint** | North reports TO you | You call North's refund API |
| **Shows in reports** | Settlement reports | Transaction history |

---

## Real-World Scenario

Let's walk through a complete example to see how they interact:

### Monday - Sales Day

**Transactions**:
- 10 customers buy products: $100 each = $1,000 total
- All transactions approved ✅
- Money is in North's holding account

**Your bank account**: No change yet (money hasn't settled)

---

### Tuesday - Settlement Day 1

**What North does**:
- Calculates yesterday's transactions
- Deducts processing fees: $1,000 × 2.9% = $29
- Deposits to your bank: **$971**

**Settlement Report**:
```
Date: March 15, 2025
Sales: $1,000.00
Fees:    -$29.00
Net:     $971.00 ✅ DEPOSITED
```

**Your bank account**: +$971.00

---

### Wednesday - Customer Returns Product

**You decide to refund**:
- Customer #5 returns their product
- You issue refund: $100

**Refund API Call**:
```go
paymentService.Refund(ctx, ports.ServiceRefundRequest{
    TransactionID: "txn_customer_5",
    Amount:        decimal.NewFromFloat(100.00),
    Reason:        "Product return",
})
```

**What happens**:
- North sends $100 back to customer's card
- North debits YOUR account: $100
- You still paid the original processing fee ($2.90)

**Your account impact**: -$100.00

---

### Thursday - Settlement Day 2

**What North does**:
- Processes yesterday's transactions
- Yesterday's sales: $800
- Yesterday's refunds: -$100
- Processing fees: $800 × 2.9% = $23.20
- Deposits to your bank: **$676.80**

**Settlement Report**:
```
Date: March 16, 2025
Sales:   $800.00
Refunds: -$100.00
Fees:     -$23.20
Net:     $676.80 ✅ DEPOSITED
```

**Your bank account**: +$676.80

---

## Why Settlement Reports Matter

### Problem: Revenue ≠ Bank Deposits

```
Your System Says:
  Total Sales: $10,000

Your Bank Shows:
  Deposits: $9,010

Why the difference?
  - Processing fees: -$290
  - Refunds: -$500
  - Chargebacks: -$200
```

**Without settlement reconciliation**, you can't explain this discrepancy!

### What Settlement Reports Tell You:

1. **Actual Cash Flow**: What money actually hit your bank
2. **Processing Costs**: Real fees charged by North
3. **Chargeback Deductions**: Money taken back for disputes
4. **Refund Timing**: When refunds were processed
5. **Reconciliation**: Match your records to bank deposits

---

## Settlement Report Use Cases

### 1. Accounting Reconciliation

**Accountant asks**: "Why did we only receive $9,010 when sales were $10,000?"

**You answer with settlement report**:
- Sales: $10,000
- Refunds: -$500
- Chargebacks: -$200
- Fees: -$290
- = $9,010 ✅

### 2. Tax Reporting

**IRS needs**: Actual revenue after refunds and chargebacks

**Settlement reports provide**: Net revenue figures

### 3. Cash Flow Management

**CFO asks**: "How much will we receive this week?"

**Settlement reports show**: Pending settlements and timing (T+1, T+2, T+3)

### 4. Detect Processing Issues

**Settlement report shows**: $8,000 expected, only $7,500 deposited

**Investigation finds**: North held back transactions for review

---

## What You've Already Implemented ✅

### Refunds - DONE!

```go
// internal/services/payment/payment_service.go
func (s *Service) Refund(ctx context.Context, req ports.ServiceRefundRequest) (*ports.PaymentResponse, error) {
    // ✅ Creates refund transaction in your database
    // ✅ Calls North API to process refund
    // ✅ Links refund to original transaction
    // ✅ Updates statuses
}
```

**You can already**:
- Issue full refunds
- Issue partial refunds
- Track refund reasons
- See refund history via ListTransactions

---

## What Settlements Would Add 🔧

### Settlement Tracking - NOT YET IMPLEMENTED

**What it would do**:
- Download daily settlement reports from North
- Import into `settlement_batches` and `settlement_transactions` tables
- Compare expected revenue vs actual deposits
- Alert on discrepancies
- Provide reconciliation reports for accounting

**Example Query After Implementation**:
```sql
-- Show settlement for March 15, 2025
SELECT
    total_sales,
    total_refunds,
    total_chargebacks,
    total_fees,
    net_amount  -- What was actually deposited
FROM settlement_batches
WHERE settlement_date = '2025-03-15';
```

**Reconciliation Check**:
```go
// Compare our transactions vs settlement report
func (s *SettlementService) ReconcileDaily(ctx context.Context, date time.Time) error {
    // 1. Sum all our transactions for the date
    ourTotal := s.sumTransactions(date)

    // 2. Get settlement report from North
    settlement := s.getSettlementReport(date)

    // 3. Compare
    if ourTotal != settlement.NetAmount {
        s.alertDiscrepancy(ourTotal, settlement.NetAmount)
    }
}
```

---

## Do You Need Settlement Reconciliation?

### ✅ YES if:

- You process >$10k/month (accounting requirement)
- You have refunds or chargebacks
- You need to reconcile bank deposits
- Your accountant asks "where's the money?"
- You need accurate tax reporting
- You want to catch payment processor errors

### ⚠️ MAYBE if:

- You process <$1k/month (low volume)
- You manually reconcile in spreadsheets
- You have simple financials

### ❌ NO if:

- You're just testing/prototyping
- You don't care about accounting accuracy
- You trust North 100% (not recommended!)

---

## Summary Table

| Feature | Refunds | Settlements |
|---------|---------|-------------|
| **Status** | ✅ Implemented | ⏳ Infrastructure ready |
| **Purpose** | Return money to customers | Track bank deposits |
| **Frequency** | As needed (manual) | Daily (automatic from North) |
| **Database** | `transactions` table | `settlement_batches` + `settlement_transactions` |
| **API You Call** | `/refund` endpoint | `/settlements` endpoint (or SFTP) |
| **Who Sees It** | Customer (gets refund) | Accountant (reconciliation) |
| **Example Amount** | -$100 to customer | +$9,010 to your bank |

---

## Visual Flow Comparison

### Refund Flow:
```
Customer: "I want to return this product"
    ↓
You: "OK, I'll refund you"
    ↓
You call: paymentService.Refund()
    ↓
North: Sends $100 to customer's card
    ↓
Your account: -$100
    ↓
Customer: Receives $100 ✅
```

### Settlement Flow:
```
Many customers buy products (Monday)
    ↓
North: Holds money (Monday-Tuesday)
    ↓
North: Calculates fees (Tuesday)
    ↓
North: Deposits net amount (Wednesday)
    ↓
Your bank account: +$9,010 ✅
    ↓
You: Download settlement report
    ↓
Accountant: Reconciles books ✅
```

---

## Bottom Line

**Refunds** = Customer service operation (you already have this! ✅)

**Settlements** = Accounting operation (infrastructure ready, waiting on North API details ⏳)

**Both are important**, but they serve completely different purposes!

**Next Question**: Should we implement settlement reconciliation?

**Answer**: If you process any significant volume (>$10k/month) or need accurate accounting, **YES!**
