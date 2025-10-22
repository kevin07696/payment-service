# Chargeback & Settlement Management Guide

## Is Chargeback Management Necessary?

### Short Answer: **YES** (for most businesses)

### When Chargebacks Are CRITICAL:

| Business Type | Priority | Why |
|---------------|----------|-----|
| **E-commerce** | =4 Critical | High fraud risk, card-not-present transactions |
| **Subscription Services** | =4 Critical | Recurring billing disputes, "forgot to cancel" |
| **Digital Goods** | =á Medium | Lower chargeback rates but still need tracking |
| **High-Ticket Items** | =4 Critical | Each chargeback = significant revenue loss |
| **B2B Payments** | =â Low | Lower dispute rates but still possible |

### The Cost of Ignoring Chargebacks

**Direct Costs:**
- Lost revenue (transaction amount)
- Chargeback fee ($15-$100 per chargeback)
- Product/service already delivered (can't get it back)

**Indirect Costs:**
- **Chargeback Ratio Penalties**: >1% ratio = monitoring program
- **Account Termination**: Visa/Mastercard can shut down your merchant account
- **Higher Processing Fees**: Processors charge more for high-risk merchants
- **Brand Damage**: Poor chargeback management signals poor business practices

### Industry Thresholds (You MUST Stay Below These)

| Network | Warning Threshold | Critical Threshold |
|---------|-------------------|-------------------|
| **Visa** | 0.65% | 1.00% |
| **Mastercard** | 1.00% | 1.50% |
| **Discover** | 1.00% | 1.50% |

**Example:** If you process 10,000 transactions/month:
- 65 chargebacks = Warning program
- 100 chargebacks = Excessive chargeback program (fees + scrutiny)
- 150+ chargebacks = Account termination risk

## Chargeback Implementation Guide

### Phase 1: Basic Tracking (Essential)

**What to Track:**
```go
type Chargeback struct {
    ID                 string
    TransactionID      string
    MerchantID         string
    CustomerID         string
    Amount             decimal.Decimal
    Currency           string
    ReasonCode         string // e.g., "10.4" = Fraud
    ReasonDescription  string
    Status             ChargebackStatus
    ReceivedDate       time.Time
    RespondByDate      time.Time
    ResponseSubmitted  *time.Time
    Outcome            *ChargebackOutcome
    EvidenceFiles      []string
    Notes              string
    CreatedAt          time.Time
    UpdatedAt          time.Time
}

type ChargebackStatus string
const (
    ChargebackPending    ChargebackStatus = "pending"
    ChargebackResponded  ChargebackStatus = "responded"
    ChargebackWon        ChargebackStatus = "won"
    ChargebackLost       ChargebackStatus = "lost"
    ChargebackAccepted   ChargebackStatus = "accepted" // Not contesting
)

type ChargebackOutcome string
const (
    OutcomeReversed     ChargebackOutcome = "reversed"     // You won
    OutcomeUpheld       ChargebackOutcome = "upheld"       // Customer won
    OutcomePartial      ChargebackOutcome = "partial"      // Split decision
)
```

**Minimum Viable Features:**
1.  Receive chargeback notifications from North
2.  Store chargeback data in database
3.  Link to original transaction
4.  Track response deadlines
5.  Alert team when chargebacks arrive

### Phase 2: Response Management (Important)

**Reason Codes You'll See:**

| Code | Category | Meaning | Win Rate |
|------|----------|---------|----------|
| **10.4** | Fraud | Card-absent fraud | 20-30% |
| **13.1** | Authorization | Merchant not authorized | 40-50% |
| **13.3** | Authorization | Not as described | 30-40% |
| **13.5** | Authorization | Merchandise not received | 35-45% |
| **13.6** | Authorization | Credit not processed | 50-60% |
| **13.7** | Authorization | Cancelled recurring | 25-35% |
| **4837** | Fraud | No cardholder authorization | 20-30% |

**Evidence You Need to Collect:**

For **Fraud** (10.4):
- AVS match results  (already captured)
- CVV match results  (already captured)
- IP address of customer
- Device fingerprint
- Delivery confirmation
- Previous successful transactions

For **Service/Product Not Received** (13.5):
- Tracking number + proof of delivery
- Signed receipts
- Service completion records
- Communication logs with customer

For **Subscription Disputes** (13.7):
- Original signup agreement
- Terms & conditions acceptance
- Email confirmation of signup
- Cancellation policy
- Proof customer was notified of renewal

### Phase 3: Automation (Optimal)

**Automated Responses:**
```go
type ChargebackRule struct {
    ReasonCode      string
    AutoRespond     bool
    EvideceTemplate string
    WinThreshold    float64 // Only auto-respond if win rate > X%
}

// Example: Auto-respond to "Credit Not Processed" with refund evidence
func (s *Service) AutoRespondChargeback(ctx context.Context, cb *Chargeback) error {
    // If we already issued a refund, automatically provide evidence
    refund, err := s.findRelatedRefund(ctx, cb.TransactionID)
    if err == nil && refund != nil {
        evidence := s.buildRefundEvidence(refund)
        return s.submitChargebackResponse(ctx, cb.ID, evidence)
    }
    return nil // Manual review required
}
```

**Prevent Chargebacks Proactively:**
```go
// Check transaction for fraud signals before processing
func (s *Service) FraudCheck(ctx context.Context, txn *Transaction) (*FraudScore, error) {
    score := &FraudScore{TransactionID: txn.ID}

    // AVS mismatch = higher risk
    if txn.AVSResponse == "N" {
        score.AddFlag("AVS_NO_MATCH", 30)
    }

    // CVV mismatch = higher risk
    if txn.CVVResponse == "N" {
        score.AddFlag("CVV_NO_MATCH", 40)
    }

    // High amount from new customer = risk
    if txn.Amount.GreaterThan(decimal.NewFromInt(500)) {
        score.AddFlag("HIGH_AMOUNT_NEW_CUSTOMER", 25)
    }

    if score.Total > 70 {
        score.Recommendation = "DECLINE"
    }

    return score, nil
}
```

## Settlement Reports - ESSENTIAL for Accounting

### Why Settlement Reports Matter

**Settlement ` Transaction**

```
Customer pays: $100 (Jan 1)
    “
Transaction approved: $100 (Jan 1)
    “
Processing fee deducted: -$2.90 (2.9%)
    “
Money hits your bank: $97.10 (Jan 3)  SETTLEMENT
```

**Without Settlement Reports:**
- L Revenue doesn't match bank deposits
- L Accounting reconciliation nightmare
- L Can't detect settlement discrepancies
- L Don't know actual processing costs
- L Tax reporting is incorrect

### What Settlement Reports Include

```go
type SettlementReport struct {
    ID               string
    SettlementDate   time.Time
    DepositDate      time.Time
    MerchantID       string
    TotalSales       decimal.Decimal
    TotalRefunds     decimal.Decimal
    TotalChargebacks decimal.Decimal
    ProcessingFees   decimal.Decimal
    NetAmount        decimal.Decimal // What actually deposited
    TransactionCount int32
    Transactions     []*SettlementTransaction
    CreatedAt        time.Time
}

type SettlementTransaction struct {
    TransactionID      string
    TransactionDate    time.Time
    SettlementDate     time.Time
    Amount             decimal.Decimal
    Fee                decimal.Decimal
    NetAmount          decimal.Decimal
    Type               string // "SALE", "REFUND", "CHARGEBACK"
    CardBrand          string
    InterchangeRate    decimal.Decimal
}
```

### Settlement Report Implementation

**Phase 1: Daily Reconciliation (Minimum)**

```go
func (s *Service) ReconcileSettlements(ctx context.Context, date time.Time) (*ReconciliationReport, error) {
    // 1. Get all transactions for the date
    transactions, err := s.txRepo.ListByDateRange(ctx, date, date.AddHours(24))

    // 2. Get settlement report from North
    settlement, err := s.gateway.GetSettlementReport(ctx, date)

    // 3. Compare
    report := &ReconciliationReport{
        Date: date,
        ExpectedAmount: sumTransactions(transactions),
        SettledAmount:  settlement.NetAmount,
    }

    // 4. Flag discrepancies
    if !report.ExpectedAmount.Equal(report.SettledAmount) {
        report.Discrepancy = report.ExpectedAmount.Sub(report.SettledAmount)
        report.Status = "MISMATCH"
        s.alertAccounting(report)
    }

    return report, nil
}
```

**Phase 2: Automated Accounting Integration**

```go
// Export to QuickBooks, Xero, etc.
func (s *Service) ExportToAccounting(ctx context.Context, settlement *SettlementReport) error {
    // Create journal entries
    entries := []JournalEntry{
        {
            Account: "Accounts Receivable",
            Debit:   settlement.TotalSales,
        },
        {
            Account: "Processing Fees Expense",
            Debit:   settlement.ProcessingFees,
        },
        {
            Account: "Cash",
            Credit:  settlement.NetAmount,
        },
    }

    return s.accountingSystem.CreateJournalEntries(ctx, entries)
}
```

### How to Get Settlement Reports from North

**API Endpoint (you'd need to implement):**
```go
// North likely has a settlement reporting API
type NorthSettlementAPI interface {
    GetSettlementReport(ctx context.Context, date time.Time) (*SettlementReport, error)
    GetSettlementBatch(ctx context.Context, batchID string) (*SettlementBatch, error)
}

// Or they might provide SFTP file drops
func (s *Service) DownloadSettlementFile(ctx context.Context, date time.Time) error {
    // Connect to North's SFTP
    // Download settlement CSV/XML file
    // Parse and import to database
}
```

## Implementation Priority

### Must-Have (Implement Now):

1.  **ListTransactions** - DONE!
2. =4 **Settlement Reconciliation** - Critical for accounting
3. =4 **Basic Chargeback Tracking** - Database + notifications

### Should-Have (Next Quarter):

4. =á **Chargeback Response Management** - Evidence submission
5. =á **Settlement Report Exports** - Accounting integration

### Nice-to-Have (Future):

6. =â **Automated Chargeback Prevention** - Fraud scoring
7. =â **Chargeback Analytics** - Trend analysis
8. =â **Real-time Settlement Tracking** - Daily reconciliation

## Questions to Ask North Payment Gateway

1. **Chargebacks:**
   - How are chargebacks communicated? (Webhook, email, API?)
   - What's the response deadline?
   - What evidence formats do you accept?
   - Do you provide representment services?

2. **Settlements:**
   - Where can I download settlement reports?
   - What format are they in? (CSV, XML, JSON?)
   - How often do settlements occur? (Daily, weekly?)
   - Is there an API for settlement data?
   - What's the settlement delay? (T+1, T+2, T+3 days?)

3. **Fees:**
   - Are processing fees in the settlement report?
   - Do you report interchange fees separately?
   - How are chargebacks reflected in settlements?

## Summary

| Feature | Necessity | Timeline | Impact |
|---------|-----------|----------|--------|
| **ListTransactions** |  Essential | DONE | Revenue tracking |
| **Chargeback Tracking** | =4 Critical | Week 1 | Account protection |
| **Settlement Reports** | =4 Critical | Week 2 | Accounting accuracy |
| **Chargeback Response** | =á Important | Month 1 | Revenue recovery |
| **Fraud Prevention** | =á Important | Month 2 | Chargeback reduction |

**Bottom Line:**
-  **ListTransactions** - Implemented!
- =4 **Chargebacks** - Necessary if processing >$10k/month
- =4 **Settlement Reports** - Necessary for ANY payment business (accounting requirement)

Start with basic tracking, then add automation as your volume grows.
