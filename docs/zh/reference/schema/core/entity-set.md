# entity_set

EntitySet 用于定义实体，实体集是具有相同属性的实体的集合，类似于数据库中的表、编程中的类概念。在建模场景中，可根据需要定义关心的实体，例如IT可观测场景中，需要定义主机、容器、进程、应用、Code Repo、运维人员等。

**Kind**: `entity_set`

> 每个元素共享标准信封 `kind` · [metadata](../shared-types#metadata) · [schema](../shared-types#schema).

**继承**: [telemetry_data](../shared-types#telemetry_data)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `first_observed_time_field` | `string` |  |  | Entity 的首次被观测时间所在字段。 |
| `last_observed_time_field` | `string` |  |  | Entity 最近被观测时间所在字段。 |
| `primary_key_fields` | array&lt;string&gt; |  |  | Entity 的主键字段。值格式为唯一标识实体的字段名称列表。 |
| `id_generator` | `string` |  |  | Entity 的 ID 生成器。表示该 EntitySet 的ID如何通过PrimaryKeyFields 生成。该字段类型为 string，需要符合 SPL 的表达式语法，执行返回为 string。如果为空，则使用默认的生成方式： lower(to_hex(md5(cast(join(primaryKeys, '#$#') as varbinary)))) ，即将 PrimaryKeyFields 拼接成字符串，然后进行 MD5… |
| `keep_alive_seconds` | `number` |  | `3600` | 保持活跃的持续时间，在最后观测时间加上保活秒数后，Entity 将被视为消失。默认为3600秒（1小时）。 |
| `dynamic` | `bool` |  | `false` | 是否为动态生成，默认为false。为true时表示该实体为动态实体，会绑定 Storage，每次实体的内容由Storage 处获取，而非存储在 EntityStore 中。 |
