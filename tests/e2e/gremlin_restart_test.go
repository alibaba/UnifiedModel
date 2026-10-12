package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestGremlinDatabaseRestartAndDisconnectedWrites(t *testing.T) {
	container := os.Getenv("UMODEL_GREMLIN_CONTAINER")
	if container == "" {
		t.Skip("set UMODEL_GREMLIN_CONTAINER for an owned disposable persistent server")
	}
	if !strings.HasPrefix(container, "umodel-gremlin-") {
		t.Fatal("restart tests only operate on explicitly named umodel-gremlin- test containers")
	}
	runtime := os.Getenv("UMODEL_CONTAINER_RUNTIME")
	if runtime == "" {
		runtime = "docker"
	}
	control := func(args ...string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, runtime, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("test container control %v: %v\n%s", args, err, out)
		}
	}
	ctx := context.Background()
	root := t.TempDir()
	app := testutil.GremlinApp(t, root)
	workspace := testutil.GremlinWorkspace(t)
	if _, err := app.LoadQuickStart(ctx, bootstrap.QuickStartOptions{WorkspaceID: workspace}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Samples.Import(ctx, workspace, "incident-investigation"); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, source := range []string{".umodel", ".entity", ".topo"} {
		result, err := app.Query.Execute(ctx, workspace, model.QueryRequest{Query: source + " | limit 1000"})
		if err != nil || len(result.Rows) == 0 {
			t.Fatalf("baseline %s: %+v %v", source, result, err)
		}
		counts[source] = len(result.Rows)
	}
	control("stop", "--time", "20", container)
	t.Cleanup(func() { control("start", container) })
	deadline, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, err := app.GraphStore.QueryEntities(deadline, model.EntityQueryPlan{Workspace: workspace})
	cancel()
	if err == nil {
		t.Fatal("disconnected server returned cached entity rows")
	}
	result, err := app.GraphStore.WriteEntities(ctx, model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{
		{"__domain__": "devops", "__entity_type__": "devops.service", "__entity_id__": "disconnected", "__method__": "Create"},
	}})
	if err == nil || result.Accepted != 0 {
		t.Fatalf("disconnected write reported success: %+v %v", result, err)
	}
	_ = app.Close()
	control("start", container)
	ready := false
	until := time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		p, err := graphstore.NewProvider(graphstore.ProviderConfig{Type: graphstore.ProviderTypeGremlin, Options: map[string]string{"timeout": "1s"}})
		if err == nil {
			_ = p.(interface{ Close() error }).Close()
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		t.Fatal("persistent server did not restart")
	}
	reopened := testutil.GremlinApp(t, root)
	for source, count := range counts {
		rows, err := reopened.Query.Execute(ctx, workspace, model.QueryRequest{Query: source + " | limit 1000"})
		if err != nil || len(rows.Rows) != count {
			t.Fatalf("database restart changed %s: got=%d want=%d err=%v", source, len(rows.Rows), count, err)
		}
	}
	rows, err := reopened.Query.Execute(ctx, workspace, model.QueryRequest{Query: `.runbook_set with(domain='platform', type='knowledge', query='retry', mode='keyword')`})
	if err != nil || len(rows.Rows) == 0 {
		t.Fatalf("search index missing after database restart: %+v %v", rows, err)
	}
}
