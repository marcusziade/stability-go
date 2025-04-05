# Image-to-Video Example

This example demonstrates how to use the Image-to-Video API from Stability AI directly through the Go SDK.

## Important Note About API Availability

The Image-to-Video API may be in private beta. You may receive a 404 (Not Found) error if your API key doesn't have access to this feature. Contact Stability AI support for access if needed.

## API Requirements

This example uses the v2beta endpoint for Image-to-Video generation. Note the following important requirements:

1. The API is asynchronous - meaning you'll receive an ID first, then need to poll for results
2. Only specific image dimensions are supported:
   - 1024×576 (16:9 landscape)
   - 576×1024 (9:16 portrait)
   - 768×768 (1:1 square)
3. The API requires a `motion_bucket_id` parameter (0-255) that controls motion intensity

## Usage

```bash
go run main.go \
  --api-key "your_stability_api_key" \
  --input "path/to/image.jpg" \
  --motion-bucket-id 127 \
  --cfg-scale 1.8 \
  --seed 0
```

This matches the curl usage:

```bash
curl -f -sS "https://api.stability.ai/v2beta/image-to-video" \
  -H "authorization: Bearer your_stability_api_key" \
  -F image=@"./image.png" \
  -F seed=0 \
  -F cfg_scale=1.8 \
  -F motion_bucket_id=127
```

## Command Line Options

- `--api-key`: Stability AI API key (required or use STABILITY_API_KEY environment variable)
- `--input`: Input image file path (required)
- `--output`: Output directory for the video (default: "output")
- `--motion-bucket-id`: Motion bucket ID controlling the amount of motion (default: 127, range: 0-255)
- `--cfg-scale`: Creativity/fidelity scale (default: 1.8)
- `--seed`: Seed value for consistent results (default: 0 for random seed)
- `--prompt`: Optional prompt to guide video generation
- `--negative-prompt`: Optional negative prompt
- `--duration`: Duration of the video in seconds (default: 2.0, range: 0.5-8.0)
- `--fps`: Frames per second (default: 30, range: 1-60)
- `--resolution`: Video resolution (default: "512x512")
  - Available options: "512x512", "768x768", "1024x576", "576x1024"
- `--format`: Output format (default: "mp4")
  - Available options: "mp4", "gif", "webm"
- `--motion`: Legacy motion type parameter (not used in new API version)
- `--proxy`: Use proxy (default: false)
- `--proxy-url`: Proxy URL (default: "your-proxy-server.com")

## Examples

### Basic Usage

```bash
go run main.go --api-key "your_stability_api_key" --input "my_image.png"
```

### Advanced Usage

```bash
go run main.go \
  --api-key "your_stability_api_key" \
  --input "my_image.png" \
  --motion-bucket-id 150 \
  --duration 4.0 \
  --fps 24 \
  --resolution 768x768 \
  --format mp4 \
  --prompt "Cinematic scene with dramatic lighting" \
  --negative-prompt "blurry, distorted" \
  --seed 12345 \
  --cfg-scale 1.8
```

## How It Works

1. The client sends the image file and parameters directly to Stability AI's Image-to-Video API v2beta endpoint.
2. The API initiates a video generation job and returns an ID.
3. The client polls for the result until the video is ready.
4. Once the video is ready, it's downloaded and saved to the output directory.
5. The SDK includes robust handling for different response formats to ensure successful video extraction.

## Robust Video Extraction

The SDK implements multiple strategies for video extraction:
1. Standard extraction from the base64-encoded `video` field in the API response
2. Direct extraction from JSON when the standard approach fails
3. Raw binary extraction when the response appears to be an MP4 file
4. Fallback to saving raw data when no other method works

## Notes

- Video generation can take several minutes depending on the settings.
- The client includes a progress indicator that shows the polling process.
- The resulting video is saved in the specified output directory with a name derived from the input image.
- The video ID is also saved to a text file in the output directory for reference.
- To use a proxy, enable the `--proxy` flag and configure the proxy URL with `--proxy-url`.
- If you receive a 404 error, it means your API key doesn't have access to the Image-to-Video API yet, as this feature may be in private beta.
- If you receive a 520 error, it means there's an issue with the Stability AI API servers. Try again later.

## Troubleshooting

If you encounter issues with video generation:
1. Check that your image dimensions match one of the supported formats (1024×576, 576×1024, or 768×768)
2. Verify that your API key has access to the Image-to-Video feature
3. Try decreasing the motion bucket ID if the generation fails
4. If the video isn't saving correctly, you can use the `videoutils.go` utilities to extract the video manually