package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupQuickstartApp(t *testing.T) *App {
	t.Helper()
	app := NewMemoryApp(t.TempDir())
	if _, err := app.LoadQuickStart(context.Background(), QuickStartOptions{}); err != nil {
		t.Fatalf("load quickstart sample: %v", err)
	}
	return app
}

// TestHTTPFormatAgentReturnsPlanObjectDirectly verifies the HTTP layer
// short-circuits the assistant envelope and writes the plan map straight to
// the response body when format=agent is requested.
func TestHTTPFormatAgentReturnsPlanObjectDirectly(t *testing.T) {
	app := setupQuickstartApp(t)
	handler := app.Handler()

	body := `{"query":".entity_set with(domain=\"devops\", name=\"devops.service\", ids=[\"10000000000000000000000000000101\"]) | entity-call get_metrics(\"devops\", \"devops.metric.service\", \"request_count\", step=\"30s\")"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute?format=agent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rr.Code, http.StatusOK, rr.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rr.Body.String())
	}

	// Body must be the plan, not the classic envelope. Classic envelope has
	// top-level "code"/"message"/"success" keys; v1.1 agent body has plan
	// schema keys.
	for _, wrapped := range []string{"code", "data", "message", "success"} {
		if _, present := payload[wrapped]; present {
			t.Fatalf("response should be unwrapped plan; found classic envelope key %q (body=%s)", wrapped, rr.Body.String())
		}
	}
	for _, planKey := range []string{"mode", "version", "operation", "data_source", "query"} {
		if _, present := payload[planKey]; !present {
			t.Fatalf("response missing plan field %q (body=%s)", planKey, rr.Body.String())
		}
	}
	if payload["version"] != "v1.1" {
		t.Fatalf(`payload["version"] = %#v, want "v1.1"`, payload["version"])
	}
	if payload["operation"] != "get_metrics" {
		t.Fatalf(`payload["operation"] = %#v, want "get_metrics"`, payload["operation"])
	}
}

// TestHTTPClassicFormatUnchanged is a regression guard: not passing format=
// must still return the existing {code, data, message, success} envelope so
// existing clients (Web UI, MCP, SDKs) keep working.
func TestHTTPClassicFormatUnchanged(t *testing.T) {
	app := setupQuickstartApp(t)
	handler := app.Handler()

	body := `{"query":".entity_set with(domain=\"devops\", name=\"devops.service\", ids=[\"10000000000000000000000000000101\"]) | entity-call get_metrics(\"devops\", \"devops.metric.service\", \"request_count\", step=\"30s\")"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rr.Code, http.StatusOK, rr.Body.String())
	}

	var envelope map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	for _, k := range []string{"code", "data", "message", "success"} {
		if _, present := envelope[k]; !present {
			t.Fatalf("classic envelope missing %q (body=%s)", k, rr.Body.String())
		}
	}
}

// TestHTTPAgentFormatFallsBackForNonPlanQueries verifies the §6 design
// decision: when format=agent is requested but the query does not produce a
// plan (here, a plain .umodel query), the response falls back to the classic
// envelope rather than erroring. Lets clients pass format=agent as a global
// switch without dispatching per query type.
func TestHTTPAgentFormatFallsBackForNonPlanQueries(t *testing.T) {
	app := setupQuickstartApp(t)
	handler := app.Handler()

	body := `{"query":".umodel | limit 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute?format=agent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rr.Code, http.StatusOK, rr.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, present := envelope["data"]; !present {
		t.Fatalf("non-plan query under format=agent must fall back to classic envelope; got body=%s", rr.Body.String())
	}
}

// TestHTTPFormatAgentWithIncludeSpec verifies the ?include=spec query param
// re-expands the folded data_source fields so a debugging/diagnostic agent
// can see the full storage config and link specs.
func TestHTTPFormatAgentWithIncludeSpec(t *testing.T) {
	app := setupQuickstartApp(t)
	handler := app.Handler()

	body := `{"query":".entity_set with(domain=\"devops\", name=\"devops.service\", ids=[\"10000000000000000000000000000101\"]) | entity-call get_metrics(\"devops\", \"devops.metric.service\", \"request_count\", step=\"30s\")"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute?format=agent&include=spec", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rr.Code, http.StatusOK, rr.Body.String())
	}

	var plan map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	dataSource, _ := plan["data_source"].(map[string]any)
	storage, _ := dataSource["storage"].(map[string]any)
	config, ok := storage["config"].(map[string]any)
	if !ok || len(config) == 0 {
		t.Fatalf("with ?include=spec, storage[config] should be populated; got %#v", storage["config"])
	}

	// Without ?include=spec the same query should NOT carry config (sanity
	// guard that the opt-in is actually doing the gating, not always-on).
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute?format=agent", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	var folded map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &folded); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	storage2, _ := folded["data_source"].(map[string]any)["storage"].(map[string]any)
	if _, present := storage2["config"]; present {
		t.Fatalf("without ?include=spec, storage[config] should be absent; got %#v", storage2["config"])
	}
}

// TestHTTPRejectsBogusFormat verifies format validation is enforced at the
// HTTP boundary and surfaces a structured error.
func TestHTTPRejectsBogusFormat(t *testing.T) {
	app := setupQuickstartApp(t)
	handler := app.Handler()

	body := `{"query":".umodel | limit 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute?format=bogus", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code < 400 {
		t.Fatalf("expected 4xx for format=bogus, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	errObj, ok := envelope["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got body=%s", rr.Body.String())
	}
	if errObj["code"] != "INVALID_ARGUMENT" {
		t.Fatalf("error code = %#v, want INVALID_ARGUMENT", errObj["code"])
	}
}
