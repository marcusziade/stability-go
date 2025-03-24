# Stability AI Proxy Server

This is a production-ready proxy server for the Stability AI Upscale API. It provides the following features:

- API-compatible proxy for the Stability AI Upscale API
- Rate limiting to avoid hitting Stability AI's rate limits
- Response caching to reduce API costs
- IP filtering to restrict access to specific hosts
- Simple health check endpoint
- Docker support for easy deployment

## Running the Server

### Using Docker Compose

1. Create a `.env` file with your Stability API key:

```
STABILITY_API_KEY=your-api-key
```

2. Start the server:

```bash
docker-compose up -d
```

### Manually

1. Build and run the server:

```bash
go build -o proxy-server
./proxy-server -api-key=your-api-key -addr=:8080
```

## Configuration

The server supports the following configuration options:

| Flag | Environment Variable | Description | Default |
|------|---------------------|-------------|---------|
| `-api-key` | `STABILITY_API_KEY` | Stability API key | (required) |
| `-addr` | `ADDR` | Address to listen on | `:8080` |
| `-cache` | `CACHE_PATH` | Cache directory | (disabled) |
| `-rate-limit` | `RATE_LIMIT` | Rate limit between requests | `500ms` |
| `-allowed-hosts` | `ALLOWED_HOSTS` | Comma-separated list of allowed hosts | (all allowed) |

## API Endpoints

### Upscale

Upscales an image using the Stability AI Upscale API. Endpoint is 100% compatible with the official API.

**Endpoint**: `POST /v1/generation/upscale`

**Form Parameters**:

- `image` (required): The image file to upscale
- `engine` (required): The upscale model to use (esrgan-v1-x2plus, stable-diffusion-x4-latent-upscaler, realesrgan-16x)
- `factor` (optional): The factor by which to upscale the image
- `width` (optional): The target width of the upscaled image
- `height` (optional): The target height of the upscaled image
- `enhance_detail` (optional): Whether to enhance the image detail (`true` or `false`)

**Response**: Same as the original Stability AI API

### Health Check

Returns a simple health check response.

**Endpoint**: `GET /health`

**Response**: `OK` with status code 200

## Caching

If caching is enabled, the server will cache responses based on the image content and request parameters. Cached responses are stored in the specified cache directory as JSON files.

To enable caching, provide a cache directory with the `-cache` flag.

## Rate Limiting

The server includes built-in rate limiting to avoid hitting Stability AI's rate limits. By default, requests are limited to one every 500ms.

To adjust the rate limit, use the `-rate-limit` flag with a Go duration (e.g., `500ms`, `1s`, etc.).

## Docker Support

The included Dockerfile and docker-compose.yml files provide easy deployment options.

### Building the Docker Image

```bash
docker build -t stability-proxy -f examples/proxy-server/Dockerfile .
```

### Running the Docker Container

```bash
docker run -p 8080:8080 -e STABILITY_API_KEY=your-api-key stability-proxy
```

## Client Usage

To use the proxy with the stability-go client:

```go
import (
    "github.com/marcusziade/stability-go"
)

// Create a client with the proxy URL
client := stability.New("your-api-key").WithBaseURL("http://your-proxy:8080")

// Use the client as normal
response, err := client.Upscale(ctx, request)
```