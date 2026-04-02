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

## Requirements

- Go `1.25+`
- `make`
- `golangci-lint` (optional, for `make lint`)

## Quick Start

From the `backend` directory:

```bash
cd backend
go run ./cmd/rate-limiter --config ./config.yml
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
internal/transport/http/ # admin/public handlers and mux
internal/middleware/     # request id and access logs
internal/obs/            # logger setup
internal/readiness/      # readiness state/probe abstraction
```
