# RCA Evidence Patterns

Use this reference only when the normal RCA loop needs sharper discrimination.
It describes generic evidence patterns; it is not an incident answer key.

## Strong Positive Evidence

Root-cause evidence is strongest when all three are present:

1. Object-graph path: the candidate is connected to the impacted service through
   a typed relation such as calls, depends_on, targets, affects, triggers, or
   runs_as.
2. Telemetry timing: metric movement begins at or after the candidate activation
   time and before the symptom breach.
3. Mechanism: logs, change details, runbook knowledge, or metric ratios explain
   how the candidate creates the observed failure mode.

Examples of mechanisms:

- Retry/timeout config change: retry rate or client request volume rises,
  downstream latency/error increases, and logs show retry-exhausted or upstream
  timeout signatures.
- Business traffic event: actual traffic exceeds forecast, the business object
  links to the affected flow/service path, and service QPS/latency inflects when
  the event activates.
- Downstream saturation: the dependency shows latency/errors before or at the
  caller symptom, and logs name that dependency as the timeout source.
- Runtime capacity: pod/node/workload saturation predates user-visible latency
  and maps to the impacted service through runtime topology.

## Red-Herring Exclusion Tests

Deployment exclusion:

- `change_summary` is logging, formatting, docs, or metadata only.
- Rollout status is successful and no rollback occurred.
- Target service metrics are flat through the deploy window.
- Logs do not show new exceptions, dependency changes, connection handling, or
  performance regressions after deployment.

Config-change exclusion:

- The change does not affect calling behavior, load, timeout, retries, pools,
  circuit breakers, feature routing, or dependency selection.
- The target service is not on a topology path to the symptom.
- The metric/log inflection happens before the config took effect.

Traffic-event exclusion:

- No active business event overlaps the symptom window.
- Actual traffic is within the expected range.
- The event is not linked to the impacted order flow or service path.

Capacity exclusion:

- Saturation appears only after upstream load amplification.
- Only unrelated workloads/nodes are saturated.
- Scaling would reduce symptoms but would not remove the primary trigger.

## Query Shapes

Use placeholders and adapt names to the model:

```bash
umctl query run <workspace> ".entity with(domain='platform', name='platform.service', query='<service>') | limit 10" -o json

umctl query run <workspace> ".topo | graph-call getNeighborNodes('full', 1, [(:\"platform@platform.service\" {__entity_id__: '<service_id>'})]) | with(__relation_type__='calls')" -o json

umctl query run <workspace> ".entity with(domain='platform', name='platform.config_change', query='<service-or-caller>') | sort applied_at desc | limit 10" -o json

umctl query run <workspace> ".entity with(domain='platform', name='platform.deployment', query='<service>') | sort deployed_at desc | limit 10" -o json

umctl query run <workspace> ".entity with(domain='business', name='business.promotion', query='active') | limit 10" -o json
```

For metrics and logs, first resolve the entity ID, then call `get_metrics` or
`get_logs` on `.entity_set`; execute the returned plan rather than inventing a
storage query.
