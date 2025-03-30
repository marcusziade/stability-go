.PHONY: build run clean test docker-build docker-run all

# Variables
BIN_NAME=stability-server
MAIN_PATH=./cmd/server
DOCKER_IMAGE=stability-go-api

# Build the binary
build:
	go build -o $(BIN_NAME) $(MAIN_PATH)

# Run the server locally
run: build
	./$(BIN_NAME)

# Clean up
clean:
	rm -f $(BIN_NAME)
	go clean

# Run tests
test:
	go test -v ./...

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE) .

# Run Docker container
docker-run: docker-build
	docker run --rm -p 8080:8080 \
		-e STABILITY_API_KEY=$(STABILITY_API_KEY) \
		-e SERVER_ADDR=:8080 \
		-e LOG_LEVEL=info \
		$(DOCKER_IMAGE)

# Build and run with Docker Compose
docker-compose:
	docker-compose up -d

# Default target
all: build test