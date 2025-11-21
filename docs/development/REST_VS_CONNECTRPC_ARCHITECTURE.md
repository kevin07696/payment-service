# REST vs ConnectRPC Architecture Analysis

## Executive Summary

The payment service correctly uses a dual-protocol architecture:
- **ConnectRPC (Port 8080)**: For all service-to-service RPC operations
- **REST HTTP (Port 8081)**: For external integrations and browser-based interactions

This architecture is **correctly designed** and should be maintained as-is.

## Current Architecture

```
┌─────────────────────────────────────────────┐
│         ConnectRPC Server (Port 8080)       │
│  • Payment Service                           │
│  • Subscription Service                      │
│  • Payment Method Service                    │
│  • Chargeback Service                        │
│  • Merchant Service                          │
│  Protocols: gRPC, Connect, gRPC-Web, HTTP/JSON │
└─────────────────────────────────────────────┘

┌─────────────────────────────────────────────┐
│         REST HTTP Server (Port 8081)        │
│  • Browser Post Endpoints                   │
│  • Cron Job Endpoints                        │
│  • Health Checks                             │
│  • Statistics                                │
└─────────────────────────────────────────────┘
```

## Analysis: What Should Be REST

### MUST Be REST (Current Implementation ✅)

#### 1. Browser Post Endpoints
**Location**: `/api/v1/payments/browser-post/*`
**Why REST Required**:
- Receives HTML form POST data from EPX payment gateway
- Generates HTML forms for browser submission
- External payment processor expects standard HTTP endpoints
- Uses `application/x-www-form-urlencoded` content type
- JavaScript in browser submits forms directly

**Example Flow**:
```
Browser → HTML Form → POST → EPX Gateway → POST Callback → Our REST Endpoint
```

**Current Endpoints** (Correctly REST):
```go
httpMux.HandleFunc("/api/v1/payments/browser-post/form", GetPaymentForm)
httpMux.HandleFunc("/api/v1/payments/browser-post/callback", HandleCallback)
```

#### 2. Cron Job Endpoints
**Location**: `/cron/*`
**Why REST Required**:
- Called by Google Cloud Scheduler (external service)
- Cloud Scheduler only supports HTTP/HTTPS endpoints
- Simple authentication via shared secret
- Returns simple JSON responses
- Health monitoring via standard HTTP status codes

**Current Endpoints** (Correctly REST):
```go
httpMux.HandleFunc("/cron/process-billing", ProcessBilling)      // Cloud Scheduler
httpMux.HandleFunc("/cron/sync-disputes", SyncDisputes)          // Cloud Scheduler
httpMux.HandleFunc("/cron/health", HealthCheck)                  // Monitoring
httpMux.HandleFunc("/cron/stats", Stats)                         // Metrics
```

#### 3. Webhook Endpoints (Future)
**Why REST Required**:
- External services send webhooks as HTTP POST
- Standard webhook format (JSON body, HMAC signatures)
- Third-party services don't support RPC protocols

**Examples**:
- Payment gateway status updates
- Fraud detection alerts
- Partner system notifications

#### 4. OAuth/OIDC Callbacks (Future)
**Why REST Required**:
- OAuth2 spec requires HTTP redirect URLs
- Identity providers redirect browsers to callback URLs
- Must handle URL query parameters and form posts

## Analysis: What Should Be ConnectRPC

### SHOULD Be ConnectRPC (Current Implementation ✅)

#### 1. Service-to-Service Communication
**Current Services** (Correctly ConnectRPC):
- Payment Service - Core transaction processing
- Subscription Service - Recurring billing logic
- Payment Method Service - Stored payment methods
- Chargeback Service - Dispute management
- Merchant Service - Agent/merchant management

**Why ConnectRPC**:
- Type-safe with Protocol Buffers
- Efficient binary serialization
- Supports streaming (future use)
- Multiple protocol support (gRPC, Connect, HTTP/JSON)
- Better performance than REST
- Automatic client generation

#### 2. Internal Admin APIs (Future)
**Why ConnectRPC**:
- Type safety for complex operations
- Can use Connect protocol from admin UI
- Supports both gRPC (backend) and Connect (frontend)

#### 3. Mobile App APIs (Future)
**Why ConnectRPC**:
- Efficient binary protocol reduces data usage
- Connect protocol works well with mobile SDKs
- Type-safe client generation

## Protocol Selection Decision Tree

```
Is it an external integration?
├─ YES → Does external system control the protocol?
│   ├─ YES → Use REST
│   └─ NO → Can they use ConnectRPC?
│       ├─ YES → Use ConnectRPC
│       └─ NO → Use REST
└─ NO → Is it browser form submission?
    ├─ YES → Use REST
    └─ NO → Is it a scheduled job from cloud provider?
        ├─ YES → Use REST
        └─ NO → Use ConnectRPC
```

## Specific Endpoint Analysis

### ✅ Correctly REST (Port 8081)

| Endpoint | Type | Reason | Correct? |
|----------|------|--------|----------|
| `/api/v1/payments/browser-post/form` | GET | HTML form generation | ✅ |
| `/api/v1/payments/browser-post/callback` | POST | EPX gateway callback | ✅ |
| `/cron/process-billing` | POST | Cloud Scheduler | ✅ |
| `/cron/sync-disputes` | POST | Cloud Scheduler | ✅ |
| `/cron/health` | GET | HTTP health monitoring | ✅ |
| `/cron/stats` | GET | Simple metrics endpoint | ✅ |

### ✅ Correctly ConnectRPC (Port 8080)

| Service | RPCs | Reason | Correct? |
|---------|------|--------|----------|
| PaymentService | 7 methods | Internal service API | ✅ |
| SubscriptionService | 8 methods | Internal service API | ✅ |
| PaymentMethodService | 8 methods | Internal service API | ✅ |
| ChargebackService | 2 methods | Internal service API | ✅ |
| MerchantService | 6 methods | Internal service API | ✅ |

## Implementation Patterns

### REST Endpoint Pattern (Correct)
```go
func (h *Handler) RESTEndpoint(w http.ResponseWriter, r *http.Request) {
    // Parse form/JSON data
    // Process request
    // Return HTTP response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### ConnectRPC Pattern (Correct)
```go
func (h *Handler) RPCMethod(
    ctx context.Context,
    req *connect.Request[proto.Request],
) (*connect.Response[proto.Response], error) {
    // Type-safe request handling
    // Process with service layer
    // Return typed response
    return connect.NewResponse(response), nil
}
```

## Security Considerations

### REST Endpoints
- **Browser Post**: HMAC signature validation from payment gateway
- **Cron**: Shared secret authentication
- **Rate Limiting**: Applied to prevent abuse
- **CORS**: Not needed (server-to-server only)

### ConnectRPC Endpoints
- **Authentication**: Can use interceptors for auth
- **TLS**: Should be enabled in production
- **Rate Limiting**: Can be applied via interceptors

## Performance Considerations

### REST (Port 8081)
- Text-based protocols (JSON/form data)
- Higher overhead for serialization
- Acceptable for low-volume external integrations
- Rate limited to prevent abuse

### ConnectRPC (Port 8080)
- Binary serialization (Protocol Buffers)
- HTTP/2 multiplexing
- Lower latency and overhead
- Better for high-volume internal traffic

## Migration Recommendations

### ✅ Keep as REST
1. All Browser Post endpoints (external gateway requirement)
2. All Cron endpoints (Cloud Scheduler requirement)
3. Future webhook receivers
4. Future OAuth callbacks

### ✅ Keep as ConnectRPC
1. All service-to-service APIs
2. All internal business logic services
3. Future admin APIs
4. Future mobile APIs

### ⚠️ Potential Issues to Address

1. **Documentation**: Clearly document which port/protocol for each use case
2. **Client Libraries**: Provide ConnectRPC client examples
3. **Testing**: Ensure both REST and ConnectRPC endpoints are tested
4. **Monitoring**: Set up separate metrics for each protocol

## Conclusion

The current architecture is **correctly designed**:

✅ **REST is used where required** by external constraints:
- Browser form submissions
- Payment gateway callbacks
- Cloud Scheduler integration
- Simple HTTP health checks

✅ **ConnectRPC is used where optimal** for internal services:
- Service-to-service communication
- Type-safe APIs
- High-performance RPC

**Recommendation**: Maintain the current dual-server architecture. Do not migrate REST endpoints to ConnectRPC as they serve different purposes and have different constraints.

## Quick Reference

| Use Case | Protocol | Port | Reason |
|----------|----------|------|--------|
| Payment Gateway Callbacks | REST | 8081 | External requirement |
| Cloud Scheduler | REST | 8081 | GCP requirement |
| Browser Forms | REST | 8081 | HTML/Browser requirement |
| Service-to-Service | ConnectRPC | 8080 | Performance & type safety |
| Internal APIs | ConnectRPC | 8080 | Type safety |
| Health Checks (External) | REST | 8081 | Standard HTTP monitoring |
| Health Checks (Internal) | ConnectRPC | 8080 | gRPC health protocol |

## Testing the Architecture

### Test REST Endpoints
```bash
# Browser Post Form
curl http://localhost:8081/api/v1/payments/browser-post/form?transaction_id=xxx

# Cron Health Check
curl http://localhost:8081/cron/health

# Cron Stats
curl -H "X-Cron-Secret: your-secret" http://localhost:8081/cron/stats
```

### Test ConnectRPC Endpoints
```bash
# gRPC Protocol
grpcurl -plaintext localhost:8080 payment.v1.PaymentService/ListTransactions

# HTTP/JSON Protocol (automatic)
curl -X POST http://localhost:8080/payment.v1.PaymentService/ListTransactions \
  -H "Content-Type: application/json" \
  -d '{"merchant_id":"test","limit":10}'
```

---

**Document Version**: 1.0
**Last Updated**: 2025-11-18
**Author**: Architecture Analysis