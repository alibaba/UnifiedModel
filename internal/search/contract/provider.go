// Package searchcontract defines the internal Provider and Embedder
// interfaces that the search Service composes. It is intentionally scoped to
// internal/search and its sub-packages so external callers go through the
// public contract.SearchService surface.
package searchcontract

import (
	"context"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

// Chunk is the unit of indexable content. A single UModel object may yield
// multiple chunks (one per field, knowledge section, etc.).
type Chunk struct {
	DocID    string
	Source   string
	Kind     string
	Domain   string
	Name     string
	Text     string
	Vector   []float32
	Attrs    map[string]any
	Metadata map[string]any
	Spec     map[string]any
}

// Hit is a single retrieval result emitted by a Provider.
type Hit struct {
	DocID    string
	Score    float64
	Type     string
	Kind     string
	Domain   string
	Name     string
	Metadata map[string]any
	Spec     map[string]any
}

// Predicate is the filter-pushdown callback evaluated by the index against
// the chunk attribute bag.
type Predicate func(attrs map[string]any) bool

// Query is the internal request shape passed from Service to Provider.
type Query struct {
	Workspace string
	Source    string
	Domain    string
	Names     []string
	Kinds     []string
	Type      string
	Text      string
	Vector    []float32
	TopK      int
	Predicate Predicate
}

// Provider is the indexing + lookup backend that SearchService delegates to.
type Provider interface {
	OpenWorkspace(ctx context.Context, ws string) error
	IndexChunks(ctx context.Context, ws string, chunks []Chunk) error
	DeleteByDocID(ctx context.Context, ws string, docIDs []string) error
	Keyword(ctx context.Context, q Query) ([]Hit, error)
	Vector(ctx context.Context, q Query) ([]Hit, error)
	Capabilities() model.SearchCapabilities
	Health() model.SearchHealth
	Close() error
}

// Embedder turns query text into a vector used on the vector axis of search.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
	Model() string
}
