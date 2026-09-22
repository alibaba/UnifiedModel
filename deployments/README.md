# Deployments

This directory contains local deployment assets for UModel Open Source.

| Path | Purpose |
|---|---|
| `docker/Dockerfile` | Builds `umodel-server` into a small runtime image. |
| `compose/docker-compose.yaml` | Runs the server with a persistent local Docker volume. |
| `compose/docker-compose.neo4j.yaml` | Runs Neo4j and UModel with separate graph and workspace metadata volumes. |

## Provider Default

The open-source deployment assets use `--graphstore file.memory` by default. This avoids local Ladybug runtime requirements and persists GraphStore JSON data under `/data`.

Use `local.ladybug` only in an environment that has:

- A Ladybug-enabled build.
- The `liblbug` runtime available to the process.
- A deliberate operational reason to use the Ladybug-backed provider.

## Docker

Build the image from the repository root:

```bash
docker build -f deployments/docker/Dockerfile -t umodel-open-source:local .
```

Run the server:

```bash
docker run --rm \
  -p 8080:8080 \
  -v umodel-data:/data \
  umodel-open-source:local
```

The container starts:

```bash
umodel-server --addr :8080 --data /data --graphstore file.memory
```

Check health:

```bash
curl http://localhost:8080/healthz
```

## Docker Compose

Start:

```bash
docker compose -f deployments/compose/docker-compose.yaml up --build
```

Stop:

```bash
docker compose -f deployments/compose/docker-compose.yaml down
```

Remove persisted data:

```bash
docker compose -f deployments/compose/docker-compose.yaml down -v
```

## Neo4j Compose

```bash
export NEO4J_PASSWORD='replace-with-your-password'
docker compose -f deployments/compose/docker-compose.neo4j.yaml up --build
```

This selects `--graphstore neo4j` and waits for Neo4j to become healthy before starting UModel. Neo4j Browser and Bolt listen on local ports 7474 and 7687; UModel uses port 8080. Graph data uses the `neo4j-data` volume, while workspace metadata uses `umodel-neo4j-metadata`. Keep both volumes when restarting.

To connect UModel to an existing database, pass `NEO4J_URI`, `NEO4J_USERNAME`, `NEO4J_PASSWORD`, and `NEO4J_DATABASE` to the container and override its command with `--addr :8080 --data /data --graphstore neo4j`. See [GraphStore Providers](../docs/en/graphstore-providers.md#neo4j) for configuration and query limits.

## Ports And Data

| Setting | Default | Notes |
|---|---|---|
| API port | `8080` | Exposed as `http://localhost:8080`. |
| Data directory | `/data` | Mounted to the `umodel-data` Docker volume in Compose. |
| GraphStore provider | `file.memory` | Stores JSON snapshots under `/data/graphstore/file-memory/`. |

Workspace metadata is persisted separately at `/data/workspaces.json` when using `file.memory`.

## Import The Demo In Docker

After the server starts:

```bash
go run ./cmd/umctl --addr http://localhost:8080 workspace create demo '{"name":"Demo"}'
curl -X POST http://localhost:8080/api/v1/samples/demo/multi-domain-quickstart:import \
  -H 'Content-Type: application/json' \
  -d '{}'
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
```

## Compatibility Notes

- The JSON files written by `file.memory` are useful for local inspection and demos, but they are not a long-term storage compatibility contract.
- Do not run multiple writers against the same file-memory directory.
- When upgrading between development builds, prefer exporting or re-importing example data rather than depending on raw JSON layout stability.
- If a future release changes storage layout, document the migration in [CHANGELOG.md](../CHANGELOG.md).
