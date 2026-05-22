package agentgateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/internal/umodel"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestToolsKeepWriteToolsDisabledByDefault(t *testing.T) {
	svc := NewService(fakeQuery{})
	tools, err := svc.Tools(context.Background())
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	byName := map[string]model.AgentTool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	if !byName["query_spl_execute"].Enabled {
		t.Fatalf("query tool should be enabled: %+v", byName["query_spl_execute"])
	}
	if byName["query_spl_execute"].InputSchema == nil || byName["query_spl_execute"].OutputSchema == nil {
		t.Fatalf("query tool should expose call schemas: %+v", byName["query_spl_execute"])
	}
	if byName["entity_write"].Enabled || !byName["entity_write"].RequiresExplicitWriteEnable {
		t.Fatalf("write tool should be disabled by default: %+v", byName["entity_write"])
	}
	if byName["umodel_import"].Enabled || byName["entity_expire"].Enabled {
		t.Fatalf("all write tools should be disabled by default: %+v %+v", byName["umodel_import"], byName["entity_expire"])
	}
}

func TestExecuteQueryTool(t *testing.T) {
	svc := NewService(fakeQuery{})
	result, err := svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{
		Name:      "query_spl_execute",
		Arguments: map[string]any{"query": ".umodel | limit 1"},
	})
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if !result.OK || result.Name != "query_spl_execute" {
		t.Fatalf("unexpected result: %+v", result)
	}

	_, err = svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{Name: "entity_write"})
	if !apperrors.IsCode(err, apperrors.CodeToolDisabled) {
		t.Fatalf("expected disabled write tool, got %v", err)
	}

	_, err = svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{Name: "query_spl_execute"})
	if !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("expected missing query validation, got %v", err)
	}
}

func TestExecuteExplainToolAndRejectUnknownToolOrResource(t *testing.T) {
	query := &recordingQuery{}
	svc := NewService(query)

	result, err := svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{
		Name:      "query_spl_explain",
		Arguments: map[string]any{"query": ".entity with(domain='apm') | limit 1"},
	})
	if err != nil {
		t.Fatalf("explain tool: %v", err)
	}
	if !result.OK || result.Name != "query_spl_explain" || query.explainCalls != 1 {
		t.Fatalf("unexpected explain result=%+v calls=%d", result, query.explainCalls)
	}
	if query.lastWorkspace != "demo" || query.lastQuery != ".entity with(domain='apm') | limit 1" {
		t.Fatalf("explain tool should call Query Service with original request, got workspace=%q query=%q", query.lastWorkspace, query.lastQuery)
	}

	_, err = svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{Name: "unknown_tool"})
	if !apperrors.IsCode(err, apperrors.CodeToolNotFound) {
		t.Fatalf("expected unknown tool error, got %v", err)
	}
	_, err = svc.ReadResource(context.Background(), "demo", model.AgentResourceReadRequest{URI: "umodel://workspace/demo/runtime-rows"})
	if !apperrors.IsCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected unknown resource error, got %v", err)
	}
}

func TestDiscoverReturnsStructuredResourcesAndQueryNextActions(t *testing.T) {
	svc := NewService(fakeQuery{})
	discovery, err := svc.Discover(context.Background(), "demo")
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(discovery.Resources) != 4 {
		t.Fatalf("expected resource catalog, got %+v", discovery.Resources)
	}
	for _, resource := range discovery.Resources {
		if resource.URI == "" || resource.Kind == "" || !resource.ReadOnly {
			t.Fatalf("resource should be structured read-only metadata: %+v", resource)
		}
	}
	if len(discovery.NextActions) == 0 {
		t.Fatalf("expected query next actions")
	}
	for _, action := range discovery.NextActions {
		if action.Tool != "query_spl_execute" {
			t.Fatalf("next action must use query tool: %+v", action)
		}
		if action.QueryAPI.Method != "POST" || action.QueryAPI.Path != "/api/v1/query/demo/execute" {
			t.Fatalf("next action must point at Query API: %+v", action)
		}
		if action.QueryAPI.Body.Query == "" {
			t.Fatalf("next action should include query request body: %+v", action)
		}
	}
}

func TestReadResourcesDoNotLeakRuntimeResults(t *testing.T) {
	svc := NewService(fakeQuery{})
	for _, resource := range resourceCatalog("demo") {
		result, err := svc.ReadResource(context.Background(), "demo", model.AgentResourceReadRequest{URI: resource.URI})
		if err != nil {
			t.Fatalf("read resource %s: %v", resource.URI, err)
		}
		payload, err := json.Marshal(result.Content)
		if err != nil {
			t.Fatalf("marshal resource: %v", err)
		}
		text := string(payload)
		for _, leaked := range []string{"cart-runtime-id", "checkout-runtime-id", "\"rows\""} {
			if strings.Contains(text, leaked) {
				t.Fatalf("resource leaked runtime result marker %q in %s: %s", leaked, resource.URI, text)
			}
		}
	}
}

func TestUModelValidateSurfacesSchemaErrorsAndWarnings(t *testing.T) {
	graph := graphstore.NewMemoryStore()
	umodelSvc := umodel.NewService(graph)
	svc := NewService(fakeQuery{}, WithWriteServices(umodelSvc, &fakeEntityStore{}))

	// entity_set_link with bad shape: wrong field names + missing required entity_link_type.
	result, err := svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{
		Name: "umodel_validate",
		Arguments: map[string]any{"elements": []map[string]any{{
			"kind":   "entity_set_link",
			"domain": "demo",
			"name":   "demo.bad",
			"spec": map[string]any{
				"source":        map[string]any{"domain": "demo", "name": "a"},
				"destination":   map[string]any{"domain": "demo", "name": "b"},
				"relation_type": "depends_on",
			},
		}}},
	})
	if err != nil {
		t.Fatalf("umodel_validate: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected ok envelope, got %+v", result)
	}
	body, err := json.Marshal(result.Output)
	if err != nil {
		t.Fatalf("marshal output: %v", err)
	}
	var validation model.ValidationResult
	if err := json.Unmarshal(body, &validation); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if validation.Valid {
		t.Fatalf("expected invalid, got %+v", validation)
	}
	if len(validation.Errors) == 0 || !strings.Contains(validation.Errors[0].Reason, "required") {
		t.Fatalf("expected required-missing error, got %+v", validation.Errors)
	}
	if len(validation.Warnings) < 3 {
		t.Fatalf("expected at least 3 warnings for unknown fields, got %+v", validation.Warnings)
	}
}

func TestWriteToolsCallServiceLayerWhenExplicitlyEnabled(t *testing.T) {
	umodelSvc := &fakeUModel{}
	entitySvc := &fakeEntityStore{}
	svc := NewService(fakeQuery{}, WithWriteToolsEnabled(true), WithWriteServices(umodelSvc, entitySvc))

	_, err := svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{
		Name: "umodel_import",
		Arguments: map[string]any{"elements": []map[string]any{{
			"kind":   "entity_set",
			"domain": "apm",
			"name":   "apm.service",
		}}},
	})
	if err != nil {
		t.Fatalf("umodel_import: %v", err)
	}
	if umodelSvc.putCalls != 1 {
		t.Fatalf("umodel_import should call service layer, got %d", umodelSvc.putCalls)
	}

	_, err = svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{
		Name: "entity_write",
		Arguments: map[string]any{"entities": []map[string]any{{
			"__domain__":      "apm",
			"__entity_type__": "apm.service",
			"__entity_id__":   "54013ba69c196820e56801f1ef5aad54",
		}}},
	})
	if err != nil {
		t.Fatalf("entity_write: %v", err)
	}
	if entitySvc.writeEntityCalls != 1 {
		t.Fatalf("entity_write should call EntityStore service, got %d", entitySvc.writeEntityCalls)
	}

	_, err = svc.ExecuteTool(context.Background(), "demo", model.AgentToolCallRequest{
		Name:      "entity_expire",
		Arguments: map[string]any{"kind": "relation", "ids": []string{"apm/apm.service/54013ba69c196820e56801f1ef5aad54/calls/apm/apm.service/177627f91af678a9b03e993f1a91917f"}},
	})
	if err != nil {
		t.Fatalf("entity_expire: %v", err)
	}
	if entitySvc.expireRelationCalls != 1 {
		t.Fatalf("relation expire should call EntityStore service, got %d", entitySvc.expireRelationCalls)
	}
}

type fakeQuery struct{}

func (fakeQuery) Execute(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryResult, error) {
	return model.QueryResult{Columns: []string{"ok"}, Rows: []map[string]any{{"ok": true}}}, nil
}

func (fakeQuery) Explain(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryExplain, error) {
	return model.QueryExplain{Source: ".umodel"}, nil
}

func (fakeQuery) Examples(ctx context.Context) ([]string, error) {
	return []string{".umodel | limit 1"}, nil
}

type recordingQuery struct {
	fakeQuery
	explainCalls  int
	lastWorkspace string
	lastQuery     string
}

func (f *recordingQuery) Explain(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryExplain, error) {
	f.explainCalls++
	f.lastWorkspace = workspace
	f.lastQuery = req.Query
	return model.QueryExplain{Source: ".entity", Provider: "memory"}, nil
}

type fakeUModel struct {
	validateCalls int
	putCalls      int
}

func (f *fakeUModel) Validate(ctx context.Context, workspace string, elements []model.UModelElement) (model.ValidationResult, error) {
	f.validateCalls++
	return model.ValidationResult{Valid: true}, nil
}

func (f *fakeUModel) PutElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error) {
	f.putCalls++
	return model.WriteResult{Accepted: len(batch.Elements)}, nil
}

type fakeEntityStore struct {
	writeEntityCalls    int
	writeRelationCalls  int
	expireEntityCalls   int
	expireRelationCalls int
}

func (f *fakeEntityStore) WriteEntities(ctx context.Context, workspace string, batch model.EntityWriteBatch) (model.WriteResult, error) {
	f.writeEntityCalls++
	return model.WriteResult{Accepted: len(batch.Entities)}, nil
}

func (f *fakeEntityStore) WriteRelations(ctx context.Context, workspace string, batch model.RelationWriteBatch) (model.WriteResult, error) {
	f.writeRelationCalls++
	return model.WriteResult{Accepted: len(batch.Relations)}, nil
}

func (f *fakeEntityStore) ExpireEntities(ctx context.Context, workspace string, req model.ExpireRequest) (model.WriteResult, error) {
	f.expireEntityCalls++
	return model.WriteResult{Accepted: len(req.IDs)}, nil
}

func (f *fakeEntityStore) ExpireRelations(ctx context.Context, workspace string, req model.ExpireRequest) (model.WriteResult, error) {
	f.expireRelationCalls++
	return model.WriteResult{Accepted: len(req.IDs)}, nil
}
