# Read the object graph with an AI agent

Point an AI agent at the object graph and let the **model scope the reads** — discover entity sets, walk topology, and pull telemetry through model-generated query plans. Don't make the agent hand-write SPL, PromQL, or log DSL; the model already carries the structure it needs.

<video controls preload="metadata" style="width: 100%" src="https://unifiedmodel-assets.oss-accelerate.aliyuncs.com/QuickStart.mp4"></video>

*A 90-second run on the `quickstart-multidomain` pack: an agent installs the `umodel-query` skill, finds the `demo` workspace, reads `devops.service`, and pulls metrics and logs — without a hand-written query.*

## Why

The object graph encodes what an agent would otherwise have to guess: which entity sets exist, how they relate across domains, which datasets hang off an entity, and how to turn an entity into a concrete telemetry query. When the agent reads *through* the model — `.umodel` → `.entity` → `.topo` → `get_metrics` / `get_logs` — the model binds the entity id and storage backend into the returned query plan. Hand-written PromQL or index names drift from the model the moment topology or storage config changes; model-scoped reads do not.

## The flow

The worked example runs on [`quickstart-multidomain`](../../../examples/quickstart-multidomain) (the default `make quickstart` pack — five domains, 35 object types). Each read is one `umctl` command; the same SPL runs over MCP via `query_spl_execute`.

**1. Discover the model.** The `domain` + `name` pairs are the arguments every other read takes.

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
```

**2. Read and search entities.** `devops.service` has four services; `checkout-service` is `degraded` (id `10000000000000000000000000000101`, reused below).

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__, display_name, status" -o json
```

**3. Walk cross-domain topology.** Find what the service `runs` on — a k8s workload in another domain. Filter graph-call output with `where`, not `with`.

```bash
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"devops@devops.service\" {__entity_id__:'10000000000000000000000000000101'})]) | where __relation_type__ = 'runs'" -o json
```

**4. Find the datasets on the entity.** Read the dataset `domain` + `name` from the entity itself — don't scan `.umodel` for them.

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call list_data_set(['metric_set','log_set','event_set'], true)" -o json
```

**5. Pull telemetry as a plan.** `get_metrics` returns a `prometheus_promql` plan with the service id already bound; the caller executes it.

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops','devops.metric.service','request_count', step='30s')" -o json
```

## With the `umodel-query` skill

The [`umodel-query`](../../../skills/umodel-query) skill runs exactly this flow from natural language. Bring up the telemetry-backed stack (`sh examples/quickstart-multidomain/deploy/start.sh`), point the skill at `http://localhost:8080`, and ask:

> "Read checkout-service's request rate, error rate, p95 latency, and recent ERROR logs."

The agent discovers the service, walks to its datasets, and executes the returned metric and log plans — no PromQL or index names typed by hand. [`umodel-rca`](../../../skills/umodel-rca) adds root-cause analysis on top.

## Do / Don't

| Do | Don't |
|---|---|
| Let `get_metrics` / `get_logs` return the query plan (entity id + storage bound). | Hand-write PromQL or an Elasticsearch index name. |
| Read dataset `domain` + `name` from the entity via `list_data_set`. | Scan `.umodel` to guess dataset names. |
| Filter graph-call output with `where __relation_type__ = '…'`. | Expect a `with(...)` clause to filter graph-call output. |
| Resolve topology rows (entity ids) to names with `.entity … with(ids=[…])`. | Treat the raw ids in graph-call rows as display names. |

> In-memory mode (`make quickstart`) serves the model, entity, search, and topology reads; `get_metrics` / `get_logs` return plans with nothing to run them against. Use `deploy/start.sh` for end-to-end execution against seeded Prometheus / Elasticsearch.

## See also

- [Multi-domain quickstart pack](../../../examples/quickstart-multidomain)
- [Query Service](/en/guides/query-service)
- [Agent Integration](/en/guides/agent-integration)
- [UModel Agent Skills](../../../skills/README.md)
