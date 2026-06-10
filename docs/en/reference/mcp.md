# MCP Reference

中文：[MCP 参考](../../zh/reference/mcp.md)

`umodel-mcp` is the local MCP surface for UModel. It exposes discovery metadata, read-only resources, prompts, completions, and query-oriented tools over the same service layer as the REST API.

## Start

Stdio, for local MCP clients:

```bash
go run ./cmd/umodel-mcp --data data --graphstore memory
```

Streamable HTTP, for remote-capable MCP clients:

```bash
go run ./cmd/umodel-mcp --transport http --addr 127.0.0.1:8090 --data data --graphstore file.memory
```

The HTTP transport exposes:

- Streamable HTTP MCP endpoint: `POST /mcp`, `GET /mcp`, `DELETE /mcp`
- Backward-compatible HTTP+SSE endpoints: `GET /sse`, `POST /messages`
- Health check: `GET /healthz`

Runnable examples: [examples/mcp](../../../examples/mcp/README.md).

For persisted stdio development:

```bash
go run ./cmd/umodel-mcp --data data --graphstore file.memory
```

For Ladybug-backed environments:

```bash
go run -tags ladybug ./cmd/umodel-mcp --data data --graphstore local.ladybug
```

## Methods

MCP schema: [api/mcp/tools.schema.json](../../../api/mcp/tools.schema.json).

Supported methods:

- `initialize`
- `notifications/initialized`
- `ping`
- `logging/setLevel`
- `tools/list`
- `tools/call`
- `resources/list`
- `resources/templates/list`
- `resources/read`
- `prompts/list`
- `prompts/get`
- `completion/complete`
- `discovery`

## Output Format

MCP JSON-RPC envelopes remain `application/json`. Tool and resource payload text uses TOON:

- Tool calls return MCP `content` text blocks encoded as TOON, plus `structuredContent` for clients that validate JSON output schemas.
- Resource reads return `contents[].mimeType` as `text/toon` and `contents[].text` as TOON.
- `initialize` advertises `_meta.outputFormat: text/toon`.

Example tool content:

```toon
name: query_spl_examples
ok: true
output[6]: ".umodel with(kind='entity_set') | project domain,name,kind | sort domain,name | limit 20",".entity with(domain='devops', name='devops.service', query='checkout', topk=20)"
```

## Tools

| Tool | Enabled by default | Purpose |
|---|---:|---|
| `query_spl_execute` | Yes | Execute `.umodel`, `.entity_set`, `.entity`, `.topo`, or `.runbook_set` SPL. |
| `query_spl_explain` | Yes | Explain a query plan. |
| `query_spl_examples` | Yes | Return safe query examples. |
| `umodel_validate` | Yes | Validate UModel elements. |
| `umodel_import` | No | Import UModel files from a server-readable path. |
| `entity_write` | No | Write entity payloads. Requires explicit write enablement. |
| `entity_expire` | No | Expire entity payloads. Requires explicit write enablement. |

Write tools are disabled by default so MCP clients start from a safe read-oriented posture.

## Resources

| Resource | URI template | Notes |
|---|---|---|
| `overview` | `umodel://workspace/{workspace}/overview` | Workspace-scoped API and capability overview. |
| `schema-index` | `umodel://workspace/{workspace}/schema-index` | Model/schema metadata summary. |
| `query-templates` | `umodel://workspace/{workspace}/query-templates` | Query templates for `.umodel`, `.entity_set`, `.entity`, and `.topo`. |
| `tool-capability-metadata` | `umodel://workspace/{workspace}/tool-capability-metadata` | Tool capability and write-enablement metadata. |

Resources are read-only and metadata-oriented. Runtime rows should be returned by tools such as `query_spl_execute`.

## Prompts And Completion

Prompts:

- `umodel_query_context`
- `umodel_object_graph_review`

Completion supports resource-template and prompt argument suggestions for workspace resources and common `.umodel`, `.entity`, and `.topo` query starts.

See [MCP Examples](../../../examples/mcp/README.md) for stdio, Streamable HTTP, HTTP+SSE, and TOON parsing examples.

## Local Smoke Test

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","workspace":"demo"}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"workspace":"demo"}}' \
  '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"workspace":"demo","name":"query_spl_examples","arguments":{}}}' \
  '{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{"workspace":"demo"}}' \
  '{"jsonrpc":"2.0","id":5,"method":"resources/templates/list","params":{"workspace":"demo"}}' \
  '{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"workspace":"demo","uri":"umodel://workspace/demo/query-templates"}}' \
  '{"jsonrpc":"2.0","id":7,"method":"prompts/list","params":{}}' \
| go run ./cmd/umodel-mcp --data data --graphstore memory
```

Expected output is one JSON-RPC response per input line. Successful responses contain `result`; tool/resource payload text is TOON.

## Boundaries

- MCP resources do not expose runtime entity or topology rows.
- Query tools are the default read path.
- Write tools must remain disabled unless a caller explicitly opts in through the server-side policy.
- TOON is used inside MCP content blocks. The MCP transport envelope stays JSON-RPC for protocol compatibility.

## See Also

- [Agent Integration Guide](../guides/agent-integration.md) - how an agent connects and uses the query surface, with a worked incident-investigation walkthrough.
