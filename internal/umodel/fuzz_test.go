package umodel

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
)

// FuzzConfineImportPath hardens the import-path confinement, the boundary that
// stops an API caller from reading arbitrary server files. The security
// invariant: if confineImportPath accepts a path (no error), the returned path
// must stay inside the import root. The fuzzer hunts for adversarial inputs
// (traversal, odd separators, NUL bytes) that get accepted yet escape.
func FuzzConfineImportPath(f *testing.F) {
	const root = "/srv/umodel/import-root"
	svc := NewService(graphstore.NewMemoryStore(), WithImportRoot(root))
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		f.Fatalf("abs root: %v", err)
	}
	rootAbs = filepath.Clean(rootAbs)

	seeds := []string{
		root + "/pack",
		root + "/a/b/c",
		"pack",
		"a/b",
		"../etc/passwd",
		"/etc/passwd",
		"",
		".",
		"..",
		root + "/../escape",
		root + "/a/../../escape",
		"\x00",
		root + "/\x00pack",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, p string) {
		out, err := svc.confineImportPath(p)
		if err != nil {
			return // rejected — safe
		}
		rel, relErr := filepath.Rel(rootAbs, out)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("confineImportPath(%q) was accepted but escapes the root: out=%q rel=%q", p, out, rel)
		}
	})
}
