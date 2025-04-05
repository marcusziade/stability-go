# Stability Go Client

A production-ready Go client library and REST API server for the Stability AI API, with support for Upscale and Image-to-Video services. The library provides a clean, idiomatic Go interface to the Stability AI API with middleware support for rate limiting, retries, and proxying.

## Hosted API

The API is hosted and publicly available at:
[https://stability-go.fly.dev](https://stability-go.fly.dev)

Visit the hosted API for documentation and examples on how to interact with the service.

## Features

- Full support for Stability AI's upscale API endpoints
- Support for Stability AI's image-to-video API endpoints
- Middleware-based architecture for customizable request handling
- Rate limiting middleware to avoid API rate limit errors
- Retry middleware with exponential backoff and jitter
- Proxy middleware for routing requests through a proxy server
- Comprehensive error handling
- Concurrent-safe
- Easy-to-use interface with fluent API design

## Installation

```bash
go get github.com/marcusziade/stability-go
```

## Quick Start

### Upscaling Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/marcusziade/stability-go"
    "github.com/marcusziade/stability-go/client"
)

func main() {
    // Create a new client with your API key
    apiKey := os.Getenv("STABILITY_API_KEY")
    if apiKey == "" {
        fmt.Println("STABILITY_API_KEY environment variable is required")
        os.Exit(1)
    }

    // Create a simple client
    stClient := stability.New(apiKey)

    // Read image data
    imageData, err := os.ReadFile("input.jpg")
    if err != nil {
        fmt.Printf("Failed to read image: %v\n", err)
        os.Exit(1)
    }

    // Create upscale request
    request := client.UpscaleRequest{
        Image:        imageData,
        Filename:     "input.jpg",
        Type:         client.UpscaleTypeFast,
        OutputFormat: client.OutputFormatPNG,
    }

    // Make the request
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    response, err := stClient.Upscale(ctx, request)
    if err != nil {
        fmt.Printf("Failed to upscale image: %v\n", err)
        os.Exit(1)
    }

    // Save the upscaled image
    if err := os.WriteFile("output.png", response.ImageData, 0644); err != nil {
        fmt.Printf("Failed to save upscaled image: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("Successfully upscaled and saved image to output.png")
}
```

### Image-to-Video Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/marcusziade/stability-go"
    "github.com/marcusziade/stability-go/client"
)

func main() {
    // Create a new client with your API key
    apiKey := os.Getenv("STABILITY_API_KEY")
    if apiKey == "" {
        fmt.Println("STABILITY_API_KEY environment variable is required")
        os.Exit(1)
    }

    // Create a simple client
    stClient := stability.New(apiKey)

    // Read image data
    imageData, err := os.ReadFile("input.jpg")
    if err != nil {
        fmt.Printf("Failed to read image: %v\n", err)
        os.Exit(1)
    }

    // Create image-to-video request
    request := client.ImageToVideoRequest{
        Image:        imageData,
        Filename:     "input.jpg",
        Motion:       client.VideoMotionZoom,
        Duration:     3.0,
        FPS:          30,
        Resolution:   client.VideoResolution512x512,
        OutputFormat: client.VideoFormatMP4,
        Prompt:       "Cinematic scene with dramatic lighting",
        CFGScale:     0.5,
    }

    // Make the request (with longer timeout for video generation)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()

    response, err := stClient.ImageToVideo(ctx, request)
    if err != nil {
        fmt.Printf("Failed to initiate video generation: %v\n", err)
        os.Exit(1)
    }

    // Poll for the video result
    fmt.Println("Video generation initiated with ID:", response.ID)
    fmt.Println("Polling for result...")

    // Poll until the video is ready (or context times out)
    videoResult, err := stClient.WaitForVideoResult(ctx, response.ID, 5*time.Second)
    if err != nil {
        fmt.Printf("Error polling for video result: %v\n", err)
        os.Exit(1)
    }

    // Save the video
    if err := os.WriteFile("output.mp4", videoResult.VideoData, 0644); err != nil {
        fmt.Printf("Failed to save video: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("Successfully generated and saved video to output.mp4")
}
```

## Advanced Usage with Middleware

```go
// Create a client with middleware
stClient := stability.NewWithMiddleware(apiKey,
    stability.WithRateLimit(500*time.Millisecond),  // Wait at least 500ms between requests
    stability.WithRetry(3, 1*time.Second, 10*time.Second),  // Retry up to 3 times
    stability.WithProxy("your-proxy-server.com"),  // Route through proxy
)

// Use the client as normal
response, err := stClient.Upscale(ctx, request)
```

## Upscale Types

The library supports all of Stability AI's upscale types:

- `client.UpscaleTypeFast` - Quick upscaling with good quality
- `client.UpscaleTypeConservative` - Preserves details with guidance from a prompt
- `client.UpscaleTypeCreative` - Adds details based on the provided prompt

## Error Handling

The library provides detailed error information for API errors:

```go
response, err := stClient.Upscale(ctx, request)
if err != nil {
    // Check if it's a rate limit error
    if errors.IsRateLimitError(err) {
        fmt.Println("Rate limit exceeded, try again later")
    } 
    // Check if it's an authentication error
    else if errors.IsAuthError(err) {
        fmt.Println("Invalid API key")
    }
    // Check if it's a credit error
    else if errors.IsCreditError(err) {
        fmt.Println("Insufficient credits")
    } 
    // Other errors
    else {
        fmt.Printf("Error: %v\n", err)
    }
}
```

## Examples

See the `examples` directory for complete examples of using the library:

- `examples/upscale/main.go` - Basic upscaling example with various options
- `examples/image-to-video/main.go` - Example for generating videos from images
- `examples/middleware/main.go` - Example using middleware for logging, rate limiting, and retries
- `examples/mini-client/main.go` - Mini client for using the upscale API via HTTP
- `examples/mini-client-video/main.go` - Mini client for using the image-to-video API via HTTP
- `examples/proxy-server/main.go` - Example REST API server that proxies requests to Stability AI

## Using the REST API Server

This package includes a full-featured REST API server for the Stability AI Upscale API. You can run it directly from source, use the provided Docker image, or deploy to Fly.io.

### Deploying to Fly.io

The easiest way to deploy this API is using Fly.io. See the [DEPLOY.md](DEPLOY.md) file for detailed deployment instructions.

```bash
# Follow these steps:
fly auth login
fly launch --name stability-go
fly secrets set STABILITY_API_KEY=your_api_key_here
fly deploy
```

### Running with Docker

```bash
# Clone the repository
git clone https://github.com/marcusziade/stability-go.git
cd stability-go

# Set your Stability AI API key
export STABILITY_API_KEY="your-api-key-here"

# Run with Docker Compose
docker-compose up -d
```

### Running from Source

```bash
# Clone the repository
git clone https://github.com/marcusziade/stability-go.git
cd stability-go

# Build the server
go build -o stability-server ./cmd/server

# Set environment variables
export STABILITY_API_KEY="your-api-key-here"
export SERVER_ADDR=":8080"
export LOG_LEVEL="info"
export CACHE_PATH="./cache"

# Run the server
./stability-server
```

### API Endpoints

The REST API server provides the following endpoints:

- `GET /` - Landing page with API overview and documentation
- `POST /api/v1/upscale` - Upscale an image
- `GET /api/v1/upscale/result/{id}` - Get the result of a creative upscale
- `POST /api/v1/image-to-video` - Generate a video from an image
- `GET /api/v1/image-to-video/result/{id}` - Get the result of a video generation
- `GET /health` - Health check endpoint
- `GET /api/docs` - API documentation (OpenAPI format)

The hosted API is available at https://stability-go.fly.dev/. Visit the root URL for an interactive documentation page with examples and endpoint details.

### Securing Your Stability AI API Key

This API server is designed with multiple layers of security:

1. **Stability AI API Key**: Stored securely on the server and never exposed to clients
2. **Client API Key**: A separate key used by clients to authenticate with your API server
3. **IP Address Filtering**: Restrict access to specific IP addresses
4. **App ID Authentication**: Require a specific App ID for each of your applications
5. **Rate Limiting**: Protect against abuse with configurable rate limits

This multi-layer approach keeps your valuable Stability AI API key secure while still allowing your authorized clients to access the API functionality. You can configure each layer as needed:

- Set `CLIENT_API_KEY` for basic authentication
- Set `ALLOWED_IPS` to restrict access to specific IP addresses (comma-separated)
- Set `ALLOWED_APP_IDS` to authorize specific applications (comma-separated)

For example, in your native iOS app you would:
1. Include your client API key in your app (more secure than the Stability AI key)
2. Set a unique App ID in the `X-App-ID` header with each request
3. Optionally restrict API access to your server's IP address

This way, even if someone extracts your client API key, they would also need to:
- Have an allowed IP address
- Know your App ID
- Work within your rate limits

### Environment Variables

The server can be configured using the following environment variables:

| Name | Description | Default |
| ---- | ----------- | ------- |
| `STABILITY_API_KEY` | Your Stability AI API key (required) | - |
| `CLIENT_API_KEY` | API key for client authentication (auto-generated if not provided) | - |
| `SERVER_ADDR` | The address to listen on | `:8080` |
| `CACHE_PATH` | Directory to cache responses (empty to disable) | - |
| `RATE_LIMIT` | Rate limit between requests (e.g., `500ms`) | `500ms` |
| `ALLOWED_HOSTS` | Comma-separated list of allowed hosts | - |
| `ALLOWED_IPS` | Comma-separated list of allowed IP addresses | - |
| `ALLOWED_APP_IDS` | Comma-separated list of allowed application IDs | - |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `STABILITY_BASE_URL` | Custom base URL for Stability API | - |

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This library is licensed under the MIT License. See the LICENSE file for details.