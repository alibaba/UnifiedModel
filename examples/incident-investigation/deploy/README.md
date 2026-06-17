# Incident-investigation demo stack

中文版本：[README.zh-CN.md](README.zh-CN.md)

Brings up UModel (serving the `incident-investigation` pack) with a seeded Prometheus and
Elasticsearch whose data matches the modeled incident — a payment-gateway SLO breach driven by a
checkout retry storm during the flash sale. Connect an agent with the
[`umodel-query`](../../../skills/umodel-query) + [`umodel-rca`](../../../skills/umodel-rca) skills
and run a live root-cause analysis, or run [`verify.sh`](verify.sh).

## Requirements

Docker or Podman with Compose. Elasticsearch needs ~2 GB available to the engine.

The demo publishes host ports `8080` (UModel), `9090` (Prometheus), and `9200` (Elasticsearch).
`start.sh` refuses to start if any of them is already in use — so it never reports "ready" while
actually talking to an unrelated service on the same port. To run alongside an existing
umodel-server / Prometheus / Elasticsearch, pick free ports:

```bash
UMODEL_PORT=18080 PROM_PORT=19090 ES_PORT=19200 sh examples/incident-investigation/deploy/start.sh
```

## Start

```bash
sh examples/incident-investigation/deploy/start.sh
```

`start.sh` runs `docker compose` (or `podman compose`) up, waits for the metric backfill, the
Elasticsearch seed and the first Prometheus scrapes, and prints the endpoints. It runs:

| Service | URL | Role |
|---|---|---|
| UModel | `http://localhost:8080` | object graph + plan provider (`demo` workspace) |
| Prometheus | `http://localhost:9090` | ~72h backfilled history + live tail |
| Elasticsearch | `http://localhost:9200` | ~72h of seeded logs |
| exporter | internal | emits the live `platform_service_*` series Prometheus scrapes |
| metrics-gen | internal (one-shot) | writes ~72h of history; `promtool` backfills it before Prometheus starts |
| es-seed | internal (one-shot) | generates and bulk-loads ~72h of logs |

### Telemetry spans the incident window

The seeded telemetry covers the whole ~72h incident, not just the current snapshot, following the
pack's [timeline](../README.md):

| Phase | Window | What the data shows |
|---|---|---|
| healthy | T-72h … T-24h | everything nominal |
| retries-up | T-24h … T-4h | the `max_retries` 2→5 config change steps `checkout-service` client-retry rate 8% → 55%; the T-12h `payment-gw v3.2.1` deploy leaves **no** metric trace |
| breach | T-4h … now | the flash sale goes active (3.5×) → retry storm: `payment-gateway` p99 ≈ 2000ms, ~14.8% errors, high upstream-timeout rate; `payment-router` and the Alipay / WeChat Pay / UnionPay channels slow and erroring |

So an instant query sees the current breach and a range query sees the arc — retry rate inflecting
at the config change, latency and errors at the promotion, and the deployment ruled out by its flat
curve. `verify.sh` prints both.

## Run the RCA

The pack's storage endpoints point at `http://localhost:9090` / `http://localhost:9200`, so
`get_metrics` / `get_logs` plans run as returned. Point an agent (with the `umodel-query` +
`umodel-rca` skills) at `http://localhost:8080` (`UMCTL_ADDR`, or the MCP target) and ask:

> payment-gateway is degraded — find the root cause.

The agent characterizes the symptom from real telemetry (`get_metrics latency_p99_ms` /
`error_rate`, `get_logs level="ERROR"`), traverses the topology to the upstream `checkout-service`,
its `checkout-retry-policy-v2` config change and the active flash-sale promotion, rules out the
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
- Everything is relative to "now", so the demo never expires. Metric history is generated relative
  to now and loaded with `promtool tsdb create-blocks-from openmetrics` before Prometheus starts; the
  exporter then continues the same series live. Logs are generated relative to now. The entity
  timeline (deployment / config-change / incident / promotion timestamps) is re-anchored to now at
  startup by the demo image's entrypoint — config ~now-24h, deploy ~now-12h, promo active ~now-4h,
  incident ~now — matching the telemetry.
- The logs include correlated request traces: a failed checkout shares one `trace_id` across
  `checkout-service → payment-gateway → payment-router → channel → provider`, so you can follow a
  single request down the stack and see the timeout originate downstream and surface up as retry
  exhaustion. They also carry the config-change, deployment, and circuit-breaker landmark events.
- The pack also models a MySQL deployment-event set and a runbook; the executable plan methods
  seeded here are `get_metrics` (Prometheus) and `get_logs` (Elasticsearch).
