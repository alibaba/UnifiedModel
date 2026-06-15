# profile_set

ProfileSet is used to define Profile data sets. A Profile data set is a collection of Profile data with the same attributes, generally used to describe a class of Profile data for a certain type of observable entity,…

**Kind**: `profile_set`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

**Inherits**: [telemetry_data](../../shared-types#telemetry_data)

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `protocol` | `string` |  | `pprof` | The profiling protocol used. Specifies the format and standard for profiling data. |
