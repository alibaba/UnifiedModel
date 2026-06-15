# trace_set

TraceSet 是具有某些共同属性或特征的相关追踪记录的集合。TraceSet 必须包含以下字段：trace_id_field、span_id_field、parent_span_id_field 和 protocol（默认值为opentelemetry）。

**Kind**: `trace_set`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**继承**: [telemetry_data](../../shared-types#telemetry_data)

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `attribute_fields` | array&lt;[field_spec](../../shared-types#field_spec)&gt; |  |  | TraceSet 的属性列表，值格式为定义追踪属性的字段规格列表。当前未使用。 |
| `trace_id_field` | `string` | 是 | `traceId` | TraceSet 的 trace_id 字段名称。 |
| `span_id_field` | `string` | 是 | `spanId` | TraceSet 的 span_id 字段名称。 |
| `parent_span_id_field` | `string` |  | `parentSpanId` | TraceSet 的 parent_span_id 字段名称。 |
| `protocol` | `string` |  | `opentelemetry` | 使用的追踪协议。指定追踪数据的格式和标准。 |
