package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	// API key for Stability AI
	APIKey string
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
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	apiKey := os.Getenv("STABILITY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("STABILITY_API_KEY environment variable is required")
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

	// Get log level
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	// Get custom base URL
	stabilityBaseURL := os.Getenv("STABILITY_BASE_URL")

	return &Config{
		APIKey:           apiKey,
		ServerAddr:       serverAddr,
		CachePath:        cachePath,
		RateLimit:        rateLimit,
		AllowedHosts:     allowedHosts,
		LogLevel:         logLevel,
		StabilityBaseURL: stabilityBaseURL,
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

	return nil
}