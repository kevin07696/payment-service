package mocks

import (
	"bytes"
	"io"
	"net/http"
)

// MockHTTPClient is a mock implementation of HTTPClient for testing
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
	Calls  []*http.Request
}

// NewMockHTTPClient creates a new mock HTTP client
func NewMockHTTPClient(doFunc func(req *http.Request) (*http.Response, error)) *MockHTTPClient {
	return &MockHTTPClient{
		DoFunc: doFunc,
		Calls:  []*http.Request{},
	}
}

// Do executes the mock function and captures the call
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.Calls = append(m.Calls, req)
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	// Default success response
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{"status":"ok"}`)),
		Header:     make(http.Header),
	}, nil
}

// Reset clears captured calls
func (m *MockHTTPClient) Reset() {
	m.Calls = []*http.Request{}
}
