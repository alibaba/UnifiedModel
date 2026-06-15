# event_set

EventSet 用于定义事件，事件集是具有相同属性的事件的集合。

**Kind**: `event_set`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [telemetry_data](../../shared-types#telemetry_data)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `entity_fields` | object |  |  | 实体字段，用于标识实体的 ID、域和类型 |
| `entity_fields.entity_id` | `string` |  | `__entity_id__` | 实体 ID 字段 |
| `entity_fields.domain` | `string` |  | `__domain__` | 实体所属的域字段 |
| `entity_fields.entity_type` | `string` |  | `__entity_type__` | 实体类型字段，用于标识实体的类型 |
