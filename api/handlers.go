package api

import (
	"context"
	"crypto/sha256"
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
	Router      http.Handler
	Client      *client.Client
	Logger      *logger.Logger
	CachePath   string
	RateLimit   time.Duration
	APIKey      string
	AllowedHost []string
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

// New creates a new API server
func New(client *client.Client, logger *logger.Logger, cachePath string, rateLimit time.Duration, apiKey string, allowedHosts []string) *Server {
	s := &Server{
		Client:      client,
		Logger:      logger,
		CachePath:   cachePath,
		RateLimit:   rateLimit,
		APIKey:      apiKey,
		AllowedHost: allowedHosts,
	}

	// Create the router
	mux := http.NewServeMux()

	// Register routes with middleware
	mux.Handle("/api/v1/upscale", WithAuth(apiKey, nil)(http.HandlerFunc(s.handleUpscale)))
	mux.Handle("/api/v1/upscale/result/", WithAuth(apiKey, nil)(http.HandlerFunc(s.handleUpscaleResult)))
	mux.Handle("/health", http.HandlerFunc(s.handleHealthCheck))
	mux.Handle("/api/docs", http.HandlerFunc(s.handleDocs))

	// Apply global middleware
	s.Router = Chain(
		WithLogger(logger),
		WithCORS(nil), // Allow all origins
	)(mux)

	// Create cache directory if it doesn't exist
	if cachePath != "" {
		if err := os.MkdirAll(cachePath, 0755); err != nil {
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
		if err := os.WriteFile(cachePath, responseData, 0644); err != nil {
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
			"title":       "Stability AI Upscale API",
			"description": "API for upscaling images using Stability AI",
			"version":     "1.0.0",
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
	return hex.EncodeToString(data)
}