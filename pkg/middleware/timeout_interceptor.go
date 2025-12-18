package middleware

import (
	"context"

	"connectrpc.com/connect"
	"github.com/kevin07696/payment-service/pkg/resilience"
	"go.uber.org/zap"
)

// TimeoutInterceptor adds timeout hierarchy to ConnectRPC handlers
type TimeoutInterceptor struct {
	config *resilience.TimeoutConfig
	logger *zap.Logger
}

// NewTimeoutInterceptor creates a new timeout interceptor
func NewTimeoutInterceptor(config *resilience.TimeoutConfig, logger *zap.Logger) *TimeoutInterceptor {
	return &TimeoutInterceptor{
		config: config,
		logger: logger,
	}
}

// WrapUnary wraps unary RPC calls with handler timeout
func (i *TimeoutInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Check if context already has a deadline
		if _, hasDeadline := ctx.Deadline(); hasDeadline {
			// Parent context has deadline - respect it
			i.logger.Debug("Context already has deadline, respecting parent timeout",
				zap.String("procedure", req.Spec().Procedure),
			)
			return next(ctx, req)
		}

		// Add handler-level timeout
		timeoutCtx, cancel := i.config.HandlerContext(ctx)
		defer cancel()

		i.logger.Debug("Applied handler timeout",
			zap.String("procedure", req.Spec().Procedure),
			zap.Duration("timeout", i.config.HTTPHandler),
		)

		return next(timeoutCtx, req)
	}
}

// WrapStreamingClient wraps client streaming RPC calls
func (i *TimeoutInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		// For streaming, use a longer timeout or no timeout
		// Streaming connections may be long-lived
		return next(ctx, spec)
	}
}

// WrapStreamingHandler wraps server streaming RPC calls
func (i *TimeoutInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		// For streaming, use a longer timeout or no timeout
		// Streaming connections may be long-lived
		return next(ctx, conn)
	}
}
