package contract_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestGremlinGraphStoreConformance(t *testing.T) {
	app := testutil.GremlinApp(t, t.TempDir())
	exerciseGraphStoreWorkspace(t, app.GraphStore, testutil.GremlinWorkspace(t))
}

func TestGremlinLifecycleIsolationAndSerializationFailure(t *testing.T) {
	ctx := context.Background()
	app := testutil.GremlinApp(t, t.TempDir())
	workspace, other := testutil.GremlinWorkspace(t), testutil.GremlinWorkspace(t)
	store, memory := app.GraphStore, graphstore.NewMemoryStore()
	a, b := entity("a'quoted"), entity("b")
	a["__method__"], a["large_number"] = "Create", int64(9007199254740993)
	a["nested"] = map[string]any{"values": []any{true, int64(42)}}
	r := relation("a'quoted", "b")
	r["__method__"] = "Create"
	from, to := time.Unix(120, 0), time.Unix(150, 0)
	history := model.TimeRange{From: &from, To: &to}
	assertSame := func() {
		t.Helper()
		for _, timeRange := range []model.TimeRange{{}, history} {
			plan := model.EntityQueryPlan{Workspace: workspace, TimeRange: timeRange, Limit: 100}
			got, err := store.QueryEntities(ctx, plan)
			want, _ := memory.QueryEntities(ctx, plan)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("entities differ: got=%+v want=%+v err=%v", got, want, err)
			}
			for _, call := range []*model.GraphCallPlan{nil, {Name: "cypher", Cypher: "MATCH (s)-[r]->(d) RETURN s, r, d"}} {
				plan.GraphCall = call
				got, err = store.QueryTopo(ctx, plan)
				want, _ = memory.QueryTopo(ctx, plan)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("topology differs: got=%+v want=%+v err=%v", got, want, err)
				}
			}
		}
	}
	for _, s := range []contract.GraphStore{store, memory} {
		result, err := s.WriteRelations(ctx, model.RelationWriteBatch{Workspace: workspace, Relations: []model.RelationPayload{r, r}})
		if err != nil || result.Accepted != 1 || result.Failed != 1 {
			t.Fatalf("relation-first Create: %+v %v", result, err)
		}
	}
	assertSame()
	for _, s := range []contract.GraphStore{store, memory} {
		result, err := s.WriteEntities(ctx, model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{a, a, b}})
		if err != nil || result.Accepted != 2 || result.Failed != 1 {
			t.Fatalf("entity Create: %+v %v", result, err)
		}
	}
	assertSame()
	if _, err := store.WriteEntities(ctx, model.EntityWriteBatch{Workspace: other, Entities: []model.EntityPayload{a}}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"Update", "Expire", "Delete", "Create"} {
		a["__method__"], r["__method__"] = method, method
		for _, s := range []contract.GraphStore{store, memory} {
			if _, err := s.WriteEntities(ctx, model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{a}}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.WriteRelations(ctx, model.RelationWriteBatch{Workspace: workspace, Relations: []model.RelationPayload{r}}); err != nil {
				t.Fatal(err)
			}
		}
		assertSame()
	}
	bad := entity("bad")
	bad["unsupported"] = make(chan int)
	result, err := store.WriteEntities(ctx, model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{entity("rollback"), bad}})
	if err == nil || result.Accepted != 0 {
		t.Fatalf("serialization failure reported accepted writes: %+v %v", result, err)
	}
	assertSame()
	isolated, err := store.QueryEntities(ctx, model.EntityQueryPlan{Workspace: other, TimeRange: history})
	if err != nil || len(isolated.Rows) != 1 {
		t.Fatalf("workspace isolation: %+v %v", isolated, err)
	}
	if _, err := store.QueryTopo(ctx, model.TopoQueryPlan{Workspace: workspace, GraphCall: &model.GraphCallPlan{Name: "cypher", Cypher: "MATCH (n) DELETE n"}}); err == nil {
		t.Fatal("write Cypher was accepted")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.QueryEntities(ctx, model.EntityQueryPlan{Workspace: workspace}); err == nil {
		t.Fatal("cancelled read succeeded")
	}
}

func TestGremlinConcurrentCreateSingleProvider(t *testing.T) {
	app := testutil.GremlinApp(t, t.TempDir())
	workspace := testutil.GremlinWorkspace(t)
	var wg sync.WaitGroup
	results := make(chan model.WriteResult, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := entity("concurrent")
			e["__method__"] = "Create"
			result, err := app.GraphStore.WriteEntities(context.Background(), model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{e}})
			if err != nil {
				t.Error(err)
				return
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	accepted, failed := 0, 0
	for result := range results {
		accepted += result.Accepted
		failed += result.Failed
	}
	if accepted != 1 || failed != 11 {
		t.Fatalf("concurrent Create accepted=%d failed=%d", accepted, failed)
	}
}

func TestGremlinRejectsDuplicateRemoteRecords(t *testing.T) {
	app := testutil.GremlinApp(t, t.TempDir())
	workspace := testutil.GremlinWorkspace(t)
	testutil.GremlinSubmit(t, `2.times { g.addV('UModelEntity').property('workspace', workspace).property('key', 'duplicate').property('data', '{}').iterate() }; 1`, map[string]any{"workspace": workspace})
	if _, err := app.GraphStore.QueryEntities(context.Background(), model.EntityQueryPlan{Workspace: workspace}); err == nil {
		t.Fatal("duplicate remote keys were silently collapsed")
	}
}
