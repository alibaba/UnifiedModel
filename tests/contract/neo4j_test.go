package contract_test

import (
	"context"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestNeo4jGraphStoreConformance(t *testing.T) {
	app := testutil.Neo4jApp(t, t.TempDir())
	exerciseGraphStoreWorkspace(t, app.GraphStore, testutil.Neo4jWorkspace(t))
}

func TestNeo4jLifecycleIsolationAndRollback(t *testing.T) {
	ctx := context.Background()
	app := testutil.Neo4jApp(t, t.TempDir())
	workspace := testutil.Neo4jWorkspace(t)
	other := testutil.Neo4jWorkspace(t)
	store := app.GraphStore
	memory := graphstore.NewMemoryStore()
	a, b := entity("a"), entity("b")
	a["__method__"], a["large_number"] = "Create", int64(9007199254740993)
	a["nested"] = map[string]any{"values": []any{true, int64(42)}}
	r := relation("a", "b")
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
	// Relation-first writes create physical endpoints without exposing phantom
	// entity rows. Duplicate Create must return the same partial batch result.
	for _, s := range []contract.GraphStore{store, memory} {
		result, err := s.WriteRelations(ctx, model.RelationWriteBatch{Workspace: workspace, Relations: []model.RelationPayload{r, r}})
		if err != nil || result.Accepted != 1 || result.Failed != 1 {
			t.Fatalf("relation create: %+v %v", result, err)
		}
	}
	assertSame()
	for _, s := range []contract.GraphStore{store, memory} {
		result, err := s.WriteEntities(ctx, model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{a, a, b}})
		if err != nil || result.Accepted != 2 || result.Failed != 1 {
			t.Fatalf("entity create: %+v %v", result, err)
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
	// A serialization error after lifecycle evaluation must roll back the batch.
	bad := entity("bad")
	bad["unsupported"] = make(chan int)
	result, err := store.WriteEntities(ctx, model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{entity("rollback"), bad}})
	if err == nil || result.Accepted != 0 {
		t.Fatalf("expected rollback: %+v %v", result, err)
	}
	assertSame()
	isolated, err := store.QueryEntities(ctx, model.EntityQueryPlan{Workspace: other, TimeRange: history})
	if err != nil || len(isolated.Rows) != 1 {
		t.Fatalf("workspace isolation: %+v %v", isolated, err)
	}
	if _, err := store.QueryTopo(ctx, model.TopoQueryPlan{Workspace: workspace, GraphCall: &model.GraphCallPlan{Name: "cypher", Cypher: "MATCH (n) DELETE n"}}); err == nil {
		t.Fatal("write Cypher was accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.QueryEntities(cancelled, model.EntityQueryPlan{Workspace: workspace}); err == nil {
		t.Fatal("cancelled read succeeded")
	}
}

func TestNeo4jConcurrentCreateAcrossProviders(t *testing.T) {
	if os.Getenv("NEO4J_DIALECT") == "opencypher" {
		t.Skip("openCypher supports one writing provider instance per database")
	}
	first := testutil.Neo4jApp(t, t.TempDir())
	second := testutil.Neo4jApp(t, t.TempDir())
	exerciseConcurrentCreate(t, []contract.GraphStore{first.GraphStore, second.GraphStore})
}

func TestNeo4jConcurrentCreateSingleProvider(t *testing.T) {
	app := testutil.Neo4jApp(t, t.TempDir())
	exerciseConcurrentCreate(t, []contract.GraphStore{app.GraphStore})
}

func exerciseConcurrentCreate(t *testing.T, stores []contract.GraphStore) {
	t.Helper()
	workspace := testutil.Neo4jWorkspace(t)
	var wg sync.WaitGroup
	results := make(chan model.WriteResult, 12)
	for i := 0; i < 12; i++ {
		store := stores[i%len(stores)]
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := entity("concurrent")
			e["__method__"] = "Create"
			result, err := store.WriteEntities(context.Background(), model.EntityWriteBatch{Workspace: workspace, Entities: []model.EntityPayload{e}})
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
