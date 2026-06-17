package query

import "github.com/alibaba/UnifiedModel/internal/query/planrender"

// labelTimeseriesRenderer renders PromQL-over-HTTP metric backends (Prometheus
// and API-compatible stores). It wraps the existing prometheusMetricQuery
// unchanged; the family name groups all PromQL-speaking kinds.
type labelTimeseriesRenderer struct{}

func (labelTimeseriesRenderer) Family() string { return "label-timeseries" }

func (labelTimeseriesRenderer) Handles(storageKind string, m planrender.Method) bool {
	return m == planrender.MethodGetMetrics &&
		(storageKind == "prometheus" || storageKind == "aliyun_prometheus")
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

func (documentSearchRenderer) Handles(storageKind string, m planrender.Method) bool {
	return m == planrender.MethodGetLogs && storageKind == "elasticsearch"
}

func (documentSearchRenderer) Render(req planrender.Request) (map[string]any, error) {
	return elasticsearchLogQuery(req.DataSet, req.Storage, req.DataLinkMapping, req.StorageLinkMapping,
		req.EntityIDs, req.EntityQuery, req.DataFilter, req.MethodQuery, req.EntityData, req.Limit), nil
}

// newDefaultRegistry returns a registry pre-loaded with the built-in family
// renderers.
func newDefaultRegistry() *planrender.Registry {
	r := planrender.NewRegistry()
	r.Register(labelTimeseriesRenderer{})
	r.Register(documentSearchRenderer{})
	return r
}
