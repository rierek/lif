package tui

// roadmap_tree_test.go — the Browse tree is the roadmap (roadmap.Build) and
// the Ready view is `lif list --ready` (Store.Ready).

import (
	"context"
	"sort"
	"testing"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
	itemmodel "github.com/rierek/lif/pkg/itemgraph/model"
)

// roadmapFixture: milestone M1 > epic E1 > tasks T1, T2; epic E2 without a
// milestone holding task T3; milestone M2 without any child.
func roadmapFixture() []*item.Item {
	return []*item.Item{
		{ID: "m1", Title: "M1", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "e1", Title: "E1", Status: "todo", Type: "epic", Priority: "normal", Parent: "m1"},
		{ID: "t1", Title: "T1", Status: "todo", Type: "task", Priority: "normal", Parent: "e1"},
		{ID: "t2", Title: "T2", Status: "draft", Type: "task", Priority: "normal", Parent: "e1"},
		{ID: "e2", Title: "E2", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "t3", Title: "T3", Status: "todo", Type: "task", Priority: "normal", Parent: "e2"},
		{ID: "m2", Title: "M2", Status: "todo", Type: "milestone", Priority: "normal"},
	}
}

// realModel is a model as a real session has it right after the first load:
// no fixtureModel shortcuts, nothing expanded.
func realModel(t *testing.T, beans []*item.Item) model {
	t.Helper()
	return step(t, newModel(nil, "/tmp/bt-fixture-repo"), beansLoadedMsg{beans: beans})
}

func TestRoadmapTreeStartsCollapsedAtMilestones(t *testing.T) {
	m := realModel(t, roadmapFixture())

	if len(m.expanded) != 0 {
		t.Fatalf("expanded = %v, want empty at start", m.expanded)
	}
	nodes := m.visibleNodes()
	want := []string{"m1", unscheduledRootID, notOnRoadmapRootID}
	if got := nodeIDs(nodes); !equalStrings(got, want) {
		t.Fatalf("visibleNodes() = %q, want %q (milestone + synthetic roots, all collapsed)", got, want)
	}
	for _, n := range nodes {
		if n.open || n.depth != 0 {
			t.Errorf("row %q open=%v depth=%d, want collapsed at depth 0", n.id, n.open, n.depth)
		}
	}
	if m.cursorID != "m1" {
		t.Errorf("cursorID = %q, want the first row m1", m.cursorID)
	}
}

func TestItemWithoutRoadmapPlaceIsUnderNotOnRoadmap(t *testing.T) {
	m := realModel(t, roadmapFixture())
	m.expanded = map[string]bool{notOnRoadmapRootID: true}

	var depth = -1
	for _, n := range m.visibleNodes() {
		if n.id == "m2" {
			depth = n.depth
		}
	}
	if depth != 1 {
		t.Fatalf("childless milestone m2 depth = %d, want 1 (flat row under Not on roadmap)", depth)
	}
	for _, n := range m.visibleNodes() {
		if n.id == "m2" && n.hasKids {
			t.Error("m2 must be a flat row without children")
		}
	}
}

func TestUnscheduledRootHoldsEpicsWithoutMilestone(t *testing.T) {
	m := realModel(t, roadmapFixture())
	m.expanded = map[string]bool{unscheduledRootID: true, "e2": true, "m1": true, "e1": true}

	want := []string{"m1", "e1", "t1", "t2", unscheduledRootID, "e2", "t3", notOnRoadmapRootID}
	if got := nodeIDs(m.visibleNodes()); !equalStrings(got, want) {
		t.Fatalf("visibleNodes() = %q, want %q", got, want)
	}
}

func TestExpandAncestorsOfWalksTreeParent(t *testing.T) {
	m := realModel(t, roadmapFixture())

	got := expandAncestorsOf(m.treeParent, m.expanded, "t3")
	if !got["e2"] || !got[unscheduledRootID] || got["t3"] {
		t.Fatalf("expandAncestorsOf(t3) = %v, want e2 and the Unscheduled root expanded", got)
	}
	got = expandAncestorsOf(m.treeParent, m.expanded, "m2")
	if !got[notOnRoadmapRootID] {
		t.Fatalf("expandAncestorsOf(m2) = %v, want the Not on roadmap root expanded", got)
	}
}

// TestReadyViewMatchesListReady drives the real store: draft, in-progress,
// blocked todo, free todo and a feature container. Only the free todo is
// ready, and the view shows exactly what the resolver returns for the
// `lif list --ready` filter.
func TestReadyViewMatchesListReady(t *testing.T) {
	store := newTestStore(t,
		&item.Item{ID: "it-draft", Title: "Draft", Status: "draft", Type: "task", Priority: "normal"},
		&item.Item{ID: "it-wip", Title: "In progress", Status: "in-progress", Type: "task", Priority: "normal"},
		&item.Item{ID: "it-blocked", Title: "Blocked", Status: "todo", Type: "task", Priority: "normal", BlockedBy: []string{"it-wip"}},
		&item.Item{ID: "it-free", Title: "Free", Status: "todo", Type: "task", Priority: "normal"},
		&item.Item{ID: "it-feat", Title: "Feature", Status: "todo", Type: "feature", Priority: "normal"},
	)

	f := &itemmodel.ItemFilter{}
	itemgraph.ApplyReadyFilter(f, store.Config())
	listed, err := store.Resolver.Items(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, it := range listed {
		want = append(want, it.ID)
	}
	if !equalStrings(want, []string{"it-free"}) {
		t.Fatalf("setup: list --ready filter = %v, want [it-free]", want)
	}

	msg, ok := loadCmd(store)().(beansLoadedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("loadCmd = %#v, want a successful beansLoadedMsg", msg)
	}
	m := step(t, newModel(store, "/tmp/bt-fixture-repo"), msg)

	var got []string
	for _, it := range m.readyVisible() {
		got = append(got, it.ID)
	}
	sort.Strings(got)
	if !equalStrings(got, want) {
		t.Fatalf("Ready view = %v, want %v (what list --ready returns)", got, want)
	}
}
