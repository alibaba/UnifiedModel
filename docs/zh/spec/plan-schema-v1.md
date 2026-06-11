# Plan Schema v1

English: [Plan Schema v1](../../en/spec/plan-schema-v1.md)

> 规范状态：**v1**，强制约束。
> 范围：unified-model（开源）与 umodel-assistant（商业版）。
> Owner：UModel maintainers。Breaking change 必须升至新的 major 版本。

本文档定义 `unified-model` 返回的 **plan** 模式与 `umodel-assistant` 支持的 **plan / data** 模式之间的共享契约。两端必须同时遵守；任一方破坏该契约都视为 P0 回归。

## 为什么需要这份规范

UModel 由两个协同的服务面组成：

- **unified-model**（开源）是 *plan provider*。接收一条 SPL 查询，解析涉及到的 EntitySet、DataLink、Storage、StorageLink，返回一个 **查询计划** —— 描述下游 executor 需要执行什么的序列化结构。
- **umodel-assistant**（商业 PaaS）是 *plan executor*。它在 plan 之上能进一步针对真实存储（SLS、Prometheus、Elasticsearch 等）执行查询，返回时序数据、日志行等具体数据。

要让用户从开源迁移到 PaaS 时不改 SPL、不改客户端代码，两端必须共享：

1. 相同的方法签名（parser 接受的参数集合）。
2. 相同的 plan JSON 形态（executor 消费的契约）。
3. 相同的 mode 协议（客户端如何区分要 plan 还是 data）。

Plan Schema v1 固定这个契约。

## 1. Mode 协议

### 请求

客户端通过 HTTP `?mode=` query 参数或请求 body 指定期望的模式：

```http
POST /api/v1/query/{workspace}/execute?mode=plan
Content-Type: application/json

{ "query": ".entity_set with(...) | entity-call get_metrics(...)", "mode": "plan" }
```

当 body 字段与 query 参数同时存在时，**body 优先**。两者都缺省时，服务端按自己的 `default_mode` 兜底。

### 支持值

| Mode    | 含义                              | unified-model | umodel-assistant |
|---------|-----------------------------------|---------------|------------------|
| `plan`  | 返回查询计划，不执行              | 支持          | 支持             |
| `data`  | 执行计划，返回真实数据            | 拒绝          | 支持             |

unified-model 收到 `mode=data` 时返回 HTTP 4xx + 错误码 `NOT_IMPLEMENTED`。错误结构 `details` 里必须带结构化迁移信息，便于 AI agent 直接消费而非解析自然语言：

| Key                   | 示例                                                                     |
|-----------------------|--------------------------------------------------------------------------|
| `requested_mode`      | `"data"`                                                                 |
| `supported_modes`     | `"plan"`                                                                 |
| `migration_service`   | `"umodel-assistant"`                                                     |
| `migration_action`    | `"switch_endpoint_to_umodel_assistant"`                                  |
| `migration_docs_url`  | 本规范或 umodel-assistant 迁移指南的 URL                                 |

### 能力发现

服务端在 `GET /api/v1/capabilities` 暴露能力：

```json
{
  "service": "unified-model",
  "version": "<服务版本>",
  "modes_supported": ["plan"],
  "default_mode": "plan"
}
```

SDK / CLI 启动时应当调用一次 `/api/v1/capabilities` 并据此调整默认行为。硬编码 mode 假设视为不合规。

## 2. Plan JSON v1

Plan 响应被包装在标准的 assistant query 信封里：

```json
{
  "responseType": 1,
  "query": "<JSON 编码后的 plan>",
  "header": [],
  "data": []
}
```

`query` 字段是一个 JSON 编码字符串。反序列化后必须符合以下结构：

```jsonc
{
  "mode": "plan",                     // 模式判别字段；plan 模式下永远是 "plan"
  "version": "v1",                    // schema 版本，遵循 SemVer
  "operation": "get_metrics",         // entity-call 方法的规范名
  "description": "Retrieve metric \"request_count\" from MetricSet devops/devops.metric.service with step 30s (storage: prometheus/devops.prometheus.core). Forward this plan to a UModel data executor (e.g. umodel-assistant) to fetch real time series.",
  "next_action": "forward_to_executor", // 给 agent 的下一步建议
  "source_query": ".entity_set with(...) | entity-call get_metrics(...)", // 原始 SPL 回显
  "data_source": {
    "data_set":    { "domain", "kind", "name" },
    "storage":     { "domain", "type", "name", "config" },
    "data_link":   { "domain", "name", "spec" },
    "storage_link":{ "domain", "name", "spec" }
  },
  "params_echo": {                    // 调用方实际传入的 entity-call 参数
    "metric": "request_count",        // nil 与空字符串被剔除
    "step":   "30s",
    "aggregate": false                // executor 在这里恢复完整调用上下文
  },
  "query": { /* 与方法相关的存储侧查询 */ },
  "time_range": {                     // 仅当请求设置了时间范围时出现
    "from": "...",
    "to":   "..."
  }
}
```

### 顶层字段

| 字段           | 类型   | 必填 | 备注                                            |
|----------------|--------|------|-------------------------------------------------|
| `mode`         | string | 是   | 永远 `"plan"`，镜像请求 mode。                  |
| `version`     | string | 是   | 当前规范为 `"v1"`，遵循 SemVer。                |
| `operation`    | string | 是   | entity-call 规范名（`get_metrics` 等）。        |
| `description`  | string | 是   | 一句话描述 plan 做什么，便于 agent 复述。       |
| `next_action`  | string | 是   | 给 agent 的下一步建议。当前固定 `"forward_to_executor"`。|
| `source_query` | string | 是   | 调用方提交的原始 SPL 回显。                     |
| `data_source`  | object | 是   | 解析后的 DataSet / Storage / DataLink / StorageLink。 |
| `params_echo`  | object | 是   | 调用方实际传入的参数，剔除 nil 与空字符串。     |
| `query`        | object | 是   | 存储侧可执行的具体查询。                        |
| `time_range`   | object | 否   | 请求带时间范围时出现。                          |

### Agent 友好字段

`description` / `next_action` / `source_query` 这三个字段是给 AI Agent 准备的，避免 agent 自己反向解析存储侧 query 或者推断用户意图：

- **`description`** —— 一句话总结，agent 可以直接复述给用户。包含 metric / log set、过滤条件（用 `[...]` 包裹）以及存储信息。
- **`next_action`** —— agent 用来判别下一步动作的字段。`"forward_to_executor"` 表示：不要自己执行存储侧 query，把 plan 转发给 UModel data executor（如 umodel-assistant）。后续可能加入 `"render_to_user"`、`"prompt_for_consent"` 等。
- **`source_query`** —— 调用方提交的原始 SPL。多 agent 协作场景里，下游 agent 没有用户输入的上下文时靠这个字段恢复。

### `data_source` 子结构

| 字段                 | 类型    | 备注                                              |
|----------------------|---------|---------------------------------------------------|
| `data_set.domain`    | string  | 所属 domain。                                     |
| `data_set.kind`      | string  | `metric_set` / `log_set` 等。                     |
| `data_set.name`      | string  | DataSet 名称。                                    |
| `storage.domain`     | string  | Storage 元素 domain。                             |
| `storage.type`       | string  | Storage 种类（`prometheus` / `elasticsearch` 等）。|
| `storage.name`       | string  | Storage 元素名称。                                |
| `storage.config`     | object  | Storage 元素 spec，对 executor 框架不透明。       |
| `data_link.spec`     | object  | 完整 DataLink spec，含 `fields_mapping`。         |
| `storage_link.spec`  | object  | 完整 StorageLink spec，含 `fields_mapping`。      |

### `params_echo` 语义

`params_echo` 必须包含调用方实际传入的每个 entity-call 参数，原生 JSON 类型保留（boolean 保留 boolean、number 保留 number、string 保留 string）。空字符串与 `null` 值必须剔除，避免 executor 把"未设置"误当作"显式空值"。方法签名里的默认值不会被回显，除非调用方真的传了。

## 3. 方法签名契约

`get_metrics` 与 `get_logs` 声明两端任意一方都可能接受的全部参数。开源 planner 只消费其中一部分；PaaS executor 全部消费。两端必须通过 `__list_method__` 暴露同一组参数。

### `get_metrics`

| Key              | 类型      | 必填 | OSS 消费 | PaaS 消费 | 默认值   |
|------------------|-----------|------|----------|-----------|----------|
| `domain`         | varchar   | 是   | ✓        | ✓         |          |
| `name`           | varchar   | 是   | ✓        | ✓         |          |
| `metric`         | varchar   | 否   | ✓        | ✓         |          |
| `query`          | varchar   | 否   | ✓        | ✓         |          |
| `query_type`     | varchar   | 否   | ✓        | ✓         |          |
| `step`           | varchar   | 否   | ✓        | ✓         |          |
| `aggregate`      | boolean   | 否   | 仅回显   | ✓         | `true`   |
| `storage_domain` | varchar   | 否   | 仅回显   | ✓         |          |
| `storage_name`   | varchar   | 否   | 仅回显   | ✓         |          |
| `storage_kind`   | varchar   | 否   | 仅回显   | ✓         |          |

### `get_logs`

| Key              | 类型    | 必填 | OSS 消费 | PaaS 消费 |
|------------------|---------|------|----------|-----------|
| `domain`         | varchar | 是   | ✓        | ✓         |
| `name`           | varchar | 是   | ✓        | ✓         |
| `query`          | varchar | 否   | ✓        | ✓         |
| `storage_domain` | varchar | 否   | 仅回显   | ✓         |
| `storage_name`   | varchar | 否   | 仅回显   | ✓         |
| `storage_kind`   | varchar | 否   | 仅回显   | ✓         |

开源 parser 必须接受这两张表里列出的全部参数。"仅回显"指 planner 把值记录到 `params_echo`，但不影响 plan 内容生成。

### 别名

两端都同时支持规范名与别名。unified-model 在 parse 阶段把 `get_log` 归一化为 `get_logs`、`get_metric` 归一化为 `get_metrics`。umodel-assistant 以单数为规范名，复数作为别名。无论写哪种拼法，行为必须一致。

## 4. 兼容性规则

契约遵循 SemVer：

- **v1 内的 minor bump**：可加顶层字段、可加方法参数、可拓宽取值类型。现有 consumer 必须仍然可用。
- **major bump（v2+）**：任何破坏性变更——重命名/删除顶层字段、删除方法参数、改字段类型、改必填/选填标记。

unified-model（开源）不得引入 umodel-assistant 没有的方法、参数或 plan 字段。PaaS 是 source of truth，开源面是其子集。

任一方提议升级到 v2 必须先发 RFC、提供迁移窗口，并在新版本发布的同时更新本文档。

## 5. 非目标

本规范**不**约定：

- umodel-assistant 在 `data` 模式下返回的具体行结构（rows、labels、采样数组）。那些是 PaaS 内部契约；此处只约束 plan→data 的转换边界。
- 存储 executor 的实现细节（PromQL 如何派发、ES DSL 如何拼装）。Plan 已含足够元数据，"怎么执行"由实现自定。
- 把 workspace / 实体从开源拷贝到 PaaS 的迁移工具，由本规范以外的工作跟进。

## 6. 强制约束

两端必须维护契约测试：

- 解码 unified-model 输出的 plan，校验所有 v1 必填字段齐全。
- 校验 umodel-assistant 的 plan parser 能原样接受 unified-model 的 plan 输出。
- 校验两端 `__list_method__` 报出的参数符合 §3。

该测试集任意失败均阻断两边 repo 的 merge。

## 7. v1.1 —— Agent envelope

状态：**与 v1 共存**，opt-in 启用。默认经典 envelope（responseType=1 信封 + `NewQueryExecuteResponse` 矩阵包装）行为完全不变。

### 触发

客户端通过 `format` 请求字段或 `?format=` query 参数 opt-in：

```http
POST /api/v1/query/{workspace}/execute?format=agent
Content-Type: application/json

{ "query": ".entity_set with(...) | entity-call get_metrics(...)" }
```

支持值：`""`（FormatAssistant，默认）和 `"agent"`（FormatAgent）。其它值返回 `INVALID_ARGUMENT`。服务端通过 `/api/v1/capabilities` 暴露支持集合：

```json
{ "service": "unified-model", "modes_supported": ["plan"], "default_mode": "plan",
  "formats_supported": ["", "agent"], "default_format": "" }
```

### 与 v1 经典 envelope 的差异

`format=agent` 生效时，响应在四个方面与经典 envelope 不同：

| 方面 | v1 经典 | v1.1 agent |
|---|---|---|
| HTTP body | `{code, data: {data: [[1, "<JSON 字符串>", [], []]], header, ...}, message, success}` | plan **对象**直接作为 body —— 无信封、无字符串编码 |
| plan 内顶层 `version` 字段 | `"v1"` | `"v1.1"` |
| `data_source.{data_set, storage, data_link, storage_link}` | 完整 `{domain, kind/type, name, spec/config}` 对象 | 紧凑的 `{ref: "domain/name", kind/type}` 引用；默认省略 `spec` / `config` |
| `__list_method__` 与方法签名契约（§3）发现 | 不变 | 不变 |

### 紧凑引用形态

agent envelope 中 `data_source.*` 各元素的形态：

```jsonc
{ "ref": "<domain>/<name>", "kind": "metric_set" }    // data_set
{ "ref": "<domain>/<name>", "type": "prometheus" }    // storage 用 "type"
{ "ref": "<domain>/<name>", "kind": "data_link" }     // data_link
{ "ref": "<domain>/<name>", "kind": "storage_link" }  // storage_link
```

`ref` 为 `"<domain>/<name>"`，作为稳定标识符，agent 可以在后续调用里传回或展示给用户。`kind`（storage 用 `type`）说明元素种类，agent 无需外部上下文就知道自己看的是什么。

### opt-in 还原完整 spec

调试 / 诊断 / 迁移类 agent 如果确实需要完整 Storage config 或 Link spec，可以加 `?include=spec`（或 body 里 `"include_spec": true`）。设置后 agent envelope 额外回填：

- `data_source.storage.config` —— 完整 Storage 元素 spec
- `data_source.data_link.spec` —— 完整 DataLink spec
- `data_source.storage_link.spec` —— 完整 StorageLink spec

v1.1 阶段 `?include=spec` 是 all-or-none。如未来出现细粒度需要（例如 `?include=data_link.spec,storage.config`），可在 v1.2 引入。

### 非 plan 请求在 `format=agent` 下的行为

`format=agent` 只影响产生 plan 的 entity-call 方法：当前是 `get_metrics` 与 `get_logs`。其它请求（`.umodel` / `.entity` / `.topo` rows、`__list_method__`、`list_data_set`、错误响应）**fallback 到经典 envelope**，而不是报错。让 client 可以把 `format=agent` 当作 session 级默认，不需要按请求类型分发。

### `format=agent` 下的错误

错误继续使用标准 `{error: {code, message, retryable, details}}` 结构，不受 `format` 影响——错误响应已经是结构化、agent 友好的形态。

### 兼容性

v1.1 是 v1 大版本下的 SemVer minor bump。agent envelope 纯 opt-in；不传 `format=agent` 的消费者看到的就是和之前完全一致的 v1 envelope。两种版本都是头等公民，必须长期共存，直到 v2 破坏性变更经 RFC 决议后才会改变。

### 最小 `curl` 对比

```bash
# 经典 v1 envelope（默认）
curl -s -X POST "http://localhost:8080/api/v1/query/demo/execute" \
  -H 'Content-Type: application/json' \
  -d '{"query":".entity_set with(domain=\"devops\", name=\"devops.service\", ids=[\"...\"]) | entity-call get_metrics(\"devops\", \"devops.metric.service\", \"request_count\", step=\"30s\")"}'
# → {"code":"200","data":{"data":[[1,"{\"mode\":\"plan\",\"version\":\"v1\",...}",[],[]]],...},"message":"successful","success":true}

# Agent v1.1 envelope，折叠引用
curl -s -X POST "http://localhost:8080/api/v1/query/demo/execute?format=agent" \
  -H 'Content-Type: application/json' \
  -d '{"query":"..."}'
# → {"mode":"plan","version":"v1.1","operation":"get_metrics","description":"...",
#    "data_source":{"storage":{"ref":"devops/devops.prometheus.core","type":"prometheus"},...},
#    "params_echo":{...},"query":{...}}

# Agent v1.1 envelope，完整 spec 展开
curl -s -X POST "http://localhost:8080/api/v1/query/demo/execute?format=agent&include=spec" \
  -H 'Content-Type: application/json' \
  -d '{"query":"..."}'
# → 同上，再加 data_source.storage.config / data_link.spec / storage_link.spec
```

## 相关文档

- [Query Service Guide](../guides/query-service.md)
- [GraphStore Providers](../graphstore-providers.md)
- [公共 Go 模型](../../../pkg/model/types.go)
- [稳定错误码](../../../pkg/errors/errors.go)
