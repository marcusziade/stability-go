package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Get config from environment variables
	apiURL := os.Getenv("UPSCALER_API_URL")
	apiKey := os.Getenv("UPSCALER_API_KEY")
	appID := os.Getenv("UPSCALER_APP_ID")

	if apiURL == "" || apiKey == "" || appID == "" {
		log.Fatalf("Missing required environment variables. Please check your .env file.")
	}

	// Check for command line arguments
	args := os.Args[1:]
	if len(args) < 1 {
		log.Fatalf("Usage: %s <image-path>", os.Args[0])
	}

	imagePath := args[0]

	// Create output directory if it doesn't exist
	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Read input file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Get filename
	filename := filepath.Base(imagePath)

	// Upscale using fast method
	fmt.Println("Upscaling image...")
	startTime := time.Now()

	responseData, err := upscaleImage(apiURL, apiKey, appID, imageData, filename)
	if err != nil {
		log.Fatalf("Failed to upscale image: %v", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("Upscale completed in %.2f seconds\n", duration.Seconds())

	// Create output filename
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_upscaled.png", filename[:len(filename)-len(filepath.Ext(filename))]))

	// Save the image
	if err := os.WriteFile(outputPath, responseData, 0644); err != nil {
		log.Fatalf("Failed to save image: %v", err)
	}

	fmt.Printf("Saved upscaled image to %s\n", outputPath)
}

// Response structure matching the API's JSON response
type UpscaleResponse struct {
	Success bool `json:"success"`
	Error   string `json:"error,omitempty"`
	Data    struct {
		Image   string `json:"image,omitempty"`
		ID      string `json:"id,omitempty"`
		Pending bool   `json:"pending,omitempty"`
	} `json:"data"`
}

func upscaleImage(apiURL, apiKey, appID string, imageData []byte, filename string) ([]byte, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		// Try to read error details
		errorBody, _ := io.ReadAll(resp.Body)
		var jsonError map[string]interface{}
		if err := json.Unmarshal(errorBody, &jsonError); err == nil {
			return nil, fmt.Errorf("API error (status %d): %v", resp.StatusCode, jsonError)
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(errorBody))
	}

	// Read response body
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
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

	// Extract the image data from the data URL format
	// Format is "data:image/png;base64,<base64 data>"
	parts := bytes.Split([]byte(upscaleResp.Data.Image), []byte(";base64,"))
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid image data format")
	}

	// Parse the hex-encoded data
	// Stability API is using hex encoding, not base64
	imageBytes, err := hex.DecodeString(string(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image data: %w", err)
	}

	return imageBytes, nil
}
