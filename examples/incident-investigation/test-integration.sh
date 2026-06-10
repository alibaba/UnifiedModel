#!/bin/bash
set -euo pipefail

# UModel Incident Investigation — MCP Integration Test
# Validates that the MCP server correctly serves the demo data
# and that the full investigation path is queryable.
#
# Usage:
#   ./test-integration.sh
#   ./test-integration.sh --verbose

VERBOSE=${1:-""}
MCP_CMD="go run ./cmd/umodel-mcp --quickstart --quickstart-sample incident-investigation --graphstore memory"
PASS=0
FAIL=0

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

echo "═══════════════════════════════════════════════════════════"
echo " UModel MCP Integration Test — Incident Investigation"
echo "═══════════════════════════════════════════════════════════"
echo ""

# --- Protocol Tests ---
echo "── Protocol ──"

result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}')
assert_contains "$result" '"result"' "MCP initialize returns result"

result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}')
assert_contains "$result" "query_spl_execute" "tools/list contains query_spl_execute"
assert_contains "$result" "query_spl_examples" "tools/list contains query_spl_examples"

result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}')
assert_contains "$result" "result" "resources/list returns result"

echo ""

# --- Investigation Path Tests ---
echo "── Investigation Path ──"

# Step 1: Find degraded service
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity with(domain='"'"'platform'"'"', name='"'"'platform.service'"'"', query='"'"'degraded'"'"')"}}}')
assert_contains "$result" "payment-gateway" "Step 1: degraded service = payment-gateway"
assert_contains "$result" "degraded" "Step 1: status is degraded"

# Step 2: Find upstream callers via topology
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".topo | graph-call getNeighborNodes('"'"'full'"'"', 1, [(:\"platform@platform.service\" {__entity_id__: '"'"'63718b78868895d2590551b27ec6f51c'"'"'})]) | with(__relation_type__='"'"'calls'"'"')"}}}')
# Topology returns entity IDs + relation properties, not display_names, so we
# assert checkout-service's entity_id (149632df…) is among the upstream callers.
assert_contains "$result" "149632df43354373835df2717cb8fb19" "Step 2: upstream caller includes checkout-service"
assert_contains "$result" "calls" "Step 2: relation type is calls"

# Step 3: Find config change on upstream
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity with(domain='"'"'platform'"'"', name='"'"'platform.config_change'"'"', query='"'"'checkout'"'"')"}}}')
assert_contains "$result" "retry" "Step 3: config change involves retry"

# Step 4: Find active promotion (business layer)
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity with(domain='"'"'business'"'"', name='"'"'business.promotion'"'"', query='"'"'active'"'"')"}}}')
assert_contains "$result" "618" "Step 4: active promotion is 618 Flash Sale"

# Step 5: Check red herring deployment
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity with(domain='"'"'platform'"'"', name='"'"'platform.deployment'"'"', query='"'"'payment'"'"')"}}}')
assert_contains "$result" "v3.2.1" "Step 5: red herring deployment v3.2.1 exists"

# Step 6: Check business impact
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity with(domain='"'"'business'"'"', name='"'"'business.order_flow'"'"', query='"'"'impacted'"'"')"}}}')
assert_contains "$result" "Purchase" "Step 6: impacted order flow includes Standard Purchase Flow"

echo ""

# --- Model Tests ---
echo "── Model Definitions ──"

# Runbook set
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".umodel with(kind='"'"'runbook_set'"'"', name='"'"'platform.service.ops'"'"')"}}}')
assert_contains "$result" "upstream_retry_amplification" "Runbook: observation upstream_retry_amplification"
assert_contains "$result" "recent_deployment_correlation" "Runbook: observation recent_deployment_correlation"
assert_contains "$result" "business_traffic_pressure" "Runbook: observation business_traffic_pressure"

# Entity set link (cross-domain)
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".umodel with(kind='"'"'entity_set_link'"'"')"}}}')
assert_contains "$result" "result" "Entity set links queryable"

echo ""

# --- Telemetry Tests ---
echo "── Telemetry (get_metrics / get_logs) ──"

# get_metrics: P99 latency plan renders PromQL with the service_id substituted
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity_set with(domain='"'"'platform'"'"', name='"'"'platform.service'"'"', ids=['"'"'63718b78868895d2590551b27ec6f51c'"'"']) | entity-call get_metrics('"'"'platform'"'"', '"'"'platform.service.metrics'"'"', '"'"'latency_p99_ms'"'"', step='"'"'30s'"'"')"}}}')
assert_contains "$result" "get_metrics" "get_metrics: plan operation"
assert_contains "$result" "platform.service.metrics" "get_metrics: targets the metric set"
assert_contains "$result" "histogram_quantile" "get_metrics: renders P99 PromQL"
assert_contains "$result" "63718b78868895d2590551b27ec6f51c" "get_metrics: substitutes service_id from entity"

# get_logs: error-level log plan resolves to the elasticsearch-backed log set
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity_set with(domain='"'"'platform'"'"', name='"'"'platform.service'"'"', ids=['"'"'63718b78868895d2590551b27ec6f51c'"'"']) | entity-call get_logs('"'"'platform'"'"', '"'"'platform.service.logs'"'"', query='"'"'level = \"ERROR\"'"'"')"}}}')
assert_contains "$result" "get_logs" "get_logs: plan operation"
assert_contains "$result" "platform.service.logs" "get_logs: targets the log set"

# list_data_set: service now exposes both datasets
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","query":".entity_set with(domain='"'"'platform'"'"', name='"'"'platform.service'"'"', ids=['"'"'63718b78868895d2590551b27ec6f51c'"'"']) | entity-call list_data_set(['"'"'metric_set'"'"', '"'"'log_set'"'"'], true)"}}}')
assert_contains "$result" "platform.service.metrics" "list_data_set: includes metric set"
assert_contains "$result" "platform.service.logs" "list_data_set: includes log set"

echo ""

# --- Summary ---
echo "═══════════════════════════════════════════════════════════"
TOTAL=$((PASS + FAIL))
echo " Results: $PASS/$TOTAL passed, $FAIL failed"
echo "═══════════════════════════════════════════════════════════"

[ $FAIL -eq 0 ] && exit 0 || exit 1
