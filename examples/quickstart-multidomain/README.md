# Multi-Domain Quickstart Example Pack

`examples/quickstart-multidomain` is the default `make quickstart` sample. It shows how one workspace can connect DevOps ownership, coarse Kubernetes runtime topology, and enterprise scenario domains from `Demo/umodel-demo-2/umodel`.

The Kubernetes domain is intentionally coarse-grained: it models the runtime objects that help explain service topology, not a full Kubernetes object model.

This pack includes entity topology plus a minimal DevOps observability chain. `devops.service` is linked to one `metric_set`, one `log_set`, and one `event_set`; each dataset is linked to a storage definition so EntitySet dataset discovery can be tested against real UModel data.

## Contents

| Area | Path | Count | Purpose |
|---|---|---:|---|
| DevOps entity sets | `devops/entity_set/` | 10 | Teams, services, repositories, pipelines, environments, deployments, releases, changes, incidents, and SLOs. |
| Kubernetes entity sets | `k8s/entity_set/` | 7 | Coarse clusters, namespaces, workloads, pods, nodes, services, and ingresses. |
| Enterprise demo entity sets | `automaker/entity_set/`, `game/entity_set/`, `supplier/entity_set/` | 18 | Entity topology reused from `Demo/umodel-demo-2/umodel`. |
| Entity links | `*/link/entity_set_link/`, `cross-domain/link/entity_set_link/` | 42 | In-domain and cross-domain topology semantics. |
| DevOps data sets | `devops/metric_set/`, `devops/log_set/`, `devops/event_set/` | 3 | Minimal service metrics, logs, and deployment events for EntitySet dataset discovery. |
| Data and storage links | `devops/link/data_link/`, `devops/link/storage_link/` | 6 | Connect `devops.service` to datasets and datasets to storage. |
| Storage definitions | `devops/storage/` | 3 | Prometheus, Elasticsearch, and MySQL query-planning metadata. |
| Runtime entities | `sample-data/entities.json` | 93 | CMS 2.0 compatible entity payloads. |
| Runtime relations | `sample-data/relations.json` | 125 | CMS 2.0 compatible topology payloads. |

## Import

Start the quickstart server:

```bash
make quickstart
```

Manual import into another workspace:

```bash
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
```

## Query Examples

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel with(kind='entity_set') | project domain,name,kind | sort domain,name | limit 20"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__,display_name,status,owner | limit 10"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20"
```

## Maintenance Rules

- Keep model YAML, entity payloads, relation payloads, and docs aligned.
- Keep the DevOps observability chain small: one service `metric_set`, one service `log_set`, one service `event_set`, and their `data_link` / `storage_link` definitions.
- Keep k8s coarse for quickstart readability.
- Reuse only `entity_set` and `entity_set_link` from `Demo/umodel-demo-2/umodel`; keep dataset/storage definitions purpose-built for quickstart discovery.
- Run `make example-validate` and `go test ./internal/sampledata ./internal/bootstrap ./internal/query` after changing this pack.
