# Extensible storage: a PromQL-compatible backend in YAML

This example adds [VictoriaMetrics](https://victoriametrics.com/) as a metrics
backend without writing any UModel code.

VictoriaMetrics speaks the Prometheus query API, so it shares Prometheus' query
model — the `label-timeseries` family. UModel routes a storage to a query-plan
renderer by **family**, not by a hardcoded storage kind. A storage selects its
family with `spec.family`:

```yaml
kind: victoriametrics
spec:
  family: label-timeseries
  endpoint: "http://localhost:8428"
```

With that declaration, `get_metrics` on an entity backed by this storage produces
the same `prometheus_promql` query plan it would for a native Prometheus storage:
the existing `label-timeseries` renderer handles it. There is no VictoriaMetrics
renderer and no new Go code.

Adding the `victoriametrics` kind was schema-only:

- [`schemas/core/storage/victoriametrics.schema.yaml`](../../schemas/core/storage/victoriametrics.schema.yaml) defines the kind.
- The Go, Python, and Java SDK types are generated from that schema by `make expand`.
- [`victoriametrics.storage.yaml`](victoriametrics.storage.yaml) is the storage definition this example validates.

A backend that uses a different query model (for example SQL) needs one renderer
for that family; every backend in the family after the first is configuration
only.
