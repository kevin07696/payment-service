# ACH Bank Account Verification Guide

## Overview

**ACH verification** is the process of confirming that a customer's bank account is valid and belongs to them before processing ACH payments. This prevents ACH returns, reduces fraud, and improves payment success rates.

**Status:** ✅ Can be implemented immediately (ACHAdapter already exists)

## Why Verify Bank Accounts?

### Benefits

**1. Reduce ACH Returns**
- Invalid account numbers cause returns (R03, R04)
- Returns cost $2-5 per transaction
- Too many returns = NACHA penalties
- Verification prevents 80-90% of invalid account returns

**2. Prevent Fraud**
- Confirm customer owns the account
- Reduces fraudulent ACH debits
- Required for high-risk transactions

**3. Better Customer Experience**
- Catch errors before first payment attempt
- Avoid failed payments and customer confusion
- Faster resolution of account issues

**4. Compliance**
- NACHA requires "commercially reasonable" fraud prevention
- Verification demonstrates due diligence

### Drawbacks

**1. Time Delay (Micro-deposits)**
- 2-3 business days for deposits to appear
- Customer must return to confirm
- Potential drop-off during waiting period

**2. Cost (Instant Verification)**
- Plaid/Yodlee charge per verification ($0.10-0.30)
- May be worth it for better UX

**3. Implementation Complexity**
- Micro-deposits: database state management
- Instant: third-party integration

## Verification Methods

### Method 1: Micro-Deposits (Traditional)

**How it works:**
1. Send 2 small deposits ($0.01 - $0.99) to customer's account
2. Wait 2-3 business days for deposits to appear
3. Customer confirms amounts in your app
4. Verify amounts match → account verified

**Pros:**
- No third-party dependency
- Works with any US bank
- Low cost (just ACH fees: ~$0.25 total)

**Cons:**
- Slow (2-3 days)
- Customer must return to confirm
- ~30% drop-off rate during waiting period

**Best for:**
- Lower-value recurring payments
- Non-urgent verification
- Cost-sensitive implementations

### Method 2: Instant Verification (Plaid/Yodlee)

**How it works:**
1. Customer clicks "Connect Bank Account"
2. Plaid widget opens
3. Customer logs into their bank
4. Plaid verifies account in real-time
5. Returns verified account token

**Pros:**
- Instant (< 30 seconds)
- Better customer experience
- Lower drop-off rate (~5%)
- Can retrieve balance info

**Cons:**
- Third-party dependency
- Cost per verification ($0.10-0.30)
- Not all banks supported (though 95%+ coverage)

**Best for:**
- One-time high-value payments
- Better UX requirements
- Subscription onboarding

### Method 3: Account Validation API (North)

**How it works:**
1. Customer enters routing + account number
2. Backend calls North validation API
3. North checks routing number validity
4. Returns account type (checking/savings)

**Pros:**
- Instant
- Very low cost
- No third-party dependency

**Cons:**
- Only validates format, not ownership
- Doesn't verify customer owns the account
- Limited fraud prevention

**Best for:**
- First-level validation (before micro-deposits/Plaid)
- Catching typos
- Low-risk transactions

## Implementation

### Method 1: Micro-Deposits Implementation

#### Step 1: Database Schema

Add verification table:

```sql
-- internal/db/migrations/000004_add_bank_account_verification.up.sql

CREATE TABLE bank_account_verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100) NOT NULL,
    routing_number VARCHAR(9) NOT NULL,
    account_number_last4 VARCHAR(4) NOT NULL,  -- Only store last 4 digits
    account_type VARCHAR(20) NOT NULL,  -- checking, savings
    status VARCHAR(20) NOT NULL,  -- pending, verified, failed
    deposit_amount1 NUMERIC(5, 2),  -- e.g., 0.32
    deposit_amount2 NUMERIC(5, 2),  -- e.g., 0.54
    attempts_remaining INT NOT NULL DEFAULT 3,
    verified_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    INDEX idx_customer_id (customer_id),
    INDEX idx_status (status),
    INDEX idx_expires_at (expires_at)
);
```

#### Step 2: Proto Definition

```protobuf
// api/proto/payment/v1/payment.proto

message InitiateBankVerificationRequest {
    string merchant_id = 1;
    string customer_id = 2;
    string routing_number = 3;
    string account_number = 4;
    string account_type = 5;  // "checking" or "savings"
}

message InitiateBankVerificationResponse {
    string verification_id = 1;
    string status = 2;  // "pending"
    google.protobuf.Timestamp expected_completion = 3;  // ~3 days from now
}

message ConfirmBankVerificationRequest {
    string verification_id = 1;
    string amount1 = 2;  // e.g., "0.32"
    string amount2 = 3;  // e.g., "0.54"
}

message ConfirmBankVerificationResponse {
    string verification_id = 1;
    string status = 2;  // "verified" or "failed"
    int32 attempts_remaining = 3;
}

service PaymentService {
    // ... existing methods ...

    rpc InitiateBankVerification(InitiateBankVerificationRequest) returns (InitiateBankVerificationResponse);
    rpc ConfirmBankVerification(ConfirmBankVerificationRequest) returns (ConfirmBankVerificationResponse);
}
```

#### Step 3: Service Layer

```go
// internal/services/payment/bank_verification.go

type BankVerificationService struct {
    dbPort      ports.DBPort
    achGateway  ports.ACHGateway
    verifyRepo  ports.BankVerificationRepository
    logger      ports.Logger
}

func (s *BankVerificationService) InitiateVerification(
    ctx context.Context,
    req *ports.InitiateBankVerificationRequest,
) (*ports.BankVerificationResponse, error) {
    // 1. Validate routing number format
    if !s.isValidRoutingNumber(req.RoutingNumber) {
        return nil, pkgerrors.NewValidationError("routing_number", "Invalid routing number")
    }

    // 2. Generate two random amounts between $0.01 and $0.99
    amount1 := s.generateRandomAmount()
    amount2 := s.generateRandomAmount()

    // 3. Send micro-deposits via ACH
    depositReq := &ports.ACHRequest{
        MerchantID:     req.MerchantID,
        CustomerID:     req.CustomerID,
        Amount:         amount1,
        Currency:       "USD",
        RoutingNumber:  req.RoutingNumber,
        AccountNumber:  req.AccountNumber,
        AccountType:    req.AccountType,
        TransactionType: "CKC3",  // Credit (deposit)
        Description:    "Bank account verification deposit 1/2",
    }

    _, err := s.achGateway.ProcessPayment(ctx, depositReq)
    if err != nil {
        s.logger.Error("Failed to send first micro-deposit", ports.Error(err))
        return nil, err
    }

    depositReq.Amount = amount2
    depositReq.Description = "Bank account verification deposit 2/2"

    _, err = s.achGateway.ProcessPayment(ctx, depositReq)
    if err != nil {
        s.logger.Error("Failed to send second micro-deposit", ports.Error(err))
        return nil, err
    }

    // 4. Store verification record
    verification := &models.BankAccountVerification{
        ID:                 generateID(),
        MerchantID:         req.MerchantID,
        CustomerID:         req.CustomerID,
        RoutingNumber:      req.RoutingNumber,
        AccountNumberLast4: req.AccountNumber[len(req.AccountNumber)-4:],
        AccountType:        req.AccountType,
        Status:             models.VerificationStatusPending,
        DepositAmount1:     amount1,
        DepositAmount2:     amount2,
        AttemptsRemaining:  3,
        ExpiresAt:          time.Now().AddDate(0, 0, 7), // 7 days to confirm
        CreatedAt:          time.Now(),
    }

    err = s.verifyRepo.Create(ctx, verification)
    if err != nil {
        return nil, err
    }

    return &ports.BankVerificationResponse{
        VerificationID:       verification.ID,
        Status:               string(verification.Status),
        ExpectedCompletion:   time.Now().AddDate(0, 0, 3), // 3 business days
    }, nil
}

func (s *BankVerificationService) ConfirmVerification(
    ctx context.Context,
    req *ports.ConfirmBankVerificationRequest,
) (*ports.BankVerificationResponse, error) {
    // 1. Get verification record
    verification, err := s.verifyRepo.GetByID(ctx, req.VerificationID)
    if err != nil {
        return nil, err
    }

    // 2. Check if expired
    if time.Now().After(verification.ExpiresAt) {
        verification.Status = models.VerificationStatusFailed
        s.verifyRepo.Update(ctx, verification)
        return nil, pkgerrors.NewValidationError("verification", "Verification expired")
    }

    // 3. Check if already verified
    if verification.Status == models.VerificationStatusVerified {
        return &ports.BankVerificationResponse{
            VerificationID: verification.ID,
            Status:         string(verification.Status),
        }, nil
    }

    // 4. Check attempts remaining
    if verification.AttemptsRemaining <= 0 {
        verification.Status = models.VerificationStatusFailed
        s.verifyRepo.Update(ctx, verification)
        return nil, pkgerrors.NewValidationError("verification", "Too many failed attempts")
    }

    // 5. Verify amounts
    if s.amountsMatch(req.Amount1, verification.DepositAmount1) &&
       s.amountsMatch(req.Amount2, verification.DepositAmount2) {
        // Success!
        verification.Status = models.VerificationStatusVerified
        verification.VerifiedAt = timePtr(time.Now())
    } else {
        // Failed attempt
        verification.AttemptsRemaining--

        if verification.AttemptsRemaining == 0 {
            verification.Status = models.VerificationStatusFailed
        }
    }

    err = s.verifyRepo.Update(ctx, verification)
    if err != nil {
        return nil, err
    }

    return &ports.BankVerificationResponse{
        VerificationID:    verification.ID,
        Status:            string(verification.Status),
        AttemptsRemaining: verification.AttemptsRemaining,
    }, nil
}

func (s *BankVerificationService) generateRandomAmount() decimal.Decimal {
    // Generate amount between $0.01 and $0.99
    cents := rand.Intn(99) + 1  // 1 to 99
    return decimal.NewFromFloat(float64(cents) / 100.0)
}

func (s *BankVerificationService) amountsMatch(input string, expected decimal.Decimal) bool {
    inputDecimal, err := decimal.NewFromString(input)
    if err != nil {
        return false
    }
    return inputDecimal.Equal(expected)
}
```

#### Step 4: Frontend

```javascript
// 1. Initiate verification
async function initiateBankVerification(bankAccount) {
    const response = await fetch('/api/payment/bank/verify/initiate', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + userJwt
        },
        body: JSON.stringify({
            merchantId: 'MERCH-001',
            customerId: 'CUST-12345',
            routingNumber: bankAccount.routingNumber,
            accountNumber: bankAccount.accountNumber,
            accountType: bankAccount.accountType
        })
    });

    const result = await response.json();

    // Show confirmation to user
    alert('We've sent 2 small deposits to your account. ' +
          'They should appear in 2-3 business days. ' +
          'Please return to confirm the amounts.');

    // Store verification ID
    localStorage.setItem('verificationId', result.verificationId);

    return result;
}

// 2. Confirm verification (user returns after 2-3 days)
async function confirmBankVerification(amount1, amount2) {
    const verificationId = localStorage.getItem('verificationId');

    const response = await fetch('/api/payment/bank/verify/confirm', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + userJwt
        },
        body: JSON.stringify({
            verificationId: verificationId,
            amount1: amount1,
            amount2: amount2
        })
    });

    const result = await response.json();

    if (result.status === 'verified') {
        alert('Bank account verified! You can now make payments.');
    } else if (result.attemptsRemaining > 0) {
        alert(`Incorrect amounts. ${result.attemptsRemaining} attempts remaining.`);
    } else {
        alert('Verification failed. Please add your bank account again.');
    }

    return result;
}
```

### Method 2: Instant Verification (Plaid) Implementation

#### Step 1: Sign Up for Plaid

1. Create account at https://plaid.com
2. Get client ID and secret
3. Install Plaid SDK:
```bash
npm install react-plaid-link
```

#### Step 2: Frontend Integration

```javascript
import { usePlaidLink } from 'react-plaid-link';

function BankAccountConnect() {
    const { open, ready } = usePlaidLink({
        token: linkToken,  // Get from backend
        onSuccess: (public_token, metadata) => {
            // Exchange public token for access token
            exchangePublicToken(public_token, metadata);
        },
        onExit: (err, metadata) => {
            if (err) {
                console.error('Plaid error:', err);
            }
        },
    });

    return (
        <button onClick={() => open()} disabled={!ready}>
            Connect Bank Account
        </button>
    );
}

async function exchangePublicToken(publicToken, metadata) {
    const response = await fetch('/api/payment/bank/plaid/exchange', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            publicToken: publicToken,
            accountId: metadata.account_id,
            institutionId: metadata.institution.institution_id
        })
    });

    const result = await response.json();

    if (result.verified) {
        alert('Bank account connected and verified!');
    }
}
```

#### Step 3: Backend Integration

```go
// internal/services/payment/plaid_verification.go

import (
    "github.com/plaid/plaid-go/v10/plaid"
)

type PlaidVerificationService struct {
    plaidClient *plaid.APIClient
    logger      ports.Logger
}

func (s *PlaidVerificationService) CreateLinkToken(ctx context.Context, customerID string) (string, error) {
    request := plaid.NewLinkTokenCreateRequest(
        "Payment App",
        "en",
        []plaid.CountryCode{plaid.COUNTRYCODE_US},
        plaid.LinkTokenCreateRequestUser{
            ClientUserId: customerID,
        },
    )
    request.SetProducts([]plaid.Products{plaid.PRODUCTS_AUTH})

    resp, _, err := s.plaidClient.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*request).Execute()
    if err != nil {
        return "", err
    }

    return resp.GetLinkToken(), nil
}

func (s *PlaidVerificationService) ExchangePublicToken(
    ctx context.Context,
    publicToken string,
) (*ports.PlaidBankAccount, error) {
    // Exchange public token for access token
    exchangeResp, _, err := s.plaidClient.PlaidApi.ItemPublicTokenExchange(ctx).
        ItemPublicTokenExchangeRequest(*plaid.NewItemPublicTokenExchangeRequest(publicToken)).
        Execute()
    if err != nil {
        return nil, err
    }

    accessToken := exchangeResp.GetAccessToken()

    // Get account details
    authResp, _, err := s.plaidClient.PlaidApi.AuthGet(ctx).
        AuthGetRequest(*plaid.NewAuthGetRequest(accessToken)).
        Execute()
    if err != nil {
        return nil, err
    }

    account := authResp.GetAccounts()[0]
    numbers := authResp.GetNumbers().GetAch()[0]

    return &ports.PlaidBankAccount{
        AccountID:     account.GetAccountId(),
        RoutingNumber: numbers.GetRouting(),
        AccountNumber: numbers.GetAccount(),
        AccountType:   string(account.GetSubtype()),
        Verified:      true,
    }, nil
}
```

## Testing

### Micro-Deposits Testing

**Test Flow:**
1. Initiate verification with test routing/account
2. Check database for deposit amounts
3. Manually confirm amounts
4. Verify status changes to "verified"

**Test Cases:**
- Correct amounts on first try → verified
- Wrong amounts → attempts decrement
- 3 wrong attempts → failed
- Expired verification → failed
- Already verified → return success

### Plaid Testing

**Test Credentials:**
```
Institution: Plaid Sandbox
Username: user_good
Password: pass_good
```

**Test Cases:**
- Successful connection → account details returned
- User cancels → handle gracefully
- Institution error → retry or fallback
- Unsupported bank → offer micro-deposits

## Best Practices

### 1. Combine Methods

```go
// Offer both methods, user chooses
func (s *Service) InitiateVerification(req *VerificationRequest) {
    if req.Method == "instant" && s.plaidEnabled {
        return s.plaidVerification.CreateLinkToken(req.CustomerID)
    } else {
        return s.microDepositVerification.Initiate(req)
    }
}
```

### 2. Expiration Handling

```go
// Auto-expire old verifications (run daily)
func (s *Service) ExpireOldVerifications(ctx context.Context) error {
    expired, err := s.verifyRepo.FindExpired(ctx, time.Now())
    if err != nil {
        return err
    }

    for _, v := range expired {
        v.Status = models.VerificationStatusExpired
        s.verifyRepo.Update(ctx, v)
    }

    return nil
}
```

### 3. Security

```go
// NEVER store full account number
type BankAccount struct {
    RoutingNumber     string
    AccountNumberLast4 string  // Only last 4 digits
    AccountNumberHash  string  // Hash for matching
}

func hashAccountNumber(accountNumber string) string {
    hash := sha256.Sum256([]byte(accountNumber))
    return hex.EncodeToString(hash[:])
}
```

## Metrics

```go
// Track verification metrics
bank_verification_initiated_total{method="micro_deposit"} 150
bank_verification_initiated_total{method="plaid"} 350

bank_verification_completed_total{result="success"} 420
bank_verification_completed_total{result="failed"} 50
bank_verification_completed_total{result="expired"} 30

bank_verification_duration_seconds{method="micro_deposit"} 259200  // 3 days
bank_verification_duration_seconds{method="plaid"} 45  // 45 seconds
```

## Next Steps

1. **Choose Verification Method(s)**
   - Micro-deposits: Low cost, slower
   - Plaid: Better UX, small cost per verification
   - Hybrid: Offer both, user chooses

2. **Implement Backend**
   - Add database schema
   - Implement verification service
   - Add gRPC endpoints

3. **Implement Frontend**
   - Add bank account form
   - Integrate Plaid Link (if using instant)
   - Add confirmation flow (if using micro-deposits)

4. **Test Thoroughly**
   - Test all verification methods
   - Test error cases
   - Test expiration handling

5. **Deploy**
   - Start with one method
   - Monitor completion rates
   - Add second method if needed

## Resources

- **NACHA Operating Rules:** https://www.nacha.org/rules
- **Plaid Documentation:** https://plaid.com/docs/
- **ACH Return Codes:** https://www.nacha.org/content/ach-return-codes

## Status

**Current Status:** ✅ Ready to implement

**Recommendation:** Start with micro-deposits (simpler), add Plaid if drop-off is too high

**Priority:** High (reduces ACH returns and fraud)
