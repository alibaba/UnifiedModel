# explorer_link

**Kind**: `explorer_link`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [link](../../shared-types#link)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `token_mapping` | map&lt;string, string&gt; |  |  | 源 Dataset 和目标 Explorer 之间的字段映射关系。 |
| `token_replace` | map&lt;string, string&gt; |  |  | 源 Dataset 和目标 Explorer 之间的字段替换关系。 |
| `config` | map&lt;string, string&gt; |  |  | Explorer 的配置动态。 |
