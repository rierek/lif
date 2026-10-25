package data

import (
	"github.com/rierek/lif/pkg/item"
	"reflect"
	"testing"
)

// idsOf extracts IDs in order, for compact order assertions.
func idsOf(beans []*item.Item) []string {
	ids := make([]string, len(beans))
	for i, b := range beans {
		ids[i] = b.ID
	}
	return ids
}

func assertIDOrder(t *testing.T, got []*item.Item, want []string) {
	t.Helper()

	gotIDs := idsOf(got)
	if len(gotIDs) != len(want) {
		t.Fatalf("got %d beans %v, want %d beans %v", len(gotIDs), gotIDs, len(want), want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("order mismatch at %d: got %v, want %v", i, gotIDs, want)
		}
	}
}

func TestIndexChildrenSorted(t *testing.T) {
	beans := []*item.Item{
		// status=scrapped -> last overall regardless of anything else.
		{ID: "b-task-zeta", Parent: "par1", Status: "scrapped", Priority: "normal", Type: "task", Title: "Zeta"},
		// status=in-progress -> first overall.
		{ID: "b-inprog", Parent: "par1", Status: "in-progress", Priority: "critical", Type: "bug", Title: "Omega"},
		// status=todo, priority=low, type=task.
		{ID: "b-task-mango", Parent: "par1", Status: "todo", Priority: "low", Type: "task", Title: "Mango"},
		// status=todo, priority="" (defaults to normal), type=task.
		{ID: "b-task-normal", Parent: "par1", Status: "todo", Priority: "", Type: "task", Title: "Normal-ish"},
		// status=todo, priority=critical, type=feature.
		{ID: "b-feat-banana", Parent: "par1", Status: "todo", Priority: "critical", Type: "feature", Title: "Banana"},
		// status=todo, priority=critical, type=epic, title differs only by case from b-epic-apple.
		{ID: "b-epic-banana", Parent: "par1", Status: "todo", Priority: "critical", Type: "epic", Title: "banana"},
		{ID: "b-epic-apple", Parent: "par1", Status: "todo", Priority: "critical", Type: "epic", Title: "Apple"},
		// unrelated parent, must not leak into par1's children.
		{ID: "b-other-child", Parent: "par2", Status: "todo", Priority: "critical", Type: "epic", Title: "Aaa"},
	}

	idx := NewIndex(beans)

	assertIDOrder(t, idx.Children["par1"], []string{
		"b-inprog",      // in-progress
		"b-epic-apple",  // todo, critical, epic, "Apple" < "banana" (case-insensitive)
		"b-epic-banana", // todo, critical, epic
		"b-feat-banana", // todo, critical, feature (type rank after epic)
		"b-task-normal", // todo, normal (empty priority default), task
		"b-task-mango",  // todo, low, task
		"b-task-zeta",   // scrapped
	})
}

func TestUnknownEnumValuesSortLast(t *testing.T) {
	beans := []*item.Item{
		// unrecognized status ("blocked") -> falls back to sort-last (rank
		// len(statusOrder)), so it must sort after "scrapped" (rank 4), the
		// last recognized status value.
		{ID: "b-blocked", Parent: "par1", Status: "blocked", Priority: "normal", Type: "task", Title: "Alpha"},
		{ID: "b-scrapped", Parent: "par1", Status: "scrapped", Priority: "normal", Type: "task", Title: "Alpha"},
	}

	idx := NewIndex(beans)

	assertIDOrder(t, idx.Children["par1"], []string{"b-scrapped", "b-blocked"})
}

// TestSortBeansExportedMatchesCanonicalTierOrder guards the I03-preparation
// export (bean bt-ms0k / bt-7jr8 T8-review): SortBeans must be the exact
// same single-source order sortBeans already provides for Index's own
// methods, now usable by callers outside this package (E2 Task 1's
// Beziehungen section resolves Blocking/BlockedBy IDs into []*Bean itself
// and must not invent a second, parallel tie-break).
func TestSortBeansExportedMatchesCanonicalTierOrder(t *testing.T) {
	beans := []*item.Item{
		{ID: "b", Status: "todo", Priority: "normal", Type: "task", Title: "B"},
		{ID: "a", Status: "in-progress", Priority: "high", Type: "bug", Title: "A"},
	}
	SortBeans(beans)
	if beans[0].ID != "a" { // in-progress sorts before todo
		t.Fatalf("SortBeans order = %v, want a before b", beans)
	}
}

// TestStatusRankOrdersLifecycle guards the exported StatusRank wrapper
// reproducing statusOrder's documented tier order.
func TestStatusRankOrdersLifecycle(t *testing.T) {
	if !(StatusRank("in-progress") < StatusRank("todo") &&
		StatusRank("todo") < StatusRank("draft") &&
		StatusRank("draft") < StatusRank("completed") &&
		StatusRank("completed") < StatusRank("scrapped")) {
		t.Fatal("StatusRank does not reproduce the documented tier order")
	}
}

// TestPriorityRankEmptyDefaultsNormal guards the exported PriorityRank
// wrapper's empty-priority handling (mirrors sortBeans' own "" -> "normal").
func TestPriorityRankEmptyDefaultsNormal(t *testing.T) {
	if PriorityRank("") != PriorityRank("normal") {
		t.Fatalf("PriorityRank(\"\") = %d, want == PriorityRank(normal) = %d",
			PriorityRank(""), PriorityRank("normal"))
	}
	if !(PriorityRank("critical") < PriorityRank("high") && PriorityRank("high") < PriorityRank("normal")) {
		t.Fatal("PriorityRank does not reproduce the documented tier order")
	}
}

// --- Enum single source (E3 Task 1, bean bt-dlgk, design decision b):
// StatusValues/TypeValues/PriorityValues are the canonical ORDERED slices
// the box_filter_facets.go/box_menu_value.go menus both consume instead of
// re-hardcoding the beans enums; statusOrder/typeOrder/priorityOrder are
// DERIVED from them (position == rank), not maintained as a second,
// independently-ordered map. ---

func TestStatusValuesCanonicalTierOrder(t *testing.T) {
	want := []string{"in-progress", "todo", "draft", "completed", "scrapped"}
	if got := StatusValues(); !reflect.DeepEqual(got, want) {
		t.Fatalf("StatusValues() = %v, want %v", got, want)
	}
}

func TestTypeValuesCanonicalTierOrder(t *testing.T) {
	want := []string{"milestone", "epic", "bug", "feature", "task"}
	if got := TypeValues(); !reflect.DeepEqual(got, want) {
		t.Fatalf("TypeValues() = %v, want %v", got, want)
	}
}

func TestPriorityValuesCanonicalTierOrder(t *testing.T) {
	want := []string{"critical", "high", "normal", "low", "deferred"}
	if got := PriorityValues(); !reflect.DeepEqual(got, want) {
		t.Fatalf("PriorityValues() = %v, want %v", got, want)
	}
}

// TestValueSlicesAreDefensiveCopies guards that a caller mutating the
// returned slice can never corrupt the package-level canonical order for the
// next caller (StatusValues/TypeValues/PriorityValues each return a fresh
// copy).
func TestValueSlicesAreDefensiveCopies(t *testing.T) {
	StatusValues()[0] = "mutated"
	if StatusValues()[0] != "in-progress" {
		t.Fatal("caller mutation of StatusValues()'s returned slice leaked into the next call")
	}
	TypeValues()[0] = "mutated"
	if TypeValues()[0] != "milestone" {
		t.Fatal("caller mutation of TypeValues()'s returned slice leaked into the next call")
	}
	PriorityValues()[0] = "mutated"
	if PriorityValues()[0] != "critical" {
		t.Fatal("caller mutation of PriorityValues()'s returned slice leaked into the next call")
	}
}

// TestOrderMapsDerivedFromValueSlices is the Single-Source-Guard: the rank
// maps behind StatusRank/PriorityRank/sortBeans MUST be derived from the
// exported ordered slices -- a bean's position in StatusValues() must equal
// its StatusRank, or the two could silently drift apart into two competing
// "canonical" orders.
func TestOrderMapsDerivedFromValueSlices(t *testing.T) {
	for i, s := range StatusValues() {
		if StatusRank(s) != i {
			t.Fatalf("StatusRank(%q)=%d, want %d (position in StatusValues())", s, StatusRank(s), i)
		}
	}
	for i, p := range PriorityValues() {
		if PriorityRank(p) != i {
			t.Fatalf("PriorityRank(%q)=%d, want %d (position in PriorityValues())", p, PriorityRank(p), i)
		}
	}
	for i, ty := range TypeValues() {
		if rank(cur.Load().typeOrder, ty) != i {
			t.Fatalf("typeOrder[%q]=%d, want %d (position in TypeValues())", ty, rank(cur.Load().typeOrder, ty), i)
		}
	}
}

func TestWithTagToReview(t *testing.T) {
	beans := []*item.Item{
		{ID: "tag-a", Status: "todo", Priority: "normal", Type: "task", Title: "Alpha", Tags: []string{"to-review", "misc"}},
		{ID: "tag-b", Status: "todo", Priority: "normal", Type: "task", Title: "Bravo", Tags: []string{"misc"}},
		{ID: "tag-c", Status: "in-progress", Priority: "normal", Type: "task", Title: "Charlie", Tags: []string{"to-review"}},
		{ID: "tag-d", Status: "todo", Priority: "normal", Type: "task", Title: "Delta"},
	}

	idx := NewIndex(beans)

	// Status order: in-progress (0) before todo (1).
	assertIDOrder(t, idx.WithTag("to-review"), []string{"tag-c", "tag-a"})
}
