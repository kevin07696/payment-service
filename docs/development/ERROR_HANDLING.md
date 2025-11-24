# Error Handling Guide

**Target Audience:** Developers working on the payment service codebase
**Topic:** Error handling patterns, error types, and HTTP status code mapping
**Goal:** Consistent, client-friendly error handling across all layers

**Last Updated:** 2025-11-23

---

## Table of Contents

1. [Error Handling Philosophy](#error-handling-philosophy)
2. [Error Types](#error-types)
3. [Creating Errors](#creating-errors)
4. [Error Propagation](#error-propagation)
5. [HTTP Status Code Mapping](#http-status-code-mapping)
6. [ConnectRPC Error Handling](#connectrpc-error-handling)
7. [Client-Friendly Error Messages](#client-friendly-error-messages)
8. [Logging Errors](#logging-errors)
9. [Common Patterns](#common-patterns)
10. [Anti-Patterns](#anti-patterns)

---

## Error Handling Philosophy

### Golden Rule

**"Errors are part of your API contract"**

Errors should be:
1. **Structured** - Machine-readable error codes + human-readable messages
2. **Actionable** - Tell clients what went wrong and how to fix it
3. **Consistent** - Same error format across all endpoints
4. **Secure** - Never leak sensitive data or stack traces to clients

### Error Layers

```
┌─────────────────────┐
│   Handler Layer     │ → HTTP status codes + ConnectRPC errors
├─────────────────────┤
│   Service Layer     │ → Domain errors (structured)
├─────────────────────┤
│   Domain Layer      │ → Domain errors (pure)
├─────────────────────┤
│   Adapter Layer     │ → Wrap external errors → domain errors
└─────────────────────┘
```

**Rule:** Each layer handles errors appropriate to its level of abstraction.

---

## Error Types

### DomainError (Preferred)

Structured error with error code, message, and context details.

```go
// internal/domain/errors.go
type DomainError struct {
    Code    ErrorCode              // Machine-readable code
    Message string                 // Human-readable message
    Err     error                  // Underlying error (optional)
    Details map[string]interface{} // Additional context
}
```

**Error Codes** (categorized by prefix):

| Prefix | Category | Examples |
|--------|----------|----------|
| `AUTH_*` | Authentication/Authorization | `AUTH_MISSING`, `AUTH_INVALID`, `AUTH_ACCESS_DENIED` |
| `MERCHANT_*` | Merchant errors | `MERCHANT_NOT_FOUND`, `MERCHANT_INACTIVE` |
| `TXN_*` | Transaction errors | `TXN_NOT_FOUND`, `TXN_INVALID_STATE` |
| `PM_*` | Payment Method errors | `PM_NOT_FOUND`, `PM_EXPIRED`, `PM_NOT_VERIFIED` |
| `CUSTOMER_*` | Customer errors | `CUSTOMER_NOT_FOUND` |
| `VALIDATION_*` | Validation errors | `VALIDATION_FAILED`, `VALIDATION_AMOUNT_INVALID` |
| `GATEWAY_*` | Payment Gateway errors | `GATEWAY_ERROR`, `GATEWAY_TIMEOUT`, `GATEWAY_DECLINED` |
| `IDEMPOTENCY_*` | Idempotency errors | `IDEMPOTENCY_CONFLICT` |
| `INTERNAL_*` | Internal errors | `INTERNAL_ERROR`, `INTERNAL_DATABASE_ERROR` |

### Legacy Errors

Simple `errors.New()` errors still exist for backward compatibility:

```go
var (
    ErrTransactionNotFound = errors.New("transaction not found")
    ErrMerchantNotFound    = errors.New("merchant not found")
)
```

**Migration Strategy:** Gradually replace with `DomainError` instances.

---

## Creating Errors

### Option 1: Pre-defined Error Instances (Recommended)

Use pre-defined error instances for common errors:

```go
// Domain layer
import "github.com/kevin07696/payment-service/internal/domain"

// Return pre-defined error
if merchant == nil {
    return nil, domain.ErrMerchantNotFoundTyped
}

// Add context with WithDetail
if tx == nil {
    return nil, domain.ErrTxnNotFound.WithDetail("transaction_id", txID)
}
```

**Available instances:**
```go
domain.ErrAuthMissing
domain.ErrAuthInvalid
domain.ErrMerchantNotFoundTyped
domain.ErrTxnNotFound
domain.ErrPMNotFound
domain.ErrValidationFailed
domain.ErrGatewayError
domain.ErrIdempotencyConflict
// ... see domain/errors.go for complete list
```

### Option 2: Create New DomainError

For errors specific to your context:

```go
// Create new domain error
err := domain.NewDomainError(
    domain.ErrorCodeValidationFailed,
    "amount must be greater than 0",
)

// Add details
err.WithDetail("amount_cents", req.AmountCents)
err.WithDetail("field", "amount_cents")

return nil, err
```

### Option 3: Wrap External Errors

When handling errors from adapters (database, EPX, etc.):

```go
// Wrap database error
tx, err := s.queries.GetTransaction(ctx, txID)
if err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        // Convert to domain error
        return nil, domain.ErrTxnNotFound.WithDetail("transaction_id", txID)
    }
    // Wrap unknown database error
    return nil, domain.WrapError(
        domain.ErrorCodeDatabaseError,
        "failed to query transaction",
        err,
    )
}

// Wrap EPX adapter error
epxResp, err := s.epxAdapter.ProcessTransaction(ctx, epxReq)
if err != nil {
    return nil, domain.WrapError(
        domain.ErrorCodeGatewayError,
        "EPX transaction failed",
        err,
    ).WithDetail("epx_request_id", epxReq.TranNbr)
}
```

---

## Error Propagation

### Services Return Domain Errors

Services should return `DomainError` instances, not generic errors:

✅ **Good:**
```go
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    // Validate inputs
    if req.AmountCents <= 0 {
        return nil, domain.ErrValidationAmountInvalid.
            WithDetail("amount_cents", req.AmountCents)
    }

    // Check merchant exists
    merchant, err := s.queries.GetMerchant(ctx, req.MerchantID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, domain.ErrMerchantNotFoundTyped.
                WithDetail("merchant_id", req.MerchantID)
        }
        return nil, domain.WrapError(domain.ErrorCodeDatabaseError, "query merchant failed", err)
    }

    // Process with EPX
    epxResp, err := s.epxAdapter.ProcessTransaction(ctx, epxReq)
    if err != nil {
        return nil, domain.WrapError(domain.ErrorCodeGatewayError, "EPX failed", err)
    }

    return tx, nil
}
```

❌ **Bad:**
```go
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    if req.AmountCents <= 0 {
        return nil, errors.New("invalid amount")  // No error code!
    }

    merchant, err := s.queries.GetMerchant(ctx, req.MerchantID)
    if err != nil {
        return nil, err  // Leaks database error!
    }

    return tx, nil
}
```

### Handlers Convert to HTTP/gRPC Errors

Handlers map domain errors to HTTP status codes or gRPC codes:

```go
func (h *paymentHandler) Sale(ctx context.Context, req *paymentv1.SaleRequest) (*paymentv1.PaymentResponse, error) {
    tx, err := h.service.Sale(ctx, svcReq)
    if err != nil {
        // Map domain error to ConnectRPC error
        return nil, mapDomainErrorToConnectError(err)
    }

    return resp, nil
}

func mapDomainErrorToConnectError(err error) error {
    var domainErr *domain.DomainError
    if !errors.As(err, &domainErr) {
        // Unknown error → internal error
        return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
    }

    switch domainErr.Code {
    case domain.ErrorCodeAuthMissing, domain.ErrorCodeAuthInvalid:
        return connect.NewError(connect.CodeUnauthenticated, err)
    case domain.ErrorCodeAuthAccessDenied, domain.ErrorCodeAuthMerchantMismatch:
        return connect.NewError(connect.CodePermissionDenied, err)
    case domain.ErrorCodeMerchantNotFound, domain.ErrorCodeTxnNotFound, domain.ErrorCodePMNotFound:
        return connect.NewError(connect.CodeNotFound, err)
    case domain.ErrorCodeValidationFailed, domain.ErrorCodeValidationAmountInvalid:
        return connect.NewError(connect.CodeInvalidArgument, err)
    case domain.ErrorCodeIdempotencyConflict:
        return connect.NewError(connect.CodeAlreadyExists, err)
    case domain.ErrorCodeGatewayTimeout:
        return connect.NewError(connect.CodeDeadlineExceeded, err)
    default:
        return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
    }
}
```

---

## HTTP Status Code Mapping

| Domain Error Code | HTTP Status | ConnectRPC Code | Use Case |
|-------------------|-------------|-----------------|----------|
| `AUTH_MISSING`, `AUTH_INVALID` | 401 Unauthorized | `Unauthenticated` | Missing/invalid JWT token |
| `AUTH_ACCESS_DENIED`, `AUTH_MERCHANT_MISMATCH` | 403 Forbidden | `PermissionDenied` | Accessing another merchant's data |
| `MERCHANT_NOT_FOUND`, `TXN_NOT_FOUND`, `PM_NOT_FOUND` | 404 Not Found | `NotFound` | Resource doesn't exist |
| `VALIDATION_*` | 400 Bad Request | `InvalidArgument` | Invalid request parameters |
| `IDEMPOTENCY_CONFLICT` | 409 Conflict | `AlreadyExists` | Duplicate idempotency key |
| `TXN_INVALID_STATE` | 412 Precondition Failed | `FailedPrecondition` | Operation not allowed in current state |
| `GATEWAY_TIMEOUT` | 504 Gateway Timeout | `DeadlineExceeded` | EPX timeout |
| `GATEWAY_ERROR`, `INTERNAL_*` | 500 Internal Server Error | `Internal` | Server errors |

---

## ConnectRPC Error Handling

### Server-Side (Handler)

```go
import (
    "connectrpc.com/connect"
    "github.com/kevin07696/payment-service/internal/domain"
)

func (h *paymentHandler) Sale(ctx context.Context, req *connect.Request[paymentv1.SaleRequest]) (*connect.Response[paymentv1.PaymentResponse], error) {
    tx, err := h.service.Sale(ctx, convertRequest(req.Msg))
    if err != nil {
        // Check for specific domain errors
        if domain.IsDomainError(err, domain.ErrorCodePMExpired) {
            return nil, connect.NewError(
                connect.CodeInvalidArgument,
                errors.New("payment method has expired"),
            )
        }

        if domain.IsNotFoundError(err) {
            return nil, connect.NewError(connect.CodeNotFound, err)
        }

        if domain.IsValidationError(err) {
            return nil, connect.NewError(connect.CodeInvalidArgument, err)
        }

        // Default: internal error (hide details from client)
        return nil, connect.NewError(
            connect.CodeInternal,
            errors.New("internal server error"),
        )
    }

    return connect.NewResponse(convertResponse(tx)), nil
}
```

### Client-Side (TypeScript/JavaScript)

```typescript
import { ConnectError } from '@connectrpc/connect';

try {
  const response = await client.sale({
    merchantId: '...',
    amountCents: BigInt(10000),
    paymentMethodId: 'pm_abc123',
    idempotencyKey: 'sale_123',
  });
} catch (err) {
  if (err instanceof ConnectError) {
    switch (err.code) {
      case Code.Unauthenticated:
        console.error('Authentication required:', err.message);
        // Redirect to login
        break;
      case Code.NotFound:
        console.error('Resource not found:', err.message);
        break;
      case Code.InvalidArgument:
        console.error('Invalid request:', err.message);
        // Show validation errors to user
        break;
      case Code.AlreadyExists:
        console.error('Duplicate request:', err.message);
        // Show idempotency message
        break;
      default:
        console.error('Server error:', err.message);
        break;
    }
  }
}
```

---

## Client-Friendly Error Messages

### Guidelines

1. **Be specific** - Tell the user exactly what went wrong
2. **Be actionable** - Suggest how to fix it
3. **Be secure** - Don't leak internal details (database errors, stack traces)
4. **Be consistent** - Same format across all errors

### Examples

✅ **Good:**
```go
// Specific and actionable
domain.NewDomainError(
    domain.ErrorCodeValidationAmountInvalid,
    "amount must be greater than 0",
).WithDetail("field", "amount_cents").WithDetail("value", req.AmountCents)

// Clear and actionable
domain.ErrPMExpired.WithDetail("payment_method_id", pmID).
    WithDetail("expired_at", pm.ExpiryDate).
    WithDetail("action", "please update your payment method")
```

❌ **Bad:**
```go
// Too vague
errors.New("invalid request")

// Leaks internal details
errors.New("sql: no rows in result set")

// Not actionable
errors.New("error occurred")
```

### User-Facing vs Technical Messages

**User-facing** (returned to client):
```json
{
  "code": "PM_EXPIRED",
  "message": "Payment method has expired",
  "details": {
    "payment_method_id": "pm_abc123",
    "expired_at": "2024-10-01",
    "action": "Please update your payment method"
  }
}
```

**Technical** (logged server-side):
```
ERROR payment_service: Sale failed
  error="PM_EXPIRED: payment method has expired"
  merchant_id="00000000-0000-0000-0000-000000000001"
  payment_method_id="pm_abc123"
  stack_trace="..."
```

---

## Logging Errors

### Structured Logging with zap

```go
import "go.uber.org/zap"

// Log domain errors with context
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    tx, err := s.processTransaction(ctx, req)
    if err != nil {
        var domainErr *domain.DomainError
        if errors.As(err, &domainErr) {
            // Log structured domain error
            s.logger.Error("Sale failed",
                zap.String("error_code", string(domainErr.Code)),
                zap.String("error_message", domainErr.Message),
                zap.String("merchant_id", req.MerchantID),
                zap.String("idempotency_key", req.IdempotencyKey),
                zap.Any("details", domainErr.Details),
                zap.Error(domainErr.Err), // Underlying error
            )
        } else {
            // Log unknown error
            s.logger.Error("Sale failed with unknown error",
                zap.String("merchant_id", req.MerchantID),
                zap.Error(err),
            )
        }
        return nil, err
    }

    s.logger.Info("Sale succeeded",
        zap.String("transaction_id", tx.ID),
        zap.String("merchant_id", req.MerchantID),
        zap.Int64("amount_cents", req.AmountCents),
    )

    return tx, nil
}
```

### Log Levels

| Level | When to Use | Example |
|-------|-------------|---------|
| **ERROR** | Domain/business errors | Payment declined, merchant not found, validation failed |
| **WARN** | Recoverable issues | Retry succeeded after failure, deprecated API used |
| **INFO** | Normal operations | Transaction created, subscription billed |
| **DEBUG** | Detailed debugging | Request/response payloads (non-PCI data) |

**Important:** Never log PCI data (full card numbers, CVV, etc.)

---

## Common Patterns

### Pattern 1: Validate, Then Execute

```go
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    // Step 1: Validate inputs
    if req.AmountCents <= 0 {
        return nil, domain.ErrValidationAmountInvalid.
            WithDetail("amount_cents", req.AmountCents)
    }

    if req.MerchantID == "" {
        return nil, domain.ErrMerchantRequired
    }

    // Step 2: Check preconditions
    pm, err := s.queries.GetPaymentMethod(ctx, req.PaymentMethodID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, domain.ErrPMNotFound.WithDetail("payment_method_id", req.PaymentMethodID)
        }
        return nil, domain.WrapError(domain.ErrorCodeDatabaseError, "query payment method failed", err)
    }

    if isExpired(pm) {
        return nil, domain.ErrPMExpired.WithDetail("payment_method_id", pm.ID)
    }

    // Step 3: Execute operation
    tx, err := s.processTransaction(ctx, req, pm)
    if err != nil {
        return nil, err
    }

    return tx, nil
}
```

### Pattern 2: Check Errors with errors.Is/As

```go
// Use errors.Is for sentinel errors
if errors.Is(err, pgx.ErrNoRows) {
    return nil, domain.ErrTxnNotFound
}

// Use errors.As for structured errors
var domainErr *domain.DomainError
if errors.As(err, &domainErr) {
    if domainErr.Code == domain.ErrorCodeGatewayTimeout {
        // Retry logic
    }
}

// Use helper functions
if domain.IsAuthError(err) {
    // Handle authentication errors
}

if domain.IsValidationError(err) {
    // Return 400 Bad Request
}
```

### Pattern 3: Add Context as Errors Bubble Up

```go
// Adapter layer
func (a *epxAdapter) ProcessTransaction(ctx context.Context, req *EPXRequest) (*EPXResponse, error) {
    resp, err := a.httpClient.Post(url, body)
    if err != nil {
        // Wrap with adapter context
        return nil, domain.WrapError(
            domain.ErrorCodeGatewayError,
            "EPX HTTP request failed",
            err,
        ).WithDetail("epx_url", url).WithDetail("request_id", req.TranNbr)
    }
    return resp, nil
}

// Service layer
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    epxResp, err := s.epxAdapter.ProcessTransaction(ctx, epxReq)
    if err != nil {
        // Add service-level context
        var domainErr *domain.DomainError
        if errors.As(err, &domainErr) {
            domainErr.WithDetail("merchant_id", req.MerchantID)
            domainErr.WithDetail("idempotency_key", req.IdempotencyKey)
        }
        return nil, err
    }
    return tx, nil
}
```

---

## Anti-Patterns

### Anti-Pattern 1: Swallowing Errors

❌ **Bad:**
```go
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    tx, err := s.processTransaction(ctx, req)
    if err != nil {
        s.logger.Error("failed", zap.Error(err))
        return nil, nil  // Returning nil error!
    }
    return tx, nil
}
```

✅ **Good:**
```go
func (s *paymentService) Sale(ctx context.Context, req *SaleRequest) (*Transaction, error) {
    tx, err := s.processTransaction(ctx, req)
    if err != nil {
        s.logger.Error("Sale failed", zap.Error(err))
        return nil, err  // Propagate error
    }
    return tx, nil
}
```

### Anti-Pattern 2: Generic Error Messages

❌ **Bad:**
```go
if err != nil {
    return nil, errors.New("error occurred")  // Useless!
}
```

✅ **Good:**
```go
if err != nil {
    return nil, domain.WrapError(
        domain.ErrorCodeGatewayError,
        "EPX transaction failed",
        err,
    ).WithDetail("transaction_type", "SALE")
}
```

### Anti-Pattern 3: Leaking Internal Errors

❌ **Bad:**
```go
// Handler returns database error to client
func (h *paymentHandler) Sale(ctx context.Context, req *SaleRequest) (*Response, error) {
    tx, err := h.queries.GetTransaction(ctx, txID)
    return nil, err  // Leaks "pq: relation 'transactions' does not exist"
}
```

✅ **Good:**
```go
func (h *paymentHandler) Sale(ctx context.Context, req *SaleRequest) (*Response, error) {
    tx, err := h.queries.GetTransaction(ctx, txID)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, connect.NewError(connect.CodeNotFound, errors.New("transaction not found"))
        }
        // Hide database details
        return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
    }
    return resp, nil
}
```

### Anti-Pattern 4: Not Using Error Codes

❌ **Bad:**
```go
// Clients must parse error messages (fragile!)
if strings.Contains(err.Error(), "not found") {
    // Handle not found
}
```

✅ **Good:**
```go
// Clients check error codes (stable!)
if domain.IsDomainError(err, domain.ErrorCodeTxnNotFound) {
    // Handle not found
}
```

---

## Checklist

Before committing error handling code:

- [ ] **Error codes:** All business errors use `DomainError` with error codes
- [ ] **Context:** Errors include relevant details (`WithDetail`)
- [ ] **Mapping:** Handlers map domain errors to HTTP/gRPC codes correctly
- [ ] **Logging:** Errors logged with structured fields (zap)
- [ ] **Security:** No PCI data or stack traces in client responses
- [ ] **Messages:** Client-friendly, actionable error messages
- [ ] **Propagation:** Errors propagated, not swallowed
- [ ] **Testing:** Error paths tested (not just happy path)

---

## Related Documentation

- **[STYLE_GUIDE.md](./STYLE_GUIDE.md)** - Code style conventions
- **[TESTING_GUIDE.md](./TESTING_GUIDE.md)** - Testing error scenarios
- **[API_SPECS.md](../integration/API_SPECS.md)** - API error formats

---

**Questions?** See examples in `internal/domain/errors.go` and `internal/handlers/payment/payment_handler_connect.go`
