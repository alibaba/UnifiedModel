# Web UI Guide

中文：[Web UI 指南](../../zh/guides/web-ui.md)

UModel Web: local console for workspaces, model definitions, runtime data, Query Service behavior, and AgentGateway metadata.


## Start

```bash
make quickstart
```

Open:

```text
http://localhost:5173
```

## Workspace Launcher

Workspace Launcher covers:

- Create a workspace.
- Select an existing workspace.
- API health.
- API endpoint override for non-default local ports.

## Explorer

Explorer shows imported UModel definitions as graph and table views.

Explorer scope:

- EntitySets.
- Datasets.
- DataLinks.
- EntitySetLinks.
- Storage and StorageLinks.

## Query

Query is the interactive surface for:

- `.umodel`
- `.entity`
- `.topo`

Query validates workflows before CLI, SDK, MCP, or documentation reuse.

## Imports & Writes

Imports & Writes supports local workflows around:

- UModel model import.
- Entity writes.
- Relation writes.

Keep write operations explicit and review payloads before submitting them.

## Data Store

Data Store provides runtime entity and topology views behind the Query Service boundary. After `make quickstart`, it shows the bundled multi-domain sample.

## Agent

Agent shows:

- Discovery output.
- Tools.
- Resources.
- Query examples.
- Suggested next actions.

Same AgentGateway surface as MCP clients.

## Settings And API Map

Settings and API Map expose:

- API health.
- Active provider.
- Common endpoints.
- Public REST paths used by the UI.

## Developer Rules

- UI code lives under [web/](../../../web).
- UI architecture is documented in [Web UI Architecture](../ui-architecture.md).
- API usage is documented in [Web UI API Map](../ui-api.md).
- The UI should use public REST APIs only.
