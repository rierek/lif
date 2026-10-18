package itemgraph

import (
	"context"
	"sort"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// TestUpdateItemKeepsConcurrentProcessWrite: a second process that loaded the
// store before another process wrote the item must not overwrite that write.
// Two cores over one directory stand in for two lif processes.
func TestUpdateItemKeepsConcurrentProcessWrite(t *testing.T) {
	r1 := newTestResolver(t)
	ctx := context.Background()
	b := &item.Item{ID: "beans-cc01", Slug: "conc", Title: "Conc", Status: "todo", Type: "task"}
	if err := r1.Core.Create(b); err != nil {
		t.Fatal(err)
	}
	r2 := &CoreResolver{Core: itemcore.New(r1.Core.Root(), config.Default())}
	if err := r2.Core.Load(); err != nil {
		t.Fatal(err)
	}

	if _, err := r1.UpdateItem(ctx, b.ID, model.UpdateItemInput{AddTags: []string{"one"}}); err != nil {
		t.Fatal(err)
	}
	// r2 still holds the stale copy without tag "one".
	if _, err := r2.UpdateItem(ctx, b.ID, model.UpdateItemInput{AddTags: []string{"two"}}); err != nil {
		t.Fatalf("stale update: %v", err)
	}

	fresh := itemcore.New(r1.Core.Root(), config.Default())
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	got, err := fresh.Get(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	tags := append([]string(nil), got.Tags...)
	sort.Strings(tags)
	if len(tags) != 2 || tags[0] != "one" || tags[1] != "two" {
		t.Fatalf("tags = %v, want [one two]", tags)
	}
}

// TestUpdateItemExplicitIfMatchStillConflicts: a caller-held etag is never
// retried away.
func TestUpdateItemExplicitIfMatchStillConflicts(t *testing.T) {
	r := newTestResolver(t)
	b := &item.Item{ID: "beans-cc02", Slug: "conc", Title: "Conc", Status: "todo", Type: "task"}
	if err := r.Core.Create(b); err != nil {
		t.Fatal(err)
	}
	stale := "deadbeef"
	_, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{AddTags: []string{"x"}, IfMatch: &stale})
	if _, ok := err.(*itemcore.ETagMismatchError); !ok {
		t.Fatalf("err = %v, want ETagMismatchError", err)
	}
}
