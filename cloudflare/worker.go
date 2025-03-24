package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/marcusziade/stability-go/client"
)

// Environment variables set by JavaScript
var STABILITY_API_KEY string

// Global vars
var (
	apiKey     string
	stClient   *client.Client
	cacheStore map[string]CacheEntry
)

// CacheEntry represents a cached response
type CacheEntry struct {
	Data       []byte
	Expiration time.Time
}

// HttpResponse is the response structure compatible with Cloudflare Workers
type HttpResponse struct {
	Body       []byte              `json:"body"`
	StatusCode int                 `json:"status"`
	Headers    map[string][]string `json:"headers"`
}

// HttpRequest is the request structure compatible with Cloudflare Workers
type HttpRequest struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

// Multipart form data boundary
const boundary = "-------------------------WebAssemblyBoundary"

// Memory management functions

//export alloc
func alloc(size uint32) *byte {
	buf := make([]byte, size)
	return &buf[0]
}

//export dealloc
func dealloc(ptr *byte, size uint32) {
	// In Go, memory is garbage collected, so this is a no-op
}

// main is the entry point for TinyGo WASM
func main() {
	// Initialize the client
	apiKey = STABILITY_API_KEY // Set by JS wrapper
	if apiKey == "" {
		fmt.Println("Warning: STABILITY_API_KEY not set")
	}
	stClient = client.NewTinyGoClient(apiKey)
	
	// Initialize cache
	cacheStore = make(map[string]CacheEntry)
}

// HandleRequest processes an incoming HTTP request
// This will be exported to JS
//export HandleRequest
func HandleRequest(reqPtr, reqLen, respPtr, respLen *uint32) {
	// Read the request data from memory
	reqData := make([]byte, *reqLen)
	for i := 0; i < int(*reqLen); i++ {
		reqData[i] = *(*byte)(unsafe.Pointer(uintptr(*reqPtr) + uintptr(i)))
	}
	
	// Parse the request
	var req HttpRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		writeErrorResponse(respPtr, respLen, "Failed to parse request", http.StatusBadRequest)
		return
	}
	
	// Get the path from the URL
	path := strings.Split(req.URL, "?")[0]
	
	// Route the request
	var resp HttpResponse
	switch {
	case path == "/v1/generation/upscale" && req.Method == "POST":
		resp = handleUpscale(req)
	case path == "/health" && req.Method == "GET":
		resp = handleHealth()
	default:
		resp = HttpResponse{
			StatusCode: http.StatusNotFound,
			Body:       []byte("Not Found"),
			Headers:    map[string][]string{"Content-Type": {"text/plain"}},
		}
	}
	
	// Write the response to memory
	respData, err := json.Marshal(resp)
	if err != nil {
		writeErrorResponse(respPtr, respLen, "Failed to serialize response", http.StatusInternalServerError)
		return
	}
	
	// Set the response length
	*respLen = uint32(len(respData))
	
	// Allocate memory for the response if needed
	if respPtr == nil || *respPtr == 0 {
		*respPtr = uint32(uintptr(unsafe.Pointer(&respData[0])))
	} else {
		// Copy the response to the allocated memory
		for i := 0; i < len(respData); i++ {
			*(*byte)(unsafe.Pointer(uintptr(*respPtr) + uintptr(i))) = respData[i]
		}
	}
}

// handleHealth handles health check requests
func handleHealth() HttpResponse {
	return HttpResponse{
		StatusCode: http.StatusOK,
		Body:       []byte("OK"),
		Headers:    map[string][]string{"Content-Type": {"text/plain"}},
	}
}

// handleUpscale handles upscale requests
func handleUpscale(req HttpRequest) HttpResponse {
	// Check if the request is multipart form data
	contentType := getHeader(req.Headers, "Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		return errorResponse("Expected multipart/form-data", http.StatusBadRequest)
	}
	
	// Parse multipart form data
	formValues, fileData, fileName, err := parseMultipartFormData(req.Body, contentType)
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to parse form data: %v", err), http.StatusBadRequest)
	}
	
	// Get form values
	engine := formValues["engine"]
	if engine == "" {
		return errorResponse("Engine is required", http.StatusBadRequest)
	}
	
	// Check cache
	cacheKey := fmt.Sprintf("%x-%s-%s", fileData, engine, formMapToString(formValues))
	if cacheEntry, ok := cacheStore[cacheKey]; ok {
		if cacheEntry.Expiration.After(time.Now()) {
			return HttpResponse{
				StatusCode: http.StatusOK,
				Body:       cacheEntry.Data,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
					"X-Cache":      {"HIT"},
				},
			}
		}
		// Cache expired, delete it
		delete(cacheStore, cacheKey)
	}
	
	// Create upscale request
	var model client.UpscaleModel
	switch engine {
	case "esrgan-v1-x2plus":
		model = client.UpscaleModelESRGAN
	case "stable-diffusion-x4-latent-upscaler":
		model = client.UpscaleModelStable
	case "realesrgan-16x":
		model = client.UpscaleModelRealESR
	default:
		return errorResponse("Invalid engine", http.StatusBadRequest)
	}
	
	factor, _ := strconv.Atoi(formValues["factor"])
	width, _ := strconv.Atoi(formValues["width"])
	height, _ := strconv.Atoi(formValues["height"])
	enhanceDetail := formValues["enhance_detail"] == "true"
	
	// Create request
	request := client.UpscaleRequest{
		Image:         fileData,
		Filename:      fileName,
		Model:         model,
		Factor:        factor,
		Width:         width,
		Height:        height,
		EnhanceDetail: enhanceDetail,
	}
	
	// Forward request to Stability AI
	ctx := context.Background()
	response, err := stClient.Upscale(ctx, request)
	if err != nil {
		return errorResponse(fmt.Sprintf("Error from Stability AI: %v", err), http.StatusInternalServerError)
	}
	
	// Convert response to JSON
	responseData, err := json.Marshal(response)
	if err != nil {
		return errorResponse("Failed to marshal response", http.StatusInternalServerError)
	}
	
	// Cache the response for 24 hours
	cacheStore[cacheKey] = CacheEntry{
		Data:       responseData,
		Expiration: time.Now().Add(24 * time.Hour),
	}
	
	return HttpResponse{
		StatusCode: http.StatusOK,
		Body:       responseData,
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
	}
}

// Helper functions

// errorResponse creates an error response
func errorResponse(message string, statusCode int) HttpResponse {
	return HttpResponse{
		StatusCode: statusCode,
		Body:       []byte(message),
		Headers:    map[string][]string{"Content-Type": {"text/plain"}},
	}
}

// writeErrorResponse writes an error response
func writeErrorResponse(respPtr, respLen *uint32, message string, statusCode int) {
	resp := HttpResponse{
		StatusCode: statusCode,
		Body:       []byte(message),
		Headers:    map[string][]string{"Content-Type": {"text/plain"}},
	}
	
	respData, _ := json.Marshal(resp)
	*respLen = uint32(len(respData))
	
	if respPtr == nil || *respPtr == 0 {
		*respPtr = uint32(uintptr(unsafe.Pointer(&respData[0])))
	} else {
		for i := 0; i < len(respData); i++ {
			*(*byte)(unsafe.Pointer(uintptr(*respPtr) + uintptr(i))) = respData[i]
		}
	}
}

// getHeader gets a header value
func getHeader(headers map[string][]string, key string) string {
	if values, ok := headers[key]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}

// parseMultipartFormData parses multipart form data
func parseMultipartFormData(body []byte, contentType string) (map[string]string, []byte, string, error) {
	// Extract boundary
	boundaryStart := strings.Index(contentType, "boundary=")
	if boundaryStart == -1 {
		return nil, nil, "", fmt.Errorf("no boundary found in Content-Type")
	}
	boundary := contentType[boundaryStart+9:]
	
	// Read form data
	formValues := make(map[string]string)
	var fileData []byte
	var fileName string
	
	parts := bytes.Split(body, []byte("--"+boundary))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		
		// Find headers and body
		headerEnd := bytes.Index(part, []byte("\r\n\r\n"))
		if headerEnd == -1 {
			continue
		}
		
		headers := part[:headerEnd]
		partBody := part[headerEnd+4:]
		
		// Check if it's a file or a form field
		isFile := bytes.Contains(headers, []byte("filename="))
		
		if isFile {
			// Extract filename
			filenameStart := bytes.Index(headers, []byte("filename="))
			if filenameStart != -1 {
				filenameEnd := bytes.IndexByte(headers[filenameStart+10:], '"')
				if filenameEnd != -1 {
					fileName = string(headers[filenameStart+10 : filenameStart+10+filenameEnd])
				}
			}
			
			// Store file data
			fileData = partBody[:len(partBody)-2] // Remove trailing \r\n
		} else {
			// Extract form field name
			nameStart := bytes.Index(headers, []byte("name="))
			if nameStart == -1 {
				continue
			}
			
			nameEnd := bytes.IndexByte(headers[nameStart+6:], '"')
			if nameEnd == -1 {
				continue
			}
			
			name := string(headers[nameStart+6 : nameStart+6+nameEnd])
			
			// Store form field value
			value := string(partBody[:len(partBody)-2]) // Remove trailing \r\n
			formValues[name] = value
		}
	}
	
	return formValues, fileData, fileName, nil
}

// formMapToString converts a form map to a string
func formMapToString(form map[string]string) string {
	var result strings.Builder
	for k, v := range form {
		if result.Len() > 0 {
			result.WriteString("&")
		}
		result.WriteString(k)
		result.WriteString("=")
		result.WriteString(v)
	}
	return result.String()
}