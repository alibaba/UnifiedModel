# Quickstart demo 栈 — 从 Prometheus + Elasticsearch 真实读数

English: [README.md](README.md)

一条命令拉起完整的**智能读数** demo：**UModel**（加载 `multi-domain-quickstart` pack）+ 真实
**Prometheus** + 真实 **Elasticsearch**，并灌入与 pack 的 `get_metrics` / `get_logs` plan 匹配的数据。
然后你用自己的 Agent（Qoder / Codex / Claude Code）装上 [`umodel-query`](../../skills/umodel-query)
skill 接上来，端到端地读对象图和遥测。

## 前置要求

- Docker + Docker Compose（Elasticsearch 需要给 Docker 约 2 GB 内存）。
- 一个装了 `umodel-query` skill 的 Agent，且 `umctl` 在 PATH（或有 Go 工具链）。见
  [skill 安装](../../skills/umodel-query/SKILL.md)。

## 1. 一键启动

```bash
docker compose -f deployments/quickstart-demo/docker-compose.yml up
```

等到 `es-seed` 服务打印 `seed complete`（首次还会拉镜像、启动 ES，约一两分钟）。此时在跑：

| 服务 | URL | 角色 |
|---|---|---|
| UModel | `http://localhost:8080` | 对象图 + plan provider（`demo` workspace） |
| Prometheus | `http://localhost:9090` | 真实指标后端（由 exporter 灌入） |
| Elasticsearch | `http://localhost:9200` | 真实日志后端（已灌入服务日志） |
| exporter | （内部） | 产出 Prometheus 抓取的 demo 指标序列 |

灌入的数据讲了一个故事：**checkout-service**（`…0101`）处于 degraded——错误率高（约 15%）、
p95 延迟高，并伴随 ERROR 日志（超时、503、重试预算耗尽）；catalog-api / delivery-service /
telemetry-collector 健康。查询前给 Prometheus 约 1 分钟抓取时间，`rate()` 才有样本。

## 2. 接入你的 Agent

装上 skill 并把 Agent 指向 UModel（skill 以 CLI 为先，MCP 同样可用）：

```bash
# Claude Code
/plugin marketplace add alibaba/UnifiedModel && /plugin install umodel@unifiedmodel
export UMCTL_ADDR=http://localhost:8080      # 让 umctl 连到 demo 的 UModel
```

Qoder / Codex：同样安装/指向 skill，把 UModel 地址设为 `http://localhost:8080`。

## 3. 读数（plan → 执行）

`get_metrics` / `get_logs` 返回一个可执行 **plan**。其 endpoint 是模型里的占位符
（`prometheus.devops.example:9090`、`https://elasticsearch.devops.example:9200`）——Agent
**用本栈的真实地址覆盖它**：

- Prometheus → **`http://localhost:9090`**（纯 HTTP；demo 无需 tenant/auth）
- Elasticsearch → **`http://localhost:9200`**（纯 HTTP；demo 关闭了安全）

这正是 [`umodel-query` skill](../../skills/umodel-query/references/metrics-logs.md) 教的
"读 plan、改 endpoint、执行" 流程。

**让 Agent 做**，例如：

> "列出 devops 的服务和状态，然后读 checkout-service 的请求速率、错误率、p95 延迟，并给出最近的
> ERROR 日志。它为什么 degraded？"

Agent 会发现模型、定位 checkout-service（`…0101`）、从 UModel 取 `get_metrics` / `get_logs`
plan、对 `localhost:9090` / `localhost:9200` 执行，并报告偏高的错误率 + 延迟 + 超时/503 的 ERROR 行。

## 冒烟测试（可选，无需 Agent）

`verify.sh` 手动跑同一条链路（需要 `jq` + `umctl`/Go）：

```bash
sh deployments/quickstart-demo/verify.sh
```

它列出服务、取每个指标的 plan 并把 PromQL 打到 `:9090`、取日志 plan 并把 `_search` 打到
`:9200`——打印 checkout-service 的请求/错误速率、p95、以及 ERROR 日志行。

## 关停

```bash
docker compose -f deployments/quickstart-demo/docker-compose.yml down -v
```

## 说明

- 这里所有遥测都是**合成数据**，为匹配 pack 的查询而生成——是 demo，不是真实生产数据。
- `multi-domain-quickstart` pack 的 storage endpoint 是有意的占位符；本栈不改 pack——由 Agent
  把 plan 指向本地后端（设计如此）。
- pack 还建模了一个 **MySQL** event_set（`devops.event.deployment`）；它可通过 `list_data_set`
  发现，但可执行的取数方法是 `get_metrics`（Prometheus）和 `get_logs`（Elasticsearch）。
