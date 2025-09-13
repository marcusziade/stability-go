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

			crw := &captureResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = generateRequestID()
			}
			ctx := context.WithValue(r.Context(), contextKeyRequestID, requestID)

			logger.Info("Request: %s %s [%s]", r.Method, r.URL.Path, requestID)

			next.ServeHTTP(crw, r.WithContext(ctx))

			duration := time.Since(start)
			logger.Info("Response: %s %s [%s] %d %v",
				r.Method, r.URL.Path, requestID, crw.statusCode, duration)
		})
	}
}

// WithRateLimit adds rate limiting to the middleware chain
func WithRateLimit(limit time.Duration) Middleware {
	bucket := make(chan struct{}, 1)

	go func() {
		ticker := time.NewTicker(limit)
		defer ticker.Stop()

		bucket <- struct{}{}

		for range ticker.C {
			select {
			case bucket <- struct{}{}:
			default:
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-bucket

			next.ServeHTTP(w, r)
		})
	}
}

// WithCORS adds CORS headers to the middleware chain
func WithCORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := len(allowedOrigins) == 0
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WithAuth adds API key authentication to the middleware chain
func WithAuth(apiKey string, excludePaths []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, path := range excludePaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, "Unauthorized: API key is missing", http.StatusUnauthorized)
				return
			}

			receivedKey := strings.TrimPrefix(auth, "Bearer ")
			if receivedKey != apiKey {
				http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// WithIPFilter restricts access to allowed IP addresses
func WithIPFilter(allowedIPs []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := getClientIP(r)

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

			next.ServeHTTP(w, r)
		})
	}
}

// WithAppIDAuth validates the App-ID header
func WithAppIDAuth(allowedAppIDs []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedAppIDs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			if r.URL.Path == "/" || r.URL.Path == "/health" || r.URL.Path == "/api/docs" {
				next.ServeHTTP(w, r)
				return
			}

			appID := r.Header.Get("X-App-ID")
			if appID == "" {
				http.Error(w, "Forbidden: App ID is required", http.StatusForbidden)
				return
			}

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
	return time.Now().Format("20060102.150405.000000")
}

// getClientIP extracts the client's IP address from the request
func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

// Context keys
type contextKey string

const (
	contextKeyRequestID contextKey = "requestID"
)
