# Quick Reference Guide

## Common Confusion: Settlements vs Refunds

**IMPORTANT**: These are NOT the same thing!

| | Settlements | Refunds |
|-|-------------|---------|
| **What** | North deposits money to YOUR bank | You return money to CUSTOMER |
| **Purpose** | Accounting/reconciliation | Customer service |
| **Frequency** | Daily (automatic) | As needed (you decide) |
| **Status** | ⏳ Infrastructure ready | ✅ Already implemented |

**Read**: `docs/SETTLEMENTS_VS_REFUNDS.md` for detailed explanation

---

## Key Decisions Made ✅

### ListTransactions
**Decision**: Keep current implementation (database query)
**Reason**: 10-50x faster, more reliable, already complete
**Status**: ✅ Production ready

### Chargebacks
**Decision**: Use North's Dispute API with hourly polling
**API**: GET /merchant/disputes/mid/search
**Status**: ⏳ Waiting for authentication details from North

### Settlement Reports
**Decision**: TBD - need North's response
**Options**: Dedicated API, SFTP, or manual download
**Status**: ⏳ Waiting for North's response

## What's Complete ✅

| Component | Status | File Location |
|-----------|--------|---------------|
| Database migrations | ✅ | `internal/db/migrations/002_chargebacks.sql`<br>`internal/db/migrations/003_settlements.sql` |
| Chargeback model | ✅ | `internal/domain/models/chargeback.go` |
| Settlement models | ✅ | `internal/domain/models/settlement.go` |
| Chargeback repository interface | ✅ | `internal/domain/ports/chargeback_repository.go` |
| Settlement repository interface | ✅ | `internal/domain/ports/settlement_repository.go` |
| Transaction lookup by gateway ID | ✅ | `internal/domain/ports/transaction_repository.go:23` |
| Implementation guides | ✅ | All in `docs/` directory |

## What's Pending ⏳

### Waiting for North Support:
1. Dispute API authentication method
2. Complete enumeration values (status, disputeType)
3. Reason code mapping by card brand
4. Settlement report access method
5. Sample settlement file

### Implementation Work (2-3 days after North responds):
1. DisputeAdapter with authentication
2. ChargebackRepository PostgreSQL implementation
3. SyncService polling logic
4. Scheduled sync job
5. Alert system integration

## Documentation Files

| Document | Purpose | Status |
|----------|---------|--------|
| `CHARGEBACK_SETTLEMENT_SUMMARY.md` | Executive overview | ✅ Complete |
| `DISPUTE_API_INTEGRATION.md` | Dispute API integration guide | ✅ Complete |
| `CHARGEBACK_MANAGEMENT.md` | Business case for chargebacks | ✅ Complete |
| `IMPLEMENTATION_CHECKLIST.md` | Step-by-step checklist | ✅ Complete |
| `BUSINESS_REPORTING_API_ANALYSIS.md` | Why not to use for ListTransactions | ✅ Complete |
| `QUICK_REFERENCE.md` | This file | ✅ Complete |

## North Dispute API

### Endpoint
```
GET /merchant/disputes/mid/search
```

### Query Format
```
?findBy=byMerchant:{merchantId},fromDate:{YYYY-MM-DD},toDate:{YYYY-MM-DD}
```

### Response Fields → Our Database
| North Field | Our Field | Notes |
|-------------|-----------|-------|
| `caseNumber` | `chargeback_id` | Unique case ID |
| `disputeDate` | `chargeback_date` | When customer filed |
| `chargebackDate` | `received_date` | When we were notified |
| `status` | `status` | NEW/PENDING/WON/LOST |
| `transactionNumber` | Link via `gateway_transaction_id` | Links to our txn |
| `reasonCode` | `reason_code` | P22, F10, etc. |
| `chargebackAmount` | `amount` | Disputed amount |

## Implementation Timeline

### Now (Before North Responds):
✅ Database migrations ready
✅ Domain models ready
✅ Repository interfaces ready
✅ Code templates ready
✅ Documentation complete

### After North Responds (2-3 days):
- Day 1: Implement DisputeAdapter + authentication
- Day 2: Implement SyncService + scheduled job
- Day 3: Testing + alerting + monitoring

### Production Deployment:
- Run database migrations
- Deploy chargeback sync service
- Configure hourly polling
- Set up Slack/email alerts
- Monitor sync job health

## Contact North Support

**Email Template**: See `IMPLEMENTATION_CHECKLIST.md` line 744

**Key Questions**:
1. Dispute API authentication method?
2. Complete status/disputeType values?
3. Reason code mapping?
4. Settlement report access?
5. Pagination support?
6. Rate limits?

## Commands

### Run Migrations
```bash
# After North responds, run these:
cd /home/kevinlam/Documents/projects/payments
./bin/migrate up
```

### Start Chargeback Sync (after implementation)
```bash
./bin/chargeback-sync
```

## Architecture Highlights

### Hexagonal Design
```
gRPC API → Service Layer → Repository → PostgreSQL
           ↓
        North Adapters (Browser Post, ACH, Recurring, Dispute)
```

### Benefits
- ⚡ Fast (< 10ms database queries)
- 🔒 Reliable (no external dependencies for core operations)
- 💰 Cost-effective (minimal API calls)
- 🧩 Testable (all interfaces, easy to mock)
- 🔄 Swappable (can change payment gateways)

## Quick Links

- Full implementation guide: `docs/DISPUTE_API_INTEGRATION.md`
- Executive summary: `docs/CHARGEBACK_SETTLEMENT_SUMMARY.md`
- Contact template: `docs/IMPLEMENTATION_CHECKLIST.md:744`
- Database schemas: `internal/db/migrations/`
- Domain models: `internal/domain/models/`

## Success Metrics

After implementation, track:
- **Chargeback ratio**: Must stay < 1% (Visa/Mastercard threshold)
- **Win rate**: % of chargebacks won
- **Response time**: Time to respond to chargebacks
- **Settlement accuracy**: % of days with zero discrepancies
- **Sync job health**: Uptime and success rate

## Emergency Contacts

**North Support**: [Get from North portal]

**Internal Team**:
- Finance: [For chargeback alerts]
- DevOps: [For sync job monitoring]
- Support: [For evidence gathering]
