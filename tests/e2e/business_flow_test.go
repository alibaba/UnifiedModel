package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
)

func TestRESTBusinessFlowCoversCoreScenarios(t *testing.T) {
	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir(), bootstrap.WithImportRoot("/")).Handler())
	defer server.Close()

	created := e2ePost(t, server.URL+"/api/v1/workspaces", map[string]any{
		"id":     "demo",
		"name":   "Demo",
		"labels": map[string]string{"env": "e2e"},
	})
	if created["id"] != "demo" || created["status"] != "active" {
		t.Fatalf("unexpected workspace create response: %+v", created)
	}
	paths := created["paths"].(map[string]any)
	if !strings.HasSuffix(paths["root"].(string), "/instances/demo") {
		t.Fatalf("workspace root should be under instances/demo, got %+v", paths)
	}

	updated := e2ePut(t, server.URL+"/api/v1/workspaces/demo", map[string]any{
		"name":             "Demo Updated",
		"if_match_version": created["resource_version"],
	})
	if updated["name"] != "Demo Updated" {
		t.Fatalf("unexpected workspace update response: %+v", updated)
	}
	listed := e2eGet(t, server.URL+"/api/v1/workspaces")
	if len(e2eItems(t, listed)) != 1 {
		t.Fatalf("expected one active workspace, got %+v", listed)
	}

	validate := e2ePost(t, server.URL+"/api/v1/umodel/demo/validate", map[string]any{"elements": []map[string]any{{
		"kind":   "entity_set",
		"domain": "e2e",
		"name":   "custom",
	}}})
	if validate["valid"] != true {
		t.Fatalf("expected valid UModel element, got %+v", validate)
	}
	put := e2ePost(t, server.URL+"/api/v1/umodel/demo/elements", map[string]any{"elements": []map[string]any{{
		"kind":   "entity_set",
		"domain": "e2e",
		"name":   "custom",
	}}})
	if put["accepted"] != float64(1) {
		t.Fatalf("expected custom UModel put accepted, got %+v", put)
	}

	imported := e2ePost(t, server.URL+"/api/v1/umodel/demo/import", map[string]any{
		"path": filepath.Join("..", "..", "examples", "quickstart-multidomain"),
	})
	if imported["imported"].(float64) < 20 {
		t.Fatalf("expected quickstart import to load schemas, got %+v", imported)
	}
	umodelRows := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".umodel with(kind='entity_set', domain='devops', name='devops.service') | project domain,name,kind | limit 5",
	})
	if got := e2eRows(t, umodelRows); len(got) != 1 || got[0]["domain"] != "devops" || got[0]["name"] != "devops.service" || got[0]["kind"] != "entity_set" {
		t.Fatalf("expected imported devops.service row, got %+v", umodelRows)
	}

	e2ePost(t, server.URL+"/api/v1/entitystore/demo/entities:write", map[string]any{"entities": []map[string]any{
		entityPayload("10000000000000000000000000000101", "Create", 100, 200, map[string]any{"display_name": "cart v1"}),
		entityPayload("10000000000000000000000000000102", "Create", 100, 200, map[string]any{"display_name": "checkout"}),
	}})
	e2ePost(t, server.URL+"/api/v1/entitystore/demo/entities:write", map[string]any{"entities": []map[string]any{
		entityPayload("10000000000000000000000000000101", "Update", 999, 300, map[string]any{"display_name": "cart v2"}),
	}})
	currentEntity := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".entity with(domain='devops', name='devops.service', query='cart v2') | project __entity_id__,display_name,__first_observed_time__ | limit 5",
	})
	entityRows := e2eRows(t, currentEntity)
	if len(entityRows) != 1 || entityRows[0]["display_name"] != "cart v2" || entityRows[0]["__first_observed_time__"] != float64(100) {
		t.Fatalf("expected updated entity with preserved first_observed_time, got %+v", currentEntity)
	}

	e2ePost(t, server.URL+"/api/v1/entitystore/demo/entities:write", map[string]any{"entities": []map[string]any{
		entityPayload("10000000000000000000000000000101", "Expire", 0, 350, nil),
	}})
	expiredCurrent := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".entity with(domain='devops', name='devops.service', query='cart') | limit 5",
	})
	if len(e2eRows(t, expiredCurrent)) != 0 {
		t.Fatalf("expired entity should be hidden from current query, got %+v", expiredCurrent)
	}
	expiredHistorical := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query":      ".entity with(domain='devops', name='devops.service', query='cart') | project __entity_id__,__deleted__ | limit 5",
		"time_range": historicalRange(150, 360),
	})
	if rows := e2eRows(t, expiredHistorical); len(rows) != 1 || rows[0]["__deleted__"] != true {
		t.Fatalf("expired entity should be visible historically, got %+v", expiredHistorical)
	}

	e2ePost(t, server.URL+"/api/v1/entitystore/demo/entities:write", map[string]any{"entities": []map[string]any{
		entityPayload("10000000000000000000000000000103", "Create", 100, 200, map[string]any{"display_name": "orders"}),
		entityPayload("10000000000000000000000000000103", "Delete", 0, 380, nil),
	}})
	deletedHistorical := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query":      ".entity with(domain='devops', name='devops.service', query='orders') | limit 5",
		"time_range": historicalRange(150, 400),
	})
	if len(e2eRows(t, deletedHistorical)) != 0 {
		t.Fatalf("deleted entity should stay hidden, got %+v", deletedHistorical)
	}

	e2ePost(t, server.URL+"/api/v1/entitystore/demo/relations:write", map[string]any{"relations": []map[string]any{
		relationPayload("10000000000000000000000000000101", "10000000000000000000000000000102", "Create", 100, 200, map[string]any{"weight": 1}),
	}})
	e2ePost(t, server.URL+"/api/v1/entitystore/demo/relations:write", map[string]any{"relations": []map[string]any{
		relationPayload("10000000000000000000000000000101", "10000000000000000000000000000102", "Update", 100, 300, map[string]any{"weight": 2}),
	}})
	topoCurrent := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest,weight | limit 5",
	})
	topoRows := e2eRows(t, topoCurrent)
	if len(topoRows) != 1 || topoRows[0]["relation"] != "calls" || topoRows[0]["weight"] != float64(2) {
		t.Fatalf("expected current relation row, got %+v", topoCurrent)
	}

	e2ePost(t, server.URL+"/api/v1/entitystore/demo/relations:write", map[string]any{"relations": []map[string]any{
		relationPayload("10000000000000000000000000000101", "10000000000000000000000000000102", "Expire", 0, 350, nil),
	}})
	topoExpiredCurrent := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query": ".topo with(relation_type='calls') | limit 5",
	})
	if len(e2eRows(t, topoExpiredCurrent)) != 0 {
		t.Fatalf("expired relation should be hidden from current query, got %+v", topoExpiredCurrent)
	}
	topoExpiredHistorical := e2ePost(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
		"query":      ".topo with(relation_type='calls') | project src,relation,dest,__deleted__ | limit 5",
		"time_range": historicalRange(150, 360),
	})
	if rows := e2eRows(t, topoExpiredHistorical); len(rows) != 1 || rows[0]["__deleted__"] != true {
		t.Fatalf("expired relation should be visible historically, got %+v", topoExpiredHistorical)
	}

	explain := e2ePost(t, server.URL+"/api/v1/query/demo/explain", map[string]any{
		"query": ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 10",
	})
	if explain["source"] != ".topo" || explain["provider"] != "memory" || explain["depth"] != float64(2) {
		t.Fatalf("unexpected topo explain: %+v", explain)
	}

	for _, badQuery := range []string{
		"select * from entity",
		".entity with(domain)",
		".entity | limit 0",
		".topo | graph-call getNeighborNodes('full', -1, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})])",
		".topo | cypher | limit 1",
	} {
		payload := e2ePostStatus(t, server.URL+"/api/v1/query/demo/execute", map[string]any{"query": badQuery}, http.StatusBadRequest)
		if e2eErrorCode(t, payload) == "" {
			t.Fatalf("expected error code for bad query %q, got %+v", badQuery, payload)
		}
	}

	discovery := e2eGet(t, server.URL+"/api/v1/agent/demo/discover")
	if discovery["workspace"] != "demo" {
		t.Fatalf("unexpected discovery: %+v", discovery)
	}
	for _, rawResource := range discovery["resources"].([]any) {
		resource := rawResource.(map[string]any)
		read := e2ePost(t, server.URL+"/api/v1/agent/demo/resources:read", map[string]any{"uri": resource["uri"]})
		body, _ := json.Marshal(read["content"])
		if bytes.Contains(body, []byte("cart v2")) || bytes.Contains(body, []byte(`"rows"`)) {
			t.Fatalf("agent resource leaked runtime result data: %s", body)
		}
	}
	queryTool := e2ePost(t, server.URL+"/api/v1/agent/demo/tools:execute", map[string]any{
		"name":      "query_spl_execute",
		"arguments": map[string]any{"query": ".umodel with(kind='entity_set') | limit 1"},
	})
	if queryTool["ok"] != true {
		t.Fatalf("expected query_spl_execute success, got %+v", queryTool)
	}
	writeTool := e2ePostStatus(t, server.URL+"/api/v1/agent/demo/tools:execute", map[string]any{
		"name":      "entity_write",
		"arguments": map[string]any{"entities": []map[string]any{entityPayload("61326117ed4a9ddf3f754e71e119e5b3", "Create", 100, 200, nil)}},
	}, http.StatusBadRequest)
	if e2eErrorCode(t, writeTool) != "TOOL_DISABLED" {
		t.Fatalf("expected write tool to be disabled by default, got %+v", writeTool)
	}

	deleted := e2eDelete(t, server.URL+"/api/v1/workspaces/demo", nil)
	if deleted["status"] != "deleted" {
		t.Fatalf("expected workspace tombstone, got %+v", deleted)
	}
	tombstone := e2eGetStatus(t, server.URL+"/api/v1/workspaces/demo", http.StatusConflict)
	if e2eErrorCode(t, tombstone) != "WORKSPACE_TOMBSTONED" {
		t.Fatalf("expected tombstone error, got %+v", tombstone)
	}
}

func entityPayload(id, method string, first, last int64, fields map[string]any) map[string]any {
	payload := map[string]any{
		"__domain__":              "devops",
		"__entity_type__":         "devops.service",
		"__entity_id__":           id,
		"__method__":              method,
		"__first_observed_time__": first,
		"__last_observed_time__":  last,
		"__keep_alive_seconds__":  int64(60),
	}
	for key, value := range fields {
		payload[key] = value
	}
	return payload
}

func relationPayload(src, dest, method string, first, last int64, fields map[string]any) map[string]any {
	payload := map[string]any{
		"__src_domain__":          "devops",
		"__src_entity_type__":     "devops.service",
		"__src_entity_id__":       src,
		"__dest_domain__":         "devops",
		"__dest_entity_type__":    "devops.service",
		"__dest_entity_id__":      dest,
		"__relation_type__":       "calls",
		"__method__":              method,
		"__first_observed_time__": first,
		"__last_observed_time__":  last,
		"__keep_alive_seconds__":  int64(60),
	}
	for key, value := range fields {
		payload[key] = value
	}
	return payload
}

func historicalRange(from, to int64) map[string]any {
	return map[string]any{
		"from": time.Unix(from, 0).UTC().Format(time.RFC3339),
		"to":   time.Unix(to, 0).UTC().Format(time.RFC3339),
	}
}

func e2eGet(t *testing.T, url string) map[string]any {
	t.Helper()
	return e2eGetStatus(t, url, http.StatusOK)
}

func e2eGetStatus(t *testing.T, url string, allowedStatuses ...int) map[string]any {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	return e2eDecodeResponse(t, "get "+url, resp, allowedStatuses...)
}

func e2ePost(t *testing.T, url string, payload any) map[string]any {
	t.Helper()
	return e2ePostStatus(t, url, payload, http.StatusOK, http.StatusCreated)
}

func e2ePostStatus(t *testing.T, url string, payload any, allowedStatuses ...int) map[string]any {
	t.Helper()
	return e2eRequest(t, http.MethodPost, url, payload, allowedStatuses...)
}

func e2ePut(t *testing.T, url string, payload any) map[string]any {
	t.Helper()
	return e2eRequest(t, http.MethodPut, url, payload, http.StatusOK)
}

func e2eDelete(t *testing.T, url string, payload any) map[string]any {
	t.Helper()
	return e2eRequest(t, http.MethodDelete, url, payload, http.StatusOK)
}

func e2eRequest(t *testing.T, method, url string, payload any, allowedStatuses ...int) map[string]any {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode %s %s: %v", method, url, err)
		}
	}
	req, err := http.NewRequest(method, url, &body)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, url, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	return e2eDecodeResponse(t, method+" "+url, resp, allowedStatuses...)
}

func e2eDecodeResponse(t *testing.T, label string, resp *http.Response, allowedStatuses ...int) map[string]any {
	t.Helper()
	allowed := false
	for _, status := range allowedStatuses {
		if resp.StatusCode == status {
			allowed = true
			break
		}
	}
	if !allowed {
		t.Fatalf("%s returned %s", label, resp.Status)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode %s response: %v", label, err)
	}
	e2eNormalizeQueryExecutePayload(out)
	return out
}

func e2eRows(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawRows, ok := payload["rows"].([]any)
	if !ok {
		t.Fatalf("payload has no rows: %+v", payload)
	}
	rows := make([]map[string]any, 0, len(rawRows))
	for _, raw := range rawRows {
		row, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("row is not an object: %+v", raw)
		}
		rows = append(rows, row)
	}
	return rows
}

func e2eNormalizeQueryExecutePayload(payload map[string]any) {
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

func e2eItems(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawItems, ok := payload["items"].([]any)
	if !ok {
		t.Fatalf("payload has no items: %+v", payload)
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("item is not an object: %+v", raw)
		}
		items = append(items, item)
	}
	return items
}

func e2eErrorCode(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("payload has no error object: %+v", payload)
	}
	code, ok := raw["code"].(string)
	if !ok {
		t.Fatalf("error has no code: %+v", raw)
	}
	return code
}

func e2eJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode json %s: %v", body, err)
	}
	e2eNormalizeQueryExecutePayload(out)
	return out
}

func e2eString(value any) string {
	return fmt.Sprint(value)
}
