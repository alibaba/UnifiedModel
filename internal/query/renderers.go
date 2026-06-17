package query

import "github.com/alibaba/UnifiedModel/internal/query/planrender"

// labelTimeseriesRenderer renders PromQL-over-HTTP metric backends (Prometheus
// and API-compatible stores). It wraps the existing prometheusMetricQuery
// unchanged; the family name groups all PromQL-speaking kinds.
type labelTimeseriesRenderer struct{}

func (labelTimeseriesRenderer) Family() string { return "label-timeseries" }

func (labelTimeseriesRenderer) SupportsMethod(m planrender.Method) bool {
	return m == planrender.MethodGetMetrics
}

func (labelTimeseriesRenderer) Render(req planrender.Request) (map[string]any, error) {
	return prometheusMetricQuery(req.DataSet, req.Storage, req.DataLinkMapping, req.StorageLinkMapping,
		req.Metrics, req.EntityIDs, req.EntityQuery, req.DataFilter, req.MethodQuery, req.EntityData,
		req.QueryType, req.Step, req.Limit), nil
}

// documentSearchRenderer renders Elasticsearch Query DSL log backends. It wraps
// the existing elasticsearchLogQuery unchanged.
type documentSearchRenderer struct{}

func (documentSearchRenderer) Family() string { return "document-search" }

func (documentSearchRenderer) SupportsMethod(m planrender.Method) bool {
	return m == planrender.MethodGetLogs
}

func (documentSearchRenderer) Render(req planrender.Request) (map[string]any, error) {
	return elasticsearchLogQuery(req.DataSet, req.Storage, req.DataLinkMapping, req.StorageLinkMapping,
		req.EntityIDs, req.EntityQuery, req.DataFilter, req.MethodQuery, req.EntityData, req.Limit), nil
}

// defaultFamilyForKind maps a built-in storage kind to its query family, used
// when a storage does not declare spec.family. A storage whose kind is not
// listed here can still select a family by setting spec.family — that is how a
// new same-family backend (e.g. a Prometheus-compatible store) routes to an
// existing renderer with no Go code, only configuration.
func defaultFamilyForKind(kind string) string {
	switch kind {
	case "prometheus", "aliyun_prometheus":
		return "label-timeseries"
	case "elasticsearch":
		return "document-search"
	default:
		return ""
	}
}

// newDefaultRegistry returns a registry pre-loaded with the built-in family
// renderers.
func newDefaultRegistry() *planrender.Registry {
	r := planrender.NewRegistry()
	r.Register(labelTimeseriesRenderer{})
	r.Register(documentSearchRenderer{})
	r.Register(sqlTableRenderer{})
	return r
}
