package graphstore

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSafeWorkspaceSegmentStaysContained verifies that no workspace id —
// including traversal attempts — can produce a path that escapes the
// workspaces directory.
func TestSafeWorkspaceSegmentStaysContained(t *testing.T) {
	root := filepath.Join("/srv", "data")
	base := filepath.Join(root, "workspaces")

	for _, ws := range []string{
		"demo", "ws-1", "a_b",
		"..", "../..", "../../etc/passwd", ".", "", "a/b", "..%2f..", "/etc/passwd",
	} {
		seg := safeWorkspaceSegment(ws)
		if strings.ContainsAny(seg, "/\\") {
			t.Fatalf("workspace %q produced a multi-segment path: %q", ws, seg)
		}
		full := filepath.Join(base, seg)
		rel, err := filepath.Rel(base, full)
		if err != nil {
			t.Fatalf("workspace %q: rel error: %v", ws, err)
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("workspace %q escaped the workspaces dir: full=%q rel=%q", ws, full, rel)
		}
	}
}

// TestSafeWorkspaceSegmentPreservesValidIDs verifies valid workspace ids are
// passed through unchanged (so existing on-disk layouts are unaffected).
func TestSafeWorkspaceSegmentPreservesValidIDs(t *testing.T) {
	for _, ws := range []string{"demo", "incident", "service-localization", "ws_1", "a0"} {
		if got := safeWorkspaceSegment(ws); got != ws {
			t.Fatalf("valid workspace %q changed to %q", ws, got)
		}
	}
}

// TestAllocHintBounded verifies the prealloc hint is capped, so a huge
// caller-supplied limit cannot drive a giant up-front allocation.
func TestAllocHintBounded(t *testing.T) {
	if got := allocHint(50); got != 50 {
		t.Fatalf("allocHint(50) = %d, want 50", got)
	}
	if got := allocHint(-5); got != 0 {
		t.Fatalf("allocHint(-5) = %d, want 0", got)
	}
	if got := allocHint(1 << 30); got != maxPreallocRows {
		t.Fatalf("allocHint(2^30) = %d, want %d (capped)", got, maxPreallocRows)
	}
}
