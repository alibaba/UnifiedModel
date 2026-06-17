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

// TestImportRejectsSymlinkEscape verifies that a symlink placed inside the
// import root but pointing outside it is rejected: symlinks are resolved before
// the containment check, so the import root cannot be escaped via a symlink.
func TestImportRejectsSymlinkEscape(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	const es = "kind: entity_set\nschema:\n  url: u\n  version: v0.1.0\nmetadata:\n  name: devops.service\n  domain: devops\nspec:\n  fields:\n    - name: id\n      type: string\n  primary_key_fields: [id]\n  id_generator: id\n"

	// A pack directory outside the root.
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "svc.yaml"), []byte(es), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	// A symlink inside the root that points at the outside directory.
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	svc := NewService(graphstore.NewMemoryStore(), WithImportRoot(root))
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: link}); !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("import via in-root symlink to outside should be INVALID_ARGUMENT, got: %v", err)
	}
}

// TestImportRejectsNestedSymlinkFileEscape covers a symlinked import file nested
// inside an allowed directory that points outside the import root. The top-level
// path (the directory) is legitimately inside the root, so confineImportPath
// passes it; the escape only happens for a file discovered by the directory
// walk, which must therefore re-check every collected file.
func TestImportRejectsNestedSymlinkFileEscape(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	const es = "kind: entity_set\nschema:\n  url: u\n  version: v0.1.0\nmetadata:\n  name: devops.service\n  domain: devops\nspec:\n  fields:\n    - name: id\n      type: string\n  primary_key_fields: [id]\n  id_generator: id\n"

	// A valid pack file outside the root — valid so that, absent the fix, it
	// would import cleanly rather than failing on a parse error.
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.yaml")
	if err := os.WriteFile(secret, []byte(es), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	// An allowed directory inside the root, containing a symlinked yaml that
	// points at the outside file.
	pack := filepath.Join(root, "pack")
	if err := os.MkdirAll(pack, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(pack, "outside.yaml")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	svc := NewService(graphstore.NewMemoryStore(), WithImportRoot(root))
	if _, err := svc.Import(ctx, "demo", model.UModelImportRequest{Path: pack}); !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("nested symlink file escaping the root should be INVALID_ARGUMENT, got: %v", err)
	}
}
