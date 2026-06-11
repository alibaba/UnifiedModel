---
name: umodel
description: >-
  Read and reason over a UModel object-graph semantic layer, primarily through
  the `umctl` CLI. Three capabilities: (1) read entity and relationship/topology
  data (returns real rows; against a PaaS-backed endpoint the same calls return
  the PaaS API's data); (2) read the UModel model itself — entity sets, datasets,
  links, runbooks; (3) use the model to guide autonomous data fetching and
  root-cause analysis over the object graph. Use when asked to query UModel
  entities/relations/topology/model, fetch a service's metrics or logs, or
  diagnose an incident / find a root cause. Triggers: UModel, object graph,
  .entity / .topo / .umodel, query entities, topology, root cause, RCA, incident,
  SLO breach, why is X slow / degraded, 实体查询, 关系/拓扑, 读模型, 根因分析, 故障排查.
---

# UModel — read the object graph, fetch data, analyze

UModel is an **object-graph semantic layer**: enterprise objects (services, Pods,
deployments, config changes, promotions, …), their typed relationships
(`calls`, `runs-on`, `affects`, `triggers`, `impacts`, …), and the datasets
(metrics, logs) that hang off them — all queryable through one SPL surface.

This skill teaches you (an agent) to use it. **Prefer the `umctl` CLI for all
reads**; an MCP alternative is noted at the end.

## Setup (CLI-first)

Point `umctl` at a running UModel server (open source, or a PaaS-backed
endpoint). For the bundled demo:

```bash
make quickstart QUICKSTART_SAMPLE=examples/incident-investigation   # serves http://localhost:8080
```

Every read is one command — **always pass `-o json`** so you get machine-readable rows:

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

So: `columns = data.header`, `rows = data.data`. Zip them to read records.

---

## Capability 1 — Read entity & relationship data

These return **real data rows directly** (from EntityStore / GraphStore), in open
source and against a PaaS endpoint alike. *(Against a PaaS-backed `--addr`, the
same commands return the PaaS API's data response — same SPL, same shape.)*

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

Each relation row carries the source ref, relation type, destination ref, and
edge properties. **Topology rows reference entities by ID** — resolve display
names with a follow-up `.entity … with(ids=[…])` when you need them.

---

## Capability 2 — Read the UModel model (`.umodel`)

The model is your **map**: what object types, datasets, links, and runbooks exist,
and how they connect. Read it before you assume structure.

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

---

## Capability 3 — Model-guided data fetch + root-cause analysis

This is the hard, valuable one: use what the model tells you to **autonomously
fetch the right data and reason to a root cause.** You decide the path.

### Model-guided data fetch (autonomous retrieval)

`get_metrics` / `get_logs` are driven by the object graph: the model knows which
metric/log set hangs off an entity and the `fields_mapping`, so it **fills in
`service_id` for you** — you never hand-write PromQL or guess an ID.

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call get_metrics('platform','platform.service.metrics','latency_p99_ms', step='30s')" -o json
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['…']) | entity-call get_logs('platform','platform.service.logs', query='level = \"ERROR\"')" -o json
```

> **Open source returns a query *plan*** (the rendered PromQL / ES query, with the
> id substituted) — a downstream executor runs it. **Against a PaaS-backed
> endpoint** (`umctl --addr <paas>` with `mode='data'`), the same call returns the
> **actual rows** as the PaaS API response (`{__labels__, __ts__, __value__}` for
> metrics). Either way, the object graph produced the exact, correctly-scoped query.

### The autonomous RCA loop

Run this loop; let evidence — not a fixed script — drive your next query:

1. **ORIENT** — locate the symptomatic entity (`.entity … query='degraded'`).
   Read its methods, datasets, neighbors, linked runbook (Capability 2).
2. **CHARACTERIZE (fetch)** — pull its own signals (`get_metrics`/`get_logs`) to
   confirm and quantify the symptom.
3. **HYPOTHESIZE** — candidate causes: upstream dependency, recent change
   (config/deploy), capacity/traffic, downstream resource. Keep several alive.
4. **GATHER EVIDENCE (multi-hop, cross-domain)** — traverse `.topo` to upstream
   callers and *their* recent `config_change`/`deployment`; follow links into the
   **business** domain (promotions/traffic) or **runtime** domain (nodes/pods).
   Cross-domain reach is where the object graph beats a flat metrics dump.
5. **CORRELATE & DISCRIMINATE** — line up changes × topology × telemetry ×
   business context on a timeline. Separate root cause from coincidence: a recent
   deploy is **not guilty just because it's recent** — read its `change_summary`
   and rule out trivial ones (the *red herring* trap). Prefer a cause with a
   **stated, ideally quantified, mechanism**.
6. **CONCLUDE** — root cause + evidence chain (cite the graph path per link) +
   quantified mechanism + confidence + a **reversible, confirmation-required**
   recommendation.

### Runbook as scaffold

If the entity links a `runbook_set`, load it and use its **observations** as a
reasoning frame (each = a hypothesis + how to check it + a conclusion rule). Use
it to structure reasoning; you may still form hypotheses it didn't list. Cite its
`knowledge` (failure patterns) and `toolkits` (allowed remediation tools).

### Output

```
## Diagnosis
Symptom: <what's broken, quantified>
Evidence chain:
- <finding>  ← <SPL / graph path traversed>
Root cause: <cause>, mechanism: <stated / quantified>
Ruled out: <red herrings and why>
Confidence: <high|medium|low>
Recommended action: <tool> — <input> (risk, requires confirmation, ETA)
```

---

## Worked example — incident-investigation demo (a TEST of the method, not a script)

Symptom: `payment-gateway` (platinum SLO) is `degraded`. A good agent reaches the
root cause **without** being told the steps:

- ORIENT: `.entity … query='degraded'` → payment-gateway (`63718b78…`), links
  runbook `platform.service.ops` + datasets `platform.service.metrics`/`.logs`.
- CHARACTERIZE: `get_metrics(… 'latency_p99_ms' …)` → P99 breaching; `get_logs(… level="ERROR")` → upstream-timeout signatures.
- GATHER: `.topo getNeighborNodes … 'calls'` → upstream `checkout-service` (`149632df…`);
  `.entity … platform.config_change query='checkout'` → `cfg-checkout-retry`, `max_retries 2→5` 24h ago.
- DISCRIMINATE: `.entity … platform.deployment query='payment'` → `payment-gw v3.2.1`, trivial logging change → **ruled out** (red herring).
- CROSS-DOMAIN: `.entity … business.promotion query='active'` → `618 Flash Sale`, actual 38000 vs expected 12000 QPS (3.5×).
- CONCLUDE: retry amplification (×2.5) × promotion traffic (×3.5) = **8.75×** load → recommend `rollback_config_change` (medium risk, confirm first).

---

## Notes & gotchas

- **Always `-o json`**; parse `rows = data.data`, `columns = data.header`.
- **Real-data vs plan**: `.entity` / `.topo` / `.umodel` reads return real rows in
  open source. `get_metrics` / `get_logs` return a *plan* in open source and
  *data* against a PaaS endpoint (`mode='data'`) — the PaaS API return.
- Topology rows carry entity **IDs**, not names — resolve with `.entity with(ids=[…])`.
- Stay **read-only**: recommend remediation, don't execute it.
- **MCP alternative** (for MCP-capable agents instead of the CLI): connect
  `umodel-mcp` and call the `query_spl_execute` tool with
  `{ "workspace": "demo", "query": "<the same SPL>" }` (arg key is `query`, not
  `spl`). Everything above is transport-agnostic — same SPL either way.
