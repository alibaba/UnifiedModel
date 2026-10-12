package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestGremlinPartialAndRepeatedWritesKeepSearchConsistent(t *testing.T) {
	ctx := context.Background()
	app := testutil.GremlinApp(t, t.TempDir())
	w := testutil.GremlinWorkspace(t)
	if _, err := app.LoadQuickStart(ctx, bootstrap.QuickStartOptions{WorkspaceID: w}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	result, err := app.EntityStore.WriteEntities(ctx, w, model.EntityWriteBatch{PartialSuccess: true, Entities: []model.EntityPayload{
		entityPayload("10000000000000000000000000000101", "Create", now, now, map[string]any{"display_name": "rejected-poison"}),
		entityPayload(strings.Repeat("f", 32), "Create", now, now, map[string]any{"display_name": "accepted-new-service"}),
	}})
	if err != nil || result.Accepted != 1 || result.Failed != 1 {
		t.Fatalf("mixed Create result: %+v %v", result, err)
	}
	rows, err := app.Query.Execute(ctx, w, model.QueryRequest{Query: `.entity with(query='rejected-poison', mode='keyword')`})
	if err != nil || len(rows.Rows) != 0 {
		t.Fatalf("failed Create poisoned search state: %+v %v", rows, err)
	}
	_, err = app.EntityStore.WriteEntities(ctx, w, model.EntityWriteBatch{Entities: []model.EntityPayload{
		entityPayload(strings.Repeat("f", 32), "Update", now, now, map[string]any{"display_name": "deleted-search-marker"}),
		entityPayload(strings.Repeat("f", 32), "Delete", 0, now, nil),
	}})
	if err != nil {
		t.Fatal(err)
	}
	rows, err = app.Query.Execute(ctx, w, model.QueryRequest{Query: `.entity with(query='deleted-search-marker', mode='keyword')`})
	if err != nil || len(rows.Rows) != 0 {
		t.Fatalf("Update followed by Delete resurrected a search row: %+v %v", rows, err)
	}
}
