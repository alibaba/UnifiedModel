---
name: umodel-query
description: >-
  Read from a UModel object-graph semantic layer with the `umctl` CLI (MCP
  alternative noted). Three kinds of read: (1) entities & relationships / topology
  (`.entity`, `.topo`) and (2) the model itself (`.umodel`, and `.entity_set`
  methods) return real rows; (3) metrics & logs (`get_metrics` / `get_logs`)
  return an executable *plan* — PromQL / Elasticsearch DSL with the entity id
  pre-substituted — that you run against the backend. Against a PaaS endpoint the
  same calls return data rows instead of a plan. Use to query or read UModel
  entities, relations, topology, or model metadata; to read a service's metrics or
  logs; to look up services and dependencies; or to discover what objects,
  datasets, and methods exist. For root-cause analysis on top of these reads, see
  the `umodel-rca` skill. Triggers: UModel, object graph, .entity / .topo /
  .umodel / .entity_set, query entities, read topology, read metrics / logs,
  get_metrics / get_logs, list services / dependencies / datasets, 实体查询,
  关系/拓扑查询, 读模型, 读指标, 读日志, 查指标, 查日志, 查服务依赖.
---

# UModel Query — read entities, relationships, the model, and telemetry

UModel is an **object-graph semantic layer**: enterprise objects (services, Pods,
deployments, config changes, promotions, …), their typed relationships (`calls`,
`depends_on`, `affects`, …), and the datasets (metrics, logs) hanging off them — all read
through one SPL surface via the `umctl` CLI (MCP alternative at the bottom).

This file is the **overview + setup**. Each query surface has a focused guide under
[`references/`](references/) — **read the one your task needs** (don't load them all).

## Setup (CLI-first)

**1. Ensure `umctl` is on PATH** — UModel's read CLI:

```bash
command -v umctl || go install github.com/alibaba/UnifiedModel/cmd/umctl@latest   # needs Go 1.22+
```

No Go toolchain? Download a prebuilt `umctl` from the repo's Releases, or build from a clone
(`make build-cli` → `./bin/umctl`). Verify with `umctl version`.

**2. Point `umctl` at your UModel server** — set the address explicitly (flag, env, or a
saved profile):

```bash
umctl --addr http://<host>:8080 query run <workspace> "<SPL>" -o json   # per call
export UMCTL_ADDR=http://<host>:8080                                    # or for the session
umctl configure                                                         # or save a profile
```

**3. Pick the workspace** — every read takes a workspace name. List what the server has and
use the one your data lives in (the bundled demo is `demo`):

```bash
umctl workspace list -o json
```

> No server yet? The bundled demo serves one with sample data on `:8080`:
> `make quickstart QUICKSTART_SAMPLE=examples/incident-investigation` (needs a repo clone + Go).

**Always pass `-o json`.** Plain reads put column names in `data.header` and rows in
`data.data` (a matrix) — zip them to read records. (Entity-call results wrap differently; see
the entity-set guide.)

## Query surfaces — open the reference you need

| Your goal | SPL surface | Guide |
|---|---|---|
| Read objects (services, deployments, config changes…) by type / search / id | `.entity` | [references/entity.md](references/entity.md) |
| Traverse relationships, dependencies, topology | `.topo` | [references/topology.md](references/topology.md) |
| List what object types / datasets / links / runbooks exist | `.umodel` | [references/model.md](references/model.md) |
| Call an EntitySet's methods (discover via `__list_method__`, list datasets) | `.entity_set \| entity-call` | [references/entity-set.md](references/entity-set.md) |
| Read a service's **metrics / logs** (fetch a plan, then run it) | `get_metrics` / `get_logs` | [references/metrics-logs.md](references/metrics-logs.md) |

**How they relate:** `.umodel` defines the **types**. The `domain` + `name` you pass
everywhere names one of those definitions — for `.entity` / `.entity_set` it's an
**EntitySet** (`.umodel with(kind='entity_set')`); for `get_metrics` / `get_logs` it's a
**MetricSet** / **LogSet**. `.entity` reads the runtime **instances** of an EntitySet;
`.entity_set` calls **methods** on the EntitySet itself; the same `domain`/`name` join them.

Typical flow: `.umodel` to learn the types → `.entity` to find an object and grab its
`__entity_id__` → `.topo` / `.entity_set` / telemetry build on that id.

## Notes

- Stay **read-only**.
- `.entity` / `.topo` / `.umodel` reads return **real rows** in open source. `get_metrics` /
  `get_logs` return an executable *plan* you run against Prometheus / Elasticsearch (or, against
  a PaaS endpoint with `mode='data'`, rows directly) — see
  [references/metrics-logs.md](references/metrics-logs.md).
- **MCP alternative** (instead of the CLI): connect `umodel-mcp` and call the
  `query_spl_execute` tool with `{ "workspace": "demo", "query": "<the same SPL>" }` (arg key
  is `query`, not `spl`). Same SPL, same results.
