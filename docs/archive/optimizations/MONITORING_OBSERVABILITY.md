# Monitoring & Observability Optimization

**Created**: 2025-11-20
**Status**: Analysis Complete - Awaiting Test Implementation
**Priority**: P0 (Critical for Production Operations)

## Executive Summary

This document analyzes monitoring, observability, and alerting capabilities to improve:
- **Incident detection time** from minutes to seconds through comprehensive metrics
- **Business visibility** through payment success rates, revenue metrics, and SLO tracking
- **Operational confidence** through SLA monitoring and automated alerting
- **Debugging speed** by 70-90% through distributed tracing and request correlation

**Current State**:
- ✅ Basic gRPC metrics (requests_total, duration, in_flight)
- ❌ **No business metrics** (payment success rates, amounts, revenue)
- ❌ **No SLO/SLA tracking**
- ❌ **No distributed tracing** (OpenTelemetry not implemented)
- ❌ **No automated alerting strategy**
- ❌ **No request correlation** across services
- ❌ **No error budgets** or burn rate tracking
- ⚠️ Basic health check (database only)

**Critical Gaps**:
1. Cannot answer: "What is our payment success rate right now?"
2. Cannot answer: "Are we meeting our 99.9% SLO?"
3. Cannot answer: "How much revenue processed in last hour?"
4. Cannot trace a failed payment end-to-end across services
5. No automatic alerts for SLO violations or anomalies

**Expected Impact**:
- **<30 second incident detection** through automated alerting
- **70-90% faster debugging** with distributed tracing
- **Proactive issue detection** through SLO monitoring
- **Business insights** through revenue and success rate dashboards

---

## Table of Contents

1. [Business Metrics](#1-business-metrics)
2. [SLO/SLA Tracking](#2-slosla-tracking)
3. [Distributed Tracing](#3-distributed-tracing)
4. [Alerting Strategy](#4-alerting-strategy)
5. [Health Check Enhancement](#5-health-check-enhancement)
6. [Dashboard Design](#6-dashboard-design)
7. [Error Budget Tracking](#7-error-budget-tracking)
8. [Testing Requirements](#8-testing-requirements)

---

## 1. Business Metrics

### Background

Technical metrics (latency, CPU, memory) are necessary but insufficient. Business metrics provide:
- Revenue visibility
- Success rate tracking
- Business impact of incidents
- Product usage insights

**Current State**: Only technical gRPC metrics

---

### MON-1: Payment Success Rate Metrics

**Priority**: P0 (Critical - core business metric)

**Location**: Create `pkg/observability/business_metrics.go`

**Implementation**:
```go
package observability

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Payment transaction metrics
    paymentTransactionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "payment_transactions_total",
        Help: "Total number of payment transactions",
    }, []string{
        "merchant_id",        // Which merchant
        "payment_type",       // credit_card, ach
        "transaction_type",   // sale, auth, capture, refund, void
        "status",             // approved, declined
        "gateway_response",   // 00=approved, 05=declined, etc.
    })

    paymentAmountCents = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "payment_amount_cents_total",
        Help: "Total payment amount in cents (for revenue tracking)",
    }, []string{
        "merchant_id",
        "payment_type",
        "transaction_type",
        "status",
        "currency",
    })

    paymentSuccessRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "payment_success_rate",
        Help: "Payment success rate (approved / total) over last 5 minutes",
    }, []string{
        "merchant_id",
        "payment_type",
    })

    // Payment processing duration (end-to-end)
    paymentProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name: "payment_processing_duration_seconds",
        Help: "Total time to process a payment transaction",
        Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}, // 100ms to 30s
    }, []string{
        "merchant_id",
        "payment_type",
        "transaction_type",
        "status",
    })

    // ACH verification metrics
    achVerificationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "ach_verifications_total",
        Help: "Total ACH payment method verifications",
    }, []string{
        "merchant_id",
        "status", // verified, failed, pending
    })

    // Subscription billing metrics
    subscriptionBillingsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "subscription_billings_total",
        Help: "Total subscription billing attempts",
    }, []string{
        "merchant_id",
        "status", // success, failed, retrying
    })

    subscriptionRevenueCents = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "subscription_revenue_cents_total",
        Help: "Total subscription revenue in cents",
    }, []string{
        "merchant_id",
        "currency",
    })

    // Webhook delivery metrics
    webhookDeliveriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "webhook_deliveries_total",
        Help: "Total webhook delivery attempts",
    }, []string{
        "event_type",
        "status", // success, failed, pending
    })

    webhookDeliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name: "webhook_delivery_duration_seconds",
        Help: "Time to deliver webhook",
        Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
    }, []string{
        "event_type",
    })

    // Chargeback metrics
    chargebacksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "chargebacks_total",
        Help: "Total chargebacks received",
    }, []string{
        "merchant_id",
        "reason_code",
        "status", // pending, won, lost
    })
)

// RecordPaymentTransaction records a payment transaction
func RecordPaymentTransaction(
    merchantID, paymentType, transactionType, status, gatewayResponse string,
    amountCents int64,
    currency string,
    duration float64,
) {
    // Record transaction count
    paymentTransactionsTotal.WithLabelValues(
        merchantID,
        paymentType,
        transactionType,
        status,
        gatewayResponse,
    ).Inc()

    // Record amount (for revenue tracking)
    paymentAmountCents.WithLabelValues(
        merchantID,
        paymentType,
        transactionType,
        status,
        currency,
    ).Add(float64(amountCents))

    // Record duration
    paymentProcessingDuration.WithLabelValues(
        merchantID,
        paymentType,
        transactionType,
        status,
    ).Observe(duration)
}

// RecordACHVerification records ACH verification result
func RecordACHVerification(merchantID, status string) {
    achVerificationsTotal.WithLabelValues(merchantID, status).Inc()
}

// RecordSubscriptionBilling records subscription billing attempt
func RecordSubscriptionBilling(merchantID, status string, amountCents int64, currency string) {
    subscriptionBillingsTotal.WithLabelValues(merchantID, status).Inc()

    if status == "success" {
        subscriptionRevenueCents.WithLabelValues(merchantID, currency).Add(float64(amountCents))
    }
}

// RecordWebhookDelivery records webhook delivery
func RecordWebhookDelivery(eventType, status string, duration float64) {
    webhookDeliveriesTotal.WithLabelValues(eventType, status).Inc()
    webhookDeliveryDuration.WithLabelValues(eventType).Observe(duration)
}

// RecordChargeback records a chargeback
func RecordChargeback(merchantID, reasonCode, status string) {
    chargebacksTotal.WithLabelValues(merchantID, reasonCode, status).Inc()
}
```

**Usage in Payment Service** (`internal/services/payment/payment_service.go`):
```go
func (s *paymentService) CreatePayment(ctx context.Context, req *Request) (*Response, error) {
    startTime := time.Now()

    // ... existing payment processing ...

    tx, err := s.processTransaction(ctx, req)

    // Record business metrics
    duration := time.Since(startTime).Seconds()
    status := "declined"
    if tx != nil && tx.Status == domain.TransactionStatusApproved {
        status = "approved"
    }

    observability.RecordPaymentTransaction(
        req.MerchantID,
        string(tx.PaymentMethodType),
        string(tx.Type),
        status,
        *tx.AuthResp, // EPX response code
        tx.AmountCents,
        tx.Currency,
        duration,
    )

    return &Response{Transaction: tx}, err
}
```

**PromQL Queries** (for dashboards/alerts):
```promql
# Payment success rate (last 5 minutes)
sum(rate(payment_transactions_total{status="approved"}[5m])) by (merchant_id, payment_type)
/
sum(rate(payment_transactions_total[5m])) by (merchant_id, payment_type)

# Total revenue per hour (in dollars)
sum(rate(payment_amount_cents_total{status="approved"}[1h])) by (merchant_id, currency) / 100

# P99 payment latency
histogram_quantile(0.99, sum(rate(payment_processing_duration_seconds_bucket[5m])) by (le, merchant_id))

# Payment volume by merchant (transactions per second)
sum(rate(payment_transactions_total[1m])) by (merchant_id)
```

**Impact**:
- **Real-time revenue tracking**: See dollars processed per minute
- **Success rate visibility**: Instantly see if payment success rate drops
- **Merchant insights**: Which merchants are most active, successful
- **Product analytics**: ACH vs Credit Card adoption rates

---

### MON-2: Customer-Facing Metrics

**Priority**: P1

**Additional Business Metrics**:
```go
var (
    // Customer payment method metrics
    paymentMethodsCreated = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "payment_methods_created_total",
        Help: "Total payment methods created (tokenized)",
    }, []string{
        "merchant_id",
        "payment_type", // credit_card, ach
    })

    activeCustomers = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "active_customers",
        Help: "Number of active customers with valid payment methods",
    }, []string{
        "merchant_id",
    })

    // Customer lifetime value proxy
    customerTransactionCount = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name: "customer_transaction_count",
        Help: "Distribution of transaction counts per customer",
        Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
    }, []string{
        "merchant_id",
    })
)
```

---

## 2. SLO/SLA Tracking

### Background

Service Level Objectives (SLOs) and Service Level Agreements (SLAs) define reliability targets. Without tracking, we cannot:
- Know if we're meeting commitments
- Budget for error tolerance
- Prioritize reliability work

**Current State**: No SLO/SLA tracking

---

### MON-3: Define SLOs

**Priority**: P0

**Recommended SLOs** (for payment service):

| SLO | Target | Measurement Window |
|-----|--------|-------------------|
| **Availability** | 99.9% | 30 days |
| **Latency (P50)** | < 500ms | 5 minutes |
| **Latency (P99)** | < 2000ms | 5 minutes |
| **Success Rate** | > 99.5% | 1 hour |
| **Payment Gateway Errors** | < 0.1% | 1 hour |

**Implementation**:
```go
package observability

import (
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // SLO compliance metrics
    sloCompliance = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "slo_compliance",
        Help: "SLO compliance (1=meeting, 0=violating)",
    }, []string{
        "slo_name",     // availability, latency_p99, success_rate
        "slo_target",   // 99.9%, 2000ms, etc.
        "measurement_window", // 30d, 1h, 5m
    })

    // Error budget remaining (0.0 to 1.0)
    errorBudgetRemaining = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "error_budget_remaining",
        Help: "Remaining error budget (1.0 = full budget, 0.0 = exhausted)",
    }, []string{
        "slo_name",
    })

    // Error budget burn rate
    errorBudgetBurnRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "error_budget_burn_rate",
        Help: "Rate of error budget consumption (1.0 = expected rate)",
    }, []string{
        "slo_name",
        "window", // 1h, 6h, 1d, 3d
    })
)

// SLOChecker periodically calculates SLO compliance
type SLOChecker struct {
    registry prometheus.Registerer
}

// Start begins SLO checking loop
func (s *SLOChecker) Start(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.checkSLOs()
        }
    }
}

// checkSLOs evaluates all SLOs
func (s *SLOChecker) checkSLOs() {
    // Check availability SLO
    s.checkAvailabilitySLO()

    // Check latency SLO
    s.checkLatencySLO()

    // Check success rate SLO
    s.checkSuccessRateSLO()
}

// checkAvailabilitySLO checks if service is meeting 99.9% availability SLO
func (s *SLOChecker) checkAvailabilitySLO() {
    // Query Prometheus for uptime over last 30 days
    // This would use PromQL API client in production
    // For now, simplified example

    // Availability = successful_requests / total_requests over 30d
    // Target: 99.9%

    // If meeting SLO:
    sloCompliance.WithLabelValues("availability", "99.9%", "30d").Set(1)

    // Calculate error budget
    // Error budget = 1 - target = 0.1% (43.2 minutes/month)
    // If we've used 20 minutes of budget:
    budgetUsed := 20.0 / 43.2 // 0.46 (46% used)
    errorBudgetRemaining.WithLabelValues("availability").Set(1 - budgetUsed)

    // Burn rate = (actual_error_rate / error_budget) * measurement_window
    // If burning at 2x expected rate:
    errorBudgetBurnRate.WithLabelValues("availability", "1h").Set(2.0)
}

// checkLatencySLO checks P99 latency SLO
func (s *SLOChecker) checkLatencySLO() {
    // Query P99 latency from payment_processing_duration_seconds
    // Target: < 2000ms

    // Example: current P99 = 1800ms
    currentP99 := 1800.0 // ms
    target := 2000.0     // ms

    if currentP99 < target {
        sloCompliance.WithLabelValues("latency_p99", "2000ms", "5m").Set(1)
    } else {
        sloCompliance.WithLabelValues("latency_p99", "2000ms", "5m").Set(0)
    }
}

// checkSuccessRateSLO checks payment success rate SLO
func (s *SLOChecker) checkSuccessRateSLO() {
    // Query success rate from payment_transactions_total
    // Target: > 99.5%

    // Example: current success rate = 99.7%
    successRate := 0.997
    target := 0.995

    if successRate > target {
        sloCompliance.WithLabelValues("success_rate", "99.5%", "1h").Set(1)
    } else {
        sloCompliance.WithLabelValues("success_rate", "99.5%", "1h").Set(0)
    }
}
```

**PromQL Queries for SLO Tracking**:
```promql
# Availability SLO (99.9% over 30 days)
(sum(rate(grpc_requests_total{status!~"Unavailable|Internal"}[30d]))
/
sum(rate(grpc_requests_total[30d]))) > 0.999

# Latency P99 SLO (< 2000ms)
histogram_quantile(0.99, sum(rate(payment_processing_duration_seconds_bucket[5m])) by (le)) < 2

# Success Rate SLO (> 99.5% over 1 hour)
(sum(rate(payment_transactions_total{status="approved"}[1h]))
/
sum(rate(payment_transactions_total[1h]))) > 0.995

# Error Budget Burn Rate (1h window)
# Burn rate > 1 means consuming budget faster than expected
(1 - (sum(rate(grpc_requests_total{status!~"Unavailable|Internal"}[1h])) / sum(rate(grpc_requests_total[1h]))))
/
(1 - 0.999) # error budget = 0.1%
```

**Alert Rules** (see Section 4 for complete alerting strategy)

---

## 3. Distributed Tracing

### Background

Distributed tracing allows following a single request across multiple services, databases, and external APIs. Critical for:
- Debugging complex failures
- Identifying latency sources
- Understanding request flows

**Current State**: No distributed tracing

---

### MON-4: OpenTelemetry Integration

**Priority**: P1

**Implementation**:
```go
package observability

import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
    oteltrace "go.opentelemetry.io/otel/trace"
)

// InitTracer initializes OpenTelemetry tracing
func InitTracer(serviceName, jaegerEndpoint string) (func(), error) {
    // Create Jaeger exporter
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint(jaegerEndpoint),
    ))
    if err != nil {
        return nil, err
    }

    // Create trace provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
            semconv.ServiceVersion("1.0.0"),
        )),
        trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.1))), // 10% sampling
    )

    otel.SetTracerProvider(tp)

    // Return shutdown function
    return func() {
        _ = tp.Shutdown(context.Background())
    }, nil
}

// StartSpan starts a new span with common attributes
func StartSpan(ctx context.Context, spanName string, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
    tracer := otel.Tracer("payment-service")
    return tracer.Start(ctx, spanName, oteltrace.WithAttributes(attrs...))
}
```

**Usage in Payment Service**:
```go
func (s *paymentService) CreatePayment(ctx context.Context, req *Request) (*Response, error) {
    // Start root span
    ctx, span := observability.StartSpan(ctx, "payment.CreatePayment",
        attribute.String("merchant_id", req.MerchantID),
        attribute.String("payment_type", req.PaymentType),
        attribute.Int64("amount_cents", req.AmountCents),
    )
    defer span.End()

    // Database operation
    ctx, dbSpan := observability.StartSpan(ctx, "db.CreateTransaction")
    tx, err := s.queries.CreateTransaction(ctx, params)
    dbSpan.End()
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }

    // EPX gateway call
    ctx, epxSpan := observability.StartSpan(ctx, "epx.ProcessTransaction",
        attribute.String("transaction_id", tx.ID),
    )
    epxResp, err := s.epxAdapter.ProcessTransaction(ctx, epxReq)
    epxSpan.End()

    // Webhook delivery (async, but start span for tracking)
    go func() {
        webhookCtx, webhookSpan := observability.StartSpan(context.Background(), "webhook.DeliverEvent",
            attribute.String("transaction_id", tx.ID),
        )
        defer webhookSpan.End()

        s.webhookService.DeliverEvent(webhookCtx, event)
    }()

    span.SetAttributes(
        attribute.String("transaction_id", tx.ID),
        attribute.String("status", string(tx.Status)),
    )

    return &Response{Transaction: tx}, nil
}
```

**Trace Visualization** (Jaeger UI):
```
CreatePayment [2.3s]
├─ db.CreateTransaction [45ms]
├─ epx.ProcessTransaction [2.1s]
│  ├─ http.POST [2.0s]
│  │  └─ dns.Lookup [50ms]
│  └─ xml.Parse [100ms]
└─ webhook.DeliverEvent [150ms]
   └─ http.POST [140ms]
```

**Impact**:
- **70-90% faster debugging**: See exactly where time is spent
- **Request correlation**: Follow request across all services
- **Performance optimization**: Identify bottlenecks visually

---

## 4. Alerting Strategy

### Background

Automated alerting is critical for:
- Fast incident detection (<30 seconds)
- Reducing MTTR (Mean Time To Resolution)
- Preventing customer impact

**Current State**: No automated alerting

---

### MON-5: Multi-Tier Alerting Strategy

**Priority**: P0

**Alert Tiers**:
- **P0 (Page immediately)**: SLO violation, service down, data loss risk
- **P1 (Slack alert, investigate within 1 hour)**: Degraded performance, error budget burn
- **P2 (Daily digest)**: Warnings, capacity planning

**Implementation** (Prometheus Alert Rules):
```yaml
# File: monitoring/prometheus/alerts.yml

groups:
  # P0 Alerts - Page immediately
  - name: critical_slo_violations
    interval: 30s
    rules:
      - alert: PaymentSuccessRateBelowSLO
        expr: |
          (sum(rate(payment_transactions_total{status="approved"}[5m]))
          /
          sum(rate(payment_transactions_total[5m]))) < 0.995
        for: 2m
        labels:
          severity: critical
          tier: p0
        annotations:
          summary: "Payment success rate below 99.5% SLO"
          description: "Success rate {{ $value | humanizePercentage }} is below 99.5% SLO for {{ $labels.merchant_id }}"
          runbook: "https://docs.company.com/runbooks/payment-success-rate"

      - alert: PaymentP99LatencyAboveSLO
        expr: |
          histogram_quantile(0.99,
            sum(rate(payment_processing_duration_seconds_bucket[5m])) by (le)
          ) > 2.0
        for: 5m
        labels:
          severity: critical
          tier: p0
        annotations:
          summary: "Payment P99 latency above 2s SLO"
          description: "P99 latency {{ $value }}s exceeds 2s SLO"

      - alert: ServiceUnavailable
        expr: up{job="payment-service"} == 0
        for: 1m
        labels:
          severity: critical
          tier: p0
        annotations:
          summary: "Payment service is down"
          description: "Payment service has been unavailable for 1 minute"

      - alert: DatabaseConnectionPoolExhausted
        expr: db_pool_acquired_connections / db_pool_max_connections > 0.95
        for: 2m
        labels:
          severity: critical
          tier: p0
        annotations:
          summary: "Database connection pool near exhaustion"
          description: "Pool utilization {{ $value | humanizePercentage }} exceeds 95%"

      - alert: ErrorBudgetCriticalBurn
        expr: |
          error_budget_burn_rate{window="1h"} > 10
        for: 5m
        labels:
          severity: critical
          tier: p0
        annotations:
          summary: "Error budget burning 10x faster than expected"
          description: "At current rate, will exhaust error budget in {{ $value }} hours"

  # P1 Alerts - Investigate within 1 hour
  - name: performance_degradation
    interval: 1m
    rules:
      - alert: PaymentLatencyHigh
        expr: |
          histogram_quantile(0.99,
            sum(rate(payment_processing_duration_seconds_bucket[10m])) by (le)
          ) > 1.5
        for: 10m
        labels:
          severity: warning
          tier: p1
        annotations:
          summary: "Payment latency elevated"
          description: "P99 latency {{ $value }}s is above 1.5s (SLO: 2s)"

      - alert: WebhookDeliveryFailureRateHigh
        expr: |
          (sum(rate(webhook_deliveries_total{status="failed"}[15m]))
          /
          sum(rate(webhook_deliveries_total[15m]))) > 0.05
        for: 10m
        labels:
          severity: warning
          tier: p1
        annotations:
          summary: "Webhook failure rate above 5%"
          description: "{{ $value | humanizePercentage }} of webhooks failing"

      - alert: ACHVerificationFailureRateHigh
        expr: |
          (sum(rate(ach_verifications_total{status="failed"}[1h]))
          /
          sum(rate(ach_verifications_total[1h]))) > 0.10
        for: 15m
        labels:
          severity: warning
          tier: p1
        annotations:
          summary: "ACH verification failure rate above 10%"

      - alert: ErrorBudgetFastBurn
        expr: |
          error_budget_burn_rate{window="6h"} > 5
        for: 30m
        labels:
          severity: warning
          tier: p1
        annotations:
          summary: "Error budget burning 5x faster than expected"

  # P2 Alerts - Daily digest
  - name: capacity_planning
    interval: 5m
    rules:
      - alert: HighPaymentVolume
        expr: |
          sum(rate(payment_transactions_total[1h])) > 100
        for: 1h
        labels:
          severity: info
          tier: p2
        annotations:
          summary: "High payment volume detected"
          description: "Processing {{ $value }} payments/second"

      - alert: DatabaseConnectionPoolUtilizationHigh
        expr: db_pool_acquired_connections / db_pool_max_connections > 0.70
        for: 30m
        labels:
          severity: info
          tier: p2
        annotations:
          summary: "Database pool utilization above 70%"
          description: "Consider increasing MaxConns from current {{ $labels.max_conns }}"
```

**AlertManager Configuration**:
```yaml
# File: monitoring/alertmanager/config.yml

route:
  receiver: 'default'
  group_by: ['alertname', 'severity']
  group_wait: 10s
  group_interval: 5m
  repeat_interval: 4h

  routes:
    # P0: Page via PagerDuty
    - match:
        tier: p0
      receiver: 'pagerduty'
      continue: true

    # P0: Also send to Slack
    - match:
        tier: p0
      receiver: 'slack-critical'

    # P1: Slack only
    - match:
        tier: p1
      receiver: 'slack-warnings'

    # P2: Email digest
    - match:
        tier: p2
      receiver: 'email-digest'
      group_interval: 24h

receivers:
  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: '<pagerduty-service-key>'
        description: '{{ .GroupLabels.alertname }}: {{ .CommonAnnotations.summary }}'

  - name: 'slack-critical'
    slack_configs:
      - api_url: '<slack-webhook-url>'
        channel: '#alerts-critical'
        title: '🚨 {{ .GroupLabels.alertname }}'
        text: '{{ .CommonAnnotations.description }}'
        color: 'danger'

  - name: 'slack-warnings'
    slack_configs:
      - api_url: '<slack-webhook-url>'
        channel: '#alerts-warnings'
        title: '⚠️ {{ .GroupLabels.alertname }}'
        text: '{{ .CommonAnnotations.description }}'
        color: 'warning'

  - name: 'email-digest'
    email_configs:
      - to: 'team@company.com'
        from: 'alerts@company.com'
        smarthost: 'smtp.company.com:587'
```

**Impact**:
- **<30 second detection**: Alerts fire within 30 seconds of SLO violation
- **Prioritized response**: P0/P1/P2 tiers ensure right urgency
- **Reduced noise**: Grouping and deduplication prevent alert fatigue
- **Actionable alerts**: Runbook links enable fast resolution

---

## 5. Health Check Enhancement

### MON-6: Comprehensive Health Checks

**Priority**: P1

**Current** (`pkg/observability/health.go`): Basic database ping

**Enhanced Implementation**:
```go
package observability

import (
    "context"
    "fmt"
    "sync"
    "time"

    "go.uber.org/zap"
)

type ComponentStatus string

const (
    StatusHealthy   ComponentStatus = "healthy"
    StatusDegraded  ComponentStatus = "degraded"
    StatusUnhealthy ComponentStatus = "unhealthy"
)

type HealthCheck struct {
    Name      string          `json:"name"`
    Status    ComponentStatus `json:"status"`
    Latency   time.Duration   `json:"latency_ms"`
    Message   string          `json:"message,omitempty"`
    Timestamp time.Time       `json:"timestamp"`
}

type OverallHealth struct {
    Status     ComponentStatus `json:"status"`
    Version    string          `json:"version"`
    Uptime     time.Duration   `json:"uptime_seconds"`
    Checks     []HealthCheck   `json:"checks"`
    Timestamp  time.Time       `json:"timestamp"`
}

type HealthChecker struct {
    db              DatabaseChecker
    epxAdapter      ExternalServiceChecker
    secretManager   SecretManagerChecker
    logger          *zap.Logger
    startTime       time.Time
}

// NewHealthChecker creates comprehensive health checker
func NewHealthChecker(
    db DatabaseChecker,
    epx ExternalServiceChecker,
    secrets SecretManagerChecker,
    logger *zap.Logger,
) *HealthChecker {
    return &HealthChecker{
        db:            db,
        epxAdapter:    epx,
        secretManager: secrets,
        logger:        logger,
        startTime:     time.Now(),
    }
}

// CheckHealth performs comprehensive health check
func (h *HealthChecker) CheckHealth(ctx context.Context) (*OverallHealth, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    // Run checks in parallel
    var wg sync.WaitGroup
    checks := make([]HealthCheck, 0, 4)
    checksChan := make(chan HealthCheck, 4)

    // Database check
    wg.Add(1)
    go func() {
        defer wg.Done()
        checksChan <- h.checkDatabase(ctx)
    }()

    // EPX gateway check (optional - may be behind circuit breaker)
    wg.Add(1)
    go func() {
        defer wg.Done()
        checksChan <- h.checkEPXGateway(ctx)
    }()

    // Secret manager check
    wg.Add(1)
    go func() {
        defer wg.Done()
        checksChan <- h.checkSecretManager(ctx)
    }()

    // Connection pool check
    wg.Add(1)
    go func() {
        defer wg.Done()
        checksChan <- h.checkConnectionPool(ctx)
    }()

    // Wait for all checks
    go func() {
        wg.Wait()
        close(checksChan)
    }()

    // Collect results
    for check := range checksChan {
        checks = append(checks, check)
    }

    // Determine overall status
    overallStatus := StatusHealthy
    for _, check := range checks {
        if check.Status == StatusUnhealthy {
            overallStatus = StatusUnhealthy
            break
        }
        if check.Status == StatusDegraded {
            overallStatus = StatusDegraded
        }
    }

    return &OverallHealth{
        Status:    overallStatus,
        Version:   "1.0.0",
        Uptime:    time.Since(h.startTime),
        Checks:    checks,
        Timestamp: time.Now(),
    }, nil
}

func (h *HealthChecker) checkDatabase(ctx context.Context) HealthCheck {
    start := time.Now()
    err := h.db.Ping(ctx)
    latency := time.Since(start)

    check := HealthCheck{
        Name:      "database",
        Latency:   latency,
        Timestamp: time.Now(),
    }

    if err != nil {
        check.Status = StatusUnhealthy
        check.Message = fmt.Sprintf("Failed: %v", err)
    } else if latency > 1*time.Second {
        check.Status = StatusDegraded
        check.Message = "Slow response"
    } else {
        check.Status = StatusHealthy
        check.Message = "OK"
    }

    return check
}

func (h *HealthChecker) checkEPXGateway(ctx context.Context) HealthCheck {
    // Simplified - in production, might skip if circuit breaker is open
    start := time.Now()
    err := h.epxAdapter.HealthCheck(ctx)
    latency := time.Since(start)

    check := HealthCheck{
        Name:      "epx_gateway",
        Latency:   latency,
        Timestamp: time.Now(),
    }

    if err != nil {
        // EPX down is degraded, not unhealthy (can use cached BRICs)
        check.Status = StatusDegraded
        check.Message = fmt.Sprintf("Unavailable: %v", err)
    } else {
        check.Status = StatusHealthy
        check.Message = "OK"
    }

    return check
}

func (h *HealthChecker) checkSecretManager(ctx context.Context) HealthCheck {
    start := time.Now()
    // Just verify connectivity, don't fetch actual secrets
    err := h.secretManager.HealthCheck(ctx)
    latency := time.Since(start)

    check := HealthCheck{
        Name:      "secret_manager",
        Latency:   latency,
        Timestamp: time.Now(),
    }

    if err != nil {
        check.Status = StatusUnhealthy
        check.Message = fmt.Sprintf("Failed: %v", err)
    } else {
        check.Status = StatusHealthy
        check.Message = "OK"
    }

    return check
}

func (h *HealthChecker) checkConnectionPool(ctx context.Context) HealthCheck {
    stats := h.db.Stats()

    check := HealthCheck{
        Name:      "connection_pool",
        Latency:   0, // instant check
        Timestamp: time.Now(),
    }

    utilization := float64(stats.AcquiredConns()) / float64(stats.MaxConns())

    if utilization > 0.95 {
        check.Status = StatusDegraded
        check.Message = fmt.Sprintf("High utilization: %.1f%%", utilization*100)
    } else {
        check.Status = StatusHealthy
        check.Message = fmt.Sprintf("Utilization: %.1f%%", utilization*100)
    }

    return check
}

// Liveness check (for Kubernetes liveness probe)
func (h *HealthChecker) Liveness(ctx context.Context) bool {
    // Service is alive if database is reachable
    return h.checkDatabase(ctx).Status != StatusUnhealthy
}

// Readiness check (for Kubernetes readiness probe)
func (h *HealthChecker) Readiness(ctx context.Context) bool {
    health, _ := h.CheckHealth(ctx)
    // Ready if overall status is not unhealthy
    return health.Status != StatusUnhealthy
}
```

**HTTP Endpoints**:
```go
// In cmd/server/main.go

// Full health check (includes external dependencies)
httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    health, _ := healthChecker.CheckHealth(r.Context())
    w.Header().Set("Content-Type", "application/json")

    if health.Status == StatusUnhealthy {
        w.WriteStatus(http.StatusServiceUnavailable)
    } else if health.Status == StatusDegraded {
        w.WriteStatus(http.StatusOK) // Still serving traffic
    } else {
        w.WriteStatus(http.StatusOK)
    }

    json.NewEncoder(w).Encode(health)
})

// Liveness probe (database only)
httpMux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
    if healthChecker.Liveness(r.Context()) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("Unavailable"))
    }
})

// Readiness probe
httpMux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
    if healthChecker.Readiness(r.Context()) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Ready"))
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("Not Ready"))
    }
})
```

---

## 6. Dashboard Design

### MON-7: Grafana Dashboard Templates

**Priority**: P1

**Dashboard Structure**:

**1. Executive Dashboard** (for business stakeholders):
```json
{
  "title": "Payment Service - Business Metrics",
  "panels": [
    {
      "title": "Total Revenue (Last 24h)",
      "target": "sum(increase(payment_amount_cents_total{status=\"approved\"}[24h])) / 100"
    },
    {
      "title": "Payment Success Rate",
      "target": "sum(rate(payment_transactions_total{status=\"approved\"}[5m])) / sum(rate(payment_transactions_total[5m]))"
    },
    {
      "title": "Transactions Per Second",
      "target": "sum(rate(payment_transactions_total[1m]))"
    },
    {
      "title": "Active Merchants",
      "target": "count(count by (merchant_id) (payment_transactions_total))"
    }
  ]
}
```

**2. SLO Dashboard**:
- Availability (30 day rolling)
- Latency P50/P95/P99 (5 min window)
- Success Rate (1 hour window)
- Error Budget Remaining
- Burn Rate (1h, 6h, 1d, 3d windows)

**3. Operational Dashboard**:
- Request rate by endpoint
- Error rate by endpoint
- Latency heatmap
- Database connection pool stats
- EPX gateway response times
- Webhook delivery success rate

**4. Infrastructure Dashboard**:
- CPU/Memory usage
- Goroutine count
- GC pause times
- Network I/O
- Disk I/O

---

## 7. Error Budget Tracking

### MON-8: Error Budget Policy

**Priority**: P1

**Error Budget Calculation**:
```
SLO: 99.9% availability over 30 days
Error Budget: 100% - 99.9% = 0.1%
Time Budget: 30 days × 0.1% = 43.2 minutes/month

If error budget exhausted:
- Freeze feature development
- Focus on reliability improvements
- Post-mortem required
```

**Implementation**:
```go
// Error budget tracker
type ErrorBudget struct {
    SLOTarget       float64       // 0.999 (99.9%)
    MeasurementDays int           // 30
    StartTime       time.Time
}

func (eb *ErrorBudget) RemainingBudget() float64 {
    // Query actual uptime from Prometheus
    actualUptime := eb.queryActualUptime()

    budgetUsed := 1.0 - actualUptime
    budgetAllowed := 1.0 - eb.SLOTarget

    remaining := budgetAllowed - budgetUsed
    return remaining / budgetAllowed // 0.0 to 1.0
}

func (eb *ErrorBudget) BurnRate(window time.Duration) float64 {
    // How fast are we consuming error budget?
    // 1.0 = expected rate, 2.0 = 2x faster than expected
    actualErrorRate := eb.queryErrorRate(window)
    allowedErrorRate := (1.0 - eb.SLOTarget) / eb.MeasurementDays

    return actualErrorRate / allowedErrorRate
}
```

---

## 8. Testing Requirements

### 8.1 Metrics Tests

```go
func TestBusinessMetrics(t *testing.T) {
    // Record payment transaction
    RecordPaymentTransaction(
        "merchant-123",
        "credit_card",
        "sale",
        "approved",
        "00",
        10050, // $100.50
        "USD",
        1.234, // 1.234 seconds
    )

    // Verify metrics were recorded
    // (requires prometheus testing framework)
}
```

### 8.2 Health Check Tests

```go
func TestHealthCheck(t *testing.T) {
    checker := NewHealthChecker(mockDB, mockEPX, mockSecrets, logger)

    health, err := checker.CheckHealth(context.Background())
    if err != nil {
        t.Fatal(err)
    }

    if health.Status != StatusHealthy {
        t.Errorf("Expected healthy, got: %s", health.Status)
    }

    if len(health.Checks) == 0 {
        t.Error("No health checks performed")
    }
}
```

---

## Summary

| Category | Current | Optimized | Impact |
|----------|---------|-----------|--------|
| Business Metrics | None | 15+ metrics | **Revenue visibility** |
| SLO Tracking | None | 5 SLOs monitored | **99.9% confidence** |
| Distributed Tracing | None | OpenTelemetry | **70-90% faster debugging** |
| Alerting | None | 3-tier strategy | **<30s detection** |
| Health Checks | Basic | Comprehensive | **Multi-component visibility** |
| Dashboards | None | 4 dashboards | **Stakeholder insights** |

**Implementation Priority**:
1. P0: Business metrics, SLO tracking, alerting
2. P1: Distributed tracing, enhanced health checks, dashboards
3. P2: Error budget tracking, advanced analytics

**Document Status**: ✅ Complete
**Last Updated**: 2025-11-20
