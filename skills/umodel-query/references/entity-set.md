# `.entity_set | entity-call` — call methods on an EntitySet

`.entity_set with(domain=…, name=…, ids=[…])` selects an EntitySet (optionally bound to
specific entities by their `__entity_id__`); `| entity-call <method>(…)` runs one of its
methods.

## Discover methods first — `__list_method__()`

Don't guess signatures — list the methods and their exact params/returns:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call __list_method__()" -o json
```

Every EntitySet exposes the core query methods below. When a direct, visible
`runbook_link` reaches a RunbookSet containing Skills or Knowledge, discovery also
includes the corresponding `list_skills` or `list_knowledge` method:

- `__list_method__()` — this method list.
- `list_data_set(types?, detail?)` — datasets on the entity; `types` e.g.
  `['metric_set','log_set']`, `detail=true` adds `fields_mapping`, `fields`, `storage_info`.
- `get_metrics(domain, name, metric?, query?, query_type?, step?)` — a metric query **plan**.
- `get_logs(domain, name, query?)` — a log query **plan**.
- `list_skills(skill_ids?, detail?)` — Skills attached through RunbookLink; present only
  when the EntitySet has at least one visible related Skill.
- `list_knowledge(knowledge_ids?, detail?)` — Knowledge attached through RunbookLink;
  present only when the EntitySet has at least one visible related Knowledge item.

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

## List and load related Skills — `list_skills`

Call this only after `__list_method__()` advertises it:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call list_skills()" -o json
```

Each row includes a canonical `skill_id`:

```text
<runbook_domain>@runbook_set@<runbook_name>@skills@<skill_name>
```

After selecting one exact candidate, reload it with detail:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call list_skills(['platform@runbook_set@platform.service.ops@skills@incident-investigation'], true)" -o json
```

`detail=true` adds `skill_detail`; `files` contains inline `SKILL.md` and supporting
files when the model embeds them. Query Service only returns these definitions. It does
not interpret the instructions, fetch `skill_url`, or execute `allowed_tools`.

## List and load related Knowledge — `list_knowledge`

Call this only after `__list_method__()` advertises it:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call list_knowledge()" -o json
```

Each row includes a canonical `knowledge_id`:

```text
<runbook_domain>@runbook_set@<runbook_name>@knowledge@<knowledge_name>
```

Load exact items with detail before evaluating their policy or content:

```bash
umctl query run demo ".entity_set with(domain='platform', name='platform.service', ids=['63718b78868895d2590551b27ec6f51c']) | entity-call list_knowledge(['platform@runbook_set@platform.service.ops@knowledge@retry_storm_pattern'], true)" -o json
```

`detail=true` adds `apply_policy`, inline `content`, `content_url`, complete
Knowledge/RunbookSet definitions, and every matching visible RunbookLink. Query
Service only returns these definitions. A caller decides which inline Knowledge to
use and must not treat `content_url` or `runbook_set_detail` as executable authority.
