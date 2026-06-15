# explorer_link

**Kind**: `explorer_link`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [link](../../shared-types#link)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `token_mapping` | map&lt;string, string&gt; |  |  | The mapping relationship between the fields of the source dataset set and the destination explorer |
| `token_replace` | map&lt;string, string&gt; |  |  | The replace relationship between the fields of the source dataset set and the destination explorer |
| `config` | map&lt;string, string&gt; |  |  | The config dynamic of explorer. |
