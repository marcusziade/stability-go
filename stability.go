package stability

import (
	"net/http"
	"time"

	"github.com/marcusziade/stability-go/client"
)

// New creates a new Stability API client with the given API key
func New(apiKey string) *client.Client {
	return client.NewClient(apiKey)
}

// NewWithMiddleware creates a new middleware-enabled Stability API client with the given API key
func NewWithMiddleware(apiKey string, middleware ...http.RoundTripper) *client.MiddlewareClient {
	return client.NewMiddlewareClient(apiKey, middleware...)
}

// WithRateLimit creates a new rate limit middleware with the given minimum interval between requests
func WithRateLimit(minInterval time.Duration) http.RoundTripper {
	return client.NewRateLimitMiddleware(minInterval)
}

// WithRetry creates a new retry middleware with the given parameters
func WithRetry(maxRetries int, baseDelay, maxDelay time.Duration) http.RoundTripper {
	return client.NewRetryMiddleware(maxRetries, baseDelay, maxDelay, nil)
}

// WithProxy creates a new proxy middleware with the given proxy URL
func WithProxy(proxyURL string) http.RoundTripper {
	return client.NewProxyMiddleware(proxyURL, nil)
}