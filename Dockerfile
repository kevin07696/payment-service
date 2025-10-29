# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o payment-server ./cmd/server

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/payment-server .

# Expose ports
EXPOSE 8080 8081

# Default command runs the server (can be overridden)
CMD ["./payment-server"]
