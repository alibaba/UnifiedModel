#!/bin/sh
# Smoke-test the incident-investigation demo stack after `start.sh`.
# Walks the RCA evidence chain: UModel returns a plan -> run it against the real
# Prometheus / Elasticsearch (endpoints already point at localhost). Sections 1-4 query the
# present (served by the live exporter); section 5 evaluates the backfilled 72h history at
# past instants to show the incident arc. Requires: jq, curl, and either `umctl` on PATH or
# a Go toolchain (falls back to `go run ./cmd/umctl`).
set -eu

# Honour the same host-port overrides as start.sh (UMODEL_PORT / PROM_PORT / ES_PORT),
# or a full URL override (UM_URL / PROM_URL / ES_URL).
UM="${UM_URL:-http://localhost:${UMODEL_PORT:-8080}}"
PROM="${PROM_URL:-http://localhost:${PROM_PORT:-9090}}"
ES="${ES_URL:-http://localhost:${ES_PORT:-9200}}"
PG="63718b78868895d2590551b27ec6f51c"   # payment-gateway
CK="149632df43354373835df2717cb8fb19"   # checkout-service
NOW=$(date +%s)

uctl() {
  if command -v umctl >/dev/null 2>&1; then umctl "$@"; else
    (cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" && go run ./cmd/umctl "$@"); fi
}
metric() { # entity_id, metric -> promql -> run now
  q=$(uctl --addr "$UM" query run demo ".entity_set with(domain='platform', name='platform.service', ids=['$1']) | entity-call get_metrics('platform','platform.service.metrics','$2', step='30s')" -o json | jq -r '.data.data[0][1] | fromjson | .query.queries[0].promql')
  v=$(curl -sG "$PROM/api/v1/query" --data-urlencode "query=$q" | jq -r '.data.result[0].value[1] // "no data yet (wait ~1 min for scrapes)"')
  printf "   %-22s %s\n" "$2" "$v"
}
arc() { # label, promql, seconds-ago -> evaluate history at a past instant
  ts=$((NOW - $3))
  v=$(curl -sG "$PROM/api/v1/query" --data-urlencode "query=$2" --data-urlencode "time=$ts" | jq -r '.data.result[0].value[1] // "no data"')
  printf "   %-24s %s\n" "$1" "$v"
}

echo "== 1) UModel: degraded services =="
uctl --addr "$UM" query run demo ".entity with(domain='platform', name='platform.service', query='degraded') | project display_name, latency_p99_ms, error_rate" -o json | jq -c '.data.data'

echo "== 2) payment-gateway signals now (plan -> PromQL -> $PROM) =="
for m in latency_p99_ms error_rate upstream_timeout_rate; do metric "$PG" "$m"; done

echo "== 3) checkout-service retry signal now — the root cause (plan -> PromQL) =="
metric "$CK" client_retry_rate

echo "== 4) payment-gateway ERROR logs (plan -> _search -> $ES) =="
body=$(uctl --addr "$UM" query run demo ".entity_set with(domain='platform', name='platform.service', ids=['$PG']) | entity-call get_logs('platform','platform.service.logs', query='level = \"ERROR\"')" -o json | jq -r '.data.data[0][1] | fromjson | .query.body')
curl -s "$ES/platform-service-logs-*/_search" -H 'Content-Type: application/json' -d "$body" | jq -r '.hits.hits[]._source | "   \(.severity)\t\(.upstream_service)\t\(.log_message)"' | head -8

echo "== 5) the 72h incident arc (history backfilled into Prometheus) =="
PG_P99="histogram_quantile(0.99, sum by(le)(rate(platform_service_request_duration_seconds_bucket{service_id=\"$PG\"}[30m])))*1000"
echo "   payment-gateway p99 (ms) — flat through the T-12h deploy, breaches at the T-4h promo:"
arc "t-60h healthy"   "$PG_P99" 216000
arc "t-12h deploy"    "$PG_P99" 43200
arc "t-1h  breach"    "$PG_P99" 3600
CK_RETRY="rate(platform_service_client_retry_total{service_id=\"$CK\"}[30m]) / rate(platform_service_client_request_total{service_id=\"$CK\"}[30m])"
echo "   checkout client_retry_rate — steps up at the T-24h config change (2->5), not before:"
arc "t-30h pre-config"  "$CK_RETRY" 108000
arc "t-12h post-config" "$CK_RETRY" 43200
arc "now"               "$CK_RETRY" 0

echo "== 6) follow one failing request across the stack (correlated trace) =="
tid=$(curl -s "$ES/platform-service-logs-*/_search" -H 'Content-Type: application/json' \
  -d '{"size":1,"query":{"bool":{"filter":[{"term":{"svc_id":"'"$PG"'"}},{"term":{"error_code":"UPSTREAM_TIMEOUT"}}]}}}' \
  | jq -r '.hits.hits[0]._source.trace_id // empty')
if [ -n "$tid" ]; then
  echo "   trace_id=$tid  (checkout -> payment-gateway -> payment-router -> channel -> provider)"
  curl -s "$ES/platform-service-logs-*/_search" -H 'Content-Type: application/json' \
    -d '{"size":10,"sort":[{"latency_ms":"desc"}],"query":{"term":{"trace_id":"'"$tid"'"}}}' \
    | jq -r '.hits.hits[]._source | "   \(.severity) \(.http_status) up=\(.upstream_service) \(.log_message)"'
else
  echo "   (no correlated trace found yet)"
fi

echo "== done =="
