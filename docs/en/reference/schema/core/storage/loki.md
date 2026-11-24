# loki

Loki storage for LogQL query plans for logs and log-derived metrics.

**Kind**: `loki`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `endpoint` | `string` | yes |  | HTTP or HTTPS URL of the Loki service. |
| `api_prefix` | `string` |  | `/loki/api/v1` | Loki HTTP API prefix; defaults to /loki/api/v1. |
| `default_query_type` | enum: `instant`, `range` |  | `instant` | Default mode for metric queries. |
| `default_step` | `string` |  |  | Default step for range metric queries, for example 30s. |
| `search_filter` | `string` |  |  | Optional static filter expressed with Loki label names. |
| `default_size` | `integer` |  | `100` | Default log entry limit when none is specified; defaults to 100. |
| `max_size` | `integer` |  | `1000` | Maximum log entries per query; defaults to 1000. |
| `tenant` | `string` |  |  | Optional tenant ID sent by the client. |
| `tenant_header` | `string` |  | `X-Scope-OrgID` | Tenant HTTP header name; defaults to X-Scope-OrgID. |
| `auth` | object |  |  | Authentication type and environment variable names; never credential values. |
| `auth.type` | enum: `bearer`, `basic` | yes |  |  |
| `auth.token_env` | `string` |  |  |  |
| `auth.username_env` | `string` |  |  |  |
| `auth.password_env` | `string` |  |  |  |
| `tls_verify` | `boolean` |  | `true` | Whether to verify TLS certificates; defaults to true. |
| `headers` | map&lt;string, string&gt; |  |  | Additional HTTP headers without authentication credentials. |
