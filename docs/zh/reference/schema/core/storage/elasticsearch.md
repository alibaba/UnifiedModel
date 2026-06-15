# elasticsearch

Elasticsearch 存储，用于描述 Elasticsearch 集群、索引以及查询规划所需的连接信息。

**Kind**: `elasticsearch`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `endpoint` | `string` | 是 |  | Elasticsearch 集群访问地址，例如 https://es.example.com:9200。 |
| `index` | `string` | 是 |  | 默认查询索引名称，也可以是 Elasticsearch 支持的索引通配表达式。 |
| `index_pattern` | `string` |  |  | 面向按时间滚动索引的索引模式，例如 logs-${yyyy.MM.dd}。如果为空，查询规划使用 index 字段。 |
| `version` | `string` |  |  | Elasticsearch 版本号或版本族，例如 7.x、8.x。 |
| `query_dialect` | enum: `elasticsearch_dsl`, `lucene`, `eql` |  | `elasticsearch_dsl` | 查询生成方言。默认生成 Elasticsearch Query DSL。 |
| `time_field` | `string` |  |  | 时间过滤字段名，用于将请求时间范围下推到 Elasticsearch 查询中。 |
| `default_size` | `integer` |  | `1000` | 未显式指定 limit 时生成查询的默认 size。 |
| `max_size` | `integer` |  | `10000` | 查询规划允许生成的最大 size，用于避免生成过大的查询计划。 |
| `routing` | `string` |  |  | 可选的 Elasticsearch routing 值。用于规划需要固定 routing 的查询。 |
| `credential_ref` | `string` |  |  | 凭据引用标识，例如 secret://es-prod-readonly。不得在 UModel 中保存明文用户名、密码或 Token。 |
| `tls_verify` | `boolean` |  | `true` | 是否校验 TLS 证书。默认值为 true。 |
| `headers` | map&lt;string, string&gt; |  |  | 非敏感 HTTP 头，用于租户、路由等查询上下文。不得存放认证密钥。 |
| `properties` | map&lt;string, string&gt; |  |  | Elasticsearch 的额外非敏感配置，以键值对形式存储。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于标注该 Elasticsearch 存储的标签。 |
