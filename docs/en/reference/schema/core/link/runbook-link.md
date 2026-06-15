# runbook_link

RunbookLink is used to define the relationship between entity sets and runbook sets.

**Kind**: `runbook_link`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [link](../../shared-types#link)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `token_replace` | map&lt;string, string&gt; |  |  | The mapping relationship of context variables, used to provide dynamic context variables when executing the Runbook. |
| `fields_mapping` | map&lt;string, string&gt; |  |  | The field mapping from source dataset fields to destination Runbook fields, used to pass field values from the dataset to Runbook input parameters. |
