package roadmap

import (
	"testing"

	"github.com/xRiErOS/lif/pkg/config"
	"github.com/xRiErOS/lif/pkg/item"
)

func ids(items []*item.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestSortSiblingsRankBuckets(t *testing.T) {
	items := []*item.Item{
		{ID: "t1", Type: "task", Status: "todo"},
		{ID: "f1", Type: "feature", Status: "todo", Order: "2"},
		{ID: "e1", Type: "epic", Status: "todo", Order: "3"},
		{ID: "m1", Type: "milestone", Status: "todo"},
		{ID: "f2", Type: "feature", Status: "todo", Order: "1"},
	}
	SortSiblings(config.Default(), items)
	want := []string{"m1", "f2", "f1", "e1", "t1"}
	got := ids(items)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestSortSiblingsStatusThenLeafType(t *testing.T) {
	items := []*item.Item{
		{ID: "t1", Type: "task", Status: "todo"},
		{ID: "b1", Type: "bug", Status: "todo"},
		{ID: "e1", Type: "epic", Status: "todo"},
		{ID: "e2", Type: "epic", Status: "in-progress"},
	}
	cfg := config.Default()
	SortSiblings(cfg, items)
	got := ids(items)
	// in-progress sorts before todo in the default status list; bug before task.
	want := []string{"e2", "e1", "b1", "t1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
