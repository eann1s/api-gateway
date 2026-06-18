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
docker compose -f docker-compose.yml up -d redis
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

Default config file: [`config.yml`](./config.yml). Every value supports env substitution, so the same file works across environments. The config below is the full schema — there are no hidden sections.

```yaml
listeners:
  public:
    addr: :${PUBLIC_PORT:-8080}      # client-facing traffic
  admin:
    addr: :${ADMIN_PORT:-9090}       # health, readiness, metrics
observability:
  logs:
    level: ${LOG_LEVEL:-info}        # debug | info | warn | error
  metrics:
    enabled: ${METRICS_ENABLED:-false}
defaults:
  timeouts:
    request: ${REQUEST_TIMEOUT:-30s}                          # whole client request
    upstream_response_header: ${UPSTREAM_RESPONSE_HEADER_TIMEOUT:-30s}  # wait for upstream headers
  body_limit: ${BODY_LIMIT:-1MB}     # max request body; larger -> 413
shutdown:
  timeout: ${SHUTDOWN_TIMEOUT:-5s}   # grace period on SIGINT/SIGTERM
rate_limit:
  capacity: ${RATE_LIMIT_CAPACITY:-20}             # token bucket size (burst)
  refill_rate_per_sec: ${RATE_LIMIT_REFILL_RATE_PER_SEC:-5}  # sustained rate per identity
  key_prefix: ${RATE_LIMIT_KEY_PREFIX:-rl}         # Redis key namespace
redis:
  addr: ${REDIS_ADDR:-localhost:6379}
  password: ${REDIS_PASSWORD:-}
  db: ${REDIS_DB:-0}
routes:
  - id: root                  # unique, used in logs and metrics labels
    host: api.example.com     # matched exactly (case-insensitive, port stripped)
    path_prefix: /            # longest matching prefix wins; "/" is catch-all
    upstream_pool: main-pool  # must reference a pool id below
upstream_pools:
  - id: main-pool
    targets:                  # round-robin; scheme required, no path/trailing slash
      - "http://svc-a:8081"
      - "http://svc-a:8082"
```

Env substitution rules:

- `${VAR}` — variable must exist or startup fails
- `${VAR:-default}` — fall back to `default` when unset
- process env overrides values loaded from `.env`

Config is fully validated at startup and the process refuses to boot on any error: ports must be 1–65535 and distinct, log level must be valid, timeouts / body limit / rate-limit values must be positive, Redis address must be `host:port`, every route needs all four fields, every pool needs at least one valid `http`/`https` target, and every `upstream_pool` referenced by a route must exist.

### Environment variables

These are the variables the default [`config.yml`](./config.yml) reads. Override any of them in the environment or `.env`.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PUBLIC_PORT` | `8080` | Public listener port |
| `ADMIN_PORT` | `9090` | Admin listener port (must differ from public) |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `METRICS_ENABLED` | `false` | Expose Prometheus metrics on `/metrics` |
| `REQUEST_TIMEOUT` | `30s` | Deadline for the whole client request |
| `UPSTREAM_RESPONSE_HEADER_TIMEOUT` | `30s` | Wait for upstream response headers |
| `BODY_LIMIT` | `1MB` | Max request body (`B`/`KB`/`MB`/`GB`/`TB`) |
| `SHUTDOWN_TIMEOUT` | `5s` | Graceful shutdown grace period |
| `RATE_LIMIT_CAPACITY` | `20` | Token bucket size — allowed burst per identity |
| `RATE_LIMIT_REFILL_RATE_PER_SEC` | `5` | Sustained requests/sec per identity |
| `RATE_LIMIT_KEY_PREFIX` | `rl` | Redis key namespace for limiter state |
| `REDIS_ADDR` | `localhost:6379` | Redis address (`host:port`) |
| `REDIS_PASSWORD` | _(empty)_ | Redis password, if required |
| `REDIS_DB` | `0` | Redis database index |

## Point the gateway at your own service

The default config routes `api.example.com` to two placeholder upstreams. To use the gateway for real, edit two sections of [`config.yml`](./config.yml):

1. Add a pool with your backend targets:

   ```yaml
   upstream_pools:
     - id: orders-pool
       targets:
         - "http://orders-1:8080"
         - "http://orders-2:8080"
   ```

2. Add a route that points a host + path prefix at that pool:

   ```yaml
   routes:
     - id: orders
       host: api.mycompany.com
       path_prefix: /orders
       upstream_pool: orders-pool
   ```

3. Restart the gateway. A request to `http://<gateway>:8080/orders/123` with `Host: api.mycompany.com` is now forwarded to one of the `orders-pool` targets (round-robin), arriving as `/orders/123` on the upstream with `X-Forwarded-*` and `X-Request-ID` headers set.

Because the host is matched exactly, send the right `Host` header when testing locally:

```bash
curl -i -H "Host: api.mycompany.com" http://localhost:8080/orders/123
```

## Routing

The gateway matches each incoming request to a single route:

- **Host** is matched exactly after lowercasing and stripping the port — there are no wildcards. A request whose `Host` matches no route gets `404`.
- **Path prefix** uses longest-prefix-wins, and only on segment boundaries. With routes for `/` and `/orders`, a request to `/orders/5` matches `/orders`, `/billing` matches `/` (the catch-all), and `/ordersx` matches `/` (not `/orders`, because the prefix must end at a `/`).
- Path prefixes are normalized at load time; each segment must match `[A-Za-z0-9._~-]+`.
- A `(host, path_prefix)` pair must be unique — duplicates fail startup.

The matched pool's targets are selected **round-robin**. There is no active health checking yet: if a target is down the proxy returns `502` for requests routed to it.

## Rate limiting

Each request is attributed to an **identity**, and a token bucket is enforced per identity:

1. An API key, if present — taken from `X-API-Key`, or from `Authorization: Bearer <token>`. Keyed as `api_key:<value>`.
2. Otherwise the client IP, keyed as `ip:<addr>`.
3. Otherwise `anonymous`.

The bucket holds `capacity` tokens and refills at `refill_rate_per_sec`. `capacity` is the allowed burst; `refill_rate_per_sec` is the sustained rate. State lives in Redis (namespaced by `key_prefix`), so the limit is shared across all gateway instances; a local in-memory limiter is used as a fallback if Redis is unavailable.

When the bucket is empty the gateway returns `429` with a `Retry-After` header (seconds). If the limiter backend itself errors, the request gets `503`.

The API key is only used as a rate-limit bucket label — it is **not** validated or authenticated. The gateway does no auth; put authn/authz in front of it or in the upstream.

## Request forwarding

When a request is forwarded upstream the gateway:

- preserves method, path, and query string;
- clones request headers and strips hop-by-hop headers (both directions);
- sets `X-Forwarded-For` (appending the client IP), `X-Forwarded-Proto`, `X-Forwarded-Host`, and `X-Request-ID`;
- enforces `body_limit` on the request body, returning `413` if exceeded;
- passes the upstream status code and body straight back to the client.

## Public response codes

What a client integrating against the public listener (`:8080`) can expect:

| Status | When |
| --- | --- |
| upstream's own code | Request matched a route and was forwarded successfully |
| `404` | No route matched the host or path |
| `413` | Request body exceeded `body_limit` |
| `429` | Rate limit exceeded — includes `Retry-After` |
| `502` | Upstream pool could not be resolved/selected, or the forward failed |
| `503` | Rate limiter could not decide (Redis and local fallback both failed, or request cancelled) |
| `500` | Internal error (e.g. misconfigured target) |

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

Build and run the backend image directly:

```bash
docker build -t gateway-backend:dev --build-arg VERSION=dev ./backend
docker run --rm -p 8080:8080 -p 9090:9090 gateway-backend:dev
```

Or bring up the gateway together with Redis using the root [`docker-compose.yml`](../docker-compose.yml), which reads ports from `.env` (copy [`.env.example`](../.env.example) first):

```bash
cp .env.example .env   # from repo root
docker compose up --build
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

- `Invalid routes config` / `Invalid upstream pools config` at startup
  - a route is missing a field, a pool has no targets, a target has a scheme other than `http`/`https` or includes a path/trailing slash, or a route references a pool id that does not exist

- public requests return `404`
  - the `Host` header does not match any route exactly, or no path prefix matches — check the `host` and `path_prefix` values

- public requests return `502`
  - the routed upstream targets are unreachable; there is no health checking, so a down target still receives its share of round-robin traffic

- public requests return `503`
  - the rate limiter could not reach a decision. Redis being unreachable on its own degrades to the local in-memory limiter and the request is still served; a `503` means both the Redis limiter and the local fallback failed (or the request was cancelled / timed out). Check `REDIS_ADDR`, that Redis is running, and the gateway logs

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
