# cmd/

中文版本：[README.md](README.md)

Entry Layer: process entry points.

| Directory | Description |
|---|---|
| `umodel-server/` | UModel HTTP service. |
| `umctl/` | CLI tool for the public REST API. |
| `umodel-mcp/` | stdio MCP server. |

## GraphStore Provider Flag

`umodel-server` and `umodel-mcp` both support `--graphstore`:

When `--graphstore` is omitted, Go entrypoints use `local.ladybug`. Builds without `-tags ladybug` report clear startup guidance: build with the Ladybug tag, or pass `--graphstore file.memory` for local development.

| Provider | Description |
|---|---|
| `memory` | In-memory provider for fast local verification; data is lost on process exit. |
| `file.memory` | In-memory querying plus JSON file persistence under `<--data>/graphstore/file-memory/`. |
| `local.ladybug` | Ladybug-backed provider; requires `-tags ladybug` and a local Ladybug runtime. |

Examples:

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
go run ./cmd/umodel-mcp --data data --graphstore file.memory
```

See [GraphStore Providers](../docs/en/graphstore-providers.md).
