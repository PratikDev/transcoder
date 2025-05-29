# Use official Golang base image
FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Install ffmpeg and clean up
RUN apt-get update && \
  apt-get install -y ffmpeg && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the project files
COPY . .

# Build the Go app
RUN go build -o transcoder .

# Use a minimal image for running
FROM debian:bookworm-slim

WORKDIR /app

# Install ffmpeg runtime
RUN apt-get update && \
  apt-get install -y ffmpeg && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

# Copy the built binary from builder
COPY --from=builder /app/transcoder .

# Create necessary directories at runtime
RUN mkdir -p /app/uploads /app/output

# Expose the API port
EXPOSE 3000

# Run the Go app
CMD ["./transcoder"]