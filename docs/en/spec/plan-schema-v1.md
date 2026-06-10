# Plan Schema v1

中文：[Plan Schema v1](../../zh/spec/plan-schema-v1.md)

> Specification status: **v1**, normative.
> Scope: unified-model (open source) and umodel-assistant (commercial).
> Owner: UModel maintainers. Breaking changes require a new major version.

This document defines the shared contract between the **plan** mode returned by `unified-model` and the **plan / data** modes supported by `umodel-assistant`. Both projects must conform; either side breaking this contract is treated as a P0 regression.

## Why this exists

UModel runs as two coordinated surfaces:

- **unified-model** (open source) is a *plan provider*. It accepts an SPL query, resolves the EntitySet, DataLink, Storage, and StorageLink involved, and returns a **query plan** — a serialized description of what a downstream executor would need to run.
- **umodel-assistant** (commercial PaaS) is a *plan executor*. It additionally takes a plan and runs it against real storage (SLS, Prometheus, Elasticsearch, etc.), returning time series, log rows, and other concrete data.

For a user to migrate from open source to PaaS without touching their SPL or client code, both surfaces must share:

1. The same method signatures (parameters a parser accepts).
2. The same plan JSON shape (what an executor consumes).
3. The same mode protocol (how clients ask for a plan vs. data).

Plan Schema v1 fixes that contract.

## 1. Mode protocol

### Request

Clients indicate the desired mode either through the HTTP `?mode=` query parameter or via the request body:

```http
POST /api/v1/query/{workspace}/execute?mode=plan
Content-Type: application/json

{ "query": ".entity_set with(...) | entity-call get_metrics(...)", "mode": "plan" }
```

When both the body field and the query parameter are present, the body wins. When neither is present, the server applies its `default_mode`.

### Supported values

| Mode    | Meaning                                    | unified-model | umodel-assistant |
|---------|--------------------------------------------|---------------|------------------|
| `plan`  | Return a query plan; do not execute        | supported     | supported        |
| `data`  | Execute the plan; return rows of data      | rejected      | supported        |

unified-model rejects `mode=data` with HTTP 4xx and error code `NOT_IMPLEMENTED`. The error message must include a pointer to umodel-assistant so clients can migrate.

### Capabilities discovery

Servers expose their capabilities at `GET /api/v1/capabilities`:

```json
{
  "service": "unified-model",
  "version": "<server version>",
  "modes_supported": ["plan"],
  "default_mode": "plan"
}
```

SDKs and CLIs should call `/api/v1/capabilities` once at startup and adjust their default behavior accordingly. Hard-coding mode assumptions is non-compliant.

## 2. Plan JSON v1

A plan response is wrapped in the standard assistant query envelope:

```json
{
  "responseType": 1,
  "query": "<JSON-encoded plan>",
  "header": [],
  "data": []
}
```

The `query` field is a JSON-encoded string. After decoding, the plan must conform to the following shape:

```jsonc
{
  "mode": "plan",                     // discriminator; always "plan" in plan mode
  "version": "v1",                    // schema version; follows SemVer
  "operation": "get_metrics",         // entity-call method canonical name
  "data_source": {
    "data_set":    { "domain", "kind", "name" },
    "storage":     { "domain", "type", "name", "config" },
    "data_link":   { "domain", "name", "spec" },
    "storage_link":{ "domain", "name", "spec" }
  },
  "params_echo": {                    // caller-supplied entity-call params
    "metric": "request_count",        // empty strings and nil stripped
    "step":   "30s",
    "aggregate": false                // executors recover full call context here
  },
  "query": { /* method-specific storage query */ },
  "time_range": {                     // present only when set on the request
    "from": "...",
    "to":   "..."
  }
}
```

### Top-level fields

| Field         | Type   | Required | Notes                                              |
|---------------|--------|----------|----------------------------------------------------|
| `mode`        | string | yes      | Always `"plan"`. Mirrors request mode.             |
| `version`     | string | yes      | `"v1"` for this spec. Bumps follow SemVer.         |
| `operation`   | string | yes      | Canonical entity-call name (`get_metrics`, etc.).  |
| `data_source` | object | yes      | Resolved DataSet, Storage, DataLink, StorageLink.  |
| `params_echo` | object | yes      | Caller-supplied params, nil/empty stripped.        |
| `query`       | object | yes      | Storage-specific executable query.                 |
| `time_range`  | object | no       | Present when the request supplied a time range.    |

### `data_source` substructure

| Field                | Type    | Notes                                              |
|----------------------|---------|----------------------------------------------------|
| `data_set.domain`    | string  | Owning domain.                                     |
| `data_set.kind`      | string  | `metric_set` / `log_set` / etc.                    |
| `data_set.name`      | string  | DataSet name.                                      |
| `storage.domain`     | string  | Storage element domain.                            |
| `storage.type`       | string  | Storage kind (`prometheus`, `elasticsearch`, ...). |
| `storage.name`       | string  | Storage element name.                              |
| `storage.config`     | object  | Storage element spec, opaque to executor framework.|
| `data_link.spec`     | object  | Full DataLink spec including `fields_mapping`.     |
| `storage_link.spec`  | object  | Full StorageLink spec including `fields_mapping`.  |

### `params_echo` semantics

`params_echo` MUST contain every entity-call parameter the caller actually supplied, with values preserved at their native JSON type (boolean stays boolean, number stays number, string stays string). Empty strings and `null` values MUST be stripped so an executor cannot mistake an unset parameter for an explicit empty value. Defaults from the method signature are **not** echoed unless the caller actually set them.

## 3. Method signature contract

`get_metrics` and `get_logs` declare the union of all parameters either side may accept. The open-source planner ignores some of them; the PaaS executor consumes them all. Both sides surface the same set through `__list_method__`.

### `get_metrics`

| Key              | Type      | Required | OSS consumes | PaaS consumes | Default |
|------------------|-----------|----------|--------------|---------------|---------|
| `domain`         | varchar   | yes      | ✓            | ✓             |         |
| `name`           | varchar   | yes      | ✓            | ✓             |         |
| `metric`         | varchar   | no       | ✓            | ✓             |         |
| `query`          | varchar   | no       | ✓            | ✓             |         |
| `query_type`     | varchar   | no       | ✓            | ✓             |         |
| `step`           | varchar   | no       | ✓            | ✓             |         |
| `aggregate`      | boolean   | no       | echo only    | ✓             | `true`  |
| `storage_domain` | varchar   | no       | echo only    | ✓             |         |
| `storage_name`   | varchar   | no       | echo only    | ✓             |         |
| `storage_kind`   | varchar   | no       | echo only    | ✓             |         |

### `get_logs`

| Key              | Type    | Required | OSS consumes | PaaS consumes |
|------------------|---------|----------|--------------|---------------|
| `domain`         | varchar | yes      | ✓            | ✓             |
| `name`           | varchar | yes      | ✓            | ✓             |
| `query`          | varchar | no       | ✓            | ✓             |
| `storage_domain` | varchar | no       | echo only    | ✓             |
| `storage_name`   | varchar | no       | echo only    | ✓             |
| `storage_kind`   | varchar | no       | echo only    | ✓             |

The open-source parser MUST accept every parameter in these tables. "Echo only" means the planner records the value in `params_echo` but does not use it to alter the plan content.

### Aliases

Both sides expose canonical and alias names. unified-model normalizes `get_log` → `get_logs` and `get_metric` → `get_metrics` at parse time. umodel-assistant uses the singular form as canonical and accepts the plural as an alias. Either spelling MUST round-trip through both sides without behavior change.

## 4. Compatibility rules

The contract follows SemVer:

- **Minor bump within v1**: additive top-level fields, additive method parameters, broader value types. Existing consumers MUST keep working.
- **Major bump (v2+)**: any breaking change — renamed/removed top-level fields, removed method parameters, altered field types, changed required/optional flags.

Open-source unified-model MUST NOT introduce any method, parameter, or plan field that does not exist in umodel-assistant. PaaS is the source of truth; the open-source surface is a subset.

Either side proposing a v2 MUST publish an RFC, ship a migration window, and update this spec in lockstep with the new release.

## 5. Non-goals

This spec deliberately does NOT specify:

- The shape of `data`-mode responses on umodel-assistant (rows, labels, sample arrays). Those are PaaS-internal contracts; only the *transition* from plan to data is bound here.
- Storage executor internals (how PromQL is dispatched, how Elasticsearch DSL is built). Plans contain enough metadata for executors to do their job; the *how* is implementation-defined.
- Migration tooling that copies workspaces or entities from open source to PaaS. That is a separate effort tracked outside this spec.

## 6. Enforcement

Both projects MUST keep contract tests that:

- Decode a plan emitted by unified-model and verify all required v1 fields are present.
- Verify that the umodel-assistant plan parser accepts the unified-model plan output verbatim.
- Verify that `__list_method__` on each side reports the parameters in §3.

Any test failure in this set blocks merge on both repos.

## See also

- [Query Service Guide](../guides/query-service.md)
- [GraphStore Providers](../graphstore-providers.md)
- [Public Domain Models](../../../pkg/model/types.go)
- [Stable Error Codes](../../../pkg/errors/errors.go)
