package search

import (
	"context"
	"sort"
	"strings"
	"sync"

	searchcontract "github.com/alibaba/UnifiedModel/internal/search/contract"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

const memoryProviderName = "memory"

// MemoryProvider is the in-process stub Provider used by the default build
// and unit tests. Keyword retrieval is substring matching against
// Chunk.Text; vector retrieval reuses a deterministic token-overlap score so
// vector-mode wiring works end-to-end without a real ANN index.
type MemoryProvider struct {
	mu         sync.RWMutex
	workspaces map[string]map[string]searchcontract.Chunk
}

func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		workspaces: map[string]map[string]searchcontract.Chunk{},
	}
}

func (m *MemoryProvider) OpenWorkspace(ctx context.Context, ws string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workspaces[ws]; !ok {
		m.workspaces[ws] = map[string]searchcontract.Chunk{}
	}
	return nil
}

func (m *MemoryProvider) IndexChunks(ctx context.Context, ws string, chunks []searchcontract.Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workspaces[ws]; !ok {
		m.workspaces[ws] = map[string]searchcontract.Chunk{}
	}
	for _, c := range chunks {
		if c.DocID == "" {
			continue
		}
		m.workspaces[ws][c.DocID] = c
	}
	return nil
}

func (m *MemoryProvider) DeleteByDocID(ctx context.Context, ws string, docIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	bucket := m.workspaces[ws]
	for _, id := range docIDs {
		delete(bucket, id)
	}
	return nil
}

func (m *MemoryProvider) Keyword(ctx context.Context, q searchcontract.Query) ([]searchcontract.Hit, error) {
	return m.search(q, false), nil
}

func (m *MemoryProvider) Vector(ctx context.Context, q searchcontract.Query) ([]searchcontract.Hit, error) {
	return m.search(q, true), nil
}

func (m *MemoryProvider) search(q searchcontract.Query, vector bool) []searchcontract.Hit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket := m.workspaces[q.Workspace]
	needle := strings.ToLower(q.Text)
	hits := make([]searchcontract.Hit, 0, len(bucket))
	for _, c := range bucket {
		if !matchesFilters(c, q) {
			continue
		}
		score := scoreChunk(c, needle, vector)
		if score <= 0 {
			continue
		}
		hits = append(hits, searchcontract.Hit{
			DocID:    c.DocID,
			Score:    score,
			Type:     typeFromChunk(c),
			Kind:     c.Kind,
			Domain:   c.Domain,
			Name:     c.Name,
			Metadata: c.Metadata,
			Spec:     c.Spec,
		})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].DocID < hits[j].DocID
		}
		return hits[i].Score > hits[j].Score
	})
	if q.TopK > 0 && len(hits) > q.TopK {
		hits = hits[:q.TopK]
	}
	return hits
}

func (m *MemoryProvider) Capabilities() model.SearchCapabilities {
	return model.SearchCapabilities{
		VectorSearch:         true,
		HybridSearch:         false,
		FilteredVectorSearch: true,
		RRF:                  false,
		EmbedderType:         "stub",
	}
}

func (m *MemoryProvider) Health() model.SearchHealth {
	return model.SearchHealth{Provider: memoryProviderName, Status: "ok"}
}

func (m *MemoryProvider) Close() error { return nil }

func matchesFilters(c searchcontract.Chunk, q searchcontract.Query) bool {
	if q.Source != "" && c.Source != q.Source {
		return false
	}
	if q.Domain != "" && c.Domain != q.Domain {
		return false
	}
	if len(q.Names) > 0 && !containsString(q.Names, c.Name) {
		return false
	}
	if len(q.Kinds) > 0 && !containsString(q.Kinds, c.Kind) {
		return false
	}
	if q.Type != "" {
		if attr, _ := c.Attrs["type"].(string); attr != "" && attr != q.Type {
			return false
		}
	}
	if q.Predicate != nil && !q.Predicate(c.Attrs) {
		return false
	}
	return true
}

func scoreChunk(c searchcontract.Chunk, needleLower string, vector bool) float64 {
	if needleLower == "" {
		return 0
	}
	corpus := strings.ToLower(c.Text)
	if !vector {
		idx := strings.Index(corpus, needleLower)
		if idx < 0 {
			return 0
		}
		return 1.0 - float64(idx)/float64(len(c.Text)+1)
	}
	tokens := strings.Fields(needleLower)
	if len(tokens) == 0 {
		return 0
	}
	hit := 0
	for _, tok := range tokens {
		if strings.Contains(corpus, tok) {
			hit++
		}
	}
	return float64(hit) / float64(len(tokens))
}

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func typeFromChunk(c searchcontract.Chunk) string {
	if c.Name != "" {
		return c.Name
	}
	return c.Kind
}
