package ports

import "net/http"

// HTTPClient is a minimal HTTP client interface for making requests
// This allows for easy mocking and testing of adapters
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
