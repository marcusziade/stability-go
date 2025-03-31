# Mini Client Example

This is a simple client example to test the upscale API endpoint directly.

## Setup

1. Create a `.env` file with your credentials:

```
UPSCALER_API_URL=https://stability-go.fly.dev/api/v1/upscale
UPSCALER_API_KEY=your-api-key
UPSCALER_APP_ID=your-app-id
```

2. Install the required dependency:

```
go get github.com/joho/godotenv
```

## Usage

```
go run main.go /path/to/your/image.jpg
```

The upscaled image will be saved to the `output` directory.

## Authentication Notes

The client sends authentication using these headers:
- `Authorization: Bearer <your-api-key>` - The API key used for authentication
- `X-App-ID: <your-app-id>` - The App ID for additional authentication
