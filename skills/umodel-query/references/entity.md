# `.entity` — read entities (objects)

Reads runtime objects (services, deployments, config changes, promotions, pods, …) from
EntityStore / GraphStore. A bare read returns **all fields** of the matching objects — no
pipe required:

```bash
umctl query run demo ".entity with(domain='platform', name='platform.service', query='degraded')" -o json
```

## Parameters — `with(...)`

- `domain=` + `name=` — **required**: the EntitySet (object type), e.g. domain `platform`,
  name `platform.service`. Discover the available ones with `.umodel with(kind='entity_set')`
  (see [model.md](model.md)).
- `query='…'` — full-text over all fields; add `mode='vector'` or `mode='hyper'` for
  semantic / hybrid search, `topk=N` to cap matches. Omit `query` to read every object.
- `ids=['<entity_id>', …]` — fetch specific objects by their `__entity_id__`.

Pipes are **optional** — add them only to shape the output: `| project a,b,c` (narrow
columns), `| where field='x'`, `| sort field`, `| limit N`.

## Returned format

Plain rows: column names in `data.header`, rows in `data.data` (a matrix) — zip them. Each row
is **system fields** (`__*__`) + the object's **own fields**. For `platform.service` the real
columns are:

```jsonc
"header": ["__domain__","__entity_type__","__entity_id__","__last_observed_time__","__deleted__",
           "display_name","status","owner","sla_tier","environment","id", "..."]
```

`__entity_id__` is the stable handle you reuse in `.topo` (see [topology.md](topology.md)) and
in `.entity_set … ids=[…]` (see [entity-set.md](entity-set.md)).

## Worked example

```bash
umctl query run demo ".entity with(domain='platform', name='platform.service', query='degraded') | project __entity_id__, display_name, status, owner, sla_tier" -o json
```

→ one row (zip with the projected header `__entity_id__, display_name, status, owner, sla_tier`):

```jsonc
"data": [["63718b78868895d2590551b27ec6f51c","payment-gateway","degraded","payments-backend","platinum"]]
```

So `payment-gateway` (`63718b78…`) is **degraded**, owned by `payments-backend`, a `platinum`
SLO service. Reuse that `__entity_id__` for topology and telemetry.
