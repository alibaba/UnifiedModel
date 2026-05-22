# UModel Web UI Architecture

中文：[Web UI 架构](../zh/ui-architecture.md)

## Product Shape

UModel Web: workspace-first local console. Flow: workspace chooser, workspace create/select, compact workbench. Workbench views: Explorer, Query, Imports & Writes, Agent, Settings, Data Store, and API Map.

UI contract: public UModel REST API only. No Aliyun internal packages, obviz libraries, cloud console SDKs, or Formily. Structured editing stays JSON-first for the open-source distribution.

## Frontend Modules

| Module | Path | Responsibility |
|---|---|---|
| API client | `web/src/api/` | Typed REST wrappers for the public OpenAPI contract. |
| Design system | `web/src/design/` | Tokens, buttons, panels, badges, tabs, fields, modal, JSON editor. |
| Workspace launcher | `web/src/features/workspaces/` | Health check, endpoint setting, workspace list, create workspace modal. |
| Workspace shell | `web/src/features/workspace/` | Sidebar navigation, top status bar, selected workspace lifecycle. |
| Explorer | `web/src/features/explorer/` | UModel graph/table, kind/search filters, JSON detail editor. |
| Query/Data | `web/src/features/query/` | Unified SPL console and entity/topology convenience views. |
| Imports | `web/src/features/imports/` | Import server-readable YAML/JSON paths, validate/write UModel JSON, write/expire EntityStore data. |
| Agent | `web/src/features/agent/` | Discovery, resources, enabled tools, next actions. |
| Settings/API Map | `web/src/features/settings/` | Workspace metadata edit/delete and UI-to-API mapping. |

## Component Boundaries

`ExplorerPage.tsx` currently contains draft state, graph filtering, ReactFlow
rendering, GraphViz layout, search, submit diff, JSON editing, and multiple
dialogs. Target module boundaries:

| Target module | Responsibility |
|---|---|
| `features/explorer/model/` | UModel kind metadata, title/tag helpers, search index, diff/filter pure functions. |
| `features/explorer/graph/` | ReactFlow nodes, edges, layout adapter, viewport interactions. |
| `features/explorer/panels/` | Summary/settings sidebar, detail JSON side panel, submit diff side panel. |
| `features/explorer/dialogs/` | Create node, upload, create link flows. |
| `features/explorer/hooks/` | Draft mutation, undo/redo, keyboard shortcuts, persisted view preferences. |

Workspace and Explorer share the same design system. The shared
`web/src/design/` layer now owns common panel radius, overlap, border, shadows,
buttons, icon buttons, tabs, and segmented controls. Feature-specific CSS may
still tune density and graph visuals, but it inherits tokens from
`--om-*` rather than defining a separate visual language.

## Design Principles

- Workspace comes first. Every data operation is scoped by a selected workspace.
- Professional, dense surface; closer to a coding workbench than a marketing page.
- Design tokens and components are local, simple, and reusable.
- JSON editing is the canonical open-source editing path; Formily schemas are not required.
- All read paths use Query Service (`.umodel`, `.entity`, `.topo`) rather than private domain APIs.
- Write surfaces call public write APIs only and show raw structured responses for debuggability.

## Runtime Modes

Requirements:

- Node.js 22 or newer
- pnpm 9 or newer is preferred; `corepack` or `npm exec` fallback is supported by the Makefile

Check and install local dependencies:

```bash
make check-env
make install-env
```

One-command local development:

```bash
make dev
```

Production from Go server:

```bash
make build-ui
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory --ui-dir web/dist
```

Separate API and Web UI processes:

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
make dev-web
```

Stop local development servers:

```bash
make stop-all
```

Persistent development storage:

```bash
make dev
```

The frontend talks only to the HTTP API. Persistence comes from the backend
GraphStore provider and `DATA_ROOT`; `make dev` uses `file.memory` and
`DATA_ROOT=data` by default.

Ladybug-enabled environment:

```bash
GO_TAGS=ladybug GRAPHSTORE=local.ladybug DATA_ROOT=data make dev
```

Both `memory` and `file.memory` support Ladybug-compatible read-only Cypher
through the pure Go engine.

One-command production-style local serving:

```bash
make serve-ui
```
