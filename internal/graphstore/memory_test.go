package graphstore

import (
	"context"
	"testing"
	"time"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestMemoryStoreEntityAndTopoRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	if err := store.EnsureSchema(ctx, "demo"); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	_, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities: []model.EntityPayload{{
			"__domain__":              "apm",
			"__entity_type__":         "apm.service",
			"__entity_id__":           "54013ba69c196820e56801f1ef5aad54",
			"__method__":              "Update",
			"__first_observed_time__": int64(100),
			"__last_observed_time__":  int64(200),
			"__keep_alive_seconds__":  int64(60),
			"display_name":            "cart service",
		}},
	})
	if err != nil {
		t.Fatalf("write entities: %v", err)
	}

	entityResult, err := store.QueryEntities(ctx, model.EntityQueryPlan{
		Workspace: "demo",
		Limit:     10,
		Filters: map[string]any{
			"domain": "apm",
			"name":   "apm.service",
			"query":  "cart",
		},
	})
	if err != nil {
		t.Fatalf("query entities: %v", err)
	}
	if len(entityResult.Rows) != 1 || entityResult.Rows[0]["__entity_id__"] != "54013ba69c196820e56801f1ef5aad54" {
		t.Fatalf("unexpected entity rows: %+v", entityResult.Rows)
	}

	_, err = store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{{
			"__src_domain__":          "apm",
			"__src_entity_type__":     "apm.service",
			"__src_entity_id__":       "54013ba69c196820e56801f1ef5aad54",
			"__dest_domain__":         "apm",
			"__dest_entity_type__":    "apm.service",
			"__dest_entity_id__":      "177627f91af678a9b03e993f1a91917f",
			"__relation_type__":       "calls",
			"__method__":              "Update",
			"__first_observed_time__": int64(100),
			"__last_observed_time__":  int64(200),
			"__keep_alive_seconds__":  int64(60),
		}},
	})
	if err != nil {
		t.Fatalf("write relations: %v", err)
	}

	topoResult, err := store.QueryTopo(ctx, model.TopoQueryPlan{
		Workspace: "demo",
		Limit:     10,
		Filters:   map[string]any{"relation_type": "calls"},
	})
	if err != nil {
		t.Fatalf("query topo: %v", err)
	}
	if len(topoResult.Rows) != 1 || topoResult.Rows[0]["relation"] != "calls" {
		t.Fatalf("unexpected topo rows: %+v", topoResult.Rows)
	}
}

func TestMemoryStoreEntityMethodSemantics(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	create, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities:  []model.EntityPayload{entityPayload("54013ba69c196820e56801f1ef5aad54", "Create", 100, 200, map[string]any{"display_name": "cart"})},
	})
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	if create.Accepted != 1 || create.Items[0].ID != "apm/apm.service/54013ba69c196820e56801f1ef5aad54" {
		t.Fatalf("unexpected create result: %+v", create)
	}

	duplicate, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities:  []model.EntityPayload{entityPayload("54013ba69c196820e56801f1ef5aad54", "Create", 100, 200, nil)},
	})
	if err != nil {
		t.Fatalf("duplicate entity create: %v", err)
	}
	if duplicate.Failed != 1 || duplicate.Items[0].Code != string(apperrors.CodeAlreadyExists) {
		t.Fatalf("expected duplicate create failure, got %+v", duplicate)
	}

	if _, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities:  []model.EntityPayload{entityPayload("54013ba69c196820e56801f1ef5aad54", "Update", 999, 300, map[string]any{"display_name": "cart v2"})},
	}); err != nil {
		t.Fatalf("update entity: %v", err)
	}
	rows, err := store.QueryEntities(ctx, model.EntityQueryPlan{Workspace: "demo", Limit: 10, Filters: map[string]any{"ids": []string{"54013ba69c196820e56801f1ef5aad54"}}})
	if err != nil {
		t.Fatalf("query updated entity: %v", err)
	}
	if len(rows.Rows) != 1 || rows.Rows[0]["__first_observed_time__"] != int64(100) || rows.Rows[0]["display_name"] != "cart v2" {
		t.Fatalf("unexpected updated entity row: %+v", rows.Rows)
	}

	if _, err := store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities:  []model.EntityPayload{entityPayload("54013ba69c196820e56801f1ef5aad54", "Expire", 0, 350, nil)},
	}); err != nil {
		t.Fatalf("expire entity: %v", err)
	}
	current, err := store.QueryEntities(ctx, model.EntityQueryPlan{Workspace: "demo", Limit: 10, Filters: map[string]any{"ids": []string{"54013ba69c196820e56801f1ef5aad54"}}})
	if err != nil {
		t.Fatalf("query expired current entity: %v", err)
	}
	if len(current.Rows) != 0 {
		t.Fatalf("expired entity should be hidden in current query, got %+v", current.Rows)
	}
	from := time.Unix(150, 0)
	to := time.Unix(360, 0)
	history, err := store.QueryEntities(ctx, model.EntityQueryPlan{
		Workspace: "demo",
		Limit:     10,
		Filters:   map[string]any{"ids": []string{"54013ba69c196820e56801f1ef5aad54"}},
		TimeRange: model.TimeRange{From: &from, To: &to},
	})
	if err != nil {
		t.Fatalf("query expired historical entity: %v", err)
	}
	if len(history.Rows) != 1 || history.Rows[0]["__deleted__"] != true {
		t.Fatalf("expired entity should be visible historically, got %+v", history.Rows)
	}
}

func TestMemoryStoreRelationMethodSemantics(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	create, err := store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{relationPayload("54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f", "Create", 100, 200, nil)},
	})
	if err != nil {
		t.Fatalf("create relation: %v", err)
	}
	if create.Accepted != 1 || create.Items[0].ID != "apm/apm.service/54013ba69c196820e56801f1ef5aad54/calls/apm/apm.service/177627f91af678a9b03e993f1a91917f" {
		t.Fatalf("unexpected create relation result: %+v", create)
	}

	duplicate, err := store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{relationPayload("54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f", "Create", 100, 200, nil)},
	})
	if err != nil {
		t.Fatalf("duplicate relation create: %v", err)
	}
	if duplicate.Failed != 1 || duplicate.Items[0].Code != string(apperrors.CodeAlreadyExists) {
		t.Fatalf("expected duplicate relation create failure, got %+v", duplicate)
	}

	if _, err := store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{relationPayload("54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f", "Delete", 0, 350, nil)},
	}); err != nil {
		t.Fatalf("delete relation: %v", err)
	}
	from := time.Unix(150, 0)
	to := time.Unix(360, 0)
	deleted, err := store.QueryTopo(ctx, model.TopoQueryPlan{
		Workspace: "demo",
		Limit:     10,
		Filters:   map[string]any{"relation_type": "calls"},
		TimeRange: model.TimeRange{From: &from, To: &to},
	})
	if err != nil {
		t.Fatalf("query deleted relation: %v", err)
	}
	if len(deleted.Rows) != 0 {
		t.Fatalf("deleted relation should stay hidden, got %+v", deleted.Rows)
	}
}

func entityPayload(id, method string, first, last int64, fields map[string]any) model.EntityPayload {
	payload := model.EntityPayload{
		"__domain__":              "apm",
		"__entity_type__":         "apm.service",
		"__entity_id__":           id,
		"__method__":              method,
		"__first_observed_time__": first,
		"__last_observed_time__":  last,
		"__keep_alive_seconds__":  int64(60),
	}
	for key, value := range fields {
		payload[key] = value
	}
	return payload
}

func relationPayload(src, dest, method string, first, last int64, fields map[string]any) model.RelationPayload {
	payload := model.RelationPayload{
		"__src_domain__":          "apm",
		"__src_entity_type__":     "apm.service",
		"__src_entity_id__":       src,
		"__dest_domain__":         "apm",
		"__dest_entity_type__":    "apm.service",
		"__dest_entity_id__":      dest,
		"__relation_type__":       "calls",
		"__method__":              method,
		"__first_observed_time__": first,
		"__last_observed_time__":  last,
		"__keep_alive_seconds__":  int64(60),
	}
	for key, value := range fields {
		payload[key] = value
	}
	return payload
}

// TestMemoryStoreMaxLimitNotBelowDefaultProvider guards query portability: the
// memory store backs --quickstart / MCP / the demos, so a `limit` valid on the
// default local.ladybug provider (MaxLimit 1000) must not be rejected here.
// Memory itself caps higher (10000, in-memory headroom). Regression for
// "query limit exceeds provider capability" when memory advertised MaxLimit 100.
func TestMemoryStoreMaxLimitNotBelowDefaultProvider(t *testing.T) {
	caps, err := NewMemoryStore().Capabilities(context.Background())
	if err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	if caps.MaxLimit < 1000 {
		t.Fatalf("memory MaxLimit = %d, want >= 1000 so production-valid limits stay portable", caps.MaxLimit)
	}
}

// TestQueryRespectsContextCancellation verifies the memory store honors a
// cancelled context and aborts instead of scanning — the store-side half of the
// query-timeout contract (the Service sets the deadline; the store respects it).
func TestQueryRespectsContextCancellation(t *testing.T) {
	store := NewMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.QueryEntities(ctx, model.EntityQueryPlan{Workspace: "demo"}); err != context.Canceled {
		t.Fatalf("QueryEntities with cancelled ctx: got %v, want context.Canceled", err)
	}
	if _, err := store.QueryTopo(ctx, model.TopoQueryPlan{Workspace: "demo"}); err != context.Canceled {
		t.Fatalf("QueryTopo with cancelled ctx: got %v, want context.Canceled", err)
	}
}
