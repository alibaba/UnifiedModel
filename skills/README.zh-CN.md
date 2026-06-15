# UModel Agent 技能

可加载的技能（Skill），让 AI Agent 使用 UModel——读取实体、关系和模型本身，
拉取遥测数据，并在对象图上做模型引导的根因分析——通过 `umctl` CLI 或 MCP。

这里的*技能*是一个自包含的 `SKILL.md`（YAML frontmatter `name` + `description`，
随后是指令），格式与 Claude Code、Cursor、Qoder、Codex 等支持技能的 Agent 运行时一致。

> **第一次用？从[快速上手](QUICKSTART.zh-CN.md)开始** —— 安装技能、载入一个 demo
> 对象图（实体 + 关系）、接入 Claude Code / Qoder / Codex、然后提问。几分钟端到端跑通，无需密钥。

## 可用技能

| 技能 | 路径 | 用途 |
|---|---|---|
| `umodel-query` | [`umodel-query/SKILL.md`](umodel-query/SKILL.md) | 读取实体 / 关系 / 拓扑数据**以及** UModel 模型（实体集、数据集、链接、Runbook）。CLI 优先（`umctl`），并提供 MCP 替代方案。 |
| `umodel-rca` | [`umodel-rca/SKILL.md`](umodel-rca/SKILL.md) | 在对象图上做模型引导的**自主根因分析**：取对的遥测、跨域遍历关系、推理到根因。构建于 `umodel-query` 之上。 |

## 前置要求

一个 Agent 可访问的 UModel 服务。最快路径是用内置 demo workspace：

```bash
make quickstart QUICKSTART_SAMPLE=examples/incident-investigation   # 服务于 http://localhost:8080
```

Agent 随后通过任一传输读取：

- **CLI**（首选，设置最少）：`umctl query run <workspace> "<SPL>" -o json`
- **MCP**：连接 `umodel-mcp`，调用 `query_spl_execute` 工具

demo 无需任何密钥或网络。

## 使用技能

### 方式 A —— Claude Code 插件市场（一条命令）

在 Claude Code 里，直接从本仓库把两个技能作为插件装上：

```
/plugin marketplace add alibaba/UnifiedModel
/plugin install umodel@unifiedmodel
```

这会装上 `umodel` 插件——含 `umodel-query` 与 `umodel-rca`——随后按你的提问自动激活。
之后用 `/plugin marketplace update unifiedmodel` 更新。

### 方式 B —— 拷贝到 Agent 的技能目录

多数支持技能的 Agent 从一个目录发现技能——把两个技能目录拷进你的 Agent 扫描的位置：

| Agent | 技能目录 |
|---|---|
| Claude Code | `.claude/skills/` |
| Cursor | `.cursor/skills/` |
| Qoder | `.qoder/skills/` |
| Codex | `.agents/skills/`（或 `~/.agents/skills/` 做用户级全局） |

```bash
# Qoder
mkdir -p .qoder/skills  && cp -R skills/umodel-query skills/umodel-rca .qoder/skills/
# Codex —— 中立的 .agents/skills/ Qoder 也读
mkdir -p .agents/skills && cp -R skills/umodel-query skills/umodel-rca .agents/skills/
```

然后正常提问——例如*"查一下这个 workspace 里 degraded 的服务"*（激活 `umodel-query`），
或*"payment-gateway 的 SLO 告警了，帮我排查"*（激活 `umodel-rca`）。每个技能的
`description` 决定 Agent 何时激活它；手动触发用 `/umodel-query`（Claude Code / Qoder）
或 `$umodel-query`（Codex）。

## 两个技能的关系

它们对应 Agent 用 UModel 做的三件事：

- **`umodel-query`** 覆盖读取——(1) 实体与关系/拓扑数据（`.entity` / `.topo`），
  (2) 模型本身（`.umodel` + `__list_method__` / `list_data_set`）。开源返回真实数据行；
  对接 PaaS 端点返回 PaaS API 的数据。
- **`umodel-rca`** 增加 (3) 模型引导取数（`get_metrics` / `get_logs`，开源*计划* /
  PaaS *数据*）和自主根因循环。它复用 `umodel-query` 的读取，调查时两个一起加载。

## 编写新技能

新增目录 `skills/<name>/`，放一个 `SKILL.md`：

```markdown
---
name: <name>
description: >-
  一两句话说明技能做什么、Agent 何时该用它。包含触发短语——这是 Agent 匹配的依据。
---

# <标题>

命令式指令：如何连接、工具面、方法、worked example 和注意事项。
```

技能尽量保持传输无关（CLI 或 MCP 用同一套 SPL），并优先用真实、验证过的命令，而非空想。

## 相关文档

- [Agent 集成指南](../docs/zh/guides/agent-integration.md) —— `umodel` 技能所基于的完整人面向走查。
- [MCP 参考](../docs/zh/reference/mcp.md) —— 传输、tools、resources。
- [故障排查 Demo](../examples/incident-investigation/README.zh-CN.md) —— `umodel` 技能验证所用的 worked example / 试验台。
