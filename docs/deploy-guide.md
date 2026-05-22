# Deployment Guide

This guide covers deploying Claude Code Hub in production environments.

## Prerequisites

- Docker 24+ and Docker Compose v2 (for Docker deployment)
- kubectl and a running Kubernetes cluster (for K8s deployment)
- PostgreSQL 15+ and Redis 7+ (if running without Docker)
- A domain name with DNS pointing to your server (recommended)

## Docker Compose Deployment

This is the recommended approach for single-server deployments.

### Step 1: Prepare the server

```bash
git clone https://github.com/ding113/claude-code-hub.git
cd claude-code-hub
```

### Step 2: Configure environment

```bash
cp .env.example .env
```

Edit `.env` and set the following at minimum:

```bash
ADMIN_TOKEN=your-secure-admin-token
DB_PASSWORD=a-strong-database-password
```

### Step 3: Start services

```bash
docker compose up -d
```

This starts PostgreSQL, Redis, and the application. The database is automatically migrated on first start.

### Step 4: Verify

```bash
# Check all containers are running
docker compose ps

# Check health endpoint
curl http://localhost:23000/health
```

Open `http://your-server:23000` in a browser and log in with the admin token.

### Updating

```bash
docker compose pull
docker compose up -d
```

The application runs database migrations automatically on startup when `AUTO_MIGRATE=true` (the default).

## Kubernetes Deployment

Manifests are provided in `deploy/k8s/`.

### Step 1: Create namespace

```bash
kubectl apply -f deploy/k8s/namespace.yaml
```

### Step 2: Configure secrets

Edit `deploy/k8s/secret.yaml` with your credentials (base64-encoded):

```bash
# Encode your values
echo -n "your-admin-token" | base64
echo -n "postgres://user:pass@host:5432/dbname?sslmode=disable" | base64
echo -n "redis://host:6379/0" | base64
```

Apply the secret:

```bash
kubectl apply -f deploy/k8s/secret.yaml
```

### Step 3: Deploy ConfigMap

Review and adjust `deploy/k8s/configmap.yaml` for your environment, then apply:

```bash
kubectl apply -f deploy/k8s/configmap.yaml
```

### Step 4: Deploy PostgreSQL and Redis

If you do not have external PostgreSQL/Redis services:

```bash
kubectl apply -f deploy/k8s/postgres/
kubectl apply -f deploy/k8s/redis/
```

Wait for the pods to become ready:

```bash
kubectl -n claude-code-hub get pods -w
```

### Step 5: Deploy the application

```bash
kubectl apply -f deploy/k8s/app/
```

### Step 6: Configure Ingress

Review `deploy/k8s/ingress/` and adjust the hostname, then apply:

```bash
kubectl apply -f deploy/k8s/ingress/
```

### Verify

```bash
kubectl -n claude-code-hub get pods
kubectl -n claude-code-hub logs deployment/claude-code-hub -f
```

## Configuration Reference

See the [README configuration table](../README.md#configuration) for all environment variables.

Key settings for production:

| Setting | Recommended Value | Notes |
|---------|-------------------|-------|
| `CCH_SERVER_MODE` | `release` | Disables debug logging in Gin |
| `CCH_LOG_LEVEL` | `info` | Use `warn` for quieter logs |
| `CCH_LOG_FORMAT` | `json` | Structured logs for log aggregation |
| `CCH_DATABASE_MAX_OPEN_CONNS` | `25` | Adjust based on workload |
| `CCH_RATE_LIMIT_ENABLED` | `true` | Protect against abuse |
| `CCH_AUTO_MIGRATE` | `true` | Safe for production; runs only pending migrations |

## TLS/SSL Setup

### Using a reverse proxy (recommended)

Place nginx, Caddy, or Traefik in front of Claude Code Hub:

**Caddy example** (`Caddyfile`):

```
hub.example.com {
    reverse_proxy localhost:23000
}
```

Caddy automatically provisions and renews TLS certificates via Let's Encrypt.

**Nginx example**:

```nginx
server {
    listen 443 ssl http2;
    server_name hub.example.com;

    ssl_certificate     /etc/letsencrypt/live/hub.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hub.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:23000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE / streaming support
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 300s;
    }
}
```

### Kubernetes Ingress with TLS

Use cert-manager with your Ingress resource:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - hub.example.com
      secretName: hub-tls
```

## Scaling Guide

### Horizontal scaling

Claude Code Hub is stateless (sessions are stored in Redis). You can run multiple replicas behind a load balancer.

**Docker Compose:**

```bash
docker compose up -d --scale app=3
```

Add an nginx or HAProxy in front to distribute traffic.

**Kubernetes:**

```bash
kubectl -n claude-code-hub scale deployment/claude-code-hub --replicas=3
```

Or configure an HPA:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: claude-code-hub
  namespace: claude-code-hub
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: claude-code-hub
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

### Database scaling

- Use connection pooling (PgBouncer) for high connection counts.
- Read replicas can be configured for read-heavy workloads (usage log queries).
- Redis Sentinel or Redis Cluster for high-availability Redis.

## Backup and Restore

### PostgreSQL backup

```bash
# Dump the database
docker compose exec postgres pg_dump -U postgres claude_code_hub > backup.sql

# Restore
docker compose exec -i postgres psql -U postgres claude_code_hub < backup.sql
```

For automated backups, use `pg_dump` on a cron schedule or a tool like WAL-G for continuous archiving.

### Redis

Redis is used for ephemeral data (sessions, rate limit counters, caches). A backup is not strictly required, but if desired:

```bash
docker compose exec redis redis-cli BGSAVE
docker compose cp redis:/data/dump.rdb ./redis-backup.rdb
```

## Monitoring

### Health endpoint

The `/health` endpoint returns HTTP 200 when the service is operational. Use it for container health checks and uptime monitoring.

### Logs

Structured JSON logs are written to stdout. Pipe them to your log aggregation system (ELK, Loki, Datadog, etc.).

### Metrics

Monitor these key indicators:

- **Request latency** -- Available in usage logs and the real-time dashboard
- **Error rate** -- Visible on the dashboard and via notification alerts
- **Provider health** -- Circuit breaker state changes trigger webhook notifications
- **Database connections** -- Monitor `pg_stat_activity` for connection pool saturation
- **Redis memory** -- Monitor with `redis-cli INFO memory`

### Built-in dashboard

The web dashboard (`/dashboard`) and big screen (`/big-screen`) provide real-time visibility into system health, request flow, and provider status.

### Notification webhooks

Configure webhook targets in the dashboard to receive alerts for:

- Circuit breaker state changes (provider failures)
- Cost threshold breaches
- Cache hit rate drops
- Leaderboard summaries
