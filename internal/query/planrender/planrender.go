// Package planrender defines the pluggable contract for rendering an EntitySet
// method (get_metrics / get_logs) into a storage-native query block.
//
// The open-source Query Service returns a *plan* — it never connects to or
// executes against the backing storage. Rendering is therefore pure
// string/map construction with no heavy dependencies (no storage drivers, no
// build tags). A Renderer turns a normalized Request into the "query" block
// embedded in the plan envelope; the Registry maps (storage kind, method) to a
// Renderer, replacing the hardcoded switch on storage.Kind that used to live in
// the executor — mirroring how GraphStore providers sit behind a contract.
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
	// "label-timeseries", "document-search"). Multiple storage kinds may share
	// one family.
	Family() string
	// Handles reports whether this renderer can render the given storage kind
	// and method.
	Handles(storageKind string, m Method) bool
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
// on overlapping (kind, method), so register built-ins first and any overrides
// after.
func (r *Registry) Register(rd Renderer) { r.renderers = append(r.renderers, rd) }

// Find returns the renderer that Handles the given storage kind and method.
// Later-registered renderers win, so an override registered after the built-ins
// shadows them.
func (r *Registry) Find(storageKind string, m Method) (Renderer, bool) {
	for i := len(r.renderers) - 1; i >= 0; i-- {
		if r.renderers[i].Handles(storageKind, m) {
			return r.renderers[i], true
		}
	}
	return nil, false
}
