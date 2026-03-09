# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/e5renew main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S e5renew && \
    adduser -u 1001 -S e5renew -G e5renew

# Set working directory
WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/bin/e5renew .

# Copy configuration template and migrations
COPY --from=builder /app/config.prod.yaml.template ./config.prod.yaml.template
COPY --from=builder /app/migrations ./migrations

# Create necessary directories
RUN mkdir -p /app/logs && \
    chown -R e5renew:e5renew /app

# Switch to non-root user
USER e5renew

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./e5renew", "--config", "/app/config/config.yaml"]
