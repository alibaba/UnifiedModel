# UModel Agent Skills

Loadable skills that let an AI agent use UModel — read entities, relationships,
and the model itself, fetch telemetry, and run model-guided root-cause analysis —
over the `umctl` CLI or MCP.

A *skill* here is a self-contained `SKILL.md` (YAML frontmatter `name` +
`description`, then instructions) in the format consumed by skill-aware agent
runtimes such as Claude Code, Cursor, Qoder, and Codex.

> **New here? Start with the [Quickstart](QUICKSTART.md)** — install the skills,
> load a demo object graph (entities + relations), connect Claude Code / Qoder /
> Codex, and ask questions. End to end in a few minutes, no API key.

## Available skills

| Skill | Path | What it does |
|---|---|---|
| `umodel-query` | [`umodel-query/SKILL.md`](umodel-query/SKILL.md) | Read entity / relationship / topology data **and** the UModel model (entity sets, datasets, links, runbooks). CLI-first (`umctl`), MCP alternative. |
| `umodel-rca` | [`umodel-rca/SKILL.md`](umodel-rca/SKILL.md) | Model-guided **autonomous root-cause analysis** over the object graph: fetch the right telemetry, traverse cross-domain relationships, reason to a root cause. Builds on `umodel-query`. |

## Prerequisites

A running UModel server the agent can reach. The quickest path uses the bundled
demo workspace:

```bash
make quickstart QUICKSTART_SAMPLE=examples/incident-investigation   # serves http://localhost:8080
```

The agent then reads through either transport:

- **CLI** (preferred, lowest setup): `umctl query run <workspace> "<SPL>" -o json`
- **MCP**: connect `umodel-mcp` and call the `query_spl_execute` tool

No API key or network is required for the demo.

## Using a skill

### Option A — Claude Code plugin marketplace (one command)

In Claude Code, install both skills as a plugin straight from this repo:

```
/plugin marketplace add alibaba/UnifiedModel
/plugin install umodel@unifiedmodel
```

This installs the `umodel` plugin — both `umodel-query` and `umodel-rca` — which
then activate automatically based on your prompt. Update later with
`/plugin marketplace update unifiedmodel`.

### Option B — copy into your agent's skills directory

Most skill-aware agents discover skills from a directory — drop both skill folders
into the one your agent scans:

| Agent | Skills directory |
|---|---|
| Claude Code | `.claude/skills/` |
| Cursor | `.cursor/skills/` |
| Qoder | `.qoder/skills/` |
| Codex | `.agents/skills/` (or `~/.agents/skills/` for user-global) |

```bash
# Qoder
mkdir -p .qoder/skills  && cp -R skills/umodel-query skills/umodel-rca .qoder/skills/
# Codex — the vendor-neutral .agents/skills/ is also read by Qoder
mkdir -p .agents/skills && cp -R skills/umodel-query skills/umodel-rca .agents/skills/
```

Then prompt the agent normally — e.g. *"query the degraded services in this
workspace"* (activates `umodel-query`) or *"payment-gateway 的 SLO 告警了，帮我排查"*
(activates `umodel-rca`). Each skill's `description` controls when the agent
activates it; trigger one manually with `/umodel-query` (Claude Code / Qoder) or
`$umodel-query` (Codex).

## How the skills relate

They map to the three things an agent does with UModel:

- **`umodel-query`** covers reads — (1) entity & relationship/topology data
  (`.entity` / `.topo`) and (2) the model itself (`.umodel` + `__list_method__` /
  `list_data_set`). Real rows in open source; the PaaS API's data against a
  PaaS-backed endpoint.
- **`umodel-rca`** adds (3) model-guided fetch (`get_metrics` / `get_logs`, a
  *plan* in open source / *data* via PaaS) and an autonomous root-cause loop. It
  reuses `umodel-query`'s reads, so load both for an investigation.

## Authoring a new skill

Add a directory `skills/<name>/` with a `SKILL.md`:

```markdown
---
name: <name>
description: >-
  One or two sentences on what the skill does and when an agent should use it.
  Include trigger phrases — this is what the agent matches on.
---

# <Title>

Imperative instructions: how to connect, the toolkit, the method, a worked
example, and gotchas.
```

Keep skills transport-agnostic where possible (same SPL over CLI or MCP), and
prefer real, verified commands over aspirational ones.

## See also

- [Agent Integration Guide](../docs/en/guides/agent-integration.md) — the full
  human-facing walkthrough the `umodel` skill is built on.
- [MCP Reference](../docs/en/reference/mcp.md) — transports, tools, resources.
- [Incident Investigation Demo](../examples/incident-investigation/README.md) —
  the worked example / test bed the `umodel` skill is validated against.
