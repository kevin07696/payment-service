package encoding

import (
	"bytes"
	"encoding/json"
	"sync"
)

var (
	// BufferPool pools bytes.Buffer for JSON encoding
	// Used for every API response serialization, webhook payload, logging, etc.
	BufferPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
		},
	}

	// EncoderPool pools JSON encoders
	// Reusing encoders is more efficient than creating new ones
	EncoderPool = sync.Pool{
		New: func() interface{} {
			return json.NewEncoder(new(bytes.Buffer))
		},
	}
)

// GetBuffer retrieves a bytes.Buffer from the pool
func GetBuffer() *bytes.Buffer {
	buf := BufferPool.Get().(*bytes.Buffer)
	buf.Reset() // Ensure buffer is empty
	return buf
}

// PutBuffer returns a bytes.Buffer to the pool
func PutBuffer(buf *bytes.Buffer) {
	// Don't pool buffers that grew too large (>64KB)
	// This prevents memory bloat from outlier large responses
	if buf.Cap() > 64*1024 {
		return
	}
	buf.Reset()
	BufferPool.Put(buf)
}

// EncodeJSON encodes v to JSON using a pooled buffer
// Returns the JSON bytes and any encoding error
func EncodeJSON(v interface{}) ([]byte, error) {
	buf := GetBuffer()
	defer PutBuffer(buf)

	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// Copy the buffer contents since we're returning the buffer to the pool
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// EncodeJSONToBuffer encodes v to JSON directly into the provided buffer
// This is useful when you already have a buffer (e.g., HTTP response writer)
func EncodeJSONToBuffer(buf *bytes.Buffer, v interface{}) error {
	encoder := json.NewEncoder(buf)
	return encoder.Encode(v)
}
