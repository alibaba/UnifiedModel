#!/bin/sh
# Smoke-test the incident-investigation demo stack after `start.sh`.
# Walks the RCA evidence chain: UModel returns a plan -> run it against the real
# Prometheus / Elasticsearch (endpoints already point at localhost). Requires: jq,
# and either `umctl` on PATH or a Go toolchain (falls back to `go run ./cmd/umctl`).
set -eu

UM="${UM_URL:-http://localhost:8080}"
PROM="${PROM_URL:-http://localhost:9090}"
ES="${ES_URL:-http://localhost:9200}"
PG="63718b78868895d2590551b27ec6f51c"   # payment-gateway
CK="149632df43354373835df2717cb8fb19"   # checkout-service

uctl() {
  if command -v umctl >/dev/null 2>&1; then umctl "$@"; else
    (cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" && go run ./cmd/umctl "$@"); fi
}
metric() { # entity_id, metric -> promql -> run
  q=$(uctl --addr "$UM" query run demo ".entity_set with(domain='platform', name='platform.service', ids=['$1']) | entity-call get_metrics('platform','platform.service.metrics','$2', step='30s')" -o json | jq -r '.data.data[0][1] | fromjson | .query.queries[0].promql')
  v=$(curl -sG "$PROM/api/v1/query" --data-urlencode "query=$q" | jq -r '.data.result[0].value[1] // "no data yet (wait ~1 min for scrapes)"')
  printf "   %-22s %s\n" "$2" "$v"
}

echo "== 1) UModel: degraded services =="
uctl --addr "$UM" query run demo ".entity with(domain='platform', name='platform.service', query='degraded') | project display_name, latency_p99_ms, error_rate" -o json | jq -c '.data.data'

echo "== 2) payment-gateway signals (plan -> PromQL -> $PROM) =="
for m in latency_p99_ms error_rate upstream_timeout_rate; do metric "$PG" "$m"; done

echo "== 3) checkout-service retry signal — the root cause (plan -> PromQL) =="
metric "$CK" client_retry_rate

echo "== 4) payment-gateway ERROR logs (plan -> _search -> $ES) =="
body=$(uctl --addr "$UM" query run demo ".entity_set with(domain='platform', name='platform.service', ids=['$PG']) | entity-call get_logs('platform','platform.service.logs', query='level = \"ERROR\"')" -o json | jq -r '.data.data[0][1] | fromjson | .query.body')
curl -s "$ES/platform-service-logs-*/_search" -H 'Content-Type: application/json' -d "$body" | jq -r '.hits.hits[]._source | "   \(.severity)\t\(.upstream_service)\t\(.log_message)"'

echo "== done =="
