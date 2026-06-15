---
name: umodel-rca
description: >-
  Model-guided autonomous root-cause analysis over a UModel object-graph semantic
  layer. Given a symptom, the agent explores the object graph, fetches the
  telemetry it needs (the model auto-scopes the query — no hand-written PromQL),
  traverses relationships across domains, and reasons from evidence to a root
  cause. Use when asked to diagnose an incident, find a root cause, explain why a
  service is slow / degraded / erroring, or investigate an alert or SLO breach.
  Builds on the read toolkit in the `umodel-query` skill (load both). Triggers:
  root cause, RCA, incident, SLO breach, outage, postmortem, why is X slow /
  degraded / erroring, 根因分析, 故障排查, 告警定位, 为什么慢.
---

# UModel RCA — autonomous root-cause analysis

Given a symptom, **investigate autonomously to a root cause** over the UModel
object graph. You decide the path; this skill gives the method, not a script.

It builds on the read toolkit in the **`umodel-query`** skill (entity / topology /
model reads via `umctl query run <ws> "<SPL>" -o json`; rows in `data.data`,
columns in `data.header`). Load both. The essentials you'll use are recapped
inline below.

## Setup

Same server and CLI as `umodel-query` — ensure `umctl` is on PATH, pointed at your
UModel server, and using the right workspace (`umctl workspace list`; the demo is
`demo`). See that skill's **Setup** for details. The bundled demo serves sample data
with `make quickstart QUICKSTART_SAMPLE=examples/incident-investigation`.

## Model-guided data fetch (autonomous retrieval)

To get evidence, call `get_metrics` / `get_logs` on the entity. The object graph
auto-scopes the query — it fills in `service_id` from the `fields_mapping`, so you
never hand-write PromQL or guess an ID:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call get_metrics('platform','platform.service.metrics','latency_p99_ms', step='30s')" -o json
```

These return an executable **plan**; run it against Prometheus / Elasticsearch to get the
values — see *"Read metrics & logs — read the plan, then run it"* in the `umodel-query`
skill for the plan's fields and how to execute. (A PaaS endpoint with `mode='data'`
returns the rows directly.)

## The autonomous RCA loop

Run this loop; let evidence — not a fixed script — drive your next query.

1. **ORIENT** — locate the symptomatic entity
   (`.entity … query='degraded'`). Read its methods, datasets, neighbors, and
   linked runbook (the `umodel-query` reads).
2. **CHARACTERIZE (fetch)** — pull its own signals (`get_metrics` / `get_logs`) to
   confirm and quantify the symptom.
3. **HYPOTHESIZE** — candidate causes: upstream dependency, recent change
   (config / deploy), capacity / traffic, downstream resource. Keep several alive.
4. **GATHER EVIDENCE (multi-hop, cross-domain)** — traverse `.topo` to upstream
   callers and *their* recent `config_change` / `deployment`; follow links into the
   **business** domain (promotions / traffic) or **runtime** domain (nodes / pods).
   Cross-domain reach is where the object graph beats a flat metrics dump.
5. **CORRELATE & DISCRIMINATE** — line up changes × topology × telemetry × business
   context on a timeline. Separate root cause from coincidence: a recent deploy is
   **not guilty just because it's recent** — read its `change_summary` and rule out
   trivial ones (the *red herring* trap). Prefer a cause with a **stated, ideally
   quantified, mechanism**.
6. **CONCLUDE** — root cause + evidence chain (cite the graph path per link) +
   quantified mechanism + confidence + a **reversible, confirmation-required**
   recommendation.

## Runbook as scaffold

If the entity links a `runbook_set` (read it with `.umodel with(kind='runbook_set',
name='…')`), use it as a reasoning frame. Its `spec.knowledge` holds documented
**failure patterns** — in the demo, a *Retry Storm* pattern with the exact amplification
formula `base_qps × promotion_multiplier × (new_retries / old_retries)`; `spec.automations`
are the wired-up context-collection / remediation actions. Cite the matching knowledge
entry as your mechanism (it's where the worked example's 8.75× comes from); you may still
form hypotheses it didn't list.

## Output

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

## Worked example — incident-investigation demo (a TEST of the method, not a script)

Symptom: `payment-gateway` (platinum SLO) is `degraded`. A good agent reaches the
root cause **without** being told the steps:

- ORIENT: `.entity … query='degraded'` → payment-gateway (`63718b78…`), links
  runbook `platform.service.ops` + datasets `platform.service.metrics`/`.logs`.
- CHARACTERIZE: `get_metrics(… 'latency_p99_ms' …)` → P99 breaching;
  `get_logs(… level="ERROR")` → upstream-timeout signatures.
- GATHER: `.topo getNeighborNodes … | where __relation_type__='calls'` → upstream
  caller `checkout-service` (`149632df…`); `.entity … platform.config_change query='checkout'`
  → `checkout-retry-policy-v2`: `max_retries 2→5`, `timeout 500→2000ms` (targets svc-checkout).
- DISCRIMINATE: `.entity … platform.deployment query='payment'` → `payment-gw
  v3.2.1`, trivial logging change → **ruled out** (red herring).
- CROSS-DOMAIN: `.entity … business.promotion query='active'` → `618 Flash Sale`,
  actual 38000 vs expected 12000 QPS (3.5×).
- CONCLUDE: retry amplification (×2.5) × promotion traffic (×3.5) = **8.75×** load
  → recommend `rollback_config_change` (medium risk, confirm first).

## Notes

- Reuse the `umodel-query` reads for everything except telemetry fetch; this skill
  adds the fetch + the reasoning loop.
- `get_metrics` / `get_logs`: *plan* in open source — execute it against Prometheus /
  Elasticsearch to get evidence (see `umodel-query`); *data* directly via a PaaS endpoint.
- Stay **read-only** — recommend remediation, do not execute it.
- **MCP alternative**: call `query_spl_execute` with `{ "workspace", "query" }`
  instead of the CLI; same SPL.
