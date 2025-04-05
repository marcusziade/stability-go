package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Response is the standard response format from the API
type Response struct {
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// VideoResponse is the response format for the video endpoint
type VideoResponse struct {
	ID      string `json:"id,omitempty"`
	Video   string `json:"video,omitempty"`
	Pending bool   `json:"pending,omitempty"`
}

func main() {
	// Parse command line flags
	serverURL := flag.String("server", "https://api.stability.ai", "Server URL")
	apiKey := flag.String("api-key", "", "API key for the server")
	appID := flag.String("app-id", "", "App ID for authentication")
	inputFile := flag.String("input", "", "Input image file path")
	outputDir := flag.String("output", "output", "Output directory")
	motion := flag.String("motion", "zoom", "Motion type (zoom, pan, tilt, rotate, etc.)")
	duration := flag.Float64("duration", 2.0, "Duration in seconds (0.5-8.0)")
	fps := flag.Int("fps", 30, "Frames per second (1-60)")
	prompt := flag.String("prompt", "", "Prompt to guide video generation (optional)")
	negPrompt := flag.String("negative-prompt", "", "Negative prompt (optional)")
	seed := flag.Int("seed", 0, "Seed for consistent results (optional)")
	cfgScale := flag.Float64("cfg-scale", 0.5, "Creativity level (0.0-1.0)")
	resolution := flag.String("resolution", "512x512", "Video resolution (512x512, 768x768, 1024x576, 576x1024)")
	outputFormat := flag.String("format", "mp4", "Output format (mp4, gif, webm)")
	flag.Parse()

	// Validate inputs
	if *inputFile == "" {
		fmt.Println("Input file is required. Provide it with -input flag.")
		os.Exit(1)
	}

	if *apiKey == "" {
		fmt.Println("API key is required. Provide it with -api-key flag.")
		os.Exit(1)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Validate duration
	if *duration < 0.5 || *duration > 8.0 {
		fmt.Println("Duration must be between 0.5 and 8.0 seconds")
		os.Exit(1)
	}

	// Validate FPS
	if *fps < 1 || *fps > 60 {
		fmt.Println("FPS must be between 1 and 60")
		os.Exit(1)
	}

	// Validate cfg_scale
	if *cfgScale < 0.0 || *cfgScale > 1.0 {
		fmt.Println("CFG scale must be between 0.0 and 1.0")
		os.Exit(1)
	}

	// Read input file
	imageFile, err := os.Open(*inputFile)
	if err != nil {
		fmt.Printf("Failed to open input file: %v\n", err)
		os.Exit(1)
	}
	defer imageFile.Close()

	// Create a buffer to store the multipart form data
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Create a form file field
	fw, err := w.CreateFormFile("image", filepath.Base(*inputFile))
	if err != nil {
		fmt.Printf("Failed to create form file: %v\n", err)
		os.Exit(1)
	}

	// Copy the file data to the form field
	if _, err = io.Copy(fw, imageFile); err != nil {
		fmt.Printf("Failed to copy file data: %v\n", err)
		os.Exit(1)
	}

	// Add other form fields
	formFields := map[string]string{
		"motion":          *motion,
		"duration":        fmt.Sprintf("%.2f", *duration),
		"fps":             fmt.Sprintf("%d", *fps),
		"resolution":      *resolution,
		"output_format":   *outputFormat,
		"cfg_scale":       fmt.Sprintf("%.2f", *cfgScale),
	}

	if *prompt != "" {
		formFields["prompt"] = *prompt
	}

	if *negPrompt != "" {
		formFields["negative_prompt"] = *negPrompt
	}

	if *seed != 0 {
		formFields["seed"] = fmt.Sprintf("%d", *seed)
	}

	// Add all fields to the form
	for key, value := range formFields {
		if err := w.WriteField(key, value); err != nil {
			fmt.Printf("Failed to write field %s: %v\n", key, err)
			os.Exit(1)
		}
	}

	// Close the writer
	if err := w.Close(); err != nil {
		fmt.Printf("Failed to close multipart writer: %v\n", err)
		os.Exit(1)
	}

	// Create the HTTP request
	url := fmt.Sprintf("%s/v1/generation/image-to-video", *serverURL)
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		os.Exit(1)
	}

	// Set headers
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *apiKey))
	if *appID != "" {
		req.Header.Set("X-App-ID", *appID)
	}

	// Send the request
	fmt.Println("Sending image-to-video request to server...")
	fmt.Printf("Using motion: %s, duration: %.1f seconds, FPS: %d\n", *motion, *duration, *fps)
	if *prompt != "" {
		fmt.Printf("With prompt: %s\n", *prompt)
	}

	startTime := time.Now()
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to send request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Server returned error (status %d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	// Parse the response
	var apiResp Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		fmt.Printf("Failed to decode response: %v\n", err)
		os.Exit(1)
	}

	if !apiResp.Success {
		fmt.Printf("API returned error: %s\n", apiResp.Error)
		os.Exit(1)
	}

	// Parse the video response
	var videoResp VideoResponse
	if err := json.Unmarshal(apiResp.Data, &videoResp); err != nil {
		fmt.Printf("Failed to decode video response: %v\n", err)
		os.Exit(1)
	}

	// If we have an ID, we need to poll for the result
	if videoResp.ID != "" && videoResp.Pending {
		fmt.Println("Video generation initiated. Polling for results...")
		fmt.Println("This may take several minutes depending on the video settings.")

		// Poll for the result
		var dotCount int
		for {
			// Print a progress indicator
			if dotCount%60 == 0 {
				// Start a new line every 60 dots
				if dotCount > 0 {
					fmt.Println()
				}
				fmt.Print("Waiting for video generation to complete")
			}
			fmt.Print(".")
			dotCount++

			// Sleep between polls
			time.Sleep(5 * time.Second)

			// Send poll request
			pollURL := fmt.Sprintf("%s/api/v1/image-to-video/result/%s", *serverURL, videoResp.ID)
			pollReq, err := http.NewRequest("GET", pollURL, nil)
			if err != nil {
				fmt.Printf("\nFailed to create poll request: %v\n", err)
				os.Exit(1)
			}

			pollReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *apiKey))
			if *appID != "" {
				pollReq.Header.Set("X-App-ID", *appID)
			}

			pollResp, err := client.Do(pollReq)
			if err != nil {
				fmt.Printf("\nFailed to send poll request: %v\n", err)
				os.Exit(1)
			}

			// Check the response
			if pollResp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(pollResp.Body)
				fmt.Printf("\nServer returned error (status %d): %s\n", pollResp.StatusCode, string(body))
				pollResp.Body.Close()
				os.Exit(1)
			}

			// Parse the response
			var pollApiResp Response
			if err := json.NewDecoder(pollResp.Body).Decode(&pollApiResp); err != nil {
				fmt.Printf("\nFailed to decode poll response: %v\n", err)
				pollResp.Body.Close()
				os.Exit(1)
			}
			pollResp.Body.Close()

			if !pollApiResp.Success {
				fmt.Printf("\nAPI returned error: %s\n", pollApiResp.Error)
				os.Exit(1)
			}

			// Parse the video response
			var pollVideoResp VideoResponse
			if err := json.Unmarshal(pollApiResp.Data, &pollVideoResp); err != nil {
				fmt.Printf("\nFailed to decode poll video response: %v\n", err)
				os.Exit(1)
			}

			// If the video is ready, save it
			if !pollVideoResp.Pending && pollVideoResp.Video != "" {
				videoResp = pollVideoResp
				break
			}
		}

		fmt.Println("\nVideo generation completed!")
	}

	// Calculate elapsed time
	elapsed := time.Since(startTime)
	fmt.Printf("Video generation completed in %.1f seconds\n", elapsed.Seconds())

	// If we have a video, save it
	if videoResp.Video != "" {
		// Extract the base64 data
		dataParts := strings.Split(videoResp.Video, ";base64,")
		if len(dataParts) != 2 {
			fmt.Println("Invalid video data format")
			os.Exit(1)
		}

		// Determine file extension from mime type
		mimeType := strings.TrimPrefix(dataParts[0], "data:")
		ext := ".mp4" // Default
		if strings.Contains(mimeType, "gif") {
			ext = ".gif"
		} else if strings.Contains(mimeType, "webm") {
			ext = ".webm"
		}

		// Decode the base64 data
		data, err := decodeBase64(dataParts[1])
		if err != nil {
			fmt.Printf("Failed to decode base64 data: %v\n", err)
			os.Exit(1)
		}

		// Save the video
		baseName := filepath.Base(*inputFile)
		baseName = baseName[:len(baseName)-len(filepath.Ext(baseName))]
		outputPath := filepath.Join(*outputDir, fmt.Sprintf("%s_video%s", baseName, ext))

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			fmt.Printf("Failed to save video: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Saved video to %s\n", outputPath)
		fmt.Printf("Video details: %s format, %s resolution, %.1f seconds, %d FPS\n", 
			*outputFormat, *resolution, *duration, *fps)
	} else {
		fmt.Println("No video data received in the response")
	}
}

// decodeBase64 decodes a base64 string to a byte slice
func decodeBase64(data string) ([]byte, error) {
	// Add padding if necessary
	if len(data)%4 != 0 {
		data += strings.Repeat("=", 4-len(data)%4)
	}
	return base64.StdEncoding.DecodeString(data)
}