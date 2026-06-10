# 变更日志

English version: [CHANGELOG.md](CHANGELOG.md)

所有值得关注的 UModel Open Source 变更都应记录在这里。

在稳定版本发布前，项目使用简单的变更日志结构：

- `Added`：新增能力。
- `Changed`：行为变化。
- `Fixed`：缺陷修复。
- `Deprecated`：即将移除的行为。
- `Removed`：已移除的行为。
- `Security`：安全修复。

## 0.2.0 - Unreleased

### Added

- **Query Service 引入 plan/data 模式协议**（#25）。新增 `mode` 请求字段和 `?mode=` query 参数，让客户端在"返回查询计划"和"执行返回数据"之间选择。开源面定位为 plan provider，`mode=data` 返回结构化的 `NOT_IMPLEMENTED` 错误，错误体里带 `migration_*` 信息便于 agent 直接消费而非解析自然语言。新增 `GET /api/v1/capabilities` 端点声明支持的 modes 与 formats。
- **Plan JSON v1 契约**（#25）。顶层 envelope 含 `mode`、`version`、`operation`、`description`、`next_action`、`source_query`、`data_source`、`params_echo`、`query`、`time_range`。规范文档落地为 `docs/{en,zh}/spec/plan-schema-v1.md`,作为发布阻断级契约。
- **Plan schema v1.1 agent 友好 envelope**(#26)。新增 `?format=agent` 让 plan 对象直接作为 HTTP body 返回（不再走 assistant envelope 与字符串编码），`data_source.*` 字段折叠成紧凑的 `{ref, kind}` 引用，agent 上下文窗口消耗显著降低。`?include=spec` 还原 `storage.config`、`data_link.spec`、`storage_link.spec`，便于调试与诊断 agent。
- **`get_metrics` / `get_logs` 查询计划与实体过滤**（#18、#23）。两个方法都生成结构化 query plan，渲染 PromQL / Elasticsearch DSL，并把 entity ID 与 label 过滤翻译进存储侧查询。
- **`get_metrics` / `get_logs` 方法签名对齐**（#25）。parser 接受新的可选参数：`aggregate`（metrics）以及 `storage_domain` / `storage_name` / `storage_kind`（两者都加）。开源 planner 通过 `params_echo` 回显，供下游 executor 消费。
- **EntitySet 数据集发现**（#17）。新增 `list_data_set` entity-call，列出相关的 `metric_set` / `log_set` / `trace_set` / `event_set`，可按需展开详情。
- **多 domain quickstart 样例包扩展**。新增 devops 域的 `log_set` / `metric_set` / `event_set`，配套 storage、links、样例数据与测试。
- **Web UI 改进**。基于 UModel URL 的 workspace 路由（#15）、查询页 example picker 与结果表优化（#21）、Explorer / Entity-Topo / Query 全面国际化、Monaco 编辑器预加载、API Debugger 面板、Imports 功能、Settings 优化。

### Changed

- `docs/{en,zh}/README.md` 文档入口新增 **Specifications** 段落，指向 `plan-schema-v1.md`。
- Plan 输出的 `params_echo` 剔除 nil 与空字符串；方法签名的默认值不会被回显，除非调用方真的传了。

### Fixed

- 本地 Ladybug provider 在服务重启后可能丢失 workspace 元数据(#22),恢复路径现在从 data root 读取。
- Vite 相关的 Dependabot 提醒解决（#16）。

### Security

- 重申：MCP 写工具默认关闭。安全报告策略不变。

## 0.1.0 - 2026-05-28

### Added

- 本地单进程 UModel 服务。
- Workspace 元数据管理。
- UModel 导入、校验、写入、删除和索引路径。
- CMS 2.0 兼容的实体与关系写入/过期路径。
- 面向 `.umodel`、`.entity`、`.topo` 的统一 Query Service。
- AgentGateway discovery、安全查询工具、resources 和 MCP stdio server。
- `umctl` CLI，覆盖 workspace、UModel、EntityStore、topology、query 和 agent 工作流。
- `memory`、`file.memory` 和可选 `local.ladybug` GraphStore providers。
- React/Vite OpenUModel Web UI。
- REST OpenAPI 和 MCP tool/resource schemas。
- 生成的 Go、Python、Java model SDK 资产。
- APM common example pack 和 sample import endpoint。
- Architecture guard、contract tests、integration tests、e2e tests 和 golden tests。

### Changed

- 开源文档采用面向外部开发者的 README 和结构化 docs index。
- Docker 和 Compose 默认显式使用 `file.memory`。

### Security

- MCP 写工具默认关闭。
- 增加安全策略和私下报告指引。
