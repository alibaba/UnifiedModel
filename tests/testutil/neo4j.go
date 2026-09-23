package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	driver "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jApp only connects when explicitly opted in. Each test owns uniquely named
// workspaces; cleanup never deletes records from any other workspace.
func Neo4jApp(t *testing.T, root string) *bootstrap.App {
	t.Helper()
	if os.Getenv("UMODEL_TEST_NEO4J") != "1" {
		t.Skip("set UMODEL_TEST_NEO4J=1 and NEO4J_* to run Neo4j integration tests")
	}
	app, err := bootstrap.NewAppWithGraphStore(root, graphstore.ProviderConfig{Type: graphstore.ProviderTypeNeo4j})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func Neo4jWorkspace(t *testing.T) string {
	t.Helper()
	id := fmt.Sprintf("neo4j-test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		uri, username, database := os.Getenv("NEO4J_URI"), os.Getenv("NEO4J_USERNAME"), os.Getenv("NEO4J_DATABASE")
		if uri == "" {
			uri = "bolt://localhost:7687"
		}
		if username == "" {
			username = "neo4j"
		}
		if database == "" && os.Getenv("NEO4J_DIALECT") != "opencypher" {
			database = "neo4j"
		}
		d, err := driver.NewDriverWithContext(uri, driver.BasicAuth(username, os.Getenv("NEO4J_PASSWORD"), ""))
		if err != nil {
			t.Error(err)
			return
		}
		defer d.Close(ctx)
		for _, statement := range []string{
			`MATCH ()-[r:UMODEL_RELATION {workspace: $workspace}]->() DELETE r`,
			`MATCH (n:UModelEntity {workspace: $workspace}) DELETE n`,
			`MATCH (n:UModelElement {workspace: $workspace}) DELETE n`,
			`MATCH (w:UModelWorkspace {id: $workspace}) DELETE w`,
		} {
			if _, err := driver.ExecuteQuery(ctx, d, statement, map[string]any{"workspace": id}, driver.EagerResultTransformer, driver.ExecuteQueryWithDatabase(database), driver.ExecuteQueryWithoutBookmarkManager()); err != nil {
				t.Errorf("cleanup test workspace: %v", err)
			}
		}
	})
	return id
}
