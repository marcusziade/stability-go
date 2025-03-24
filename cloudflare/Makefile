.PHONY: build deploy dev clean

# Variables
WORKER_NAME = stability-go-proxy
WASM_FILE = worker.wasm
GO_FILE = worker.go
JS_FILE = worker.js

# Build the WASM module
build:
	@echo "Building WASM module..."
	tinygo build -o $(WASM_FILE) -target=wasm -no-debug $(GO_FILE)

# Deploy the worker
deploy: build
	@echo "Deploying to Cloudflare Workers..."
	wrangler deploy

# Run in development mode
dev: build
	@echo "Starting development server..."
	wrangler dev

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(WASM_FILE)

# Help message
help:
	@echo "Stability Go Cloudflare Worker"
	@echo ""
	@echo "Usage:"
	@echo "  make build   - Build the WASM module"
	@echo "  make deploy  - Deploy to Cloudflare Workers"
	@echo "  make dev     - Run in development mode"
	@echo "  make clean   - Clean build artifacts"
	@echo "  make help    - Show this help message"