# sls_logstore

SLS LogStore 存储，一般用于存储日志数据。

**Kind**: `sls_logstore`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `region` | `string` | 是 |  | SLS LogStore 的区域。 |
| `project` | `string` | 是 |  | SLS LogStore 的项目。 |
| `store` | `string` | 是 |  | SLS LogStore 的名称。 |
| `search_filter` | `string` |  | `*` | 日志库的过滤条件，如果日志库中存储了多种类型的数据，可使用此字段过滤目标数据。 |
| `spl_filter` | `string` |  |  | 日志库的 SPL 过滤条件，相比 search_filter 字段，SPL 过滤条件更加灵活，支持更复杂的过滤条件，但性能相对较差。 |
| `spl_view` | `string` |  |  | 基于 SPL 生成的视图，一般用于动态生成实体列表、连接等场景。注意：此场景每次需要重新生成视图，性能较差。 |
| `spl_notebook` | `string` |  |  | 基于 SPL Notebook 计算出的结果，相比 spl_view 字段，相对更加独立，可直接执行且能 Join 多种数据源。注意：此场景每次需要重新计算生成，性能较差。 |
| `sql_filter` | `string` |  |  | 日志库的 SQL 过滤条件，相比 search_filter 字段，SQL 过滤条件更加灵活，支持更复杂的过滤条件，但性能相对较差。 |
| `sql_view` | `string` |  |  | 基于 SQL 生成的视图，一般用于动态生成实体列表、连接等场景。注意：此场景每次需要重新生成视图，性能较差。 |
