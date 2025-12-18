package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig holds configuration for distributed tracing
type TracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Endpoint       string // OTLP endpoint (e.g., "localhost:4318" for HTTP, "localhost:4317" for gRPC)
	Enabled        bool
	SampleRate     float64 // 0.0 to 1.0, where 1.0 = 100% sampling
}

// TracerProvider wraps the OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	config   TracingConfig
}

// InitTracing initializes OpenTelemetry tracing with OTLP exporter
// Returns a shutdown function that must be called on application exit
func InitTracing(ctx context.Context, config TracingConfig) (*TracerProvider, func(context.Context) error, error) {
	if !config.Enabled {
		// Return no-op provider when tracing is disabled
		return &TracerProvider{
			provider: sdktrace.NewTracerProvider(),
			config:   config,
		}, func(context.Context) error { return nil }, nil
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(config.Endpoint),
		otlptracehttp.WithInsecure(), // Use WithTLSCredentials() for production
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider with sampling
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(config.SampleRate),
	)

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider)

	// Set global propagator for distributed context
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	tp := &TracerProvider{
		provider: provider,
		config:   config,
	}

	// Shutdown function
	shutdown := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return provider.Shutdown(ctx)
	}

	return tp, shutdown, nil
}

// Tracer returns a tracer for the given name
func (tp *TracerProvider) Tracer(name string) trace.Tracer {
	return tp.provider.Tracer(name)
}

// PaymentSpanAttributes contains common attributes for payment tracing
type PaymentSpanAttributes struct {
	MerchantID      string
	CustomerID      string
	TransactionID   string
	TransactionType string
	AmountCents     int64
	Currency        string
	PaymentMethod   string
}

// AddPaymentAttributes adds payment-specific attributes to a span
func AddPaymentAttributes(span trace.Span, attrs PaymentSpanAttributes) {
	if attrs.MerchantID != "" {
		span.SetAttributes(attribute.String("payment.merchant_id", attrs.MerchantID))
	}
	if attrs.CustomerID != "" {
		span.SetAttributes(attribute.String("payment.customer_id", attrs.CustomerID))
	}
	if attrs.TransactionID != "" {
		span.SetAttributes(attribute.String("payment.transaction_id", attrs.TransactionID))
	}
	if attrs.TransactionType != "" {
		span.SetAttributes(attribute.String("payment.transaction_type", attrs.TransactionType))
	}
	if attrs.AmountCents > 0 {
		span.SetAttributes(attribute.Int64("payment.amount_cents", attrs.AmountCents))
	}
	if attrs.Currency != "" {
		span.SetAttributes(attribute.String("payment.currency", attrs.Currency))
	}
	if attrs.PaymentMethod != "" {
		span.SetAttributes(attribute.String("payment.method", attrs.PaymentMethod))
	}
}

// RecordError records an error on the span with additional context
func RecordError(span trace.Span, err error, description string) {
	if err == nil {
		return
	}
	span.RecordError(err, trace.WithAttributes(
		attribute.String("error.description", description),
	))
}

// RecordGatewayCall records attributes for external gateway calls
func RecordGatewayCall(span trace.Span, gateway string, endpoint string, responseCode string) {
	span.SetAttributes(
		attribute.String("gateway.name", gateway),
		attribute.String("gateway.endpoint", endpoint),
		attribute.String("gateway.response_code", responseCode),
	)
}

// P2-7: Distributed Tracing with OpenTelemetry
// This enables end-to-end request tracing across services for performance debugging and bottleneck identification
