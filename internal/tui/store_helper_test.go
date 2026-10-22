package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
)

// newTestStore opens a real data.Store over a temp repo that holds items,
// written as the files `lif` would write. Items keep their IDs; fields the
// test leaves empty stay empty. The store closes with the test.
func newTestStore(t *testing.T, items ...*item.Item) *data.Store {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte("project:\n    name: tui-test\nlif:\n    path: .lif\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	storeDir := filepath.Join(dir, ".lif")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		c := it.Clone()
		raw, err := c.Render()
		if err != nil {
			t.Fatalf("render %s: %v", it.ID, err)
		}
		name := it.ID + "--" + item.Slugify(it.Title) + ".md"
		if err := os.WriteFile(filepath.Join(storeDir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store, err := data.Open(dir, "", "")
	if err != nil {
		t.Fatalf("data.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// storeItem returns the item as it is on disk now.
func storeItem(t *testing.T, s *data.Store, id string) *item.Item {
	t.Helper()
	it, err := s.Core.Get(id)
	if err != nil {
		t.Fatalf("store.Core.Get(%s): %v", id, err)
	}
	return it
}

// fixtureStore is a store over fixtureBeans(): its items carry the same ETags
// as the fixture items a fixtureModel holds.
func fixtureStore(t *testing.T) *data.Store {
	t.Helper()
	return newTestStore(t, fixtureBeans()...)
}

// conflictingStore is fixtureStore after every item was rewritten on disk, so
// every ETag a fixtureModel holds is stale and any mutation through it is a
// genuine conflict.
func conflictingStore(t *testing.T) *data.Store {
	t.Helper()
	return conflictingStoreOf(t, fixtureBeans())
}

// conflictingStoreOf is conflictingStore over the given items.
func conflictingStoreOf(t *testing.T, items []*item.Item) *data.Store {
	t.Helper()
	s := newTestStore(t, items...)
	for _, it := range s.Items() {
		if err := s.SetTitle(it.ID, it.Title+" (rewritten)", it.ETag()); err != nil {
			t.Fatalf("rewrite %s: %v", it.ID, err)
		}
	}
	return s
}

// taggedItem is a todo task carrying tags.
func taggedItem(id string, tags ...string) *item.Item {
	return &item.Item{ID: id, Title: "Item " + id, Status: "todo", Type: "task", Priority: "normal", Tags: tags}
}
