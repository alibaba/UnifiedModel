# Incident Investigation Demo

A scenario-driven example showing how UModel's object graph + Runbook enables AI Agent-assisted incident investigation. A payment gateway SLO breach requires cross-domain topology traversal and runbook-guided diagnostics to resolve.

```
payment-gateway (degraded, platinum SLO)
  <- calls <- checkout-service
               <- affects <- cfg-checkout-retry (max_retries 2->5, 24h ago)
                              <- triggers <- 618 Flash Sale (3.5x traffic)

Ruled out: payment-gw v3.2.1 (12h ago, trivial logging change)
Root cause: 4000 x 3.5 x 2.5 = 35,000 QPS -> 8.75x overload
```

## Scenario

> **02:17 AM — payment-gateway P99 latency breaches SLO.**
> An oncall SRE (or AI Agent) must find the root cause.

### Timeline

| Time | Event |
|------|-------|
| T-48h | Promotion "618 Flash Sale" created, status=scheduled |
| T-24h | Config change `cfg-checkout-retry` applied: max_retries 2->5, timeout 500->2000ms |
| T-12h | Deployment `payment-gw v3.2.1` rolled out (trivial: logging format change) |
| T-4h | Promotion goes active, traffic ramp begins (3.5x multiplier) |
| T-7min | Incident `INC-0042` created: payment-gateway P99 > 2000ms |
| T-0 | **Investigation begins** |

### Root Cause

Upstream retry amplification x promotion traffic = cascading overload.

```
effective_load = base_qps x promotion_multiplier x (new_retries / old_retries)
             = 4000 x 3.5 x (5/2)
             = 35,000 QPS (8.75x normal capacity)
```

### Red Herring

`payment-gw v3.2.1` was deployed 12 hours ago. The oncall's first instinct is to blame it — but its `change_summary` reveals only logging format changes. The runbook's `recent_deployment_correlation` observation helps rule this out quickly.

## Prerequisites

- Go 1.22+
- Make
- Node.js 22+ (for Web UI)

## Quick Start

```bash
make quickstart QUICKSTART_SAMPLE=examples/incident-investigation
```

API: `http://localhost:8080` | Web UI: `http://localhost:5173`

Loads 3 domains (Platform / Runtime / Business), 11 entity sets, 65 entities, 83 relations, 1 runbook.

Alternative (API only, no Web UI):

```bash
go run ./cmd/umodel-server --quickstart --sample examples/incident-investigation
```

Docker:

```bash
docker build -t umodel-demo .
docker run -p 8080:8080 -p 5173:5173 \
  -e QUICKSTART_SAMPLE=examples/incident-investigation \
  umodel-demo
```

## Runbook Capabilities

The `platform.service.ops` runbook provides structured diagnostic protocol for AI agents:

| Type | Name | Purpose |
|------|------|---------|
| Observation | upstream_retry_amplification | Detect retry storm via topology + config_change correlation |
| Observation | recent_deployment_correlation | Rule out or confirm recent deployments (LLM judges change_summary) |
| Observation | business_traffic_pressure | Identify promotion-driven traffic amplification |
| Toolkit | config_management | rollback_config_change, apply_rate_limit |
| Toolkit | k8s_operations | scale_workload, restart_pods |
| Knowledge | retry_storm_pattern | Failure pattern explanation with calculation formula |
| Knowledge | deployment_triage_guide | How to avoid blaming innocent deployments |
| Automation | slo_breach_context_collector | Auto-collect context on SLO breach events |
| Skill | incident-investigation | Full investigation protocol for AI agents |

## Investigation Walkthrough

> **Note:** The `query=` parameter in `.entity` queries performs full-text search across all entity fields — just include the target text to match.

### Step 1: Identify the degraded service

```bash
umctl query run demo \
  ".entity with(domain='platform', name='platform.service', query='degraded') \
  | project display_name, status, owner, sla_tier"
```

Expected: `payment-gateway | degraded | payments-backend | platinum`

### Step 2: Check upstream callers via topology

```bash
umctl query run demo \
  ".topo | graph-call getNeighborNodes('full', 1, \
  [(:\"platform@platform.service\" {__entity_id__: '63718b78868895d2590551b27ec6f51c'})]) \
  | with(__relation_type__='calls')"
```

Expected: `checkout-service` and `order-service` call payment-gateway.

### Step 3: Find config changes on upstream

```bash
umctl query run demo \
  ".entity with(domain='platform', name='platform.config_change', query='checkout') \
  | project display_name, change_detail, applied_at"
```

Expected: `checkout-retry-increase` — max_retries 2->5, applied 24h ago.

### Step 4: Rule out recent deployment (red herring)

```bash
umctl query run demo \
  ".entity with(domain='platform', name='platform.deployment', query='payment') \
  | project display_name, change_summary, deployed_at"
```

Expected: `payment-gw v3.2.1 | Minor: updated logging format | 12h ago` — ruled out.

### Step 5: Check business traffic amplification (cross-domain)

```bash
umctl query run demo \
  ".entity with(domain='business', name='business.promotion', query='active') \
  | project display_name, traffic_multiplier, expected_peak_qps, actual_peak_qps"
```

Expected: `618 Flash Sale | 3.5 | 12000 | 38000` — actual traffic far exceeds plan.

### Step 6: Assess business impact (cross-domain)

```bash
umctl query run demo \
  ".entity with(domain='business', name='business.order_flow', query='impacted') \
  | project display_name, error_rate"
```

Expected: Standard Purchase Flow (3.2%) and Subscription Renewal (1.8%) impacted.

### Step 7: Load the runbook

```bash
umctl query run demo ".umodel with(kind='runbook_set', name='platform.service.ops')"
```

## Agent Integration

Connect an MCP client and ask:

> "payment-gateway SLO breached, help me investigate."

The agent follows the runbook protocol:

1. **Locate service** — `.entity` query finds `payment-gateway` (status=degraded, sla_tier=platinum)
2. **Load runbook** — discovers `runbook_link`, loads `platform.service.ops`
3. **Observation #1** (upstream_retry_amplification)
   - Topology finds upstream `checkout-service`
   - Config_change reveals `cfg-checkout-retry` (max_retries 2->5)
   - Conclusion: **Retry Storm Detected** (severity=error)
4. **Observation #2** (recent_deployment_correlation)
   - Deployment `payment-gw v3.2.1` found
   - LLM judges change_summary as trivial
   - Conclusion: **Deployment Ruled Out** (severity=info)
5. **Observation #3** (business_traffic_pressure)
   - Cross-domain query finds `618 Flash Sale` (actual 38000 > expected 12000)
   - Conclusion: **Promotion Traffic Exceeds Capacity** (severity=error)
6. **Correlation** — calculates amplification: 3.5 x (5/2) = 8.75x
7. **Knowledge** — loads `retry_storm_pattern`, confirms pattern match
8. **Recommendation** — `rollback_config_change` (risk=medium, confirmation_required=true)

### Agent Output Example

```
## Diagnosis

Based on Runbook platform.service.ops:

| Observation | Conclusion | Severity |
|-------------|-----------|----------|
| upstream_retry_amplification | Retry Storm Detected | error |
| recent_deployment_correlation | Deployment Ruled Out | info |
| business_traffic_pressure | Promotion Traffic Exceeds Capacity | error |

Root cause: checkout-service config change (retry 2->5) x 618 promotion (3.5x) = 8.75x overload

Recommended action:
  Tool: rollback_config_change
  Input: { config_change_id: "cfg-checkout-retry", target_service: "svc-checkout" }
  Risk: medium | Confirmation required: yes | ETA: 2-3 minutes
```

## MCP Connection

### Local (stdio)

Add to `.mcp.json`:

```json
{
  "mcpServers": {
    "umodel": {
      "command": "go",
      "args": [
        "run", "./cmd/umodel-mcp",
        "--quickstart",
        "--quickstart-sample", "examples/incident-investigation",
        "--graphstore", "memory"
      ]
    }
  }
}
```

### Remote (Streamable HTTP)

Start server:

```bash
go run ./cmd/umodel-mcp --quickstart \
  --quickstart-sample examples/incident-investigation \
  --graphstore file.memory \
  --transport http --addr 0.0.0.0:8090
```

Connect from `.mcp.json`:

```json
{
  "mcpServers": {
    "umodel": {
      "type": "streamable-http",
      "url": "http://<remote-host>:8090/mcp"
    }
  }
}
```

See the [MCP Reference](../../docs/en/reference/mcp.md) for transports (stdio, Streamable HTTP, HTTP+SSE), tools, resources, and a local smoke test.

## Contents

| Area | Path | Count | Purpose |
|------|------|------:|---------|
| Platform entity sets | `platform/entity_set/` | 5 | Services, deployments, config changes, incidents, teams |
| Platform links | `platform/link/entity_set_link/` | 5 | In-domain topology (calls, targets, owns, impacts, affects) |
| Runtime entity sets | `runtime/entity_set/` | 4 | Clusters, namespaces, workloads, pods |
| Runtime links | `runtime/link/entity_set_link/` | 3 | Containment hierarchy (contains, schedules) |
| Business entity sets | `business/entity_set/` | 2 | Promotions, order flows |
| Cross-domain links | `cross-domain/link/entity_set_link/` | 3 | Platform-Runtime, Platform-Business topology |
| Runbook set | `platform/runbook_set/` | 1 | Service operations runbook |
| Runbook link | `platform/link/runbook_link/` | 1 | Links platform.service to its ops runbook |
| Sample entities | `sample-data/entities.json` | 65 | Runtime entity payloads |
| Sample relations | `sample-data/relations.json` | 83 | Runtime topology payloads |
| Manifest | `sample-data/manifest.json` | — | Sample metadata, seed entities, scenario description |

## Extending This Demo

**Add a new EntitySet** (e.g., `platform.alert`):
1. Create `platform/entity_set/platform.alert.yaml` following existing files as template
2. Add entity instances to `sample-data/entities.json`
3. Update `manifest.json` counts

**Add a new relationship** (e.g., alert -> service):
1. Create `platform/link/entity_set_link/platform.alert_fires_on_platform.service.yaml`
2. Add relation entries to `sample-data/relations.json`
3. Update `manifest.json` counts

**Add a new Observation to the Runbook**:
1. Edit `platform/runbook_set/platform.service.ops.yaml`
2. Add a new entry under `observations[]` with: name, description, priority, steps, and conclusions
3. Add corresponding sample data if the observation needs new entity types

**Add a cross-domain relationship**:
1. Create file in `cross-domain/link/entity_set_link/`
2. Convention: `{source_domain}.{source_type}_{verb}_{target_domain}.{target_type}.yaml`

## Design Decisions

- **Scenario-driven**: Not "look at data" but "solve a mystery"
- **3-domain coupling**: Business -> Platform -> Runtime forms a natural investigation path
- **32-char hex entity IDs**: Required by CMS 2.0 format; `display_name` field provides human-readable names
- **Runbook-guided diagnosis**: Agent follows structured observations instead of free-form reasoning
- **Red herring included**: Tests whether investigation correctly rules out innocent deployments
- **Cross-domain value**: "Who is affected?" requires the business layer
- **Manifest as test anchor**: `sample-data/manifest.json` contains `seed_entities` for programmatic verification
