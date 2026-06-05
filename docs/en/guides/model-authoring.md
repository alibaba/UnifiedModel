# Model Authoring Guide

中文：[模型编写指南](../../zh/guides/model-authoring.md)

Model pack authoring and import workflow.


## Model Pack Shape

A model pack is a directory of YAML or JSON UModel elements. The multi-domain quickstart example uses this shape:

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

Keep categories separated for reviewable diffs. The quickstart pack includes a compact entity topology plus a small DevOps observability chain: `metric_set`, `log_set`, `event_set`, their `data_link` / `storage_link` definitions, and storage definitions for Prometheus, Elasticsearch, and MySQL.

## Authoring Order

1. Define EntitySets.
2. Connect EntitySets to each other with EntitySetLinks when topology matters.
3. Add small sample entity and relation data.
4. Document example queries.

## Minimal EntitySet

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

## Minimal EntitySetLink

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

## Validate Examples

Run the repository example validator:

```bash
make example-validate
```

For generated schema and SDK changes:

```bash
make expand
make verify
```

## Import Into A Workspace

```bash
go run ./cmd/umctl --addr http://localhost:8080 workspace create demo '{"name":"Demo"}'
go run ./cmd/umctl --addr http://localhost:8080 umodel import demo examples/quickstart-multidomain
```

## Inspect The Model

```bash
go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | sort kind,name | limit 50"
```

## Review Checklist

- Names are stable and domain-scoped.
- EntitySet fields use durable identity fields.
- Topology relation names are explicit.
- The quickstart pack stays free of DataSet, DataLink, and StorageLink definitions.
- The pack includes sample data when possible.
- The README explains what the model demonstrates.
