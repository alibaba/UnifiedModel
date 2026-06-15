# storage_link

StorageLink 用于定义 EntitySet/DataSet 和 Storage 之间的关系。StorageLink 必须包含源 Set 、目标 Storage。

**Kind**: `storage_link`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [link](../../shared-types#link)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `fields_mapping` | map&lt;string, string&gt; |  |  | 用于定义简单的字段映射关系，例如 DataSet 中 Field 的名称与 Storage 中字段名称的映射关系。 |
