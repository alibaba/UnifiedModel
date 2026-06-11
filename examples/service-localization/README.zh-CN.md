# 服务定位 Demo

English: [Service Localization Demo](README.md)

基于场景驱动的示例，展示 AI Agent 如何用 UModel **拉取可观测数据、沿四层请求栈定位瓶颈**——产品 API → 服务 → 数据存储 → 基础设施。一个性能退化的 checkout API 被逐跳追踪到饱和的数据库连接池，同时基础设施层被排除。

本 Demo 与 [故障排查 Demo](../incident-investigation/README.zh-CN.md) 互补：那个是*反应式根因分析*（症状 → 根因，Runbook 引导）；这个是*纵向定位*（沿关键路径走，把延迟归因到某一层），并把**取数**放在最核心。

```
Checkout Flow（旅程，impacted，2.1% 错误）
  └─ depends_on → checkout-api（degraded，P99 > 300ms SLO）
                   └─ calls → order-svc（degraded，但 CPU 健康）
                               └─ reads_writes → orders-db（饱和——连接池约 98%）   ← 根因
                                                  └─ hosted_on → node-a（健康）       ← 基础设施排除

健康的兄弟节点：catalog-api/svc、search-api/svc、payment-svc、inventory-svc
```

## 四层栈

| Domain | EntitySet | 角色 |
|---|---|---|
| `product` | `product.journey`、`product.api` | 用户旅程 + 用户侧 API 端点（延迟 SLO） |
| `service` | `service.app` | 后端微服务 |
| `data` | `data.store` | 数据存储（postgres / redis / kafka） |
| `infra` | `infra.node`、`infra.pod` | Kubernetes 节点与 Pod |

每一层都带可观测数据：`product.api.metrics`、`service.app.metrics`（+ `service.app.logs`）、`data.store.metrics`（含关键信号 `connection_pool_usage`）、`infra.node.metrics`。

## 快速启动

```bash
make quickstart QUICKSTART_SAMPLE=examples/service-localization
```

API: `http://localhost:8080` | Web UI: `http://localhost:5173`

加载 4 个域（Product / Service / Data / Infra）、6 个对象类型、23 个实体、29 条关系、4 个指标集、1 个日志集。

仅启动 API（不含 Web UI）：

```bash
go run ./cmd/umodel-server --quickstart --quickstart-sample service-localization
```

## 取数演练

Agent 通过**在每一跳拉取数据**来定位瓶颈。同样的四个动作在每一层重复：*找到实体 → 看它有哪些可观测数据 → 拉取信号 → 走到下一跳。*

### 1. 找到性能退化的入口

```bash
umctl query run demo \
  ".entity with(domain='product', name='product.api', query='degraded') \
  | project display_name, status, sla_tier, latency_slo_ms"
```

预期输出：`checkout-api | degraded | platinum | 300`。

### 2. 发现能拉取它的哪些数据

```bash
# 这个 EntitySet 暴露哪些方法？
umctl query run demo \
  ".entity_set with(domain='service', name='service.app') | entity-call __list_method__()"

# 服务挂了哪些数据集？
umctl query run demo \
  ".entity_set with(domain='service', name='service.app') | entity-call list_data_set(['metric_set','log_set'], true)"
```

### 3. 沿关键路径逐跳下行

`getDirectRelations` 返回一个节点的直接边。在每一跳沿*下游*（`calls` / `reads_writes` / `hosted_on`）边走。

```bash
# checkout-api → order-svc
umctl query run demo \
  ".topo | graph-call getDirectRelations([(:\"product@product.api\" {__entity_id__: '3a44ea48396a812d5a1f4eb12ae51e39'})])"

# order-svc → orders-db（沿 reads_writes 边）
umctl query run demo \
  ".topo | graph-call getDirectRelations([(:\"service@service.app\" {__entity_id__: 'f25ae2923f5df058b6119ea79e434459'})])"
```

### 4. 拉取定位该层的信号

在服务跳，拉取延迟**和**服务自身的 CPU——延迟高但 CPU 健康，说明根因在下游：

```bash
umctl query run demo \
  ".entity_set with(domain='service', name='service.app', ids=['f25ae2923f5df058b6119ea79e434459']) \
  | entity-call get_metrics('service', 'service.app.metrics', 'cpu_usage', step='30s')"
```

在数据存储跳，拉取饱和信号——这就是根因：

```bash
umctl query run demo \
  ".entity_set with(domain='data', name='data.store', ids=['60794de7878447582b1a4d5fe11e37a0']) \
  | entity-call get_metrics('data', 'data.store.metrics', 'connection_pool_usage', step='30s')"
```

计划会渲染出 `max(data_store_connection_pool_in_use{target_id="60794de7…"}) / max(data_store_connection_pool_max{…})`——对象图把"order-svc 依赖的那个存储"翻译成精确的饱和查询，无需手写 PromQL。Agent 客户端加 `?format=agent` 可拿到紧凑的 v1.1 信封（见 [Plan Schema v1](../../docs/zh/spec/plan-schema-v1.md)）。

### 5. 排除下面一层

```bash
# orders-db → node-a；再检查节点是否健康（排除基础设施）
umctl query run demo \
  ".topo | graph-call getDirectRelations([(:\"data@data.store\" {__entity_id__: '60794de7878447582b1a4d5fe11e37a0'})])"

umctl query run demo \
  ".entity_set with(domain='infra', name='infra.node', ids=['6cec8a5bb33ae85cefde09a76ebeca4c']) \
  | entity-call get_metrics('infra', 'infra.node.metrics', 'cpu_usage', step='30s')"
```

**结论：** 延迟在 `order-svc` 处出现，但无法用它自身的 CPU 解释；它的下游 `orders-db` 连接池饱和，而托管它的节点健康 → **瓶颈定位到数据存储连接池**。

### 一键回放整个演练

服务运行中时（`make quickstart QUICKSTART_SAMPLE=examples/service-localization`），回放完整的定位叙事——每跳的 SPL、结果与 Agent 推理：

```bash
./examples/service-localization/demo.sh
```

同一条路径在 CI 中由 `TestServiceLocalizationPath`（`internal/bootstrap/localization_test.go`）守护，因此 Demo 不会悄悄失效；MCP 驱动的 `test-integration.sh` 也覆盖该路径。

## Agent 集成

连接 MCP 客户端（见 [Agent 集成指南](../../docs/zh/guides/agent-integration.md)）后提问：

> "checkout-api 延迟 SLO 违约了——帮我定位瓶颈。"

Agent 在每一跳执行同样的四个动作，沿 product → service → data → infra 下行，把延迟归因到第一个"下游依赖饱和、但自身资源健康"的跳。

### MCP 连接

```json
{
  "mcpServers": {
    "umodel": {
      "command": "go",
      "args": [
        "run", "./cmd/umodel-mcp",
        "--quickstart",
        "--quickstart-sample", "service-localization",
        "--graphstore", "memory"
      ]
    }
  }
}
```

## 目录结构

| 区域 | 路径 | 数量 | 用途 |
|------|------|-----:|------|
| 产品实体集 | `product/entity_set/` | 2 | 旅程 + API |
| 服务实体集 | `service/entity_set/` | 1 | 微服务 |
| 数据实体集 | `data/entity_set/` | 1 | 数据存储 |
| 基础设施实体集 | `infra/entity_set/` | 2 | 节点 + Pod |
| 域内关系 | `*/link/entity_set_link/` | 3 | journey→api、svc→svc、pod→node |
| 跨域关系 | `cross-domain/link/entity_set_link/` | 4 | api→svc、svc→store、svc→pod、store→node |
| 指标集 | `*/metric_set/` | 4 | API / 服务 / 存储 / 节点黄金指标 |
| 日志集 | `service/log_set/` | 1 | 服务应用日志 |
| 存储 | `*/storage/` | 2 | Prometheus（指标）+ Elasticsearch（日志） |
| 数据链接 | `*/link/data_link/` | 5 | 实体集 → 其数据集 |
| 存储链接 | `*/link/storage_link/` | 5 | 数据集 → 存储 |
| 运行时实体 | `sample-data/entities.json` | 23 | 实体数据 |
| 运行时关系 | `sample-data/relations.json` | 29 | 拓扑数据 |
| 清单 | `sample-data/manifest.json` | — | 场景元数据、种子实体、计数 |

## 设计说明

- **纵向，而非横向。** incident-investigation 是横着走（上游调用方 + 业务）；这个是沿请求栈往下走（app → svc → data → infra）。
- **取数是主角。** 每一跳都拉取一个指标；故事讲的是"拉什么、怎么拉"，这正是 Agent 必须学会的。
- **只产出计划。** UModel 开源版返回查询*计划*，由 executor（如 umodel-assistant）针对真实存储执行。瓶颈藏在实体 `status` 与埋好的拓扑里，因此定位路径完全可离线复现。
- **逐跳遍历。** 每跳用 `getDirectRelations` 而非一次性深度 `getNeighborNodes`，更贴近 Agent 真实的定位方式——根据每跳所见决定下一步走向。
