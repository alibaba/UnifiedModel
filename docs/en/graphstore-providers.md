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
