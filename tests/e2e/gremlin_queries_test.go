package e2e_test

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestGremlinAllQuerySourcesAndSearchSurviveAppRestart(t *testing.T) {
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
	queries := []string{
		`.entity_set with(domain='devops', name='devops.service') | entity-call __list_method__()`,
		`.entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set','log_set'], true)`,
		`.entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = "ERROR"')`,
		`.entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')`,
		`.entity_set with(domain='platform', name='platform.service') | entity-call list_skills()`,
		`.entity_set with(domain='platform', name='platform.service') | entity-call list_knowledge()`,
	}
	for _, mode := range []string{"keyword", "vector", "hyper"} {
		queries = append(queries,
			`.runbook_set with(domain='platform', type='knowledge', query='retry', mode='`+mode+`', topk=5)`,
			`.umodel with(domain='platform', query='retry', mode='`+mode+`', topk=5)`,
			`.entity with(domain='devops', name='devops.service', query='checkout', mode='`+mode+`', topk=5)`)
	}
	assertQueries := func(app *bootstrap.App) {
		t.Helper()
		_, err := app.Query.Execute(ctx, workspace, model.QueryRequest{Query: `.topo | graph-match (s:"devops@devops.service")-[e]-(d)`})
		if !apperrors.IsCode(err, apperrors.CodeProviderUnsupported) {
			t.Fatalf("graph-match must match the advertised unsupported capability: %v", err)
		}
		for _, spl := range queries {
			rows, err := app.Query.Execute(ctx, workspace, model.QueryRequest{Query: spl})
			if err != nil || len(rows.Rows) == 0 {
				t.Fatalf("query failed: %s\nrows=%+v err=%v", spl, rows, err)
			}
		}
	}
	assertQueries(app)
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	assertQueries(testutil.GremlinApp(t, root))
}

func TestGremlinGoSDKPublicServiceExample(t *testing.T) {
	app := testutil.GremlinApp(t, t.TempDir(), bootstrap.WithImportRoot("/"))
	server := httptest.NewServer(app.Handler())
	defer server.Close()
	workspace := testutil.GremlinWorkspace(t)
	cmd := exec.Command("go", "run", "./service-quickstart", "--addr", server.URL, "--workspace", workspace)
	cmd.Dir = filepath.Join("..", "..", "examples", "sdk", "go")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Query returned 5 rows") || !strings.Contains(string(out), "Agent discovery:") {
		t.Fatalf("real SDK workflow: %v\n%s", err, out)
	}
	// Execute every method in the handwritten service client against real REST,
	// including entity/relation writes and lifecycle endpoints.
	file := filepath.Join(t.TempDir(), "sdk-flow.go")
	code := `package main
import ("context"; "fmt"; "os"; "strings"; "time"; "umodel_go_cli/service")
func main() {
 c := service.NewClient(os.Args[1]); w := os.Args[2]; ctx := context.Background()
 check := func(err error) { if err != nil { panic(err) } }
 _, err := c.CreateWorkspace(ctx, service.CreateWorkspaceRequest{ID:w}); check(err)
 elements := []service.UModelElement{{Kind:"entity_set", Domain:"devops", Name:"devops.service"}}
 validation, err := c.ValidateUModel(ctx,w,elements); check(err); if !validation.Valid { panic("invalid model") }
 put, err := c.PutUModelElements(ctx,w,elements); check(err); if put.Accepted != 1 { panic("model not written") }
 id := strings.Repeat("e",32); dest := strings.Repeat("f",32); now := time.Now().Unix()
 written, err := c.WriteEntities(ctx,w,[]map[string]any{{"__domain__":"devops","__entity_type__":"devops.service","__entity_id__":id,"__method__":"Create","__first_observed_time__":now,"__last_observed_time__":now,"display_name":"sdk-write-marker"}}); check(err); if written.Accepted != 1 { panic("entity not written") }
 rows, err := c.Query(ctx,w,service.QueryRequest{Query:".entity with(query='sdk-write-marker')"}); check(err); if len(rows.Rows)!=1 { panic("entity query lost rows") }
 plan, err := c.Explain(ctx,w,service.QueryRequest{Query:".entity"}); check(err); if plan.Provider!="gremlin" { panic("incorrect SDK provider") }
 relation, err := c.WriteRelations(ctx,w,[]map[string]any{{"__src_domain__":"devops","__src_entity_type__":"devops.service","__src_entity_id__":id,"__dest_domain__":"devops","__dest_entity_type__":"devops.service","__dest_entity_id__":dest,"__relation_type__":"calls","__method__":"Create","__first_observed_time__":now,"__last_observed_time__":now}}); check(err); if relation.Accepted!=1 { panic("relation not written") }
 expired, err := c.ExpireEntities(ctx,w,[]string{"devops/devops.service/"+id},"test"); check(err); if expired.Accepted!=1 { panic("entity not expired") }
 expiredRelation, err := c.ExpireRelations(ctx,w,[]string{"devops/devops.service/"+id+"/calls/devops/devops.service/"+dest},"test"); check(err); if expiredRelation.Accepted!=1 { panic("relation not expired") }
 rows, err = c.Query(ctx,w,service.QueryRequest{Query:".entity with(query='sdk-write-marker')"}); check(err); if len(rows.Rows)!=0 { panic("expired entity remained visible") }
 _, err = c.Discover(ctx,w); check(err); fmt.Println("all SDK service methods passed")
}
`
	if err := os.WriteFile(file, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "run", file, server.URL, testutil.GremlinWorkspace(t))
	cmd.Dir = filepath.Join("..", "..", "examples", "sdk", "go")
	out, err = cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "all SDK service methods passed") {
		t.Fatalf("SDK entity/relation lifecycle workflow: %v\n%s", err, out)
	}
}
