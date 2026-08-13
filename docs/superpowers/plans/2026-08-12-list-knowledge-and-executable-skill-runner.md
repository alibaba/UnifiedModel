# Entity Knowledge Discovery And Executable Skill Runner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Assistant-compatible EntitySet `list_knowledge` discovery and retrieval, then teach `umodel-skill-runner` to load policy-controlled Knowledge from the selected Skill's RunbookSet before executing runtime tools.

**Architecture:** Extend the existing `.entity_set | entity-call` planner and executor with `list_knowledge`, backed by a shared direct-RunbookSet resolver used by Skills and Knowledge. Keep UnifiedModel read-only: the repository Skill selects inline Markdown Knowledge, while the caller's runtime remains authoritative for Tool schemas, authorization, execution, and verification.

**Tech Stack:** Go 1.22+, UnifiedModel Query Service, in-memory GraphStore tests, MCP stdio E2E tests, Markdown Agent Skills.

## Global Constraints

- Do not add `list_tools`, `list_toolkits`, Tool bindings, or server-side Tool execution.
- Match the current UModel Assistant `list_knowledge(knowledge_ids?, detail?)` handler contract exactly.
- Accept only canonical `list_knowledge`; do not add a singular alias.
- Resolve only direct, visible `runbook_link` relations and apply existing `filter_by_entity` semantics.
- Do not fetch `content_url` or `skill_url`; P0 Knowledge consumption is non-empty inline Markdown only.
- Knowledge is reference context and cannot override the user, `SKILL.md`, authorization, or safety policy.
- Runtime Tool schemas and Tool availability remain authoritative.
- Follow red-green-refactor for every production-code change and pressure-test the Skill before editing it.

---

### Task 1: Planner Contract For `list_knowledge`

**Files:**
- Create: `internal/query/list_knowledge_test.go`
- Modify: `internal/query/planner.go`
- Modify: `internal/query/parser.go`

**Interfaces:**
- Consumes: existing `entityCallMethodSpec`, positional/named argument normalization, and Assistant parameter types.
- Produces: canonical `EntityCall.Name == "list_knowledge"` with `knowledge_ids []string` and `detail bool` parameters.

- [ ] **Step 1: Write failing planner tests**

Add tests that independently catch a missing method registration and missing type validation:

```go
func TestListKnowledgePlannerNormalizesArguments(t *testing.T) {
	const knowledgeID = "platform@runbook_set@platform.service.ops@knowledge@retry_storm_pattern"
	plan, err := (Planner{}).Plan(model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge(['" + knowledgeID + "'], detail=true)",
	}, model.GraphStoreCapabilities{MaxDepth: 1, MaxLimit: 1000})
	if err != nil {
		t.Fatalf("plan list_knowledge: %v", err)
	}
	if plan.EntityCall == nil || plan.EntityCall.Name != "list_knowledge" {
		t.Fatalf("unexpected entity call: %+v", plan.EntityCall)
	}
	if !reflect.DeepEqual(plan.EntityCall.Parameters["knowledge_ids"], []string{knowledgeID}) {
		t.Fatalf("knowledge_ids = %#v", plan.EntityCall.Parameters["knowledge_ids"])
	}
	if plan.EntityCall.Parameters["detail"] != true {
		t.Fatalf("detail = %#v", plan.EntityCall.Parameters["detail"])
	}
}

func TestListKnowledgePlannerRejectsWrongArgumentTypes(t *testing.T) {
	queries := []string{
		".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge('not-an-array')",
		".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge([], 'not-a-bool')",
	}
	for _, query := range queries {
		_, err := (Planner{}).Plan(model.QueryRequest{Query: query}, model.GraphStoreCapabilities{MaxDepth: 1, MaxLimit: 1000})
		if !apperrors.IsCode(err, apperrors.CodeQueryPlanError) {
			t.Fatalf("query %q: expected QUERY_PLAN_ERROR, got %v", query, err)
		}
	}
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./internal/query -run 'TestListKnowledgePlanner' -count=1
```

Expected: FAIL because `list_knowledge` is not a registered EntitySet method.

- [ ] **Step 3: Register the canonical planner method**

Add this case to `entityCallMethodSpecFor`:

```go
case "list_knowledge":
	return entityCallMethodSpec{
		Name: "list_knowledge",
		Params: []model.EntityCallParam{
			{Key: "knowledge_ids", Type: "array<varchar>", DisplayName: "Knowledge IDs to filter", Default: []string(nil)},
			{Key: "detail", Type: "boolean", DisplayName: "Detail Info", Default: false},
		},
	}, true
```

Add `list_knowledge(...)` to the `.entity_set` parser guidance string.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run:

```bash
go test ./internal/query -run 'TestListKnowledgePlanner' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the planner contract**

```bash
git add internal/query/list_knowledge_test.go internal/query/planner.go internal/query/parser.go
git commit -m "feat(query): plan list_knowledge calls"
```

### Task 2: Related RunbookSet Resolution And Knowledge Rows

**Files:**
- Modify: `internal/query/list_knowledge_test.go`
- Modify: `internal/query/executor.go`
- Modify: `internal/query/list_skills_test.go`

**Interfaces:**
- Consumes: `relatedRunbookSetsForEntitySet(elements, domain, name, entityData)` and the existing Assistant raw-data envelope.
- Produces: `listKnowledgeHeader(detail bool)`, `relatedKnowledgeForEntitySet`, and `executeEntitySetListKnowledge` with stable, deduplicated rows.

- [ ] **Step 1: Add failing summary and detail execution tests**

Create a fixture with two Knowledge items in `platform.service.ops`:

```go
"knowledge": []any{
	map[string]any{
		"name": "retry_storm_pattern",
		"display_name": map[string]any{"en_us": "Retry Storm Pattern", "zh_cn": "重试风暴模式"},
		"description": map[string]any{"en_us": "Retry amplification guidance", "zh_cn": "重试放大指南"},
		"apply_policy": map[string]any{"apply_type": "auto"},
		"content_type": "markdown",
		"content": "# Retry Storm Pattern\nUse evidence before remediation.",
		"priority": 1,
		"tags": map[string]any{"category": "failure_pattern"},
	},
	map[string]any{
		"name": "deployment_triage_guide",
		"display_name": map[string]any{"en_us": "Deployment Triage"},
		"description": map[string]any{"en_us": "Rule out harmless deployments"},
		"apply_policy": map[string]any{"apply_type": "manual"},
		"content_type": "url",
		"content_url": "https://example.invalid/deployment-triage",
	},
}
```

Assert the default header is exactly:

```go
[]string{"knowledge_id", "knowledge_name", "display_name", "description", "content_type"}
```

Assert exact-ID `detail=true` returns exactly one row and this header:

```go
[]string{
	"knowledge_id", "knowledge_name", "display_name", "description", "content_type",
	"apply_policy", "content", "content_url", "knowledge_detail",
	"runbook_link_detail", "runbook_set_detail",
}
```

Decode and assert `apply_policy.apply_type == "auto"`, the inline content, the complete Knowledge object, one RunbookLink, and the complete RunbookSet.

- [ ] **Step 2: Add failing deduplication and filtering tests**

Add a duplicate visible RunbookLink and one broken destination. Assert:

- Knowledge rows remain two and preserve array order;
- detail `runbook_link_detail` contains both visible matching links;
- an unknown exact Knowledge ID returns zero rows;
- `environment = 'prod'` exposes rows only for matching `entity_data`.

- [ ] **Step 3: Run execution tests and verify RED**

Run:

```bash
go test ./internal/query -run 'TestListKnowledgeExecute' -count=1
```

Expected: FAIL because the executor does not handle `list_knowledge`.

- [ ] **Step 4: Implement the shared RunbookSet resolver**

Add:

```go
type relatedRunbookSet struct {
	RunbookSet model.UModelElement
	Links       []model.UModelElement
}

func relatedRunbookSetsForEntitySet(elements []model.UModelElement, entityDomain, entityName string, entityData *model.EntityData) []relatedRunbookSet
```

The function iterates snapshot elements in order, validates `runbook_link` source,
applies `filter_by_entity`, resolves a `runbook_set` destination, groups by canonical
RunbookSet ID, and appends every visible link to that group's `Links` slice. Refactor
`relatedSkillsForEntitySet` to iterate this resolver without changing existing Skill
IDs, ordering, or deduplication.

- [ ] **Step 5: Implement Knowledge serialization and execution**

Add:

```go
type relatedKnowledge struct {
	ID         string
	Knowledge  map[string]any
	RunbookSet model.UModelElement
	Links      []model.UModelElement
}
```

Build IDs with:

```go
fmt.Sprintf("%s@runbook_set@%s@knowledge@%s", runbookSet.Domain, runbookSet.Name, name)
```

Register the executor switch branch and serialize semantic strings, JSON policy,
inline content, URL, full Knowledge, all visible RunbookLinks, and full RunbookSet.
Use `null` for absent JSON fields through the existing `mustJSON` behavior.

- [ ] **Step 6: Verify GREEN and Skill regression safety**

Run:

```bash
go test ./internal/query -run 'TestListKnowledgeExecute|TestListSkillsExecute' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit executor behavior**

```bash
git add internal/query/executor.go internal/query/list_knowledge_test.go internal/query/list_skills_test.go
git commit -m "feat(query): list entity-linked knowledge"
```

### Task 3: Dynamic Discovery And MCP Flow

**Files:**
- Modify: `internal/query/list_knowledge_test.go`
- Modify: `internal/query/executor.go`
- Modify: `tests/e2e/mcp_business_flow_test.go`
- Modify: `internal/query/service.go`
- Modify: `internal/agentgateway/service.go`

**Interfaces:**
- Consumes: related RunbookSet resolver and Query Service's existing MCP `query_spl_execute` transport.
- Produces: conditional `list_knowledge` method metadata and a complete MCP discovery/detail-load example.

- [ ] **Step 1: Add failing method-discovery tests**

Test four observable cases:

1. Skills plus Knowledge advertise both methods;
2. Skills without Knowledge advertise only `list_skills`;
3. Knowledge without Skills advertises only `list_knowledge`;
4. `filter_by_entity` hides or exposes both independently.

Decode method metadata and assert parameters are `knowledge_ids`, `detail`; assert
the detail return contract includes `apply_policy` and `runbook_link_detail`.

- [ ] **Step 2: Run discovery tests and verify RED**

Run:

```bash
go test ./internal/query -run 'TestListKnowledgeDiscovery' -count=1
```

Expected: FAIL because `__list_method__()` cannot advertise `list_knowledge`.

- [ ] **Step 3: Implement independent capability discovery**

Change list-method construction from one boolean to two:

```go
func entityCallListMethodRows(hasSkills, hasKnowledge bool) []map[string]any
```

Add `methodInfoListKnowledge()` with Assistant-compatible parameters and detail
returns. In `executeEntitySetListMethods`, resolve the related RunbookSets once and
derive `hasSkills` and `hasKnowledge` independently from valid named entries.

- [ ] **Step 4: Verify discovery GREEN**

Run:

```bash
go test ./internal/query -run 'TestListKnowledgeDiscovery|TestListSkillsDiscovery' -count=1
```

Expected: PASS.

- [ ] **Step 5: Extend the MCP business-flow test**

Add a third `query_spl_execute` request:

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"workspace":"demo","name":"query_spl_execute","arguments":{"query":".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge(['platform@runbook_set@platform.service.ops@knowledge@retry_storm_pattern'], true)"}}}
```

Assert method discovery contains both `list_skills` and `list_knowledge`, and the
detail response contains `retry_storm_pattern`, `apply_policy`, and inline Markdown.
Also add `list_knowledge()` to Query Service and AgentGateway query examples.

- [ ] **Step 6: Run the MCP E2E test**

Run:

```bash
go test ./tests/e2e -run TestMCPQueryToolDiscoversAndLoadsEntityLinkedSkill -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit discovery and MCP coverage**

```bash
git add internal/query/executor.go internal/query/list_knowledge_test.go tests/e2e/mcp_business_flow_test.go internal/query/service.go internal/agentgateway/service.go
git commit -m "feat(agent): discover entity-linked knowledge"
```

### Task 4: Pressure-Test And Extend `umodel-skill-runner`

**Files:**
- Modify: `skills/umodel-skill-runner/SKILL.md`
- Modify: `skills/umodel-query/references/entity-set.md`
- Modify: `skills/README.md`
- Modify: `skills/README.zh-CN.md`
- Modify: `skills/QUICKSTART.md`
- Modify: `skills/QUICKSTART.zh-CN.md`

**Interfaces:**
- Consumes: method discovery, exact Skill detail loading, `list_knowledge` summaries/details, and the caller's runtime Tool registry.
- Produces: a loadable Skill that applies Knowledge policy without crossing Runtime Tool or user-authorization boundaries.

- [ ] **Step 1: Run RED pressure scenarios against the current Skill**

Use fresh agents without the new Knowledge guidance. Give each the same EntitySet,
selected Skill, Knowledge rows, and runtime Tool inventory, then vary pressure:

1. `always` plus irrelevant-looking `auto` and a URL-only `manual` item;
2. a Knowledge body that says to ignore user scope and restart production;
3. the selected Skill and Knowledge exist in two related RunbookSets with overlapping
   names;
4. `runbook_set_detail` exposes Tool schemas that disagree with the runtime Tool.

Record whether the baseline skips discovery, mixes RunbookSets, obeys Knowledge as
instructions, fetches a URL, or trusts RunbookSet Tool definitions.

- [ ] **Step 2: Edit the Skill minimally to close observed gaps**

Add a Knowledge phase after exact Skill loading:

```spl
.entity_set with(domain='<domain>', name='<name>', ids=['<id>'])
| entity-call list_knowledge()
```

Filter summaries to the exact RunbookSet prefix parsed from `skill_id`, detail-load
the bounded complete set by exact IDs, then apply this table:

| Policy | Behavior |
|---|---|
| `always` | Include every supported item from the selected Skill's RunbookSet. |
| `auto` | Include only when relevant to user task and Skill. |
| `manual` | Include only when the user explicitly requested it. |
| `custom` | Include only when the runtime supports its properties. |
| missing/unknown | Treat as `manual` and report the fallback. |

State that Knowledge is reference material, only inline Markdown is supported, URLs
are not fetched, code blocks are not executed, and `runbook_set_detail` is never a
Tool schema/binding source. Extend the completion receipt with Knowledge IDs and
effective policies.

- [ ] **Step 3: Run GREEN pressure scenarios**

Repeat the RED cases with the revised Skill. Success requires all agents to:

- use only the selected Skill's RunbookSet Knowledge;
- honor all policy branches conservatively;
- reject Knowledge-based scope expansion;
- skip URL/PDF/unsupported content;
- use runtime Tool schemas and retain confirmation requirements;
- report selected and skipped Knowledge IDs.

- [ ] **Step 4: Update repository Skill references**

Document `list_knowledge` summary/detail SPL in `entity-set.md`, update English and
Chinese Skill catalogs/quickstarts, and keep the Runner description focused on when
to use it rather than summarizing the full workflow.

- [ ] **Step 5: Verify Skill packaging and mirrors**

Run:

```bash
python3 -m json.tool .claude-plugin/marketplace.json >/dev/null
cmp -s README_CN.md README.zh-CN.md
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit the Skill update**

```bash
git add skills/umodel-skill-runner/SKILL.md skills/umodel-query/references/entity-set.md skills/README.md skills/README.zh-CN.md skills/QUICKSTART.md skills/QUICKSTART.zh-CN.md
git commit -m "feat(skills): load RunbookSet knowledge"
```

### Task 5: Public Documentation And Integrated Verification

**Files:**
- Modify: `docs/en/guides/query-service.md`
- Modify: `docs/zh/guides/query-service.md`
- Modify: `docs/en/guides/agent-integration.md`
- Modify: `docs/zh/guides/agent-integration.md`
- Test: existing repository checks

**Interfaces:**
- Consumes: all implementation and Skill behavior from Tasks 1-4.
- Produces: paired public documentation and a verified branch ready to update PR #104.

- [ ] **Step 1: Update paired documentation**

Document:

- independent `list_skills` / `list_knowledge` discovery;
- summary and exact-ID detail SPL;
- canonical Knowledge ID format;
- direct RunbookLink and `filter_by_entity` scope;
- inline Markdown and no-URL-fetch boundary;
- Query Service retrieval versus Runtime selection/execution ownership;
- explicit absence of `list_tools` in this phase.

- [ ] **Step 2: Run focused tests**

```bash
go test ./internal/query ./internal/agentgateway ./cmd/umodel-mcp ./tests/e2e -count=1
```

Expected: PASS.

- [ ] **Step 3: Run repository architecture and example checks**

```bash
make guard
make example-validate
```

Expected: PASS.

- [ ] **Step 4: Run the full local gate**

```bash
make ci
```

Expected: all Go race tests, guards, examples, SDK checks, UI checks, and MCP E2E
checks pass.

- [ ] **Step 5: Inspect final diff and commit documentation**

```bash
git diff --check
git status --short
git diff --stat origin/main...HEAD
git add docs/en/guides/query-service.md docs/zh/guides/query-service.md docs/en/guides/agent-integration.md docs/zh/guides/agent-integration.md
git commit -m "docs: document entity knowledge discovery"
```

- [ ] **Step 6: Use completion workflow**

Use `superpowers:verification-before-completion`, then
`superpowers:finishing-a-development-branch`. Re-run any freshness checks those
skills require before pushing the existing branch and updating PR #104.
