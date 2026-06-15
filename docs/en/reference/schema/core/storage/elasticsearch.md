# elasticsearch

Elasticsearch storage, used to define the Elasticsearch cluster, index, and connection metadata required for query planning.

**Kind**: `elasticsearch`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `endpoint` | `string` | yes |  | The Elasticsearch cluster endpoint, for example https://es.example.com:9200. |
| `index` | `string` | yes |  | The default index name to query. It may also be an Elasticsearch-supported index wildcard expression. |
| `index_pattern` | `string` |  |  | Index pattern for time-rolled indices, for example logs-${yyyy.MM.dd}. When empty, query planning uses the index field. |
| `version` | `string` |  |  | The Elasticsearch version or version family, for example 7.x or 8.x. |
| `query_dialect` | enum: `elasticsearch_dsl`, `lucene`, `eql` |  | `elasticsearch_dsl` | Query generation dialect. Elasticsearch Query DSL is used by default. |
| `time_field` | `string` |  |  | Time field used to push request time ranges into Elasticsearch queries. |
| `default_size` | `integer` |  | `1000` | Default query size when no explicit limit is provided. |
| `max_size` | `integer` |  | `10000` | Maximum query size allowed by the planner to avoid producing excessively large query plans. |
| `routing` | `string` |  |  | Optional Elasticsearch routing value for queries that should target a fixed routing key. |
| `credential_ref` | `string` |  |  | Credential reference, for example secret://es-prod-readonly. Plaintext usernames, passwords, or tokens must not be stored in UModel. |
| `tls_verify` | `boolean` |  | `true` | Whether TLS certificates should be verified. Defaults to true. |
| `headers` | map&lt;string, string&gt; |  |  | Non-sensitive HTTP headers for tenant or routing context. Authentication secrets must not be stored here. |
| `properties` | map&lt;string, string&gt; |  |  | Additional non-sensitive Elasticsearch configuration, stored as key-value pairs. |
| `tags` | map&lt;string, string&gt; |  |  | Tags used to describe this Elasticsearch storage. |
