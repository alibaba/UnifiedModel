package query

import (
	"context"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

// TestGetMetricsAgentFormatPlanShape verifies that requesting format=agent
// produces an agent-format result: a single sentinel-column row carrying the
// plan map directly, with version "v1.1" and compact {ref, kind/type}
// data_source entries instead of full spec dumps.
func TestGetMetricsAgentFormatPlanShape(t *testing.T) {
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
		Query:  ".entity_set with(domain='devops', name='devops.service', ids=['svc-1']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')",
		Format: model.FormatAgent,
	})
	if err != nil {
		t.Fatalf("execute get_metrics agent: %v", err)
	}

	if !model.IsAgentPlanResult(result) {
		t.Fatalf("expected agent-format result (sentinel column %q), got columns=%v", model.AgentPlanResultColumn, result.Columns)
	}
	plan := model.AgentPlanPayload(result)
	if plan == nil {
		t.Fatalf("AgentPlanPayload returned nil; result=%+v", result)
	}

	if plan["version"] != "v1.1" {
		t.Fatalf(`plan["version"] = %#v, want "v1.1"`, plan["version"])
	}
	if plan["mode"] != "plan" {
		t.Fatalf(`plan["mode"] = %#v, want "plan"`, plan["mode"])
	}

	dataSource, ok := plan["data_source"].(map[string]any)
	if !ok {
		t.Fatalf("plan[data_source] not a map: %#v", plan["data_source"])
	}

	storage, ok := dataSource["storage"].(map[string]any)
	if !ok {
		t.Fatalf("data_source[storage] not a map: %#v", dataSource["storage"])
	}
	if storage["ref"] != "devops/devops.prometheus.core" {
		t.Fatalf("storage[ref] = %#v, want devops/devops.prometheus.core", storage["ref"])
	}
	if storage["type"] != "prometheus" {
		t.Fatalf("storage[type] = %#v, want prometheus", storage["type"])
	}
	if _, present := storage["config"]; present {
		t.Fatalf("storage[config] should be omitted in agent format (folded by default); got %#v", storage["config"])
	}
	if _, present := storage["domain"]; present {
		t.Fatalf("storage[domain] should be omitted in agent format (folded into ref); got %#v", storage["domain"])
	}

	dataSet, ok := dataSource["data_set"].(map[string]any)
	if !ok {
		t.Fatalf("data_source[data_set] not a map: %#v", dataSource["data_set"])
	}
	if dataSet["ref"] != "devops/devops.metric.service" {
		t.Fatalf("data_set[ref] = %#v, want devops/devops.metric.service", dataSet["ref"])
	}
	if dataSet["kind"] != "metric_set" {
		t.Fatalf("data_set[kind] = %#v, want metric_set", dataSet["kind"])
	}

	dataLink, ok := dataSource["data_link"].(map[string]any)
	if !ok {
		t.Fatalf("data_source[data_link] not a map: %#v", dataSource["data_link"])
	}
	if _, present := dataLink["spec"]; present {
		t.Fatalf("data_link[spec] should be omitted in agent format (folded by default)")
	}

	storageLink, ok := dataSource["storage_link"].(map[string]any)
	if !ok {
		t.Fatalf("data_source[storage_link] not a map: %#v", dataSource["storage_link"])
	}
	if _, present := storageLink["spec"]; present {
		t.Fatalf("storage_link[spec] should be omitted in agent format (folded by default)")
	}
}

// TestGetLogsAgentFormatPlanShape verifies the same envelope for get_logs.
func TestGetLogsAgentFormatPlanShape(t *testing.T) {
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
		Query:  ".entity_set with(domain='devops', name='devops.service') | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')",
		Format: model.FormatAgent,
	})
	if err != nil {
		t.Fatalf("execute get_logs agent: %v", err)
	}

	plan := model.AgentPlanPayload(result)
	if plan == nil {
		t.Fatalf("AgentPlanPayload returned nil; result=%+v", result)
	}
	if plan["version"] != "v1.1" {
		t.Fatalf(`plan["version"] = %#v, want "v1.1"`, plan["version"])
	}

	dataSource, _ := plan["data_source"].(map[string]any)
	storage, _ := dataSource["storage"].(map[string]any)
	if _, present := storage["config"]; present {
		t.Fatalf("storage[config] should be omitted in agent format")
	}
	if storage["type"] != "elasticsearch" {
		t.Fatalf("storage[type] = %#v, want elasticsearch", storage["type"])
	}
}

// TestAgentFormatPreservesV1AdditiveFields verifies that the agent envelope
// keeps all the v1 additive fields (description / next_action / source_query /
// params_echo / query / time_range) — switching to agent format is about
// shape, not content.
func TestAgentFormatPreservesV1AdditiveFields(t *testing.T) {
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
	result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: spl, Format: model.FormatAgent})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	plan := model.AgentPlanPayload(result)
	if plan == nil {
		t.Fatalf("AgentPlanPayload nil")
	}

	for _, k := range []string{"description", "next_action", "source_query", "params_echo", "query"} {
		if _, present := plan[k]; !present {
			t.Fatalf("agent plan missing %q (must be preserved across envelopes)", k)
		}
	}
	if plan["source_query"] != spl {
		t.Fatalf("plan[source_query] = %#v, want %q", plan["source_query"], spl)
	}
}

// TestGetMetricsAgentFormatIncludeSpec verifies that IncludeSpec=true
// re-expands the folded data_source fields so a debugging or diagnostic agent
// can see the full storage config, data_link spec, and storage_link spec
// without having to round-trip through a separate model lookup.
func TestGetMetricsAgentFormatIncludeSpec(t *testing.T) {
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
		Query:       ".entity_set with(domain='devops', name='devops.service', ids=['svc-1']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')",
		Format:      model.FormatAgent,
		IncludeSpec: true,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	plan := model.AgentPlanPayload(result)
	if plan == nil {
		t.Fatalf("AgentPlanPayload nil; result=%+v", result)
	}
	dataSource, _ := plan["data_source"].(map[string]any)

	storage, _ := dataSource["storage"].(map[string]any)
	if storage["ref"] != "devops/devops.prometheus.core" {
		t.Fatalf("storage[ref] = %#v, want devops/devops.prometheus.core", storage["ref"])
	}
	config, ok := storage["config"].(map[string]any)
	if !ok || len(config) == 0 {
		t.Fatalf("with IncludeSpec, storage[config] should hold the full storage spec; got %#v", storage["config"])
	}

	dataLink, _ := dataSource["data_link"].(map[string]any)
	if _, present := dataLink["spec"]; !present {
		t.Fatalf("with IncludeSpec, data_link[spec] should be populated; got %+v", dataLink)
	}

	storageLink, _ := dataSource["storage_link"].(map[string]any)
	if _, present := storageLink["spec"]; !present {
		t.Fatalf("with IncludeSpec, storage_link[spec] should be populated; got %+v", storageLink)
	}
}

// TestGetLogsAgentFormatIncludeSpec is the get_logs counterpart.
func TestGetLogsAgentFormatIncludeSpec(t *testing.T) {
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
		Query:       ".entity_set with(domain='devops', name='devops.service') | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')",
		Format:      model.FormatAgent,
		IncludeSpec: true,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	plan := model.AgentPlanPayload(result)
	dataSource, _ := plan["data_source"].(map[string]any)
	storage, _ := dataSource["storage"].(map[string]any)
	if _, present := storage["config"]; !present {
		t.Fatalf("with IncludeSpec, storage[config] should be populated for get_logs; got %+v", storage)
	}
}

// TestClassicFormatUnchangedByV1Point1 verifies that the v1.1 agent envelope
// work does not break the classic (default) envelope: version is still "v1"
// and data_source.storage.config is still the full spec.
func TestClassicFormatUnchangedByV1Point1(t *testing.T) {
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
		// Format omitted → FormatAssistant (default)
	})
	if err != nil {
		t.Fatalf("execute classic: %v", err)
	}

	if model.IsAgentPlanResult(result) {
		t.Fatalf("classic format should not be detected as agent payload")
	}
	plan := unmarshalPlan(t, result.Rows[0]["query"])
	if plan["version"] != "v1" {
		t.Fatalf(`classic plan["version"] = %#v, want "v1" (v1.1 must not leak into classic envelope)`, plan["version"])
	}
	dataSource, _ := plan["data_source"].(map[string]any)
	storage, _ := dataSource["storage"].(map[string]any)
	if _, present := storage["config"]; !present {
		t.Fatalf("classic storage[config] must remain populated (v1.1 fold is agent-only)")
	}
	if storage["domain"] != "devops" {
		t.Fatalf("classic storage[domain] = %#v, want devops (no ref-folding in classic)", storage["domain"])
	}
}
