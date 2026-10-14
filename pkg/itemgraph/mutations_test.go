package itemgraph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xRiErOS/lif/pkg/config"
	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemcore"
	"github.com/xRiErOS/lif/pkg/itemgraph/model"
)

// newTestResolver builds a resolver over a throwaway store.
func newTestResolver(t *testing.T) *CoreResolver {
	t.Helper()
	itemsDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("creating test .beans dir: %v", err)
	}
	c := itemcore.New(itemsDir, config.Default())
	if err := c.Load(); err != nil {
		t.Fatalf("loading core: %v", err)
	}
	return &CoreResolver{Core: c}
}

// TestUpdateItemWritesTagsInDeterministicOrder pins the tag order down. The
// merge builds its result from a Go map, whose iteration order is randomized
// per run, so without an explicit sort the same input can produce different
// files on consecutive runs.
func TestUpdateItemWritesTagsInDeterministicOrder(t *testing.T) {
	r := newTestResolver(t)
	ctx := context.Background()

	b := &item.Item{
		ID:     "beans-tag1",
		Slug:   item.Slugify("Tag ordering"),
		Title:  "Tag ordering",
		Status: "todo",
		Type:   "task",
		Tags:   []string{"zeta", "mu"},
	}
	if err := r.Core.Create(b); err != nil {
		t.Fatalf("core.Create: %v", err)
	}

	got, err := r.UpdateItem(ctx, b.ID, model.UpdateItemInput{
		AddTags:    []string{"alpha", "omega"},
		RemoveTags: []string{"mu"},
	})
	if err != nil {
		t.Fatalf("UpdateBean: %v", err)
	}

	want := []string{"alpha", "omega", "zeta"}
	if len(got.Tags) != len(want) {
		t.Fatalf("tags = %v, want %v", got.Tags, want)
	}
	for i := range want {
		if got.Tags[i] != want[i] {
			t.Fatalf("tags = %v, want %v", got.Tags, want)
		}
	}
}

func storeFiles(t *testing.T, r *CoreResolver) []string {
	t.Helper()
	entries, err := os.ReadDir(r.Core.Root())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// A slug derived from the old title follows a title change; the file is
// moved, not copied, and carries the new content.
func TestUpdateItemTitleChangeRenamesDerivedSlug(t *testing.T) {
	r := newTestResolver(t)
	b := &item.Item{ID: "beans-sl01", Slug: item.Slugify("Old title"), Title: "Old title", Status: "todo", Type: "task"}
	if err := r.Core.Create(b); err != nil {
		t.Fatal(err)
	}
	title := "New title"
	got, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	want := item.BuildFilename(b.ID, item.Slugify(title))
	if names := storeFiles(t, r); len(names) != 1 || names[0] != want {
		t.Fatalf("store files = %v, want [%s]", names, want)
	}
	if got.Path != want {
		t.Errorf("returned Path = %q, want %q", got.Path, want)
	}
	data, err := os.ReadFile(filepath.Join(r.Core.Root(), want))
	if err != nil || !strings.Contains(string(data), "title: New title") {
		t.Errorf("renamed file lacks new title (err=%v): %s", err, data)
	}
	if stored, _ := r.Core.Get(b.ID); stored.Path != want {
		t.Errorf("stored Path = %q, want %q", stored.Path, want)
	}
}

// A hand-set slug survives a title change.
func TestUpdateItemTitleChangeKeepsCustomSlug(t *testing.T) {
	r := newTestResolver(t)
	b := &item.Item{ID: "beans-sl02", Slug: "hand-set", Title: "Old title", Status: "todo", Type: "task"}
	if err := r.Core.Create(b); err != nil {
		t.Fatal(err)
	}
	title := "New title"
	if _, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{Title: &title}); err != nil {
		t.Fatal(err)
	}
	want := item.BuildFilename(b.ID, "hand-set")
	if names := storeFiles(t, r); len(names) != 1 || names[0] != want {
		t.Fatalf("store files = %v, want [%s]", names, want)
	}
}
