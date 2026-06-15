# sls_entitystore

SLS EntityStore 存储，一般用于存储可观测实体以及关系数据。

**Kind**: `sls_entitystore`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `region` | `string` | 是 |  | SLS EntityStore 的区域。 |
| `project` | `string` | 是 |  | SLS EntityStore 的项目。 |
| `workspace` | `string` | 是 |  | SLS EntityStore 的工作空间。 |
