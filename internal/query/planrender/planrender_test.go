package planrender

import "testing"

type fakeRenderer struct {
	family string
	kinds  map[string]bool
	method Method
}

func (f fakeRenderer) Family() string { return f.family }
func (f fakeRenderer) Handles(kind string, m Method) bool {
	return m == f.method && f.kinds[kind]
}
func (f fakeRenderer) Render(Request) (map[string]any, error) {
	return map[string]any{"family": f.family}, nil
}

func TestRegistryFind(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Find("prometheus", MethodGetMetrics); ok {
		t.Fatal("empty registry should match nothing")
	}
	r.Register(fakeRenderer{family: "a", kinds: map[string]bool{"prometheus": true}, method: MethodGetMetrics})

	got, ok := r.Find("prometheus", MethodGetMetrics)
	if !ok || got.Family() != "a" {
		t.Fatalf("want family a, got ok=%v family=%v", ok, got)
	}
	if _, ok := r.Find("prometheus", MethodGetLogs); ok {
		t.Fatal("method mismatch must not match")
	}
	if _, ok := r.Find("elasticsearch", MethodGetMetrics); ok {
		t.Fatal("kind mismatch must not match")
	}
}

func TestRegistryLaterRegistrationWins(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeRenderer{family: "old", kinds: map[string]bool{"prometheus": true}, method: MethodGetMetrics})
	r.Register(fakeRenderer{family: "new", kinds: map[string]bool{"prometheus": true}, method: MethodGetMetrics})
	got, ok := r.Find("prometheus", MethodGetMetrics)
	if !ok || got.Family() != "new" {
		t.Fatalf("later registration should win, got ok=%v family=%v", ok, got)
	}
}
