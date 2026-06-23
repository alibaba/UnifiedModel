# clickhouse

ClickHouse 存储，用于描述 ClickHouse 或其他 SQL 表后端的配置和查询规划信息。它通过 spec.family 路由到 sql-table 查询家族，把 get_metrics 渲染为 SQL。

**Kind**: `clickhouse`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `family` | `string` |  |  | 该存储渲染所用的查询家族。设为 sql-table 即可经由 sql-table 渲染器把 get_metrics 渲染为 SQL；不设则该存储仅透传、不渲染。该家族此后的每个后端都只是配置。 |
| `endpoint` | `string` | 是 |  | ClickHouse 的访问地址，例如 http://clickhouse:8123。 |
| `database` | `string` |  |  | 查询所用的数据库名。 |
| `table` | `string` | 是 |  | 指标所在的表名。sql-table 渲染器从该表 SELECT 数据。 |
| `time_column` | `string` |  | `timestamp` | 时间列名，用于时间范围过滤与降采样。默认值为 timestamp。 |
| `value_column` | `string` |  | `value` | 指标数值列名。默认值为 value。 |
| `metric_column` | `string` |  | `metric_name` | 指标名所在的列名，用于按指标过滤。默认值为 metric_name。 |
| `default_query_type` | enum: `instant`, `range` |  | `range` | 默认查询类型。instant 表示取最新值，range 表示区间降采样。 |
| `default_step` | `string` |  |  | 区间查询的默认降采样步长，例如 60s、1m。 |
| `credential_ref` | `string` |  |  | 凭据引用标识，例如 secret://clickhouse-prod-readonly。不得在 UModel 中保存明文用户名、密码或 Token。 |
| `tls_verify` | `boolean` |  | `true` | 是否校验 TLS 证书。默认值为 true。 |
| `properties` | map&lt;string, string&gt; |  |  | ClickHouse 的额外非敏感配置，以键值对形式存储。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于标注该 ClickHouse 存储的标签。 |
