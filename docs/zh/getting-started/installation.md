# 安装与本地环境

English: [Installation](../../en/getting-started/installation.md)

UModel 本地开发环境、服务构建、API/Web UI 启动和 GraphStore provider 选择。


## 依赖

| 工具 | 用途 | 最低版本 |
|---|---|---:|
| Go | 服务端、CLI、MCP、Go 测试 | 1.22 |
| Make | 本地工作流 | 近期版本 |
| Python | Schema 展开、验证、SDK 生成 | 3.10 |
| Node.js | Web UI | 22 |
| pnpm | Web UI 依赖 | 首选 9 |
| Java + Maven | Java SDK 验证 | Java 8 / Maven 3.6 |

## 准备环境

仓库根目录设置：

```bash
make check-env
make install-env
```

`make check-env` 检查 Go、Python、Node.js、Web package-manager 支持，以及可选 Java/Maven 工具。`make install-env` 创建 `.venv`、安装 `tools/requirements.txt` 中的 Python 依赖、下载 Go modules、安装 Web UI 依赖，并在 Maven 可用时预解析 Java 依赖。

`make setup` 保留为 `make install-env` 的别名。

## Web Package Manager 解析

Web UI 首选 pnpm 9 或更新版本，但 Makefile 也支持：

- 当 `corepack` 可用时使用 `corepack pnpm`。
- 当 npm 可用时使用 `npm exec --package pnpm@<version>`。
- 当依赖已经安装时，使用已有 `web/node_modules` 执行本地 build/dev 命令。

## 构建服务二进制

```bash
make build-service
```

构建产物：

- `umodel-server`
- `umctl`
- `umodel-mcp`

## 启动本地开发服务

```bash
make dev
```

立即加载 demo workspace：

```bash
make quickstart
```

`make quickstart` 使用 `GRAPHSTORE=memory` 启动同一套本地服务，并把内置多域样例预加载到 `demo`。

默认值：

| 配置 | 默认值 |
|---|---|
| API | `http://localhost:8080` |
| Web UI | `http://localhost:5173` |
| GraphStore | `file.memory` |
| 数据目录 | `data` |

查看状态：

```bash
make status
```

停止服务：

```bash
make stop-all
```

## 只启动 API

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

## 启动 Web UI

另开终端执行：

```bash
make dev-web
```

`make dev-web` 使用与 `make install-env` 相同的 package-manager 解析逻辑。

## Provider 选择

常规本地开发：

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

短生命周期测试：

```bash
go run ./cmd/umodel-server --addr :8080 --graphstore memory
```

Ladybug runtime：

```bash
go run -tags ladybug ./cmd/umodel-server --addr :8080 --data data --graphstore local.ladybug
```

Neo4j（使用 [NEO4J_* 环境变量](../graphstore-providers.md#neo4j)配置连接）：

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore neo4j
```

## 验证仓库

```bash
make guard
make test-service
make example-validate
make verify
```

Web UI 改动还应执行：

```bash
make test-ui
```

`make test-ui` 通过 Makefile 依赖 wrapper 执行 Web UI type check 和 production build。

## 下一步

继续阅读 [快速开始](quickstart.md)。
