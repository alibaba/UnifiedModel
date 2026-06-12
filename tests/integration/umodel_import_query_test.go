package integration_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestImportQuickstartExamplesAndQueryUModel(t *testing.T) {
	ctx := context.Background()
	app := bootstrap.NewMemoryApp(t.TempDir(), bootstrap.WithImportRoot("/"))
	examples := filepath.Join("..", "..", "examples", "quickstart-multidomain")

	result, err := app.UModel.Import(ctx, "demo", model.UModelImportRequest{Path: examples})
	if err != nil {
		t.Fatalf("import examples: %v", err)
	}
	if result.Imported < 20 {
		t.Fatalf("expected quickstart import to include many elements, got %+v", result)
	}

	byName, err := app.Query.Execute(ctx, "demo", model.QueryRequest{
		Query: ".umodel with(kind='entity_set', domain='devops', name='devops.service') | limit 10",
	})
	if err != nil {
		t.Fatalf("query by name: %v", err)
	}
	if len(byName.Rows) != 1 || byName.Rows[0]["domain"] != "devops" || byName.Rows[0]["name"] != "devops.service" || byName.Rows[0]["kind"] != "entity_set" {
		t.Fatalf("unexpected name query rows: %+v", byName.Rows)
	}

	bySpec, err := app.Query.Execute(ctx, "demo", model.QueryRequest{
		Query: ".umodel with(kind='entity_set', domain='devops', query='Service') | limit 10",
	})
	if err != nil {
		t.Fatalf("query by spec: %v", err)
	}
	if len(bySpec.Rows) == 0 {
		t.Fatalf("expected spec query to match imported examples")
	}
}
