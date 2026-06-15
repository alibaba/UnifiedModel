# metric_set

MetricSet 用于定义指标，指标集是具有相同属性的指标的集合，一般用于描述某类可观测实体的一类指标，例如主机的CPU、内存、磁盘等指标。

**Kind**: `metric_set`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `labels` | object |  |  | 指标集的标签列表，建议标签通过 dynamic 方式自动生成。注意：此处的标签是 MetricSet 的通用标签，一般建议 MetricSet 只定义通用标签，不在 Metric 下定义额外的标签。 |
| `labels.keys` | array&lt;[field_spec](../../shared-types#field_spec)&gt; |  |  | 标签列表，值格式参考 field 定义。 |
| `labels.dynamic` | `boolean` |  | `false` | 是否动态生成，一般建议设置为 true。 |
| `labels.filter` | `string` |  |  | 标签的过滤器，在 dynamic 为 true 时，会根据此过滤器进行过滤。 |
| `label_keys` | array&lt;[field_spec](../../shared-types#field_spec)&gt; |  |  | 指标集的标签键。当前已废弃，待删除，请使用 labels 字段代替。 |
| `metrics` | array&lt;[metric](../../shared-types#metric)&gt; |  |  | 详细的指标列表。 |
| `query_type` | enum: `prom`, `spl`, `cms` |  |  | 指标的查询语法，取值包括：prom、spl、cms 。 |
| `needs_processing` | `boolean` |  | `false` | 是否需要二次处理，即该 MetricSet 中指标是否需要二次处理才能被使用。例如 Prometheus 中的 counter/summary/histogram 等指标需要二次处理才能被使用。 |
