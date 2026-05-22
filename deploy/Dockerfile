# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Production Dockerfile (deploy directory variant)
# Identical to the root Dockerfile; kept here for deploy/ convention.
# ---------------------------------------------------------------------------

# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Stage 2: Runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 appgroup \
    && adduser -u 1000 -G appgroup -D -h /app appuser

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY config.example.yaml /app/config.yaml

RUN chown -R appuser:appgroup /app

USER appuser
EXPOSE 23000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:23000/health || exit 1

ENTRYPOINT ["/app/server"]
