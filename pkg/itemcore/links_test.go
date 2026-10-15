package itemcore

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

func TestFindIncomingLinks(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create items with relationships
	// A -> B (blocks)
	// A -> C (parent)
	// D -> B (blocks)
	itemA := &item.Item{
		ID:       "aaa1",
		Title:    "Bean A",
		Status:   "todo",
		Blocking: []string{"bbb2"},
		Parent:   "ccc3",
	}
	itemB := &item.Item{ID: "bbb2", Title: "Bean B", Status: "todo"}
	itemC := &item.Item{ID: "ccc3", Title: "Bean C", Status: "todo"}
	itemD := &item.Item{
		ID:       "ddd4",
		Title:    "Bean D",
		Status:   "todo",
		Blocking: []string{"bbb2"},
	}

	for _, b := range []*item.Item{itemA, itemB, itemC, itemD} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	t.Run("multiple incoming blocks", func(t *testing.T) {
		links := core.FindIncomingLinks("bbb2")
		if len(links) != 2 {
			t.Errorf("FindIncomingLinks(bbb2) = %d links, want 2", len(links))
		}

		// Check both A and D block B
		fromIDs := make(map[string]string)
		for _, link := range links {
			fromIDs[link.FromItem.ID] = link.LinkType
		}
		if fromIDs["aaa1"] != "blocking" {
			t.Error("expected aaa1 -> bbb2 via blocks")
		}
		if fromIDs["ddd4"] != "blocking" {
			t.Error("expected ddd4 -> bbb2 via blocks")
		}
	})

	t.Run("single incoming parent link", func(t *testing.T) {
		links := core.FindIncomingLinks("ccc3")
		if len(links) != 1 {
			t.Errorf("FindIncomingLinks(ccc3) = %d links, want 1", len(links))
		}
		if links[0].FromItem.ID != "aaa1" || links[0].LinkType != "parent" {
			t.Errorf("expected aaa1 -> ccc3 via parent, got %s -> ccc3 via %s", links[0].FromItem.ID, links[0].LinkType)
		}
	})

	t.Run("no incoming links", func(t *testing.T) {
		links := core.FindIncomingLinks("aaa1")
		if len(links) != 0 {
			t.Errorf("FindIncomingLinks(aaa1) = %d links, want 0", len(links))
		}
	})

	t.Run("nonexistent bean", func(t *testing.T) {
		links := core.FindIncomingLinks("nonexistent")
		if len(links) != 0 {
			t.Errorf("FindIncomingLinks(nonexistent) = %d links, want 0", len(links))
		}
	})
}

// createAll stores the items, failing the test when one is refused.
func createAll(t *testing.T, core *Core, items ...*item.Item) {
	t.Helper()
	for _, b := range items {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create(%s) error: %v", b.ID, err)
		}
	}
}

func TestDetectCycle(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create a chain: A blocks B, B blocks C
	itemA := &item.Item{
		ID:       "aaa1",
		Title:    "Bean A",
		Status:   "todo",
		Blocking: []string{"bbb2"},
	}
	itemB := &item.Item{
		ID:       "bbb2",
		Title:    "Bean B",
		Status:   "todo",
		Blocking: []string{"ccc3"},
	}
	itemC := &item.Item{
		ID:     "ccc3",
		Title:  "Bean C",
		Status: "todo",
	}
	createAll(t, core, itemA, itemB, itemC)

	t.Run("would create cycle", func(t *testing.T) {
		// Adding C blocks A would create: A -> B -> C -> A
		cycle := core.DetectCycle(itemC, "blocking", "aaa1")
		if cycle == nil {
			t.Fatal("DetectCycle should return the cycle when one would be created")
		}
		if want := []string{"ccc3", "aaa1", "bbb2", "ccc3"}; !slices.Equal(cycle.Path, want) {
			t.Errorf("cycle path = %v, want %v", cycle.Path, want)
		}
		if cycle.LinkType != "blocking" {
			t.Errorf("cycle link type = %q, want %q", cycle.LinkType, "blocking")
		}
	})

	t.Run("no cycle", func(t *testing.T) {
		// Adding D blocks A would not create a cycle (D doesn't exist in chain)
		itemD := &item.Item{ID: "ddd4", Title: "Bean D", Status: "todo"}
		createAll(t, core, itemD)

		if cycle := core.DetectCycle(itemD, "blocking", "aaa1"); cycle != nil {
			t.Errorf("DetectCycle should return nil when no cycle, got: %v", cycle.Path)
		}
	})

	t.Run("parent cycle detection", func(t *testing.T) {
		// Create parent chain: X -> Y -> Z (X has parent Y, Y has parent Z)
		itemX := &item.Item{
			ID:     "xxx1",
			Title:  "Bean X",
			Status: "todo",
			Parent: "yyy2",
		}
		itemY := &item.Item{
			ID:     "yyy2",
			Title:  "Bean Y",
			Status: "todo",
			Parent: "zzz3",
		}
		itemZ := &item.Item{
			ID:     "zzz3",
			Title:  "Bean Z",
			Status: "todo",
		}
		createAll(t, core, itemX, itemY, itemZ)

		// Adding Z parent of X would create: X -> Y -> Z -> X
		cycle := core.DetectCycle(itemZ, "parent", "xxx1")
		if cycle == nil {
			t.Fatal("DetectCycle should detect parent cycles")
		}
		if cycle.LinkType != "parent" {
			t.Errorf("cycle link type = %q, want %q", cycle.LinkType, "parent")
		}
	})
}

// beans-et2a: a blocking link is stored on one of the two items it connects,
// as `blocking` on the blocker or as `blocked_by` on the blocked item, but it
// is one edge. A cycle is a cycle whatever mix of the two fields carries it.
func TestDetectCycleAcrossBlockingFields(t *testing.T) {
	core, _ := setupTestCore(t)

	// A blocks B (stored on A), B blocks C (stored on C).
	a := &item.Item{ID: "mx-a", Title: "A", Status: "todo", Blocking: []string{"mx-b"}}
	b := &item.Item{ID: "mx-b", Title: "B", Status: "todo"}
	c := &item.Item{ID: "mx-c", Title: "C", Status: "todo", BlockedBy: []string{"mx-b"}}
	createAll(t, core, a, b, c)

	t.Run("a blocking link closes the ring", func(t *testing.T) {
		cycle := core.DetectCycle(c, "blocking", "mx-a") // C blocks A
		if cycle == nil {
			t.Fatal("C blocks A closes A -> B -> C -> A, but DetectCycle found none")
		}
		if want := []string{"mx-c", "mx-a", "mx-b", "mx-c"}; !slices.Equal(cycle.Path, want) {
			t.Errorf("cycle path = %v, want %v", cycle.Path, want)
		}
	})

	t.Run("a blocked_by link closes the same ring", func(t *testing.T) {
		cycle := core.DetectCycle(a, "blocked_by", "mx-c") // A is blocked by C: the same link
		if cycle == nil {
			t.Fatal("A blocked by C closes A -> B -> C -> A, but DetectCycle found none")
		}
		if want := []string{"mx-a", "mx-b", "mx-c", "mx-a"}; !slices.Equal(cycle.Path, want) {
			t.Errorf("cycle path = %v, want %v", cycle.Path, want)
		}
	})

	t.Run("a new item closes the ring with two links of its own", func(t *testing.T) {
		// The item has no ID yet. It blocks A and waits for C, and A -> B -> C
		// already leads from the first to the second.
		fresh := &item.Item{Blocking: []string{"mx-a"}, BlockedBy: []string{"mx-c"}}
		cycle := core.DetectCycle(fresh, "blocking", "mx-a")
		if cycle == nil {
			t.Fatal("a new item that blocks A and is blocked by C closes a ring, but DetectCycle found none")
		}
		if want := []string{newItemID, "mx-a", "mx-b", "mx-c", newItemID}; !slices.Equal(cycle.Path, want) {
			t.Errorf("cycle path = %v, want %v", cycle.Path, want)
		}
	})

	t.Run("a link the edit removes does not count", func(t *testing.T) {
		// Turning "A blocks B" into "A is blocked by B" in one edit is no cycle.
		flipped := a.Clone()
		flipped.Blocking = nil
		if cycle := core.DetectCycle(flipped, "blocked_by", "mx-b"); cycle != nil {
			t.Errorf("the flipped link is no cycle, got %v", cycle.Path)
		}
		// Control: with the stored link still on A the same request is one.
		if cycle := core.DetectCycle(a, "blocked_by", "mx-b"); cycle == nil {
			t.Error("A blocks B is stored, so B blocks A must be a cycle")
		}
	})

	t.Run("a cycle elsewhere in the store does not refuse an unrelated link", func(t *testing.T) {
		p1 := &item.Item{ID: "ex-p1", Title: "P1", Status: "todo", Blocking: []string{"ex-p2"}}
		p2 := &item.Item{ID: "ex-p2", Title: "P2", Status: "todo", Blocking: []string{"ex-p1"}}
		q := &item.Item{ID: "ex-q", Title: "Q", Status: "todo"}
		createAll(t, core, p1, p2, q)
		if cycle := core.DetectCycle(q, "blocking", "ex-p1"); cycle != nil {
			t.Errorf("Q blocks P1 is no new cycle, got %v", cycle.Path)
		}
	})
}

func TestCheckAllLinks(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create items with various link issues:
	// - Broken parent link to nonexistent item
	// - Self-reference in blocks
	// - Cycle (A -> B -> A via blocks)
	itemA := &item.Item{
		ID:       "aaa1",
		Title:    "Bean A",
		Status:   "todo",
		Blocking: []string{"bbb2", "aaa1"}, // aaa1 is self-reference
		Parent:   "nonexistent",
	}
	itemB := &item.Item{
		ID:       "bbb2",
		Title:    "Bean B",
		Status:   "todo",
		Blocking: []string{"aaa1"}, // creates cycle
	}

	for _, b := range []*item.Item{itemA, itemB} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	result := core.CheckAllLinks()

	t.Run("detects broken links", func(t *testing.T) {
		if len(result.BrokenLinks) != 1 {
			t.Errorf("BrokenLinks = %d, want 1", len(result.BrokenLinks))
		}
		if len(result.BrokenLinks) > 0 {
			bl := result.BrokenLinks[0]
			if bl.ItemID != "aaa1" || bl.LinkType != "parent" || bl.Target != "nonexistent" {
				t.Errorf("unexpected broken link: %+v", bl)
			}
		}
	})

	t.Run("detects self-references", func(t *testing.T) {
		if len(result.SelfLinks) != 1 {
			t.Errorf("SelfLinks = %d, want 1", len(result.SelfLinks))
		}
		if len(result.SelfLinks) > 0 {
			sl := result.SelfLinks[0]
			if sl.ItemID != "aaa1" || sl.LinkType != "blocking" {
				t.Errorf("unexpected self-link: %+v", sl)
			}
		}
	})

	t.Run("detects cycles", func(t *testing.T) {
		if len(result.Cycles) != 1 {
			t.Errorf("Cycles = %d, want 1", len(result.Cycles))
		}
		if len(result.Cycles) > 0 {
			c := result.Cycles[0]
			if c.LinkType != "blocking" {
				t.Errorf("cycle link type = %q, want 'blocks'", c.LinkType)
			}
			if len(c.Path) < 3 {
				t.Errorf("cycle path too short: %v", c.Path)
			}
		}
	})

	t.Run("HasIssues returns true", func(t *testing.T) {
		if !result.HasIssues() {
			t.Error("HasIssues() should return true")
		}
	})

	t.Run("TotalIssues counts all", func(t *testing.T) {
		if result.TotalIssues() != 3 {
			t.Errorf("TotalIssues() = %d, want 3", result.TotalIssues())
		}
	})
}

func TestCheckAllLinksClean(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create clean items with no issues
	itemA := &item.Item{
		ID:       "aaa1",
		Title:    "Bean A",
		Status:   "todo",
		Blocking: []string{"bbb2"},
	}
	itemB := &item.Item{
		ID:     "bbb2",
		Title:  "Bean B",
		Status: "todo",
	}

	for _, b := range []*item.Item{itemA, itemB} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	result := core.CheckAllLinks()

	if result.HasIssues() {
		t.Errorf("HasIssues() should return false for clean beans, got broken=%d self=%d cycles=%d",
			len(result.BrokenLinks), len(result.SelfLinks), len(result.Cycles))
	}
}

// beans-et2a: lif check must see a ring whose links are stored in both fields.
// beans-0ish: and print it the same way on every run, so the first ID of the
// path is the smallest one rather than the one a map walk happened to enter at.
func TestCheckAllLinksReportsRingsInAStableOrder(t *testing.T) {
	rings := map[string][]*item.Item{
		// A blocks B (on A), B blocks C (on C), C blocks A (on C).
		"mixed fields": {
			{ID: "mx-a", Title: "A", Status: "todo", Blocking: []string{"mx-b"}},
			{ID: "mx-b", Title: "B", Status: "todo"},
			{ID: "mx-c", Title: "C", Status: "todo", BlockedBy: []string{"mx-b"}, Blocking: []string{"mx-a"}},
		},
		// The same ring stored only as blocked_by.
		"blocked_by only": {
			{ID: "mx-a", Title: "A", Status: "todo", BlockedBy: []string{"mx-c"}},
			{ID: "mx-b", Title: "B", Status: "todo", BlockedBy: []string{"mx-a"}},
			{ID: "mx-c", Title: "C", Status: "todo", BlockedBy: []string{"mx-b"}},
		},
		// The same ring stored only as blocking.
		"blocking only": {
			{ID: "mx-a", Title: "A", Status: "todo", Blocking: []string{"mx-b"}},
			{ID: "mx-b", Title: "B", Status: "todo", Blocking: []string{"mx-c"}},
			{ID: "mx-c", Title: "C", Status: "todo", Blocking: []string{"mx-a"}},
		},
	}
	want := []Cycle{{LinkType: "blocking", Path: []string{"mx-a", "mx-b", "mx-c", "mx-a"}}}

	for name, items := range rings {
		t.Run(name, func(t *testing.T) {
			core, _ := setupTestCore(t)
			createAll(t, core, items...)

			// A map walk picks a different entry item from run to run, so
			// one comparison is not enough to tell a stable path from luck.
			for i := range 50 {
				got := core.CheckAllLinks().Cycles
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("run %d: Cycles = %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

// Several cycles come out grouped by link type and, inside a group, by path.
func TestCheckAllLinksListsCyclesInOrder(t *testing.T) {
	core, _ := setupTestCore(t)
	createAll(t, core,
		// A parent loop, written by hand.
		&item.Item{ID: "pa-1", Title: "P1", Status: "todo", Parent: "pa-2"},
		&item.Item{ID: "pa-2", Title: "P2", Status: "todo", Parent: "pa-1"},
		// Two blocking rings; the later ID is created first.
		&item.Item{ID: "zz-1", Title: "Z1", Status: "todo", Blocking: []string{"zz-2"}},
		&item.Item{ID: "zz-2", Title: "Z2", Status: "todo", BlockedBy: []string{"zz-1"}, Blocking: []string{"zz-1"}},
		&item.Item{ID: "bb-1", Title: "B1", Status: "todo", Blocking: []string{"bb-2"}},
		&item.Item{ID: "bb-2", Title: "B2", Status: "todo", Blocking: []string{"bb-1"}},
	)

	want := []Cycle{
		{LinkType: "blocking", Path: []string{"bb-1", "bb-2", "bb-1"}},
		{LinkType: "blocking", Path: []string{"zz-1", "zz-2", "zz-1"}},
		{LinkType: "parent", Path: []string{"pa-1", "pa-2", "pa-1"}},
	}
	for i := range 20 {
		if got := core.CheckAllLinks().Cycles; !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: Cycles = %+v, want %+v", i, got, want)
		}
	}
}

// beans-o8bs: a blocker below the blocked item, or above it, can never be
// released, and lif check reports it as a cycle over blocking and parent links
// that carries a note.
func TestCheckAllLinksReportsBlockersAboveOrBelowTheBlockedItem(t *testing.T) {
	cases := map[string][]*item.Item{
		"a parent blocked by its child": {
			{ID: "ep-1", Title: "P", Status: "todo", Type: "epic", BlockedBy: []string{"tk-1"}},
			{ID: "tk-1", Title: "D", Status: "todo", Type: "task", Parent: "ep-1"},
		},
		"a child blocking its parent": {
			{ID: "ep-1", Title: "P", Status: "todo", Type: "epic"},
			{ID: "tk-1", Title: "D", Status: "todo", Type: "task", Parent: "ep-1", Blocking: []string{"ep-1"}},
		},
		"a child blocked by its parent": {
			{ID: "ep-1", Title: "P", Status: "todo", Type: "epic"},
			{ID: "tk-1", Title: "D", Status: "todo", Type: "task", Parent: "ep-1", BlockedBy: []string{"ep-1"}},
		},
		"a parent blocking its child": {
			{ID: "ep-1", Title: "P", Status: "todo", Type: "epic", Blocking: []string{"tk-1"}},
			{ID: "tk-1", Title: "D", Status: "todo", Type: "task", Parent: "ep-1"},
		},
	}
	want := []Cycle{{LinkType: "blocking+parent", Path: []string{"ep-1", "tk-1", "ep-1"}}}

	for name, items := range cases {
		t.Run(name, func(t *testing.T) {
			core, _ := setupTestCore(t)
			createAll(t, core, items...)
			for i := range 20 {
				if got := core.CheckAllLinks().Cycles; !reflect.DeepEqual(got, want) {
					t.Fatalf("run %d: Cycles = %+v, want %+v", i, got, want)
				}
			}
		})
	}

	t.Run("a parent and its child that do not block each other are clean", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createAll(t, core,
			&item.Item{ID: "ep-1", Title: "P", Status: "todo", Type: "epic", BlockedBy: []string{"tk-9"}},
			&item.Item{ID: "tk-1", Title: "D", Status: "todo", Type: "task", Parent: "ep-1", BlockedBy: []string{"tk-2"}},
			&item.Item{ID: "tk-2", Title: "S", Status: "todo", Type: "task", Parent: "ep-1"},
			&item.Item{ID: "tk-9", Title: "X", Status: "todo", Type: "task"},
		)
		if got := core.CheckAllLinks().Cycles; len(got) != 0 {
			t.Errorf("Cycles = %+v, want none", got)
		}
	})
}

func TestRemoveLinksTo(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create items where multiple items link to one target
	itemA := &item.Item{
		ID:       "aaa1",
		Title:    "Bean A",
		Status:   "todo",
		Blocking: []string{"target"},
		Parent:   "target",
	}
	itemB := &item.Item{
		ID:       "bbb2",
		Title:    "Bean B",
		Status:   "todo",
		Blocking: []string{"target"},
	}
	target := &item.Item{
		ID:     "target",
		Title:  "Target Bean",
		Status: "todo",
	}

	for _, b := range []*item.Item{itemA, itemB, target} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	// Remove all links to target
	removed, err := core.RemoveLinksTo("target")
	if err != nil {
		t.Fatalf("RemoveLinksTo error: %v", err)
	}

	if removed != 3 {
		t.Errorf("removed = %d, want 3", removed)
	}

	// Verify links are gone
	loadedA, _ := core.Get("aaa1")
	if loadedA.Parent != "" || len(loadedA.Blocking) != 0 {
		t.Errorf("Bean A still has relationships: parent=%q blocks=%v", loadedA.Parent, loadedA.Blocking)
	}

	loadedB, _ := core.Get("bbb2")
	if len(loadedB.Blocking) != 0 {
		t.Errorf("Bean B still has %d blocks, want 0", len(loadedB.Blocking))
	}
}

func TestFixBrokenLinks(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create item with broken link and self-reference
	itemA := &item.Item{
		ID:       "aaa1",
		Title:    "Bean A",
		Status:   "todo",
		Blocking: []string{"bbb2", "aaa1"}, // bbb2 is valid, aaa1 is self-reference
		Parent:   "nonexistent",            // broken
	}
	itemB := &item.Item{
		ID:     "bbb2",
		Title:  "Bean B",
		Status: "todo",
	}

	for _, b := range []*item.Item{itemA, itemB} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	// Fix broken links
	fixed, err := core.FixBrokenLinks()
	if err != nil {
		t.Fatalf("FixBrokenLinks error: %v", err)
	}

	if fixed != 2 {
		t.Errorf("fixed = %d, want 2", fixed)
	}

	// Verify only valid link remains
	loadedA, _ := core.Get("aaa1")
	if len(loadedA.Blocking) != 1 {
		t.Errorf("Bean A has %d blocks, want 1", len(loadedA.Blocking))
	}
	if !loadedA.IsBlocking("bbb2") {
		t.Error("valid 'blocks' link should be preserved")
	}
	if loadedA.Parent != "" {
		t.Errorf("broken parent should be removed, got %q", loadedA.Parent)
	}
}

func TestLinkCheckResultMethods(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		r := &LinkCheckResult{
			BrokenLinks: []BrokenLink{},
			SelfLinks:   []SelfLink{},
			Cycles:      []Cycle{},
		}
		if r.HasIssues() {
			t.Error("empty result should not have issues")
		}
		if r.TotalIssues() != 0 {
			t.Errorf("TotalIssues() = %d, want 0", r.TotalIssues())
		}
	})

	t.Run("with issues", func(t *testing.T) {
		r := &LinkCheckResult{
			BrokenLinks: []BrokenLink{{ItemID: "a", LinkType: "blocking", Target: "x"}},
			SelfLinks:   []SelfLink{{ItemID: "b", LinkType: "parent"}},
			Cycles:      []Cycle{{LinkType: "blocking", Path: []string{"a", "b", "a"}}},
		}
		if !r.HasIssues() {
			t.Error("result with issues should have issues")
		}
		if r.TotalIssues() != 3 {
			t.Errorf("TotalIssues() = %d, want 3", r.TotalIssues())
		}
	})
}

func TestIsBlocked(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create test items with various blocking scenarios
	activeBlocker := &item.Item{
		ID:       "active-blocker",
		Title:    "Active Blocker",
		Status:   "todo",
		Blocking: []string{"blocked-by-active"},
	}
	completedBlocker := &item.Item{
		ID:       "completed-blocker",
		Title:    "Completed Blocker",
		Status:   "completed",
		Blocking: []string{"blocked-by-completed"},
	}
	scrappedBlocker := &item.Item{
		ID:       "scrapped-blocker",
		Title:    "Scrapped Blocker",
		Status:   "scrapped",
		Blocking: []string{"blocked-by-scrapped"},
	}
	blockedByActive := &item.Item{
		ID:     "blocked-by-active",
		Title:  "Blocked by Active",
		Status: "todo",
	}
	blockedByCompleted := &item.Item{
		ID:     "blocked-by-completed",
		Title:  "Blocked by Completed",
		Status: "todo",
	}
	blockedByScrapped := &item.Item{
		ID:     "blocked-by-scrapped",
		Title:  "Blocked by Scrapped",
		Status: "todo",
	}
	notBlocked := &item.Item{
		ID:     "not-blocked",
		Title:  "Not Blocked",
		Status: "todo",
	}
	// Item with direct blocked_by field
	blockedByFieldActive := &item.Item{
		ID:        "blocked-by-field-active",
		Title:     "Blocked by Field (Active)",
		Status:    "todo",
		BlockedBy: []string{"active-blocker"},
	}
	blockedByFieldCompleted := &item.Item{
		ID:        "blocked-by-field-completed",
		Title:     "Blocked by Field (Completed)",
		Status:    "todo",
		BlockedBy: []string{"completed-blocker"},
	}
	// Item with broken blocker link
	blockedByBroken := &item.Item{
		ID:        "blocked-by-broken",
		Title:     "Blocked by Broken Link",
		Status:    "todo",
		BlockedBy: []string{"nonexistent"},
	}
	// Item with multiple blockers (one active, one completed)
	mixedBlockers := &item.Item{
		ID:        "mixed-blockers",
		Title:     "Mixed Blockers",
		Status:    "todo",
		BlockedBy: []string{"active-blocker", "completed-blocker"},
	}
	// Item with multiple blockers (all completed)
	allResolvedBlockers := &item.Item{
		ID:        "all-resolved-blockers",
		Title:     "All Resolved Blockers",
		Status:    "todo",
		BlockedBy: []string{"completed-blocker", "scrapped-blocker"},
	}

	items := []*item.Item{
		activeBlocker, completedBlocker, scrappedBlocker,
		blockedByActive, blockedByCompleted, blockedByScrapped,
		notBlocked, blockedByFieldActive, blockedByFieldCompleted,
		blockedByBroken, mixedBlockers, allResolvedBlockers,
	}
	for _, b := range items {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	tests := []struct {
		name   string
		itemID string
		want   bool
	}{
		{"blocked by active via Blocking field", "blocked-by-active", true},
		{"blocked by completed via Blocking field", "blocked-by-completed", false},
		{"blocked by scrapped via Blocking field", "blocked-by-scrapped", false},
		{"not blocked", "not-blocked", false},
		{"blocked by active via BlockedBy field", "blocked-by-field-active", true},
		{"blocked by completed via BlockedBy field", "blocked-by-field-completed", false},
		{"broken blocker link", "blocked-by-broken", false},
		{"mixed blockers (one active)", "mixed-blockers", true},
		{"all resolved blockers", "all-resolved-blockers", false},
		{"nonexistent bean", "nonexistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := core.IsBlocked(tt.itemID)
			if got != tt.want {
				t.Errorf("IsBlocked(%q) = %v, want %v", tt.itemID, got, tt.want)
			}
		})
	}
}

func TestFindActiveBlockers(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create test items
	activeBlocker1 := &item.Item{
		ID:       "active-blocker-1",
		Title:    "Active Blocker 1",
		Status:   "in-progress",
		Blocking: []string{"target"},
	}
	activeBlocker2 := &item.Item{
		ID:     "active-blocker-2",
		Title:  "Active Blocker 2",
		Status: "todo",
	}
	completedBlocker := &item.Item{
		ID:       "completed-blocker",
		Title:    "Completed Blocker",
		Status:   "completed",
		Blocking: []string{"target"},
	}
	target := &item.Item{
		ID:        "target",
		Title:     "Target Bean",
		Status:    "todo",
		BlockedBy: []string{"active-blocker-2", "completed-blocker"},
	}
	noBlockers := &item.Item{
		ID:     "no-blockers",
		Title:  "No Blockers",
		Status: "todo",
	}

	items := []*item.Item{activeBlocker1, activeBlocker2, completedBlocker, target, noBlockers}
	for _, b := range items {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	t.Run("returns active blockers from both sources", func(t *testing.T) {
		blockers := core.FindActiveBlockers("target")
		if len(blockers) != 2 {
			t.Errorf("FindActiveBlockers() returned %d blockers, want 2", len(blockers))
		}
		// Check that both active blockers are present
		ids := make(map[string]bool)
		for _, b := range blockers {
			ids[b.ID] = true
		}
		if !ids["active-blocker-1"] {
			t.Error("expected active-blocker-1 in result")
		}
		if !ids["active-blocker-2"] {
			t.Error("expected active-blocker-2 in result")
		}
		if ids["completed-blocker"] {
			t.Error("completed-blocker should not be in result")
		}
	})

	t.Run("returns nil for bean with no blockers", func(t *testing.T) {
		blockers := core.FindActiveBlockers("no-blockers")
		if len(blockers) != 0 {
			t.Errorf("FindActiveBlockers() returned %d blockers, want 0", len(blockers))
		}
	})

	t.Run("returns nil for nonexistent bean", func(t *testing.T) {
		blockers := core.FindActiveBlockers("nonexistent")
		if blockers != nil {
			t.Errorf("FindActiveBlockers() returned %v, want nil", blockers)
		}
	})
}

func TestImplicitStatus(t *testing.T) {
	core, _ := setupTestCore(t)

	// Hierarchy: milestone (scrapped) -> epic (todo) -> task (in-progress)
	milestone := &item.Item{ID: "m001", Title: "Milestone", Status: "scrapped", Type: "milestone"}
	epic := &item.Item{ID: "e001", Title: "Epic", Status: "todo", Type: "epic", Parent: "m001"}
	task := &item.Item{ID: "t001", Title: "Task", Status: "in-progress", Type: "task", Parent: "e001"}
	orphan := &item.Item{ID: "x001", Title: "Orphan", Status: "todo", Type: "task"}

	// completed parent hierarchy: milestone2 (completed) -> task2
	milestone2 := &item.Item{ID: "m002", Title: "Milestone2", Status: "completed", Type: "milestone"}
	task2 := &item.Item{ID: "t002", Title: "Task2", Status: "todo", Type: "task", Parent: "m002"}

	for _, b := range []*item.Item{milestone, epic, task, orphan, milestone2, task2} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	t.Run("direct scrapped parent", func(t *testing.T) {
		status, fromID := core.ImplicitStatus("e001")
		if status != "scrapped" {
			t.Errorf("want status=scrapped, got %q", status)
		}
		if fromID != "m001" {
			t.Errorf("want fromID=m001, got %q", fromID)
		}
	})

	t.Run("transitive scrapped grandparent", func(t *testing.T) {
		status, fromID := core.ImplicitStatus("t001")
		if status != "scrapped" {
			t.Errorf("want status=scrapped, got %q", status)
		}
		if fromID != "m001" {
			t.Errorf("want fromID=m001, got %q", fromID)
		}
	})

	t.Run("no terminal ancestor", func(t *testing.T) {
		status, fromID := core.ImplicitStatus("x001")
		if status != "" || fromID != "" {
			t.Errorf("want empty, got status=%q fromID=%q", status, fromID)
		}
	})

	t.Run("no parent at all (milestone itself)", func(t *testing.T) {
		status, fromID := core.ImplicitStatus("m001")
		if status != "" || fromID != "" {
			t.Errorf("want empty, got status=%q fromID=%q", status, fromID)
		}
	})

	t.Run("completed parent", func(t *testing.T) {
		status, fromID := core.ImplicitStatus("t002")
		if status != "completed" {
			t.Errorf("want status=completed, got %q", status)
		}
		if fromID != "m002" {
			t.Errorf("want fromID=m002, got %q", fromID)
		}
	})

	t.Run("nonexistent bean", func(t *testing.T) {
		status, fromID := core.ImplicitStatus("nope")
		if status != "" || fromID != "" {
			t.Errorf("want empty, got status=%q fromID=%q", status, fromID)
		}
	})
}

func TestImplicitBlocking(t *testing.T) {
	t.Run("IsBlocked includes implicit blocking", func(t *testing.T) {
		core, _ := setupTestCore(t)

		blocker := &item.Item{ID: "blocker", Title: "Blocker", Status: "todo", Type: "epic", Blocking: []string{"epic-b"}}
		epicB := &item.Item{ID: "epic-b", Title: "Epic B", Status: "todo", Type: "epic"}
		child := &item.Item{ID: "child", Title: "Child", Status: "todo", Type: "feature", Parent: "epic-b"}

		for _, b := range []*item.Item{blocker, epicB, child} {
			if err := core.Create(b); err != nil {
				t.Fatalf("Create error: %v", err)
			}
		}

		// child is not explicitly blocked, but parent epic-b is
		if !core.IsBlocked("child") {
			t.Error("child should be blocked (parent epic-b is blocked by blocker)")
		}
		if !core.IsBlocked("epic-b") {
			t.Error("epic-b should be blocked (directly by blocker)")
		}
	})

	t.Run("IsExplicitlyBlocked only checks direct blockers", func(t *testing.T) {
		core, _ := setupTestCore(t)

		blocker := &item.Item{ID: "blocker", Title: "Blocker", Status: "todo", Type: "epic", Blocking: []string{"epic-b"}}
		epicB := &item.Item{ID: "epic-b", Title: "Epic B", Status: "todo", Type: "epic"}
		child := &item.Item{ID: "child", Title: "Child", Status: "todo", Type: "feature", Parent: "epic-b"}

		for _, b := range []*item.Item{blocker, epicB, child} {
			if err := core.Create(b); err != nil {
				t.Fatalf("Create error: %v", err)
			}
		}

		if core.IsExplicitlyBlocked("child") {
			t.Error("child should NOT be explicitly blocked (no direct blockers)")
		}
		if !core.IsExplicitlyBlocked("epic-b") {
			t.Error("epic-b should be explicitly blocked")
		}
	})

	t.Run("IsImplicitlyBlocked only checks ancestor blockers", func(t *testing.T) {
		core, _ := setupTestCore(t)

		blocker := &item.Item{ID: "blocker", Title: "Blocker", Status: "todo", Type: "epic", Blocking: []string{"epic-b"}}
		epicB := &item.Item{ID: "epic-b", Title: "Epic B", Status: "todo", Type: "epic"}
		child := &item.Item{ID: "child", Title: "Child", Status: "todo", Type: "feature", Parent: "epic-b"}

		for _, b := range []*item.Item{blocker, epicB, child} {
			if err := core.Create(b); err != nil {
				t.Fatalf("Create error: %v", err)
			}
		}

		if !core.IsImplicitlyBlocked("child") {
			t.Error("child should be implicitly blocked (parent epic-b is blocked)")
		}
		if core.IsImplicitlyBlocked("epic-b") {
			t.Error("epic-b should NOT be implicitly blocked (it's directly blocked, no blocked ancestor)")
		}
	})

	t.Run("grandchild is implicitly blocked", func(t *testing.T) {
		core, _ := setupTestCore(t)

		blocker := &item.Item{ID: "blocker", Title: "Blocker", Status: "todo", Type: "milestone", Blocking: []string{"ms"}}
		ms := &item.Item{ID: "ms", Title: "Milestone", Status: "todo", Type: "milestone"}
		epic := &item.Item{ID: "epic", Title: "Epic", Status: "todo", Type: "epic", Parent: "ms"}
		task := &item.Item{ID: "task", Title: "Task", Status: "todo", Type: "task", Parent: "epic"}

		for _, b := range []*item.Item{blocker, ms, epic, task} {
			if err := core.Create(b); err != nil {
				t.Fatalf("Create error: %v", err)
			}
		}

		if !core.IsBlocked("task") {
			t.Error("task should be blocked (grandparent ms is blocked)")
		}
		if !core.IsImplicitlyBlocked("task") {
			t.Error("task should be implicitly blocked")
		}
	})

	t.Run("resolved blocker means not blocked", func(t *testing.T) {
		core, _ := setupTestCore(t)

		blocker := &item.Item{ID: "blocker", Title: "Blocker", Status: "completed", Type: "epic", Blocking: []string{"epic-b"}}
		epicB := &item.Item{ID: "epic-b", Title: "Epic B", Status: "todo", Type: "epic"}
		child := &item.Item{ID: "child", Title: "Child", Status: "todo", Type: "feature", Parent: "epic-b"}

		for _, b := range []*item.Item{blocker, epicB, child} {
			if err := core.Create(b); err != nil {
				t.Fatalf("Create error: %v", err)
			}
		}

		if core.IsBlocked("child") {
			t.Error("child should NOT be blocked (blocker is completed)")
		}
	})

	t.Run("bean with no parent is not implicitly blocked", func(t *testing.T) {
		core, _ := setupTestCore(t)

		task := &item.Item{ID: "task", Title: "Task", Status: "todo", Type: "task"}
		if err := core.Create(task); err != nil {
			t.Fatalf("Create error: %v", err)
		}

		if core.IsImplicitlyBlocked("task") {
			t.Error("task with no parent should not be implicitly blocked")
		}
	})

	t.Run("FindAllBlockers returns both direct and ancestor blockers", func(t *testing.T) {
		core, _ := setupTestCore(t)

		ancestorBlocker := &item.Item{ID: "anc-blocker", Title: "Ancestor Blocker", Status: "todo", Type: "milestone", Blocking: []string{"ms"}}
		ms := &item.Item{ID: "ms", Title: "Milestone", Status: "todo", Type: "milestone"}
		directBlocker := &item.Item{ID: "dir-blocker", Title: "Direct Blocker", Status: "todo", Type: "task", Blocking: []string{"task"}}
		task := &item.Item{ID: "task", Title: "Task", Status: "todo", Type: "task", Parent: "ms"}

		for _, b := range []*item.Item{ancestorBlocker, ms, directBlocker, task} {
			if err := core.Create(b); err != nil {
				t.Fatalf("Create error: %v", err)
			}
		}

		allBlockers := core.FindAllBlockers("task")
		if len(allBlockers) != 2 {
			t.Errorf("want 2 blockers (direct + ancestor), got %d", len(allBlockers))
		}

		directBlockers := core.FindActiveBlockers("task")
		if len(directBlockers) != 1 {
			t.Errorf("want 1 direct blocker, got %d", len(directBlockers))
		}
	})
}

func TestIsResolvedStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"completed", true},
		{"scrapped", true},
		{"todo", false},
		{"in-progress", false},
		{"draft", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := isResolvedStatus(tt.status)
			if got != tt.want {
				t.Errorf("isResolvedStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestValidParentTypesReproducesTheBuiltInTable(t *testing.T) {
	cfg := &config.Config{}
	cases := map[string][]string{
		"milestone": nil,
		"epic":      {"milestone"},
		"feature":   {"milestone", "epic"},
		"task":      {"milestone", "epic", "feature"},
		"bug":       {"milestone", "epic", "feature"},
	}
	for child, want := range cases {
		got := ValidParentTypes(cfg, child)
		if len(got) != len(want) {
			t.Fatalf("ValidParentTypes(%q) = %v, want %v", child, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("ValidParentTypes(%q)[%d] = %q, want %q", child, i, got[i], want[i])
			}
		}
	}
}

func TestUnknownTypeBehavesLikeALeaf(t *testing.T) {
	cfg := &config.Config{}
	got := ValidParentTypes(cfg, "chore")
	want := []string{"milestone", "epic", "feature"}
	if len(got) != len(want) {
		t.Fatalf("ValidParentTypes(\"chore\") = %v, want %v", got, want)
	}
}

func TestSameRankIsNotAParentRelation(t *testing.T) {
	cfg := &config.Config{}
	if IsValidParentRank(cfg, "feature", "feature") {
		t.Error("feature under feature must stay rejected")
	}
	if IsValidParentRank(cfg, "epic", "epic") {
		t.Error("epic under epic must stay rejected")
	}
}

func TestRankJumpStaysAllowed(t *testing.T) {
	cfg := &config.Config{}
	if !IsValidParentRank(cfg, "task", "milestone") {
		t.Error("task directly under a milestone must stay allowed")
	}
}

func TestConfiguredSiblingRankForbidsNesting(t *testing.T) {
	rank := 2
	cfg := &config.Config{Types: []config.TypeOverride{{Name: "feature", Rank: &rank}}}
	if IsValidParentRank(cfg, "feature", "epic") {
		t.Error("with feature on epic's rank, nesting must be rejected")
	}
	if !IsValidParentRank(cfg, "feature", "milestone") {
		t.Error("feature under milestone must stay allowed")
	}
}

// BrokenLink and SelfLink name the offending item with the item vocabulary.
func TestLinkIssueJSONKeysUseItemID(t *testing.T) {
	core, _ := setupTestCore(t)
	for _, b := range []*item.Item{
		{ID: "aaa1", Title: "A", Status: "todo", Parent: "nonexistent"},
		{ID: "bbb2", Title: "B", Status: "todo", Blocking: []string{"bbb2"}},
	} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	data, err := json.Marshal(core.CheckAllLinks())
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var doc map[string][]map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Unmarshal error: %v; data = %s", err, data)
	}
	for list, want := range map[string]string{"broken_links": "aaa1", "self_links": "bbb2"} {
		if len(doc[list]) != 1 {
			t.Fatalf("%s = %v, want one entry; data = %s", list, doc[list], data)
		}
		entry := doc[list][0]
		if entry["item_id"] != want {
			t.Errorf("%s[0].item_id = %v, want %q", list, entry["item_id"], want)
		}
		if _, ok := entry["bean_id"]; ok {
			t.Errorf("%s[0] still carries legacy key bean_id", list)
		}
	}
}
