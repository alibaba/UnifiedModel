# sls_entitystore

SLS EntityStore storage, usually used to store observable entities and relationship data.

**Kind**: `sls_entitystore`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `region` | `string` | yes |  | The region of the SLS EntityStore. |
| `project` | `string` | yes |  | The project of the SLS EntityStore. |
| `workspace` | `string` | yes |  | The workspace of the SLS EntityStore. |
