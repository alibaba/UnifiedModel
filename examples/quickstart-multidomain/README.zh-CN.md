# 多域 Quickstart 示例包

English: [Multi-Domain Quickstart Example Pack](README.md)

`examples/quickstart-multidomain` 是 `make quickstart` 默认导入的样例。它展示一个 workspace 如何把 DevOps 归属、粗粒度 Kubernetes 运行拓扑，以及 `Demo/umodel-demo-2/umodel` 中的企业场景域连接起来。

Kubernetes 域刻意保持粗粒度：只建模能解释服务拓扑的运行时对象，不照搬完整 Kubernetes 对象模型。

这个样例覆盖实体拓扑，并包含一条最小 DevOps 可观测链路。`devops.service` 关联一个 `metric_set`、一个 `log_set` 和一个 `event_set`，每个 DataSet 都关联到对应 Storage，用于基于真实 UModel 数据验证 EntitySet 的 DataSet 发现能力。

## 内容

| 区域 | 路径 | 数量 | 作用 |
|---|---|---:|---|
| DevOps EntitySet | `devops/entity_set/` | 10 | 团队、服务、仓库、流水线、环境、部署、发布、变更、故障、SLO。 |
| Kubernetes EntitySet | `k8s/entity_set/` | 7 | 粗粒度集群、命名空间、工作负载、Pod、节点、Service、Ingress。 |
| 企业 demo EntitySet | `automaker/entity_set/`, `game/entity_set/`, `supplier/entity_set/` | 18 | 直接复用 `Demo/umodel-demo-2/umodel` 的实体拓扑定义。 |
| EntitySetLink | `*/link/entity_set_link/`, `cross-domain/link/entity_set_link/` | 42 | 域内和跨域拓扑语义。 |
| DevOps DataSet | `devops/metric_set/`, `devops/log_set/`, `devops/event_set/` | 3 | 用于 EntitySet DataSet 发现的最小服务指标、日志和部署事件。 |
| DataLink 和 StorageLink | `devops/link/data_link/`, `devops/link/storage_link/` | 6 | 连接 `devops.service` 到 DataSet，并连接 DataSet 到 Storage。 |
| Storage 定义 | `devops/storage/` | 3 | Prometheus、Elasticsearch 和 MySQL 查询规划元数据。 |
| Runtime entities | `sample-data/entities.json` | 93 | CMS 2.0 兼容实体 payload。 |
| Runtime relations | `sample-data/relations.json` | 125 | CMS 2.0 兼容拓扑 payload。 |

## 导入

启动 quickstart：

```bash
make quickstart
```

手动导入到其他 workspace：

```bash
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
```

## 查询示例

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel with(kind='entity_set') | project domain,name,kind | sort domain,name | limit 20"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__,display_name,status,owner | limit 10"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')"

go run ./cmd/umctl --addr http://localhost:8080 query run demo ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20"
```

## 维护规则

- 保持模型 YAML、entity payload、relation payload 和文档一致。
- 保持 DevOps 可观测链路足够小：一个服务 `metric_set`、一个服务 `log_set`、一个服务 `event_set`，以及对应的 `data_link` / `storage_link`。
- quickstart 里的 k8s 保持粗粒度，避免变成完整 Kubernetes 规范。
- 只从 `Demo/umodel-demo-2/umodel` 复用 `entity_set` 和 `entity_set_link`；DataSet/Storage 定义保持为 quickstart discovery 专用的最小版本。
- 修改后运行 `make example-validate` 和 `go test ./internal/sampledata ./internal/bootstrap ./internal/query`。
