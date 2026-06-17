#!/usr/bin/env sh
# One-command bring-up for the incident-investigation RCA demo:
#   UModel (serving the incident-investigation pack) + a seeded Prometheus + a seeded
#   Elasticsearch, matching the modeled incident. Connect an agent with the umodel-query
#   + umodel-rca skills and run a live root-cause analysis. See ../README.md.
set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
COMPOSE_FILE="$DIR/docker-compose.yml"

# Host ports. Override any of these to run the demo alongside an existing umodel-server /
# Prometheus / Elasticsearch instead of colliding with it, e.g.:
#   UMODEL_PORT=18080 PROM_PORT=19090 ES_PORT=19200 sh start.sh
UMODEL_PORT="${UMODEL_PORT:-8080}"
PROM_PORT="${PROM_PORT:-9090}"
ES_PORT="${ES_PORT:-9200}"
export UMODEL_PORT PROM_PORT ES_PORT     # consumed by docker-compose.yml port mappings
UM_URL="http://localhost:$UMODEL_PORT"
PROM_URL="http://localhost:$PROM_PORT"
ES_URL="http://localhost:$ES_PORT"

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

# Preflight: refuse to start if a host port is already taken. Otherwise the stack can come up
# while localhost:<port> actually points at an unrelated service (a previous demo, or a local
# umodel-server / Prometheus / Elasticsearch), and the readiness/verify checks below would
# silently talk to the wrong instance and report a misleading result.
port_busy() {   # $1 = port; true if something is already listening
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
  else
    curl -s -o /dev/null --max-time 2 "http://127.0.0.1:$1"
  fi
}
busy=""
for p in "$UMODEL_PORT" "$PROM_PORT" "$ES_PORT"; do
  if port_busy "$p"; then busy="$busy $p"; fi
done
if [ -n "$busy" ]; then
  echo "error: host port(s) already in use:$busy" >&2
  echo "       Something is already listening there — a previous demo, or a local" >&2
  echo "       umodel-server / Prometheus / Elasticsearch. The demo will not start on top of it." >&2
  echo "       Fix it one of these ways:" >&2
  echo "         - if it is a previous run:   sh \"$DIR/stop.sh\"" >&2
  echo "         - stop the conflicting service, or pick free ports:" >&2
  echo "             UMODEL_PORT=18080 PROM_PORT=19090 ES_PORT=19200 sh \"$DIR/start.sh\"" >&2
  exit 1
fi

echo "==> Bringing up the incident-investigation demo stack with: $COMPOSE"
# shellcheck disable=SC2086
$COMPOSE -f "$COMPOSE_FILE" up -d --build

echo "==> Waiting for the 72h metric backfill + Elasticsearch seed + Prometheus scrapes (up to ~5 min)..."
hist_ts=$(( $(date +%s) - 216000 ))   # 60h ago: confirms the history was backfilled
# Sentinel query: confirms localhost:$UMODEL_PORT is THIS demo (returns the degraded payment
# path), not some other umodel. A quoted heredoc keeps the SPL single quotes literal, no escaping.
um_body() { cat <<'JSON'
{"query":".entity with(domain='platform', name='platform.service', query='degraded')"}
JSON
}
i=0
while [ "$i" -lt 60 ]; do
  um="$(um_body | curl -s --max-time 4 -X POST "$UM_URL/api/v1/query/demo/execute" \
          -H 'Content-Type: application/json' --data-binary @- 2>/dev/null || true)"
  es="$(curl -s --max-time 3 "$ES_URL/platform-service-logs-*/_count" 2>/dev/null || true)"
  prom="$(curl -s --max-time 3 -G "$PROM_URL/api/v1/query" \
            --data-urlencode 'query=sum(rate(platform_service_request_total[1m]))' 2>/dev/null || true)"
  hist="$(curl -s --max-time 3 -G "$PROM_URL/api/v1/query" \
            --data-urlencode 'query=platform_service_request_total' \
            --data-urlencode "time=$hist_ts" 2>/dev/null || true)"
  case "$um"   in *payment-gateway*)  um_ok=1 ;;   *) um_ok=0 ;; esac
  case "$es"   in *'"count":'[1-9]*)  es_ok=1 ;;   *) es_ok=0 ;; esac
  case "$prom" in *'"value":'*)       prom_ok=1 ;; *) prom_ok=0 ;; esac
  case "$hist" in *'"value":'*)       hist_ok=1 ;; *) hist_ok=0 ;; esac
  if [ "$um_ok" = 1 ] && [ "$es_ok" = 1 ] && [ "$prom_ok" = 1 ] && [ "$hist_ok" = 1 ]; then echo "    ready."; break; fi
  i=$((i + 1)); sleep 5
done
if [ "${um_ok:-0}" != 1 ] || [ "${es_ok:-0}" != 1 ] || [ "${prom_ok:-0}" != 1 ] || [ "${hist_ok:-0}" != 1 ]; then
  echo "    (still warming up — UModel/backfill/Elasticsearch/Prometheus may need another minute)"
fi

# Pass the resolved ports to verify.sh only when they differ from the defaults.
VENV=""
if [ "$UMODEL_PORT" != 8080 ] || [ "$PROM_PORT" != 9090 ] || [ "$ES_PORT" != 9200 ]; then
  VENV="UMODEL_PORT=$UMODEL_PORT PROM_PORT=$PROM_PORT ES_PORT=$ES_PORT "
fi

cat <<EOF

==> Demo stack is up (telemetry spans the ~72h incident window):
    UModel         $UM_URL   object graph + plan provider (workspace 'demo')
    Prometheus     $PROM_URL   72h backfilled history + live tail
    Elasticsearch  $ES_URL   72h of logs (healthy INFO -> ERROR flood)

==> Run a live RCA with an agent (umodel-query + umodel-rca skills):
    export UMCTL_ADDR=$UM_URL
    Then ask, e.g.:
      "payment-gateway is degraded — find the root cause."
    The agent reads its metrics/logs (get_metrics / get_logs run against the real
    Prometheus / Elasticsearch above), traverses the topology to checkout-service's
    retry config change and the flash-sale promotion, and concludes the retry storm.

    Smoke-test without an agent:  ${VENV}sh "$DIR/verify.sh"
    Stop & clean up:              sh "$DIR/stop.sh"   (add --all to also remove the image)
EOF
