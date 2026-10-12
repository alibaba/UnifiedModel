package e2e_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/tests/graphfixture"
	"github.com/alibaba/UnifiedModel/tests/testutil"
)

func TestQuickstartWithUMCtl(t *testing.T) {
	server := httptest.NewServer(testutil.ProviderApp(t, t.TempDir()).Handler())
	defer server.Close()
	root := filepath.Join("..", "..")
	tmp := t.TempDir()

	workspace := writeJSONFile(t, tmp, "workspace.json", map[string]any{"name": "Demo"})
	umodel := writeJSONFile(t, tmp, "umodel.json", map[string]any{
		"kind":   "entity_set",
		"domain": "devops",
		"name":   "devops.service",
	})
	entity := writeJSONFile(t, tmp, "entity.json", map[string]any{
		"__domain__":              "devops",
		"__entity_type__":         "devops.service",
		"__entity_id__":           "10000000000000000000000000000101",
		"__method__":              "Update",
		"__first_observed_time__": 100,
		"__last_observed_time__":  200,
		"display_name":            "cart",
	})
	topo := writeJSONFile(t, tmp, "topo.json", map[string]any{
		"__src_domain__":          "devops",
		"__src_entity_type__":     "devops.service",
		"__src_entity_id__":       "10000000000000000000000000000101",
		"__dest_domain__":         "devops",
		"__dest_entity_type__":    "devops.service",
		"__dest_entity_id__":      "10000000000000000000000000000102",
		"__relation_type__":       "calls",
		"__method__":              "Update",
		"__first_observed_time__": 100,
		"__last_observed_time__":  200,
	})

	runUMCtl(t, root, server.URL, "workspace", "create", "demo", workspace)
	runUMCtl(t, root, server.URL, "umodel", "put", "demo", umodel)
	runUMCtl(t, root, server.URL, "umodel", "validate", "demo", umodel)
	runUMCtl(t, root, server.URL, "entity", "write", "demo", entity)
	runUMCtl(t, root, server.URL, "topo", "write", "demo", topo)

	out := runUMCtl(t, root, server.URL, "query", "run", "demo", ".umodel with(kind='entity_set') | limit 5")
	if !bytes.Contains(out, []byte("devops.service")) {
		t.Fatalf("expected umctl query output to include devops.service, got %s", out)
	}
	entityOut := runUMCtl(t, root, server.URL, "query", "run", "demo", ".entity with(domain='devops', name='devops.service', query='cart') | limit 5")
	if !bytes.Contains(entityOut, []byte("cart")) {
		t.Fatalf("expected entity query output to include cart, got %s", entityOut)
	}
	topoOut := runUMCtl(t, root, server.URL, "query", "run", "demo", ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 5")
	if !bytes.Contains(topoOut, []byte("calls")) {
		t.Fatalf("expected topo query output to include calls, got %s", topoOut)
	}
	explain := runUMCtl(t, root, server.URL, "query", "explain", "demo", ".entity with(domain='devops', name='devops.service') | limit 5")
	if !bytes.Contains(explain, []byte(graphfixture.Provider())) {
		t.Fatalf("expected query explain to include storage provider, got %s", explain)
	}

	agent := runUMCtl(t, root, server.URL, "query", "examples")
	if !bytes.Contains(agent, []byte(".entity")) {
		t.Fatalf("expected query examples, got %s", agent)
	}
	discovery := runUMCtl(t, root, server.URL, "agent", "discover", "demo")
	if !bytes.Contains(discovery, []byte("query_spl_execute")) {
		t.Fatalf("expected agent discovery tools, got %s", discovery)
	}
	tool := runUMCtl(t, root, server.URL, "agent", "tool", "demo", "query_spl_explain", `{"query":".umodel | limit 5"}`)
	if !bytes.Contains(tool, []byte(`"ok":true`)) {
		t.Fatalf("expected agent tool success, got %s", tool)
	}

	expired := runUMCtl(t, root, server.URL, "entity", "expire", "demo", "devops/devops.service/10000000000000000000000000000101")
	if !bytes.Contains(expired, []byte(`"accepted":1`)) {
		t.Fatalf("expected entity expire result, got %s", expired)
	}
	topoExpired := runUMCtl(t, root, server.URL, "topo", "delete", "demo", "devops/devops.service/10000000000000000000000000000101/calls/devops/devops.service/10000000000000000000000000000102")
	if !bytes.Contains(topoExpired, []byte(`"accepted":1`)) {
		t.Fatalf("expected topo delete result, got %s", topoExpired)
	}
}

func TestMCPStdioToolsAndResources(t *testing.T) {
	root := filepath.Join("..", "..")
	workspace := testutil.MCPWorkspace(t, "demo")
	cmd := exec.Command("go", "run", "./cmd/umodel-mcp", "--data", t.TempDir(), "--graphstore", graphfixture.Provider())
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.ReplaceAll(strings.Join([]string{
		`{"id":1,"method":"tools/list","params":{"workspace":"demo"}}`,
		`{"id":2,"method":"tools/call","params":{"workspace":"demo","name":"query_spl_examples","arguments":{}}}`,
		`{"id":3,"method":"resources/list","params":{"workspace":"demo"}}`,
		`{"id":4,"method":"resources/read","params":{"workspace":"demo","uri":"umodel://workspace/demo/query-templates"}}`,
	}, "\n"), "demo", workspace))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("umodel-mcp failed: %v\n%s", err, out)
	}
	if !bytes.Contains(out, []byte("query_spl_execute")) || !bytes.Contains(out, []byte("query_spl_examples")) || !bytes.Contains(out, []byte("query-templates")) {
		t.Fatalf("expected MCP output to include query tool and resources, got %s", out)
	}
}

func runUMCtl(t *testing.T, root, addr string, args ...string) []byte {
	t.Helper()
	all := append([]string{"run", "./cmd/umctl", "--addr", addr}, args...)
	cmd := exec.Command("go", all...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("umctl %v failed: %v\n%s", args, err, out)
	}
	return out
}

func writeJSONFile(t *testing.T, dir, name string, payload any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
