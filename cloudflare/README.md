# Stability AI Cloudflare Worker

A WebAssembly-based Cloudflare Worker for the Stability AI Upscale API, built with TinyGo.

## Features

- Full WebAssembly implementation compiled from Go code
- API-compatible proxy for the Stability AI Upscale API
- In-memory caching with optional Cloudflare KV integration
- Easy deployment with Wrangler
- Environment variable support for configuration
- Production-ready with error handling and security features

## Prerequisites

- [TinyGo](https://tinygo.org/getting-started/) (v0.30.0 or later)
- [Wrangler CLI](https://developers.cloudflare.com/workers/wrangler/install-and-update/) (v3.0.0 or later)
- Cloudflare account
- Stability AI API key

## Getting Started

1. Install TinyGo:

```bash
# On macOS
brew install tinygo

# On Ubuntu/Debian
wget https://github.com/tinygo-org/tinygo/releases/download/v0.30.0/tinygo_0.30.0_amd64.deb
sudo dpkg -i tinygo_0.30.0_amd64.deb
```

2. Install Wrangler:

```bash
npm install -g wrangler
```

3. Log in to Cloudflare:

```bash
wrangler login
```

4. Configure your API key in wrangler.toml:

```toml
[vars]
STABILITY_API_KEY = "your-api-key"
```

5. Build and deploy:

```bash
# Use the Makefile
make deploy

# Or manually
tinygo build -o worker.wasm -target=wasm -no-debug worker.go
wrangler deploy
```

## Development

For local development with hot reloading:

```bash
make dev
```

This will build the WASM module and start a local development server with Wrangler.

## Configuration

### Environment Variables

- `STABILITY_API_KEY`: Your Stability AI API key (required)

### Optional KV Namespace

For persistent caching, you can create a KV namespace:

```bash
wrangler kv:namespace create STABILITY_CACHE
```

Then uncomment the KV section in wrangler.toml:

```toml
[kv_namespaces]
binding = "STABILITY_CACHE"
```

## API Endpoints

### Upscale

Upscales an image using the Stability AI Upscale API.

**Endpoint**: `POST /v1/generation/upscale`

**Form Parameters**:

- `image` (required): The image file to upscale
- `engine` (required): The upscale model to use
  - `esrgan-v1-x2plus`
  - `stable-diffusion-x4-latent-upscaler`
  - `realesrgan-16x`
- `factor` (optional): The factor by which to upscale the image
- `width` (optional): The target width of the upscaled image
- `height` (optional): The target height of the upscaled image
- `enhance_detail` (optional): Whether to enhance the image detail

**Response**: Same as the original Stability AI API

### Health Check

Returns a simple health check response.

**Endpoint**: `GET /health`

**Response**: `OK` with status code 200

## TinyGo Compatibility

This project uses TinyGo to compile Go code to WebAssembly (WASM). TinyGo has limitations compared to the standard Go compiler:

- Limited standard library support
- No runtime reflection
- Simplified garbage collection
- Limited concurrency

The code has been carefully written to work within these limitations.

## Deployment Size Optimization

The resulting WASM binary is optimized for size to stay within Cloudflare Workers limits:

- TinyGo's `-no-debug` flag removes debug information
- Custom minimal JavaScript wrapper with lightweight imports
- In-memory caching with optional KV persistence
- No external dependencies in the Go code

## Security Considerations

- API keys are stored as environment variables, not hardcoded
- All requests are validated before processing
- Input validation for all parameters
- Proper error handling and logging
- Rate limiting is handled at the Cloudflare level

## Customization

You can customize the worker behavior by editing:

- `worker.go`: The Go implementation
- `worker.js`: The JavaScript wrapper
- `wrangler.toml`: Cloudflare configuration