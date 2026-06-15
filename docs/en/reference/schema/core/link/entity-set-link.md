# entity_set_link

EntitySetLink is used to define the relationship between two EntitySet. EntitySetLink must contain the source EntitySet, destination EntitySet and link type.

**Kind**: `entity_set_link`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [link](../../shared-types#link)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `fields_mapping` | map&lt;string, string&gt; |  |  | The destination EntitySet fields mapping of the EntitySetLink. |
| `constructor` | object |  |  | The constructor configuration for EntitySetLink. If this field is configured, the background will automatically create and maintain tasks to automatically generate EntitySetLink. |
| `entity_link_type` | `string` | yes |  | The type of the EntityLinkSet. The value cannot be empty. The recommended values are: - calls - runs - instance_of - parent_of - contains - balances - can_access - clustered_by - manages - monitors - sends_to - affect… |
| `dynamic` | `bool` |  | `false` | Whether the EntitySetLink is dynamically generated, default is false. If true, the EntitySetLink is a dynamic Link, which will be bound to a Storage, and the content of the Link will be obtained from the Storage, rath… |
