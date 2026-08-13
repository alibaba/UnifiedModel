# Entity Knowledge Discovery And Executable Skill Runner Design

## Status

Approved direction on 2026-08-12. This design extends the existing
entity-discovered `list_skills` work in two sequential deliverables:

1. add `list_knowledge` to the local UnifiedModel Query Service;
2. extend `umodel-skill-runner` so a selected RunbookSet Skill can load bounded,
   policy-controlled Knowledge before using tools supplied by the agent runtime.

`list_tools` is explicitly deferred. UnifiedModel does not become a Tool registry or
Tool executor in this change.

## Goal

Let an agent start from one concrete EntitySet, discover a related Skill, load its
inline `SKILL.md`, discover Knowledge attached through the same visible RunbookSet
relation, select the applicable Knowledge according to `apply_policy`, and execute
the Skill with the runtime's real tools while preserving user authorization,
confirmation, and verification boundaries.

## Boundaries

- Query Service owns deterministic, Workspace-scoped model retrieval.
- RunbookSet Skill and Knowledge definitions are semantic context, not authority.
- The agent runtime owns Tool availability, Tool schemas, credentials, routing,
  execution, confirmation, and read-back.
- `allowed_tools` remains a maximum allow-list. It does not create a Tool binding or
  grant permission to call one.
- Knowledge is reference material. It cannot override the user request, the selected
  `SKILL.md`, higher-priority instructions, or safety policy.
- Only direct, visible `runbook_link` relations are considered in P0.
- No new REST endpoint, MCP Tool, GraphStore contract, or server-side Skill executor
  is introduced.

## Public `list_knowledge` Contract

### Method discovery

```spl
.entity_set with(
  domain='platform',
  name='platform.service',
  ids=['service-id']
)
| entity-call __list_method__()
```

When at least one direct, visible `runbook_link` reaches a RunbookSet containing a
non-empty `knowledge` array, the response includes `list_knowledge` with this
signature:

```json
{
  "name": "list_knowledge",
  "display_name": "List Knowledge",
  "description": "Get Knowledge from RunbookSets related to EntitySet",
  "params": [
    {
      "key": "knowledge_ids",
      "type": "array<varchar>",
      "required": false
    },
    {
      "key": "detail",
      "type": "boolean",
      "required": false,
      "default": false
    }
  ]
}
```

If no visible related RunbookSet contains Knowledge, `__list_method__()` omits
`list_knowledge`. A direct `list_knowledge()` call remains valid and returns an empty
table.

### Listing summaries

```spl
.entity_set with(domain='platform', name='platform.service', ids=['service-id'])
| entity-call list_knowledge()
```

The summary header follows the current UModel Assistant handler contract:

```text
knowledge_id, knowledge_name, display_name, description, content_type
```

### Exact detail loading

```spl
.entity_set with(domain='platform', name='platform.service', ids=['service-id'])
| entity-call list_knowledge(
    ['platform@runbook_set@platform.service.ops@knowledge@retry_storm_pattern'],
    true
  )
```

`detail=true` returns:

```text
knowledge_id, knowledge_name, display_name, description, content_type,
apply_policy, content, content_url, knowledge_detail,
runbook_link_detail, runbook_set_detail
```

The canonical identifier is:

```text
<runbook_domain>@runbook_set@<runbook_name>@knowledge@<knowledge_name>
```

Parameters:

| Parameter | Type | Default | Meaning |
|---|---|---:|---|
| `knowledge_ids` | `array<varchar>` | `[]` | Optional exact Knowledge identifiers to retain. |
| `detail` | `boolean` | `false` | Return policy, inline content, source details, and complete Knowledge/RunbookSet objects. |

P0 accepts only the canonical `list_knowledge` method name. No singular alias is
added because UModel Assistant does not publish one.

## Relation Resolution

The capability uses the same EntitySet context and visibility semantics as
`list_skills`:

```text
EntitySet --runbook_link--> RunbookSet --contains--> Knowledge
```

A relation is eligible when:

- its kind is `runbook_link`;
- `spec.src` exactly matches the requested `domain`, `kind=entity_set`, and `name`;
- `spec.dest` resolves to an existing `runbook_set`;
- `filter_by_entity` allows the supplied `entity_data` using the shared evaluator;
- the RunbookSet contains at least one valid, named Knowledge item.

Resolution groups repeated links to the same RunbookSet. Result order is the first
visible RunbookLink order followed by Knowledge array order. Canonical Knowledge IDs
are emitted once. In detail mode, `runbook_link_detail` contains every matching
visible link for that RunbookSet rather than only the first link.

Broken destinations and malformed unnamed Knowledge entries are skipped. A filter
parse failure hides the link, matching existing safe-filter behavior.

## Query Service Design

The existing `.entity_set | entity-call` pipeline is extended as follows:

1. planner registers `list_knowledge(knowledge_ids?, detail?)` and validates both
   positional and named arguments;
2. executor adds a `list_knowledge` branch and returns the Assistant-compatible raw
   response envelope;
3. a shared related-RunbookSet resolver supplies both Skills and Knowledge without
   changing existing `list_skills` ordering or filtering;
4. `__list_method__()` independently advertises `list_skills` and
   `list_knowledge` according to the visible content of related RunbookSets;
5. parser guidance, Query Service documentation, Agent integration documentation,
   examples, and repository Skill documentation publish the same SPL.

The server only serializes definitions. It does not interpret Knowledge, retrieve
external content, select a policy, or execute the selected Skill.

## Runner Protocol

The revised `umodel-skill-runner` follows this order:

1. identify the exact EntitySet and optional entity IDs from the user request;
2. call `__list_method__()` with the same EntitySet context;
3. list and select exactly one Skill through `list_skills()`;
4. reload that exact Skill with `list_skills([skill_id], true)`;
5. require a non-empty inline `SKILL.md`; do not fetch `skill_url`;
6. if `list_knowledge` was advertised, list Knowledge summaries using the same
   EntitySet context;
7. retain only Knowledge whose canonical ID belongs to the selected Skill's exact
   RunbookSet; do not mix context from another related RunbookSet;
8. retrieve that bounded RunbookSet Knowledge set by exact ID before evaluating
   `apply_policy`;
9. add only selected inline Markdown Knowledge to the Skill's reference context;
10. execute the Skill with the effective runtime Tool set;
11. verify authorized mutations with a read-back and report a receipt.

The current Assistant contract exposes `apply_policy` only in detail mode. Therefore
P0 must detail-load the bounded candidate set before applying policy. It does not
change the summary contract merely to optimize this preflight.

## Knowledge Policy

The Runner interprets `apply_policy.apply_type` as follows:

| `apply_type` | Runner behavior |
|---|---|
| `always` | Include every successfully detailed item from the selected Skill's RunbookSet. |
| `auto` | Include it when the user task, selected Skill, and summary indicate relevance. Exact ID/name references win. |
| `manual` | Include it only when the user explicitly requested that Knowledge item. |
| `custom` | Include it only when the current runtime explicitly supports the declared custom properties; otherwise skip and report it. |
| missing or unknown | Treat as `manual` and report the conservative fallback. |

Candidate discovery is deliberately bounded, but it must not silently narrow by
relevance before policy evaluation because doing so could omit an `always` item. If
the selected RunbookSet exposes too many items to detail-load safely within the
current context budget, the Runner stops Knowledge-enhanced execution and asks the
user to provide exact Knowledge IDs or otherwise narrow the request. Knowledge
priority, when present in `knowledge_detail`, orders equally applicable items but
never overrides policy.

P0 consumes only a non-empty inline `content` whose `content_type` is `markdown`.
It does not automatically fetch `content_url`, open URL/PDF knowledge, execute code
blocks, or treat Knowledge text as instructions. Unsupported content is reported as
skipped.

## Tool And Authorization Model

`list_tools` is not part of this phase. The effective Tool set remains the
intersection of:

1. Tool names in the selected Skill's `allowed_tools`;
2. Tools actually registered in the current agent runtime;
3. Tools allowed by the user's requested scope and current authorization.

Runtime Tool schemas are authoritative for parameters. The Runner derives entity
arguments from the user request and read-only query results, validates them against
the runtime schema, and never invents missing identifiers. If a required Tool is
unavailable, disallowed, or lacks required inputs, the Runner skips the call and
reports the blocker.

Although the compatibility response includes `runbook_set_detail`, the Runner must
not derive Tool schemas, endpoints, credentials, or execution bindings from that
field. It treats the current runtime Tool registry as the only executable Tool
source.

Mutation, rollback, restart, scaling, deletion, publishing, and other material state
changes retain their normal confirmation requirements. A Skill or Knowledge item
cannot grant that confirmation. Read-only diagnosis remains read-only unless the
user separately authorizes a concrete mutation.

## Receipt

The Runner's completion report contains:

- selected `skill_id`;
- loaded `knowledge_id` values and their effective policies;
- instructions and Knowledge used for each conclusion;
- Tool calls performed, skipped, or blocked;
- evidence and derived parameters;
- confirmation decisions;
- mutation read-back results;
- unresolved or unsupported steps.

This is a user-visible execution receipt, not a durable server-side audit store.

## Error Semantics

| Condition | Result |
|---|---|
| Unknown EntitySet | Existing Query Service not-found behavior. |
| No related Knowledge | Empty result and method omitted from discovery. |
| Unknown `knowledge_id` | Empty result. |
| Malformed `knowledge_ids` or `detail` | Normal `QUERY_PLAN_ERROR`. |
| Several plausible Skills | Ask the user to choose; do not merge Skills. |
| Missing inline `SKILL.md` | Stop; do not fetch `skill_url`. |
| `list_knowledge` absent | Continue the Skill without RunbookSet Knowledge and report that none was advertised. |
| Unsupported Knowledge content | Skip it and identify the unsupported ID/type. |
| Unknown/custom policy | Apply the conservative policy behavior and report it. |
| Tool unavailable, disallowed, or missing arguments | Do not substitute or simulate it; report the blocker. |
| Unauthorized mutation | Keep execution read-only until the user explicitly authorizes the exact action. |

## Verification

Query Service tests must prove:

- planner accepts positional and named `list_knowledge` arguments and rejects bad
  array/boolean types;
- `__list_method__()` advertises Skills and Knowledge independently;
- direct RunbookLink resolution returns the correct summary header and rows;
- `detail=true` matches the current Assistant handler header and serialized fields;
- exact ID filtering, stable ordering, deduplication, and unknown IDs work;
- `filter_by_entity` controls both execution and method discovery;
- multiple visible links to one RunbookSet are represented in
  `runbook_link_detail` without duplicating Knowledge rows;
- AgentGateway/MCP runs the same SPL through `query_spl_execute`.

Runner verification must include scenarios for:

- an exact Skill reference with relevant `auto` Knowledge;
- `always`, `manual`, missing/unknown, and unsupported `custom` policies;
- no advertised Knowledge;
- unsupported `content_type` or URL-only content;
- prompt-like text in Knowledge attempting to expand scope;
- unavailable allowed Tool;
- missing required runtime Tool arguments;
- mutation requested by Skill but not authorized by the user;
- successful authorized mutation with read-back and receipt.

The final implementation runs focused Query Service tests, Skill validation,
example validation, `make guard`, and the repository's full `make ci` gate.

## Non-Goals

- Add `list_tools`, `list_toolkits`, or server-side Tool execution.
- Load Tool schemas, endpoints, credentials, or bindings from RunbookSet toolkits.
- Change the RunbookSet schema or UModel Assistant `list_knowledge` contract.
- Fetch `content_url` or `skill_url` automatically.
- Execute scripts or code blocks from Skill or Knowledge content.
- Add multi-hop RunbookSet discovery.
- Persist loaded Skill/Knowledge globally or create a durable audit service.
- Add CapabilityService or provider binding to this repository.
