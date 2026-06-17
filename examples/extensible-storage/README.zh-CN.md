# 可扩展存储：用 YAML 接入 PromQL 兼容后端

这个示例在不写任何 UModel 代码的前提下，新增 [VictoriaMetrics](https://victoriametrics.com/) 作为指标后端。

VictoriaMetrics 使用 Prometheus 查询 API，与 Prometheus 共享同一查询模型——`label-timeseries`
家族。UModel 按**家族**而非写死的存储 kind 把存储路由到查询计划渲染器。存储通过 `spec.family`
选择家族：

```yaml
kind: victoriametrics
spec:
  family: label-timeseries
  endpoint: "http://localhost:8428"
```

声明之后，对以该存储为后端的实体执行 `get_metrics`，生成的查询计划与原生 Prometheus 存储完全一致
（`prometheus_promql`）：复用现有的 `label-timeseries` 渲染器。没有 VictoriaMetrics 渲染器，也没有新增 Go 代码。

新增 `victoriametrics` kind 只动了 schema：

- [`schemas/core/storage/victoriametrics.schema.yaml`](../../schemas/core/storage/victoriametrics.schema.yaml) 定义该 kind。
- Go、Python、Java 三套 SDK 类型由 `make expand` 从该 schema 生成。
- [`victoriametrics.storage.yaml`](victoriametrics.storage.yaml) 是本示例校验的存储定义。

使用不同查询模型（例如 SQL）的后端，只需为该家族写一个渲染器；该家族此后的每个后端都只是配置。
