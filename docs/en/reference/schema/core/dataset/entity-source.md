# entity_source

EntitySource defines the import job for a specific entity and its source storage (e.g. SLS LogStore / MetricStore).

**Kind**: `entity_source`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `constructor` | map&lt;string, any&gt; | yes |  | Constructor/scheduling configuration of the import job, supports flexible key-value pairs. |
| `storages` | array&lt;map&gt; | yes |  | List of source storage configurations, each element is a map supporting flexible fields. |
