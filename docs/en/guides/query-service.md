# Query Service Guide

中文：[Query Service 指南](../../zh/guides/query-service.md)

Query Service is the only public read path for UModel definitions, entities, relations, topology, and EntitySet call planning. It accepts SPL strings that start with `.umodel`, `.entity_set`, `.entity`, `.topo`, or `.runbook_set`.

## Why Reads Go Through Query Service

UModel intentionally avoids separate public domain read APIs such as entity lookup, relation lookup, graph traversal, or model search endpoints. One read surface keeps the CLI, Web UI, REST API, MCP tools, and SDK clients aligned.

## Entry Points

REST:

```http
POST /api/v1/query/{workspace}/execute
POST /api/v1/query/{workspace}/explain
```

CLI:

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
go run ./cmd/umctl --addr http://localhost:8080 query explain demo ".umodel | limit 5"
```

Agent tool:

```bash
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_execute '{"query":".umodel | limit 5"}'
```

## `.umodel`

`.umodel` reads UModel definitions.

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel with(kind='entity_set') | sort name | limit 20"
```

Common reads:

- List entity sets.
- Inspect metric, log, trace, event, storage, and link definitions.
- Power the Web UI Explorer graph/table view.

## `.entity`

`.entity` reads runtime entity records.

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__,display_name | limit 20"
```

Common reads:

- Search entities in a domain and entity type.
- Inspect object properties.
- Feed object IDs into topology queries.

Agent and REST callers can bind named parameters into `with(...)` filters and `where` predicates:

```json
{
  "query": ".entity with(domain='devops', name='devops.service', query=$query) | limit 20",
  "parameters": {
    "query": "checkout"
  }
}
```

## `.entity_set`

`.entity_set` handles EntitySet method calls with UModel Assistant-compatible response data. Metadata/discovery methods return `responseType=2` with `header` and `data`; query-planning methods return `responseType=1` with a downstream query plan in `query`.

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()"
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)"
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')"
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')"
```

The required filters are `domain` and `name`; `ids` is accepted as EntitySet call context. The currently supported methods are `__list_method__`, `list_data_set` (`list_dataset` alias), `get_logs` (`get_log` alias), and `get_metrics` (`get_metric` alias); method parameters are validated against the UModel Assistant signatures. `get_logs` and `get_metrics` parse basic SPL where syntax, map EntitySet fields through `data_link.fields_mapping`, map DataSet fields through `storage_link.fields_mapping`, and return translated storage query plans without querying the storage itself.

## `.topo`

`.topo` reads runtime topology relations.

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest | limit 20"
```

`.topo` supports graph-call style topology operations. The `memory`, `file.memory`, and optional `local.ladybug` providers support controlled read-only Cypher-compatible graph calls through the shared Go engine. `local.ladybug` still persists graph data in Ladybug when built with `-tags ladybug` and a local Ladybug runtime.

Cypher can return full entity and relation property maps in one query:

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".topo | graph-call cypher(`MATCH (src)-[r]->(dest) RETURN src, r AS relation, dest LIMIT 20`)"
```

Use `properties(src)`, `properties(r)`, and `properties(dest)` when callers want to make the property-map shape explicit.

## Common Pipe Operations

The local query layer supports the operations used by tests, examples, and the Web UI:

- `with(...)` for source-specific filters.
- `project` to select output fields.
- `sort` to order rows.
- `limit` to bound output.
- `entity-call` for EntitySet method planning.
- `graph-call` for topology functions.

Run the built-in examples:

```bash
go run ./cmd/umctl --addr http://localhost:8080 query examples
```

## Explain Output

Run `query explain` before wiring queries into UI or agent workflows:

```bash
go run ./cmd/umctl --addr http://localhost:8080 query explain demo ".entity with(domain='devops', name='devops.service') | limit 5"
```

Explain output reports:

- Query source: `.umodel`, `.entity_set`, `.entity`, `.topo`, or `.runbook_set`.
- Active provider.
- Storage provider.
- Planned filters and limits.

## Boundary Rules

- Do not add public entity, relation, or topology read endpoints outside Query Service.
- Keep CLI domain reads behind `umctl query run` and `umctl query explain`.
- Keep AgentGateway resources metadata-only. Runtime rows should be returned by tools, not resources.
