# 多域 Quickstart — 智能读数 demo

English: [Multi-Domain Quickstart Example Pack](README.md)

`examples/quickstart-multidomain` 是 `make quickstart` 默认导入的样例，也是**智能读数**的展示场景：一个 workspace、**5 个域、35 种对象类型**，通过同一套 SPL 入口端到端读取——发现模型、跨域读对象与拓扑、语义检索、把遥测变成可执行的查询 plan。它与 [`umodel-query`](../../skills/umodel-query) skill 配套，让 Agent 自主完成这套走查。

五个域：**devops**（服务、部署、流水线、SLO、故障…）、**k8s**（粗粒度集群/命名空间/工作负载/Pod），以及三个企业场景——**automaker**、**game**、**supplier**。`devops.service` 关联一个 metric / log / event 数据集，各自连到对应 Storage（Prometheus / Elasticsearch / MySQL），因此数据集发现是基于真实数据的。

## 快速开始

```bash
make quickstart
```

API：`http://localhost:8080` | Web UI：`http://localhost:5173`。把 5 个域、35 个 entity set、93 个实体、125 条关系载入 `demo` workspace（内存，无需密钥）。

> 仅 API（不起 Web UI）：
> `go run ./cmd/umodel-server --quickstart --quickstart-sample multi-domain-quickstart --graphstore memory`

## 智能读数走查

每次读取都是一条 `umctl` 命令——务必带 `-o json`（行在 `data.data`，列名在 `data.header`）。同样的 SPL 也可走 MCP 的 `query_spl_execute` 工具。[`umodel-query`](../../skills/umodel-query) skill 会自主执行这些；这里手动走一遍。

### 1. 发现模型

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
```

→ `devops`、`k8s`、`automaker`、`game`、`supplier` 五个域共 35 种对象类型。`.umodel` 是地图；这里看到的 `domain` + `name` 就是后面每个读取要传的参数。

### 2. 读对象

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service')" -o json   # 全字段
umctl query run demo ".entity with(domain='devops', name='devops.service') | project __entity_id__, display_name, status, owner" -o json
```

→ `checkout-service`（degraded）、`catalog-api`（active）、`delivery-service`（warning）、`telemetry-collector`（active）。`__entity_id__`（checkout-service = `10000000000000000000000000000101`）是后续复用的句柄。`| project` 等 pipe 是可选的——裸读返回全部字段。

### 3. 检索

普通全文——填任意文本即可跨所有字段匹配：

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__, display_name, status" -o json
# → checkout-service | degraded
```

需要按语义排序时加 `mode='vector'`（或 `mode='hyper'` 混合）和 `topk=N`——`query='payment checkout'` 的首位命中是 `checkout-service`（"Converts carts into paid orders"）。语义检索结果以**完整行**按相似度返回（`| project` 简写适用于普通读取）。

### 4. 遍历拓扑（含跨域）

```bash
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"devops@devops.service\" {__entity_id__:'10000000000000000000000000000101'})]) | where __relation_type__ = 'runs'" -o json
```

→ `checkout-service --runs--> ` 一个 **k8s** 工作负载：一条跨域边（devops → k8s）。该节点还有 `deploys`、`measured_by`、`runs_in`、`impacts`、`contains` 等关系。按关系过滤用 `where __relation_type__='…'`（`with(...)` 子句**不会**过滤 graph-call 输出）。行里是实体 **ID**——用 `.entity … with(ids=[…])` 还原名字。

### 5. 发现对象的方法与数据集

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()" -o json
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call list_data_set(['metric_set','log_set','event_set'], true)" -o json
```

→ 方法：`__list_method__`、`list_data_set`、`get_metrics`、`get_logs`。服务上的数据集：`devops.metric.service`（**Prometheus**）、`devops.log.service`（**Elasticsearch**）、`devops.event.deployment`（**MySQL**，表 `deployment_events`）。一个对象、三种存储后端，都在同一套模型之下。

### 6. 读指标 → plan → 执行

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops','devops.metric.service','request_count', step='30s')" -o json
```

→ 一个 `prometheus_promql` plan，携带
`sum(rate(devops_service_request_total{service_id="10000000000000000000000000000101"}[1m]))`，
打到 `http://prometheus.devops.example:9090`——service id 已替换好，无需手写 PromQL。把 endpoint 改成你的 Prometheus 再执行（见 skill 的 [metrics-logs 指南](../../skills/umodel-query/references/metrics-logs.md)）。

### 7. 读日志 → plan → 执行

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops','devops.log.service', query='level = \"ERROR\"')" -o json
```

→ 一个 `elasticsearch_dsl` plan：对索引 `devops-service-logs-*`、`https://elasticsearch.devops.example:9200` 的 bool/filter 查询。同样改成你的 ES 再执行。

> `devops.event.deployment` event_set 建模在 **MySQL** 上（第 5 步 / `.umodel` 可发现），展示一套模型覆盖三种后端。目前可执行的 plan 方法是 `get_metrics`（Prometheus）和 `get_logs`（Elasticsearch）。

## 给 Agent

[`umodel-query`](../../skills/umodel-query) skill 教 Agent 自主完成以上全部——发现模型、跨域读取、检索、把 `get_metrics` / `get_logs` 的 plan 变成真实数值。加载它（再加 [`umodel-rca`](../../skills/umodel-rca) 做根因分析），通过 MCP 或 CLI 接入，用自然语言提问即可。

## 内容

| 区域 | 路径 | 数量 | 作用 |
|---|---|---:|---|
| DevOps EntitySet | `devops/entity_set/` | 10 | 团队、服务、仓库、流水线、环境、部署、发布、变更、故障、SLO。 |
| Kubernetes EntitySet | `k8s/entity_set/` | 7 | 粗粒度集群、命名空间、工作负载、Pod、节点、Service、Ingress。 |
| 企业 demo EntitySet | `automaker/entity_set/`, `game/entity_set/`, `supplier/entity_set/` | 18 | 复用的企业实体拓扑定义。 |
| EntitySetLink | `*/link/entity_set_link/`, `cross-domain/link/entity_set_link/` | 42 | 域内和跨域拓扑语义。 |
| DevOps DataSet | `devops/metric_set/`, `devops/log_set/`, `devops/event_set/` | 3 | 用于 EntitySet DataSet 发现的最小服务指标、日志和部署事件。 |
| DataLink 和 StorageLink | `devops/link/data_link/`, `devops/link/storage_link/` | 6 | 连接 `devops.service` 到 DataSet，并连接 DataSet 到 Storage。 |
| Storage 定义 | `devops/storage/` | 3 | Prometheus、Elasticsearch 和 MySQL 查询规划元数据。 |
| Runtime entities | `sample-data/entities.json` | 93 | CMS 2.0 兼容实体 payload。 |
| Runtime relations | `sample-data/relations.json` | 125 | CMS 2.0 兼容拓扑 payload。 |

## 手动导入

导入到其他 workspace（quickstart 服务会自动导入）：

```bash
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
```

## 维护规则

- 保持模型 YAML、entity payload、relation payload 和文档一致。
- 保持 DevOps 可观测链路足够小：一个服务 `metric_set`、一个服务 `log_set`、一个服务 `event_set`，以及对应的 `data_link` / `storage_link`。
- quickstart 里的 k8s 保持粗粒度。
- DataSet/Storage 定义保持为 quickstart discovery 专用的最小版本。
- 修改后运行 `make example-validate` 和 `go test ./internal/sampledata ./internal/bootstrap ./internal/query`。
