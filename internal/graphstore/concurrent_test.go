package graphstore

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

// TestConcurrentReadWrite stresses the memory store's RWMutex with overlapping
// readers and writers. On its own it asserts no operation errors; under `go test
// -race` (the gate's mode) it is a data-race regression guard for the shared
// entity/relation maps.
func TestConcurrentReadWrite(t *testing.T) {
	store := NewMemoryStore()
	seedMemoryStore(t, store, 200)

	const (
		readers = 24
		writers = 8
		iters   = 100
	)
	errCh := make(chan error, readers+writers)
	var wg sync.WaitGroup

	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < iters; i++ {
				if _, err := store.QueryEntities(ctx, model.EntityQueryPlan{Workspace: "demo", Limit: 100}); err != nil {
					errCh <- fmt.Errorf("query entities: %w", err)
					return
				}
				if _, err := store.QueryTopo(ctx, model.TopoQueryPlan{Workspace: "demo", Limit: 100}); err != nil {
					errCh <- fmt.Errorf("query topo: %w", err)
					return
				}
			}
		}()
	}
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < iters; i++ {
				id := fmt.Sprintf("%032x", 1_000_000+w*iters+i)
				if _, err := store.WriteEntities(ctx, model.EntityWriteBatch{
					Workspace: "demo",
					Entities:  []model.EntityPayload{entityPayload(id, "Update", 100, 200, nil)},
				}); err != nil {
					errCh <- fmt.Errorf("write entities: %w", err)
					return
				}
			}
		}(w)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent operation failed: %v", err)
	}
}
