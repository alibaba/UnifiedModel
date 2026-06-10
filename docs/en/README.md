# UModel Documentation

中文：[UModel 中文文档](../zh/README.md)

UModel English documentation entry. English and Chinese documentation are maintained as separate documents under `docs/en` and `docs/zh`, with matching structure and aligned examples, commands, and public contract references.

Documentation root: [docs/README.md](../README.md)

## Getting Started

- [Repository README](../../README.md) - project overview, Quick Start, architecture, and governance links.
- [Installation](getting-started/installation.md) - prerequisites, local setup, build commands, and GraphStore provider selection.
- [Quick Start](getting-started/quickstart.md) - create a workspace, import the multi-domain sample, and run the first queries.
- [GraphStore Providers](graphstore-providers.md) - choose between `memory`, `file.memory`, and `local.ladybug`.
- [Deployments](../../deployments/README.md) - Docker, Compose, ports, data directories, and provider configuration.

## Concepts

- [Concepts Index](concepts/index.md) - recommended reading order and concept map.
- [Object Graph Semantic Layer](concepts/object-graph-semantic-layer.md) - why UModel exists and how it relates to enterprise data, telemetry, runtime systems, and agent context.
- [Workspaces And Domains](concepts/workspaces-and-domains.md) - isolation, naming, and local persistence boundaries.
- [Model Elements](concepts/model-elements.md) - common model envelope and supported model kinds.
- [Entity Sets](concepts/entity-sets.md) - object type definitions and runtime entity relationship.
- [Datasets](concepts/datasets.md) - metrics, logs, traces, events, profiles, and runbooks.
- [Links And Field Mappings](concepts/links-and-field-mappings.md) - DataLink, EntitySetLink, StorageLink, and `fields_mapping`.
- [Storage And GraphStore Providers](concepts/storage-and-graphstore.md) - modeled storage versus runtime providers.
- [Entities And Relations](concepts/entities-and-relations.md) - runtime graph records and lifecycle.
- [Query Surfaces](concepts/query-surfaces.md) - `.umodel`, `.entity`, `.topo`, explain, and agent usage.

## Guides

- [Model Authoring Guide](guides/model-authoring.md)
- [Entity And Relation Write Guide](guides/entity-relation-writes.md)
- [Query Service Guide](guides/query-service.md)
- [Web UI Guide](guides/web-ui.md)
- [SDK And Client Guide](guides/sdk-clients.md) - REST, CLI, MCP, generated model SDKs, and integration examples.
- [Agent Integration Guide](guides/agent-integration.md) - connect an MCP client, the agent tool/resource surface, the query surface an agent uses, and a worked incident-investigation walkthrough.
- [MCP Examples](../../examples/mcp/README.md) - stdio, Streamable HTTP, HTTP+SSE, and TOON payload examples.

## Examples

- [Multi-Domain Quickstart Example Pack](../../examples/quickstart-multidomain/README.md) - five domains connected in one workspace; the default `make quickstart` sample.
- [Incident Investigation Demo](../../examples/incident-investigation/README.md) - scenario-driven, AI-agent-assisted root-cause analysis of a payment-gateway SLO breach across business, platform, and runtime domains, with a runbook-guided diagnosis path.

## Architecture

- [Architecture Overview](architecture/overview.md) - system view, layers, public contracts, and guardrails.
- [Runtime Flow](architecture/runtime-flow.md) - startup, import, write, query, agent, and persistence flows.
- [Query And Agent Architecture](architecture/query-and-agent.md) - Query Service and AgentGateway boundaries.
- [Extension Points](architecture/extension-points.md) - model packs, schemas, providers, query, API, SDK, and UI extensions.
- [Web UI Architecture](ui-architecture.md)

## Reference

- [CLI Reference](reference/cli.md)
- [MCP Reference](reference/mcp.md)
- [Web UI API Map](ui-api.md)
- [UModel SDK Specification](umodel-sdk-specification.md)
- [REST OpenAPI](../../api/openapi/openapi.yaml)
- [MCP Tool And Resource Schema](../../api/mcp/tools.schema.json)
- [Public Go Contracts](../../pkg/contract/contracts.go)
- [Public Domain Models](../../pkg/model/types.go)
- [Stable Error Codes](../../pkg/errors/errors.go)

## Specifications

Cross-implementation contracts that bind both `unified-model` (open source) and `umodel-assistant` (commercial). Breaking either is a P0 regression.

- [Plan Schema v1](spec/plan-schema-v1.md) - mode protocol, plan JSON envelope, and aligned method signatures for `get_metrics` / `get_logs`.

## Generated Schema HTML

- [English generated schema HTML](../html_en/index.html)
- [Chinese generated schema HTML](../html_cn/index.html)
- [Default generated schema HTML](../html/index.html)

Regenerate schema HTML from the repository root:

```bash
make doc
```

## Contributing And Release

- [Contributing Guide](../../CONTRIBUTING.md)
- [Code of Conduct](../../CODE_OF_CONDUCT.md)
- [Security Policy](../../SECURITY.md)
- [Support](../../SUPPORT.md)
- [Changelog](../../CHANGELOG.md)
- [Documentation Internationalization](i18n.md)
