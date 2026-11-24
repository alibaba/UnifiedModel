# loki

Loki 存储，用于生成日志及日志派生指标的 LogQL 查询计划。

**Kind**: `loki`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `endpoint` | `string` | 是 |  | Loki 服务的 HTTP 或 HTTPS 地址。 |
| `api_prefix` | `string` |  | `/loki/api/v1` | Loki HTTP API 前缀，默认 /loki/api/v1。 |
| `default_query_type` | enum: `instant`, `range` |  | `instant` | 指标查询的默认模式。 |
| `default_step` | `string` |  |  | 区间指标查询的默认采样步长，例如 30s。 |
| `search_filter` | `string` |  |  | 使用 Loki 标签名表达的可选静态过滤条件。 |
| `default_size` | `integer` |  | `100` | 日志查询未指定 limit 时的默认条数，默认 100。 |
| `max_size` | `integer` |  | `1000` | 单次日志查询的最大条数，默认 1000。 |
| `tenant` | `string` |  |  | 由客户端传递的可选租户 ID。 |
| `tenant_header` | `string` |  | `X-Scope-OrgID` | 租户 HTTP 头名称，默认 X-Scope-OrgID。 |
| `auth` | object |  |  | 认证方式与环境变量名；不得存放凭据值。 |
| `auth.type` | enum: `bearer`, `basic` | 是 |  |  |
| `auth.token_env` | `string` |  |  |  |
| `auth.username_env` | `string` |  |  |  |
| `auth.password_env` | `string` |  |  |  |
| `tls_verify` | `boolean` |  | `true` | 是否校验 TLS 证书，默认 true。 |
| `headers` | map&lt;string, string&gt; |  |  | 不包含认证凭据的附加 HTTP 头。 |
