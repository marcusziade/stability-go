package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/marcusziade/stability-go/client"
)

// ProxyServer is a simple proxy server for the Stability AI API
type ProxyServer struct {
	apiKey       string
	client       *client.Client
	allowedHosts []string
	cachePath    string
	rateLimit    time.Duration
}

// NewProxyServer creates a new proxy server
func NewProxyServer(apiKey string, cachePath string, rateLimit time.Duration, allowedHosts []string) *ProxyServer {
	return &ProxyServer{
		apiKey:       apiKey,
		client:       client.NewClient(apiKey),
		allowedHosts: allowedHosts,
		cachePath:    cachePath,
		rateLimit:    rateLimit,
	}
}

// Start starts the proxy server
func (s *ProxyServer) Start(addr string) error {
	// Create cache directory if it doesn't exist
	if s.cachePath != "" {
		if err := os.MkdirAll(s.cachePath, 0755); err != nil {
			return fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	// Create rate limiter using a channel and goroutine
	if s.rateLimit > 0 {
		log.Printf("Rate limiting enabled: %v between requests", s.rateLimit)
		limiter := make(chan struct{}, 1)
		// Initialize with a token
		limiter <- struct{}{}

		// Start rate limiter goroutine
		go func() {
			for {
				time.Sleep(s.rateLimit)
				select {
				case limiter <- struct{}{}:
					// Added a token
				default:
					// Channel is full, do nothing
				}
			}
		}()

		// Wrap the handler with rate limiting
		http.HandleFunc("/v1/generation/upscale", func(w http.ResponseWriter, r *http.Request) {
			// Wait for a token
			<-limiter
			s.handleUpscale(w, r)
		})
	} else {
		http.HandleFunc("/v1/generation/upscale", s.handleUpscale)
	}

	// Add health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Starting proxy server on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleUpscale handles upscale requests
func (s *ProxyServer) handleUpscale(w http.ResponseWriter, r *http.Request) {
	// Check if request is from allowed host
	if len(s.allowedHosts) > 0 {
		host := r.Header.Get("X-Forwarded-For")
		if host == "" {
			host = r.RemoteAddr
		}

		allowed := false
		for _, allowedHost := range s.allowedHosts {
			if host == allowedHost {
				allowed = true
				break
			}
		}

		if !allowed {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	}

	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get form values
	engine := r.FormValue("engine")
	if engine == "" {
		http.Error(w, "Engine is required", http.StatusBadRequest)
		return
	}

	// Get image file
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Failed to get image file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read image data
	imageData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read image data", http.StatusInternalServerError)
		return
	}

	// Check cache if enabled
	if s.cachePath != "" {
		cacheKey := fmt.Sprintf("%x-%s-%s", imageData, engine, r.Form.Encode())
		cachePath := filepath.Join(s.cachePath, cacheKey+".json")

		// Check if cache file exists
		if _, err := os.Stat(cachePath); err == nil {
			log.Printf("Cache hit for %s", cacheKey)

			// Read cache file
			cacheData, err := os.ReadFile(cachePath)
			if err == nil {
				// Set content type
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				w.Write(cacheData)
				return
			}
		}
	}

	// Create upscale request
	var model client.UpscaleModel
	switch engine {
	case "esrgan-v1-x2plus":
		model = client.UpscaleModelESRGAN
	case "stable-diffusion-x4-latent-upscaler":
		model = client.UpscaleModelStable
	case "realesrgan-16x":
		model = client.UpscaleModelRealESR
	default:
		http.Error(w, "Invalid engine", http.StatusBadRequest)
		return
	}

	// Create upscale request
	request := client.UpscaleRequest{
		Image:         imageData,
		Filename:      header.Filename,
		Model:         model,
		Factor:        formValueInt(r, "factor", 0),
		Width:         formValueInt(r, "width", 0),
		Height:        formValueInt(r, "height", 0),
		EnhanceDetail: formValueBool(r, "enhance_detail", false),
	}

	// Forward request to Stability AI
	log.Printf("Forwarding upscale request to Stability AI (engine: %s)", engine)
	ctx := r.Context()
	response, err := s.client.Upscale(ctx, request)
	if err != nil {
		log.Printf("Error from Stability AI: %v", err)
		http.Error(w, fmt.Sprintf("Error from Stability AI: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert response to JSON
	responseData, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	// Cache response if enabled
	if s.cachePath != "" {
		cacheKey := fmt.Sprintf("%x-%s-%s", imageData, engine, r.Form.Encode())
		cachePath := filepath.Join(s.cachePath, cacheKey+".json")
		if err := os.WriteFile(cachePath, responseData, 0644); err != nil {
			log.Printf("Failed to write cache file: %v", err)
		} else {
			log.Printf("Cached response at %s", cachePath)
		}
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}

// Helper functions
func formValueInt(r *http.Request, key string, defaultValue int) int {
	value := r.FormValue(key)
	if value == "" {
		return defaultValue
	}

	var intValue int
	fmt.Sscanf(value, "%d", &intValue)
	return intValue
}

func formValueBool(r *http.Request, key string, defaultValue bool) bool {
	value := r.FormValue(key)
	if value == "" {
		return defaultValue
	}

	return value == "true" || value == "1" || value == "yes"
}

func main() {
	// Parse command line flags
	apiKey := flag.String("api-key", os.Getenv("STABILITY_API_KEY"), "Stability API key")
	addr := flag.String("addr", ":8080", "Address to listen on")
	cachePath := flag.String("cache", "", "Cache directory (empty to disable)")
	rateLimitStr := flag.String("rate-limit", "500ms", "Rate limit between requests (empty to disable)")
	allowedHosts := flag.String("allowed-hosts", "", "Comma-separated list of allowed hosts (empty to allow all)")
	flag.Parse()

	// Validate inputs
	if *apiKey == "" {
		fmt.Println("API key is required. Provide it with -api-key flag or STABILITY_API_KEY environment variable.")
		os.Exit(1)
	}

	// Parse rate limit
	var rateLimit time.Duration
	if *rateLimitStr != "" {
		var err error
		rateLimit, err = time.ParseDuration(*rateLimitStr)
		if err != nil {
			fmt.Printf("Invalid rate limit: %v\n", err)
			os.Exit(1)
		}
	}

	// Parse allowed hosts
	var allowedHostsList []string
	if *allowedHosts != "" {
		allowedHostsList = filepath.SplitList(*allowedHosts)
	}

	// Create and start proxy server
	server := NewProxyServer(*apiKey, *cachePath, rateLimit, allowedHostsList)
	if err := server.Start(*addr); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
		os.Exit(1)
	}
}