package observability

import (
	"context"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/kevin07696/payment-service"
)

// TracingInterceptor creates a ConnectRPC interceptor that adds distributed tracing
func TracingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			// Extract trace context from incoming request headers
			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(req.Header()))

			// Start a new span
			tracer := otel.Tracer(tracerName)

			// Get schema as string (it may be nil)
			schema := ""
			if req.Spec().Schema != nil {
				if s, ok := req.Spec().Schema.(string); ok {
					schema = s
				}
			}

			ctx, span := tracer.Start(ctx, req.Spec().Procedure,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("rpc.system", "connect"),
					attribute.String("rpc.service", schema),
					attribute.String("rpc.method", req.Spec().Procedure),
				),
			)
			defer span.End()

			// Call the handler
			resp, err := next(ctx, req)

			// Record error if present
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())

				// Add gRPC-style status code
				if connectErr, ok := err.(*connect.Error); ok {
					span.SetAttributes(attribute.String("rpc.connect.status_code", connectErr.Code().String()))
				}
			} else {
				span.SetStatus(codes.Ok, "success")
			}

			// Inject trace context into response headers
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(resp.Header()))

			return resp, err
		}
	}
}

// Note: ConnectRPC streaming interceptors work differently than unary.
// For streaming, tracing should be added manually in the handler using StartSpan()
// since streaming connections don't have the same interceptor pattern.
//
// Example streaming handler with tracing:
//
//	func (s *PaymentService) StreamPayments(ctx context.Context, stream *connect.BidiStream[...]) error {
//	    ctx, span := observability.StartSpan(ctx, "payment_service.stream_payments")
//	    defer span.End()
//	    // ... streaming logic ...
//	}

// StartSpan is a helper to start a new span with common payment service attributes
func StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	return tracer.Start(ctx, spanName, opts...)
}

// StartDBSpan starts a span for database operations with standard attributes
// Usage:
//
//	ctx, span := observability.StartDBSpan(ctx, "GetPaymentMethod", "payment_methods")
//	defer span.End()
//	result, err := queries.GetPaymentMethod(ctx, id)
//	observability.EndDBSpan(span, err)
func StartDBSpan(ctx context.Context, operation string, table string) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "db."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", operation),
			attribute.String("db.sql.table", table),
		),
	)
	return ctx, span
}

// EndDBSpan records the result of a database operation on the span
func EndDBSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "database operation failed")
	} else {
		span.SetStatus(codes.Ok, "success")
	}
}

// StartDBSpanWithQuery starts a span for database operations including the SQL query
// SECURITY: Only use this for debugging - queries may contain sensitive data
func StartDBSpanWithQuery(ctx context.Context, operation string, table string, query string) (context.Context, trace.Span) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "db."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", operation),
			attribute.String("db.sql.table", table),
			attribute.String("db.statement", query),
		),
	)
	return ctx, span
}

// AddDBResultAttributes adds result-specific attributes to a database span
func AddDBResultAttributes(span trace.Span, rowsAffected int64) {
	span.SetAttributes(attribute.Int64("db.rows_affected", rowsAffected))
}

// P2-7: ConnectRPC Tracing Interceptors
// These interceptors automatically trace all RPC calls, extract/inject distributed context,
// and record errors for comprehensive request flow visibility
