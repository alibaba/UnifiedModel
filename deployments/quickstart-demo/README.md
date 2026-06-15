# Quickstart demo stack — read real data from Prometheus + Elasticsearch

中文版本：[README.zh-CN.md](README.zh-CN.md)

One command brings up the full **smart data-retrieval (智能读数)** demo: **UModel** (serving the
`multi-domain-quickstart` pack) + a real **Prometheus** + a real **Elasticsearch**, seeded so the
pack's `get_metrics` / `get_logs` plans return real values. You then connect your own agent
(Qoder / Codex / Claude Code) with the [`umodel-query`](../../skills/umodel-query) skill and read
the graph and the telemetry end to end.

## Prerequisites

- Docker + Docker Compose (Elasticsearch needs ~2 GB of memory available to Docker).
- An agent with the `umodel-query` skill, and `umctl` on PATH (or a Go toolchain). See the
  [skill setup](../../skills/umodel-query/SKILL.md).

## 1. Start the stack (one command)

```bash
docker compose -f deployments/quickstart-demo/docker-compose.yml up
```

Wait until the `es-seed` service prints `seed complete` (first run also pulls images and starts
Elasticsearch — a minute or two). What's now running:

| Service | URL | Role |
|---|---|---|
| UModel | `http://localhost:8080` | object graph + plan provider (the `demo` workspace) |
| Prometheus | `http://localhost:9090` | real metrics backend (seeded by the exporter) |
| Elasticsearch | `http://localhost:9200` | real logs backend (seeded with service logs) |
| exporter | (internal) | emits the demo metric series Prometheus scrapes |

The seeded data tells a story: **checkout-service** (`…0101`) is degraded — high error rate
(~15%) and high p99/p95 latency, with ERROR logs (timeouts, 503s, retry-budget exhaustion);
catalog-api / delivery-service / telemetry-collector are healthy. Give Prometheus ~1 minute of
scrapes before querying so `rate()` has samples.

## 2. Connect your agent

Install the skill and point the agent at UModel (the skill is CLI-first; MCP works too):

```bash
# Claude Code
/plugin marketplace add alibaba/UnifiedModel && /plugin install umodel@unifiedmodel
export UMCTL_ADDR=http://localhost:8080      # so umctl talks to the demo UModel
```

Qoder / Codex: install/point the skill the same way and set the UModel address to
`http://localhost:8080`.

## 3. Read data (the plan → execute flow)

`get_metrics` / `get_logs` return an executable **plan**. Its endpoint is a model placeholder
(`prometheus.devops.example:9090`, `https://elasticsearch.devops.example:9200`) — the agent
**overrides it with this stack's real endpoints**:

- Prometheus → **`http://localhost:9090`** (plain HTTP; no tenant/auth needed for the demo)
- Elasticsearch → **`http://localhost:9200`** (plain HTTP; security disabled for the demo)

This is exactly the "read the plan, adapt the endpoint, run it" flow the
[`umodel-query` skill](../../skills/umodel-query/references/metrics-logs.md) teaches.

**Ask the agent**, e.g.:

> "List the devops services and their status, then for checkout-service read its request rate,
> error rate, and p95 latency, and show its recent ERROR logs. Why is it degraded?"

The agent discovers the model, finds checkout-service (`…0101`), pulls the `get_metrics` /
`get_logs` plans from UModel, runs them against `localhost:9090` / `localhost:9200`, and reports
the elevated error rate + latency + the timeout/503 ERROR lines.

## Smoke test (optional, no agent)

`verify.sh` runs the same chain by hand (needs `jq` + `umctl`/Go):

```bash
sh deployments/quickstart-demo/verify.sh
```

It lists the services, fetches each metric plan and runs the PromQL against `:9090`, and fetches
the log plan and runs the `_search` against `:9200` — printing checkout-service's request/error
rates, p95, and ERROR log lines.

## Teardown

```bash
docker compose -f deployments/quickstart-demo/docker-compose.yml down -v
```

## Notes

- All telemetry here is **synthetic**, generated to match the pack's queries — it is a demo, not
  real production data.
- The `multi-domain-quickstart` pack's storage endpoints are intentional placeholders; this stack
  doesn't modify the pack — the agent points the plan at the local backends (as designed).
- The pack also models a **MySQL** event_set (`devops.event.deployment`); it's discoverable via
  `list_data_set`, but the plan-returning fetch methods are `get_metrics` (Prometheus) and
  `get_logs` (Elasticsearch).
