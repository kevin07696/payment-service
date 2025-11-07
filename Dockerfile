# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies (cached layer)
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary
# -ldflags="-w -s" to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o payment-server \
    ./cmd/server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /home/appuser

# Copy binary from builder with correct ownership
COPY --from=builder --chown=appuser:appuser /app/payment-server .

# Create secrets directory (populated at runtime via volume mount)
RUN mkdir -p secrets && chown appuser:appuser secrets

# Switch to non-root user
USER appuser

# Expose ports
# 8080: gRPC server
# 8081: HTTP server (cron endpoints + Browser Post)
EXPOSE 8080 8081

# Health check for container orchestration
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/cron/health || exit 1

# Run the application
CMD ["./payment-server"]
