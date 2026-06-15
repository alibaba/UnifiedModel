# entity_source_link

EntitySourceLink 用于定义 EntitySource 与 EntitySet 之间的关联关系。

**Kind**: `entity_source_link`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [link](../../shared-types#link)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `src` | object | 是 |  | 源 Set 的标识。值不能为空。 |
| `src.domain` | `string` | 是 |  | 源 Set 的域。 |
| `src.kind` | enum: `entity_source` | 是 |  | 源 Set 的类型。值不能为空。 |
| `src.name` | `string` | 是 |  | 源 Set 的名称。值不能为空。 |
| `dest` | object | 是 |  | 目标 Set 的标识。值不能为空。 |
| `dest.domain` | `string` | 是 |  | 目标 Set 的域。 |
| `dest.kind` | enum: `entity_set` | 是 |  | 目标 Set 的类型。值不能为空。 |
| `dest.name` | `string` | 是 |  | 目标 Set 的名称。值不能为空。 |
