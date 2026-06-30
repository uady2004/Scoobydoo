FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
ENV GONOSUMCHECK=* GOFLAGS=-mod=mod
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /like-service .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /like-service .
EXPOSE 8098
ENTRYPOINT ["/app/like-service"]
