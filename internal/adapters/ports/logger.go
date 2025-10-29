package ports

// Logger is a minimal logging interface for adapters
// This allows for easy mocking and different logger implementations
type Logger interface {
	Info(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Debug(msg string, fields ...Field)
}

// Field represents a structured logging field
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, val string) Field {
	return Field{Key: key, Value: val}
}

// Int creates an integer field
func Int(key string, val int) Field {
	return Field{Key: key, Value: val}
}

// Err creates an error field
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}
