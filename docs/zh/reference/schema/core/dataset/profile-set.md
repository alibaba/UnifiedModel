# profile_set

ProfileSet 用于定义 Profile 数据集，Profile 数据集是具有相同属性的 Profile 数据的集合，一般用于描述某类可观测实体的一类 Profile 数据，例如主机的 CPU、内存、磁盘等 Profile 数据。

**Kind**: `profile_set`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [telemetry_data](../../shared-types#telemetry_data)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `protocol` | `string` |  | `pprof` | 使用的 Profile 协议。指定 Profile 数据的格式和标准。 |
