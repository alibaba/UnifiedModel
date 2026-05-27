// Package search hosts the runtime SearchService implementation that backs
// the `.umodel`, `.entity`, and `.runbook_set` query sources. The package
// composes a Provider (keyword + vector retrieval) and an Embedder, and
// implements RRF fusion in-tree for hybrid mode.
package search

import (
	"context"
	"fmt"
	"sort"

	searchcontract "github.com/alibaba/UnifiedModel/internal/search/contract"
	"github.com/alibaba/UnifiedModel/internal/search/embed"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

const (
	defaultTopK    = 10
	defaultHybridK = 60
)

// Service is the runtime SearchService that wires a Provider and an Embedder
// together and implements pkg/contract.SearchService.
type Service struct {
	provider     searchcontract.Provider
	embedder     searchcontract.Embedder
	providerName string
}

// NewService builds a Service from an already-constructed Provider and
// Embedder. Pass embed.Noop{} if no embedding stack is configured; the stub
// memory provider ignores the produced vector anyway.
func NewService(provider searchcontract.Provider, embedder searchcontract.Embedder, providerName string) *Service {
	if embedder == nil {
		embedder = embed.Noop{}
	}
	if providerName == "" {
		providerName = "memory"
	}
	return &Service{provider: provider, embedder: embedder, providerName: providerName}
}

// OpenWorkspace forwards to the underlying Provider so callers can prepare a
// workspace before issuing any IndexChunks/Search calls.
func (s *Service) OpenWorkspace(ctx context.Context, ws string) error {
	return s.provider.OpenWorkspace(ctx, ws)
}

// Index forwards a batch of chunks into the underlying Provider. The entity
// and umodel write paths call this on every mutation.
func (s *Service) Index(ctx context.Context, ws string, chunks []searchcontract.Chunk) error {
	return s.provider.IndexChunks(ctx, ws, chunks)
}

// DeleteByDocID forwards a tombstone batch.
func (s *Service) DeleteByDocID(ctx context.Context, ws string, docIDs []string) error {
	return s.provider.DeleteByDocID(ctx, ws, docIDs)
}

func (s *Service) Keyword(ctx context.Context, workspace string, req model.SearchRequest) (model.SearchResult, error) {
	q := s.buildQuery(workspace, req, nil)
	hits, err := s.provider.Keyword(ctx, q)
	if err != nil {
		return model.SearchResult{}, fmt.Errorf("search keyword: %w", err)
	}
	return s.rowsFromHits(hits, q.TopK), nil
}

func (s *Service) Vector(ctx context.Context, workspace string, req model.SearchRequest) (model.SearchResult, error) {
	vec, err := s.embedQuery(ctx, req.Query)
	if err != nil {
		return model.SearchResult{}, err
	}
	q := s.buildQuery(workspace, req, vec)
	hits, err := s.provider.Vector(ctx, q)
	if err != nil {
		return model.SearchResult{}, fmt.Errorf("search vector: %w", err)
	}
	return s.rowsFromHits(hits, q.TopK), nil
}

func (s *Service) Hybrid(ctx context.Context, workspace string, req model.SearchRequest) (model.SearchResult, error) {
	vec, err := s.embedQuery(ctx, req.Query)
	if err != nil {
		return model.SearchResult{}, err
	}
	q := s.buildQuery(workspace, req, vec)

	kwHits, err := s.provider.Keyword(ctx, q)
	if err != nil {
		return model.SearchResult{}, fmt.Errorf("search hybrid keyword: %w", err)
	}
	vecHits, err := s.provider.Vector(ctx, q)
	if err != nil {
		return model.SearchResult{}, fmt.Errorf("search hybrid vector: %w", err)
	}

	k := req.HybridK
	if k <= 0 {
		k = defaultHybridK
	}
	wKw, wVec := weights(req.Weights)
	fused := rrf(kwHits, vecHits, k, wKw, wVec)
	return s.rowsFromHits(fused, q.TopK), nil
}

func (s *Service) Capabilities(ctx context.Context) (model.SearchCapabilities, error) {
	caps := s.provider.Capabilities()
	caps.HybridSearch = true
	caps.RRF = true
	if caps.EmbedderType == "" {
		caps.EmbedderType = s.embedder.Model()
	}
	if caps.MaxDim == 0 {
		caps.MaxDim = s.embedder.Dim()
	}
	return caps, nil
}

func (s *Service) Health(ctx context.Context) (model.SearchHealth, error) {
	return s.provider.Health(), nil
}

func (s *Service) embedQuery(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, nil
	}
	vecs, err := s.embedder.Embed(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	return vecs[0], nil
}

func (s *Service) buildQuery(workspace string, req model.SearchRequest, vec []float32) searchcontract.Query {
	topK := req.TopK
	if topK <= 0 {
		topK = defaultTopK
	}
	q := searchcontract.Query{
		Workspace: workspace,
		Source:    req.Source,
		Domain:    req.Domain,
		Names:     req.Names,
		Kinds:     req.Kinds,
		Text:      req.Query,
		Vector:    vec,
		TopK:      topK,
	}
	if t, ok := req.Filters["type"].(string); ok {
		q.Type = t
	}
	return q
}

func (s *Service) rowsFromHits(hits []searchcontract.Hit, topK int) model.SearchResult {
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	rows := make([]model.SearchRow, 0, len(hits))
	for _, h := range hits {
		rows = append(rows, model.SearchRow{
			Type:       h.Type,
			Domain:     h.Domain,
			Kind:       h.Kind,
			Name:       h.Name,
			Metadata:   h.Metadata,
			Spec:       h.Spec,
			Score:      h.Score,
			Provider:   s.providerName,
			EmbedModel: s.embedder.Model(),
		})
	}
	return model.SearchResult{Rows: rows}
}

func weights(w map[string]float64) (float64, float64) {
	kw, vec := 1.0, 1.0
	if v, ok := w["keyword"]; ok {
		kw = v
	}
	if v, ok := w["vector"]; ok {
		vec = v
	}
	return kw, vec
}

// rrf fuses two ranked hit lists with Reciprocal Rank Fusion. Each axis
// contributes `weight / (k + rank)` to the doc's score; rank is 1-based per
// the original RRF paper.
func rrf(kwHits, vecHits []searchcontract.Hit, k int, wKw, wVec float64) []searchcontract.Hit {
	score := map[string]float64{}
	merged := map[string]searchcontract.Hit{}

	for rank, h := range kwHits {
		score[h.DocID] += wKw / float64(k+rank+1)
		merged[h.DocID] = h
	}
	for rank, h := range vecHits {
		score[h.DocID] += wVec / float64(k+rank+1)
		if _, ok := merged[h.DocID]; !ok {
			merged[h.DocID] = h
		}
	}

	out := make([]searchcontract.Hit, 0, len(score))
	for id, s := range score {
		h := merged[id]
		h.Score = s
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].DocID < out[j].DocID
		}
		return out[i].Score > out[j].Score
	})
	return out
}
