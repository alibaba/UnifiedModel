# Entity-Discovered Skills And Skill Runner Design

## Status

Approved direction for implementation on 2026-08-12. The work is split into two sequential deliverables:

1. add `list_skills` to the local UnifiedModel Query Service;
2. add the loadable `umodel-skill-runner` agent skill on top of that public query capability.

## Goal

Let an agent start from a concrete UModel EntitySet, discover through `__list_method__()` that related Agent Skills exist, list those Skills through `list_skills()`, load one selected Skill in detail, and follow its instructions without adding a second model-read API or executing arbitrary Skill content inside the UnifiedModel server.

## Design Principles

- Capability discovery is entity-driven. The agent first asks the EntitySet what methods it supports.
- `list_skills` is visible only when the EntitySet has at least one visible Skill through a direct `runbook_link`.
- Query Service remains the only public read path. REST, CLI, MCP, and SDK callers use the existing `.entity_set | entity-call` surface.
- UnifiedModel returns Skill definitions; it does not interpret `SKILL.md`, bind provider credentials, or execute external Tool implementations.
- The agent runtime follows `SKILL.md`. Every Tool call remains subject to the caller's available tools, current authorization, confirmation rules, and sandbox policy.
- P0 follows the existing UModel Assistant `list_skills` identity and response contract so local and PaaS-backed callers use the same SPL.

## Public Query Contract

### Method discovery

```spl
.entity_set with(
  domain='platform',
  name='platform.service',
  ids=['service-id']
)
| entity-call __list_method__()
```

When at least one direct, visible `runbook_link` leads to a `runbook_set` containing a non-empty `skills` array, the response includes:

```json
{
  "name": "list_skills",
  "display_name": "List Skills",
  "description": "Get Skills from RunbookSets related to EntitySet",
  "params": [
    {
      "key": "skill_ids",
      "type": "array<varchar>",
      "required": false
    },
    {
      "key": "detail",
      "type": "boolean",
      "required": false,
      "default": false
    }
  ],
  "returns": [
    {"key": "skill_id", "type": "varchar"},
    {"key": "skill_name", "type": "varchar"},
    {"key": "display_name", "type": "varchar"},
    {"key": "description", "type": "varchar"},
    {"key": "license", "type": "varchar"},
    {"key": "compatibility", "type": "varchar"},
    {"key": "allowed_tools", "type": "varchar"},
    {"key": "skill_url", "type": "varchar"},
    {"key": "priority", "type": "integer"},
    {"key": "metadata", "type": "varchar"},
    {"key": "tags", "type": "varchar"},
    {"key": "files", "type": "varchar"},
    {"key": "skill_detail", "type": "varchar"}
  ]
}
```

If no related visible RunbookSet contains Skills, `__list_method__()` omits `list_skills`. Directly calling `list_skills()` remains valid and returns an empty table. This makes method discovery truthful without turning absence into an error.

### Listing and loading

List all visible Skills:

```spl
.entity_set with(domain='platform', name='platform.service')
| entity-call list_skills()
```

Load one Skill by exact identifier:

```spl
.entity_set with(domain='platform', name='platform.service')
| entity-call list_skills(
    ['platform@runbook_set@platform.service.ops@skills@incident-investigation'],
    true
  )
```

Parameters:

| Parameter | Type | Default | Meaning |
|---|---|---:|---|
| `skill_ids` | `array<varchar>` | `[]` | Optional exact Skill identifiers to retain. |
| `detail` | `boolean` | `false` | When true, append `skill_detail`, containing the complete Skill object as JSON. |

The canonical identifier remains:

```text
<runbook_domain>@runbook_set@<runbook_name>@skills@<skill_name>
```

The P0 response follows the current UModel Assistant contract. The default response includes `files`; `detail=true` additionally includes `skill_detail`. Changing the PaaS contract to defer `files` is a separate compatibility decision and is outside this change.

### Relation and entity filtering

Only direct links are considered:

```text
EntitySet --runbook_link--> RunbookSet --contains--> Skill
```

A link is eligible when:

- `kind` is `runbook_link`;
- `spec.src` exactly matches the queried EntitySet by `domain`, `kind=entity_set`, and `name`;
- `spec.dest` resolves to an existing `runbook_set`;
- `filter_by_entity` allows the supplied `entity_data`, using the same evaluator as DataLink and StorageLink filtering;
- the RunbookSet contains a non-empty `spec.skills` array.

When no `entity_data` is supplied, existing filter semantics apply: a non-empty `filter_by_entity` does not hide the link because there is no entity row against which to reject it.

Results preserve UModel snapshot order: RunbookLink order, then Skill array order. Duplicate Skill IDs are emitted once.

## Local Query Service Changes

The implementation extends the existing `.entity_set` method pipeline:

1. parser accepts `list_skills` and alias `list_skill`;
2. planner normalizes the alias, validates positional/named parameters, and publishes the canonical signature;
3. executor loads the Workspace UModel snapshot, resolves related RunbookSets, filters optional Skill IDs, and returns the Assistant-compatible raw-data envelope;
4. `__list_method__()` loads the same snapshot and conditionally adds `list_skills` using the same relation resolver;
5. examples, English/Chinese Query Service documentation, Agent integration documentation, and skills documentation use the same SPL and response contract.

No new REST endpoint, MCP tool, GraphStore contract, or public service interface is added.

## `umodel-skill-runner` Agent Skill

The new repository skill lives at:

```text
skills/umodel-skill-runner/SKILL.md
```

It builds on `umodel-query` and follows this protocol:

1. identify the EntitySet and optional entity IDs from the user's task;
2. call `__list_method__()`;
3. stop the dynamic-Skill path if `list_skills` is absent;
4. call `list_skills()` to obtain candidates;
5. choose an exact candidate from user selection or the returned name, description, tags, priority, and compatibility;
6. if multiple candidates remain materially plausible, ask the user instead of silently choosing;
7. call `list_skills([skill_id], true)` and require exactly one row;
8. load `SKILL.md` from inline `files`; do not automatically fetch `skill_url` in P0;
9. treat referenced files as progressive context and open only those required by the selected instructions;
10. follow the loaded instructions while enforcing current tool availability and the user's authorization boundary;
11. require separate confirmation for destructive, publishing, mutation, or high-risk actions;
12. report unavailable allowed tools explicitly instead of substituting a different operation.

The runner does not install the fetched Skill globally, persist it outside the current task, expose credentials, or bypass the agent runtime's normal Skill and tool safety rules.

## Error Semantics

| Condition | Result |
|---|---|
| Unknown EntitySet | existing Query Service not-found behavior |
| No related RunbookSet or no Skills | empty `list_skills` result; method omitted from discovery |
| Unknown `skill_id` filter | empty result |
| Malformed `skill_ids` or `detail` | `QUERY_PLAN_ERROR` from normal entity-call validation |
| Link destination missing | skip the broken link; model validation remains responsible for reporting invalid references |
| `filter_by_entity` parse failure | hide that link, matching current safe filtering behavior |
| Selected Skill has neither inline `SKILL.md` nor an approved supported source | runner stops and reports the unsupported Skill package |
| Required Tool unavailable | runner reports the missing Tool and does not invent a substitute |

## Verification

Service tests must prove:

- parser and planner accept positional and named `list_skills` arguments and normalize `list_skill`;
- `__list_method__()` includes `list_skills` only for an EntitySet with a visible related Skill;
- direct RunbookLink resolution returns expected Skill summaries;
- exact `skill_ids` filtering and `detail=true` work;
- duplicate links do not duplicate Skill rows;
- `filter_by_entity` hides or exposes Skills using supplied `entity_data`;
- missing RunbookSets and unknown Skill IDs return empty results;
- AgentGateway/MCP executes the same SPL through `query_spl_execute` without a new tool.

Skill verification must prove:

- frontmatter is valid and the marketplace bundles the new Skill;
- documentation lists all three repository Skills consistently;
- examples use only supported SPL;
- repository guard, focused service tests, skill/example validation, and documentation consistency checks pass.

## Non-Goals

- Implement `search_skills` in the local Query Service.
- Execute arbitrary scripts embedded in a RunbookSet inside the UnifiedModel server.
- Add CapabilityService, provider Binding, credentials, HIL, or business Tool implementations to this repository.
- Download arbitrary `skill_url` packages automatically.
- Add multi-hop Runbook discovery to `list_skills`.
- Change the RunbookSet or Skill schema.
- Change the existing PaaS `list_skills` response contract.
