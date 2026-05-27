package query

import (
	"context"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type fakeSearch struct {
	lastReq  model.SearchRequest
	lastMode string
	rows     []model.SearchRow
}

func (f *fakeSearch) Keyword(ctx context.Context, ws string, req model.SearchRequest) (model.SearchResult, error) {
	f.lastReq, f.lastMode = req, "keyword"
	return model.SearchResult{Rows: f.rows}, nil
}
func (f *fakeSearch) Vector(ctx context.Context, ws string, req model.SearchRequest) (model.SearchResult, error) {
	f.lastReq, f.lastMode = req, "vector"
	return model.SearchResult{Rows: f.rows}, nil
}
func (f *fakeSearch) Hybrid(ctx context.Context, ws string, req model.SearchRequest) (model.SearchResult, error) {
	f.lastReq, f.lastMode = req, "hyper"
	return model.SearchResult{Rows: f.rows}, nil
}
func (f *fakeSearch) Capabilities(ctx context.Context) (model.SearchCapabilities, error) {
	return model.SearchCapabilities{VectorSearch: true, HybridSearch: true, RRF: true, EmbedderType: "noop"}, nil
}
func (f *fakeSearch) Health(ctx context.Context) (model.SearchHealth, error) {
	return model.SearchHealth{Provider: "memory", Status: "ok"}, nil
}

func TestParseAcceptsRunbookSet(t *testing.T) {
	plan, err := Parse(model.QueryRequest{Query: ".runbook_set with(domain='apm', type='knowledge', query='slow request', mode='hyper', topk=5)"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.Source != ".runbook_set" {
		t.Fatalf("source = %q, want .runbook_set", plan.Source)
	}
	if plan.Filters["type"] != "knowledge" {
		t.Fatalf("filters[type] = %v, want knowledge", plan.Filters["type"])
	}
	if plan.Filters["mode"] != "hyper" {
		t.Fatalf("filters[mode] = %v, want hyper", plan.Filters["mode"])
	}
	if plan.TopK != 5 {
		t.Fatalf("topk = %d, want 5", plan.TopK)
	}
}

func TestExecuteRoutesRunbookSetToSearch(t *testing.T) {
	ctx := context.Background()
	fake := &fakeSearch{rows: []model.SearchRow{{
		Type:   "apm.runbook_set",
		Domain: "apm",
		Kind:   "runbook_set",
		Name:   "slow-request",
		Score:  0.42,
	}}}
	svc := NewServiceWithSearch(graphstore.NewMemoryStore(), fake)

	result, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".runbook_set with(domain='apm', type='knowledge', query='slow request', mode='hyper', topk=5)",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fake.lastMode != "hyper" {
		t.Fatalf("dispatched mode = %q, want hyper", fake.lastMode)
	}
	if fake.lastReq.Source != ".runbook_set" || fake.lastReq.Domain != "apm" {
		t.Fatalf("search request misprojected: %+v", fake.lastReq)
	}
	if fake.lastReq.TopK != 5 {
		t.Fatalf("topk lost: %d", fake.lastReq.TopK)
	}
	if got := fake.lastReq.Filters["type"]; got != "knowledge" {
		t.Fatalf("filters[type] = %v, want knowledge", got)
	}
	if len(result.Rows) != 1 || result.Rows[0]["__score__"].(float64) != 0.42 {
		t.Fatalf("rows not projected as search rows: %+v", result.Rows)
	}
	if result.Explain == nil || result.Explain.SearchProvider != "memory" || result.Explain.EmbedModel != "noop" || result.Explain.SearchMode != "hyper" {
		t.Fatalf("explain missing search annotations: %+v", result.Explain)
	}
}

func TestExecuteRoutesEntityVectorMode(t *testing.T) {
	ctx := context.Background()
	fake := &fakeSearch{rows: []model.SearchRow{{
		Domain: "apm",
		Kind:   "apm.service",
		Name:   "apm.service",
		Spec: map[string]any{
			"__category__":            "entity",
			"__domain__":              "apm",
			"__entity_type__":         "apm.service",
			"__entity_id__":           "10000000000000000000000000000101",
			"__method__":              "Update",
			"__first_observed_time__": int64(1704067200),
			"__last_observed_time__":  int64(4102444800),
			"__keep_alive_seconds__":  int64(3600),
			"display_name":            "checkout-service",
			"status":                  "degraded",
		},
		Score: 0.9,
	}}}
	svc := NewServiceWithSearch(graphstore.NewMemoryStore(), fake)

	result, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity with(domain='apm', name='apm.service', query='checkout', mode='vector', topk=20)",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fake.lastMode != "vector" {
		t.Fatalf("dispatched mode = %q, want vector", fake.lastMode)
	}
	if len(fake.lastReq.Names) != 1 || fake.lastReq.Names[0] != "apm.service" {
		t.Fatalf("name not forwarded: %+v", fake.lastReq.Names)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected one result row: %+v", result.Rows)
	}
	row := result.Rows[0]
	if _, ok := row["spec"]; ok {
		t.Fatalf("entity search row should flatten payload instead of returning spec: %+v", row)
	}
	if row["display_name"] != "checkout-service" || row["status"] != "degraded" || row["__score__"] != 0.9 {
		t.Fatalf("entity payload was not flattened: %+v", row)
	}
	for _, column := range []string{"__category__", "__domain__", "__entity_type__", "__entity_id__", "__method__", "__first_observed_time__", "__last_observed_time__", "__keep_alive_seconds__", "__deleted__", "display_name", "status"} {
		if !containsColumn(result.Columns, column) {
			t.Fatalf("entity search columns should include %q, got %+v", column, result.Columns)
		}
	}
	if containsColumn(result.Columns, "spec") {
		t.Fatalf("entity search columns should expose flattened attributes, got %+v", result.Columns)
	}
}

func TestExecuteEntityKeywordModeStillRoutesToSearch(t *testing.T) {
	ctx := context.Background()
	fake := &fakeSearch{}
	svc := NewServiceWithSearch(graphstore.NewMemoryStore(), fake)

	if _, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity with(domain='apm', name='apm.service', query='checkout', mode='keyword')",
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fake.lastMode != "keyword" {
		t.Fatalf("dispatched mode = %q, want keyword", fake.lastMode)
	}
}

func TestExecuteEntityWithoutModeStaysOnGraphStore(t *testing.T) {
	ctx := context.Background()
	fake := &fakeSearch{}
	svc := NewServiceWithSearch(graphstore.NewMemoryStore(), fake)

	if _, err := svc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity with(domain='apm', name='apm.service', query='checkout')",
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fake.lastMode != "" {
		t.Fatalf("search service called for non-mode query: %q", fake.lastMode)
	}
}

func TestRunbookSetRequiresSearchService(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".runbook_set with(domain='apm', type='knowledge', query='x')",
	})
	if !apperrors.IsCode(err, apperrors.CodeProviderUnsupported) {
		t.Fatalf("want provider-unsupported error, got %v", err)
	}
}

func containsColumn(columns []string, want string) bool {
	for _, column := range columns {
		if column == want {
			return true
		}
	}
	return false
}
