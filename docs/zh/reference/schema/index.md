# Schema 参考

UModel 的 schema 按分层标准组织。本参考记录 **L0 Core** 抽象——构建一切模型的基础词汇。

## 标准分层

| 层级 | 名称 | 状态 |
|---|---|---|
| **L0** | **UModel Core** —— EntitySet、DataSet、Link、Storage | 见下文 |
| L1 | Semantic Conventions —— 通用 service/host/pod/database 语义 | 规划中 |
| L2 | Domain Profiles —— DevOps、APM、Kubernetes、AIOps…… | 规划中 |
| L3 | Conformance —— 自动化标准兼容性校验 | 规划中 |

L1–L3 的贡献方式见贡献指南。

## L0 Core 抽象

### EntitySet（实体集）

- [entity_set](./core/entity-set)

### DataSet（数据集）

- [entity_source](./core/dataset/entity-source)
- [event_set](./core/dataset/event-set)
- [explorer](./core/dataset/explorer)
- [log_set](./core/dataset/log-set)
- [metric_set](./core/dataset/metric-set)
- [profile_set](./core/dataset/profile-set)
- [runbook_set](./core/dataset/runbook-set)
- [trace_set](./core/dataset/trace-set)

### Link（链接）

- [data_link](./core/link/data-link)
- [entity_set_link](./core/link/entity-set-link)
- [entity_source_link](./core/link/entity-source-link)
- [explorer_link](./core/link/explorer-link)
- [runbook_link](./core/link/runbook-link)
- [storage_link](./core/link/storage-link)

### Storage（存储）

- [aliyun_prometheus](./core/storage/aliyun-prometheus)
- [elasticsearch](./core/storage/elasticsearch)
- [external_storage](./core/storage/external-storage)
- [mysql](./core/storage/mysql)
- [prometheus](./core/storage/prometheus)
- [sls_entitystore](./core/storage/sls-entitystore)
- [sls_logstore](./core/storage/sls-logstore)
- [sls_metricstore](./core/storage/sls-metricstore)

### 构建块

- [共享类型](./shared-types)
