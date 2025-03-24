package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/webp"
)

// ReadImageFile reads an image file and returns its contents as a byte slice
func ReadImageFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}

	return data, nil
}

// ExtractFilename extracts the filename from a file path
func ExtractFilename(filePath string) string {
	return filepath.Base(filePath)
}

// Base64ToImage converts a base64 encoded image to a byte array
func Base64ToImage(base64Data, mimeType string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 image: %w", err)
	}
	return decoded, nil
}

// SaveImage saves an image to a file, determining the file format based on mime type
func SaveImage(imageData []byte, mimeType, outputPath string) error {
	// Decode the image
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// If mime type is specified, use it to determine format
	if mimeType != "" {
		format = mimeTypeToFormat(mimeType)
	}

	// If file extension is specified, use it to determine format
	if ext := strings.ToLower(filepath.Ext(outputPath)); ext != "" {
		ext = strings.TrimPrefix(ext, ".")
		if ext == "jpg" || ext == "jpeg" || ext == "png" || ext == "webp" {
			format = ext
		}
	}

	// Encode the image in the correct format
	switch format {
	case "jpeg", "jpg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
	case "png":
		return png.Encode(file, img)
	default:
		return fmt.Errorf("unsupported image format: %s", format)
	}
}

// mimeTypeToFormat converts a mime type to an image format string
func mimeTypeToFormat(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return "jpeg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return ""
	}
}