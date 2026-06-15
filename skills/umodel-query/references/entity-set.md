# `.entity_set | entity-call` — call methods on an EntitySet

`.entity_set with(domain=…, name=…, ids=[…])` selects an EntitySet (optionally bound to
specific entities by their `__entity_id__`); `| entity-call <method>(…)` runs one of its
methods.

## Discover methods first — `__list_method__()`

Don't guess signatures — list the methods and their exact params/returns:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call __list_method__()" -o json
```

The demo EntitySet exposes four:

- `__list_method__()` — this method list.
- `list_data_set(types?, detail?)` — datasets on the entity; `types` e.g.
  `['metric_set','log_set']`, `detail=true` adds `fields_mapping`, `fields`, `storage_info`.
- `get_metrics(domain, name, metric?, query?, query_type?, step?)` — a metric query **plan**.
- `get_logs(domain, name, query?)` — a log query **plan**.

For `get_metrics` / `get_logs` (fetch the plan, then run it), see [metrics-logs.md](metrics-logs.md).

## Returned format — entity-call result shape

Every entity-call returns **one wrapped row** whose outer columns are
`["responseType","query","header","data"]` (so `data.data[0]` is `[responseType, query,
innerHeader, innerData]`):

- **Table methods** (`__list_method__`, `list_data_set`) → `responseType = 2`: the real table
  is the inner header at `data.data[0][2]` and inner rows at `data.data[0][3]` (each
  `{"values":[…]}`).
- **Plan methods** (`get_metrics`, `get_logs`) → `responseType = 1`: the **plan is the JSON
  string at `data.data[0][1]`** (the `query` column); inner header/data are empty.

## Worked example — `__list_method__()`

The call above returns `responseType = 2`; the inner header (`data.data[0][2]`) is
`["name","display_name","description","params","returns"]`, and each inner row
(`data.data[0][3]`) is a method, e.g. (trimmed):

```jsonc
{"values": ["get_metrics", "Get Metrics", "Get metric query plan from a MetricSet",
  "[{\"key\":\"domain\",\"type\":\"varchar\",\"required\":true},{\"key\":\"metric\",\"type\":\"varchar\",\"required\":false},{\"key\":\"step\",\"type\":\"varchar\",\"required\":false}]",
  "[{\"key\":\"query\",\"type\":\"varchar\",\"display_name\":\"Metric query plan\"}]"]}
```

So `params` and `returns` are **JSON strings** — parse them to get each method's signature
(`{key, type, required, default}`) before you call it.
