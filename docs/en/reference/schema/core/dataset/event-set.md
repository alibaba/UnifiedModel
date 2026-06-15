# event_set

EventSet is a collection of related events that share certain common attributes or characteristics.

**Kind**: `event_set`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [telemetry_data](../../shared-types#telemetry_data)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `entity_fields` | object |  |  | Entity fields, used to identify the ID, domain and type of entity |
| `entity_fields.entity_id` | `string` |  | `__entity_id__` | Entity ID field, used to identify the ID of entity |
| `entity_fields.domain` | `string` |  | `__domain__` | Domain field which entity belongs to |
| `entity_fields.entity_type` | `string` |  | `__entity_type__` | Entity type field, used to identify the type of entity |
