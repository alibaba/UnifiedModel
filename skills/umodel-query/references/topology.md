# `.topo` — relationships & topology

`graph-call` runs a graph method. The start node is `(:"<domain>@<entity_set>"
{__entity_id__:'<id>'})`; `getNeighborNodes(scope, hops, [start])` takes a scope (`'full'`),
a hop count (raise it for multi-hop), and the start node.

```bash
# all neighbors of a node
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"platform@platform.service\" {__entity_id__:'63718b78868895d2590551b27ec6f51c'})])" -o json

# filter rows by relation type with a `where` pipe — use `where`, NOT `with(...)`, which
# does NOT filter graph-call output:
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"platform@platform.service\" {__entity_id__:'63718b78868895d2590551b27ec6f51c'})]) | where __relation_type__ = 'calls'" -o json

# direct relations of a node; or full (read-only) Cypher
umctl query run demo ".topo | graph-call getDirectRelations([(:\"platform@platform.service\" {__entity_id__:'…'})])" -o json
umctl query run demo ".topo | graph-call cypher(\`MATCH (s)-[r]->(d) RETURN properties(s), type(r), properties(d) LIMIT 20\`)" -o json
```

## Returned format

Each row is an **edge**. The real columns are:

```jsonc
"header": ["src","relation","dest","__relation_type__","__src_entity_id__","__dest_entity_id__",
           "__src_domain__","__dest_domain__","__src_entity_type__","__dest_entity_type__", "display_name"]
```

Read it as `__src_entity_id__` →(`__relation_type__`)→ `__dest_entity_id__`, plus a
human `display_name`. `getNeighborNodes` returns edges in **both directions**, so use src/dest
to separate **upstream from downstream**:

- who *calls* / *depends on* this node → rows where `__dest_entity_id__` is your node;
- what it calls / depends on → rows where `__src_entity_id__` is your node.

Demo relation types: `calls`, `depends_on`, `affects`, `targets`, `impacts`, `runs_as`,
`owns`. Rows reference entities by **ID** — resolve display names with `.entity … with(ids=[…])`
(see [entity.md](entity.md)).

## Worked example

Find who *calls* payment-gateway (`63718b78…`) — filter to `calls`, then read direction:

```bash
umctl query run demo ".topo | graph-call getNeighborNodes('full', 1, [(:\"platform@platform.service\" {__entity_id__:'63718b78868895d2590551b27ec6f51c'})]) | where __relation_type__ = 'calls'" -o json
```

→ 3 `calls` edges (`__src_entity_id__` → `__dest_entity_id__`, `display_name`):

```
149632df… → 63718b78…   Checkout service calls payment gateway — critical payment path
f25ae292… → 63718b78…   Order service confirms payment via payment gateway
63718b78… → 394ba4c2…   Payment gateway validates auth token via auth service
```

The first two have payment-gateway as `dest` → **upstream callers** (checkout, order). The
third has it as `src` → auth is **downstream**. Resolve `149632df…` with `.entity … with(ids=['149632df…'])`
→ `checkout-service`.
