# Refund Idempotency Pattern

## Overview

This document describes how refund operations achieve idempotency using client-generated transaction IDs. This pattern matches the Browser Post implementation and ensures safe retry behavior without requiring additional infrastructure.

## Pattern: Client-Generated UUID

### How It Works

```
┌─────────┐                    ┌─────────────┐                    ┌──────────┐
│ Client  │                    │   Service   │                    │ Database │
└────┬────┘                    └──────┬──────┘                    └────┬─────┘
     │                                │                                 │
     │ 1. Generate UUID               │                                 │
     ├───────────────────────────────>│                                 │
     │    transaction_id: "uuid-123"  │                                 │
     │    group_id: "sale-456"        │                                 │
     │    amount: "50.00"             │                                 │
     │                                │ 2. INSERT with uuid-123         │
     │                                ├────────────────────────────────>│
     │                                │    ON CONFLICT DO NOTHING       │
     │                                │<────────────────────────────────┤
     │                                │ Transaction created/returned    │
     │<───────────────────────────────┤                                 │
     │ 200 OK: transaction_id         │                                 │
     │                                │                                 │
     │ 3. RETRY (same UUID)           │                                 │
     ├───────────────────────────────>│                                 │
     │    transaction_id: "uuid-123"  │                                 │
     │                                │ 4. INSERT with uuid-123         │
     │                                ├────────────────────────────────>│
     │                                │    ON CONFLICT DO NOTHING       │
     │                                │<────────────────────────────────┤
     │                                │ EXISTING transaction returned   │
     │<───────────────────────────────┤ (no duplicate created)          │
     │ 200 OK: SAME transaction_id    │                                 │
     │                                │                                 │
```

### Key Characteristics

1. **Database-Enforced**: PRIMARY KEY constraint on `transactions.id` prevents duplicates
2. **Automatic**: No application-level idempotency checks needed
3. **Stateless**: No idempotency keys to store or manage
4. **Retry-Safe**: Network retries automatically deduplicated
5. **Consistent**: Same pattern as Browser Post (proven in production)

## API Request Format

### Refund Request

```bash
POST /api/v1/payments/refund
Content-Type: application/json

{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",  // Client-generated UUID
  "group_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",       // Original transaction group
  "amount": "50.00",
  "reason": "Customer requested refund"
}
```

### Response

```json
{
  "transactionId": "550e8400-e29b-41d4-a716-446655440000",  // Same as request
  "groupId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "type": "refund",
  "amount": "50.00",
  "status": "approved",
  "createdAt": "2025-11-12T10:30:00Z"
}
```

## Implementation Details

### Database Schema

```sql
CREATE TABLE transactions (
    id UUID PRIMARY KEY,              -- Client-provided UUID
    group_id UUID NOT NULL,           -- Links related transactions
    merchant_id UUID NOT NULL,
    amount NUMERIC(19, 2) NOT NULL,
    type VARCHAR(20) NOT NULL,        -- 'sale', 'refund', 'void', etc.
    status VARCHAR(20) NOT NULL,
    -- ... other fields
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### SQL Query Pattern

```sql
-- name: CreateTransaction :one
INSERT INTO transactions (
    id,
    group_id,
    merchant_id,
    amount,
    type,
    status,
    -- ... other fields
) VALUES (
    $1, $2, $3, $4, $5, $6  -- $1 = client-provided UUID
)
ON CONFLICT (id) DO NOTHING  -- Idempotency: retry returns existing record
RETURNING *;
```

### Service Layer

```go
func (s *PaymentService) ProcessRefund(ctx context.Context, req *RefundRequest) (*Transaction, error) {
    // Validate transaction_id is a valid UUID
    if err := validateUUID(req.TransactionID); err != nil {
        return nil, fmt.Errorf("invalid transaction_id: %w", err)
    }

    // Insert transaction (ON CONFLICT handles retries)
    tx, err := s.db.CreateTransaction(ctx, sqlc.CreateTransactionParams{
        ID:       req.TransactionID,  // Use client-provided UUID
        GroupID:  req.GroupID,
        Type:     "refund",
        Amount:   req.Amount,
        // ...
    })

    // ON CONFLICT DO NOTHING returns existing record on retry
    return tx, nil
}
```

## Client Implementation Examples

### JavaScript/TypeScript

```typescript
import { v4 as uuidv4 } from 'uuid';

async function refundPayment(groupId: string, amount: string, reason: string) {
  // Generate transaction ID upfront
  const transactionId = uuidv4();

  const request = {
    transaction_id: transactionId,
    group_id: groupId,
    amount: amount,
    reason: reason
  };

  // Safe to retry with same request object
  const response = await fetch('/api/v1/payments/refund', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request)
  });

  return await response.json();
}
```

### Go Client

```go
import "github.com/google/uuid"

func RefundPayment(groupID, amount, reason string) (*Transaction, error) {
    // Generate transaction ID upfront
    transactionID := uuid.New().String()

    req := RefundRequest{
        TransactionID: transactionID,
        GroupID:       groupID,
        Amount:        amount,
        Reason:        reason,
    }

    // Safe to retry with same request
    resp, err := client.Post("/api/v1/payments/refund", req)
    if err != nil {
        return nil, err
    }

    return resp.Transaction, nil
}
```

### Python Client

```python
import uuid
import requests

def refund_payment(group_id: str, amount: str, reason: str) -> dict:
    # Generate transaction ID upfront
    transaction_id = str(uuid.uuid4())

    request = {
        'transaction_id': transaction_id,
        'group_id': group_id,
        'amount': amount,
        'reason': reason
    }

    # Safe to retry with same request
    response = requests.post('/api/v1/payments/refund', json=request)
    return response.json()
```

## Retry Scenarios

### Network Timeout

```
Client                              Service
  │                                    │
  ├─ POST refund (uuid-123) ────────> │
  │                                    ├─ Processing...
  │                                    │  (Network timeout)
  X                                    │
  │                                    ├─ Transaction created ✓
  │                                    │
  ├─ POST refund (uuid-123) ────────> │
  │  (RETRY with same UUID)            ├─ ON CONFLICT DO NOTHING
  │                                    │  Returns existing record
  │<─ 200 OK (same transaction) ────  │
```

### Concurrent Retries

```
Client A                    Client B                    Service
  │                           │                            │
  ├─ POST (uuid-123) ─────────┼──────────────────────────> │
  │                           │                            ├─ INSERT
  │                           ├─ POST (uuid-123) ────────> │  (race)
  │                           │  (concurrent retry)        ├─ CONFLICT
  │<─ 200 OK ─────────────────┼────────────────────────────┤  First wins
  │                           │<─ 200 OK (same tx) ────────┤  Second gets existing
```

## Benefits

### 1. **Simplicity**
- No separate idempotency key storage
- No key expiration management
- No complex distributed locking

### 2. **Reliability**
- Database constraint enforcement (atomic)
- Impossible to create duplicates
- Works across service restarts

### 3. **Performance**
- Single database operation
- No additional lookups
- Minimal overhead

### 4. **Consistency**
- Same pattern as Browser Post
- Familiar to developers
- Proven in production

## Comparison with Alternative Approaches

### Idempotency-Key Header

**Alternative:**
```bash
POST /api/v1/payments/refund
Idempotency-Key: some-unique-key

{
  "group_id": "...",
  "amount": "50.00"
}
```

**Drawbacks:**
- Requires separate idempotency key storage (Redis/database)
- Key expiration management (how long to keep?)
- Additional lookups on every request
- More complex implementation

### Server-Generated UUIDs

**Alternative:**
```go
// Server generates UUID
transactionID := uuid.New()
```

**Drawbacks:**
- No idempotency - retries create duplicates
- Client has no way to identify retries
- Requires additional deduplication logic

## Best Practices

### ✅ DO

- Generate UUID on client side before making request
- Store UUID locally before sending request
- Retry with same UUID on network errors
- Use different UUIDs for different refund operations

### ❌ DON'T

- Generate new UUID on retry (defeats idempotency)
- Use same UUID for multiple different refunds
- Assume server will generate UUID for you
- Implement custom idempotency key logic

## Testing

### Integration Test Example

```go
func TestRefund_Idempotency(t *testing.T) {
    // Client generates UUID
    refundTxID := uuid.New().String()

    request := map[string]interface{}{
        "transaction_id": refundTxID,
        "group_id": saleGroupID,
        "amount": "50.00",
        "reason": "Test refund",
    }

    // First request
    resp1, err := client.Do("POST", "/api/v1/payments/refund", request)
    require.NoError(t, err)
    assert.Equal(t, 200, resp1.StatusCode)

    var result1 map[string]interface{}
    json.NewDecoder(resp1.Body).Decode(&result1)
    firstTxID := result1["transactionId"].(string)

    // RETRY with same UUID
    resp2, err := client.Do("POST", "/api/v1/payments/refund", request)
    require.NoError(t, err)
    assert.Equal(t, 200, resp2.StatusCode)

    var result2 map[string]interface{}
    json.NewDecoder(resp2.Body).Decode(&result2)
    secondTxID := result2["transactionId"].(string)

    // Verify: Same transaction returned (idempotent)
    assert.Equal(t, firstTxID, secondTxID)
    assert.Equal(t, refundTxID, firstTxID)
}
```

## Security Considerations

### UUID Predictability

Client-generated UUIDs are **safe** because:
1. UUIDs are 128-bit random values (UUID v4)
2. Collision probability is negligible (2^128 space)
3. Cannot be used to access other transactions (requires correct group_id and authorization)

### Authorization

The service **must** still validate:
- User has access to the group_id being refunded
- Merchant credentials match original transaction
- Amount doesn't exceed available refundable balance

UUID-based idempotency does not bypass authorization checks.

## Monitoring

### Key Metrics

- **Conflict Rate**: `COUNT(*) WHERE conflict_detected` / total requests
  - High rate indicates legitimate retries working
- **Duplicate Prevention**: Should be 0% duplicate transactions
- **Retry Success Rate**: % of retries returning 200 OK with existing transaction

### Logging

```go
if conflictDetected {
    logger.Info("idempotent retry detected",
        zap.String("transaction_id", txID),
        zap.String("group_id", groupID),
        zap.String("operation", "refund"),
    )
}
```

## References

- Browser Post Implementation: `internal/handlers/payment/browser_post_callback_handler.go`
- Database Queries: `internal/db/queries/transactions.sql`
- Integration Tests: `tests/integration/payment/idempotency_test.go`
- PostgreSQL ON CONFLICT: https://www.postgresql.org/docs/current/sql-insert.html
