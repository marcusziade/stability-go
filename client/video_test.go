package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestVideoExtraction(t *testing.T) {
	// Test cases for different API response scenarios
	testCases := []struct {
		name         string
		responseFile string
		expectError  bool
	}{
		{
			name:         "Standard JSON response with video field",
			responseFile: "testdata/video_standard.json",
			expectError:  false,
		},
		{
			name:         "Direct video field in JSON",
			responseFile: "testdata/video_direct.json",
			expectError:  false,
		},
		{
			name:         "Raw MP4 response",
			responseFile: "testdata/video_raw.mp4",
			expectError:  false,
		},
		{
			name:         "Empty response",
			responseFile: "testdata/video_empty.json",
			expectError:  true,
		},
	}

	// Create test directory if it doesn't exist
	testDataDir := "testdata"
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		t.Fatalf("Failed to create test data directory: %v", err)
	}

	// Create sample test data
	// 1. Standard JSON response
	standardResp := VideoResultResponse{
		Finished: true,
		Video:    "AAAAIAAAA", // Sample base64-encoded video data
		Type:     "video/mp4",
	}
	standardJSON, _ := json.Marshal(standardResp)
	if err := os.WriteFile("testdata/video_standard.json", standardJSON, 0644); err != nil {
		t.Fatalf("Failed to create test data file: %v", err)
	}

	// 2. Direct video field in JSON
	directJSON := []byte(`{"video": "AAAAIAAAA", "finished": true, "mime_type": "video/mp4"}`)
	if err := os.WriteFile("testdata/video_direct.json", directJSON, 0644); err != nil {
		t.Fatalf("Failed to create test data file: %v", err)
	}

	// 3. Raw MP4 response
	rawVideo := []byte("AAAAIAAAAftypcFFFFmp4")
	if err := os.WriteFile("testdata/video_raw.mp4", rawVideo, 0644); err != nil {
		t.Fatalf("Failed to create test data file: %v", err)
	}

	// 4. Empty response
	emptyJSON := []byte(`{"finished": true}`)
	if err := os.WriteFile("testdata/video_empty.json", emptyJSON, 0644); err != nil {
		t.Fatalf("Failed to create test data file: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server to simulate API responses
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Return the predefined response
				data, err := os.ReadFile(tc.responseFile)
				if err != nil {
					t.Fatalf("Failed to read test data file: %v", err)
				}

				// Set appropriate content type based on the file
				if tc.responseFile == "testdata/video_raw.mp4" {
					w.Header().Set("Content-Type", "video/mp4")
				} else {
					w.Header().Set("Content-Type", "application/json")
				}

				w.WriteHeader(http.StatusOK)
				w.Write(data)
			}))
			defer server.Close()

			// Create a client that uses the test server
			client := &Client{
				BaseURL:    server.URL,
				APIKey:     "test-api-key",
				HTTPClient: http.DefaultClient,
			}

			// Test the extraction
			resp, finished, err := client.PollVideoResult(context.Background(), "test-id")

			// Check for expected errors
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			// Check for unexpected errors
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check that the response is finished
			if !finished {
				t.Errorf("Expected finished=true but got false")
			}

			// Check that we have video data
			if resp == nil {
				t.Errorf("Expected response but got nil")
				return
			}

			if len(resp.VideoData) == 0 {
				t.Errorf("Expected non-empty video data")
			}

			// Log successful extraction
			fmt.Printf("Successfully extracted %d bytes of video data\n", len(resp.VideoData))
		})
	}
}