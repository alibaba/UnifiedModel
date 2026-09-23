# Installation

中文：[安装与本地环境](../../zh/getting-started/installation.md)

Local development setup for UModel.


## Requirements

| Tool | Required for | Minimum |
|---|---|---:|
| Go | Service, CLI, MCP, Go tests | 1.22 |
| Make | Local workflows | any recent version |
| Python | Schema expansion, validation, SDK generation | 3.10 |
| Node.js | Web UI | 22 |
| pnpm | Web UI dependencies | 9 preferred |
| Java + Maven | Java SDK verification | Java 8 / Maven 3.6 |

## Clone And Prepare

Repository-root setup:

```bash
make check-env
make install-env
```

`make check-env` verifies Go, Python, Node.js, Web package-manager support, and optional Java/Maven tooling. `make install-env` creates `.venv`, installs Python dependencies from `tools/requirements.txt`, downloads Go modules, installs Web UI dependencies, and pre-resolves Java dependencies when Maven is available.

`make setup` is kept as an alias for `make install-env`.

## Web Package Manager Resolution

The Web UI prefers pnpm 9 or newer, but the Makefile also supports:

- `corepack pnpm` when `corepack` is available.
- `npm exec --package pnpm@<version>` when npm is available.
- Existing `web/node_modules` for local build/dev commands when dependencies are already installed.

## Build Service Binaries

```bash
make build-service
```

This builds:

- `umodel-server`
- `umctl`
- `umodel-mcp`

## Start Local Development Services

```bash
make dev
```

Immediate demo workspace:

```bash
make quickstart
```

`make quickstart` starts the same local services with `GRAPHSTORE=memory` and preloads the bundled multi-domain sample into `demo`.

Defaults:

| Setting | Value |
|---|---|
| API | `http://localhost:8080` |
| Web UI | `http://localhost:5173` |
| GraphStore | `file.memory` |
| Data root | `data` |

Check status:

```bash
make status
```

Stop local services:

```bash
make stop-all
```

## Run The API Without The Web UI

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

## Run The Web UI

In another terminal:

```bash
make dev-web
```

`make dev-web` uses the same package-manager resolution as `make install-env`.

## Provider Selection

Normal local development:

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

Short-lived tests:

```bash
go run ./cmd/umodel-server --addr :8080 --graphstore memory
```

Ladybug runtime:

```bash
go run -tags ladybug ./cmd/umodel-server --addr :8080 --data data --graphstore local.ladybug
```

Neo4j (configure the connection using [NEO4J_* variables](../graphstore-providers.md#neo4j)):

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore neo4j
```

## Verify The Checkout

```bash
make guard
make test-service
make example-validate
make verify
```

For Web UI changes:

```bash
make test-ui
```

`make test-ui` runs the Web UI type check and production build through the Makefile dependency wrapper.

## Next Step

Continue with [Quick Start](quickstart.md).
