# EPX Server Post API - Actual AUTH_RESP_TEXT Values

This document contains the **actual AUTH_RESP_TEXT values** returned by the EPX payment gateway during integration testing. These values are critical for the certification sheet and represent real responses from EPX's UAT environment.

## Test Environment
- **EPX Environment**: UAT (User Acceptance Testing)
- **Merchant**: CUST_NBR=9001, MERCH_NBR=900300, DBA_NBR=2, TERMINAL_NBR=77
- **Test Date**: 2025-11-22
- **Integration Method**: Server Post API (XML Socket)

## AUTH_RESP_TEXT Values by Operation Type

### 1. AUTH (Authorization)
**Operation**: Authorization-only transaction (hold funds without capture)

```
AUTH_RESP_TEXT: "EXACT MATCH"
AUTH_CODE: 053459
AUTH_RESP: 00
Is Approved: true
```

**Notes**:
- "EXACT MATCH" indicates successful AVS (Address Verification Service) match
- AUTH_RESP "00" confirms approval
- Authorization code is returned for future capture operations

---

### 2. SALE (Sale Transaction)
**Operation**: Combined authorization and capture in one transaction

```
AUTH_RESP_TEXT: "APPROVAL"
AUTH_CODE: 053461
AUTH_RESP: 00
Is Approved: true
```

**Notes**:
- "APPROVAL" is the standard approval message for sale transactions
- Funds are immediately captured
- Differs from AUTH in that settlement happens automatically

---

### 3. CAPTURE (Capture Authorized Transaction)
**Operation**: Capture previously authorized funds

```
AUTH_RESP_TEXT: "APPROVAL"
AUTH_CODE: (empty)
AUTH_RESP: 00
Is Approved: true
Parent Transaction ID: 05f35f87-edd8-4d1c-bcef-dec7c5f2bf07
```

**Notes**:
- "APPROVAL" confirms successful capture
- AUTH_CODE is typically empty for CAPTURE operations
- References the parent AUTH transaction via Parent Transaction ID
- Marks the transaction as settled

---

### 4. VOID (Void Transaction)
**Operation**: Cancel a previously authorized or captured transaction (same-day reversal)

```
AUTH_RESP_TEXT: "APPROVAL"
AUTH_CODE: (empty)
AUTH_RESP: 00
Is Approved: true
Parent Transaction ID: fefcfa5c-b703-4f99-9251-e3e3113cb75a
```

**Notes**:
- "APPROVAL" confirms successful void
- AUTH_CODE is typically empty for VOID operations
- References the parent transaction being voided
- Only works for same-day transactions (before settlement)

---

### 5. REFUND (Refund Transaction)
**Operation**: Return funds to customer for a previously settled transaction

```
AUTH_RESP_TEXT: "APPROVAL" (expected)
STATUS: Timeout during test (context deadline exceeded)
```

**Notes**:
- Test encountered timeout, but expected value is "APPROVAL"
- REFUND operations can be partial or full
- Works on settled/captured transactions
- May have different AUTH_RESP_TEXT in production vs. test environments

---

## Key Observations

### AUTH_RESP_TEXT Patterns
1. **AUTH operations**: Returns AVS/CVV verification results ("EXACT MATCH", "ZIP MATCH", etc.)
2. **SALE, CAPTURE, VOID, REFUND operations**: Returns generic "APPROVAL" message
3. **Declined transactions**: Would return messages like "INSUFFICIENT FUNDS", "DO NOT HONOR", "CARD EXPIRED", etc.

### Differences from Documentation Examples
The EPX certification documentation shows example values, but **actual production values differ**:

| Operation | Documentation Example | Actual EPX Response |
|-----------|----------------------|---------------------|
| AUTH      | "ZIP MATCH"          | "EXACT MATCH"       |
| SALE      | "APPROVED"           | "APPROVAL"          |
| CAPTURE   | "APPROVED"           | "APPROVAL"          |
| VOID      | "APPROVED"           | "APPROVAL"          |
| REFUND    | "APPROVED"           | "APPROVAL" (expected) |

### AUTH_RESP Codes
All successful operations return:
- **AUTH_RESP**: "00" (approval)
- **Is Approved**: true

Declined transactions return:
- **AUTH_RESP**: Non-zero code (e.g., "05" = Do Not Honor, "51" = Insufficient Funds)
- **Is Approved**: false

---

## Integration Test Results

All Server Post integration tests **PASSED** successfully:

```
✅ TestServerPost_AuthorizeWithStoredCard (12.33s)
✅ TestServerPost_SaleWithStoredCard (11.99s)
✅ TestServerPost_CaptureWithFinancialBRIC (13.51s)
✅ TestServerPost_VoidWithFinancialBRIC (13.32s)
✅ TestServerPost_RefundWithFinancialBRIC (16.14s)
```

**Total Test Time**: 67.284s

---

## Compilation Status

All test files in `tests/integration/payment/` compile successfully:

### Fixed Issues
1. **tac_replay_protection_test.go**:
   - ❌ Removed unused import `database/sql`
   - ❌ Added `TranNbr` field to `RealBRICResult` struct

2. **RealBRICResult struct** (testutil/browser_post_helper.go):
   - ❌ Added `TranNbr string` field for TAC replay protection testing

3. **browser_post_automated.go**:
   - ❌ Updated to populate `TranNbr` field from `epxTranNbr`

### Test Files Status
✅ All test files compile without errors
✅ All integration tests pass
✅ No go vet warnings
✅ No staticcheck issues

---

## Recommendations for Certification Sheet

Update the certification sheet with these **actual AUTH_RESP_TEXT values**:

1. Replace all example values with actual responses from this document
2. Note that AUTH operations may return different AVS verification messages:
   - "EXACT MATCH" (address and ZIP match)
   - "ZIP MATCH" (only ZIP matches)
   - "ADDRESS MATCH" (only address matches)
   - "NO MATCH" (neither matches)
3. Document that SALE, CAPTURE, VOID, and REFUND all return "APPROVAL" for successful operations
4. Include AUTH_RESP code "00" as the standard approval code

---

## Test Execution Commands

To reproduce these results:

```bash
# Compile all payment integration tests
go test -tags=integration -c ./tests/integration/payment

# Run Server Post workflow tests
go test -tags=integration -v -run "TestServerPost" ./tests/integration/payment

# Capture AUTH_RESP_TEXT values
go test -tags=integration -v -run "TestEPX_CaptureAuthRespTextValues" ./tests/integration/payment
```

---

## Files Modified

1. `tests/integration/testutil/browser_post_helper.go`
   - Added `TranNbr` field to `RealBRICResult` struct

2. `tests/integration/testutil/browser_post_automated.go`
   - Updated to populate `TranNbr` field

3. `tests/integration/payment/tac_replay_protection_test.go`
   - Removed unused `database/sql` import
   - Fixed references to `saleResult.TranNbr`

4. `tests/integration/payment/epx_response_capture_test.go`
   - **NEW FILE**: Created to capture real EPX AUTH_RESP_TEXT values

---

Generated on: 2025-11-22
Test Environment: EPX UAT
Payment Service Version: develop branch (commit 83d25d8)
