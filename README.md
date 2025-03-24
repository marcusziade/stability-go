# Stability Go Client

A production-ready Go client library for the Stability AI API with focus on the Upscale services. The library provides a clean, idiomatic Go interface to the Stability AI API with middleware support for rate limiting, retries, and proxying.

## Features

- Full support for Stability AI's upscale API endpoints
- Middleware-based architecture for customizable request handling
- Rate limiting middleware to avoid API rate limit errors
- Retry middleware with exponential backoff and jitter
- Proxy middleware for routing requests through a proxy server
- WebAssembly (WASM) support for Cloudflare Workers
- Comprehensive error handling
- Concurrent-safe
- Easy-to-use interface with fluent API design

## Installation

```bash
go get github.com/marcus/stability-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/marcus/stability-go"
    "github.com/marcus/stability-go/client"
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
        Image:    imageData,
        Filename: "input.jpg",
        Model:    client.UpscaleModelESRGAN,
        Factor:   2,
    }

    // Make the request
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    response, err := stClient.Upscale(ctx, request)
    if err != nil {
        fmt.Printf("Failed to upscale image: %v\n", err)
        os.Exit(1)
    }

    // Process the response (first artifact)
    if len(response.Artifacts) > 0 {
        artifact := response.Artifacts[0]
        // ... handle the upscaled image (e.g., save to disk)
        fmt.Println("Successfully upscaled image")
    }
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

## Available Upscale Models

The library supports all of Stability AI's upscale models:

- `client.UpscaleModelESRGAN` (esrgan-v1-x2plus) - Standard upscaler
- `client.UpscaleModelStable` (stable-diffusion-x4-latent-upscaler) - Better for Stable Diffusion generated images
- `client.UpscaleModelRealESR` (realesrgan-16x) - High quality upscaler

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

## WebAssembly (WASM) and Cloudflare Workers

The library includes support for WebAssembly compilation with TinyGo, which allows you to run the code in environments like Cloudflare Workers.

### Quick Start with Cloudflare Workers

```bash
# Navigate to the cloudflare directory
cd cloudflare

# Build the WASM binary
make build

# Deploy to Cloudflare Workers
make deploy
```

See the [Cloudflare Worker README](cloudflare/README.md) for more details.

### Benefits of Using WASM on Cloudflare Workers

- **API Key Security**: Hide your Stability API key behind the Cloudflare Worker
- **Custom Rate Limiting**: Control access to the API from a centralized location
- **Caching**: Reduce costs by caching responses
- **Global Distribution**: Take advantage of Cloudflare's global network
- **Zero Infrastructure**: Serverless deployment with no infrastructure to manage

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This library is licensed under the MIT License. See the LICENSE file for details.