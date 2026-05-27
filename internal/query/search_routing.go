package query

import (
	"context"
	"sort"
	"strings"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

const (
	searchModeKeyword = "keyword"
	searchModeVector  = "vector"
	searchModeHybrid  = "hyper"
)

// searchService is the subset of contract.SearchService that query.Service
// depends on. Declaring it here keeps the package self-contained and lets
// tests fake the search path without pulling in pkg/contract.
type searchService interface {
	Keyword(ctx context.Context, workspace string, req model.SearchRequest) (model.SearchResult, error)
	Vector(ctx context.Context, workspace string, req model.SearchRequest) (model.SearchResult, error)
	Hybrid(ctx context.Context, workspace string, req model.SearchRequest) (model.SearchResult, error)
	Capabilities(ctx context.Context) (model.SearchCapabilities, error)
	Health(ctx context.Context) (model.SearchHealth, error)
}

// shouldRouteToSearch reports whether the plan must be executed by the
// SearchService instead of the graph store. .runbook_set always routes;
// the other sources route only when the user picked a search mode.
func shouldRouteToSearch(plan model.QueryPlan) bool {
	if plan.Source == ".runbook_set" {
		return true
	}
	switch normalizeSearchMode(plan.Filters["mode"]) {
	case searchModeKeyword, searchModeVector, searchModeHybrid:
		return true
	}
	return false
}

func normalizeSearchMode(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(typed))
	default:
		return ""
	}
}

// buildSearchRequest projects a QueryPlan into the public SearchRequest shape.
// Keys lifted from filters: domain / name(s) / kind(s) / query / origin /
// embedding_model / hybrid_k. Remaining unknown keys are forwarded under
// Filters so providers can use them for predicate pushdown.
func buildSearchRequest(workspace string, plan model.QueryPlan) model.SearchRequest {
	req := model.SearchRequest{
		Workspace: workspace,
		Source:    plan.Source,
		TopK:      plan.TopK,
	}
	if req.TopK <= 0 {
		req.TopK = plan.Limit
	}

	if d := stringFilter(plan.Filters["domain"]); d != "" {
		req.Domain = d
	}
	req.Names = collectStrings(plan.Filters["name"], plan.Filters["names"])
	req.Kinds = collectStrings(plan.Filters["kind"], plan.Filters["kinds"])
	if q := stringFilter(plan.Filters["query"]); q != "" {
		req.Query = q
	}
	if origin := stringFilter(plan.Filters["origin"]); origin != "" {
		req.Origin = origin
	}
	if em := stringFilter(plan.Filters["embedding_model"]); em != "" {
		req.EmbedModel = em
	}
	if hk := intFilter(plan.Filters["hybrid_k"]); hk > 0 {
		req.HybridK = hk
	}

	known := map[string]struct{}{
		"mode": {}, "topk": {}, "domain": {}, "name": {}, "names": {},
		"kind": {}, "kinds": {}, "query": {}, "origin": {},
		"embedding_model": {}, "hybrid_k": {},
	}
	for k, v := range plan.Filters {
		if _, ok := known[k]; ok {
			continue
		}
		if req.Filters == nil {
			req.Filters = map[string]any{}
		}
		req.Filters[k] = v
	}
	return req
}

func collectStrings(values ...any) []string {
	var out []string
	for _, v := range values {
		switch typed := v.(type) {
		case string:
			if typed != "" {
				out = append(out, typed)
			}
		case []string:
			out = append(out, typed...)
		case []any:
			for _, item := range typed {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// dispatchSearch runs the SearchService for plans selected by
// shouldRouteToSearch and returns a QueryResult compatible with the existing
// REST surface.
func dispatchSearch(ctx context.Context, svc searchService, workspace string, plan model.QueryPlan) (model.SearchResult, string, error) {
	if svc == nil {
		return model.SearchResult{}, "", apperrors.New(apperrors.CodeProviderUnsupported, "search service is not configured for this query")
	}
	req := buildSearchRequest(workspace, plan)
	mode := normalizeSearchMode(plan.Filters["mode"])
	if mode == "" {
		mode = searchModeKeyword
	}
	switch mode {
	case searchModeVector:
		res, err := svc.Vector(ctx, workspace, req)
		return res, mode, err
	case searchModeHybrid:
		res, err := svc.Hybrid(ctx, workspace, req)
		return res, mode, err
	default:
		res, err := svc.Keyword(ctx, workspace, req)
		return res, mode, err
	}
}

func searchResultToQueryResult(res model.SearchResult, source string, limit int) model.QueryResult {
	if source == ".entity" {
		return entitySearchResultToQueryResult(res, limit)
	}
	rows := make([]map[string]any, 0, len(res.Rows))
	for _, row := range res.Rows {
		rows = append(rows, row.AsMap())
	}
	return model.QueryResult{
		Columns: []string{"__type__", "__domain__", "kind", "name", "metadata", "spec", "__score__"},
		Rows:    rows,
		Page:    model.PageRequest{Limit: limit},
	}
}

func entitySearchResultToQueryResult(res model.SearchResult, limit int) model.QueryResult {
	rows := make([]map[string]any, 0, len(res.Rows))
	baseColumns := []string{
		"__category__",
		"__domain__",
		"__entity_type__",
		"__entity_id__",
		"__method__",
		"__first_observed_time__",
		"__last_observed_time__",
		"__keep_alive_seconds__",
		"__deleted__",
	}
	seen := map[string]struct{}{}
	for _, column := range baseColumns {
		seen[column] = struct{}{}
	}
	extra := map[string]struct{}{}

	for _, searchRow := range res.Rows {
		row := make(map[string]any, len(searchRow.Spec)+4)
		for key, value := range searchRow.Spec {
			row[key] = value
			if _, ok := seen[key]; !ok && key != "__score__" {
				extra[key] = struct{}{}
			}
		}
		if _, ok := row["__domain__"]; !ok && searchRow.Domain != "" {
			row["__domain__"] = searchRow.Domain
		}
		if _, ok := row["__entity_type__"]; !ok && searchRow.Kind != "" {
			row["__entity_type__"] = searchRow.Kind
		}
		if _, ok := row["__deleted__"]; !ok {
			row["__deleted__"] = false
		}
		row["__score__"] = searchRow.Score
		rows = append(rows, row)
	}

	extraColumns := make([]string, 0, len(extra))
	for column := range extra {
		extraColumns = append(extraColumns, column)
	}
	sort.Strings(extraColumns)
	columns := append(append([]string(nil), baseColumns...), extraColumns...)
	columns = append(columns, "__score__")

	return model.QueryResult{
		Columns: columns,
		Rows:    rows,
		Page:    model.PageRequest{Limit: limit},
	}
}
