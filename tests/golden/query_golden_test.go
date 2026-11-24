package golden_test

import (
	"context"
	"encoding/json"
	_ "github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/tests/graphfixture"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/query"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestQueryGoldenResults(t *testing.T) {
	ctx := context.Background()
	store := graphfixture.New(t)
	seedQueryGoldenData(t, ctx, store)
	svc := query.NewService(store)

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "umodel",
			query: ".umodel with(kind='entity_set') | where domain = 'apm' | project domain,name,kind | sort name | limit 2",
			want: `{
  "columns": [
    "domain",
    "name",
    "kind"
  ],
  "rows": [
    {
      "domain": "apm",
      "kind": "entity_set",
      "name": "operation"
    },
    {
      "domain": "apm",
      "kind": "entity_set",
      "name": "service"
    }
  ],
  "page": {
    "limit": 2
  },
  "explain": {
    "source": ".umodel",
    "provider": "memory",
    "storage_provider": "memory",
    "fallback": [
      "snapshot_filter",
      "application_filter",
      "application_project",
      "application_sort"
    ],
    "operators": [
      "with",
      "where",
      "project",
      "sort",
      "limit"
    ],
    "limit": 2,
    "time_range_applied": false
  }
}`,
		},
		{
			name:  "entity",
			query: ".entity with(domain='apm', name='apm.service', ids=['54013ba69c196820e56801f1ef5aad54','177627f91af678a9b03e993f1a91917f'], topk=2) | project __entity_id__,display_name | sort __entity_id__",
			want: `{
  "columns": [
    "__entity_id__",
    "display_name"
  ],
  "rows": [
    {
      "__entity_id__": "177627f91af678a9b03e993f1a91917f",
      "display_name": "checkout service"
    },
    {
      "__entity_id__": "54013ba69c196820e56801f1ef5aad54",
      "display_name": "cart service"
    }
  ],
  "page": {
    "limit": 2
  },
  "explain": {
    "source": ".entity",
    "provider": "memory",
    "storage_provider": "memory",
    "pushdown": [
      "entity_search"
    ],
    "fallback": [
      "application_filter",
      "application_project",
      "application_sort"
    ],
    "operators": [
      "with",
      "project",
      "sort"
    ],
    "limit": 2,
    "time_range_applied": false
  }
}`,
		},
		{
			name:  "topo",
			query: ".topo | graph-call getNeighborNodes('full', 2, [(:\"apm@apm.service\" {__entity_id__: '54013ba69c196820e56801f1ef5aad54'})]) | project src,relation,dest | sort dest | limit 5",
			want: `{
  "columns": [
    "src",
    "relation",
    "dest"
  ],
  "rows": [
    {
      "dest": "apm/apm.service/177627f91af678a9b03e993f1a91917f",
      "relation": "calls",
      "src": "apm/apm.service/54013ba69c196820e56801f1ef5aad54"
    }
  ],
  "page": {
    "limit": 5
  },
  "explain": {
    "source": ".topo",
    "provider": "memory",
    "storage_provider": "memory",
    "pushdown": [
      "graph_call:getNeighborNodes"
    ],
    "fallback": [
      "application_project",
      "application_sort"
    ],
    "operators": [
      "graph-call:getNeighborNodes",
      "project",
      "sort",
      "limit"
    ],
    "depth": 2,
    "limit": 5,
    "time_range_applied": false
  }
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: tc.query})
			if err != nil {
				t.Fatalf("execute: %v", err)
			}
			got := mustGoldenJSON(t, result)
			want := strings.ReplaceAll(tc.want, `"memory"`, `"`+graphfixture.Provider()+`"`)
			if got != strings.TrimSpace(want) {
				t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
			}
		})
	}
}

func seedQueryGoldenData(t *testing.T, ctx context.Context, store contract.GraphStore) {
	t.Helper()
	_, err := store.PutUModelElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements: []model.UModelElement{
			{Kind: "entity_set", Domain: "apm", Name: "operation"},
			{Kind: "entity_set", Domain: "apm", Name: "service"},
			{Kind: "entity_set", Domain: "k8s", Name: "pod"},
		},
	})
	if err != nil {
		t.Fatalf("put umodel: %v", err)
	}
	_, err = store.WriteEntities(ctx, model.EntityWriteBatch{
		Workspace: "demo",
		Entities: []model.EntityPayload{
			{"__domain__": "apm", "__entity_type__": "apm.service", "__entity_id__": "54013ba69c196820e56801f1ef5aad54", "__method__": "Update", "display_name": "cart service"},
			{"__domain__": "apm", "__entity_type__": "apm.service", "__entity_id__": "177627f91af678a9b03e993f1a91917f", "__method__": "Update", "display_name": "checkout service"},
			{"__domain__": "apm", "__entity_type__": "apm.service", "__entity_id__": "f83c2a85d972a89238f31296c63f0dbc", "__method__": "Update", "display_name": "payment service"},
		},
	})
	if err != nil {
		t.Fatalf("write entities: %v", err)
	}
	_, err = store.WriteRelations(ctx, model.RelationWriteBatch{
		Workspace: "demo",
		Relations: []model.RelationPayload{{
			"__src_domain__":       "apm",
			"__src_entity_type__":  "apm.service",
			"__src_entity_id__":    "54013ba69c196820e56801f1ef5aad54",
			"__dest_domain__":      "apm",
			"__dest_entity_type__": "apm.service",
			"__dest_entity_id__":   "177627f91af678a9b03e993f1a91917f",
			"__relation_type__":    "calls",
			"__method__":           "Update",
		}},
	})
	if err != nil {
		t.Fatalf("write relation: %v", err)
	}
}

func mustGoldenJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(body)
}
