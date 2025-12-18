package middleware

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"connectrpc.com/connect"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/kevin07696/payment-service/internal/auth"
)

// LoggingInterceptor creates a Connect interceptor for logging requests with full context
// Includes: request_id, merchant_id, trace_id, span_id for correlation
func LoggingInterceptor(logger *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()

			// Extract context fields for correlation
			fields := extractContextFields(ctx)
			fields = append(fields,
				zap.String("procedure", req.Spec().Procedure),
				zap.String("protocol", req.Peer().Protocol),
				zap.String("peer_addr", req.Peer().Addr),
			)

			logger.Info("RPC request", fields...)

			resp, err := next(ctx, req)

			// Add duration to fields
			duration := time.Since(start)
			fields = append(fields, zap.Duration("duration", duration))

			if err != nil {
				// Add error details
				fields = append(fields, zap.Error(err))
				if connectErr, ok := err.(*connect.Error); ok {
					fields = append(fields, zap.String("error_code", connectErr.Code().String()))
				}
				logger.Error("RPC error", fields...)
			} else {
				logger.Info("RPC response", fields...)
			}

			return resp, err
		}
	}
}

// extractContextFields extracts request_id, merchant_id, trace_id, span_id from context
func extractContextFields(ctx context.Context) []zap.Field {
	fields := make([]zap.Field, 0, 5)

	// Extract auth info (request_id, merchant_id)
	authInfo := auth.GetAuthInfo(ctx)
	if authInfo.RequestID != "" {
		fields = append(fields, zap.String("request_id", authInfo.RequestID))
	}
	if authInfo.MerchantID != "" {
		fields = append(fields, zap.String("merchant_id", authInfo.MerchantID))
	}
	if authInfo.ServiceID != "" {
		fields = append(fields, zap.String("service_id", authInfo.ServiceID))
	}

	// Extract OpenTelemetry trace context
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}

	return fields
}

// RecoveryInterceptor creates a Connect interceptor for panic recovery with full context
func RecoveryInterceptor(logger *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					// Include context fields for correlation
					fields := extractContextFields(ctx)
					fields = append(fields,
						zap.String("procedure", req.Spec().Procedure),
						zap.Any("panic", r),
						zap.String("stack", string(debug.Stack())),
					)
					logger.Error("Panic recovered in RPC handler", fields...)
					err = connect.NewError(
						connect.CodeInternal,
						fmt.Errorf("internal server error: panic recovered"),
					)
				}
			}()

			resp, err = next(ctx, req)
			return resp, err
		}
	}
}

// ContextLogger returns a logger with request context fields pre-populated
// Use this in handlers/services to log with automatic correlation
func ContextLogger(ctx context.Context, logger *zap.Logger) *zap.Logger {
	fields := extractContextFields(ctx)
	return logger.With(fields...)
}
