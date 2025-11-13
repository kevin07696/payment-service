# Build stage
FROM golang:1.24-alpine AS builder

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

# Install goose for database migrations
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
# postgresql-client provides pg_isready for database health checks
RUN apk --no-cache add ca-certificates tzdata curl postgresql-client

# Create non-root user for security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /home/appuser

# Copy binary from builder with correct ownership
COPY --from=builder --chown=appuser:appuser /app/payment-server .

# Copy goose binary from builder
COPY --from=builder --chown=appuser:appuser /go/bin/goose /usr/local/bin/goose

# Copy database migrations
COPY --chown=appuser:appuser internal/db/migrations ./migrations

# Copy entrypoint script
COPY --chown=appuser:appuser scripts/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# Create secrets directory (populated at runtime via volume mount)
RUN mkdir -p secrets && chown appuser:appuser secrets

# Switch to non-root user
USER appuser

# Expose ports
# 8080: gRPC server
# 8081: HTTP server (cron endpoints + Browser Post)
EXPOSE 8080 8081

# Health check for container orchestration
# Using curl for consistency with cloud-init docker-compose
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8081/cron/health || exit 1

# Run the application via entrypoint (runs migrations first)
CMD ["./entrypoint.sh"]
