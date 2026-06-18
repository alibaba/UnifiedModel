package query

import (
	"testing"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

// TestDefaultFamilyForKind pins the built-in kind->family mapping that the
// dispatch falls back to when a storage does not declare spec.family.
func TestDefaultFamilyForKind(t *testing.T) {
	cases := map[string]string{
		"prometheus":        "label-timeseries",
		"aliyun_prometheus": "label-timeseries",
		"elasticsearch":     "document-search",
		"mysql":             "", // discoverable, no built-in family
		"victoriametrics":   "", // new kind: no default, must declare spec.family
	}
	for kind, want := range cases {
		if got := defaultFamilyForKind(kind); got != want {
			t.Errorf("defaultFamilyForKind(%q) = %q, want %q", kind, got, want)
		}
	}
}

// TestDefaultRegistryResolvesFamilies pins which (family, method) pairs the
// built-in registry serves, and which fall through to the passthrough.
func TestDefaultRegistryResolvesFamilies(t *testing.T) {
	r := newDefaultRegistry()
	cases := []struct {
		family string
		method planrender.Method
		ok     bool
	}{
		{"label-timeseries", planrender.MethodGetMetrics, true},
		{"document-search", planrender.MethodGetLogs, true},
		{"label-timeseries", planrender.MethodGetLogs, false},   // metrics family, not logs
		{"document-search", planrender.MethodGetMetrics, false}, // log family, not metrics
		{"sql-table", planrender.MethodGetMetrics, true},        // SQL metrics family
		{"sql-table", planrender.MethodGetLogs, false},          // sql-table is metrics-only for now
		{"", planrender.MethodGetMetrics, false},                // passthrough
	}
	for _, c := range cases {
		got, ok := r.Find(c.family, c.method)
		if ok != c.ok {
			t.Errorf("Find(%q, %s): ok=%v, want %v", c.family, c.method, ok, c.ok)
			continue
		}
		if ok && got.Family() != c.family {
			t.Errorf("Find(%q, %s): returned family %q", c.family, c.method, got.Family())
		}
	}
}

// TestFamilyOverrideRoutesNewBackendWithoutCode is the Phase-2 extension proof:
// a storage kind unknown to the built-in kind->family map (here a
// VictoriaMetrics-style backend) renders through the existing label-timeseries
// renderer purely by declaring spec.family — no new Go renderer code. Without
// spec.family the same kind falls through to the unrendered passthrough. This is
// the "new same-family backend = configuration only" guarantee.
func TestFamilyOverrideRoutesNewBackendWithoutCode(t *testing.T) {
	e := NewExecutor(nil) // rendering is pure; it never touches the graph store
	metricSet := model.UModelElement{Kind: "metric_set", Name: "svc.metrics"}
	metrics := []map[string]any{{"name": "http_requests_total"}}

	// New kind, no family declared -> no renderer -> passthrough echoes the kind.
	bare := model.UModelElement{Kind: "victoriametrics", Spec: map[string]any{"endpoint": "http://localhost:8428"}}
	plain, err := e.buildMetricStorageQuery(metricSet, bare, nil, nil, metrics, []string{"id1"}, "", "", "", nil, "", "", 100)
	if err != nil {
		t.Fatalf("passthrough should not error: %v", err)
	}
	if plain["dialect"] != "victoriametrics" {
		t.Fatalf("without spec.family: expected passthrough dialect %q, got %v", "victoriametrics", plain["dialect"])
	}

	// Same kind + spec.family=label-timeseries -> routes to the PromQL renderer.
	configured := model.UModelElement{Kind: "victoriametrics", Spec: map[string]any{
		"family":   "label-timeseries",
		"endpoint": "http://localhost:8428",
	}}
	rendered, err := e.buildMetricStorageQuery(metricSet, configured, nil, nil, metrics, []string{"id1"}, "", "", "", nil, "", "", 100)
	if err != nil {
		t.Fatalf("label-timeseries render should not error: %v", err)
	}
	if rendered["dialect"] != "prometheus_promql" {
		t.Fatalf("with spec.family=label-timeseries: expected %q plan, got %v", "prometheus_promql", rendered["dialect"])
	}
	if rendered["endpoint"] != "http://localhost:8428" {
		t.Fatalf("rendered plan should carry the configured endpoint, got %v", rendered["endpoint"])
	}
}
