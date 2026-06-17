package contract_test

import (
	"context"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestMemoryGraphStoreConformance(t *testing.T) {
	exerciseGraphStore(t, graphstore.NewMemoryStore())
}

func TestFileMemoryGraphStoreConformance(t *testing.T) {
	store, err := graphstore.NewProvider(graphstore.ProviderConfig{
		Type:     graphstore.ProviderTypeFileMemory,
		DataRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("new file memory provider: %v", err)
	}
	exerciseGraphStore(t, store)
}

func TestDefaultAppUsesLadybugProvider(t *testing.T) {
	app := bootstrap.NewApp(t.TempDir())
	health, err := app.GraphStore.Health(context.Background())
	if err != nil {
		t.Fatalf("default provider health: %v", err)
	}
	if health.Provider != graphstore.ProviderTypeLadybug {
		t.Fatalf("default provider should be local.ladybug, got %+v", health)
	}
}

func TestGraphStoreProviderRegistryRejectsUnknownProvider(t *testing.T) {
	if _, err := graphstore.NewProvider(graphstore.ProviderConfig{Type: "missing.provider"}); err == nil {
		t.Fatalf("expected unknown provider to fail")
	}
}

func TestGraphStoreProviderRegistryCanSelectMemory(t *testing.T) {
	provider, err := graphstore.NewProvider(graphstore.ProviderConfig{Type: graphstore.ProviderTypeMemory})
	if err != nil {
		t.Fatalf("new memory provider: %v", err)
	}
	health, err := provider.Health(context.Background())
	if err != nil {
		t.Fatalf("memory provider health: %v", err)
	}
	if health.Provider != graphstore.ProviderTypeMemory {
		t.Fatalf("explicit memory provider should be memory, got %+v", health)
	}
}

func TestGraphStoreProviderRegistryCanSelectFileMemory(t *testing.T) {
	provider, err := graphstore.NewProvider(graphstore.ProviderConfig{Type: graphstore.ProviderTypeFileMemory, DataRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("new file memory provider: %v", err)
	}
	health, err := provider.Health(context.Background())
	if err != nil {
		t.Fatalf("file memory provider health: %v", err)
	}
	if health.Provider != graphstore.ProviderTypeFileMemory {
		t.Fatalf("explicit file memory provider should be file.memory, got %+v", health)
	}
}

func exerciseGraphStore(t *testing.T, store contract.GraphStore) {
	t.Helper()
	ctx := context.Background()
	if err := store.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: "demo"}); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	if err := store.EnsureSchema(ctx, "demo"); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if _, err := store.Capabilities(ctx); err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if health, err := store.Health(ctx); err != nil || health.Provider == "" {
		t.Fatalf("health: %+v err=%v", health, err)
	}

	write, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements: []model.UModelElement{{
			Kind:    "entity_set",
			Domain:  "apm",
			Name:    "apm.service",
			Version: "v1",
			Spec: map[string]any{
				"display_name": "APM Service",
			},
		}},
	})
	if err != nil || write.Accepted != 1 {
		t.Fatalf("put umodel: %+v err=%v", write, err)
	}
	snapshot, err := store.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: "demo"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Elements) != 1 {
		t.Fatalf("expected one umodel element, got %+v", snapshot.Elements)
	}
	if snapshot.Elements[0].Version != "v1" || snapshot.Elements[0].Spec["display_name"] != "APM Service" {
		t.Fatalf("unexpected umodel snapshot: %+v", snapshot.Elements[0])
	}
	missingDelete, err := store.DeleteUModelElements(ctx, "demo", []string{"apm/missing/entity_set"})
	if err != nil {
		t.Fatalf("delete missing umodel: %v", err)
	}
	if missingDelete.Failed != 1 || missingDelete.Items[0].Code != string(apperrors.CodeNotFound) {
		t.Fatalf("expected missing umodel delete failure, got %+v", missingDelete)
	}
	deleteResult, err := store.DeleteUModelElements(ctx, "demo", []string{"apm/apm.service/entity_set"})
	if err != nil {
		t.Fatalf("delete umodel: %v", err)
	}
	if deleteResult.Accepted != 1 || deleteResult.Failed != 0 || deleteResult.Items[0].ID != "apm/apm.service/entity_set" {
		t.Fatalf("unexpected umodel delete result: %+v", deleteResult)
	}
	snapshot, err = store.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: "demo"})
	if err != nil {
		t.Fatalf("snapshot after delete: %v", err)
	}
	if len(snapshot.Elements) != 0 {
		t.Fatalf("expected deleted umodel element to be absent, got %+v", snapshot.Elements)
	}

	if _, err := store.WriteEntities(ctx, model.EntityWriteBatch{Workspace: "demo", Entities: []model.EntityPayload{entity("54013ba69c196820e56801f1ef5aad54")}}); err != nil {
		t.Fatalf("write entity: %v", err)
	}
	from := time.Unix(150, 0)
	to := time.Unix(180, 0)
	entityRows, err := store.QueryEntities(ctx, model.EntityQueryPlan{
		Workspace: "demo",
		Filters:   map[string]any{"domain": "apm", "name": "apm.*", "ids": []string{"54013ba69c196820e56801f1ef5aad54"}, "query": "cart service"},
		TimeRange: model.TimeRange{From: &from, To: &to},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("query entity: %v", err)
	}
	if len(entityRows.Rows) != 1 {
		t.Fatalf("expected one entity row, got %+v", entityRows.Rows)
	}
	if entityRows.Rows[0]["__entity_id__"] != "54013ba69c196820e56801f1ef5aad54" || entityRows.Rows[0]["display_name"] != "cart service" {
		t.Fatalf("unexpected entity row: %+v", entityRows.Rows[0])
	}
	future := time.Unix(1000, 0)
	futureRows, err := store.QueryEntities(ctx, model.EntityQueryPlan{
		Workspace: "demo",
		TimeRange: model.TimeRange{From: &future},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("query future entity: %v", err)
	}
	if len(futureRows.Rows) != 0 {
		t.Fatalf("expected no future entity rows, got %+v", futureRows.Rows)
	}

	if _, err := store.WriteRelations(ctx, model.RelationWriteBatch{Workspace: "demo", Relations: []model.RelationPayload{relation("54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f")}}); err != nil {
		t.Fatalf("write relation: %v", err)
	}
	topoRows, err := store.QueryTopo(ctx, model.TopoQueryPlan{
		Workspace: "demo",
		Filters:   map[string]any{"relation_type": "calls"},
		TimeRange: model.TimeRange{From: &from, To: &to},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("query topo: %v", err)
	}
	if len(topoRows.Rows) != 1 {
		t.Fatalf("expected one topo row, got %+v", topoRows.Rows)
	}
	if topoRows.Rows[0]["src"] != "apm/apm.service/54013ba69c196820e56801f1ef5aad54" || topoRows.Rows[0]["dest"] != "apm/apm.service/177627f91af678a9b03e993f1a91917f" || topoRows.Rows[0]["relation"] != "calls" {
		t.Fatalf("unexpected topo row: %+v", topoRows.Rows[0])
	}
}

func entity(id string) model.EntityPayload {
	displayName := id + " service"
	if id == "54013ba69c196820e56801f1ef5aad54" {
		displayName = "cart service"
	}
	return model.EntityPayload{
		"__domain__":              "apm",
		"__entity_type__":         "apm.service",
		"__entity_id__":           id,
		"__method__":              "Update",
		"__first_observed_time__": int64(100),
		"__last_observed_time__":  int64(200),
		"__keep_alive_seconds__":  int64(60),
		"display_name":            displayName,
	}
}

func relation(src, dest string) model.RelationPayload {
	return model.RelationPayload{
		"__src_domain__":          "apm",
		"__src_entity_type__":     "apm.service",
		"__src_entity_id__":       src,
		"__dest_domain__":         "apm",
		"__dest_entity_type__":    "apm.service",
		"__dest_entity_id__":      dest,
		"__relation_type__":       "calls",
		"__method__":              "Update",
		"__first_observed_time__": int64(100),
		"__last_observed_time__":  int64(200),
		"__keep_alive_seconds__":  int64(60),
		"weight":                  "critical",
	}
}
