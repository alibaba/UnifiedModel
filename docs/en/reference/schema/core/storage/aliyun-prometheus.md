# aliyun_prometheus

The aliyun_prometheus is used to define the aliyun prometheus instance.

**Kind**: `aliyun_prometheus`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `region` | `string` | yes |  | The region of the Aliyun Prometheus instance. |
| `instance_id` | `string` | yes |  | The instance ID of Aliyun Prometheus. |
| `sls_project` | `string` |  |  | The SLS project name where the Aliyun Prometheus data is stored. |
| `sls_metricstore` | `string` |  |  | The SLS metricstore name where the Aliyun Prometheus data is stored. |
