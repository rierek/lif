package itemcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// benchCore builds a core holding n items without the *testing.T-bound
// helpers, so both tests and benchmarks can use it.
func benchCore(tb testing.TB, n int) *Core {
	tb.Helper()
	tb.Setenv("HOME", tb.TempDir()) // keep the persisted search index out of the real ~/.beans
	itemsDir := filepath.Join(tb.TempDir(), config.DefaultStoreDir)
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		tb.Fatalf("creating test .beans dir: %v", err)
	}
	core := New(itemsDir, config.Default())
	core.SetWarnWriter(nil)
	if err := core.Load(); err != nil {
		tb.Fatalf("core.Load() error = %v", err)
	}
	for i := 0; i < n; i++ {
		b := &item.Item{
			ID:     fmt.Sprintf("beans-b%04d", i),
			Slug:   fmt.Sprintf("bean-%04d", i),
			Title:  fmt.Sprintf("bean %04d", i),
			Status: "todo",
			Type:   "task",
			Tags:   []string{"alpha", "beta"},
			Body:   "some body text that makes the bean non-trivial to copy",
			Extra:  map[string]any{"release": "0-4-1"},
		}
		if err := core.Create(b); err != nil {
			tb.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}
	return core
}

// BenchmarkAllThenSerialize measures the shape the GraphQL query resolver
// and the subscription snapshot actually run: take the snapshot, then
// serialize it. It puts the copy cost of BenchmarkCoreAll in proportion to
// the work that always follows it.
func BenchmarkAllThenSerialize(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			core := benchCore(b, n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out, err := json.Marshal(core.All())
				if err != nil {
					b.Fatalf("json.Marshal: %v", err)
				}
				if len(out) == 0 {
					b.Fatal("empty payload")
				}
			}
		})
	}
}

func BenchmarkCoreAll(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			core := benchCore(b, n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				all := core.All()
				if len(all) != n {
					b.Fatalf("All() returned %d beans, want %d", len(all), n)
				}
			}
		})
	}
}

// TestAllReturnsIndependentCopies pins the fix for beans-sy1v: All() must
// hand out snapshots, not the live structs it keeps in its map. Callers
// serialize the result long after the read lock is gone, while writers
// mutate the stored items in place under the write lock (RemoveLinksTo,
// FixBrokenLinks, applyRenameSlug), which would let a reader observe a
// torn item.
func TestAllReturnsIndependentCopies(t *testing.T) {
	core := benchCore(t, 3)

	stored := liveItem(t, core, "beans-b0000")

	var snapshot *item.Item
	for _, b := range core.All() {
		if b.ID == stored.ID {
			snapshot = b
		}
	}
	if snapshot == nil {
		t.Fatalf("All() did not return %s", stored.ID)
	}
	if snapshot == stored {
		t.Fatal("All() returned the live bean pointer, want a copy")
	}

	// Mutating the snapshot must not reach the store — neither the scalar
	// fields nor the slices and maps behind them.
	snapshot.Title = "mutated"
	snapshot.Tags[0] = "mutated"
	snapshot.Extra["release"] = "mutated"
	snapshot.Blocking = append(snapshot.Blocking, "beans-b0001")

	if stored.Title == "mutated" {
		t.Error("mutating the snapshot's Title reached the stored bean")
	}
	if stored.Tags[0] == "mutated" {
		t.Error("mutating the snapshot's Tags reached the stored bean")
	}
	if stored.Extra["release"] == "mutated" {
		t.Error("mutating the snapshot's Extra reached the stored bean")
	}
	if len(stored.Blocking) != 0 {
		t.Errorf("appending to the snapshot's Blocking reached the stored bean: %v", stored.Blocking)
	}

	// The snapshot must still carry the identity fields callers rely on,
	// including the unexported content etag.
	if snapshot.ETag() != stored.ETag() {
		t.Errorf("snapshot ETag = %q, want %q", snapshot.ETag(), stored.ETag())
	}
}

// TestSearchReturnsIndependentCopies covers the sibling path: the GraphQL
// query resolver reads through Search() whenever a search filter is set,
// so that path must snapshot too.
func TestSearchReturnsIndependentCopies(t *testing.T) {
	core := benchCore(t, 3)

	stored := liveItem(t, core, "beans-b0000")

	results, err := core.Search("bean")
	if err != nil {
		t.Fatalf("core.Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search() returned no results")
	}
	for _, b := range results {
		if b == stored {
			t.Fatal("Search() returned the live bean pointer, want a copy")
		}
	}
}

// TestAllIsRaceFreeAgainstInPlaceWrites runs the exact shape beans-sy1v
// describes: a reader takes an All() snapshot and reads it after the lock
// is released, while a writer mutates the stored items in place under the
// write lock. Meaningful under -race (`just test-race`); harmless without.
func TestAllIsRaceFreeAgainstInPlaceWrites(t *testing.T) {
	core := benchCore(t, 20)

	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			// Re-establish a link, then have the core strip it in place.
			b, err := core.Get("beans-b0001")
			if err != nil {
				return
			}
			copied := b.Clone()
			copied.Blocking = []string{"beans-b0002"}
			if err := core.Update(copied, nil); err != nil {
				return
			}
			if _, err := core.RemoveLinksTo("beans-b0002"); err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			for _, b := range core.All() {
				// Read every field a serializer would touch.
				_ = fmt.Sprintf("%s %s %s %v %v %v", b.ID, b.Title, b.Status, b.Tags, b.Blocking, b.Extra)
			}
		}
	}()

	wg.Wait()
}
