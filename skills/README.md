# UModel Agent Skills

Loadable skills that let an AI agent use UModel — read entities, relationships,
and the model itself, fetch telemetry, and run model-guided root-cause analysis —
over the `umctl` CLI or MCP.

A *skill* here is a self-contained `SKILL.md` (YAML frontmatter `name` +
`description`, then instructions) in the format consumed by skill-aware agent
runtimes such as Claude Code, Cursor, and Qoder.

## Available skills

| Skill | Path | What it does |
|---|---|---|
| `umodel` | [`umodel/SKILL.md`](umodel/SKILL.md) | Read entity / relationship / topology data and the UModel model, then run model-guided root-cause analysis over the object graph. CLI-first (`umctl`), with an MCP alternative. |

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

Most skill-aware agents discover skills from a directory. Point your agent at a
skill here, or copy it into the location your agent scans, for example:

```bash
# Claude Code / Cursor / Qoder (Claude-Code-compatible skill loaders)
mkdir -p .claude/skills
cp -R skills/umodel .claude/skills/umodel
```

Then prompt the agent normally — for the `umodel` skill, e.g.
*"payment-gateway 的 SLO 告警了，帮我排查"* or *"query the degraded services in
this workspace"*. The skill's `description` controls when the agent activates it.

## How the `umodel` skill is organized

Three capabilities (see [`umodel/SKILL.md`](umodel/SKILL.md) for the full method
and commands):

1. **Read entity & relationship data** — `.entity` / `.topo`. Returns real rows in
   open source; against a PaaS-backed endpoint the same commands return the PaaS
   API's data.
2. **Read the UModel model** — `.umodel` + `__list_method__` / `list_data_set`:
   the map of object types, datasets, links, and runbooks.
3. **Model-guided fetch + root-cause analysis** — `get_metrics` / `get_logs`
   driven by the object graph (a *plan* in open source, *data* via a PaaS
   endpoint), plus an autonomous investigation loop.

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
