# 用 AI Agent 读取对象图

让 AI Agent 指向对象图，由**模型来界定读取范围**——发现实体集、遍历拓扑、并通过模型生成的查询计划拉取遥测。不要让 Agent 手写 SPL、PromQL 或日志 DSL；模型已经携带了它需要的结构。

<video controls preload="metadata" style="width: 100%" src="https://unifiedmodel-assets.oss-cn-hangzhou.aliyuncs.com/QuickStart.mp4"></video>

*一段 90 秒的演示，跑在 `quickstart-multidomain` 包上：Agent 安装 `umodel-query` 技能，找到 `demo` workspace，读取 `devops.service`，并拉取指标和日志——全程不手写一条查询。*

## 为什么

对象图把 Agent 否则只能靠猜的东西编码了下来：有哪些实体集、它们如何跨域关联、一个实体挂了哪些数据集、以及如何把实体变成一条具体的遥测查询。当 Agent *经由*模型读取——`.umodel` → `.entity` → `.topo` → `get_metrics` / `get_logs`——模型会把实体 id 和存储后端绑定进返回的查询计划里。手写的 PromQL 或索引名一旦拓扑或存储配置变化就会漂移；模型界定的读取不会。

## 流程

这个示例跑在 [`quickstart-multidomain`](../../../examples/quickstart-multidomain)（默认 `make quickstart` 包——五个域、35 个对象类型）。每步读取就是一条 `umctl` 命令；同样的 SPL 也能经 MCP 的 `query_spl_execute` 运行。

**1. 发现模型。** `domain` + `name` 这一对是其余每步读取的参数。

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
```

**2. 读取并搜索实体。** `devops.service` 有四个服务；`checkout-service` 处于 `degraded`（id `10000000000000000000000000000101`，后面复用）。

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__, display_name, status" -o json
```

**3. 遍历跨域拓扑。** 找到该服务 `runs` 在什么上面——另一个域里的一个 k8s workload。用 `where` 过滤 graph-call 输出，不要用 `with`。

```bash
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"devops@devops.service\" {__entity_id__:'10000000000000000000000000000101'})]) | where __relation_type__ = 'runs'" -o json
```

**4. 找到实体上的数据集。** 直接从实体本身读取数据集的 `domain` + `name`——不要去 `.umodel` 里扫。

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call list_data_set(['metric_set','log_set','event_set'], true)" -o json
```

**5. 以计划的形式拉取遥测。** `get_metrics` 返回一个 `prometheus_promql` 计划，服务 id 已经绑好；由调用方执行。

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops','devops.metric.service','request_count', step='30s')" -o json
```

## 配合 `umodel-query` 技能

[`umodel-query`](../../../skills/umodel-query) 技能用自然语言跑完全相同的流程。拉起带遥测后端的 stack（`sh examples/quickstart-multidomain/deploy/start.sh`），把技能指向 `http://localhost:8080`，然后问：

> “读一下 checkout-service 的请求速率、错误率、p95 延迟，以及最近的 ERROR 日志。”

Agent 会发现该服务、走到它的数据集、并执行返回的指标和日志计划——不用手敲 PromQL 或索引名。[`umodel-rca`](../../../skills/umodel-rca) 在此之上再加根因分析。

## 该做 / 不该做

| 该做 | 不该做 |
|---|---|
| 让 `get_metrics` / `get_logs` 返回查询计划（实体 id + 存储已绑定）。 | 手写 PromQL 或 Elasticsearch 索引名。 |
| 通过 `list_data_set` 从实体读取数据集的 `domain` + `name`。 | 去 `.umodel` 里扫来猜数据集名。 |
| 用 `where __relation_type__ = '…'` 过滤 graph-call 输出。 | 指望 `with(...)` 子句能过滤 graph-call 输出。 |
| 用 `.entity … with(ids=[…])` 把拓扑行（实体 id）解析成名称。 | 把 graph-call 行里的裸 id 当作展示名。 |

> 内存模式（`make quickstart`）支持模型、实体、搜索和拓扑读取；`get_metrics` / `get_logs` 返回的计划没有可执行的后端。需要端到端执行时，用 `deploy/start.sh` 跑起 seeded Prometheus / Elasticsearch。

## 相关文档

- [多域 quickstart 包](../../../examples/quickstart-multidomain)
- [Query Service](/zh/guides/query-service)
- [Agent 集成](/zh/guides/agent-integration)
- [UModel Agent 技能](../../../skills/README.zh-CN.md)
