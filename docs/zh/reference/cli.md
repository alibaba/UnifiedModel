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
| `--addr` | 从配置解析 | `umodel-server` 的 base URL。设置后优先级最高。 |
| `--profile` | 当前配置 profile | 要使用的本地配置 profile。 |
| `--output`, `-o` | 配置默认值或 `json` | 输出格式。使用 `text` 输出更易读的 key/value 文本。 |

当 HTTP 请求失败，或 HTTP 成功但响应顶层 `failed` 字段大于 `0` 时，命令会返回非 0 退出码。

## 地址解析与 Profiles

`umctl` 按以下顺序解析 server 地址：

1. `--addr`
2. `UMCTL_ADDR`
3. `~/.umctl/config.yaml` 中选中的 profile
4. 当本地没有配置文件时，使用内置默认 profile：`http://localhost:8080`

如果 `~/.umctl/config.yaml` 存在但不是合法 YAML，命令会以退出码 `1` 失败，并提示检查配置语法。如果选中的 profile 存在但 `addr` 为空，需要连接 server 的命令会以退出码 `1` 失败，并提示使用 `umctl configure` 或 `--addr` 修复。

配置或查看 profiles：

```bash
umctl configure
umctl --profile dev configure
umctl configure list
umctl configure show
```

配置文件 schema：

```yaml
current: default
output_format: json
profiles:
  default:
    addr: http://localhost:8080
  dev:
    addr: http://127.0.0.1:8080
```

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
| `--import-root` | 当前工作目录 | 将 UModel API 导入（`umctl umodel import`、`POST /api/v1/umodel/{workspace}/import`）限定在该目录内，目录外的路径被拒绝。传 `/` 允许任意路径。`--quickstart` 内置样例加载不受限。 |

## 命令组

| 组 | 命令 | 用途 |
|---|---|---|
| `workspace` | `create`, `get`, `list`, `update`, `delete` | 管理 workspace 元数据。 |
| `umodel` | `put`, `delete`, `import`, `export`, `validate` | 写入、校验、导入、导出 UModel 定义。 |
| `entity` | `write`, `delete`, `expire` | 写入或过期 entity records。 |
| `topo` | `write`, `delete`, `expire` | 写入或过期 relation records。 |
| `query` | `run`, `explain`, `examples` | 通过 Query Service 读取模型、实体、拓扑。 |
| `agent` | `discover`, `tool`, `mcp` | 查看 Agent 元数据并执行安全工具。 |
| `configure` | `list`, `show` | 创建和查看本地 CLI profiles。 |
| `meta` | `export` | 导出 CLI 命令元数据，供 agent discovery 使用。 |
| `version` | | 查看版本、git commit 和构建时间。 |

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

`query examples` 输出离线 bootstrap SPL 示例，因此不需要发起 workspace 相关的 server 调用。需要 server 暴露的运行时 canonical 示例时，使用：

```bash
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_examples '{}'
```

如果需要保留 SPL 中的空白，建议把 SPL 作为一个带引号的参数传入。如果拆成多个 shell 参数，`umctl` 会用单个空格把它们拼接起来。

参考：[Query Service 指南](../guides/query-service.md)。

## Agent

```bash
go run ./cmd/umctl --addr http://localhost:8080 agent discover demo
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_examples '{}'
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_explain '{"query":".umodel | limit 5"}'
```

`agent mcp` 提示使用 `umodel-mcp` binary 处理 stdio MCP workflows。

## 元数据与版本

```bash
umctl meta export
umctl version
```

`meta export` 以 JSON 输出已注册的 CLI 命令元数据，供 agent discovery 使用。`version` 输出版本号、git commit 和构建时间。`make build-cli` 与 `make install-cli` 会通过 Go linker flags 注入这些值；构建命名版本时可以覆盖 `VERSION`：

```bash
make build-cli VERSION=0.1.0
./bin/umctl version
```
