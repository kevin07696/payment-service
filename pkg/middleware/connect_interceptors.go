package middleware

import (
	"context"
	"fmt"
	"runtime/debug"

	"connectrpc.com/connect"
	"go.uber.org/zap"
)

// LoggingInterceptor creates a Connect interceptor for logging requests
func LoggingInterceptor(logger *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			logger.Info("RPC request",
				zap.String("procedure", req.Spec().Procedure),
				zap.String("protocol", req.Peer().Protocol),
			)

			resp, err := next(ctx, req)

			if err != nil {
				logger.Error("RPC error",
					zap.String("procedure", req.Spec().Procedure),
					zap.Error(err),
				)
			} else {
				logger.Info("RPC response",
					zap.String("procedure", req.Spec().Procedure),
				)
			}

			return resp, err
		}
	}
}

// RecoveryInterceptor creates a Connect interceptor for panic recovery
func RecoveryInterceptor(logger *zap.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic recovered in RPC handler",
						zap.String("procedure", req.Spec().Procedure),
						zap.Any("panic", r),
						zap.String("stack", string(debug.Stack())),
					)
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
