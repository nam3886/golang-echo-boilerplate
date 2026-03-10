# Stage 1: Build
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.24.3
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /server ./cmd/server

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -S app && adduser -S app -G app
COPY --from=builder /server /server
COPY --from=builder /go/bin/goose /usr/local/bin/goose
COPY --from=builder /app/db/migrations /db/migrations
USER app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=60s --retries=5 \
    CMD curl -f http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/server"]
