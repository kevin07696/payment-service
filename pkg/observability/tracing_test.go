package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTestTracer creates a test tracer that records spans in memory
func setupTestTracer(t *testing.T) (*tracetest.InMemoryExporter, func()) {
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	oldTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)

	cleanup := func() {
		otel.SetTracerProvider(oldTP)
		_ = tp.Shutdown(context.Background())
	}

	return exporter, cleanup
}

func TestStartSpan(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartSpan(ctx, "test.operation")
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "test.operation", spans[0].Name)
}

func TestStartDBSpan(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartDBSpan(ctx, "GetPaymentMethod", "payment_methods")
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "db.GetPaymentMethod", spans[0].Name)

	// Check attributes
	attrs := spans[0].Attributes
	assertHasAttribute(t, attrs, "db.system", "postgresql")
	assertHasAttribute(t, attrs, "db.operation", "GetPaymentMethod")
	assertHasAttribute(t, attrs, "db.sql.table", "payment_methods")
}

func TestEndDBSpan_Success(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartDBSpan(ctx, "GetPaymentMethod", "payment_methods")
	EndDBSpan(span, nil)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	// Status should be OK on success
	assert.Equal(t, "Ok", spans[0].Status.Code.String())
}

func TestEndDBSpan_Error(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartDBSpan(ctx, "GetPaymentMethod", "payment_methods")
	testErr := errors.New("connection refused")
	EndDBSpan(span, testErr)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	// Status should be Error on failure
	assert.Equal(t, "Error", spans[0].Status.Code.String())
	// Error should be recorded
	require.Len(t, spans[0].Events, 1)
	assert.Equal(t, "exception", spans[0].Events[0].Name)
}

func TestAddDBResultAttributes(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartDBSpan(ctx, "ListPaymentMethods", "payment_methods")
	AddDBResultAttributes(span, 5)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assertHasAttributeInt(t, spans[0].Attributes, "db.rows_affected", 5)
}

func TestStartDBSpanWithQuery(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	query := "SELECT * FROM payment_methods WHERE merchant_id = $1"
	_, span := StartDBSpanWithQuery(ctx, "ListPaymentMethods", "payment_methods", query)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assertHasAttribute(t, spans[0].Attributes, "db.statement", query)
}

func TestNestedSpans(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()

	// Parent span (e.g., API handler)
	ctx, parentSpan := StartSpan(ctx, "api.GetSubscription")

	// Child span (e.g., database query)
	_, childSpan := StartDBSpan(ctx, "GetSubscriptionByID", "subscriptions")
	childSpan.End()

	parentSpan.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 2)

	// Find spans by name
	var dbSpan, apiSpan *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "db.GetSubscriptionByID" {
			dbSpan = &spans[i]
		} else if spans[i].Name == "api.GetSubscription" {
			apiSpan = &spans[i]
		}
	}

	require.NotNil(t, dbSpan, "database span should exist")
	require.NotNil(t, apiSpan, "API span should exist")

	// Verify parent-child relationship via trace ID
	assert.Equal(t, apiSpan.SpanContext.TraceID(), dbSpan.SpanContext.TraceID())
	assert.Equal(t, apiSpan.SpanContext.SpanID(), dbSpan.Parent.SpanID())
}

func TestRecordGatewayCall(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartSpan(ctx, "epx.server_post")
	RecordGatewayCall(span, "EPX", "/ServerPOST.asmx", "00")
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assertHasAttribute(t, spans[0].Attributes, "gateway.name", "EPX")
	assertHasAttribute(t, spans[0].Attributes, "gateway.endpoint", "/ServerPOST.asmx")
	assertHasAttribute(t, spans[0].Attributes, "gateway.response_code", "00")
}

func TestRecordError(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartSpan(ctx, "test.operation")
	testErr := errors.New("something went wrong")
	RecordError(span, testErr, "operation failed")
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	// RecordError only records the error event, doesn't set status
	// Status setting is done separately by the caller
	require.Len(t, spans[0].Events, 1)
	assert.Equal(t, "exception", spans[0].Events[0].Name)
	// Verify error description attribute is recorded
	found := false
	for _, attr := range spans[0].Events[0].Attributes {
		if string(attr.Key) == "error.description" {
			assert.Equal(t, "operation failed", attr.Value.AsString())
			found = true
		}
	}
	assert.True(t, found, "error.description attribute should be present")
}

func TestAddPaymentAttributes(t *testing.T) {
	exporter, cleanup := setupTestTracer(t)
	defer cleanup()

	ctx := context.Background()
	_, span := StartSpan(ctx, "payment.process")
	AddPaymentAttributes(span, PaymentSpanAttributes{
		MerchantID:      "merchant-123",
		CustomerID:      "customer-456",
		AmountCents:     10000,
		Currency:        "USD",
		TransactionType: "SALE",
		PaymentMethod:   "credit_card",
	})
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assertHasAttribute(t, spans[0].Attributes, "payment.merchant_id", "merchant-123")
	assertHasAttribute(t, spans[0].Attributes, "payment.customer_id", "customer-456")
	assertHasAttributeInt(t, spans[0].Attributes, "payment.amount_cents", 10000)
	assertHasAttribute(t, spans[0].Attributes, "payment.currency", "USD")
	assertHasAttribute(t, spans[0].Attributes, "payment.transaction_type", "SALE")
	assertHasAttribute(t, spans[0].Attributes, "payment.method", "credit_card")
}

// Helper functions for attribute assertions

func assertHasAttribute(t *testing.T, attrs []attribute.KeyValue, key, expectedValue string) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			assert.Equal(t, expectedValue, attr.Value.AsString(), "attribute %s value mismatch", key)
			return
		}
	}
	t.Errorf("attribute %s not found", key)
}

func assertHasAttributeInt(t *testing.T, attrs []attribute.KeyValue, key string, expectedValue int64) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			assert.Equal(t, expectedValue, attr.Value.AsInt64(), "attribute %s value mismatch", key)
			return
		}
	}
	t.Errorf("attribute %s not found", key)
}
