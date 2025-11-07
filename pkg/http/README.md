# HTTP Client Package

A production-ready HTTP client package with automatic token management, caching, and retry logic for service-to-service communication.

## Features

- **Automatic Token Management**: Fetches and caches service tokens with expiration handling
- **Token Refresh on 401**: Automatically refreshes tokens when receiving 401 Unauthorized responses
- **Retry Logic**: Configurable retry with exponential backoff for failed requests
- **Thread-Safe**: Safe for concurrent use with proper locking
- **Context Support**: Full context.Context support for cancellation and timeouts
- **Request/Response Hooks**: Extensible hooks for custom request/response processing
- **JSON Helpers**: Convenient methods for JSON requests and responses

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "time"
    
    "github.com/milan604/core-lab/pkg/http"
    "github.com/milan604/core-lab/pkg/logger"
)

func main() {
    // Create a token provider (OAuth2 client credentials)
    provider := http.NewOAuth2ClientCredentialsProvider(
        "https://auth.example.com/oauth/token",
        "client-id",
        "client-secret",
        "read write",
    )
    
    // Create HTTP client with token management
    client := http.NewClient(
        http.WithTokenProvider(provider, 30*time.Second), // refresh 30s before expiration
        http.WithLogger(logger.MustNewDefaultLogger()),
        http.WithRetry(3, 100*time.Millisecond), // 3 attempts with exponential backoff
    )
    
    ctx := context.Background()
    
    // Make a request - token is automatically injected
    // Each service call uses its own full URL
    resp, err := client.Get(ctx, "https://api.example.com/api/v1/users")
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    
    // Or use JSON helper
    var users []User
    err = client.GetJSON(ctx, "https://api.example.com/api/v1/users", &users)
    if err != nil {
        panic(err)
    }
}
```

### Token Providers

The package includes several token provider implementations:

#### OAuth2 Client Credentials

```go
provider := http.NewOAuth2ClientCredentialsProvider(
    "https://auth.example.com/oauth/token",
    "client-id",
    "client-secret",
    "read write", // optional scope
)
```

#### Static Token

```go
provider := http.NewStaticTokenProvider("your-static-token")
```

#### Custom Provider

```go
provider := http.NewCustomTokenProvider(func(ctx context.Context) (string, time.Time, error) {
    // Your custom token fetching logic
    token, expiresIn, err := fetchTokenFromYourService(ctx)
    expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
    return token, expiresAt, err
})
```

### Client Options

```go
client := http.NewClient(
    // Token management
    http.WithTokenProvider(provider, 30*time.Second),
    
    // HTTP client configuration
    http.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
    }),
    
    // Retry configuration
    http.WithRetry(3, 100*time.Millisecond), // max attempts, initial delay
    
    // Logging
    http.WithLogger(logger),
    
    // Request hooks (run before each request)
    http.WithRequestHook(func(req *http.Request) error {
        req.Header.Set("X-Custom-Header", "value")
        return nil
    }),
    
    // Response hooks (run after each response)
    http.WithResponseHook(func(resp *http.Response) error {
        // Log response status, etc.
        return nil
    }),
)
```

## How It Works

### Token Caching

1. When a request is made, the client checks if a valid token exists in cache
2. If the token is missing or about to expire (within refresh buffer), a new token is fetched
3. Tokens are cached until expiration, reducing unnecessary token requests
4. The cache is thread-safe and handles concurrent requests efficiently

### Automatic 401 Retry

1. When a request receives a 401 Unauthorized response:
   - The cached token is invalidated
   - A new token is fetched
   - The request is automatically retried with the new token
2. This happens transparently - you don't need to handle 401 errors manually

### Retry Logic

- Failed requests are automatically retried with exponential backoff
- Network errors and 401 responses trigger retries
- Maximum retry attempts are configurable
- Context cancellation is respected during retries

## API Reference

### Client Methods

#### `Do(ctx, req)`
Executes an HTTP request with automatic token injection and retry logic.

#### `Get(ctx, url)`
Performs a GET request.

#### `Post(ctx, url, body)`
Performs a POST request with JSON body.

#### `Put(ctx, url, body)`
Performs a PUT request with JSON body.

#### `Patch(ctx, url, body)`
Performs a PATCH request with JSON body.

#### `Delete(ctx, url)`
Performs a DELETE request.

#### `GetJSON(ctx, url, v)`
Performs a GET request and unmarshals JSON response into `v`.

#### `PostJSON(ctx, url, body, v)`
Performs a POST request with JSON body and unmarshals JSON response into `v`.

#### `DoJSON(ctx, req, v)`
Executes a request and unmarshals JSON response into `v`.

### Token Cache Methods

#### `GetToken(ctx)`
Retrieves a valid token, fetching a new one if needed.

#### `Invalidate()`
Clears the cached token, forcing a refresh on next `GetToken` call.

#### `IsValid()`
Checks if the current cached token is still valid.

## Examples

### Making API Calls

```go
// Simple GET request - each service has its own URL
resp, err := client.Get(ctx, "https://api.example.com/api/v1/users")
if err != nil {
    return err
}
defer resp.Body.Close()

// POST with JSON body
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

req := CreateUserRequest{
    Name:  "John Doe",
    Email: "john@example.com",
}

resp, err := client.Post(ctx, "https://api.example.com/api/v1/users", req)
if err != nil {
    return err
}
defer resp.Body.Close()

// GET with JSON response
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

var users []User
err = client.GetJSON(ctx, "https://api.example.com/api/v1/users", &users)
if err != nil {
    return err
}
```

### Custom Request Headers

```go
client := http.NewClient(
    http.WithTokenProvider(provider, 30*time.Second),
    http.WithRequestHook(func(req *http.Request) error {
        req.Header.Set("X-Request-ID", generateRequestID())
        req.Header.Set("X-Client-Version", "1.0.0")
        return nil
    }),
)
```

### Error Handling

```go
resp, err := client.Get(ctx, "https://api.example.com/api/v1/users")
if err != nil {
    // Handle error (network error, token fetch failure, etc.)
    return fmt.Errorf("request failed: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
    // Handle non-2xx responses
    bodyBytes, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
}
```

### Using the Same Client for Multiple Services

The HTTP client is designed to be reusable across different services. Each service call uses its own full URL:

```go
client := http.NewClient(
    http.WithTokenProvider(provider, 30*time.Second),
    http.WithLogger(logger),
)

// Call different services with their own URLs
users, _ := client.GetJSON(ctx, "https://user-service.example.com/api/v1/users", &users)
orders, _ := client.GetJSON(ctx, "https://order-service.example.com/api/v1/orders", &orders)
products, _ := client.GetJSON(ctx, "https://product-service.example.com/api/v1/products", &products)
```

## Best Practices

1. **Token Refresh Buffer**: Set a reasonable refresh buffer (e.g., 30 seconds) to avoid token expiration during requests
2. **Retry Configuration**: Configure retry attempts based on your service's reliability requirements
3. **Context Usage**: Always use context.Context for cancellation and timeouts
4. **Error Handling**: Check response status codes and handle errors appropriately
5. **Resource Cleanup**: Always close response bodies to avoid resource leaks
6. **Service URLs**: Each service call should use its own full URL - the client doesn't maintain a base URL

## Thread Safety

The HTTP client and token cache are fully thread-safe and can be used concurrently from multiple goroutines.

## Integration with Other Packages

This package integrates seamlessly with other core-lab packages:

- **logger**: Use `WithLogger()` to enable request/response logging
- **config**: Configure token providers using the config package
- **errors**: Return structured errors compatible with the errors package

