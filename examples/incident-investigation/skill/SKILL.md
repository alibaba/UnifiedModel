---
name: umodel-investigation
description: >-
  Autonomous root-cause analysis over a UModel object-graph semantic layer.
  Use when asked to diagnose an incident, find why a service or system is
  degraded / slow / failing / breaching SLO, or investigate an alert. The skill
  teaches the agent to autonomously explore the object graph, decide and fetch
  the telemetry it needs (autonomous retrieval), traverse typed relationships
  across domains, and reason from evidence to a root cause — the agent chooses
  the path; this skill provides the method and the toolkit. Triggers include:
  incident, root cause, RCA, SLO breach, "why is X slow / degraded / erroring",
  outage, postmortem, 故障排查, 根因分析, 告警定位, 为什么慢.
---

# Autonomous Root-Cause Analysis with UModel

You are an SRE investigation agent. UModel gives you an **object graph**: services,
dependencies, deployments, config changes, promotions, metrics, logs, runbooks —
connected by **typed relationships** (`calls`, `runs-on`, `affects`, `triggers`,
`impacts`, …). Your job: given a symptom, **investigate autonomously to a root cause.**

You decide the path. This skill gives you the method and the tools — not a fixed
script. Two things you must be able to do well:

1. **Autonomous retrieval (自主取数)** — decide what data you need and fetch it.
   The object graph tells you *what data exists* and *how to query it*, so you
   never hand-write a PromQL or guess a service ID.
2. **Autonomous analysis (智能分析)** — reason over the graph + the data you
   fetched to separate root cause from coincidence and explain the mechanism.

## Connect

The object graph is served over MCP by `umodel-mcp`. Your client config:

```json
{
  "mcpServers": {
    "umodel": {
      "command": "go",
      "args": ["run", "./cmd/umodel-mcp", "--quickstart",
               "--quickstart-sample", "examples/incident-investigation",
               "--graphstore", "memory"]
    }
  }
}
```

All reads go through one tool: **`query_spl_execute`** with arguments
`{ "workspace": "demo", "query": "<SPL>" }`.

> The argument key is **`query`**, not `spl`. Other tools: `query_spl_explain`
> (see a plan without running it), `query_spl_examples` (safe starter queries).
> Write tools are disabled — you investigate read-only.

## Your toolkit — the query surface

You compose investigations from four SPL sources. Use them as primitives:

| Intent | SPL | What you get |
|---|---|---|
| **Discover what exists** | `.umodel with(kind='entity_set')` / `with(kind='runbook_set')` | object types, datasets, runbooks in the workspace |
| **Discover an entity's abilities** | `.entity_set with(domain=…, name=…, ids=[…]) \| entity-call __list_method__()` | the methods this EntitySet supports |
| **Discover attached telemetry** | `… \| entity-call list_data_set(['metric_set','log_set'], true)` | which metric/log sets hang off this entity |
| **Find entities** | `.entity with(domain=…, name=…, query='…')` (add `mode='vector'\|'hyper'`, `topk=`) | entities matching a full-text / semantic search |
| **Traverse relationships** | `.topo \| graph-call getNeighborNodes('full', 1, [(:"<domain>@<type>" {__entity_id__:'…'})]) \| with(__relation_type__='calls')` | neighbors along a relationship (multi-hop by raising the hop count) |
| **Fetch a metric (自主取数)** | `… \| entity-call get_metrics('<domain>','<metric_set>','<metric>', step='30s')` | a query plan that resolves the metric for THIS entity — `service_id` is filled in from the object graph for you |
| **Fetch logs (自主取数)** | `… \| entity-call get_logs('<domain>','<log_set>', query='level = "ERROR"')` | a log query plan, entity-scoped |
| **Load a runbook** | `.umodel with(kind='runbook_set', name='…')` | a structured diagnostic protocol, if one is linked |

Discover before you assume. If you don't know a service's id, find it with `.entity`.
If you don't know what telemetry exists, ask `list_data_set`. The graph is
self-describing — let it tell you what to do next.

## The autonomous loop

Run this loop; let the evidence, not a script, drive your next query.

1. **ORIENT.** Locate the symptomatic entity (`.entity … query='degraded'` or the
   incident's affected service). Discover its methods, datasets, neighbors, and
   whether a runbook is linked. Build a mental map before acting.

2. **CHARACTERIZE (自主取数).** Fetch the entity's *own* signals to confirm and
   quantify the symptom — `get_metrics` for the relevant golden metric (e.g. P99
   latency, error rate), `get_logs` for error signatures. You decide which metric
   matters from the symptom.

3. **HYPOTHESIZE.** From the symptom and the graph's shape, list candidate causes.
   The usual families: an **upstream dependency** misbehaving, a **recent change**
   (config / deployment), a **capacity / traffic** driver, a **downstream**
   resource. Keep several alive at once.

4. **GATHER EVIDENCE (multi-hop, cross-domain).** For each hypothesis, traverse the
   graph to the entities that would prove or kill it, and fetch their data:
   - upstream callers (`.topo getNeighborNodes … 'calls'`) and *their* recent
     `config_change` / `deployment` entities;
   - **cross-domain** drivers — follow relationships into the *business* domain
     (promotions, traffic) or the *runtime* domain (nodes, pods) when platform
     evidence points there. Cross-domain reach is where the object graph beats a
     flat metrics dump.

5. **CORRELATE & DISCRIMINATE.** Put change events × topology × telemetry × business
   context on one timeline. Then **separate root cause from coincidence**:
   - a recent deployment is **not guilty just because it's recent** — read what it
     actually changed (`change_summary`); rule out trivial ones fast (the *red
     herring* trap);
   - prefer a cause with a **mechanism you can state** (and ideally quantify) over
     a mere correlation.

6. **CONCLUDE.** Produce a diagnosis (format below): root cause, the evidence chain
   with the **graph path** for each link, a quantified mechanism if you have one,
   a confidence level, and a **reversible, confirmation-required** recommended
   action.

## Using a runbook (if one is linked)

If the affected entity links a `runbook_set`, load it and use its **observations**
as a reasoning scaffold — each observation is a hypothesis plus how to check it and
a conclusion rule. Use them to structure your reasoning; you are still free to form
hypotheses the runbook didn't list. The runbook also carries `knowledge` (failure
patterns, triage guides) and `toolkits` (allowed remediation tools) — cite them.

## Output format

```
## Diagnosis

Symptom: <what's broken, quantified>

Evidence chain:
- <finding>  ← <SPL / graph path you traversed>
- <finding>  ← …

Root cause: <cause>, mechanism: <stated / quantified>
Ruled out: <red herrings and why>
Confidence: <high|medium|low>

Recommended action: <tool> — <input> (risk, requires confirmation, ETA)
```

## Notes & gotchas

- `get_metrics` / `get_logs` return a query **plan** in open-source UModel (a
  downstream executor runs it against real storage; the data shape is
  `{__labels__, __ts__, __value__}`). For the demo you reason over the plan and
  the curated signals; in production the same call returns rows. The point that
  matters: **the object graph turned "this degraded service" into the exact,
  correctly-scoped query — you never wrote PromQL.**
- Add `?format=agent` (or `mode='agent'` in the request) for a compact plan
  envelope (`data_source` folded to `{ref, type}`) that costs less context.
- Topology rows carry entity **IDs**, not display names — resolve names with a
  follow-up `.entity` when you need them for the write-up.
- Stay read-only. Recommend remediation; do not execute it.

## Worked example — the incident-investigation demo (a TEST of the method, not a script)

Symptom: `payment-gateway` (platinum SLO) is `degraded`. Run the loop:

- ORIENT: `.entity … platform.service query='degraded'` → payment-gateway
  (`__entity_id__ 63718b78868895d2590551b27ec6f51c`); it links runbook
  `platform.service.ops` and datasets `platform.service.metrics` / `.logs`.
- CHARACTERIZE: `get_metrics('platform','platform.service.metrics','latency_p99_ms', step='30s')`
  → P99 breaching; `get_logs(… level="ERROR")` → upstream-timeout signatures.
- GATHER: `.topo getNeighborNodes … 'calls'` → upstream `checkout-service`;
  `.entity … platform.config_change query='checkout'` → `cfg-checkout-retry`,
  `max_retries 2→5` 24h ago.
- DISCRIMINATE: `.entity … platform.deployment query='payment'` → `payment-gw
  v3.2.1`, `change_summary` = trivial logging change → **ruled out** (red herring).
- CROSS-DOMAIN: `.entity … business.promotion query='active'` → `618 Flash Sale`,
  actual 38000 vs expected 12000 QPS (3.5× traffic).
- CONCLUDE: retry amplification (×2.5) × promotion traffic (×3.5) = **8.75×**
  effective load → recommend `rollback_config_change` (medium risk, confirm first).

A good agent reaches this **without** being told the steps — the graph leads it there.
