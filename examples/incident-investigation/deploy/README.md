# Incident-investigation demo stack

中文版本：[README.zh-CN.md](README.zh-CN.md)

Brings up UModel (serving the `incident-investigation` pack) with a seeded Prometheus and
Elasticsearch whose data matches the modeled incident — a payment-gateway SLO breach driven by a
checkout retry storm during the 618 flash sale. Connect an agent with the
[`umodel-query`](../../../skills/umodel-query) + [`umodel-rca`](../../../skills/umodel-rca) skills
and run a live root-cause analysis, or run [`verify.sh`](verify.sh).

## Requirements

Docker or Podman with Compose. Elasticsearch needs ~2 GB available to the engine.

## Start

```bash
sh examples/incident-investigation/deploy/start.sh
```

`start.sh` runs `docker compose` (or `podman compose`) up, waits for the Elasticsearch seed and
the first Prometheus scrapes, and prints the endpoints. It runs:

| Service | URL | Role |
|---|---|---|
| UModel | `http://localhost:8080` | object graph + plan provider (`demo` workspace) |
| Prometheus | `http://localhost:9090` | seeded metrics, fed by the exporter |
| Elasticsearch | `http://localhost:9200` | seeded logs |
| exporter | internal | emits the `platform_service_*` series Prometheus scrapes |

The seeded telemetry matches the graph: `payment-gateway` p99 ≈ 2150ms with ~14.8% errors and
high upstream-timeout rate; `checkout-service` drives a ~55% client-retry rate (the `max_retries`
2→5 config change); `payment-router` and the Alipay / WeChat Pay / UnionPay channels are slow and
erroring; the rest of the platform is healthy.

## Run the RCA

The pack's storage endpoints point at `http://localhost:9090` / `http://localhost:9200`, so
`get_metrics` / `get_logs` plans run as returned. Point an agent (with the `umodel-query` +
`umodel-rca` skills) at `http://localhost:8080` (`UMCTL_ADDR`, or the MCP target) and ask:

> payment-gateway is degraded — find the root cause.

The agent characterizes the symptom from real telemetry (`get_metrics latency_p99_ms` /
`error_rate`, `get_logs level="ERROR"`), traverses the topology to the upstream `checkout-service`,
its `checkout-retry-policy-v2` config change and the active 618 promotion, rules out the
`payment-gw v3.2.1` deployment (a logging change), and concludes the retry-amplification storm.

Without an agent:

```bash
sh examples/incident-investigation/deploy/verify.sh
```

## Teardown

```bash
sh examples/incident-investigation/deploy/stop.sh          # stop + remove containers, network, volumes
sh examples/incident-investigation/deploy/stop.sh --all    # also remove the built image
```

## Notes

- Telemetry is synthetic, shaped to match the modeled incident — a demo, not production data.
- The pack also models a MySQL deployment-event set and a runbook; the executable plan methods
  seeded here are `get_metrics` (Prometheus) and `get_logs` (Elasticsearch).
