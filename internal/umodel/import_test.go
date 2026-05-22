package umodel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestValidateRejectsInvalidUModelElement(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	result, err := svc.Validate(context.Background(), "demo", []model.UModelElement{{Kind: "entity_set"}})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid result")
	}
}

func TestValidateAcceptsValidUModelElements(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	result, err := svc.Validate(context.Background(), "demo", []model.UModelElement{{
		Kind:   "entity_set",
		Domain: "devops",
		Name:   "devops.service",
		Spec:   map[string]any{"fields": []any{map[string]any{"name": "service_id", "type": "string"}}},
	}})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !result.Valid || len(result.Errors) != 0 {
		t.Fatalf("expected valid result, got %+v", result)
	}
}

func TestImportFileBuildsIndexAndRebuildsFromSnapshot(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	svc := NewService(store)
	path := writeTestFile(t, "devops.service.yaml", `
kind: entity_set
schema:
  version: v0.1.0
metadata:
  name: devops.service
  domain: devops
spec:
  fields:
    - name: service_id
      display_name:
        en_us: Service ID
      type: string
`)

	result, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: path})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Imported != 1 || len(result.Elements) != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	element := result.Elements[0]
	if element.Name != "devops.service" || element.Kind != "entity_set" || element.Domain != "devops" || element.Version != "v0.1.0" {
		t.Fatalf("unexpected imported element: %+v", element)
	}

	if _, ok := svc.findIndexedElement("entity_set", "devops", "devops.service"); !ok {
		t.Fatalf("expected import to populate schema index")
	}

	svc.mu.Lock()
	svc.indexes = map[string]*schemaIndex{}
	svc.mu.Unlock()
	if _, ok := svc.findIndexedElement("entity_set", "devops", "devops.service"); ok {
		t.Fatalf("expected test to clear schema index")
	}
	if err := svc.RebuildIndex(ctx, "demo"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	schema, err := svc.ResolveEntitySet(ctx, model.EntityTypeRef{Domain: "devops", Name: "devops.service"})
	if err != nil {
		t.Fatalf("resolve entity set: %v", err)
	}
	if _, ok := schema.Fields["service_id"]; !ok {
		t.Fatalf("expected indexed schema fields, got %+v", schema.Fields)
	}
}

func TestImportDuplicateElementIsIdempotentOverwrite(t *testing.T) {
	ctx := context.Background()
	store := graphstore.NewMemoryStore()
	svc := NewService(store)
	path := writeTestFile(t, "devops.service.yaml", `
kind: entity_set
metadata:
  name: devops.service
  domain: devops
spec:
  display_name: DevOps Service
`)

	first, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: path})
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	second, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: path})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if first.Imported != 1 || second.Imported != 1 || len(second.Errors) != 0 {
		t.Fatalf("expected duplicate import to be idempotent overwrite, first=%+v second=%+v", first, second)
	}
	snapshot, err := store.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: "demo"})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snapshot.Elements) != 1 || snapshot.Elements[0].Domain != "devops" || snapshot.Elements[0].Name != "devops.service" || snapshot.Elements[0].Kind != "entity_set" {
		t.Fatalf("duplicate import should keep one element, got %+v", snapshot.Elements)
	}
}

func TestImportDirectoryParsesJSONYAMLAndSkipsOtherFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "entity.yaml"), `
kind: entity_set
metadata:
  name: devops.service
  domain: devops
spec: {}
`)
	writeFile(t, filepath.Join(dir, "metric.json"), `{
  "kind": "metric_set",
  "schema": {"version": "v0.1.0"},
  "metadata": {"name": "devops.metric.devops.service", "domain": "devops"},
  "spec": {"fields": [{"name": "service_id"}]}
}`)
	writeFile(t, filepath.Join(dir, "README.md"), "not a schema")

	svc := NewService(graphstore.NewMemoryStore())
	result, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: dir})
	if err != nil {
		t.Fatalf("import directory: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 1 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
}

func TestRelationTypeOfUsesEntityLinkType(t *testing.T) {
	element := model.UModelElement{
		Kind:   "entity_set_link",
		Domain: "devops",
		Name:   "devops.service_runs_k8s.workload",
		Spec:   map[string]any{"entity_link_type": "runs"},
	}

	if got := relationTypeOf(element); got != "runs" {
		t.Fatalf("expected entity_link_type to define relation type, got %q", got)
	}
}

func TestImportRejectsInvalidSourcesWithStableErrors(t *testing.T) {
	ctx := context.Background()
	svc := NewService(graphstore.NewMemoryStore())

	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{}); !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("expected missing path invalid argument, got %v", err)
	}
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: filepath.Join(t.TempDir(), "missing.yaml")}); !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("expected missing file invalid argument, got %v", err)
	}
	txt := writeTestFile(t, "README.txt", "not a schema")
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: txt}); !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("expected unsupported extension invalid argument, got %v", err)
	}
	badYAML := writeTestFile(t, "bad.yaml", "kind: [")
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: badYAML}); !apperrors.IsCode(err, apperrors.CodeValidationFailed) {
		t.Fatalf("expected malformed yaml validation failure, got %v", err)
	}
}

func TestImportCommonSchemaPackHookIsExplicit(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Import(context.Background(), "demo", model.UModelImportRequest{
		Path:              writeTestFile(t, "empty.yaml", "kind: entity_set\nmetadata:\n  name: devops.service\n  domain: devops\nspec: {}\n"),
		CommonSchemaPacks: []string{"quickstart-multidomain"},
	})
	if !apperrors.IsCode(err, apperrors.CodeNotImplemented) {
		t.Fatalf("expected common schema hook to be explicit, got %v", err)
	}
}

func writeTestFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	writeFile(t, path, content)
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
