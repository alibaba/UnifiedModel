# GraphStore Providers

English: [GraphStore Providers](../en/graphstore-providers.md)

UModel 通过 `GraphStore` 接口保存和查询 UModel elements、CMS 2.0 实体以及拓扑关系。运行时通过 `--graphstore` 选择 provider。


未显式传入 `--graphstore` 时，Go 入口默认使用 `local.ladybug`。如果构建时没有 `-tags ladybug`，该 provider 会报告不可用状态。使用 `-tags ladybug` 构建即可启用真实的 `local.ladybug` provider；本地开发且不使用 Ladybug 时请显式传入 `--graphstore file.memory`。

## Providers

| Provider | 持久化 | 典型用途 |
|---|---|---|
| `memory` | 进程内存 | 快速测试和一次性本地实验，进程退出后数据消失。 |
| `file.memory` | `--data` 下的 JSON 文件 | 本地开发和文档演示的默认选择，重启后数据保留。 |
| `local.ladybug` | Ladybug 数据库文件 | Ladybug-backed 环境，需要 `-tags ladybug` 和本地 Ladybug runtime。 |

启动示例：

```bash
go run ./cmd/umodel-server --addr :8080 --data data --graphstore file.memory
```

当前 provider 位置：

- `GET /healthz` 的 `graphstore.provider`
- Query explain 的 `provider` 和 `storage_provider`

## `file.memory` 布局

`file.memory` 将数据保存在：

```text
<data-root>/graphstore/file-memory/
└── workspaces/
    └── demo/
        ├── umodels.json
        ├── entities.json
        └── relations.json
```

Workspace 元数据单独保存在：

```text
<data-root>/workspaces.json
```

## 边界

- `file.memory` 面向单本地进程，不要让多个 writer 写同一个目录。
- JSON 文件服务于检查和演示，不是长期兼容性存储合约。
- 运行时读取仍通过 Query Service，不应绕过 `.umodel`、`.entity`、`.topo`。

## 烟测

```bash
go run ./cmd/umodel-server --addr :8080 --data /tmp/umodel-demo --graphstore file.memory
go run ./cmd/umctl --addr http://localhost:8080 umodel put demo '{"id":"devops.service","kind":"entity_set","domain":"devops","name":"devops.service"}'
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
find /tmp/umodel-demo/graphstore/file-memory -maxdepth 4 -type f
```
