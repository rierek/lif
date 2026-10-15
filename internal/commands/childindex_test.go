package commands

import (
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// TestBuildChildrenIndexEmptyTree verifies that an empty input yields an
// empty index, and that looking up any ID on it returns nil (not a panic).
func TestBuildChildrenIndexEmptyTree(t *testing.T) {
	idx := buildChildrenIndex(nil)
	if len(idx) != 0 {
		t.Fatalf("expected empty index, got %d entries", len(idx))
	}
	if got := idx["missing"]; got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}
}

// TestBuildChildrenIndexSingleLevel verifies that direct parent -> children
// relationships are indexed, and items without a parent are not indexed
// under any key.
func TestBuildChildrenIndexSingleLevel(t *testing.T) {
	parent := &item.Item{ID: "beans-p1"}
	child1 := &item.Item{ID: "beans-c1", Parent: "beans-p1"}
	child2 := &item.Item{ID: "beans-c2", Parent: "beans-p1"}
	orphan := &item.Item{ID: "beans-o1"}

	idx := buildChildrenIndex([]*item.Item{parent, child1, child2, orphan})

	children := idx["beans-p1"]
	if len(children) != 2 {
		t.Fatalf("expected 2 children under beans-p1, got %d", len(children))
	}

	if len(idx["beans-o1"]) != 0 {
		t.Fatalf("expected no children indexed under orphan, got %v", idx["beans-o1"])
	}
}

// TestDescendantsMultiLevel verifies that descendants walks transitively
// through multiple generations (milestone -> epic -> task), not just direct
// children.
func TestDescendantsMultiLevel(t *testing.T) {
	milestone := &item.Item{ID: "beans-m1"}
	epic := &item.Item{ID: "beans-e1", Parent: "beans-m1"}
	task1 := &item.Item{ID: "beans-t1", Parent: "beans-e1"}
	task2 := &item.Item{ID: "beans-t2", Parent: "beans-e1"}
	grandchild := &item.Item{ID: "beans-g1", Parent: "beans-t1"}

	idx := buildChildrenIndex([]*item.Item{milestone, epic, task1, task2, grandchild})

	got := descendants("beans-m1", idx)
	if len(got) != 4 {
		t.Fatalf("expected 4 descendants, got %d: %v", len(got), ids(got))
	}

	wantIDs := map[string]bool{"beans-e1": true, "beans-t1": true, "beans-t2": true, "beans-g1": true}
	for _, d := range got {
		if !wantIDs[d.ID] {
			t.Errorf("unexpected descendant %q", d.ID)
		}
		delete(wantIDs, d.ID)
	}
	if len(wantIDs) != 0 {
		t.Errorf("missing expected descendants: %v", wantIDs)
	}

	// milestone itself must not be included in its own descendants.
	for _, d := range got {
		if d.ID == "beans-m1" {
			t.Errorf("descendants must not include the queried bean itself")
		}
	}
}

// TestDescendantsEmptyTree verifies that an ID with no children yields no
// descendants and does not panic.
func TestDescendantsEmptyTree(t *testing.T) {
	idx := buildChildrenIndex(nil)
	got := descendants("beans-nope", idx)
	if len(got) != 0 {
		t.Fatalf("expected no descendants, got %v", ids(got))
	}
}

// TestDescendantProgressCountsCompletedAndScrapped verifies the percent-
// complete convention: scrapped items are excluded from both completed and
// total, matching the plan's Global Constraints formula.
func TestDescendantProgressCountsCompletedAndScrapped(t *testing.T) {
	milestone := &item.Item{ID: "beans-m1"}
	done := &item.Item{ID: "beans-t1", Parent: "beans-m1", Status: "completed"}
	todo := &item.Item{ID: "beans-t2", Parent: "beans-m1", Status: "todo"}
	inProgress := &item.Item{ID: "beans-t3", Parent: "beans-m1", Status: "in-progress"}
	scrapped := &item.Item{ID: "beans-t4", Parent: "beans-m1", Status: "scrapped"}

	idx := buildChildrenIndex([]*item.Item{milestone, done, todo, inProgress, scrapped})

	completed, total := descendantProgress("beans-m1", idx)
	if completed != 1 {
		t.Errorf("completed = %d, want 1", completed)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 (scrapped excluded)", total)
	}
}

// TestDescendantProgressAllScrappedIsZeroZero verifies the 0/0 edge case:
// a subtree that is entirely scrapped reports 0 completed of 0 total,
// rather than a division artifact.
func TestDescendantProgressAllScrappedIsZeroZero(t *testing.T) {
	milestone := &item.Item{ID: "beans-m1"}
	scrapped1 := &item.Item{ID: "beans-t1", Parent: "beans-m1", Status: "scrapped"}
	scrapped2 := &item.Item{ID: "beans-t2", Parent: "beans-m1", Status: "scrapped"}

	idx := buildChildrenIndex([]*item.Item{milestone, scrapped1, scrapped2})

	completed, total := descendantProgress("beans-m1", idx)
	if completed != 0 || total != 0 {
		t.Errorf("descendantProgress() = (%d, %d), want (0, 0)", completed, total)
	}
}

// TestDescendantProgressChildlessIsZeroZero verifies that a milestone with
// no descendants at all also reports 0/0.
func TestDescendantProgressChildlessIsZeroZero(t *testing.T) {
	milestone := &item.Item{ID: "beans-m1"}
	idx := buildChildrenIndex([]*item.Item{milestone})

	completed, total := descendantProgress("beans-m1", idx)
	if completed != 0 || total != 0 {
		t.Errorf("descendantProgress() = (%d, %d), want (0, 0)", completed, total)
	}
}

// ids is a small test helper for readable failure messages.
func ids(items []*item.Item) []string {
	out := make([]string, len(items))
	for i, b := range items {
		out[i] = b.ID
	}
	return out
}
