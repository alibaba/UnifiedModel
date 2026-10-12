package gremlin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestConfiguration(t *testing.T) {
	t.Setenv("GREMLIN_CA_FILE", "")
	t.Setenv("GREMLIN_URI", "wss://example.com:8182/gremlin")
	t.Setenv("GREMLIN_USERNAME", "environment-user")
	t.Setenv("GREMLIN_PASSWORD", "private-password")
	t.Setenv("GREMLIN_TRAVERSAL_SOURCE", "customGraph")
	t.Setenv("GREMLIN_TIMEOUT", "3s")
	c, err := configuration(nil)
	if err != nil || c.uri != "wss://example.com:8182/gremlin" || c.username != "environment-user" || c.password != "private-password" || c.traversalSource != "customGraph" || c.timeout != 3*time.Second {
		t.Fatalf("environment configuration not applied: %v", err)
	}
	c, err = configuration(map[string]string{"uri": "ws://localhost:18182/gremlin", "username": "", "password": "", "traversal_source": "g", "timeout": "1s"})
	if err != nil || c.username != "" || c.password != "" || c.traversalSource != "g" || c.timeout != time.Second {
		t.Fatalf("explicit options must override environment, including anonymous auth: %v", err)
	}
	for _, options := range []map[string]string{
		{"uri": "http://localhost:8182"}, {"uri": "ws://"}, {"uri": "ws://user:private-password@localhost:8182/gremlin"},
		{"uri": "ws://user:private-password@%"}, {"uri": "ws://localhost:8182/gremlin?password=private-password"},
		{"uri": "ws://localhost:8182/gremlin#fragment"}, {"username": ""}, {"password": ""},
		{"traversal_source": " "}, {"timeout": "0s"}, {"timeout": "-1s"}, {"timeout": "1ns"}, {"timeout": "25h"}, {"timeout": "invalid"},
	} {
		_, err := configuration(options)
		if err == nil {
			t.Fatal("invalid configuration accepted")
		}
		if strings.Contains(err.Error(), "private-password") {
			t.Fatal("configuration error exposed credentials")
		}
	}
}

func TestRegistryRejectsInvalidConfigurationWithoutConnecting(t *testing.T) {
	_, err := graphstore.NewProvider(graphstore.ProviderConfig{Type: graphstore.ProviderTypeGremlin, Options: map[string]string{"uri": "invalid"}})
	if err == nil || !strings.Contains(err.Error(), "gremlin:") {
		t.Fatalf("expected registered provider configuration error: %v", err)
	}
}

func TestDecodeSnapshotRetainsNumbersAndRejectsCorruption(t *testing.T) {
	row := func(kind, key, data string) *result {
		b, _ := json.Marshal(map[string]string{"kind": kind, "key": key, "data": data})
		return &result{Data: string(b)}
	}
	valid := row("entity", "devops/service/a", `{"large":9007199254740993,"nested":{"count":42},"__deleted__":true}`)
	snapshot, err := decodeSnapshot([]*result{valid})
	if err != nil || snapshot.Entities["devops/service/a"]["large"] != json.Number("9007199254740993") || snapshot.Entities["devops/service/a"]["__deleted__"] != true {
		t.Fatalf("raw payload changed: %+v %v", snapshot, err)
	}
	for _, rows := range [][]*result{
		{valid, valid}, {{Data: 1}}, {{Data: "not JSON"}},
		{row("entity", "", `{}`)}, {row("unknown", "a", `{}`)}, {row("entity", "a", `broken`)},
	} {
		if _, err := decodeSnapshot(rows); err == nil {
			t.Fatalf("corrupt/duplicate records accepted: %+v", rows)
		}
	}
}

func TestChangesEncodeWholeBatchBeforeRemoteWrite(t *testing.T) {
	snapshot := graphstore.WorkspaceSnapshot{Entities: map[string]model.EntityPayload{
		"good": {"display_name": "quote' and ${binding}"}, "bad": {"unsupported": make(chan int)},
	}}
	result := model.WriteResult{Accepted: 2, Items: []model.BatchItemResult{{ID: "good", OK: true}, {ID: "bad", OK: true}}}
	if rows, deleted, err := changes(entities, snapshot, result); err == nil || rows != nil || deleted != nil {
		t.Fatalf("serialization failure must produce no remote batch: %+v %v", rows, err)
	}
	result.Items = []model.BatchItemResult{{ID: "good", OK: true}, {ID: "good", OK: true}, {ID: "bad", OK: false}}
	rows, _, err := changes(entities, snapshot, result)
	if err != nil || len(rows) != 1 || rows[0]["key"] != "good" {
		t.Fatalf("batch must retain final state once per accepted key: %+v %v", rows, err)
	}
	rows, deleted, err := changes(models, graphstore.WorkspaceSnapshot{}, model.WriteResult{Accepted: 1, Items: []model.BatchItemResult{{ID: "removed", OK: true}}})
	if err != nil || len(rows) != 0 || len(deleted) != 1 || deleted[0] != "removed" {
		t.Fatalf("model deletion lost: %+v %+v %v", rows, deleted, err)
	}
}

func TestOperationGateRespectsCancellationAndTimeout(t *testing.T) {
	p := &Provider{timeout: 20 * time.Millisecond, gate: make(chan struct{}, 1)}
	_, unlock, err := p.lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, _, err := p.lock(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting for gate must time out: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := p.lock(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled operation acquired gate: %v", err)
	}
}

func TestGremlinCancelInFlightClosesProvider(t *testing.T) {
	if os.Getenv("UMODEL_TEST_GREMLIN") != "1" {
		t.Skip("set UMODEL_TEST_GREMLIN=1 to test real driver cancellation")
	}
	p, err := NewProvider(graphstore.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, unlock, err := p.lock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	_, err = p.submit(ctx, "Thread.sleep(5000); 1", nil)
	timer.Stop()
	unlock()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("in-flight cancellation did not return context error: %v", err)
	}
	if health, err := p.Health(context.Background()); err == nil || health.Status != "unavailable" {
		t.Fatalf("cancelled provider must be closed rather than replaying writes: %+v %v", health, err)
	}
}
