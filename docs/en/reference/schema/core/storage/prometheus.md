# prometheus

Prometheus storage, used to define open-source Prometheus or Prometheus-compatible endpoint metadata for query planning.

**Kind**: `prometheus`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `endpoint` | `string` | yes |  | The endpoint URL of Prometheus or a Prometheus-compatible service, for example http://prometheus:9090. |
| `api_prefix` | `string` |  | `/api/v1` | Prometheus HTTP API prefix. Defaults to /api/v1. |
| `default_query_type` | enum: `instant`, `range` |  | `instant` | Default PromQL query type. instant means instant query and range means range query. |
| `default_step` | `string` |  |  | Default step for range queries, for example 60s or 1m. |
| `lookback_delta` | `string` |  |  | Default PromQL lookback window used by query planning, for example 5m. |
| `tenant` | `string` |  |  | Optional tenant identifier for multi-tenant Prometheus-compatible systems. |
| `tenant_header` | `string` |  |  | Tenant HTTP header name used by multi-tenant systems, for example X-Scope-OrgID. |
| `credential_ref` | `string` |  |  | Credential reference, for example secret://prometheus-prod-readonly. Plaintext usernames, passwords, or tokens must not be stored in UModel. |
| `tls_verify` | `boolean` |  | `true` | Whether TLS certificates should be verified. Defaults to true. |
| `external_labels` | map&lt;string, string&gt; |  |  | Prometheus external labels used to enrich query-planning context or describe result origin. |
| `headers` | map&lt;string, string&gt; |  |  | Non-sensitive HTTP headers for tenant or routing context. Authentication secrets must not be stored here. |
| `properties` | map&lt;string, string&gt; |  |  | Additional non-sensitive Prometheus configuration, stored as key-value pairs. |
| `tags` | map&lt;string, string&gt; |  |  | Tags used to describe this Prometheus storage. |
