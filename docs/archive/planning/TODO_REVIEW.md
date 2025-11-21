# TODO Review - Stale vs Valid

**Date:** 2025-11-20
**Total TODOs Found:** 33

## ✅ STALE TODOs (Should be Removed/Updated) - 11

### Browser Post STORAGE - Already Implemented!

**FACT:** `TokenizeAndSaveCardViaBrowserPost` is **FULLY IMPLEMENTED** at `tests/integration/testutil/tokenization.go:308-377`

**FACT:** Browser Post STORAGE flow is **WORKING** - used successfully in `server_post_workflow_test.go` (6 tests)

**STALE TODOs claiming it needs implementation:**

1. ❌ `tests/integration/payment_method/payment_method_test.go:20`
   ```
   t.Skip("TODO: Update to use Browser Post STORAGE flow with TokenizeAndSaveCardViaBrowserPost")
   ```
   **Status:** STALE - The function exists! Just update the test to use it.

2. ❌ `tests/integration/payment_method/payment_method_test.go:25`
   ```
   // TODO: Replace with TokenizeAndSaveCardViaBrowserPost
   ```
   **Status:** STALE - Just replace the call.

3. ❌ `tests/integration/payment_method/payment_method_test.go:91`
   ```
   t.Skip("TODO: Update to use Browser Post STORAGE flow - depends on deprecated TokenizeAndSaveCard")
   ```
   **Status:** STALE - TokenizeAndSaveCardViaBrowserPost exists!

4. ❌ `tests/integration/payment_method/payment_method_test.go:129`
   ```
   t.Skip("TODO: Update to use Browser Post STORAGE flow - depends on deprecated TokenizeAndSaveCard")
   ```
   **Status:** STALE - Same as above.

5. ❌ `tests/integration/payment_method/payment_method_test.go:168`
   ```
   t.Skip("TODO: Update to use Browser Post STORAGE flow - depends on deprecated TokenizeAndSaveCard")
   ```
   **Status:** STALE - Same as above.

6. ❌ `tests/integration/payment_method/payment_method_test.go:269`
   ```
   t.Skip("TODO: Update to use Browser Post STORAGE flow with TokenizeAndSaveCardViaBrowserPost")
   ```
   **Status:** STALE - Function exists!

7. ❌ `tests/integration/testutil/tokenization.go:392`
   ```
   // TODO: Update to use Browser Post STORAGE flow
   ```
   **Status:** STALE - This is IN the deprecated `TokenizeAndSaveCard` function, which already says "use TokenizeAndSaveCardViaBrowserPost instead"

8. ❌ `tests/integration/payment/browser_post_workflow_test.go:224`
   ```
   // TODO: Implement SALE with stored BRIC when STORAGE endpoint is available
   ```
   **Status:** STALE - STORAGE endpoint IS available and working!

### Deprecated Feature References

9. ❌ `tests/integration/payment_method/payment_method_test.go:216`
   ```
   t.Skip("TODO: Update to use ConnectRPC StorePaymentMethod endpoint")
   ```
   **Status:** UNCLEAR - There's no `StorePaymentMethod` RPC. Should probably use `TokenizeAndSaveCardViaBrowserPost` instead.

### EPX Sandbox BRICs

10. ❌ `tests/integration/fixtures/epx_brics.go:24`
    ```
    // TODO: Replace with real BRIC from EPX sandbox
    ```
    **Status:** LOW PRIORITY - These are test fixtures. If tests work with mock BRICs, this is fine.

11. ❌ `tests/integration/fixtures/epx_brics.go:37`
    ```
    // TODO: Replace with real BRIC from EPX sandbox
    ```
    **Status:** LOW PRIORITY - Same as above.

---

## ⚠️ VALID TODOs (Features Not Implemented) - 22

### ACH Account Storage - Actually Not Implemented

**FACT:** `StoreACHAccount` RPC exists but returns `CodeUnimplemented` at `internal/handlers/payment_method/payment_method_handler_connect.go:255`

**FACT:** `TokenizeAndSaveACH` stub returns error "not yet implemented" at `tests/integration/testutil/tokenization.go:388`

**VALID TODOs (13 total):**

1. ✅ `internal/handlers/payment_method/payment_method_handler_connect.go:250`
   ```
   // TODO: Implement ACH account storage functionality
   ```
   **Status:** VALID - Handler returns Unimplemented

2. ✅ `tests/integration/testutil/tokenization.go:386`
   ```
   // TODO: Implement when StoreACHAccount RPC is available
   ```
   **Status:** VALID - RPC not implemented

**ACH Test Skips (11 more):**
- `tests/integration/cron/ach_verification_cron_test.go:20, 109, 184` (3 tests)
- `tests/integration/payment/payment_ach_verification_test.go:24, 53, 102, 159, 216` (5 tests)
- `tests/integration/payment_method/payment_method_test.go:56, 61` (2 tests)

All say: `t.Skip("TODO: Update to use StoreACHAccount RPC once implemented (currently returns Unimplemented)")`

**Status:** VALID - Waiting for StoreACHAccount implementation

### Admin/Audit Features

3. ✅ `internal/handlers/admin/service_handler.go:77`
   ```
   // TODO: Get admin ID from auth context
   ```
   **Status:** VALID - Not implemented

4. ✅ `internal/handlers/admin/service_handler.go:84`
   ```
   // TODO: Audit log the service creation
   ```
   **Status:** VALID - No audit logging implemented

5. ✅ `internal/handlers/admin/service_handler.go:137`
   ```
   // TODO: Audit log the key rotation with reason
   ```
   **Status:** VALID - No audit logging implemented

6. ✅ `internal/handlers/admin/service_handler.go:265`
   ```
   // TODO: Audit log deactivation with reason
   ```
   **Status:** VALID - No audit logging implemented

7. ✅ `internal/handlers/admin/service_handler.go:241`
   ```
   Total: int64(len(services)), // TODO: Get actual count from DB
   ```
   **Status:** VALID - Using array length instead of DB count

### Payment Metadata

8. ✅ `internal/handlers/payment/payment_handler.go:381`
   ```
   // TODO: Extract from metadata or other gateway fields
   ```
   **Status:** VALID - Feature not implemented

9. ✅ `internal/handlers/payment_method/payment_method_handler_connect.go:259`
   ```
   // TODO: Implement payment method metadata update functionality
   ```
   **Status:** VALID - UpdatePaymentMethod returns Unimplemented

### Security Tests

10. ✅ `tests/integration/auth/epx_callback_auth_test.go:237`
    ```
    t.Skip("TODO: Implement replay attack test")
    ```
    **Status:** VALID - Test not implemented

11. ✅ `tests/integration/auth/epx_callback_auth_test.go:246`
    ```
    t.Skip("TODO: Implement IP whitelist test")
    ```
    **Status:** VALID - Test not implemented

### Database Integration

12. ✅ `internal/adapters/database/postgres_test.go:383`
    ```
    // TODO: Add full integration test with actual transaction creation
    ```
    **Status:** VALID - Test not implemented

---

## 📊 Summary

| Status | Count | Action Required |
|--------|-------|-----------------|
| **STALE** | 11 | Remove/Update TODOs |
| **VALID** | 22 | Keep as reminders for future work |

## 🎯 Recommended Actions

### Immediate: Clean Up Stale TODOs (30 minutes)

**Update these test files to use existing `TokenizeAndSaveCardViaBrowserPost`:**
1. `tests/integration/payment_method/payment_method_test.go` - 6 tests can be unskipped
2. `tests/integration/payment/browser_post_workflow_test.go:224` - Remove TODO comment
3. `tests/integration/testutil/tokenization.go:392` - Remove TODO (already deprecated)
4. `tests/integration/fixtures/epx_brics.go` - Remove or downgrade priority

### Future Work: Implement Missing Features

**High Priority:**
- Implement `StoreACHAccount` RPC (unlocks 13 TODOs)
- Implement admin audit logging (4 TODOs)

**Medium Priority:**
- Implement `UpdatePaymentMethod` (1 TODO)
- Add payment metadata extraction (1 TODO)

**Low Priority:**
- Add security tests (2 TODOs)
- Add DB integration test (1 TODO)
- Fix service count query (1 TODO)

---

**Generated:** 2025-11-20
**Review Type:** Manual verification against actual implementations
