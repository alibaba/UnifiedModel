package query

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

// TestPlanV1EnvelopeForGetMetrics verifies that the get_metrics plan carries
// the v1 envelope fields (mode, version, params_echo) — these are the
// additive fields the shared plan ↔ data contract relies on. See
// docs/en/spec/plan-schema-v1.md.
func TestPlanV1EnvelopeForGetMetrics(t *testing.T) {
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
		Query: ".entity_set with(domain='devops', name='devops.service', ids=['svc-1']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')",
	})
	if err != nil {
		t.Fatalf("execute get_metrics: %v", err)
	}

	plan := unmarshalPlan(t, result.Rows[0]["query"])
	if plan["mode"] != "plan" {
		t.Fatalf(`plan["mode"] = %#v, want "plan"`, plan["mode"])
	}
	if plan["version"] != "v1" {
		t.Fatalf(`plan["version"] = %#v, want "v1"`, plan["version"])
	}
	echo, ok := plan["params_echo"].(map[string]any)
	if !ok {
		t.Fatalf("plan[params_echo] is not a map: %#v", plan["params_echo"])
	}
	// Caller-supplied params (metric, step) survive echo so an executor can
	// recover the full call context. The full param set including
	// aggregate/storage_* is exercised end-to-end in commit 3 once the parse
	// spec accepts those keys.
	if echo["metric"] != "request_count" {
		t.Fatalf("params_echo[metric] = %#v, want request_count", echo["metric"])
	}
	if echo["step"] != "30s" {
		t.Fatalf("params_echo[step] = %#v, want 30s", echo["step"])
	}
	// The OSS planner ignores aggregate/storage_*, but the inner query object
	// must still be present. Asserting only its presence keeps this test
	// focused on the v1 envelope.
	if _, ok := plan["query"]; !ok {
		t.Fatalf("plan missing query field: %+v", plan)
	}
}

// TestPlanV1EnvelopeForGetLogs verifies the same envelope for get_logs.
func TestPlanV1EnvelopeForGetLogs(t *testing.T) {
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
		Query: ".entity_set with(domain='devops', name='devops.service') | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')",
	})
	if err != nil {
		t.Fatalf("execute get_logs: %v", err)
	}

	plan := unmarshalPlan(t, result.Rows[0]["query"])
	if plan["mode"] != "plan" {
		t.Fatalf(`plan["mode"] = %#v, want "plan"`, plan["mode"])
	}
	if plan["version"] != "v1" {
		t.Fatalf(`plan["version"] = %#v, want "v1"`, plan["version"])
	}
	echo, ok := plan["params_echo"].(map[string]any)
	if !ok {
		t.Fatalf("plan[params_echo] is not a map: %#v", plan["params_echo"])
	}
	if echo["query"] != `level = "ERROR"` {
		t.Fatalf("params_echo[query] = %#v", echo["query"])
	}
}

// TestEchoParamsStripsEmptyAndNil verifies that the params_echo helper drops
// nil and empty-string values so executors don't accidentally re-introduce
// a parameter the caller never set.
func TestEchoParamsStripsEmptyAndNil(t *testing.T) {
	in := map[string]any{
		"keep_string":  "yes",
		"keep_bool":    true,
		"keep_number":  int64(7),
		"drop_empty":   "",
		"drop_nil":     nil,
		"keep_zero":    0,
		"keep_false":   false,
	}
	out := echoParams(in)

	for _, k := range []string{"keep_string", "keep_bool", "keep_number", "keep_zero", "keep_false"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("expected %q to survive echo, got %+v", k, out)
		}
	}
	for _, k := range []string{"drop_empty", "drop_nil"} {
		if _, ok := out[k]; ok {
			t.Fatalf("expected %q to be stripped, got %+v", k, out)
		}
	}
}

// TestPlanV1AgentFriendlyFieldsForGetMetrics verifies that the agent-friendly
// additions land in the metric plan: a human-readable description, a
// next_action hint, and an echo of the original SPL source query.
func TestPlanV1AgentFriendlyFieldsForGetMetrics(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	if _, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements:  metricQueryPlanElements(),
	}); err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	const spl = `.entity_set with(domain='devops', name='devops.service', ids=['svc-1']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')`
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: spl})
	if err != nil {
		t.Fatalf("execute get_metrics: %v", err)
	}

	plan := unmarshalPlan(t, result.Rows[0]["query"])
	desc, _ := plan["description"].(string)
	if desc == "" {
		t.Fatalf("plan[description] missing or empty: %#v", plan["description"])
	}
	for _, want := range []string{"request_count", "devops.metric.service", "umodel-assistant"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("description should mention %q, got %q", want, desc)
		}
	}
	if plan["next_action"] != "forward_to_executor" {
		t.Fatalf("plan[next_action] = %#v, want forward_to_executor", plan["next_action"])
	}
	if plan["source_query"] != spl {
		t.Fatalf("plan[source_query] = %#v, want %q", plan["source_query"], spl)
	}
}

// TestPlanV1AgentFriendlyFieldsForGetLogs verifies the same for logs.
func TestPlanV1AgentFriendlyFieldsForGetLogs(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	if _, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements:  logQueryPlanElements(),
	}); err != nil {
		t.Fatalf("put umodel: %v", err)
	}

	svc := NewService(store)
	const spl = `.entity_set with(domain='devops', name='devops.service') | entity-call get_logs('devops', 'devops.log.service', query='level = "ERROR"')`
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: spl})
	if err != nil {
		t.Fatalf("execute get_logs: %v", err)
	}

	plan := unmarshalPlan(t, result.Rows[0]["query"])
	desc, _ := plan["description"].(string)
	if desc == "" {
		t.Fatalf("plan[description] missing or empty: %#v", plan["description"])
	}
	for _, want := range []string{"devops.log.service", `level = "ERROR"`, "umodel-assistant"} {
		if !strings.Contains(desc, want) {
			t.Fatalf("description should mention %q, got %q", want, desc)
		}
	}
	if plan["next_action"] != "forward_to_executor" {
		t.Fatalf("plan[next_action] = %#v, want forward_to_executor", plan["next_action"])
	}
	if plan["source_query"] != spl {
		t.Fatalf("plan[source_query] = %#v, want %q", plan["source_query"], spl)
	}
}

func unmarshalPlan(t *testing.T, raw any) map[string]any {
	t.Helper()
	s, ok := raw.(string)
	if !ok || s == "" {
		t.Fatalf("expected JSON-stringified plan, got %#v", raw)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(s), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	return plan
}
