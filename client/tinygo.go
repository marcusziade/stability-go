//go:build tinygo
// +build tinygo

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TinyGoClient is a client optimized for TinyGo WASM
type TinyGoClient struct {
	BaseURL string
	APIKey  string
}

// NewTinyGoClient creates a new client for TinyGo
func NewTinyGoClient(apiKey string) *TinyGoClient {
	return &TinyGoClient{
		BaseURL: DefaultBaseURL,
		APIKey:  apiKey,
	}
}

// WithBaseURL sets a custom base URL for the client
func (c *TinyGoClient) WithBaseURL(baseURL string) *TinyGoClient {
	c.BaseURL = baseURL
	return c
}

// Upscale upscales an image using the specified parameters
// Note: This is a simplified version for TinyGo that doesn't use http.Client
func (c *TinyGoClient) Upscale(ctx context.Context, request UpscaleRequest) (*UpscaleResponse, error) {
	// Create multipart form data
	var b bytes.Buffer
	w := NewMultipartWriter(&b)

	// Add the file
	if err := w.AddFile("image", request.Filename, request.Image); err != nil {
		return nil, fmt.Errorf("failed to add file: %w", err)
	}

	// Add fields
	w.AddField("engine", string(request.Model))

	if request.Factor > 0 {
		w.AddField("factor", fmt.Sprintf("%d", request.Factor))
	}

	if request.Width > 0 {
		w.AddField("width", fmt.Sprintf("%d", request.Width))
	}

	if request.Height > 0 {
		w.AddField("height", fmt.Sprintf("%d", request.Height))
	}

	if request.EnhanceDetail {
		w.AddField("enhance_detail", "true")
	}

	// Close the writer
	boundary := w.Boundary()
	w.Close()

	// Create URL
	url := fmt.Sprintf("%s%s", c.BaseURL, UpscalePath)

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	// Send request
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upscale API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Decode response
	var upscaleResp UpscaleResponse
	if err := json.NewDecoder(resp.Body).Decode(&upscaleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &upscaleResp, nil
}

// MultipartWriter is a simplified version of multipart.Writer for TinyGo
type MultipartWriter struct {
	w        io.Writer
	boundary string
}

// NewMultipartWriter creates a new MultipartWriter
func NewMultipartWriter(w io.Writer) *MultipartWriter {
	return &MultipartWriter{
		w:        w,
		boundary: fmt.Sprintf("------------------------%d", time.Now().UnixNano()),
	}
}

// Boundary returns the boundary
func (w *MultipartWriter) Boundary() string {
	return w.boundary
}

// AddField adds a form field
func (w *MultipartWriter) AddField(name, value string) error {
	h := fmt.Sprintf("\r\n--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n",
		w.boundary, name)
	if _, err := w.w.Write([]byte(h)); err != nil {
		return err
	}
	if _, err := w.w.Write([]byte(value)); err != nil {
		return err
	}
	return nil
}

// AddFile adds a file
func (w *MultipartWriter) AddFile(name, filename string, content []byte) error {
	h := fmt.Sprintf("\r\n--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\nContent-Type: application/octet-stream\r\n\r\n",
		w.boundary, name, filename)
	if _, err := w.w.Write([]byte(h)); err != nil {
		return err
	}
	if _, err := w.w.Write(content); err != nil {
		return err
	}
	return nil
}

// Close closes the writer
func (w *MultipartWriter) Close() error {
	_, err := w.w.Write([]byte(fmt.Sprintf("\r\n--%s--\r\n", w.boundary)))
	return err
}

