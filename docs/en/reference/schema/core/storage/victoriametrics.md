# victoriametrics

VictoriaMetrics storage, used to define VictoriaMetrics or other PromQL-compatible time-series backend metadata for query planning. It reuses the label-timeseries query family via spec.family, requiring no UModel code.

**Kind**: `victoriametrics`

> Every element shares the standard envelope `kind` · [metadata](../../shared-types#metadata) · [schema](../../shared-types#schema).

## `spec` fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `family` | `string` |  |  | The query family this storage renders through. VictoriaMetrics is PromQL-compatible, so setting label-timeseries reuses the Prometheus query-plan renderer with no Go code — exactly how a new same-family backend is add… |
| `endpoint` | `string` | yes |  | The endpoint URL of VictoriaMetrics or a PromQL-compatible service, for example http://victoriametrics:8428. |
| `api_prefix` | `string` |  | `/api/v1` | PromQL HTTP API prefix. Defaults to /api/v1. |
| `default_query_type` | enum: `instant`, `range` |  | `instant` | Default PromQL query type. instant means instant query and range means range query. |
| `default_step` | `string` |  |  | Default step for range queries, for example 60s or 1m. |
| `lookback_delta` | `string` |  |  | Default PromQL lookback window used by query planning, for example 5m. |
| `tenant` | `string` |  |  | Optional tenant identifier for multi-tenant VictoriaMetrics cluster deployments. |
| `tenant_header` | `string` |  |  | Tenant HTTP header name used by multi-tenant systems, for example X-Scope-OrgID. |
| `credential_ref` | `string` |  |  | Credential reference, for example secret://victoriametrics-prod-readonly. Plaintext usernames, passwords, or tokens must not be stored in UModel. |
| `tls_verify` | `boolean` |  | `true` | Whether TLS certificates should be verified. Defaults to true. |
| `external_labels` | map&lt;string, string&gt; |  |  | External labels used to enrich query-planning context or describe result origin. |
| `headers` | map&lt;string, string&gt; |  |  | Non-sensitive HTTP headers for tenant or routing context. Authentication secrets must not be stored here. |
| `properties` | map&lt;string, string&gt; |  |  | Additional non-sensitive VictoriaMetrics configuration, stored as key-value pairs. |
| `tags` | map&lt;string, string&gt; |  |  | Tags used to describe this VictoriaMetrics storage. |
