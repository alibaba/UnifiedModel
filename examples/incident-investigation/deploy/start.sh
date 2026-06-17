#!/usr/bin/env sh
# One-command bring-up for the incident-investigation RCA demo:
#   UModel (serving the incident-investigation pack) + a seeded Prometheus + a seeded
#   Elasticsearch, matching the modeled incident. Connect an agent with the umodel-query
#   + umodel-rca skills and run a live root-cause analysis. See ../README.md.
set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
COMPOSE_FILE="$DIR/docker-compose.yml"

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

echo "==> Bringing up the incident-investigation demo stack with: $COMPOSE"
# shellcheck disable=SC2086
$COMPOSE -f "$COMPOSE_FILE" up -d --build

echo "==> Waiting for the 72h metric backfill + Elasticsearch seed + Prometheus scrapes (up to ~5 min)..."
hist_ts=$(( $(date +%s) - 216000 ))   # 60h ago: confirms the history was backfilled
i=0
while [ "$i" -lt 60 ]; do
  es="$(curl -s --max-time 3 'http://localhost:9200/platform-service-logs-*/_count' 2>/dev/null || true)"
  prom="$(curl -s --max-time 3 -G 'http://localhost:9090/api/v1/query' \
            --data-urlencode 'query=sum(rate(platform_service_request_total[1m]))' 2>/dev/null || true)"
  hist="$(curl -s --max-time 3 -G 'http://localhost:9090/api/v1/query' \
            --data-urlencode 'query=platform_service_request_total' \
            --data-urlencode "time=$hist_ts" 2>/dev/null || true)"
  case "$es"   in *'"count":'[1-9]*) es_ok=1 ;; *) es_ok=0 ;; esac
  case "$prom" in *'"value":'*)       prom_ok=1 ;; *) prom_ok=0 ;; esac
  case "$hist" in *'"value":'*)       hist_ok=1 ;; *) hist_ok=0 ;; esac
  if [ "$es_ok" = 1 ] && [ "$prom_ok" = 1 ] && [ "$hist_ok" = 1 ]; then echo "    ready."; break; fi
  i=$((i + 1)); sleep 5
done
if [ "${es_ok:-0}" != 1 ] || [ "${prom_ok:-0}" != 1 ] || [ "${hist_ok:-0}" != 1 ]; then
  echo "    (still warming up — backfill/Elasticsearch/Prometheus may need another minute)"
fi

cat <<EOF

==> Demo stack is up (telemetry spans the ~72h incident window):
    UModel         http://localhost:8080   object graph + plan provider (workspace 'demo')
    Prometheus     http://localhost:9090   72h backfilled history + live tail
    Elasticsearch  http://localhost:9200   72h of logs (healthy INFO -> ERROR flood)

==> Run a live RCA with an agent (umodel-query + umodel-rca skills):
    export UMCTL_ADDR=http://localhost:8080
    Then ask, e.g.:
      "payment-gateway is degraded — find the root cause."
    The agent reads its metrics/logs (get_metrics / get_logs run against the real
    Prometheus / Elasticsearch above), traverses the topology to checkout-service's
    retry config change and the 618 promotion, and concludes the retry storm.

    Smoke-test without an agent:  sh "$DIR/verify.sh"
    Stop & clean up:              sh "$DIR/stop.sh"   (add --all to also remove the image)
EOF
