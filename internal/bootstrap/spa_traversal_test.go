package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestSPAHandlerRejectsTraversal verifies the SPA static file handler confines
// requests to uiDir and never serves a file outside it via path traversal.
func TestSPAHandlerRejectsTraversal(t *testing.T) {
	uiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiDir, "index.html"), []byte("<html>app</html>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	// A secret sibling of uiDir that must never be reachable.
	secret := filepath.Join(filepath.Dir(uiDir), "secret.txt")
	if err := os.WriteFile(secret, []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	handler := spaFileHandler(uiDir)

	for _, target := range []string{
		"/../secret.txt",
		"/../../secret.txt",
		"/%2e%2e/secret.txt",
		"/assets/../../secret.txt",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if got := rr.Body.String(); got == "TOP SECRET" {
			t.Fatalf("traversal %q leaked the secret file", target)
		}
	}

	// A normal request still serves index.html (SPA fallback intact).
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "<html>app</html>" {
		t.Fatalf("expected index.html for '/', got code=%d body=%q", rr.Code, rr.Body.String())
	}
}
