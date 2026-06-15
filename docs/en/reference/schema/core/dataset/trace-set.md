# trace_set

TraceSet is a collection of related trace records that share certain common attributes or characteristics. Each TraceSet must include the following fields: trace_id_field, span_id_field, parent_span_id_field, and prot…

**Kind**: `trace_set`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [telemetry_data](../../shared-types#telemetry_data)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `attribute_fields` | array&lt;[field_spec](../../shared-types#field_spec)&gt; |  |  | The attributes of the TraceSet. The value format is a list of field specifications that define the trace attributes. It is currently not used. |
| `trace_id_field` | `string` | yes | `traceId` | The field name of trace_id of the TraceSet. |
| `span_id_field` | `string` | yes | `spanId` | The field name of span_id of the TraceSet. |
| `parent_span_id_field` | `string` |  | `parentSpanId` | The field name of parent_span_id of the TraceSet. |
| `protocol` | `string` |  | `opentelemetry` | The tracing protocol used. Specifies the format and standard for trace data. |
