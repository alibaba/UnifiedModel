package integration_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
)

func TestHTTPQuickFlow(t *testing.T) {
	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir()).Handler())
	defer server.Close()

	root := get(t, server.URL+"/")
	if root["service"] != "umodel-server" {
		t.Fatalf("unexpected root response: %+v", root)
	}
	graphstore, ok := root["graphstore"].(map[string]any)
	if !ok || graphstore["provider"] != "memory" {
		t.Fatalf("root response should include graphstore health: %+v", root)
	}

	post(t, server.URL+"/api/v1/workspaces", map[string]any{"id": "demo"})
	post(t, server.URL+"/api/v1/umodel/demo/elements", map[string]any{"elements": []map[string]any{{
		"kind":   "entity_set",
		"domain": "devops",
		"name":   "devops.service",
	}}})
	post(t, server.URL+"/api/v1/entitystore/demo/entities:write", map[string]any{"entities": []map[string]any{{
		"__domain__":              "devops",
		"__entity_type__":         "devops.service",
		"__entity_id__":           "10000000000000000000000000000101",
		"__method__":              "Update",
		"__first_observed_time__": 100,
		"__last_observed_time__":  200,
		"display_name":            "cart",
	}}})
	post(t, server.URL+"/api/v1/entitystore/demo/relations:write", map[string]any{"relations": []map[string]any{{
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
	}}})

	umodelRows := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{"query": ".umodel with(kind='entity_set') | where domain = 'devops' | project domain,name,kind | sort name | limit 10"})
	if len(rows(t, umodelRows)) != 1 {
		t.Fatalf("expected one umodel row: %+v", umodelRows)
	}
	entityRows := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{"query": ".entity with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101'], topk=1) | project __entity_id__,display_name"})
	if len(rows(t, entityRows)) != 1 {
		t.Fatalf("expected one entity row: %+v", entityRows)
	}
	assertQueryEnvelope(t, entityRows, []string{"__entity_id__", "display_name"})
	topoRows := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{"query": ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest | limit 10"})
	if len(rows(t, topoRows)) != 1 {
		t.Fatalf("expected one topo row: %+v", topoRows)
	}
	cypherPayload := map[string]any{
		"query":      ".topo | graph-call cypher(`MATCH (svc:``devops@devops.service`` {__entity_id__: $svc}) OPTIONAL MATCH path = (svc)-[r*1..2]-(neighbor) WITH svc, neighbor, relationships(path) AS rels WHERE neighbor IS NULL OR coalesce(neighbor.__deleted__, false) = false RETURN svc.__entity_id__ AS service, neighbor.__entity_id__ AS neighbor, [rel IN rels | type(rel)] AS relation_types, size(rels) AS hops ORDER BY hops, neighbor LIMIT 20`) | limit 20",
		"parameters": map[string]any{"svc": "10000000000000000000000000000101"},
	}
	cypherRows := post(t, server.URL+"/api/v1/query/demo/execute", cypherPayload)
	cypherResultRows := rows(t, cypherRows)
	if len(cypherResultRows) == 0 || cypherResultRows[0].(map[string]any)["hops"] != float64(1) {
		t.Fatalf("expected cypher topo rows: %+v", cypherRows)
	}
	cypherExplain := post(t, server.URL+"/api/v1/query/demo/explain", cypherPayload)
	if cypherExplain["cypher_dialect"] != "ladybug" || cypherExplain["cypher_engine"] != "go" {
		t.Fatalf("unexpected cypher explain metadata: %+v", cypherExplain)
	}
	explain := post(t, server.URL+"/api/v1/query/demo/explain", map[string]any{"query": ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 10"})
	if explain["source"] != ".topo" || explain["provider"] != "memory" || explain["depth"].(float64) != 2 {
		t.Fatalf("unexpected explain: %+v", explain)
	}
	discovery := get(t, server.URL+"/api/v1/agent/demo/discover")
	if discovery["workspace"] != "demo" {
		t.Fatalf("unexpected discovery: %+v", discovery)
	}
	resources, ok := discovery["resources"].([]any)
	if !ok || len(resources) == 0 {
		t.Fatalf("expected structured discovery resources: %+v", discovery)
	}
	firstResource, ok := resources[0].(map[string]any)
	if !ok || firstResource["uri"] == "" {
		t.Fatalf("expected resource uri: %+v", resources[0])
	}
	resource := post(t, server.URL+"/api/v1/agent/demo/resources:read", map[string]any{"uri": firstResource["uri"]})
	if resource["uri"] != firstResource["uri"] || resource["content"] == nil {
		t.Fatalf("unexpected resource response: %+v", resource)
	}
	actions, ok := discovery["next_actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("expected query next actions: %+v", discovery)
	}
	action := actions[0].(map[string]any)
	queryAPI := action["query_api"].(map[string]any)
	if action["tool"] != "query_spl_execute" || queryAPI["path"] != "/api/v1/query/demo/execute" {
		t.Fatalf("next action should point at Query API: %+v", action)
	}
	toolResult := post(t, server.URL+"/api/v1/agent/demo/tools:execute", map[string]any{
		"name":      "query_spl_explain",
		"arguments": map[string]any{"query": ".umodel | limit 1"},
	})
	if toolResult["ok"] != true {
		t.Fatalf("expected query_spl_explain tool success: %+v", toolResult)
	}
}

func TestHTTPErrorContractsAndWriteToolDefault(t *testing.T) {
	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir()).Handler())
	defer server.Close()

	queryError := postStatus(t, server.URL+"/api/v1/query/demo/execute", map[string]any{"query": "select * from entity"}, http.StatusBadRequest)
	if errorCode(t, queryError) != "QUERY_PARSE_ERROR" {
		t.Fatalf("expected query parse error, got %+v", queryError)
	}

	entityError := postStatus(t, server.URL+"/api/v1/entitystore/demo/entities:write", map[string]any{"entities": []map[string]any{{
		"__domain__":      "devops",
		"__entity_type__": "devops.service",
		"__entity_id__":   "10000000000000000000000000000101",
		"__method__":      "Create",
	}}}, http.StatusBadRequest)
	if errorCode(t, entityError) != "VALIDATION_FAILED" {
		t.Fatalf("expected entity validation error, got %+v", entityError)
	}

	disabledTool := postStatus(t, server.URL+"/api/v1/agent/demo/tools:execute", map[string]any{
		"name":      "entity_write",
		"arguments": map[string]any{"entities": []map[string]any{}},
	}, http.StatusBadRequest)
	if errorCode(t, disabledTool) != "TOOL_DISABLED" {
		t.Fatalf("expected disabled write tool, got %+v", disabledTool)
	}

	discovery := get(t, server.URL+"/api/v1/agent/demo/discover")
	tools, ok := discovery["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools in discovery: %+v", discovery)
	}
	for _, rawTool := range tools {
		tool := rawTool.(map[string]any)
		if tool["name"] == "entity_write" && tool["enabled"] == true {
			t.Fatalf("entity_write should be disabled by default: %+v", tool)
		}
	}
}

func TestHTTPUModelImportThenQuery(t *testing.T) {
	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir()).Handler())
	defer server.Close()

	post(t, server.URL+"/api/v1/workspaces", map[string]any{"id": "demo"})
	importResult := post(t, server.URL+"/api/v1/umodel/demo/import", map[string]any{
		"path": filepath.Join("..", "..", "examples", "quickstart-multidomain", "devops", "entity_set", "devops.service.yaml"),
	})
	if importResult["imported"] != float64(1) {
		t.Fatalf("unexpected import result: %+v", importResult)
	}
	umodelRows := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".umodel with(kind='entity_set', domain='devops', name='devops.service') | limit 10",
	})
	if len(rows(t, umodelRows)) != 1 {
		t.Fatalf("expected imported umodel row: %+v", umodelRows)
	}
}

func TestHTTPSampleImportThenQuery(t *testing.T) {
	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir()).Handler())
	defer server.Close()

	post(t, server.URL+"/api/v1/workspaces", map[string]any{"id": "demo"})
	importResult := post(t, server.URL+"/api/v1/samples/demo/multi-domain-quickstart:import", map[string]any{})
	if importResult["sample"] != "multi-domain-quickstart" || importResult["entity_count"].(float64) == 0 || importResult["relation_count"].(float64) == 0 {
		t.Fatalf("unexpected sample import result: %+v", importResult)
	}
	entityRows := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".entity with(domain='devops', name='devops.service', query='checkout') | limit 10",
	})
	if len(rows(t, entityRows)) == 0 {
		t.Fatalf("expected sample service rows: %+v", entityRows)
	}
	topoRows := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 10",
	})
	if len(rows(t, topoRows)) == 0 {
		t.Fatalf("expected sample topology rows: %+v", topoRows)
	}
}

func TestHTTPServesSPAWhenUIDirConfigured(t *testing.T) {
	uiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiDir, "index.html"), []byte(`<html><body>UModel UI</body></html>`), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.Mkdir(filepath.Join(uiDir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "assets", "app.js"), []byte(`console.log("ok")`), 0o644); err != nil {
		t.Fatalf("write asset: %v", err)
	}

	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir()).HandlerWithUI(uiDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("get ui root: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected UI root 200, got %s", resp.Status)
	}

	resp, err = http.Get(server.URL + "/workspace/demo/explorer")
	if err != nil {
		t.Fatalf("get ui route: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected SPA route 200, got %s", resp.Status)
	}

	resp, err = http.Get(server.URL + "/assets/app.js")
	if err != nil {
		t.Fatalf("get ui asset: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected asset 200, got %s", resp.Status)
	}
}

func post(t *testing.T, url string, payload any) map[string]any {
	t.Helper()
	return postStatus(t, url, payload, http.StatusOK, http.StatusCreated)
}

func postStatus(t *testing.T, url string, payload any, allowedStatuses ...int) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatalf("encode: %v", err)
	}
	resp, err := http.Post(url, "application/json", &buf)
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer resp.Body.Close()
	allowed := false
	for _, status := range allowedStatuses {
		if resp.StatusCode == status {
			allowed = true
			break
		}
	}
	if !allowed {
		t.Fatalf("post %s returned %s", url, resp.Status)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	normalizeQueryExecutePayload(out)
	return out
}

func get(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		t.Fatalf("get %s returned %s", url, resp.Status)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func rows(t *testing.T, payload map[string]any) []any {
	t.Helper()
	rows, ok := payload["rows"].([]any)
	if !ok {
		t.Fatalf("payload has no rows: %+v", payload)
	}
	return rows
}

func assertQueryEnvelope(t *testing.T, payload map[string]any, wantHeader []string) {
	t.Helper()
	if payload["code"] != "200" || payload["message"] != "successful" || payload["success"] != true {
		t.Fatalf("unexpected query envelope: %+v", payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("query envelope has no data object: %+v", payload)
	}
	header, ok := data["header"].([]any)
	if !ok {
		t.Fatalf("query envelope has no header: %+v", payload)
	}
	if len(header) != len(wantHeader) {
		t.Fatalf("header length = %d, want %d: %+v", len(header), len(wantHeader), header)
	}
	for i, want := range wantHeader {
		if header[i] != want {
			t.Fatalf("header[%d] = %v, want %s: %+v", i, header[i], want, header)
		}
	}
	matrix, ok := data["data"].([]any)
	if !ok || len(matrix) == 0 {
		t.Fatalf("query envelope has no data matrix: %+v", payload)
	}
	status, ok := data["responseStatus"].(map[string]any)
	if !ok || status["result"] != "Success" || status["retryPolicy"] != "None" || status["level"] != "Info" {
		t.Fatalf("unexpected responseStatus: %+v", data["responseStatus"])
	}
}

func normalizeQueryExecutePayload(payload map[string]any) {
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return
	}
	rawHeader, ok := data["header"].([]any)
	if !ok {
		return
	}
	rawData, ok := data["data"].([]any)
	if !ok {
		return
	}
	header := make([]any, 0, len(rawHeader))
	headerStrings := make([]string, 0, len(rawHeader))
	for _, raw := range rawHeader {
		column, _ := raw.(string)
		header = append(header, column)
		headerStrings = append(headerStrings, column)
	}
	rows := make([]any, 0, len(rawData))
	for _, rawRow := range rawData {
		values, ok := rawRow.([]any)
		if !ok {
			continue
		}
		row := map[string]any{}
		for i, column := range headerStrings {
			if i < len(values) {
				row[column] = values[i]
			} else {
				row[column] = nil
			}
		}
		rows = append(rows, row)
	}
	payload["columns"] = header
	payload["rows"] = rows
}

func errorCode(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("payload has no error object: %+v", payload)
	}
	code, _ := raw["code"].(string)
	return code
}
