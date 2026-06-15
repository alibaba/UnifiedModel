# runbook_set

RunbookSet is used to define a collection of operational runbooks for entities and data, including analysis tools, operation guides and best practices.

**Kind**: `runbook_set`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `observations` | array&lt;[observation](../../shared-types#observation)&gt; |  |  | List of observation configurations |
| `actions` | array&lt;object&gt; |  |  | List of action configurations |
| `toolkits` | array&lt;object&gt; |  |  | List of toolkit configurations, each toolkit contains shared config and a set of tools |
| `knowledge` | array&lt;object&gt; |  |  | List of knowledge base configurations |
| `automations` | array&lt;object&gt; |  |  | List of automation configurations |
| `skills` | array&lt;object&gt; |  |  | List of skill configurations following Agent Skills specification |
