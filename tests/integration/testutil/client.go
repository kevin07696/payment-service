package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client wraps HTTP client for API calls
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Headers    map[string]string // Custom headers to add to all requests
}

// NewClient creates a new test API client for ConnectRPC
func NewClient(baseURL string) *Client {
	// Use standard HTTP/1.1 transport for Connect protocol
	// The h2c server supports both HTTP/1.1 and HTTP/2
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Headers: make(map[string]string),
	}
}

// SetHeader sets a custom header for all subsequent requests
func (c *Client) SetHeader(key, value string) {
	c.Headers[key] = value
}

// ClearHeaders removes all custom headers
func (c *Client) ClearHeaders() {
	c.Headers = make(map[string]string)
}

// applyHeaders adds all custom headers to the request
func (c *Client) applyHeaders(req *http.Request) {
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}
}

// Do performs an HTTP request
func (c *Client) Do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply custom headers
	c.applyHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

// DoForm performs an HTTP request with form data (application/x-www-form-urlencoded)
func (c *Client) DoForm(method, path string, formData url.Values) (*http.Response, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Apply custom headers
	c.applyHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

// DoConnectRPC performs a ConnectRPC call using HTTP/JSON protocol
// serviceName: e.g., "payment.v1.PaymentService"
// method: e.g., "Sale"
// body: request message as map or struct
func (c *Client) DoConnectRPC(serviceName, method string, body interface{}) (*http.Response, error) {
	// ConnectRPC path format: /package.service.ServiceName/Method
	path := fmt.Sprintf("/%s/%s", serviceName, method)

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest("POST", c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// ConnectRPC HTTP/JSON protocol headers
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Use Connect protocol (works with HTTP/1.1 and HTTP/2)
	req.Header.Set("Connect-Protocol-Version", "1")

	// Apply custom headers (e.g., Authorization)
	c.applyHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

// DecodeResponse decodes JSON response body
func DecodeResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
