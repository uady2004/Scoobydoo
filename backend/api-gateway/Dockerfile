# syntax=docker/dockerfile:1.6
# =============================================================================
# Stage 1 â€” Build
# =============================================================================
FROM golang:1.25-alpine AS builder

# Install build dependencies (git for module downloads, ca-certs for TLS).
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for the final image.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /build

# â”€â”€ Module cache â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
# Copy dependency manifests first so this layer is cache-friendly.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# â”€â”€ Source copy & compile â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
COPY . .

# Build flags:
#   -trimpath       â€” remove build machine paths from binary (reproducibility)
#   -ldflags        â€” strip debug info and set version variables
#   CGO_ENABLED=0   â€” fully static binary; no libc dependency
#   GOOS/GOARCH     â€” explicit target platform
RUN --mount=type=cache,target=/root/.cache/go \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
                    -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      -o /build/api-gateway \
      ./main.go

# Verify the binary was produced.
RUN test -f /build/api-gateway && echo "Build OK"

# =============================================================================
# Stage 2 â€” Final image (minimal alpine for shell-based health checks)
# =============================================================================
# Using alpine instead of scratch so that:
#   1. The HEALTHCHECK CMD can use wget/sh.
#   2. Debugging tools (sh, ls) are available in QA environments.
# For a fully hardened production image, switch FROM to scratch and use a
# Kubernetes httpGet liveness probe targeting GET /health instead.
FROM alpine:3.19 AS final

# Install only what is needed at runtime.
RUN apk add --no-cache ca-certificates tzdata wget && \
    rm -rf /var/cache/apk/*

# Reuse the non-root user/group created in the builder.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the compiled static binary.
COPY --from=builder /build/api-gateway /usr/local/bin/api-gateway

# Run as non-root.
USER appuser:appgroup

# â”€â”€ Runtime configuration â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
# These are defaults only; override via environment variables or a ConfigMap.
ENV GIN_MODE=release \
    PORT=8080 \
    REDIS_ADDR=redis:6379 \
    KAFKA_BROKERS=kafka:9092 \
    KAFKA_AUDIT_TOPIC=audit-logs \
    JWT_ISSUER=tiktok-clone \
    JWT_AUDIENCE=tiktok-api \
    RATE_LIMIT_USER_RPM=100 \
    RATE_LIMIT_IP_RPM=1000 \
    RATE_LIMIT_WINDOW_SECONDS=60 \
    TZ=UTC

EXPOSE 8080

# Health check â€” hits the unauthenticated /health endpoint.
# In Kubernetes prefer an httpGet probe over this HEALTHCHECK.
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://localhost:${PORT}/health" || exit 1

ENTRYPOINT ["/usr/local/bin/api-gateway"]
