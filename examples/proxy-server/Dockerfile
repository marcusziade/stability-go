FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /proxy-server ./examples/proxy-server

# Create a minimal container
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /proxy-server .

# Create cache directory
RUN mkdir -p /cache

# Expose port
EXPOSE 8080

# Set environment variables
ENV STABILITY_API_KEY=""
ENV CACHE_PATH="/cache"
ENV RATE_LIMIT="500ms"
ENV ALLOWED_HOSTS=""
ENV ADDR=":8080"

# Run the application
CMD ["sh", "-c", "./proxy-server -api-key=${STABILITY_API_KEY} -cache=${CACHE_PATH} -rate-limit=${RATE_LIMIT} -allowed-hosts=${ALLOWED_HOSTS} -addr=${ADDR}"]