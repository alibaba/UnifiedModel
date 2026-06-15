# runbook_link

RunbookLink 用于定义实体集合与操作手册集合之间的关联关系。

**Kind**: `runbook_link`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [link](../../shared-types#link)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `token_replace` | map&lt;string, string&gt; |  |  | 上下文变量的映射关系，用于在执行 Runbook 时提供动态上下文变量。 |
| `fields_mapping` | map&lt;string, string&gt; |  |  | 源数据集字段到目标 Runbook 字段的映射关系，用于将数据集中的字段值传递给 Runbook 的输入参数。 |
