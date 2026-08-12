# Entity-Discovered Skills And Skill Runner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add entity-discovered `list_skills` support to the local Query Service, then add a repository agent Skill that loads and follows a selected RunbookSet Skill.

**Architecture:** Extend the existing `.entity_set | entity-call` parser/planner/executor path and reuse UModel snapshots, RunbookLink references, and `filter_by_entity`. Keep dynamic Skill interpretation in the agent Skill; the server only returns definitions through Query Service.

**Tech Stack:** Go 1.22+, existing Query Service/GraphStore contracts, Markdown Agent Skills, JSON/YAML model fixtures.

## Global Constraints

- Query Service remains the only public read path; add no REST endpoint or MCP tool.
- `list_skills` uses direct `runbook_link` relations only.
- `list_skill` normalizes to canonical `list_skills`.
- Skill IDs use `<runbook_domain>@runbook_set@<runbook_name>@skills@<skill_name>`.
- `filter_by_entity` uses the existing safe evaluator.
- The server never interprets `SKILL.md` or executes embedded scripts.
- `umodel-skill-runner` does not automatically fetch `skill_url` in P0.

---

### Task 1: Query Contract And Planner

**Files:**
- Modify: `internal/query/planner.go`
- Modify: `internal/query/parser.go`
- Test: `internal/query/list_skills_test.go`

**Interfaces:**
- Consumes: existing `model.EntityCallPlan` and `normalizeEntityCall`.
- Produces: canonical method `list_skills` with parameters `skill_ids []string` and `detail bool`.

- [x] **Step 1: Write failing parser/planner tests**

Add table tests that call:

```go
plan, err := Planner{}.Plan(model.QueryRequest{
    Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills(['platform@runbook_set@platform.service.ops@skills@incident-investigation'], true)",
}, graphstore.NewMemoryStore().Capabilities())
```

Assert the canonical method, positional parameters, named parameters, defaults, alias normalization, and rejection of the wrong parameter types.

- [x] **Step 2: Run the focused test and confirm RED**

Run: `go test ./internal/query -run 'TestListSkillsPlanner' -count=1`

Expected: FAIL with `unsupported entity-call method`.

- [x] **Step 3: Add the minimal method specification**

Add this planner case:

```go
case "list_skill", "list_skills":
    return entityCallMethodSpec{
        Name: "list_skills",
        Params: []model.EntityCallParam{
            {Key: "skill_ids", Type: "array<varchar>", DisplayName: "Skill IDs to filter"},
            {Key: "detail", Type: "boolean", DisplayName: "Detail Info", Default: false},
        },
    }, true
```

Update the parser's EntitySet guidance to name `list_skills`.

- [x] **Step 4: Run the focused tests and confirm GREEN**

Run: `go test ./internal/query -run 'TestListSkillsPlanner' -count=1`

Expected: PASS.

### Task 2: RunbookLink Resolution And `list_skills` Execution

**Files:**
- Modify: `internal/query/executor.go`
- Test: `internal/query/list_skills_test.go`

**Interfaces:**
- Consumes: `refFromSpec`, `findUModelElement`, `filterByEntityAllows`, `filterByEntityExpression`, `entitySetAssistantRawResponse`.
- Produces: `relatedSkillsForEntitySet(...)`, `executeEntitySetListSkills(...)`, and Assistant-compatible Skill rows.

- [x] **Step 1: Write failing execution tests**

Build a real memory GraphStore snapshot with an EntitySet, RunbookLink, RunbookSet, and two literal Skills. Assert:

```go
result, err := svc.Execute(ctx, "demo", model.QueryRequest{
    Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills()",
})
```

returns exactly the documented header and literal `skill_id` values. Add separate tests for `skill_ids`, `detail=true`, duplicate links, missing destination, and matching/non-matching `entity_data`.

- [x] **Step 2: Run execution tests and confirm RED**

Run: `go test ./internal/query -run 'TestListSkillsExecute' -count=1`

Expected: FAIL with unsupported execution or empty results.

- [x] **Step 3: Implement minimal direct-link resolution and row projection**

Add a `list_skills` executor branch. Resolve direct RunbookLinks, deduplicate by canonical Skill ID, preserve snapshot/array order, marshal `metadata`, `tags`, `files`, and optional `skill_detail`, and return an empty table for no matches.

- [x] **Step 4: Run execution tests and confirm GREEN**

Run: `go test ./internal/query -run 'TestListSkillsExecute' -count=1`

Expected: PASS.

### Task 3: Dynamic `__list_method__` Discovery

**Files:**
- Modify: `internal/query/executor.go`
- Modify: `internal/query/signature_alignment_test.go`
- Test: `internal/query/list_skills_test.go`

**Interfaces:**
- Consumes: `relatedSkillsForEntitySet(...)` from Task 2.
- Produces: `__list_method__()` includes `list_skills` only when at least one visible related Skill exists.

- [x] **Step 1: Write failing method-discovery tests**

Assert a linked EntitySet includes the literal `list_skills` signature, an unrelated EntitySet omits it, and `filter_by_entity` controls visibility when `entity_data` is supplied.

- [x] **Step 2: Run discovery tests and confirm RED**

Run: `go test ./internal/query -run 'TestListSkillsDiscovery' -count=1`

Expected: FAIL because method rows are currently static.

- [x] **Step 3: Make list-method execution snapshot-aware**

Replace the static executor return with an executor method that loads the UModel snapshot, checks `relatedSkillsForEntitySet`, and conditionally appends `methodInfoListSkills()`.

- [x] **Step 4: Run all Query tests and confirm GREEN**

Run: `go test ./internal/query -count=1`

Expected: PASS.

### Task 4: Public Documentation And Examples

**Files:**
- Modify: `docs/en/guides/query-service.md`
- Modify: `docs/zh/guides/query-service.md`
- Modify: `docs/en/guides/agent-integration.md`
- Modify: `docs/zh/guides/agent-integration.md`
- Modify: `skills/umodel-query/SKILL.md`
- Modify: `skills/umodel-query/references/entity-set.md`

**Interfaces:**
- Consumes: supported SPL from Tasks 1-3.
- Produces: aligned English/Chinese user guidance and agent instructions.

- [x] **Step 1: Add exact examples and boundary text**

Document `__list_method__()` discovery, `list_skills()`, exact-ID detail loading, direct RunbookLink scope, and the server/runtime execution boundary.

- [x] **Step 2: Verify documentation formatting**

Run: `git diff --check`

Expected: no output.

### Task 5: `umodel-skill-runner`

**Files:**
- Create: `skills/umodel-skill-runner/SKILL.md`
- Modify: `skills/README.md`
- Modify: `skills/README.zh-CN.md`
- Modify: `skills/QUICKSTART.md`
- Modify: `skills/QUICKSTART.zh-CN.md`
- Modify: `.claude-plugin/marketplace.json`
- Modify: `README.md`
- Modify: `README_CN.md`
- Modify: `README.zh-CN.md`

**Interfaces:**
- Consumes: `query_spl_execute` or `umctl query run` with `__list_method__` and `list_skills` SPL.
- Produces: loadable Skill `umodel-skill-runner` that self-gates on discovered capability and follows one exact inline `SKILL.md`.

- [x] **Step 1: Use superpowers:writing-skills to create pressure scenarios**

Cover: no `list_skills`, one candidate, ambiguous candidates, missing inline `SKILL.md`, unavailable allowed Tool, and a Skill requesting an unauthorized mutation.

- [x] **Step 2: Write the minimal `SKILL.md`**

The instructions must discover methods first, list summaries, resolve exactly one ID, reload with `detail=true`, reject automatic `skill_url` fetching, honor `allowed_tools`, and preserve user authorization/HIL boundaries.

- [x] **Step 3: Update catalogs and plugin manifest**

Add `./skills/umodel-skill-runner`, describe three bundled skills, and keep Chinese README mirrors aligned.

- [x] **Step 4: Verify skill packaging and documentation consistency**

Run:

```bash
python3 -m json.tool .claude-plugin/marketplace.json >/dev/null
cmp -s README_CN.md README.zh-CN.md
git diff --check
```

Expected: all commands exit 0.

### Task 6: Integrated Verification

**Files:**
- Test: existing repository checks only

**Interfaces:**
- Consumes: all earlier tasks.
- Produces: verified branch ready for user review.

- [x] **Step 1: Run focused and architecture checks**

Run:

```bash
go test ./internal/query ./internal/agentgateway ./cmd/umodel-mcp -count=1
make guard
make example-validate
```

Expected: all pass.

- [x] **Step 2: Run the full Go suite**

Run: `go test ./...`

Expected: all packages pass.

- [x] **Step 3: Inspect final scope**

Run:

```bash
git status --short
git diff --check
git diff --stat origin/main...HEAD
```

Expected: only list-skills, Skill, paired docs, design, and plan files are changed.
