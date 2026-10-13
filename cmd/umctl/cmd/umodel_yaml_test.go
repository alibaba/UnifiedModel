package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestUModelValidateAcceptsSchemaStyleYAMLFile(t *testing.T) {
	yamlPath := filepath.Join(t.TempDir(), "service.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
kind: entity_set
schema:
  version: v0.1.0
metadata:
  name: devops.service
  domain: devops
spec:
  fields:
    - name: id
      type: string
`), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cleanup := setupTestEnv(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertMethod(t, r, http.MethodPost)
		assertPath(t, r, "/api/v1/umodel/demo/validate")
		body := decodeBody(t, r)
		elements, ok := body["elements"].([]any)
		if !ok || len(elements) != 1 {
			t.Fatalf("expected one normalized element, got %+v", body)
		}
		element, ok := elements[0].(map[string]any)
		if !ok {
			t.Fatalf("expected object element, got %+v", elements[0])
		}
		assertField(t, element, "kind", "entity_set")
		assertField(t, element, "domain", "devops")
		assertField(t, element, "name", "devops.service")
		assertField(t, element, "version", "v0.1.0")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"valid":true}`))
	}))
	defer cleanup()

	out, code := captureStdoutAndExitCode(t, func() {
		rootCmd.SetArgs([]string{"umodel", "validate", "demo", yamlPath})
		rootCmd.Execute()
	})
	if code != 0 {
		t.Fatalf("exit code = %d, output = %s", code, out)
	}
}
