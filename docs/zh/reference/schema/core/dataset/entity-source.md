# entity_source

EntitySource 用于定义特定 Entity 数据的导入任务以及其源数据存储（如 SLS LogStore / MetricStore）。

**Kind**: `entity_source`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `constructor` | map&lt;string, any&gt; | 是 |  | 导入任务的构造/调度配置，支持灵活扩展键值对。 |
| `storages` | array&lt;map&gt; | 是 |  | 源数据存储配置列表，每个元素为一个 map，支持灵活扩展字段。 |
