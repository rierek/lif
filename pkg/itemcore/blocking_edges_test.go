package itemcore

import (
	"reflect"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

func TestRemoveBlockingEdgesRemovesEveryDeclarationAndKeepsTheRest(t *testing.T) {
	core, itemsDir := setupTestCore(t)
	for _, b := range []*item.Item{
		{ID: "pa", Slug: "pa", Title: "Parent", Status: "todo", Type: "epic"},
		// a->c is declared on both ends, a->b only on a, b->c only on c.
		{ID: "a", Slug: "a", Title: "A", Status: "todo", Parent: "pa", Blocking: []string{"b", "c"}},
		{ID: "b", Slug: "b", Title: "B", Status: "todo"},
		{ID: "c", Slug: "c", Title: "C", Status: "todo", BlockedBy: []string{"a", "b", "z"}},
	} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create(%s): %v", b.ID, err)
		}
	}
	warmIncoming(core, "c")

	if err := core.RemoveBlockingEdges([]BlockingEdge{{Blocker: "a", Waiting: "c"}}); err != nil {
		t.Fatalf("RemoveBlockingEdges(): %v", err)
	}
	assertIncomingConsistent(t, core, "c")

	reloaded := New(itemsDir, core.config)
	reloaded.SetWarnWriter(nil)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}
	a, _ := reloaded.Get("a")
	c, _ := reloaded.Get("c")
	if want := []string{"b"}; !reflect.DeepEqual(a.Blocking, want) {
		t.Errorf("a.Blocking = %v, want %v", a.Blocking, want)
	}
	if a.Parent != "pa" {
		t.Errorf("a.Parent = %q, want pa", a.Parent)
	}
	if want := []string{"b", "z"}; !reflect.DeepEqual(c.BlockedBy, want) {
		t.Errorf("c.BlockedBy = %v, want %v", c.BlockedBy, want)
	}
}
