# Agent 集成指南

English: [Agent Integration Guide](../../en/guides/agent-integration.md)

本指南介绍如何把 AI Agent 接入 UModel，以及 Agent 如何借助对象图完成真实工作。全文贯穿的样例是 [故障排查 Demo](../../../examples/incident-investigation/README.zh-CN.md)——Agent 通过遍历对象图并遵循 Runbook，定位一次支付网关 SLO 违约的根因。

UModel 通过 **Model Context Protocol（MCP）** 暴露 Agent 接口，底层由 AgentGateway 和 Query Service 支撑。Agent 读取的一切都走同一个 SPL 查询面，因此只需学习一套契约，即可复用于模型、实体、拓扑、数据集和 Runbook。

## 1. 接入 MCP 客户端

MCP server 是 `cmd/umodel-mcp`，启动时可用 `--quickstart-sample` 预载一个 Demo workspace，Agent 连上即有数据可查。

### 本地（stdio）

加到 MCP 客户端配置（`.mcp.json`，或 Claude Code / Cursor / Qoder 的等价配置）：

```json
{
  "mcpServers": {
    "umodel": {
      "command": "go",
      "args": [
        "run", "./cmd/umodel-mcp",
        "--quickstart",
        "--quickstart-sample", "examples/incident-investigation",
        "--graphstore", "memory"
      ]
    }
  }
}
```

### 远程（Streamable HTTP）

启动 server：

```bash
go run ./cmd/umodel-mcp --quickstart \
  --quickstart-sample examples/incident-investigation \
  --graphstore file.memory \
  --transport http --addr 0.0.0.0:8090
```

客户端连接：

```json
{
  "mcpServers": {
    "umodel": { "type": "streamable-http", "url": "http://<host>:8090/mcp" }
  }
}
```

传输方式（stdio、Streamable HTTP、HTTP+SSE）、协议版本以及本地冒烟测试见 [MCP 参考](../reference/mcp.md)。

## 2. Agent 能看到什么

连接后，Agent 发现一个固定、安全的接口（AgentGateway）。工具与资源发现也可通过 REST `/api/v1/agent/{workspace}/discover` 获取。

### 工具

| 工具 | 默认 | 用途 |
|---|---|---|
| `query_spl_execute` | 启用 | 执行 SPL 查询（`.umodel` / `.entity` / `.entity_set` / `.topo` / `.runbook_set`） |
| `query_spl_explain` | 启用 | 返回查询计划与生效的 provider，不执行 |
| `query_spl_examples` | 启用 | 返回安全、可直接运行的示例查询 |
| `umodel_validate` | 启用 | 校验 UModel 元素定义 |
| `umodel_import` | **禁用** | 导入 UModel 文件（写操作；需服务端显式开启） |
| `entity_write` | **禁用** | 写入实体数据（写操作） |
| `entity_expire` | **禁用** | 过期实体数据（写操作） |

读工具默认启用，写工具默认关闭，需运维显式开启。Agent 开箱即可安全探索与诊断。

> `query_spl_execute` 的参数键是 **`query`**，不是 `spl`：
> `query_spl_execute {"workspace": "demo", "query": ".umodel | limit 5"}`。

### 资源

只读、仅元数据——在查询前帮 Agent 建立认知：

| 资源 | URI | 用途 |
|---|---|---|
| overview | `umodel://workspace/{ws}/overview` | API 总览与安全入口 |
| schema-index | `umodel://workspace/{ws}/schema-index` | 模型 / schema 摘要 |
| query-templates | `umodel://workspace/{ws}/query-templates` | `.umodel` / `.entity` / `.entity_set` / `.topo` 模板 |
| tool-capability-metadata | `umodel://workspace/{ws}/tool-capability-metadata` | 工具开关 + 输入/输出 schema |

## 3. Agent 使用的查询面

四种数据源都通过 `query_spl_execute` 走。下面是故障排查 walkthrough 里的真实查询。

### `.entity`——按全文检索找实体

```
.entity with(domain='platform', name='platform.service', query='degraded')
  | project display_name, status, owner, sla_tier
```

`query=` 在实体字段上做检索。配置了搜索 provider 时，`mode='vector'` / `mode='hyper'` 切换到语义 / 混合检索；`topk=` 限制结果数。

### `.topo`——遍历对象图

```
.topo | graph-call getNeighborNodes('full', 1,
  [(:"platform@platform.service" {__entity_id__: '63718b78868895d2590551b27ec6f51c'})])
  | with(__relation_type__='calls')
```

Graph-call：`getNeighborNodes(direction, hops, nodes)`、`getDirectRelations(nodes)`，以及支持完整 Cypher 的 `cypher(\`...\`)`。拓扑行携带实体 ID 与关系属性；需要 display_name 时再用一次 `.entity` 查询解析。

### `.entity_set`——发现数据集并生成查询计划

```
# 这个 EntitySet 能做什么？
.entity_set with(domain='platform', name='platform.service', ids=['...']) | entity-call __list_method__()

# 挂了哪些数据集？
.entity_set with(domain='platform', name='platform.service', ids=['...']) | entity-call list_data_set(['metric_set','log_set'], true)

# 拉取可观测信号——返回可执行的查询计划
.entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c'])
  | entity-call get_metrics('platform', 'platform.service.metrics', 'latency_p99_ms', step='30s')

.entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c'])
  | entity-call get_logs('platform', 'platform.service.logs', query='level = "ERROR"')
```

`get_metrics` / `get_logs` 返回的是 **查询计划**——UModel 开源版只产出计划，因此会渲染下游 PromQL / Elasticsearch 查询（`service_id` 直接从对象图取值），但不执行。由下游 executor 针对真实存储执行。完整管道词汇见 [Query Service 指南](query-service.md)。

## 4. `?format=agent`——给 Agent 的紧凑信封

默认情况下计划被包在 assistant 信封里、且计划体被 JSON 编码进字符串。对 Agent，请求 v1.1 agent 信封后，计划作为 **顶层 JSON 对象** 返回，`data_source.*` 折叠成紧凑的 `{ref, kind}` 引用，几乎不占上下文：

```bash
curl -s -X POST 'http://localhost:8080/api/v1/query/demo/execute?format=agent' \
  -H 'Content-Type: application/json' \
  -d '{"query":".entity_set with(domain=\"platform\", name=\"platform.service\", ids=[\"63718b78868895d2590551b27ec6f51c\"]) | entity-call get_metrics(\"platform\",\"platform.service.metrics\",\"latency_p99_ms\", step=\"30s\")"}'
```

调试 Agent 需要完整 storage config 与 link spec 时，加 `&include=spec`。完整契约：[Plan Schema v1](../spec/plan-schema-v1.md)。

## 5. 端到端：Agent 排查一次故障

载入故障排查 workspace 后，连接 MCP 客户端并提问：

> "payment-gateway SLO 违约了，帮我排查。"

Agent 执行对象图循环：

1. **定位**——`.entity` 找到 `payment-gateway`（`status=degraded`、`sla_tier=platinum`）。
2. **读信号**——`get_metrics` 返回 P99 延迟计划，`get_logs` 返回错误日志计划。对象图把"那个 degraded 的服务"翻译成精确的可观测查询，Agent 无需手写 PromQL。
3. **加载 Runbook**——服务关联 `platform.service.ops`，Agent 遵循其结构化观察项。
4. **查上游**——`.topo` 找到调用 payment-gateway 的 `checkout-service`；一条 `config_change` 显示重试从 2 提到 5。
5. **排除 red herring**——最近的 `payment-gw v3.2.1` 部署其实只是改了日志格式。
6. **跨域压力**——业务层显示 `618 Flash Sale` 促销带来 3.5× 流量。
7. **关联与建议**——重试放大 × 促销流量 = 8.75× 过载；Runbook 建议 `rollback_config_change`。

完整 walkthrough、Runbook 内容和 Agent 输出示例见 [故障排查 Demo](../../../examples/incident-investigation/README.zh-CN.md)。

## 相关文档

- [MCP 参考](../reference/mcp.md)——传输、工具、资源、prompts、冒烟测试
- [Query Service 指南](query-service.md)——完整 SPL 查询面
- [Query 与 Agent 架构](../architecture/query-and-agent.md)——AgentGateway / Query Service 边界
- [Plan Schema v1](../spec/plan-schema-v1.md)——计划契约与 `?format=agent` 信封
- [故障排查 Demo](../../../examples/incident-investigation/README.zh-CN.md)——贯穿样例
