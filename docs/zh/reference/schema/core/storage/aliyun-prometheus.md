# aliyun_prometheus

阿里云 Prometheus 实例的定义，用于描述阿里云 Prometheus 实例的配置和连接信息。

**Kind**: `aliyun_prometheus`

> 每个元素共享标准信封 `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` 字段

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `region` | `string` | 是 |  | 阿里云 Prometheus 实例所属区域。 |
| `instance_id` | `string` | 是 |  | 阿里云 Prometheus 的实例ID。 |
| `sls_project` | `string` |  |  | 阿里云 Prometheus 数据存储所在的 SLS 项目名称。 |
| `sls_metricstore` | `string` |  |  | 阿里云 Prometheus 数据存储所在的 SLS 时序库名称。 |
