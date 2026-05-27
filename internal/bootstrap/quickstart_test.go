package bootstrap

import (
	"context"
	"testing"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestLoadQuickStartCreatesWorkspaceAndImportsSample(t *testing.T) {
	ctx := context.Background()
	app := NewMemoryApp(t.TempDir())

	result, err := app.LoadQuickStart(ctx, QuickStartOptions{})
	if err != nil {
		t.Fatalf("load quickstart: %v", err)
	}
	if result.Workspace != DefaultQuickStartWorkspaceID || result.Sample != DefaultQuickStartSample {
		t.Fatalf("unexpected quickstart result: %+v", result)
	}
	if result.UModel.Imported == 0 || result.EntityCount == 0 || result.RelationCount == 0 {
		t.Fatalf("quickstart should import model, entity, and topology data: %+v", result)
	}

	workspace, err := app.Workspace.GetWorkspace(ctx, DefaultQuickStartWorkspaceID)
	if err != nil {
		t.Fatalf("get quickstart workspace: %v", err)
	}
	if workspace.Name != DefaultQuickStartWorkspaceName || workspace.Labels["umodel.io/quickstart"] != "true" {
		t.Fatalf("unexpected quickstart workspace metadata: %+v", workspace)
	}

	rows, err := app.Query.Execute(ctx, DefaultQuickStartWorkspaceID, model.QueryRequest{
		Query: ".entity with(domain='devops', name='devops.service', query='checkout') | limit 5",
	})
	if err != nil {
		t.Fatalf("query quickstart sample: %v", err)
	}
	if len(rows.Rows) == 0 {
		t.Fatalf("expected quickstart sample entity rows, got %+v", rows)
	}

	entitySearch, err := app.Query.Execute(ctx, DefaultQuickStartWorkspaceID, model.QueryRequest{
		Query: ".entity with(domain='devops', name='devops.service', query='checkout', mode='vector', topk=5)",
	})
	if err != nil {
		t.Fatalf("search quickstart entities: %v", err)
	}
	assertSearchResult(t, entitySearch, "vector")

	umodelSearch, err := app.Query.Execute(ctx, DefaultQuickStartWorkspaceID, model.QueryRequest{
		Query: ".umodel with(kind='entity_set', query='service', mode='keyword', topk=5)",
	})
	if err != nil {
		t.Fatalf("search quickstart umodel: %v", err)
	}
	assertSearchResult(t, umodelSearch, "keyword")
}

func TestRunbookSetSearchIsConfigured(t *testing.T) {
	ctx := context.Background()
	app := NewMemoryApp(t.TempDir())

	if _, err := app.Samples.Import(ctx, "incident", "incident-investigation"); err != nil {
		t.Fatalf("import incident sample: %v", err)
	}

	result, err := app.Query.Execute(ctx, "incident", model.QueryRequest{
		Query: ".runbook_set with(domain='platform', type='knowledge', query='retry', mode='keyword', topk=5)",
	})
	if err != nil {
		t.Fatalf("search runbook set: %v", err)
	}
	assertSearchResult(t, result, "keyword")
}

func assertSearchResult(t *testing.T, result model.QueryResult, mode string) {
	t.Helper()
	if len(result.Rows) == 0 {
		t.Fatalf("expected search rows, got %+v", result)
	}
	if result.Explain == nil {
		t.Fatalf("expected search explain, got %+v", result)
	}
	if result.Explain.SearchMode != mode {
		t.Fatalf("unexpected search mode: %+v", result.Explain)
	}
	if result.Explain.SearchProvider != "memory" {
		t.Fatalf("unexpected search provider: %+v", result.Explain)
	}
}

func TestLoadQuickStartIsSafeWithExistingWorkspace(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	first := NewFileMemoryApp(root)
	if _, err := first.LoadQuickStart(ctx, QuickStartOptions{}); err != nil {
		t.Fatalf("first quickstart load: %v", err)
	}

	second := NewFileMemoryApp(root)
	result, err := second.LoadQuickStart(ctx, QuickStartOptions{})
	if err != nil {
		t.Fatalf("second quickstart load: %v", err)
	}
	if result.Workspace != DefaultQuickStartWorkspaceID || result.EntityCount == 0 || result.RelationCount == 0 {
		t.Fatalf("unexpected second quickstart result: %+v", result)
	}
}
