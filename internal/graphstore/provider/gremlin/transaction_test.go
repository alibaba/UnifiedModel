package gremlin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
)

func TestGremlinServerTransactionRollsBackFailedWriteScript(t *testing.T) {
	uri := os.Getenv("GREMLIN_TX_URI")
	if uri == "" {
		t.Skip("set GREMLIN_TX_URI for a transaction-capable real server")
	}
	p, err := NewProvider(graphstore.ProviderConfig{Options: map[string]string{"uri": uri, "ca_file": "", "username": "", "password": ""}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, unlock, err := p.lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	rows, err := p.submit(ctx, `g.getGraph().features().graph().supportsTransactions()`, nil)
	if err != nil || len(rows) != 1 || rows[0].Data != true {
		t.Fatalf("test backend must support transactions: %+v %v", rows, err)
	}
	w := fmt.Sprintf("gremlin-rollback-test-%d", time.Now().UnixNano())
	// Inject a server exception after writing the first record through the
	// production write script, exercising the server's actual rollback path.
	script := saveScript
	index := strings.LastIndex(script, "rows.each { row ->")
	script = script[:index] + strings.Replace(script[index:], "rows.each { row ->", "rows.each { row ->\nif (row.key == 'fail') throw new IllegalStateException('injected backend failure')", 1)
	_, err = p.submit(ctx, script, map[string]any{"workspace": w, "kind": "entity", "deleted": []string{}, "rows": []map[string]any{
		{"key": "first", "data": `{"__domain__":"test","__entity_type__":"service","__entity_id__":"first"}`},
		{"key": "fail", "data": `{}`},
	}})
	if err == nil {
		t.Fatal("injected server failure was accepted")
	}
	snapshot, err := p.loadSnapshot(ctx, w, entities, nil)
	if err != nil || len(snapshot.Entities) != 0 {
		t.Fatalf("failed server transaction retained a partial write: %+v %v", snapshot, err)
	}
	// A subsequent request must still be able to write and read committed data.
	_, err = p.submit(ctx, saveScript, map[string]any{"workspace": w, "kind": "entity", "deleted": []string{}, "rows": []map[string]any{
		{"key": "committed", "data": `{"__domain__":"test","__entity_type__":"service","__entity_id__":"committed"}`},
	}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = p.loadSnapshot(ctx, w, entities, nil)
	if err != nil || len(snapshot.Entities) != 1 {
		t.Fatalf("subsequent transaction did not commit: %+v %v", snapshot, err)
	}
	_, err = p.submit(ctx, `g.V().hasLabel('UModelEntity').has('workspace', workspace).drop().iterate(); 1`, map[string]any{"workspace": w})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGremlinNonTransactionalFailureRetainsPartialWrite(t *testing.T) {
	if os.Getenv("UMODEL_TEST_GREMLIN") != "1" {
		t.Skip("requires a real non-transactional Gremlin Server")
	}
	p, err := NewProvider(graphstore.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, unlock, err := p.lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	features, err := p.submit(ctx, `g.getGraph().features().graph().supportsTransactions()`, nil)
	if err != nil || len(features) != 1 {
		t.Fatalf("backend features: %+v %v", features, err)
	}
	if features[0].Data == true {
		t.Skip("this assertion applies to non-transactional graphs")
	}
	w := fmt.Sprintf("gremlin-nontx-test-%d", time.Now().UnixNano())
	defer func() {
		_, err := p.submit(ctx, `g.V().hasLabel('UModelEntity').has('workspace', workspace).drop().iterate(); 1`, map[string]any{"workspace": w})
		if err != nil {
			t.Error(err)
		}
	}()
	_, err = p.submit(ctx, `g.addV('UModelEntity').property('workspace', workspace).property('key', 'first').property('data', '{}').iterate(); throw new IllegalStateException('injected backend failure')`, map[string]any{"workspace": w})
	if err == nil {
		t.Fatal("injected backend failure succeeded")
	}
	snapshot, err := p.loadSnapshot(ctx, w, entities, nil)
	if err != nil || len(snapshot.Entities) != 1 {
		t.Fatalf("non-transactional failure behavior changed: %+v %v", snapshot, err)
	}
}
