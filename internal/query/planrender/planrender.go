// Package planrender defines the pluggable contract for rendering an EntitySet
// method (get_metrics / get_logs) into a storage-native query block.
//
// The open-source Query Service returns a *plan* — it never connects to or
// executes against the backing storage. Rendering is therefore pure
// string/map construction with no heavy dependencies (no storage drivers, no
// build tags). A Renderer turns a normalized Request into the "query" block
// embedded in the plan envelope. The Registry maps (family, method) to a
// Renderer; the query package bridges a storage kind to a family with a small
// default map, and a storage may override that by declaring spec.family — so a
// new backend that shares an existing query family needs no Go code, only
// configuration. This replaces the hardcoded switch on storage.Kind that used
// to live in the executor, mirroring how GraphStore providers sit behind a
// contract.
package planrender

import "github.com/alibaba/UnifiedModel/pkg/model"

// Method is the EntitySet entity-call whose storage query is being rendered.
type Method string

const (
	MethodGetMetrics Method = "get_metrics"
	MethodGetLogs    Method = "get_logs"
)

// Request is the normalized input a Renderer needs to build a storage query.
// It is the superset of the get_metrics and get_logs inputs; log rendering
// leaves Metrics / QueryType / Step zero.
type Request struct {
	Method             Method
	DataSet            model.UModelElement // the metric_set or log_set
	Storage            model.UModelElement
	DataLinkMapping    map[string]any // entity field -> dataset field (fields_mapping)
	StorageLinkMapping map[string]any // dataset field -> storage field (fields_mapping)
	EntityIDs          []string
	EntityQuery        string
	DataFilter         string
	MethodQuery        string
	EntityData         *model.EntityData
	Metrics            []map[string]any // get_metrics only
	QueryType          string           // get_metrics only
	Step               string           // get_metrics only
	Limit              int
}

// Renderer renders one storage family's native query block. Renderers are pure
// (no I/O, no storage connection): they only construct the query a downstream
// executor will run.
type Renderer interface {
	// Family is the query-model archetype this renderer implements (e.g.
	// "label-timeseries", "document-search"). Storages route to a renderer by
	// family, so multiple storage kinds may share one renderer.
	Family() string
	// SupportsMethod reports whether this renderer's family handles the given
	// method (a metrics family renders get_metrics, a log family get_logs).
	SupportsMethod(m Method) bool
	// Render builds the storage-native query block (the plan's "query" field).
	Render(req Request) (map[string]any, error)
}

// Registry maps (storage kind, method) to a Renderer. It is constructed
// explicitly and injected into the executor — there is no global registry.
type Registry struct {
	renderers []Renderer
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a renderer to the registry. Later registrations take precedence
// on overlapping (family, method), so register built-ins first and any
// overrides after.
func (r *Registry) Register(rd Renderer) { r.renderers = append(r.renderers, rd) }

// Find returns the renderer for the given family that supports method m. An
// empty family never matches, routing to the executor's passthrough.
// Later-registered renderers win, so an override registered after the built-ins
// shadows them.
func (r *Registry) Find(family string, m Method) (Renderer, bool) {
	if family == "" {
		return nil, false
	}
	for i := len(r.renderers) - 1; i >= 0; i-- {
		if r.renderers[i].Family() == family && r.renderers[i].SupportsMethod(m) {
			return r.renderers[i], true
		}
	}
	return nil, false
}
