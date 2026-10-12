package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/agentgateway"
	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestRunQuickStartPreloadsMCPWorkspace(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_spl_execute","arguments":{"query":".entity with(domain='devops', name='devops.service', query='checkout') | limit 1"}}}`)
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run([]string{"--quickstart"}, input, &out, &errOut)
	if err != nil {
		t.Fatalf("run quickstart MCP: %v\nstderr:\n%s", err, errOut.String())
	}
	if !strings.Contains(errOut.String(), "quickstart loaded workspace=demo sample=multi-domain-quickstart") {
		t.Fatalf("expected quickstart preload log, got %s", errOut.String())
	}
	if !strings.Contains(out.String(), `"isError":false`) || !strings.Contains(out.String(), "checkout") {
		t.Fatalf("expected query tool to read quickstart sample data, got %s", out.String())
	}
}

func TestEncodeTOONCoversObjectsAndTables(t *testing.T) {
	got := encodeTOON(map[string]any{
		"workspace": "demo",
		"rows": []map[string]any{
			{"id": 1, "name": "checkout"},
			{"id": 2, "name": "payment"},
		},
		"tags": []string{"umodel", "mcp"},
	})
	for _, want := range []string{"workspace: demo", "rows[2]{id,name}:", "1,checkout", "tags[2]: umodel,mcp"} {
		if !strings.Contains(got, want) {
			t.Fatalf("TOON output missing %q:\n%s", want, got)
		}
	}
}

func TestMCPProtocolShapesUseTOONContent(t *testing.T) {
	app := testutil.ProviderApp(t, t.TempDir())
	ctx := context.Background()

	for _, payload := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","workspace":"demo"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"workspace":"demo"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/templates/list","params":{"workspace":"demo"}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"workspace":"demo","name":"query_spl_examples","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"workspace":"demo","uri":"umodel://workspace/demo/overview"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"prompts/list","params":{}}`,
		`{"jsonrpc":"2.0","id":7,"method":"completion/complete","params":{"ref":{"type":"ref/resource","uri":"umodel://workspace/{workspace}/overview"},"argument":{"name":"uri","value":"query"}}}`,
	} {
		resp, ok := handleRawRPC(ctx, app, "demo", []byte(payload))
		if !ok {
			t.Fatalf("expected response for %s", payload)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error for %s: %+v", payload, resp.Error)
		}
		body, _ := json.Marshal(resp.Result)
		if resp.ID == float64(4) || resp.ID == float64(5) {
			if !strings.Contains(string(body), toonMimeType) {
				t.Fatalf("expected TOON metadata for %s, got %s", payload, body)
			}
		}
	}
}

func TestStreamableHTTPBatchSkipsNotifications(t *testing.T) {
	app := testutil.ProviderApp(t, t.TempDir())
	responses, err := handleHTTPPayload(context.Background(), app, "demo", []byte(`[
		{"jsonrpc":"2.0","method":"notifications/initialized"},
		{"jsonrpc":"2.0","id":1,"method":"ping"}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 1 || responses[0].ID != float64(1) || responses[0].Error != nil {
		t.Fatalf("unexpected batch responses: %+v", responses)
	}
}

func TestStreamableHTTPPostEndpoint(t *testing.T) {
	app := testutil.ProviderApp(t, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","workspace":"demo"}}`))
	req.Header.Set("Origin", "http://127.0.0.1")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	rec := httptest.NewRecorder()

	streamableHTTPHandler(context.Background(), app, "demo").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "http://127.0.0.1/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"ping"}`))
	req.Header.Set("MCP-Protocol-Version", "1900-01-01")
	rec = httptest.NewRecorder()
	streamableHTTPHandler(context.Background(), app, "demo").ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid protocol header to fail, got %d", rec.Code)
	}
}

func TestMCPEnabledWriteToolsUseRealServices(t *testing.T) {
	app := testutil.ProviderApp(t, t.TempDir(), bootstrap.WithImportRoot("/"))
	if _, err := app.LoadQuickStart(context.Background(), bootstrap.QuickStartOptions{WorkspaceID: "demo"}); err != nil {
		t.Fatal(err)
	}
	app.AgentGateway = agentgateway.NewService(app.Query, agentgateway.WithWriteToolsEnabled(true), agentgateway.WithWriteServices(app.UModel, app.EntityStore))
	server := httptest.NewServer(streamableHTTPHandler(context.Background(), app, "demo"))
	defer server.Close()
	call := func(tool string, args map[string]any) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"workspace": "demo", "name": tool, "arguments": args}})
		response, err := http.Post(server.URL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var result map[string]any
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(result)
		if response.StatusCode != http.StatusOK || result["error"] != nil || bytes.Contains(payload, []byte(`"isError":true`)) {
			t.Fatalf("enabled MCP tool %s failed: %s", tool, payload)
		}
		return string(payload)
	}
	now := time.Now().Unix()
	id := strings.Repeat("e", 32)
	written := call("entity_write", map[string]any{"entities": []map[string]any{{"__domain__": "devops", "__entity_type__": "devops.service", "__entity_id__": id, "__method__": "Create", "__first_observed_time__": now, "__last_observed_time__": now, "display_name": "mcp-write-marker"}}})
	if !strings.Contains(written, `"accepted":1`) {
		t.Fatalf("MCP entity write did not reach EntityStore: %s", written)
	}
	if rows := call("query_spl_execute", map[string]any{"query": ".entity with(query='mcp-write-marker')"}); !strings.Contains(rows, "mcp-write-marker") {
		t.Fatalf("MCP query lost written entity: %s", rows)
	}
	if expired := call("entity_expire", map[string]any{"ids": []string{"devops/devops.service/" + id}}); !strings.Contains(expired, `"accepted":1`) {
		t.Fatalf("MCP entity expiration failed: %s", expired)
	}
	if rows := call("query_spl_execute", map[string]any{"query": ".entity with(query='mcp-write-marker')"}); !strings.Contains(rows, "rows[0]") {
		t.Fatalf("expired MCP entity remained visible: %s", rows)
	}
}
