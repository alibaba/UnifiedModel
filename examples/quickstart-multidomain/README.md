# Multi-Domain Quickstart — the smart data-retrieval demo

中文版本：[多域 Quickstart 示例包](README.zh-CN.md)

`examples/quickstart-multidomain` is the default `make quickstart` sample and the **smart
data-retrieval (智能读数) showcase**: one workspace, **5 domains, 35 object types**, read
end-to-end through a single SPL surface — discover the model, read objects and topology across
domains, search semantically, and turn telemetry into executable query plans. It pairs with the
[`umodel-query`](../../skills/umodel-query) skill, which lets an agent do this walkthrough
autonomously.

The domains: **devops** (services, deployments, pipelines, SLOs, incidents…), **k8s** (coarse
clusters / namespaces / workloads / pods), and three enterprise scenarios — **automaker**,
**game**, **supplier**. `devops.service` carries a metric / log / event dataset, each linked to
its storage (Prometheus / Elasticsearch / MySQL), so dataset discovery runs against real data.

## Quick Start

```bash
make quickstart
```

API: `http://localhost:8080` | Web UI: `http://localhost:5173`. Loads 5 domains, 35 entity sets,
93 entities, 125 relations into the `demo` workspace (in-memory, no API key).

> API only (no Web UI):
> `go run ./cmd/umodel-server --quickstart --quickstart-sample multi-domain-quickstart --graphstore memory`

## Smart data-retrieval walkthrough

Every read is one `umctl` command — always pass `-o json` (rows in `data.data`, columns in
`data.header`). The same SPL works over MCP via the `query_spl_execute` tool. The
[`umodel-query`](../../skills/umodel-query) skill performs these autonomously; here they are by hand.

### 1. Discover the model

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
```

→ 35 object types across `devops`, `k8s`, `automaker`, `game`, `supplier`. `.umodel` is the map;
the `domain` + `name` you see here are what every other read takes.

### 2. Read objects

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service')" -o json   # all fields
umctl query run demo ".entity with(domain='devops', name='devops.service') | project __entity_id__, display_name, status, owner" -o json
```

→ `checkout-service` (degraded), `catalog-api` (active), `delivery-service` (warning),
`telemetry-collector` (active). `__entity_id__` (checkout-service = `10000000000000000000000000000101`)
is the handle you reuse below. Pipes like `| project` are optional — a bare read returns all fields.

### 3. Search

Plain full-text — include any text and it matches across all fields:

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__, display_name, status" -o json
# → checkout-service | degraded
```

For meaning-based ranking add `mode='vector'` (or `mode='hyper'` for hybrid) with `topk=N` — the
top hit for `query='payment checkout'` is `checkout-service` ("Converts carts into paid orders").
Semantic results come back as **full rows** ranked by similarity (the `| project` shortcut
applies to plain reads).

### 4. Traverse topology (including cross-domain)

```bash
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"devops@devops.service\" {__entity_id__:'10000000000000000000000000000101'})]) | where __relation_type__ = 'runs'" -o json
```

→ `checkout-service --runs--> ` a **k8s** workload: a cross-domain edge (devops → k8s). Other
relations on this node include `deploys`, `measured_by`, `runs_in`, `impacts`, `contains`. Filter
rows with `where __relation_type__='…'` (a `with(...)` clause does **not** filter graph-call
output). Rows carry entity **IDs** — resolve display names with `.entity … with(ids=[…])`.

### 5. Discover an object's methods & datasets

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()" -o json
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call list_data_set(['metric_set','log_set','event_set'], true)" -o json
```

→ methods: `__list_method__`, `list_data_set`, `get_metrics`, `get_logs`. Datasets on the
service: `devops.metric.service` (**Prometheus**), `devops.log.service` (**Elasticsearch**),
`devops.event.deployment` (**MySQL**, table `deployment_events`). One object, three storage
backends — all behind one model.

### 6. Read metrics → plan → run

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops','devops.metric.service','request_count', step='30s')" -o json
```

→ a `prometheus_promql` plan carrying
`sum(rate(devops_service_request_total{service_id="10000000000000000000000000000101"}[1m]))`
against `http://prometheus.devops.example:9090` — the service id already substituted, no
hand-written PromQL. Point the endpoint at your Prometheus and run it (see the skill's
[metrics-logs guide](../../skills/umodel-query/references/metrics-logs.md)).

### 7. Read logs → plan → run

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops','devops.log.service', query='level = \"ERROR\"')" -o json
```

→ an `elasticsearch_dsl` plan: a bool/filter query on index `devops-service-logs-*` against
`https://elasticsearch.devops.example:9200`. Run it against your Elasticsearch the same way.

> The `devops.event.deployment` event_set is modeled on **MySQL** (discoverable in Step 5 and via
> `.umodel`), showing one model over three backends. The executable plan methods today are
> `get_metrics` (Prometheus) and `get_logs` (Elasticsearch).

## For an agent

The [`umodel-query`](../../skills/umodel-query) skill teaches an agent to do all of the above —
discover the model, read across domains, search, and turn `get_metrics` / `get_logs` plans into
real values — on its own. Load it (and [`umodel-rca`](../../skills/umodel-rca) for root-cause
analysis on top), connect over MCP or CLI, and ask in natural language.

## Contents

| Area | Path | Count | Purpose |
|---|---|---:|---|
| DevOps entity sets | `devops/entity_set/` | 10 | Teams, services, repositories, pipelines, environments, deployments, releases, changes, incidents, and SLOs. |
| Kubernetes entity sets | `k8s/entity_set/` | 7 | Coarse clusters, namespaces, workloads, pods, nodes, services, and ingresses. |
| Enterprise demo entity sets | `automaker/entity_set/`, `game/entity_set/`, `supplier/entity_set/` | 18 | Reused enterprise entity topology. |
| Entity links | `*/link/entity_set_link/`, `cross-domain/link/entity_set_link/` | 42 | In-domain and cross-domain topology semantics. |
| DevOps data sets | `devops/metric_set/`, `devops/log_set/`, `devops/event_set/` | 3 | Minimal service metrics, logs, and deployment events for EntitySet dataset discovery. |
| Data and storage links | `devops/link/data_link/`, `devops/link/storage_link/` | 6 | Connect `devops.service` to datasets and datasets to storage. |
| Storage definitions | `devops/storage/` | 3 | Prometheus, Elasticsearch, and MySQL query-planning metadata. |
| Runtime entities | `sample-data/entities.json` | 93 | CMS 2.0 compatible entity payloads. |
| Runtime relations | `sample-data/relations.json` | 125 | CMS 2.0 compatible topology payloads. |

## Manual import

Into another workspace (the quickstart server imports automatically):

```bash
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
```

## Maintenance Rules

- Keep model YAML, entity payloads, relation payloads, and docs aligned.
- Keep the DevOps observability chain small: one service `metric_set`, one service `log_set`, one
  service `event_set`, and their `data_link` / `storage_link` definitions.
- Keep k8s coarse for quickstart readability.
- Keep dataset/storage definitions purpose-built for quickstart discovery.
- Run `make example-validate` and `go test ./internal/sampledata ./internal/bootstrap ./internal/query` after changing this pack.
