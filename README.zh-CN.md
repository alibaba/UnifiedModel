# UModel

[![CI](https://github.com/alibaba/UnifiedModel/actions/workflows/ci.yml/badge.svg)](https://github.com/alibaba/UnifiedModel/actions/workflows/ci.yml)
![Go 1.22+](https://img.shields.io/badge/Go-1.22%2B-00ADD8)
![Node 22+](https://img.shields.io/badge/Node.js-22%2B-339933)
![License](https://img.shields.io/badge/License-Apache--2.0-blue)

English version: [README.md](README.md)

UModel（Unified Model）是面向企业 AI、数据治理和智能运维的厂商中立语义运行时。它把分散的 schema、实体、业务对象、遥测链接和拓扑关系组织成 workspace-scoped 的对象图上下文，让人、系统和 AI Agent 通过一个本地服务理解并使用这些语义。

UModel 支持：

- 编写和导入模型包，定义企业对象、运维对象、数据集、链接、存储和拓扑语义。
- 写入 CMS 2.0 兼容的运行时实体与关系。
- 通过 `.umodel`、`.entity`、`.topo` 这一组 SPL 入口统一查询模型、实体和拓扑。
- 通过本地 Web UI 探索 workspace。
- 通过 AgentGateway 和 MCP 连接 Agent client。
- 使用公开 REST、CLI 和 SDK 契约，不依赖服务端内部实现。

## 为什么需要 UModel

- 加速企业 AI 规模化落地。统一语义标准让 AI 模型理解来自不同平台、不同部门、不同工具和不同领域的数据含义，提升智能运维、智能客服、智能分析、智能预测和 Agent 工作流的落地效率。
- 降低数据治理成本。多数据源、多工具、多系统共享同一套语义语言，数据团队不再反复消耗在口径对齐、字段翻译和上下文重建上，可以把更多精力投入数据价值挖掘。
- 保障厂商中立与选择自由。UModel 独立于特定平台、数据工具、可观测技术栈或 AI 供应商，企业构建数字化基础设施时可以避免语义层面的厂商锁定。
- 构建企业级语义操作系统。UModel 从被动查阅的数据辞典，升级为活的、主动的、可被 AI Agent 编程调用的语义运行时，为未来企业多智能体协作提供共享上下文。

## 项目范围

本仓库包含本地 UModel 服务、`umctl` CLI、MCP server、OpenAPI 契约、React Web UI、生成 SDK 资产、示例包、Docker/Compose 资产和测试套件。

开源核心聚焦本地运行、公共契约、语义建模、Agent 集成和 contributor-friendly 扩展点。Cloud-hosted control plane、multi-tenant authorization、Aliyun 内部前端包，以及 Query Service 之外的领域专用读取 API 不属于公共核心。

## 五分钟快速开始

依赖：

- Go 1.22 或更新版本。
- Make。
- 运行 Web UI 需要 Node.js 22 或更新版本。
- Web UI 依赖首选 pnpm 9 或更新版本；Makefile 支持 `corepack` 或 `npm exec` fallback。

检查本地工具链：

```bash
make check-env
```

启动 API 和 Web UI，并预加载 demo workspace：

```bash
make quickstart
```

`make quickstart` 会启动本地 API、启动 Web UI，并用 `GRAPHSTORE=memory` 预加载 `demo` workspace。进程停止后不保留本地 demo 数据。

下一步：

- 打开 `http://localhost:5173`，选择 `demo`，通过 Explorer、Query、Data Store 和 Agent 视图查看 workspace。
- 通过 AgentGateway 或 MCP 集成 Agent。先运行 `umctl agent discover demo`，再通过 `umodel-mcp` 连接 MCP client。
- 通过 CLI 或 REST 使用 Query Service 查询模型、实体和拓扑。

详细流程：

- [快速开始](docs/zh/getting-started/quickstart.md)
- [Web UI 指南](docs/zh/guides/web-ui.md)
- [Query Service 指南](docs/zh/guides/query-service.md)
- [MCP 参考](docs/zh/reference/mcp.md)

停止本地服务：

```bash
make stop-all
```

## 架构

![UModel 架构](images/architecture.png)

UModel 围绕 workspace-scoped object graph 运行本地服务：

- 模型包定义对象词汇：EntitySet、DataSet、Link、Storage 和关系语义。
- EntityStore 写入运行时实体与拓扑关系，用运行时数据实例化模型。
- Query Service 是 `.umodel`、`.entity`、`.topo` 的统一读取入口。
- AgentGateway 和 MCP 为 Agent client 暴露 discovery、resources、query examples 和安全工具。
- Web UI、CLI、REST 和 SDK client 共享同一套公开契约。

架构细节：

- [架构总览](docs/zh/architecture/overview.md)
- [运行时流程](docs/zh/architecture/runtime-flow.md)
- [Query 与 Agent 架构](docs/zh/architecture/query-and-agent.md)

## 文档

从双语文档索引开始：[docs/README.md](docs/README.md)。

| 领域 | 入口 |
|---|---|
| 入门 | [安装](docs/zh/getting-started/installation.md)、[快速开始](docs/zh/getting-started/quickstart.md) |
| 概念 | [概念索引](docs/zh/concepts/index.md)、[对象图语义层](docs/zh/concepts/object-graph-semantic-layer.md) |
| 指南 | [模型编写](docs/zh/guides/model-authoring.md)、[实体与关系写入](docs/zh/guides/entity-relation-writes.md)、[Query Service](docs/zh/guides/query-service.md)、[Web UI](docs/zh/guides/web-ui.md)、[SDK 与客户端](docs/zh/guides/sdk-clients.md) |
| 架构 | [架构总览](docs/zh/architecture/overview.md)、[运行时流程](docs/zh/architecture/runtime-flow.md)、[Query 与 Agent 架构](docs/zh/architecture/query-and-agent.md) |
| 参考 | [CLI](docs/zh/reference/cli.md)、[MCP](docs/zh/reference/mcp.md)、[REST OpenAPI](api/openapi/openapi.yaml)、[MCP Tool 和 Resource Schema](api/mcp/tools.schema.json) |
| 示例 | [多域 Quickstart 示例包](examples/quickstart-multidomain/README.zh-CN.md)、[故障排查 Demo（AI Agent）](examples/incident-investigation/README.zh-CN.md) |
| 部署 | [Docker 与 Compose](deployments/README.zh-CN.md) |

英文文档：[docs/en/README.md](docs/en/README.md)。

## 开发

安装本地依赖：

```bash
make install-env
```

构建：

```bash
make build
```

运行定向检查：

```bash
make guard
make test-service
make verify
make example-validate
```

运行本地 CI gate：

```bash
make ci
```

生成的 Go 和 Python 模型 SDK 位于 `sdk/`。Java SDK 当前仍在 `generated/java/`。最小 Go service client 位于 `sdk/go/service`，只封装公开 REST 契约。

## GraphStore Providers

运行时 GraphStore provider 通过 `--graphstore` 选择。

| Provider | 典型用途 |
|---|---|
| `memory` | 临时本地测试和 quickstart demo。进程退出后数据丢失。 |
| `file.memory` | `--data` 下的 JSON 持久化。这是 `make dev`、Docker 和 Compose 的默认值。 |
| `local.ladybug` | Ladybug-backed 环境。需要 `-tags ladybug` 和本地 Ladybug runtime。 |

Provider 细节：[GraphStore Providers](docs/zh/graphstore-providers.md)。

## 治理与支持

- License: [Apache-2.0](LICENSE)
- 贡献：[CONTRIBUTING.zh-CN.md](CONTRIBUTING.zh-CN.md)
- 安全策略：[SECURITY.zh-CN.md](SECURITY.zh-CN.md)
- 支持渠道：[SUPPORT.zh-CN.md](SUPPORT.zh-CN.md)
- 行为准则：[CODE_OF_CONDUCT.zh-CN.md](CODE_OF_CONDUCT.zh-CN.md)
- 变更日志：[CHANGELOG.zh-CN.md](CHANGELOG.zh-CN.md)
