# UModel Web UI

UModel Web is a small, open-source friendly React/Vite workspace UI for the public UModel REST API.

## Development

Requirements:

- Node.js 22 or newer
- pnpm 9 or newer

Start the API server:

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

Start the web dev server:

```bash
cd web
pnpm install
pnpm dev
```

The Vite server proxies `/api/*` and `/healthz` to `http://localhost:8080`.
Set `UMODEL_API_TARGET` to point at another API server.

## Production Build

```bash
cd web
pnpm build
```

The Go server can serve the generated assets:

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory --ui-dir web/dist
```

From the repository root you can also use:

```bash
make dev
```

This starts the API at `http://localhost:8080` and the web UI at `http://localhost:5173`.

Stop the local API and web dev servers:

```bash
make stop-all
```

For file-backed UI development, run the default local workflow from the
repository root:

```bash
make dev
```

The UI still uses only the REST API. `make dev` uses `file.memory` and
`DATA_ROOT=data` by default, so data added through the UI survives API restarts.
Both `memory` and `file.memory` support Ladybug-compatible read-only Cypher
through the pure Go engine. Use `GRAPHSTORE=memory GO_TAGS= make dev` for
temporary UI work where data can be lost.

For a single production-style server:

```bash
make serve-ui
```

This builds the UI and serves API plus UI from `http://localhost:8080`.
