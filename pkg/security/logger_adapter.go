package security

import (
	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"go.uber.org/zap"
)

// ZapLoggerAdapter adapts zap.Logger to our Logger port interface
type ZapLoggerAdapter struct {
	logger *zap.Logger
}

// NewZapLogger creates a new ZapLoggerAdapter
func NewZapLogger(logger *zap.Logger) *ZapLoggerAdapter {
	return &ZapLoggerAdapter{logger: logger}
}

// NewZapLoggerDevelopment creates a development logger
func NewZapLoggerDevelopment() (*ZapLoggerAdapter, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &ZapLoggerAdapter{logger: logger}, nil
}

// NewZapLoggerProduction creates a production logger
func NewZapLoggerProduction() (*ZapLoggerAdapter, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &ZapLoggerAdapter{logger: logger}, nil
}

// Info logs an info message
func (z *ZapLoggerAdapter) Info(msg string, fields ...ports.Field) {
	z.logger.Info(msg, convertFields(fields)...)
}

// Error logs an error message
func (z *ZapLoggerAdapter) Error(msg string, fields ...ports.Field) {
	z.logger.Error(msg, convertFields(fields)...)
}

// Warn logs a warning message
func (z *ZapLoggerAdapter) Warn(msg string, fields ...ports.Field) {
	z.logger.Warn(msg, convertFields(fields)...)
}

// Debug logs a debug message
func (z *ZapLoggerAdapter) Debug(msg string, fields ...ports.Field) {
	z.logger.Debug(msg, convertFields(fields)...)
}

// convertFields converts our Field type to zap.Field
func convertFields(fields []ports.Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, f := range fields {
		zapFields[i] = zap.Any(f.Key, f.Value)
	}
	return zapFields
}
