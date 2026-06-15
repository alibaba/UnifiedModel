# sls_metricstore

SLS MetricStore storage, usually used to store metric data.

**Kind**: `sls_metricstore`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `region` | `string` | yes |  | The region of the SLS MetricStore. |
| `project` | `string` | yes |  | The project of the SLS MetricStore. |
| `store` | `string` | yes |  | The name of the SLS MetricStore. |
