package gremlin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestGremlinAuthenticationTLSAndTraversalSource(t *testing.T) {
	uri, ca := os.Getenv("GREMLIN_SECURE_URI"), os.Getenv("GREMLIN_SECURE_CA")
	if uri == "" || ca == "" {
		t.Skip("set GREMLIN_SECURE_URI and GREMLIN_SECURE_CA for a real secure server")
	}
	options := func() map[string]string {
		return map[string]string{"uri": uri, "ca_file": ca, "username": "umodel-test-user", "password": "umodel-test-password", "timeout": "3s"}
	}
	p, err := NewProvider(graphstore.ProviderConfig{Options: options()})
	if err != nil {
		t.Fatalf("verified TLS and password authentication failed: %v", err)
	}
	defer p.Close()
	if health, err := p.Health(context.Background()); err != nil || health.Status != "ok" {
		t.Fatalf("secure health: %+v %v", health, err)
	}
	for _, tc := range []struct{ name, key, value string }{
		{"untrusted_certificate", "ca_file", ""},
		{"wrong_password", "password", "incorrect-secret"},
		{"wrong_username", "username", "unknown-user"},
		{"missing_traversal_source", "traversal_source", "missing_graph"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := options()
			opts[tc.key] = tc.value
			client, err := NewProvider(graphstore.ProviderConfig{Options: opts})
			if err == nil {
				_ = client.Close()
				t.Fatal("invalid secure connection was accepted")
			}
			if strings.Contains(err.Error(), "incorrect-secret") {
				t.Fatal("connection error exposed a password")
			}
		})
	}
}

func TestGremlinOversizedBatchDoesNotWriteAnyRecord(t *testing.T) {
	if os.Getenv("UMODEL_TEST_GREMLIN") != "1" {
		t.Skip("requires real Gremlin Server")
	}
	p, err := NewProvider(graphstore.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	w := fmt.Sprintf("gremlin-buffer-test-%d", time.Now().UnixNano())
	result, err := p.WriteEntities(context.Background(), model.EntityWriteBatch{Workspace: w, Entities: []model.EntityPayload{
		{"__domain__": "test", "__entity_type__": "service", "__entity_id__": "small", "__method__": "Create"},
		{"__domain__": "test", "__entity_type__": "service", "__entity_id__": "large", "__method__": "Create", "large": strings.Repeat("x", 1<<20)},
	}})
	if err == nil || result.Accepted != 0 {
		t.Fatalf("oversized batch must report no accepted records: %+v %v", result, err)
	}
	rows, err := p.QueryEntities(context.Background(), model.EntityQueryPlan{Workspace: w})
	if err != nil || len(rows.Rows) != 0 {
		t.Fatalf("oversized request partially wrote records: %+v %v", rows, err)
	}
}
