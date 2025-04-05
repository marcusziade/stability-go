# Image-to-Video Mini Client Example

This is a small client application that demonstrates how to use the Image-to-Video API endpoint of the Stability AI SDK Server.

## Usage

```bash
go run main.go \
  --server "http://localhost:8080" \
  --api-key "your_api_key" \
  --input "path/to/image.jpg" \
  --motion zoom \
  --duration 3.0 \
  --fps 30 \
  --resolution 512x512 \
  --format mp4
```

## Command Line Options

- `--server`: The URL of the server (default: http://localhost:8080)
- `--api-key`: API key for the server (required)
- `--app-id`: App ID for authentication (if required)
- `--input`: Input image file path (required)
- `--output`: Output directory for the video (default: "output")
- `--motion`: Motion type to apply to the video (default: "zoom")
  - Available options: "none", "zoom", "pan", "tilt", "rotate", "zoom_out", "pan_left", "pan_right", "tilt_up", "tilt_down", "rotate_left", "rotate_right"
- `--duration`: Duration of the video in seconds (default: 2.0, range: 0.5-8.0)
- `--fps`: Frames per second (default: 30, range: 1-60)
- `--prompt`: Optional prompt to guide video generation
- `--negative-prompt`: Optional negative prompt
- `--seed`: Optional seed for consistent results
- `--cfg-scale`: Creativity level (default: 0.5, range: 0.0-1.0)
- `--resolution`: Video resolution (default: "512x512")
  - Available options: "512x512", "768x768", "1024x576", "576x1024"
- `--format`: Output format (default: "mp4")
  - Available options: "mp4", "gif", "webm"

## Examples

### Basic Usage

```bash
go run main.go --server "http://localhost:8080" --api-key "your_api_key" --input "my_image.png"
```

### Advanced Usage

```bash
go run main.go \
  --server "http://localhost:8080" \
  --api-key "your_api_key" \
  --input "my_image.png" \
  --motion zoom_out \
  --duration 4.0 \
  --fps 24 \
  --resolution 768x768 \
  --format mp4 \
  --prompt "Cinematic scene with dramatic lighting" \
  --negative-prompt "blurry, distorted" \
  --seed 12345 \
  --cfg-scale 0.7
```

## How It Works

1. The client sends the image file and parameters to the server's `/api/v1/image-to-video` endpoint.
2. The server processes the request and returns an ID for the video generation job.
3. The client polls the `/api/v1/image-to-video/result/{id}` endpoint to check if the video is ready.
4. Once the video is ready, it's downloaded and saved to the output directory.

## Notes

- Video generation can take several minutes depending on the settings.
- The client includes a progress indicator that shows the polling process.
- The resulting video is saved in the specified output directory with a name derived from the input image.
- If the server is configured with App ID authentication, use the `--app-id` flag to provide it.