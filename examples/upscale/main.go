package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/marcusziade/stability-go"
	"github.com/marcusziade/stability-go/client"
)

func main() {
	// Parse command line flags
	apiKey := flag.String("api-key", os.Getenv("STABILITY_API_KEY"), "Stability API key")
	inputFile := flag.String("input", "", "Input image file path")
	outputDir := flag.String("output", "output", "Output directory")
	upscaleType := flag.String("type", "fast", "Upscale type (fast, conservative, creative)")
	prompt := flag.String("prompt", "", "Prompt to guide upscaling (required for conservative and creative)")
	negPrompt := flag.String("negative-prompt", "", "Negative prompt (optional)")
	seed := flag.Int64("seed", 0, "Seed for consistent results (optional)")
	creativity := flag.Float64("creativity", 0.0, "Creativity level (0.1-0.5 for creative, 0.2-0.5 for conservative)")
	stylePreset := flag.String("style", "", "Style preset for creative upscale (3d-model, anime, etc.)")
	outputFormat := flag.String("format", "png", "Output format (jpeg, png, webp)")
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

	// Get filename
	filename := filepath.Base(*inputFile)

	// Parse upscale type
	var upscaleTypeEnum client.UpscaleType
	switch *upscaleType {
	case "fast":
		upscaleTypeEnum = client.UpscaleTypeFast
	case "conservative":
		upscaleTypeEnum = client.UpscaleTypeConservative
	case "creative":
		upscaleTypeEnum = client.UpscaleTypeCreative
	default:
		fmt.Printf("Invalid upscale type: %s\n", *upscaleType)
		os.Exit(1)
	}

	// Parse output format
	var outputFormatEnum client.OutputFormat
	switch *outputFormat {
	case "jpeg":
		outputFormatEnum = client.OutputFormatJPEG
	case "png":
		outputFormatEnum = client.OutputFormatPNG
	case "webp":
		outputFormatEnum = client.OutputFormatWEBP
	default:
		fmt.Printf("Invalid output format: %s\n", *outputFormat)
		os.Exit(1)
	}

	// Parse style preset if provided
	var stylePresetEnum client.StylePreset
	if *stylePreset != "" {
		switch *stylePreset {
		case "3d-model":
			stylePresetEnum = client.StylePreset3DModel
		case "analog-film":
			stylePresetEnum = client.StylePresetAnalogFilm
		case "anime":
			stylePresetEnum = client.StylePresetAnime
		case "cinematic":
			stylePresetEnum = client.StylePresetCinematic
		case "comic-book":
			stylePresetEnum = client.StylePresetComicBook
		case "digital-art":
			stylePresetEnum = client.StylePresetDigitalArt
		case "enhance":
			stylePresetEnum = client.StylePresetEnhance
		case "fantasy-art":
			stylePresetEnum = client.StylePresetFantasyArt
		case "isometric":
			stylePresetEnum = client.StylePresetIsometric
		case "line-art":
			stylePresetEnum = client.StylePresetLineArt
		case "low-poly":
			stylePresetEnum = client.StylePresetLowPoly
		case "modeling-compound":
			stylePresetEnum = client.StylePresetModelingCompound
		case "neon-punk":
			stylePresetEnum = client.StylePresetNeonPunk
		case "origami":
			stylePresetEnum = client.StylePresetOrigami
		case "photographic":
			stylePresetEnum = client.StylePresetPhotographic
		case "pixel-art":
			stylePresetEnum = client.StylePresetPixelArt
		case "tile-texture":
			stylePresetEnum = client.StylePresetTileTexture
		default:
			fmt.Printf("Invalid style preset: %s\n", *stylePreset)
			os.Exit(1)
		}
	}

	// Check if prompt is provided for conservative and creative types
	if (upscaleTypeEnum == client.UpscaleTypeConservative || upscaleTypeEnum == client.UpscaleTypeCreative) && *prompt == "" {
		fmt.Printf("Prompt is required for %s upscale type\n", *upscaleType)
		os.Exit(1)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
	request := client.UpscaleRequest{
		Image:          imageData,
		Filename:       filename,
		Type:           upscaleTypeEnum,
		Prompt:         *prompt,
		NegativePrompt: *negPrompt,
		Seed:           *seed,
		OutputFormat:   outputFormatEnum,
		Creativity:     *creativity,
		StylePreset:    stylePresetEnum,
	}

	// Make the request
	fmt.Println("Upscaling image...")
	startTime := time.Now()
	
	response, err := stClient.Upscale(ctx, request)
	if err != nil {
		fmt.Printf("Failed to upscale image: %v\n", err)
		os.Exit(1)
	}

	// For Creative upscale, we need to poll for the result
	if upscaleTypeEnum == client.UpscaleTypeCreative {
		fmt.Println("Creative upscale initiated. Polling for results...")
		
		// Poll for the result (every 2 seconds)
		for {
			time.Sleep(2 * time.Second)
			
			result, finished, err := stClient.PollCreativeResult(ctx, response.CreativeID)
			if err != nil {
				fmt.Printf("Error polling for results: %v\n", err)
				os.Exit(1)
			}
			
			if finished {
				response = result
				break
			}
			
			fmt.Print(".")
		}
		fmt.Println("\nCreative upscale completed!")
	}

	duration := time.Since(startTime)
	fmt.Printf("Upscale completed in %.2f seconds\n", duration.Seconds())

	// Create output filename
	ext := "." + string(outputFormatEnum)
	baseName := filename[:len(filename)-len(filepath.Ext(filename))]
	outputPath := filepath.Join(*outputDir, fmt.Sprintf("%s_upscaled%s", baseName, ext))

	// Save the image
	if err := os.WriteFile(outputPath, response.ImageData, 0644); err != nil {
		fmt.Printf("Failed to save image: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved upscaled image to %s\n", outputPath)
}