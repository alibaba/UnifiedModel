package query

import (
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
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
		// Field names are validated and backtick-quoted; values are single-quoted.
		{Constraint{Field: "svc", Op: "eq", Values: []string{"checkout"}}, "`svc` = 'checkout'"},
		{Constraint{Field: "svc", Op: "neq", Values: []string{"cart"}}, "`svc` != 'cart'"},
		{Constraint{Field: "region", Op: "in", Values: []string{"cn", "us"}}, "`region` IN ('cn', 'us')"},
		{Constraint{Field: "region", Op: "in", Values: []string{"cn"}}, "`region` = 'cn'"},
		{Constraint{Field: "region", Op: "notin", Values: []string{"cn", "us"}}, "`region` NOT IN ('cn', 'us')"},
		{Constraint{Field: "name", Op: "eq", Values: []string{"o'brien"}}, "`name` = 'o''brien'"}, // quote escaping
	}
	for _, c := range cases {
		got, err := constraintsToSQLWhere([]Constraint{c.c})
		if err != nil {
			t.Errorf("constraintsToSQLWhere(%+v) errored: %v", c.c, err)
			continue
		}
		if got != c.want {
			t.Errorf("constraintsToSQLWhere(%+v) = %q, want %q", c.c, got, c.want)
		}
	}
}

// TestConstraintsToSQLWhereRejectsInjectableField guards the identifier surface:
// a field name that is not a plain SQL identifier must be rejected, not emitted.
func TestConstraintsToSQLWhereRejectsInjectableField(t *testing.T) {
	bad := Constraint{Field: "svc`); DROP TABLE x; --", Op: "eq", Values: []string{"x"}}
	if _, err := constraintsToSQLWhere([]Constraint{bad}); err == nil {
		t.Fatal("expected an invalid filter field to be rejected")
	}
}

func TestSqlInterval(t *testing.T) {
	valid := map[string]string{
		"1m": "1 MINUTE", "30s": "30 SECOND", "100ms": "100 MILLISECOND",
		"2h": "2 HOUR", "7d": "7 DAY", "1w": "1 WEEK",
	}
	for step, want := range valid {
		got, err := sqlInterval(step)
		if err != nil || got != want {
			t.Errorf("sqlInterval(%q) = (%q, %v), want (%q, nil)", step, got, err, want)
		}
	}
	// Strict: empty, missing/invalid unit, non-positive, and injection attempts
	// are rejected rather than interpolated into the INTERVAL.
	for _, bad := range []string{"", "5", "0s", "-1m", "1x", "1 m", "1m; DROP TABLE t", "abc", "9999999999999999999999s"} {
		if _, err := sqlInterval(bad); err == nil {
			t.Errorf("sqlInterval(%q) should be rejected", bad)
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
	// Identifiers are backtick-quoted; values stay single-quoted.
	for _, want := range []string{
		"toStartOfInterval(`timestamp`, INTERVAL 1 MINUTE)",
		"avg(`val`) AS value",
		"FROM `metrics`",
		"`id` = 'id1'",
		"`name` = 'request_latency'",
		"`timestamp` BETWEEN {from} AND {to}",
		"GROUP BY bucket",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL missing %q:\n%s", want, sql)
		}
	}
}

// TestSQLTableRendererFailsClosedOnRawFilters proves the renderer refuses to emit
// an under-constrained query: a filter shape the IR cannot express (here an OR)
// lands in raw, and Render returns an error rather than silently dropping it.
func TestSQLTableRendererFailsClosedOnRawFilters(t *testing.T) {
	for _, filter := range []string{"a = 1 or b = 2", "this is (not parseable"} {
		req := planrender.Request{
			Method:     planrender.MethodGetMetrics,
			DataSet:    model.UModelElement{Kind: "metric_set", Name: "svc.metrics"},
			Storage:    model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"table": "metrics"}},
			Metrics:    []map[string]any{{"name": "m"}},
			DataFilter: filter,
		}
		if _, err := (sqlTableRenderer{}).Render(req); err == nil {
			t.Errorf("Render with unsupported filter %q should fail closed", filter)
		}
	}
}

// TestSQLTableRendererRejectsInjection covers the identifier and step injection
// surfaces: a malformed table/column mapping or a non-duration step must be
// rejected, not interpolated into the generated SQL.
func TestSQLTableRendererRejectsInjection(t *testing.T) {
	base := func() planrender.Request {
		return planrender.Request{
			Method:  planrender.MethodGetMetrics,
			DataSet: model.UModelElement{Kind: "metric_set", Name: "svc.metrics"},
			Storage: model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"table": "metrics"}},
			Metrics: []map[string]any{{"name": "m"}},
			Step:    "1m",
		}
	}
	cases := map[string]func(*planrender.Request){
		"injectable table":     func(r *planrender.Request) { r.Storage.Spec["table"] = "metrics; DROP TABLE x" },
		"injectable value col": func(r *planrender.Request) { r.Storage.Spec["value_column"] = "val`)" },
		"injectable step":      func(r *planrender.Request) { r.Step = "1m; DROP TABLE t" },
	}
	for name, mutate := range cases {
		req := base()
		mutate(&req)
		if _, err := (sqlTableRenderer{}).Render(req); err == nil {
			t.Errorf("%s: Render should reject the request", name)
		}
	}
}

// TestSQLTableRendererQualifiesDatabase proves spec.database is honored: the FROM
// targets `database`.`table` and the plan echoes the database, so the query never
// runs against whichever default database the executing connection happens to use.
func TestSQLTableRendererQualifiesDatabase(t *testing.T) {
	req := planrender.Request{
		Method:    planrender.MethodGetMetrics,
		DataSet:   model.UModelElement{Kind: "metric_set", Name: "svc.metrics"},
		Storage:   model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"database": "observability", "table": "service_metrics"}},
		Metrics:   []map[string]any{{"name": "m"}},
		QueryType: "range",
		Step:      "1m",
	}
	out, err := sqlTableRenderer{}.Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if out["database"] != "observability" {
		t.Errorf("database = %v, want observability", out["database"])
	}
	queries, _ := out["queries"].([]map[string]any)
	sql, _ := queries[0]["sql"].(string)
	if !strings.Contains(sql, "FROM `observability`.`service_metrics`") {
		t.Errorf("SQL should qualify the table with the database:\n%s", sql)
	}
	// An injectable database is rejected like any other identifier.
	bad := req
	bad.Storage = model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"database": "obs; DROP TABLE x", "table": "service_metrics"}}
	if _, err := (sqlTableRenderer{}).Render(bad); err == nil {
		t.Errorf("an injectable database should be rejected")
	}
}

// TestSQLTableRendererRejectsInvalidQueryType proves the renderer fails closed on a
// query_type other than instant/range, instead of silently emitting range SQL that
// the plan then labels with the bogus value.
func TestSQLTableRendererRejectsInvalidQueryType(t *testing.T) {
	render := func(qt string) error {
		_, err := sqlTableRenderer{}.Render(planrender.Request{
			Method:    planrender.MethodGetMetrics,
			DataSet:   model.UModelElement{Kind: "metric_set", Name: "svc.metrics"},
			Storage:   model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"table": "metrics"}},
			Metrics:   []map[string]any{{"name": "m"}},
			QueryType: qt,
			Step:      "1m",
		})
		return err
	}
	for _, qt := range []string{"instant", "range"} {
		if err := render(qt); err != nil {
			t.Errorf("query_type %q should be accepted: %v", qt, err)
		}
	}
	for _, qt := range []string{"foo", "INSTANT", "both", "1; DROP TABLE t"} {
		if err := render(qt); err == nil {
			t.Errorf("query_type %q should be rejected", qt)
		}
	}
}

func TestQuoteSQLIdent(t *testing.T) {
	ok := map[string]string{"svc": "`svc`", "service_id": "`service_id`", "db.metrics": "`db`.`metrics`"}
	for in, want := range ok {
		got, err := quoteSQLIdent(in)
		if err != nil || got != want {
			t.Errorf("quoteSQLIdent(%q) = (%q, %v), want (%q, nil)", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "1col", "a b", "a;b", "a`b", "a-b", "a.", ".a", "a..b"} {
		if _, err := quoteSQLIdent(bad); err == nil {
			t.Errorf("quoteSQLIdent(%q) should be rejected", bad)
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
	plain, err := e.buildMetricStorageQuery(metricSet, bare, nil, nil, metrics, []string{"id1"}, "", "", "", nil, "", "", 100)
	if err != nil {
		t.Fatalf("passthrough should not error: %v", err)
	}
	if plain["dialect"] != "clickhouse" {
		t.Fatalf("without spec.family: expected passthrough dialect clickhouse, got %v", plain["dialect"])
	}

	configured := model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"family": "sql-table", "table": "metrics"}}
	rendered, err := e.buildMetricStorageQuery(metricSet, configured, nil, nil, metrics, []string{"id1"}, "", "", "", nil, "", "", 100)
	if err != nil {
		t.Fatalf("sql-table render should not error: %v", err)
	}
	if rendered["dialect"] != "clickhouse_sql" {
		t.Fatalf("with spec.family=sql-table: expected clickhouse_sql plan, got %v", rendered["dialect"])
	}
}

// TestRendererErrorPropagatesNotPassthrough proves the executor surfaces a
// matched renderer's error instead of falling back to an unrendered passthrough
// that would drop the resolved filters.
func TestRendererErrorPropagatesNotPassthrough(t *testing.T) {
	e := NewExecutor(nil)
	metricSet := model.UModelElement{Kind: "metric_set", Name: "svc.metrics"}
	metrics := []map[string]any{{"name": "m"}}
	// family=sql-table is matched, but the OR filter cannot be expressed -> error.
	storage := model.UModelElement{Kind: "clickhouse", Spec: map[string]any{"family": "sql-table", "table": "metrics"}}
	out, err := e.buildMetricStorageQuery(metricSet, storage, nil, nil, metrics, nil, "", "a = 1 or b = 2", "", nil, "", "1m", 100)
	if err == nil {
		t.Fatalf("expected the renderer error to propagate, got passthrough: %v", out)
	}
	if !apperrors.IsCode(err, apperrors.CodeQueryPlanError) {
		t.Fatalf("expected CodeQueryPlanError, got %v", err)
	}
}
