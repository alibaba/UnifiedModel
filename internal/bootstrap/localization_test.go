package bootstrap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

// Verified entity IDs for the service-localization sample (md5 of semantic names).
const (
	locAPICheckout   = "3a44ea48396a812d5a1f4eb12ae51e39"
	locSvcOrder      = "f25ae2923f5df058b6119ea79e434459"
	locStoreOrdersDB = "60794de7878447582b1a4d5fe11e37a0"
	locNodeA         = "6cec8a5bb33ae85cefde09a76ebeca4c"
)

// TestServiceLocalizationPath gates the end-to-end bottleneck-localization
// walkthrough for the service-localization demo: the agent's critical-path
// walk (product API -> service -> datastore -> infra) must stay queryable and
// the datastore saturation metric must keep rendering. If the sample data,
// links, or datasets drift, this fails in `make ci` rather than only in the
// demo script.
func TestServiceLocalizationPath(t *testing.T) {
	ctx := context.Background()
	app := NewMemoryApp(t.TempDir())

	if _, err := app.Samples.Import(ctx, "loc", "service-localization"); err != nil {
		t.Fatalf("import service-localization sample: %v", err)
	}

	run := func(spl string) string {
		t.Helper()
		res, err := app.Query.Execute(ctx, "loc", model.QueryRequest{Query: spl})
		if err != nil {
			t.Fatalf("query %q: %v", spl, err)
		}
		blob, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		return string(blob)
	}

	mustContain := func(out, needle, label string) {
		t.Helper()
		if !strings.Contains(out, needle) {
			t.Fatalf("%s: expected result to contain %q\n got: %s", label, needle, out)
		}
	}

	// Symptom + entry point.
	mustContain(run(".entity with(domain='product', name='product.journey', query='impacted') | project display_name, status"),
		"Checkout Flow", "impacted journey is Checkout Flow")
	mustContain(run(".entity with(domain='product', name='product.api', query='degraded') | project display_name, status"),
		"checkout-api", "degraded API is checkout-api")

	// Hop 1: checkout-api --calls--> order-svc.
	hop1 := run(".topo | graph-call getDirectRelations([(:\"product@product.api\" {__entity_id__: '" + locAPICheckout + "'})])")
	mustContain(hop1, locSvcOrder, "hop1: checkout-api -> order-svc")
	mustContain(hop1, "calls", "hop1: relation type calls")

	// Hop 2: order-svc --reads_writes--> orders-db.
	hop2 := run(".topo | graph-call getDirectRelations([(:\"service@service.app\" {__entity_id__: '" + locSvcOrder + "'})])")
	mustContain(hop2, locStoreOrdersDB, "hop2: order-svc -> orders-db")
	mustContain(hop2, "reads_writes", "hop2: relation type reads_writes")

	// Hop 3: orders-db --hosted_on--> node-a (then node is healthy => infra ruled out).
	hop3 := run(".topo | graph-call getDirectRelations([(:\"data@data.store\" {__entity_id__: '" + locStoreOrdersDB + "'})])")
	mustContain(hop3, locNodeA, "hop3: orders-db -> node-a")
	mustContain(hop3, "hosted_on", "hop3: relation type hosted_on")
	mustContain(run(".entity with(domain='infra', name='infra.node', query='node-a') | project display_name, status"),
		"healthy", "hosting node is healthy")

	// The localization signal: connection_pool_usage renders a Prometheus plan
	// with the orders-db id substituted from the object graph.
	plan := run(".entity_set with(domain='data', name='data.store', ids=['" + locStoreOrdersDB + "']) | entity-call get_metrics('data', 'data.store.metrics', 'connection_pool_usage', step='30s')")
	mustContain(plan, "get_metrics", "store metric plan operation")
	mustContain(plan, "data_store_connection_pool", "connection_pool_usage PromQL rendered")
	mustContain(plan, locStoreOrdersDB, "store id substituted into PromQL")

	// Datasets are discoverable on the service entity set.
	ds := run(".entity_set with(domain='service', name='service.app') | entity-call list_data_set(['metric_set','log_set'], true)")
	mustContain(ds, "service.app.metrics", "list_data_set: service metric set")
	mustContain(ds, "service.app.logs", "list_data_set: service log set")
}
