# Gateway Backend

Go backend service for API gateway and rate-limiter workloads.

## Capabilities

- YAML config loading with env substitution
- Startup config validation (including duration and byte-size parsing)
- Separate public and admin HTTP listeners
- Graceful shutdown on `SIGINT` / `SIGTERM`
- Admin endpoints: `/healthz`, `/readyz`, `/metrics`
- Structured JSON logging (`zerolog`)
- Request middleware: request ID + access logs
- Router core module (`internal/router`) with host + path-prefix matching
- Reverse proxy to upstream pools (round-robin target selection)
- Rate limiting:
  - Redis token bucket limiter
  - Local limiter fallback via composite limiter
  - `429` with `Retry-After`
- Prometheus request metrics:
  - total requests
  - request duration histogram

## Requirements

- Go `1.25+`
- `make`
- `golangci-lint` (optional, for `make lint`)
- Redis (`localhost:6379` by default)
- `curl` (for smoke script)
- `python3` (for local upstream servers in smoke script)
- `sudo` access (smoke script temporarily edits `/etc/hosts`)

## Quick Start

From the `backend` directory:

```bash
cd backend
go run ./cmd/rate-limiter --config ./config.yml
```

Start Redis first (from repo root):

```bash
docker compose -f docker-compose.local.yml up -d redis
```

By default:

- public listener: `:8080`
- admin listener: `:9090`

## Health and Metrics

```bash
curl -i http://localhost:9090/healthz
curl -i http://localhost:9090/readyz
curl -i http://localhost:9090/metrics
```

Expected behavior:

- `/healthz` -> `200`
- `/readyz` -> `200` when ready, `503` during startup/shutdown
- `/metrics` -> Prometheus metrics output

## Configuration

Default config file: [`config.yml`](./config.yml)

Example:

```yaml
listeners:
  public:
    addr: :${PUBLIC_PORT:-8080}
  admin:
    addr: :${ADMIN_PORT:-9090}
observability:
  logs:
    level: ${LOG_LEVEL:-info}
  metrics:
    enabled: ${METRICS_ENABLED:-false}
defaults:
  timeouts:
    request: ${REQUEST_TIMEOUT:-30s}
    upstream_response_header: ${UPSTREAM_RESPONSE_HEADER_TIMEOUT:-30s}
  body_limit: ${BODY_LIMIT:-1MB}
shutdown:
  timeout: ${SHUTDOWN_TIMEOUT:-5s}
```

Env substitution rules:

- `${VAR}` -> variable must exist
- `${VAR:-default}` -> fallback to `default` when missing
- process env overrides `.env` values

## Build and Dev Commands

```bash
cd backend
make help
make test
make lint
make build APP=rate-limiter
./bin/rate-limiter --config ./config.yml
```

## Smoke Test

From repo root:

```bash
./scripts/smoke/gateway_smoke.sh
```

What it validates:

- admin endpoints: `/readyz`, `/healthz`, `/metrics`
- public unknown host/path -> `404`
- oversized body -> `413`
- routed success path -> `200`
- rate-limit burst -> `429` + `Retry-After`

Notes:

- the script starts temporary local upstream HTTP servers on `8081` and `8082`
- the script adds/removes a temporary `svc-a` entry in `/etc/hosts`
- expected to prompt for `sudo` at start

## Docker

Build:

```bash
docker build -t gateway-backend:dev --build-arg VERSION=dev ./backend
```

Run:

```bash
docker run --rm -p 8080:8080 -p 9090:9090 gateway-backend:dev
```

## Troubleshooting

- `undefined environment variable "..."`
  - set variable in environment, or use `${VAR:-default}` in config

- `Invalid log level value "..."`
  - allowed: `debug`, `info`, `warn`, `error`

- `listen tcp ...: bind: address already in use`
  - change `PUBLIC_PORT` / `ADMIN_PORT` or stop conflicting process

- `/readyz` returns `503`
  - app is still starting or already shutting down

## Project Layout

```text
cmd/rate-limiter/        # entrypoint
internal/app/            # lifecycle orchestration
internal/config/         # config load + validation
internal/router/         # route normalization and matching core
internal/proxy/          # reverse proxy logic
internal/ratelimiter/    # redis/local/composite limiters
internal/transport/http/ # admin/public handlers and mux
internal/middleware/     # request id and access logs
internal/obs/            # logger and metrics setup
internal/readiness/      # readiness state/probe abstraction
internal/sweeper/        # periodic cleanup jobs
```
