#!/usr/bin/env sh
# One-command bring-up for the multi-domain quickstart demo:
#   UModel (serving the quickstart pack) + a seeded Prometheus + a seeded Elasticsearch.
# Then connect an agent with the umodel-query skill and read real data — see ../README.md.
set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
COMPOSE_FILE="$DIR/docker-compose.yml"

# Pick a compose engine: docker compose / podman compose / docker-compose.
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif podman compose version >/dev/null 2>&1; then
  COMPOSE="podman compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  echo "error: need 'docker compose' or 'podman compose' (or docker-compose) on PATH" >&2
  exit 1
fi

echo "==> Bringing up the demo stack with: $COMPOSE"
# shellcheck disable=SC2086
$COMPOSE -f "$COMPOSE_FILE" up -d --build

echo "==> Waiting for Elasticsearch seed + Prometheus scrapes (up to ~3 min)..."
i=0
while [ "$i" -lt 36 ]; do
  es="$(curl -s --max-time 3 'http://localhost:9200/devops-service-logs-*/_count' 2>/dev/null || true)"
  prom="$(curl -s --max-time 3 -G 'http://localhost:9090/api/v1/query' \
            --data-urlencode 'query=sum(rate(devops_service_request_total[1m]))' 2>/dev/null || true)"
  case "$es"   in *'"count":'[1-9]*) es_ok=1 ;; *) es_ok=0 ;; esac
  case "$prom" in *'"value":'*)       prom_ok=1 ;; *) prom_ok=0 ;; esac
  if [ "$es_ok" = 1 ] && [ "$prom_ok" = 1 ]; then
    echo "    ready."
    break
  fi
  i=$((i + 1))
  sleep 5
done
if [ "${es_ok:-0}" != 1 ] || [ "${prom_ok:-0}" != 1 ]; then
  echo "    (still warming up — Elasticsearch/Prometheus may need another minute; queries below will fill in)"
fi

cat <<EOF

==> Demo stack is up:
    UModel         http://localhost:8080   object graph + plan provider (workspace 'demo')
    Prometheus     http://localhost:9090   seeded metrics
    Elasticsearch  http://localhost:9200   seeded logs

==> Read data with an agent (umodel-query skill):
    export UMCTL_ADDR=http://localhost:8080
    Then ask, e.g.:
      "List the devops services and their status, then read checkout-service's request rate,
       error rate, and p95 latency, and show its recent ERROR logs. Why is it degraded?"

    Smoke-test without an agent:  sh "$DIR/verify.sh"
    Tear down:                    $COMPOSE -f "$COMPOSE_FILE" down -v
EOF
