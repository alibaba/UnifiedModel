# `get_metrics` / `get_logs` — read the plan, then run it

`get_metrics` / `get_logs` return an executable **plan**, not values (open source = plan
provider). Read the plan, then run its query against the backend with whatever tooling you
have. These are `.entity_set` entity-call methods — see [entity-set.md](entity-set.md) for the
call mechanics and the wrapped result shape.

## 1. Fetch the plan

Two lookups give you the call arguments — **get both from the entity, not from a global model
scan**:

- **Entity id** — `.entity … query='payment-gateway' | project __entity_id__` (see
  [entity.md](entity.md)).
- **Dataset name** (the metric_set / log_set this entity exposes) — call
  `entity-call list_data_set(['metric_set'])` (or `['log_set']`) on the entity-set (see
  [entity-set.md](entity-set.md)); the returned row gives the `domain`+`name` to pass below. Do
  **not** reach for `.umodel with(kind='metric_set')` — that lists every dataset in the
  workspace, unscoped to your entity, so it can't tell you which one applies (and `| project
  name` on `.umodel` comes back null).

Then call the method:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call get_metrics('platform','platform.service.metrics','latency_p99_ms', step='30s')" -o json
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call get_logs('platform','platform.service.logs', query='level = \"ERROR\"')" -o json
```

## 2. The returned plan — format

The call returns a wrapped row; the plan is the JSON string at `data.data[0][1]` (see
[entity-set.md](entity-set.md)). Parsed, it is a `v1` envelope — the same shape for both
operations, with an operation-specific `query` block:

```jsonc
{
  "mode": "plan", "version": "v1", "operation": "get_metrics",
  "description": "<human summary>",
  "params_echo": { "domain": "...", "name": "...", "metric": "...", "step": "30s" },
  "source_query": "<the SPL you ran>",
  "data_source": {                 // model provenance + where the data lives
    "data_set": { "kind": "metric_set", "name": "platform.service.metrics", ... },
    "data_link": { ... }, "storage_link": { ... },
    "storage": {
      "type": "prometheus",        // or "elasticsearch"
      "config": { "endpoint": "http://prometheus.platform.example:9090",
                  "api_prefix": "/api/v1", "tenant_header": "X-Scope-OrgID", ... }
    }
  },
  "query": { "dialect": "...", ... } // ← the executable query; dispatch on dialect
}
```

Dispatch on **`query.dialect`**:

### `prometheus_promql` (real `query` block, trimmed)

```jsonc
"query": {
  "dialect": "prometheus_promql",
  "endpoint": "http://prometheus.platform.example:9090",
  "api_prefix": "/api/v1",
  "query_type": "range",            // or "instant"
  "step": "30s",
  "queries": [
    { "name": "latency_p99_ms", "unit": "ms",
      "promql": "histogram_quantile(0.99, sum(rate(platform_service_request_duration_seconds_bucket{service_id=\"63718b78868895d2590551b27ec6f51c\"}[1m])) by (le)) * 1000" }
  ],
  "label_matchers": [ { "label": "service_id", "operator": "=", "value": "63718b78868895d2590551b27ec6f51c" } ],
  "tenant": "incident-investigation", "tenant_header": "X-Scope-OrgID", "limit": 100
}
```

Run each `queries[].promql` against `endpoint` + `api_prefix`; send the header
`<tenant_header>: <tenant>` if present.

### `elasticsearch_dsl` (real `query` block, trimmed)

```jsonc
"query": {
  "dialect": "elasticsearch_dsl",
  "index": "platform-service-logs-*",
  "body": {
    "size": 100,
    "query": { "bool": { "filter": [
      { "term": { "svc_id": "63718b78868895d2590551b27ec6f51c" } },
      { "term": { "severity": "ERROR" } }
    ] } },
    "sort": [ { "timestamp": { "order": "desc" } } ],
    "_source": [ "timestamp", "svc_id", "env", "severity", "log_message", "trace_id" ]
  }
}
```

The ES `query` block has **no `endpoint`** — take it from
`data_source.storage.config.endpoint` (here `https://elasticsearch.platform.example:9200`).
POST `body` to `<endpoint>/<index>/_search`.

## 3. Execute it with what you have

**You are the executor.** The plan describes the query and where it lives; running it is your
job — don't stop at the plan. (If a plan's `description` or `next_action` suggests forwarding it
to a separate data executor, that's the PaaS path; in this skill you execute it yourself.)

The query is ready to run — you decide how: if a Prometheus / Elasticsearch CLI is available
in your environment, call it; otherwise hit the HTTP API directly, or use any client you have.
The backend's own response format is what you parse next: Prometheus →
`.data.result[]` (each `{metric, value:[ts,"v"]}` for instant, or `values:[[ts,"v"],…]` for
range); Elasticsearch → `.hits.hits[]._source` (one object per log line).

## 4. Adapt the parameters

- **Endpoint:** the plan's `endpoint` / `index` come from the model's storage config and may
  be placeholders (e.g. `prometheus.platform.example:9090`) — point at your real backend.
- **Time window:** the plan carries `step` but **no range** — an instant query uses "now"; a
  range query needs a start/end you choose. Tune `size` for logs.
- **Auth:** pass credentials (or the `tenant_header`) via env — never hardcode or echo them.
- **Read-only:** run only the plan's query; don't invent mutating calls.
- **Field/label names:** the matcher value is the entity's mapped id (the demo maps it to a
  hash, `service_id="63718b78…"`), and log rows return **mapped** names (`svc_id`, `severity`,
  …) — your backend's labels/fields must match, or adjust them.

## Worked example — payment-gateway P99 latency

1. **Fetch:** `… | entity-call get_metrics('platform','platform.service.metrics','latency_p99_ms', step='30s')` → the `prometheus_promql` plan above.
2. **Read:** `query.queries[0].promql` (ready PromQL), `query.endpoint` (placeholder), `query.tenant`/`tenant_header`.
3. **Override the endpoint and run** (here via the HTTP API + jq; a Prometheus CLI works too):

   ```bash
   PROMQL='histogram_quantile(0.99, sum(rate(platform_service_request_duration_seconds_bucket{service_id="63718b78868895d2590551b27ec6f51c"}[1m])) by (le)) * 1000'
   curl -sG http://YOUR-PROMETHEUS:9090/api/v1/query \
     --data-urlencode "query=$PROMQL" \
     -H "X-Scope-OrgID: incident-investigation" | jq '.data.result'
   ```
4. **Parse:** `.data.result[].value[1]` is the P99 value (e.g. a `~2150` ms spike on your data).

Logs are analogous: take `query.body` + `query.index` + the storage `endpoint`, then
`curl -s https://YOUR-ES:9200/platform-service-logs-*/_search -H 'Content-Type: application/json' -d '<body>' | jq '.hits.hits[]._source'`.

> **PaaS shortcut:** against a PaaS endpoint with `mode='data'`, `get_metrics` / `get_logs`
> return the **rows directly** — no execution step. Same SPL.
