package planrender

import "testing"

type fakeRenderer struct {
	family  string
	methods map[Method]bool
	tag     string
}

func (f fakeRenderer) Family() string               { return f.family }
func (f fakeRenderer) SupportsMethod(m Method) bool { return f.methods[m] }
func (f fakeRenderer) Render(Request) (map[string]any, error) {
	return map[string]any{"tag": f.tag}, nil
}

func TestRegistryFind(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Find("label-timeseries", MethodGetMetrics); ok {
		t.Fatal("empty registry should match nothing")
	}
	r.Register(fakeRenderer{family: "label-timeseries", methods: map[Method]bool{MethodGetMetrics: true}})

	got, ok := r.Find("label-timeseries", MethodGetMetrics)
	if !ok || got.Family() != "label-timeseries" {
		t.Fatalf("want family label-timeseries, got ok=%v family=%v", ok, got)
	}
	if _, ok := r.Find("label-timeseries", MethodGetLogs); ok {
		t.Fatal("method mismatch must not match")
	}
	if _, ok := r.Find("document-search", MethodGetMetrics); ok {
		t.Fatal("family mismatch must not match")
	}
	if _, ok := r.Find("", MethodGetMetrics); ok {
		t.Fatal("empty family must not match (routes to passthrough)")
	}
}

func TestRegistryLaterRegistrationWins(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeRenderer{family: "label-timeseries", methods: map[Method]bool{MethodGetMetrics: true}, tag: "old"})
	r.Register(fakeRenderer{family: "label-timeseries", methods: map[Method]bool{MethodGetMetrics: true}, tag: "new"})

	got, ok := r.Find("label-timeseries", MethodGetMetrics)
	if !ok {
		t.Fatal("expected a renderer")
	}
	out, _ := got.Render(Request{})
	if out["tag"] != "new" {
		t.Fatalf("later registration should win, got tag=%v", out["tag"])
	}
}
