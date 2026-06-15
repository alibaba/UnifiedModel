# Schema Reference

UModel schemas are organized as a layered standard. This reference documents the **L0 Core** abstractions — the base vocabulary every model is built from.

## Standard layers

| Layer | Name | Status |
|---|---|---|
| **L0** | **UModel Core** — EntitySet, DataSet, Link, Storage | Documented below |
| L1 | Semantic Conventions — shared service/host/pod/database semantics | Roadmap |
| L2 | Domain Profiles — DevOps, APM, Kubernetes, AIOps, … | Roadmap |
| L3 | Conformance — automated standard-compliance checks | Roadmap |

See the contribution guide for how to propose L1–L3 content.

## L0 Core abstractions

### EntitySet

- [entity_set](./core/entity-set)

### DataSet

- [entity_source](./core/dataset/entity-source)
- [event_set](./core/dataset/event-set)
- [explorer](./core/dataset/explorer)
- [log_set](./core/dataset/log-set)
- [metric_set](./core/dataset/metric-set)
- [profile_set](./core/dataset/profile-set)
- [runbook_set](./core/dataset/runbook-set)
- [trace_set](./core/dataset/trace-set)

### Link

- [data_link](./core/link/data-link)
- [entity_set_link](./core/link/entity-set-link)
- [entity_source_link](./core/link/entity-source-link)
- [explorer_link](./core/link/explorer-link)
- [runbook_link](./core/link/runbook-link)
- [storage_link](./core/link/storage-link)

### Storage

- [aliyun_prometheus](./core/storage/aliyun-prometheus)
- [elasticsearch](./core/storage/elasticsearch)
- [external_storage](./core/storage/external-storage)
- [mysql](./core/storage/mysql)
- [prometheus](./core/storage/prometheus)
- [sls_entitystore](./core/storage/sls-entitystore)
- [sls_logstore](./core/storage/sls-logstore)
- [sls_metricstore](./core/storage/sls-metricstore)

### Building blocks

- [Shared types](./shared-types)
