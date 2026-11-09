package tui

// beans-teuf: a milestone's leafless epic and an empty open milestone are on
// the roadmap, so setItems must not sink them into "Not on roadmap".

import (
	"testing"

	"github.com/rierek/lif/pkg/item"
)

func TestEmptyContainersStayOnTheRoadmap(t *testing.T) {
	m := fixtureModel(t, []*item.Item{
		{ID: "ms-full", Type: "milestone", Title: "Full", Status: "todo"},
		{ID: "ep-empty", Type: "epic", Title: "Empty epic", Status: "todo", Parent: "ms-full"},
		{ID: "ep-full", Type: "epic", Title: "Full epic", Status: "todo", Parent: "ms-full"},
		{ID: "t1", Type: "task", Title: "Work", Status: "todo", Parent: "ep-full"},
		{ID: "ms-empty", Type: "milestone", Title: "Empty milestone", Status: "todo"},
	})

	if len(m.notOnRoadmap) != 0 {
		t.Errorf("notOnRoadmap = %d nodes, want 0", len(m.notOnRoadmap))
	}
	if got := m.treeParent["ep-empty"]; got != "ms-full" {
		t.Errorf("treeParent[ep-empty] = %q, want ms-full", got)
	}
	var roots []string
	for _, n := range m.roadmapScheduled {
		roots = append(roots, n.Item.ID)
	}
	if len(roots) != 2 {
		t.Errorf("scheduled roots = %v, want both milestones", roots)
	}
}
