package errors

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError represents an error returned by the Stability API
type APIError struct {
	StatusCode int               `json:"-"`
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("stability API error (status %d): %s - %s", e.StatusCode, e.Name, e.Message)
}

// ParseAPIError attempts to parse an API error from an HTTP response
func ParseAPIError(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read error response: %w", err)
	}

	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err != nil {
		// If we can't parse the JSON, return a generic error with the response body
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	apiErr.StatusCode = resp.StatusCode
	return &apiErr
}

// IsRateLimitError checks if the error is a rate limit error
func IsRateLimitError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusTooManyRequests
	}
	return false
}

// IsAuthError checks if the error is an authentication error
func IsAuthError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// IsCreditError checks if the error is due to insufficient credits
func IsCreditError(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.StatusCode == http.StatusPaymentRequired || 
			apiErr.Name == "insufficient_credits" ||
			apiErr.Name == "payment_required"
	}
	return false
}