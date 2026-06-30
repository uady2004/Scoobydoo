# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /bin/livestream-service ./

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata ffmpeg
WORKDIR /app
COPY --from=builder /bin/livestream-service .
EXPOSE 8085 1935
ENTRYPOINT ["/app/livestream-service"]
