# 快速上手 —— 用你的 Agent 跑通 UModel 技能

几分钟内让一个 AI Agent（Claude Code / Qoder / Codex / …）端到端用上 UModel：
**安装技能**、**初始化一个 demo 对象图**（实体 + 关系）、**接入你的 Agent**、**提问**。

demo 数据集是内置的 **incident-investigation** 包——3 个域（business / platform /
runtime）、约 65 个实体、约 83 条关系、1 个 Runbook，以及指标/日志集——也就是两个
技能验证所用的数据。全部本地、内存运行，**无需任何密钥、无需网络**。

## 前置要求

- Go 1.22+ 与 Make
- 本仓库的一份 clone（MCP server 从源码运行）
- 一个支持 MCP 的 Agent：Claude Code、Qoder、Codex、Cursor……

## 1. 安装技能

技能就是 [`skills/`](README.zh-CN.md) 下的 `SKILL.md` 文件。安装方式取决于你的 Agent：

- **Claude Code**（原生技能）—— 拷贝到它扫描的技能目录：
  ```bash
  mkdir -p .claude/skills
  cp -R skills/umodel-query skills/umodel-rca .claude/skills/
  ```
  （或用户级 `~/.claude/skills/`）。
- **Qoder / Codex / 没有 `SKILL.md` 加载器的 Agent** —— 技能就是一段指令，把内容作为
  Agent 上下文带上即可：把 [`skills/umodel-query/SKILL.md`](umodel-query/SKILL.md)
  和 [`skills/umodel-rca/SKILL.md`](umodel-rca/SKILL.md) 引用或粘贴进项目指令
  （如 `AGENTS.md`），或直接附到对话里。

> 技能的 `description` 决定支持技能的 Agent 何时激活它。

## 2. 初始化 demo 数据

对象图（实体 + 关系 + 模型 + Runbook）位于 `examples/incident-investigation/`，**内存**
加载——不落盘、不留痕。

- **若通过 MCP 接入**（最常见）：MCP server 启动时用 `--quickstart-sample` 载入数据——
  由你的 Agent 拉起（见 §3），这一步无需手动操作。
- **若通过 CLI / HTTP 驱动**（或想用 Web UI）：启动 HTTP server：
  ```bash
  make quickstart QUICKSTART_SAMPLE=examples/incident-investigation   # API :8080，Web UI :5173
  ```
  确认数据已载入：
  ```bash
  umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
  umctl query run demo ".entity with(domain='platform', name='platform.service', query='degraded')" -o json
  # → payment-gateway | degraded | …
  ```

## 3. 接入你的 Agent（MCP）

三个客户端用**同一条** MCP server 启动命令——它通过 `--quickstart-sample` 载入 demo
数据并对 Agent 提供服务：

```
command: go
args:    run ./cmd/umodel-mcp --quickstart --quickstart-sample examples/incident-investigation --graphstore memory
```

### Claude Code

项目 `.mcp.json`（或 `claude mcp add`）：

```json
{
  "mcpServers": {
    "umodel": {
      "command": "go",
      "args": ["run", "./cmd/umodel-mcp", "--quickstart",
               "--quickstart-sample", "examples/incident-investigation",
               "--graphstore", "memory"]
    }
  }
}
```

### Qoder

Qoder 设置 → **MCP** → **My Servers** → **+ Add**，粘贴同样的
`{ "mcpServers": { "umodel": { … } } }`，**Save**。MCP 在 **Agent 模式**下生效。
（或用 `qodercli mcp add`。）

### Codex

`~/.codex/config.toml`（或 `codex mcp add`）；会话里用 `/mcp` 验证：

```toml
[mcp_servers.umodel]
command = "go"
args = ["run", "./cmd/umodel-mcp", "--quickstart",
        "--quickstart-sample", "examples/incident-investigation",
        "--graphstore", "memory"]
```

> 从仓库根目录启动（`./cmd/umodel-mcp` 是相对路径），或用绝对 `go` 路径 / 预编译的
> `umodel-mcp` 二进制。远程 server 用 `--transport http --addr 0.0.0.0:8090` 启动，
> 客户端指向 `http://<host>:8090/mcp`。

### 没有 MCP？用 CLI

技能也能走 CLI——启动 server（§2），Agent 执行 `umctl query run demo "<SPL>" -o json`。

## 4. 提问

数据载入、Agent 接好后，直接用自然语言问。

**读取类 —— 激活 `umodel-query`：**

- "列一下这个 workspace 里的服务和它们的状态。"
- "payment-gateway 依赖了什么？把拓扑给我看看。"
- "payment-gateway 挂了哪些指标集和日志集？"

**根因分析 —— 激活 `umodel-rca`：**

- "payment-gateway 的 SLO 告警了，帮我排查。" / "Investigate why payment-gateway is degraded."

Agent 随后自主工作：定位 degraded 服务 → 拉它的指标/日志 → 顺拓扑找到上游调用方 →
发现重试配置变更 → 排除红鲱鱼部署 → 找到促销流量 → 得出根因（重试 ×2.5 × 促销 ×3.5
= **8.75×** 过载）并建议回滚。

## 排错

- **Agent 里看不到 UModel 工具** → 先手动跑 `go run ./cmd/umodel-mcp …`，应能正常启动；
  确认处于 Agent 模式（Qoder）/ 用 `/mcp` 确认已连接（Codex）。
- **结果为空** → 数据是按 server 进程驻留在内存的；确认该 server 启动时带了
  `--quickstart-sample examples/incident-investigation`。
- **Agent 找不到 `go`** → 用绝对 `go` 路径，或先 `go build -o umodel-mcp ./cmd/umodel-mcp`
  再把 `command` 指向该二进制。

## 相关文档

- [Agent 技能目录](README.zh-CN.md)
- [Agent 集成指南](../docs/zh/guides/agent-integration.md)
- [故障排查 Demo](../examples/incident-investigation/README.zh-CN.md)
