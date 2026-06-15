---
layout: home

title: UModel — semantic runtime for enterprise AI
titleTemplate: false

hero:
  name: UModel
  text: The vendor-neutral semantic runtime for enterprise AI
  tagline: One object-graph semantic layer for enterprise AI, data governance, and operational intelligence — that humans, systems, and AI agents query through a single local service.
  image:
    src: /openumodel-mark.svg
    alt: UModel
  actions:
    - theme: brand
      text: Get Started
      link: /en/getting-started/quickstart
    - theme: alt
      text: English Docs
      link: /en/
    - theme: alt
      text: 中文文档
      link: /zh/
    - theme: alt
      text: GitHub
      link: https://github.com/alibaba/UnifiedModel

features:
  - icon: 🤖
    title: Accelerate enterprise AI at scale
    details: Give agents a queryable object graph instead of raw telemetry — entities, relationships, topology, and the model itself through one SPL surface (.umodel / .entity / .topo).
  - icon: 🧭
    title: Reduce data governance cost
    details: One shared semantic language across metrics, logs, traces, events, and entities, so teams stop re-aligning fields and rebuilding context for every tool.
  - icon: 🔓
    title: Vendor neutrality — no lock-in
    details: Independent of any platform, data tool, observability stack, or AI vendor. Bring your own backends behind GraphStore providers.
  - icon: ⚙️
    title: An enterprise semantic OS
    details: A live, programmable semantic runtime that AI agents can query, reason over, and share as context for multi-agent collaboration.
---

## See it in action

Two worked AI-agent demos run end to end on the bundled data — no API key, no network:

- **Incident Investigation** — a payment-gateway SLO breach. The agent traverses a Business → Platform → Runtime object graph, pulls the right telemetry, and concludes the root cause (retry storm × promotion traffic = **8.75× overload**) while ruling out an innocent deployment. [Open the demo →](https://github.com/alibaba/UnifiedModel/tree/main/examples/incident-investigation)
- **Service Localization** — a degraded checkout API. The agent walks the Product → Service → Datastore → Infra critical path, fetching latency and saturation at each hop, and localizes the bottleneck to a **datastore connection pool at 98%** — infrastructure ruled out as healthy. [Open the demo →](https://github.com/alibaba/UnifiedModel/tree/main/examples/service-localization)

## Get started in one minute

Start the API and Web UI with a preloaded demo workspace — runs in memory, leaves nothing behind:

```bash
make quickstart        # API on :8080, Web UI on :5173, demo workspace preloaded
```

Query models, entities, and topology through one surface:

```bash
umctl query run demo ".umodel | limit 5"
umctl query run demo ".entity with(domain='platform', name='platform.service', query='degraded')"
```

Drive UModel from a skill-aware agent — in Claude Code, install both skills in one command:

```
/plugin marketplace add alibaba/UnifiedModel
/plugin install umodel@unifiedmodel
```

Then read on: **[English documentation](/en/)** · **[中文文档](/zh/)** · [Quick Start](/en/getting-started/quickstart) · [Concepts](/en/concepts/) · [Agent Integration](/en/guides/agent-integration).
