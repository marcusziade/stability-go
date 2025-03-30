package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/marcusziade/stability-go/internal/logger"
)

// Middleware defines an HTTP middleware function
type Middleware func(http.Handler) http.Handler

// Chain combines multiple middleware into a single middleware
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// WithLogger adds request logging to the middleware chain
func WithLogger(logger *logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Create a response writer that captures the status code
			crw := &captureResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Add request ID to context
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}
			ctx := context.WithValue(r.Context(), contextKeyRequestID, requestID)
			
			// Log the request
			logger.Info("Request: %s %s [%s]", r.Method, r.URL.Path, requestID)
			
			// Call the next handler with the updated context
			next.ServeHTTP(crw, r.WithContext(ctx))
			
			// Log the response
			duration := time.Since(start)
			logger.Info("Response: %s %s [%s] %d %v", 
				r.Method, r.URL.Path, requestID, crw.statusCode, duration)
		})
	}
}

// WithRateLimit adds rate limiting to the middleware chain
func WithRateLimit(limit time.Duration) Middleware {
	// Create a channel to act as a token bucket
	bucket := make(chan struct{}, 1)
	
	// Start a goroutine to add tokens to the bucket at the specified rate
	go func() {
		ticker := time.NewTicker(limit)
		defer ticker.Stop()
		
		// Add initial token
		bucket <- struct{}{}
		
		for range ticker.C {
			select {
			case bucket <- struct{}{}:
				// Added token
			default:
				// Bucket is full, do nothing
			}
		}
	}()
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Wait for a token
			<-bucket
			
			// Process the request
			next.ServeHTTP(w, r)
		})
	}
}

// WithCORS adds CORS headers to the middleware chain
func WithCORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Check if the origin is allowed
			allowed := len(allowedOrigins) == 0 // If no origins specified, allow all
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}
			
			if allowed {
				// Set CORS headers
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
			}
			
			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			// Process the request
			next.ServeHTTP(w, r)
		})
	}
}

// WithAuth adds API key authentication to the middleware chain
func WithAuth(apiKey string, excludePaths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the path is excluded from authentication
			for _, path := range excludePaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}
			
			// Get the API key from the request
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, "Unauthorized: API key is missing", http.StatusUnauthorized)
				return
			}
			
			// Check if the API key is valid
			receivedKey := strings.TrimPrefix(auth, "Bearer ")
			if receivedKey != apiKey {
				http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
				return
			}
			
			// Process the request
			next.ServeHTTP(w, r)
		})
	}
}

// WithIPFilter restricts access to allowed IP addresses
func WithIPFilter(allowedIPs []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip check if no IPs are specified (allow all)
			if len(allowedIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			
			// Get client IP
			clientIP := getClientIP(r)
			
			// Check if the IP is allowed
			allowed := false
			for _, ip := range allowedIPs {
				if ip == clientIP {
					allowed = true
					break
				}
			}
			
			if !allowed {
				http.Error(w, "Forbidden: IP address not allowed", http.StatusForbidden)
				return
			}
			
			// Process the request
			next.ServeHTTP(w, r)
		})
	}
}

// WithAppIDAuth validates the App-ID header
func WithAppIDAuth(allowedAppIDs []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip check if no app IDs are specified (allow all)
			if len(allowedAppIDs) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			
			// Get App ID from header
			appID := r.Header.Get("X-App-ID")
			if appID == "" {
				http.Error(w, "Forbidden: App ID is required", http.StatusForbidden)
				return
			}
			
			// Check if the App ID is allowed
			allowed := false
			for _, id := range allowedAppIDs {
				if id == appID {
					allowed = true
					break
				}
			}
			
			if !allowed {
				http.Error(w, "Forbidden: Invalid App ID", http.StatusForbidden)
				return
			}
			
			// Process the request
			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions and types

// captureResponseWriter captures the status code of the response
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (crw *captureResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}

// generateRequestID generates a random request ID
func generateRequestID() string {
	// Simple implementation: use current timestamp
	return time.Now().Format("20060102.150405.000000")
}

// getClientIP extracts the client's IP address from the request
func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header first (for proxies)
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For can contain multiple IPs - use the first one
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	
	// Check for X-Real-IP header (set by some proxies)
	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}
	
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If there's an error, just return the RemoteAddr as is
		return r.RemoteAddr
	}
	
	return ip
}

// Context keys
type contextKey string

const (
	contextKeyRequestID contextKey = "requestID"
)