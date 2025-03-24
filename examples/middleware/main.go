package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/marcusziade/stability-go"
	"github.com/marcusziade/stability-go/client"
)

// Custom logging middleware implementation
type LoggingMiddleware struct {
	transport http.RoundTripper
}

func NewLoggingMiddleware(transport http.RoundTripper) *LoggingMiddleware {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &LoggingMiddleware{transport: transport}
}

func (m *LoggingMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	
	fmt.Printf("[%s] Request: %s %s\n", time.Now().Format(time.RFC3339), req.Method, req.URL.String())
	
	resp, err := m.transport.RoundTrip(req)
	
	duration := time.Since(start)
	
	if err != nil {
		fmt.Printf("[%s] Error: %v (took %.2fs)\n", time.Now().Format(time.RFC3339), err, duration.Seconds())
	} else {
		fmt.Printf("[%s] Response: %s (took %.2fs)\n", time.Now().Format(time.RFC3339), resp.Status, duration.Seconds())
	}
	
	return resp, err
}

func main() {
	// Get API key from environment
	apiKey := os.Getenv("STABILITY_API_KEY")
	if apiKey == "" {
		fmt.Println("STABILITY_API_KEY environment variable is required")
		os.Exit(1)
	}

	// Check if input file is provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: middleware <input-file>")
		os.Exit(1)
	}
	
	inputFile := os.Args[1]
	
	// Read image data
	imageData, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("Failed to read image: %v\n", err)
		os.Exit(1)
	}

	// Create output directory
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Create custom middleware chain
	// The middleware is applied in reverse order (last first)
	// 1. Logging (will be executed first)
	// 2. Retry (will be executed second)
	// 3. Rate limit (will be executed third)
	stClient := stability.NewWithMiddleware(
		apiKey,
		NewLoggingMiddleware(nil),                            // Custom logging middleware
		stability.WithRetry(3, 1*time.Second, 10*time.Second), // Retry middleware
		stability.WithRateLimit(500*time.Millisecond),         // Rate limit middleware
	)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create upscale request
	request := client.UpscaleRequest{
		Image:    imageData,
		Filename: filepath.Base(inputFile),
		Model:    client.UpscaleModelESRGAN,
		Factor:   2,
	}

	// Make the request
	fmt.Println("Upscaling image with middleware...")
	response, err := stClient.Upscale(ctx, request)
	if err != nil {
		fmt.Printf("Failed to upscale image: %v\n", err)
		os.Exit(1)
	}

	// Save results
	for i, artifact := range response.Artifacts {
		// Decode base64 image
		imageData, err := base64.StdEncoding.DecodeString(artifact.Base64)
		if err != nil {
			fmt.Printf("Failed to decode image %d: %v\n", i, err)
			continue
		}

		// Create output filename
		ext := filepath.Ext(inputFile)
		baseName := filepath.Base(inputFile[:len(inputFile)-len(ext)])
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_upscaled_%d%s", baseName, i, ext))

		// Save image
		if err := os.WriteFile(outputPath, imageData, 0644); err != nil {
			fmt.Printf("Failed to save image %d: %v\n", i, err)
			continue
		}

		fmt.Printf("Saved upscaled image to %s\n", outputPath)
	}
}