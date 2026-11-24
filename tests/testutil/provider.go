package testutil

import (
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/tests/graphfixture"
	"github.com/google/uuid"
)

// ProviderApp preserves the logical workspace IDs used by existing client
// tests while mapping their actual graph records to isolated physical IDs.
func ProviderApp(t *testing.T, root string, opts ...bootstrap.AppOption) *bootstrap.App {
	t.Helper()
	if graphfixture.Provider() == graphstore.ProviderTypeMemory {
		return bootstrap.NewMemoryApp(root, opts...)
	}
	store := graphfixture.New(t)
	name := "test-" + uuid.NewString()
	graphstore.RegisterProvider(name, func(graphstore.ProviderConfig) (contract.GraphStore, error) { return store, nil })
	app, err := bootstrap.NewAppWithGraphStore(root, graphstore.ProviderConfig{Type: name}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func MCPWorkspace(t *testing.T, logical string) string {
	t.Helper()
	if graphfixture.Provider() == graphstore.ProviderTypeGremlin {
		return GremlinWorkspace(t)
	}
	return logical
}
