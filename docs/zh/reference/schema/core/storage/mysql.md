# mysql

MySQL 存储，用于描述 MySQL 实例、数据库、表以及查询规划所需的连接信息。

**Kind**: `mysql`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `endpoint` | `string` | 是 |  | MySQL 实例访问地址，格式通常为 host:port。 |
| `database` | `string` | 是 |  | 默认查询数据库名称。 |
| `table` | `string` |  |  | 默认查询表名。若 DataSet 或查询参数中指定表名，可覆盖此字段。 |
| `sql_template` | `string` |  |  | 可选 SQL 模板，用于特殊查询规划场景。模板必须保持只读语义，不应包含 INSERT、UPDATE、DELETE、DDL 等语句。 |
| `sql_dialect` | enum: `mysql`, `ansi` |  | `mysql` | SQL 方言。MySQL 存储默认使用 mysql。 |
| `time_field` | `string` |  |  | 时间过滤字段名，用于将请求时间范围下推到 SQL WHERE 条件中。 |
| `time_unit` | enum: `second`, `millisecond`, `microsecond`, `nanosecond`, `datetime` |  | `second` | time_field 的时间单位。默认值为 second。 |
| `default_limit` | `integer` |  | `1000` | 未显式指定 limit 时生成查询的默认 LIMIT。 |
| `max_limit` | `integer` |  | `10000` | 查询规划允许生成的最大 LIMIT，用于避免生成过大的查询计划。 |
| `read_only` | `boolean` |  | `true` | 标识该存储是否只能规划只读查询。默认值为 true；UModel PaaS 查询规划不应生成写入语句。 |
| `credential_ref` | `string` |  |  | 凭据引用标识，例如 secret://mysql-prod-readonly。不得在 UModel 中保存明文用户名、密码或 Token。 |
| `tls_mode` | enum: `disabled`, `preferred`, `required`, `verify_ca`, `verify_identity` |  | `preferred` | MySQL TLS 模式。默认值为 preferred。 |
| `properties` | map&lt;string, string&gt; |  |  | MySQL 的额外非敏感配置，以键值对形式存储。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于标注该 MySQL 存储的标签。 |
