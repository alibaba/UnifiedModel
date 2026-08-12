package query

import (
	"reflect"
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
