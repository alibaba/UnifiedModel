# 故障排查 demo 栈

English: [README.md](README.md)

拉起 UModel(载入 `incident-investigation` pack)加一个已灌数的 Prometheus 和 Elasticsearch,数据与建模的故障一致——大促期间 checkout 重试风暴导致 payment-gateway SLO 击穿。用带 [`umodel-query`](../../../skills/umodel-query) + [`umodel-rca`](../../../skills/umodel-rca) skill 的 Agent 接入做一次现场根因分析,或直接跑 [`verify.sh`](verify.sh)。

## 前置要求

Docker 或 Podman,带 Compose。Elasticsearch 需要给引擎约 2 GB 内存。

demo 占用宿主端口 `8080`(UModel)、`9090`(Prometheus)、`9200`(Elasticsearch)。`start.sh` 会先做端口预检:任一被占用就直接报错退出——避免它在"localhost:端口其实是别的服务"时误报 ready。要与已存在的 umodel-server / Prometheus / Elasticsearch 共存,改用空闲端口:

```bash
UMODEL_PORT=18080 PROM_PORT=19090 ES_PORT=19200 sh examples/incident-investigation/deploy/start.sh
```

## 启动

```bash
sh examples/incident-investigation/deploy/start.sh
```

`start.sh` 调 `docker compose`(或 `podman compose`)up,等指标 backfill、Elasticsearch 灌数和 Prometheus 首批抓取就绪,并打印地址。它会拉起:

| 服务 | URL | 角色 |
|---|---|---|
| UModel | `http://localhost:8080` | 对象图 + plan provider(`demo` workspace) |
| Prometheus | `http://localhost:9090` | ~72h backfill 历史 + 实时尾 |
| Elasticsearch | `http://localhost:9200` | ~72h 日志 |
| exporter | 内部 | 产出 Prometheus 抓取的实时 `platform_service_*` 序列 |
| metrics-gen | 内部(一次性) | 生成 ~72h 历史,Prometheus 启动前由 `promtool` backfill |
| es-seed | 内部(一次性) | 生成并 bulk 写入 ~72h 日志 |

### 遥测覆盖整个故障窗口

灌入的遥测覆盖完整 ~72h 故障弧线(而非仅当前快照),沿 pack 的[时间线](../README.md):

| 相位 | 窗口 | 数据呈现 |
|---|---|---|
| healthy | T-72h … T-24h | 全平台正常 |
| retries-up | T-24h … T-4h | `max_retries` 2→5 配置变更使 `checkout-service` 客户端重试率 8% → 55%;T-12h 的 `payment-gw v3.2.1` 部署**无任何指标痕迹** |
| breach | T-4h … now | 促销激活(3.5×)→ 重试风暴:`payment-gateway` p99≈2000ms、错误率约 14.8%、上游超时率高;`payment-router` 与 支付宝/微信/银联 通道又慢又报错 |

所以 instant 查询看到当前击穿、range 查询看到整条弧线——重试率在配置变更时刻拐头、延迟与错误在大促时刻拐头、而部署因曲线平直被排除。`verify.sh` 两者都会打印。

## 跑 RCA

pack 的 storage endpoint 已指向 `http://localhost:9090` / `http://localhost:9200`,所以 `get_metrics` / `get_logs` 的 plan 按返回直接执行。把带 `umodel-query` + `umodel-rca` skill 的 Agent 指向 `http://localhost:8080`(`UMCTL_ADDR`,或 MCP 目标),提问:

> payment-gateway 劣化了,找根因。

Agent 会用真实遥测刻画症状(`get_metrics latency_p99_ms` / `error_rate`、`get_logs level="ERROR"`),沿拓扑回溯到上游 `checkout-service`、它的 `checkout-retry-policy-v2` 配置变更和正在进行的 促销,排除 `payment-gw v3.2.1` 部署(只是日志变更),最终定位到重试放大风暴。

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
- 一切都相对"现在",所以 demo 永不过期。指标历史相对 now 生成,在 Prometheus 启动前用 `promtool tsdb create-blocks-from openmetrics` 载入,exporter 随后实时续上;日志相对 now 生成;实体时间线(部署 / 配置变更 / 故障 / 促销时间戳)由 demo 镜像入口脚本在启动时统一位移到 now —— 配置 ~now-24h、部署 ~now-12h、促销激活 ~now-4h、故障 ~now —— 与遥测一致。
- 日志包含**跨服务关联 trace**:一次失败的 checkout 用同一个 `trace_id` 串起 `checkout-service → payment-gateway → payment-router → channel → provider`,可顺着一条请求下钻、看到超时源自下游并上溯为重试耗尽;另含配置变更、部署、熔断等地标事件。
- pack 还建模了 MySQL 部署事件集和一个 runbook;这里灌的可执行 plan 方法是 `get_metrics`(Prometheus)和 `get_logs`(Elasticsearch)。
