# Web UI 架构

English: [UModel Web UI Architecture](../en/ui-architecture.md)

## 产品形态

UModel Web 是 workspace-first 的本地控制台。流程：workspace chooser，workspace 创建或选择，紧凑工作台。工作台视图包含 Explorer、Query、Imports & Writes、Agent、Settings、Data Store 和 API Map。

UI 契约：只使用公开 UModel REST API。不导入 Aliyun 内部包、obviz libraries、云控制台 SDK 或 Formily。开源发行版的结构化编辑路径保持 JSON-first。

## 前端模块

| 模块 | 路径 | 职责 |
|---|---|---|
| API client | `web/src/api/` | 面向 public OpenAPI contract 的 typed REST wrappers。 |
| Design system | `web/src/design/` | Tokens、buttons、panels、badges、tabs、fields、modal、JSON editor。 |
| Workspace launcher | `web/src/features/workspaces/` | Health check、endpoint setting、workspace list、create workspace modal。 |
| Workspace shell | `web/src/features/workspace/` | Sidebar navigation、top status bar、selected workspace lifecycle。 |
| Explorer | `web/src/features/explorer/` | UModel graph/table、kind/search filters、JSON detail editor。 |
| Query/Data | `web/src/features/query/` | Unified SPL console 与 entity/topology convenience views。 |
| Imports | `web/src/features/imports/` | Import server-readable YAML/JSON paths、validate/write UModel JSON、write/expire EntityStore data。 |
| Agent | `web/src/features/agent/` | Discovery、resources、enabled tools、next actions。 |
| Settings/API Map | `web/src/features/settings/` | Workspace metadata edit/delete 与 UI-to-API mapping。 |

## 组件边界

`ExplorerPage.tsx` 当前包含 draft state、graph filtering、ReactFlow rendering、GraphViz layout、search、submit diff、JSON editing 和多个 dialogs。目标模块边界：

| 目标模块 | 职责 |
|---|---|
| `features/explorer/model/` | UModel kind metadata、title/tag helpers、search index、diff/filter pure functions。 |
| `features/explorer/graph/` | ReactFlow nodes、edges、layout adapter、viewport interactions。 |
| `features/explorer/panels/` | Summary/settings sidebar、detail JSON side panel、submit diff side panel。 |
| `features/explorer/dialogs/` | Create node、upload、create link flows。 |
| `features/explorer/hooks/` | Draft mutation、undo/redo、keyboard shortcuts、persisted view preferences。 |

Workspace 与 Explorer 共享同一套 design system。共享的 `web/src/design/` 层负责 common panel radius、overlap、border、shadows、buttons、icon buttons、tabs 和 segmented controls。Feature-specific CSS 可调整 density 和 graph visuals，但继承 `--om-*` tokens，不定义另一套视觉语言。

## 设计原则

- Workspace comes first。每个数据操作都由选中的 workspace 限定范围。
- 界面是专业且高密度的工作台，更接近 coding workbench，而不是 marketing page。
- Design tokens 与 components 保持本地、简单、可复用。
- JSON editing 是开源版本的标准编辑路径；不要求 Formily schemas。
- 所有读取路径都使用 Query Service（`.umodel`、`.entity`、`.topo`），而不是 private domain APIs。
- 写入界面只调用 public write APIs，并展示 raw structured responses，便于 debug。

## 运行模式

依赖：

- Node.js 22 或更新版本
- 首选 pnpm 9 或更新版本；Makefile 支持 `corepack` 或 `npm exec` fallback

检查并安装本地依赖：

```bash
make check-env
make install-env
```

一条命令启动本地开发：

```bash
make dev
```

通过 Go server 提供生产构建：

```bash
make build-ui
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory --ui-dir web/dist
```

分别启动 API 和 Web UI：

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
make dev-web
```

停止本地开发服务：

```bash
make stop-all
```

持久化开发存储：

```bash
make dev
```

前端只与 HTTP API 通信。持久化由后端 GraphStore provider 和 `DATA_ROOT` 决定；`make dev` 默认使用 `file.memory` 和 `DATA_ROOT=data`。

Ladybug-enabled 环境：

```bash
GO_TAGS=ladybug GRAPHSTORE=local.ladybug DATA_ROOT=data make dev
```

`memory` 和 `file.memory` 都通过 pure Go engine 支持 Ladybug-compatible read-only Cypher。

一条命令启动 production-style local serving：

```bash
make serve-ui
```
