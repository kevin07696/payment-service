package ports

import (
	"context"
	"io"
	"net/http"
)

// HTTPClient defines the interface for making HTTP requests
// This allows us to mock HTTP calls in tests and swap implementations
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPRequest represents an HTTP request to be made
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    io.Reader
	Context context.Context
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int
	Body       []byte
	Headers    map[string][]string
}
