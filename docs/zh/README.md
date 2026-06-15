# UModel 中文文档

UModel 中文文档入口。中文文档与英文文档分别维护在 `docs/zh` 和 `docs/en` 下，目录结构保持一致，示例、命令和公共契约引用保持对齐。

文档总入口：[docs/README.md](../README.md)

English: [UModel Documentation](../en/README.md)

## 入门

- [项目 README](../../README_CN.md) - 项目定位、快速开始、架构和治理入口。
- [安装与本地环境](getting-started/installation.md) - 依赖、构建、启动和 GraphStore provider 选择。
- [快速开始](getting-started/quickstart.md) - 创建 workspace、导入多域样例、运行第一组查询。
- [GraphStore Providers](graphstore-providers.md) - `memory`、`file.memory`、`local.ladybug` 的选择和边界。
- [部署](../../deployments/README.zh-CN.md) - Docker、Compose、端口、数据目录和 provider 配置。

## 概念

- [概念索引](concepts/index.md) - 推荐阅读顺序和概念地图。
- [对象图语义层](concepts/object-graph-semantic-layer.md) - UModel 解决的问题，以及它与企业数据、遥测、运行时系统和 Agent 上下文的关系。
- [Workspace 与 Domain](concepts/workspaces-and-domains.md) - 隔离、命名和本地持久化边界。
- [Model Elements](concepts/model-elements.md) - 模型元素的通用 envelope 和支持的 kind。
- [EntitySet](concepts/entity-sets.md) - 对象类型定义与运行时实体的关系。
- [DataSet](concepts/datasets.md) - 指标、日志、链路、事件、Profile 和 Runbook。
- [Link 与字段映射](concepts/links-and-field-mappings.md) - DataLink、EntitySetLink、StorageLink 和 `fields_mapping`。
- [Storage 与 GraphStore](concepts/storage-and-graphstore.md) - 模型中的存储定义与运行时 provider。
- [Entity 与 Relation](concepts/entities-and-relations.md) - 运行时对象图数据和生命周期。
- [查询入口](concepts/query-surfaces.md) - `.umodel`、`.entity`、`.topo`、explain 和 Agent 用法。

## 使用指南

- [模型编写指南](guides/model-authoring.md)
- [实体与关系写入指南](guides/entity-relation-writes.md)
- [Query Service 指南](guides/query-service.md)
- [Web UI 指南](guides/web-ui.md)
- [SDK 与客户端指南](guides/sdk-clients.md) - REST、CLI、MCP、生成模型 SDK 和集成示例。
- [Agent 集成指南](guides/agent-integration.md) - 接入 MCP 客户端、Agent 的工具/资源接口、Agent 使用的查询面，以及一个完整的故障排查样例。
- [Agent 技能](../../skills/README.zh-CN.md) - 可加载给 MCP/CLI Agent 的 `SKILL.md`：读实体/关系/模型数据，做模型引导的根因分析。
- [MCP 示例](../../examples/mcp/README.zh-CN.md) - stdio、Streamable HTTP、HTTP+SSE 和 TOON payload 示例。

## 示例

- [多域 Quickstart 示例包](../../examples/quickstart-multidomain/README.zh-CN.md) - 一个 workspace 里连接五个 domain；`make quickstart` 默认样例。
- [故障排查 Demo](../../examples/incident-investigation/README.zh-CN.md) - 场景驱动、AI Agent 辅助的根因分析：跨业务、平台、运行时三个 domain 排查支付网关 SLO 违约，演示 runbook 引导的诊断路径。

## 架构

- [架构总览](architecture/overview.md) - 系统视图、分层、公共契约和 guardrails。
- [运行时流程](architecture/runtime-flow.md) - 启动、导入、写入、查询、Agent 和持久化流程。
- [Query 与 Agent 架构](architecture/query-and-agent.md) - Query Service 与 AgentGateway 边界。
- [扩展点](architecture/extension-points.md) - 模型包、Schema、Provider、Query、API、SDK 和 UI 扩展。
- [Web UI 架构](ui-architecture.md)

## 参考

- [CLI 参考](reference/cli.md)
- [MCP 参考](reference/mcp.md)
- [Web UI API 对照](ui-api.md)
- [UModel SDK 规范](umodel-sdk-specification.md)
- [REST OpenAPI](../../api/openapi/openapi.yaml)
- [MCP Tool 和 Resource Schema](../../api/mcp/tools.schema.json)
- [公共 Go Contracts](../../pkg/contract/contracts.go)
- [公共领域模型](../../pkg/model/types.go)
- [稳定错误码](../../pkg/errors/errors.go)

## 规范

跨实现的契约文档，同时约束 `unified-model`（开源）与 `umodel-assistant`（商业版）。任一方破坏视为 P0 回归。

- [Plan Schema v1](spec/plan-schema-v1.md) - mode 协议、plan JSON 信封以及 `get_metrics` / `get_logs` 的对齐方法签名。

## 生成的 Schema 文档

- [中文 Schema HTML](../html_cn/index.html)
- [英文 Schema HTML](../html_en/index.html)
- [默认 Schema HTML](../html/index.html)

从仓库根目录重新生成：

```bash
make doc
```

## 贡献与发布

- [贡献指南](../../CONTRIBUTING.zh-CN.md)
- [行为准则](../../CODE_OF_CONDUCT.zh-CN.md)
- [安全策略](../../SECURITY.zh-CN.md)
- [支持渠道](../../SUPPORT.zh-CN.md)
- [变更日志](../../CHANGELOG.zh-CN.md)
- [文档国际化规则](i18n.md)
