package query

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestListSkillsPlannerNormalizesArguments(t *testing.T) {
	caps := model.GraphStoreCapabilities{MaxDepth: 1, MaxLimit: 1000}
	skillID := "platform@runbook_set@platform.service.ops@skills@incident-investigation"

	plan, err := (Planner{}).Plan(model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills(['" + skillID + "'], true)",
	}, caps)
	if err != nil {
		t.Fatalf("plan list_skills: %v", err)
	}
	if plan.EntityCall == nil || plan.EntityCall.Name != "list_skills" {
		t.Fatalf("unexpected entity call: %+v", plan.EntityCall)
	}
	if !reflect.DeepEqual(plan.EntityCall.Parameters["skill_ids"], []string{skillID}) {
		t.Fatalf("skill_ids = %#v", plan.EntityCall.Parameters["skill_ids"])
	}
	if plan.EntityCall.Parameters["detail"] != true {
		t.Fatalf("detail = %#v", plan.EntityCall.Parameters["detail"])
	}

	alias, err := (Planner{}).Plan(model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skill(detail=true)",
	}, caps)
	if err != nil {
		t.Fatalf("plan list_skill alias: %v", err)
	}
	if alias.EntityCall == nil || alias.EntityCall.Name != "list_skills" {
		t.Fatalf("alias did not normalize: %+v", alias.EntityCall)
	}
	if !reflect.DeepEqual(alias.EntityCall.Parameters["skill_ids"], []string(nil)) || alias.EntityCall.Parameters["detail"] != true {
		t.Fatalf("unexpected alias defaults: %#v", alias.EntityCall.Parameters)
	}
}

func TestListSkillsPlannerRejectsWrongArgumentTypes(t *testing.T) {
	caps := model.GraphStoreCapabilities{MaxDepth: 1, MaxLimit: 1000}
	queries := []string{
		".entity_set with(domain='platform', name='platform.service') | entity-call list_skills('not-an-array')",
		".entity_set with(domain='platform', name='platform.service') | entity-call list_skills([], 'not-a-bool')",
	}
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			_, err := (Planner{}).Plan(model.QueryRequest{Query: query}, caps)
			if !apperrors.IsCode(err, apperrors.CodeQueryPlanError) {
				t.Fatalf("expected QUERY_PLAN_ERROR, got %v", err)
			}
		})
	}
}

func TestListSkillsExecuteReturnsRelatedSkills(t *testing.T) {
	svc := listSkillsTestService(t, listSkillsElements(false, false))
	result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills()",
	})
	if err != nil {
		t.Fatalf("execute list_skills: %v", err)
	}

	header, rows := listSkillsTable(t, result)
	wantHeader := []string{"skill_id", "skill_name", "display_name", "description", "license", "compatibility", "allowed_tools", "skill_url", "priority", "metadata", "tags", "files"}
	if !reflect.DeepEqual(header, wantHeader) {
		t.Fatalf("header = %#v, want %#v", header, wantHeader)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two skills", rows)
	}
	wantID := "platform@runbook_set@platform.service.ops@skills@incident-investigation"
	if rows[0][0] != wantID || rows[0][1] != "incident-investigation" || rows[0][2] != "故障排查" || rows[0][3] != "排查故障" {
		t.Fatalf("first skill = %#v", rows[0])
	}
	if rows[0][6] != "query_spl_execute rollback_config_change" || rows[0][8] != "1" {
		t.Fatalf("first skill capability fields = %#v", rows[0])
	}
	var files []map[string]any
	if err := json.Unmarshal([]byte(rows[0][11]), &files); err != nil || len(files) != 1 || files[0]["name"] != "SKILL.md" {
		t.Fatalf("files = %q, err=%v", rows[0][11], err)
	}
	if rows[1][9] != "null" || rows[1][10] != "null" || rows[1][11] != "null" {
		t.Fatalf("absent detail fields = %#v, want JSON nulls", rows[1][9:12])
	}
}

func TestListSkillsExecuteFiltersExactIDAndReturnsDetail(t *testing.T) {
	svc := listSkillsTestService(t, listSkillsElements(false, false))
	skillID := "platform@runbook_set@platform.service.ops@skills@capacity-review"
	result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skill(['" + skillID + "'], detail=true)",
	})
	if err != nil {
		t.Fatalf("execute detailed list_skill: %v", err)
	}

	header, rows := listSkillsTable(t, result)
	if len(header) != 13 || header[12] != "skill_detail" || len(rows) != 1 {
		t.Fatalf("unexpected detailed table: header=%#v rows=%#v", header, rows)
	}
	if rows[0][0] != skillID || rows[0][1] != "capacity-review" {
		t.Fatalf("filtered row = %#v", rows[0])
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(rows[0][12]), &detail); err != nil || detail["name"] != "capacity-review" {
		t.Fatalf("skill_detail = %q, err=%v", rows[0][12], err)
	}
}

func TestListSkillsExecuteDeduplicatesLinksAndSkipsMissingDestination(t *testing.T) {
	svc := listSkillsTestService(t, listSkillsElements(true, true))
	result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills()",
	})
	if err != nil {
		t.Fatalf("execute list_skills: %v", err)
	}
	_, rows := listSkillsTable(t, result)
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two deduplicated skills", rows)
	}

	unknown, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills(['platform@runbook_set@platform.service.ops@skills@missing'])",
	})
	if err != nil {
		t.Fatalf("execute unknown skill filter: %v", err)
	}
	_, unknownRows := listSkillsTable(t, unknown)
	if len(unknownRows) != 0 {
		t.Fatalf("unknown skill rows = %#v, want empty", unknownRows)
	}
}

func TestListSkillsExecuteAppliesFilterByEntity(t *testing.T) {
	elements := listSkillsElements(false, false)
	for i := range elements {
		if elements[i].Kind == "runbook_link" {
			elements[i].Spec["filter_by_entity"] = "environment = 'prod'"
		}
	}
	svc := listSkillsTestService(t, elements)

	staging, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills()",
		FilterByEntities: &model.EntityData{
			Header: []string{"environment"},
			Data:   [][]string{{"staging"}},
		},
	})
	if err != nil {
		t.Fatalf("execute staging list_skills: %v", err)
	}
	_, stagingRows := listSkillsTable(t, staging)
	if len(stagingRows) != 0 {
		t.Fatalf("staging rows = %#v, want empty", stagingRows)
	}

	prod, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_skills()",
		FilterByEntities: &model.EntityData{
			Header: []string{"environment"},
			Data:   [][]string{{"prod"}},
		},
	})
	if err != nil {
		t.Fatalf("execute prod list_skills: %v", err)
	}
	_, prodRows := listSkillsTable(t, prod)
	if len(prodRows) != 2 {
		t.Fatalf("prod rows = %#v, want two", prodRows)
	}
}

func TestListSkillsDiscoveryIncludesMethodOnlyWhenRelatedSkillExists(t *testing.T) {
	svc := listSkillsTestService(t, append(listSkillsElements(false, false),
		model.UModelElement{Kind: "entity_set", Domain: "platform", Name: "platform.database"},
	))

	serviceResult, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call __list_method__()",
	})
	if err != nil {
		t.Fatalf("discover service methods: %v", err)
	}
	serviceMethods := listMethodRowsByName(t, serviceResult)
	listSkills, ok := serviceMethods["list_skills"]
	if !ok {
		t.Fatalf("service methods = %#v, want list_skills", serviceMethods)
	}
	var params []map[string]any
	if err := json.Unmarshal([]byte(listSkills[3]), &params); err != nil {
		t.Fatalf("decode list_skills params: %v", err)
	}
	if len(params) != 2 || params[0]["key"] != "skill_ids" || params[1]["key"] != "detail" {
		t.Fatalf("list_skills params = %#v", params)
	}
	var returns []map[string]any
	if err := json.Unmarshal([]byte(listSkills[4]), &returns); err != nil {
		t.Fatalf("decode list_skills returns: %v", err)
	}
	if len(returns) < 9 || returns[8]["key"] != "priority" || returns[8]["type"] != "integer" {
		t.Fatalf("list_skills returns = %#v", returns)
	}

	databaseResult, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.database') | entity-call __list_method__()",
	})
	if err != nil {
		t.Fatalf("discover database methods: %v", err)
	}
	if _, ok := listMethodRowsByName(t, databaseResult)["list_skills"]; ok {
		t.Fatalf("unrelated EntitySet must not advertise list_skills")
	}
}

func TestListSkillsDiscoveryAppliesFilterByEntity(t *testing.T) {
	elements := listSkillsElements(false, false)
	for i := range elements {
		if elements[i].Kind == "runbook_link" {
			elements[i].Spec["filter_by_entity"] = "environment = 'prod'"
		}
	}
	svc := listSkillsTestService(t, elements)

	staging, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call __list_method__()",
		FilterByEntities: &model.EntityData{
			Header: []string{"environment"},
			Data:   [][]string{{"staging"}},
		},
	})
	if err != nil {
		t.Fatalf("discover staging methods: %v", err)
	}
	if _, ok := listMethodRowsByName(t, staging)["list_skills"]; ok {
		t.Fatalf("staging Entity must not advertise filtered list_skills")
	}

	prod, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call __list_method__()",
		FilterByEntities: &model.EntityData{
			Header: []string{"environment"},
			Data:   [][]string{{"prod"}},
		},
	})
	if err != nil {
		t.Fatalf("discover prod methods: %v", err)
	}
	if _, ok := listMethodRowsByName(t, prod)["list_skills"]; !ok {
		t.Fatalf("prod Entity must advertise list_skills")
	}
}

func listSkillsTestService(t *testing.T, elements []model.UModelElement) *Service {
	t.Helper()
	store := graphstore.NewMemoryStore()
	if _, err := store.PutUModelElements(context.Background(), model.UModelElementBatch{Workspace: "demo", Elements: elements}); err != nil {
		t.Fatalf("put list-skills fixture: %v", err)
	}
	return NewService(store)
}

func listSkillsElements(duplicateLink, missingDestination bool) []model.UModelElement {
	link := model.UModelElement{
		Kind:   "runbook_link",
		Domain: "platform",
		Name:   "platform.service_to_platform.service.ops",
		Spec: map[string]any{
			"src":  map[string]any{"domain": "platform", "kind": "entity_set", "name": "platform.service"},
			"dest": map[string]any{"domain": "platform", "kind": "runbook_set", "name": "platform.service.ops"},
		},
	}
	elements := []model.UModelElement{
		{Kind: "entity_set", Domain: "platform", Name: "platform.service"},
		link,
		{
			Kind: "runbook_set", Domain: "platform", Name: "platform.service.ops", Version: "v1.0.0",
			Spec: map[string]any{"skills": []any{
				map[string]any{
					"name":         "incident-investigation",
					"display_name": map[string]any{"en_us": "Incident Investigation", "zh_cn": "故障排查"},
					"description":  map[string]any{"en_us": "Investigate an incident", "zh_cn": "排查故障"},
					"license":      "Apache-2.0", "compatibility": "umctl or MCP",
					"allowed_tools": "query_spl_execute rollback_config_change", "priority": 1,
					"metadata": map[string]any{"author": "ops-team"}, "tags": map[string]any{"trigger": "incident"},
					"files": []any{map[string]any{"name": "SKILL.md", "content": "# Investigate"}},
				},
				map[string]any{
					"name":          "capacity-review",
					"display_name":  map[string]any{"en_us": "Capacity Review"},
					"description":   map[string]any{"en_us": "Review capacity"},
					"allowed_tools": "query_spl_execute", "priority": 3,
				},
			}},
		},
	}
	if duplicateLink {
		duplicate := link
		duplicate.Name = "platform.service_to_platform.service.ops.duplicate"
		elements = append(elements, duplicate)
	}
	if missingDestination {
		elements = append(elements, model.UModelElement{
			Kind: "runbook_link", Domain: "platform", Name: "platform.service_to_missing",
			Spec: map[string]any{
				"src":  map[string]any{"domain": "platform", "kind": "entity_set", "name": "platform.service"},
				"dest": map[string]any{"domain": "platform", "kind": "runbook_set", "name": "platform.missing"},
			},
		})
	}
	return elements
}

func listSkillsTable(t *testing.T, result model.QueryResult) ([]string, [][]string) {
	t.Helper()
	if len(result.Rows) != 1 {
		t.Fatalf("result rows = %#v", result.Rows)
	}
	header, ok := result.Rows[0]["header"].([]string)
	if !ok {
		t.Fatalf("header = %#v", result.Rows[0]["header"])
	}
	rawRows, ok := result.Rows[0]["data"].([]map[string]any)
	if !ok {
		t.Fatalf("data = %#v", result.Rows[0]["data"])
	}
	rows := make([][]string, 0, len(rawRows))
	for _, row := range rawRows {
		values, ok := row["values"].([]string)
		if !ok {
			t.Fatalf("row = %#v", row)
		}
		rows = append(rows, values)
	}
	return header, rows
}

func listMethodRowsByName(t *testing.T, result model.QueryResult) map[string][]string {
	t.Helper()
	_, rows := listSkillsTable(t, result)
	out := make(map[string][]string, len(rows))
	for _, row := range rows {
		if len(row) > 0 {
			out[row[0]] = row
		}
	}
	return out
}
