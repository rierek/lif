package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
)

// depthFixture builds a roadmap that exercises every branch the pruner has
// to walk: a milestone carrying an epic (with items and a nested feature), a
// direct feature and a direct leaf, plus an unscheduled branch with the same
// shape. Every level from 1 to 4 is populated, so a test can pin exactly
// which of them survives a given --depth.
func depthFixture() *roadmap.Data {
	ms := &item.Item{ID: "beans-ms01", Title: "Milestone", Type: "milestone", Status: "todo"}
	msEpic := &item.Item{ID: "beans-ep01", Title: "Epic", Type: "epic", Status: "todo", Parent: ms.ID}
	msEpicItem := &item.Item{ID: "beans-it01", Title: "Epic item", Type: "task", Status: "todo", Parent: msEpic.ID}
	msEpicFeat := &item.Item{ID: "beans-ft01", Title: "Epic feature", Type: "feature", Status: "todo", Parent: msEpic.ID}
	msEpicFeatItem := &item.Item{ID: "beans-it02", Title: "Epic feature item", Type: "task", Status: "todo", Parent: msEpicFeat.ID}
	msFeat := &item.Item{ID: "beans-ft02", Title: "Milestone feature", Type: "feature", Status: "todo", Parent: ms.ID}
	msFeatItem := &item.Item{ID: "beans-it03", Title: "Milestone feature item", Type: "task", Status: "todo", Parent: msFeat.ID}
	msOther := &item.Item{ID: "beans-it04", Title: "Milestone leaf", Type: "task", Status: "todo", Parent: ms.ID}

	unEpic := &item.Item{ID: "beans-ep02", Title: "Unscheduled epic", Type: "epic", Status: "todo"}
	unEpicItem := &item.Item{ID: "beans-it05", Title: "Unscheduled epic item", Type: "task", Status: "todo", Parent: unEpic.ID}
	unEpicFeat := &item.Item{ID: "beans-ft03", Title: "Unscheduled epic feature", Type: "feature", Status: "todo", Parent: unEpic.ID}
	unEpicFeatItem := &item.Item{ID: "beans-it06", Title: "Unscheduled epic feature item", Type: "task", Status: "todo", Parent: unEpicFeat.ID}
	unFeat := &item.Item{ID: "beans-ft04", Title: "Unscheduled feature", Type: "feature", Status: "todo"}
	unFeatItem := &item.Item{ID: "beans-it07", Title: "Unscheduled feature item", Type: "task", Status: "todo", Parent: unFeat.ID}
	unOther := &item.Item{ID: "beans-it08", Title: "Orphan leaf", Type: "task", Status: "todo"}

	return &roadmap.Data{
		Milestones: []roadmap.MilestoneGroup{
			{
				Milestone: ms,
				Containers: []roadmap.Container{
					{Epic: &roadmap.EpicGroup{
						Epic:     msEpic,
						Items:    []*item.Item{msEpicItem},
						Features: []roadmap.FeatureGroup{{Feature: msEpicFeat, Items: []*item.Item{msEpicFeatItem}}},
					}},
					{Feature: &roadmap.FeatureGroup{Feature: msFeat, Items: []*item.Item{msFeatItem}}},
				},
				Other: []*item.Item{msOther},
			},
		},
		Unscheduled: &roadmap.UnscheduledGroup{
			Containers: []roadmap.Container{
				{Epic: &roadmap.EpicGroup{
					Epic:     unEpic,
					Items:    []*item.Item{unEpicItem},
					Features: []roadmap.FeatureGroup{{Feature: unEpicFeat, Items: []*item.Item{unEpicFeatItem}}},
				}},
				{Feature: &roadmap.FeatureGroup{Feature: unFeat, Items: []*item.Item{unFeatItem}}},
			},
			Other: []*item.Item{unOther},
		},
	}
}

// scopedEpicFixture is what roadmap.BuildScoped produces for an epic root.
func scopedEpicFixture() *roadmap.Data {
	epic := &item.Item{ID: "beans-ep01", Title: "Epic", Type: "epic", Status: "todo"}
	epicTask := &item.Item{ID: "beans-it01", Title: "Epic item", Type: "task", Status: "todo", Parent: epic.ID}
	feat := &item.Item{ID: "beans-ft01", Title: "Epic feature", Type: "feature", Status: "todo", Parent: epic.ID}
	featItem := &item.Item{ID: "beans-it02", Title: "Epic feature item", Type: "task", Status: "todo", Parent: feat.ID}

	return &roadmap.Data{Root: &roadmap.RootGroup{Epic: &roadmap.EpicGroup{
		Epic:     epic,
		Items:    []*item.Item{epicTask},
		Features: []roadmap.FeatureGroup{{Feature: feat, Items: []*item.Item{featItem}}},
	}}}
}

// TestPruneRoadmapDepthUnscopedDepth1 pins the `tree -L 1` reading of an
// unscoped roadmap: the roadmap itself is the root, so milestones and the
// unscheduled branch's top-level entries are level 1 and survive, while
// everything below them is cut.
func TestPruneRoadmapDepthUnscopedDepth1(t *testing.T) {
	data := depthFixture()
	roadmap.PruneDepth(data, 1, false)

	if len(data.Milestones) != 1 {
		t.Fatalf("milestones = %d, want 1", len(data.Milestones))
	}
	mg := data.Milestones[0]
	if mg.Milestone.ID != "beans-ms01" {
		t.Errorf("milestone ID = %q, want beans-ms01", mg.Milestone.ID)
	}
	if len(mg.Containers) != 0 || len(mg.Other) != 0 {
		t.Errorf("milestone children survived depth 1: containers=%d other=%d",
			len(mg.Containers), len(mg.Other))
	}

	un := data.Unscheduled
	if un == nil {
		t.Fatal("unscheduled branch dropped at depth 1")
	}
	unEpics := epicGroupsOf(un.Containers)
	unFeatures := featureGroupsOf(un.Containers)
	if len(unEpics) != 1 || len(unFeatures) != 1 || len(un.Other) != 1 {
		t.Fatalf("unscheduled top level = epics %d, features %d, other %d; want 1/1/1",
			len(unEpics), len(unFeatures), len(un.Other))
	}
	if len(unEpics[0].Items) != 0 || len(unEpics[0].Features) != 0 {
		t.Errorf("unscheduled epic children survived depth 1: items=%d features=%d",
			len(unEpics[0].Items), len(unEpics[0].Features))
	}
	if len(unFeatures[0].Items) != 0 {
		t.Errorf("unscheduled feature items survived depth 1: %d", len(unFeatures[0].Items))
	}
}

// TestPruneRoadmapDepthUnscopedDepth2 pins level 2: a milestone's epics,
// features and leaves appear, but nothing below them does. In the
// unscheduled branch, level 2 is one step deeper -- the epic's items and its
// nested feature row.
func TestPruneRoadmapDepthUnscopedDepth2(t *testing.T) {
	data := depthFixture()
	roadmap.PruneDepth(data, 2, false)

	mg := data.Milestones[0]
	mgEpics := epicGroupsOf(mg.Containers)
	mgFeatures := featureGroupsOf(mg.Containers)
	if len(mgEpics) != 1 || len(mgFeatures) != 1 || len(mg.Other) != 1 {
		t.Fatalf("milestone level 2 = epics %d, features %d, other %d; want 1/1/1",
			len(mgEpics), len(mgFeatures), len(mg.Other))
	}
	if len(mgEpics[0].Items) != 0 || len(mgEpics[0].Features) != 0 {
		t.Errorf("epic children survived depth 2: items=%d features=%d",
			len(mgEpics[0].Items), len(mgEpics[0].Features))
	}
	if len(mgFeatures[0].Items) != 0 {
		t.Errorf("milestone feature items survived depth 2: %d", len(mgFeatures[0].Items))
	}

	un := data.Unscheduled
	unEpics := epicGroupsOf(un.Containers)
	unFeatures := featureGroupsOf(un.Containers)
	if len(unEpics[0].Items) != 1 || len(unEpics[0].Features) != 1 {
		t.Fatalf("unscheduled epic level 2 = items %d, features %d; want 1/1",
			len(unEpics[0].Items), len(unEpics[0].Features))
	}
	if len(unEpics[0].Features[0].Items) != 0 {
		t.Errorf("unscheduled nested feature items survived depth 2: %d",
			len(unEpics[0].Features[0].Items))
	}
	if len(unFeatures[0].Items) != 1 {
		t.Errorf("unscheduled feature items at depth 2 = %d, want 1", len(unFeatures[0].Items))
	}
}

// TestPruneRoadmapDepthUnscopedDepth3 pins level 3: the epic's items and its
// nested feature row are in, that feature's own items are out.
func TestPruneRoadmapDepthUnscopedDepth3(t *testing.T) {
	data := depthFixture()
	roadmap.PruneDepth(data, 3, false)

	eg := epicGroupsOf(data.Milestones[0].Containers)[0]
	if len(eg.Items) != 1 || len(eg.Features) != 1 {
		t.Fatalf("epic level 3 = items %d, features %d; want 1/1", len(eg.Items), len(eg.Features))
	}
	if len(eg.Features[0].Items) != 0 {
		t.Errorf("nested feature items survived depth 3: %d", len(eg.Features[0].Items))
	}
	if len(featureGroupsOf(data.Milestones[0].Containers)[0].Items) != 1 {
		t.Errorf("milestone feature items at depth 3 = %d, want 1",
			len(featureGroupsOf(data.Milestones[0].Containers)[0].Items))
	}
	if len(epicGroupsOf(data.Unscheduled.Containers)[0].Features[0].Items) != 1 {
		t.Errorf("unscheduled nested feature items at depth 3 = %d, want 1",
			len(epicGroupsOf(data.Unscheduled.Containers)[0].Features[0].Items))
	}
}

// TestPruneRoadmapDepthUnscopedDepth4 pins the deepest populated level: at 4
// the fixture is untouched, which is also what any larger depth must yield.
func TestPruneRoadmapDepthUnscopedDepth4(t *testing.T) {
	for _, depth := range []int{4, 5, 99} {
		data := depthFixture()
		roadmap.PruneDepth(data, depth, false)

		if got := len(epicGroupsOf(data.Milestones[0].Containers)[0].Features[0].Items); got != 1 {
			t.Errorf("depth %d: deepest items = %d, want 1", depth, got)
		}
		if got := len(epicGroupsOf(data.Unscheduled.Containers)[0].Features[0].Items); got != 1 {
			t.Errorf("depth %d: deepest unscheduled items = %d, want 1", depth, got)
		}
	}
}

// TestPruneRoadmapDepthZeroIsNoOp pins that a depth the command rejects (or
// never sets) leaves the roadmap alone rather than emptying it.
func TestPruneRoadmapDepthZeroIsNoOp(t *testing.T) {
	for _, depth := range []int{0, -1} {
		data := depthFixture()
		roadmap.PruneDepth(data, depth, false)

		if got := len(epicGroupsOf(data.Milestones[0].Containers)[0].Features[0].Items); got != 1 {
			t.Errorf("depth %d: deepest items = %d, want 1 (no-op)", depth, got)
		}
	}
}

// TestPruneRoadmapDepthScopedMilestone pins that a scope ID shifts the
// counting by one: the scoped milestone is the root and never counts, so its
// direct children are level 1.
func TestPruneRoadmapDepthScopedMilestone(t *testing.T) {
	data := depthFixture()
	data.Unscheduled = nil // roadmap.BuildScoped returns the milestone alone
	roadmap.PruneDepth(data, 1, true)

	mg := data.Milestones[0]
	if mg.Milestone.ID != "beans-ms01" {
		t.Fatalf("scoped milestone dropped: %+v", mg.Milestone)
	}
	mgEpics := epicGroupsOf(mg.Containers)
	mgFeatures := featureGroupsOf(mg.Containers)
	if len(mgEpics) != 1 || len(mgFeatures) != 1 || len(mg.Other) != 1 {
		t.Fatalf("scoped depth 1 = epics %d, features %d, other %d; want 1/1/1",
			len(mgEpics), len(mgFeatures), len(mg.Other))
	}
	if len(mgEpics[0].Items) != 0 || len(mgEpics[0].Features) != 0 {
		t.Errorf("scoped epic children survived depth 1: items=%d features=%d",
			len(mgEpics[0].Items), len(mgEpics[0].Features))
	}
	if len(mgFeatures[0].Items) != 0 {
		t.Errorf("scoped feature items survived depth 1: %d", len(mgFeatures[0].Items))
	}
}

// TestPruneRoadmapDepthScopedEpicRoot pins the same shift for an epic root:
// the epic row is the root, its items and nested feature rows are level 1.
func TestPruneRoadmapDepthScopedEpicRoot(t *testing.T) {
	data := scopedEpicFixture()
	roadmap.PruneDepth(data, 1, true)

	eg := data.Root.Epic
	if eg == nil || eg.Epic.ID != "beans-ep01" {
		t.Fatalf("scoped epic root dropped: %+v", data.Root)
	}
	if len(eg.Items) != 1 || len(eg.Features) != 1 {
		t.Fatalf("scoped epic depth 1 = items %d, features %d; want 1/1", len(eg.Items), len(eg.Features))
	}
	if len(eg.Features[0].Items) != 0 {
		t.Errorf("nested feature items survived scoped depth 1: %d", len(eg.Features[0].Items))
	}
}

// TestPruneRoadmapDepthScopedFeatureRoot pins the feature root: its items are
// level 1, so depth 1 keeps them and the root row stays either way.
func TestPruneRoadmapDepthScopedFeatureRoot(t *testing.T) {
	feat := &item.Item{ID: "beans-ft01", Title: "Feature", Type: "feature", Status: "todo"}
	featTask := &item.Item{ID: "beans-it01", Title: "Item", Type: "task", Status: "todo", Parent: feat.ID}
	data := &roadmap.Data{Root: &roadmap.RootGroup{Feature: &roadmap.FeatureGroup{Feature: feat, Items: []*item.Item{featTask}}}}

	roadmap.PruneDepth(data, 1, true)

	if data.Root.Feature == nil || data.Root.Feature.Feature.ID != "beans-ft01" {
		t.Fatalf("scoped feature root dropped: %+v", data.Root)
	}
	if len(data.Root.Feature.Items) != 1 {
		t.Errorf("feature items at scoped depth 1 = %d, want 1", len(data.Root.Feature.Items))
	}
}

// TestValidateRoadmapDepth pins the flag validation: an explicitly passed
// depth below 1 is a user error, while the unset zero value means "no limit"
// and must pass.
func TestValidateRoadmapDepth(t *testing.T) {
	tests := []struct {
		name    string
		depth   int
		changed bool
		wantErr bool
	}{
		{name: "unset zero is no limit", depth: 0, changed: false, wantErr: false},
		{name: "explicit zero rejected", depth: 0, changed: true, wantErr: true},
		{name: "explicit negative rejected", depth: -3, changed: true, wantErr: true},
		{name: "explicit one accepted", depth: 1, changed: true, wantErr: false},
		{name: "explicit four accepted", depth: 4, changed: true, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRoadmapDepth(tt.depth, tt.changed)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateRoadmapDepth(%d, %v) error = %v, wantErr %v",
					tt.depth, tt.changed, err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "--depth") {
				t.Errorf("error = %q, want it to name --depth", err)
			}
		})
	}
}

// TestRoadmapCmdJSONRespectsDepth pins that the truncation reaches --json,
// not just the two text renderers.
func TestRoadmapCmdJSONRespectsDepth(t *testing.T) {
	testCore := setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("v1"), Title: "v1", Status: "todo", Type: "milestone"}
	if err := testCore.Create(milestone); err != nil {
		t.Fatalf("core.Create(milestone) error = %v", err)
	}
	epic := &item.Item{ID: "beans-epic1", Slug: item.Slugify("Auth"), Title: "Auth", Status: "todo", Type: "epic", Parent: milestone.ID}
	if err := testCore.Create(epic); err != nil {
		t.Fatalf("core.Create(epic) error = %v", err)
	}
	task := &item.Item{ID: "beans-task1", Slug: item.Slugify("Login"), Title: "Login", Status: "todo", Type: "task", Parent: epic.ID}
	if err := testCore.Create(task); err != nil {
		t.Fatalf("core.Create(task) error = %v", err)
	}

	out := new(bytes.Buffer)
	roadmapCmd.SetOut(out)
	roadmapJSON = true
	roadmapDepth = 1

	if err := roadmapCmd.RunE(roadmapCmd, nil); err != nil {
		t.Fatalf("roadmapCmd.RunE() error = %v", err)
	}

	var got roadmap.Data
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(got.Milestones) != 1 {
		t.Fatalf("got Milestones = %+v, want exactly one", got.Milestones)
	}
	if epics := epicGroupsOf(got.Milestones[0].Containers); len(epics) != 0 {
		t.Errorf("epics survived --depth 1 in JSON: %+v", epics)
	}
}

// TestRoadmapCmdDepthWithScopeCountsFromRoot pins the scoped shift end to
// end: `roadmap <milestone> --depth 1` shows the milestone's epic rows,
// where the unscoped `--depth 1` would stop at the milestone itself.
func TestRoadmapCmdDepthWithScopeCountsFromRoot(t *testing.T) {
	testCore := setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("v1"), Title: "v1", Status: "todo", Type: "milestone"}
	if err := testCore.Create(milestone); err != nil {
		t.Fatalf("core.Create(milestone) error = %v", err)
	}
	epic := &item.Item{ID: "beans-epic1", Slug: item.Slugify("Auth"), Title: "Auth", Status: "todo", Type: "epic", Parent: milestone.ID}
	if err := testCore.Create(epic); err != nil {
		t.Fatalf("core.Create(epic) error = %v", err)
	}
	task := &item.Item{ID: "beans-task1", Slug: item.Slugify("Login"), Title: "Login", Status: "todo", Type: "task", Parent: epic.ID}
	if err := testCore.Create(task); err != nil {
		t.Fatalf("core.Create(task) error = %v", err)
	}

	out := new(bytes.Buffer)
	roadmapCmd.SetOut(out)
	roadmapJSON = true
	roadmapDepth = 1

	if err := roadmapCmd.RunE(roadmapCmd, []string{milestone.ID}); err != nil {
		t.Fatalf("roadmapCmd.RunE() error = %v", err)
	}

	var got roadmap.Data
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(got.Milestones) != 1 {
		t.Fatalf("scoped --depth 1 = %+v, want the milestone with its one epic", got.Milestones)
	}
	epics := epicGroupsOf(got.Milestones[0].Containers)
	if len(epics) != 1 {
		t.Fatalf("scoped --depth 1 = %+v, want the milestone with its one epic", got.Milestones)
	}
	if len(epics[0].Items) != 0 {
		t.Errorf("epic items survived scoped --depth 1: %+v", epics[0].Items)
	}
}
