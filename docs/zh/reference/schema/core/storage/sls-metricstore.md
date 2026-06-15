# sls_metricstore

SLS MetricStore 存储，一般用于存储指标数据。

**Kind**: `sls_metricstore`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `region` | `string` | 是 |  | SLS MetricStore 的区域。 |
| `project` | `string` | 是 |  | SLS MetricStore 的项目。 |
| `store` | `string` | 是 |  | SLS MetricStore 的名称。 |
