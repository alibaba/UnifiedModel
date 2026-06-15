# sls_logstore

SLS LogStore storage, usually used to store log data.

**Kind**: `sls_logstore`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `region` | `string` | yes |  | The region of the SLS LogStore. |
| `project` | `string` | yes |  | The project of the SLS LogStore. |
| `store` | `string` | yes |  | The name of the SLS LogStore. |
| `search_filter` | `string` |  | `*` | The filter condition of the logstore, if the logstore contains multiple types of data, you can use this field to filter the target data. |
| `spl_filter` | `string` |  |  | The SPL filter condition of the logstore, compared to the search_filter field, SPL filter conditions are more flexible and support more complex filter conditions, but the performance is relatively poor. |
| `spl_view` | `string` |  |  | The SPL view of the logstore, usually used to dynamically generate entity lists, connections, etc. Note: This scenario requires re-generating the view each time, and the performance is very poor. |
| `spl_notebook` | `string` |  |  | The result calculated by the SPL Notebook, compared to the spl_view field, it is relatively independent, can be executed directly and can join multiple data sources. Note: This scenario requires re-calculating and gen… |
| `sql_filter` | `string` |  |  | The SQL filter condition of the logstore, compared to the search_filter field, SQL filter conditions are more flexible and support more complex filter conditions, but the performance is relatively poor. |
| `sql_view` | `string` |  |  | The SQL view of the logstore, usually used to dynamically generate entity lists, connections, etc. Note: This scenario requires re-generating the view each time, and the performance is very poor. |
