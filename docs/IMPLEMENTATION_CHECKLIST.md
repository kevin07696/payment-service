# Chargeback & Settlement Implementation Checklist

## 2� Chargeback Tracking Implementation

### =� Step 1: Information Needed from North Payment Gateway

Contact North support and ask these specific questions:

#### Dispute API (GET /merchant/disputes/mid/search):
- [ ] **Authentication Method**:
  - HMAC-SHA256 (like Browser Post API)?
  - JWT (like Business Reporting API)?
  - API Key?
  - What headers are required?

- [ ] **Status Values - Complete List**:
  - Confirmed: NEW, PENDING, WON, LOST, ACCEPTED
  - Are there others? (IN_ARBITRATION, WITHDRAWN, etc.)

- [ ] **Dispute Types - Complete List**:
  - Confirmed: "First Chargeback", "Pre-Arbitration"
  - Are there others? (Retrieval Request, Second Chargeback, etc.)

- [ ] **Reason Codes**:
  - Complete list by card brand (Visa, Mastercard, Amex, Discover)?
  - Mapping guide for categories?

- [ ] **Response Deadline**:
  - Is `respond_by_date` included in API response?
  - If not, how to calculate it?
  - Typical deadline varies by card brand?

- [ ] **Pagination**:
  - Is there pagination support?
  - Maximum results per request?
  - How to request next page?

- [ ] **Rate Limits**:
  - What are rate limits for Dispute API?
  - Recommended polling frequency?

- [ ] **Webhook Alternative**:
  - Is there a webhook for new disputes?
  - Webhook URL registration process?
  - Webhook payload format?

#### Chargeback Response/Representment:
- [ ] **How do we respond to chargebacks?**
  - API endpoint for evidence submission?
  - Email with attachments?
  - Portal upload?

- [ ] **What evidence formats do you accept?**
  - PDF documents?
  - Image files (JPG, PNG)?
  - File size limits?

- [ ] **Response deadline** (typically 7-10 days)

- [ ] **Chargeback status updates**
  - How are outcomes communicated?
  - Webhook for resolution?

#### Chargeback Fees:
- [ ] **Chargeback fee amount** (typically $15-$25 per chargeback)
- [ ] **How are fees charged?** (deducted from settlement, separate invoice?)

### =� Step 2: Database Schema

Create migration file: `migrations/XXXXXX_create_chargebacks.sql`

```sql
-- Chargebacks table
CREATE TABLE chargebacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    merchant_id VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,

    -- Chargeback details
    chargeback_id VARCHAR(255) UNIQUE, -- North's chargeback ID
    amount DECIMAL(19,4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Reason information
    reason_code VARCHAR(50) NOT NULL, -- e.g., "10.4", "13.1"
    reason_description TEXT,
    category VARCHAR(50), -- "FRAUD", "AUTHORIZATION", "PROCESSING_ERROR"

    -- Dates
    chargeback_date TIMESTAMPTZ NOT NULL, -- When chargeback was filed
    received_date TIMESTAMPTZ NOT NULL,   -- When we were notified
    respond_by_date TIMESTAMPTZ,          -- Deadline to respond
    response_submitted_at TIMESTAMPTZ,    -- When we submitted evidence
    resolved_at TIMESTAMPTZ,              -- When outcome was determined

    -- Status tracking
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- "pending", "responded", "won", "lost", "accepted"

    outcome VARCHAR(50), -- "reversed", "upheld", "partial"

    -- Evidence and notes
    evidence_files JSONB DEFAULT '[]', -- Array of file URLs/paths
    response_notes TEXT,
    internal_notes TEXT,

    -- Metadata
    raw_data JSONB, -- Store full webhook/notification payload
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_chargebacks_transaction ON chargebacks(transaction_id);
CREATE INDEX idx_chargebacks_merchant ON chargebacks(merchant_id);
CREATE INDEX idx_chargebacks_status ON chargebacks(status);
CREATE INDEX idx_chargebacks_respond_by ON chargebacks(respond_by_date)
    WHERE status = 'pending';

-- Trigger for updated_at
CREATE TRIGGER update_chargebacks_updated_at
    BEFORE UPDATE ON chargebacks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

### =� Step 3: Code Implementation

#### 3.1 Domain Models

Create: `internal/domain/models/chargeback.go`

```go
package models

import (
    "time"
    "github.com/shopspring/decimal"
)

type ChargebackStatus string

const (
    ChargebackPending    ChargebackStatus = "pending"
    ChargebackResponded  ChargebackStatus = "responded"
    ChargebackWon        ChargebackStatus = "won"
    ChargebackLost       ChargebackStatus = "lost"
    ChargebackAccepted   ChargebackStatus = "accepted"
)

type ChargebackOutcome string

const (
    OutcomeReversed ChargebackOutcome = "reversed" // You won
    OutcomeUpheld   ChargebackOutcome = "upheld"   // Customer won
    OutcomePartial  ChargebackOutcome = "partial"  // Split decision
)

type ChargebackCategory string

const (
    CategoryFraud            ChargebackCategory = "fraud"
    CategoryAuthorization    ChargebackCategory = "authorization"
    CategoryProcessingError  ChargebackCategory = "processing_error"
    CategoryConsumerDispute  ChargebackCategory = "consumer_dispute"
)

type Chargeback struct {
    ID                  string
    TransactionID       string
    MerchantID          string
    CustomerID          string
    ChargebackID        string // North's ID
    Amount              decimal.Decimal
    Currency            string
    ReasonCode          string
    ReasonDescription   string
    Category            ChargebackCategory
    ChargebackDate      time.Time
    ReceivedDate        time.Time
    RespondByDate       *time.Time
    ResponseSubmittedAt *time.Time
    ResolvedAt          *time.Time
    Status              ChargebackStatus
    Outcome             *ChargebackOutcome
    EvidenceFiles       []string
    ResponseNotes       string
    InternalNotes       string
    RawData             map[string]interface{}
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

#### 3.2 Repository Interface

Add to: `internal/domain/ports/chargeback_repository.go`

```go
package ports

import (
    "context"
    "github.com/kevin07696/payment-service/internal/domain/models"
)

type ChargebackRepository interface {
    Create(ctx context.Context, tx DBTX, chargeback *models.Chargeback) error
    GetByID(ctx context.Context, db DBTX, id string) (*models.Chargeback, error)
    GetByTransactionID(ctx context.Context, db DBTX, txnID string) ([]*models.Chargeback, error)
    ListByMerchant(ctx context.Context, db DBTX, merchantID string, limit, offset int32) ([]*models.Chargeback, error)
    ListPendingResponses(ctx context.Context, db DBTX) ([]*models.Chargeback, error)
    Update(ctx context.Context, tx DBTX, chargeback *models.Chargeback) error
    UpdateStatus(ctx context.Context, tx DBTX, id string, status models.ChargebackStatus, outcome *models.ChargebackOutcome) error
}
```

#### 3.3 Webhook Handler (if North provides webhooks)

Create: `internal/api/webhooks/chargeback_handler.go`

```go
package webhooks

import (
    "context"
    "encoding/json"
    "net/http"
    "time"

    "github.com/kevin07696/payment-service/internal/domain/models"
    "github.com/kevin07696/payment-service/internal/domain/ports"
)

type ChargebackWebhookHandler struct {
    chargebackRepo ports.ChargebackRepository
    txRepo         ports.TransactionRepository
    logger         ports.Logger
    webhookSecret  string // For HMAC validation
}

// North webhook payload (this will vary - you need actual format from North)
type NorthChargebackWebhook struct {
    EventType       string    `json:"event_type"` // "chargeback.created"
    ChargebackID    string    `json:"chargeback_id"`
    TransactionID   string    `json:"transaction_id"`
    Amount          string    `json:"amount"`
    Currency        string    `json:"currency"`
    ReasonCode      string    `json:"reason_code"`
    ReasonDesc      string    `json:"reason_description"`
    ChargebackDate  time.Time `json:"chargeback_date"`
    RespondByDate   time.Time `json:"respond_by_date"`
}

func (h *ChargebackWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
    // 1. Validate webhook signature
    if !h.validateSignature(r) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    // 2. Parse webhook payload
    var webhook NorthChargebackWebhook
    if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
        http.Error(w, "Invalid payload", http.StatusBadRequest)
        return
    }

    // 3. Process based on event type
    switch webhook.EventType {
    case "chargeback.created":
        h.handleChargebackCreated(r.Context(), &webhook)
    case "chargeback.resolved":
        h.handleChargebackResolved(r.Context(), &webhook)
    default:
        h.logger.Warn("Unknown webhook event type", ports.String("type", webhook.EventType))
    }

    // 4. Acknowledge receipt
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "received"})
}

func (h *ChargebackWebhookHandler) validateSignature(r *http.Request) bool {
    // Implement HMAC validation based on North's specification
    // Similar to how you validate gateway requests
    return true // Placeholder
}
```

#### 3.4 Service Layer

Create: `internal/services/chargeback/chargeback_service.go`

```go
package chargeback

import (
    "context"
    "fmt"
    "time"

    "github.com/kevin07696/payment-service/internal/domain/models"
    "github.com/kevin07696/payment-service/internal/domain/ports"
)

type Service struct {
    chargebackRepo ports.ChargebackRepository
    txRepo         ports.TransactionRepository
    db             ports.DBPort
    logger         ports.Logger
}

func NewService(
    chargebackRepo ports.ChargebackRepository,
    txRepo ports.TransactionRepository,
    db ports.DBPort,
    logger ports.Logger,
) *Service {
    return &Service{
        chargebackRepo: chargebackRepo,
        txRepo:         txRepo,
        db:             db,
        logger:         logger,
    }
}

func (s *Service) CreateChargeback(ctx context.Context, req CreateChargebackRequest) error {
    // Create chargeback record
    chargeback := &models.Chargeback{
        TransactionID:     req.TransactionID,
        MerchantID:        req.MerchantID,
        CustomerID:        req.CustomerID,
        ChargebackID:      req.ChargebackID,
        Amount:            req.Amount,
        Currency:          req.Currency,
        ReasonCode:        req.ReasonCode,
        ReasonDescription: req.ReasonDescription,
        ChargebackDate:    req.ChargebackDate,
        ReceivedDate:      time.Now(),
        RespondByDate:     req.RespondByDate,
        Status:            models.ChargebackPending,
        RawData:           req.RawData,
    }

    err := s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
        return s.chargebackRepo.Create(ctx, tx, chargeback)
    })

    if err != nil {
        return fmt.Errorf("create chargeback: %w", err)
    }

    // Send alert to team
    s.alertTeam(chargeback)

    return nil
}

func (s *Service) alertTeam(cb *models.Chargeback) {
    // Send email/Slack notification to team
    s.logger.Warn("CHARGEBACK RECEIVED",
        ports.String("chargeback_id", cb.ChargebackID),
        ports.String("transaction_id", cb.TransactionID),
        ports.String("amount", cb.Amount.String()),
        ports.String("reason", cb.ReasonCode),
    )
}
```

###  Step 4: Testing Checklist

- [ ] Create chargeback manually in database
- [ ] Test webhook endpoint (use Postman/curl)
- [ ] Verify chargeback links to transaction
- [ ] Test alert notifications
- [ ] Test listing chargebacks by merchant
- [ ] Test filtering by status

---

## 3� Settlement Reports Implementation

### =� Step 1: Information Needed from North Payment Gateway

Contact North and ask:

#### Settlement Report Access:
- [ ] **How do you provide settlement reports?**
  - API endpoint?
  - SFTP file server?
  - Email with attachment?
  - Download from merchant portal?

- [ ] **What format are settlement reports in?**
  - CSV?
  - XML?
  - JSON?
  - Fixed-width text?
  - Excel?

- [ ] **Sample settlement report file** (request actual example)

- [ ] **Settlement schedule**
  - How often? (Daily, weekly, monthly?)
  - What time are reports available?
  - Time zone?

- [ ] **Settlement timing**
  - When do transactions settle? (T+1, T+2, T+3 days?)
  - Do different card types settle differently?

#### Settlement Report Contents:
- [ ] **What data is included?**
  - Transaction ID
  - Transaction date
  - Settlement date
  - Gross amount
  - Processing fee
  - Net amount
  - Card brand
  - Interchange rate?
  - Chargeback deductions?
  - Refund amounts?

- [ ] **Batching information**
  - Batch ID/number
  - Batch totals

- [ ] **API credentials** (if API-based)
  - Separate API key for settlement API?
  - Same EPI-Id/EPI-Key?

### =� Step 2: Database Schema

Create migration: `migrations/XXXXXX_create_settlements.sql`

```sql
-- Settlement batches table
CREATE TABLE settlement_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id VARCHAR(255) NOT NULL,

    -- Settlement identification
    settlement_batch_id VARCHAR(255) NOT NULL UNIQUE, -- North's batch ID
    settlement_date DATE NOT NULL,
    deposit_date DATE,

    -- Financial summary
    total_sales DECIMAL(19,4) NOT NULL DEFAULT 0,
    total_refunds DECIMAL(19,4) NOT NULL DEFAULT 0,
    total_chargebacks DECIMAL(19,4) NOT NULL DEFAULT 0,
    total_fees DECIMAL(19,4) NOT NULL DEFAULT 0,
    net_amount DECIMAL(19,4) NOT NULL, -- What actually deposited
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Transaction counts
    sales_count INTEGER NOT NULL DEFAULT 0,
    refund_count INTEGER NOT NULL DEFAULT 0,
    chargeback_count INTEGER NOT NULL DEFAULT 0,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- "pending", "reconciled", "discrepancy", "completed"

    -- Reconciliation
    reconciled_at TIMESTAMPTZ,
    discrepancy_amount DECIMAL(19,4),
    discrepancy_notes TEXT,

    -- Metadata
    raw_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Settlement transactions (detail)
CREATE TABLE settlement_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    settlement_batch_id UUID NOT NULL REFERENCES settlement_batches(id),
    transaction_id UUID REFERENCES transactions(id),

    -- Transaction details
    gateway_transaction_id VARCHAR(255),
    transaction_date TIMESTAMPTZ NOT NULL,
    settlement_date DATE NOT NULL,

    -- Amounts
    gross_amount DECIMAL(19,4) NOT NULL,
    fee_amount DECIMAL(19,4) NOT NULL DEFAULT 0,
    net_amount DECIMAL(19,4) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',

    -- Transaction info
    transaction_type VARCHAR(50), -- "SALE", "REFUND", "CHARGEBACK"
    card_brand VARCHAR(50),
    card_type VARCHAR(50), -- "CREDIT", "DEBIT"

    -- Interchange (if available)
    interchange_rate DECIMAL(6,4),
    interchange_fee DECIMAL(19,4),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_settlement_batches_merchant ON settlement_batches(merchant_id);
CREATE INDEX idx_settlement_batches_date ON settlement_batches(settlement_date);
CREATE INDEX idx_settlement_batches_status ON settlement_batches(status);

CREATE INDEX idx_settlement_txns_batch ON settlement_transactions(settlement_batch_id);
CREATE INDEX idx_settlement_txns_transaction ON settlement_transactions(transaction_id);
CREATE INDEX idx_settlement_txns_date ON settlement_transactions(settlement_date);
```

### =� Step 3: Code Implementation

#### 3.1 Domain Models

Create: `internal/domain/models/settlement.go`

```go
package models

import (
    "time"
    "github.com/shopspring/decimal"
)

type SettlementStatus string

const (
    SettlementPending     SettlementStatus = "pending"
    SettlementReconciled  SettlementStatus = "reconciled"
    SettlementDiscrepancy SettlementStatus = "discrepancy"
    SettlementCompleted   SettlementStatus = "completed"
)

type SettlementBatch struct {
    ID                string
    MerchantID        string
    SettlementBatchID string
    SettlementDate    time.Time
    DepositDate       *time.Time
    TotalSales        decimal.Decimal
    TotalRefunds      decimal.Decimal
    TotalChargebacks  decimal.Decimal
    TotalFees         decimal.Decimal
    NetAmount         decimal.Decimal
    Currency          string
    SalesCount        int32
    RefundCount       int32
    ChargebackCount   int32
    Status            SettlementStatus
    ReconciledAt      *time.Time
    DiscrepancyAmount *decimal.Decimal
    DiscrepancyNotes  string
    RawData           map[string]interface{}
    CreatedAt         time.Time
    UpdatedAt         time.Time
}

type SettlementTransaction struct {
    ID                   string
    SettlementBatchID    string
    TransactionID        *string
    GatewayTransactionID string
    TransactionDate      time.Time
    SettlementDate       time.Time
    GrossAmount          decimal.Decimal
    FeeAmount            decimal.Decimal
    NetAmount            decimal.Decimal
    Currency             string
    TransactionType      string
    CardBrand            string
    CardType             string
    InterchangeRate      *decimal.Decimal
    InterchangeFee       *decimal.Decimal
    CreatedAt            time.Time
}
```

#### 3.2 Settlement Parser

Create: `internal/adapters/north/settlement_parser.go`

```go
package north

import (
    "encoding/csv"
    "fmt"
    "io"
    "time"

    "github.com/kevin07696/payment-service/internal/domain/models"
    "github.com/shopspring/decimal"
)

// This will vary based on North's actual format
type SettlementCSVParser struct{}

func (p *SettlementCSVParser) Parse(reader io.Reader) (*models.SettlementBatch, error) {
    csvReader := csv.NewReader(reader)

    // Skip header
    _, err := csvReader.Read()
    if err != nil {
        return nil, fmt.Errorf("read header: %w", err)
    }

    batch := &models.SettlementBatch{
        Transactions: []*models.SettlementTransaction{},
    }

    for {
        record, err := csvReader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("read record: %w", err)
        }

        // Parse based on North's CSV format
        // Example (you'll need to adjust based on actual format):
        txn := &models.SettlementTransaction{
            GatewayTransactionID: record[0],
            TransactionDate:      parseDate(record[1]),
            SettlementDate:       parseDate(record[2]),
            GrossAmount:          parseAmount(record[3]),
            FeeAmount:            parseAmount(record[4]),
            NetAmount:            parseAmount(record[5]),
            CardBrand:            record[6],
            TransactionType:      record[7],
        }

        batch.Transactions = append(batch.Transactions, txn)
    }

    return batch, nil
}
```

#### 3.3 Settlement Service

Create: `internal/services/settlement/settlement_service.go`

```go
package settlement

import (
    "context"
    "fmt"
    "time"

    "github.com/kevin07696/payment-service/internal/domain/models"
    "github.com/kevin07696/payment-service/internal/domain/ports"
)

type Service struct {
    settlementRepo ports.SettlementRepository
    txRepo         ports.TransactionRepository
    db             ports.DBPort
    logger         ports.Logger
}

func (s *Service) ImportSettlement(ctx context.Context, batch *models.SettlementBatch) error {
    return s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
        // Save batch
        if err := s.settlementRepo.CreateBatch(ctx, tx, batch); err != nil {
            return fmt.Errorf("create batch: %w", err)
        }

        // Save transactions
        for _, txn := range batch.Transactions {
            if err := s.settlementRepo.CreateTransaction(ctx, tx, txn); err != nil {
                return fmt.Errorf("create transaction: %w", err)
            }
        }

        return nil
    })
}

func (s *Service) ReconcileSettlement(ctx context.Context, batchID string) error {
    // Get settlement batch
    batch, err := s.settlementRepo.GetBatchByID(ctx, nil, batchID)
    if err != nil {
        return err
    }

    // Get our transactions for that date
    ourTxns, err := s.txRepo.ListByDateRange(ctx, nil,
        batch.SettlementDate, batch.SettlementDate.Add(24*time.Hour))

    // Compare
    ourTotal := calculateTotal(ourTxns)
    discrepancy := ourTotal.Sub(batch.NetAmount)

    if !discrepancy.IsZero() {
        // Flag discrepancy
        s.logger.Warn("Settlement discrepancy detected",
            ports.String("batch_id", batchID),
            ports.String("expected", ourTotal.String()),
            ports.String("actual", batch.NetAmount.String()),
            ports.String("difference", discrepancy.String()))

        // Update batch status
        s.settlementRepo.UpdateStatus(ctx, nil, batchID,
            models.SettlementDiscrepancy, &discrepancy)
    } else {
        s.settlementRepo.UpdateStatus(ctx, nil, batchID,
            models.SettlementReconciled, nil)
    }

    return nil
}
```

###  Step 4: Testing Checklist

- [ ] Get sample settlement file from North
- [ ] Test CSV/XML parser with sample file
- [ ] Import settlement to database
- [ ] Verify all transactions imported
- [ ] Test reconciliation logic
- [ ] Test discrepancy detection
- [ ] Test settlement report export

---

## =� Summary Checklist

### Chargeback Implementation:
- [ ] Contact North about chargeback notifications
- [ ] Get sample chargeback data
- [ ] Create database schema
- [ ] Implement domain models
- [ ] Create repository
- [ ] Build webhook handler (if available)
- [ ] Implement service layer
- [ ] Add alert notifications
- [ ] Test end-to-end

### Settlement Implementation:
- [ ] Contact North about settlement reports
- [ ] Get sample settlement file
- [ ] Determine access method (API/SFTP/etc)
- [ ] Create database schema
- [ ] Implement parser
- [ ] Build import service
- [ ] Implement reconciliation logic
- [ ] Test with sample data
- [ ] Schedule daily reconciliation job

### North Contact Template:

```
Subject: Settlement Reports & Chargeback Integration - [Your Merchant ID]

Hi North Support,

We're integrating our payment system with your APIs and need information about:

1. **Dispute API (Chargebacks) - GET /merchant/disputes/mid/search:**
   - What authentication method does this API use? (HMAC/JWT/API Key?)
   - What headers are required for authentication?
   - What are all possible `status` values? (We see: NEW, PENDING, WON, LOST, ACCEPTED)
   - What are all possible `disputeType` values? (We see: "First Chargeback", "Pre-Arbitration")
   - Complete list of `reasonCode` values by card brand?
   - Is `respond_by_date` included in the response?
   - Is there pagination? Maximum results per request?
   - What are the rate limits? Recommended polling frequency?
   - Is there a webhook alternative to polling?
   - How do we submit evidence/responses to disputes?

2. **Settlement Reports:**
   - How do we access settlement reports? (API/SFTP/portal download)
   - **Business Reporting API Questions:**
     - Does GET /accounts/{accountId}/transactions provide settlement batch data?
     - Can we filter transactions by settlement date?
     - Are processing fees included in the transaction data?
   - What format are they in? (CSV/XML/JSON)
   - Can you provide a sample settlement report?
   - What's the settlement schedule? (daily/weekly)
   - Settlement timing? (T+1, T+2, T+3 days?)

3. **API Documentation:**
   - Is there API documentation for settlement and chargeback endpoints?
   - Do we use the same EPI-Id/EPI-Key credentials?

Please provide:
- Sample files/payloads
- API endpoint URLs (if applicable)
- SFTP credentials (if applicable)
- Any additional documentation

Thank you!
```

---

**Next Steps:**
1. Copy the email template above
2. Send to North support
3. Wait for their response with sample data
4. Then implement based on their actual formats

Would you like me to start implementing the database migrations while you wait for North's response?
