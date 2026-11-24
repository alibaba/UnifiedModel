package integration_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestHTTPLokiPlansFromImportedExample(t *testing.T) {
	exampleRoot, err := filepath.Abs(filepath.Join("..", "..", "examples", "loki-storage"))
	if err != nil {
		t.Fatalf("resolve Loki example path: %v", err)
	}
	server := httptest.NewServer(testutil.ProviderApp(t, t.TempDir(), bootstrap.WithImportRoot(exampleRoot)).Handler())
	defer server.Close()

	post(t, server.URL+"/api/v1/workspaces", map[string]any{"id": "demo"})
	imported := post(t, server.URL+"/api/v1/umodel/demo/import", map[string]any{"path": exampleRoot})
	if imported["imported"] != float64(8) {
		t.Fatalf("expected all eight Loki example elements to import: %+v", imported)
	}

	endpoint := server.URL + "/api/v1/query/demo/execute?format=agent"
	timeRange := map[string]any{"from": "2026-11-03T02:00:00Z", "to": "2026-11-03T03:00:00Z"}
	logQuery := `.entity_set with(domain='ops', name='ops.node') | entity-call get_logs('ops', 'ops.node.logs', query='level = "ERROR"', storage_kind='loki')`
	logs := post(t, endpoint, map[string]any{
		"query":      logQuery,
		"time_range": timeRange,
		"limit":      20,
		"entity_data": map[string]any{
			"header": []string{"id", "environment"},
			"data":   [][]string{{"node-01", "prod"}, {"node-02", "staging"}},
		},
	})
	if logs["mode"] != "plan" || logs["operation"] != "get_logs" {
		t.Fatalf("expected a Loki log plan: %+v", logs)
	}
	logPlan := lokiQueryBlock(t, logs)
	if logPlan["dialect"] != "loki_logql" || logPlan["endpoint"] != "http://localhost:3100" || logPlan["size"] != float64(20) {
		t.Fatalf("unexpected Loki log query metadata: %+v", logPlan)
	}
	logQL := lokiQueryStrings(t, logPlan)
	if len(logQL) != 2 || !strings.Contains(logQL[0], `host="node-01"`) || !strings.Contains(logQL[0], `env="prod"`) || !strings.Contains(logQL[0], `severity="ERROR"`) || strings.Contains(logQL[0], "staging") || !strings.Contains(logQL[1], `host="node-02"`) || !strings.Contains(logQL[1], `env="staging"`) {
		t.Fatalf("entity rows were not preserved in the log queries: %q", logQL)
	}

	for _, mode := range []string{"instant", "range"} {
		t.Run(mode, func(t *testing.T) {
			metricTimeRange := map[string]any{"to": timeRange["to"]}
			if mode == "range" {
				metricTimeRange["from"] = timeRange["from"]
			}
			metricQuery := `.entity_set with(domain='ops', name='ops.node') | entity-call get_metrics('ops', 'ops.node.metrics', 'error_rate', query_type='` + mode + `', step='30s', storage_kind='loki')`
			metric := post(t, endpoint, map[string]any{
				"query":      metricQuery,
				"time_range": metricTimeRange,
				"entity_data": map[string]any{
					"header": []string{"id", "environment"},
					"data":   [][]string{{"node-01", "prod"}},
				},
			})
			if metric["mode"] != "plan" || metric["operation"] != "get_metrics" {
				t.Fatalf("expected a Loki metric plan: %+v", metric)
			}
			metricPlan := lokiQueryBlock(t, metric)
			if metricPlan["dialect"] != "loki_logql" || metricPlan["query_type"] != mode {
				t.Fatalf("unexpected Loki metric query metadata: %+v", metricPlan)
			}
			if mode == "range" && metricPlan["step"] != "30s" {
				t.Fatalf("range metric step was not retained: %+v", metricPlan)
			}
			if mode == "instant" && metricPlan["step"] != nil {
				t.Fatalf("instant metric should not have a step: %+v", metricPlan)
			}
			metricLogQL := lokiQueryStrings(t, metricPlan)
			if len(metricLogQL) != 1 || !strings.Contains(metricLogQL[0], `host="node-01"`) || !strings.Contains(metricLogQL[0], `env="prod"`) || strings.Contains(metricLogQL[0], "$host") {
				t.Fatalf("metric LogQL was not bound to the entity: %q", metricLogQL)
			}
		})
	}

	missingTime := postStatus(t, endpoint, map[string]any{
		"query": logQuery,
		"entity_data": map[string]any{
			"header": []string{"id", "environment"},
			"data":   [][]string{{"node-01", "prod"}},
		},
	}, http.StatusBadRequest)
	if errorCode(t, missingTime) != "QUERY_PLAN_ERROR" {
		t.Fatalf("expected a missing-time plan error: %+v", missingTime)
	}
}

func lokiQueryBlock(t *testing.T, plan map[string]any) map[string]any {
	t.Helper()
	query, ok := plan["query"].(map[string]any)
	if !ok {
		t.Fatalf("plan has no query block: %+v", plan)
	}
	return query
}

func lokiQueryStrings(t *testing.T, plan map[string]any) []string {
	t.Helper()
	items, ok := plan["queries"].([]any)
	if !ok {
		t.Fatalf("Loki plan has no query list: %+v", plan)
	}
	queries := make([]string, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("invalid Loki query entry: %+v", item)
		}
		logQL, ok := entry["logql"].(string)
		if !ok {
			t.Fatalf("Loki query entry has no LogQL: %+v", entry)
		}
		queries = append(queries, logQL)
	}
	return queries
}
