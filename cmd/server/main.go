package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/marcusziade/stability-go"
	"github.com/marcusziade/stability-go/api"
	"github.com/marcusziade/stability-go/config"
	"github.com/marcusziade/stability-go/internal/logger"
)

func main() {
	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Create logger
	log := logger.NewFromString(cfg.LogLevel)
	log.Info("Starting Stability AI Upscale API Server")

	// Create Stability AI client
	client := stability.New(cfg.APIKey)
	if cfg.StabilityBaseURL != "" {
		client = client.WithBaseURL(cfg.StabilityBaseURL)
	}

	// Create API server
	server := api.New(client, log, cfg.CachePath, cfg.RateLimit, cfg.APIKey, cfg.ClientAPIKey, cfg.AllowedHosts)

	// Handle graceful shutdown
	go handleSignals(log)

	// Start server
	log.Info("Server listening on %s", cfg.ServerAddr)
	if err := server.Start(cfg.ServerAddr); err != nil {
		log.Error("Server error: %v", err)
		os.Exit(1)
	}
}

// handleSignals handles OS signals for graceful shutdown
func handleSignals(log *logger.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Info("Received signal %v, shutting down gracefully...", sig)

	// Perform cleanup if needed

	os.Exit(0)
}