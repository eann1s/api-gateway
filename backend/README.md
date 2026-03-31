# Gateway Backend

Gateway Backend is a Go service for API gateway and rate-limiter workloads.

## Features

- YAML config loading with env substitution
- Config validation on startup
- Separate public and admin listeners
- Graceful shutdown on `SIGINT` / `SIGTERM`
- Admin endpoints: `/healthz`, `/readyz`, `/metrics`
- Structured JSON logging

## Requirements

- Go `1.25+`
- `make`
- `golangci-lint` (for `make lint`)

## Quick Start

Run from the `backend` directory:

```bash
cd backend
make run
```

The app loads config from `./config.yml` by default.

If you want an explicit config path:

```bash
go run ./cmd/rate-limiter --config ./config.yml
```

## Verify Endpoints

By default:
- public listener: `:8080`
- admin listener: `:9090`

Checks:

```bash
curl -i http://localhost:9090/healthz
curl -i http://localhost:9090/readyz
curl -i http://localhost:9090/metrics
```

Expected:
- `/healthz` -> `200`
- `/readyz` -> `200` when the app is ready, `503` during startup/shutdown
- `/metrics` -> `200` with Prometheus metrics output

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

### Env Substitution

- `${VAR}`: variable must exist, otherwise startup fails
- `${VAR:-default}`: uses `default` if variable is not set
- Process environment variables override values loaded from `.env`

## Development Commands

```bash
make help
make test
make lint
make build
make run
```

## Build Binary

```bash
make build
./bin/backend --config ./config.yml
```

## Docker

Build image:

```bash
docker build -t gateway-backend:dev --build-arg VERSION=dev ./backend
```

Run:

```bash
docker run --rm -p 8080:8080 -p 9090:9090 gateway-backend:dev
```

## Troubleshooting

- `undefined environment variable "..."`
  - Add the variable to your environment or use `${VAR:-default}` in config.

- `Invalid log level value "..."`
  - Allowed levels: `debug`, `info`, `warn`, `error`.

- `listen tcp ...: bind: address already in use`
  - Change `PUBLIC_PORT` / `ADMIN_PORT` or stop the process that already uses the port.

- `/readyz` returns `503`
  - The app is not ready yet or is shutting down.

## Project Layout

```text
cmd/rate-limiter/        # entrypoint
internal/app/            # app lifecycle (run/shutdown)
internal/config/         # config load/defaults/validation
internal/transport/http/ # HTTP handlers and mux
internal/middleware/     # request id and access log middleware
internal/obs/            # logger factory
```
