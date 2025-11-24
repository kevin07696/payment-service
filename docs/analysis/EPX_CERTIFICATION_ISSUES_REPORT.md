# EPX Certification Issues Report

**Generated:** 2025-11-24
**Context:** EPX feedback on certification sheet submission

---

## Executive Summary

EPX reviewed our certification samples and provided feedback on 6 issues. This report details the status of each issue and required fixes.

**Overall Status:** 2 of 6 issues are fixed, 4 require code changes

---

## Issue #1: BrowserPost Form Missing Address Fields

**EPX Feedback:**
> "Your BrowserPost form is missing the card data and address fields"

**Status:** ⚠️ **PARTIALLY FIXED**

**Analysis:**
- ✅ **Card data fields ARE present:**
  - `examples/browser_post_form.html:178-180`
  - Fields: CARD_NBR, EXP_DATE, CVV

- ❌ **Address fields are MISSING:**
  - No ADDRESS field
  - No CITY field
  - No STATE field
  - No ZIP_CODE field

**Impact:** Medium - Missing billing address affects AVS (Address Verification System)

**Required Changes:**
1. Add address fields to `examples/browser_post_form.html`
2. Update `ports/browser_post.go` BrowserPostFormData struct to include address fields
3. Update `internal/adapters/epx/browser_post_adapter.go` BuildFormData() method

**Code Locations:**
- `examples/browser_post_form.html:148-186` - Browser form HTML
- `internal/adapters/ports/browser_post.go:7-29` - BrowserPostFormData struct
- `internal/adapters/epx/browser_post_adapter.go:60-103` - BuildFormData() method

---

## Issue #2: Add INDUSTRY_TYPE=E to KeyExchange

**EPX Feedback:**
> "Add INDUSTRY_TYPE=E to your KeyExchange and/or Browser POST, as well as to all ServerPost requests"

**Status:** ❌ **NOT FIXED** (KeyExchange)

**Analysis:**
- ❌ **KeyExchange requests do NOT include INDUSTRY_TYPE:**
  - `internal/adapters/epx/key_exchange_adapter.go:214-251` - buildFormData()
  - Only sends: TRAN_NBR, AMOUNT, MAC, TRAN_GROUP, REDIRECT_URL, CUSTOMER_ID
  - Missing INDUSTRY_TYPE parameter

**Impact:** High - EPX requires INDUSTRY_TYPE=E for ecommerce transactions

**Required Changes:**
1. Add `IndustryType string` field to `ports/key_exchange.go` KeyExchangeRequest struct
2. Update `internal/adapters/epx/key_exchange_adapter.go` buildFormData() to include INDUSTRY_TYPE
3. Update all callers to pass IndustryType="E"

**Code Locations:**
- `internal/adapters/ports/key_exchange.go:8-28` - KeyExchangeRequest struct
- `internal/adapters/epx/key_exchange_adapter.go:214-251` - buildFormData() method

**Example Fix:**
```go
// In ports/key_exchange.go
type KeyExchangeRequest struct {
    // ... existing fields ...
    IndustryType string // E = Ecommerce, RE = Retail
}

// In key_exchange_adapter.go buildFormData()
if req.IndustryType != "" {
    data.Set("INDUSTRY_TYPE", req.IndustryType)
}
```

---

## Issue #3: Add INDUSTRY_TYPE=E to BrowserPost

**EPX Feedback:**
> "Add INDUSTRY_TYPE=E to your KeyExchange and/or Browser POST, as well as to all ServerPost requests"

**Status:** ⚠️ **PARTIALLY FIXED** (BrowserPost)

**Analysis:**
- ✅ **HTML form example includes INDUSTRY_TYPE=E:**
  - `examples/browser_post_form.html:185` has `<input type="hidden" name="INDUSTRY_TYPE" value="E">`

- ❌ **BrowserPost adapter does NOT support INDUSTRY_TYPE:**
  - `ports/browser_post.go:7-29` - BrowserPostFormData struct has no IndustryType field
  - `internal/adapters/epx/browser_post_adapter.go:60-103` - BuildFormData() doesn't set INDUSTRY_TYPE

**Impact:** High - EPX requires INDUSTRY_TYPE=E for ecommerce transactions

**Required Changes:**
1. Add `IndustryType string` field to BrowserPostFormData struct in `ports/browser_post.go`
2. Update BuildFormData() in `browser_post_adapter.go` to accept and include IndustryType
3. Update all callers to pass IndustryType="E"

**Code Locations:**
- `internal/adapters/ports/browser_post.go:7-29` - BrowserPostFormData struct
- `internal/adapters/epx/browser_post_adapter.go:60-103` - BuildFormData() method

**Example Fix:**
```go
// In ports/browser_post.go
type BrowserPostFormData struct {
    // ... existing fields ...
    IndustryType string // E = Ecommerce, RE = Retail
}

// In browser_post_adapter.go BuildFormData()
return &ports.BrowserPostFormData{
    // ... existing fields ...
    IndustryType: "E", // Ecommerce
}
```

---

## Issue #4: REDIRECT_URL_DECLINE/ERROR Not Valid EPX Fields

**EPX Feedback:**
> "REDIRECT_URL_DECLINE and REDIRECT_URL_ERROR are not valid EPX fields, however, you can use INVALID_REDIRECT_URL"

**Status:** ❌ **NOT FIXED**

**Analysis:**
- ❌ **Code uses invalid field names:**
  - `internal/adapters/epx/browser_post_adapter.go:98-99`:
    ```go
    RedirectURLDecline: redirectURL,
    RedirectURLError:   redirectURL,
    ```
  - `ports/browser_post.go:23-24`:
    ```go
    RedirectURLDecline string // Decline redirect URL (optional)
    RedirectURLError   string // Error redirect URL (optional)
    ```

- **EPX expects:** INVALID_REDIRECT_URL instead

**Impact:** Medium - Using invalid field names, but EPX may ignore them

**Required Changes:**
1. Remove RedirectURLDecline and RedirectURLError from BrowserPostFormData struct
2. Add InvalidRedirectURL field to BrowserPostFormData struct
3. Update BuildFormData() to use InvalidRedirectURL instead
4. Update HTML form examples

**Code Locations:**
- `internal/adapters/ports/browser_post.go:23-24` - Invalid fields
- `internal/adapters/epx/browser_post_adapter.go:98-99` - Sets invalid fields
- `examples/browser_post_form.html` - May have REDIRECT_URL_DECLINE/ERROR

**Example Fix:**
```go
// In ports/browser_post.go
type BrowserPostFormData struct {
    // ... existing fields ...
    RedirectURL         string // Success redirect URL
    InvalidRedirectURL  string // Invalid/error redirect URL (optional)
    // Remove: RedirectURLDecline, RedirectURLError
}

// In browser_post_adapter.go BuildFormData()
return &ports.BrowserPostFormData{
    // ... existing fields ...
    InvalidRedirectURL: redirectURL, // Use INVALID_REDIRECT_URL
}
```

---

## Issue #5: CARD_ENT_METH=Z for BRIC-Based Transactions

**EPX Feedback:**
> "When requests are based on ORIG_AUTH_GUID you would want to have CARD_ENT_METH=Z"

**Status:** ✅ **FIXED**

**Analysis:**
- ✅ **BRIC storage adapter correctly sets CARD_ENT_METH=Z:**
  - `internal/adapters/epx/bric_storage_adapter.go:200`:
    ```go
    cardEntMeth := "Z" // BRIC-based transaction
    ```
  - `internal/adapters/epx/bric_storage_adapter.go:209`:
    ```go
    sb.WriteString(fmt.Sprintf(`<CARD_ENT_METH>%s</CARD_ENT_METH>`, cardEntMeth))
    ```

**Impact:** None - Already implemented correctly

**No Changes Required**

---

## Issue #6: ACI_EXT for Merchant Initiated Transactions (MIT)

**EPX Feedback:**
> "If your BRIC (ORIG_AUTH_GUID) transactions are Merchant Initiated (MIT) such as recurring billing transactions, then you would want to include ACI_EXT with an appropriate value to your use case. For Customer Initiated (CIT) transactions there is no need to include ACI_EXT."

**Status:** ✅ **INFRASTRUCTURE IMPLEMENTED**

**Analysis:**
- ✅ **Server Post adapter supports ACI_EXT:**
  - `internal/adapters/epx/server_post_adapter.go:540-543`:
    ```go
    // Authorization Characteristics Indicator Extension (for COF, MIT, Recurring)
    if req.ACIExt != nil && *req.ACIExt != "" {
        data.Set("ACI_EXT", *req.ACIExt)
    }
    ```

- **ACI_EXT Values:**
  - For MIT (Merchant Initiated): Set appropriate value based on use case
  - For CIT (Customer Initiated): Omit ACI_EXT field

**Impact:** None - Infrastructure exists, just needs to be used when making MIT transactions

**Usage Guidance:**
- Recurring billing: Include ACI_EXT with appropriate MIT value
- Customer-initiated payments: Omit ACI_EXT

**No Code Changes Required** - Feature already implemented

---

## Summary Table

| # | Issue | Status | Priority | Effort |
|---|-------|--------|----------|--------|
| 1 | BrowserPost form missing address fields | ⚠️ Partial | Medium | Small |
| 2 | KeyExchange missing INDUSTRY_TYPE=E | ❌ Not Fixed | High | Medium |
| 3 | BrowserPost missing INDUSTRY_TYPE=E | ⚠️ Partial | High | Small |
| 4 | Invalid redirect URL field names | ❌ Not Fixed | Medium | Small |
| 5 | CARD_ENT_METH=Z for BRIC | ✅ Fixed | - | - |
| 6 | ACI_EXT for MIT transactions | ✅ Implemented | - | - |

---

## Recommended Fix Priority

1. **HIGH:** Add INDUSTRY_TYPE=E to KeyExchange (Issue #2)
2. **HIGH:** Add INDUSTRY_TYPE=E to BrowserPost adapter (Issue #3)
3. **MEDIUM:** Fix redirect URL field names (Issue #4)
4. **MEDIUM:** Add address fields to BrowserPost form (Issue #1)

---

## Additional Notes

### EPX Requirements
- Certification requires **actual sandbox requests/responses**, not mock/simulated examples
- The certification_sheets.md file needs to be updated with real EPX responses

### ServerPost INDUSTRY_TYPE Support
- Server Post adapter ALREADY supports INDUSTRY_TYPE:
  - `internal/adapters/epx/server_post_adapter.go:536-538`
  - No changes needed for ServerPost

### BRIC Storage Adapter INDUSTRY_TYPE Support
- BRIC storage adapter ALREADY includes INDUSTRY_TYPE=E:
  - `internal/adapters/epx/bric_storage_adapter.go:211, 264`
  - No changes needed

---

## Next Steps

1. Create feature branches for each fix
2. Implement fixes in priority order
3. Test with EPX sandbox environment
4. Capture actual sandbox requests/responses
5. Update certification_sheets.md with real data
6. Resubmit to EPX for certification approval

---

## Files Requiring Changes

### Priority 1 (HIGH)
- `internal/adapters/ports/key_exchange.go` - Add IndustryType field
- `internal/adapters/epx/key_exchange_adapter.go` - Include INDUSTRY_TYPE in requests
- `internal/adapters/ports/browser_post.go` - Add IndustryType field
- `internal/adapters/epx/browser_post_adapter.go` - Include INDUSTRY_TYPE in form data

### Priority 2 (MEDIUM)
- `internal/adapters/ports/browser_post.go` - Fix redirect URL field names
- `internal/adapters/epx/browser_post_adapter.go` - Use INVALID_REDIRECT_URL
- `examples/browser_post_form.html` - Add address fields and fix redirect URLs

### Documentation
- `docs/certification_sheets.md` - Update with actual sandbox requests/responses
