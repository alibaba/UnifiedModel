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
    ((PASS++))
  else
    echo "✗ $test_name"
    echo "  expected to contain: $expected"
    if [ "$VERBOSE" = "--verbose" ]; then
      echo "  actual: $result"
    fi
    ((FAIL++))
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
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".entity with(domain='"'"'platform'"'"', name='"'"'platform.service'"'"', query='"'"'degraded'"'"')"}}}')
assert_contains "$result" "payment-gateway" "Step 1: degraded service = payment-gateway"
assert_contains "$result" "degraded" "Step 1: status is degraded"

# Step 2: Find upstream callers via topology
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".topo | graph-call getNeighborNodes('"'"'full'"'"', 1, [(:\"platform@platform.service\" {__entity_id__: '"'"'63718b78868895d2590551b27ec6f51c'"'"'})]) | with(__relation_type__='"'"'calls'"'"')"}}}')
assert_contains "$result" "checkout" "Step 2: upstream caller includes checkout"
assert_contains "$result" "calls" "Step 2: relation type is calls"

# Step 3: Find config change on upstream
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".entity with(domain='"'"'platform'"'"', name='"'"'platform.config_change'"'"', query='"'"'checkout'"'"')"}}}')
assert_contains "$result" "retry" "Step 3: config change involves retry"

# Step 4: Find active promotion (business layer)
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".entity with(domain='"'"'business'"'"', name='"'"'business.promotion'"'"', query='"'"'active'"'"')"}}}')
assert_contains "$result" "618" "Step 4: active promotion is 618 Flash Sale"

# Step 5: Check red herring deployment
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".entity with(domain='"'"'platform'"'"', name='"'"'platform.deployment'"'"', query='"'"'payment'"'"')"}}}')
assert_contains "$result" "v3.2.1" "Step 5: red herring deployment v3.2.1 exists"

# Step 6: Check business impact
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".entity with(domain='"'"'business'"'"', name='"'"'business.order_flow'"'"', query='"'"'impacted'"'"')"}}}')
assert_contains "$result" "purchase" "Step 6: impacted order flow includes purchase"

echo ""

# --- Model Tests ---
echo "── Model Definitions ──"

# Runbook set
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".umodel with(kind='"'"'runbook_set'"'"', name='"'"'platform.service.ops'"'"')"}}}')
assert_contains "$result" "upstream_retry_amplification" "Runbook: observation upstream_retry_amplification"
assert_contains "$result" "recent_deployment_correlation" "Runbook: observation recent_deployment_correlation"
assert_contains "$result" "business_traffic_pressure" "Runbook: observation business_traffic_pressure"

# Entity set link (cross-domain)
result=$(run_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"workspace":"demo","spl":".umodel with(kind='"'"'entity_set_link'"'"')"}}}')
assert_contains "$result" "result" "Entity set links queryable"

echo ""

# --- Summary ---
echo "═══════════════════════════════════════════════════════════"
TOTAL=$((PASS + FAIL))
echo " Results: $PASS/$TOTAL passed, $FAIL failed"
echo "═══════════════════════════════════════════════════════════"

[ $FAIL -eq 0 ] && exit 0 || exit 1
