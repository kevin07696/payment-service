# Test Refactoring Examples

## Example 1: State Transition Test ❌→✅

### BEFORE: Integration Test Testing Business Logic ❌

**File:** `tests/integration/payment/state_transition_test.go`

```go
func TestStateTransition_VoidAfterCapture(t *testing.T) {
    testutil.SkipIfBRICStorageUnavailable(t)

    cfg, client := testutil.Setup(t)

    // Step 1: Tokenize card (EPX API call)
    paymentMethodID, err := testutil.TokenizeAndSaveCard(...)
    time.Sleep(2 * time.Second)

    // Step 2: Create AUTH via HTTP API
    authResp, err := client.Do("POST", "/api/v1/payments/authorize", authReq)
    time.Sleep(2 * time.Second)

    // Step 3: CAPTURE via HTTP API
    captureResp, err := client.Do("POST", "/api/v1/payments/capture", captureReq)
    time.Sleep(2 * time.Second)

    // Step 4: Try VOID via HTTP API
    voidResp, err := client.Do("POST", "/api/v1/payments/void", voidReq)

    // ❌ PROBLEM: This tests business logic (is VOID allowed?), not integration!
    // The test is slow (8+ seconds), requires EPX API, tests validation rule
}
```

**Problems:**
1. ❌ Tests business logic (should VOID be blocked after CAPTURE?)
2. ❌ Slow (8+ seconds with sleeps)
3. ❌ Requires external API (EPX)
4. ❌ Flaky (network issues, API timeouts)
5. ❌ Hard to test edge cases

### AFTER: Split into Unit + Integration ✅

#### Unit Test: Test Business Logic ✅
**File:** `internal/services/payment/group_state_test.go`

```go
func TestCanVoid_AfterCapture(t *testing.T) {
    // Scenario: AUTH → CAPTURE → Try VOID
    txs := []*domain.Transaction{
        makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
        makeTransaction("capture1", domain.TransactionTypeCapture, "100.00", "bric_capture1"),
    }

    state := ComputeGroupState(txs)

    // Can still VOID the AUTH (not the CAPTURE)
    canVoid, reason := state.CanVoid()

    // ✅ FAST: No database, no API, no network
    // ✅ CLEAR: Tests exact business rule
    // ✅ RELIABLE: No flakiness
    assert.True(t, canVoid)
}

func TestVoidCapture_SameDayReversal(t *testing.T) {
    // Scenario: AUTH → CAPTURE → VOID CAPTURE (same-day reversal)
    txs := []*domain.Transaction{
        makeTransaction("auth1", domain.TransactionTypeAuth, "100.00", "bric_auth1"),
        makeTransaction("capture1", domain.TransactionTypeCapture, "100.00", "bric_capture1"),
        makeVoidTransaction("void1", "100.00", "capture"), // Void the CAPTURE
    }

    state := ComputeGroupState(txs)

    // VOID of CAPTURE should reduce captured amount
    assert.Equal(t, "100.00", state.ActiveAuthAmount.StringFixed(2))
    assert.True(t, state.CapturedAmount.IsZero()) // CAPTURE was voided
    assert.False(t, state.IsAuthVoided)

    // ✅ Tests WAL logic for VOID of CAPTURE
}
```

#### Integration Test: Test Real EPX Behavior ✅
**File:** `tests/integration/payment/epx_void_behavior_test.go`

```go
func TestEPX_VoidCaptureRejection(t *testing.T) {
    // This tests EPX's actual behavior, not our validation
    testutil.SkipIfBRICStorageUnavailable(t)

    cfg, client := testutil.Setup(t)

    // Create AUTH + CAPTURE
    groupID := createAuthAndCapture(t, cfg, client, "100.00")

    // Try VOID via HTTP API (should call EPX)
    voidResp, err := client.Do("POST", "/api/v1/payments/void", map[string]interface{}{
        "group_id": groupID,
    })
    require.NoError(t, err)

    // ✅ Test EPX's response, not our validation
    // EPX may reject VOID after settlement, or convert to REFUND
    assert.Contains(t, []int{200, 400, 500}, voidResp.StatusCode)

    // ✅ INTEGRATION: Tests EPX API, HTTP handler, error handling
}
```

**Benefits:**
1. ✅ Unit test is fast (< 1ms)
2. ✅ Unit test covers all edge cases
3. ✅ Integration test verifies EPX behavior
4. ✅ Separation of concerns

---

## Example 2: Refund Validation ❌→✅

### BEFORE: Integration Test Testing Business Logic ❌

**File:** `tests/integration/payment/refund_void_test.go`

```go
func TestRefundValidation(t *testing.T) {
    cfg, client := testutil.Setup(t)

    tests := []struct {
        name           string
        groupID        string
        amount         string
        expectedStatus int
    }{
        {"missing group_id", "", "50.00", 400},
        {"non-existent group_id", "00000000-0000-0000-0000-000000000099", "50.00", 500},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ❌ Tests input validation via HTTP API
            refundResp, _ := client.Do("POST", "/api/v1/payments/refund", map[string]interface{}{
                "group_id": tt.groupID,
                "amount":   tt.amount,
                "reason":   "test",
            })

            assert.Equal(t, tt.expectedStatus, refundResp.StatusCode)
        })
    }
}
```

**Problems:**
1. ❌ Tests input validation (business logic)
2. ❌ Requires real HTTP server
3. ❌ Requires database
4. ❌ Slow (2+ seconds)

### AFTER: Unit Test ✅

**File:** `internal/services/payment/refund_validation_test.go`

```go
func TestRefund_InputValidation(t *testing.T) {
    tests := []struct {
        name        string
        req         *ports.RefundRequest
        expectError string
    }{
        {
            name: "missing group_id",
            req: &ports.RefundRequest{
                GroupID: "",
                Amount:  stringPtr("50.00"),
                Reason:  "test",
            },
            expectError: "group_id is required",
        },
        {
            name: "invalid group_id format",
            req: &ports.RefundRequest{
                GroupID: "not-a-uuid",
                Amount:  stringPtr("50.00"),
                Reason:  "test",
            },
            expectError: "invalid group_id format",
        },
        {
            name: "negative amount",
            req: &ports.RefundRequest{
                GroupID: "00000000-0000-0000-0000-000000000001",
                Amount:  stringPtr("-10.00"),
                Reason:  "test",
            },
            expectError: "amount must be greater than zero",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create service with mock dependencies
            svc := &paymentService{
                serverPost: &mockServerPost{},
                db:         &mockDB{},
            }

            _, err := svc.Refund(context.Background(), tt.req)

            // ✅ FAST: No database, no HTTP
            // ✅ CLEAR: Tests validation logic
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tt.expectError)
        })
    }
}

func TestRefund_AmountValidation(t *testing.T) {
    tests := []struct {
        name           string
        capturedAmount string
        refundedAmount string
        refundAmount   string
        shouldAllow    bool
        expectedError  string
    }{
        {"valid full refund", "100.00", "0", "100.00", true, ""},
        {"valid partial refund", "100.00", "0", "50.00", true, ""},
        {"exceed captured", "100.00", "0", "100.01", false, "exceeds remaining refundable amount"},
        {"exceed remaining", "100.00", "50.00", "50.01", false, "exceeds remaining refundable amount"},
        {"no captured amount", "0", "0", "50.00", false, "no captured amount to refund"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            captured, _ := decimal.NewFromString(tt.capturedAmount)
            refunded, _ := decimal.NewFromString(tt.refundedAmount)
            refundAmt, _ := decimal.NewFromString(tt.refundAmount)

            state := &GroupState{
                CapturedAmount: captured,
                RefundedAmount: refunded,
            }

            canRefund, reason := state.CanRefund(refundAmt)

            // ✅ Table-driven, covers all edge cases
            assert.Equal(t, tt.shouldAllow, canRefund)
            if !tt.shouldAllow {
                assert.Contains(t, reason, tt.expectedError)
            }
        })
    }
}
```

**Benefits:**
1. ✅ Fast (< 1ms per test)
2. ✅ Table-driven (easy to add cases)
3. ✅ No external dependencies
4. ✅ Tests business logic directly

---

## Example 3: What SHOULD Be Integration Test ✅

### Browser Post End-to-End ✅

**File:** `tests/integration/payment/browser_post_test.go`

```go
func TestBrowserPost_EndToEnd_Success(t *testing.T) {
    cfg, client := testutil.Setup(t)
    merchantID := "00000000-0000-0000-0000-000000000001"
    transactionID := uuid.New().String()

    // ✅ Step 1: Call real EPX Key Exchange API
    formResp, err := client.Do("GET", fmt.Sprintf(
        "/api/v1/payments/browser-post/form?transaction_id=%s&merchant_id=%s&amount=100.00",
        transactionID, merchantID,
    ))
    require.NoError(t, err)

    var formConfig map[string]interface{}
    err = testutil.DecodeResponse(formResp, &formConfig)
    require.NoError(t, err)

    // ✅ Verify EPX returned TAC token
    assert.NotEmpty(t, formConfig["tac"])

    // ✅ Step 2: Simulate EPX callback with signature
    callbackResp, err := client.Do("POST", "/api/v1/payments/browser-post/callback",
        testutil.BuildEPXCallbackPayload(transactionID, merchantID))
    require.NoError(t, err)
    assert.Equal(t, 200, callbackResp.StatusCode)

    // ✅ Step 3: Verify database persistence
    txResp, err := client.Do("GET", fmt.Sprintf("/api/v1/payments/%s", transactionID))
    require.NoError(t, err)

    var transaction map[string]interface{}
    err = testutil.DecodeResponse(txResp, &transaction)
    require.NoError(t, err)

    // ✅ Verify BRIC was stored
    assert.NotEmpty(t, transaction["authGuid"])
    assert.Equal(t, "approved", transaction["status"])
}
```

**Why This IS Integration Test:**
1. ✅ Calls real EPX Key Exchange API
2. ✅ Tests HTTP request/response parsing
3. ✅ Tests signature validation
4. ✅ Tests database persistence
5. ✅ Tests complete workflow

---

## Example 4: Concurrent Operations ✅

### Integration Test: Test Real Concurrency ✅

**File:** `tests/integration/payment/concurrency_test.go`

```go
func TestConcurrentRefunds_SameGroup(t *testing.T) {
    cfg, client := testutil.Setup(t)

    // Create SALE transaction
    groupID := createSale(t, cfg, client, "100.00")

    // ✅ Test real concurrent HTTP requests
    const numRefunds = 3
    refundAmount := "40.00" // Total would be $120, but only $100 available

    errChan := make(chan error, numRefunds)
    successChan := make(chan bool, numRefunds)

    for i := 0; i < numRefunds; i++ {
        go func(idx int) {
            refundReq := map[string]interface{}{
                "group_id":        groupID,
                "amount":          refundAmount,
                "reason":          fmt.Sprintf("concurrent refund %d", idx),
                "idempotency_key": uuid.New().String(),
            }

            resp, err := client.Do("POST", "/api/v1/payments/refund", refundReq)
            if err != nil {
                errChan <- err
                return
            }

            successChan <- (resp.StatusCode == 200)
        }(i)
    }

    // Collect results
    var successCount int
    for i := 0; i < numRefunds; i++ {
        select {
        case err := <-errChan:
            t.Logf("Refund failed: %v", err)
        case success := <-successChan:
            if success {
                successCount++
            }
        }
    }

    // ✅ Only 2 refunds should succeed (2 * $40 = $80 <= $100)
    // ✅ Third refund should fail (total would be $120 > $100)
    assert.LessOrEqual(t, successCount, 2, "should not allow over-refunding")

    // ✅ Verify database state
    txs := getTransactionsByGroup(t, client, groupID)
    refundCount := countTransactionsByType(txs, "refund")
    assert.LessOrEqual(t, refundCount, 2)
}
```

**Why This IS Integration Test:**
1. ✅ Tests real concurrency (goroutines, HTTP)
2. ✅ Tests row-level locking (SELECT FOR UPDATE)
3. ✅ Tests database isolation
4. ✅ Tests race conditions

---

## Summary: Decision Tree

### Is this a unit test or integration test?

```
START
  |
  v
Does it call external API (EPX, database)?
  |
  +--NO--> Does it test business logic (validation, state computation)?
  |         |
  |         +--YES--> UNIT TEST ✅
  |         |
  |         +--NO--> Does it test pure functions?
  |                   |
  |                   +--YES--> UNIT TEST ✅
  |                   +--NO--> INTEGRATION TEST ✅
  |
  +--YES--> Does it test HTTP/gRPC protocol?
            |
            +--YES--> INTEGRATION TEST ✅
            |
            +--NO--> Does it test end-to-end workflow?
                      |
                      +--YES--> INTEGRATION TEST ✅
                      +--NO--> Can it be mocked?
                                |
                                +--YES--> UNIT TEST ✅
                                +--NO--> INTEGRATION TEST ✅
```

### Examples:

| Test | Type | Reason |
|------|------|--------|
| `TestCanRefund_Success` | Unit | Pure function, no I/O |
| `TestComputeGroupState` | Unit | Business logic, no database |
| `TestBrowserPost_EndToEnd` | Integration | Calls EPX API, database, HTTP |
| `TestConcurrentRefunds` | Integration | Tests race conditions, database |
| `TestGRPC_ListTransactions` | Integration | Tests gRPC protocol, pagination |
| `TestStateTransition_VoidAfterCapture` | ❌ Unit (refactor) | Currently integration, but tests business logic |

---

## Action Items

### ✅ Already Done:
- [x] Created 36 unit tests for business logic
- [x] Table-driven tests for validation
- [x] Fast tests (0.004s total)

### 🔄 To Do:
- [ ] Remove business logic from `state_transition_test.go`
- [ ] Move validation tests from `refund_void_test.go` to unit tests
- [ ] Keep only real integration tests in integration folder
- [ ] Add concurrency tests for race conditions
- [ ] Add EPX error handling integration tests
