package itemgraph

import (
	"context"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// hierarchyNote is the sentence that explains a cycle through the hierarchy.
const hierarchyNote = "a child is blocked while its parent is blocked and counts as done once its parent is done"

func createItems(t *testing.T, r *CoreResolver, items ...*item.Item) {
	t.Helper()
	for _, b := range items {
		if b.Slug == "" {
			b.Slug = item.Slugify(b.Title)
		}
		if b.Status == "" {
			b.Status = "todo"
		}
		if err := r.Core.Create(b); err != nil {
			t.Fatalf("Create(%s) error: %v", b.ID, err)
		}
	}
}

func wantRefusal(t *testing.T, err error, path string, note bool) {
	t.Helper()
	if err == nil {
		t.Fatalf("want a refusal naming %s, got none", path)
	}
	if !strings.Contains(err.Error(), "would create cycle: "+path) {
		t.Errorf("error = %q, want it to name the cycle %s", err, path)
	}
	if got := strings.Contains(err.Error(), hierarchyNote); got != note {
		t.Errorf("error = %q, mentions the hierarchy note = %v, want %v", err, got, note)
	}
}

// beans-et2a: a ring whose links sit in both fields is refused on every write
// path, as a ring in one field always was.
func TestMixedFieldCyclesAreRefused(t *testing.T) {
	ctx := context.Background()
	r := newTestResolver(t)
	// A blocks B (stored on A), B blocks C (stored on C).
	createItems(t, r,
		&item.Item{ID: "t-a", Title: "A", Type: "task", Blocking: []string{"t-b"}},
		&item.Item{ID: "t-b", Title: "B", Type: "task"},
		&item.Item{ID: "t-c", Title: "C", Type: "task", BlockedBy: []string{"t-b"}},
		&item.Item{ID: "t-d", Title: "D", Type: "task"},
	)

	t.Run("update --blocking", func(t *testing.T) {
		_, err := r.UpdateItem(ctx, "t-c", model.UpdateItemInput{AddBlocking: []string{"t-a"}})
		wantRefusal(t, err, "[t-c t-a t-b t-c]", false)
	})
	t.Run("update --blocked-by", func(t *testing.T) {
		_, err := r.UpdateItem(ctx, "t-a", model.UpdateItemInput{AddBlockedBy: []string{"t-c"}})
		wantRefusal(t, err, "[t-a t-b t-c t-a]", false)
	})
	t.Run("addBlocking", func(t *testing.T) {
		_, err := r.AddBlocking(ctx, "t-c", "t-a", nil)
		wantRefusal(t, err, "[t-c t-a t-b t-c]", false)
	})
	t.Run("addBlockedBy", func(t *testing.T) {
		_, err := r.AddBlockedBy(ctx, "t-a", "t-c", nil)
		wantRefusal(t, err, "[t-a t-b t-c t-a]", false)
	})
	t.Run("two links of one update close the ring together", func(t *testing.T) {
		_, err := r.UpdateItem(ctx, "t-d", model.UpdateItemInput{
			AddBlocking: []string{"t-a"}, AddBlockedBy: []string{"t-c"},
		})
		wantRefusal(t, err, "[t-d t-a t-b t-c t-d]", false)
	})
	t.Run("create", func(t *testing.T) {
		_, err := r.CreateItem(ctx, model.CreateItemInput{
			Title: "E", Blocking: []string{"t-a"}, BlockedBy: []string{"t-c"},
		})
		wantRefusal(t, err, "[<new item> t-a t-b t-c <new item>]", false)
	})

	t.Run("nothing was written", func(t *testing.T) {
		for id, wantBlocking := range map[string]int{"t-a": 1, "t-b": 0, "t-c": 0, "t-d": 0} {
			got, err := r.Core.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Blocking) != wantBlocking {
				t.Errorf("%s has blocking %v, want %d entries", id, got.Blocking, wantBlocking)
			}
		}
	})

	t.Run("an unrelated link is accepted", func(t *testing.T) {
		if _, err := r.UpdateItem(ctx, "t-d", model.UpdateItemInput{AddBlocking: []string{"t-a"}}); err != nil {
			t.Errorf("D blocks A closes no ring, got %v", err)
		}
	})
}

// beans-o8bs: a blocked parent blocks its children, and a child counts as done
// once its parent is done. A blocker below the blocked item, or above it, can
// therefore never be released, and the write is refused like a cycle.
func TestBlockersAboveOrBelowTheBlockedItemAreRefused(t *testing.T) {
	ctx := context.Background()
	r := newTestResolver(t)
	createItems(t, r,
		&item.Item{ID: "e-p", Title: "P", Type: "epic"},
		&item.Item{ID: "t-d", Title: "D", Type: "task", Parent: "e-p"},
		&item.Item{ID: "t-s", Title: "S", Type: "task", Parent: "e-p"},
		&item.Item{ID: "t-free", Title: "Free", Type: "task"},
	)

	t.Run("a parent blocked by its own child", func(t *testing.T) {
		_, err := r.UpdateItem(ctx, "e-p", model.UpdateItemInput{AddBlockedBy: []string{"t-d"}})
		wantRefusal(t, err, "[e-p t-d e-p]", true)
	})
	t.Run("a child that blocks its own parent", func(t *testing.T) {
		_, err := r.AddBlocking(ctx, "t-d", "e-p", nil)
		wantRefusal(t, err, "[t-d e-p t-d]", true)
	})
	t.Run("a child blocked by its own parent", func(t *testing.T) {
		_, err := r.UpdateItem(ctx, "t-d", model.UpdateItemInput{AddBlockedBy: []string{"e-p"}})
		wantRefusal(t, err, "[t-d e-p t-d]", true)
	})
	t.Run("a parent that blocks its own child", func(t *testing.T) {
		_, err := r.AddBlocking(ctx, "e-p", "t-d", nil)
		wantRefusal(t, err, "[e-p t-d e-p]", true)
	})
	t.Run("parent and blocker in one update", func(t *testing.T) {
		_, err := r.UpdateItem(ctx, "t-free", model.UpdateItemInput{
			Parent: new("e-p"), AddBlocking: []string{"e-p"},
		})
		wantRefusal(t, err, "[t-free e-p t-free]", true)
	})
	t.Run("a new item under a parent it blocks", func(t *testing.T) {
		_, err := r.CreateItem(ctx, model.CreateItemInput{
			Title: "N", Type: new("task"), Parent: new("e-p"), Blocking: []string{"e-p"},
		})
		wantRefusal(t, err, "[<new item> e-p <new item>]", true)
	})
	t.Run("a chain that leads back into the subtree", func(t *testing.T) {
		createItems(t, r,
			&item.Item{ID: "e-q", Title: "Q", Type: "epic"},
		)
		// P waits for Q. Q waiting for a child of P would wait for itself.
		if _, err := r.UpdateItem(ctx, "e-p", model.UpdateItemInput{AddBlockedBy: []string{"e-q"}}); err != nil {
			t.Fatalf("P blocked by Q is fine on its own: %v", err)
		}
		_, err := r.UpdateItem(ctx, "e-q", model.UpdateItemInput{AddBlockedBy: []string{"t-d"}})
		wantRefusal(t, err, "[e-q e-p t-d e-q]", true)
	})
	t.Run("moving an item under the item it blocks", func(t *testing.T) {
		// P is blocked by Q now; a free item that blocks Q must not move under P.
		if _, err := r.UpdateItem(ctx, "t-free", model.UpdateItemInput{AddBlocking: []string{"e-q"}}); err != nil {
			t.Fatalf("a free item may block Q: %v", err)
		}
		_, err := r.SetParent(ctx, "t-free", new("e-p"), nil)
		wantRefusal(t, err, "[t-free e-q e-p t-free]", true)
	})
	t.Run("a sibling blocker is fine", func(t *testing.T) {
		if _, err := r.UpdateItem(ctx, "t-d", model.UpdateItemInput{AddBlockedBy: []string{"t-s"}}); err != nil {
			t.Errorf("a sibling may block a sibling: %v", err)
		}
	})
}
