# syntax=docker/dockerfile:1.7

# ---- Build stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache module downloads separately from source.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath \
    -ldflags="-s -w" \
    -o /out/recommendation-service \
    ./main.go

# ---- Runtime stage ----
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /out/recommendation-service /recommendation-service

# Non-root user (numeric UID; scratch has no /etc/passwd).
USER 65534:65534

EXPOSE 8080 50051

ENTRYPOINT ["/recommendation-service"]
