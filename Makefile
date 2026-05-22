.PHONY: dev build test lint clean run migrate check

# Development
dev:
	air

run:
	go run cmd/server/main.go

# Build
build:
	go build -o bin/server cmd/server/main.go

# Testing
test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-verbose:
	go test -v ./...

# Quality
lint:
	go vet ./...

typecheck:
	go build ./...

# Database
migrate:
	go run cmd/migrate/main.go

# Clean
clean:
	rm -rf bin/ tmp/ coverage.out coverage.html

# Docker
docker-build:
	docker build -t claude-code-hub:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# All checks
check: lint test build
	@echo "All checks passed"
