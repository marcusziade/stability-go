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
	"os"
	"strconv"
	"strings"
	"time"
)

// Image-to-Video API endpoints
const (
	ImageToVideoPath   = "/v2beta/image-to-video"
	VideoResultPath    = "/v2beta/image-to-video/result"
)

// VideoMotion represents the motion available for image-to-video
type VideoMotion string

const (
	VideoMotionNone          VideoMotion = "none"
	VideoMotionZoom          VideoMotion = "zoom"
	VideoMotionPan           VideoMotion = "pan"
	VideoMotionTilt          VideoMotion = "tilt"
	VideoMotionRotate        VideoMotion = "rotate"
	VideoMotionZoomOut       VideoMotion = "zoom_out"
	VideoMotionPanLeft       VideoMotion = "pan_left"
	VideoMotionPanRight      VideoMotion = "pan_right"
	VideoMotionTiltUp        VideoMotion = "tilt_up"
	VideoMotionTiltDown      VideoMotion = "tilt_down"
	VideoMotionRotateLeft    VideoMotion = "rotate_left"
	VideoMotionRotateRight   VideoMotion = "rotate_right"
)

// VideoResolution represents the available video resolutions
type VideoResolution string

const (
	VideoResolution512x512   VideoResolution = "512x512"
	VideoResolution768x768   VideoResolution = "768x768"
	VideoResolution1024x576  VideoResolution = "1024x576"
	VideoResolution576x1024  VideoResolution = "576x1024"
)

// OutputFormat defines the available video output formats
type VideoFormat string

const (
	VideoFormatMP4  VideoFormat = "mp4"
	VideoFormatGIF  VideoFormat = "gif"
	VideoFormatWEBM VideoFormat = "webm"
)

// ImageToVideoRequest represents the parameters for an image-to-video request
type ImageToVideoRequest struct {
	// The image to use as the base for the video (binary data)
	Image []byte
	// The filename of the image
	Filename string
	// Motion bucket ID (controls the amount of motion)
	MotionBucketID int
	// The motion to apply to the image (legacy parameter)
	Motion VideoMotion
	// The optional prompt to guide video generation
	Prompt string
	// Optional negative prompt
	NegativePrompt string
	// Optional seed value (0 for random)
	Seed int64
	// Video duration in seconds (0.5-8.0)
	Duration float64
	// Frames per second (1-60)
	FPS int
	// Video resolution
	Resolution VideoResolution
	// Output format (mp4, gif, webm)
	OutputFormat VideoFormat
	// Creativity level (default 1.8)
	CFGScale float64
	// Whether to return video as base64 JSON instead of binary
	ReturnAsJSON bool
}

// ImageToVideoResponse represents the response from the image-to-video API
type ImageToVideoResponse struct {
	// For async operation, this will contain the ID for polling
	ID string
	// The video data (only filled when returnAsJSON is false and sync response)
	VideoData []byte
	// The mime type of the video (e.g., "video/mp4")
	MimeType string
}

// VideoAsyncResponse represents the ID returned by the image-to-video endpoint
type VideoAsyncResponse struct {
	// The ID to use for polling the result
	ID string `json:"id"`
}

// VideoResultResponse represents the final response from the video polling endpoint
type VideoResultResponse struct {
	// Whether the video generation is finished
	Finished bool `json:"finished"`
	// The base64 encoded video data (only present when finished is true)
	Video string `json:"video,omitempty"`
	// The video type (only present when finished is true)
	Type string `json:"mime_type,omitempty"`
	// Any error that occurred during processing
	Error string `json:"error,omitempty"`
	// Raw JSON data
	RawJSON []byte `json:"-"`
}

// ImageToVideo generates a video from an image using the specified parameters
func (c *Client) ImageToVideo(ctx context.Context, request ImageToVideoRequest) (*ImageToVideoResponse, error) {
	// Validate required parameters
	if len(request.Image) == 0 {
		return nil, fmt.Errorf("image is required")
	}

	if request.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	// Create form fields
	fields := map[string]string{}

	// Add motion bucket ID (new API parameter)
	if request.MotionBucketID > 0 {
		fields["motion_bucket_id"] = strconv.Itoa(request.MotionBucketID)
	} else {
		// Default to 127 if not specified
		fields["motion_bucket_id"] = "127"
	}
	
	// Legacy motion parameter for backward compatibility
	if request.Motion != "" {
		fields["motion"] = string(request.Motion)
	}
	
	// Add duration and FPS if specified
	if request.Duration > 0 {
		if request.Duration < 0.5 || request.Duration > 8.0 {
			return nil, fmt.Errorf("duration must be between 0.5 and 8.0 seconds")
		}
		fields["duration"] = strconv.FormatFloat(request.Duration, 'f', 2, 64)
	}

	if request.FPS > 0 {
		if request.FPS < 1 || request.FPS > 60 {
			return nil, fmt.Errorf("FPS must be between 1 and 60")
		}
		fields["fps"] = strconv.Itoa(request.FPS)
	}

	// Add optional fields
	if request.Prompt != "" {
		fields["prompt"] = request.Prompt
	}

	if request.NegativePrompt != "" {
		fields["negative_prompt"] = request.NegativePrompt
	}

	// Seed parameter (explicitly include 0 for random seeds)
	fields["seed"] = strconv.FormatInt(request.Seed, 10)

	if request.Resolution != "" {
		fields["resolution"] = string(request.Resolution)
	} else {
		// Default to 512x512 if not specified
		fields["resolution"] = string(VideoResolution512x512)
	}

	if request.OutputFormat != "" {
		fields["output_format"] = string(request.OutputFormat)
	} else {
		// Default to MP4 if not specified
		fields["output_format"] = string(VideoFormatMP4)
	}

	// Set cfg_scale (default is 1.8 if not specified)
	if request.CFGScale > 0 {
		fields["cfg_scale"] = strconv.FormatFloat(request.CFGScale, 'f', 2, 64)
	} else {
		// Default to 1.8 if not specified
		fields["cfg_scale"] = "1.8"
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
	url := c.BaseURL + ImageToVideoPath
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
		httpReq.Header.Set("Accept", "video/*")
	}

	// Send the request
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send image-to-video request: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			// Check for content policy violation (HTTP 403)
			if resp.StatusCode == http.StatusForbidden {
				if errorResp.Name == "content_policy_violation" ||
					errorResp.Name == "safety_violation" ||
					errorResp.Message == "Your request has been rejected as a result of our safety system." {
					return nil, fmt.Errorf("content policy violation: the image violates Stability AI's content policy - %s", errorResp.Message)
				}
				return nil, fmt.Errorf("forbidden: %s - %s", errorResp.Name, errorResp.Message)
			}
			return nil, fmt.Errorf("image-to-video API error (status %d): %s - %s", resp.StatusCode, errorResp.Name, errorResp.Message)
		}
		// Fallback for unparseable errors
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("content policy violation: the image appears to violate Stability AI's content policy")
		}
		return nil, fmt.Errorf("image-to-video API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Image-to-Video is an asynchronous operation, we get an ID for polling
	var videoResp VideoAsyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&videoResp); err != nil {
		return nil, fmt.Errorf("failed to decode image-to-video response: %w", err)
	}

	return &ImageToVideoResponse{
		ID: videoResp.ID,
	}, nil
}

// PollVideoResult polls for the result of an image-to-video job
func (c *Client) PollVideoResult(ctx context.Context, id string) (*ImageToVideoResponse, bool, error) {
	url := fmt.Sprintf("%s%s/%s", c.BaseURL, VideoResultPath, id)
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

	// Status 202 means the generation is still processing
	if resp.StatusCode == http.StatusAccepted {
		// For 202, we just return normally but with finished=false
		return nil, false, nil
	}
	
	// Handle other non-200 responses as errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			// Check for content policy violation (HTTP 403)
			if resp.StatusCode == http.StatusForbidden {
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

	// Read the raw response body for debugging
	body, _ := io.ReadAll(resp.Body)
	
	// Save the raw JSON for debugging
	fmt.Printf("Debug: Raw API response: %s\n", string(body))
	
	// Save the response to a debugging file
	debugFile := fmt.Sprintf("/tmp/stability_video_debug_%s.json", id)
	if err := os.WriteFile(debugFile, body, 0644); err != nil {
		fmt.Printf("Warning: Could not save debug file: %v\n", err)
	} else {
		fmt.Printf("Debug: Saved response to %s\n", debugFile)
	}

	// Try to extract the video using multiple strategies
	// 1. First try to extract it from the proper VideoResultResponse structure
	var resultResp VideoResultResponse
	if err := json.Unmarshal(body, &resultResp); err != nil {
		return nil, false, fmt.Errorf("failed to decode poll response: %w, body: %s", err, string(body))
	}
	
	// Store the raw JSON for later use
	resultResp.RawJSON = body

	// Check if there was an error during processing
	if resultResp.Error != "" {
		return nil, false, fmt.Errorf("video processing error: %s", resultResp.Error)
	}

	// If not finished yet, return with the finished flag set to false
	if !resultResp.Finished {
		return nil, false, nil
	}

	// Debug info about the video response
	fmt.Printf("Debug: Video base64 length: %d chars, MIME type: %s\n", len(resultResp.Video), resultResp.Type)
	
	// Variable to store the final video data
	var videoData []byte
	var extractionMethod string
	
	// 2. Try to extract the video from the standardized response structure
	if resultResp.Video != "" {
		// If the response starts with "data:" prefix, extract only the base64 part
		base64Data := resultResp.Video
		if strings.HasPrefix(base64Data, "data:") {
			parts := strings.Split(base64Data, ",")
			if len(parts) == 2 {
				base64Data = parts[1]
			}
		}
		
		// Remove any whitespace from the base64 string
		base64Data = strings.ReplaceAll(base64Data, " ", "")
		base64Data = strings.ReplaceAll(base64Data, "\n", "")
		base64Data = strings.ReplaceAll(base64Data, "\r", "")
		base64Data = strings.ReplaceAll(base64Data, "\t", "")
		
		// Save the base64 data to a file for manual debugging
		base64File := fmt.Sprintf("/tmp/video_base64_%s.txt", id)
		if err := os.WriteFile(base64File, []byte(base64Data), 0644); err != nil {
			fmt.Printf("Warning: Could not save base64 to file: %v\n", err)
		} else {
			fmt.Printf("Debug: Saved base64 data to %s\n", base64File)
		}
		
		// Decode the base64 video data
		data, err := base64.StdEncoding.DecodeString(base64Data)
		if err == nil {
			videoData = data
			extractionMethod = "standard base64 field"
			fmt.Printf("Debug: Successfully decoded video data using standard method, length: %d bytes\n", len(videoData))
		} else {
			fmt.Printf("Warning: Failed to decode video using standard method: %v\n", err)
		}
	}
	
	// 3. If standard extraction failed, try alternate approaches
	if videoData == nil || len(videoData) == 0 {
		// Try to parse the response as a generic JSON map
		var jsonData map[string]interface{}
		if err := json.Unmarshal(body, &jsonData); err == nil {
			// Check if we have a video field
			if video, ok := jsonData["video"]; ok {
				if videoStr, ok := video.(string); ok {
					fmt.Printf("Debug: Found video field in JSON response, length: %d\n", len(videoStr))
					videoData = []byte(videoStr)
					extractionMethod = "direct video field"
				}
			}
		}
	}
	
	// 4. If it still failed and the response looks like MP4 data, use it directly
	if videoData == nil || len(videoData) == 0 {
		// Check if it looks like an MP4 (should start with some magic bytes like "AAAAI" or contains "ftyp")
		if len(body) > 5 && (string(body[:5]) == "AAAAI" || strings.Contains(string(body[:100]), "ftyp")) {
			fmt.Println("Debug: Response appears to be raw MP4 format")
			videoData = body
			extractionMethod = "raw MP4 content"
		}
	}
	
	// If we still don't have video data, return an error
	if videoData == nil || len(videoData) == 0 {
		// Save the full raw JSON to a file in the output directory for analysis
		outputPath := fmt.Sprintf("/tmp/empty_video_response_%s.json", id)
		if err := os.WriteFile(outputPath, resultResp.RawJSON, 0644); err != nil {
			fmt.Printf("Warning: Could not save debug file: %v\n", err)
		}
		return nil, true, fmt.Errorf("could not extract video data using any available method")
	}
	
	// Save the raw video data to a separate file so we can verify it outside the app
	rawVideoFile := fmt.Sprintf("/tmp/video_raw_%s.mp4", id)
	if err := os.WriteFile(rawVideoFile, videoData, 0644); err != nil {
		fmt.Printf("Warning: Could not save raw video to file: %v\n", err)
	} else {
		fmt.Printf("Debug: Saved raw video data to %s (%d bytes)\n", rawVideoFile, len(videoData))
	}
	
	fmt.Printf("Debug: Successfully extracted video data using %s method, length: %d bytes\n", 
		extractionMethod, len(videoData))

	return &ImageToVideoResponse{
		VideoData: videoData,
		MimeType:  resultResp.Type,
	}, true, nil
}

// WaitForVideoResult waits for a video to be generated with a simple polling mechanism
func (c *Client) WaitForVideoResult(ctx context.Context, id string, interval time.Duration) (*ImageToVideoResponse, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			response, finished, err := c.PollVideoResult(ctx, id)
			if err != nil {
				return nil, err
			}

			if finished {
				return response, nil
			}
		}
	}
}