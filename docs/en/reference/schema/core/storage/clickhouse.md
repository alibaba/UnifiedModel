# clickhouse

ClickHouse storage, used to define ClickHouse or other SQL-table backend metadata for query planning. It routes to the sql-table query family via spec.family, rendering get_metrics as SQL.

**Kind**: `clickhouse`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `family` | `string` |  |  | The query family this storage renders through. Set sql-table to render get_metrics as SQL via the sql-table renderer; omit it and the storage stays passthrough-only. Every backend in the family after the first is conf… |
| `endpoint` | `string` | yes |  | The endpoint URL of ClickHouse, for example http://clickhouse:8123. |
| `database` | `string` |  |  | The database name used for queries. |
| `table` | `string` | yes |  | The table holding the metrics. The sql-table renderer SELECTs from this table. |
| `time_column` | `string` |  | `timestamp` | The time column used for time-range filtering and downsampling. Defaults to timestamp. |
| `value_column` | `string` |  | `value` | The metric value column. Defaults to value. |
| `metric_column` | `string` |  | `metric_name` | The column holding the metric name, used to filter by metric. Defaults to metric_name. |
| `default_query_type` | enum: `instant`, `range` |  | `range` | Default query type. instant returns the latest value, range downsamples over an interval. |
| `default_step` | `string` |  |  | Default downsampling step for range queries, for example 60s or 1m. |
| `credential_ref` | `string` |  |  | Credential reference, for example secret://clickhouse-prod-readonly. Plaintext usernames, passwords, or tokens must not be stored in UModel. |
| `tls_verify` | `boolean` |  | `true` | Whether TLS certificates should be verified. Defaults to true. |
| `properties` | map&lt;string, string&gt; |  |  | Additional non-sensitive ClickHouse configuration, stored as key-value pairs. |
| `tags` | map&lt;string, string&gt; |  |  | Tags used to describe this ClickHouse storage. |
