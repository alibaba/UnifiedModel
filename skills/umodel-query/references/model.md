# `.umodel` — the model catalog

The model is the **map**: what object types, datasets, links, and runbooks exist. Read it
before assuming structure.

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
umctl query run demo ".umodel with(kind='runbook_set', name='platform.service.ops')" -o json
```

`with(kind=…)` filters by element kind; add `name=…` to fetch one element in full. Kinds:
`entity_set` (object types `.entity` reads), `metric_set` / `log_set` / `event_set` (datasets),
`entity_set_link` / `data_link` / `storage_link` (how objects, datasets, and storage connect —
incl. the `fields_mapping` that scopes telemetry), `runbook_set` (operational runbooks).

## Returned format

Rows describe model elements. A full read (`with(kind=…, name=…)`) returns the columns
`["kind","domain","name","version","spec"]`, where **`spec`** is the element's full definition
(JSON). With `| project domain, name` you get just those two columns.

## Worked example

List the object types in the workspace:

```bash
umctl query run demo ".umodel with(kind='entity_set') | project domain, name" -o json
```

→ (real)

```jsonc
"data": [["business","business.order_flow"],["business","business.promotion"],
         ["platform","platform.config_change"],["platform","platform.deployment"],
         ["platform","platform.incident"],["platform","platform.service"],
         ["platform","platform.team"],["runtime","runtime.cluster"],
         ["runtime","runtime.namespace"],["runtime","runtime.pod"],["runtime","runtime.workload"]]
```

Fetch one element in full to see its `spec` (here a runbook, whose `spec` holds
failure-pattern `knowledge` and `automations`):

```bash
umctl query run demo ".umodel with(kind='runbook_set', name='platform.service.ops')" -o json
# → ["runbook_set","platform","platform.service.ops","v1.0.0", { "knowledge":[…], "automations":[…] }]
```

The `domain` + `name` of each `entity_set` listed here are exactly what you pass to `.entity`
(read its runtime instances, see [entity.md](entity.md)) and `.entity_set` (call its methods).
`.umodel` defines the **types**; `.entity` returns their **instances**.

`.umodel with(kind='metric_set')` browses the dataset **catalog** — every dataset in the
workspace. To find which metric_set / log_set a **specific entity** exposes (the `domain`+`name`
`get_metrics` / `get_logs` actually take), don't scan the catalog — ask the entity:
`.entity_set … | entity-call list_data_set(['metric_set'])` (see [entity-set.md](entity-set.md)).

To go from a model element to what you can *do* with it (its methods, its datasets), use
`.entity_set | entity-call` — see [entity-set.md](entity-set.md).
