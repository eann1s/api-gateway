#!/bin/bash

set -euo pipefail

sudo -v

log() {
    printf '[smoke][%s] %s\n' "$1" "$2"
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../" && pwd)"

if [ -f "$REPO_ROOT/.env" ]; then
    set -a
    source "$REPO_ROOT/.env"
    set +a
fi
export METRICS_ENABLED="${METRICS_ENABLED:-true}"
export RATE_LIMIT_CAPACITY="${RATE_LIMIT_CAPACITY:-5}"

PUBLIC_PORT=${PUBLIC_PORT:-8080}
ADMIN_PORT=${ADMIN_PORT:-9090}

cd "$REPO_ROOT/backend"
make run &
APP_PID=$!

LARGE_BODY_TXT="/tmp/large_body.txt"
HEADERS_TXT="/tmp/headers.txt"

if ! grep -qE '^[[:space:]]*127\.0\.0\.1[[:space:]]+svc-a([[:space:]]|$).*# gateway-smoke$' /etc/hosts; then
    echo '127.0.0.1 svc-a # gateway-smoke' | sudo tee -a /etc/hosts >/dev/null
fi

python3 -m http.server 8081 &
SERVER1_PID=$!
python3 -m http.server 8082 &
SERVER2_PID=$!

cleanup() {
    kill "$APP_PID" 2>/dev/null || true
    kill "$SERVER1_PID" 2>/dev/null || true
    kill "$SERVER2_PID" 2>/dev/null || true

    rm -f "$LARGE_BODY_TXT" "$HEADERS_TXT"

    sudo -n sed -i '' '/# gateway-smoke/d' /etc/hosts 2>/dev/null || true
}

trap cleanup EXIT


DEADLINE=$((SECONDS + 30))

# wait for the app to be ready
log INFO "=== waiting for the app to be ready ==="
while [ "$SECONDS" -lt "$DEADLINE" ]; do
    code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${ADMIN_PORT}/readyz" || true)
    if [ "$code" = "200" ]; then
        break
    fi
    sleep 1
done

if [ "$code" != "200" ]; then
    log ERROR "readyz didn't become 200 in 30 sec"
    exit 1
fi

# assert that the app is healthy
log INFO "=== asserting that the app is healthy ==="
code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${ADMIN_PORT}/healthz" || true)

if [ "$code" != "200" ]; then
    log ERROR "app is not healthy"
    exit 1
fi

# assert that the metrics endpoint returns 200
log INFO "=== assert that the metrics endpoint returns 200 ==="
code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${ADMIN_PORT}/metrics" || true)

if [ "$code" != "200" ]; then
    log ERROR "metrics endpoint hasn't returned 200"
    exit 1
fi

# success route
log INFO "=== success route ==="
code=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: api.example.com" "http://localhost:${PUBLIC_PORT}" || true)

if [ "$code" != "200" ]; then
    log ERROR "success route didn't return 200"
    exit 1
fi

# unknown route returns 404
log INFO "=== unknown route ==="
code=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: no-such-host.example.com" "http://localhost:${PUBLIC_PORT}/unknown" || true)

if [ "$code" != "404" ]; then
    log ERROR "unknown route didn't return 404"
    exit 1
fi

# oversized body check
log INFO "=== oversized body ==="
head -c 2000000 </dev/zero | tr '\0' 'a' > "$LARGE_BODY_TXT"

code=$(curl -s -X POST --data-binary "@${LARGE_BODY_TXT}" -o /dev/null -w "%{http_code}" -H "Host: api.example.com" "http://localhost:${PUBLIC_PORT}" || true)

if [ "$code" != "413" ]; then
    log ERROR "oversized body didn't return 413"
    exit 1
fi

# requests burst returns 429
log INFO "=== requests burst ==="
CAPACITY=${RATE_LIMIT_CAPACITY:-20}

for i in $(seq 1 $CAPACITY); do
    log INFO "=== valid request $i ==="
    curl -s -o /dev/null -H "X-API-Key: smoke-test" -H "Host: api.example.com" "http://localhost:${PUBLIC_PORT}" || true
done

log INFO "=== overflow request ==="
code=$(curl -s -o /dev/null -w "%{http_code}" -D "$HEADERS_TXT" -H "X-API-Key: smoke-test" -H "Host: api.example.com" "http://localhost:${PUBLIC_PORT}" || true)
if [ "$code" != "429" ]; then
    log ERROR "requests burst didn't return 429"
    exit 1
fi

retry_after=$(tr -d '\r' < "$HEADERS_TXT" | grep -i -E "^Retry-After: [0-9]+$" || true)
if [ -z "$retry_after" ]; then
    log ERROR "requests burst didn't return Retry-After header"
    exit 1
fi

# success
log INFO "smoke test passed"
exit 0


