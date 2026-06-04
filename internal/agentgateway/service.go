package agentgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type queryService interface {
	Execute(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryResult, error)
	Explain(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryExplain, error)
	Examples(ctx context.Context) ([]string, error)
}

type umodelService interface {
	Validate(ctx context.Context, workspace string, elements []model.UModelElement) (model.ValidationResult, error)
	PutElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error)
}

type entityWriteService interface {
	WriteEntities(ctx context.Context, workspace string, batch model.EntityWriteBatch) (model.WriteResult, error)
	WriteRelations(ctx context.Context, workspace string, batch model.RelationWriteBatch) (model.WriteResult, error)
	ExpireEntities(ctx context.Context, workspace string, req model.ExpireRequest) (model.WriteResult, error)
	ExpireRelations(ctx context.Context, workspace string, req model.ExpireRequest) (model.WriteResult, error)
}

type Service struct {
	query        queryService
	umodel       umodelService
	entity       entityWriteService
	writeEnabled bool
	tools        map[string]model.AgentTool
}

type Option func(*Service)

func WithWriteToolsEnabled(enabled bool) Option {
	return func(s *Service) {
		s.writeEnabled = enabled
	}
}

func WithWriteServices(umodel umodelService, entity entityWriteService) Option {
	return func(s *Service) {
		s.umodel = umodel
		s.entity = entity
	}
}

func NewService(query queryService, opts ...Option) *Service {
	svc := &Service{query: query}
	for _, opt := range opts {
		opt(svc)
	}
	svc.tools = svc.defaultTools()
	return svc
}

func (s *Service) Discover(ctx context.Context, workspace string) (model.AgentDiscovery, error) {
	examples, err := s.examples(ctx)
	if err != nil {
		return model.AgentDiscovery{}, err
	}
	return model.AgentDiscovery{
		Workspace:   workspace,
		Tools:       s.toolList(),
		Resources:   resourceCatalog(workspace),
		NextActions: nextActions(workspace, examples),
	}, nil
}

func (s *Service) Tools(ctx context.Context) ([]model.AgentTool, error) {
	return s.toolList(), nil
}

func (s *Service) ReadResource(ctx context.Context, workspace string, req model.AgentResourceReadRequest) (model.AgentResourceReadResult, error) {
	resource, ok := findResource(workspace, req.URI)
	if !ok {
		return model.AgentResourceReadResult{}, apperrors.New(apperrors.CodeNotFound, "agent resource not found")
	}

	var content any
	switch resource.Kind {
	case "overview":
		content = map[string]any{
			"workspace": workspace,
			"purpose":   "UModel local agent metadata for discovery and safe query entry points.",
			"read_model": map[string]any{
				"entrypoint": "/api/v1/query/" + workspace + "/execute",
				"tool":       "query_spl_execute",
				"sources":    []string{".umodel", ".entity_set", ".entity", ".topo", ".runbook_set"},
			},
			"resource_policy": "Resources expose metadata and templates only; runtime rows are returned through Query API calls.",
		}
	case "schema-index":
		content = map[string]any{
			"workspace": workspace,
			"sources": []map[string]any{
				{"source": ".umodel", "description": "UModel snapshot metadata", "query_api": "/api/v1/query/" + workspace + "/execute"},
				{"source": ".entity_set", "description": "EntitySet method call planning through the Query Service", "query_api": "/api/v1/query/" + workspace + "/execute"},
				{"source": ".entity", "description": "CMS 2.0 entity reads through the Query Service", "query_api": "/api/v1/query/" + workspace + "/execute"},
				{"source": ".topo", "description": "Topology reads through the Query Service", "query_api": "/api/v1/query/" + workspace + "/execute"},
			},
			"filters": []string{"with(...)", "limit", "time_range", "parameters"},
		}
	case "query-templates":
		content = map[string]any{
			"workspace": workspace,
			"templates": []map[string]any{
				{"id": "list-umodel", "query": ".umodel with(kind='entity_set') | limit 20"},
				{"id": "entity-set-methods", "query": ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()"},
				{"id": "entity-set-data-sets", "query": ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)"},
				{"id": "find-entity", "query": ".entity with(domain='devops', name='devops.service', query=$query) | limit 20", "parameters": map[string]any{"query": "checkout"}},
				{"id": "topology-neighbors", "query": ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20"},
				{"id": "topology-cypher", "query": ".topo | graph-call cypher(`MATCH (src)-[r]->(dest) RETURN properties(src) AS src, properties(r) AS relation, properties(dest) AS dest LIMIT 20`)"},
			},
		}
	case "tool-metadata":
		content = map[string]any{
			"workspace": workspace,
			"tools":     s.toolList(),
			"schemas":   toolSchemas(),
		}
	default:
		return model.AgentResourceReadResult{}, apperrors.New(apperrors.CodeNotFound, "agent resource not found")
	}
	return model.AgentResourceReadResult{URI: resource.URI, MIMEType: resource.MIMEType, Content: content}, nil
}

func (s *Service) ExecuteTool(ctx context.Context, workspace string, req model.AgentToolCallRequest) (model.AgentToolCallResult, error) {
	tool, ok := s.tools[req.Name]
	if !ok {
		return model.AgentToolCallResult{}, apperrors.New(apperrors.CodeToolNotFound, "agent tool not found")
	}
	if !tool.Enabled {
		return model.AgentToolCallResult{}, apperrors.New(apperrors.CodeToolDisabled, "agent tool is disabled")
	}

	switch req.Name {
	case "query_spl_execute":
		queryReq, err := s.queryRequest(req.Arguments)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		result, err := s.query.Execute(ctx, workspace, queryReq)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		return model.AgentToolCallResult{Name: req.Name, OK: true, Output: result}, nil
	case "query_spl_explain":
		queryReq, err := s.queryRequest(req.Arguments)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		result, err := s.query.Explain(ctx, workspace, queryReq)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		return model.AgentToolCallResult{Name: req.Name, OK: true, Output: result}, nil
	case "query_spl_examples":
		result, err := s.examples(ctx)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		return model.AgentToolCallResult{Name: req.Name, OK: true, Output: result}, nil
	case "umodel_validate":
		elements, err := elementsArg(req.Arguments)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		result, err := s.validateUModel(ctx, workspace, elements)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		return model.AgentToolCallResult{Name: req.Name, OK: true, Output: result}, nil
	case "umodel_import":
		if s.umodel == nil {
			return model.AgentToolCallResult{}, apperrors.New(apperrors.CodeProviderUnavailable, "umodel service is not configured")
		}
		elements, err := elementsArg(req.Arguments)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		result, err := s.umodel.PutElements(ctx, model.UModelElementBatch{Workspace: workspace, Elements: elements})
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		return model.AgentToolCallResult{Name: req.Name, OK: true, Output: result}, nil
	case "entity_write":
		if s.entity == nil {
			return model.AgentToolCallResult{}, apperrors.New(apperrors.CodeProviderUnavailable, "entity store service is not configured")
		}
		result, err := s.writeEntitiesAndRelations(ctx, workspace, req.Arguments)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		return model.AgentToolCallResult{Name: req.Name, OK: true, Output: result}, nil
	case "entity_expire":
		if s.entity == nil {
			return model.AgentToolCallResult{}, apperrors.New(apperrors.CodeProviderUnavailable, "entity store service is not configured")
		}
		result, err := s.expireEntityStore(ctx, workspace, req.Arguments)
		if err != nil {
			return model.AgentToolCallResult{}, err
		}
		return model.AgentToolCallResult{Name: req.Name, OK: true, Output: result}, nil
	default:
		return model.AgentToolCallResult{}, apperrors.New(apperrors.CodeToolDisabled, "write tools require explicit enablement and adapter wiring")
	}
}

func (s *Service) defaultTools() map[string]model.AgentTool {
	tools := map[string]model.AgentTool{}
	schemas := toolSchemas()
	for _, tool := range []model.AgentTool{
		{Name: "query_spl_execute", Description: "Execute unified SPL query", Enabled: true},
		{Name: "query_spl_explain", Description: "Explain unified SPL query", Enabled: true},
		{Name: "query_spl_examples", Description: "List safe SPL examples", Enabled: true},
		{Name: "umodel_validate", Description: "Validate UModel elements", Enabled: true},
		{Name: "umodel_import", Description: "Import UModel package", Enabled: s.writeEnabled, RequiresExplicitWriteEnable: true},
		{Name: "entity_write", Description: "Write CMS 2.0 compatible entities", Enabled: s.writeEnabled, RequiresExplicitWriteEnable: true},
		{Name: "entity_expire", Description: "Expire entities", Enabled: s.writeEnabled, RequiresExplicitWriteEnable: true},
	} {
		if schema, ok := schemas[tool.Name].(map[string]any); ok {
			tool.InputSchema = schema["input_schema"]
			tool.OutputSchema = schema["output_schema"]
		}
		tools[tool.Name] = tool
	}
	return tools
}

func (s *Service) toolList() []model.AgentTool {
	order := []string{"query_spl_execute", "query_spl_explain", "query_spl_examples", "umodel_validate", "umodel_import", "entity_write", "entity_expire"}
	tools := make([]model.AgentTool, 0, len(order))
	for _, name := range order {
		if tool, ok := s.tools[name]; ok {
			tools = append(tools, tool)
		}
	}
	return tools
}

func (s *Service) examples(ctx context.Context) ([]string, error) {
	if s.query == nil {
		return defaultExamples(), nil
	}
	return s.query.Examples(ctx)
}

func (s *Service) queryRequest(args map[string]any) (model.QueryRequest, error) {
	if s.query == nil {
		return model.QueryRequest{}, apperrors.New(apperrors.CodeProviderUnavailable, "query service is not configured")
	}
	req := model.QueryRequest{
		Query:     strings.TrimSpace(stringArg(args, "query")),
		Format:    stringArg(args, "format"),
		Limit:     intArg(args, "limit"),
		TimeoutMS: intArg(args, "timeout_ms"),
	}
	if req.Query == "" {
		return model.QueryRequest{}, apperrors.New(apperrors.CodeInvalidArgument, "query argument is required")
	}
	if params, ok := args["parameters"].(map[string]any); ok {
		req.Params = params
	}
	if raw, ok := args["time_range"]; ok {
		if err := decodeValue(raw, &req.TimeRange); err != nil {
			return model.QueryRequest{}, apperrors.New(apperrors.CodeInvalidArgument, "time_range argument is invalid")
		}
	}
	return req, nil
}

func (s *Service) validateUModel(ctx context.Context, workspace string, elements []model.UModelElement) (model.ValidationResult, error) {
	if s.umodel != nil {
		return s.umodel.Validate(ctx, workspace, elements)
	}
	for _, element := range elements {
		if element.Kind == "" || element.Domain == "" || element.Name == "" {
			return model.ValidationResult{Valid: false, Errors: []model.ErrorDetail{{
				Field:  "kind/domain/name",
				Reason: "umodel element kind, domain, and name are required",
			}}}, nil
		}
	}
	return model.ValidationResult{Valid: true}, nil
}

func (s *Service) writeEntitiesAndRelations(ctx context.Context, workspace string, args map[string]any) (map[string]model.WriteResult, error) {
	entities, err := entityPayloadsArg(args, "entities")
	if err != nil {
		return nil, err
	}
	relations, err := relationPayloadsArg(args, "relations")
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 && len(relations) == 0 {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "entities or relations argument is required")
	}

	out := map[string]model.WriteResult{}
	if len(entities) > 0 {
		result, err := s.entity.WriteEntities(ctx, workspace, model.EntityWriteBatch{
			IdempotencyKey: stringArg(args, "idempotency_key"),
			Entities:       entities,
		})
		if err != nil {
			return nil, err
		}
		out["entities"] = result
	}
	if len(relations) > 0 {
		result, err := s.entity.WriteRelations(ctx, workspace, model.RelationWriteBatch{
			IdempotencyKey: stringArg(args, "idempotency_key"),
			Relations:      relations,
		})
		if err != nil {
			return nil, err
		}
		out["relations"] = result
	}
	return out, nil
}

func (s *Service) expireEntityStore(ctx context.Context, workspace string, args map[string]any) (model.WriteResult, error) {
	var ids []string
	if err := decodeArgument(args, "ids", &ids); err != nil {
		return model.WriteResult{}, err
	}
	if len(ids) == 0 {
		return model.WriteResult{}, apperrors.New(apperrors.CodeInvalidArgument, "ids argument is required")
	}
	req := model.ExpireRequest{IDs: ids, Reason: stringArg(args, "reason")}
	switch strings.ToLower(stringArg(args, "kind")) {
	case "relation", "relations", "topo", "topology":
		return s.entity.ExpireRelations(ctx, workspace, req)
	default:
		return s.entity.ExpireEntities(ctx, workspace, req)
	}
}

func resourceCatalog(workspace string) []model.AgentResource {
	return []model.AgentResource{
		{
			URI:         resourceURI(workspace, "overview"),
			Name:        "overview",
			Kind:        "overview",
			Description: "Workspace agent overview and safe query entry points.",
			MIMEType:    "application/json",
			ReadOnly:    true,
		},
		{
			URI:         resourceURI(workspace, "schema-index"),
			Name:        "schema-index",
			Kind:        "schema-index",
			Description: "Schema index summary for query planning; no runtime rows.",
			MIMEType:    "application/json",
			ReadOnly:    true,
		},
		{
			URI:         resourceURI(workspace, "query-templates"),
			Name:        "query-templates",
			Kind:        "query-templates",
			Description: "Reusable SPL templates that must be executed through Query API.",
			MIMEType:    "application/json",
			ReadOnly:    true,
		},
		{
			URI:         resourceURI(workspace, "tool-capability-metadata"),
			Name:        "tool-capability-metadata",
			Kind:        "tool-metadata",
			Description: "Tool enablement and input/output schema metadata.",
			MIMEType:    "application/json",
			ReadOnly:    true,
		},
	}
}

func findResource(workspace, uri string) (model.AgentResource, bool) {
	for _, resource := range resourceCatalog(workspace) {
		if resource.URI == uri || resource.Name == uri {
			return resource, true
		}
	}
	return model.AgentResource{}, false
}

func resourceURI(workspace, name string) string {
	return "umodel://workspace/" + workspace + "/" + name
}

func nextActions(workspace string, examples []string) []model.AgentNextAction {
	actions := make([]model.AgentNextAction, 0, len(examples))
	for index, example := range examples {
		actions = append(actions, model.AgentNextAction{
			ID:          fmt.Sprintf("query-example-%d", index+1),
			Title:       fmt.Sprintf("Run query example %d", index+1),
			Description: "Execute this SPL through the Query API; resources never embed runtime results.",
			Tool:        "query_spl_execute",
			QueryAPI: model.AgentQueryAction{
				Method: http.MethodPost,
				Path:   "/api/v1/query/" + workspace + "/execute",
				Body:   model.QueryRequest{Query: example, Limit: 20},
			},
		})
	}
	return actions
}

func defaultExamples() []string {
	return []string{
		".umodel with(kind='entity_set') | limit 20",
		".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()",
		".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)",
		".entity with(domain='devops', name='devops.service', query='checkout') | limit 20",
		".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20",
		".topo | graph-call cypher(`MATCH (src)-[r]->(dest) RETURN properties(src) AS src, properties(r) AS relation, properties(dest) AS dest LIMIT 20`)",
	}
}

func toolSchemas() map[string]any {
	queryInput := map[string]any{
		"type":     "object",
		"required": []string{"query"},
		"properties": map[string]any{
			"query":      map[string]any{"type": "string"},
			"limit":      map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
			"timeout_ms": map[string]any{"type": "integer", "minimum": 1},
			"parameters": map[string]any{"type": "object", "additionalProperties": true},
			"time_range": map[string]any{"type": "object"},
		},
	}
	return map[string]any{
		"query_spl_execute":  map[string]any{"input_schema": queryInput, "output_schema": map[string]any{"$ref": "QueryResult"}},
		"query_spl_explain":  map[string]any{"input_schema": queryInput, "output_schema": map[string]any{"$ref": "QueryExplain"}},
		"query_spl_examples": map[string]any{"input_schema": map[string]any{"type": "object"}, "output_schema": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}},
		"umodel_validate":    map[string]any{"input_schema": elementsInputSchema(), "output_schema": map[string]any{"$ref": "ValidationResult"}},
		"umodel_import":      map[string]any{"input_schema": elementsInputSchema(), "output_schema": map[string]any{"$ref": "WriteResult"}},
		"entity_write":       map[string]any{"input_schema": entityWriteInputSchema(), "output_schema": map[string]any{"$ref": "WriteResult"}},
		"entity_expire":      map[string]any{"input_schema": expireInputSchema(), "output_schema": map[string]any{"$ref": "WriteResult"}},
	}
}

func elementsInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"elements"},
		"properties": map[string]any{
			"elements": map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}},
		},
	}
}

func entityWriteInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"entities":        map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}},
			"relations":       map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}},
			"idempotency_key": map[string]any{"type": "string"},
		},
	}
}

func expireInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"ids"},
		"properties": map[string]any{
			"ids":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"kind":   map[string]any{"type": "string", "enum": []string{"entity", "relation"}},
			"reason": map[string]any{"type": "string"},
		},
	}
}

func elementsArg(args map[string]any) ([]model.UModelElement, error) {
	var elements []model.UModelElement
	if err := decodeArgument(args, "elements", &elements); err != nil {
		return nil, err
	}
	return elements, nil
}

func entityPayloadsArg(args map[string]any, key string) ([]model.EntityPayload, error) {
	if _, ok := args[key]; !ok {
		return nil, nil
	}
	var payloads []model.EntityPayload
	if err := decodeArgument(args, key, &payloads); err != nil {
		return nil, err
	}
	return payloads, nil
}

func relationPayloadsArg(args map[string]any, key string) ([]model.RelationPayload, error) {
	if _, ok := args[key]; !ok {
		return nil, nil
	}
	var payloads []model.RelationPayload
	if err := decodeArgument(args, key, &payloads); err != nil {
		return nil, err
	}
	return payloads, nil
}

func decodeArgument(args map[string]any, key string, target any) error {
	if args == nil {
		return apperrors.New(apperrors.CodeInvalidArgument, key+" argument is required")
	}
	raw, ok := args[key]
	if !ok {
		return apperrors.New(apperrors.CodeInvalidArgument, key+" argument is required")
	}
	if err := decodeValue(raw, target); err != nil {
		return apperrors.New(apperrors.CodeInvalidArgument, key+" argument is invalid")
	}
	return nil
}

func decodeValue(raw any, target any) error {
	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch value := args[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
}
