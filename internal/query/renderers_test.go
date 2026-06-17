package query

import (
	"testing"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
)

// TestDefaultRegistryFamilies pins which (storage kind, method) pairs resolve to
// which built-in family renderer, and which fall through to the passthrough.
func TestDefaultRegistryFamilies(t *testing.T) {
	r := newDefaultRegistry()
	cases := []struct {
		kind   string
		method planrender.Method
		family string // "" = expect no renderer (passthrough)
	}{
		{"prometheus", planrender.MethodGetMetrics, "label-timeseries"},
		{"aliyun_prometheus", planrender.MethodGetMetrics, "label-timeseries"},
		{"elasticsearch", planrender.MethodGetLogs, "document-search"},
		{"elasticsearch", planrender.MethodGetMetrics, ""}, // ES is logs-only here
		{"prometheus", planrender.MethodGetLogs, ""},       // prometheus is metrics-only
		{"mysql", planrender.MethodGetLogs, ""},            // discoverable, no renderer
	}
	for _, c := range cases {
		got, ok := r.Find(c.kind, c.method)
		if c.family == "" {
			if ok {
				t.Errorf("(%s, %s): expected passthrough, got renderer %s", c.kind, c.method, got.Family())
			}
			continue
		}
		if !ok || got.Family() != c.family {
			t.Errorf("(%s, %s): want family %q, got ok=%v family=%v", c.kind, c.method, c.family, ok, got)
		}
	}
}
