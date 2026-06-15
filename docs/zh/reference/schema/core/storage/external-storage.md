# external_storage

外部存储，用于存放自定义存储的详细信息。

**Kind**: `external_storage`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `type` | `string` | 是 |  | 外部存储的类型。 |
| `name` | `string` | 是 |  | 外部存储的名称。 |
| `properties` | map&lt;string, string&gt; |  |  | 外部存储的详细信息，以键值对形式存储。 |
| `tags` | map&lt;string, string&gt; |  |  | 用于表示该外部存储的标签。 |
