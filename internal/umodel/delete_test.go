package umodel

import (
	"context"
	"reflect"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	searchcontract "github.com/alibaba/UnifiedModel/internal/search/contract"
	"github.com/alibaba/UnifiedModel/internal/umodel/schemaspec"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestDeleteElementsRemovesOnlyUModelState(t *testing.T) {
	ctx := context.Background()
	search := &recordingSearchIndexer{}
	svc := NewService(graphstore.NewMemoryStore(), WithValidator(schemaspec.NewNoopValidator()), WithSearchIndexer(search))

	result, err := svc.PutElements(ctx, model.UModelElementBatch{
		Workspace: "demo",
		Elements: []model.UModelElement{
			{
				Kind:   "entity_set",
				Domain: "devops",
				Name:   "devops.service",
				Spec: map[string]any{
					"fields": []any{map[string]any{"name": "service_id", "type": "string"}},
				},
			},
			{
				Kind:   "runbook_set",
				Domain: "devops",
				Name:   "devops.service.runbook",
				Spec: map[string]any{
					"knowledge": "check the golden path first",
					"actions":   []any{"restart"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("put elements: %v", err)
	}
	if result.Accepted != 2 || result.Failed != 0 {
		t.Fatalf("unexpected put result: %+v", result)
	}
	if _, ok := svc.findIndexedElement("entity_set", "devops", "devops.service"); !ok {
		t.Fatalf("expected entity_set to be indexed before delete")
	}

	deleteResult, err := svc.DeleteElements(ctx, "demo", []string{
		"devops/devops.service/entity_set",
		"devops/devops.service.runbook/runbook_set",
	})
	if err != nil {
		t.Fatalf("delete elements: %v", err)
	}
	if deleteResult.Accepted != 2 || deleteResult.Failed != 0 {
		t.Fatalf("unexpected delete result: %+v", deleteResult)
	}
	if _, ok := svc.findIndexedElement("entity_set", "devops", "devops.service"); ok {
		t.Fatalf("deleted entity_set should be removed from schema index")
	}

	wantDocIDs := []string{
		"umodel/devops/devops.service/entity_set",
		"umodel/devops/devops.service.runbook/runbook_set",
		"runbook_set/devops/devops.service.runbook/runbook_set/knowledge",
		"runbook_set/devops/devops.service.runbook/runbook_set/actions",
	}
	if !reflect.DeepEqual(search.deletedDocIDs, wantDocIDs) {
		t.Fatalf("unexpected search deletes: got %#v want %#v", search.deletedDocIDs, wantDocIDs)
	}
}

type recordingSearchIndexer struct {
	deletedDocIDs []string
}

func (r *recordingSearchIndexer) Index(ctx context.Context, workspace string, chunks []searchcontract.Chunk) error {
	return nil
}

func (r *recordingSearchIndexer) DeleteByDocID(ctx context.Context, workspace string, docIDs []string) error {
	r.deletedDocIDs = append(r.deletedDocIDs, docIDs...)
	return nil
}
