package e2e_test

import (
	"bytes"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
)

func TestCLIBusinessFlowUsesOnlyPublicContracts(t *testing.T) {
	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir(), bootstrap.WithImportRoot("/")).Handler())
	defer server.Close()

	root := filepath.Join("..", "..")
	bin := buildUMCtl(t, root)
	tmp := t.TempDir()

	workspace := writeJSONFile(t, tmp, "workspace.json", map[string]any{"name": "CLI Demo", "labels": map[string]string{"env": "e2e"}})
	created := runUMCtlBinaryJSON(t, bin, server.URL, "workspace", "create", "cli-demo", workspace)
	if created["id"] != "cli-demo" || created["status"] != "active" {
		t.Fatalf("unexpected workspace create output: %+v", created)
	}
	got := runUMCtlBinaryJSON(t, bin, server.URL, "workspace", "get", "cli-demo")
	if got["name"] != "CLI Demo" {
		t.Fatalf("unexpected workspace get output: %+v", got)
	}

	updated := runUMCtlBinaryJSON(t, bin, server.URL, "workspace", "update", "cli-demo", `{"name":"CLI Demo Updated"}`)
	if updated["name"] != "CLI Demo Updated" {
		t.Fatalf("unexpected workspace update output: %+v", updated)
	}
	listed := runUMCtlBinaryJSON(t, bin, server.URL, "workspace", "list")
	if len(e2eItems(t, listed)) != 1 {
		t.Fatalf("expected one workspace in CLI list, got %+v", listed)
	}

	umodel := writeJSONFile(t, tmp, "umodel.json", map[string]any{
		"kind":   "entity_set",
		"domain": "devops",
		"name":   "devops.service",
	})
	if out := runUMCtlBinary(t, bin, server.URL, "umodel", "validate", "cli-demo", umodel); !bytes.Contains(out, []byte(`"valid":true`)) {
		t.Fatalf("expected umodel validate success, got %s", out)
	}
	if out := runUMCtlBinary(t, bin, server.URL, "umodel", "put", "cli-demo", umodel); !bytes.Contains(out, []byte(`"accepted":1`)) {
		t.Fatalf("expected umodel put accepted, got %s", out)
	}
	imported := runUMCtlBinaryJSON(t, bin, server.URL, "umodel", "import", "cli-demo", filepath.Join(root, "examples", "quickstart-multidomain", "devops", "entity_set", "devops.pipeline.yaml"))
	if imported["imported"] != float64(1) {
		t.Fatalf("expected one imported schema through CLI, got %+v", imported)
	}
	exported := runUMCtlBinaryJSON(t, bin, server.URL, "umodel", "export", "cli-demo", "10")
	if len(e2eRows(t, exported)) < 2 {
		t.Fatalf("expected CLI umodel export to query imported schemas, got %+v", exported)
	}

	entity := writeJSONFile(t, tmp, "entity.json", entityPayload("10000000000000000000000000000101", "Create", 100, 200, map[string]any{"display_name": "cart"}))
	if out := runUMCtlBinary(t, bin, server.URL, "entity", "write", "cli-demo", entity); !bytes.Contains(out, []byte(`"accepted":1`)) {
		t.Fatalf("expected entity write accepted, got %s", out)
	}
	entityQuery := runUMCtlBinaryJSON(t, bin, server.URL, "query", "run", "cli-demo", ".entity with(domain='devops', name='devops.service', query='cart') | project __entity_id__,display_name | limit 5")
	if rows := e2eRows(t, entityQuery); len(rows) != 1 || rows[0]["__entity_id__"] != "10000000000000000000000000000101" {
		t.Fatalf("expected CLI entity query row, got %+v", entityQuery)
	}

	topo := writeJSONFile(t, tmp, "topo.json", relationPayload("10000000000000000000000000000101", "10000000000000000000000000000102", "Create", 100, 200, nil))
	if out := runUMCtlBinary(t, bin, server.URL, "topo", "write", "cli-demo", topo); !bytes.Contains(out, []byte(`"accepted":1`)) {
		t.Fatalf("expected topo write accepted, got %s", out)
	}
	topoQuery := runUMCtlBinaryJSON(t, bin, server.URL, "query", "run", "cli-demo", ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest | limit 5")
	if rows := e2eRows(t, topoQuery); len(rows) != 1 || rows[0]["relation"] != "calls" {
		t.Fatalf("expected CLI topo query row, got %+v", topoQuery)
	}

	discovery := runUMCtlBinaryJSON(t, bin, server.URL, "agent", "discover", "cli-demo")
	if discovery["workspace"] != "cli-demo" || !strings.Contains(string(runUMCtlBinary(t, bin, server.URL, "query", "examples")), ".entity") {
		t.Fatalf("expected CLI discovery and query examples, got %+v", discovery)
	}
	tool := runUMCtlBinaryJSON(t, bin, server.URL, "agent", "tool", "cli-demo", "query_spl_explain", `{"query":".umodel | limit 1"}`)
	if tool["ok"] != true {
		t.Fatalf("expected CLI query tool success, got %+v", tool)
	}
	disabledOut, err := runUMCtlBinaryAllowError(bin, server.URL, "agent", "tool", "cli-demo", "entity_write", `{"entities":[]}`)
	if err == nil {
		t.Fatalf("expected disabled write tool to fail, got %s", disabledOut)
	}
	if !bytes.Contains(disabledOut, []byte("TOOL_DISABLED")) {
		t.Fatalf("expected disabled write tool error code, got %s", disabledOut)
	}

	if out := runUMCtlBinary(t, bin, server.URL, "entity", "expire", "cli-demo", "devops/devops.service/10000000000000000000000000000101"); !bytes.Contains(out, []byte(`"accepted":1`)) {
		t.Fatalf("expected entity expire accepted, got %s", out)
	}
	if out := runUMCtlBinary(t, bin, server.URL, "topo", "delete", "cli-demo", "devops/devops.service/10000000000000000000000000000101/calls/devops/devops.service/10000000000000000000000000000102"); !bytes.Contains(out, []byte(`"accepted":1`)) {
		t.Fatalf("expected topo delete accepted, got %s", out)
	}
	deleted := runUMCtlBinaryJSON(t, bin, server.URL, "workspace", "delete", "cli-demo")
	if deleted["status"] != "deleted" {
		t.Fatalf("expected workspace delete tombstone, got %+v", deleted)
	}
}

func buildUMCtl(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "umctl")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/umctl")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build umctl: %v\n%s", err, out)
	}
	return bin
}

func runUMCtlBinaryJSON(t *testing.T, bin, addr string, args ...string) map[string]any {
	t.Helper()
	return e2eJSONMap(t, runUMCtlBinary(t, bin, addr, args...))
}

func runUMCtlBinary(t *testing.T, bin, addr string, args ...string) []byte {
	t.Helper()
	out, err := runUMCtlBinaryAllowError(bin, addr, args...)
	if err != nil {
		t.Fatalf("umctl %v failed: %v\n%s", args, err, out)
	}
	return out
}

func runUMCtlBinaryAllowError(bin, addr string, args ...string) ([]byte, error) {
	all := append([]string{"--addr", addr}, args...)
	cmd := exec.Command(bin, all...)
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}
