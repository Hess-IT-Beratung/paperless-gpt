# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Install necessary packages
RUN apk add --no-cache \
    git \
    gcc \
    musl-dev

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Ensure go.mod is tidy
RUN go mod tidy

# Build the Go binary with the musl build tag
RUN go build -tags musl -o paperless-gpt ./cmd/paperless-gpt

# Stage 2: Create a lightweight image with the Go binary and frontend
FROM alpine:3.18

# Install necessary runtime dependencies
RUN apk add --no-cache \
    ca-certificates

# Set the working directory inside the container
WORKDIR /app/

# Copy the Go binary from the builder stage
COPY --from=builder /app/paperless-gpt .

# Ensure the binary has execute permissions
RUN chmod +x /app/paperless-gpt

RUN mkdir -p /app/config/prompts

ENV PROMPTS_DIR="/app/config/prompts"

# Command to run the binary
CMD ["/app/paperless-gpt"]