package model

import (
	"encoding/json"
	"testing"
)

func TestQueryRequestEntityDataAliasesDecode(t *testing.T) {
	for _, key := range []string{"entity_data", "entityData", "filterByEntities"} {
		raw := []byte(`{"query":".entity_set with(domain='devops', name='devops.service') | entity-call get_metrics('devops','devops.metric.service')","` + key + `":{"version":1,"header":["id","environment"],"data":[["svc-1","prod"],{"values":["svc-2","staging"]}]}}`)
		var req QueryRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Fatalf("decode %s: %v", key, err)
		}
		entityData := req.EntityFilterData()
		if entityData == nil || len(entityData.Data) != 2 || entityData.Data[1][1] != "staging" {
			t.Fatalf("unexpected entity data for %s: %+v", key, entityData)
		}
	}
}

func TestEntityDataDecodesObjectRows(t *testing.T) {
	var data EntityData
	if err := json.Unmarshal([]byte(`[{"id":"svc-2","environment":"staging"},{"id":"svc-1","environment":"prod"}]`), &data); err != nil {
		t.Fatalf("decode row objects: %v", err)
	}
	if len(data.Header) != 2 || data.Header[0] != "environment" || data.Header[1] != "id" {
		t.Fatalf("expected sorted inferred header, got %+v", data.Header)
	}
	if data.Data[0][0] != "staging" || data.Data[1][1] != "svc-1" {
		t.Fatalf("unexpected row data: %+v", data.Data)
	}
}
