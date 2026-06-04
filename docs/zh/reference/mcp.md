# MCP 参考

English: [MCP Reference](../../en/reference/mcp.md)

`umodel-mcp` 是 UModel 的本地 MCP surface。它通过与 REST API 相同的服务层暴露 discovery metadata、read-only resources、prompts、completions 和 query-oriented tools。


## 启动

Stdio，用于本地 MCP client：

```bash
go run ./cmd/umodel-mcp --data data --graphstore memory
```

Streamable HTTP，用于支持远程连接的 MCP client：

```bash
go run ./cmd/umodel-mcp --transport http --addr 127.0.0.1:8090 --data data --graphstore file.memory
```

HTTP transport 暴露：

- Streamable HTTP MCP endpoint：`POST /mcp`、`GET /mcp`、`DELETE /mcp`
- 向后兼容的 HTTP+SSE endpoints：`GET /sse`、`POST /messages`
- 健康检查：`GET /healthz`

可运行示例：[examples/mcp](../../../examples/mcp/README.zh-CN.md)。

需要持久化的 stdio 开发：

```bash
go run ./cmd/umodel-mcp --data data --graphstore file.memory
```

Ladybug-backed 环境：

```bash
go run -tags ladybug ./cmd/umodel-mcp --data data --graphstore local.ladybug
```

## Methods

MCP schema：[api/mcp/tools.schema.json](../../../api/mcp/tools.schema.json)。

支持：

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

## 输出格式

MCP JSON-RPC 外壳保持 `application/json`。Tool 和 Resource 的 payload text 使用 TOON：

- Tool call 返回 MCP `content` text block，文本按 TOON 编码，同时保留 `structuredContent` 供校验 JSON 输出 schema 的 client 使用。
- Resource read 返回 `contents[].mimeType: text/toon`，`contents[].text` 为 TOON。
- `initialize` 通过 `_meta.outputFormat: text/toon` 暴露输出格式。

Tool content 示例：

```toon
name: query_spl_examples
ok: true
output[6]: ".umodel with(kind='entity_set') | project domain,name,kind | sort domain,name | limit 20",".entity with(domain='devops', name='devops.service', query='checkout', topk=20)"
```

## Tools

| Tool | 默认启用 | 用途 |
|---|---:|---|
| `query_spl_execute` | Yes | 执行 `.umodel`、`.entity_set`、`.entity`、`.topo` 或 `.runbook_set` SPL。 |
| `query_spl_explain` | Yes | 返回查询计划。 |
| `query_spl_examples` | Yes | 返回安全查询示例。 |
| `umodel_validate` | Yes | 校验 UModel elements。 |
| `umodel_import` | No | 从服务端可读路径导入 UModel 文件。 |
| `entity_write` | No | 写入 entity payload，需要显式启用写能力。 |
| `entity_expire` | No | 过期 entity payload，需要显式启用写能力。 |

写工具默认关闭，使 MCP client 从安全的 read-oriented 姿态开始。

## Resources

| Resource | URI template | 描述 |
|---|---|---|
| `overview` | `umodel://workspace/{workspace}/overview` | Workspace-scoped API 和能力概览。 |
| `schema-index` | `umodel://workspace/{workspace}/schema-index` | 模型/schema 元数据摘要。 |
| `query-templates` | `umodel://workspace/{workspace}/query-templates` | `.umodel`、`.entity_set`、`.entity` 和 `.topo` 查询模板。 |
| `tool-capability-metadata` | `umodel://workspace/{workspace}/tool-capability-metadata` | Tool 能力和写启用元数据。 |

Resources 只读且面向元数据。运行时 rows 应由 `query_spl_execute` 等 tools 返回。

## Prompts 和 Completion

Prompts：

- `umodel_query_context`
- `umodel_object_graph_review`

Completion 支持 resource-template 和 prompt argument 建议，用于 workspace resources 以及常见 `.umodel`、`.entity`、`.topo` 查询起点。

Stdio、Streamable HTTP、HTTP+SSE 和 TOON 解析示例见 [MCP 示例](../../../examples/mcp/README.zh-CN.md)。

## 本地烟测

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

期望输出为每个输入行对应一条 JSON-RPC response。成功响应包含 `result`；tool/resource payload text 使用 TOON。

## 边界

- MCP resources 不暴露运行时 entity 或 topology rows。
- Query tools 是默认读取路径。
- 写 tools 必须保持默认关闭，除非调用方通过服务端策略显式启用。
- TOON 用在 MCP content block 内部。MCP transport 外壳保持 JSON-RPC，以保证协议兼容。
