# CLI Reference

中文：[CLI 参考](../../zh/reference/cli.md)

`umctl` is the public CLI for the local UModel REST API.

Repository-root command:

```bash
go run ./cmd/umctl --addr http://localhost:8080 help
```

Build a local binary:

```bash
make build-cli
./bin/umctl --addr http://localhost:8080 help
```

Install `umctl` into the active Go bin directory so it can be found from `PATH`:

```bash
make install-cli
umctl --addr http://localhost:8080 help
```

## Global Options

| Option | Default | Description |
|---|---|---|
| `--addr` | Resolved from config | Base URL for `umodel-server`. Highest priority when set. |
| `--profile` | Current config profile | Configuration profile to use. |
| `--output`, `-o` | Config default or `json` | Output format. Use `text` for a readable key/value view. |

Commands return a non-zero exit code for HTTP errors and for successful HTTP responses whose top-level `failed` field is greater than `0`.

## Address Resolution And Profiles

`umctl` resolves the server address in this order:

1. `--addr`
2. `UMCTL_ADDR`
3. The selected profile in `~/.umctl/config.yaml`
4. The built-in default profile, `http://localhost:8080`, when no config file exists

If `~/.umctl/config.yaml` exists but is not valid YAML, the command exits with code `1` and prints guidance to check the config syntax. If the selected profile exists but leaves `addr` empty, server-backed commands exit with code `1` and suggest `umctl configure` or `--addr`.

Configure or inspect profiles:

```bash
umctl configure
umctl --profile dev configure
umctl configure list
umctl configure show
```

Config file schema:

```yaml
current: default
output_format: json
profiles:
  default:
    addr: http://localhost:8080
  dev:
    addr: http://127.0.0.1:8080
```

## Server Quickstart Flags

`umodel-server` preloads the bundled demo before it starts serving requests:

```bash
go run ./cmd/umodel-server --quickstart
```

| Flag | Default | Description |
|---|---|---|
| `--quickstart` | `false` | Create the quickstart workspace and import bundled sample data before listening. Uses `memory` unless `--graphstore` is explicitly set. |
| `--quickstart-workspace` | `demo` | Workspace id used by `--quickstart`. |
| `--quickstart-sample` | `multi-domain-quickstart` | Sample package imported by `--quickstart`. |
| `--import-root` | current working directory | Confine UModel API imports (`umctl umodel import`, `POST /api/v1/umodel/{workspace}/import`) to this directory. Paths outside it are rejected. Pass `/` to allow any path. Bundled `--quickstart` sample loads are never confined. |

## Command Groups

| Group | Commands | Purpose |
|---|---|---|
| `workspace` | `create`, `get`, `list`, `update`, `delete` | Manage workspace metadata. |
| `umodel` | `put`, `delete`, `import`, `export`, `validate` | Write, validate, import, and export UModel definitions. |
| `entity` | `write`, `delete`, `expire` | Write or expire entity records. |
| `topo` | `write`, `delete`, `expire` | Write or expire relation records. |
| `query` | `run`, `explain`, `examples` | Read model, entity, and topology data through Query Service. |
| `agent` | `discover`, `tool`, `mcp` | Inspect agent metadata and execute safe tools. |
| `configure` | `list`, `show` | Create and inspect local CLI profiles. |
| `meta` | `export` | Export CLI command metadata for agent discovery. |
| `version` | | Show build version, git commit, and build time. |

Entity and topology read commands are intentionally absent. Use `query run` or `query explain` for all reads.

## Workspace

```bash
go run ./cmd/umctl --addr http://localhost:8080 workspace create demo '{"name":"Demo"}'
go run ./cmd/umctl --addr http://localhost:8080 workspace get demo
go run ./cmd/umctl --addr http://localhost:8080 workspace list
go run ./cmd/umctl --addr http://localhost:8080 workspace update demo '{"description":"Updated"}'
go run ./cmd/umctl --addr http://localhost:8080 workspace delete demo
```

## UModel

```bash
go run ./cmd/umctl --addr http://localhost:8080 umodel validate demo examples/quickstart-multidomain/devops/entity_set/devops.service.yaml
go run ./cmd/umctl --addr http://localhost:8080 umodel put demo '{"kind":"entity_set","domain":"devops","name":"devops.service"}'
go run ./cmd/umctl --addr http://localhost:8080 umodel import demo examples/quickstart-multidomain
go run ./cmd/umctl --addr http://localhost:8080 umodel export demo 100
```

`umodel put` and `umodel validate` accept a JSON object, a JSON array, or a JSON payload with an `elements` field.

## EntityStore

```bash
go run ./cmd/umctl --addr http://localhost:8080 entity write demo /tmp/entity.json
go run ./cmd/umctl --addr http://localhost:8080 entity expire demo devops/devops.service/10000000000000000000000000000101
go run ./cmd/umctl --addr http://localhost:8080 topo write demo /tmp/relation.json
go run ./cmd/umctl --addr http://localhost:8080 topo delete demo devops/devops.service/10000000000000000000000000000101/runs/k8s/k8s.workload/20000000000000000000000000000201
```

Write commands accept JSON objects, arrays, or payloads with `entities` or `relations`.

## Query

```bash
go run ./cmd/umctl --addr http://localhost:8080 query examples
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
go run ./cmd/umctl --addr http://localhost:8080 query explain demo ".entity with(domain='devops', name='devops.service') | limit 5"
```

`query examples` prints offline bootstrap SPL examples so it works without a workspace-specific server call. For canonical runtime examples exposed by the server, use:

```bash
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_examples '{}'
```

Pass SPL as one quoted argument when preserving whitespace matters. If SPL is passed as multiple shell arguments, `umctl` joins them with a single space.

Reference: [Query Service Guide](../guides/query-service.md).

## Agent

```bash
go run ./cmd/umctl --addr http://localhost:8080 agent discover demo
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_examples '{}'
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_explain '{"query":".umodel | limit 5"}'
```

`agent mcp` prints a reminder to use the `umodel-mcp` binary for stdio MCP workflows.

## Metadata And Version

```bash
umctl meta export
umctl version
```

`meta export` emits the registered CLI command metadata as JSON for agent discovery. `version` prints the version, git commit, and build time. `make build-cli` and `make install-cli` inject these values through Go linker flags; override `VERSION` when building a named release:

```bash
make build-cli VERSION=0.1.0
./bin/umctl version
```
