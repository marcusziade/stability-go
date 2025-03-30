FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o stability-server ./cmd/server

# Create a minimal runtime image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/stability-server .

# Expose the port
EXPOSE 8080

# Set environment variables
ENV SERVER_ADDR=:8080
ENV LOG_LEVEL=info

# Run the application
CMD ["./stability-server"]