package commands

import (
	"reflect"
	"testing"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
)

// containerIDsOfRoadmap lists milestones, epics and features in roadmap
// output order (pre-order, unscheduled containers after the milestones).
func containerIDsOfRoadmap(data *roadmap.Data) []string {
	var out []string
	var walk func(cs []roadmap.Container)
	walk = func(cs []roadmap.Container) {
		for _, c := range cs {
			if c.Epic != nil {
				out = append(out, c.Epic.Epic.ID)
				for _, f := range c.Epic.Features {
					out = append(out, f.Feature.ID)
				}
			} else {
				out = append(out, c.Feature.Feature.ID)
			}
		}
	}
	for _, m := range data.Milestones {
		out = append(out, m.Milestone.ID)
		walk(m.Containers)
	}
	if data.Unscheduled != nil {
		walk(data.Unscheduled.Containers)
	}
	return out
}

func containerIDsOfTree(nodes []*ui.TreeNode, c *config.Config) []string {
	var out []string
	for _, n := range nodes {
		if r := c.RankOf(n.Item.Type); r >= 1 && r <= config.MaxContainerRank {
			out = append(out, n.Item.ID)
		}
		out = append(out, containerIDsOfTree(n.Children, c)...)
	}
	return out
}

// TestRoadmapOrderEqualsSharedTreeOrder pins that ui.BuildTree sorted by
// roadmap.SortSiblings lists containers exactly as lif roadmap does.
func TestRoadmapOrderEqualsSharedTreeOrder(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "c", Status: "todo", Order: "2"},
		{ID: "m2", Type: "milestone", Title: "b", Status: "todo", Order: "1"},
		{ID: "e1", Type: "epic", Title: "e1", Status: "todo", Parent: "m1", Order: "2"},
		{ID: "f1", Type: "feature", Title: "f1", Status: "todo", Parent: "m1", Order: "1"},
		{ID: "t1", Type: "task", Title: "t1", Status: "todo", Parent: "e1"},
		{ID: "f2", Type: "feature", Title: "c", Status: "todo", Parent: "e1", Order: "2"},
		{ID: "f3", Type: "feature", Title: "b", Status: "todo", Parent: "e1", Order: "1"},
		{ID: "t2", Type: "task", Title: "t2", Status: "todo", Parent: "f2"},
		{ID: "t3", Type: "task", Title: "t3", Status: "todo", Parent: "f3"},
		{ID: "t4", Type: "task", Title: "t4", Status: "todo", Parent: "f1"},
		{ID: "t5", Type: "task", Title: "t5", Status: "todo", Parent: "m2"},
		{ID: "e2", Type: "epic", Title: "e2", Status: "todo", Parent: "m2"},
		{ID: "t6", Type: "task", Title: "t6", Status: "todo", Parent: "e2"},
		{ID: "e3", Type: "epic", Title: "c", Status: "todo", Order: "2"},
		{ID: "f4", Type: "feature", Title: "b", Status: "todo", Order: "1"},
		{ID: "t7", Type: "task", Title: "t7", Status: "todo", Parent: "e3"},
		{ID: "t8", Type: "task", Title: "t8", Status: "todo", Parent: "f4"},
		{ID: "m3", Type: "milestone", Title: "a", Status: "todo", Order: "3"},
		{ID: "f5", Type: "feature", Title: "a", Status: "todo", Parent: "e1", Order: "3"},
		{ID: "e4", Type: "epic", Title: "a", Status: "todo", Order: "3"},
		{ID: "t9", Type: "task", Title: "t9", Status: "todo", Parent: "m3"},
		{ID: "t10", Type: "task", Title: "t10", Status: "todo", Parent: "f5"},
		{ID: "t11", Type: "task", Title: "t11", Status: "todo", Parent: "e4"},
	}

	want := containerIDsOfRoadmap(roadmap.Build(cfg, items, false, nil, nil))
	if len(want) == 0 {
		t.Fatal("roadmap produced no containers")
	}
	sortFn := func(s []*item.Item) { roadmap.SortSiblings(cfg, s) }
	got := containerIDsOfTree(ui.BuildTree(items, items, sortFn, nil), cfg)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tree container order = %v, roadmap order = %v", got, want)
	}
}
