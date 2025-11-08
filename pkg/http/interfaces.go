package http

import (
	"context"
	"net/http"
)

// HTTPClient defines the interface for HTTP client operations.
// This interface allows for mocking and alternative implementations.
type HTTPClient interface {
	// Do executes an HTTP request with automatic token injection and retry logic.
	Do(ctx context.Context, req *http.Request) (*http.Response, error)

	// Get performs a GET request.
	Get(ctx context.Context, url string) (*http.Response, error)

	// Post performs a POST request with JSON body.
	Post(ctx context.Context, url string, body interface{}) (*http.Response, error)

	// Put performs a PUT request with JSON body.
	Put(ctx context.Context, url string, body interface{}) (*http.Response, error)

	// Patch performs a PATCH request with JSON body.
	Patch(ctx context.Context, url string, body interface{}) (*http.Response, error)

	// Delete performs a DELETE request.
	Delete(ctx context.Context, url string) (*http.Response, error)

	// DoJSON performs a request and unmarshals the JSON response.
	DoJSON(ctx context.Context, req *http.Request, v interface{}) error

	// GetJSON performs a GET request and unmarshals the JSON response.
	GetJSON(ctx context.Context, url string, v interface{}) error

	// PostJSON performs a POST request with JSON body and unmarshals the JSON response.
	PostJSON(ctx context.Context, url string, body interface{}, v interface{}) error
}

// Ensure Client implements HTTPClient interface.
var _ HTTPClient = (*Client)(nil)
