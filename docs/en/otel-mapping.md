# UModel ↔ OpenTelemetry Semantic Convention Mapping

**Community Discussion Draft**

> 本文档由信通院（CAICT）发起，作为 UModel 社区和 OpenTelemetry 社区之间语义约定对齐的讨论起点。后续可由信通院发起主题研讨会，邀请相关社区共同讨论和完善。

---

## Document Status

**This is a discussion draft, not a finalized specification.**

This document was prepared by CAICT (China Academy of Information and Communications Technology) as a strawman proposal to facilitate community conversation. It does not represent an official position of the UModel maintainers, the OpenTelemetry project, or the CNCF.

**Process:**

1. **Community review** — Submitted as a PR to `alibaba/UnifiedModel` at `docs/en/otel-mapping.md` for open discussion (this PR)
2. **Workshop discussion** — CAICT may convene a thematic workshop where domain experts from the OTel and UModel communities review and refine this draft
3. **Revision** — Incorporate feedback; re-submit for final review
4. **Publication** — Publish as a stable reference once consensus is reached

An extended version with worked examples and schema design sketches is available from CAICT upon request, intended as workshop discussion material rather than a prescriptive design.

---

## 1. Purpose

This document proposes a conceptual mapping between UModel's data model and OpenTelemetry's semantic conventions. Its goals are:

- **Onboarding**: Help observability engineers familiar with OTel understand UModel concepts in their own vocabulary.
- **Interop**: Explore how OTel signals (traces, metrics, logs) could map to UModel entities, relations, and datasets.
- **Alignment**: Serve as a reference so that future UModel schema contributions can align with OTel conventions where appropriate.

This is a discussion draft, not an implementation guide.

---

## 2. Architecture Alignment

Both systems separate **model definitions** from **runtime instances**:

| Layer | UModel | OpenTelemetry |
|-------|--------|---------------|
| Definition | EntitySet (declares entity shape in schema YAML) | Semantic Convention Registry (declares attribute schemas in YAML) |
| Definition | EntitySetLink (declares relation semantics) | Span Link / Service Map conventions |
| Definition | DataSet (MetricSet, LogSet, TraceSet) | Instrumentation Scope + Pipeline configuration |
| Runtime | Entity record (domain, type, id, properties) | Resource + Span / LogRecord / Metric data point |
| Runtime | Relation record (source, dest, type) | Span Link / Service Map edge |
| Query | SPL (`.entity`, `.topo`, `.umodel`) | OTLP pipeline → backend query language |

A key observation for discussion: an OTel-instrumented service appears to project naturally to a set of UModel EntitySets, Entities, and Relations. Whether this holds across diverse deployment scenarios is one of the questions this draft aims to test.

---

## 3. Proposed Conceptual Mapping

> Each row is open for discussion. Field names, mapping choices, and boundary cases are expected to evolve through community feedback.

### 3.1 Core Concept Map

| UModel Concept | OpenTelemetry Equivalent | Notes |
|---------------|-------------------------|-------|
| EntitySet | Instrumentation Scope | Both define a typed schema for runtime instances |
| Entity (domain, type, id) | Resource Attributes (`service.namespace`, `service.name`, `service.instance.id`) | The three-part entity key maps naturally to OTel's service identity model |
| Relation (topo edge) | Span Link / Service Map Edge | Cross-service relationships captured as topology edges |
| Field constraints | Semantic Convention attributes | Schema-level field definitions correspond to OTel attribute conventions |

### 3.2 Signal Mapping (Initial Proposal)

| OTel Signal | UModel Construct | Notes |
|-------------|-----------------|-------|
| Trace (Span) | TraceSet + Entity (`operation`) + Relations | Spans can project to operation entities; parent-child → `parent_of` relations; CLIENT-SERVER pairs → `calls` relations |
| Metric | MetricSet + DataLink to Entity | Metrics linked to producing entity via DataLink; keyed by `service.name` or trace context |
| Log | LogSet + DataLink to Entity | Logs linked to entity via DataLink; trace context enables correlation |
| Span Event | EventSet | Events as first-class UModel objects when warranted |

### 3.3 Resource Attributes → Entity Identity

UModel entities are identified by a three-part convention (domain, entity type, entity id). The following OTel resource attributes are proposed as the natural mapping:

| OTel Resource Attribute | Role in UModel Entity Identity | Notes |
|--------------------------|-------------------------------|-------|
| `service.namespace` | Domain | Organizational boundary (e.g., "production", "staging") |
| `service.name` | Entity type | The kind of thing this entity is (e.g., a microservice name) |
| `service.instance.id` | Entity id | Unique instance identifier |
| `service.version` | Entity property | Service version as an entity property |
| `deployment.environment` | Entity property | Environment context, may also inform domain |

---

## 4. Open Questions

The following are proposed as discussion topics for the UModel community and for any future thematic workshops convened by CAICT:

1. **Span → Entity volume**: An active microservice generates millions of spans per hour. Should every span become an Entity, or only sampled/ERROR spans?
2. **Agent-specific signals**: UModel targets "agent-era observability," but current OTel semantic conventions don't yet cover LLM/Agent spans (work is ongoing in the OTel LLM SIG). How should agent concepts (model name, token count, tool calls) map to UModel constructs?
3. **Metrics temporality**: OTel supports cumulative and delta temporality. UModel MetricSet currently has no temporality field — should one be added?
4. **Baggage propagation**: OTel Baggage carries cross-cutting context (`tenant_id`, `user_id`). These could map to entity properties, but the pattern needs discussion.
5. **Resource Schema URL**: OTel v1.26+ `schema_url` maps naturally to UModel's `schema.version` — should this correspondence be formalized?

---

## 5. Relationship to Other UModel Schemas

This document does not propose new schema kinds. It explores a convention for using existing schema kinds (EntitySet, TraceSet, MetricSet, LogSet, EventSet, EntitySetLink, DataLink) to model OTel-instrumented systems.

---

## 6. Acknowledgments

This discussion draft was prepared by CAICT as part of the "Semantic Conventions for Agentic Workloads" initiative.

An extended version of this document with worked examples and illustrative schema sketches is available for workshop discussions. Please contact the authors or comment on this PR if interested.

**Feedback welcome.** Please comment on this PR or join the upcoming workshop discussion.
