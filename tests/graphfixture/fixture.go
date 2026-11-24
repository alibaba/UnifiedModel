// Package graphfixture runs the shared query and client suites on an opted-in
// GraphStore, isolating every test's logical workspaces in the remote graph.
package graphfixture

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/pkg/model"
	driver "github.com/apache/tinkerpop/gremlin-go/v3/driver"
	"github.com/google/uuid"
)

func Provider() string {
	if os.Getenv("UMODEL_TEST_GRAPHSTORE") == "gremlin" {
		return graphstore.ProviderTypeGremlin
	}
	return graphstore.ProviderTypeMemory
}

func New(t *testing.T) contract.GraphStore {
	t.Helper()
	if Provider() == graphstore.ProviderTypeMemory {
		return graphstore.NewMemoryStore()
	}
	if os.Getenv("UMODEL_TEST_GREMLIN") != "1" {
		t.Fatal("UMODEL_TEST_GRAPHSTORE=gremlin requires UMODEL_TEST_GREMLIN=1")
	}
	s, err := graphstore.NewProvider(graphstore.ProviderConfig{Type: graphstore.ProviderTypeGremlin})
	if err != nil {
		t.Fatal(err)
	}
	f := &scoped{GraphStore: s, prefix: "gremlin-suite-" + uuid.NewString() + "-", workspaces: map[string]bool{}}
	t.Cleanup(func() {
		_ = f.Close()
		ids := make([]string, 0, len(f.workspaces))
		for id := range f.workspaces {
			ids = append(ids, id)
		}
		Cleanup(t, ids)
	})
	return f
}

// Cleanup only removes exact workspace IDs owned by a test. Never clear a graph.
func Cleanup(t *testing.T, ids []string) {
	t.Helper()
	if len(ids) == 0 {
		return
	}
	uri := os.Getenv("GREMLIN_URI")
	if uri == "" {
		uri = "ws://localhost:8182/gremlin"
	}
	c, err := driver.NewClient(uri, func(s *driver.ClientSettings) {
		s.LogVerbosity = driver.Off
		s.ConnectionTimeout, s.WriteDeadline = 10*time.Second, 10*time.Second
		s.WriteBufferSize = 1 << 20
		s.AuthInfo = driver.BasicAuthInfo(os.Getenv("GREMLIN_USERNAME"), os.Getenv("GREMLIN_PASSWORD"))
		s.TlsConfig = TLSConfig(t)
		if source := os.Getenv("GREMLIN_TRAVERSAL_SOURCE"); source != "" {
			s.TraversalSource = source
		}
	})
	if err != nil {
		t.Errorf("open cleanup connection: %v", err)
		return
	}
	defer c.Close()
	r, err := c.SubmitWithOptions(`workspaces.each { workspace ->
g.E().hasLabel('UMODEL_RELATION').has('workspace', workspace).drop().iterate()
g.V().hasLabel('UModelWorkspace','UModelElement','UModelEntity').has('workspace', workspace).drop().iterate()
}; 1`, new(driver.RequestOptionsBuilder).SetBindings(map[string]any{"workspaces": ids}).SetEvaluationTimeout(10000).Create())
	if err == nil {
		_, err = r.All()
	}
	if err != nil {
		t.Errorf("cleanup own workspaces: %v", err)
	}
}

func TLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	c := &tls.Config{MinVersion: tls.VersionTLS12}
	if file := os.Getenv("GREMLIN_CA_FILE"); file != "" {
		pem, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			t.Fatal("invalid test CA")
		}
		c.RootCAs = roots
	}
	return c
}

type scoped struct {
	contract.GraphStore
	prefix     string
	mu         sync.Mutex
	workspaces map[string]bool
}

func (s *scoped) workspace(id string) string {
	if id == "" {
		return ""
	}
	id = s.prefix + id
	s.mu.Lock()
	s.workspaces[id] = true
	s.mu.Unlock()
	return id
}

func (s *scoped) OpenWorkspace(ctx context.Context, w model.WorkspaceMetadata) error {
	w.ID = s.workspace(w.ID)
	return s.GraphStore.OpenWorkspace(ctx, w)
}

func (s *scoped) EnsureSchema(ctx context.Context, w string) error {
	return s.GraphStore.EnsureSchema(ctx, s.workspace(w))
}

func (s *scoped) PutUModelElements(ctx context.Context, b model.UModelElementBatch) (model.WriteResult, error) {
	b.Workspace = s.workspace(b.Workspace)
	return s.GraphStore.PutUModelElements(ctx, b)
}

func (s *scoped) DeleteUModelElements(ctx context.Context, w string, ids []string) (model.WriteResult, error) {
	return s.GraphStore.DeleteUModelElements(ctx, s.workspace(w), ids)
}

func (s *scoped) GetUModelSnapshot(ctx context.Context, r model.UModelSnapshotRequest) (model.UModelSnapshot, error) {
	w := r.Workspace
	r.Workspace = s.workspace(w)
	result, err := s.GraphStore.GetUModelSnapshot(ctx, r)
	result.Workspace = w
	return result, err
}

func (s *scoped) WriteEntities(ctx context.Context, b model.EntityWriteBatch) (model.WriteResult, error) {
	b.Workspace = s.workspace(b.Workspace)
	return s.GraphStore.WriteEntities(ctx, b)
}

func (s *scoped) WriteRelations(ctx context.Context, b model.RelationWriteBatch) (model.WriteResult, error) {
	b.Workspace = s.workspace(b.Workspace)
	return s.GraphStore.WriteRelations(ctx, b)
}

func (s *scoped) QueryEntities(ctx context.Context, p model.EntityQueryPlan) (model.QueryResult, error) {
	p.Workspace = s.workspace(p.Workspace)
	return s.GraphStore.QueryEntities(ctx, p)
}

func (s *scoped) QueryTopo(ctx context.Context, p model.TopoQueryPlan) (model.QueryResult, error) {
	p.Workspace = s.workspace(p.Workspace)
	return s.GraphStore.QueryTopo(ctx, p)
}

func (s *scoped) Close() error {
	if c, ok := s.GraphStore.(interface{ Close() error }); ok {
		return c.Close()
	}
	return fmt.Errorf("test provider cannot close connections")
}
