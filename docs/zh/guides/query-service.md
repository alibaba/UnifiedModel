# Query Service 指南

English: [Query Service Guide](../../en/guides/query-service.md)

Query Service 是 UModel 定义、实体、关系、拓扑和 EntitySet 调用规划的唯一公共读取路径。它接受以 `.umodel`、`.entity_set`、`.entity`、`.topo` 或 `.runbook_set` 开头的 SPL 字符串。


## 为什么读取统一走 Query Service

UModel 不暴露分散的公共读取 API，例如 entity lookup、relation lookup、graph traversal 或 model search endpoint。统一读取面让 CLI、Web UI、REST API、MCP tools 和 SDK 保持一致。

## 入口

REST：

```http
POST /api/v1/query/{workspace}/execute
POST /api/v1/query/{workspace}/explain
```

CLI：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"
go run ./cmd/umctl --addr http://localhost:8080 query explain demo ".umodel | limit 5"
```

Agent tool：

```bash
go run ./cmd/umctl --addr http://localhost:8080 agent tool demo query_spl_execute '{"query":".umodel | limit 5"}'
```

## `.umodel`

读取 UModel 定义：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel with(kind='entity_set') | sort name | limit 20"
```

常见读取：

- 列出 EntitySet。
- 查看 metric、log、trace、event、storage、link 定义。
- 支撑 Web UI Explorer 的图/表视图。

## `.entity`

读取运行时实体：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__,display_name | limit 20"
```

Agent 和 REST 调用方可以把命名参数绑定到 `with(...)` filters 和 `where` predicates：

```json
{
  "query": ".entity with(domain='devops', name='devops.service', query=$query) | limit 20",
  "parameters": {
    "query": "checkout"
  }
}
```

## `.entity_set`

`.entity_set` 用于处理 EntitySet 方法调用，返回与 UModel Assistant 一致的响应数据。本轮范围支持元信息/发现类方法，返回 `responseType=2` 以及 `header`/`data`。

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()"
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)"
```

`domain` 和 `name` 是必填 filter；`ids` 可作为 EntitySet 调用上下文。当前支持的方法是 `__list_method__` 和 `list_data_set`（兼容 `list_dataset` 别名）；方法参数按 UModel Assistant 的签名校验。

## `.topo`

读取运行时拓扑关系：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest | limit 20"
```

`.topo` 支持 graph-call 风格的拓扑操作。`memory`、`file.memory` 和可选的 `local.ladybug` provider 都通过共享的 Go engine 支持受控只读 Cypher 兼容查询。`local.ladybug` 在使用 `-tags ladybug` 和本地 Ladybug runtime 构建时，仍然把图数据持久化到 Ladybug。

Cypher 可以在一次查询里返回完整实体属性和关系属性：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".topo | graph-call cypher(`MATCH (src)-[r]->(dest) RETURN src, r AS relation, dest LIMIT 20`)"
```

调用方如果希望显式表达属性 map 返回形态，可以使用 `properties(src)`、`properties(r)` 和 `properties(dest)`。

## 常用管道操作

- `with(...)`：source-specific 过滤。
- `project`：选择字段。
- `sort`：排序。
- `limit`：限制输出。
- `entity-call`：EntitySet 方法调用规划。
- `graph-call`：拓扑函数。

查看内置示例：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query examples
```

## Explain

```bash
go run ./cmd/umctl --addr http://localhost:8080 query explain demo ".entity with(domain='devops', name='devops.service') | limit 5"
```

Explain 输出包含 source、provider、storage provider、filters 和 limits。

## 边界规则

- 不新增 Query Service 之外的公共 entity/relation/topology 读取 endpoint。
- CLI 领域读取保持在 `query run` 和 `query explain` 后面。
- AgentGateway resources 保持 metadata-only，运行时 rows 通过 tools 返回。
