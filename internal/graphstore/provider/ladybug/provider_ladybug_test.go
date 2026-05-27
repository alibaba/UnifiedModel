//go:build ladybug

package ladybug

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestLadybugProviderConformance(t *testing.T) {
	if os.Getenv("UMODEL_TEST_LADYBUG") != "1" {
		t.Skip("set UMODEL_TEST_LADYBUG=1 and provide liblbug to run local.ladybug conformance tests")
	}

	dataRoot := t.TempDir()
	provider, err := NewProvider(graphstore.ProviderConfig{DataRoot: dataRoot})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	defer provider.Close()

	ctx := context.Background()
	if err := provider.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: "demo"}); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	if err := provider.EnsureSchema(ctx, "demo"); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if capabilities, err := provider.Capabilities(ctx); err != nil || !capabilities.ControlledCypher {
		t.Fatalf("capabilities: %+v err=%v", capabilities, err)
	}
	if health, err := provider.Health(ctx); err != nil || health.Provider != graphstore.ProviderTypeLadybug || health.Status != "ok" {
		t.Fatalf("health: %+v err=%v", health, err)
	}

	if _, err := provider.PutUModelElements(ctx, model.UModelElementBatch{
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
	}); err != nil {
		t.Fatalf("put umodel: %v", err)
	}
	snapshot, err := provider.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: "demo"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Elements) != 1 {
		t.Fatalf("expected one element, got %+v", snapshot.Elements)
	}
	if snapshot.Elements[0].Version != "v1" || snapshot.Elements[0].Spec["display_name"] != "APM Service" {
		t.Fatalf("unexpected umodel snapshot: %+v", snapshot.Elements[0])
	}

	if _, err := provider.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities: []model.EntityPayload{
			entity("54013ba69c196820e56801f1ef5aad54"),
			entity("177627f91af678a9b03e993f1a91917f"),
		},
	}); err != nil {
		t.Fatalf("write entity: %v", err)
	}

	from := time.Unix(150, 0)
	to := time.Unix(180, 0)
	entityRows, err := provider.QueryEntities(ctx, model.EntityQueryPlan{
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
	futureRows, err := provider.QueryEntities(ctx, model.EntityQueryPlan{
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

	if _, err := provider.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{relation("54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f")},
	}); err != nil {
		t.Fatalf("write relation: %v", err)
	}
	topoRows, err := provider.QueryTopo(ctx, model.TopoQueryPlan{
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
	if _, err := provider.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{relation("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "177627f91af678a9b03e993f1a91917f")},
	}); err != nil {
		t.Fatalf("write unrelated relation: %v", err)
	}
	seedTopoRows, err := provider.QueryTopo(ctx, model.TopoQueryPlan{
		Workspace: "demo",
		GraphCall: &model.GraphCallPlan{
			Name:    "getDirectRelations",
			SeedIDs: []string{"54013ba69c196820e56801f1ef5aad54"},
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("query topo graph-call seed: %v", err)
	}
	if len(seedTopoRows.Rows) != 1 || seedTopoRows.Rows[0]["src"] != "apm/apm.service/54013ba69c196820e56801f1ef5aad54" {
		t.Fatalf("expected graph-call seed to filter unrelated relations, got %+v", seedTopoRows.Rows)
	}
	cypherRows, err := provider.QueryTopo(ctx, model.TopoQueryPlan{
		Workspace: "demo",
		GraphCall: &model.GraphCallPlan{
			Name:   "cypher",
			Cypher: "MATCH (src:`apm@apm.service` {__entity_id__: $src})-[r:calls]->(dest) RETURN properties(src) AS src, properties(r) AS relation, properties(dest) AS dest",
		},
		Params: map[string]any{"src": "54013ba69c196820e56801f1ef5aad54"},
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("query cypher topo properties: %v", err)
	}
	if len(cypherRows.Rows) != 1 {
		t.Fatalf("expected one cypher row, got %+v", cypherRows.Rows)
	}
	cypherRow := cypherRows.Rows[0]
	src, ok := cypherRow["src"].(map[string]any)
	if !ok || src["display_name"] != "cart service" {
		t.Fatalf("unexpected cypher source properties: %#v", cypherRow["src"])
	}
	relation, ok := cypherRow["relation"].(map[string]any)
	if !ok || relation["weight"] != "critical" || relation["__relation_type__"] != "calls" {
		t.Fatalf("unexpected cypher relation properties: %#v", cypherRow["relation"])
	}
	dest, ok := cypherRow["dest"].(map[string]any)
	if !ok || dest["display_name"] != "177627f91af678a9b03e993f1a91917f service" {
		t.Fatalf("unexpected cypher destination properties: %#v", cypherRow["dest"])
	}

	provider.Close()
	reopened, err := NewProvider(graphstore.ProviderConfig{DataRoot: dataRoot})
	if err != nil {
		t.Fatalf("new reopened provider: %v", err)
	}
	defer reopened.Close()
	if err := reopened.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: "demo"}); err != nil {
		t.Fatalf("reopen workspace: %v", err)
	}
	reopenedSnapshot, err := reopened.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: "demo"})
	if err != nil {
		t.Fatalf("reopened snapshot: %v", err)
	}
	if len(reopenedSnapshot.Elements) != 1 || reopenedSnapshot.Elements[0].Domain != "apm" || reopenedSnapshot.Elements[0].Name != "apm.service" || reopenedSnapshot.Elements[0].Kind != "entity_set" {
		t.Fatalf("expected persisted umodel element after reopen, got %+v", reopenedSnapshot.Elements)
	}
	reopenedRows, err := reopened.QueryEntities(ctx, model.EntityQueryPlan{
		Workspace: "demo",
		Filters:   map[string]any{"domain": "apm", "name": "apm.*", "ids": []string{"54013ba69c196820e56801f1ef5aad54"}},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("reopened query entity: %v", err)
	}
	if len(reopenedRows.Rows) != 1 {
		t.Fatalf("expected persisted entity after reopen, got %+v", reopenedRows.Rows)
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
