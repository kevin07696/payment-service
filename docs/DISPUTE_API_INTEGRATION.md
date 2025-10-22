# North Dispute API Integration Guide

## Overview

North provides a dedicated **Dispute API** for retrieving chargeback data. This is the recommended approach for chargeback tracking (not the Business Reporting API).

## API Endpoint

### GET /merchant/disputes/mid/search

**Purpose**: Search for disputes (chargebacks) by Merchant ID with date range filtering

**Authentication**: ❓ Need to confirm (HMAC? JWT? API Key?)

**Query Parameters**:
```
findBy=byMerchant:12345,toDate:2024-12-30,fromDate:2023-12-24
```

**Parameter Format**:
- `byMerchant:{merchantId}` - Required
- `fromDate:{YYYY-MM-DD}` - Optional (start date)
- `toDate:{YYYY-MM-DD}` - Optional (end date)

**Example Request**:
```http
GET /merchant/disputes/mid/search?findBy=byMerchant:12345,fromDate:2024-01-01,toDate:2024-12-31
Authorization: ??? (Need to confirm auth method)
```

## Response Format

**Success Response (200)**:
```json
{
  "status": "success",
  "data": {
    "disputes": [
      {
        "caseNumber": "12345",
        "disputeDate": "2024-03-08",
        "chargebackDate": "2024-03-18",
        "disputeType": "First Chargeback",
        "status": "NEW",
        "cardBrand": "American Express",
        "cardNumberLastFour": "*0005",
        "transactionNumber": "123456789",
        "reasonCode": "P22",
        "reasonDescription": "Non-Matching Card Number",
        "transactionAmount": 452.9,
        "transactionDate": "2015-06-17",
        "chargebackAmount": 30
      }
    ],
    "meta": {
      "totalDisputes": 3,
      "currentResultCount": 3
    }
  },
  "link": "/merchant/disputes/mid/search"
}
```

**Error Responses**:
- `400 Bad Request`: Invalid query parameters
- `403 Forbidden`: Authentication/authorization failure

## Field Mapping to Our Database Schema

| North API Field | Our Database Field | Type | Notes |
|----------------|-------------------|------|-------|
| `caseNumber` | `chargeback_id` | VARCHAR(255) | Unique dispute case ID |
| `disputeDate` | `chargeback_date` | TIMESTAMPTZ | When customer filed dispute |
| `chargebackDate` | `received_date` | TIMESTAMPTZ | When merchant was notified |
| `disputeType` | `category` (derived) | VARCHAR(50) | Map to fraud/authorization/etc |
| `status` | `status` | VARCHAR(50) | NEW → pending, WON → won, LOST → lost |
| `cardBrand` | *(metadata)* | JSONB | Store in raw_data |
| `cardNumberLastFour` | *(metadata)* | JSONB | Store in raw_data |
| `transactionNumber` | *(link via)* `gateway_transaction_id` | - | Links to our transactions table |
| `reasonCode` | `reason_code` | VARCHAR(50) | P22, F10, P23, etc. |
| `reasonDescription` | `reason_description` | TEXT | Human-readable reason |
| `transactionAmount` | *(verify)* | - | Original transaction amount |
| `transactionDate` | *(verify)* | - | Original transaction date |
| `chargebackAmount` | `amount` | DECIMAL(19,4) | Disputed amount |

## Status Mapping

### North API Status → Our ChargebackStatus

| North Status | Our Status | Description |
|-------------|-----------|-------------|
| `NEW` | `pending` | Newly received chargeback |
| `PENDING` | `pending` | Awaiting merchant response |
| `WON` | `won` | Merchant won the dispute |
| `LOST` | `lost` | Customer won the dispute |
| `ACCEPTED` | `accepted` | Merchant accepted without contesting |
| ❓ Others? | ❓ | **Need full list from North** |

## Dispute Type → Category Mapping

### North disputeType → Our ChargebackCategory

| North Dispute Type | Our Category | Reason |
|-------------------|-------------|--------|
| `First Chargeback` | *(depends on reasonCode)* | Need to check reason code |
| `Pre-Arbitration` | *(depends on reasonCode)* | Second-level dispute |
| ❓ Others? | ❓ | **Need full list from North** |

### Reason Code → Category Mapping

**American Express Codes** (from example):
- `P22` - Non-Matching Card Number → `fraud`
- `F10` - Missing Imprint → `authorization`
- `P23` - Currency Discrepancy → `processing_error`

**Visa/Mastercard Codes** (need confirmation):
- `10.4` - Fraud, Card-Absent Environment → `fraud`
- `13.1` - Services Not Provided → `consumer_dispute`
- `13.3` - Not as Described → `consumer_dispute`
- `13.5` - Misrepresentation → `consumer_dispute`
- `13.6` - Credit Not Processed → `processing_error`
- `13.7` - Cancelled Recurring → `consumer_dispute`

## Implementation: Chargeback Polling Service

### Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Chargeback Sync Job                       │
│                                                              │
│  1. Scheduled (Hourly/Daily)                                │
│  2. Call North Dispute API                                   │
│  3. Parse Response                                           │
│  4. Link to Transactions                                     │
│  5. Create/Update Chargebacks                                │
│  6. Alert Team on New Disputes                               │
└─────────────────────────────────────────────────────────────┘
```

### Go Implementation

**1. North Dispute API Client** (`internal/adapters/north/dispute_adapter.go`):

```go
package north

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "time"

    "github.com/kevin07696/payment-service/internal/domain/models"
    "github.com/kevin07696/payment-service/internal/domain/ports"
    "github.com/shopspring/decimal"
)

type DisputeAdapter struct {
    baseURL    string
    httpClient ports.HTTPClient
    authConfig AuthConfig
    logger     ports.Logger
}

func NewDisputeAdapter(baseURL string, authConfig AuthConfig, httpClient ports.HTTPClient, logger ports.Logger) *DisputeAdapter {
    return &DisputeAdapter{
        baseURL:    baseURL,
        httpClient: httpClient,
        authConfig: authConfig,
        logger:     logger,
    }
}

// DisputeResponse represents North's dispute API response
type DisputeResponse struct {
    Status string `json:"status"`
    Data   struct {
        Disputes []DisputeData `json:"disputes"`
        Meta     struct {
            TotalDisputes       int `json:"totalDisputes"`
            CurrentResultCount int `json:"currentResultCount"`
        } `json:"meta"`
    } `json:"data"`
    Link string `json:"link"`
}

type DisputeData struct {
    CaseNumber          string  `json:"caseNumber"`
    DisputeDate         string  `json:"disputeDate"`          // "2024-03-08"
    ChargebackDate      string  `json:"chargebackDate"`       // "2024-03-18"
    DisputeType         string  `json:"disputeType"`          // "First Chargeback"
    Status              string  `json:"status"`               // "NEW", "WON", "LOST"
    CardBrand           string  `json:"cardBrand"`            // "American Express"
    CardNumberLastFour  string  `json:"cardNumberLastFour"`   // "*0005"
    TransactionNumber   string  `json:"transactionNumber"`    // "123456789"
    ReasonCode          string  `json:"reasonCode"`           // "P22"
    ReasonDescription   string  `json:"reasonDescription"`    // "Non-Matching Card Number"
    TransactionAmount   float64 `json:"transactionAmount"`    // 452.9
    TransactionDate     string  `json:"transactionDate"`      // "2015-06-17"
    ChargebackAmount    float64 `json:"chargebackAmount"`     // 30
}

// SearchDisputes retrieves disputes for a merchant by date range
func (a *DisputeAdapter) SearchDisputes(ctx context.Context, merchantID string, fromDate, toDate time.Time) ([]DisputeData, error) {
    // Build query parameters
    findBy := fmt.Sprintf("byMerchant:%s,fromDate:%s,toDate:%s",
        merchantID,
        fromDate.Format("2006-01-02"),
        toDate.Format("2006-01-02"))

    // Build URL
    u, err := url.Parse(a.baseURL + "/merchant/disputes/mid/search")
    if err != nil {
        return nil, fmt.Errorf("parse URL: %w", err)
    }

    q := u.Query()
    q.Set("findBy", findBy)
    u.RawQuery = q.Encode()

    // Create request
    req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    // Add authentication headers (TODO: confirm auth method with North)
    // Option 1: HMAC (like other APIs)
    // signature := CalculateSignature(a.authConfig.EPIKey, "/merchant/disputes/mid/search", "")
    // req.Header.Set("EPI-Id", a.authConfig.EPIId)
    // req.Header.Set("EPI-Signature", signature)
    //
    // Option 2: JWT (like Business Reporting API)
    // req.Header.Set("Authorization", "Bearer " + jwt)
    //
    // Option 3: API Key
    // req.Header.Set("X-API-Key", apiKey)

    // Execute request
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute request: %w", err)
    }
    defer resp.Body.Close()

    // Check status code
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
    }

    // Parse response
    var disputeResp DisputeResponse
    if err := json.NewDecoder(resp.Body).Decode(&disputeResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    // Check response status
    if disputeResp.Status != "success" {
        return nil, fmt.Errorf("API returned error status: %s", disputeResp.Status)
    }

    a.logger.Info("Retrieved disputes from North API",
        ports.String("merchant_id", merchantID),
        ports.Int("count", len(disputeResp.Data.Disputes)))

    return disputeResp.Data.Disputes, nil
}

// ConvertToChargeback converts North dispute data to our domain model
func (a *DisputeAdapter) ConvertToChargeback(dispute DisputeData, transactionID string, merchantID string, customerID string) (*models.Chargeback, error) {
    // Parse dates
    chargebackDate, err := time.Parse("2006-01-02", dispute.DisputeDate)
    if err != nil {
        return nil, fmt.Errorf("parse dispute date: %w", err)
    }

    receivedDate, err := time.Parse("2006-01-02", dispute.ChargebackDate)
    if err != nil {
        return nil, fmt.Errorf("parse chargeback date: %w", err)
    }

    // Convert amount
    amount := decimal.NewFromFloat(dispute.ChargebackAmount)

    // Map status
    status := mapDisputeStatus(dispute.Status)

    // Map category based on reason code
    category := mapReasonCodeToCategory(dispute.ReasonCode)

    // Build raw data
    rawData := map[string]interface{}{
        "cardBrand":          dispute.CardBrand,
        "cardNumberLastFour": dispute.CardNumberLastFour,
        "transactionNumber":  dispute.TransactionNumber,
        "transactionAmount":  dispute.TransactionAmount,
        "transactionDate":    dispute.TransactionDate,
        "disputeType":        dispute.DisputeType,
    }

    chargeback := &models.Chargeback{
        TransactionID:     transactionID,
        MerchantID:        merchantID,
        CustomerID:        customerID,
        ChargebackID:      dispute.CaseNumber,
        Amount:            amount,
        Currency:          "USD", // Default, could be in response
        ReasonCode:        dispute.ReasonCode,
        ReasonDescription: dispute.ReasonDescription,
        Category:          category,
        ChargebackDate:    chargebackDate,
        ReceivedDate:      receivedDate,
        Status:            status,
        RawData:           rawData,
        CreatedAt:         time.Now(),
        UpdatedAt:         time.Now(),
    }

    return chargeback, nil
}

// mapDisputeStatus maps North's status to our ChargebackStatus
func mapDisputeStatus(northStatus string) models.ChargebackStatus {
    switch northStatus {
    case "NEW", "PENDING":
        return models.ChargebackPending
    case "WON":
        return models.ChargebackWon
    case "LOST":
        return models.ChargebackLost
    case "ACCEPTED":
        return models.ChargebackAccepted
    default:
        return models.ChargebackPending
    }
}

// mapReasonCodeToCategory maps reason codes to categories
func mapReasonCodeToCategory(reasonCode string) models.ChargebackCategory {
    // American Express codes
    switch reasonCode {
    case "P22", "F29", "F30", "F31": // Fraud-related
        return models.CategoryFraud
    case "F10", "F14", "F24": // Authorization-related
        return models.CategoryAuthorization
    case "P23", "P25": // Processing errors
        return models.CategoryProcessingError
    default:
        // Visa/Mastercard codes
        if reasonCode == "10.4" || reasonCode == "4837" {
            return models.CategoryFraud
        }
        if reasonCode[:2] == "13" { // 13.x codes
            return models.CategoryConsumerDispute
        }
        return models.CategoryConsumerDispute
    }
}
```

**2. Chargeback Sync Service** (`internal/services/chargeback/sync_service.go`):

```go
package chargeback

import (
    "context"
    "fmt"
    "time"

    "github.com/kevin07696/payment-service/internal/domain/models"
    "github.com/kevin07696/payment-service/internal/domain/ports"
)

type SyncService struct {
    disputeAPI     *north.DisputeAdapter
    chargebackRepo ports.ChargebackRepository
    txRepo         ports.TransactionRepository
    db             ports.DBPort
    logger         ports.Logger
}

func NewSyncService(
    disputeAPI *north.DisputeAdapter,
    chargebackRepo ports.ChargebackRepository,
    txRepo ports.TransactionRepository,
    db ports.DBPort,
    logger ports.Logger,
) *SyncService {
    return &SyncService{
        disputeAPI:     disputeAPI,
        chargebackRepo: chargebackRepo,
        txRepo:         txRepo,
        db:             db,
        logger:         logger,
    }
}

// SyncChargebacks polls North's Dispute API and syncs chargebacks to database
func (s *SyncService) SyncChargebacks(ctx context.Context, merchantID string, lookbackDays int) error {
    // Calculate date range (look back X days to catch updates)
    toDate := time.Now()
    fromDate := toDate.AddDate(0, 0, -lookbackDays)

    s.logger.Info("Starting chargeback sync",
        ports.String("merchant_id", merchantID),
        ports.String("from_date", fromDate.Format("2006-01-02")),
        ports.String("to_date", toDate.Format("2006-01-02")))

    // Fetch disputes from North API
    disputes, err := s.disputeAPI.SearchDisputes(ctx, merchantID, fromDate, toDate)
    if err != nil {
        return fmt.Errorf("fetch disputes: %w", err)
    }

    if len(disputes) == 0 {
        s.logger.Info("No disputes found")
        return nil
    }

    // Process each dispute
    newCount := 0
    updatedCount := 0
    errorCount := 0

    for _, dispute := range disputes {
        // Link to our transaction via gateway_transaction_id
        txn, err := s.findTransactionByGatewayID(ctx, dispute.TransactionNumber)
        if err != nil {
            s.logger.Error("Failed to find transaction for dispute",
                ports.String("case_number", dispute.CaseNumber),
                ports.String("gateway_txn_id", dispute.TransactionNumber),
                ports.String("error", err.Error()))
            errorCount++
            continue
        }

        // Check if chargeback already exists
        existing, err := s.chargebackRepo.GetByChargebackID(ctx, nil, dispute.CaseNumber)
        if err == nil && existing != nil {
            // Update existing chargeback
            if err := s.updateChargeback(ctx, existing, dispute); err != nil {
                s.logger.Error("Failed to update chargeback",
                    ports.String("case_number", dispute.CaseNumber),
                    ports.String("error", err.Error()))
                errorCount++
            } else {
                updatedCount++
            }
        } else {
            // Create new chargeback
            if err := s.createChargeback(ctx, dispute, txn); err != nil {
                s.logger.Error("Failed to create chargeback",
                    ports.String("case_number", dispute.CaseNumber),
                    ports.String("error", err.Error()))
                errorCount++
            } else {
                newCount++
                // Alert team about new chargeback
                s.alertNewChargeback(dispute, txn)
            }
        }
    }

    s.logger.Info("Chargeback sync completed",
        ports.Int("new", newCount),
        ports.Int("updated", updatedCount),
        ports.Int("errors", errorCount))

    return nil
}

func (s *SyncService) findTransactionByGatewayID(ctx context.Context, gatewayTxnID string) (*models.Transaction, error) {
    // TODO: Add GetByGatewayTransactionID to TransactionRepository
    // For now, this is a placeholder
    return nil, fmt.Errorf("not implemented")
}

func (s *SyncService) createChargeback(ctx context.Context, dispute north.DisputeData, txn *models.Transaction) error {
    chargeback, err := s.disputeAPI.ConvertToChargeback(dispute, txn.ID, txn.MerchantID, txn.CustomerID)
    if err != nil {
        return fmt.Errorf("convert dispute: %w", err)
    }

    return s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
        return s.chargebackRepo.Create(ctx, tx, chargeback)
    })
}

func (s *SyncService) updateChargeback(ctx context.Context, existing *models.Chargeback, dispute north.DisputeData) error {
    // Update status if changed
    newStatus := north.mapDisputeStatus(dispute.Status)
    if existing.Status != newStatus {
        return s.db.WithTransaction(ctx, func(ctx context.Context, tx pgx.Tx) error {
            var outcome *models.ChargebackOutcome
            if newStatus == models.ChargebackWon {
                o := models.OutcomeReversed
                outcome = &o
            } else if newStatus == models.ChargebackLost {
                o := models.OutcomeUpheld
                outcome = &o
            }
            return s.chargebackRepo.UpdateStatus(ctx, tx, existing.ID, newStatus, outcome)
        })
    }
    return nil
}

func (s *SyncService) alertNewChargeback(dispute north.DisputeData, txn *models.Transaction) {
    // Send alert (email, Slack, PagerDuty, etc.)
    s.logger.Warn("🚨 NEW CHARGEBACK RECEIVED",
        ports.String("case_number", dispute.CaseNumber),
        ports.String("transaction_id", txn.ID),
        ports.String("amount", fmt.Sprintf("$%.2f", dispute.ChargebackAmount)),
        ports.String("reason", dispute.ReasonDescription))

    // TODO: Integrate with alerting system
    // - Send email to finance team
    // - Post to Slack channel
    // - Create ticket in support system
}
```

**3. Scheduled Job** (using cron or time.Ticker):

```go
// cmd/chargeback-sync/main.go
package main

import (
    "context"
    "log"
    "time"

    "github.com/kevin07696/payment-service/internal/config"
    "github.com/kevin07696/payment-service/internal/adapters/north"
    "github.com/kevin07696/payment-service/internal/services/chargeback"
)

func main() {
    // Load config
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }

    // Initialize dependencies (similar to cmd/server/main.go)
    // ... logger, db, repositories, etc.

    // Create dispute adapter
    disputeAdapter := north.NewDisputeAdapter(
        cfg.Gateway.BaseURL,
        cfg.Gateway.AuthConfig,
        httpClient,
        logger,
    )

    // Create sync service
    syncService := chargeback.NewSyncService(
        disputeAdapter,
        chargebackRepo,
        txRepo,
        db,
        logger,
    )

    // Run sync every hour
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    // Run once immediately
    if err := syncService.SyncChargebacks(context.Background(), cfg.MerchantID, 30); err != nil {
        logger.Error("Chargeback sync failed", ports.String("error", err.Error()))
    }

    // Then run on schedule
    for range ticker.C {
        if err := syncService.SyncChargebacks(context.Background(), cfg.MerchantID, 30); err != nil {
            logger.Error("Chargeback sync failed", ports.String("error", err.Error()))
        }
    }
}
```

## Questions for North Support

### Critical Questions:

1. **Authentication**:
   - What authentication method does the Dispute API use?
     - HMAC-SHA256 (like Browser Post API)?
     - JWT (like Business Reporting API)?
     - API Key?
   - What headers are required?

2. **Status Values**:
   - What are all possible `status` values?
   - Confirmed: NEW, PENDING, WON, LOST, ACCEPTED
   - Are there others? (IN_ARBITRATION, WITHDRAWN, etc.)

3. **Dispute Types**:
   - What are all possible `disputeType` values?
   - Confirmed: "First Chargeback", "Pre-Arbitration"
   - Are there others? (Retrieval Request, Second Chargeback, etc.)

4. **Reason Codes**:
   - Complete list of reason codes by card brand?
   - Mapping guide for Visa, Mastercard, Amex, Discover?

5. **Response Deadlines**:
   - Is `respond_by_date` included in the API response?
   - If not, how do we calculate it?
   - Typical deadline (7 days, 10 days, varies by card brand)?

6. **Evidence Submission**:
   - Is there an API endpoint to submit evidence?
   - What format (JSON, multipart/form-data)?
   - File size limits for evidence documents?

7. **Webhooks**:
   - Is there a webhook alternative to polling?
   - Webhook URL registration process?
   - Webhook payload format?

8. **Pagination**:
   - Is there pagination support?
   - Maximum results per request?
   - How to request next page?

9. **Rate Limits**:
   - What are the rate limits for this API?
   - Recommended polling frequency?

10. **Alternative Search by External Key**:
    - What is "external key" in GET /merchant/disputes/key/search?
    - When would we use this instead of MID search?

## Implementation Checklist

- [ ] Contact North support with questions above
- [ ] Get authentication credentials/method
- [ ] Implement DisputeAdapter with proper auth
- [ ] Add `GetByGatewayTransactionID` method to TransactionRepository
- [ ] Implement SyncService
- [ ] Create scheduled job (cron or background service)
- [ ] Set up alerting system (email/Slack/PagerDuty)
- [ ] Test with North sandbox environment
- [ ] Configure sync frequency (hourly recommended)
- [ ] Set up monitoring for sync job health
- [ ] Document operational procedures

## Next Steps

1. Send updated email to North support (see `IMPLEMENTATION_CHECKLIST.md`)
2. Get authentication details for Dispute API
3. Implement DisputeAdapter
4. Set up hourly sync job
5. Configure alerts for new chargebacks
