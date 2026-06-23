# victoriametrics

VictoriaMetrics 存储，用于描述 VictoriaMetrics 或其他 PromQL 兼容时序后端的配置和查询规划信息。它通过 spec.family 复用 label-timeseries 查询家族，UModel 无需为其编写任何代码。

**Kind**: `victoriametrics`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `family` | `string` |  |  | 该存储渲染所用的查询家族。VictoriaMetrics 兼容 PromQL，设为 label-timeseries 即可复用 Prometheus 查询计划渲染器，无需编写任何 Go 代码——这正是“新增同族后端只写配置”的体现；不设则该存储仅透传、不渲染。 |
| `endpoint` | `string` | 是 |  | VictoriaMetrics 或 PromQL 兼容服务的访问地址，例如 http://victoriametrics:8428。 |
| `api_prefix` | `string` |  | `/api/v1` | PromQL HTTP API 前缀。默认值为 /api/v1。 |
| `default_query_type` | enum: `instant`, `range` |  | `instant` | 默认 PromQL 查询类型。instant 表示即时查询，range 表示区间查询。 |
| `default_step` | `string` |  |  | 区间查询的默认 step，例如 60s、1m。 |
| `lookback_delta` | `string` |  |  | PromQL 查询规划使用的默认回看窗口，例如 5m。 |
| `tenant` | `string` |  |  | 可选租户标识，用于多租户 VictoriaMetrics 集群版。 |
| `tenant_header` | `string` |  |  | 多租户系统使用的租户 HTTP 头名称，例如 X-Scope-OrgID。 |
| `credential_ref` | `string` |  |  | 凭据引用标识，例如 secret://victoriametrics-prod-readonly。不得在 UModel 中保存明文用户名、密码或 Token。 |
| `tls_verify` | `boolean` |  | `true` | 是否校验 TLS 证书。默认值为 true。 |
| `external_labels` | map&lt;string, string&gt; |  |  | 外部标签，用于 query planning 时补充查询上下文或结果来源说明。 |
| `headers` | map&lt;string, string&gt; |  |  | 非敏感 HTTP 头，用于租户、路由等查询上下文。不得存放认证密钥。 |
| `properties` | map&lt;string, string&gt; |  |  | VictoriaMetrics 的额外非敏感配置，以键值对形式存储。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于标注该 VictoriaMetrics 存储的标签。 |
