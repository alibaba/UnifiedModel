package sampledata

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/entitystore"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/internal/query"
	"github.com/alibaba/UnifiedModel/internal/umodel"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestLookupSampleCanonicalizesAliases(t *testing.T) {
	for _, input := range []string{
		MultiDomainQuickStartSample,
		"quickstart-multidomain",
		"quickstart",
		"  QUICKSTART  ",
	} {
		def, ok := lookupSample(input)
		if !ok {
			t.Fatalf("expected %q to resolve", input)
		}
		if def.Name != MultiDomainQuickStartSample {
			t.Fatalf("expected canonical sample %q for %q, got %q", MultiDomainQuickStartSample, input, def.Name)
		}
	}
}

func TestImportUnknownSampleListsAvailableSamples(t *testing.T) {
	graph := graphstore.NewMemoryStore()
	umodelSvc := umodel.NewService(graph)
	entitySvc := entitystore.NewService(graph, umodelSvc)
	svc := NewService(umodelSvc, entitySvc)

	_, err := svc.Import(context.Background(), "demo", "missing-sample")
	coded, ok := apperrors.As(err)
	if !ok || coded.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found error, got %v", err)
	}
	if !strings.Contains(coded.Details["available"], MultiDomainQuickStartSample) {
		t.Fatalf("expected available samples to include %q, got %+v", MultiDomainQuickStartSample, coded.Details)
	}
}

func TestImportMultiDomainQuickStartWritesSchemaEntitiesAndTopology(t *testing.T) {
	ctx := context.Background()
	graph := graphstore.NewMemoryStore()
	umodelSvc := umodel.NewService(graph)
	entitySvc := entitystore.NewService(graph, umodelSvc)
	svc := NewService(umodelSvc, entitySvc)

	result, err := svc.Import(ctx, "demo", MultiDomainQuickStartSample)
	if err != nil {
		t.Fatalf("import sample: %v", err)
	}
	if result.Sample != MultiDomainQuickStartSample || result.UModel.Imported == 0 {
		t.Fatalf("expected multi-domain sample import, got %+v", result)
	}
	if result.EntityCount == 0 || result.Entities.Accepted != result.EntityCount {
		t.Fatalf("expected all sample entities accepted, got %+v", result)
	}
	if result.RelationCount == 0 || result.Relations.Accepted != result.RelationCount {
		t.Fatalf("expected all sample relations accepted, got %+v", result)
	}

	querySvc := query.NewService(graph)
	for _, kind := range []string{"metric_set", "log_set", "event_set", "data_link", "storage_link", "prometheus", "elasticsearch", "mysql"} {
		rows, err := querySvc.Execute(ctx, "demo", model.QueryRequest{
			Query: ".umodel with(kind='" + kind + "') | limit 1",
		})
		if err != nil {
			t.Fatalf("query imported kind %s: %v", kind, err)
		}
		if len(rows.Rows) == 0 {
			t.Fatalf("quickstart should import %s definitions, got %+v", kind, rows)
		}
	}

	dataSetRows, err := querySvc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)",
	})
	if err != nil {
		t.Fatalf("list quickstart data sets: %v", err)
	}
	if len(dataSetRows.Rows) != 1 {
		t.Fatalf("expected assistant response row, got %+v", dataSetRows.Rows)
	}
	data, ok := dataSetRows.Rows[0]["data"].([]map[string]any)
	if !ok || len(data) != 3 {
		t.Fatalf("expected metric, log, and event data sets, got %#v", dataSetRows.Rows[0]["data"])
	}
	joinedData := ""
	for _, item := range data {
		values, ok := item["values"].([]string)
		if !ok {
			t.Fatalf("unexpected list_data_set row: %#v", item)
		}
		joinedData += strings.Join(values, "\n")
	}
	for _, want := range []string{"devops.metric.service", "devops.log.service", "devops.event.deployment", "prometheus", "devops.prometheus.core", "elasticsearch", "devops.elasticsearch.logs", "mysql", "devops.mysql.events", "\"id\":\"service_id\""} {
		if !strings.Contains(joinedData, want) {
			t.Fatalf("expected list_data_set output to contain %q, got %s", want, joinedData)
		}
	}

	logPlanRows, err := querySvc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')",
	})
	if err != nil {
		t.Fatalf("plan quickstart logs query: %v", err)
	}
	if len(logPlanRows.Rows) != 1 || logPlanRows.Rows[0]["responseType"] != 1 {
		t.Fatalf("expected get_logs query plan row, got %+v", logPlanRows.Rows)
	}
	logQuery := ""
	if text, ok := logPlanRows.Rows[0]["query"].(string); ok {
		logQuery = text
	}
	for _, want := range []string{"get_logs", "devops.log.service", "devops.elasticsearch.logs", "elasticsearch_dsl", "devops-service-logs-*", "svc_id", "severity", "event_time", "10000000000000000000000000000101", "ERROR"} {
		if !strings.Contains(logQuery, want) {
			t.Fatalf("expected get_logs query plan to contain %q, got %s", want, logQuery)
		}
	}
	var logPlan map[string]any
	if err := json.Unmarshal([]byte(logQuery), &logPlan); err != nil {
		t.Fatalf("decode get_logs query plan: %v", err)
	}
	translatedQuery, err := json.Marshal(logPlan["query"])
	if err != nil {
		t.Fatalf("encode translated logs query: %v", err)
	}
	for _, want := range []string{"\"term\"", "\"svc_id\":\"10000000000000000000000000000101\"", "\"severity\":\"ERROR\"", "\"event_time\""} {
		if !strings.Contains(string(translatedQuery), want) {
			t.Fatalf("expected translated get_logs query to contain %q, got %s", want, string(translatedQuery))
		}
	}

	metricPlanRows, err := querySvc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101'], query='environment = \"prod\"') | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')",
	})
	if err != nil {
		t.Fatalf("plan quickstart metrics query: %v", err)
	}
	if len(metricPlanRows.Rows) != 1 || metricPlanRows.Rows[0]["responseType"] != 1 {
		t.Fatalf("expected get_metrics query plan row, got %+v", metricPlanRows.Rows)
	}
	metricQuery := ""
	if text, ok := metricPlanRows.Rows[0]["query"].(string); ok {
		metricQuery = text
	}
	for _, want := range []string{"get_metrics", "devops.metric.service", "devops.prometheus.core", "prometheus_promql", "request_count", "service_id", "environment", "prod", "30s"} {
		if !strings.Contains(metricQuery, want) {
			t.Fatalf("expected get_metrics query plan to contain %q, got %s", want, metricQuery)
		}
	}
	var metricPlan map[string]any
	if err := json.Unmarshal([]byte(metricQuery), &metricPlan); err != nil {
		t.Fatalf("decode get_metrics query plan: %v", err)
	}
	translatedMetricQuery, err := json.Marshal(metricPlan["query"])
	if err != nil {
		t.Fatalf("encode translated metrics query: %v", err)
	}
	for _, want := range []string{"\"prometheus_promql\"", "devops_service_request_total", "service_id=\\\"10000000000000000000000000000101\\\"", "environment=\\\"prod\\\""} {
		if !strings.Contains(string(translatedMetricQuery), want) {
			t.Fatalf("expected translated get_metrics query to contain %q, got %s", want, string(translatedMetricQuery))
		}
	}

	entityRows, err := querySvc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".entity with(domain='devops', name='devops.service', query='checkout') | project __entity_id__,display_name,business_value | limit 10",
	})
	if err != nil {
		t.Fatalf("query sample entity: %v", err)
	}
	if len(entityRows.Rows) == 0 {
		t.Fatalf("expected cart service entity, got %+v", entityRows)
	}

	topoRows, err := querySvc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest | limit 20",
	})
	if err != nil {
		t.Fatalf("query sample topology: %v", err)
	}
	if len(topoRows.Rows) == 0 {
		t.Fatalf("expected cart topology relations, got %+v", topoRows)
	}

	cypherRows, err := querySvc.Execute(ctx, "demo", model.QueryRequest{
		Query: ".topo | graph-call cypher(`match (svc:``devops@devops.service`` {__entity_id__: '10000000000000000000000000000101'}) optional match path = (svc)-[r*1..2]-(neighbor) with svc, neighbor, relationships(path) as rels where neighbor is null or coalesce(neighbor.__deleted__, false) = false return svc.__entity_id__ as service, neighbor.__entity_id__ as neighbor, [rel in rels | type(rel)] as relation_types, size(rels) as hops order by hops, neighbor limit 10`) | limit 10",
	})
	if err != nil {
		t.Fatalf("query sample cypher topology: %v", err)
	}
	if len(cypherRows.Rows) == 0 {
		t.Fatalf("expected cypher to see sample topology, got %+v", cypherRows)
	}
}
