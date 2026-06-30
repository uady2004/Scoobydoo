FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY main.go .

RUN go mod init backend && \
    go get github.com/gin-gonic/gin && \
    go get github.com/gin-contrib/cors && \
    go get github.com/golang-jwt/jwt/v5 && \
    go mod tidy

RUN go build -o server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]