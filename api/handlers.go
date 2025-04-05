package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/marcusziade/stability-go/client"
	"github.com/marcusziade/stability-go/internal/logger"
)

// Server represents the API server
type Server struct {
	Router        http.Handler
	Client        *client.Client
	Logger        *logger.Logger
	CachePath     string
	RateLimit     time.Duration
	APIKey        string
	ClientAPIKey  string
	AllowedHost   []string
	AllowedIPs    []string
	AllowedAppIDs []string
}

// Response is the standard JSON response format
type Response struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// UpscaleResponse is the response format for the upscale endpoint
type UpscaleResponse struct {
	ID      string `json:"id,omitempty"`
	Image   string `json:"image,omitempty"`
	Pending bool   `json:"pending,omitempty"`
}

// VideoResponse is the response format for the image-to-video endpoint
type VideoResponse struct {
	ID      string `json:"id,omitempty"`
	Video   string `json:"video,omitempty"`
	Pending bool   `json:"pending,omitempty"`
}

// New creates a new API server
func New(client *client.Client, logger *logger.Logger, cachePath string, rateLimit time.Duration, apiKey string, clientAPIKey string, allowedHosts []string, allowedIPs []string, allowedAppIDs []string) *Server {
	s := &Server{
		Client:        client,
		Logger:        logger,
		CachePath:     cachePath,
		RateLimit:     rateLimit,
		APIKey:        apiKey,
		ClientAPIKey:  clientAPIKey,
		AllowedHost:   allowedHosts,
		AllowedIPs:    allowedIPs,
		AllowedAppIDs: allowedAppIDs,
	}

	// Create the router
	mux := http.NewServeMux()

	// Register routes with middleware
	mux.Handle("/", http.HandlerFunc(s.handleRoot))
	mux.Handle("/api/v1/upscale", WithAuth(clientAPIKey, nil)(http.HandlerFunc(s.handleUpscale)))
	mux.Handle("/api/v1/upscale/result/", WithAuth(clientAPIKey, nil)(http.HandlerFunc(s.handleUpscaleResult)))
	mux.Handle("/api/v1/image-to-video", WithAuth(clientAPIKey, nil)(http.HandlerFunc(s.handleImageToVideo)))
	mux.Handle("/api/v1/image-to-video/result/", WithAuth(clientAPIKey, nil)(http.HandlerFunc(s.handleVideoResult)))
	mux.Handle("/health", http.HandlerFunc(s.handleHealthCheck))
	mux.Handle("/api/docs", http.HandlerFunc(s.handleDocs))

	// Apply global middleware
	s.Router = Chain(
		WithLogger(logger),
		WithCORS(nil), // Allow all origins
		WithIPFilter(s.AllowedIPs),
		WithAppIDAuth(s.AllowedAppIDs),
	)(mux)

	// Create cache directory if it doesn't exist
	if cachePath != "" {
		if err := os.MkdirAll(cachePath, 0o755); err != nil {
			logger.Error("Failed to create cache directory: %v", err)
		} else {
			logger.Info("Cache enabled at %s", cachePath)
		}
	}

	return s
}

// Start starts the API server
func (s *Server) Start(addr string) error {
	s.Logger.Info("Starting API server on %s", addr)
	return http.ListenAndServe(addr, s.Router)
}

// handleUpscale handles upscale requests
func (s *Server) handleUpscale(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.sendError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get upscale type
	upscaleType := r.FormValue("type")
	if upscaleType == "" {
		upscaleType = "fast" // Default to fast upscaling
	}

	// Map upscale type to enum
	var upscaleTypeEnum client.UpscaleType
	switch upscaleType {
	case "fast":
		upscaleTypeEnum = client.UpscaleTypeFast
	case "conservative":
		upscaleTypeEnum = client.UpscaleTypeConservative
	case "creative":
		upscaleTypeEnum = client.UpscaleTypeCreative
	default:
		s.sendError(w, "Invalid upscale type", http.StatusBadRequest)
		return
	}

	// Check if prompt is provided for conservative and creative types
	prompt := r.FormValue("prompt")
	if (upscaleTypeEnum == client.UpscaleTypeConservative || upscaleTypeEnum == client.UpscaleTypeCreative) && prompt == "" {
		s.sendError(w, "Prompt is required for conservative and creative upscale types", http.StatusBadRequest)
		return
	}

	// Get image file
	file, header, err := r.FormFile("image")
	if err != nil {
		s.sendError(w, "Failed to get image file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read image data
	imageData, err := io.ReadAll(file)
	if err != nil {
		s.sendError(w, "Failed to read image data", http.StatusInternalServerError)
		return
	}

	// Generate cache key
	cacheKey := generateCacheKey(imageData, r.Form)

	// Check cache if enabled
	if s.CachePath != "" {
		cachePath := filepath.Join(s.CachePath, cacheKey+".json")

		// Check if cache file exists
		if _, err := os.Stat(cachePath); err == nil {
			s.Logger.Info("Cache hit for %s", cacheKey)

			// Read cache file
			cacheData, err := os.ReadFile(cachePath)
			if err == nil {
				// Return cached response
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				w.Write(cacheData)
				return
			}
		}
	}

	// Get optional parameters
	negativePrompt := r.FormValue("negative_prompt")
	seed, _ := strconv.ParseInt(r.FormValue("seed"), 10, 64)
	creativityStr := r.FormValue("creativity")
	var creativity float64
	if creativityStr != "" {
		creativity, _ = strconv.ParseFloat(creativityStr, 64)
	}

	// Get output format
	outputFormat := r.FormValue("output_format")
	var outputFormatEnum client.OutputFormat
	switch outputFormat {
	case "jpeg":
		outputFormatEnum = client.OutputFormatJPEG
	case "webp":
		outputFormatEnum = client.OutputFormatWEBP
	default:
		outputFormatEnum = client.OutputFormatPNG // Default to PNG
	}

	// Get style preset for creative upscale
	stylePreset := r.FormValue("style_preset")
	var stylePresetEnum client.StylePreset
	if stylePreset != "" {
		switch stylePreset {
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
			s.sendError(w, "Invalid style preset", http.StatusBadRequest)
			return
		}
	}

	// Create upscale request
	request := client.UpscaleRequest{
		Image:          imageData,
		Filename:       header.Filename,
		Type:           upscaleTypeEnum,
		Prompt:         prompt,
		NegativePrompt: negativePrompt,
		Seed:           seed,
		OutputFormat:   outputFormatEnum,
		Creativity:     creativity,
		StylePreset:    stylePresetEnum,
		ReturnAsJSON:   true,
	}

	// Send request to Stability AI
	s.Logger.Info("Sending upscale request to Stability AI (type: %s)", upscaleType)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	response, err := s.Client.Upscale(ctx, request)
	if err != nil {
		s.Logger.Error("Error from Stability AI: %v", err)
		s.sendError(w, fmt.Sprintf("Error from Stability AI: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare response
	var apiResp Response
	var upscaleResp UpscaleResponse

	if upscaleTypeEnum == client.UpscaleTypeCreative {
		// For creative upscale, we get an ID for polling
		upscaleResp = UpscaleResponse{
			ID:      response.CreativeID,
			Pending: true,
		}
	} else {
		// For fast and conservative upscale, we get the image directly
		// Base64 encode the image for JSON response
		upscaleResp = UpscaleResponse{
			Image: "data:" + response.MimeType + ";base64," + encodeBase64(response.ImageData),
		}
	}

	apiResp = Response{
		Success: true,
		Data:    upscaleResp,
	}

	// Convert response to JSON
	responseData, err := json.Marshal(apiResp)
	if err != nil {
		s.sendError(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	// Cache response if enabled
	if s.CachePath != "" {
		cachePath := filepath.Join(s.CachePath, cacheKey+".json")
		if err := os.WriteFile(cachePath, responseData, 0o644); err != nil {
			s.Logger.Error("Failed to write cache file: %v", err)
		} else {
			s.Logger.Info("Cached response at %s", cachePath)
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}

// handleUpscaleResult handles polling for creative upscale results
func (s *Server) handleUpscaleResult(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get creative ID from URL
	id := filepath.Base(r.URL.Path)
	if id == "" {
		s.sendError(w, "Missing creative ID", http.StatusBadRequest)
		return
	}

	// Poll for the result
	s.Logger.Info("Polling for creative upscale result (ID: %s)", id)
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, finished, err := s.Client.PollCreativeResult(ctx, id)
	if err != nil {
		s.Logger.Error("Error polling for creative upscale result: %v", err)
		s.sendError(w, fmt.Sprintf("Error polling for creative upscale result: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare response
	upscaleResp := UpscaleResponse{
		ID:      id,
		Pending: !finished,
	}

	// If the upscale is finished, include the image data
	if finished {
		upscaleResp.Image = "data:" + result.MimeType + ";base64," + encodeBase64(result.ImageData)
	}

	// Send response
	s.sendJSON(w, Response{
		Success: true,
		Data:    upscaleResp,
	})
}

// handleHealthCheck handles health check requests
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Perform a simple API check
	info := map[string]interface{}{
		"status":  "ok",
		"version": "1.0.0",
		"uptime":  "up",
	}

	// Send response
	s.sendJSON(w, Response{
		Success: true,
		Data:    info,
	})
}

// handleDocs serves the API documentation
func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Define API documentation
	docs := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "Stability AI SDK API",
			"description": "API for upscaling images and generating videos using Stability AI",
			"version":     "1.1.0",
		},
		"paths": map[string]interface{}{
			"/api/v1/upscale": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Upscale an image",
					"description": "Upscales an image using Stability AI's upscale API",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"multipart/form-data": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"image": map[string]interface{}{
											"type":        "string",
											"format":      "binary",
											"description": "The image file to upscale",
										},
										"type": map[string]interface{}{
											"type":        "string",
											"description": "The upscale type (fast, conservative, creative)",
											"enum":        []string{"fast", "conservative", "creative"},
											"default":     "fast",
										},
										"prompt": map[string]interface{}{
											"type":        "string",
											"description": "The prompt to guide upscaling (required for conservative and creative)",
										},
										"negative_prompt": map[string]interface{}{
											"type":        "string",
											"description": "The negative prompt to guide upscaling (optional)",
										},
										"seed": map[string]interface{}{
											"type":        "integer",
											"description": "The seed for consistent results (optional)",
										},
										"creativity": map[string]interface{}{
											"type":        "number",
											"description": "The creativity level (0.1-0.5 for creative, 0.2-0.5 for conservative)",
										},
										"style_preset": map[string]interface{}{
											"type":        "string",
											"description": "The style preset for creative upscale",
											"enum": []string{
												"3d-model", "analog-film", "anime", "cinematic", "comic-book",
												"digital-art", "enhance", "fantasy-art", "isometric", "line-art",
												"low-poly", "modeling-compound", "neon-punk", "origami",
												"photographic", "pixel-art", "tile-texture",
											},
										},
										"output_format": map[string]interface{}{
											"type":        "string",
											"description": "The output format",
											"enum":        []string{"png", "jpeg", "webp"},
											"default":     "png",
										},
									},
									"required": []string{"image"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/UpscaleResponse",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Bad request",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ErrorResponse",
									},
								},
							},
						},
					},
				},
			},
			"/api/v1/upscale/result/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Get the result of a creative upscale",
					"description": "Polls for the result of a creative upscale",
					"parameters": []map[string]interface{}{
						{
							"name":        "id",
							"in":          "path",
							"description": "The creative upscale ID",
							"required":    true,
							"schema": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/UpscaleResponse",
									},
								},
							},
						},
						"400": map[string]interface{}{
							"description": "Bad request",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ErrorResponse",
									},
								},
							},
						},
					},
				},
			},
			"/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Health check",
					"description": "Checks if the API is healthy",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "API is healthy",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/HealthResponse",
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"UpscaleResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the request was successful",
						},
						"error": map[string]interface{}{
							"type":        "string",
							"description": "Error message if the request failed",
						},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id": map[string]interface{}{
									"type":        "string",
									"description": "The creative upscale ID (only for creative upscale)",
								},
								"image": map[string]interface{}{
									"type":        "string",
									"description": "The upscaled image data as a base64-encoded string",
								},
								"pending": map[string]interface{}{
									"type":        "boolean",
									"description": "Whether the upscale is still pending (only for creative upscale)",
								},
							},
						},
					},
				},
				"ErrorResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the request was successful",
						},
						"error": map[string]interface{}{
							"type":        "string",
							"description": "Error message",
						},
					},
				},
				"HealthResponse": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"success": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the request was successful",
						},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"status": map[string]interface{}{
									"type":        "string",
									"description": "API status",
								},
								"version": map[string]interface{}{
									"type":        "string",
									"description": "API version",
								},
								"uptime": map[string]interface{}{
									"type":        "string",
									"description": "API uptime",
								},
							},
						},
					},
				},
			},
			"securitySchemes": map[string]interface{}{
				"BearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "API key",
				},
			},
		},
		"security": []map[string]interface{}{
			{
				"BearerAuth": []string{},
			},
		},
	}

	// Send response
	s.sendJSON(w, docs)
}

// Helper functions

// sendError sends an error response
func (s *Server) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success: false,
		Error:   message,
	}

	json.NewEncoder(w).Encode(response)
}

// sendJSON sends a JSON response
func (s *Server) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// generateCacheKey generates a cache key for the request
func generateCacheKey(imageData []byte, formData map[string][]string) string {
	// Create a hash of the image data
	hash := sha256.Sum256(imageData)
	imageHash := hex.EncodeToString(hash[:])

	// Add form data to the hash
	formString := fmt.Sprintf("%v", formData)
	combinedHash := sha256.Sum256([]byte(imageHash + formString))

	return hex.EncodeToString(combinedHash[:])
}

// encodeBase64 encodes data as base64
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// handleImageToVideo handles image-to-video requests
func (s *Server) handleImageToVideo(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.sendError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get image file
	file, header, err := r.FormFile("image")
	if err != nil {
		s.sendError(w, "Failed to get image file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read image data
	imageData, err := io.ReadAll(file)
	if err != nil {
		s.sendError(w, "Failed to read image data", http.StatusInternalServerError)
		return
	}

	// Generate cache key
	cacheKey := generateCacheKey(imageData, r.Form)

	// Check cache if enabled
	if s.CachePath != "" {
		cachePath := filepath.Join(s.CachePath, cacheKey+".json")

		// Check if cache file exists
		if _, err := os.Stat(cachePath); err == nil {
			s.Logger.Info("Cache hit for %s", cacheKey)

			// Read cache file
			cacheData, err := os.ReadFile(cachePath)
			if err == nil {
				// Return cached response
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				w.Write(cacheData)
				return
			}
		}
	}

	// Get required parameters
	motion := r.FormValue("motion")
	if motion == "" {
		s.sendError(w, "Motion is required", http.StatusBadRequest)
		return
	}

	// Map motion to enum
	var motionEnum client.VideoMotion
	switch motion {
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
		s.sendError(w, "Invalid motion type", http.StatusBadRequest)
		return
	}

	// Get required duration
	durationStr := r.FormValue("duration")
	if durationStr == "" {
		s.sendError(w, "Duration is required", http.StatusBadRequest)
		return
	}

	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil || duration < 0.5 || duration > 8.0 {
		s.sendError(w, "Duration must be between 0.5 and 8.0 seconds", http.StatusBadRequest)
		return
	}

	// Get required fps
	fpsStr := r.FormValue("fps")
	if fpsStr == "" {
		s.sendError(w, "FPS is required", http.StatusBadRequest)
		return
	}

	fps, err := strconv.Atoi(fpsStr)
	if err != nil || fps < 1 || fps > 60 {
		s.sendError(w, "FPS must be between 1 and 60", http.StatusBadRequest)
		return
	}

	// Get optional parameters
	prompt := r.FormValue("prompt")
	negativePrompt := r.FormValue("negative_prompt")
	seed, _ := strconv.ParseInt(r.FormValue("seed"), 10, 64)
	cfgScaleStr := r.FormValue("cfg_scale")
	var cfgScale float64
	if cfgScaleStr != "" {
		cfgScale, _ = strconv.ParseFloat(cfgScaleStr, 64)
		if cfgScale < 0 || cfgScale > 1.0 {
			s.sendError(w, "CFG scale must be between 0.0 and 1.0", http.StatusBadRequest)
			return
		}
	}

	// Get resolution
	resolution := r.FormValue("resolution")
	var resolutionEnum client.VideoResolution
	switch resolution {
	case "512x512":
		resolutionEnum = client.VideoResolution512x512
	case "768x768":
		resolutionEnum = client.VideoResolution768x768
	case "1024x576":
		resolutionEnum = client.VideoResolution1024x576
	case "576x1024":
		resolutionEnum = client.VideoResolution576x1024
	default:
		// Default to 512x512 if not specified or invalid
		resolutionEnum = client.VideoResolution512x512
	}

	// Get output format
	outputFormat := r.FormValue("output_format")
	var outputFormatEnum client.VideoFormat
	switch outputFormat {
	case "mp4":
		outputFormatEnum = client.VideoFormatMP4
	case "gif":
		outputFormatEnum = client.VideoFormatGIF
	case "webm":
		outputFormatEnum = client.VideoFormatWEBM
	default:
		// Default to mp4 if not specified or invalid
		outputFormatEnum = client.VideoFormatMP4
	}

	// Create image-to-video request
	request := client.ImageToVideoRequest{
		Image:          imageData,
		Filename:       header.Filename,
		Motion:         motionEnum,
		Prompt:         prompt,
		NegativePrompt: negativePrompt,
		Seed:           seed,
		Duration:       duration,
		FPS:            fps,
		Resolution:     resolutionEnum,
		OutputFormat:   outputFormatEnum,
		CFGScale:       cfgScale,
		ReturnAsJSON:   true,
	}

	// Send request to Stability AI
	s.Logger.Info("Sending image-to-video request to Stability AI (motion: %s)", motion)
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second) // Longer timeout for video
	defer cancel()

	response, err := s.Client.ImageToVideo(ctx, request)
	if err != nil {
		s.Logger.Error("Error from Stability AI: %v", err)
		s.sendError(w, fmt.Sprintf("Error from Stability AI: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare response
	videoResp := VideoResponse{
		ID:      response.ID,
		Pending: true,
	}

	apiResp := Response{
		Success: true,
		Data:    videoResp,
	}

	// Convert response to JSON
	responseData, err := json.Marshal(apiResp)
	if err != nil {
		s.sendError(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}

	// Cache response if enabled
	if s.CachePath != "" {
		cachePath := filepath.Join(s.CachePath, cacheKey+".json")
		if err := os.WriteFile(cachePath, responseData, 0o644); err != nil {
			s.Logger.Error("Failed to write cache file: %v", err)
		} else {
			s.Logger.Info("Cached response at %s", cachePath)
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}

// handleVideoResult handles polling for image-to-video results
func (s *Server) handleVideoResult(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get video ID from URL
	id := filepath.Base(r.URL.Path)
	if id == "" {
		s.sendError(w, "Missing video ID", http.StatusBadRequest)
		return
	}

	// Poll for the result
	s.Logger.Info("Polling for image-to-video result (ID: %s)", id)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	result, finished, err := s.Client.PollVideoResult(ctx, id)
	if err != nil {
		s.Logger.Error("Error polling for image-to-video result: %v", err)
		s.sendError(w, fmt.Sprintf("Error polling for image-to-video result: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare response
	videoResp := VideoResponse{
		ID:      id,
		Pending: !finished,
	}

	// If the video generation is finished, include the video data
	if finished {
		videoResp.Video = "data:" + result.MimeType + ";base64," + encodeBase64(result.VideoData)
	}

	// Send response
	s.sendJSON(w, Response{
		Success: true,
		Data:    videoResp,
	})
}

// handleRoot serves the landing page with API documentation
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		s.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If requesting the root path exactly, serve HTML
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Stability AI Upscale API</title>
    <style>
        body {
            font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
        }
        h1, h2, h3 {
            margin-top: 30px;
            margin-bottom: 15px;
        }
        code {
            background-color: #f4f4f4;
            padding: 2px 5px;
            border-radius: 3px;
            font-family: monospace;
        }
        pre {
            background-color: #f4f4f4;
            padding: 15px;
            border-radius: 5px;
            overflow-x: auto;
        }
        .endpoint {
            margin-bottom: 30px;
            padding-bottom: 15px;
            border-bottom: 1px solid #eee;
        }
        .method {
            display: inline-block;
            padding: 3px 8px;
            border-radius: 3px;
            margin-right: 10px;
            font-weight: bold;
        }
        .get {
            background-color: #61affe;
            color: white;
        }
        .post {
            background-color: #49cc90;
            color: white;
        }
        .url {
            font-family: monospace;
            font-size: 1.1em;
        }
    </style>
</head>
<body>
    <h1>Stability AI SDK API</h1>
    <p>A REST API service for upscaling images and generating videos using Stability AI's APIs.</p>
    
    <h2>API Endpoints</h2>
    
    <div class="endpoint">
        <h4>
            <span class="method post">POST</span>
            <span class="url">/api/v1/upscale</span>
        </h4>
        <p>Upscale an image using Stability AI's upscaling API.</p>
        <p>Supports fast, conservative, and creative upscaling types.</p>
        <p>Required parameters:</p>
        <ul>
            <li><code>image</code>: The image file to upscale (multipart/form-data)</li>
        </ul>
        <p>Optional parameters:</p>
        <ul>
            <li><code>type</code>: Upscale type - "fast", "conservative", or "creative" (default: "fast")</li>
            <li><code>prompt</code>: Text prompt to guide upscaling (required for "conservative" and "creative" types)</li>
            <li><code>negative_prompt</code>: Negative prompt to guide upscaling</li>
            <li><code>seed</code>: Seed for consistent results</li>
            <li><code>creativity</code>: Creativity level (0.1-0.5)</li>
            <li><code>output_format</code>: Output format - "png", "jpeg", or "webp" (default: "png")</li>
            <li><code>style_preset</code>: Style preset for creative upscaling (e.g., "enhance", "anime", "photographic")</li>
        </ul>
    </div>
    
    <div class="endpoint">
        <h4>
            <span class="method get">GET</span>
            <span class="url">/api/v1/upscale/result/{id}</span>
        </h4>
        <p>Poll for the result of a creative upscale request.</p>
        <p>Replace <code>{id}</code> with the ID returned from a creative upscale request.</p>
    </div>
    
    <div class="endpoint">
        <h4>
            <span class="method post">POST</span>
            <span class="url">/api/v1/image-to-video</span>
        </h4>
        <p>Generate a video from an image using Stability AI's image-to-video API.</p>
        <p>Required parameters:</p>
        <ul>
            <li><code>image</code>: The image file to use as the base for video generation (multipart/form-data)</li>
            <li><code>motion</code>: The motion type to apply (e.g., "zoom", "pan", "rotate", "zoom_out", "pan_left", etc.)</li>
            <li><code>duration</code>: The duration of the video in seconds (0.5-8.0)</li>
            <li><code>fps</code>: Frames per second (1-60)</li>
        </ul>
        <p>Optional parameters:</p>
        <ul>
            <li><code>prompt</code>: Text prompt to guide video generation</li>
            <li><code>negative_prompt</code>: Negative prompt to guide video generation</li>
            <li><code>seed</code>: Seed for consistent results</li>
            <li><code>cfg_scale</code>: Creativity level (0.0-1.0)</li>
            <li><code>resolution</code>: Video resolution - "512x512", "768x768", "1024x576", or "576x1024" (default: "512x512")</li>
            <li><code>output_format</code>: Output format - "mp4", "gif", or "webm" (default: "mp4")</li>
        </ul>
    </div>
    
    <div class="endpoint">
        <h4>
            <span class="method get">GET</span>
            <span class="url">/api/v1/image-to-video/result/{id}</span>
        </h4>
        <p>Poll for the result of a video generation request.</p>
        <p>Replace <code>{id}</code> with the ID returned from an image-to-video request.</p>
    </div>
    
    <div class="endpoint">
        <h4>
            <span class="method get">GET</span>
            <span class="url">/health</span>
        </h4>
        <p>Check the health status of the API.</p>
    </div>
    
    <h2>Authentication & Security</h2>
    <p>This API implements multiple layers of security:</p>
    
    <h3>1. API Key Authentication</h3>
    <p>All API endpoints (except /health) require authentication using a bearer token. Include the client API key in the Authorization header:</p>
    <pre>Authorization: Bearer your_client_api_key</pre>
    <p>Note: This client API key is different from the Stability AI API key. The Stability AI key is kept secure on the server.</p>
    
    <h3>2. App ID Authentication</h3>
    <p>If configured, each request must include an approved App ID in the X-App-ID header:</p>
    <pre>X-App-ID: your_approved_app_id</pre>
    
    <h3>3. IP Address Filtering</h3>
    <p>The server can be configured to only accept requests from specific IP addresses.</p>
    
    <h3>4. Rate Limiting</h3>
    <p>The API implements rate limiting to prevent abuse.</p>
    
    <h2>Example Usage</h2>
    <p>Example of upscaling an image using the fast method:</p>
    <pre>curl -X POST https://stability-go.fly.dev/api/v1/upscale \
-H "Authorization: Bearer your_client_api_key" \
-H "X-App-ID: your_app_id" \
-F "image=@path/to/image.jpg" \
-F "type=fast"</pre>

    <p>Example of creative upscaling (returns an ID for polling):</p>
    <pre>curl -X POST https://stability-go.fly.dev/api/v1/upscale \
-H "Authorization: Bearer your_client_api_key" \
-H "X-App-ID: your_app_id" \
-F "image=@path/to/image.jpg" \
-F "type=creative" \
-F "prompt=high quality detailed fantasy landscape"</pre>

    <p>Polling for a creative upscale result:</p>
    <pre>curl -X GET https://stability-go.fly.dev/api/v1/upscale/result/your_id_here \
-H "Authorization: Bearer your_client_api_key" \
-H "X-App-ID: your_app_id"</pre>
    
    <h2>GitHub Repository</h2>
    <p><a href="https://github.com/marcusziade/stability-go" target="_blank">https://github.com/marcusziade/stability-go</a></p>
</body>
</html>`

		w.Write([]byte(html))
		return
	}

	// Otherwise return 404
	s.sendError(w, "Not found", http.StatusNotFound)
}

