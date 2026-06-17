# 故障排查 demo 栈

English: [README.md](README.md)

拉起 UModel(载入 `incident-investigation` pack)加一个已灌数的 Prometheus 和 Elasticsearch,数据与建模的故障一致——618 大促期间 checkout 重试风暴导致 payment-gateway SLO 击穿。用带 [`umodel-query`](../../../skills/umodel-query) + [`umodel-rca`](../../../skills/umodel-rca) skill 的 Agent 接入做一次现场根因分析,或直接跑 [`verify.sh`](verify.sh)。

## 前置要求

Docker 或 Podman,带 Compose。Elasticsearch 需要给引擎约 2 GB 内存。

## 启动

```bash
sh examples/incident-investigation/deploy/start.sh
```

`start.sh` 调 `docker compose`(或 `podman compose`)up,等 Elasticsearch 灌数和 Prometheus 首批抓取就绪,并打印地址。它会拉起:

| 服务 | URL | 角色 |
|---|---|---|
| UModel | `http://localhost:8080` | 对象图 + plan provider(`demo` workspace) |
| Prometheus | `http://localhost:9090` | 灌好的指标,由 exporter 喂入 |
| Elasticsearch | `http://localhost:9200` | 灌好的日志 |
| exporter | 内部 | 产出 Prometheus 抓取的 `platform_service_*` 序列 |

灌入的遥测与图一致:`payment-gateway` p99≈2150ms、错误率约 14.8%、上游超时率高;`checkout-service` 客户端重试率约 55%(`max_retries` 2→5 的配置变更);`payment-router` 与 支付宝/微信/银联 通道又慢又报错;其余健康。

## 跑 RCA

pack 的 storage endpoint 已指向 `http://localhost:9090` / `http://localhost:9200`,所以 `get_metrics` / `get_logs` 的 plan 按返回直接执行。把带 `umodel-query` + `umodel-rca` skill 的 Agent 指向 `http://localhost:8080`(`UMCTL_ADDR`,或 MCP 目标),提问:

> payment-gateway 劣化了,找根因。

Agent 会用真实遥测刻画症状(`get_metrics latency_p99_ms` / `error_rate`、`get_logs level="ERROR"`),沿拓扑回溯到上游 `checkout-service`、它的 `checkout-retry-policy-v2` 配置变更和正在进行的 618 促销,排除 `payment-gw v3.2.1` 部署(只是日志变更),最终定位到重试放大风暴。

不接 Agent:

```bash
sh examples/incident-investigation/deploy/verify.sh
```

## 关停

```bash
sh examples/incident-investigation/deploy/stop.sh          # 停止并删除容器、网络、卷
sh examples/incident-investigation/deploy/stop.sh --all    # 连构建的镜像也删
```

## 说明

- 遥测均为合成数据,按建模的故障塑形——是 demo,不是生产数据。
- pack 还建模了 MySQL 部署事件集和一个 runbook;这里灌的可执行 plan 方法是 `get_metrics`(Prometheus)和 `get_logs`(Elasticsearch)。
