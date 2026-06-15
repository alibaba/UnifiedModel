# external_storage

External storage, used to store details of custom storage.

**Kind**: `external_storage`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `type` | `string` | yes |  | The type of the external storage. |
| `name` | `string` | yes |  | The name of the external storage. |
| `properties` | map&lt;string, string&gt; |  |  | Details of the external storage, stored as key-value pairs. |
| `tags` | map&lt;string, string&gt; |  |  | This field is used to represent the tags of the external storage. |
