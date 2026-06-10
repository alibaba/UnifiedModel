package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestCapabilitiesReportsPlanOnly(t *testing.T) {
	handler := NewMemoryApp(t.TempDir()).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var payload struct {
		Service        string   `json:"service"`
		Version        string   `json:"version"`
		ModesSupported []string `json:"modes_supported"`
		DefaultMode    string   `json:"default_mode"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if payload.Service != "unified-model" {
		t.Fatalf("service = %q, want %q", payload.Service, "unified-model")
	}
	if payload.DefaultMode != "plan" {
		t.Fatalf("default_mode = %q, want %q", payload.DefaultMode, "plan")
	}
	if len(payload.ModesSupported) != 1 || payload.ModesSupported[0] != "plan" {
		t.Fatalf("modes_supported = %+v, want [plan]", payload.ModesSupported)
	}
	if payload.Version == "" {
		t.Fatalf("version should not be empty")
	}
}

func TestCapabilitiesRejectsNonGET(t *testing.T) {
	handler := NewMemoryApp(t.TempDir()).Handler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/capabilities", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code < 400 {
		t.Fatalf("expected 4xx for POST, got %d", rr.Code)
	}
}

func TestQueryEndpointRejectsModeDataInQueryParam(t *testing.T) {
	app := NewMemoryApp(t.TempDir())
	if _, err := app.Workspace.CreateWorkspace(context.Background(), model.CreateWorkspaceRequest{ID: "demo", Name: "Demo"}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	handler := app.Handler()

	body := `{"query":".umodel | limit 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute?mode=data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code < 400 {
		t.Fatalf("expected 4xx for mode=data, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestQueryEndpointAcceptsModePlanInQueryParam(t *testing.T) {
	app := NewMemoryApp(t.TempDir())
	if _, err := app.Workspace.CreateWorkspace(context.Background(), model.CreateWorkspaceRequest{ID: "demo", Name: "Demo"}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	handler := app.Handler()

	body := `{"query":".umodel | limit 1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query/demo/execute?mode=plan", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", rr.Code, http.StatusOK, rr.Body.String())
	}
}
