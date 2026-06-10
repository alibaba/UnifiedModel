# Agent Integration Guide

中文：[Agent 集成指南](../../zh/guides/agent-integration.md)

This guide shows how to connect an AI agent to UModel and how the agent uses the object graph to do real work. The worked example throughout is the [Incident Investigation Demo](../../../examples/incident-investigation/README.md) — an agent diagnoses a payment-gateway SLO breach by traversing the object graph and following a runbook.

UModel exposes an agent surface through the **Model Context Protocol (MCP)**, backed by AgentGateway and Query Service. Everything an agent reads goes through one SPL query surface, so the agent learns one contract and reuses it across models, entities, topology, datasets, and runbooks.

## 1. Connect an MCP client

The MCP server is `cmd/umodel-mcp`. It can load a demo workspace on startup with `--quickstart-sample`, so an agent has data to query immediately.

### Local (stdio)

Add to your MCP client config (`.mcp.json`, or the equivalent for Claude Code / Cursor / Qoder):

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

Start the server:

```bash
go run ./cmd/umodel-mcp --quickstart \
  --quickstart-sample examples/incident-investigation \
  --graphstore file.memory \
  --transport http --addr 0.0.0.0:8090
```

Connect from the client:

```json
{
  "mcpServers": {
    "umodel": { "type": "streamable-http", "url": "http://<host>:8090/mcp" }
  }
}
```

Transports (stdio, Streamable HTTP, HTTP+SSE), protocol versions, and a local smoke test are documented in the [MCP Reference](../reference/mcp.md).

## 2. What the agent can see

On connect, the agent discovers a fixed, safe surface (AgentGateway). Tool and resource discovery is also available over REST at `/api/v1/agent/{workspace}/discover`.

### Tools

| Tool | Default | Purpose |
|---|---|---|
| `query_spl_execute` | enabled | Run an SPL query (`.umodel` / `.entity` / `.entity_set` / `.topo` / `.runbook_set`) |
| `query_spl_explain` | enabled | Return the query plan and active providers without executing |
| `query_spl_examples` | enabled | Return safe, ready-to-run example queries |
| `umodel_validate` | enabled | Validate UModel element definitions |
| `umodel_import` | **disabled** | Import UModel files (write; enable server-side explicitly) |
| `entity_write` | **disabled** | Write entity payloads (write) |
| `entity_expire` | **disabled** | Expire entity payloads (write) |

Read tools are enabled by default; write tools are off unless the operator turns them on. An agent can explore and diagnose safely out of the box.

> The `query_spl_execute` argument key is **`query`**, not `spl`:
> `query_spl_execute {"workspace": "demo", "query": ".umodel | limit 5"}`.

### Resources

Metadata-only, read-only — they orient the agent before it queries:

| Resource | URI | Purpose |
|---|---|---|
| overview | `umodel://workspace/{ws}/overview` | API overview and safe entry points |
| schema-index | `umodel://workspace/{ws}/schema-index` | Model / schema summary |
| query-templates | `umodel://workspace/{ws}/query-templates` | Templates for `.umodel` / `.entity` / `.entity_set` / `.topo` |
| tool-capability-metadata | `umodel://workspace/{ws}/tool-capability-metadata` | Tool enablement + input/output schemas |

## 3. The query surface an agent uses

All four sources flow through `query_spl_execute`. The examples below are the actual queries from the incident-investigation walkthrough.

### `.entity` — find entities by full-text search

```
.entity with(domain='platform', name='platform.service', query='degraded')
  | project display_name, status, owner, sla_tier
```

`query=` searches across entity fields. `mode='vector'` or `mode='hyper'` switch to semantic / hybrid search when a search provider is configured; `topk=` bounds results.

### `.topo` — traverse the object graph

```
.topo | graph-call getNeighborNodes('full', 1,
  [(:"platform@platform.service" {__entity_id__: '63718b78868895d2590551b27ec6f51c'})])
  | with(__relation_type__='calls')
```

Graph-calls: `getNeighborNodes(direction, hops, nodes)`, `getDirectRelations(nodes)`, and `cypher(\`...\`)` for full Cypher. Topology rows carry entity IDs and relation properties; resolve display names with a follow-up `.entity` query when needed.

### `.entity_set` — discover and plan against datasets

```
# What can this EntitySet do?
.entity_set with(domain='platform', name='platform.service', ids=['...']) | entity-call __list_method__()

# Which datasets are attached?
.entity_set with(domain='platform', name='platform.service', ids=['...']) | entity-call list_data_set(['metric_set','log_set'], true)

# Pull telemetry — returns an executable query plan
.entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c'])
  | entity-call get_metrics('platform', 'platform.service.metrics', 'latency_p99_ms', step='30s')

.entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c'])
  | entity-call get_logs('platform', 'platform.service.logs', query='level = "ERROR"')
```

`get_metrics` / `get_logs` return a **query plan** — UModel open source is plan-only, so it renders the downstream PromQL / Elasticsearch query (with the entity's `service_id` substituted from the object graph) but does not execute it. A downstream executor runs the plan against real storage. See the [Query Service Guide](query-service.md) for the full pipe vocabulary.

## 4. `?format=agent` — the compact envelope for agents

By default a plan comes back wrapped in the assistant envelope with the plan JSON-encoded inside it. For agents, request the v1.1 agent envelope and the plan is returned as a **top-level JSON object**, with `data_source.*` folded to compact `{ref, kind}` references so it costs little context:

```bash
curl -s -X POST 'http://localhost:8080/api/v1/query/demo/execute?format=agent' \
  -H 'Content-Type: application/json' \
  -d '{"query":".entity_set with(domain=\"platform\", name=\"platform.service\", ids=[\"63718b78868895d2590551b27ec6f51c\"]) | entity-call get_metrics(\"platform\",\"platform.service.metrics\",\"latency_p99_ms\", step=\"30s\")"}'
```

Add `&include=spec` when a debugging agent needs the full storage config and link specs back. Full contract: [Plan Schema v1](../spec/plan-schema-v1.md).

## 5. End to end: an agent investigating an incident

With the incident-investigation workspace loaded, connect an MCP client and ask:

> "payment-gateway SLO breached, help me investigate."

The agent runs the object-graph loop:

1. **Locate** — `.entity` finds `payment-gateway` (`status=degraded`, `sla_tier=platinum`).
2. **Read its signals** — `get_metrics` returns the P99 latency plan; `get_logs` returns the error-log plan. The object graph turns "the degraded service" into the exact telemetry query without the agent hand-writing PromQL.
3. **Load the runbook** — the service links to `platform.service.ops`; the agent follows its structured observations.
4. **Observe upstream** — `.topo` finds `checkout-service` calling payment-gateway; a `config_change` shows retries raised 2→5.
5. **Rule out the red herring** — a recent `payment-gw v3.2.1` deploy turns out to be a trivial logging change.
6. **Cross-domain pressure** — the business layer shows the `618 Flash Sale` promotion driving 3.5× traffic.
7. **Correlate & recommend** — retry amplification × promotion traffic = 8.75× overload; the runbook recommends `rollback_config_change`.

The full walkthrough, runbook contents, and agent output example are in the [Incident Investigation Demo](../../../examples/incident-investigation/README.md).

## See also

- [MCP Reference](../reference/mcp.md) — transports, tools, resources, prompts, smoke test
- [Query Service Guide](query-service.md) — the SPL surface in full
- [Query And Agent Architecture](../architecture/query-and-agent.md) — AgentGateway / Query Service boundary
- [Plan Schema v1](../spec/plan-schema-v1.md) — plan contract and the `?format=agent` envelope
- [Incident Investigation Demo](../../../examples/incident-investigation/README.md) — the worked example
