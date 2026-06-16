# Quickstart demo 栈

English: [README.md](README.md)

拉起 UModel（载入 `multi-domain-quickstart` pack）加一个已灌数的 Prometheus 和 Elasticsearch，使 pack 的 `get_metrics` / `get_logs` plan 能对真实后端执行。用带 [`umodel-query`](../../../skills/umodel-query) skill 的 Agent 接入，或直接跑 [`verify.sh`](verify.sh)。

## 前置要求

Docker 或 Podman,带 Compose。Elasticsearch 需要给引擎约 2 GB 内存。

## 启动

```bash
sh examples/quickstart-multidomain/deploy/start.sh
```

`start.sh` 调 `docker compose`（或 `podman compose`）up，等 Elasticsearch 灌数和 Prometheus 首批抓取就绪，并打印地址。它会拉起:

| 服务 | URL | 角色 |
|---|---|---|
| UModel | `http://localhost:8080` | 对象图 + plan provider（`demo` workspace） |
| Prometheus | `http://localhost:9090` | 指标后端,由 exporter 灌入 |
| Elasticsearch | `http://localhost:9200` | 日志后端,启动时灌入 |
| exporter | 内部 | 产出 Prometheus 抓取的指标序列 |

灌入的数据:`checkout-service` 处于 degraded——错误率约 15%、p95 偏高,并伴随 ERROR 日志（超时、503、重试预算耗尽）；其余服务健康。遥测均为合成数据,按 pack 的查询塑形。

## 读数

pack 的 storage endpoint 已指向 `http://localhost:9090` / `http://localhost:9200`，所以 `get_metrics` / `get_logs` 的 plan 按返回的内容直接执行——无需改 endpoint。

用 [`umodel-query`](../../../skills/umodel-query) skill,把 Agent 指向 `http://localhost:8080`（`UMCTL_ADDR`，或 MCP 目标），用自然语言提问,例如"读 checkout-service 的请求速率、错误率、p95 延迟,以及最近的 ERROR 日志"。

不接 Agent:

```bash
sh examples/quickstart-multidomain/deploy/verify.sh
```

`verify.sh` 列出服务、对每个指标 plan 的 PromQL 打到 `:9090`、对日志 plan 的 `_search` 打到 `:9200`。

## 关停

```bash
sh examples/quickstart-multidomain/deploy/stop.sh          # 停止并删除容器、网络、卷
sh examples/quickstart-multidomain/deploy/stop.sh --all    # 连构建的镜像也删
```

## 说明

- 遥测均为合成数据——是 demo,不是生产数据。
- `devops.event.deployment` 建模在 MySQL 上、可通过 `list_data_set` 发现,但可执行的 plan 方法是 `get_metrics`（Prometheus）和 `get_logs`（Elasticsearch）；本栈只灌 Prometheus 和 Elasticsearch。
