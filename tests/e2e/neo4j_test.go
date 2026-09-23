package e2e_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestNeo4jQuickstartPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	app := testutil.Neo4jApp(t, root)
	workspace := testutil.Neo4jWorkspace(t)
	loaded, err := app.LoadQuickStart(ctx, bootstrap.QuickStartOptions{WorkspaceID: workspace})
	if err != nil || loaded.UModel.Imported == 0 || loaded.EntityCount == 0 || loaded.RelationCount == 0 {
		t.Fatalf("load quickstart: %+v %v", loaded, err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := testutil.Neo4jApp(t, root)
	metadata, err := reopened.Workspace.GetWorkspace(ctx, workspace)
	if err != nil || metadata.ID != workspace {
		t.Fatalf("workspace metadata lost: %+v %v", metadata, err)
	}
	server := httptest.NewServer(reopened.Handler())
	defer server.Close()
	health := e2eGet(t, server.URL+"/healthz")
	graphHealth, ok := health["graphstore"].(map[string]any)
	if !ok || graphHealth["provider"] != "neo4j" || graphHealth["status"] != "ok" {
		t.Fatalf("health: %+v", health)
	}
	for _, query := range []string{
		".umodel with(kind='entity_set') | limit 5",
		".entity with(domain='devops', name='devops.service') | limit 5",
		".topo | limit 5",
		".topo | graph-call cypher(`MATCH (s)-[r]->(d) RETURN s, r, d LIMIT 5`)",
	} {
		rows := e2eRows(t, e2ePost(t, server.URL+"/api/v1/query/"+workspace+"/execute", map[string]any{"query": query}))
		if len(rows) == 0 {
			t.Fatalf("no persisted rows for %s", query)
		}
	}
	plan, err := reopened.Query.Explain(ctx, workspace, model.QueryRequest{Query: ".entity | limit 5"})
	if err != nil || plan.Provider != "neo4j" {
		t.Fatalf("explain: %+v %v", plan, err)
	}
	if _, err := reopened.AgentGateway.Discover(ctx, workspace); err != nil {
		t.Fatalf("agent discovery: %v", err)
	}
}
