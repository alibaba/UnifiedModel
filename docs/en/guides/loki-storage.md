# Loki Storage

中文：[Loki Storage 指南](../../zh/guides/loki-storage.md)

Model Loki as a `loki` Storage linked to a `log_set` or a `metric_set`. Query Service returns a `loki_logql` plan for `get_logs` and for log-derived `get_metrics`; the caller runs that plan against Loki. Loki is telemetry Storage, not a GraphStore provider.

## Model and query

The [Loki example pack](../../../examples/loki-storage/README.md) contains an EntitySet, LogSet, MetricSet, Loki Storage, and both link layers. Its `StorageLink.spec.fields_mapping` maps DataSet fields to Loki **stream labels**. An entity field must have both DataLink and StorageLink mappings before the Loki renderer can use it. Log body fields are outside this mapping contract.

Import the example into an existing `demo` workspace:

```bash
go run ./cmd/umctl --addr http://localhost:8080 umodel import demo examples/loki-storage
```

Generate a log plan with an explicit time range and entity attributes:

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo --plan-format agent \
  --from '2026-11-03T10:00:00+08:00' --to '2026-11-03T11:00:00+08:00' \
  --entity-data '{"header":["id","environment"],"data":[["node-01","prod"]]}' \
  ".entity_set with(domain='ops', name='ops.node') | entity-call get_logs('ops', 'ops.node.logs', query='level = \"ERROR\"', storage_kind='loki')"
```

Use `get_metrics('ops', 'ops.node.metrics', 'error_rate', step='30s', storage_kind='loki')` for the LogQL metric in the same pack. Set `--from` and `--to` for log and range metric plans. An instant metric plan uses `--to` as its evaluation time. The caller must supply the time window before requesting a Loki plan.

The plan contains `query.dialect=loki_logql`, `query.queries[].logql`, endpoint, API prefix, mode, and relevant HTTP parameters. Logs use `/query_range`; range metrics use `/query_range`; instant metrics use `/query`. Multiple EntityData rows produce separate LogQL queries so their attribute combinations stay together. A Loki plan rejects unsupported filters and missing label mappings instead of dropping conditions.

## Execute the plan

The bundled read-only helper accepts the agent-format plan on standard input:

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo --plan-format agent \
  --from '2026-11-03T10:00:00+08:00' --to '2026-11-03T11:00:00+08:00' \
  --entity-data '{"header":["id"],"data":[["node-01"]]}' \
  ".entity_set with(domain='ops', name='ops.node') | entity-call get_logs('ops', 'ops.node.logs', storage_kind='loki')" \
  | python3 skills/umodel-query/scripts/run_loki_plan.py
```

The example only supplies model definitions. Point `spec.endpoint` at a real Loki service and ensure its stream labels match the StorageLink mapping. For optional authentication, configure `spec.auth` with `type: bearer` and `token_env`, or `type: basic` with `username_env` and `password_env`; set those environment variables only in the caller's process. The plan contains variable names, never credential values. If a trusted proxy supplies `X-Scope-OrgID`, leave `spec.tenant` unset.

Log query size uses explicit `QueryRequest.limit` (or SPL `| limit`) before `Storage.default_size`; a request limit takes precedence over an SPL limit. `Storage.max_size` bounds the result. The helper sorts, deduplicates, and applies the final size after combining multiple log queries. Metric results retain their separate query names and Loki vector or matrix values.

See [Query Service](query-service.md), [CLI reference](../reference/cli.md), and [Loki HTTP API](https://grafana.com/docs/loki/latest/reference/loki-http-api/).
