# entity_source_link

EntitySourceLink is used to define the relationship between EntitySource and EntitySet.

**Kind**: `entity_source_link`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [link](../../shared-types#link)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `src` | object | yes |  | The identifier of the source Set. The value cannot be empty. |
| `src.domain` | `string` | yes |  | The domain of the source Set. |
| `src.kind` | enum: `entity_source` | yes |  | The type of the source Set. The value cannot be empty. |
| `src.name` | `string` | yes |  | The name of the source Set. The value cannot be empty. |
| `dest` | object | yes |  | The identifier of the destination Set. The value cannot be empty. |
| `dest.domain` | `string` | yes |  | The domain of the destination Set. |
| `dest.kind` | enum: `entity_set` | yes |  | The type of the destination Set. The value cannot be empty. |
| `dest.name` | `string` | yes |  | The name of the destination Set. The value cannot be empty. |
