package query

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

// TestGetMetricsAcceptsAggregateAndStorageParams verifies that the parser
// accepts the aggregate / storage_domain / storage_name / storage_kind named
// arguments aligned with umodel-assistant's get_metric handler. The
// open-source planner does not consume them, but they round-trip through
// params_echo so a downstream executor can act on them.
func TestGetMetricsAcceptsAggregateAndStorageParams(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	if _, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements:  metricQueryPlanElements(),
	}); err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service', ids=['svc-1']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s', aggregate=false, storage_domain='alt', storage_name='primary', storage_kind='prometheus')",
	})
	if err != nil {
		t.Fatalf("execute get_metrics: %v", err)
	}

	plan := unmarshalPlan(t, result.Rows[0]["query"])
	echo, ok := plan["params_echo"].(map[string]any)
	if !ok {
		t.Fatalf("params_echo missing: %+v", plan)
	}
	if echo["aggregate"] != false {
		t.Fatalf("params_echo[aggregate] = %#v, want false", echo["aggregate"])
	}
	if echo["storage_domain"] != "alt" {
		t.Fatalf("params_echo[storage_domain] = %#v, want alt", echo["storage_domain"])
	}
	if echo["storage_name"] != "primary" {
		t.Fatalf("params_echo[storage_name] = %#v, want primary", echo["storage_name"])
	}
	if echo["storage_kind"] != "prometheus" {
		t.Fatalf("params_echo[storage_kind] = %#v, want prometheus", echo["storage_kind"])
	}
}

// TestGetLogsAcceptsStorageParams verifies the same alignment for get_logs.
func TestGetLogsAcceptsStorageParams(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	if _, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements:  logQueryPlanElements(),
	}); err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service') | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"', storage_domain='alt', storage_name='primary', storage_kind='elasticsearch')",
	})
	if err != nil {
		t.Fatalf("execute get_logs: %v", err)
	}

	plan := unmarshalPlan(t, result.Rows[0]["query"])
	echo, ok := plan["params_echo"].(map[string]any)
	if !ok {
		t.Fatalf("params_echo missing: %+v", plan)
	}
	if echo["storage_domain"] != "alt" {
		t.Fatalf("params_echo[storage_domain] = %#v, want alt", echo["storage_domain"])
	}
	if echo["storage_name"] != "primary" {
		t.Fatalf("params_echo[storage_name] = %#v, want primary", echo["storage_name"])
	}
	if echo["storage_kind"] != "elasticsearch" {
		t.Fatalf("params_echo[storage_kind] = %#v, want elasticsearch", echo["storage_kind"])
	}
}

// TestListMethodReportsAlignedSignatures verifies that __list_method__ surfaces
// the same params the parser accepts, so external clients (PaaS executor,
// MCP, SDK) discover them through the canonical metadata path.
func TestListMethodReportsAlignedSignatures(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	if _, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements: []model.UModelElement{
			{Kind: "entity_set", Domain: "devops", Name: "devops.service"},
		},
	}); err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service') | entity-call __list_method__()",
	})
	if err != nil {
		t.Fatalf("execute __list_method__: %v", err)
	}

	if len(result.Rows) == 0 {
		t.Fatalf("expected __list_method__ to return a row")
	}
	envelope := result.Rows[0]
	data, ok := envelope["data"].([]map[string]any)
	if !ok {
		t.Fatalf("expected envelope.data to be []map[string]any, got %T", envelope["data"])
	}

	signatures := map[string][]string{} // method name -> param keys
	for _, entry := range data {
		values, ok := entry["values"].([]string)
		if !ok || len(values) < 4 {
			continue
		}
		name := values[0]
		paramsJSON := values[3]
		var params []map[string]any
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			t.Fatalf("decode params for %q: %v", name, err)
		}
		keys := make([]string, 0, len(params))
		for _, p := range params {
			if k, ok := p["key"].(string); ok {
				keys = append(keys, k)
			}
		}
		signatures[name] = keys
	}

	mustContain := func(method string, wanted []string) {
		t.Helper()
		got, ok := signatures[method]
		if !ok {
			t.Fatalf("method %q missing from __list_method__: %+v", method, signatures)
		}
		joined := strings.Join(got, ",")
		for _, w := range wanted {
			if !strings.Contains(joined, w) {
				t.Fatalf("method %q params missing %q (got %s)", method, w, joined)
			}
		}
	}

	mustContain("get_metrics", []string{"domain", "name", "metric", "query", "query_type", "step", "aggregate", "storage_domain", "storage_name", "storage_kind"})
	mustContain("get_logs", []string{"domain", "name", "query", "storage_domain", "storage_name", "storage_kind"})
}
