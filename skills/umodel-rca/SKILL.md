---
name: umodel-rca
description: >-
  Model-guided autonomous root-cause analysis over a UModel object-graph semantic
  layer. Use with `umodel-query` when diagnosing service degradation, SLO breach,
  latency/error spikes, incidents, outages, or questions like why a service is
  slow, degraded, erroring, or timing out. The skill guides evidence collection
  across object graph, metrics, logs, topology, runbooks, deployments, config
  changes, business traffic, and runtime capacity; it is especially useful for
  ruling out red herrings such as recent but unrelated deployments. Triggers:
  root cause, RCA, incident, SLO breach, outage, postmortem, latency spike, error
  spike, timeout, 根因分析, 故障排查, 告警定位, 为什么慢, SLO 击穿.
---

# UModel RCA

Investigate to a root cause over the UModel object graph. Use this as a method,
not a script: let evidence decide the next query, keep competing hypotheses alive,
and cite every conclusion with an object-graph, metric, or log source.

Load `umodel-query` first. It provides the read surfaces used here:
`.entity`, `.topo`, `.umodel`, `.entity_set | entity-call get_metrics`, and
`.entity_set | entity-call get_logs`.

## Setup

Same CLI as `umodel-query` — load that skill and follow its **Setup**; the essentials:

**1. Ensure `umctl` is on PATH** — UModel's read CLI:

```bash
command -v umctl || go install github.com/alibaba/UnifiedModel/cmd/umctl@latest   # needs Go 1.22+
```

No Go toolchain? Download a prebuilt `umctl` from the repo's Releases, or build from a clone
(`make build-cli` → `./bin/umctl`). Verify with `umctl version`.

**2. Point `umctl` at your UModel server and pick the workspace**:

```bash
export UMCTL_ADDR=http://<host>:8080     # or pass --addr per call, or `umctl configure`
umctl workspace list -o json            # the bundled demo workspace is `demo`
umctl query run <workspace> "<SPL>" -o json
```

For the incident-investigation demo with real Prometheus and Elasticsearch plans:

```bash
sh examples/incident-investigation/deploy/start.sh
```

If the user gives a different address, workspace, or transport, follow that instead.

## RCA Loop

### 1. Orient

Identify the incident and symptomatic entity:

- Search for an incident ID, service name, or degraded/SLO-critical entity.
- Read the impacted service fields: name, environment, owner, tier/SLO, status,
  recent errors, and linked datasets/runbook.
- Discover neighbors and runbook metadata before narrowing too early.

Useful starting shapes:

```bash
umctl query run <workspace> ".entity with(domain='platform', name='platform.service', query='<service-or-symptom>') | limit 10" -o json
umctl query run <workspace> ".entity with(domain='platform', name='platform.incident', query='<incident-or-service>') | limit 10" -o json
umctl query run <workspace> ".umodel with(kind='runbook_set')" -o json
```

### 2. Characterize

Confirm the symptom with telemetry, not only entity status or alert text.

- Pull target service metrics such as latency, error rate, timeout rate, QPS,
  retry rate, saturation, and error budget where available.
- Pull target service logs for timeout, retry-exhausted, circuit-breaker,
  dependency, deploy, or config landmarks.
- Prefer a short current window plus a longer timeline around suspected changes.

Use `get_metrics` / `get_logs` through the entity. The object graph fills the
storage selectors such as `service_id`; do not guess labels manually. In open
source these calls return executable Prometheus / Elasticsearch plans. Run the
returned plans as described in `umodel-query/references/metrics-logs.md`.

### 3. Hypothesize

Build a candidate set before deciding:

- Recent deployments on the degraded service.
- Config changes on the degraded service and upstream callers, especially retry,
  timeout, circuit breaker, pool, rate-limit, and feature flag changes.
- Upstream traffic pressure from callers, promotions, campaigns, batch jobs, or
  order flows.
- Downstream dependency saturation or channel/provider failures.
- Runtime capacity issues such as workload/pod/node pressure.

Do not treat recency as causality. A recent change is only suspicious when it has
a plausible mechanism, a topological path to the symptom, and telemetry/log
movement at the right time.

### 4. Gather Cross-Domain Evidence

Traverse both directions around the symptomatic service:

- Upstream callers: `.topo` relations where the caller sends traffic into the
  degraded service.
- Downstream dependencies: services, routers, channels, providers, databases,
  queues, or runtime workloads the degraded service calls or depends on.
- Change graph: `platform.config_change` and `platform.deployment` linked by
  target, affected service, or business trigger.
- Business graph: active promotions, campaigns, order flows, traffic forecasts,
  and actual traffic.
- Runtime graph: workloads, pods, namespaces, clusters, and capacity signals.

Use runbooks as a scaffold. Read linked `runbook_set` observations, knowledge,
and toolkits, but still verify each observation with graph/metric/log evidence.

If the path is ambiguous, read `references/evidence-patterns.md` for generic
positive and negative evidence patterns. Do not load it when the normal loop is
enough.

### 5. Correlate And Discriminate

Build a timeline with absolute or relative times from the evidence:

- When did the symptom start or breach?
- When did candidate changes take effect?
- Which metric/log series inflected at each candidate time?
- Does topology explain how the candidate can affect the degraded service?
- Does the mechanism quantitatively explain the blast radius or load?

Rule out candidates explicitly when evidence contradicts them. Common exclusion
tests:

- Deployment: trivial `change_summary`, successful rollout, no rollback, no
  latency/error/timeout inflection near deploy time, or wrong blast radius.
- Config change: no retry/timeout/pool/circuit behavior, wrong target, no
  upstream/downstream path, or no metric/log movement after activation.
- Traffic event: no active event, actual traffic close to forecast, or no path to
  the impacted service.
- Capacity issue: saturation appears after the overload rather than before it,
  or scaling pressure is localized away from the failing path.

Prefer the cause that explains the full timeline with the fewest unsupported
assumptions. Mark confidence lower if telemetry is missing or only indirect.

### 6. Conclude

Return a concise incident diagnosis:

```markdown
## Diagnosis
Symptom: <quantified service impact>

Timeline:
| Time | Evidence | Source |
|---|---|---|
| <T> | <event or inflection> | <object graph / metric / log reference> |

Root cause:
<one sentence with the triggering change/event and mechanism>

Causal chain:
1. <trigger>
2. <amplifier/mechanism>
3. <saturation/failure mode>
4. <user-visible impact>

Ruled out:
- <candidate>: <why excluded, with source>

Recommended action:
<read-only recommendation, tool if present, risk, confirmation requirement, and
verification metrics>

Confidence: <high|medium|low> because <evidence strength and gaps>
```

Every row in the timeline and every ruled-out item must cite its evidence type:
object graph, metric, log, or runbook knowledge.

## Evaluation Discipline

When validating this skill on a known scenario, keep the ground truth out of the
agent prompt. Use a fresh SubAgent or fresh thread, pass only the alert, UModel
endpoint, workspace, and skill paths, and score whether the agent independently
collects graph, metric, and log evidence before concluding.

Stay read-only. Recommend remediation, but do not execute rollback, rate-limit,
scale, or restart actions unless the user separately confirms the exact action.
