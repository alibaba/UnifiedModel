package graphstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

// seedMemoryStore fills a workspace with n entities and n relations (a ring) so
// the scan paths have realistic work to do.
func seedMemoryStore(tb testing.TB, store *MemoryStore, n int) {
	tb.Helper()
	ctx := context.Background()
	ents := make([]model.EntityPayload, n)
	rels := make([]model.RelationPayload, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%032x", i+1)
		ents[i] = entityPayload(id, "Update", 100, 200, map[string]any{"display_name": fmt.Sprintf("svc-%d", i)})
	}
	if _, err := store.WriteEntities(ctx, model.EntityWriteBatch{Workspace: "demo", Entities: ents}); err != nil {
		tb.Fatalf("seed entities: %v", err)
	}
	for i := 0; i < n; i++ {
		src := fmt.Sprintf("%032x", i+1)
		dst := fmt.Sprintf("%032x", (i+1)%n+1)
		rels[i] = relationPayload(src, dst, "Update", 100, 200, nil)
	}
	if _, err := store.WriteRelations(ctx, model.RelationWriteBatch{Workspace: "demo", Relations: rels}); err != nil {
		tb.Fatalf("seed relations: %v", err)
	}
}

func BenchmarkQueryEntities(b *testing.B) {
	store := NewMemoryStore()
	seedMemoryStore(b, store, 2000)
	ctx := context.Background()
	plan := model.EntityQueryPlan{Workspace: "demo", Limit: 1000}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.QueryEntities(ctx, plan); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQueryTopo(b *testing.B) {
	store := NewMemoryStore()
	seedMemoryStore(b, store, 2000)
	ctx := context.Background()
	plan := model.TopoQueryPlan{Workspace: "demo", Limit: 1000}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.QueryTopo(ctx, plan); err != nil {
			b.Fatal(err)
		}
	}
}
