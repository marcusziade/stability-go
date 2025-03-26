package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
)

// Upscale API endpoints
const (
	UpscaleConservativePath = "/v2beta/stable-image/upscale/conservative"
	UpscaleCreativePath     = "/v2beta/stable-image/upscale/creative"
	UpscaleFastPath         = "/v2beta/stable-image/upscale/fast"
	CreativeResultPath      = "/v2beta/stable-image/upscale/result" // For polling creative results
)

// UpscaleType represents the available upscaling methods
type UpscaleType string

const (
	UpscaleTypeConservative UpscaleType = "conservative"
	UpscaleTypeCreative     UpscaleType = "creative"
	UpscaleTypeFast         UpscaleType = "fast"
)

// OutputFormat defines the available output image formats
type OutputFormat string

const (
	OutputFormatJPEG OutputFormat = "jpeg"
	OutputFormatPNG  OutputFormat = "png"
	OutputFormatWEBP OutputFormat = "webp"
)

// StylePreset defines the available style presets for creative upscaling
type StylePreset string

const (
	StylePreset3DModel          StylePreset = "3d-model"
	StylePresetAnalogFilm       StylePreset = "analog-film"
	StylePresetAnime            StylePreset = "anime"
	StylePresetCinematic        StylePreset = "cinematic"
	StylePresetComicBook        StylePreset = "comic-book"
	StylePresetDigitalArt       StylePreset = "digital-art"
	StylePresetEnhance          StylePreset = "enhance"
	StylePresetFantasyArt       StylePreset = "fantasy-art"
	StylePresetIsometric        StylePreset = "isometric"
	StylePresetLineArt          StylePreset = "line-art"
	StylePresetLowPoly          StylePreset = "low-poly"
	StylePresetModelingCompound StylePreset = "modeling-compound"
	StylePresetNeonPunk         StylePreset = "neon-punk"
	StylePresetOrigami          StylePreset = "origami"
	StylePresetPhotographic     StylePreset = "photographic"
	StylePresetPixelArt         StylePreset = "pixel-art"
	StylePresetTileTexture      StylePreset = "tile-texture"
)

// UpscaleRequest represents the parameters for an image upscale request
type UpscaleRequest struct {
	// The image to upscale (binary data)
	Image []byte
	// The filename of the image
	Filename string
	// The upscale type to use
	Type UpscaleType
	// The prompt to guide the upscale (required for conservative and creative)
	Prompt string
	// Optional negative prompt
	NegativePrompt string
	// Optional seed value
	Seed int64
	// Output format (jpeg, png, webp)
	OutputFormat OutputFormat
	// Creativity level (0.1-0.5 for creative, 0.2-0.5 for conservative)
	Creativity float64
	// Style preset (only for creative upscale)
	StylePreset StylePreset
	// Whether to return image as base64 JSON instead of binary
	ReturnAsJSON bool
}

// UpscaleResponse represents the response from the upscale API for fast and conservative modes
type UpscaleResponse struct {
	// The image data, either binary or base64 encoded depending on the accept header
	ImageData []byte
	// The mime type of the image (e.g., "image/png")
	MimeType string
	// For creative upscale, this will contain the ID for polling
	CreativeID string
}

// CreativeAsyncResponse represents the ID returned by the creative upscale endpoint
type CreativeAsyncResponse struct {
	// The ID to use for polling the result
	ID string `json:"id"`
}

// UpscaleResultResponse represents the final response from the creative upscale polling endpoint
type UpscaleResultResponse struct {
	// Whether the upscale is finished
	Finished bool `json:"finished"`
	// The base64 encoded image data (only present when finished is true)
	Image string `json:"image,omitempty"`
	// The image type (only present when finished is true)
	Type string `json:"mime_type,omitempty"`
	// Any error that occurred during processing
	Error string `json:"error,omitempty"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Message string `json:"message"`
	Errors  []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// Upscale upscales an image using the specified parameters
func (c *Client) Upscale(ctx context.Context, request UpscaleRequest) (*UpscaleResponse, error) {
	var endpoint string

	// Determine the endpoint based on upscale type
	switch request.Type {
	case UpscaleTypeConservative:
		endpoint = UpscaleConservativePath
	case UpscaleTypeCreative:
		endpoint = UpscaleCreativePath
	case UpscaleTypeFast:
		endpoint = UpscaleFastPath
	default:
		return nil, fmt.Errorf("invalid upscale type: %s", request.Type)
	}

	// Create form fields based on the upscale type
	fields := map[string]string{}

	// Add common fields
	if request.OutputFormat != "" {
		fields["output_format"] = string(request.OutputFormat)
	}

	// Add type-specific fields
	if request.Type != UpscaleTypeFast {
		// Conservative and Creative require prompt
		if request.Prompt == "" {
			return nil, fmt.Errorf("prompt is required for %s upscale", request.Type)
		}
		fields["prompt"] = request.Prompt

		if request.NegativePrompt != "" {
			fields["negative_prompt"] = request.NegativePrompt
		}

		if request.Seed > 0 {
			fields["seed"] = strconv.FormatInt(request.Seed, 10)
		}

		if request.Creativity > 0 {
			// Validate creativity range
			if request.Type == UpscaleTypeConservative && (request.Creativity < 0.2 || request.Creativity > 0.5) {
				return nil, fmt.Errorf("creativity for conservative upscale must be between 0.2 and 0.5")
			} else if request.Type == UpscaleTypeCreative && (request.Creativity < 0.1 || request.Creativity > 0.5) {
				return nil, fmt.Errorf("creativity for creative upscale must be between 0.1 and 0.5")
			}

			fields["creativity"] = strconv.FormatFloat(request.Creativity, 'f', 2, 64)
		}

		// Style preset is only for creative
		if request.Type == UpscaleTypeCreative && request.StylePreset != "" {
			fields["style_preset"] = string(request.StylePreset)
		}
	}

	// Create multipart request body
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Add the file part
	part, err := writer.CreateFormFile("image", request.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(request.Image); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	// Add other form fields
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("failed to write form field %s: %w", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create the HTTP request
	url := c.BaseURL + endpoint
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	// Set the accept header based on whether we want JSON or binary response
	if request.ReturnAsJSON {
		httpReq.Header.Set("Accept", "application/json")
	} else {
		httpReq.Header.Set("Accept", "image/*")
	}

	// Send the request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send upscale request: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			// Check for content policy violation (HTTP 403)
			if resp.StatusCode == http.StatusForbidden {
				// Look for specific content policy error patterns
				if errorResp.Name == "content_policy_violation" ||
					errorResp.Name == "safety_violation" ||
					errorResp.Message == "Your request has been rejected as a result of our safety system." {
					return nil, fmt.Errorf("content policy violation: the image violates Stability AI's content policy - %s", errorResp.Message)
				}
				return nil, fmt.Errorf("forbidden: %s - %s", errorResp.Name, errorResp.Message)
			}
			return nil, fmt.Errorf("upscale API error (status %d): %s - %s", resp.StatusCode, errorResp.Name, errorResp.Message)
		}
		// Fallback for unparseable errors
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("content policy violation: the image appears to violate Stability AI's content policy")
		}
		return nil, fmt.Errorf("upscale API error (status %d): %s", resp.StatusCode, string(body))
	}

	// For Creative upscale, we get an ID for polling
	if request.Type == UpscaleTypeCreative {
		var creativeResp CreativeAsyncResponse
		if err := json.NewDecoder(resp.Body).Decode(&creativeResp); err != nil {
			return nil, fmt.Errorf("failed to decode creative upscale response: %w", err)
		}
		return &UpscaleResponse{
			CreativeID: creativeResp.ID,
		}, nil
	}

	// For Conservative and Fast upscale, we get the image directly
	// Add a buffer size limit to prevent excessive memory usage
	bodyData, err := io.ReadAll(io.LimitReader(resp.Body, 100*1024*1024)) // 100MB limit
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// If no data was received but no error, it might be a silent failure or content policy violation
	if len(bodyData) == 0 {
		return nil, fmt.Errorf("no data received in response; this may indicate a content policy violation")
	}

	return &UpscaleResponse{
		ImageData: bodyData,
		MimeType:  resp.Header.Get("Content-Type"),
	}, nil
}

// PollCreativeResult polls for the result of a creative upscale job
func (c *Client) PollCreativeResult(ctx context.Context, id string) (*UpscaleResponse, bool, error) {
	url := fmt.Sprintf("%s%s/%s", c.BaseURL, CreativeResultPath, id)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create poll request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to send poll request: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			// Check for content policy violation (HTTP 403)
			if resp.StatusCode == http.StatusForbidden {
				// Look for specific content policy error patterns
				if errorResp.Name == "content_policy_violation" ||
					errorResp.Name == "safety_violation" ||
					errorResp.Message == "Your request has been rejected as a result of our safety system." {
					return nil, false, fmt.Errorf("content policy violation: the image violates Stability AI's content policy - %s", errorResp.Message)
				}
				return nil, false, fmt.Errorf("forbidden: %s - %s", errorResp.Name, errorResp.Message)
			}
			return nil, false, fmt.Errorf("poll API error (status %d): %s - %s", resp.StatusCode, errorResp.Name, errorResp.Message)
		}
		// Fallback for unparseable errors
		if resp.StatusCode == http.StatusForbidden {
			return nil, false, fmt.Errorf("content policy violation: the image appears to violate Stability AI's content policy")
		}
		return nil, false, fmt.Errorf("poll API error (status %d): %s", resp.StatusCode, string(body))
	}

	var resultResp UpscaleResultResponse
	if err := json.NewDecoder(resp.Body).Decode(&resultResp); err != nil {
		return nil, false, fmt.Errorf("failed to decode poll response: %w", err)
	}

	// Check if there was an error during processing
	if resultResp.Error != "" {
		return nil, false, fmt.Errorf("upscale processing error: %s", resultResp.Error)
	}

	// If not finished yet, return with the finished flag set to false
	if !resultResp.Finished {
		return nil, false, nil
	}

	// Decode the base64 image data
	imageData, err := base64.StdEncoding.DecodeString(resultResp.Image)
	if err != nil {
		return nil, true, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	return &UpscaleResponse{
		ImageData: imageData,
		MimeType:  resultResp.Type,
	}, true, nil
}
