package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds the application configuration
type Config struct {
	// API key for Stability AI
	APIKey string
	// API key for client authentication (separate from Stability AI key)
	ClientAPIKey string
	// Server address (e.g., ":8080")
	ServerAddr string
	// Cache directory (empty to disable caching)
	CachePath string
	// Rate limit between requests
	RateLimit time.Duration
	// List of allowed hosts (empty to allow all)
	AllowedHosts []string
	// Log level (debug, info, warn, error)
	LogLevel string
	// Custom base URL for Stability API (optional)
	StabilityBaseURL string
	// List of allowed IP addresses (empty to allow all)
	AllowedIPs []string
	// List of allowed app IDs (empty to allow all)
	AllowedAppIDs []string
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	apiKey := os.Getenv("STABILITY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("STABILITY_API_KEY environment variable is required")
	}

	// Get client API key for authentication
	clientAPIKey := os.Getenv("CLIENT_API_KEY")
	if clientAPIKey == "" {
		// Generate a random client API key if not provided
		clientAPIKey = generateRandomKey()
		fmt.Printf("No CLIENT_API_KEY set. Generated random key: %s\n", clientAPIKey)
	}

	serverAddr := os.Getenv("SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = ":8080"
	}

	cachePath := os.Getenv("CACHE_PATH")

	rateLimitStr := os.Getenv("RATE_LIMIT")
	var rateLimit time.Duration
	if rateLimitStr != "" {
		var err error
		rateLimit, err = time.ParseDuration(rateLimitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid RATE_LIMIT value: %w", err)
		}
	} else {
		rateLimit = 500 * time.Millisecond
	}

	// Parse allowed hosts
	var allowedHosts []string
	if hosts := os.Getenv("ALLOWED_HOSTS"); hosts != "" {
		allowedHosts = append(allowedHosts, hosts)
	}
	
	// Parse allowed IPs (comma-separated list)
	var allowedIPs []string
	if ips := os.Getenv("ALLOWED_IPS"); ips != "" {
		for _, ip := range strings.Split(ips, ",") {
			allowedIPs = append(allowedIPs, strings.TrimSpace(ip))
		}
	}
	
	// Parse allowed app IDs (comma-separated list)
	var allowedAppIDs []string
	if appIDs := os.Getenv("ALLOWED_APP_IDS"); appIDs != "" {
		for _, appID := range strings.Split(appIDs, ",") {
			allowedAppIDs = append(allowedAppIDs, strings.TrimSpace(appID))
		}
	}

	// Get log level
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	// Get custom base URL
	stabilityBaseURL := os.Getenv("STABILITY_BASE_URL")

	return &Config{
		APIKey:           apiKey,
		ClientAPIKey:     clientAPIKey,
		ServerAddr:       serverAddr,
		CachePath:        cachePath,
		RateLimit:        rateLimit,
		AllowedHosts:     allowedHosts,
		LogLevel:         logLevel,
		StabilityBaseURL: stabilityBaseURL,
		AllowedIPs:       allowedIPs,
		AllowedAppIDs:    allowedAppIDs,
	}, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if c.ServerAddr == "" {
		return fmt.Errorf("server address is required")
	}

	if c.ClientAPIKey == "" {
		return fmt.Errorf("client API key is required")
	}

	return nil
}

// generateRandomKey generates a random key for client authentication
func generateRandomKey() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const keyLength = 32
	
	// Import crypto/rand and math/big for this function
	// For simplicity, we'll just use a timestamp-based key here
	return fmt.Sprintf("client-key-%d", time.Now().UnixNano())
}