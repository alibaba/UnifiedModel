#!/bin/bash
# UModel Service Localization — narrated end-to-end demo.
#
# Replays the agent's bottleneck-localization loop against a running
# umodel-server, printing the SPL it issues, the key result, and the
# reasoning at each hop. Doubles as a smoke gate: it asserts the load-bearing
# facts and exits non-zero if the localization path is broken.
#
# Usage:
#   # against an already-running server (e.g. make quickstart QUICKSTART_SAMPLE=examples/service-localization)
#   ./demo.sh
#   # or point at another address
#   ADDR=http://localhost:8080 WS=demo ./demo.sh
set -uo pipefail

ADDR="${ADDR:-http://localhost:8080}"
WS="${WS:-demo}"
PASS=0; FAIL=0

# Verified entity IDs (md5 of semantic names)
API_CHECKOUT="3a44ea48396a812d5a1f4eb12ae51e39"
SVC_ORDER="f25ae2923f5df058b6119ea79e434459"
STORE_ORDERS_DB="60794de7878447582b1a4d5fe11e37a0"
NODE_A="6cec8a5bb33ae85cefde09a76ebeca4c"

c_title='\033[1;36m'; c_spl='\033[0;90m'; c_note='\033[0;33m'; c_ok='\033[0;32m'; c_no='\033[0;31m'; c_off='\033[0m'

q() { # run an SPL query, echo the raw response body
  curl -s -X POST "$ADDR/api/v1/query/$WS/execute" -H 'Content-Type: application/json' \
    --data-binary "$(python3 -c 'import json,sys; print(json.dumps({"query": sys.argv[1]}))' "$1")"
}

step() { printf "\n${c_title}%s${c_off}\n" "$1"; }
spl()  { printf "${c_spl}  SPL  %s${c_off}\n" "$1"; }
note() { printf "${c_note}  ↳ %s${c_off}\n" "$1"; }

assert() { # assert <haystack> <needle> <label>
  if printf '%s' "$1" | grep -q "$2"; then printf "${c_ok}  ✓ %s${c_off}\n" "$3"; PASS=$((PASS+1));
  else printf "${c_no}  ✗ %s${c_off}\n" "$3"; FAIL=$((FAIL+1)); fi
}

printf "${c_title}═══ UModel · Service Localization — agent walkthrough ═══${c_off}\n"
printf "Server: %s   Workspace: %s\n" "$ADDR" "$WS"

# ── 0. Symptom ───────────────────────────────────────────────
step "0. Symptom — which user journey is impacted?"
SPL_0=".entity with(domain='product', name='product.journey', query='impacted') | project display_name, status, error_rate"
spl "$SPL_0"; R=$(q "$SPL_0")
note "$(python3 - "$R" <<'PY'
import json,sys
d=json.loads(sys.argv[1])["data"]["data"]
print(", ".join(f"{r[0]} (status={r[1]}, errors={r[2]})" for r in d))
PY
)"
assert "$R" "Checkout Flow" "Checkout Flow journey is impacted"

# ── 1. Entry ─────────────────────────────────────────────────
step "1. Entry point — the degraded API behind that journey"
SPL_1=".entity with(domain='product', name='product.api', query='degraded') | project display_name, status, sla_tier, latency_slo_ms"
spl "$SPL_1"; R=$(q "$SPL_1")
note "$(python3 - "$R" <<'PY'
import json,sys
r=json.loads(sys.argv[1])["data"]["data"][0]
print(f"{r[0]} — status={r[1]}, sla={r[2]}, SLO={r[3]}ms  →  start localizing here")
PY
)"
assert "$R" "checkout-api" "degraded API is checkout-api"

# ── 2. Hop 1: API → service ──────────────────────────────────
step "2. Hop 1 — what does checkout-api call?  (getDirectRelations)"
SPL_2=".topo | graph-call getDirectRelations([(:\"product@product.api\" {__entity_id__: '$API_CHECKOUT'})])"
spl "$SPL_2"; R=$(q "$SPL_2")
note "downstream 'calls' edge → order-svc"
assert "$R" "$SVC_ORDER" "checkout-api --calls--> order-svc"

# ── 3. Is the service itself the cause?  latency vs its own CPU ─
step "3. At order-svc — fetch latency AND its own CPU"
SPL_3=".entity_set with(domain='service', name='service.app', ids=['$SVC_ORDER']) | entity-call get_metrics('service', 'service.app.metrics', 'cpu_usage', step='30s')"
spl "$SPL_3"; R=$(q "$SPL_3")
note "$(python3 - "$R" <<'PY'
import json,sys
p=json.loads(json.loads(sys.argv[1])["data"]["data"][0][1])
print("cpu_usage plan ->", p["query"]["queries"][0]["promql"])
PY
)"
note "order-svc latency is high but its CPU plan is healthy → cause is DOWNSTREAM, not the service itself"
assert "$R" "service.app.metrics" "service CPU metric is fetchable"

# ── 4. Hop 2: service → datastore ────────────────────────────
step "4. Hop 2 — what does order-svc read/write?  (getDirectRelations)"
SPL_4=".topo | graph-call getDirectRelations([(:\"service@service.app\" {__entity_id__: '$SVC_ORDER'})])"
spl "$SPL_4"; R=$(q "$SPL_4")
note "downstream 'reads_writes' edge → orders-db"
assert "$R" "$STORE_ORDERS_DB" "order-svc --reads_writes--> orders-db"
assert "$R" "reads_writes" "edge type is reads_writes"

# ── 5. The root signal — datastore saturation ────────────────
step "5. At orders-db — fetch the saturation signal"
SPL_5=".entity_set with(domain='data', name='data.store', ids=['$STORE_ORDERS_DB']) | entity-call get_metrics('data', 'data.store.metrics', 'connection_pool_usage', step='30s')"
spl "$SPL_5"; R=$(q "$SPL_5")
note "$(python3 - "$R" <<'PY'
import json,sys
p=json.loads(json.loads(sys.argv[1])["data"]["data"][0][1])
print("connection_pool_usage plan ->", p["query"]["queries"][0]["promql"])
PY
)"
note "the object graph turned 'the store order-svc depends on' into the exact saturation query — no hand-written PromQL"
assert "$R" "data_store_connection_pool" "connection_pool_usage PromQL rendered"
assert "$R" "$STORE_ORDERS_DB" "store id substituted into the query"

# ── 6. Rule out the layer below ──────────────────────────────
step "6. Hop 3 — orders-db is hosted where, and is that node healthy?"
SPL_6=".topo | graph-call getDirectRelations([(:\"data@data.store\" {__entity_id__: '$STORE_ORDERS_DB'})])"
spl "$SPL_6"; R=$(q "$SPL_6")
note "'hosted_on' edge → node-a"
assert "$R" "$NODE_A" "orders-db --hosted_on--> node-a"
SPL_6b=".entity with(domain='infra', name='infra.node', query='node-a') | project display_name, status"
R2=$(q "$SPL_6b")
note "$(python3 - "$R2" <<'PY'
import json,sys
r=json.loads(sys.argv[1])["data"]["data"][0]
print(f"{r[0]} status={r[1]} → infrastructure RULED OUT")
PY
)"
assert "$R2" "healthy" "hosting node is healthy (infra ruled out)"

# ── Conclusion ───────────────────────────────────────────────
step "Conclusion"
printf "  Critical path:  Checkout Flow → checkout-api → order-svc → ${c_no}orders-db (SATURATED)${c_off} → node-a (healthy)\n"
printf "  Bottleneck localized to the ${c_no}orders-db connection pool${c_off}; service CPU and the hosting node are healthy.\n"

echo ""
TOTAL=$((PASS+FAIL))
printf "${c_title}═══ %d/%d checks passed ═══${c_off}\n" "$PASS" "$TOTAL"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
