#!/bin/bash
set -euo pipefail

# UModel Service Localization — MCP Integration Test
# Validates that the MCP server serves the demo data and that the full
# bottleneck-localization path (product -> service -> data -> infra) is
# queryable, including the connection_pool_usage saturation signal.
#
# Usage:
#   ./test-integration.sh
#   ./test-integration.sh --verbose

VERBOSE=${1:-""}
MCP_CMD="go run ./cmd/umodel-mcp --quickstart --quickstart-sample service-localization --graphstore memory"
PASS=0
FAIL=0

# Verified entity IDs (md5 of semantic names)
API_CHECKOUT="3a44ea48396a812d5a1f4eb12ae51e39"
SVC_ORDER="f25ae2923f5df058b6119ea79e434459"
STORE_ORDERS_DB="60794de7878447582b1a4d5fe11e37a0"
NODE_A="6cec8a5bb33ae85cefde09a76ebeca4c"

run_mcp() {
  local input="$1"
  if [ "$VERBOSE" = "--verbose" ]; then
    echo "  → $input" >&2
  fi
  echo "$input" | $MCP_CMD 2>/dev/null
}

assert_contains() {
  local result="$1" expected="$2" test_name="$3"
  if echo "$result" | grep -q "$expected"; then
    echo "✓ $test_name"
    PASS=$((PASS + 1))
  else
    echo "✗ $test_name"
    echo "  expected to contain: $expected"
    if [ "$VERBOSE" = "--verbose" ]; then
      echo "  actual: $result"
    fi
    FAIL=$((FAIL + 1))
  fi
}

call() {
  # $1 = SPL query
  run_mcp "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"query_spl_execute\",\"arguments\":{\"workspace\":\"demo\",\"query\":$1}}}"
}

echo "═══════════════════════════════════════════════════════════"
echo " UModel MCP Integration Test — Service Localization"
echo "═══════════════════════════════════════════════════════════"
echo ""

# --- Protocol ---
echo "── Protocol ──"
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}')
assert_contains "$result" '"result"' "MCP initialize returns result"
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}')
assert_contains "$result" "query_spl_execute" "tools/list contains query_spl_execute"
echo ""

# --- Localization Path ---
echo "── Localization Path ──"

# Entry: degraded checkout-api
result=$(call '".entity with(domain='"'"'product'"'"', name='"'"'product.api'"'"', query='"'"'degraded'"'"')"')
assert_contains "$result" "checkout-api" "Entry: degraded API is checkout-api"

# Hop 1: checkout-api -> order-svc (calls)
result=$(call "\".topo | graph-call getDirectRelations([(:\\\"product@product.api\\\" {__entity_id__: '$API_CHECKOUT'})])\"")
assert_contains "$result" "$SVC_ORDER" "Hop 1: checkout-api calls order-svc"
assert_contains "$result" "calls" "Hop 1: relation type is calls"

# Hop 2: order-svc -> orders-db (reads_writes)
result=$(call "\".topo | graph-call getDirectRelations([(:\\\"service@service.app\\\" {__entity_id__: '$SVC_ORDER'})])\"")
assert_contains "$result" "$STORE_ORDERS_DB" "Hop 2: order-svc reads/writes orders-db"
assert_contains "$result" "reads_writes" "Hop 2: relation type is reads_writes"

# Hop 3: orders-db -> node-a (hosted_on)
result=$(call "\".topo | graph-call getDirectRelations([(:\\\"data@data.store\\\" {__entity_id__: '$STORE_ORDERS_DB'})])\"")
assert_contains "$result" "$NODE_A" "Hop 3: orders-db hosted on node-a"
assert_contains "$result" "hosted_on" "Hop 3: relation type is hosted_on"
echo ""

# --- Telemetry Retrieval ---
echo "── Telemetry Retrieval ──"

# Datasets discoverable on the service entity set
result=$(call '".entity_set with(domain='"'"'service'"'"', name='"'"'service.app'"'"') | entity-call list_data_set(['"'"'metric_set'"'"', '"'"'log_set'"'"'], true)"')
assert_contains "$result" "service.app.metrics" "list_data_set: service metric set present"
assert_contains "$result" "service.app.logs" "list_data_set: service log set present"

# The root signal: connection_pool_usage renders PromQL with orders-db id substituted
result=$(call "\".entity_set with(domain='data', name='data.store', ids=['$STORE_ORDERS_DB']) | entity-call get_metrics('data', 'data.store.metrics', 'connection_pool_usage', step='30s')\"")
assert_contains "$result" "get_metrics" "get_metrics: plan operation"
assert_contains "$result" "data.store.metrics" "get_metrics: targets the store metric set"
assert_contains "$result" "data_store_connection_pool" "get_metrics: renders connection pool PromQL"
assert_contains "$result" "$STORE_ORDERS_DB" "get_metrics: substitutes orders-db id"

# Service CPU is fetchable (used to rule order-svc's own resources in/out)
result=$(call "\".entity_set with(domain='service', name='service.app', ids=['$SVC_ORDER']) | entity-call get_metrics('service', 'service.app.metrics', 'cpu_usage', step='30s')\"")
assert_contains "$result" "service.app.metrics" "get_metrics: service cpu_usage targets service metrics"
echo ""

# --- Summary ---
echo "═══════════════════════════════════════════════════════════"
TOTAL=$((PASS + FAIL))
echo " Results: $PASS/$TOTAL passed, $FAIL failed"
echo "═══════════════════════════════════════════════════════════"

[ $FAIL -eq 0 ] && exit 0 || exit 1
