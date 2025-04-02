package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Response structure matching the API's JSON response
type UpscaleResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		Image string `json:"image,omitempty"`
	} `json:"data"`
}

// Global loggers
var (
	logger    *log.Logger
	debugMode bool
)

// maskString masks sensitive information like API keys for logging
func maskString(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// debugLog logs information only when debug mode is enabled
func debugLog(format string, v ...interface{}) {
	if debugMode {
		logger.Printf("DEBUG: "+format, v...)
	}
}

// upscaleImage handles the API request and response processing
func upscaleImage(apiURL, apiKey, appID string, imageData []byte, filename string) ([]byte, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	debugLog("Preparing to upscale image %s (%d bytes)", filename, len(imageData))

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file part
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := part.Write(imageData); err != nil {
		return nil, fmt.Errorf("failed to write image data: %w", err)
	}

	// Add fast upscale type
	if err := writer.WriteField("type", "fast"); err != nil {
		return nil, fmt.Errorf("failed to write upscale type: %w", err)
	}

	// Close multipart writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-App-ID", appID)

	debugLog("Sending request to %s", apiURL)

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	debugLog("Request completed with status %d", resp.StatusCode)

	// Read response body
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	debugLog("Response body size: %d bytes", len(responseData))

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	// Parse the JSON response
	var upscaleResp UpscaleResponse
	if err := json.Unmarshal(responseData, &upscaleResp); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	// Check for API-level errors
	if !upscaleResp.Success {
		return nil, fmt.Errorf("API returned error: %s", upscaleResp.Error)
	}

	// Check if we have an image in the response
	if upscaleResp.Data.Image == "" {
		return nil, fmt.Errorf("no image data in response")
	}

	// Extract the base64 data from the data URL
	parts := strings.Split(upscaleResp.Data.Image, ";base64,")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid data format, expected 'data:type;base64,DATA'")
	}

	// Decode the outer base64 layer
	debugLog("Decoding outer base64 layer")
	decodedData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// The decoded data is actually JSON, so parse it
	debugLog("Decoding JSON wrapper")
	var nestedJson struct {
		Image string `json:"image"`
	}

	if err = json.Unmarshal(decodedData, &nestedJson); err != nil {
		return nil, fmt.Errorf("failed to parse nested JSON: %w", err)
	}

	// Now decode the inner base64 data (second layer of encoding)
	debugLog("Decoding inner base64 layer (actual image data)")
	imageData, err = base64.StdEncoding.DecodeString(nestedJson.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image data: %w", err)
	}

	// Return the actual image data
	return imageData, nil
}

func main() {
	// Parse command-line flags
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	outputPath := flag.String("output", "", "Output file path (default: ./output/<filename>_upscaled.png)")
	flag.Parse()

	debugMode = *debugFlag
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime)

	// Get remaining arguments after flags
	args := flag.Args()

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil && debugMode {
		logger.Printf("Warning: Error loading .env file: %v", err)
	}

	// Get config from environment variables
	apiURL := os.Getenv("UPSCALER_API_URL")
	apiKey := os.Getenv("UPSCALER_API_KEY")
	appID := os.Getenv("UPSCALER_APP_ID")

	if apiURL == "" || apiKey == "" || appID == "" {
		logger.Fatalf("Error: Missing required environment variables. Please check your .env file.")
	}

	// Check for command line arguments
	if len(args) < 1 {
		logger.Fatalf("Usage: %s [--debug] [--output path] <image-path>", os.Args[0])
	}

	imagePath := args[0]

	// Create output directory if needed
	var outputFilePath string
	if *outputPath != "" {
		outputFilePath = *outputPath
		outputDir := filepath.Dir(outputFilePath)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			logger.Fatalf("Error: Failed to create output directory: %v", err)
		}
	} else {
		outputDir := "output"
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			logger.Fatalf("Error: Failed to create output directory: %v", err)
		}
		baseName := filepath.Base(imagePath)
		baseName = baseName[:len(baseName)-len(filepath.Ext(baseName))]
		outputFilePath = filepath.Join(outputDir, baseName+"_upscaled.png")
	}

	debugLog("API URL: %s", apiURL)
	debugLog("App ID: %s", appID)
	debugLog("API Key: %s", maskString(apiKey))
	debugLog("Input file: %s", imagePath)
	debugLog("Output file: %s", outputFilePath)

	// Read input file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		logger.Fatalf("Error: Failed to read input file: %v", err)
	}

	// Upscale image
	logger.Println("Upscaling image...")
	startTime := time.Now()

	upscaledData, err := upscaleImage(apiURL, apiKey, appID, imageData, filepath.Base(imagePath))
	if err != nil {
		logger.Fatalf("Error: Failed to upscale image: %v", err)
	}

	// Decode the image to verify it's valid
	img, _, err := image.Decode(bytes.NewReader(upscaledData))
	if err != nil {
		logger.Fatalf("Error: Failed to decode upscaled image: %v", err)
	}

	// Save the image as PNG
	outFile, err := os.Create(outputFilePath)
	if err != nil {
		logger.Fatalf("Error: Failed to create output file: %v", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, img); err != nil {
		logger.Fatalf("Error: Failed to encode PNG: %v", err)
	}

	duration := time.Since(startTime)
	logger.Printf("Successfully upscaled image in %.2f seconds: %s", duration.Seconds(), outputFilePath)
}

