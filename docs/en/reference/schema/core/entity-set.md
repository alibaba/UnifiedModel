# entity_set

EntitySet is used to define the entity, which is a collection of entities that share the same properties. In the modeling scenario, entities can be defined according to needs, such as in the IT observable scenario, it…

**Kind**: `entity_set`

> Every element shares the standard envelope `kind` · [metadata](../shared-types#metadata) · [schema](../shared-types#schema).

**Inherits**: [telemetry_data](../shared-types#telemetry_data)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `first_observed_time_field` | `string` |  |  | The field containing the first observed time of the Entity. |
| `last_observed_time_field` | `string` |  |  | The field containing the last observed time of the Entity. |
| `primary_key_fields` | array&lt;string&gt; |  |  | The primary key fields of the Entity. The value format is a list of field names that uniquely identify an entity. |
| `id_generator` | `string` |  |  | Entity 的 ID 生成器。表示该 EntitySet 的ID如何通过PrimaryKeyFields 生成。该字段类型为 string，需要符合 SPL 的表达式语法，执行返回为 string。如果为空，则使用默认的生成方式： lower(to_hex(md5(cast(join(primaryKeys, '#$#') as varbinary)))) ，即将 PrimaryKeyFields 拼接成字符串，然后进行 MD5… |
| `keep_alive_seconds` | `number` |  | `3600` | The duration to keep active is, after the last observed time plus the keep-alive seconds, the Entity will be considered as disappeared. Default is 3600 seconds (1 hour). |
| `dynamic` | `bool` |  | `false` | Whether the Entity is dynamically generated, default is false. If true, the Entity is a dynamic entity, which will be bound to a Storage, and the content of the Entity will be obtained from the Storage, rather than be… |
