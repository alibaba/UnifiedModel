# data_link

DataLink is used to define the relationship between EntitySet/Link and DataSet, and also describe the relationship between two DataSet. DataLink must contain the source DataSet, destination DataSet and link type.

**Kind**: `data_link`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [link](../../shared-types#link)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `fields_mapping` | map&lt;string, string&gt; |  |  | The mapping relationship between the fields of the source DataSet and the destination DataSet. |
| `data_link_type` | enum: `produce`, `related_to` |  |  | The type of the DataLink. The value cannot be empty. The value must be one of the following: produce, related_to. |
| `data_filter` | `string` |  |  | The data filter, used to filter data. For example, there is a general MetricSet that contains various types of metrics, and the source entity may be DBClient, then the metrics of all db types can be filtered out throu… |
