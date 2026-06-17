# Extensible storage: adding backends with configuration

UModel routes a storage to a query-plan renderer by **query family**, not by a
hardcoded storage kind. A family is a query model — `label-timeseries` (PromQL),
`document-search` (Elasticsearch DSL), `sql-table` (SQL). A storage selects its
family with `spec.family`. This example shows the two ways a new backend is added.

## Same family — configuration only

[VictoriaMetrics](https://victoriametrics.com/) speaks the Prometheus query API,
so it shares the `label-timeseries` family. It needs **no Go code at all**:

```yaml
kind: victoriametrics
spec:
  family: label-timeseries
  endpoint: "http://localhost:8428"
```

`get_metrics` on an entity backed by this storage produces the same
`prometheus_promql` plan it would for native Prometheus — the existing
`label-timeseries` renderer handles it. See [`victoriametrics.storage.yaml`](victoriametrics.storage.yaml).

## New family — one renderer, then configuration

[ClickHouse](https://clickhouse.com/) uses SQL, a query model entirely different
from PromQL. The `sql-table` family adds one renderer for that model; it consumes
the same resolved constraints (entity bindings + filters) the other families do
and formats them as a `WHERE` clause. A ClickHouse backend then routes to it by
configuration:

```yaml
kind: clickhouse
spec:
  family: sql-table
  endpoint: "http://localhost:8123"
  table: service_metrics
```

`get_metrics` renders to SQL — `SELECT toStartOfInterval(ts, INTERVAL 1 MINUTE), avg(value) … WHERE … GROUP BY …` — with `{from}` / `{to}` placeholders the caller fills from the plan's time range. See [`clickhouse.storage.yaml`](clickhouse.storage.yaml).

Every backend in the `sql-table` family after the first is configuration only,
exactly like VictoriaMetrics in `label-timeseries`.

## How a kind is added

Both kinds were schema-only:

- [`schemas/core/storage/victoriametrics.schema.yaml`](../../schemas/core/storage/victoriametrics.schema.yaml) and [`clickhouse.schema.yaml`](../../schemas/core/storage/clickhouse.schema.yaml) define the kinds.
- The Go, Python, and Java SDK types and the reference docs are generated from those schemas by `make expand` and `make docs-schema`.
