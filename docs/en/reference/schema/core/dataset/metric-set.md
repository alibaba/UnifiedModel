# metric_set

MetricSet is used to define metrics. A metric set is a collection of metrics with the same attributes, generally used to describe a class of metrics for a certain type of observable entity, such as CPU, memory, disk a…

**Kind**: `metric_set`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `labels` | object |  |  | The list of labels for the MetricSet. It is recommended to use dynamic to automatically generate labels. Note: The labels here are the general labels of the MetricSet. It is generally recommended that the MetricSet on… |
| `labels.keys` | array&lt;[field_spec](../../shared-types#field_spec)&gt; |  |  | The list of labels. The value format is reference to the field definition. |
| `labels.dynamic` | `boolean` |  | `false` | Whether the key is dynamic. It is generally recommended to set it to true. |
| `labels.filter` | `string` |  |  | The filter of the label. When dynamic is true, it will be filtered according to this filter. |
| `label_keys` | array&lt;[field_spec](../../shared-types#field_spec)&gt; |  |  | The label keys of the MetricSet. It is currently deprecated and will be deleted. Please use the labels field instead. |
| `metrics` | array&lt;[metric](../../shared-types#metric)&gt; |  |  | The detailed list of metrics. |
| `query_type` | enum: `prom`, `spl`, `cms` |  |  | The query type of the metric. The value can be prom, spl or cms. |
| `needs_processing` | `boolean` |  | `false` | Whether the MetricSet needs to be processed again. That is, whether the metrics in the MetricSet need to be processed again to be used. For example, the counter/summary/histogram metrics in Prometheus need to be proce… |
