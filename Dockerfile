# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1: Build the Go binary
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---------------------------------------------------------------------------
# Stage 2: Minimal runtime image
# ---------------------------------------------------------------------------
FROM alpine:3.20

# TLS root certificates + timezone data
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 appgroup \
    && adduser -u 1000 -G appgroup -D -h /app appuser

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /out/server /app/server

# Provide a default config file (operators should mount their own)
COPY config.example.yaml /app/config.yaml

# Own everything by the non-root user
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 23000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:23000/health || exit 1

ENTRYPOINT ["/app/server"]
