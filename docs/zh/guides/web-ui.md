# Web UI 指南

English: [Web UI Guide](../../en/guides/web-ui.md)

UModel Web：面向 workspace、模型定义、运行时数据、Query Service 行为和 AgentGateway 元数据的本地控制台。


## 启动

```bash
make quickstart
```

打开：

```text
http://localhost:5173
```

## Workspace Launcher

Workspace Launcher 覆盖：

- 创建 workspace。
- 选择已有 workspace。
- API health。
- 非默认本地端口下的 API endpoint override。

## Explorer

Explorer 显示已导入的 UModel 定义，支持图视图和表视图。

Explorer 范围：

- EntitySet。
- DataSet。
- DataLink。
- EntitySetLink。
- Storage 和 StorageLink。

## Query

Query 是交互式查询入口：

- `.umodel`
- `.entity`
- `.topo`

Query 负责在 CLI、SDK、MCP 或文档复用之前验证工作流。

## Imports & Writes

Imports & Writes 支持：

- UModel 模型导入。
- Entity 写入。
- Relation 写入。

写入操作应保持显式，并在提交前检查 payload。

## Data Store

Data Store 在 Query Service 边界之后提供运行时 entity 和 topology 视图。`make quickstart` 之后展示内置多域样例。

## Agent

Agent 视图展示：

- Discovery output。
- Tools。
- Resources。
- Query examples。
- Suggested next actions。

同一套 AgentGateway surface 供 MCP client 使用。

## Settings And API Map

Settings 和 API Map 暴露：

- API health。
- Active provider。
- Common endpoints。
- UI 使用的 public REST paths。

## 开发规则

- UI 代码：[web/](../../../web)
- UI 架构：[Web UI Architecture](../ui-architecture.md)
- API 对照：[Web UI API Map](../ui-api.md)
- UI 应只调用公开 REST API。
