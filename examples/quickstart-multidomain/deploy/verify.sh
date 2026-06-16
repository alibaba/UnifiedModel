#!/bin/sh
# Smoke-test the quickstart demo stack after `docker compose ... up`.
# Demonstrates the full read chain: UModel returns a plan -> run it against the real
# Prometheus / Elasticsearch (overriding the plan's placeholder endpoint, as the agent does).
# Requires: jq, and either `umctl` on PATH or a Go toolchain (falls back to `go run ./cmd/umctl`).
set -eu

UM="${UM_URL:-http://localhost:8080}"
PROM="${PROM_URL:-http://localhost:9090}"
ES="${ES_URL:-http://localhost:9200}"
ID="${SERVICE_ID:-10000000000000000000000000000101}"   # checkout-service

uctl() {
  if command -v umctl >/dev/null 2>&1; then
    umctl "$@"
  else
    (cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)" && go run ./cmd/umctl "$@")
  fi
}

echo "== 1) UModel: devops.service objects =="
uctl --addr "$UM" query run demo ".entity with(domain='devops', name='devops.service') | project __entity_id__, display_name, status" -o json | jq -c '.data.data'

echo "== 2) get_metrics plan -> run the PromQL against $PROM =="
for m in request_count error_count latency_ms; do
  promql=$(uctl --addr "$UM" query run demo ".entity_set with(domain='devops', name='devops.service', ids=['$ID']) | entity-call get_metrics('devops','devops.metric.service','$m', step='30s')" -o json | jq -r '.data.data[0][1] | fromjson | .query.queries[0].promql')
  val=$(curl -sG "$PROM/api/v1/query" --data-urlencode "query=$promql" | jq -r '.data.result[0].value[1] // "no data yet (wait ~1 min for scrapes)"')
  printf "   %-14s %s\n" "$m" "$val"
done

echo "== 3) get_logs plan -> run the _search against $ES =="
body=$(uctl --addr "$UM" query run demo ".entity_set with(domain='devops', name='devops.service', ids=['$ID']) | entity-call get_logs('devops','devops.log.service', query='level = \"ERROR\"')" -o json | jq -r '.data.data[0][1] | fromjson | .query.body')
curl -s "$ES/devops-service-logs-*/_search" -H 'Content-Type: application/json' -d "$body" | jq -r '.hits.hits[]._source | "\(.severity)\t\(.log_message)"'

echo "== done =="
