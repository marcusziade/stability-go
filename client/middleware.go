package client

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// RateLimitMiddleware is a middleware for handling rate limiting
type RateLimitMiddleware struct {
	mutex       sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(minInterval time.Duration) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		minInterval: minInterval,
	}
}

// RoundTrip implements the http.RoundTripper interface
func (m *RateLimitMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mutex.Lock()

	// Calculate time since last request
	elapsed := time.Since(m.lastRequest)

	// If we need to wait, do so
	if elapsed < m.minInterval {
		time.Sleep(m.minInterval - elapsed)
	}

	// Update last request time
	m.lastRequest = time.Now()
	m.mutex.Unlock()

	// Continue with the request
	return http.DefaultTransport.RoundTrip(req)
}

// RetryMiddleware is a middleware for handling retries
type RetryMiddleware struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	transport  http.RoundTripper
}

// NewRetryMiddleware creates a new retry middleware
func NewRetryMiddleware(maxRetries int, baseDelay, maxDelay time.Duration, transport http.RoundTripper) *RetryMiddleware {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &RetryMiddleware{
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
		transport:  transport,
	}
}

// RoundTrip implements the http.RoundTripper interface
func (m *RetryMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	// Make a copy of the request body
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, err = readAndReplaceBody(req)
		if err != nil {
			return nil, err
		}
	}

	// Try the request up to maxRetries times
	for attempt := 0; attempt <= m.maxRetries; attempt++ {
		// If this is a retry, restore the request body
		if attempt > 0 && bodyBytes != nil {
			req.Body = createReadCloser(bodyBytes)
		}

		// Make the request
		resp, err = m.transport.RoundTrip(req)

		// If there's no error and response is successful, return it
		if err == nil && (resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests) {
			return resp, nil
		}

		// If this was the last attempt, return the error
		if attempt == m.maxRetries {
			if err != nil {
				return nil, err
			}
			return resp, nil
		}

		// Close the response body if we're going to retry
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		// Calculate delay using exponential backoff (2^attempt * baseDelay)
		delay := m.baseDelay * (1 << uint(attempt))
		if delay > m.maxDelay {
			delay = m.maxDelay
		}

		// Add jitter (±20%)
		delay = addJitter(delay, 0.2)

		// Create a new context for the sleep
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
			// Continue to the next attempt
		}
	}

	// We should never reach here because of the return in the last attempt
	return resp, err
}

// ProxyMiddleware is a middleware that proxies requests through a different URL
type ProxyMiddleware struct {
	proxyURL  string
	transport http.RoundTripper
}

// NewProxyMiddleware creates a new proxy middleware
func NewProxyMiddleware(proxyURL string, transport http.RoundTripper) *ProxyMiddleware {
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &ProxyMiddleware{
		proxyURL:  proxyURL,
		transport: transport,
	}
}

// RoundTrip implements the http.RoundTripper interface
func (m *ProxyMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the host with the proxy host
	originalURL := req.URL.String()
	req.URL.Scheme = "https"
	req.URL.Host = m.proxyURL
	req.URL.Path = "/proxy" + req.URL.Path

	// Add the original URL as a query parameter
	q := req.URL.Query()
	q.Add("target", originalURL)
	req.URL.RawQuery = q.Encode()

	// Continue with the request
	return m.transport.RoundTrip(req)
}

// MiddlewareClient is a client that uses middleware
type MiddlewareClient struct {
	*Client
	middleware []http.RoundTripper
}

// NewMiddlewareClient creates a new middleware client
func NewMiddlewareClient(apiKey string, middleware ...http.RoundTripper) *MiddlewareClient {
	return &MiddlewareClient{
		Client:     NewClient(apiKey),
		middleware: middleware,
	}
}

// Upscale upscales an image using the middleware chain
func (c *MiddlewareClient) Upscale(ctx context.Context, request UpscaleRequest) (*UpscaleResponse, error) {
	// Create a chain of middleware
	var transport http.RoundTripper = http.DefaultTransport
	for i := len(c.middleware) - 1; i >= 0; i-- {
		transport = c.middleware[i]
	}

	// Replace the HTTPClient's transport with our middleware chain
	c.Client.HTTPClient.Transport = transport

	// Call the regular Upscale method
	return c.Client.Upscale(ctx, request)
}

// PollCreativeResult polls for the result of a creative upscale job using the middleware chain
func (c *MiddlewareClient) PollCreativeResult(ctx context.Context, id string) (*UpscaleResponse, bool, error) {
	// Create a chain of middleware
	var transport http.RoundTripper = http.DefaultTransport
	for i := len(c.middleware) - 1; i >= 0; i-- {
		transport = c.middleware[i]
	}

	// Replace the HTTPClient's transport with our middleware chain
	c.Client.HTTPClient.Transport = transport

	// Call the regular PollCreativeResult method
	return c.Client.PollCreativeResult(ctx, id)
}

// GetClient returns the underlying Client
func (c *MiddlewareClient) GetClient() *Client {
	// Create a chain of middleware
	var transport http.RoundTripper = http.DefaultTransport
	for i := len(c.middleware) - 1; i >= 0; i-- {
		transport = c.middleware[i]
	}

	// Replace the HTTPClient's transport with our middleware chain
	c.Client.HTTPClient.Transport = transport

	return c.Client
}

// Helper functions for middlewares

// readAndReplaceBody reads the request body and replaces it
func readAndReplaceBody(req *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	// Close the original body
	req.Body.Close()

	// Replace the body
	req.Body = createReadCloser(bodyBytes)

	return bodyBytes, nil
}

// createReadCloser creates a new ReadCloser from a byte slice
func createReadCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

// addJitter adds random jitter to a duration
func addJitter(d time.Duration, factor float64) time.Duration {
	jitter := float64(d) * (1 - factor + 2*factor*rand.Float64())
	return time.Duration(jitter)
}

