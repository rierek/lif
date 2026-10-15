package itemcore

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rierek/lif/internal/search"
	"github.com/rierek/lif/pkg/item"
)

// runWithTimeout is pkg/itemcore's equivalent of internal/search's
// openWithTimeout (concurrent_reader_test.go): it runs fn in a goroutine
// and fails the test itself, by name, if fn has not returned within
// timeout, instead of trusting the package's own -test.timeout (10 minutes
// by default) to notice a regression to a blocking call. beans-8pde exists
// because exactly this guard was missing once; every test in this file
// that exercises the upgrade path goes through it.
func runWithTimeout(t *testing.T, timeout time.Duration, fn func() error) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("operation returned an error: %v", err)
		}
	case <-time.After(timeout):
		t.Fatalf("operation did not return within %s -- regressed to blocking on a read-only search index instead of upgrading it", timeout)
	}
}

// seedPersistedIndex creates a real, persisted (on-disk, exclusive-mode)
// search index at core's indexDir and indexes the given items into it, then
// closes it -- mirroring what an earlier `lif` write invocation would
// have left behind. Core.Search's first-ever call for a store still needs
// something on disk before OpenRead can obtain a genuinely persisted (as
// opposed to in-memory-fallback) handle, exactly like
// TestOpen_ConcurrentReadersShareOnDiskIndex's Phase 0 in
// internal/search/concurrent_reader_test.go.
func seedPersistedIndex(t *testing.T, core *Core, items ...*item.Item) {
	t.Helper()

	dir, err := core.indexDir()
	if err != nil {
		t.Fatalf("indexDir() error = %v", err)
	}
	seed, err := search.Open(dir)
	if err != nil {
		t.Fatalf("search.Open() (seed) error = %v", err)
	}
	for _, b := range items {
		if err := seed.IndexItem(b); err != nil {
			t.Fatalf("IndexBean() (seed) error = %v", err)
		}
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("Close() (seed) error = %v", err)
	}
}

// TestSearch_ThenCreate_UpgradesReadOnlyIndexWithoutBlocking proves AC-02:
// a read (Search) followed by a write (Create) in the same Core, against a
// real persisted index directory, completes without blocking. Before
// beans-4t2m, Search always cached an exclusive handle, so a later write
// never touched a read-only one; this proves the read-then-write sequence
// beans-4t2m introduces (Search now caches a shared handle) still
// completes, because Create upgrades that cached handle in place instead
// of calling IndexItem directly on it.
func TestSearch_ThenCreate_UpgradesReadOnlyIndexWithoutBlocking(t *testing.T) {
	core, _ := setupTestCore(t)
	defer core.Close()

	alpha := &item.Item{ID: "aaa1", Slug: "a", Title: "Alpha", Body: "x"}
	if err := core.Create(alpha); err != nil {
		t.Fatalf("Create() (alpha) error = %v", err)
	}
	seedPersistedIndex(t, core, alpha)

	if _, err := core.Search("Alpha"); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if core.searchIndex == nil || !core.searchIndex.ReadOnly() {
		t.Fatal("Search() did not cache a shared, read-only persisted handle; test setup did not exercise the shared-handle path")
	}

	beta := &item.Item{ID: "bbb2", Slug: "b", Title: "Beta", Body: "y"}
	runWithTimeout(t, 5*time.Second, func() error {
		return core.Create(beta)
	})

	if core.searchIndex.ReadOnly() {
		t.Fatal("cached search index is still read-only after a write; Create should have upgraded it")
	}

	results, err := core.Search("Beta")
	if err != nil {
		t.Fatalf("Search() (after write) error = %v", err)
	}
	if len(results) != 1 || results[0].ID != "bbb2" {
		t.Fatalf("Search() (after write) = %v, want [bbb2]", results)
	}
}

// TestEnsureSearchIndexLocked_UpgradeAvoidsSyncDeadlock proves AC-03
// specifically at the ensureSearchIndexLocked seam: with a shared,
// read-only handle already cached (via a prior Search), a direct write-
// intent call to ensureSearchIndexLocked(true) must upgrade rather than
// calling Sync on the read-only handle -- before beans-4t2m's upgrade
// path, Sync on a read-only-opened Bleve index hangs forever (see
// search.OpenRead's doc comment), which this test's timeout guard would
// catch.
func TestEnsureSearchIndexLocked_UpgradeAvoidsSyncDeadlock(t *testing.T) {
	core, _ := setupTestCore(t)
	defer core.Close()

	alpha := &item.Item{ID: "aaa1", Slug: "a", Title: "Alpha", Body: "x"}
	if err := core.Create(alpha); err != nil {
		t.Fatalf("Create() (alpha) error = %v", err)
	}
	seedPersistedIndex(t, core, alpha)

	core.mu.Lock()
	if err := core.ensureSearchIndexLocked(false); err != nil {
		core.mu.Unlock()
		t.Fatalf("ensureSearchIndexLocked(false) error = %v", err)
	}
	core.mu.Unlock()

	if core.searchIndex == nil || !core.searchIndex.ReadOnly() {
		t.Fatal("setup did not produce a cached, persisted, read-only search index")
	}

	runWithTimeout(t, 5*time.Second, func() error {
		core.mu.Lock()
		defer core.mu.Unlock()
		return core.ensureSearchIndexLocked(true)
	})

	if core.searchIndex.ReadOnly() {
		t.Fatal("ensureSearchIndexLocked(true) left the cached index read-only; want an upgraded, writable handle")
	}

	results, err := core.searchIndex.Search("Alpha", search.DefaultSearchLimit)
	if err != nil {
		t.Fatalf("Search() (raw index, after upgrade) error = %v", err)
	}
	if len(results) != 1 || results[0] != "aaa1" {
		t.Fatalf("Search() (raw index, after upgrade) = %v, want [aaa1]", results)
	}
}

// TestSearch_UpgradesWhenPersistedIndexIsStale proves the staleness half
// of ensureSearchIndexLocked's contract: a freshly opened shared, read-only
// index whose etags are behind the current in-memory items (another
// process wrote and saved its sidecar since this index was last synced)
// upgrades and syncs immediately, rather than silently serving stale
// results forever because a read-only handle is never Sync'd
// unconditionally.
func TestSearch_UpgradesWhenPersistedIndexIsStale(t *testing.T) {
	core, itemsDir := setupTestCore(t)
	defer core.Close()

	alpha := &item.Item{ID: "aaa1", Slug: "a", Title: "Alpha", Body: "x"}
	if err := core.Create(alpha); err != nil {
		t.Fatalf("Create() (alpha) error = %v", err)
	}
	seedPersistedIndex(t, core, alpha)

	// A second item written directly to disk, bypassing core -- core's
	// in-memory items map does not know about it yet, but Load() below
	// will pick it up, simulating a reload after an external edit.
	writeTestItemFile(t, filepath.Join(itemsDir, "ccc3--gamma.md"), "ccc3", "Gamma", "todo")

	// Load picks up the new on-disk item into c.items, but the persisted
	// index sidecar on disk (seeded above) still only knows about alpha:
	// the freshly opened shared handle Search obtains below is therefore
	// stale relative to c.items the moment it is opened.
	if err := core.Load(); err != nil {
		t.Fatalf("Load() (reload) error = %v", err)
	}

	runWithTimeout(t, 5*time.Second, func() error {
		_, err := core.Search("Gamma")
		return err
	})

	if core.searchIndex.ReadOnly() {
		t.Fatal("Search() left the cached index read-only after detecting staleness; want an upgraded, writable handle")
	}

	results, err := core.Search("Gamma")
	if err != nil {
		t.Fatalf("Search() (after upgrade) error = %v", err)
	}
	if len(results) != 1 || results[0].ID != "ccc3" {
		t.Fatalf("Search() (after upgrade) = %v, want [ccc3]", results)
	}
}
