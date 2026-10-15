package itemcore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// TestSearchIndexField_OnlyTouchedByKnownSeams is the structural half of
// AC-06 ("no command other than search depends on the index"): it parses
// every non-test source file in this package -- not just core.go, so a
// dependency introduced from another file or a free function in the
// package would still be caught -- and asserts the Core.searchIndex field
// is referenced only inside the known, already-audited seams: the lazy
// initializer, Search itself, the incremental maintenance hooks on
// Create/Update/Delete, the resync on reload, and Close. Any other
// function touching it (in particular anything backing
// list/roadmap/milestones/progress/next/show/graph) fails this test before
// it ever reaches a behavioral difference.
func TestSearchIndexField_OnlyTouchedByKnownSeams(t *testing.T) {
	allowed := map[string]bool{
		"ensureSearchIndexLocked": true,
		"Search":                  true,
		"loadFromDisk":            true,
		"Create":                  true,
		"Update":                  true,
		"Delete":                  true,
		"Close":                   true,
		// The widened, whole-package scan also caught fsnotify-driven
		// incremental maintenance in watcher.go/worktree_watcher.go for
		// long-running processes (lif-serve, the TUI): the same
		// best-effort, nil-gated IndexItem/DeleteItem pattern as
		// Create/Update/Delete, just triggered by a filesystem event
		// instead of a direct API call. Not a correctness dependency for
		// any non-search command: errors are logged, never surfaced.
		"handleChanges":            true,
		"loadWorktreeItemsInitial": true,
		"handleWorktreeChanges":    true,
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	fset := token.NewFileSet()
	var offenders []string
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%q) error = %v", name, err)
		}
		scanned++

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			fnName := fn.Name.Name
			touches := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "searchIndex" {
					touches = true
				}
				return true
			})
			if touches && !allowed[fnName] {
				offenders = append(offenders, name+":"+fnName)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned zero source files -- test is not exercising the package")
	}

	if len(offenders) > 0 {
		t.Fatalf("functions outside the audited search-index seams reference c.searchIndex: %v", offenders)
	}
}

// TestSearch_CrossProcessFreshness proves AC-02/SC-02: an item file written
// directly to disk -- bypassing the Core instance doing the searching, as a
// second lif process or an editor would -- is found by the next Search
// once that instance loads. Every Core builds its own in-memory index from
// the files on disk, so nothing stale can survive between processes.
func TestSearch_CrossProcessFreshness(t *testing.T) {
	coreA, itemsDir := setupTestCore(t)
	if err := coreA.Create(&item.Item{ID: "aaa1", Slug: "a", Title: "From Process A", Body: "x"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := coreA.Search("Process"); err != nil {
		t.Fatalf("Search() (process A) error = %v", err)
	}
	if err := coreA.Close(); err != nil {
		t.Fatalf("Close() (process A) error = %v", err)
	}

	// A second process/editor writes an item file directly -- coreA (and any
	// index it built) never sees this write.
	content := "---\ntitle: From External Editor\nstatus: todo\ntype: task\n---\n\nexternal body\n"
	if err := os.WriteFile(filepath.Join(itemsDir, "ext1--external.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	coreB := New(itemsDir, coreA.Config())
	coreB.SetWarnWriter(nil)
	if err := coreB.Load(); err != nil {
		t.Fatalf("Load() (process B) error = %v", err)
	}
	defer coreB.Close()

	results, err := coreB.Search("External")
	if err != nil {
		t.Fatalf("Search() (process B) error = %v", err)
	}
	if len(results) != 1 || results[0].ID != "ext1" {
		t.Fatalf("Search() (process B) = %v, want [ext1]: external edit not reflected", results)
	}
}
