package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestWorkspaceCRUDIsMetadataOnly(t *testing.T) {
	ctx := context.Background()
	svc := NewService("data", nil)

	created, err := svc.CreateWorkspace(ctx, model.CreateWorkspaceRequest{ID: "demo", Labels: map[string]string{"env": "test"}})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if created.ID != "demo" || created.Status != model.WorkspaceStatusActive {
		t.Fatalf("unexpected workspace: %+v", created)
	}
	if created.Paths.Root != "data/instances/demo" || created.Paths.Tmp != "data/instances/demo/tmp" {
		t.Fatalf("workspace paths should be fixed under data/instances/{workspace}, got %+v", created.Paths)
	}

	name := "Demo Workspace"
	updated, err := svc.UpdateWorkspace(ctx, "demo", model.UpdateWorkspaceRequest{Name: &name, IfMatchVersion: created.ResourceVersion})
	if err != nil {
		t.Fatalf("update workspace: %v", err)
	}
	if updated.Name != name || updated.ResourceVersion != created.ResourceVersion+1 {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	deleted, err := svc.DeleteWorkspace(ctx, "demo")
	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if deleted.Status != model.WorkspaceStatusDeleted {
		t.Fatalf("workspace should be tombstoned: %+v", deleted)
	}

	if _, err := svc.GetWorkspace(ctx, "demo"); !apperrors.IsCode(err, apperrors.CodeWorkspaceTombstoned) {
		t.Fatalf("expected tombstone error, got %v", err)
	}
}

func TestWorkspaceDuplicateDeleteAndTombstoneVisibility(t *testing.T) {
	ctx := context.Background()
	svc := NewService("data", nil)

	if _, err := svc.CreateWorkspace(ctx, model.CreateWorkspaceRequest{ID: "demo"}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := svc.CreateWorkspace(ctx, model.CreateWorkspaceRequest{ID: "demo"}); !apperrors.IsCode(err, apperrors.CodeConflict) {
		t.Fatalf("expected duplicate create conflict, got %v", err)
	}
	if _, err := svc.DeleteWorkspace(ctx, "missing"); !apperrors.IsCode(err, apperrors.CodeNotFound) {
		t.Fatalf("expected deleting missing workspace to return not found, got %v", err)
	}
	if _, err := svc.DeleteWorkspace(ctx, "demo"); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	if _, err := svc.CreateWorkspace(ctx, model.CreateWorkspaceRequest{ID: "demo"}); !apperrors.IsCode(err, apperrors.CodeWorkspaceTombstoned) {
		t.Fatalf("expected recreate tombstoned workspace to fail, got %v", err)
	}

	activeOnly, err := svc.ListWorkspaces(ctx, model.WorkspaceListRequest{})
	if err != nil {
		t.Fatalf("list active workspaces: %v", err)
	}
	if len(activeOnly.Items) != 0 {
		t.Fatalf("deleted workspace should be hidden by default, got %+v", activeOnly.Items)
	}
	withDeleted, err := svc.ListWorkspaces(ctx, model.WorkspaceListRequest{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("list deleted workspaces: %v", err)
	}
	if len(withDeleted.Items) != 1 || withDeleted.Items[0].Status != model.WorkspaceStatusDeleted {
		t.Fatalf("expected deleted tombstone in include_deleted list, got %+v", withDeleted.Items)
	}
}

func TestInvalidWorkspaceID(t *testing.T) {
	svc := NewService("data", nil)
	_, err := svc.CreateWorkspace(context.Background(), model.CreateWorkspaceRequest{ID: "../bad"})
	if !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestPersistentWorkspaceMetadataSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	first, err := NewPersistentService(root, nil)
	if err != nil {
		t.Fatalf("new persistent service: %v", err)
	}
	created, err := first.CreateWorkspace(ctx, model.CreateWorkspaceRequest{
		ID:          "demo",
		Name:        "Demo",
		Description: "Persistent workspace",
		Labels:      map[string]string{"env": "local"},
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	second, err := NewPersistentService(root, nil)
	if err != nil {
		t.Fatalf("reopen persistent service: %v", err)
	}
	page, err := second.ListWorkspaces(ctx, model.WorkspaceListRequest{})
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected persisted workspace, got %+v", page.Items)
	}
	if page.Items[0].ID != created.ID || page.Items[0].Name != "Demo" || page.Items[0].Labels["env"] != "local" {
		t.Fatalf("unexpected persisted workspace: %+v", page.Items[0])
	}
}

func TestPersistentWorkspaceMetadataRecoversFileMemoryDirectories(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "graphstore", "file-memory", "workspaces", "demo"), 0o755); err != nil {
		t.Fatalf("create file memory workspace dir: %v", err)
	}

	svc, err := NewPersistentService(root, nil)
	if err != nil {
		t.Fatalf("new persistent service: %v", err)
	}
	page, err := svc.ListWorkspaces(ctx, model.WorkspaceListRequest{})
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "demo" || page.Items[0].Status != model.WorkspaceStatusActive {
		t.Fatalf("expected recovered demo workspace, got %+v", page.Items)
	}
}

func TestPersistentWorkspaceMetadataRecoversLadybugDirectories(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "instances", "demo", "storage", "graph", "local", "ladybug"), 0o755); err != nil {
		t.Fatalf("create ladybug workspace dir: %v", err)
	}

	svc, err := NewPersistentServiceForProvider(root, nil, providerTypeLadybug)
	if err != nil {
		t.Fatalf("new persistent service: %v", err)
	}
	page, err := svc.ListWorkspaces(ctx, model.WorkspaceListRequest{})
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "demo" || page.Items[0].Status != model.WorkspaceStatusActive {
		t.Fatalf("expected recovered demo workspace, got %+v", page.Items)
	}
}

func TestPersistentWorkspaceMetadataDoesNotReviveDeletedLadybugWorkspace(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	first, err := NewPersistentServiceForProvider(root, nil, providerTypeLadybug)
	if err != nil {
		t.Fatalf("new persistent service: %v", err)
	}
	if _, err := first.CreateWorkspace(ctx, model.CreateWorkspaceRequest{ID: "demo"}); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "instances", "demo", "storage", "graph", "local", "ladybug"), 0o755); err != nil {
		t.Fatalf("create ladybug workspace dir: %v", err)
	}
	if _, err := first.DeleteWorkspace(ctx, "demo"); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	second, err := NewPersistentServiceForProvider(root, nil, providerTypeLadybug)
	if err != nil {
		t.Fatalf("reopen persistent service: %v", err)
	}
	activeOnly, err := second.ListWorkspaces(ctx, model.WorkspaceListRequest{})
	if err != nil {
		t.Fatalf("list active workspaces: %v", err)
	}
	if len(activeOnly.Items) != 0 {
		t.Fatalf("deleted workspace should not be revived, got %+v", activeOnly.Items)
	}
	withDeleted, err := second.ListWorkspaces(ctx, model.WorkspaceListRequest{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("list deleted workspaces: %v", err)
	}
	if len(withDeleted.Items) != 1 || withDeleted.Items[0].Status != model.WorkspaceStatusDeleted {
		t.Fatalf("expected persisted tombstone, got %+v", withDeleted.Items)
	}
}
