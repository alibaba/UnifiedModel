package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
)

func TestCapabilityGate(t *testing.T) {
	server := httptest.NewServer(bootstrap.NewMemoryApp(t.TempDir()).Handler())
	defer server.Close()

	post(t, server.URL+"/api/v1/workspaces", map[string]any{"id": "demo"})
	importResult := post(t, server.URL+"/api/v1/samples/demo/multi-domain-quickstart:import", map[string]any{})

	if importResult["sample"] != "multi-domain-quickstart" {
		t.Fatalf("unexpected sample: %+v", importResult)
	}
	if importResult["entity_count"].(float64) < 90 {
		t.Fatalf("expected >=90 entities, got %v", importResult["entity_count"])
	}
	if importResult["relation_count"].(float64) < 120 {
		t.Fatalf("expected >=120 relations, got %v", importResult["relation_count"])
	}

	t.Run("ModelDiscovery", func(t *testing.T) {
		allEntitySets := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".umodel with(kind='entity_set') | limit 100",
		})
		if got := len(rows(t, allEntitySets)); got < 35 {
			t.Fatalf("expected >=35 entity_sets, got %d", got)
		}

		allLinks := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".umodel with(kind='entity_set_link') | limit 100",
		})
		if got := len(rows(t, allLinks)); got < 42 {
			t.Fatalf("expected >=42 entity_set_links, got %d", got)
		}

		devopsEntitySets := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".umodel with(kind='entity_set') | where domain = 'devops' | limit 100",
		})
		if got := len(rows(t, devopsEntitySets)); got != 10 {
			t.Fatalf("expected 10 devops entity_sets, got %d", got)
		}

		k8sEntitySets := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".umodel with(kind='entity_set') | where domain = 'k8s' | limit 100",
		})
		if got := len(rows(t, k8sEntitySets)); got != 7 {
			t.Fatalf("expected 7 k8s entity_sets, got %d", got)
		}

		pipeline := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".umodel with(kind='entity_set') | where domain = 'devops' | project domain,name,kind | sort name | limit 3",
		})
		pipelineRows := rows(t, pipeline)
		if len(pipelineRows) != 3 {
			t.Fatalf("expected 3 rows from pipeline, got %d", len(pipelineRows))
		}
		for _, r := range pipelineRows {
			row := r.(map[string]any)
			if row["domain"] != "devops" || row["kind"] != "entity_set" {
				t.Fatalf("unexpected pipeline row: %+v", row)
			}
		}
		cols := pipeline["columns"].([]any)
		if len(cols) != 3 {
			t.Fatalf("expected 3 projected columns, got %+v", cols)
		}
	})

	t.Run("EntityQuery", func(t *testing.T) {
		devopsServices := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".entity with(domain='devops', name='devops.service') | limit 20",
		})
		if got := len(rows(t, devopsServices)); got != 4 {
			t.Fatalf("expected 4 devops.service entities, got %d", got)
		}

		checkout := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".entity with(domain='devops', name='devops.service', query='checkout') | limit 10",
		})
		checkoutRows := rows(t, checkout)
		if len(checkoutRows) == 0 {
			t.Fatalf("expected checkout entity in search results")
		}

		byID := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".entity with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | project __entity_id__,display_name | limit 5",
		})
		idRows := rows(t, byID)
		if len(idRows) != 1 {
			t.Fatalf("expected 1 entity by ID, got %d", len(idRows))
		}
		if idRows[0].(map[string]any)["__entity_id__"] != "10000000000000000000000000000101" {
			t.Fatalf("unexpected entity ID: %+v", idRows[0])
		}

		topk := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".entity with(domain='devops', name='devops.service', query='service', topk=2) | project __entity_id__ | limit 10",
		})
		if got := len(rows(t, topk)); got == 0 {
			t.Fatalf("expected topk query to return entities, got 0")
		}

		entityPipeline := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".entity with(domain='devops', name='devops.team') | project display_name,status | sort display_name | limit 2",
		})
		epRows := rows(t, entityPipeline)
		if len(epRows) != 2 {
			t.Fatalf("expected 2 team rows, got %d", len(epRows))
		}
	})

	t.Run("TopologyQuery", func(t *testing.T) {
		direct := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest | limit 20",
		})
		directRows := rows(t, direct)
		if len(directRows) == 0 {
			t.Fatalf("expected direct relations for checkout_service")
		}

		neighbors := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 30",
		})
		neighborRows := rows(t, neighbors)
		if len(neighborRows) == 0 {
			t.Fatalf("expected neighbor nodes at depth 2")
		}

		cypherPayload := map[string]any{
			"query":      ".topo | graph-call cypher(`MATCH (svc:``devops@devops.service`` {__entity_id__: $svc}) OPTIONAL MATCH path = (svc)-[r*1..2]-(neighbor) WITH svc, neighbor, relationships(path) AS rels WHERE neighbor IS NULL OR coalesce(neighbor.__deleted__, false) = false RETURN svc.__entity_id__ AS service, neighbor.__entity_id__ AS neighbor, [rel IN rels | type(rel)] AS relation_types, size(rels) AS hops ORDER BY hops, neighbor LIMIT 20`) | limit 20",
			"parameters": map[string]any{"svc": "10000000000000000000000000000101"},
		}
		cypher := post(t, server.URL+"/api/v1/query/demo/execute", cypherPayload)
		cypherRows := rows(t, cypher)
		if len(cypherRows) == 0 {
			t.Fatalf("expected cypher query results")
		}
		cypherExplain := post(t, server.URL+"/api/v1/query/demo/explain", cypherPayload)
		if cypherExplain["cypher_dialect"] != "ladybug" || cypherExplain["cypher_engine"] != "go" {
			t.Fatalf("unexpected cypher explain: %+v", cypherExplain)
		}
	})

	t.Run("CrossDomainTopology", func(t *testing.T) {
		crossDomain := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".topo with(relation_type='maps') | project src,relation,dest | limit 20",
		})
		crossRows := rows(t, crossDomain)
		if len(crossRows) == 0 {
			t.Fatalf("expected cross-domain 'maps' relations (devops→k8s)")
		}
		for _, r := range crossRows {
			row := r.(map[string]any)
			if row["relation"] != "maps" {
				t.Fatalf("expected 'maps' relation, got %+v", row)
			}
		}

		runs := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".topo with(relation_type='runs') | project src,relation,dest | limit 20",
		})
		runsRows := rows(t, runs)
		if len(runsRows) == 0 {
			t.Fatalf("expected 'runs' relations")
		}

		supportedBy := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
			"query": ".topo with(relation_type='supported_by') | project src,relation,dest | limit 20",
		})
		supportedByRows := rows(t, supportedBy)
		if len(supportedByRows) == 0 {
			t.Fatalf("expected 'supported_by' cross-domain relations")
		}
	})

	t.Run("AgentGateway", func(t *testing.T) {
		discovery := get(t, server.URL+"/api/v1/agent/demo/discover")
		if discovery["workspace"] != "demo" {
			t.Fatalf("unexpected discovery workspace: %+v", discovery)
		}

		tools, ok := discovery["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Fatalf("expected tools in discovery: %+v", discovery)
		}
		foundExecute := false
		for _, rawTool := range tools {
			tool := rawTool.(map[string]any)
			if tool["name"] == "query_spl_execute" {
				foundExecute = true
			}
			if tool["name"] == "entity_write" && tool["enabled"] == true {
				t.Fatalf("entity_write should be disabled by default")
			}
		}
		if !foundExecute {
			t.Fatalf("expected query_spl_execute tool in discovery")
		}

		resources, ok := discovery["resources"].([]any)
		if !ok || len(resources) == 0 {
			t.Fatalf("expected resources in discovery: %+v", discovery)
		}
		for _, rawResource := range resources {
			resource := rawResource.(map[string]any)
			read := post(t, server.URL+"/api/v1/agent/demo/resources:read", map[string]any{"uri": resource["uri"]})
			if read["uri"] != resource["uri"] || read["content"] == nil {
				t.Fatalf("unexpected resource read: %+v", read)
			}
		}

		actions, ok := discovery["next_actions"].([]any)
		if !ok || len(actions) == 0 {
			t.Fatalf("expected next_actions in discovery: %+v", discovery)
		}

		toolExecute := post(t, server.URL+"/api/v1/agent/demo/tools:execute", map[string]any{
			"name":      "query_spl_execute",
			"arguments": map[string]any{"query": ".umodel with(kind='entity_set') | limit 5"},
		})
		if toolExecute["ok"] != true {
			t.Fatalf("expected query_spl_execute success: %+v", toolExecute)
		}

		toolExplain := post(t, server.URL+"/api/v1/agent/demo/tools:execute", map[string]any{
			"name":      "query_spl_explain",
			"arguments": map[string]any{"query": ".umodel | limit 1"},
		})
		if toolExplain["ok"] != true {
			t.Fatalf("expected query_spl_explain success: %+v", toolExplain)
		}

		disabledTool := postStatus(t, server.URL+"/api/v1/agent/demo/tools:execute", map[string]any{
			"name":      "entity_write",
			"arguments": map[string]any{"entities": []map[string]any{}},
		}, http.StatusBadRequest)
		if errorCode(t, disabledTool) != "TOOL_DISABLED" {
			t.Fatalf("expected disabled write tool: %+v", disabledTool)
		}
	})

	t.Run("QueryExplain", func(t *testing.T) {
		explain := post(t, server.URL+"/api/v1/query/demo/explain", map[string]any{
			"query": ".entity with(domain='devops', name='devops.service') | limit 5",
		})
		if explain["source"] != ".entity" {
			t.Fatalf("expected .entity source in explain: %+v", explain)
		}
		if explain["provider"] != "memory" {
			t.Fatalf("expected memory provider in explain: %+v", explain)
		}

		topoExplain := post(t, server.URL+"/api/v1/query/demo/explain", map[string]any{
			"query": ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 10",
		})
		if topoExplain["source"] != ".topo" || topoExplain["depth"] != float64(2) {
			t.Fatalf("unexpected topo explain: %+v", topoExplain)
		}

		umodelExplain := post(t, server.URL+"/api/v1/query/demo/explain", map[string]any{
			"query": ".umodel with(kind='entity_set') | where domain = 'devops' | project domain,name | sort name | limit 5",
		})
		operators, ok := umodelExplain["operators"].([]any)
		if !ok || len(operators) < 4 {
			t.Fatalf("expected at least 4 operators in umodel explain: %+v", umodelExplain)
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		for _, badQuery := range []string{
			"select * from entity",
			".entity with(domain)",
			".entity | limit 0",
			".entity_set",
		} {
			payload := postStatus(t, server.URL+"/api/v1/query/demo/execute", map[string]any{"query": badQuery}, http.StatusBadRequest)
			if errorCode(t, payload) == "" {
				t.Fatalf("expected error code for bad query %q, got %+v", badQuery, payload)
			}
		}
	})

	t.Run("AllDomains", func(t *testing.T) {
		for _, domain := range []string{"devops", "k8s", "automaker", "game", "supplier"} {
			result := post(t, server.URL+"/api/v1/query/demo/execute", map[string]any{
				"query": ".umodel with(kind='entity_set') | where domain = '" + domain + "' | limit 100",
			})
			if got := len(rows(t, result)); got == 0 {
				t.Fatalf("expected entity_sets for domain %s, got 0", domain)
			}
		}
	})
}
