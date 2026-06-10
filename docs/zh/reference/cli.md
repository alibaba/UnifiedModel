# CLI 参考

English: [CLI Reference](../../en/reference/cli.md)

`umctl` 是本地 UModel REST API 的公共 CLI。


仓库根目录命令：

```bash
go run ./cmd/umctl --addr http://localhost:8080 help
```

构建本地二进制：

```bash
make build-cli
./bin/umctl --addr http://localhost:8080 help
```

将 `umctl` 安装到当前 Go bin 目录，便于通过 `PATH` 直接识别：

```bash
make install-cli
umctl --addr http://localhost:8080 help
```

## 全局参数

| 参数 | 默认值 | 描述 |
|---|---|---|
| `--addr` | `http://localhost:8080` | `umodel-server` 的 base URL。 |

## Server Quickstart 参数

`umodel-server` 在开始处理请求前预加载内置 demo：

```bash
go run ./cmd/umodel-server --quickstart
```

| 参数 | 默认值 | 描述 |
|---|---|---|
| `--quickstart` | `false` | 监听前创建 quickstart workspace，并导入内置样例数据。除非显式设置 `--graphstore`，否则使用 `memory`。 |
| `--quickstart-workspace` | `demo` | `--quickstart` 使用的 workspace id。 |
| `--quickstart-sample` | `multi-domain-quickstart` | `--quickstart` 导入的样例包。 |

## 命令组

| 组 | 命令 | 用途 |
|---|---|---|
| `workspace` | `create`, `get`, `list`, `update`, `delete` | 管理 workspace 元数据。 |
| `umodel` | `put`, `delete`, `import`, `export`, `validate` | 写入、校验、导入、导出 UModel 定义。 |
| `entity` | `write`, `delete`, `expire` | 写入或过期 entity records。 |
| `topo` | `write`, `delete`, `expire` | 写入或过期 relation records。 |
| `query` | `run`, `explain`, `examples` | 通过 Query Service 读取模型、实体、拓扑。 |
| `agent` | `discover`, `tool`, `mcp` | 查看 Agent 元数据并执行安全工具。 |

Entity 和 topology 没有独立读取命令；所有读取使用 `query run` 或 `query explain`。

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

`umodel put` 和 `umodel validate` 接受 JSON object、JSON array，或包含 `elements` 字段的 payload。

## EntityStore

```bash
go run ./cmd/umctl --addr http://localhost:8080 entity write demo /tmp/entity.json
go run ./cmd/umctl --addr http://localhost:8080 entity expire demo devops/devops.service/10000000000000000000000000000101
go run ./cmd/umctl --addr http://localhost:8080 topo write demo /tmp/relation.json
go run ./cmd/umctl --addr http://localhost:8080 topo delete demo devops/devops.service/10000000000000000000000000000101/runs/k8s/k8s.workload/20000000000000000000000000000201
```

写入命令接受 JSON object、array，或包含 `entities` / `relations` 字段的 payload。

## Query

```bash
go run ./cmd/umctl --addr http://localhost:8080 query examples
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
go run ./cmd/umctl --addr http://localhost:8080 query explain demo ".entity with(domain='devops', name='devops.service') | limit 5"
```

参考：[Query Service 指南](../guides/query-service.md)。

## Agent

```bash
go run ./cmd/umctl --addr http://localhost:8080 agent discover demo
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_examples '{}'
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_explain '{"query":".umodel | limit 5"}'
```

`agent mcp` 提示使用 `umodel-mcp` binary 处理 stdio MCP workflows。
