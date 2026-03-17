# Multi-stage build for video-list with multi-platform support
# Build with: docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t video-list:latest .

# Stage 1: Build frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /build

# Copy package files
COPY package.json package-lock.json ./

# Install dependencies
RUN npm ci

# Copy frontend source
COPY tsconfig*.json vite.config.ts index.html ./
COPY src ./src
COPY public ./public

# Build frontend
RUN npm run build

# Stage 2: Build Go backend for multiple architectures
FROM golang:1.24-alpine AS backend-builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies - this captures build args
ARG TARGETARCH=amd64
ARG TARGETVARIANT

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go ./
COPY internal ./internal

# Copy frontend dist from previous stage
COPY --from=frontend-builder /build/dist ./dist

# Build binary with architecture-specific flags
RUN if [ "$TARGETARCH" = "arm" ] && [ "$TARGETVARIANT" = "v7" ]; then \
      CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
        go build -ldflags="-s -w" -o video-list main.go; \
    else \
      CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
        go build -ldflags="-s -w" -o video-list main.go; \
    fi

# Stage 3: Runtime - single base image supports all architectures
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && adduser -D -u 1000 video-list

WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder --chown=video-list:video-list /build/video-list /app/video-list

# Copy timezone data
COPY --from=backend-builder /usr/share/zoneinfo /usr/share/zoneinfo

# Create necessary directories with proper permissions
RUN mkdir -p /app/logs /app/config && \
    chown -R video-list:video-list /app && \
    chmod 700 /app/logs /app/config

# Create non-root user for running the app
USER video-list

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3001/api/auth/status || exit 1

# Expose port
EXPOSE 3001

# Volumes for persistent data
VOLUME ["/app/config", "/app/logs"]

# Default command
CMD ["/app/video-list", "--config", "/app/config/config.yaml"]

# Labels with architecture info
LABEL org.opencontainers.image.title="video-list" \
      org.opencontainers.image.description="Web-based video file manager and editor (multi-arch)" \
      org.opencontainers.image.url="https://github.com/your-org/video-list" \
      org.opencontainers.image.documentation="https://github.com/your-org/video-list" \
      org.opencontainers.image.source="https://github.com/your-org/video-list" \
      org.opencontainers.image.version="1.0.0"
