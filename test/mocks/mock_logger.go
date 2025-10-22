package mocks

import "github.com/kevin07696/payment-service/internal/domain/ports"

// MockLogger is a mock implementation of Logger for testing
type MockLogger struct {
	InfoCalls  []LogCall
	ErrorCalls []LogCall
	WarnCalls  []LogCall
	DebugCalls []LogCall
}

// LogCall represents a captured log call
type LogCall struct {
	Message string
	Fields  []ports.Field
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *MockLogger {
	return &MockLogger{
		InfoCalls:  []LogCall{},
		ErrorCalls: []LogCall{},
		WarnCalls:  []LogCall{},
		DebugCalls: []LogCall{},
	}
}

// Info logs an info message
func (m *MockLogger) Info(msg string, fields ...ports.Field) {
	m.InfoCalls = append(m.InfoCalls, LogCall{Message: msg, Fields: fields})
}

// Error logs an error message
func (m *MockLogger) Error(msg string, fields ...ports.Field) {
	m.ErrorCalls = append(m.ErrorCalls, LogCall{Message: msg, Fields: fields})
}

// Warn logs a warning message
func (m *MockLogger) Warn(msg string, fields ...ports.Field) {
	m.WarnCalls = append(m.WarnCalls, LogCall{Message: msg, Fields: fields})
}

// Debug logs a debug message
func (m *MockLogger) Debug(msg string, fields ...ports.Field) {
	m.DebugCalls = append(m.DebugCalls, LogCall{Message: msg, Fields: fields})
}

// Reset clears all captured calls
func (m *MockLogger) Reset() {
	m.InfoCalls = []LogCall{}
	m.ErrorCalls = []LogCall{}
	m.WarnCalls = []LogCall{}
	m.DebugCalls = []LogCall{}
}
