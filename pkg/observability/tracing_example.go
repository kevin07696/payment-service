package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Example: How to use distributed tracing in payment service handlers
//
// This file demonstrates integration patterns. Actual integration should be done
// in the payment service handlers and adapters.

// ExamplePaymentServiceTracing shows how to add tracing to payment processing
func ExamplePaymentServiceTracing(ctx context.Context, merchantID, customerID string, amountCents int64) error {
	// Start a span for the payment operation
	ctx, span := StartSpan(ctx, "payment.process_transaction",
		trace.WithAttributes(
			attribute.String("merchant_id", merchantID),
			attribute.String("customer_id", customerID),
			attribute.Int64("amount_cents", amountCents),
		),
	)
	defer span.End()

	// Add custom attributes as processing progresses
	span.SetAttributes(attribute.String("transaction.type", "SALE"))

	// Example: Trace database query
	ctx, dbSpan := StartSpan(ctx, "db.find_payment_method")
	defer dbSpan.End()
	// ... database operation ...
	dbSpan.SetAttributes(attribute.String("db.operation", "SELECT"))

	// Example: Trace gateway call
	_, gatewaySpan := StartSpan(ctx, "gateway.epx.server_post")
	defer gatewaySpan.End()
	// ... EPX API call ...
	RecordGatewayCall(gatewaySpan, "EPX", "/ServerPOST.asmx", "00")

	return nil
}

// ExampleDatabaseTracing shows how to add tracing to database operations
func ExampleDatabaseTracing(ctx context.Context, query string) error {
	_, span := StartSpan(ctx, "db.query",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
		),
	)
	defer span.End()

	// Add query information (be careful not to log sensitive data)
	span.SetAttributes(attribute.String("db.statement", sanitizeQuery(query)))

	// ... execute query ...

	return nil
}

// ExampleCacheTracing shows how to add tracing to cache operations
func ExampleCacheTracing(ctx context.Context, key string) (interface{}, error) {
	_, span := StartSpan(ctx, "cache.lookup",
		trace.WithAttributes(
			attribute.String("cache.key", key),
		),
	)
	defer span.End()

	// ... cache lookup ...
	hit := true // example
	span.SetAttributes(attribute.Bool("cache.hit", hit))

	return nil, nil
}

// sanitizeQuery removes sensitive data from SQL queries for tracing
func sanitizeQuery(query string) string {
	// In production, implement proper query sanitization
	// For now, just return the first 100 characters
	if len(query) > 100 {
		return query[:100] + "..."
	}
	return query
}

/*
Integration Guide:

1. Initialize tracing in cmd/server/main.go:

	config := observability.TracingConfig{
		ServiceName:    "payment-service",
		ServiceVersion: "1.0.0",
		Environment:    os.Getenv("ENVIRONMENT"),
		Endpoint:       os.Getenv("OTLP_ENDPOINT"), // e.g., "localhost:4318"
		Enabled:        os.Getenv("TRACING_ENABLED") == "true",
		SampleRate:     0.1, // 10% sampling in production
	}

	tp, shutdown, err := observability.InitTracing(ctx, config)
	if err != nil {
		log.Fatalf("failed to initialize tracing: %v", err)
	}
	defer shutdown(ctx)

2. Add interceptors to ConnectRPC server:

	interceptors := connect.WithInterceptors(
		observability.TracingInterceptor(),
	)

	mux := http.NewServeMux()
	mux.Handle(paymentv1connect.NewPaymentServiceHandler(
		paymentHandler,
		interceptors,
	))

3. Add spans to service methods:

	func (s *PaymentService) ProcessPayment(ctx context.Context, req *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
		ctx, span := observability.StartSpan(ctx, "payment_service.process_payment")
		defer span.End()

		// Add payment attributes
		observability.AddPaymentAttributes(span, observability.PaymentSpanAttributes{
			MerchantID:      req.MerchantId,
			CustomerID:      req.CustomerId,
			AmountCents:     req.AmountCents,
			Currency:        req.Currency,
			TransactionType: "SALE",
		})

		// ... business logic ...

		if err != nil {
			observability.RecordError(span, err, "payment processing failed")
			return nil, err
		}

		return resp, nil
	}

4. Run with Jaeger for local development:

	docker run -d --name jaeger \
		-e COLLECTOR_OTLP_ENABLED=true \
		-p 16686:16686 \
		-p 4318:4318 \
		jaegertracing/all-in-one:latest

	Access UI at: http://localhost:16686

5. For production, use Tempo or other OTLP-compatible backend:

	# Grafana Tempo endpoint
	OTLP_ENDPOINT=tempo.example.com:4318
	TRACING_ENABLED=true

Expected Benefits:
- End-to-end request tracing across all service components
- Gateway call timing and success rates visible in traces
- Database query performance visibility
- Cache hit/miss patterns in distributed context
- Error propagation tracking across service boundaries
- P50/P90/P99 latency breakdowns per operation
*/
