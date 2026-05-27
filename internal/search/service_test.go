package search_test

import (
	"context"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/search"
	searchcontract "github.com/alibaba/UnifiedModel/internal/search/contract"
	"github.com/alibaba/UnifiedModel/internal/search/embed"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestService_ImplementsSearchServiceContract(t *testing.T) {
	var _ contract.SearchService = (*search.Service)(nil)
}

func TestService_KeywordVectorHybrid(t *testing.T) {
	ctx := context.Background()

	provider, err := search.NewProvider(search.ProviderConfig{Type: search.ProviderTypeMemory})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	svc := search.NewService(provider, embed.Noop{}, search.ProviderTypeMemory)

	const ws = "demo"
	if err := svc.OpenWorkspace(ctx, ws); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	chunks := []searchcontract.Chunk{
		{DocID: "a", Source: ".entity", Domain: "apm", Kind: "service", Name: "checkout", Text: "checkout service payment latency"},
		{DocID: "b", Source: ".entity", Domain: "apm", Kind: "service", Name: "cart", Text: "cart service add remove item"},
		{DocID: "c", Source: ".entity", Domain: "k8s", Kind: "pod", Name: "checkout-pod", Text: "checkout pod kubernetes"},
	}
	if err := svc.Index(ctx, ws, chunks); err != nil {
		t.Fatalf("index: %v", err)
	}

	t.Run("keyword scoped by domain", func(t *testing.T) {
		res, err := svc.Keyword(ctx, ws, model.SearchRequest{
			Source: ".entity",
			Domain: "apm",
			Query:  "checkout",
			TopK:   5,
		})
		if err != nil {
			t.Fatalf("keyword: %v", err)
		}
		if len(res.Rows) != 1 {
			t.Fatalf("want 1 row scoped to apm, got %d", len(res.Rows))
		}
		row := res.Rows[0]
		if row.Name != "checkout" {
			t.Fatalf("want name=checkout, got %q", row.Name)
		}
		if row.Provider != search.ProviderTypeMemory {
			t.Fatalf("want provider tag, got %q", row.Provider)
		}
		if row.EmbedModel == "" {
			t.Fatalf("want embed model populated")
		}
		if row.Score <= 0 {
			t.Fatalf("want positive score, got %v", row.Score)
		}
	})

	t.Run("vector matches token overlap", func(t *testing.T) {
		res, err := svc.Vector(ctx, ws, model.SearchRequest{
			Source: ".entity",
			Query:  "checkout payment",
			TopK:   5,
		})
		if err != nil {
			t.Fatalf("vector: %v", err)
		}
		if len(res.Rows) == 0 {
			t.Fatalf("want at least one hit")
		}
		if res.Rows[0].Name != "checkout" {
			t.Fatalf("expected checkout to outrank cart/checkout-pod, got %q", res.Rows[0].Name)
		}
	})

	t.Run("hybrid fuses both axes", func(t *testing.T) {
		res, err := svc.Hybrid(ctx, ws, model.SearchRequest{
			Source: ".entity",
			Query:  "checkout",
			TopK:   5,
		})
		if err != nil {
			t.Fatalf("hybrid: %v", err)
		}
		if len(res.Rows) < 2 {
			t.Fatalf("want at least 2 fused rows, got %d", len(res.Rows))
		}
		if res.Rows[0].Name != "checkout" && res.Rows[0].Name != "checkout-pod" {
			t.Fatalf("want checkout-* on top, got %q", res.Rows[0].Name)
		}
	})

	t.Run("topk clamps", func(t *testing.T) {
		res, err := svc.Keyword(ctx, ws, model.SearchRequest{
			Source: ".entity",
			Query:  "service",
			TopK:   1,
		})
		if err != nil {
			t.Fatalf("keyword: %v", err)
		}
		if len(res.Rows) != 1 {
			t.Fatalf("want topk=1 clamp, got %d", len(res.Rows))
		}
	})

	t.Run("capabilities advertise hybrid + rrf", func(t *testing.T) {
		caps, err := svc.Capabilities(ctx)
		if err != nil {
			t.Fatalf("caps: %v", err)
		}
		if !caps.HybridSearch || !caps.RRF {
			t.Fatalf("want hybrid+rrf advertised, got %+v", caps)
		}
	})

	t.Run("health reports ok", func(t *testing.T) {
		h, err := svc.Health(ctx)
		if err != nil {
			t.Fatalf("health: %v", err)
		}
		if h.Status != "ok" {
			t.Fatalf("want ok, got %q", h.Status)
		}
	})
}

func TestRegisteredProviders_HasMemory(t *testing.T) {
	found := false
	for _, p := range search.RegisteredProviders() {
		if p == search.ProviderTypeMemory {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("memory provider not registered")
	}
}
