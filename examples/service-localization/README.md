# Service Localization Demo

中文：[服务定位 Demo](README.zh-CN.md)

A scenario-driven example showing how an AI agent uses UModel to **fetch telemetry and localize a bottleneck down a four-layer request stack** — product API → service → datastore → infrastructure. A degraded checkout API is traced, hop by hop, to a saturated database connection pool, while the infrastructure layer is ruled out.

This demo is the companion to [Incident Investigation](../incident-investigation/README.md): that one is *reactive root-cause analysis* (symptom → cause via a runbook); this one is *vertical localization* (walk the critical path, attribute the latency to a layer) and puts **data retrieval** front and center.

```
Checkout Flow (journey, impacted, 2.1% errors)
  └─ depends_on → checkout-api (degraded, P99 > 300ms SLO)
                   └─ calls → order-svc (degraded, but CPU healthy)
                               └─ reads_writes → orders-db (SATURATED — pool ~98%)   ← root
                                                  └─ hosted_on → node-a (healthy)     ← infra ruled out

Healthy siblings: catalog-api/svc, search-api/svc, payment-svc, inventory-svc
```

## The four-layer stack

| Domain | EntitySet | Role |
|---|---|---|
| `product` | `product.journey`, `product.api` | user journeys + user-facing API endpoints (latency SLO) |
| `service` | `service.app` | backend microservices |
| `data` | `data.store` | datastores (postgres / redis / kafka) |
| `infra` | `infra.node`, `infra.pod` | Kubernetes nodes and pods |

Each layer carries telemetry: `product.api.metrics`, `service.app.metrics` (+ `service.app.logs`), `data.store.metrics` (with the key `connection_pool_usage` signal), and `infra.node.metrics`.

## Quick Start

```bash
make quickstart QUICKSTART_SAMPLE=examples/service-localization
```

API: `http://localhost:8080` | Web UI: `http://localhost:5173`

Loads 4 domains (Product / Service / Data / Infra), 6 entity sets, 23 entities, 29 relations, 4 metric sets, 1 log set.

API only (no Web UI):

```bash
go run ./cmd/umodel-server --quickstart --quickstart-sample service-localization
```

## Data-Retrieval Walkthrough

The agent localizes the bottleneck by **fetching data at each hop**. The same four moves repeat at every layer: *find the entity → see what telemetry it has → pull the signal → step to the next hop.*

### 1. Find the degraded entry point

```bash
umctl query run demo \
  ".entity with(domain='product', name='product.api', query='degraded') \
  | project display_name, status, sla_tier, latency_slo_ms"
```

Expected: `checkout-api | degraded | platinum | 300`.

### 2. Discover what you can fetch about it

```bash
# What methods does this EntitySet expose?
umctl query run demo \
  ".entity_set with(domain='service', name='service.app') | entity-call __list_method__()"

# Which datasets are attached to services?
umctl query run demo \
  ".entity_set with(domain='service', name='service.app') | entity-call list_data_set(['metric_set','log_set'], true)"
```

### 3. Step down the critical path (one hop at a time)

`getDirectRelations` returns a node's immediate edges. Walk the *downstream* (`calls` / `reads_writes` / `hosted_on`) edge at each hop.

```bash
# checkout-api → order-svc
umctl query run demo \
  ".topo | graph-call getDirectRelations([(:\"product@product.api\" {__entity_id__: '3a44ea48396a812d5a1f4eb12ae51e39'})])"

# order-svc → orders-db (follow the reads_writes edge)
umctl query run demo \
  ".topo | graph-call getDirectRelations([(:\"service@service.app\" {__entity_id__: 'f25ae2923f5df058b6119ea79e434459'})])"
```

### 4. Pull the signal that localizes the layer

At the service hop, fetch latency **and** the service's own CPU — latency high, CPU healthy means the cause is downstream:

```bash
umctl query run demo \
  ".entity_set with(domain='service', name='service.app', ids=['f25ae2923f5df058b6119ea79e434459']) \
  | entity-call get_metrics('service', 'service.app.metrics', 'cpu_usage', step='30s')"
```

At the datastore hop, fetch the saturation signal — this is the root:

```bash
umctl query run demo \
  ".entity_set with(domain='data', name='data.store', ids=['60794de7878447582b1a4d5fe11e37a0']) \
  | entity-call get_metrics('data', 'data.store.metrics', 'connection_pool_usage', step='30s')"
```

The plan renders `max(data_store_connection_pool_in_use{target_id="60794de7…"}) / max(data_store_connection_pool_max{…})` — the object graph turned "the store order-svc depends on" into the exact saturation query, no hand-written PromQL. For an agent client, add `?format=agent` to get the compact v1.1 envelope (see [Plan Schema v1](../../docs/en/spec/plan-schema-v1.md)).

### 5. Rule out the layer below

```bash
# orders-db → node-a ; then check the node is healthy (rules out infra)
umctl query run demo \
  ".topo | graph-call getDirectRelations([(:\"data@data.store\" {__entity_id__: '60794de7878447582b1a4d5fe11e37a0'})])"

umctl query run demo \
  ".entity_set with(domain='infra', name='infra.node', ids=['6cec8a5bb33ae85cefde09a76ebeca4c']) \
  | entity-call get_metrics('infra', 'infra.node.metrics', 'cpu_usage', step='30s')"
```

**Conclusion:** latency is introduced at `order-svc` but not explained by its CPU; its downstream `orders-db` connection pool is saturated while the node hosting it is healthy → **bottleneck localized to the datastore connection pool**.

### Run the whole walkthrough at once

With a server running (`make quickstart QUICKSTART_SAMPLE=examples/service-localization`), replay the full narrated localization loop — each SPL, result, and the agent's reasoning per hop:

```bash
./examples/service-localization/demo.sh
```

The same path is gated in CI by `TestServiceLocalizationPath` (`internal/bootstrap/localization_test.go`), so the demo cannot silently rot, and by the MCP-driven `test-integration.sh`.

## Agent Integration

Connect an MCP client (see the [Agent Integration Guide](../../docs/en/guides/agent-integration.md)) and ask:

> "checkout-api is breaching its latency SLO — localize the bottleneck."

The agent runs the same four moves per hop, walking product → service → data → infra and attributing the latency to the first hop whose downstream dependency is saturated while its own resources are healthy.

### MCP Connection

```json
{
  "mcpServers": {
    "umodel": {
      "command": "go",
      "args": [
        "run", "./cmd/umodel-mcp",
        "--quickstart",
        "--quickstart-sample", "service-localization",
        "--graphstore", "memory"
      ]
    }
  }
}
```

## Contents

| Area | Path | Count | Purpose |
|------|------|------:|---------|
| Product entity sets | `product/entity_set/` | 2 | journeys + APIs |
| Service entity set | `service/entity_set/` | 1 | microservices |
| Data entity set | `data/entity_set/` | 1 | datastores |
| Infra entity sets | `infra/entity_set/` | 2 | nodes + pods |
| In-domain links | `*/link/entity_set_link/` | 3 | journey→api, svc→svc, pod→node |
| Cross-domain links | `cross-domain/link/entity_set_link/` | 4 | api→svc, svc→store, svc→pod, store→node |
| Metric sets | `*/metric_set/` | 4 | API / service / store / node golden metrics |
| Log set | `service/log_set/` | 1 | service application logs |
| Storage | `*/storage/` | 2 | Prometheus (metrics) + Elasticsearch (logs) |
| Data links | `*/link/data_link/` | 5 | entity set → its datasets |
| Storage links | `*/link/storage_link/` | 5 | datasets → storage |
| Sample entities | `sample-data/entities.json` | 23 | runtime entity payloads |
| Sample relations | `sample-data/relations.json` | 29 | runtime topology payloads |
| Manifest | `sample-data/manifest.json` | — | scenario metadata, seed entities, counts |

## Design Notes

- **Vertical, not horizontal.** incident-investigation walks *across* (upstream callers + business). This walks *down* a request stack (app → svc → data → infra).
- **Data retrieval is the hero.** Every hop pulls a metric; the story is "what to fetch and how," which is exactly what an agent must learn.
- **Plan-only.** UModel open source returns query *plans*; an executor (e.g. umodel-assistant) runs them against real storage. The bottleneck lives in the entity `status` and the planted topology, so the localization path is fully reproducible offline.
- **One-hop traversal.** `getDirectRelations` is used per hop rather than a single deep `getNeighborNodes`, mirroring how an agent actually localizes — decide where to step next based on what each hop reveals.
