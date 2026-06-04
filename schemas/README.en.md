# UModel Schema Overview

中文版本：[README.md](README.md)

UModel schemas define the syntax and validation rules for model elements. They are the source for generated SDK types, generated HTML references, and example validation.

## Layout

| Path | Purpose |
|---|---|
| `manifest.yaml` | Lists supported model kinds and versions. |
| `base.yaml` | Base schema metadata rules. |
| `includes/` | Reusable schema components such as fields, metrics, metadata, links, and telemetry data. |
| `core/dataset/` | EntitySet and dataset schemas. |
| `core/link/` | Link schemas such as DataLink, EntitySetLink, StorageLink. |
| `core/storage/` | Storage schemas such as SLS and Prometheus storage definitions. |

## Core Model Families

- Entity: `entity_set`.
- Datasets: `metric_set`, `log_set`, `trace_set`, `event_set`, `profile_set`, `runbook_set`.
- Links: `data_link`, `entity_set_link`, `storage_link`, `runbook_link`, and related link kinds.
- Storage: `sls_logstore`, `sls_metricstore`, `sls_entitystore`, `aliyun_prometheus`, `prometheus`, `mysql`, `elasticsearch`, `external_storage`.

## Workflows

Regenerate expanded schemas and SDK assets:

```bash
make expand
```

Generate schema HTML:

```bash
make doc
```

Validate examples:

```bash
make example-validate
```

## Best Practices

- Keep schema definitions modular and versioned.
- Put shared structures under `includes/`.
- Register new model kinds in `manifest.yaml`.
- Update generated SDKs, docs, examples, and tests together.
