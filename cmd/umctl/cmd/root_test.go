package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/client"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/output"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

// exitError is used by tests to intercept response.ExitWithSuccess/Error
// instead of real os.Exit terminating the test process.
type exitError struct {
	code int
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit %d", e.code)
}

func setupTestEnv(t *testing.T, handler http.Handler) func() {
	t.Helper()
	server := httptest.NewServer(handler)
	apiClient = client.NewClient(server.URL)
	flagAddr = ""
	flagOutput = ""
	flagProfile = ""
	output.SetFormat("json")

	oldExit := response.ExitFunc
	response.ExitFunc = func(code int) {
		panic(&exitError{code: code})
	}

	return func() {
		server.Close()
		response.ExitFunc = oldExit
		apiClient = nil
		flagAddr = ""
		flagOutput = ""
		flagProfile = ""
		output.SetFormat("json")
	}
}

func setupExitOnlyEnv(t *testing.T) func() {
	t.Helper()
	apiClient = nil
	flagAddr = ""
	flagOutput = ""
	flagProfile = ""
	output.SetFormat("json")

	oldExit := response.ExitFunc
	response.ExitFunc = func(code int) {
		panic(&exitError{code: code})
	}

	return func() {
		response.ExitFunc = oldExit
		apiClient = nil
		flagAddr = ""
		flagOutput = ""
		flagProfile = ""
		output.SetFormat("json")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	out, _ := captureStdoutAndExitCode(t, fn)
	return out
}

func captureStdoutAndExitCode(t *testing.T, fn func()) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	exitCode := 0

	defer func() {
		os.Stdout = old
		r.Close()
	}()

	func() {
		defer func() {
			if rv := recover(); rv != nil {
				if e, ok := rv.(*exitError); ok {
					exitCode = e.code
				} else {
					panic(rv)
				}
			}
		}()
		fn()
	}()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String(), exitCode
}

func TestCLICommandsRouteToCorrectEndpoints(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		handler func(t *testing.T, r *http.Request)
	}{
		{
			name: "workspace list",
			args: []string{"workspace", "list"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodGet)
				assertPath(t, r, "/api/v1/workspaces")
			},
		},
		{
			name: "workspace get",
			args: []string{"workspace", "get", "demo"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodGet)
				assertPath(t, r, "/api/v1/workspaces/demo")
			},
		},
		{
			name: "workspace create",
			args: []string{"workspace", "create", "test-ws"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/workspaces")
				body := decodeBody(t, r)
				assertField(t, body, "id", "test-ws")
			},
		},
		{
			name: "workspace create with inline json",
			args: []string{"workspace", "create", "test-ws", `{"name":"My WS"}`},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				body := decodeBody(t, r)
				assertField(t, body, "name", "My WS")
				assertField(t, body, "id", "test-ws")
			},
		},
		{
			name: "workspace update with inline json",
			args: []string{"workspace", "update", "demo", `{"name":"Updated"}`},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPut)
				assertPath(t, r, "/api/v1/workspaces/demo")
			},
		},
		{
			name: "workspace delete",
			args: []string{"workspace", "delete", "demo"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodDelete)
				assertPath(t, r, "/api/v1/workspaces/demo")
			},
		},
		{
			name: "umodel import positional",
			args: []string{"umodel", "import", "demo", "/tmp/models"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/umodel/demo/import")
				body := decodeBody(t, r)
				assertField(t, body, "path", "/tmp/models")
			},
		},
		{
			name: "umodel import flag",
			args: []string{"umodel", "import", "-w", "demo", "--path", "/tmp/models"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/umodel/demo/import")
			},
		},
		{
			name: "umodel put wraps single element",
			args: []string{"umodel", "put", "demo", `{"kind":"entity_set","domain":"devops","name":"svc"}`},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/umodel/demo/elements")
				body := decodeBody(t, r)
				elements, ok := body["elements"].([]any)
				if !ok || len(elements) != 1 {
					t.Fatalf("expected wrapped elements, got %+v", body)
				}
			},
		},
		{
			name: "umodel put wraps array",
			args: []string{"umodel", "put", "demo", `[{"kind":"entity_set"}]`},
			handler: func(t *testing.T, r *http.Request) {
				body := decodeBody(t, r)
				elements, ok := body["elements"].([]any)
				if !ok || len(elements) != 1 {
					t.Fatalf("expected wrapped elements, got %+v", body)
				}
			},
		},
		{
			name: "umodel validate",
			args: []string{"umodel", "validate", "demo", `{"kind":"entity_set"}`},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/umodel/demo/validate")
			},
		},
		{
			name: "umodel delete positional ids",
			args: []string{"umodel", "delete", "demo", "id1,id2"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodDelete)
				assertPath(t, r, "/api/v1/umodel/demo/elements")
				body := decodeBody(t, r)
				ids, ok := body["ids"].([]any)
				if !ok || len(ids) != 2 {
					t.Fatalf("expected 2 ids, got %+v", body)
				}
			},
		},
		{
			name: "umodel export via query",
			args: []string{"umodel", "export", "demo", "10"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/query/demo/execute")
				body := decodeBody(t, r)
				assertField(t, body, "query", ".umodel | limit 10")
			},
		},
		{
			name: "entity write wraps single record",
			args: []string{"entity", "write", "demo", `{"__entity_id__":"abc123"}`},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/entitystore/demo/entities:write")
				body := decodeBody(t, r)
				entities, ok := body["entities"].([]any)
				if !ok || len(entities) != 1 {
					t.Fatalf("expected wrapped entities, got %+v", body)
				}
				item := entities[0].(map[string]any)
				assertField(t, item, "__entity_id__", "abc123")
			},
		},
		{
			name: "entity expire positional csv ids",
			args: []string{"entity", "expire", "demo", "id1,id2"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/entitystore/demo/entities:expire")
				body := decodeBody(t, r)
				ids := body["ids"].([]any)
				if len(ids) != 2 {
					t.Fatalf("expected 2 ids, got %+v", ids)
				}
			},
		},
		{
			name: "entity delete has delete reason",
			args: []string{"entity", "delete", "demo", "id1"},
			handler: func(t *testing.T, r *http.Request) {
				body := decodeBody(t, r)
				reason, _ := body["reason"].(string)
				if !strings.Contains(reason, "delete") {
					t.Fatalf("expected delete reason, got %q", reason)
				}
			},
		},
		{
			name: "topo write wraps relation",
			args: []string{"topo", "write", "demo", `{"__relation_type__":"calls"}`},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/entitystore/demo/relations:write")
				body := decodeBody(t, r)
				rels, ok := body["relations"].([]any)
				if !ok || len(rels) != 1 {
					t.Fatalf("expected wrapped relations, got %+v", body)
				}
			},
		},
		{
			name: "topo delete routes to expire with reason",
			args: []string{"topo", "delete", "demo", "rel1"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/entitystore/demo/relations:expire")
				body := decodeBody(t, r)
				reason, _ := body["reason"].(string)
				if !strings.Contains(reason, "delete") {
					t.Fatalf("expected delete reason, got %q", reason)
				}
			},
		},
		{
			name: "query run joins args",
			args: []string{"query", "run", "demo", ".umodel", "|", "limit", "5"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/query/demo/execute")
				body := decodeBody(t, r)
				assertField(t, body, "query", ".umodel | limit 5")
			},
		},
		{
			name: "query explain",
			args: []string{"query", "explain", "demo", ".entity | limit 1"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/query/demo/explain")
			},
		},
		{
			name: "agent discover",
			args: []string{"agent", "discover", "demo"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodGet)
				assertPath(t, r, "/api/v1/agent/demo/discover")
			},
		},
		{
			name: "agent tool positional",
			args: []string{"agent", "tool", "demo", "query_spl_execute", `{"query":".umodel | limit 1"}`},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/agent/demo/tools:execute")
				body := decodeBody(t, r)
				assertField(t, body, "name", "query_spl_execute")
				args, ok := body["arguments"].(map[string]any)
				if !ok || args["query"] != ".umodel | limit 1" {
					t.Fatalf("unexpected tool arguments: %+v", body)
				}
			},
		},
		{
			name: "health",
			args: []string{"health"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodGet)
				assertPath(t, r, "/healthz")
			},
		},
		{
			name: "sample import",
			args: []string{"sample", "import", "-w", "demo", "quickstart"},
			handler: func(t *testing.T, r *http.Request) {
				assertMethod(t, r, http.MethodPost)
				assertPath(t, r, "/api/v1/samples/demo/quickstart:import")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.handler(t, r)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"ok":true}`))
			}))
			defer cleanup()

			out := captureStdout(t, func() {
				rootCmd.SetArgs(tt.args)
				rootCmd.Execute()
			})

			if !strings.Contains(out, `"ok":true`) {
				t.Fatalf("expected {\"ok\":true} in output, got %q", out)
			}
		})
	}
}

func TestCLIGuardsRejectForbiddenCommands(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"entity get", []string{"entity", "get", "demo"}},
		{"entity list", []string{"entity", "list", "demo"}},
		{"entity search", []string{"entity", "search", "demo"}},
		{"topo neighbors", []string{"topo", "neighbors", "demo"}},
		{"topo subgraph", []string{"topo", "subgraph", "demo"}},
		{"topo path", []string{"topo", "path", "demo"}},
		{"umodel get", []string{"umodel", "get", "demo"}},
		{"umodel list", []string{"umodel", "list", "demo"}},
		{"umodel graph", []string{"umodel", "graph", "demo"}},
		{"workspace start", []string{"workspace", "start", "demo"}},
		{"workspace backup", []string{"workspace", "backup", "demo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetArgs(tc.args)
			err := rootCmd.Execute()
			if err == nil {
				t.Fatalf("expected %q to fail", tc.name)
			}
			if !strings.Contains(err.Error(), "forbidden") {
				t.Fatalf("expected forbidden message, got %q", err.Error())
			}
		})
	}
}

func TestCLIHandlesServerError(t *testing.T) {
	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer cleanup()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"workspace", "list"})
		rootCmd.Execute()
	})

	var resp response.ErrorResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("expected JSON error response, got %q: %v", out, err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
	if !strings.Contains(resp.Error, "boom") {
		t.Fatalf("expected server error in response, got %q", resp.Error)
	}
}

func TestCLIFailsFastWhenConfiguredAddrIsMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".umctl")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	configData := []byte("current: manual\nprofiles:\n  manual: {}\n")
	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		t.Fatal(err)
	}

	cleanup := setupExitOnlyEnv(t)
	defer cleanup()

	out, code := captureStdoutAndExitCode(t, func() {
		rootCmd.SetArgs([]string{"health"})
		rootCmd.Execute()
	})

	if code != response.ExitParam {
		t.Fatalf("expected exit code %d, got %d; output=%s", response.ExitParam, code, out)
	}
	if !strings.Contains(out, "No UModel server address configured") || !strings.Contains(out, "umctl configure") {
		t.Fatalf("expected missing addr guidance, got %q", out)
	}
}

func TestCLIExitsNonZeroOnBusinessFailure(t *testing.T) {
	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"accepted":0,"failed":1,"items":[{"id":"x","ok":false,"code":"NOT_IMPLEMENTED"}]}`))
	}))
	defer cleanup()

	out, code := captureStdoutAndExitCode(t, func() {
		rootCmd.SetArgs([]string{"workspace", "list"})
		rootCmd.Execute()
	})

	if code != response.ExitOperation {
		t.Fatalf("expected exit code %d, got %d; output=%s", response.ExitOperation, code, out)
	}
	if !strings.Contains(out, `"failed":1`) || !strings.Contains(out, "NOT_IMPLEMENTED") {
		t.Fatalf("expected original failure response in output, got %q", out)
	}
}

func TestTextOutputUsesReadableFallback(t *testing.T) {
	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","graphstore":{"provider":"memory","status":"ok"}}`))
	}))
	defer cleanup()

	out, code := captureStdoutAndExitCode(t, func() {
		rootCmd.SetArgs([]string{"--output", "text", "health"})
		rootCmd.Execute()
	})

	if code != 0 {
		t.Fatalf("expected exit 0, got %d; output=%s", code, out)
	}
	if !strings.Contains(out, "status: ok") || strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("expected readable text output, got %q", out)
	}
}

func TestConfigureShowAndListWriteToStdout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".umctl")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "config.yaml")
	configData := []byte("current: manual\noutput_format: text\nprofiles:\n  manual:\n    addr: http://example.test\n")
	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		t.Fatal(err)
	}

	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("configure show/list should not hit the server")
	}))
	defer cleanup()

	showOut := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"configure", "show"})
		rootCmd.Execute()
	})
	if !strings.Contains(showOut, "Profile:       manual") || !strings.Contains(showOut, "Server Addr:   http://example.test") {
		t.Fatalf("expected configure show on stdout, got %q", showOut)
	}

	listOut := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"configure", "list"})
		rootCmd.Execute()
	})
	if !strings.Contains(listOut, "* manual") || !strings.Contains(listOut, "addr=http://example.test") {
		t.Fatalf("expected configure list on stdout, got %q", listOut)
	}
}

func TestConfigureReadsMultiplePromptValuesFromOneInputStream(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("configure should not hit the server")
	}))
	defer cleanup()

	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("http://example.test\ntext\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	captureStdout(t, func() {
		rootCmd.SetArgs([]string{"--profile", "manual", "configure"})
		rootCmd.Execute()
	})

	data, err := os.ReadFile(filepath.Join(home, ".umctl", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	configText := string(data)
	if !strings.Contains(configText, "addr: http://example.test") || !strings.Contains(configText, "output_format: text") {
		t.Fatalf("expected both prompt values saved, got:\n%s", configText)
	}
}

func TestQueryExamplesDoesNotRequireServer(t *testing.T) {
	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("query examples should not hit the server")
	}))
	defer cleanup()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"query", "examples"})
		rootCmd.Execute()
	})

	if !strings.Contains(out, ".entity") {
		t.Fatalf("expected query examples in output, got %q", out)
	}
}

func TestMetaExportReturnsCommandRegistry(t *testing.T) {
	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("meta export should not hit the server")
	}))
	defer cleanup()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"meta", "export"})
		rootCmd.Execute()
	})

	var commands []map[string]any
	if err := json.Unmarshal([]byte(out), &commands); err != nil {
		t.Fatalf("expected JSON array from meta export, got %q: %v", out, err)
	}
	if len(commands) == 0 {
		t.Fatal("expected non-empty command registry")
	}
}

// Helper functions

func assertMethod(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if r.Method != want {
		t.Fatalf("expected method %s, got %s", want, r.Method)
	}
}

func assertPath(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if r.URL.Path != want {
		t.Fatalf("expected path %s, got %s", want, r.URL.Path)
	}
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
	}
	return body
}

func assertField(t *testing.T, body map[string]any, key string, want any) {
	t.Helper()
	if body[key] != want {
		t.Fatalf("expected %s=%v, got %v", key, want, body[key])
	}
}
