# 模型编写指南

English: [Model Authoring Guide](../../en/guides/model-authoring.md)

UModel 模型包编写和导入流程。


## 模型包结构

模型包是一个包含 YAML 或 JSON UModel elements 的目录。多域 quickstart 样例使用以下结构：

```text
examples/quickstart-multidomain/
├── devops/
│   └── entity_set/
├── automaker/
│   └── entity_set/
├── game/
│   └── entity_set/
├── supplier/
│   └── entity_set/
├── k8s/
│   └── entity_set/
├── cross-domain/
│   └── link/entity_set_link/
└── sample-data/
```

按类别拆分目录，保持 diff 易审阅。Quickstart 包包含紧凑的实体拓扑，以及一条小型 DevOps 可观测链路：`metric_set`、`log_set`、`event_set`，对应的 `data_link` / `storage_link`，以及 Prometheus、Elasticsearch、MySQL storage 定义。

## 推荐编写顺序

1. 定义 EntitySet。
2. 有拓扑语义时，用 EntitySetLink 连接 EntitySet。
3. 添加小规模 sample entity/relation 数据。
4. 补充 README 和示例查询。

## 最小 EntitySet

```yaml
kind: entity_set
schema:
  url: "umodel.aliyun.com"
  version: "v0.1.0"
metadata:
  name: "demo.service"
  domain: demo
spec:
  fields:
    - name: service_id
      type: string
    - name: service
      type: string
```

## 最小 EntitySetLink

```yaml
kind: entity_set_link
schema:
  url: "umodel.aliyun.com"
  version: "v0.1.0"
metadata:
  name: "demo.service_calls_demo.service"
  domain: demo
spec:
  src:
    domain: demo
    kind: entity_set
    name: demo.service
  dest:
    domain: demo
    kind: entity_set
    name: demo.service
  entity_link_type: calls
```

## 校验示例

```bash
make example-validate
```

生成 schema 和 SDK 变化时：

```bash
make expand
make verify
```

## 导入到 Workspace

```bash
go run ./cmd/umctl --addr http://localhost:8080 workspace create demo '{"name":"Demo"}'
go run ./cmd/umctl --addr http://localhost:8080 umodel import demo examples/quickstart-multidomain
```

## 检查模型

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | sort kind,name | limit 50"
```

## 审阅清单

- 名称稳定且带 domain。
- EntitySet 字段包含稳定身份字段。
- 拓扑关系名称明确。
- Quickstart 包保持不包含 DataSet、DataLink 和 StorageLink 定义。
- 尽可能包含 sample data。
- README 列出模型场景、资产和查询。
