package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "https://api.stability.ai"
	DefaultTimeout = 30 * time.Second
)

// Client represents a Stability AI API client
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new Stability AI client with the given API key
func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

// WithBaseURL sets a custom base URL for the client
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.BaseURL = baseURL
	return c
}

// WithHTTPClient sets a custom HTTP client
func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	c.HTTPClient = httpClient
	return c
}

// request sends an HTTP request and returns the response
func (c *Client) request(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.HTTPClient.Do(req)
}

// createMultipartRequest creates a multipart request with a file
func (c *Client) createMultipartRequest(ctx context.Context, path string, fileName string, fileData []byte, fields map[string]string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Add the file
	fw, err := w.CreateFormFile("image", fileName)
	if err != nil {
		return nil, err
	}
	if _, err = fw.Write(fileData); err != nil {
		return nil, err
	}

	// Add fields
	for key, value := range fields {
		if err := w.WriteField(key, value); err != nil {
			return nil, err
		}
	}

	if err = w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	return c.HTTPClient.Do(req)
}

