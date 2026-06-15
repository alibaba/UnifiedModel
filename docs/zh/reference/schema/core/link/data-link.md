# data_link

DataLink 用于定义 EntitySet/Link 和 DataSet 之间的关系，同时也可以描述两个 DataSet 之间的关系。DataLink 必须包含源 DataSet 、目标 DataSet 和链接类型。

**Kind**: `data_link`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [link](../../shared-types#link)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `fields_mapping` | map&lt;string, string&gt; |  |  | 源 DataSet/EntitySet 和目标 DataSet 字段之间的映射关系。 |
| `data_link_type` | enum: `produce`, `related_to` |  |  | DataLink 的类型。值不能为空。值必须是以下之一：produce（产生）、related_to（有关联，即弱连接）。 |
| `data_filter` | `string` |  |  | 关联目标数据的过滤器，即目标 DataSet 中只有部分数据和源数据有关联。例如存在一个通用的MetricSet，包含各种调用类型的指标，而源实体可能是DBClient，则可通过 data_fiter = "type='db'" 过滤出所有 db 类型的指标。注意：data_filter 需要使用query类型的filter语法。此方式过于复杂，不建议使用。 |
