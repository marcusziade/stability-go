package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/gif"  // Register GIF format
	_ "image/jpeg" // Register JPEG format
	_ "image/png"  // Register PNG format
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marcusziade/stability-go"
	"github.com/marcusziade/stability-go/client"
)

func main() {
	// Parse command line flags
	apiKey := flag.String("api-key", os.Getenv("STABILITY_API_KEY"), "Stability API key")
	inputFile := flag.String("input", "", "Input image file path")
	outputDir := flag.String("output", "output", "Output directory")
	motionBucketID := flag.Int("motion-bucket-id", 127, "Motion bucket ID (0-255, higher values = more motion)")
	motion := flag.String("motion", "", "Motion type (legacy parameter: zoom, pan, tilt, rotate, etc.)")
	duration := flag.Float64("duration", 2.0, "Duration in seconds (0.5-8.0)")
	fps := flag.Int("fps", 30, "Frames per second (1-60)")
	prompt := flag.String("prompt", "", "Prompt to guide video generation (optional)")
	negPrompt := flag.String("negative-prompt", "", "Negative prompt (optional)")
	seed := flag.Int64("seed", 0, "Seed for consistent results (0 for random)")
	cfgScale := flag.Float64("cfg-scale", 1.8, "Creativity level (default 1.8)")
	resolution := flag.String("resolution", "512x512", "Video resolution (512x512, 768x768, 1024x576, 576x1024)")
	outputFormat := flag.String("format", "mp4", "Output format (mp4, gif, webm)")
	useProxy := flag.Bool("proxy", false, "Use proxy")
	proxyURL := flag.String("proxy-url", "your-proxy-server.com", "Proxy URL")
	flag.Parse()

	// Validate inputs
	if *apiKey == "" {
		fmt.Println("API key is required. Provide it with -api-key flag or STABILITY_API_KEY environment variable.")
		os.Exit(1)
	}

	if *inputFile == "" {
		fmt.Println("Input file is required. Provide it with -input flag.")
		os.Exit(1)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	// Read input file
	imageData, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Printf("Failed to read input file: %v\n", err)
		os.Exit(1)
	}
	
	// Check image dimensions (API currently only supports specific dimensions)
	img, _, err := image.DecodeConfig(bytes.NewReader(imageData))
	if err != nil {
		fmt.Printf("Failed to decode image: %v\n", err)
		os.Exit(1)
	}
	
	validDimensions := []struct{ width, height int }{
		{1024, 576},
		{576, 1024},
		{768, 768},
	}
	
	isValidDimension := false
	for _, dim := range validDimensions {
		if img.Width == dim.width && img.Height == dim.height {
			isValidDimension = true
			break
		}
	}
	
	if !isValidDimension {
		fmt.Printf("Invalid image dimensions: %dx%d\n", img.Width, img.Height)
		fmt.Println("Supported dimensions: 1024x576, 576x1024, 768x768")
		os.Exit(1)
	}

	// Get filename
	filename := filepath.Base(*inputFile)

	// Parse motion type (legacy parameter)
	var motionEnum client.VideoMotion
	if *motion != "" {
		switch *motion {
		case "none":
			motionEnum = client.VideoMotionNone
		case "zoom":
			motionEnum = client.VideoMotionZoom
		case "pan":
			motionEnum = client.VideoMotionPan
		case "tilt":
			motionEnum = client.VideoMotionTilt
		case "rotate":
			motionEnum = client.VideoMotionRotate
		case "zoom_out":
			motionEnum = client.VideoMotionZoomOut
		case "pan_left":
			motionEnum = client.VideoMotionPanLeft
		case "pan_right":
			motionEnum = client.VideoMotionPanRight
		case "tilt_up":
			motionEnum = client.VideoMotionTiltUp
		case "tilt_down":
			motionEnum = client.VideoMotionTiltDown
		case "rotate_left":
			motionEnum = client.VideoMotionRotateLeft
		case "rotate_right":
			motionEnum = client.VideoMotionRotateRight
		default:
			fmt.Printf("Invalid motion type: %s\n", *motion)
			os.Exit(1)
		}
	}

	// Parse output format
	var outputFormatEnum client.VideoFormat
	switch *outputFormat {
	case "mp4":
		outputFormatEnum = client.VideoFormatMP4
	case "gif":
		outputFormatEnum = client.VideoFormatGIF
	case "webm":
		outputFormatEnum = client.VideoFormatWEBM
	default:
		fmt.Printf("Invalid output format: %s\n", *outputFormat)
		os.Exit(1)
	}

	// Parse resolution
	var resolutionEnum client.VideoResolution
	switch *resolution {
	case "512x512":
		resolutionEnum = client.VideoResolution512x512
	case "768x768":
		resolutionEnum = client.VideoResolution768x768
	case "1024x576":
		resolutionEnum = client.VideoResolution1024x576
	case "576x1024":
		resolutionEnum = client.VideoResolution576x1024
	default:
		fmt.Printf("Invalid resolution: %s\n", *resolution)
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

	// Validate motion bucket ID
	if *motionBucketID < 0 || *motionBucketID > 255 {
		fmt.Println("Motion bucket ID must be between 0 and 255")
		os.Exit(1)
	}

	// Create context with timeout - increased for video generation
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create client with middleware if proxy is enabled
	var stClient *client.Client
	if *useProxy {
		stClient = stability.NewWithMiddleware(*apiKey,
			stability.WithRateLimit(500*time.Millisecond),
			stability.WithRetry(3, 1*time.Second, 10*time.Second),
			stability.WithProxy(*proxyURL),
		).GetClient()
	} else {
		stClient = stability.New(*apiKey)
	}

	// Prepare request
	request := client.ImageToVideoRequest{
		Image:          imageData,
		Filename:       filename,
		MotionBucketID: *motionBucketID,
		Motion:         motionEnum,
		Prompt:         *prompt,
		NegativePrompt: *negPrompt,
		Seed:           *seed,
		Duration:       *duration,
		FPS:            *fps,
		Resolution:     resolutionEnum,
		OutputFormat:   outputFormatEnum,
		CFGScale:       *cfgScale,
	}

	// Make the request
	fmt.Println("Generating video from image...")
	fmt.Printf("Using motion bucket ID: %d, duration: %.1f seconds, FPS: %d\n", *motionBucketID, *duration, *fps)
	if *prompt != "" {
		fmt.Printf("With prompt: %s\n", *prompt)
	}
	
	startTime := time.Now()
	
	response, err := stClient.ImageToVideo(ctx, request)
	if err != nil {
		// Check if it's a content policy violation
		if err.Error() != "" && (strings.Contains(err.Error(), "content policy violation") || 
		   strings.Contains(err.Error(), "forbidden")) {
			fmt.Printf("Content policy error: %v\n", err)
			fmt.Println("This image likely violates Stability AI's content policies.")
			fmt.Println("Please try a different image or check the image content against Stability AI's guidelines.")
		} else {
			fmt.Printf("Failed to generate video: %v\n", err)
		}
		os.Exit(1)
	}

	// Display video ID and how to get the result directly
	fmt.Printf("\nVideo generation initiated with ID: %s\n", response.ID)
	fmt.Printf("This job is now processing asynchronously. You can:\n")
	fmt.Printf("1. Wait for completion (may take several minutes)\n")
	fmt.Printf("2. Check status later with this curl command:\n   curl -H \"Authorization: Bearer YOUR_API_KEY\" https://api.stability.ai/v2beta/image-to-video/result/%s\n", response.ID)
	
	fmt.Printf("\nSaving video ID to file for later reference...\n")
	idFilePath := filepath.Join(*outputDir, "video_id.txt")
	idFileContent := fmt.Sprintf("Video ID: %s\nCheck status: curl -H \"Authorization: Bearer YOUR_API_KEY\" https://api.stability.ai/v2beta/image-to-video/result/%s\n", 
		response.ID, response.ID)
	if err := os.WriteFile(idFilePath, []byte(idFileContent), 0644); err != nil {
		fmt.Printf("Warning: Could not save video ID to file: %v\n", err)
	} else {
		fmt.Printf("Video ID saved to %s\n", idFilePath)
	}
	
	waitForCompletion := true
	
	// If user wants to wait, then poll for the result
	if waitForCompletion {
		fmt.Println("Waiting for video generation to complete...")
		
		// Poll for the result with a progress indicator
		var dotCount int
		startPolling := time.Now()
		for {
			// Print a progress indicator
			if dotCount%60 == 0 && dotCount > 0 {
				fmt.Println()
				elapsedTime := time.Since(startPolling)
				fmt.Printf("Still waiting... (%.0f seconds elapsed) ", elapsedTime.Seconds())
			}
			fmt.Print(".")
			dotCount++
			
			// Sleep between polls
			time.Sleep(5 * time.Second)
			
			// Check if context is done
			select {
			case <-ctx.Done():
				fmt.Printf("\nTimeout reached: %v\n", ctx.Err())
				os.Exit(1)
			default:
				// Continue
			}
			
			result, finished, err := stClient.PollVideoResult(ctx, response.ID)
			if err != nil {
				// Check if it's a content policy violation
				if err.Error() != "" && (strings.Contains(err.Error(), "content policy violation") || 
				   strings.Contains(err.Error(), "forbidden")) {
					fmt.Printf("\nContent policy error during processing: %v\n", err)
					fmt.Println("This may indicate that the generated content violates Stability AI's content policies.")
				} else {
					// For 202 status, just continue polling
					if strings.Contains(err.Error(), "status 202") {
						continue
					}
					fmt.Printf("\nError polling for results: %v\n", err)
				}
				os.Exit(1)
			}
			
			if finished {
				fmt.Printf("\nVideo received! Video data length: %d bytes, MIME type: %s\n", len(result.VideoData), result.MimeType)
				response = result
				break
			}
		}
	} else {
		fmt.Println("Video generation is continuing in the background.")
		fmt.Printf("Use the curl commands above to check the status and download when complete.\n")
		os.Exit(0)
	}
	
	fmt.Println("\nVideo generation completed!")
	elapsed := time.Since(startTime)
	fmt.Printf("Generation completed in %.1f seconds\n", elapsed.Seconds())

	// Create output filename
	ext := "." + string(outputFormatEnum)
	baseName := filename[:len(filename)-len(filepath.Ext(filename))]
	outputPath := filepath.Join(*outputDir, fmt.Sprintf("%s_video%s", baseName, ext))

	// Save the video
	if err := os.WriteFile(outputPath, response.VideoData, 0644); err != nil {
		fmt.Printf("Failed to save video: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved video to %s\n", outputPath)
	fmt.Printf("Video details: %s format, %s resolution, %.1f seconds, %d FPS\n", 
		*outputFormat, *resolution, *duration, *fps)
}