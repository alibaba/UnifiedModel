# 多域 Quickstart

English: [Multi-domain quickstart](README.md)

一个跨域、端到端读取对象图的样例——一个 workspace、五个域、35 种对象类型——通过 `.umodel` / `.entity` / `.topo` 这套 SPL 入口完成:发现模型、读实体与拓扑、检索、遥测查询规划。它同时是仓库默认的 `make quickstart` sample。[`umodel-query`](../../skills/umodel-query) skill 用同一条路径从 Agent 侧驱动。

五个域:**devops**(服务、部署、流水线、SLO、故障…)、**k8s**(集群、命名空间、工作负载、Pod),以及三个企业场景——**automaker**、**game**、**supplier**。`devops.service` 关联一个 metric / log / event 数据集，各自连到对应 Storage（Prometheus、Elasticsearch、MySQL）。

## 启动

```bash
sh examples/quickstart-multidomain/deploy/start.sh
```

拉起 UModel（载入本 pack）加一个已灌数的 Prometheus 和 Elasticsearch（docker 或 podman），下面每一步——直到 `get_metrics` / `get_logs` 的 plan——都能端到端跑通。UModel `http://localhost:8080`、Prometheus `:9090`、Elasticsearch `:9200`。见 [deploy/](deploy/)。

本 pack 同时是仓库默认的 `make quickstart` sample：`make quickstart` 把模型和样例数据载入一个内存服务、不带遥测后端——下面的模型、实体、检索、拓扑读取都可用，但 `get_metrics` / `get_logs` 只会返回无后端可执行的 plan。

## 读取

每次读取都是一条 `umctl` 命令；带 `-o json`（行在 `data.data`，列名在 `data.header`）。同样的 SPL 也可走 MCP 的 `query_spl_execute`。

### 模型

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
```

列出 35 种对象类型。这里的 `domain` + `name` 就是后面每个读取要传的参数。

### 实体

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service')" -o json
umctl query run demo ".entity with(domain='devops', name='devops.service') | project __entity_id__, display_name, status" -o json
```

四个服务：`checkout-service`（degraded）、`catalog-api`（active）、`delivery-service`（warning）、`telemetry-collector`（active）。`checkout-service` 的 `__entity_id__` 是 `10000000000000000000000000000101`，后面复用。`| project` 可选；裸读返回全字段。

### 检索

```bash
umctl query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__, display_name, status" -o json
```

`query=` 是跨所有字段的全文检索。加 `mode='vector'`（或 `mode='hyper'`）和 `topk=N` 按相似度排序；向量结果以完整行按得分返回。

### 拓扑

```bash
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"devops@devops.service\" {__entity_id__:'10000000000000000000000000000101'})]) | where __relation_type__ = 'runs'" -o json
```

返回 `checkout-service` 的 `runs` 边，指向一个 k8s 工作负载——一条跨域关系。用 `where __relation_type__ = '…'` 过滤；`with(...)` 子句不会过滤 graph-call 输出。行里是实体 id，用 `.entity … with(ids=[…])` 还原名字。

### 数据集

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()" -o json
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call list_data_set(['metric_set','log_set','event_set'], true)" -o json
```

方法：`__list_method__`、`list_data_set`、`get_metrics`、`get_logs`。服务的数据集：`devops.metric.service`（Prometheus）、`devops.log.service`（Elasticsearch）、`devops.event.deployment`（MySQL）。`list_data_set` 返回 `get_metrics` / `get_logs` 要用的 `domain` + `name`——从实体取，而不是去扫 `.umodel`。

### 指标

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops','devops.metric.service','request_count', step='30s')" -o json
```

返回一个 `prometheus_promql` plan：`sum(rate(devops_service_request_total{service_id="10000000000000000000000000000101"}[1m]))`，endpoint `http://localhost:9090`，service id 已绑定。开源返回 plan，由调用方执行。`deploy/` 起着时直接跑；否则把 endpoint 改成你的 Prometheus。见 [metrics-logs](../../skills/umodel-query/references/metrics-logs.md)。

### 日志

```bash
umctl query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops','devops.log.service', query='level = \"ERROR\"')" -o json
```

返回一个 `elasticsearch_dsl` plan，查询索引 `devops-service-logs-*`，endpoint `http://localhost:9200`。`devops.event.deployment` 建模在 MySQL 上、可被发现，但可执行的 plan 方法是 `get_metrics`（Prometheus）和 `get_logs`（Elasticsearch）。

## 配合 Agent

[`umodel-query`](../../skills/umodel-query) skill 执行上面这些读取，并执行返回的 plan。`deploy/` 起着时，把它指向 `http://localhost:8080`（`UMCTL_ADDR`，或 MCP 目标），用自然语言提问——例如"读 checkout-service 的请求速率、错误率、p95 延迟，以及最近的 ERROR 日志"。[`umodel-rca`](../../skills/umodel-rca) 在其上做根因分析。安装见 [skills/README.md](../../skills/README.md)。

## 内容

| 区域 | 路径 | 数量 | 作用 |
|---|---|---:|---|
| DevOps EntitySet | `umodel/devops/entity_set/` | 10 | 团队、服务、仓库、流水线、环境、部署、发布、变更、故障、SLO。 |
| Kubernetes EntitySet | `umodel/k8s/entity_set/` | 7 | 粗粒度集群、命名空间、工作负载、Pod、节点、Service、Ingress。 |
| 企业 demo EntitySet | `umodel/automaker/entity_set/`, `umodel/game/entity_set/`, `umodel/supplier/entity_set/` | 18 | 复用的企业实体拓扑定义。 |
| EntitySetLink | `umodel/*/link/entity_set_link/`, `umodel/cross-domain/link/entity_set_link/` | 42 | 域内和跨域拓扑语义。 |
| DevOps DataSet | `umodel/devops/metric_set/`, `umodel/devops/log_set/`, `umodel/devops/event_set/` | 3 | 用于数据集发现的服务指标、日志、部署事件。 |
| DataLink 和 StorageLink | `umodel/devops/link/data_link/`, `umodel/devops/link/storage_link/` | 6 | 连接 `devops.service` 到数据集，数据集到 Storage。 |
| Storage 定义 | `umodel/devops/storage/` | 3 | Prometheus、Elasticsearch、MySQL 查询规划元数据。 |
| 部署栈 | `deploy/` | — | 一键 demo：`docker-compose` + 已灌数的 Prometheus / Elasticsearch + `start.sh` / `verify.sh`。 |
| Runtime entities | `sample-data/entities.json` | 93 | CMS 2.0 兼容实体 payload。 |
| Runtime relations | `sample-data/relations.json` | 125 | CMS 2.0 兼容拓扑 payload。 |

## 导入到其他 workspace

quickstart 服务会自动导入。导入到其他 workspace：

```bash
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
```

## 维护

- 模型 YAML、entity payload、relation payload、文档保持一致。
- DevOps 可观测链路保持最小：一个服务 `metric_set`、一个 `log_set`、一个 `event_set`，及对应 `data_link` / `storage_link`。
- k8s 保持粗粒度。
- 改完本 pack 后运行 `make example-validate` 和 `go test ./internal/sampledata ./internal/bootstrap ./internal/query`。
