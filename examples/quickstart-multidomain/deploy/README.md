# Quickstart demo stack

中文版本：[README.zh-CN.md](README.zh-CN.md)

Brings up UModel (serving the `multi-domain-quickstart` pack) with a seeded Prometheus and
Elasticsearch, so the pack's `get_metrics` / `get_logs` plans execute against real backends.
Connect an agent with the [`umodel-query`](../../../skills/umodel-query) skill, or run
[`verify.sh`](verify.sh).

## Requirements

Docker or Podman with Compose. Elasticsearch needs ~2 GB available to the engine.

## Start

```bash
sh examples/quickstart-multidomain/deploy/start.sh
```

`start.sh` runs `docker compose` (or `podman compose`) up, waits for the Elasticsearch seed and
the first Prometheus scrapes, and prints the endpoints. It runs:

| Service | URL | Role |
|---|---|---|
| UModel | `http://localhost:8080` | object graph + plan provider (`demo` workspace) |
| Prometheus | `http://localhost:9090` | metrics backend, fed by the exporter |
| Elasticsearch | `http://localhost:9200` | logs backend, seeded at startup |
| exporter | internal | emits the metric series Prometheus scrapes |

Seeded data: `checkout-service` is degraded — ~15% error rate, high p95, and ERROR logs (timeouts,
503s, retry-budget exhaustion); the other services are healthy. The telemetry is synthetic, shaped
to the pack's queries.

## Read it

The pack's storage endpoints point at `http://localhost:9090` / `http://localhost:9200`, so the
`get_metrics` / `get_logs` plans run as returned — no endpoint override.

With the [`umodel-query`](../../../skills/umodel-query) skill, point the agent at
`http://localhost:8080` (`UMCTL_ADDR`, or the MCP target) and ask in natural language, e.g. "read
checkout-service's request rate, error rate, p95 latency, and recent ERROR logs."

Without an agent:

```bash
sh examples/quickstart-multidomain/deploy/verify.sh
```

`verify.sh` lists the services, runs each metric plan's PromQL against `:9090`, and runs the log
plan's `_search` against `:9200`.

## Teardown

```bash
docker compose -f examples/quickstart-multidomain/deploy/docker-compose.yml down -v   # or: podman compose …
```

## Notes

- Telemetry is synthetic — a demo, not production data.
- `devops.event.deployment` is modeled on MySQL and is discoverable via `list_data_set`, but the
  executable plan methods are `get_metrics` (Prometheus) and `get_logs` (Elasticsearch); the stack
  seeds Prometheus and Elasticsearch only.
