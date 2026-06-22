package query

import (
	"context"
	"fmt"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

// BenchmarkExecuteEntityQuery measures the full read entry point: normalize ->
// parse -> plan -> executor -> store scan, returning matched rows.
func BenchmarkExecuteEntityQuery(b *testing.B) {
	store := graphstore.NewMemoryStore()
	ctx := context.Background()
	ents := make([]model.EntityPayload, 1000)
	for i := range ents {
		ents[i] = model.EntityPayload{
			"__domain__":              "devops",
			"__entity_type__":         "devops.service",
			"__entity_id__":           fmt.Sprintf("%032x", i+1),
			"__method__":              "Update",
			"__first_observed_time__": int64(100),
			"__last_observed_time__":  int64(200),
			"__keep_alive_seconds__":  int64(60),
			"display_name":            fmt.Sprintf("svc-%d", i),
		}
	}
	if _, err := store.WriteEntities(ctx, model.EntityWriteBatch{Workspace: "demo", Entities: ents}); err != nil {
		b.Fatalf("seed: %v", err)
	}

	svc := NewService(store)
	req := model.QueryRequest{Query: ".entity with(domain='devops', name='devops.service') | limit 1000"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := svc.Execute(ctx, "demo", req)
		if err != nil {
			b.Fatal(err)
		}
		if len(res.Rows) == 0 {
			b.Fatal("expected rows")
		}
	}
}
