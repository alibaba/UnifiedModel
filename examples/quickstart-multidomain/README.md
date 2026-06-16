# Multi-domain quickstart

中文版本：[多域 Quickstart 示例包](README.zh-CN.md)

A multi-domain sample for reading across the object graph end to end — one workspace, five
domains, 35 object types — over the `.umodel` / `.entity` / `.topo` SPL surface: model discovery,
entity and topology reads, search, and telemetry query-planning. It is also the repo's default
`make quickstart` sample. The [`umodel-query`](../../skills/umodel-query) skill drives the same
path from an agent.

Domains: **devops** (services, deployments, pipelines, SLOs, incidents…), **k8s** (clusters,
namespaces, workloads, pods), and three enterprise sets — **automaker**, **game**, **supplier**.
`devops.service` carries a metric, log, and event dataset, each linked to a storage backend
(Prometheus, Elasticsearch, MySQL).

## Running it

```bash
sh examples/quickstart-multidomain/deploy/start.sh
```

Brings up UModel serving this pack plus a seeded Prometheus and Elasticsearch (docker or podman),
so every read below — through the `get_metrics` / `get_logs` plans — runs end to end. UModel on
`http://localhost:8080`, Prometheus on `:9090`, Elasticsearch on `:9200`. See [deploy/](deploy/).

This pack is also the repo's default `make quickstart` sample, which loads the model and sample
data into an in-memory server without telemetry backends: the model, entity, search, and topology
reads below work, but `get_metrics` / `get_logs` return plans with nothing to run them against.

## Reads

Each read is one `umctl` command; pass `-o json` (rows in `data.data`, columns in `data.header`).
The same SPL runs over MCP via `query_spl_execute`.

### Model

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
```

Lists the 35 object types. The `domain` + `name` pairs are the arguments every other read takes.

### Entities

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service')" -o json
umctl query run demo ".entity with(domain='devops', name='devops.service') | project __entity_id__, display_name, status" -o json
```

Four services: `checkout-service` (degraded), `catalog-api` (active), `delivery-service`
(warning), `telemetry-collector` (active). `checkout-service`'s `__entity_id__` is
`10000000000000000000000000000101`, reused below. `| project` is optional; a bare read returns all
fields.

### Search

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__, display_name, status" -o json
```

`query=` is full-text across all fields. Add `mode='vector'` (or `mode='hyper'`) with `topk=N` for
similarity ranking; vector results come back as full rows ordered by score.

### Topology

```bash
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"devops@devops.service\" {__entity_id__:'10000000000000000000000000000101'})]) | where __relation_type__ = 'runs'" -o json
```

Returns `checkout-service`'s `runs` edge to a k8s workload — a cross-domain relation. Filter with
`where __relation_type__ = '…'`; a `with(...)` clause does not filter graph-call output. Rows carry
entity ids; resolve names with `.entity … with(ids=[…])`.

### Datasets

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()" -o json
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call list_data_set(['metric_set','log_set','event_set'], true)" -o json
```

Methods: `__list_method__`, `list_data_set`, `get_metrics`, `get_logs`. The service's datasets:
`devops.metric.service` (Prometheus), `devops.log.service` (Elasticsearch),
`devops.event.deployment` (MySQL). `list_data_set` returns the `domain` + `name` that
`get_metrics` / `get_logs` take — read them from the entity rather than scanning `.umodel`.

### Metrics

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops','devops.metric.service','request_count', step='30s')" -o json
```

Returns a `prometheus_promql` plan:
`sum(rate(devops_service_request_total{service_id="10000000000000000000000000000101"}[1m]))`,
endpoint `http://localhost:9090`, with the service id bound. Open source returns the plan; the
caller executes it. With `deploy/` up it runs as-is; otherwise repoint the endpoint at your
Prometheus. See [metrics-logs](../../skills/umodel-query/references/metrics-logs.md).

### Logs

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops','devops.log.service', query='level = \"ERROR\"')" -o json
```

Returns an `elasticsearch_dsl` plan over index `devops-service-logs-*`, endpoint
`http://localhost:9200`. `devops.event.deployment` is modeled on MySQL and is discoverable, but the
executable plan methods are `get_metrics` (Prometheus) and `get_logs` (Elasticsearch).

## With an agent

The [`umodel-query`](../../skills/umodel-query) skill runs the reads above and executes the
returned plans. With `deploy/` up, point it at `http://localhost:8080` (`UMCTL_ADDR`, or the MCP
target) and ask in natural language — e.g. "read checkout-service's request rate, error rate, p95
latency, and recent ERROR logs." [`umodel-rca`](../../skills/umodel-rca) adds root-cause analysis.
Install: [skills/README.md](../../skills/README.md).

## Contents

| Area | Path | Count | Purpose |
|---|---|---:|---|
| DevOps entity sets | `umodel/devops/entity_set/` | 10 | Teams, services, repositories, pipelines, environments, deployments, releases, changes, incidents, and SLOs. |
| Kubernetes entity sets | `umodel/k8s/entity_set/` | 7 | Coarse clusters, namespaces, workloads, pods, nodes, services, and ingresses. |
| Enterprise demo entity sets | `umodel/automaker/entity_set/`, `umodel/game/entity_set/`, `umodel/supplier/entity_set/` | 18 | Reused enterprise entity topology. |
| Entity links | `umodel/*/link/entity_set_link/`, `umodel/cross-domain/link/entity_set_link/` | 42 | In-domain and cross-domain topology semantics. |
| DevOps data sets | `umodel/devops/metric_set/`, `umodel/devops/log_set/`, `umodel/devops/event_set/` | 3 | Service metrics, logs, and deployment events for dataset discovery. |
| Data and storage links | `umodel/devops/link/data_link/`, `umodel/devops/link/storage_link/` | 6 | Connect `devops.service` to datasets, and datasets to storage. |
| Storage definitions | `umodel/devops/storage/` | 3 | Prometheus, Elasticsearch, and MySQL query-planning metadata. |
| Deploy stack | `deploy/` | — | One-command demo: `docker-compose` + seeded Prometheus / Elasticsearch + `start.sh` / `verify.sh`. |
| Runtime entities | `sample-data/entities.json` | 93 | CMS 2.0 compatible entity payloads. |
| Runtime relations | `sample-data/relations.json` | 125 | CMS 2.0 compatible topology payloads. |

## Importing into another workspace

The quickstart server imports automatically. For another workspace:

```bash
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
```

## Maintenance

- Keep model YAML, entity payloads, relation payloads, and docs in sync.
- Keep the DevOps observability chain minimal: one service `metric_set`, one `log_set`, one
  `event_set`, with their `data_link` / `storage_link`.
- Keep k8s coarse.
- Run `make example-validate` and `go test ./internal/sampledata ./internal/bootstrap ./internal/query` after changing this pack.
