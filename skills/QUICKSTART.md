# Quickstart — use the UModel skills with your agent

Get an AI agent (Claude Code / Qoder / Codex / …) using UModel end-to-end in a few
minutes: **install the skills**, **initialize a demo object graph** (entities +
relations), **connect your agent**, and **ask questions**.

The demo dataset is the bundled **incident-investigation** pack — 3 domains
(business / platform / runtime), ~65 entities, ~83 relations, a runbook, and
metric/log sets — the same data the skills are validated against. Everything runs
locally, in memory, with **no API key and no network**.

## Prerequisites

- Go 1.22+ and Make
- A clone of this repo (the MCP server runs from source)
- An MCP-capable agent: Claude Code, Qoder, Codex, Cursor, …

## 1. Install the skills

The skills are plain `SKILL.md` folders under [`skills/`](README.md). All three
agents load them natively — pick your client:

- **Claude Code** — install both skills as a plugin in one command:
  ```
  /plugin marketplace add alibaba/UnifiedModel
  /plugin install umodel@unifiedmodel
  ```
  The `umodel` plugin bundles both skills; they auto-activate by prompt. (Or copy
  them into `.claude/skills/`.)
- **Qoder** — copy both skills into the workspace skills directory:
  ```bash
  mkdir -p .qoder/skills && cp -R skills/umodel-query skills/umodel-rca .qoder/skills/
  ```
  They auto-activate by prompt, or trigger one manually with `/umodel-query`.
  (Qoder also has a Skills Marketplace and a built-in `create-skill` helper.)
- **Codex** — copy both into the vendor-neutral `.agents/skills/` directory (use
  `~/.agents/skills/` to make them user-global):
  ```bash
  mkdir -p .agents/skills && cp -R skills/umodel-query skills/umodel-rca .agents/skills/
  ```
  They auto-activate by description, or mention one with `$umodel-query` (type `$`,
  or run `/skills`, to browse). Restart Codex if a new skill doesn't appear.

> `.agents/skills/` is the cross-agent open-standard path — Qoder reads it too, so a
> single copy there can serve both Qoder and Codex.
>
> A skill's `description` is what triggers activation in agents that support skills.

## 2. Initialize the demo data

The object graph (entities + relations + model + runbook) ships in
`examples/incident-investigation/` and loads **in memory** — nothing is written to
disk, nothing is left behind.

- **If you'll connect over MCP** (most common): the MCP server loads the data on
  startup via `--quickstart-sample` — your agent launches it (see §3). Nothing to
  do here.
- **If you'll drive over CLI / HTTP** (or want the Web UI): start the HTTP server:
  ```bash
  make quickstart QUICKSTART_SAMPLE=examples/incident-investigation   # API :8080, Web UI :5173
  ```
  Sanity-check the data loaded:
  ```bash
  umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
  umctl query run demo ".entity with(domain='platform', name='platform.service', query='degraded')" -o json
  # → payment-gateway | degraded | …
  ```

## 3. Connect your agent (MCP)

All three clients use the **same** MCP server invocation — it loads the demo data
via `--quickstart-sample` and serves the agent:

```
command: go
args:    run ./cmd/umodel-mcp --quickstart --quickstart-sample examples/incident-investigation --graphstore memory
```

### Claude Code

Project `.mcp.json` (or `claude mcp add`):

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

Qoder Settings → **MCP** → **My Servers** → **+ Add**, paste the same
`{ "mcpServers": { "umodel": { … } } }` block, then **Save**. MCP works in
**Agent mode**. (Or use `qodercli mcp add`.)

### Codex

`~/.codex/config.toml` (or `codex mcp add`); verify with `/mcp` in a session:

```toml
[mcp_servers.umodel]
command = "go"
args = ["run", "./cmd/umodel-mcp", "--quickstart",
        "--quickstart-sample", "examples/incident-investigation",
        "--graphstore", "memory"]
```

> Launch from the repo root (the `./cmd/umodel-mcp` path is relative), or use an
> absolute `go` path / a prebuilt `umodel-mcp` binary. For a remote server, run it
> with `--transport http --addr 0.0.0.0:8090` and point the client at
> `http://<host>:8090/mcp`.

### No MCP? Use the CLI

The skills work over the CLI too — start the server (§2) and the agent runs
`umctl query run demo "<SPL>" -o json`.

## 4. Ask questions

With the data loaded and the agent connected, just ask in natural language.

**Reads — activates `umodel-query`:**

- "List the services in this workspace and their status."
- "What does payment-gateway depend on? Show me the topology."
- "Which metric and log sets are attached to payment-gateway?"
- "Read payment-gateway's p99 latency over the last hour." (fetches the plan, runs it against your Prometheus)

**Root-cause analysis — activates `umodel-rca`:**

- "Investigate why payment-gateway is degraded." / "payment-gateway 的 SLO 告警了，帮我排查。"

The agent then works autonomously: locate the degraded service → pull its
metrics/logs → traverse to the upstream caller → find the retry config change →
rule out the red-herring deployment → find the promotion traffic → conclude the
root cause (retry ×2.5 × promotion ×3.5 = **8.75×** overload) and recommend a
rollback.

> **Metrics & logs need a backend.** `.entity` / `.topo` / `.umodel` reads work out
> of the box; for metric/log **values**, `get_metrics` / `get_logs` return an
> executable plan (PromQL / Elasticsearch DSL) that the agent runs against **your
> Prometheus / Elasticsearch** — point it at where you loaded the data. See
> *Read metrics & logs* in [umodel-query](umodel-query/SKILL.md).

## Troubleshooting

- **No UModel tools in the agent** → run the `go run ./cmd/umodel-mcp …` line
  manually first; it should start cleanly. Ensure Agent mode (Qoder) / run `/mcp`
  (Codex) to confirm the server is connected.
- **Empty results** → data is in memory per server process; make sure that server
  was started with `--quickstart-sample examples/incident-investigation`.
- **`go` not found by the agent** → use an absolute path to `go`, or build once
  (`go build -o umodel-mcp ./cmd/umodel-mcp`) and point `command` at the binary.

## See also

- [Agent Skills catalog](README.md)
- [Agent Integration Guide](../docs/en/guides/agent-integration.md)
- [Incident Investigation Demo](../examples/incident-investigation/README.md)
