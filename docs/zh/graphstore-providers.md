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
| `neo4j` | 远程 Neo4j 数据库 | 通过 Bolt 持久化节点和拓扑边，受控只读 Cypher 使用共享 Go 引擎，无需 build tag 或本地原生运行库。 |

## `neo4j`

默认 `neo4j` 方言面向 Neo4j 5.26 或更新版本。同一 provider 还提供 `opencypher` 兼容方言，用于兼容 Bolt 的数据库。Provider 使用官方 Go driver v5，保留仓库的 Go 1.22 构建基线。`umodel-server` 和 `umodel-mcp` 均支持 `--graphstore neo4j`。

```bash
export NEO4J_URI=bolt://localhost:7687
export NEO4J_USERNAME=neo4j
export NEO4J_PASSWORD='replace-with-your-password'
export NEO4J_DATABASE=neo4j
go run ./cmd/umodel-server --addr :8080 --data data --graphstore neo4j
```

| 环境变量 | `ProviderConfig.Options` 键 | 默认值 |
|---|---|---|
| `NEO4J_URI` | `uri` | `bolt://localhost:7687` |
| `NEO4J_USERNAME` | `username` | `neo4j` |
| `NEO4J_PASSWORD` | `password` | 必填，无默认值 |
| `NEO4J_DIALECT` | `dialect` | `neo4j`，也可设为 `opencypher` |
| `NEO4J_DATABASE` | `database` | 原生模式为 `neo4j`；`opencypher` 模式为空，使用服务端默认库 |
| `NEO4J_TIMEOUT` | `timeout` | 每次数据库操作 `10s` |

代码中的 options 优先于环境变量。原生模式下，`neo4j://` 启用路由，`neo4j+s://` 或 `bolt+s://` 使用验证证书的 TLS。原生模式启动时会连接所选数据库并创建约束和索引，因此数据库必须预先存在，账号需要 schema 和数据读写权限。两种模式下连接或认证失败时启动都会报错退出。

Provider 使用 `(workspace, key)` 标识 `UModelElement` 和 `UModelEntity` 节点，实体节点之间保存 `UMODEL_RELATION` 边。JSON 属性保留嵌套 payload 和整数精度。关系可先于实体写入，其占位端点不会出现在实体查询中。原生模式下，每个写入批次使用事务和数据库工作区锁，多个 provider 进程并发写入时仍保持 `Create`、`Update`、`Expire`、`Delete` 语义。事务失败时不会返回已接受的写入。

每次查询从 Neo4j 读取所选工作区的记录，再由共享 Go 引擎执行。受控 Cypher 是只读子集，不是原生 Neo4j Cypher 透传。过滤和遍历尚未下推到 Neo4j；读取开销和应用内存随工作区集合大小增长。`Expire` 记录可通过历史查询读取；`Delete` 记录仍隐藏，与 `memory` 一致。

Workspace 元数据仍由 Workspace Service 管理，保存在 `<data-root>/workspaces.json`。重启时保留 `--data`，并将其与数据库一起备份。原生模式的图数据写入可跨进程协调，但 workspace 元数据属于各服务实例本地文件，并非多实例共享元数据存储。

使用 Docker Compose 同时启动 Neo4j 和 UModel：

```bash
export NEO4J_PASSWORD='replace-with-your-password'
docker compose -f deployments/compose/docker-compose.neo4j.yaml up --build
```

对测试数据库执行 provider 合约与重启测试：

```bash
UMODEL_TEST_NEO4J=1 make test-neo4j
```

测试创建唯一工作区，只清理自己创建的记录。CI 使用 Neo4j Community 5.26 验证两种方言。普通 `make test-service` 不依赖 Neo4j 实例。

### openCypher 兼容模式

同一 provider 设置 `NEO4J_DIALECT=opencypher` 即可选择兼容模式。面向阿里云 GDB OpenCypher 内核等数据库的配置示例：

```bash
export NEO4J_DIALECT=opencypher
export NEO4J_URI='bolt://your-gdb-endpoint:your-port'
export NEO4J_USERNAME='your-database-user'
export NEO4J_PASSWORD='replace-with-your-password'
export NEO4J_DATABASE=''
export NEO4J_TIMEOUT=30s
go run ./cmd/umodel-server --addr :8080 --data data --graphstore neo4j
```

使用数据库服务提供的地址和端口。如果后端提供 TLS，也可使用 `bolt+s://` 或 `bolt+ssc://`。该模式拒绝 `neo4j://` 及其变体等路由协议。空数据库名使用服务端默认库，避免 Bolt v3 的多数据库错误；只有后端支持时才能指定命名数据库。连接 GDB 时，应显式清空继承的 `NEO4J_DATABASE=neo4j`。

| 能力 | `neo4j` | `opencypher` |
|---|---|---|
| 启动 | 创建原生约束和索引 | 使用 `RETURN 1` 检查连接，不发送 schema DDL |
| Bookmark 共享 | 启用 | 关闭 |
| 驱动遥测 | 驱动默认行为 | 关闭 |
| 工作区写入协调 | 数据库锁与唯一约束 | 单个 provider 实例内串行写入 |
| 模型、实体、关系、历史查询、受控 Cypher | 共享语义 | 共享语义 |

**openCypher 模式下，每个数据库只运行一个可写 provider 实例。** 同一实例的并发写请求会串行执行，包括工作区初始化；等待写入时会响应取消和操作超时。不要让独立的 server/MCP 写入进程或外部 writer 同时修改这些记录。该限制是部署要求，不是分布式锁：通用 Cypher 不能提供跨实例的原生唯一性和锁保证。读取遇到重复工作区标记或记录键时，provider 会报错，不会静默选取其中一条。索引及外部约束仍由数据库服务或运维人员管理。

`/healthz` 保持 `graphstore.provider=neo4j`，通过 `graphstore.message` 标明兼容模式及单写入实例要求。兼容模式仍使用前述本地工作区元数据和共享 Go 查询引擎。

[GDB 兼容性文档](https://help.aliyun.com/zh/gdb/developer-reference/compatibility-of-gdb-cypher)列出了 Bolt v3、不支持 Bookmark 以及独立索引/约束管理等差异。后端兼容性仍需在实际服务上验证事务回滚、各写入方法、重启和查询行为。仓库提供 Bolt v3 协议测试端点，并在 Neo4j 上验证两种方言；这些测试不等于真实 GDB 实例验证。

配置专用测试数据库后，使用相同测试集验证兼容模式：

```bash
NEO4J_DIALECT=opencypher NEO4J_DATABASE='' UMODEL_TEST_NEO4J=1 make test-neo4j
```

该模式跳过原生模式的跨 provider 并发测试，保留单 provider 内的并发请求测试。测试期间不要让其他 writer 使用该数据库。

测试包串行执行（`go test -p 1`），避免合约和 E2E 测试包同时创建可写 provider 实例。

### 已验证的开源后端

已在 Podman 的 Linux ARM64 容器中，使用 `opencypher` 方言验证 ArcadeDB 26.9.1。GraphStore 合约、生命周期/隔离/回滚、单实例并发 Create、quickstart 应用重启 E2E 均通过，分别覆盖了 `database=neo4j` 和数据库名留空两种配置。这些结果验证的是 ArcadeDB，不代表阿里云 GDB 验收。

启动相同的隔离测试后端：

```bash
podman run -d --name umodel-opencypher-e2e \
  --memory=1536m --cpus=2 \
  -p 127.0.0.1:17688:7687 \
  -e 'JAVA_OPTS=-Xms256m -Xmx768m -Darcadedb.server.rootPassword=umodel-e2e-password -Darcadedb.server.defaultDatabases=neo4j[root] -Darcadedb.server.plugins=Bolt:com.arcadedb.bolt.BoltProtocolPlugin' \
  docker.io/arcadedata/arcadedb:26.9.1
podman logs umodel-opencypher-e2e
```

日志显示服务启动完成后，执行：

```bash
NEO4J_DIALECT=opencypher NEO4J_DATABASE='' \
NEO4J_URI=bolt://127.0.0.1:17688 \
NEO4J_USERNAME=root NEO4J_PASSWORD=umodel-e2e-password \
NEO4J_TIMEOUT=30s UMODEL_TEST_NEO4J=1 make test-neo4j
podman stop umodel-opencypher-e2e
podman rm -v umodel-opencypher-e2e
```

上述凭据仅用于一次性的本地测试容器。清理后会保留镜像。

## 查看当前 provider

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
