package query

import (
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func constraintByField(cs []Constraint, field string) (Constraint, bool) {
	for _, c := range cs {
		if c.Field == field {
			return c, true
		}
	}
	return Constraint{}, false
}

// TestResolveConstraints checks that entity ids and filter sources resolve into
// the normalized IR. With nil mappings, storage field names are the identity of
// the entity/dataset field names.
func TestResolveConstraints(t *testing.T) {
	req := planrender.Request{
		Storage:     model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"search_filter": "tenant = acme"}},
		EntityIDs:   []string{"id1", "id2"},
		DataFilter:  "env = prod",
		EntityQuery: "region in [cn, us]",
	}
	cs, raw := ResolveConstraints(req)
	if len(raw) != 0 {
		t.Fatalf("expected no unsupported filters, got %v", raw)
	}
	if c, ok := constraintByField(cs, "id"); !ok || c.Op != "in" || strings.Join(c.Values, ",") != "id1,id2" {
		t.Errorf("id constraint wrong: %+v (ok=%v)", c, ok)
	}
	if c, ok := constraintByField(cs, "tenant"); !ok || c.Op != "eq" || c.Values[0] != "acme" {
		t.Errorf("search_filter constraint wrong: %+v (ok=%v)", c, ok)
	}
	if c, ok := constraintByField(cs, "env"); !ok || c.Op != "eq" || c.Values[0] != "prod" {
		t.Errorf("data_filter constraint wrong: %+v (ok=%v)", c, ok)
	}
	if c, ok := constraintByField(cs, "region"); !ok || c.Op != "in" || strings.Join(c.Values, ",") != "cn,us" {
		t.Errorf("entity_query constraint wrong: %+v (ok=%v)", c, ok)
	}
}

func TestResolveConstraintsPassesThroughUnparseable(t *testing.T) {
	req := planrender.Request{DataFilter: "this is (not parseable"}
	_, raw := ResolveConstraints(req)
	if len(raw) != 1 || raw[0] != "this is (not parseable" {
		t.Fatalf("expected the raw filter to pass through, got %v", raw)
	}
}

func TestConstraintsToSQLWhere(t *testing.T) {
	cases := []struct {
		c    Constraint
		want string
	}{
		{Constraint{Field: "svc", Op: "eq", Values: []string{"checkout"}}, "svc = 'checkout'"},
		{Constraint{Field: "svc", Op: "neq", Values: []string{"cart"}}, "svc != 'cart'"},
		{Constraint{Field: "region", Op: "in", Values: []string{"cn", "us"}}, "region IN ('cn', 'us')"},
		{Constraint{Field: "region", Op: "in", Values: []string{"cn"}}, "region = 'cn'"},
		{Constraint{Field: "region", Op: "notin", Values: []string{"cn", "us"}}, "region NOT IN ('cn', 'us')"},
		{Constraint{Field: "name", Op: "eq", Values: []string{"o'brien"}}, "name = 'o''brien'"}, // quote escaping
	}
	for _, c := range cases {
		if got := constraintsToSQLWhere([]Constraint{c.c}); got != c.want {
			t.Errorf("constraintsToSQLWhere(%+v) = %q, want %q", c.c, got, c.want)
		}
	}
}

func TestSqlInterval(t *testing.T) {
	cases := map[string]string{"1m": "1 MINUTE", "30s": "30 SECOND", "100ms": "100 MILLISECOND", "2h": "2 HOUR", "": "1 MINUTE", "5": "5 SECOND"}
	for step, want := range cases {
		if got := sqlInterval(step); got != want {
			t.Errorf("sqlInterval(%q) = %q, want %q", step, got, want)
		}
	}
}

func TestSQLTableRendererGetMetrics(t *testing.T) {
	req := planrender.Request{
		Method:    planrender.MethodGetMetrics,
		DataSet:   model.UModelElement{Kind: "metric_set", Name: "svc.metrics"},
		Storage:   model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"table": "metrics", "value_column": "val", "metric_column": "name"}},
		Metrics:   []map[string]any{{"name": "request_latency"}},
		EntityIDs: []string{"id1"},
		QueryType: "range",
		Step:      "1m",
		Limit:     100,
	}
	out, err := sqlTableRenderer{}.Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out["dialect"] != "clickhouse_sql" {
		t.Fatalf("dialect = %v, want clickhouse_sql", out["dialect"])
	}
	if out["table"] != "metrics" {
		t.Errorf("table = %v, want metrics", out["table"])
	}
	queries, _ := out["queries"].([]map[string]any)
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	sql, _ := queries[0]["sql"].(string)
	for _, want := range []string{
		"toStartOfInterval(timestamp, INTERVAL 1 MINUTE)",
		"avg(val) AS value",
		"FROM metrics",
		"id = 'id1'",
		"name = 'request_latency'",
		"timestamp BETWEEN {from} AND {to}",
		"GROUP BY bucket",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL missing %q:\n%s", want, sql)
		}
	}
}

// TestClickHouseRoutesViaSQLFamilyWithoutCode is the new-family extension proof:
// a clickhouse storage routes to the sql-table renderer purely by declaring
// spec.family — the renderer is new code (the new query syntax), but reaching it
// from a new backend kind is configuration only. Without spec.family the kind
// falls through to the unrendered passthrough.
func TestClickHouseRoutesViaSQLFamilyWithoutCode(t *testing.T) {
	e := NewExecutor(nil)
	metricSet := model.UModelElement{Kind: "metric_set", Name: "svc.metrics"}
	metrics := []map[string]any{{"name": "request_latency"}}

	bare := model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"table": "metrics"}}
	plain := e.buildMetricStorageQuery(metricSet, bare, nil, nil, metrics, []string{"id1"}, "", "", "", nil, "", "", 100)
	if plain["dialect"] != "clickhouse" {
		t.Fatalf("without spec.family: expected passthrough dialect clickhouse, got %v", plain["dialect"])
	}

	configured := model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"family": "sql-table", "table": "metrics"}}
	rendered := e.buildMetricStorageQuery(metricSet, configured, nil, nil, metrics, []string{"id1"}, "", "", "", nil, "", "", 100)
	if rendered["dialect"] != "clickhouse_sql" {
		t.Fatalf("with spec.family=sql-table: expected clickhouse_sql plan, got %v", rendered["dialect"])
	}
}
