package tui

// bean_id_short_test.go — bean bt-pl5p (Nebenbefund N5, epic bt-vy1q): the
// left pane's rows drop the CURRENT repo's own bean-ID prefix ("sproutling-
// btv7" -> "btv7"), the freed columns go to the title. A foreign/dangling ID
// that does NOT carry the current prefix keeps its full form (cross-repo
// references must stay unambiguous).

import (
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tui/data"
)

func TestShortBeanID(t *testing.T) {
	cases := []struct {
		name, id, slug, want string
	}{
		{"own repo prefix dropped", "sproutling-btv7", "sproutling", "btv7"},
		{"short prefix dropped", "bt-pl5p", "bt", "pl5p"},
		{"foreign prefix kept in full", "lean-stack-58j0", "bt", "lean-stack-58j0"},
		{"empty slug is a no-op", "bt-pl5p", "", "bt-pl5p"},
		{"prefix without separator is not a prefix", "btx-pl5p", "bt", "btx-pl5p"},
		{"suffix must stay non-empty", "bt-", "bt", "bt-"},
		{"multi-segment slug dropped", "lean-stack-58j0", "lean-stack", "58j0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shortBeanID(c.id, c.slug); got != c.want {
				t.Fatalf("shortBeanID(%q, %q) = %q, want %q", c.id, c.slug, got, c.want)
			}
		})
	}
}

// TestTreeRowTextDropsRepoPrefix and its Backlog twin below guard the
// Grounding's own "zwei Render-Stellen" pitfall: Tree and Flat/Backlog use
// DIFFERENT row renderers, so shortening only one would let them drift.
func TestTreeRowTextDropsRepoPrefix(t *testing.T) {
	n := treeNode{bean: &data.Bean{ID: "sproutling-btv7", Title: "Go deeper", Status: "todo", Type: "task"}}
	got := stripHint(treeRowText(n, "sproutling"))
	if strings.Contains(got, "sproutling-btv7") {
		t.Fatalf("treeRowText kept the repo prefix: %q", got)
	}
	if !strings.Contains(got, "btv7") {
		t.Fatalf("treeRowText lost the ID suffix: %q", got)
	}
}

func TestBacklogRowTextDropsRepoPrefix(t *testing.T) {
	b := &data.Bean{ID: "sproutling-btv7", Title: "Go deeper", Status: "todo", Type: "task"}
	got := stripHint(backlogRowText(b, "sproutling"))
	if strings.Contains(got, "sproutling-btv7") {
		t.Fatalf("backlogRowText kept the repo prefix: %q", got)
	}
	if !strings.Contains(got, "btv7") {
		t.Fatalf("backlogRowText lost the ID suffix: %q", got)
	}
}

// TestBacklogRowTextKeepsForeignID: a bean whose ID does not carry the
// current repo's prefix is a cross-repo/dangling reference -- shortening it
// would make it ambiguous, so it stays whole.
func TestBacklogRowTextKeepsForeignID(t *testing.T) {
	b := &data.Bean{ID: "lean-stack-58j0", Title: "Foreign", Status: "todo", Type: "task"}
	got := stripHint(backlogRowText(b, "sproutling"))
	if !strings.Contains(got, "lean-stack-58j0") {
		t.Fatalf("backlogRowText shortened a foreign ID: %q", got)
	}
}

// --- bean bt-pt1r: the SAME shortening in the Picker-/Overlay rows ---
//
// bt-pl5p shortened the left pane only; the Blocking-/Parent-Picker kept
// rendering "sproutling-xglu" although their titles wrap at a much narrower
// width (wideModalWidth) -- the app contradicted itself, short in the list,
// long in the picker (PO-Beleg: tmux 80x30 gegen sproutling nach bt-a3a8).
// relationRowPrefix now takes the same `slug` treeRowText/backlogRowText do,
// so all three row renderers shorten through the ONE shortBeanID rule
// instead of each growing its own.

func TestRelationRowPrefixDropsRepoPrefix(t *testing.T) {
	rel := &data.Bean{ID: "sproutling-xglu", Title: "Picked", Status: "todo", Type: "task"}
	got := stripHint(relationRowPrefix(rel, "sproutling"))
	if strings.Contains(got, "sproutling-xglu") {
		t.Fatalf("relationRowPrefix kept the repo prefix: %q", got)
	}
	if !strings.Contains(got, "xglu") {
		t.Fatalf("relationRowPrefix lost the ID suffix: %q", got)
	}
}

func TestRelationRowPrefixKeepsForeignID(t *testing.T) {
	rel := &data.Bean{ID: "lean-stack-58j0", Title: "Foreign", Status: "todo", Type: "task"}
	got := stripHint(relationRowPrefix(rel, "sproutling"))
	if !strings.Contains(got, "lean-stack-58j0") {
		t.Fatalf("relationRowPrefix shortened a foreign ID: %q", got)
	}
}

// TestRelationRowKeepsFullIDInDetail is the explicit counter-guard for the
// bean's "bewusst NICHT aendern": relationRow is the DETAIL pane's renderer
// (view_detail_bean.go), and bt-pl5p's decision that the full, copyable ID
// must remain visible somewhere applies there. It passes slug "" -- a
// no-op -- so a future refactor that "helpfully" threads the real slug
// through this path trips this test instead of silently removing the last
// place the whole ID can be read.
func TestRelationRowKeepsFullIDInDetail(t *testing.T) {
	rel := &data.Bean{ID: "sproutling-xglu", Title: "Picked", Status: "todo", Type: "task"}
	got := stripHint(relationRow(rel, relationRowMarker(false, 0, 1), 60))
	if !strings.Contains(got, "sproutling-xglu") {
		t.Fatalf("relationRow (detail pane) shortened the ID -- the full ID must stay copyable there: %q", got)
	}
}

// TestParentPickerItemsDropRepoPrefix / TestBlockingPickerItemsDropRepoPrefix
// guard BOTH picker builders separately for the same reason bt-pl5p guarded
// Tree and Backlog separately: they are two independent call sites, and
// shortening only one would let the two overlays drift apart visually.
func TestParentPickerItemsDropRepoPrefix(t *testing.T) {
	idx := data.NewIndex([]data.Bean{
		{ID: "sproutling-aaaa", Title: "Child", Status: "todo", Type: "task"},
		{ID: "sproutling-xglu", Title: "Epic", Status: "todo", Type: "epic"},
	})
	child := idx.ByID["sproutling-aaaa"]

	items := buildParentItems(idx, child, "sproutling")
	var found bool
	for _, it := range items {
		if it.id != "sproutling-xglu" {
			continue
		}
		found = true
		got := stripHint(it.prefix)
		if strings.Contains(got, "sproutling-xglu") {
			t.Fatalf("parent picker row kept the repo prefix: %q", got)
		}
		if !strings.Contains(got, "xglu") {
			t.Fatalf("parent picker row lost the ID suffix: %q", got)
		}
	}
	if !found {
		t.Fatalf("buildParentItems did not offer the eligible parent at all: %+v", items)
	}
}

func TestBlockingPickerItemsDropRepoPrefix(t *testing.T) {
	idx := data.NewIndex([]data.Bean{
		{ID: "sproutling-aaaa", Title: "Self", Status: "todo", Type: "task"},
		{ID: "sproutling-xglu", Title: "Other", Status: "todo", Type: "task"},
	})

	items := buildBlockingItems(idx, "sproutling-aaaa", "sproutling")
	if len(items) == 0 {
		t.Fatalf("buildBlockingItems returned no candidates")
	}
	for _, it := range items {
		got := stripHint(it.prefix)
		if strings.Contains(got, "sproutling-xglu") {
			t.Fatalf("blocking picker row kept the repo prefix: %q", got)
		}
	}
}

// TestPickerItemIDStaysFullForMutation: only the RENDERED prefix is
// shortened. pickerItem.id is the mutation target handed to SetParent/
// AddBlocking -- shortening THAT would send a non-existent ID to the beans
// CLI, which is the one way this cosmetic change could become a real bug.
func TestPickerItemIDStaysFullForMutation(t *testing.T) {
	idx := data.NewIndex([]data.Bean{
		{ID: "sproutling-aaaa", Title: "Self", Status: "todo", Type: "task"},
		{ID: "sproutling-xglu", Title: "Other", Status: "todo", Type: "task"},
	})

	for _, it := range buildBlockingItems(idx, "sproutling-aaaa", "sproutling") {
		if it.id == "xglu" {
			t.Fatalf("pickerItem.id was shortened -- the mutation target must stay the full ID: %+v", it)
		}
	}
}
