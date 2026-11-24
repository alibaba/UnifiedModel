# Loki Storage example

This pack connects `ops.node` to Loki logs and a LogQL error-rate metric. Import it into an existing workspace:

```bash
go run ./cmd/umctl --addr http://localhost:8080 umodel import demo examples/loki-storage
```

The pack contains model definitions only; it does not start Loki or ingest log lines. Point `ops.loki.spec.endpoint` at your Loki service. The expected stream labels are `host`, `env`, and `severity`.

Generate and execute a plan using the commands in the [English guide](../../docs/en/guides/loki-storage.md) or [中文指南](../../docs/zh/guides/loki-storage.md).
