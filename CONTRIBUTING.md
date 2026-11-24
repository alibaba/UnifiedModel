# Contributing To UModel

Thank you for helping improve UModel. This repository is an open-source implementation of the UModel object graph semantic layer, so contributions should keep the public API, CLI, MCP, SDK, examples, and documentation aligned.

Chinese version: [CONTRIBUTING.zh-CN.md](CONTRIBUTING.zh-CN.md)

## Requirements

| Tool | Minimum version | Required | Used for |
|---|---:|---:|---|
| Go | 1.22 | Yes | Service, CLI, MCP, tests, Go SDK checks |
| Python | 3.10 | Yes | Schema expansion, validation, code generation |
| Make | Any recent version | Yes | Local workflows |
| Node.js | 22 | Web UI changes only | React/Vite build |
| pnpm | 9 preferred | Web UI changes only | Web UI dependencies; corepack or npm exec fallback is supported |
| Java + Maven | Java 8 / Maven 3.6 | Java SDK checks only | Generated Java SDK validation |

Check and install local dependencies:

```bash
make check-env
make install-env
```

`make check-env` verifies local tooling. `make install-env` creates `.venv`, installs Python dependencies from `tools/requirements.txt`, downloads Go module dependencies, installs Web UI dependencies, and pre-resolves Java dependencies when Maven is installed. `make setup` is kept as an alias for `make install-env`.

## Repository Map

| Path | Purpose |
|---|---|
| `api/` | Public REST OpenAPI and MCP tool/resource contracts. |
| `cmd/` | Entry binaries: `umodel-server`, `umctl`, `umodel-mcp`. |
| `docs/` | Concept, guide, reference, SDK, UI, and GraphStore documentation. |
| `examples/` | UModel packs and runnable sample data. |
| `schemas/` | UModel schema source definitions. |
| `sdk/go`, `sdk/python` | Generated Go and Python model SDKs plus Go service client. |
| `generated/java` | Generated Java model SDK. |
| `internal/` | Private service modules and infrastructure. |
| `pkg/` | Public domain models, errors, and service contracts. |
| `tests/` | Contract, integration, e2e, golden, and architecture tests. |
| `tools/` | Generators, validators, converters, guards, and release tooling. |
| `web/` | Open-source React/Vite workspace UI. |

## Local Development

Run the local API and Web UI:

```bash
make dev
```

`make dev` starts `umodel-server` with `GRAPHSTORE=file.memory`, stores local data under `data/`, and starts the Vite UI on `http://localhost:5173`.

Stop local services:

```bash
make stop-all
```

Build service binaries:

```bash
make build-service
```

Build the Web UI:

```bash
make build-ui
```

`make build-ui`, `make test-ui`, and `make dev-web` use the Makefile environment wrapper, so contributors do not need to call `pnpm` directly when `corepack`, `npm exec`, or an existing `web/node_modules` can satisfy the workflow.

## Contribution Workflows

### Schema Or SDK Changes

1. Edit `schemas/**/*.yaml`.
2. Update `schemas/manifest.yaml` when adding a new schema kind or version.
3. Regenerate expanded schemas and SDKs:

```bash
make expand
```

4. Add or update examples under `examples/`.
5. Validate generated assets and examples:

```bash
make verify
make example-validate
make public-links-check
```

Generated SDK locations:

- Go: `sdk/go/umodel`
- Python: `sdk/python/umodel`
- Java: `generated/java`

Do not hand-edit generated SDK files unless you are changing generated output as part of a generator update.

Keep schema descriptions and public documentation self-contained. Remove private DingTalk document references from source schemas before regenerating SDKs and documentation. `make public-links-check` scans source schemas, expanded and embedded schemas, SDKs, and public documentation; `make guard` and `make ci` run this check and reject private document links.

### Service, CLI, Query, Or MCP Changes

1. Keep all public contracts in sync: `api/openapi/openapi.yaml`, `api/mcp/tools.schema.json`, `pkg/model`, and `pkg/errors`.
2. Keep reads behind the Query Service. Do not add domain-specific entity, relation, or graph read APIs.
3. Update `umctl` and documentation when public behavior changes.
4. Add focused tests in `internal/` plus contract, integration, e2e, or golden coverage when behavior crosses module boundaries.

Recommended checks:

```bash
make guard
make test-service
```

### Web UI Changes

1. Use the public REST API only.
2. Keep workspace context explicit.
3. Keep UModel editing JSON-first unless a schema-aware editor is intentionally added behind the same public validate/write APIs.
4. Build before submitting:

```bash
make test-ui
```

### Documentation Changes

1. Update the closest guide or reference page, not only the top-level README.
2. Keep commands copy-pasteable from the repository root unless stated otherwise.
3. Verify relative links.
4. If behavior changes, update both user-facing docs and contract docs.

## Quality Gates

Run the local CI gate before opening a pull request:

```bash
make ci
```

For Web UI changes, also run:

```bash
make test-ui
```

For Ladybug-backed provider changes, run this in an environment with `liblbug` installed:

```bash
UMODEL_TEST_LADYBUG=1 make test-ladybug
```

## Architecture Rules

- Workspace is metadata only.
- All model, entity, relation, and topology reads go through Query Service.
- EntityStore writes, expires, or deletes entity and relation data only.
- Business modules depend on `pkg/contract` and `pkg/model`, not provider implementations.
- AgentGateway resources expose metadata, templates, and capabilities. They must not expose runtime entity or topology rows as resources.
- Public SDKs wrap public REST/MCP contracts and must not expose server internals.

The architecture guard enforces the most important rules:

```bash
make guard
```

## Pull Request Checklist

- [ ] The change has a clear user or contributor-facing purpose.
- [ ] Public API, CLI, MCP, SDK, examples, and docs are updated together when affected.
- [ ] `make guard` passes.
- [ ] `make test-service` passes for service changes.
- [ ] `make verify` and `make example-validate` pass for schema or SDK changes.
- [ ] `make test-ui` passes for Web UI changes.
- [ ] New or changed behavior has focused tests.
- [ ] The PR description calls out compatibility or migration notes.

## Support And Security

Use [SUPPORT.md](SUPPORT.md) for support paths and [SECURITY.md](SECURITY.md) for vulnerability reporting. Do not disclose security issues publicly before maintainers have had time to triage them.
