# Loki Storage 指南

English：[Loki Storage](../../en/guides/loki-storage.md)

将 Loki 建模为 `loki` Storage，并关联到 `log_set` 或 `metric_set`。Query Service 为 `get_logs` 和日志派生的 `get_metrics` 返回 `loki_logql` 计划，由调用方访问 Loki 执行。Loki 是遥测 Storage，不是 GraphStore provider。

## 建模和查询

[Loki 示例包](../../../examples/loki-storage/README.md)包含 EntitySet、LogSet、MetricSet、Loki Storage 和两层链接。`StorageLink.spec.fields_mapping` 把 DataSet 字段映射到 Loki **流标签**。实体字段必须同时具备 DataLink 和 StorageLink 映射才能进入 Loki 查询；日志正文中的字段不在此映射契约内。

导入到已有的 `demo` 工作区：

```bash
go run ./cmd/umctl --addr http://localhost:8080 umodel import demo examples/loki-storage
```

携带明确的时间范围和实体属性生成日志计划：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo --plan-format agent \
  --from '2026-11-03T10:00:00+08:00' --to '2026-11-03T11:00:00+08:00' \
  --entity-data '{"header":["id","environment"],"data":[["node-01","prod"]]}' \
  ".entity_set with(domain='ops', name='ops.node') | entity-call get_logs('ops', 'ops.node.logs', query='level = \"ERROR\"', storage_kind='loki')"
```

使用 `get_metrics('ops', 'ops.node.metrics', 'error_rate', step='30s', storage_kind='loki')` 查询示例中的 LogQL 指标。日志和区间指标都需要 `--from`、`--to`；即时指标以 `--to` 为评估时间。调用方应先确定时间，再请求 Loki 计划。

计划包含 `query.dialect=loki_logql`、`query.queries[].logql`、端点、API 前缀、模式及相关 HTTP 参数。日志和区间指标走 `/query_range`，即时指标走 `/query`。多行 EntityData 分别生成 LogQL，保留各实体的属性组合。无法转换的过滤条件或缺失的标签映射会返回错误，不会被省略。

## 执行计划

仓库附带的只读辅助脚本从标准输入读取 agent 格式计划：

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo --plan-format agent \
  --from '2026-11-03T10:00:00+08:00' --to '2026-11-03T11:00:00+08:00' \
  --entity-data '{"header":["id"],"data":[["node-01"]]}' \
  ".entity_set with(domain='ops', name='ops.node') | entity-call get_logs('ops', 'ops.node.logs', storage_kind='loki')" \
  | python3 skills/umodel-query/scripts/run_loki_plan.py
```

示例只提供模型定义。请将 `spec.endpoint` 指向真实 Loki 服务，并确保流标签与 StorageLink 映射一致。可选认证在 `spec.auth` 中配置 `type: bearer` 和 `token_env`，或配置 `type: basic`、`username_env`、`password_env`；环境变量值只设置在调用进程中。计划只包含环境变量名，不包含凭据值。若可信代理已写入 `X-Scope-OrgID`，无需设置 `spec.tenant`。

日志条数的优先级是显式 `QueryRequest.limit`（或 SPL `| limit`）高于 `Storage.default_size`；请求 limit 高于 SPL limit。`Storage.max_size` 限制最终条数。辅助脚本在合并多条日志查询后排序、去重并截取；指标结果保留各查询名称及 Loki vector/matrix 值。

另见 [Query Service 指南](query-service.md)、[CLI 参考](../reference/cli.md)和 [Loki HTTP API](https://grafana.com/docs/loki/latest/reference/loki-http-api/)。
