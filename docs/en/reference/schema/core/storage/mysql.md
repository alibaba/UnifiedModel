# mysql

MySQL storage, used to define the MySQL instance, database, table, and connection metadata required for query planning.

**Kind**: `mysql`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `endpoint` | `string` | yes |  | The MySQL instance endpoint, usually in host:port format. |
| `database` | `string` | yes |  | The default database name to query. |
| `table` | `string` |  |  | The default table name to query. It can be overridden by the DataSet or query parameters. |
| `sql_template` | `string` |  |  | Optional SQL template for special query planning scenarios. The template must keep read-only semantics and should not contain INSERT, UPDATE, DELETE, DDL, or similar statements. |
| `sql_dialect` | enum: `mysql`, `ansi` |  | `mysql` | SQL dialect. MySQL storage uses mysql by default. |
| `time_field` | `string` |  |  | Time field used to push request time ranges into SQL WHERE conditions. |
| `time_unit` | enum: `second`, `millisecond`, `microsecond`, `nanosecond`, `datetime` |  | `second` | Time unit of time_field. Defaults to second. |
| `default_limit` | `integer` |  | `1000` | Default SQL LIMIT when no explicit limit is provided. |
| `max_limit` | `integer` |  | `10000` | Maximum SQL LIMIT allowed by the planner to avoid producing excessively large query plans. |
| `read_only` | `boolean` |  | `true` | Indicates whether this storage should only plan read-only queries. Defaults to true; UModel PaaS query planning should not generate write statements. |
| `credential_ref` | `string` |  |  | Credential reference, for example secret://mysql-prod-readonly. Plaintext usernames, passwords, or tokens must not be stored in UModel. |
| `tls_mode` | enum: `disabled`, `preferred`, `required`, `verify_ca`, `verify_identity` |  | `preferred` | MySQL TLS mode. Defaults to preferred. |
| `properties` | map&lt;string, string&gt; |  |  | Additional non-sensitive MySQL configuration, stored as key-value pairs. |
| `tags` | map&lt;string, string&gt; |  |  | Tags used to describe this MySQL storage. |
