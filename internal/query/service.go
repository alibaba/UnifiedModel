package query

import (
	"context"
	"errors"
	"time"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

// ModePlan is the only query mode supported by unified-model. umodel-assistant
// additionally supports "data". See docs/en/guides/agent-integration.md for the
// shared mode protocol.
const ModePlan = "plan"

// normalizeFormat applies the unified-model format policy: empty becomes the
// default assistant envelope, "agent" passes through, anything else is
// rejected. See model.QueryRequest.Format for the response shape difference.
func normalizeFormat(format string) (string, error) {
	switch format {
	case model.FormatAssistant, model.FormatAgent:
		return format, nil
	default:
		return "", apperrors.WithDetails(
			apperrors.CodeInvalidArgument,
			`unsupported format. unified-model supports format="" (default assistant envelope) or format="agent" (agent-native envelope).`,
			map[string]string{
				"requested_format":  format,
				"supported_formats": `"" (assistant), "agent"`,
			},
		)
	}
}

// normalizeMode applies the unified-model mode policy: empty becomes "plan",
// "plan" passes through, anything else is rejected. unified-model never
// executes plans against real storage.
//
// On rejection the error carries migration_* keys in Details so an AI agent
// (or any structured client) can act on the failure programmatically rather
// than parsing the message.
func normalizeMode(mode string) (string, error) {
	switch mode {
	case "", ModePlan:
		return ModePlan, nil
	default:
		return "", apperrors.WithDetails(
			apperrors.CodeNotImplemented,
			"unified-model only supports mode=plan. To execute queries against real storage, use umodel-assistant.",
			map[string]string{
				"requested_mode":     mode,
				"supported_modes":    ModePlan,
				"migration_service":  "umodel-assistant",
				"migration_action":   "switch_endpoint_to_umodel_assistant",
				"migration_docs_url": "https://github.com/alibaba/UnifiedModel/blob/main/docs/en/guides/agent-integration.md",
			},
		)
	}
}

type graphStore interface {
	GetUModelSnapshot(ctx context.Context, req model.UModelSnapshotRequest) (model.UModelSnapshot, error)
	QueryEntities(ctx context.Context, plan model.EntityQueryPlan) (model.QueryResult, error)
	QueryTopo(ctx context.Context, plan model.TopoQueryPlan) (model.QueryResult, error)
	Capabilities(ctx context.Context) (model.GraphStoreCapabilities, error)
	Health(ctx context.Context) (model.GraphStoreHealth, error)
}

type Service struct {
	graph    graphStore
	search   searchService
	planner  Planner
	executor *Executor
}

func NewService(graph graphStore) *Service {
	return NewServiceWithSearch(graph, nil)
}

// NewServiceWithSearch builds a Service that can route .runbook_set and any
// query carrying mode=keyword|vector|hyper to the supplied SearchService.
// Pass nil for legacy graph-only behavior.
func NewServiceWithSearch(graph graphStore, search searchService) *Service {
	return &Service{
		graph:    graph,
		search:   search,
		planner:  Planner{},
		executor: NewExecutor(graph),
	}
}

func (s *Service) Execute(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryResult, error) {
	mode, err := normalizeMode(req.Mode)
	if err != nil {
		return model.QueryResult{}, err
	}
	req.Mode = mode
	format, err := normalizeFormat(req.Format)
	if err != nil {
		return model.QueryResult{}, err
	}
	req.Format = format
	plan, caps, health, err := s.plan(ctx, workspace, req)
	if err != nil {
		return model.QueryResult{}, err
	}

	// Bound execution by the active provider's declared timeout. The deadline is
	// set here; the stores honor ctx and abort in-flight scans, and ctx errors
	// are mapped to CodeTimeout below.
	if d := parseQueryTimeout(caps.Timeout); d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}

	if shouldRouteToSearch(plan) {
		searchRes, mode, err := dispatchSearch(ctx, s.search, workspace, plan)
		if err != nil {
			return model.QueryResult{}, asQueryTimeout(ctx, err)
		}
		result := searchResultToQueryResult(searchRes, plan.Source, plan.Limit)
		explain := buildExplain(plan, caps, health)
		s.annotateSearchExplain(ctx, &explain, mode)
		result.Explain = &explain
		return result, nil
	}

	result, err := s.executor.Execute(ctx, workspace, plan)
	if err != nil {
		return model.QueryResult{}, asQueryTimeout(ctx, err)
	}
	explain := buildExplain(plan, caps, health)
	result.Explain = &explain
	return result, nil
}

// parseQueryTimeout parses a provider's declared timeout (e.g. "10s"). Empty or
// non-positive values disable the deadline.
func parseQueryTimeout(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}

// asQueryTimeout maps a query that exceeded the provider's deadline to a stable,
// machine-actionable CodeTimeout. Only deadline expiry counts: a cancelled
// context (client disconnect, parent cancellation) is not a provider timeout and
// must not be reported as one — that would be misleading and, since CodeTimeout
// is retryable, would wrongly mark a cancelled request as retryable. Cancellation
// and all other errors pass through unchanged.
func asQueryTimeout(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
		return apperrors.New(apperrors.CodeTimeout, "query exceeded the provider time limit")
	}
	return err
}

func (s *Service) Explain(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryExplain, error) {
	mode, err := normalizeMode(req.Mode)
	if err != nil {
		return model.QueryExplain{}, err
	}
	req.Mode = mode
	format, err := normalizeFormat(req.Format)
	if err != nil {
		return model.QueryExplain{}, err
	}
	req.Format = format
	plan, caps, health, err := s.plan(ctx, workspace, req)
	if err != nil {
		return model.QueryExplain{}, err
	}
	explain := buildExplain(plan, caps, health)
	if shouldRouteToSearch(plan) {
		mode := normalizeSearchMode(plan.Filters["mode"])
		if mode == "" {
			mode = searchModeKeyword
		}
		s.annotateSearchExplain(ctx, &explain, mode)
	}
	return explain, nil
}

func (s *Service) Examples(ctx context.Context) ([]string, error) {
	return []string{
		".umodel with(kind='entity_set') | project domain,name,kind | sort domain,name | limit 20",
		".entity with(domain='devops', name='devops.service', query='checkout', topk=20)",
		".entity with(domain='k8s', name='k8s.workload', query='checkout', topk=20)",
		".entity with(domain='apm', name='apm.service', query='payment latency spikes', mode='vector', topk=20)",
		".entity with(domain='apm', name='apm.service', query='checkout failure', mode='hyper', topk=20, hybrid_k=60)",
		".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()",
		".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)",
		".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')",
		".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')",
		".runbook_set with(domain='apm', type='knowledge', query='how to mitigate slow request', mode='hyper', topk=5)",
		".runbook_set with(domain='apm', type='observations', query='cache miss spike', mode='vector', topk=10)",
		".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20",
		".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20",
		".topo | graph-call cypher(`MATCH (src)-[r]->(dest) RETURN properties(src) AS src, properties(r) AS relation, properties(dest) AS dest LIMIT 20`)",
	}, nil
}

func (s *Service) annotateSearchExplain(ctx context.Context, explain *model.QueryExplain, mode string) {
	explain.SearchMode = mode
	if s.search == nil {
		return
	}
	if health, err := s.search.Health(ctx); err == nil {
		explain.SearchProvider = health.Provider
	}
	if caps, err := s.search.Capabilities(ctx); err == nil {
		explain.EmbedModel = caps.EmbedderType
	}
}

func (s *Service) plan(ctx context.Context, workspace string, req model.QueryRequest) (model.QueryPlan, model.GraphStoreCapabilities, model.GraphStoreHealth, error) {
	caps, err := s.graph.Capabilities(ctx)
	if err != nil {
		return model.QueryPlan{}, model.GraphStoreCapabilities{}, model.GraphStoreHealth{}, err
	}
	plan, err := s.planner.Plan(req, caps)
	if err != nil {
		return model.QueryPlan{}, model.GraphStoreCapabilities{}, model.GraphStoreHealth{}, err
	}
	plan.Workspace = workspace
	health, err := s.graph.Health(ctx)
	if err != nil {
		return model.QueryPlan{}, model.GraphStoreCapabilities{}, model.GraphStoreHealth{}, err
	}
	return plan, caps, health, nil
}
