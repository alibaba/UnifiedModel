# GraphStore Providers

中文：[GraphStore Providers](../zh/graphstore-providers.md)

UModel stores UModel elements, CMS 2.0 entities, and topology relations behind the `GraphStore` interface. Go entry binaries default to `local.ladybug` when `--graphstore` is omitted; select another provider with `--graphstore` on `umodel-server` or `umodel-mcp`.

If a build omits the `ladybug` tag, the default `local.ladybug` provider reports an unavailable health status. Build with `-tags ladybug` to enable the real `local.ladybug` provider, or pass `--graphstore file.memory` for local development without Ladybug.

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

Active provider locations:

- `GET /healthz` as `graphstore.provider`
- query explain output as `provider` and `storage_provider`

## Providers

| Provider | Persistence | Typical use |
|---|---|---|
| `memory` | Process memory only | Fast local tests and demos where data can disappear on restart; supports Ladybug-compatible read-only Cypher through the pure Go engine. |
| `file.memory` | JSON files under `--data` | Local demos and development where data should survive process restart without Ladybug; supports the same pure Go read-only Cypher engine as `memory`. `make dev` selects this provider by default. |
| `local.ladybug` | Ladybug database files | Ladybug-backed provider with graph-match and Cypher passthrough enabled; requires building with `-tags ladybug` and a local Ladybug runtime. |
| `neo4j` | Remote Neo4j database | Bolt-backed nodes and topology edges; controlled read-only Cypher uses the shared Go engine. No build tag or native runtime required. |
| `gremlin` | Remote TinkerPop graph | WebSocket-backed vertices and topology edges through Gremlin Server; queries use the shared Go engine. No build tag or native runtime required. |

## `gremlin`

Both `umodel-server` and `umodel-mcp` accept `--graphstore gremlin`. The provider uses synchronous WebSocket requests with GraphSON 3, preserving the Go 1.22 baseline. The endpoint must support `application/vnd.gremlin-v3.0+json`, bound `gremlin-groovy` scripts, and a graph traversal source. `wss://` verifies TLS certificates; optional username/password authentication uses SASL PLAIN. The test fixtures also use the official TinkerPop Go driver over GraphBinary to independently inspect and clean up stored records.

```bash
export GREMLIN_URI=ws://localhost:8182/gremlin
export GREMLIN_TRAVERSAL_SOURCE=g
go run ./cmd/umodel-server --addr :8080 --data data --graphstore gremlin
```

| Environment variable | `ProviderConfig.Options` key | Default |
|---|---|---|
| `GREMLIN_URI` | `uri` | `ws://localhost:8182/gremlin` |
| `GREMLIN_USERNAME` | `username` | Empty; configure together with password |
| `GREMLIN_PASSWORD` | `password` | Empty; configure together with username |
| `GREMLIN_TRAVERSAL_SOURCE` | `traversal_source` | `g` |
| `GREMLIN_CA_FILE` | `ca_file` | Empty; uses system roots. Optional PEM CA file for `wss://` |
| `GREMLIN_TIMEOUT` | `timeout` | `10s` for an entire operation, including queueing; accepts `1ms` to `24h` |

Programmatic options override environment variables, including explicitly empty credentials. Credentials belong in the username/password settings rather than the URI. Connection or traversal-source failures stop startup. The account needs graph read/write permissions; backend-specific indexes and constraints remain operator-managed.

Requests are limited to 1 MiB; configure the server's `maxContentLength` accordingly and keep write batches below both limits. Each request is sent in one WebSocket frame; partial result responses are collected until completion. The bundled test server configuration uses 1 MiB.

The provider persists `UModelElement` and `UModelEntity` vertices keyed by `(workspace, key)`, and `UMODEL_RELATION` edges between entity vertices. JSON payload properties preserve nested values and integer precision. Relations can arrive before their entities; placeholder endpoints are excluded from entity reads. `Create`, `Update`, `Expire`, `Delete`, and historical visibility use the same evaluator as `memory`.

**Run one writing provider instance per graph.** Operations within that instance are serialized, and duplicate records are rejected. Separate REST/MCP writers or external writers cannot coordinate through portable Gremlin. Each accepted batch is serialized completely before one bound write script is submitted. Server-side rollback depends on the backend's transaction support and Gremlin Server configuration; non-transactional graphs can retain partial writes after a server failure. An error reports no accepted items and is never automatically retried. Cancellation of an in-flight request closes the provider; reopen the application and inspect remote state before retrying writes whose outcome is unknown.

Query Service continues to accept SPL and controlled read-only Cypher. The provider fetches workspace collections and evaluates them in Go; arbitrary user Gremlin is not exposed, and filters/traversals are not pushed down. Read cost and application memory grow with the workspace collection size.

Workspace metadata remains at `<data-root>/workspaces.json`. Preserve `--data` across application restarts and back it up together with the remote graph. Startup restores derived schema and search indexes from persisted records, including `.runbook_set` and keyword/vector/hyper search routing. The default embedder is `noop`; these tests validate routing and persistence, not embedding quality. Graph persistence across database restarts must be configured in the backend; the default Gremlin Server TinkerGraph example is in memory.

Run an isolated local test server and the opt-in contract/restart suite:

```bash
docker run -d --name umodel-gremlin-e2e -p 127.0.0.1:18182:8182 \
  -v "$PWD/tests/fixtures/gremlin:/test:ro" \
  --entrypoint /opt/gremlin-server/bin/gremlin-server.sh tinkerpop/gremlin-server:3.7.3 /test/server.yaml
docker logs umodel-gremlin-e2e
# After Gremlin Server reports that it is listening:
GREMLIN_URI=ws://localhost:18182/gremlin UMODEL_TEST_GREMLIN=1 make test-gremlin
docker rm -f umodel-gremlin-e2e
```

The opt-in suite runs the existing Query Service, REST integration, CLI, MCP stdio/HTTP, golden, GraphStore contract, SDK example, and restart tests on Gremlin. Tests use isolated workspace IDs and clean up only their own records; packages run serially. `graph-match` is not advertised by this provider, and its `PROVIDER_UNSUPPORTED` response is checked. The standard `make test-service` needs no Gremlin Server.

Run the full acceptance matrix, including separate plain, authenticated TLS, persistent, and transaction-capable servers:

```bash
make test-gremlin-matrix
# With Podman:
UMODEL_CONTAINER_RUNTIME=podman make test-gremlin-matrix
```

The runner creates disposable TLS certificates and credential graphs, verifies bad credentials/untrusted certificates, oversized batches, rollback after a server exception, server disconnection, and database restart with search recovery. It removes only its own containers and temporary files. CI runs this matrix on the official Gremlin Server 3.7.3 image. Rollback is tested on TinkerTransactionGraph; non-transactional TinkerGraph retains the limitation above. Protocol details: [Apache TinkerPop provider protocol](https://tinkerpop.apache.org/docs/3.7.3/dev/provider/).

Earlier contract and application-restart tests also passed against standalone Gremlin Server 3.8.1 with TinkerGraph, using TinkerPop libraries bundled in ArcadeDB 26.9.1. Hosted services and other graph backends still require their own compatibility checks.

## `neo4j`

The default `neo4j` dialect targets Neo4j 5.26 or later. The same provider offers an `opencypher` compatibility dialect for Bolt-compatible databases. It uses the official Go driver v5 to retain the repository's Go 1.22 build baseline. Both `umodel-server` and `umodel-mcp` accept `--graphstore neo4j`.

```bash
export NEO4J_URI=bolt://localhost:7687
export NEO4J_USERNAME=neo4j
export NEO4J_PASSWORD='replace-with-your-password'
export NEO4J_DATABASE=neo4j
go run ./cmd/umodel-server --addr :8080 --data data --graphstore neo4j
```

| Environment variable | `ProviderConfig.Options` key | Default |
|---|---|---|
| `NEO4J_URI` | `uri` | `bolt://localhost:7687` |
| `NEO4J_USERNAME` | `username` | `neo4j` |
| `NEO4J_PASSWORD` | `password` | Required; no default |
| `NEO4J_DIALECT` | `dialect` | `neo4j`; also accepts `opencypher` |
| `NEO4J_DATABASE` | `database` | `neo4j` in native mode; empty (server default) in `opencypher` mode |
| `NEO4J_TIMEOUT` | `timeout` | `10s` per database operation |

Programmatic options override environment variables. In native mode, use `neo4j://` for routing and `neo4j+s://` or `bolt+s://` for TLS with certificate validation. Native startup connects to the selected database and creates constraints/indexes, so the database must already exist and the account must have schema and data read/write permissions. Connection and authentication failures stop startup in either mode.

The provider stores `UModelElement` and `UModelEntity` nodes keyed by `(workspace, key)`, with `UMODEL_RELATION` edges between entity nodes. JSON properties retain nested payloads and integer precision. Relations can arrive before entities; their placeholder endpoints are not exposed as entity records. In native mode, each write batch uses a transaction and a database workspace lock, so concurrent provider processes preserve `Create`, `Update`, `Expire`, and `Delete` behavior. A failed transaction reports no accepted writes.

Queries fetch the selected workspace's records from Neo4j on each request and evaluate them with the shared Go engine. Controlled Cypher is a read-only subset, not native Neo4j Cypher passthrough. Filters/traversals are not pushed down to Neo4j; read cost and application memory grow with the workspace collection size. `Expire` records remain available to historical queries; `Delete` records remain hidden, matching `memory`.

Workspace metadata still belongs to Workspace Service and is persisted at `<data-root>/workspaces.json`. Keep `--data` across restarts and back it up together with the database. Native graph writes can coordinate across processes, but workspace metadata is local to each service instance; this is not a shared multi-instance metadata store.

Start Neo4j and UModel together using Docker Compose:

```bash
export NEO4J_PASSWORD='replace-with-your-password'
docker compose -f deployments/compose/docker-compose.neo4j.yaml up --build
```

Run the opt-in provider contract and restart tests against a test database:

```bash
UMODEL_TEST_NEO4J=1 make test-neo4j
```

Tests create unique workspaces and clean up only their own records. CI runs both dialects against Neo4j Community 5.26. The standard `make test-service` requires no Neo4j instance.

### openCypher compatibility mode

Use the same provider with `NEO4J_DIALECT=opencypher`. For a database such as Alibaba Cloud GDB's OpenCypher engine:

```bash
export NEO4J_DIALECT=opencypher
export NEO4J_URI='bolt://your-gdb-endpoint:your-port'
export NEO4J_USERNAME='your-database-user'
export NEO4J_PASSWORD='replace-with-your-password'
export NEO4J_DATABASE=''
export NEO4J_TIMEOUT=30s
go run ./cmd/umodel-server --addr :8080 --data data --graphstore neo4j
```

Use the endpoint and port supplied by the database service. `bolt+s://` and `bolt+ssc://` are also accepted when the backend provides TLS. Routing schemes (`neo4j://` and variants) are rejected in this mode. An empty database setting selects the server default and avoids Bolt v3 multi-database errors; named databases are only usable when the backend supports them. Explicitly clear an inherited `NEO4J_DATABASE=neo4j` when connecting to GDB.

| Capability | `neo4j` | `opencypher` |
|---|---|---|
| Startup | Creates native constraints/indexes | Checks connectivity with `RETURN 1`; issues no schema DDL |
| Bookmark sharing | Enabled | Disabled |
| Driver telemetry | Driver default | Disabled |
| Workspace write coordination | Database lock plus uniqueness constraints | Serializes writes within one provider instance |
| Models, entities, relations, history, controlled Cypher | Shared semantics | Shared semantics |

**Run exactly one writing provider instance per database in openCypher mode.** Concurrent requests to that instance are serialized, including workspace initialization; waiting for the writer slot respects cancellation and the operation timeout. Do not run separate server/MCP writer processes or external writers against the same records. This restriction is a deployment requirement, not a distributed lock: portable Cypher cannot supply the native uniqueness and locking guarantees across instances. The provider rejects duplicate workspace markers or record keys encountered during reads instead of silently choosing one. Indexes and any external constraints remain managed by the database service/operator.

`/healthz` retains `graphstore.provider=neo4j`; `graphstore.message` identifies compatibility mode and its single-writer requirement. Compatibility mode retains local workspace metadata and the shared Go query engine described above.

[GDB's compatibility reference](https://help.aliyun.com/zh/gdb/developer-reference/compatibility-of-gdb-cypher) documents Bolt v3, no Bookmark support, and separate index/constraint management. Backend compatibility still requires testing transactional rollback, all write methods, and restart/query behavior against the actual service. The repository includes a Bolt v3 protocol fixture and tests both dialects against Neo4j; these do not certify a live GDB instance.

After configuring a dedicated test database, run the same suite in compatibility mode:

```bash
NEO4J_DIALECT=opencypher NEO4J_DATABASE='' UMODEL_TEST_NEO4J=1 make test-neo4j
```

The native cross-provider concurrency test is skipped in this mode; concurrent requests to a single provider are tested. Keep the test database free of other writers during the run.

The suite runs test packages serially (`go test -p 1`) so contract and E2E packages do not create simultaneous writing provider instances.

### Verified open-source backend

ArcadeDB 26.9.1 was tested with the `opencypher` dialect on Linux ARM64 under Podman. The GraphStore contract, lifecycle/isolation/rollback, single-instance concurrent Create, and quickstart application-restart E2E tests all passed, both with `database=neo4j` and with the database name left empty. These results validate ArcadeDB, not Alibaba Cloud GDB.

Start the same isolated test backend:

```bash
podman run -d --name umodel-opencypher-e2e \
  --memory=1536m --cpus=2 \
  -p 127.0.0.1:17688:7687 \
  -e 'JAVA_OPTS=-Xms256m -Xmx768m -Darcadedb.server.rootPassword=umodel-e2e-password -Darcadedb.server.defaultDatabases=neo4j[root] -Darcadedb.server.plugins=Bolt:com.arcadedb.bolt.BoltProtocolPlugin' \
  docker.io/arcadedata/arcadedb:26.9.1
podman logs umodel-opencypher-e2e
```

After the log reports that the server has started, run:

```bash
NEO4J_DIALECT=opencypher NEO4J_DATABASE='' \
NEO4J_URI=bolt://127.0.0.1:17688 \
NEO4J_USERNAME=root NEO4J_PASSWORD=umodel-e2e-password \
NEO4J_TIMEOUT=30s UMODEL_TEST_NEO4J=1 make test-neo4j
podman stop umodel-opencypher-e2e
podman rm -v umodel-opencypher-e2e
```

The credential above is for the disposable local test container only. The image is retained after cleanup.

## `file.memory`

`file.memory` keeps the same query and lifecycle semantics as `memory`, but loads and saves JSON snapshots on disk:

- Default directory: `<data-root>/graphstore/file-memory/`
- Custom directory from code: `graphstore.ProviderConfig{Options: map[string]string{"path": "/path/to/file-memory-dir"}}`
- Loading: reads workspace collection files once during provider startup
- Querying: serves `.umodel`, `.entity`, and `.topo` from memory
- Saving: atomically rewrites the changed workspace collection files after successful UModel, entity, relation, or direct GraphStore workspace/schema writes

The default layout splits data by workspace and collection:

```text
<data-root>/graphstore/file-memory/
└── workspaces/
    └── demo/
        ├── umodels.json
        ├── entities.json
        └── relations.json
```

Each collection file has a small envelope:

```json
{
  "version": 1,
  "items": {}
}
```

For compatibility, the provider can still read the old single-file layout at
`<data-root>/graphstore/file-memory.json`. When that legacy file is loaded and
the new directory layout is absent, the provider writes the data back out using
the split workspace layout.

Current entity and topology queries hide expired/deleted rows unless a historical `time_range` is supplied. The expired/deleted records are still kept in the file so historical queries can read them after restart.

## Scope And Limits

- `file.memory` persists GraphStore data: UModel elements, entities, and relations.
- Workspace metadata managed by `/api/v1/workspaces` is persisted separately at `<data-root>/workspaces.json` when the server starts with the `file.memory` provider.
- Single local process only. Do not run multiple writers against the same file-memory directory.
- The JSON files are useful for inspection and demos, but they are not a long-term storage compatibility contract.

## Smoke Test

```bash
go run ./cmd/umodel-server --addr :8080 --data /tmp/umodel-demo --graphstore file.memory
go run ./cmd/umctl --addr http://localhost:8080 umodel put demo '{"id":"devops.service","kind":"entity_set","domain":"devops","name":"devops.service"}'
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
find /tmp/umodel-demo/graphstore/file-memory -maxdepth 4 -type f
```

Restart the server with the same `--data` path and rerun the query to confirm the data was loaded back from disk.
