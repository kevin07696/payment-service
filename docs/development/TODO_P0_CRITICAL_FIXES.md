# P0 Critical Fixes

Critical issues that must be fixed immediately before production deployment.

## P0-001: Void/Refund Operations Using Empty TranGroup Instead of EPX Classification

**Priority**: P0 - CRITICAL
**Status**: FIXED
**Impact**: HIGH - Void and Refund operations were using empty TranGroup instead of EPX transaction type classifiers

**Root Cause**:
Both `Void()` and `Refund()` operations in `internal/services/payment/payment_service.go` were sending `TranGroup: ""` (empty string) to EPX instead of the required EPX TRAN_GROUP classification values ("VOID" and "REFUND").

**Affected Code**:

1. **Void Operation** (`payment_service.go:1013`) - BEFORE:
```go
epxReq := &adapterports.ServerPostRequest{
    CustNbr:          merchant.CustNbr,
    MerchNbr:         merchant.MerchNbr,
    DBAnbr:           merchant.DbaNbr,
    TerminalNbr:      merchant.TerminalNbr,
    TransactionType:  adapterports.TransactionTypeVoid,
    Amount:           centsToDecimalString(voidAmountCents),
    PaymentType:      adapterports.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
    OriginalAuthGUID: authBRIC,
    TranNbr:          epxTranNbr,
    TranGroup:        "",  // ❌ WRONG: Was empty
    CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
}
```

**AFTER (FIXED)**:
```go
epxReq := &adapterports.ServerPostRequest{
    // ... same fields ...
    TranGroup:        "VOID",  // ✅ FIXED: Uses EPX TRAN_GROUP classification
    CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
}
```

2. **Refund Operation** (`payment_service.go:1276`) - BEFORE:
```go
epxReq := &adapterports.ServerPostRequest{
    CustNbr:          merchant.CustNbr,
    MerchNbr:         merchant.MerchNbr,
    DBAnbr:           merchant.DbaNbr,
    TerminalNbr:      merchant.TerminalNbr,
    TransactionType:  adapterports.TransactionTypeRefund,
    Amount:           centsToDecimalString(finalRefundAmountCents),
    PaymentType:      adapterports.PaymentMethodType(domainTxsRefetch[0].PaymentMethodType),
    OriginalAuthGUID: authBRIC,
    TranNbr:          epxTranNbr,
    TranGroup:        "",  // ❌ WRONG: Was empty
    CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
}
```

**AFTER (FIXED)**:
```go
epxReq := &adapterports.ServerPostRequest{
    // ... same fields ...
    TranGroup:        "REFUND",  // ✅ FIXED: Uses EPX TRAN_GROUP classification
    CustomerID:       stringOrEmpty(domainTxsRefetch[0].CustomerID),
}
```

**Expected Behavior**:
- Void operations should use EPX TRAN_GROUP classification "VOID"
- Refund operations should use EPX TRAN_GROUP classification "REFUND"
- TranGroup is an EPX transaction type classifier, not a transaction ID
- Valid TranGroup values per EPX Data Dictionary: SALE, AUTH, CAPTURE, REFUND, AVS, PRENOTE, STORAGE

**Fix Applied**:

1. **For Void** (line 1013) - Use EPX classification "VOID":
```go
TranGroup: "VOID",  // EPX TRAN_GROUP classification
```

2. **For Refund** (line 1276) - Use EPX classification "REFUND":
```go
TranGroup: "REFUND",  // EPX TRAN_GROUP classification
```

**Test Cases to Add**:
1. Test void operation sends correct TranGroup to EPX
2. Test refund operation sends correct TranGroup to EPX
3. Integration test verifying EPX accepts void with proper TranGroup
4. Integration test verifying EPX accepts refund with proper TranGroup

**Files to Modify**:
- `internal/services/payment/payment_service.go` (lines 1013 and 1276)
- Add integration tests for void/refund with proper TranGroup validation

**Deployment Risk**: MEDIUM
- Existing void/refund operations may be working despite empty TranGroup
- Changing this could affect existing behavior
- Requires thorough testing with EPX sandbox before production deployment

**Related Issues**:
- None

**References**:
- EPX Server Post API documentation (TranGroup parameter requirements)
- Transaction group design in `docs/integration/DATAFLOW.md`
