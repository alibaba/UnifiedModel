//go:build ladybug

package e2e_test

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
)

func TestLadybugOptionalBusinessFlowPersistsAcrossRestart(t *testing.T) {
	if os.Getenv("UMODEL_TEST_LADYBUG") != "1" {
		t.Skip("set UMODEL_TEST_LADYBUG=1 and provide liblbug to run local.ladybug E2E tests")
	}

	dataRoot := t.TempDir()
	app := bootstrap.NewApp(dataRoot)
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	e2ePost(t, server.URL+"/api/v1/workspaces", map[string]any{"id": "ladybug-demo"})
	e2ePost(t, server.URL+"/api/v1/umodel/ladybug-demo/elements", map[string]any{"elements": []map[string]any{{
		"kind":   "entity_set",
		"domain": "devops",
		"name":   "devops.service",
	}}})
	e2ePost(t, server.URL+"/api/v1/entitystore/ladybug-demo/entities:write", map[string]any{"entities": []map[string]any{
		entityPayload("54013ba69c196820e56801f1ef5aad54", "Update", 100, 200, map[string]any{"display_name": "cart"}),
	}})
	e2ePost(t, server.URL+"/api/v1/entitystore/ladybug-demo/relations:write", map[string]any{"relations": []map[string]any{
		relationPayload("54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f", "Update", 100, 200, nil),
	}})

	if rows := e2eRows(t, e2ePost(t, server.URL+"/api/v1/query/ladybug-demo/execute", map[string]any{
		"query": ".umodel with(kind='entity_set', domain='devops', name='devops.service') | limit 5",
	})); len(rows) != 1 {
		t.Fatalf("expected ladybug umodel row before restart, got %+v", rows)
	}
	if closer, ok := app.GraphStore.(interface{ Close() }); ok {
		closer.Close()
	}
	server.Close()

	reopened := bootstrap.NewApp(dataRoot)
	reopenedServer := httptest.NewServer(reopened.Handler())
	defer reopenedServer.Close()
	defer func() {
		if closer, ok := reopened.GraphStore.(interface{ Close() }); ok {
			closer.Close()
		}
	}()

	workspacePage := e2eGet(t, reopenedServer.URL+"/api/v1/workspaces")
	workspaces, ok := workspacePage["items"].([]any)
	if !ok {
		t.Fatalf("workspace list has no items: %+v", workspacePage)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected persisted ladybug workspace metadata after restart, got %+v", workspacePage)
	}
	workspace, ok := workspaces[0].(map[string]any)
	if !ok || workspace["id"] != "ladybug-demo" {
		t.Fatalf("expected ladybug-demo workspace metadata after restart, got %+v", workspacePage)
	}

	umodelRows := e2eRows(t, e2ePost(t, reopenedServer.URL+"/api/v1/query/ladybug-demo/execute", map[string]any{
		"query": ".umodel with(kind='entity_set', domain='devops', name='devops.service') | limit 5",
	}))
	if len(umodelRows) != 1 {
		t.Fatalf("expected persisted ladybug umodel row after restart, got %+v", umodelRows)
	}
	entityRows := e2eRows(t, e2ePost(t, reopenedServer.URL+"/api/v1/query/ladybug-demo/execute", map[string]any{
		"query": ".entity with(domain='devops', name='devops.*', ids=['54013ba69c196820e56801f1ef5aad54'], query='cart') | limit 5",
	}))
	if len(entityRows) != 1 || entityRows[0]["__entity_id__"] != "54013ba69c196820e56801f1ef5aad54" {
		t.Fatalf("expected persisted ladybug entity row after restart, got %+v", entityRows)
	}
	topoRows := e2eRows(t, e2ePost(t, reopenedServer.URL+"/api/v1/query/ladybug-demo/execute", map[string]any{
		"query": ".topo with(relation_type='calls') | limit 5",
	}))
	if len(topoRows) != 1 || topoRows[0]["relation"] != "calls" {
		t.Fatalf("expected persisted ladybug topo row after restart, got %+v", topoRows)
	}
}
