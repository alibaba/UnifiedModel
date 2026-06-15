# storage_link

StorageLink is used to define the relationship between EntitySet/DataSet and Storage. StorageLink must contain the source Set, destination Storage.

**Kind**: `storage_link`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [link](../../shared-types#link)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `fields_mapping` | map&lt;string, string&gt; |  |  | Used to define simple field mapping relationships, such as the mapping relationship between the name of Field in DataSet and the name of Field in Storage. |
