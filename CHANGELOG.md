# Changelog

All notable changes to UModel Open Source should be documented in this file.

The project follows a simple changelog structure until stable releases are published:

- `Added` for new features.
- `Changed` for behavior changes.
- `Fixed` for bug fixes.
- `Deprecated` for soon-to-be removed behavior.
- `Removed` for removed behavior.
- `Security` for vulnerability fixes.

## 0.2.0 - 2026-06-10

### Added

- **Plan/data mode protocol on Query Service** (#25). New `mode` request field and `?mode=` query parameter let clients ask for a query plan or a data execution. This open-source surface stays plan-only and rejects `mode=data` with a structured `NOT_IMPLEMENTED` error that carries `migration_*` details so agents can act on the failure without parsing prose. New `GET /api/v1/capabilities` endpoint advertises supported modes and formats.
- **Plan JSON v1 contract** (#25). Top-level envelope with `mode`, `version`, `operation`, `description`, `next_action`, `source_query`, `data_source`, `params_echo`, `query`, and `time_range`. Documented in `docs/{en,zh}/spec/plan-schema-v1.md` and treated as a release-blocking contract.
- **Plan schema v1.1 agent-friendly envelope** (#26). New `?format=agent` returns the plan as a top-level JSON object (no assistant envelope, no string-encoded plan), with `data_source.*` fields folded to compact `{ref, kind}` references for compact agent context. `?include=spec` re-expands `storage.config`, `data_link.spec`, and `storage_link.spec` for debugging and diagnostic agents.
- **`get_metrics` / `get_logs` plan emission with entity filters** (#18, #23). Both methods produce a structured query plan, render the downstream PromQL / Elasticsearch DSL, and translate entity ID and label filters into the storage-side query.
- **`get_metrics` / `get_logs` parameter alignment** (#25). Parser now accepts `aggregate` (metrics) and `storage_domain` / `storage_name` / `storage_kind` (both). The open-source planner echoes them in `params_echo` for downstream executors.
- **EntitySet dataset discovery** (#17). New `list_data_set` entity-call lists the related `metric_set` / `log_set` / `trace_set` / `event_set`, with optional detail expansion.
- **Multi-domain quickstart sample pack expanded.** New devops domain entries — `log_set` / `metric_set` / `event_set`, the corresponding storage and links, sample data, and tests.
- **Web UI improvements.** Workspace routing with UModel URLs (#15), Query Page example picker and result-table polish (#21), full internationalization across Explorer / Entity-Topo / Query components, Monaco editor preload, API Debugger panel, Imports feature, Settings refresh.

### Changed

- `docs/{en,zh}/README.md` documentation index gained a new **Specifications** section, pointing at `plan-schema-v1.md`.
- `params_echo` in plan output strips nil and empty-string entries; default values from the method signature are not echoed unless the caller actually set them.

### Fixed

- Local Ladybug provider could lose workspace metadata across service restarts (#22). Recovery now reads metadata back from the data root.
- Dependabot alerts on Vite resolved (#16).

### Security

- Reaffirmed: MCP write tools remain disabled by default. Security reporting policy unchanged.

## 0.1.0 - 2026-05-28

### Added

- Local single-process UModel service.
- Workspace metadata management.
- UModel import, validate, write, delete, and index paths.
- CMS 2.0 compatible entity and relation write/expire paths.
- Unified Query Service for `.umodel`, `.entity`, and `.topo`.
- AgentGateway discovery, safe query tools, resources, and MCP stdio server.
- `umctl` CLI for workspace, UModel, EntityStore, topology, query, and agent workflows.
- `memory`, `file.memory`, and optional `local.ladybug` GraphStore providers.
- React/Vite OpenUModel Web UI.
- REST OpenAPI and MCP tool/resource schemas.
- Generated Go, Python, and Java model SDK assets.
- APM common example pack and sample import endpoint.
- Architecture guard, contract tests, integration tests, e2e tests, and golden tests.

### Changed

- Open-source documentation now uses an external-developer-first README and structured docs index.
- Docker and Compose defaults now explicitly use `file.memory`.

### Security

- MCP write tools are disabled by default.
- Security policy and private-reporting guidance added.
