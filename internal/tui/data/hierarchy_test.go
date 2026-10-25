package data

// hierarchy_test.go — TDD coverage for the Parent-Picker's eligibility rules
// (E3 Task 3, bean bt-p1uz, epic-E3-plan.md »Task 3« Step 1).

import (
	"github.com/rierek/lif/pkg/item"
	"testing"
	"time"
)

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestValidParentTypesMirrorsBeancore(t *testing.T) {
	cases := []struct {
		beanType string
		want     []string
	}{
		{"milestone", nil},
		{"epic", []string{"milestone"}},
		{"feature", []string{"milestone", "epic"}},
		{"task", []string{"milestone", "epic", "feature"}},
		{"bug", []string{"milestone", "epic", "feature"}},
		{"unbekannt", []string{"milestone", "epic", "feature"}},
	}
	for _, tc := range cases {
		if got := validParentTypes(tc.beanType); !equalStringSlices(got, tc.want) {
			t.Errorf("validParentTypes(%q) = %v, want %v", tc.beanType, got, tc.want)
		}
	}
}

// TestCollectDescendantsBFS guards the plain hierarchy case: m -> e -> t1,
// t2.
func TestCollectDescendantsBFS(t *testing.T) {
	beans := []*item.Item{
		{ID: "m", Title: "Milestone", Type: "milestone"},
		{ID: "e", Title: "Epic", Type: "epic", Parent: "m"},
		{ID: "t1", Title: "Task One", Type: "task", Parent: "e"},
		{ID: "t2", Title: "Task Two", Type: "task", Parent: "e"},
	}
	idx := NewIndex(beans)

	got := CollectDescendants(idx, "m")
	want := map[string]bool{"e": true, "t1": true, "t2": true}
	if len(got) != len(want) {
		t.Fatalf("CollectDescendants(idx, \"m\") = %v, want %v", got, want)
	}
	for id := range want {
		if !got[id] {
			t.Errorf("CollectDescendants(idx, \"m\") missing %q, got %v", id, got)
		}
	}

	if got := CollectDescendants(idx, "t1"); len(got) != 0 {
		t.Errorf("CollectDescendants(idx, \"t1\") = %v, want empty (leaf)", got)
	}
}

// TestCollectDescendantsSurvivesParentCycle guards the defensive visited
// guard: a hand-edited Kinder-Zyklus (a's parent is b, b's parent is a) must
// terminate, never hang -- run on a goroutine with a timeout so a
// regression (missing visited guard) fails the test instead of hanging the
// whole suite forever.
func TestCollectDescendantsSurvivesParentCycle(t *testing.T) {
	beans := []*item.Item{
		{ID: "a", Title: "A", Type: "task", Parent: "b"},
		{ID: "b", Title: "B", Type: "task", Parent: "a"},
	}
	idx := NewIndex(beans)

	done := make(chan map[string]bool, 1)
	go func() { done <- CollectDescendants(idx, "a") }()

	select {
	case got := <-done:
		if len(got) == 0 {
			t.Error("CollectDescendants on a parent cycle returned nothing, want it to still terminate with a result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CollectDescendants hung on a parent cycle (missing visited guard)")
	}
}
