# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Repository Info

- **Project**: Claude Code Hub (Go rewrite)
- **Source**: Rewritten from https://github.com/ding113/claude-code-hub
- **PR Target Branch**: `dev`

## Critical Rules

1. **No Emoji in Code** - Never use emoji characters in any code, comments, or string literals
2. **Test Coverage** - All new features must have unit test coverage of at least 80%
3. **Go Conventions** - Follow standard Go conventions (gofmt, golint, govet)

## Build & Development Commands

```bash
# Development
go run cmd/server/main.go     # Run dev server
air                            # Live reload (requires .air.toml)

# Build
go build -o bin/server cmd/server/main.go

# Quality Checks
go vet ./...                   # Static analysis
go test ./...                  # Run all tests
go test -race ./...            # Race condition detection
go test -cover ./...           # Coverage report

# Database
go run cmd/migrate/main.go    # Run migrations
```

## Architecture Overview

### Tech Stack
- **Framework**: Gin v1.11
- **Database**: PostgreSQL (Bun ORM) + Redis (go-redis)
- **Logging**: zerolog
- **Config**: viper (YAML + env)
- **Testing**: testify + miniredis

### Directory Structure
```
cmd/server/main.go              # Application entry point
internal/
  config/                       # Configuration (viper)
  database/                     # DB connections (postgres + redis)
  model/                        # Data models (Bun ORM, 18 tables)
  repository/                   # Data access layer (20+ repos)
  service/                      # Business logic
    auth/                       # API key + session auth
    session/                    # Session manager + tracker
    ratelimit/                  # Multi-dimensional rate limiting
    circuitbreaker/             # 3-level circuit breaker
    cost/                       # Token cost calculation
    cache/                      # Provider + settings cache
    notification/               # Webhook notifications
  proxy/                        # Core proxy engine
    guard/                      # 13-step guard pipeline
    selector/                   # Provider selection (weighted/priority)
    forwarder/                  # Upstream forwarding (HTTP/2, proxy)
    response/                   # Response handling + SSE streaming
    converter/                  # Format converters (5 formats)
    rectifier/                  # Request/response rectifiers
    error/                      # Error handling + rules
  handler/
    v1/                         # /v1 proxy routes
    api/                        # /api management routes
    middleware/                  # HTTP middleware
  pkg/                          # Internal utilities
  redis/                        # Redis helpers + Lua scripts
web/                            # Frontend (React + Vite)
tests/                          # Integration + parity tests
```

### Core Proxy Flow
```
Request -> GuardPipeline -> [auth -> sensitive -> client -> model -> version ->
                            probe -> session -> warmup -> requestFilter ->
                            rateLimit -> provider -> providerRequestFilter ->
                            messageContext] ->
           Forwarder -> ResponseHandler -> Response
```

## Code Conventions

- **Formatting**: gofmt (standard Go formatting)
- **Exports**: Follow Go naming conventions (PascalCase for exported)
- **Error handling**: Always handle errors, use wrapped errors with context
- **Testing**: Tests in same package, use testify for assertions
