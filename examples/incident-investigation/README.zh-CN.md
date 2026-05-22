# 故障排查 Demo

基于场景驱动的示例，展示 UModel 对象图 + Runbook 如何支撑 AI Agent 故障排查。一次由上游重试风暴引发的支付网关 SLO 违规，需要跨域拓扑遍历和 Runbook 引导诊断才能定位根因。

```
payment-gateway (degraded, platinum SLO)
  ← calls ← checkout-service
               ← affects ← cfg-checkout-retry (max_retries 2→5, 24h前)
                              ← triggers ← 618 Flash Sale (3.5x traffic)

排除: payment-gw v3.2.1 (12h前, trivial logging change)
根因: 4000 × 3.5 × 2.5 = 35,000 QPS → 8.75x 过载
```

## 场景

> **凌晨 02:17 — payment-gateway P99 延迟突破 SLO。**
> 值班 SRE（或 AI Agent）需要找到根因。

### 时间线

| 时间 | 事件 |
|------|------|
| T-48h | 促销活动 "618 Flash Sale" 创建，status=scheduled |
| T-24h | 配置变更 `cfg-checkout-retry` 生效：max_retries 2→5, timeout 500→2000ms |
| T-12h | 部署 `payment-gw v3.2.1` 上线（轻微变更：日志格式调整） |
| T-4h | 促销活动激活，流量开始爬升（3.5 倍放大） |
| T-7min | 事件 `INC-0042` 创建：payment-gateway P99 > 2000ms |
| T-0 | **排查开始** |

### 根因

上游重试放大 × 促销流量 = 级联过载。

```
有效负载 = 基础QPS × 促销倍数 × (新重试次数 / 旧重试次数)
        = 4000 × 3.5 × (5/2)
        = 35,000 QPS（8.75 倍正常容量）
```

### 红鲱鱼

`payment-gw v3.2.1` 在 12 小时前部署。值班人员的第一直觉是怀疑它——但其 `change_summary` 显示仅为日志格式变更。Runbook 的 `recent_deployment_correlation` 观察帮助快速排除。

## 前置要求

- Go 1.22+
- Make
- Node.js 22+（Web UI）

## 快速启动

```bash
make quickstart QUICKSTART_SAMPLE=examples/incident-investigation
```

API: `http://localhost:8080` | Web UI: `http://localhost:5173`

加载 3 个域（Platform / Runtime / Business）、11 个对象类型、65 个实体、83 条关系、1 个 Runbook。

仅启动 API（不含 Web UI）：

```bash
go run ./cmd/umodel-server --quickstart --sample examples/incident-investigation
```

Docker：

```bash
docker build -t umodel-demo .
docker run -p 8080:8080 -p 5173:5173 \
  -e QUICKSTART_SAMPLE=examples/incident-investigation \
  umodel-demo
```

## Runbook 能力总览

`platform.service.ops` 为 AI Agent 提供结构化诊断协议：

| 类型 | 名称 | 用途 |
|------|------|------|
| Observation | upstream_retry_amplification | 通过拓扑 + config_change 关联检测重试风暴 |
| Observation | recent_deployment_correlation | 排除或确认近期部署（LLM 判断 change_summary） |
| Observation | business_traffic_pressure | 识别促销驱动的流量放大 |
| Toolkit | config_management | rollback_config_change, apply_rate_limit |
| Toolkit | k8s_operations | scale_workload, restart_pods |
| Knowledge | retry_storm_pattern | 故障模式解释 + 计算公式 |
| Knowledge | deployment_triage_guide | 如何避免误判部署为根因 |
| Automation | slo_breach_context_collector | SLO 违规时自动收集上下文 |
| Skill | incident-investigation | AI Agent 完整排查协议 |

## 排查演练

> **提示：** `.entity` 查询中 `query=` 参数对实体所有字段执行全文检索——只需包含目标文字即可匹配。

### 第 1 步：找到故障服务

```bash
umctl query run demo \
  ".entity with(domain='platform', name='platform.service', query='degraded') \
  | project display_name, status, owner, sla_tier"
```

预期输出：`payment-gateway | degraded | payments-backend | platinum`

### 第 2 步：查看上游调用方（拓扑查询）

```bash
umctl query run demo \
  ".topo | graph-call getNeighborNodes('full', 1, \
  [(:\"platform@platform.service\" {__entity_id__: '63718b78868895d2590551b27ec6f51c'})]) \
  | with(__relation_type__='calls')"
```

预期输出：发现 `checkout-service` 和 `order-service` 调用了 payment-gateway。

### 第 3 步：检查上游的配置变更

```bash
umctl query run demo \
  ".entity with(domain='platform', name='platform.config_change', query='checkout') \
  | project display_name, change_detail, applied_at"
```

预期输出：发现 `checkout-retry-increase` — max_retries 2→5，24 小时前生效。

### 第 4 步：排除最近部署（红鲱鱼）

```bash
umctl query run demo \
  ".entity with(domain='platform', name='platform.deployment', query='payment') \
  | project display_name, change_summary, deployed_at"
```

预期输出：`payment-gw v3.2.1 | Minor: updated logging format | 12h前` — 排除。

### 第 5 步：确认流量放大因子（跨域查询）

```bash
umctl query run demo \
  ".entity with(domain='business', name='business.promotion', query='active') \
  | project display_name, traffic_multiplier, expected_peak_qps, actual_peak_qps"
```

预期输出：`618 Flash Sale | 3.5 | 12000 | 38000` — 实际流量远超预期。

### 第 6 步：评估业务影响（跨域查询）

```bash
umctl query run demo \
  ".entity with(domain='business', name='business.order_flow', query='impacted') \
  | project display_name, error_rate"
```

预期输出：Standard Purchase Flow (3.2%) 和 Subscription Renewal (1.8%) 受影响。

### 第 7 步：加载 Runbook

```bash
umctl query run demo ".umodel with(kind='runbook_set', name='platform.service.ops')"
```

## Agent 集成

连接 MCP 客户端后，输入：

> "payment-gateway SLO 告警了，帮我排查。"

Agent 按 Runbook 协议执行：

1. **定位服务** — `.entity` 查询找到 `payment-gateway` (status=degraded, sla_tier=platinum)
2. **加载 Runbook** — 通过 `runbook_link` 发现 `platform.service.ops`
3. **执行 Observation #1** (upstream_retry_amplification)
   - 查拓扑找到上游 `checkout-service`
   - 查 config_change 发现 `cfg-checkout-retry` (max_retries 2→5)
   - 结论匹配：**Retry Storm Detected** (severity=error)
4. **执行 Observation #2** (recent_deployment_correlation)
   - 查 deployment 发现 `payment-gw v3.2.1`
   - LLM 判断 change_summary 为 trivial
   - 结论匹配：**Deployment Ruled Out** (severity=info)
5. **执行 Observation #3** (business_traffic_pressure)
   - 跨域查询发现 `618 Flash Sale` (actual 38000 > expected 12000)
   - 结论匹配：**Promotion Traffic Exceeds Capacity** (severity=error)
6. **关联分析** — 计算放大因子：3.5 × (5/2) = 8.75x
7. **加载 Knowledge** — `retry_storm_pattern` 确认模式匹配
8. **推荐 Tool** — `rollback_config_change` (risk=medium, confirmation_required=true)

### Agent 输出示例

```
## 诊断结果

基于 Runbook platform.service.ops 的系统化排查：

| Observation | 结论 | Severity |
|-------------|------|----------|
| upstream_retry_amplification | Retry Storm Detected | error |
| recent_deployment_correlation | Deployment Ruled Out | info |
| business_traffic_pressure | Promotion Traffic Exceeds Capacity | error |

根因：checkout-service 配置变更 (retry 2→5) × 618 促销 (3.5x) = 8.75x 过载

推荐操作：
  Tool: rollback_config_change
  Input: { config_change_id: "cfg-checkout-retry", target_service: "svc-checkout" }
  Risk: medium | 需要确认: 是 | 预计生效: 2-3 分钟
```

## MCP 连接

### 本地（stdio）

在项目 `.mcp.json` 中添加：

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

启动服务：

```bash
go run ./cmd/umodel-mcp --quickstart \
  --quickstart-sample examples/incident-investigation \
  --graphstore file.memory \
  --transport http --addr 0.0.0.0:8090
```

客户端 `.mcp.json` 配置：

```json
{
  "mcpServers": {
    "umodel": {
      "type": "streamable-http",
      "url": "http://<remote-host>:8090/mcp"
    }
  }
}
```

详细的远程部署指南（SSH 隧道、Nginx 代理、Docker、故障排查）参见 [远程 MCP 连接操作手册](../../docs/zh/guides/remote-mcp.md)。

## 目录结构

| 区域 | 路径 | 数量 | 用途 |
|------|------|-----:|------|
| 平台实体集 | `platform/entity_set/` | 5 | 服务、部署、配置变更、事件、团队 |
| 平台关系 | `platform/link/entity_set_link/` | 5 | 域内拓扑（calls, targets, owns, impacts, affects） |
| 运行时实体集 | `runtime/entity_set/` | 4 | 集群、命名空间、工作负载、Pod |
| 运行时关系 | `runtime/link/entity_set_link/` | 3 | 包含层次（contains, schedules） |
| 业务实体集 | `business/entity_set/` | 2 | 促销活动、订单流程 |
| 跨域关系 | `cross-domain/link/entity_set_link/` | 3 | Platform-Runtime, Platform-Business 拓扑 |
| Runbook 集 | `platform/runbook_set/` | 1 | 服务运维手册 |
| Runbook 链接 | `platform/link/runbook_link/` | 1 | 将 platform.service 链接到运维手册 |
| 运行时实体 | `sample-data/entities.json` | 65 | 实体数据 |
| 运行时关系 | `sample-data/relations.json` | 83 | 拓扑数据 |
| 清单 | `sample-data/manifest.json` | — | 样例元数据、种子实体、场景描述 |

## 扩展此 Demo

**添加新的 EntitySet**（例如 `platform.alert`）：
1. 创建 `platform/entity_set/platform.alert.yaml`，参照现有文件格式
2. 在 `sample-data/entities.json` 中添加实体实例
3. 更新 `manifest.json` 中的计数

**添加新的关系**（例如 alert → service）：
1. 创建 `platform/link/entity_set_link/platform.alert_fires_on_platform.service.yaml`
2. 在 `sample-data/relations.json` 中添加关系记录
3. 更新 `manifest.json` 中的计数

**添加新的 Observation 到 Runbook**：
1. 编辑 `platform/runbook_set/platform.service.ops.yaml`
2. 在 `observations[]` 中新增条目：name、description、priority、steps、conclusions
3. 如果观察需要新的实体类型，同时添加对应的 sample data

**添加跨域关系**：
1. 在 `cross-domain/link/entity_set_link/` 中创建文件
2. 命名约定：`{源域}.{源类型}_{动词}_{目标域}.{目标类型}.yaml`

## 设计原则

- **场景驱动**：不是"看数据"，而是"解一个谜"
- **三层紧密耦合**：Business → Platform → Runtime 形成自然调查路径
- **32 位 hex 实体 ID**：CMS 2.0 格式要求；`display_name` 字段提供人类可读名称
- **Runbook 引导**：Agent 按结构化观察执行，而非自由推理
- **包含红鲱鱼**：测试排查流程是否正确排除无关部署
- **跨域价值**："谁受影响"需要 Business 层
- **Manifest 作为测试锚点**：`sample-data/manifest.json` 包含 `seed_entities` 供程序化验证
