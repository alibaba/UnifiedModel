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

// TestImportConfinesToRoot verifies that an API-originated import path is
// confined to the configured import root: a pack inside the root imports, and
// a path that escapes the root (absolute or via "..") is rejected before any
// file is read.
func TestImportConfinesToRoot(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	// A valid entity_set pack inside the root.
	pack := filepath.Join(root, "pack")
	if err := os.MkdirAll(pack, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const es = "kind: entity_set\nschema:\n  url: u\n  version: v0.1.0\nmetadata:\n  name: devops.service\n  domain: devops\nspec:\n  fields:\n    - name: id\n      type: string\n  primary_key_fields: [id]\n  id_generator: id\n"
	if err := os.WriteFile(filepath.Join(pack, "svc.yaml"), []byte(es), 0o644); err != nil {
		t.Fatalf("write pack: %v", err)
	}

	// A secret outside the root that must never be importable.
	secret := filepath.Join(t.TempDir(), "secret.yaml")
	if err := os.WriteFile(secret, []byte(es), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	svc := NewService(graphstore.NewMemoryStore(), WithImportRoot(root))

	// Inside the root: imports fine.
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: pack}); err != nil {
		t.Fatalf("import inside root should succeed, got: %v", err)
	}

	// Absolute path outside the root: rejected.
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: secret}); !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("import outside root should be INVALID_ARGUMENT, got: %v", err)
	}

	// Traversal out of the root: rejected.
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: filepath.Join(root, "..", "secret.yaml")}); !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("traversal out of root should be INVALID_ARGUMENT, got: %v", err)
	}

	// ImportTrusted bypasses confinement (used by bundled sample loads).
	if _, err := svc.ImportTrusted(ctx, "demo", model.UModelImportRequest{Path: secret}); err != nil {
		t.Fatalf("ImportTrusted should bypass confinement, got: %v", err)
	}
}
