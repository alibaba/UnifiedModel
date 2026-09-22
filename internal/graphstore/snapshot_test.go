package graphstore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestSnapshotRetainsLifecycleAndNumericPrecision(t *testing.T) {
	ctx := context.Background()
	payload := model.EntityPayload{
		"__domain__": "test", "__entity_type__": "service", "__entity_id__": "one",
		"__method__": "Expire", "__deleted__": true,
		"__first_observed_time__": json.Number("100"), "__last_observed_time__": json.Number("200"),
		"__keep_alive_seconds__": json.Number("0"), "large_number": json.Number("9007199254740993"),
	}
	key := EntityKey(payload)
	s := NewMemoryStoreFromSnapshot("one", WorkspaceSnapshot{Entities: map[string]model.EntityPayload{key: payload}})
	current, err := s.QueryEntities(ctx, model.EntityQueryPlan{Workspace: "one"})
	if err != nil || len(current.Rows) != 0 {
		t.Fatalf("restoring a tombstone revived it: %+v %v", current, err)
	}
	from, to := time.Unix(120, 0), time.Unix(150, 0)
	history, err := s.QueryEntities(ctx, model.EntityQueryPlan{Workspace: "one", TimeRange: model.TimeRange{From: &from, To: &to}})
	if err != nil || len(history.Rows) != 1 || history.Rows[0]["large_number"] != int64(9007199254740993) {
		t.Fatalf("history or integer precision was lost: %+v %v", history, err)
	}
	snapshot := s.SnapshotWorkspace("one")
	snapshot.Entities[key]["__deleted__"] = false
	if s.SnapshotWorkspace("one").Entities[key]["__deleted__"] != true {
		t.Fatal("snapshot mutation changed evaluator state")
	}
	if _, err := s.WriteEntities(ctx, model.EntityWriteBatch{Workspace: "one", Entities: []model.EntityPayload{{"__entity_id__": "two"}}}); err != nil {
		t.Fatalf("missing collections were not initialized: %v", err)
	}
}
