package query

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/alibaba/UnifiedModel/tests/graphfixture"
)

func lokiTestRequest(method planrender.Method) planrender.Request {
	from := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	return planrender.Request{
		Method:             method,
		Storage:            model.UModelElement{Kind: "loki", Spec: map[string]any{"endpoint": "http://localhost:3100", "default_size": 37, "max_size": 100}},
		DataSet:            model.UModelElement{Kind: "log_set", Spec: map[string]any{"default_order": "desc"}},
		DataLinkMapping:    map[string]any{"id": "node_id", "environment": "environment"},
		StorageLinkMapping: map[string]any{"node_id": "host", "environment": "env", "level": "severity"},
		EntityData: &model.EntityData{Header: []string{"id", "environment"}, Data: [][]string{
			{"node-01", "prod"}, {"node-02", "staging"},
		}},
		TimeRange: model.TimeRange{From: &from, To: &to},
	}
}

func TestLokiLogsKeepEntityRowsAndLimitPrecedence(t *testing.T) {
	req := lokiTestRequest(planrender.MethodGetLogs)
	req.MethodQuery = `level = "ERROR"`
	plan, err := (lokiRenderer{}).Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if plan["dialect"] != "loki_logql" || plan["size"] != 37 || plan["direction"] != "backward" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	queries := plan["queries"].([]map[string]any)
	if len(queries) != 2 {
		t.Fatalf("expected one query per entity row: %#v", queries)
	}
	first := queries[0]["logql"].(string)
	second := queries[1]["logql"].(string)
	for _, want := range []string{`host="node-01"`, `env="prod"`, `severity="ERROR"`} {
		if !strings.Contains(first, want) {
			t.Fatalf("first query %q lacks %s", first, want)
		}
	}
	if strings.Contains(first, "staging") || !strings.Contains(second, `host="node-02"`) || !strings.Contains(second, `env="staging"`) {
		t.Fatalf("entity attributes were crossed: %#v", queries)
	}
	req.LimitExplicit = true
	req.Limit = 7
	plan, err = (lokiRenderer{}).Render(req)
	if err != nil || plan["size"] != 7 {
		t.Fatalf("explicit limit was not used: plan=%#v err=%v", plan, err)
	}
	req.Limit = 101
	if _, err := (lokiRenderer{}).Render(req); err == nil {
		t.Fatal("expected max_size rejection")
	}
}

func TestLokiMetricsBindLogQLAndRange(t *testing.T) {
	req := lokiTestRequest(planrender.MethodGetMetrics)
	req.DataSet = model.UModelElement{Kind: "metric_set", Spec: map[string]any{"query_type": "logql"}}
	req.Metrics = []map[string]any{{"name": "error_rate", "query_mode": "range", "generator": `sum(rate({host="$host",env="$env"} |= "ERROR" [5m]))`}}
	req.Step = "30s"
	plan, err := (lokiRenderer{}).Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if plan["query_type"] != "range" || plan["step"] != "30s" {
		t.Fatalf("wrong range metadata: %#v", plan)
	}
	queries := plan["queries"].([]map[string]any)
	if len(queries) != 2 || !strings.Contains(queries[0]["logql"].(string), `host="node-01"`) || !strings.Contains(queries[1]["logql"].(string), `env="staging"`) {
		t.Fatalf("wrong metric queries: %#v", queries)
	}
	if strings.Contains(queries[0]["logql"].(string), "$host") {
		t.Fatal("unbound placeholder")
	}
}

func TestLokiRejectsUnsafeOrIncompletePlans(t *testing.T) {
	base := lokiTestRequest(planrender.MethodGetLogs)
	tests := map[string]func(*planrender.Request){
		"missing mapping":    func(r *planrender.Request) { delete(r.StorageLinkMapping, "environment") },
		"unsupported filter": func(r *planrender.Request) { r.MethodQuery = `level = "ERROR" or level = "WARN"` },
		"missing time":       func(r *planrender.Request) { r.TimeRange.From = nil },
		"invalid auth": func(r *planrender.Request) {
			r.Storage.Spec["auth"] = map[string]any{"type": "bearer", "token_env": "TOKEN", "password_env": "PASSWORD"}
		},
		"reserved header": func(r *planrender.Request) { r.Storage.Spec["headers"] = map[string]any{"Authorization": "secret"} },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			req := base
			req.Storage.Spec = map[string]any{"endpoint": "http://localhost:3100"}
			req.StorageLinkMapping = map[string]any{"node_id": "host", "environment": "env", "level": "severity"}
			change(&req)
			if _, err := (lokiRenderer{}).Render(req); err == nil {
				t.Fatal("expected plan error")
			}
		})
	}
}

func TestLokiStorageSelectionFailsOnMissingOrAmbiguousTarget(t *testing.T) {
	bindings := []storageBinding{
		{Storage: model.UModelElement{Kind: "elasticsearch", Domain: "ops", Name: "logs"}},
		{Storage: model.UModelElement{Kind: "loki", Domain: "ops", Name: "primary"}},
		{Storage: model.UModelElement{Kind: "loki", Domain: "ops", Name: "secondary"}},
	}
	selected, err := selectStorageBinding(bindings, map[string]any{"storage_name": "primary", "storage_kind": "loki"})
	if err != nil || selected.Storage.Name != "primary" {
		t.Fatalf("wrong selection: %#v, %v", selected, err)
	}
	for _, params := range []map[string]any{{"storage_kind": "loki"}, {"storage_name": "missing"}} {
		if _, err := selectStorageBinding(bindings, params); err == nil {
			t.Fatalf("expected selection error for %#v", params)
		}
	}
}

func TestLokiPlanThroughQueryServiceSelectsStorage(t *testing.T) {
	ctx := context.Background()
	store := graphfixture.New(t)
	elements := append(logQueryPlanElements(),
		model.UModelElement{Kind: "storage_link", Domain: "devops", Name: "devops.log.service_to_loki", Spec: map[string]any{
			"src":            map[string]any{"domain": "devops", "kind": "log_set", "name": "devops.log.service"},
			"dest":           map[string]any{"domain": "devops", "kind": "loki", "name": "devops.loki"},
			"fields_mapping": map[string]any{"service_id": "host", "level": "severity"},
		}},
		model.UModelElement{Kind: "loki", Domain: "devops", Name: "devops.loki", Spec: map[string]any{"endpoint": "http://localhost:3100"}},
	)
	if _, err := store.PutUModelElements(ctx, model.UModelElementBatch{Workspace: "demo", Elements: elements}); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	result, err := NewService(store).Execute(ctx, "demo", model.QueryRequest{
		Query:     `.entity_set with(domain='devops', name='devops.service', ids=['node-01']) | entity-call get_logs('devops', 'devops.log.service', query='level = "ERROR"', storage_kind='loki')`,
		Format:    model.FormatAgent,
		TimeRange: model.TimeRange{From: &from, To: &to},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := model.AgentPlanPayload(result)
	if plan == nil || plan["version"] != "v1.1" {
		t.Fatalf("expected agent plan: %#v", plan)
	}
	query := plan["query"].(map[string]any)
	if query["dialect"] != "loki_logql" {
		t.Fatalf("wrong storage plan: %#v", query)
	}
	queries := query["queries"].([]map[string]any)
	if !strings.Contains(queries[0]["logql"].(string), `host="node-01"`) || !strings.Contains(queries[0]["logql"].(string), `severity="ERROR"`) {
		t.Fatalf("wrong LogQL: %#v", queries)
	}
}
