package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExtractAndSaveVideo extracts video data from the given data and saves it to the specified output directory
// Returns the path to the saved video file or an error
func ExtractAndSaveVideo(data []byte, outputDir string, filename string) (string, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}

	// Variable to store the video data
	var videoData []byte
	var extractionMethod string

	// 1. Try to parse as JSON with standard video field
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		// Check if we have a video field
		if video, ok := jsonData["video"]; ok {
			if videoStr, ok := video.(string); ok {
				fmt.Printf("Found video field in JSON, length: %d\n", len(videoStr))
				videoData = []byte(videoStr)
				extractionMethod = "direct video field"
			}
		}
	}

	// 2. If we still don't have video data and it looks like an MP4, use it directly
	if videoData == nil || len(videoData) == 0 {
		// Check if it looks like an MP4 (should start with some magic bytes like "AAAAI" or contains "ftyp")
		if len(data) > 5 && (string(data[:5]) == "AAAAI" || strings.Contains(string(data[:100]), "ftyp")) {
			fmt.Println("File appears to be an MP4 format")
			videoData = data
			extractionMethod = "raw MP4 content"
		}
	}

	// 3. If we still don't have video data, just use the raw data as a last resort
	if videoData == nil || len(videoData) == 0 {
		fmt.Println("Could not identify video format, saving raw data as video")
		videoData = data
		extractionMethod = "raw data fallback"
	}

	// Generate output filename
	ext := ".mp4" // Default to MP4
	if !strings.HasSuffix(filename, ext) {
		filename = filename + ext
	}
	outPath := filepath.Join(outputDir, filename)

	// Save the video
	if err := os.WriteFile(outPath, videoData, 0644); err != nil {
		return "", fmt.Errorf("error writing video file: %v", err)
	}

	fmt.Printf("Saved video to %s using %s method (%d bytes)\n", 
		outPath, extractionMethod, len(videoData))
	
	return outPath, nil
}