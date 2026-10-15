package search

import (
	"path/filepath"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// TestIndex_ReadOnly_ExclusiveAndInMemoryAreWritable proves the negative
// half of beans-4t2m's ReadOnly contract: an exclusively opened persisted
// index and an in-memory index are never reported read-only, only a real
// persisted index obtained through OpenRead is.
func TestIndex_ReadOnly_ExclusiveAndInMemoryAreWritable(t *testing.T) {
	mem, err := NewIndex()
	if err != nil {
		t.Fatalf("NewIndex() error = %v", err)
	}
	defer mem.Close()
	if mem.ReadOnly() {
		t.Fatal("NewIndex() reported ReadOnly() == true; an in-memory index must always be writable")
	}

	dir := filepath.Join(t.TempDir(), "idx")
	writer, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer writer.Close()
	if !writer.persistent {
		t.Fatal("Open() on a fresh, uncontended directory should have acquired the persistent index")
	}
	if writer.ReadOnly() {
		t.Fatal("Open() (exclusive) reported ReadOnly() == true; the write-caller entry point must always be writable")
	}
}

// TestIndex_ReadOnly_TrueForSharedPersisted proves the positive half: a
// real persisted index obtained through OpenRead reports ReadOnly() ==
// true, which is exactly the signal pkg/itemcore's upgrade path relies on.
func TestIndex_ReadOnly_TrueForSharedPersisted(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "idx")

	seed, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() (seed) error = %v", err)
	}
	if err := seed.IndexItem(itemWith("seed1", "Seed", "x")); err != nil {
		t.Fatalf("IndexBean() (seed) error = %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("Close() (seed) error = %v", err)
	}

	reader, err := OpenRead(dir)
	if err != nil {
		t.Fatalf("OpenRead() error = %v", err)
	}
	defer reader.Close()
	if !reader.persistent {
		t.Fatal("OpenRead() degraded to an in-memory index; nothing else holds the lock yet")
	}
	if !reader.ReadOnly() {
		t.Fatal("OpenRead() reported ReadOnly() == false for a real persisted index; want true")
	}
}

// TestIndex_ReadOnly_InMemoryFallbackFromOpenReadIsWritable proves the
// fallback edge case: when OpenRead itself degrades to an in-memory index
// (e.g. the on-disk lock is contended), the result must still be writable
// -- only a genuinely persisted, read-only-opened Bleve handle is not.
func TestIndex_ReadOnly_InMemoryFallbackFromOpenReadIsWritable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "idx")

	holder, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() (holder) error = %v", err)
	}
	defer holder.Close()

	reader, err := OpenRead(dir)
	if err != nil {
		t.Fatalf("OpenRead() error = %v", err)
	}
	defer reader.Close()
	if reader.persistent {
		t.Fatal("OpenRead() acquired the persistent index while an exclusive holder was active; want degrade to in-memory")
	}
	if reader.ReadOnly() {
		t.Fatal("OpenRead()'s in-memory fallback reported ReadOnly() == true; the fallback must always be writable")
	}
}

// TestIndex_NeedsSync proves NeedsSync answers exactly what a later Sync
// with the same items would do, without mutating the index or its etags:
// unchanged items report no work, and an added, edited, or removed item
// each report work is needed.
func TestIndex_NeedsSync(t *testing.T) {
	idx := setupTestIndex(t)

	a := itemWith("aaa1", "Alpha", "x")
	b := itemWith("bbb2", "Beta", "y")

	if !idx.NeedsSync([]*item.Item{a, b}) {
		t.Fatal("NeedsSync() = false on a never-synced index with beans present; want true")
	}

	if err := idx.Sync([]*item.Item{a, b}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if idx.NeedsSync([]*item.Item{a, b}) {
		t.Fatal("NeedsSync() = true immediately after Sync() with the same beans; want false")
	}

	// Editing an item's body changes its ETag, and therefore its staleKey.
	edited := itemWith("aaa1", "Alpha", "x-edited")
	if !idx.NeedsSync([]*item.Item{edited, b}) {
		t.Fatal("NeedsSync() = false after editing a bean's body; want true")
	}

	// A new item not yet indexed.
	c := itemWith("ccc3", "Gamma", "z")
	if !idx.NeedsSync([]*item.Item{a, b, c}) {
		t.Fatal("NeedsSync() = false with a bean added since the last Sync; want true")
	}

	// An item removed since the last Sync.
	if !idx.NeedsSync([]*item.Item{a}) {
		t.Fatal("NeedsSync() = false with a bean removed since the last Sync; want true")
	}

	// NeedsSync must not have mutated idx.etags: a real Sync afterwards
	// still has work to do for the same diff.
	if err := idx.Sync([]*item.Item{a}); err != nil {
		t.Fatalf("Sync() (after NeedsSync probes) error = %v", err)
	}
	if idx.NeedsSync([]*item.Item{a}) {
		t.Fatal("NeedsSync() = true after Sync() caught up to the probed state; want false")
	}
}
