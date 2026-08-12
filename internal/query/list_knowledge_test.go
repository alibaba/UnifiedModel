package query

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

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

func TestListKnowledgeExecuteReturnsRelatedKnowledge(t *testing.T) {
	svc := listSkillsTestService(t, listKnowledgeElements(false, false))
	result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge()",
	})
	if err != nil {
		t.Fatalf("execute list_knowledge: %v", err)
	}

	header, rows := listSkillsTable(t, result)
	wantHeader := []string{"knowledge_id", "knowledge_name", "display_name", "description", "content_type"}
	if !reflect.DeepEqual(header, wantHeader) {
		t.Fatalf("header = %#v, want %#v", header, wantHeader)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two Knowledge items", rows)
	}
	wantID := "platform@runbook_set@platform.service.ops@knowledge@retry_storm_pattern"
	if rows[0][0] != wantID || rows[0][1] != "retry_storm_pattern" || rows[0][2] != "重试风暴模式" || rows[0][3] != "重试放大指南" || rows[0][4] != "markdown" {
		t.Fatalf("first Knowledge = %#v", rows[0])
	}
	if rows[1][1] != "deployment_triage_guide" || rows[1][4] != "url" {
		t.Fatalf("second Knowledge = %#v", rows[1])
	}
}

func TestListKnowledgeExecuteFiltersExactIDAndReturnsDetail(t *testing.T) {
	svc := listSkillsTestService(t, listKnowledgeElements(false, false))
	knowledgeID := "platform@runbook_set@platform.service.ops@knowledge@retry_storm_pattern"
	result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge(['" + knowledgeID + "'], true)",
	})
	if err != nil {
		t.Fatalf("execute detailed list_knowledge: %v", err)
	}

	header, rows := listSkillsTable(t, result)
	wantHeader := []string{
		"knowledge_id", "knowledge_name", "display_name", "description", "content_type",
		"apply_policy", "content", "content_url", "knowledge_detail",
		"runbook_link_detail", "runbook_set_detail",
	}
	if !reflect.DeepEqual(header, wantHeader) || len(rows) != 1 {
		t.Fatalf("unexpected detailed table: header=%#v rows=%#v", header, rows)
	}
	if rows[0][0] != knowledgeID || rows[0][6] != "# Retry Storm Pattern\nUse evidence before remediation." || rows[0][7] != "" {
		t.Fatalf("filtered row = %#v", rows[0])
	}

	var policy map[string]any
	if err := json.Unmarshal([]byte(rows[0][5]), &policy); err != nil || policy["apply_type"] != "auto" {
		t.Fatalf("apply_policy = %q, err=%v", rows[0][5], err)
	}
	var knowledge map[string]any
	if err := json.Unmarshal([]byte(rows[0][8]), &knowledge); err != nil || knowledge["name"] != "retry_storm_pattern" || knowledge["priority"] != float64(1) {
		t.Fatalf("knowledge_detail = %q, err=%v", rows[0][8], err)
	}
	var links []map[string]any
	if err := json.Unmarshal([]byte(rows[0][9]), &links); err != nil || len(links) != 1 || links[0]["kind"] != "runbook_link" {
		t.Fatalf("runbook_link_detail = %q, err=%v", rows[0][9], err)
	}
	var runbookSet map[string]any
	if err := json.Unmarshal([]byte(rows[0][10]), &runbookSet); err != nil || runbookSet["name"] != "platform.service.ops" {
		t.Fatalf("runbook_set_detail = %q, err=%v", rows[0][10], err)
	}
}

func TestListKnowledgeExecuteDeduplicatesLinksAndSkipsMissingDestination(t *testing.T) {
	svc := listSkillsTestService(t, listKnowledgeElements(true, true))
	result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge([], true)",
	})
	if err != nil {
		t.Fatalf("execute list_knowledge: %v", err)
	}
	_, rows := listSkillsTable(t, result)
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two deduplicated Knowledge items", rows)
	}
	for _, row := range rows {
		var links []map[string]any
		if err := json.Unmarshal([]byte(row[9]), &links); err != nil || len(links) != 2 {
			t.Fatalf("runbook_link_detail = %q, want two links, err=%v", row[9], err)
		}
	}

	unknown, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge(['platform@runbook_set@platform.service.ops@knowledge@missing'])",
	})
	if err != nil {
		t.Fatalf("execute unknown Knowledge filter: %v", err)
	}
	_, unknownRows := listSkillsTable(t, unknown)
	if len(unknownRows) != 0 {
		t.Fatalf("unknown Knowledge rows = %#v, want empty", unknownRows)
	}
}

func TestListKnowledgeExecuteAppliesFilterByEntity(t *testing.T) {
	elements := listKnowledgeElements(false, false)
	for i := range elements {
		if elements[i].Kind == "runbook_link" {
			elements[i].Spec["filter_by_entity"] = "environment = 'prod'"
		}
	}
	svc := listSkillsTestService(t, elements)

	for _, tc := range []struct {
		name      string
		env       string
		wantCount int
	}{
		{name: "hidden", env: "staging", wantCount: 0},
		{name: "visible", env: "prod", wantCount: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
				Query: ".entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge()",
				FilterByEntities: &model.EntityData{
					Header: []string{"environment"},
					Data:   [][]string{{tc.env}},
				},
			})
			if err != nil {
				t.Fatalf("execute %s list_knowledge: %v", tc.env, err)
			}
			_, rows := listSkillsTable(t, result)
			if len(rows) != tc.wantCount {
				t.Fatalf("%s rows = %#v, want %d", tc.env, rows, tc.wantCount)
			}
		})
	}
}

func TestListKnowledgeDiscoveryAdvertisesCapabilitiesIndependently(t *testing.T) {
	knowledgeOnly := listKnowledgeElements(false, false)
	for i := range knowledgeOnly {
		if knowledgeOnly[i].Kind == "runbook_set" {
			delete(knowledgeOnly[i].Spec, "skills")
		}
	}

	for _, tc := range []struct {
		name          string
		elements      []model.UModelElement
		wantSkills    bool
		wantKnowledge bool
	}{
		{name: "skills and Knowledge", elements: listKnowledgeElements(false, false), wantSkills: true, wantKnowledge: true},
		{name: "skills only", elements: listSkillsElements(false, false), wantSkills: true, wantKnowledge: false},
		{name: "Knowledge only", elements: knowledgeOnly, wantSkills: false, wantKnowledge: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := listSkillsTestService(t, tc.elements)
			result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
				Query: ".entity_set with(domain='platform', name='platform.service') | entity-call __list_method__()",
			})
			if err != nil {
				t.Fatalf("discover methods: %v", err)
			}
			methods := listMethodRowsByName(t, result)
			_, hasSkills := methods["list_skills"]
			knowledgeMethod, hasKnowledge := methods["list_knowledge"]
			if hasSkills != tc.wantSkills || hasKnowledge != tc.wantKnowledge {
				t.Fatalf("methods = %#v, want skills=%t Knowledge=%t", methods, tc.wantSkills, tc.wantKnowledge)
			}
			if !hasKnowledge {
				return
			}

			var params []map[string]any
			if err := json.Unmarshal([]byte(knowledgeMethod[3]), &params); err != nil {
				t.Fatalf("decode list_knowledge params: %v", err)
			}
			if len(params) != 2 || params[0]["key"] != "knowledge_ids" || params[1]["key"] != "detail" {
				t.Fatalf("list_knowledge params = %#v", params)
			}
			var returns []map[string]any
			if err := json.Unmarshal([]byte(knowledgeMethod[4]), &returns); err != nil {
				t.Fatalf("decode list_knowledge returns: %v", err)
			}
			if len(returns) != 11 || returns[5]["key"] != "apply_policy" || returns[9]["key"] != "runbook_link_detail" {
				t.Fatalf("list_knowledge returns = %#v", returns)
			}
		})
	}
}

func TestListKnowledgeDiscoveryAppliesFilterByEntity(t *testing.T) {
	elements := listKnowledgeElements(false, false)
	for i := range elements {
		if elements[i].Kind == "runbook_link" {
			elements[i].Spec["filter_by_entity"] = "environment = 'prod'"
		}
	}
	svc := listSkillsTestService(t, elements)

	for _, tc := range []struct {
		name        string
		env         string
		wantMethods bool
	}{
		{name: "hidden", env: "staging", wantMethods: false},
		{name: "visible", env: "prod", wantMethods: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
				Query: ".entity_set with(domain='platform', name='platform.service') | entity-call __list_method__()",
				FilterByEntities: &model.EntityData{
					Header: []string{"environment"},
					Data:   [][]string{{tc.env}},
				},
			})
			if err != nil {
				t.Fatalf("discover %s methods: %v", tc.env, err)
			}
			methods := listMethodRowsByName(t, result)
			_, hasSkills := methods["list_skills"]
			_, hasKnowledge := methods["list_knowledge"]
			if hasSkills != tc.wantMethods || hasKnowledge != tc.wantMethods {
				t.Fatalf("%s methods = %#v, want both=%t", tc.env, methods, tc.wantMethods)
			}
		})
	}
}

func TestQueryExamplesIncludeListKnowledge(t *testing.T) {
	examples, err := (&Service{}).Examples(context.Background())
	if err != nil {
		t.Fatalf("list Query Service examples: %v", err)
	}
	for _, example := range examples {
		if strings.Contains(example, "entity-call list_knowledge(") {
			return
		}
	}
	t.Fatalf("Query Service examples = %#v, want list_knowledge", examples)
}

func listKnowledgeElements(duplicateLink, missingDestination bool) []model.UModelElement {
	elements := listSkillsElements(duplicateLink, missingDestination)
	for i := range elements {
		if elements[i].Kind != "runbook_set" || elements[i].Domain != "platform" || elements[i].Name != "platform.service.ops" {
			continue
		}
		elements[i].Spec["knowledge"] = []any{
			map[string]any{
				"name":         "retry_storm_pattern",
				"display_name": map[string]any{"en_us": "Retry Storm Pattern", "zh_cn": "重试风暴模式"},
				"description":  map[string]any{"en_us": "Retry amplification guidance", "zh_cn": "重试放大指南"},
				"apply_policy": map[string]any{"apply_type": "auto"},
				"content_type": "markdown",
				"content":      "# Retry Storm Pattern\nUse evidence before remediation.",
				"priority":     1,
				"tags":         map[string]any{"category": "failure_pattern"},
				"last_updated": "2024-06-01T00:00:00Z",
			},
			map[string]any{
				"name":         "deployment_triage_guide",
				"display_name": map[string]any{"en_us": "Deployment Triage"},
				"description":  map[string]any{"en_us": "Rule out harmless deployments"},
				"apply_policy": map[string]any{"apply_type": "manual"},
				"content_type": "url",
				"content_url":  "https://example.invalid/deployment-triage",
			},
		}
	}
	return elements
}
