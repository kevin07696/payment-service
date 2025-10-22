# Business Reporting API Analysis

## Overview

North provides a **Business Reporting API** that offers transaction listing, refund/void capabilities, and potentially chargeback data. This document analyzes whether we should use it for `ListTransactions` and chargeback tracking.

## API Endpoints

### 1. GET /accounts/{accountId}/transactions
**Purpose**: List transactions by date range

**Query Parameters**:
- `start`: Start date (format: `2017-07-19` or `2017-07-19 13:30:00`)
- `end`: End date (format: `2017-07-19` or `2017-07-19 13:30:00`)
- `count`: Maximum items to return
- `type`: `open` or `closed` (for batch transactions)
- `deviceId`: Required for manual batch transactions
- `bids`: Comma-separated batch IDs
- `onlyLastClosed`: Boolean for most recent closed batch

**Response Data**:
```json
{
  "transactions": [
    {
      "uniq_id": "cas_15218",
      "id": 15218,
      "type": "Cash Sale",
      "amount": "47.70",
      "datetime": "2017-07-20T13:28:19.000Z",
      "mid": "9999999999999",
      "txn_source": "Phone Swipe iOS",
      "cc_type": null,
      "cc_last4": null,
      "cc_auth_code": null,
      "parent_uniq_id": null,     // Links to parent transaction
      "parent_type": null,
      "void_uniq_id": null,
      "is_reversal": false,       // 🚨 MIGHT INDICATE CHARGEBACKS
      "is_complete": true,
      "receipt_id": 101742,
      // ... extensive item-level details
    }
  ]
}
```

### 2. POST /accounts/{accountId}/transactions
**Purpose**: Refund or void a transaction

**Request**:
```json
{
  "type": "void",              // or "refund"
  "transaction_id": 87654321,  // For voids
  "ccs_pk": 87654321,          // For refunds
  "amount": "2",               // Refund amount (optional, partial refund)
  "comment": "some comment",
  "username": "user@example.com",
  "transaction_source": "PA-iOS-SDK|5.10.0-x86_64-12.2"
}
```

### 3. GET /accounts/{accountId}/transactions/{transactionId}
**Purpose**: Get single transaction by ID

## Comparison: Business Reporting API vs Our Current Implementation

### Our Current Implementation (✅ Recommended)

**Architecture**:
```
gRPC Request → PaymentService → TransactionRepository → PostgreSQL
                                                          ↓
                                                    Query local DB
                                                    Return results
```

**Advantages**:
- ⚡ **Fast**: < 10ms database query vs 100-500ms+ API call
- 💰 **Free**: No API rate limits or additional costs
- 🔒 **Reliable**: No dependency on North's API availability
- 🏗️ **Clean Architecture**: Follows hexagonal pattern
- ✅ **Already Complete**: Fully implemented and tested
- 📊 **Full Control**: We define the data model and queries

**Our Implementation**:
```go
// internal/api/grpc/payment/payment_handler.go:231-266
func (h *Handler) ListTransactions(ctx context.Context, req *paymentv1.ListTransactionsRequest) (*paymentv1.ListTransactionsResponse, error) {
    // Validates merchant_id
    // Calls PaymentService.ListTransactions
    // Returns paginated transactions from our database
}

// internal/services/payment/payment_service.go:598-643
func (s *Service) ListTransactions(ctx context.Context, req ports.ServiceListTransactionsRequest) (*ports.ServiceListTransactionsResponse, error) {
    // Default limit: 100, max: 500
    // Queries by merchant or customer
    // Returns transactions from our PostgreSQL database
}
```

### Business Reporting API Approach (❌ Not Recommended for ListTransactions)

**Architecture**:
```
gRPC Request → PaymentService → North Business Reporting API
                                  ↓
                            1. Authenticate (JWT)
                            2. GET /accounts/{accountId}/transactions
                            3. Transform response
                            4. Return results
```

**Disadvantages for ListTransactions**:
- 🐌 **Slow**: 2 API calls (auth + list) with network latency
- 💸 **Costs**: Subject to API rate limits
- 🔌 **External Dependency**: Reliant on North's API uptime
- 🔄 **Different Auth**: JWT-based (not HMAC like our other APIs)
- 🏗️ **Complexity**: Requires new authentication flow
- 📊 **Data Model Mismatch**: Receipt/item fields we don't need

**Authentication Required**:
```
1. POST /auth endpoint → Get JWT token
2. Use JWT in Authorization header for subsequent requests
3. Token expires (need refresh logic)
```

## Use Cases: Where Business Reporting API MIGHT Be Valuable

### 1. Chargeback Detection (❓ Need to Verify)

**Hypothesis**: The `is_reversal` field might indicate chargebacks

**Questions for North Support**:
- Does `is_reversal=true` mean chargeback?
- What does the `type` field show for chargebacks? (e.g., "Chargeback")
- Are chargebacks linked via `parent_uniq_id` to original transaction?
- Does this API include chargeback reason codes?
- When do chargebacks appear in this API (immediately or after settlement)?

**Potential Implementation**:
```go
// Poll Business Reporting API for chargebacks
func (s *ChargebackService) SyncChargebacks(ctx context.Context) error {
    // 1. Authenticate to get JWT
    jwt, err := s.northAuth.GetJWT(ctx)

    // 2. List transactions with is_reversal=true (if filterable)
    transactions, err := s.northAPI.ListTransactions(ctx, jwt, ListFilter{
        Start: yesterday,
        End:   today,
    })

    // 3. Filter for chargebacks
    for _, txn := range transactions {
        if txn.IsReversal {
            // Create chargeback record in our database
            // Link to original transaction via parent_uniq_id
        }
    }
}
```

### 2. Settlement Reconciliation (❓ Need to Verify)

**Hypothesis**: Might provide transaction-level settlement data

**Questions for North Support**:
- Can we filter by settlement date?
- Are processing fees included in the transaction data?
- Is this transaction-level only, or are there batch summaries?
- How do we identify which settlement batch a transaction belongs to?

**Potential Implementation**:
```go
// Reconcile settlements using Business Reporting API
func (s *SettlementService) ReconcileDaily(ctx context.Context, date time.Time) error {
    // 1. Get transactions from Business Reporting API
    apiTransactions := s.northAPI.ListTransactions(ctx, jwt, ListFilter{
        Start: date,
        End:   date.Add(24 * time.Hour),
    })

    // 2. Get our transactions from database
    ourTransactions := s.txRepo.ListByDateRange(ctx, nil, date, date.Add(24*time.Hour))

    // 3. Compare and flag discrepancies
    discrepancies := s.compare(apiTransactions, ourTransactions)

    // 4. Alert if mismatch
    if len(discrepancies) > 0 {
        s.alertTeam(discrepancies)
    }
}
```

### 3. Data Verification (✅ Good Use Case)

**Use Business Reporting API for periodic data verification**:

```go
// Run nightly reconciliation job
func (s *DataVerificationService) NightlyReconciliation(ctx context.Context) error {
    yesterday := time.Now().AddDate(0, 0, -1)

    // Our transactions
    ourCount := s.txRepo.CountByDate(ctx, nil, yesterday)

    // North's transactions
    northTxns := s.northAPI.ListTransactions(ctx, jwt, ListFilter{
        Start: yesterday,
        End:   yesterday.Add(24 * time.Hour),
    })

    // Compare counts
    if ourCount != len(northTxns) {
        s.logger.Error("Transaction count mismatch",
            ports.Int("our_count", ourCount),
            ports.Int("north_count", len(northTxns)))
        s.alertTeam()
    }
}
```

## Recommendation

### ✅ Keep Current Implementation for ListTransactions

**Reasons**:
1. **Performance**: Direct database access is 10-50x faster
2. **Reliability**: No external API dependency
3. **Architecture**: Clean hexagonal design
4. **Already Complete**: Tested and working
5. **Cost**: Free (no API rate limits)

### ❓ Investigate Business Reporting API for Chargebacks

**Action Items**:
1. Contact North support (use updated template in `IMPLEMENTATION_CHECKLIST.md`)
2. Ask specific questions about `is_reversal` field
3. Get sample responses with chargebacks
4. Determine if this is better than webhook/polling approach

### ✅ Consider Business Reporting API for Data Verification

**Good Use Case**:
- Nightly reconciliation job
- Compare our transaction count vs North's count
- Flag discrepancies for investigation
- Secondary data source for auditing

## Implementation Priority

### Phase 1: Current (✅ Complete)
- ListTransactions using local database
- Fast, reliable, clean architecture

### Phase 2: Chargeback Integration (⏳ Pending North Response)
- If Business Reporting API includes chargebacks → Implement polling service
- If webhook available → Implement webhook handler
- Either way → Use our chargeback database schema

### Phase 3: Data Verification (Optional)
- Implement nightly reconciliation job
- Use Business Reporting API as verification source
- Alert on discrepancies

## Authentication Flow (If We Need Business Reporting API)

**JWT Authentication**:
```go
// North JWT authentication (different from HMAC)
type BusinessReportingAuth struct {
    username string
    password string
    accountId string
}

func (a *BusinessReportingAuth) GetJWT(ctx context.Context) (string, error) {
    // POST /auth endpoint
    // Returns JWT token
    // Token has expiration
    // Need refresh logic
}

// Use JWT in requests
func (c *BusinessReportingClient) ListTransactions(ctx context.Context, jwt string, filter ListFilter) ([]*Transaction, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET",
        fmt.Sprintf("/accounts/%s/transactions", c.accountId), nil)

    // JWT in Authorization header (not HMAC signature)
    req.Header.Set("Authorization", "Bearer " + jwt)

    // Add query parameters
    q := req.URL.Query()
    q.Add("start", filter.Start.Format("2006-01-02 15:04:05"))
    q.Add("end", filter.End.Format("2006-01-02 15:04:05"))
    req.URL.RawQuery = q.Encode()

    // Execute request...
}
```

## Questions to Ask North Support

### Critical Questions:

1. **Chargebacks in Business Reporting API**:
   - Does `GET /accounts/{accountId}/transactions` include chargebacks?
   - What does `is_reversal=true` indicate?
   - What `type` value represents a chargeback?
   - Are chargebacks linked to original transactions via `parent_uniq_id`?
   - When do chargebacks appear (real-time or delayed)?

2. **Settlement Data**:
   - Does this API provide settlement batch information?
   - Can we filter by settlement date?
   - Are processing fees included?
   - How do we identify settlement batches?

3. **Authentication**:
   - How do we get JWT credentials?
   - Token expiration time?
   - Rate limits for this API?

4. **Alternative Approaches**:
   - Is there a dedicated chargeback webhook/API?
   - Is there a dedicated settlement report API/SFTP?
   - Which approach do you recommend?

## Conclusion

**For ListTransactions**: ✅ **Keep our current implementation** (database query)

**For Chargebacks**: ❓ **Need North's clarification** on whether Business Reporting API includes chargeback data

**For Settlement Reports**: ❓ **Need North's clarification** on whether this provides settlement batch data

**For Data Verification**: ✅ **Good secondary use case** for reconciliation jobs

---

**Next Steps**:
1. Send updated email template to North support (includes Business Reporting API questions)
2. Wait for response about `is_reversal` field and chargeback data
3. Decide on chargeback integration approach based on their response
4. Keep our current ListTransactions implementation (it's already optimal)
