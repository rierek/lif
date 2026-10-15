package itemcore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// TestIndexDir_OutsideStoreRoot proves AC-04: the persisted index location is
// never inside the store root, so writeGitignore's ".conversations/"-only
// exclusion can never leave it git-tracked.
func TestIndexDir_OutsideStoreRoot(t *testing.T) {
	core, itemsDir := setupTestCore(t)
	defer core.Close()

	dir, err := core.indexDir()
	if err != nil {
		t.Fatalf("indexDir() error = %v", err)
	}

	absRoot, err := filepath.Abs(itemsDir)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	rel, err := filepath.Rel(absRoot, dir)
	if err != nil {
		t.Fatalf("Rel() error = %v", err)
	}
	if rel == "." || !strings.HasPrefix(rel, "..") {
		t.Fatalf("indexDir() = %q is inside the store root %q (rel = %q), want outside", dir, absRoot, rel)
	}
}

// TestIndexDir_DistinctPerStore proves two different stores never collide on
// the same persisted index directory (which would let one project's search
// results leak into another's).
func TestIndexDir_DistinctPerStore(t *testing.T) {
	core1, _ := setupTestCore(t)
	defer core1.Close()
	core2, _ := setupTestCore(t)
	defer core2.Close()

	dir1, err := core1.indexDir()
	if err != nil {
		t.Fatalf("indexDir() #1 error = %v", err)
	}
	dir2, err := core2.indexDir()
	if err != nil {
		t.Fatalf("indexDir() #2 error = %v", err)
	}
	if dir1 == dir2 {
		t.Fatalf("two distinct stores resolved to the same index directory %q", dir1)
	}
}

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
		// beans-4t2m's lazy-upgrade helpers: ensureWritableSearchIndexLocked
		// is the single seam every write path (Create/Update/Delete, the
		// watcher paths below, and loadFromDisk's resync) goes through
		// before touching c.searchIndex, and upgradeSearchIndexLocked is
		// the shared close-and-reopen-exclusively implementation it and
		// ensureSearchIndexLocked both call.
		"ensureWritableSearchIndexLocked": true,
		"upgradeSearchIndexLocked":        true,
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
// once that instance reloads. The two Core instances share the same
// persisted index directory (derived purely from the store root), and the
// first releases its lock via Close() before the second opens it, so the
// second genuinely opens the persisted index the first one wrote.
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

// TestSearch_ConcurrentCoresDegradeGracefully forces AC-07/SC-04's
// concurrency, rather than hoping for it: two Core instances over the same
// store -- and therefore the same persisted index directory -- run Search
// at the same time via a barrier, without either closing first. Both must
// return correct results with no hang, crash, or error; at most one of them
// can be holding the persisted index's lock, so at least one is proven to
// be running on the in-memory fallback.
func TestSearch_ConcurrentCoresDegradeGracefully(t *testing.T) {
	coreA, itemsDir := setupTestCore(t)
	defer coreA.Close()
	if err := coreA.Create(&item.Item{ID: "aaa1", Slug: "a", Title: "Concurrent Alpha", Body: "x"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	coreB := New(itemsDir, coreA.Config())
	coreB.SetWarnWriter(nil)
	if err := coreB.Load(); err != nil {
		t.Fatalf("Load() (process B) error = %v", err)
	}
	defer coreB.Close()

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([][]*item.Item, 2)
	errs := make([]error, 2)
	for i, c := range []*Core{coreA, coreB} {
		wg.Add(1)
		go func(i int, c *Core) {
			defer wg.Done()
			<-start
			results[i], errs[i] = c.Search("Concurrent")
		}(i, c)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Search() core %d error = %v", i, err)
		}
		if len(results[i]) != 1 || results[i][0].ID != "aaa1" {
			t.Fatalf("Search() core %d = %v, want [aaa1]", i, results[i])
		}
	}
}

// TestSearch_SurvivesDeletedPersistedIndex proves AC-05/SC-03 at the Core
// level: deleting the persisted index directory out from under a running
// process still yields a correct search, at the cost of one rebuild, and
// does not affect non-search reads like All()/Get().
func TestSearch_SurvivesDeletedPersistedIndex(t *testing.T) {
	core, _ := setupTestCore(t)
	defer core.Close()
	if err := core.Create(&item.Item{ID: "aaa1", Slug: "a", Title: "Findable", Body: "x"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := core.Search("Findable"); err != nil {
		t.Fatalf("Search() #1 error = %v", err)
	}

	dir, err := core.indexDir()
	if err != nil {
		t.Fatalf("indexDir() error = %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}

	if all := core.All(); len(all) != 1 {
		t.Fatalf("All() = %d beans, want 1: unaffected by index deletion", len(all))
	}
	if _, err := core.Get("aaa1"); err != nil {
		t.Fatalf("Get() error = %v: unaffected by index deletion", err)
	}

	// The in-process index handle is still the one built before deletion;
	// deletion under a live process is expected to only matter on the next
	// open (a fresh Core, simulating the next invocation).
	core2 := New(core.Root(), core.Config())
	core2.SetWarnWriter(nil)
	if err := core2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	defer core2.Close()
	results, err := core2.Search("Findable")
	if err != nil {
		t.Fatalf("Search() after deletion error = %v", err)
	}
	if len(results) != 1 || results[0].ID != "aaa1" {
		t.Fatalf("Search() after deletion = %v, want [aaa1]", results)
	}
}

// setupTestCoreWithHome is setupTestCore's shape but with the caller
// choosing HOME explicitly, so several stores in one test can share the
// same ~/.beans/index/ parent directory (setupTestCore always points HOME
// at a fresh t.TempDir() per call, which would give each store its own,
// unrelated index tree and make the prune-on-open tests below vacuous).
func setupTestCoreWithHome(t *testing.T, home string) (*Core, string) {
	t.Helper()
	t.Setenv("HOME", home)
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, config.DefaultStoreDir)
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}
	core := New(itemsDir, config.Default())
	core.SetWarnWriter(nil)
	if err := core.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}
	return core, itemsDir
}

// TestIndexDir_Marker proves AC-01/AC-02/SC-03: opening a persisted index
// writes a marker recording the store's absolute root, and reopening the
// same store later leaves that marker untouched rather than rewriting
// identical content on every open.
func TestIndexDir_Marker(t *testing.T) {
	home := t.TempDir()
	coreA, itemsDir := setupTestCoreWithHome(t, home)
	if _, err := coreA.Search("x"); err != nil {
		t.Fatalf("Search() #1 error = %v", err)
	}

	dir, err := coreA.indexDir()
	if err != nil {
		t.Fatalf("indexDir() error = %v", err)
	}
	markerPath := filepath.Join(dir, indexMarkerFileName)
	wantRoot, err := filepath.Abs(itemsDir)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("reading marker after first open: %v", err)
	}
	if string(content) != wantRoot {
		t.Fatalf("marker content = %q, want %q", content, wantRoot)
	}
	before, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("stat marker after first open: %v", err)
	}
	if err := coreA.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	coreB := New(itemsDir, coreA.Config())
	coreB.SetWarnWriter(nil)
	if err := coreB.Load(); err != nil {
		t.Fatalf("Load() (reopen) error = %v", err)
	}
	defer coreB.Close()
	if _, err := coreB.Search("x"); err != nil {
		t.Fatalf("Search() #2 (reopen) error = %v", err)
	}

	after, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("stat marker after reopen: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("marker was rewritten on reopen: mtime before = %v, after = %v", before.ModTime(), after.ModTime())
	}
	content2, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("reading marker after reopen: %v", err)
	}
	if string(content2) != wantRoot {
		t.Fatalf("marker content after reopen = %q, want %q (should be unchanged)", content2, wantRoot)
	}
}

// TestIndexDir_PruneOnOpen_RemovesOrphan proves AC-03/SC-01: a sibling
// index directory whose marker points at a store root that no longer
// exists on disk is removed the next time any store under the same HOME
// opens its index.
func TestIndexDir_PruneOnOpen_RemovesOrphan(t *testing.T) {
	home := t.TempDir()
	tmpA := t.TempDir()
	itemsDirA := filepath.Join(tmpA, config.DefaultStoreDir)
	if err := os.MkdirAll(itemsDirA, 0755); err != nil {
		t.Fatalf("failed to create store A .beans dir: %v", err)
	}
	t.Setenv("HOME", home)
	coreA := New(itemsDirA, config.Default())
	coreA.SetWarnWriter(nil)
	if err := coreA.Load(); err != nil {
		t.Fatalf("Load() (store A) error = %v", err)
	}
	if _, err := coreA.Search("x"); err != nil {
		t.Fatalf("Search() (store A) error = %v", err)
	}
	dirA, err := coreA.indexDir()
	if err != nil {
		t.Fatalf("indexDir() (store A) error = %v", err)
	}
	if err := coreA.Close(); err != nil {
		t.Fatalf("Close() (store A) error = %v", err)
	}

	// Store A's root is gone: its index directory is now an orphan.
	if err := os.RemoveAll(tmpA); err != nil {
		t.Fatalf("RemoveAll(tmpA) error = %v", err)
	}

	indexRoot := filepath.Dir(dirA)
	before, err := os.ReadDir(indexRoot)
	if err != nil {
		t.Fatalf("ReadDir(indexRoot) before error = %v", err)
	}
	nBefore := len(before)

	// Store B, a second, unrelated store under the same HOME: opening it
	// creates its own sibling directory (indexRoot now holds {A, B}) and
	// triggers the prune-on-open pass in the same call, which should
	// remove A and leave only B -- so the net count from nBefore (which
	// only saw A) is unchanged, but the *survivor* must be B, not A.
	coreB, _ := setupTestCoreWithHome(t, home)
	defer coreB.Close()
	if _, err := coreB.Search("y"); err != nil {
		t.Fatalf("Search() (store B) error = %v", err)
	}
	dirB, err := coreB.indexDir()
	if err != nil {
		t.Fatalf("indexDir() (store B) error = %v", err)
	}

	after, err := os.ReadDir(indexRoot)
	if err != nil {
		t.Fatalf("ReadDir(indexRoot) after error = %v", err)
	}
	if len(after) != nBefore {
		t.Fatalf("index dir entries after = %d, want %d (A pruned, B added: net unchanged from nBefore)", len(after), nBefore)
	}
	if _, err := os.Stat(dirA); !os.IsNotExist(err) {
		t.Fatalf("orphaned index dir %s still present after prune (stat err = %v)", dirA, err)
	}
	if _, err := os.Stat(dirB); err != nil {
		t.Fatalf("store B's own index dir %s missing after its own open: %v", dirB, err)
	}
}

// TestIndexDir_PruneOnOpen_IgnoresUnparseableMarker guards the AC-04 edge
// case an early version of pruneOrphanIndexDirs got wrong: os.Stat("")
// reports ENOENT just like a genuinely deleted store root, so a truncated
// or empty marker must never be read as "verified gone" -- only a marker
// whose content actually resolves to an absolute, now-missing path counts
// as evidence of an orphan.
func TestIndexDir_PruneOnOpen_IgnoresUnparseableMarker(t *testing.T) {
	home := t.TempDir()
	indexRoot := filepath.Join(home, ".lif", "index")
	staleDir := filepath.Join(indexRoot, "not-a-real-hash")
	if err := os.MkdirAll(staleDir, 0755); err != nil {
		t.Fatalf("MkdirAll(staleDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(staleDir, indexMarkerFileName), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(marker) error = %v", err)
	}

	coreB, _ := setupTestCoreWithHome(t, home)
	defer coreB.Close()
	if _, err := coreB.Search("y"); err != nil {
		t.Fatalf("Search() (store B) error = %v", err)
	}

	if _, err := os.Stat(staleDir); err != nil {
		t.Fatalf("directory with an empty/unparseable marker was pruned: stat err = %v", err)
	}
}

// TestIndexDir_PruneOnOpen_KeepsLiveStore proves AC-06: a store whose root
// still exists on disk keeps its index directory across a sibling's
// prune-on-open pass.
func TestIndexDir_PruneOnOpen_KeepsLiveStore(t *testing.T) {
	home := t.TempDir()
	coreA, _ := setupTestCoreWithHome(t, home)
	if _, err := coreA.Search("x"); err != nil {
		t.Fatalf("Search() (store A) error = %v", err)
	}
	dirA, err := coreA.indexDir()
	if err != nil {
		t.Fatalf("indexDir() (store A) error = %v", err)
	}
	if err := coreA.Close(); err != nil {
		t.Fatalf("Close() (store A) error = %v", err)
	}
	if _, err := os.Stat(dirA); err != nil {
		t.Fatalf("test precondition failed: index dir %s missing before prune pass: %v", dirA, err)
	}

	coreB, _ := setupTestCoreWithHome(t, home)
	defer coreB.Close()
	if _, err := coreB.Search("y"); err != nil {
		t.Fatalf("Search() (store B) error = %v", err)
	}

	if _, err := os.Stat(dirA); err != nil {
		t.Fatalf("live store's index dir %s was pruned: stat err = %v", dirA, err)
	}
}

// TestIndexDir_PruneOnOpen_ContinuesPastRemoveFailure proves pruneOrphanIndexDirs
// does not abort on the first os.RemoveAll failure: with two orphaned
// siblings and the first (in os.ReadDir sorted order) made undeletable, the
// second orphan must still be removed, and the returned error must mention
// the first sibling's failure rather than being swallowed.
func TestIndexDir_PruneOnOpen_ContinuesPastRemoveFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: chmod does not prevent removal")
	}

	home := t.TempDir()
	indexRoot := filepath.Join(home, ".lif", "index")
	siblingA := filepath.Join(indexRoot, "sibling-a-undeletable")
	siblingB := filepath.Join(indexRoot, "sibling-b-removable")

	for _, dir := range []string{siblingA, siblingB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
		goneRoot := filepath.Join(home, "gone-"+filepath.Base(dir))
		if err := os.WriteFile(filepath.Join(dir, indexMarkerFileName), []byte(goneRoot), 0o644); err != nil {
			t.Fatalf("WriteFile(marker, %s) error = %v", dir, err)
		}
	}

	if err := os.Chmod(siblingA, 0o500); err != nil {
		t.Fatalf("Chmod(siblingA) error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(siblingA, 0o755); err != nil {
			t.Logf("Chmod(siblingA) cleanup error = %v", err)
		}
	})

	err := pruneOrphanIndexDirs(indexRoot)
	if err == nil {
		t.Fatalf("pruneOrphanIndexDirs() error = nil, want non-nil mentioning %s", siblingA)
	}
	if !strings.Contains(err.Error(), "sibling-a-undeletable") {
		t.Fatalf("pruneOrphanIndexDirs() error = %q, want it to mention the undeletable sibling", err.Error())
	}

	if _, statErr := os.Stat(siblingB); !os.IsNotExist(statErr) {
		t.Fatalf("orphaned sibling %s still present after prune (stat err = %v), want it removed despite siblingA's failure", siblingB, statErr)
	}
	if _, statErr := os.Stat(siblingA); statErr != nil {
		t.Fatalf("undeletable sibling %s missing entirely (stat err = %v), want it left in place after the failed removal", siblingA, statErr)
	}
}
