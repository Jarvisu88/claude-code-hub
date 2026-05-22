#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# Claude Code Hub - One-click Docker Compose deployment
# ---------------------------------------------------------------------------
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------
check_prerequisites() {
    if ! command -v docker &>/dev/null; then
        error "docker is not installed. Please install Docker first: https://docs.docker.com/get-docker/"
    fi

    # Accept either "docker compose" (plugin) or "docker-compose" (standalone)
    if docker compose version &>/dev/null; then
        COMPOSE_CMD="docker compose"
    elif command -v docker-compose &>/dev/null; then
        COMPOSE_CMD="docker-compose"
    else
        error "docker compose is not available. Install Docker Compose: https://docs.docker.com/compose/install/"
    fi

    info "Using compose command: $COMPOSE_CMD"
}

# ---------------------------------------------------------------------------
# Generate .env if missing
# ---------------------------------------------------------------------------
setup_env() {
    local env_file="$PROJECT_DIR/.env"
    local example_file="$PROJECT_DIR/.env.example"

    if [[ -f "$env_file" ]]; then
        info ".env already exists, skipping generation."
        return
    fi

    if [[ ! -f "$example_file" ]]; then
        error ".env.example not found at $example_file"
    fi

    cp "$example_file" "$env_file"

    # Generate a random ADMIN_TOKEN if the placeholder is still "change-me"
    if grep -q '^ADMIN_TOKEN=change-me' "$env_file"; then
        local token
        token="$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)"
        sed -i "s/^ADMIN_TOKEN=change-me/ADMIN_TOKEN=${token}/" "$env_file"
        info "Generated random ADMIN_TOKEN."
    fi

    # Generate a random DB_PASSWORD if the placeholder is still the default
    if grep -q '^DB_PASSWORD=your-secure-password_change-me' "$env_file"; then
        local dbpass
        dbpass="$(head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 24)"
        sed -i "s/^DB_PASSWORD=your-secure-password_change-me/DB_PASSWORD=${dbpass}/" "$env_file"
        info "Generated random DB_PASSWORD."
    fi

    info ".env created from .env.example with random secrets."
    warn "Review $env_file and adjust settings before production use."
}

# ---------------------------------------------------------------------------
# Deploy
# ---------------------------------------------------------------------------
deploy() {
    cd "$PROJECT_DIR"

    info "Pulling / building images..."
    $COMPOSE_CMD build --pull 2>&1 | tail -5

    info "Starting services..."
    $COMPOSE_CMD up -d

    info "Waiting for health check..."
    local retries=30
    local delay=2
    local ok=0

    for ((i = 1; i <= retries; i++)); do
        if $COMPOSE_CMD exec -T app wget -qO- http://127.0.0.1:23000/health 2>/dev/null | grep -q '"healthy"'; then
            ok=1
            break
        fi
        printf "  Attempt %d/%d...\r" "$i" "$retries"
        sleep "$delay"
    done

    echo ""

    if [[ "$ok" -eq 1 ]]; then
        info "Claude Code Hub is up and healthy!"
    else
        warn "Health check did not pass within $(( retries * delay ))s."
        warn "Check logs: $COMPOSE_CMD logs app"
    fi

    # Determine access URL
    local port
    port="$(grep -oP '(?<=^APP_PORT=)\d+' "$PROJECT_DIR/.env" 2>/dev/null || echo 23000)"
    local host_ip
    host_ip="$(hostname -I 2>/dev/null | awk '{print $1}' || echo '127.0.0.1')"

    echo ""
    info "Access URL:  http://${host_ip}:${port}"
    info "Admin token: (see .env file)"
    echo ""
    info "Useful commands:"
    echo "  $COMPOSE_CMD logs -f app     # Follow app logs"
    echo "  $COMPOSE_CMD ps              # Service status"
    echo "  $COMPOSE_CMD down            # Stop all services"
    echo "  $COMPOSE_CMD down -v         # Stop and remove volumes"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
    info "Claude Code Hub - Docker Compose Deployment"
    echo ""
    check_prerequisites
    setup_env
    deploy
}

main "$@"
