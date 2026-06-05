package query

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestExecuteRequiresUnifiedSPLSource(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{Query: "select * from entity"})
	if !apperrors.IsCode(err, apperrors.CodeQueryParseError) {
		t.Fatalf("expected query parse error, got %v", err)
	}
}

func TestExecuteUModelQuery(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements: []model.UModelElement{{
			Kind:   "entity_set",
			Domain: "apm",
			Name:   "service",
			Spec: map[string]any{
				"fields": []any{map[string]any{"name": "service_id", "display_name": map[string]any{"en_us": "Service ID"}}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".umodel with(kind='entity_set') | limit 20"})
	if err != nil {
		t.Fatalf("execute query: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected one row, got %d", len(result.Rows))
	}
	if result.Explain == nil || result.Explain.Source != ".umodel" {
		t.Fatalf("missing explain: %+v", result.Explain)
	}

	searchResult, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".umodel with(query='Service ID') | limit 20"})
	if err != nil {
		t.Fatalf("execute spec search: %v", err)
	}
	if len(searchResult.Rows) != 1 {
		t.Fatalf("expected spec query to match one row, got %+v", searchResult.Rows)
	}
}

func TestExecuteUModelPipeline(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements: []model.UModelElement{
			{Kind: "entity_set", Domain: "apm", Name: "operation"},
			{Kind: "entity_set", Domain: "apm", Name: "service"},
			{Kind: "entity_set", Domain: "k8s", Name: "pod"},
		},
	})
	if err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".umodel with(kind='entity_set') | where domain = 'apm' | project domain,name,kind | sort name desc | limit 1"})
	if err != nil {
		t.Fatalf("execute query: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0]["name"] != "service" {
		t.Fatalf("unexpected rows: %+v", result.Rows)
	}
	if len(result.Columns) != 3 || result.Columns[0] != "domain" || result.Columns[1] != "name" || result.Columns[2] != "kind" {
		t.Fatalf("unexpected columns: %+v", result.Columns)
	}
	if result.Explain == nil || !containsString(result.Explain.Fallback, "application_sort") {
		t.Fatalf("unexpected explain: %+v", result.Explain)
	}
}

func TestExecuteEntityQueryUsesGraphStoreRows(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities: []model.EntityPayload{{
			"__domain__":              "apm",
			"__entity_type__":         "apm.service",
			"__entity_id__":           "54013ba69c196820e56801f1ef5aad54",
			"__method__":              "Update",
			"__first_observed_time__": int64(100),
			"__last_observed_time__":  int64(200),
			"display_name":            "cart",
		}},
	})
	if err != nil {
		t.Fatalf("write entity: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity with(domain='apm', name='apm.service', query='cart') | limit 20"})
	if err != nil {
		t.Fatalf("execute entity query: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0]["__entity_id__"] != "54013ba69c196820e56801f1ef5aad54" {
		t.Fatalf("unexpected rows: %+v", result.Rows)
	}
	if result.Explain == nil || result.Explain.StorageProvider != "memory" {
		t.Fatalf("unexpected explain: %+v", result.Explain)
	}

	paramResult, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity with(domain='apm', name=$name, query=$query) | limit 20",
		Params: map[string]any{
			"name":  "apm.service",
			"query": "cart",
		},
	})
	if err != nil {
		t.Fatalf("execute parameterized entity query: %v", err)
	}
	if len(paramResult.Rows) != 1 || paramResult.Rows[0]["__entity_id__"] != "54013ba69c196820e56801f1ef5aad54" {
		t.Fatalf("unexpected parameterized entity rows: %+v", paramResult.Rows)
	}
}

func TestExecuteEntitySetListMethodsReturnsAssistantRawData(t *testing.T) {
	ctx := context.Background()
	svc := NewService(graphstore.NewMemoryStore())
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity_set with(domain='apm', name='apm.service') | entity-call __list_method__()"})
	if err != nil {
		t.Fatalf("execute __list_method__: %v", err)
	}
	row := result.Rows[0]
	if row["responseType"] != 2 || row["query"] != "" {
		t.Fatalf("expected assistant raw data response, got %+v", row)
	}
	header, ok := row["header"].([]string)
	if !ok || !containsString(header, "name") || !containsString(header, "params") || !containsString(header, "returns") {
		t.Fatalf("unexpected __list_method__ header: %#v", row["header"])
	}
	data, ok := row["data"].([]map[string]any)
	if !ok || len(data) != 3 {
		t.Fatalf("unexpected __list_method__ data: %#v", row["data"])
	}
	values, ok := data[1]["values"].([]string)
	if !ok || len(values) == 0 || values[0] != "list_data_set" {
		t.Fatalf("expected assistant canonical list_data_set method row, got %#v", data[1])
	}
	values, ok = data[2]["values"].([]string)
	if !ok || len(values) == 0 || values[0] != "get_logs" {
		t.Fatalf("expected get_logs method row, got %#v", data[2])
	}
}

func TestExecuteEntitySetListDataSetAliasReturnsAssistantRawData(t *testing.T) {
	ctx := context.Background()
	svc := NewService(graphstore.NewMemoryStore())
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity_set with(domain='apm', name='apm.service') | entity-call list_dataset(['metric_set'], true)"})
	if err != nil {
		t.Fatalf("execute list_dataset alias: %v", err)
	}
	row := result.Rows[0]
	if row["responseType"] != 2 || row["query"] != "" {
		t.Fatalf("expected assistant raw data response, got %+v", row)
	}
	header, ok := row["header"].([]string)
	if !ok || !containsString(header, "data_set_id") || !containsString(header, "storage_link_detail") {
		t.Fatalf("unexpected list_data_set header: %#v", row["header"])
	}
	data, ok := row["data"].([]map[string]any)
	if !ok || len(data) != 0 {
		t.Fatalf("memory quickstart has no data links; expected empty data, got %#v", row["data"])
	}
	if result.Explain == nil || result.Explain.EntityCall == nil || result.Explain.EntityCall.Name != "list_data_set" {
		t.Fatalf("expected canonical list_data_set in explain, got %+v", result.Explain)
	}
}

func TestExecuteEntitySetGetLogsReturnsQueryPlan(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements:  logQueryPlanElements(),
	})
	if err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service', ids=['svc-1'], query='environment = \"prod\"') | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\" and service_id in [\"svc-1\", \"svc-2\"]')",
	})
	if err != nil {
		t.Fatalf("execute get_logs: %v", err)
	}
	row := result.Rows[0]
	if row["responseType"] != 1 {
		t.Fatalf("expected responseType=1 query plan, got %+v", row)
	}
	queryText, ok := row["query"].(string)
	if !ok || queryText == "" {
		t.Fatalf("expected query string, got %#v", row["query"])
	}
	for _, want := range []string{"get_logs", "devops.log.service", "devops.elasticsearch.logs", "elasticsearch_dsl", "devops-service-logs-*", "svc_id", "svc-1", "severity", "ERROR", "env", "prod"} {
		if !strings.Contains(queryText, want) {
			t.Fatalf("expected get_logs query plan to contain %q, got %s", want, queryText)
		}
	}
	var queryPlan map[string]any
	if err := json.Unmarshal([]byte(queryText), &queryPlan); err != nil {
		t.Fatalf("decode query plan: %v", err)
	}
	query, ok := queryPlan["query"].(map[string]any)
	if !ok {
		t.Fatalf("expected query object, got %#v", queryPlan["query"])
	}
	body, ok := query["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected elasticsearch body, got %#v", query["body"])
	}
	bodyQuery := mustJSON(body["query"])
	if strings.Contains(bodyQuery, "query_string") {
		t.Fatalf("expected translated DSL filters, got %s", bodyQuery)
	}
	for _, want := range []string{"svc_id", "severity", "env", "terms", "term"} {
		if !strings.Contains(bodyQuery, want) {
			t.Fatalf("expected translated DSL to contain %q, got %s", want, bodyQuery)
		}
	}
	if result.Explain == nil || result.Explain.EntityCall == nil || result.Explain.EntityCall.Name != "get_logs" {
		t.Fatalf("expected canonical get_logs in explain, got %+v", result.Explain)
	}
}

func TestExecuteEntitySetGetLogAliasReturnsQueryPlan(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements:  logQueryPlanElements(),
	})
	if err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service') | entity-call get_log(domain='devops', name='devops.log.service')",
	})
	if err != nil {
		t.Fatalf("execute get_log alias: %v", err)
	}
	if result.Rows[0]["responseType"] != 1 {
		t.Fatalf("expected responseType=1 alias query plan, got %+v", result.Rows[0])
	}
	if result.Explain == nil || result.Explain.EntityCall == nil || result.Explain.EntityCall.Name != "get_logs" {
		t.Fatalf("expected get_log alias to normalize to get_logs, got %+v", result.Explain)
	}
}

func TestExecuteEntitySetRejectsPlaceholderMethod(t *testing.T) {
	ctx := context.Background()
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity_set with(domain='apm', name='apm.service') | entity-call METHOD()"})
	if !apperrors.IsCode(err, apperrors.CodeQueryPlanError) {
		t.Fatalf("expected query plan error for placeholder method, got %v", err)
	}
}

func TestExecuteEntitySetRejectsUnsupportedMethod(t *testing.T) {
	ctx := context.Background()
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity_set with(domain='apm', name='apm.service') | entity-call get_metric('apm', 'devops.metric.service', 'request_count')"})
	if !apperrors.IsCode(err, apperrors.CodeQueryPlanError) {
		t.Fatalf("expected query plan error for unsupported method, got %v", err)
	}
}

func TestExecuteEntityTopKAndProject(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities: []model.EntityPayload{
			entityPayload("54013ba69c196820e56801f1ef5aad54", "cart service"),
			entityPayload("177627f91af678a9b03e993f1a91917f", "checkout service"),
			entityPayload("f83c2a85d972a89238f31296c63f0dbc", "payment service"),
		},
	})
	if err != nil {
		t.Fatalf("write entities: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity with(domain='apm', name='apm.service', query='service', topk=2) | project __entity_id__"})
	if err != nil {
		t.Fatalf("execute entity query: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected two rows, got %+v", result.Rows)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "__entity_id__" {
		t.Fatalf("unexpected columns: %+v", result.Columns)
	}
	if result.Page.Limit != 2 || result.Explain == nil || result.Explain.Limit != 2 {
		t.Fatalf("unexpected limit/explain: page=%+v explain=%+v", result.Page, result.Explain)
	}
}

func TestExecuteTopoDirectRelations(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{{
			"__src_domain__":          "apm",
			"__src_entity_type__":     "apm.service",
			"__src_entity_id__":       "54013ba69c196820e56801f1ef5aad54",
			"__dest_domain__":         "apm",
			"__dest_entity_type__":    "apm.service",
			"__dest_entity_id__":      "177627f91af678a9b03e993f1a91917f",
			"__relation_type__":       "calls",
			"__method__":              "Update",
			"__first_observed_time__": int64(100),
			"__last_observed_time__":  int64(200),
		}},
	})
	if err != nil {
		t.Fatalf("write relation: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".topo | graph-call getDirectRelations([(:\"apm@apm.service\" {__entity_id__: '54013ba69c196820e56801f1ef5aad54'})]) | project src,relation,dest | limit 10"})
	if err != nil {
		t.Fatalf("execute topo query: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0]["relation"] != "calls" {
		t.Fatalf("unexpected rows: %+v", result.Rows)
	}
	if result.Explain == nil || result.Explain.Depth != 1 || !containsString(result.Explain.Pushdown, "graph_call:getDirectRelations") {
		t.Fatalf("unexpected explain: %+v", result.Explain)
	}
}

func TestExecuteTopoCypherOnMemory(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{{
			"__src_domain__":          "apm",
			"__src_entity_type__":     "apm.service",
			"__src_entity_id__":       "54013ba69c196820e56801f1ef5aad54",
			"__dest_domain__":         "apm",
			"__dest_entity_type__":    "apm.service",
			"__dest_entity_id__":      "177627f91af678a9b03e993f1a91917f",
			"__relation_type__":       "calls",
			"__method__":              "Update",
			"__first_observed_time__": int64(100),
			"__last_observed_time__":  int64(200),
		}},
	})
	if err != nil {
		t.Fatalf("write relation: %v", err)
	}

	svc := NewService(store)
	query := ".topo | graph-call cypher(`match (svc:``apm@apm.service`` {__entity_id__: $svc}) optional match path = (svc)-[r*1..2]-(neighbor) with svc, neighbor, relationships(path) as rels where neighbor is null or coalesce(neighbor.__deleted__, false) = false return svc.__entity_id__ as service, neighbor.__entity_id__ as neighbor, [rel in rels | type(rel)] as relation_types, size(rels) as hops order by hops, neighbor limit 20`) | limit 20"
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: query, Params: map[string]any{"svc": "54013ba69c196820e56801f1ef5aad54"}})
	if err != nil {
		t.Fatalf("execute cypher topo query: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected one cypher row, got %+v", result.Rows)
	}
	row := result.Rows[0]
	if row["service"] != "54013ba69c196820e56801f1ef5aad54" || row["neighbor"] != "177627f91af678a9b03e993f1a91917f" || row["hops"] != 1 {
		t.Fatalf("unexpected cypher row: %+v", row)
	}
	if types, ok := row["relation_types"].([]string); !ok || len(types) != 1 || types[0] != "calls" {
		t.Fatalf("unexpected relation types: %#v", row["relation_types"])
	}
	if result.Explain == nil || !containsString(result.Explain.Pushdown, "graph_call:cypher") || !containsString(result.Explain.Pushdown, "controlled_cypher") {
		t.Fatalf("unexpected cypher explain: %+v", result.Explain)
	}
	if result.Explain.CypherDialect != "ladybug" || result.Explain.CypherEngine != "go" {
		t.Fatalf("unexpected cypher explain metadata: %+v", result.Explain)
	}
}

func TestExecuteTopoCypherReturnsFullEntityAndRelationProperties(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	_, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities: []model.EntityPayload{
			{
				"__domain__":              "apm",
				"__entity_type__":         "apm.service",
				"__entity_id__":           "54013ba69c196820e56801f1ef5aad54",
				"__method__":              "Update",
				"__first_observed_time__": int64(100),
				"__last_observed_time__":  int64(200),
				"display_name":            "cart service",
				"owner":                   "checkout-team",
			},
			{
				"__domain__":              "apm",
				"__entity_type__":         "apm.service",
				"__entity_id__":           "177627f91af678a9b03e993f1a91917f",
				"__method__":              "Update",
				"__first_observed_time__": int64(100),
				"__last_observed_time__":  int64(200),
				"display_name":            "checkout service",
				"tier":                    "gold",
			},
		},
	})
	if err != nil {
		t.Fatalf("write entities: %v", err)
	}
	_, err = store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{{
			"__src_domain__":          "apm",
			"__src_entity_type__":     "apm.service",
			"__src_entity_id__":       "54013ba69c196820e56801f1ef5aad54",
			"__dest_domain__":         "apm",
			"__dest_entity_type__":    "apm.service",
			"__dest_entity_id__":      "177627f91af678a9b03e993f1a91917f",
			"__relation_type__":       "calls",
			"__method__":              "Update",
			"__first_observed_time__": int64(100),
			"__last_observed_time__":  int64(200),
			"latency_ms":              int64(12),
			"criticality":             "high",
		}},
	})
	if err != nil {
		t.Fatalf("write relation: %v", err)
	}

	svc := NewService(store)
	query := ".topo | graph-call cypher(`match (src:``apm@apm.service`` {__entity_id__: $src})-[r:calls]->(dest) return properties(src) as src, properties(r) as relation, properties(dest) as dest`) | limit 20"
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: query, Params: map[string]any{"src": "54013ba69c196820e56801f1ef5aad54"}})
	if err != nil {
		t.Fatalf("execute full property cypher query: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected one cypher row, got %+v", result.Rows)
	}
	row := result.Rows[0]
	src, ok := row["src"].(map[string]any)
	if !ok || src["display_name"] != "cart service" || src["owner"] != "checkout-team" {
		t.Fatalf("unexpected source properties: %#v", row["src"])
	}
	relation, ok := row["relation"].(map[string]any)
	if !ok || relation["latency_ms"] != int64(12) || relation["criticality"] != "high" {
		t.Fatalf("unexpected relation properties: %#v", row["relation"])
	}
	dest, ok := row["dest"].(map[string]any)
	if !ok || dest["display_name"] != "checkout service" || dest["tier"] != "gold" {
		t.Fatalf("unexpected destination properties: %#v", row["dest"])
	}
}

func TestExecuteTopoCypherRejectsMutationsOnMemory(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{Query: ".topo | graph-call cypher(`MATCH (n) SET n.name = 'bad' RETURN n`)"})
	if !apperrors.IsCode(err, apperrors.CodeQueryPlanError) {
		t.Fatalf("expected read-only cypher rejection, got %v", err)
	}
}

func entityPayload(id, displayName string) model.EntityPayload {
	return model.EntityPayload{
		"__domain__":              "apm",
		"__entity_type__":         "apm.service",
		"__entity_id__":           id,
		"__method__":              "Update",
		"__first_observed_time__": int64(100),
		"__last_observed_time__":  int64(200),
		"display_name":            displayName,
	}
}

func logQueryPlanElements() []model.UModelElement {
	return []model.UModelElement{
		{Kind: "entity_set", Domain: "devops", Name: "devops.service"},
		{
			Kind:   "data_link",
			Domain: "devops",
			Name:   "devops.service_related_to_devops.log.service",
			Spec: map[string]any{
				"src":            map[string]any{"domain": "devops", "kind": "entity_set", "name": "devops.service"},
				"dest":           map[string]any{"domain": "devops", "kind": "log_set", "name": "devops.log.service"},
				"fields_mapping": map[string]any{"id": "service_id", "environment": "environment"},
			},
		},
		{
			Kind:   "log_set",
			Domain: "devops",
			Name:   "devops.log.service",
			Spec: map[string]any{
				"time_field":    "timestamp",
				"default_order": "desc",
				"fields": []any{
					map[string]any{"name": "timestamp", "type": "time"},
					map[string]any{"name": "service_id", "type": "string"},
					map[string]any{"name": "level", "type": "string"},
					map[string]any{"name": "message", "type": "string"},
				},
			},
		},
		{
			Kind:   "storage_link",
			Domain: "devops",
			Name:   "devops.log.service_to_elasticsearch",
			Spec: map[string]any{
				"src":            map[string]any{"domain": "devops", "kind": "log_set", "name": "devops.log.service"},
				"dest":           map[string]any{"domain": "devops", "kind": "elasticsearch", "name": "devops.elasticsearch.logs"},
				"fields_mapping": map[string]any{"service_id": "svc_id", "environment": "env", "level": "severity", "message": "log_message", "timestamp": "event_time"},
			},
		},
		{
			Kind:   "elasticsearch",
			Domain: "devops",
			Name:   "devops.elasticsearch.logs",
			Spec: map[string]any{
				"endpoint":      "https://elasticsearch.devops.example:9200",
				"index":         "devops-service-logs-*",
				"query_dialect": "elasticsearch_dsl",
				"time_field":    "event_time",
				"default_size":  1000,
			},
		},
	}
}
