package search

import (
	"reflect"
	"sync"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

func syncItem(id, title, path string) *item.Item {
	return &item.Item{ID: id, Title: title, Path: path}
}

func hits(t *testing.T, idx *Index, q string) []string {
	t.Helper()
	ids, err := idx.Search(q, 0)
	if err != nil {
		t.Fatalf("Search(%q) error = %v", q, err)
	}
	return ids
}

func TestSync_IndexesNewChangedAndRetractsDeletedItems(t *testing.T) {
	idx := setupTestIndex(t)
	a := syncItem("aaa1", "Keep Me", "aaa1.md")
	b := syncItem("bbb2", "Delete Me", "bbb2.md")
	if err := idx.Sync([]*item.Item{a, b}); err != nil {
		t.Fatalf("Sync() #1 error = %v", err)
	}
	if got := hits(t, idx, "Delete"); !reflect.DeepEqual(got, []string{"bbb2"}) {
		t.Fatalf("after first Sync, Search(Delete) = %v, want [bbb2]", got)
	}

	a2 := syncItem("aaa1", "Edited Title", "aaa1.md")
	if err := idx.Sync([]*item.Item{a2}); err != nil {
		t.Fatalf("Sync() #2 error = %v", err)
	}
	if got := hits(t, idx, "Delete"); len(got) != 0 {
		t.Errorf("Search(Delete) = %v, want none: deleted item not retracted", got)
	}
	if got := hits(t, idx, "Edited"); !reflect.DeepEqual(got, []string{"aaa1"}) {
		t.Errorf("Search(Edited) = %v, want [aaa1]: changed item not reindexed", got)
	}
	if got := hits(t, idx, "Keep"); len(got) != 0 {
		t.Errorf("Search(Keep) = %v, want none: old title still indexed", got)
	}
}

// A rename changes Path but not the ETag; Sync must still notice it.
func TestSync_ReindexesOnPathChangeEvenWithUnchangedETag(t *testing.T) {
	idx := setupTestIndex(t)
	a := &item.Item{ID: "aaa1", Slug: "old-slug", Title: "T", Path: "aaa1--old-slug.md"}
	if err := idx.Sync([]*item.Item{a}); err != nil {
		t.Fatalf("Sync() #1 error = %v", err)
	}
	renamed := &item.Item{ID: "aaa1", Slug: "new-slug", Title: "T", Path: "aaa1--new-slug.md"}
	if err := idx.Sync([]*item.Item{renamed}); err != nil {
		t.Fatalf("Sync() #2 error = %v", err)
	}
	if got := hits(t, idx, "slug:new"); !reflect.DeepEqual(got, []string{"aaa1"}) {
		t.Errorf("Search(slug:new) = %v, want [aaa1]", got)
	}
}

// Core.Search runs Bleve queries outside the Core lock while writers index.
func TestSearch_ConcurrentWithIndexItem(t *testing.T) {
	idx := setupTestIndex(t)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := idx.IndexItem(syncItem("aaa1", "Login", "aaa1.md")); err != nil {
				t.Errorf("IndexItem() error = %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if _, err := idx.Search("login", 0); err != nil {
				t.Errorf("Search() error = %v", err)
				return
			}
		}
	}()
	wg.Wait()
}
