package testutil

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/tests/graphfixture"
	driver "github.com/apache/tinkerpop/gremlin-go/v3/driver"
)

func GremlinApp(t *testing.T, root string, opts ...bootstrap.AppOption) *bootstrap.App {
	t.Helper()
	if os.Getenv("UMODEL_TEST_GREMLIN") != "1" {
		t.Skip("set UMODEL_TEST_GREMLIN=1 and GREMLIN_* to run Gremlin integration tests")
	}
	app, err := bootstrap.NewAppWithGraphStore(root, graphstore.ProviderConfig{Type: graphstore.ProviderTypeGremlin}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

// GremlinWorkspace owns uniquely named graph records. Cleanup is scoped to the
// workspace and never drops the graph or data belonging to another test/user.
func GremlinWorkspace(t *testing.T) string {
	t.Helper()
	id := fmt.Sprintf("gremlin-test-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		GremlinSubmit(t, `g.E().hasLabel('UMODEL_RELATION').has('workspace', workspace).drop().iterate(); g.V().hasLabel('UModelWorkspace','UModelElement','UModelEntity').has('workspace', workspace).drop().iterate(); 1`, map[string]any{"workspace": id})
	})
	return id
}

func GremlinSubmit(t *testing.T, script string, bindings map[string]any) {
	t.Helper()
	uri := os.Getenv("GREMLIN_URI")
	if uri == "" {
		uri = "ws://localhost:8182/gremlin"
	}
	client, err := driver.NewClient(uri, func(s *driver.ClientSettings) {
		s.LogVerbosity = driver.Off
		s.ConnectionTimeout = 10 * time.Second
		s.WriteBufferSize = 1 << 20
		s.AuthInfo = driver.BasicAuthInfo(os.Getenv("GREMLIN_USERNAME"), os.Getenv("GREMLIN_PASSWORD"))
		s.TlsConfig = graphfixture.TLSConfig(t)
		if source := os.Getenv("GREMLIN_TRAVERSAL_SOURCE"); source != "" {
			s.TraversalSource = source
		}
	})
	if err != nil {
		t.Errorf("open Gremlin test client: %v", err)
		return
	}
	defer client.Close()
	results, err := client.SubmitWithOptions(script, new(driver.RequestOptionsBuilder).SetBindings(bindings).SetEvaluationTimeout(10000).Create())
	if err == nil {
		_, err = results.All()
	}
	if err != nil {
		t.Errorf("Gremlin test request: %v", err)
	}
}
