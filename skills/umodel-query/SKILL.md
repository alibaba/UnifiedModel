---
name: umodel-query
description: >-
  Read data and model from a UModel object-graph semantic layer, primarily
  through the `umctl` CLI (MCP alternative noted). Reads (1) entity and
  relationship/topology data and (2) the UModel model itself — entity sets,
  datasets, links, runbooks. Entity/relation/model reads return real rows in
  open source; against a PaaS-backed endpoint the same calls return the PaaS
  API's data. Use when asked to query or read UModel entities, relations,
  topology, or model metadata; look up services / dependencies; or list what
  objects and datasets exist. For root-cause analysis built on these reads,
  see the `umodel-rca` skill. Triggers: UModel, object graph, .entity / .topo /
  .umodel, query entities, read topology, list services / dependencies /
  datasets, 实体查询, 关系/拓扑查询, 读模型, 查服务依赖.
---

# UModel Query — read entities, relationships, and the model

UModel is an **object-graph semantic layer**: enterprise objects (services, Pods,
deployments, config changes, promotions, …), their typed relationships (`calls`,
`runs-on`, `affects`, `triggers`, `impacts`, …), and the datasets (metrics, logs)
that hang off them — all queryable through one SPL surface.

This skill teaches you (an agent) to **read** it. Prefer the `umctl` CLI; an MCP
alternative is at the end. *(For model-guided root-cause analysis on top of these
reads, load the `umodel-rca` skill.)*

## Setup (CLI-first)

Point `umctl` at a running UModel server (open source, or a PaaS-backed
endpoint). For the bundled demo:

```bash
make quickstart QUICKSTART_SAMPLE=examples/incident-investigation   # serves http://localhost:8080
```

Every read is one command — **always pass `-o json`** for machine-readable rows:

```bash
umctl query run <workspace> "<SPL>" -o json     # execute
umctl query explain <workspace> "<SPL>"          # see the plan/providers without running
umctl --addr http://<host>:8080 query run …       # target a specific server (e.g. a PaaS endpoint)
```

**Response shape** (parse this): rows live in `data.data` (a matrix), column names
in `data.header`.

```jsonc
{ "code": "200", "success": true,
  "data": {
    "header": ["display_name", "status", "owner", "sla_tier"],
    "data":   [ ["payment-gateway", "degraded", "payments-backend", "platinum"] ]
  } }
```

So `columns = data.header`, `rows = data.data`. Zip them to read records.

## Read entity & relationship data

Real rows directly (from EntityStore / GraphStore), in open source and against a
PaaS endpoint alike. *(Against a PaaS-backed `--addr`, the same commands return
the PaaS API's data response — same SPL, same shape.)*

### Entities — `.entity`

```bash
umctl query run demo ".entity with(domain='platform', name='platform.service', query='degraded') | project display_name, status, owner, sla_tier" -o json
# → ["payment-gateway","degraded","payments-backend","platinum"]
```

- `query='…'` is full-text over all entity fields. Add `mode='vector'` or
  `mode='hyper'` for semantic / hybrid search, `topk=N` to bound matches.
- `with(ids=['<entity_id>'])` fetches specific entities by id.
- Pipe `| project a,b,c`, `| where …`, `| sort …`, `| limit N`.

### Relationships & topology — `.topo`

```bash
# neighbors along a relationship (raise the hop count for multi-hop)
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"platform@platform.service\" {__entity_id__:'63718b78868895d2590551b27ec6f51c'})]) | with(__relation_type__='calls')" -o json

# direct relations of a node; or full Cypher
umctl query run demo ".topo | graph-call getDirectRelations([(:\"platform@platform.service\" {__entity_id__:'…'})])" -o json
umctl query run demo ".topo | graph-call cypher(\`MATCH (s)-[r]->(d) RETURN properties(s), type(r), properties(d) LIMIT 20\`)" -o json
```

Each relation row carries the source ref, relation type, destination ref, and edge
properties. **Topology rows reference entities by ID** — resolve display names with
a follow-up `.entity … with(ids=[…])` when you need them.

## Read the UModel model — `.umodel`

The model is the **map**: what object types, datasets, links, and runbooks exist,
and how they connect. Read it before assuming structure.

```bash
# what object types / datasets / runbooks exist
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
umctl query run demo ".umodel with(kind='runbook_set', name='platform.service.ops')" -o json

# what can a given EntitySet do, and what telemetry hangs off it
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['…']) | entity-call __list_method__()" -o json
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['…']) | entity-call list_data_set(['metric_set','log_set'], true)" -o json
```

Kinds you can list: `entity_set`, `metric_set`, `log_set`, `event_set`,
`entity_set_link`, `data_link`, `storage_link`, `runbook_set`. Use `.umodel` +
`__list_method__` + `list_data_set` to discover capabilities instead of guessing.

## Notes & gotchas

- **Always `-o json`**; parse `rows = data.data`, `columns = data.header`.
- `.entity` / `.topo` / `.umodel` reads return **real rows** in open source.
  (Telemetry reads — `get_metrics` / `get_logs` — return a *plan* in open source
  and *data* via a PaaS endpoint; those belong to the `umodel-rca` skill.)
- Topology rows carry entity **IDs**, not names — resolve with `.entity with(ids=[…])`.
- Stay **read-only**.
- **MCP alternative** (instead of the CLI): connect `umodel-mcp` and call the
  `query_spl_execute` tool with `{ "workspace": "demo", "query": "<the same SPL>" }`
  (arg key is `query`, not `spl`). Same SPL either way.
