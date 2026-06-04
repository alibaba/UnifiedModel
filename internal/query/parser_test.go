package query

import (
	"reflect"
	"testing"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

const cartServiceNode = "(:\"apm@apm.service\" {__entity_id__: '54013ba69c196820e56801f1ef5aad54'})"

func TestParseUModelPipeline(t *testing.T) {
	plan, err := Parse(model.QueryRequest{Query: ".umodel with(kind='entity_set') | where domain = 'apm' | project domain,name,kind | sort name desc | limit 5"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.Source != ".umodel" || plan.Limit != 5 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Filters["kind"] != "entity_set" {
		t.Fatalf("unexpected filters: %+v", plan.Filters)
	}
	if !reflect.DeepEqual(plan.Operators, []string{"with", "where", "project", "sort", "limit"}) {
		t.Fatalf("unexpected operators: %+v", plan.Operators)
	}
	if len(plan.Predicates) != 1 || plan.Predicates[0].Field != "domain" || plan.Predicates[0].Value != "apm" {
		t.Fatalf("unexpected predicates: %+v", plan.Predicates)
	}
	if !reflect.DeepEqual(plan.Project, []string{"domain", "name", "kind"}) {
		t.Fatalf("unexpected project: %+v", plan.Project)
	}
	if len(plan.Sort) != 1 || plan.Sort[0].Field != "name" || !plan.Sort[0].Desc {
		t.Fatalf("unexpected sort: %+v", plan.Sort)
	}
}

func TestParseG4WithParams(t *testing.T) {
	plan, err := Parse(model.QueryRequest{Query: ".entity with(domain='apm', ids=['54013ba69c196820e56801f1ef5aad54','177627f91af678a9b03e993f1a91917f'], pairs=[('src','dest')], enabled=true, ratio=1.5, raw=`MATCH (n)`, query=$query) | limit 5"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.Source != ".entity" || plan.Filters["domain"] != "apm" {
		t.Fatalf("unexpected source/filters: %+v", plan)
	}
	if !reflect.DeepEqual(plan.Filters["ids"], []string{"54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f"}) {
		t.Fatalf("unexpected ids: %#v", plan.Filters["ids"])
	}
	if plan.Filters["enabled"] != true || plan.Filters["ratio"] != 1.5 || plan.Filters["raw"] != "MATCH (n)" || plan.Filters["query"] != "$query" {
		t.Fatalf("unexpected scalar filters: %#v", plan.Filters)
	}
	pairs, ok := plan.Filters["pairs"].([]any)
	if !ok || len(pairs) != 1 || !reflect.DeepEqual(pairs[0], []string{"src", "dest"}) {
		t.Fatalf("unexpected tuple list: %#v", plan.Filters["pairs"])
	}
}

func TestParseResolvesRequestParameters(t *testing.T) {
	plan, err := Parse(model.QueryRequest{
		Query: ".entity with(domain=$domain, name='apm.service', ids=[$id], query=$query) | where display_name = $display_name | limit 5",
		Params: map[string]any{
			"domain":       "apm",
			"id":           "54013ba69c196820e56801f1ef5aad54",
			"query":        "cart",
			"display_name": "cart service",
		},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.Filters["domain"] != "apm" || plan.Filters["query"] != "cart" {
		t.Fatalf("unexpected parameterized filters: %#v", plan.Filters)
	}
	if !reflect.DeepEqual(plan.Filters["ids"], []string{"54013ba69c196820e56801f1ef5aad54"}) {
		t.Fatalf("unexpected parameterized ids: %#v", plan.Filters["ids"])
	}
	if len(plan.Predicates) != 1 || plan.Predicates[0].Value != "cart service" {
		t.Fatalf("unexpected parameterized predicate: %+v", plan.Predicates)
	}
	if plan.Pipeline[1].Predicate == nil || plan.Pipeline[1].Predicate.Value != "cart service" {
		t.Fatalf("pipeline predicate should keep resolved parameter: %+v", plan.Pipeline)
	}
}

func TestParseEntityFiltersTopKAndIDs(t *testing.T) {
	plan, err := Parse(model.QueryRequest{Query: ".entity with(domain='apm', name='apm.service', ids=['54013ba69c196820e56801f1ef5aad54','177627f91af678a9b03e993f1a91917f'], query='shop', topk=50)"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if plan.Source != ".entity" || plan.Limit != 50 || plan.TopK != 50 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Filters["domain"] != "apm" || plan.Filters["name"] != "apm.service" || plan.Filters["query"] != "shop" {
		t.Fatalf("unexpected filters: %+v", plan.Filters)
	}
	ids, ok := plan.Filters["ids"].([]string)
	if !ok || !reflect.DeepEqual(ids, []string{"54013ba69c196820e56801f1ef5aad54", "177627f91af678a9b03e993f1a91917f"}) {
		t.Fatalf("unexpected ids: %#v", plan.Filters["ids"])
	}
}

func TestParseEntitySetEntityCall(t *testing.T) {
	plan, err := Parse(model.QueryRequest{
		Query: ".entity_set with(domain='apm', name='apm.service', ids=['svc-1'], query=$query) | entity-call list_data_set($types, detail=$detail) | limit 10",
		Params: map[string]any{
			"query":  "service_id = 'svc-1'",
			"types":  []string{"metric_set", "log_set"},
			"detail": true,
		},
	})
	if err != nil {
		t.Fatalf("parse entity-call: %v", err)
	}
	if plan.Source != ".entity_set" || plan.Limit != 10 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Filters["domain"] != "apm" || plan.Filters["name"] != "apm.service" || plan.Filters["query"] != "service_id = 'svc-1'" {
		t.Fatalf("unexpected filters: %+v", plan.Filters)
	}
	if !reflect.DeepEqual(plan.Operators, []string{"with", "entity-call:list_data_set", "limit"}) {
		t.Fatalf("unexpected operators: %+v", plan.Operators)
	}
	if plan.EntityCall == nil || plan.EntityCall.Name != "list_data_set" {
		t.Fatalf("unexpected entity call: %+v", plan.EntityCall)
	}
	if !reflect.DeepEqual(plan.EntityCall.Arguments, []any{[]string{"metric_set", "log_set"}}) {
		t.Fatalf("unexpected entity-call arguments: %#v", plan.EntityCall.Arguments)
	}
	if !reflect.DeepEqual(plan.EntityCall.NamedArguments, map[string]any{"detail": true}) {
		t.Fatalf("unexpected entity-call named arguments: %#v", plan.EntityCall.NamedArguments)
	}
	if plan.Pipeline[1].EntityCall == nil || !reflect.DeepEqual(plan.Pipeline[1].EntityCall.Arguments, plan.EntityCall.Arguments) {
		t.Fatalf("pipeline entity-call should keep resolved arguments: %+v", plan.Pipeline)
	}
	if !reflect.DeepEqual(plan.Pipeline[1].EntityCall.NamedArguments, plan.EntityCall.NamedArguments) {
		t.Fatalf("pipeline entity-call should keep resolved named arguments: %+v", plan.Pipeline)
	}
}

func TestParseTopoControlledGraphCalls(t *testing.T) {
	neighbors, err := Parse(model.QueryRequest{Query: ".topo | graph-call getNeighborNodes('full', 3, [" + cartServiceNode + "]) | limit 10"})
	if err != nil {
		t.Fatalf("parse neighbors: %v", err)
	}
	if neighbors.Depth != 3 || neighbors.GraphCall == nil || neighbors.GraphCall.Name != "getNeighborNodes" || neighbors.GraphCall.Type != "full" {
		t.Fatalf("unexpected neighbor plan: %+v", neighbors)
	}
	if !reflect.DeepEqual(neighbors.Operators, []string{"graph-call:getNeighborNodes", "limit"}) {
		t.Fatalf("unexpected operators: %+v", neighbors.Operators)
	}
	if !reflect.DeepEqual(neighbors.GraphCall.SeedIDs, []string{"54013ba69c196820e56801f1ef5aad54"}) {
		t.Fatalf("unexpected seeds: %+v", neighbors.GraphCall.SeedIDs)
	}
	if len(neighbors.GraphCall.Nodes) != 1 || neighbors.GraphCall.Nodes[0].Label != "apm@apm.service" {
		t.Fatalf("unexpected nodes: %+v", neighbors.GraphCall.Nodes)
	}

	relations, err := Parse(model.QueryRequest{Query: ".topo | graph-call getDirectRelations([" + cartServiceNode + "])"})
	if err != nil {
		t.Fatalf("parse direct relations: %v", err)
	}
	if relations.GraphCall == nil || relations.GraphCall.Name != "getDirectRelations" || relations.Depth != 1 {
		t.Fatalf("unexpected direct relation plan: %+v", relations)
	}
	if !reflect.DeepEqual(relations.GraphCall.SeedIDs, []string{"54013ba69c196820e56801f1ef5aad54"}) {
		t.Fatalf("unexpected direct relation seeds: %+v", relations.GraphCall.SeedIDs)
	}
}

func TestParseTopoGraphMatchAndCypher(t *testing.T) {
	match, err := Parse(model.QueryRequest{Query: ".topo | graph-match (s:\"apm@apm.service\" {__entity_id__: '54013ba69c196820e56801f1ef5aad54'})-[e]-(d) | project s,e,d"})
	if err != nil {
		t.Fatalf("parse graph-match: %v", err)
	}
	if !reflect.DeepEqual(match.Operators, []string{"graph-match", "project"}) || match.Pipeline[0].Expression == "" {
		t.Fatalf("unexpected graph-match plan: %+v", match)
	}

	cypher, err := Parse(model.QueryRequest{Query: ".topo | graph-call cypher(`MATCH (s:``apm@apm.service`` {__entity_id__: '54013ba69c196820e56801f1ef5aad54'}) RETURN s`)"})
	if err != nil {
		t.Fatalf("parse cypher: %v", err)
	}
	if cypher.GraphCall == nil || cypher.GraphCall.Name != "cypher" || cypher.GraphCall.Cypher != "MATCH (s:`apm@apm.service` {__entity_id__: '54013ba69c196820e56801f1ef5aad54'}) RETURN s" {
		t.Fatalf("unexpected cypher plan: %+v", cypher)
	}
}

func TestParseRejectsUnknownOperator(t *testing.T) {
	_, err := Parse(model.QueryRequest{Query: ".umodel | native('MATCH n RETURN n')"})
	if !apperrors.IsCode(err, apperrors.CodeQueryParseError) {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestParseRejectsMalformedWithLimitAndDepth(t *testing.T) {
	cases := []string{
		".entity with(domain)",
		".entity with(domain='apm'",
		".entity | limit 0",
		".entity_set",
		".entity_set with(domain='apm')",
		".entity_set with(domain='apm', name='apm.service')",
		".entity_set with(domain='apm', name='apm.service') | entity-call",
		".entity_set with(domain='apm', name='apm.service') | entity-call get_metric(,)",
		".entity with(domain='apm', name='apm.service') | entity-call get_log('apm', 'apm.log.app')",
		".topo | graph-call getNeighborNodes('both', 2, [" + cartServiceNode + "])",
		".topo | graph-call getNeighborNodes('full', -1, [" + cartServiceNode + "])",
		".topo | graph-call getNeighborNodes('full', 2, [])",
		".topo | graph-call getDirectRelations([(:\"apm@apm.service\" {__entity_id__: 'cart'})])",
		".topo | graph-call getDirectRelations('both', ['cart'])",
		".topo | cypher | limit 1",
	}
	for _, query := range cases {
		t.Run(query, func(t *testing.T) {
			_, err := Parse(model.QueryRequest{Query: query})
			if !apperrors.IsCode(err, apperrors.CodeQueryParseError) {
				t.Fatalf("expected parse error for %q, got %v", query, err)
			}
		})
	}
}
