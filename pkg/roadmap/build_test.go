package roadmap

import (
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// epicGroupsOf extracts the epic-slot groups from a mixed container list, in
// order, for tests written against milestoneGroup/unscheduledGroup's former
// dedicated Epics field (beans-swy1).
func epicGroupsOf(containers []Container) []EpicGroup {
	var out []EpicGroup
	for _, c := range containers {
		if c.Epic != nil {
			out = append(out, *c.Epic)
		}
	}
	return out
}

// featureGroupsOf extracts the feature-slot groups from a mixed container
// list, in order, for tests written against milestoneGroup/unscheduledGroup's
// former dedicated Features field (beans-swy1).
func featureGroupsOf(containers []Container) []FeatureGroup {
	var out []FeatureGroup
	for _, c := range containers {
		if c.Feature != nil {
			out = append(out, *c.Feature)
		}
	}
	return out
}

func TestBuildRoadmap(t *testing.T) {

	cfg := config.Default()

	now := time.Now()

	tests := []struct {
		name                 string
		items                []*item.Item
		includeDone          bool
		wantMilestones       int
		wantUnscheduledEpics int
		wantUnscheduledOther int
	}{
		{
			name:           "empty beans",
			items:          []*item.Item{},
			wantMilestones: 0,
		},
		{
			name: "milestone with epic and items",
			items: []*item.Item{
				{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
				{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
				{ID: "t1", Type: "task", Title: "Login", Status: "todo", Parent: "e1"},
			},
			wantMilestones: 1,
		},
		{
			name: "milestone with direct children (no epic)",
			items: []*item.Item{
				{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
				{ID: "t1", Type: "task", Title: "Docs", Status: "todo", Parent: "m1"},
			},
			wantMilestones: 1,
		},
		{
			name: "unscheduled epic",
			items: []*item.Item{
				{ID: "e1", Type: "epic", Title: "Future", Status: "todo"},
				{ID: "t1", Type: "task", Title: "Nice to have", Status: "todo", Parent: "e1"},
			},
			wantMilestones:       0,
			wantUnscheduledEpics: 1,
		},
		{
			name: "done items excluded by default",
			items: []*item.Item{
				{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
				{ID: "t1", Type: "task", Title: "Done task", Status: "completed", Parent: "m1"},
			},
			includeDone:    false,
			wantMilestones: 0, // milestone has no visible children
		},
		{
			name: "done items included when requested",
			items: []*item.Item{
				{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
				{ID: "t1", Type: "task", Title: "Done task", Status: "completed", Parent: "m1"},
			},
			includeDone:    true,
			wantMilestones: 1,
		},
		{
			name: "orphan bean appears in unscheduled other",
			items: []*item.Item{
				{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
				{ID: "t1", Type: "task", Title: "Orphan", Status: "todo"}, // no parent link
			},
			wantMilestones:       0, // milestone has no children
			wantUnscheduledOther: 1, // orphan appears in unscheduled
		},
		{
			name: "leaf nested under feature under epic under milestone is not lost",
			items: []*item.Item{
				{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
				{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
				{ID: "f1", Type: "feature", Title: "SSO", Status: "todo", Parent: "e1"},
				{ID: "t1", Type: "task", Title: "OIDC login", Status: "todo", Parent: "f1"},
			},
			wantMilestones: 1,
		},
		{
			name: "milestone with direct feature child and no epic is not dropped",
			items: []*item.Item{
				{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
				{ID: "f1", Type: "feature", Title: "SSO", Status: "todo", Parent: "m1"},
				{ID: "t1", Type: "task", Title: "OIDC login", Status: "todo", Parent: "f1"},
			},
			wantMilestones: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Build(cfg, tt.items, tt.includeDone, nil, nil)

			if got := len(result.Milestones); got != tt.wantMilestones {
				t.Errorf("got %d milestones, want %d", got, tt.wantMilestones)
			}

			gotUnscheduledEpics := 0
			gotUnscheduledOther := 0
			if result.Unscheduled != nil {
				gotUnscheduledEpics = len(epicGroupsOf(result.Unscheduled.Containers))
				gotUnscheduledOther = len(result.Unscheduled.Other)
			}
			if gotUnscheduledEpics != tt.wantUnscheduledEpics {
				t.Errorf("got %d unscheduled epics, want %d", gotUnscheduledEpics, tt.wantUnscheduledEpics)
			}
			if gotUnscheduledOther != tt.wantUnscheduledOther {
				t.Errorf("got %d unscheduled other, want %d", gotUnscheduledOther, tt.wantUnscheduledOther)
			}
		})
	}
}

func TestStatusFiltering(t *testing.T) {

	cfg := config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "Todo Milestone", Status: "todo", CreatedAt: &now},
		{ID: "m2", Type: "milestone", Title: "In Progress Milestone", Status: "in-progress", CreatedAt: &now},
		{ID: "t1", Type: "task", Title: "Task 1", Status: "todo", Parent: "m1"},
		{ID: "t2", Type: "task", Title: "Task 2", Status: "todo", Parent: "m2"},
	}

	t.Run("filter by status", func(t *testing.T) {
		result := Build(cfg, items, false, []string{"todo"}, nil)
		if len(result.Milestones) != 1 {
			t.Errorf("expected 1 milestone, got %d", len(result.Milestones))
		}
		if result.Milestones[0].Milestone.Status != "todo" {
			t.Errorf("expected todo milestone, got %s", result.Milestones[0].Milestone.Status)
		}
	})

	t.Run("exclude by status", func(t *testing.T) {
		result := Build(cfg, items, false, nil, []string{"in-progress"})
		if len(result.Milestones) != 1 {
			t.Errorf("expected 1 milestone, got %d", len(result.Milestones))
		}
		if result.Milestones[0].Milestone.Status != "todo" {
			t.Errorf("expected todo milestone, got %s", result.Milestones[0].Milestone.Status)
		}
	})
}

func TestMilestoneOrderRespectsManualOrderKey(t *testing.T) {

	cfg := config.Default()

	older := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	items := []*item.Item{
		// Created later but manually ordered first via the "order" key.
		{ID: "m2", Type: "milestone", Title: "v2.0", Status: "todo", Order: "a", CreatedAt: &newer},
		{ID: "t2", Type: "task", Title: "Task 2", Status: "todo", Parent: "m2"},
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", Order: "z", CreatedAt: &older},
		{ID: "t1", Type: "task", Title: "Task 1", Status: "todo", Parent: "m1"},
	}

	result := Build(cfg, items, false, nil, nil)

	if len(result.Milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(result.Milestones))
	}
	if result.Milestones[0].Milestone.ID != "m2" || result.Milestones[1].Milestone.ID != "m1" {
		t.Errorf("milestone order = [%q, %q], want [m2, m1] (manual order overrides created_at)",
			result.Milestones[0].Milestone.ID, result.Milestones[1].Milestone.ID)
	}
}

func TestMilestoneOrderRespectsBlockingDependency(t *testing.T) {

	cfg := config.Default()

	// IDs deliberately sort the "wrong" way (aa- before zz-) so the ID
	// fallback alone cannot make this test pass -- only Blocking/BlockedBy
	// awareness can put the blocker first.
	items := []*item.Item{
		{ID: "aa-blocked", Type: "milestone", Title: "Blocked", Status: "todo", BlockedBy: []string{"zz-blocker"}},
		{ID: "t2", Type: "task", Title: "Task 2", Status: "todo", Parent: "aa-blocked"},
		{ID: "zz-blocker", Type: "milestone", Title: "Blocker", Status: "todo", Blocking: []string{"aa-blocked"}},
		{ID: "t1", Type: "task", Title: "Task 1", Status: "todo", Parent: "zz-blocker"},
	}

	result := Build(cfg, items, false, nil, nil)

	if len(result.Milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(result.Milestones))
	}
	if result.Milestones[0].Milestone.ID != "zz-blocker" || result.Milestones[1].Milestone.ID != "aa-blocked" {
		t.Errorf("milestone order = [%q, %q], want [zz-blocker, aa-blocked] (blocker sorts first)",
			result.Milestones[0].Milestone.ID, result.Milestones[1].Milestone.ID)
	}
}

func TestSplitByContainerType(t *testing.T) {
	cfg := config.Default()
	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "F1"},
		{ID: "t1", Type: "task", Title: "T1"},
		{ID: "b1", Type: "bug", Title: "B1"},
	}

	leafs, features := splitByContainerType(cfg, items)

	if len(leafs) != 2 {
		t.Errorf("got %d leafs, want 2", len(leafs))
	}
	if len(features) != 1 {
		t.Errorf("got %d features, want 1", len(features))
	}
	if features[0].ID != "f1" {
		t.Errorf("got feature %s, want f1", features[0].ID)
	}
}

func TestCollectLeafDescendants(t *testing.T) {
	cfg := config.Default()

	children := map[string][]*item.Item{
		"feat1": {
			{ID: "t1", Type: "task", Title: "Direct leaf", Status: "todo", Parent: "feat1"},
			{ID: "feat2", Type: "feature", Title: "Nested feature", Status: "todo", Parent: "feat1"},
		},
		"feat2": {
			{ID: "t2", Type: "task", Title: "Nested leaf", Status: "todo", Parent: "feat2"},
			{ID: "t3", Type: "task", Title: "Done nested leaf", Status: "completed", Parent: "feat2"},
		},
	}

	t.Run("flattens through nested features, excludes done by default", func(t *testing.T) {
		got := collectLeafDescendants(cfg, "feat1", children, false, nil)
		if len(got) != 2 {
			t.Fatalf("got %d leafs, want 2 (t1, t2)", len(got))
		}
		ids := map[string]bool{got[0].ID: true, got[1].ID: true}
		if !ids["t1"] || !ids["t2"] {
			t.Errorf("got ids %v, want t1 and t2", ids)
		}
	})

	t.Run("includes done when requested", func(t *testing.T) {
		got := collectLeafDescendants(cfg, "feat1", children, true, nil)
		if len(got) != 3 {
			t.Fatalf("got %d leafs, want 3 (t1, t2, t3)", len(got))
		}
	})

	t.Run("no children returns empty, not nil panic", func(t *testing.T) {
		got := collectLeafDescendants(cfg, "nonexistent", children, false, nil)
		if len(got) != 0 {
			t.Errorf("got %d leafs, want 0", len(got))
		}
	})

	t.Run("hand-authored parent cycle does not stack-overflow", func(t *testing.T) {
		// The CLI's ValidateParent/DetectCycle reject this at write time, but
		// items are hand-editable markdown -- a manually edited cycle must not
		// crash roadmap generation. This subtest hangs/panics without the
		// visited guard in collectLeafDescendantsVisited.
		cyclic := map[string][]*item.Item{
			"featA": {
				{ID: "featB", Type: "feature", Title: "B", Status: "todo", Parent: "featA"},
			},
			"featB": {
				{ID: "featA", Type: "feature", Title: "A", Status: "todo", Parent: "featB"},
				{ID: "t1", Type: "task", Title: "Reachable leaf", Status: "todo", Parent: "featB"},
			},
		}
		got := collectLeafDescendants(cfg, "featA", cyclic, false, nil)
		if len(got) != 1 || got[0].ID != "t1" {
			t.Errorf("got %v, want exactly [t1]", got)
		}
	})
}

func TestBuildMilestoneGroupResolvesFeatureNesting(t *testing.T) {
	cfg := config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
		{ID: "f1", Type: "feature", Title: "SSO", Status: "todo", Parent: "e1"},
		{ID: "t1", Type: "task", Title: "OIDC login", Status: "todo", Parent: "f1"},
		{ID: "b1", Type: "bug", Title: "Direct epic bug", Status: "todo", Parent: "e1"},
	}

	result := Build(cfg, items, false, nil, nil)

	if len(result.Milestones) != 1 {
		t.Fatalf("got %d milestones, want 1", len(result.Milestones))
	}
	epics := epicGroupsOf(result.Milestones[0].Containers)
	if len(epics) != 1 {
		t.Fatalf("got %d epics, want 1", len(epics))
	}
	epic := epics[0]
	if len(epic.Items) != 1 || epic.Items[0].ID != "b1" {
		t.Errorf("epic.Items = %v, want [b1]", epic.Items)
	}
	if len(epic.Features) != 1 {
		t.Fatalf("got %d feature groups, want 1", len(epic.Features))
	}
	if epic.Features[0].Feature.ID != "f1" {
		t.Errorf("feature group is for %s, want f1", epic.Features[0].Feature.ID)
	}
	if len(epic.Features[0].Items) != 1 || epic.Features[0].Items[0].ID != "t1" {
		t.Errorf("feature.Items = %v, want [t1]", epic.Features[0].Items)
	}
}

func TestUnscheduledFeatureResolvesNesting(t *testing.T) {
	cfg := config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Standalone feature", Status: "todo"},
		{ID: "t1", Type: "task", Title: "Leaf under orphan feature", Status: "todo", Parent: "f1"},
	}

	result := Build(cfg, items, false, nil, nil)

	if result.Unscheduled == nil {
		t.Fatal("expected Unscheduled to be non-nil")
	}
	unscheduledFeatures := featureGroupsOf(result.Unscheduled.Containers)
	if len(unscheduledFeatures) != 1 {
		t.Fatalf("got %d unscheduled features, want 1", len(unscheduledFeatures))
	}
	fg := unscheduledFeatures[0]
	if fg.Feature.ID != "f1" {
		t.Errorf("feature group is for %s, want f1", fg.Feature.ID)
	}
	if len(fg.Items) != 1 || fg.Items[0].ID != "t1" {
		t.Errorf("fg.Items = %v, want [t1]", fg.Items)
	}
	// The leaf must not also leak into Other (it has a parent, so it's
	// handled entirely via the feature group).
	if len(result.Unscheduled.Other) != 0 {
		t.Errorf("got %d unscheduled other, want 0", len(result.Unscheduled.Other))
	}
}

func TestUnscheduledEpicWithFeatureNesting(t *testing.T) {
	cfg := config.Default()

	items := []*item.Item{
		{ID: "e1", Type: "epic", Title: "Unscheduled epic", Status: "todo"},
		{ID: "f1", Type: "feature", Title: "Feature under unscheduled epic", Status: "todo", Parent: "e1"},
		{ID: "t1", Type: "task", Title: "Leaf", Status: "todo", Parent: "f1"},
	}

	result := Build(cfg, items, false, nil, nil)

	unscheduledEpics := epicGroupsOf(result.Unscheduled.Containers)
	if len(unscheduledEpics) != 1 {
		t.Fatalf("got %d unscheduled epics, want 1", len(unscheduledEpics))
	}
	eg := unscheduledEpics[0]
	if len(eg.Features) != 1 || eg.Features[0].Feature.ID != "f1" {
		t.Fatalf("eg.Features = %+v, want feature f1", eg.Features)
	}
	if len(eg.Features[0].Items) != 1 || eg.Features[0].Items[0].ID != "t1" {
		t.Errorf("eg.Features[0].Items = %v, want [t1]", eg.Features[0].Items)
	}
}

func TestUnscheduledNestedFeatureNotDoubleRendered(t *testing.T) {
	// A feature nested under another orphan feature (hand-edited data --
	// ValidateParent rejects this via the CLI) must be flattened into the
	// top feature's Items exactly once, never also appear as its own
	// top-level unscheduled feature entry.
	cfg := config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Top feature", Status: "todo"},
		{ID: "f2", Type: "feature", Title: "Nested feature", Status: "todo", Parent: "f1"},
		{ID: "t1", Type: "task", Title: "Leaf", Status: "todo", Parent: "f2"},
	}

	result := Build(cfg, items, false, nil, nil)

	unscheduledFeatures := featureGroupsOf(result.Unscheduled.Containers)
	if len(unscheduledFeatures) != 1 {
		t.Fatalf("got %d unscheduled features, want 1 (f1 only, f2 must not double-render)", len(unscheduledFeatures))
	}
	fg := unscheduledFeatures[0]
	if fg.Feature.ID != "f1" {
		t.Errorf("unscheduled feature is %s, want f1", fg.Feature.ID)
	}
	if len(fg.Items) != 1 || fg.Items[0].ID != "t1" {
		t.Errorf("fg.Items = %v, want [t1]", fg.Items)
	}
}

// Ta: orphan feature, 0 children, status todo -> flat leaf in
// Unscheduled.Other, never as a featureGroup, never dropped.
func TestOrphanChildlessFeatureAppearsAsFlatLeaf(t *testing.T) {
	cfg := config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Lonely", Status: "todo"},
	}

	result := Build(cfg, items, false, nil, nil)

	if result.Unscheduled == nil {
		t.Fatal("expected Unscheduled to be non-nil")
	}
	if unscheduledFeatures := featureGroupsOf(result.Unscheduled.Containers); len(unscheduledFeatures) != 0 {
		t.Errorf("got %d unscheduled featureGroups, want 0 (childless feature must not become a container)", len(unscheduledFeatures))
	}
	if len(result.Unscheduled.Other) != 1 || result.Unscheduled.Other[0].ID != "f1" {
		t.Errorf("Unscheduled.Other = %v, want [f1]", result.Unscheduled.Other)
	}
}

// Tb: feature under epic, 0 children -> flat leaf in epic.Items, not in
// epic.Features.
func TestChildlessFeatureUnderEpicAppearsInEpicItems(t *testing.T) {
	cfg := config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
		{ID: "f1", Type: "feature", Title: "Lonely", Status: "todo", Parent: "e1"},
	}

	result := Build(cfg, items, false, nil, nil)

	if len(result.Milestones) != 1 {
		t.Fatalf("got %d milestones, want 1", len(result.Milestones))
	}
	epics := epicGroupsOf(result.Milestones[0].Containers)
	if len(epics) != 1 {
		t.Fatalf("got %d epics, want 1", len(epics))
	}
	epic := epics[0]
	if len(epic.Features) != 0 {
		t.Errorf("got %d epic feature groups, want 0", len(epic.Features))
	}
	if len(epic.Items) != 1 || epic.Items[0].ID != "f1" {
		t.Errorf("epic.Items = %v, want [f1]", epic.Items)
	}
}

// Tc: feature directly under milestone, 0 children -> flat leaf in
// milestone.Other.
func TestChildlessFeatureDirectUnderMilestoneAppearsInOther(t *testing.T) {
	cfg := config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "f1", Type: "feature", Title: "Lonely", Status: "todo", Parent: "m1"},
	}

	result := Build(cfg, items, false, nil, nil)

	if len(result.Milestones) != 1 {
		t.Fatalf("got %d milestones, want 1", len(result.Milestones))
	}
	ms := result.Milestones[0]
	if msFeatures := featureGroupsOf(ms.Containers); len(msFeatures) != 0 {
		t.Errorf("got %d milestone feature groups, want 0", len(msFeatures))
	}
	if len(ms.Other) != 1 || ms.Other[0].ID != "f1" {
		t.Errorf("milestone.Other = %v, want [f1]", ms.Other)
	}
}

// Td (regression guard): a feature with >=1 live child still renders as a
// featureGroup container, even when a childless sibling feature is present
// in the same epic -- the two code paths must not cross-contaminate.
func TestFeatureWithChildRemainsContainerAlongsideChildlessSibling(t *testing.T) {
	cfg := config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
		{ID: "f1", Type: "feature", Title: "Childless", Status: "todo", Parent: "e1"},
		{ID: "f2", Type: "feature", Title: "Has a child", Status: "todo", Parent: "e1"},
		{ID: "t1", Type: "task", Title: "OIDC login", Status: "todo", Parent: "f2"},
	}

	result := Build(cfg, items, false, nil, nil)

	epic := epicGroupsOf(result.Milestones[0].Containers)[0]
	if len(epic.Items) != 1 || epic.Items[0].ID != "f1" {
		t.Errorf("epic.Items = %v, want [f1] (childless feature flattened)", epic.Items)
	}
	if len(epic.Features) != 1 || epic.Features[0].Feature.ID != "f2" {
		t.Fatalf("epic.Features = %+v, want exactly [f2]", epic.Features)
	}
	if len(epic.Features[0].Items) != 1 || epic.Features[0].Items[0].ID != "t1" {
		t.Errorf("epic.Features[0].Items = %v, want [t1]", epic.Features[0].Items)
	}
}

// Te (edge case): a childless feature with an archive status is dropped by
// the normal archive-status filter, exactly like any other leaf.
func TestChildlessCompletedFeatureDroppedByArchiveFilter(t *testing.T) {
	cfg := config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Done and lonely", Status: "completed"},
	}

	result := Build(cfg, items, false, nil, nil)

	if result.Unscheduled != nil {
		t.Errorf("expected Unscheduled to be nil (completed childless feature dropped), got %+v", result.Unscheduled)
	}
}

// beans-36fa: a childless orphan epic (type epic, status open, no parent)
// must render as a flat leaf row in the Unscheduled group, analogous to
// childless orphan features (beans-n8zw).
func TestOrphanChildlessEpicAppearsAsFlatLeaf(t *testing.T) {
	cfg := config.Default()

	items := []*item.Item{
		{ID: "e1", Type: "epic", Title: "Lonely Epic", Status: "todo"},
	}

	result := Build(cfg, items, false, nil, nil)

	if result.Unscheduled == nil {
		t.Fatal("expected Unscheduled to be non-nil")
	}
	if unscheduledEpics := epicGroupsOf(result.Unscheduled.Containers); len(unscheduledEpics) != 0 {
		t.Errorf("got %d unscheduled epicGroups, want 0 (childless epic must not become a container)", len(unscheduledEpics))
	}
	if len(result.Unscheduled.Other) != 1 || result.Unscheduled.Other[0].ID != "e1" {
		t.Errorf("Unscheduled.Other = %v, want [e1]", result.Unscheduled.Other)
	}
}

// TestRoadmapInterleavesEpicAndFeatureContainersByOrder is the case
// beans-swy1 fixes: before the unified Containers list, a milestone's epic-
// and feature-typed children rendered in two rank-segregated blocks, so a
// feature's manual order could never move it ahead of an epic under the
// same parent. With one order-comparable Containers slot, a feature with a
// lower order renders before an epic with a higher order.
func TestRoadmapInterleavesEpicAndFeatureContainersByOrder(t *testing.T) {
	cfg := config.Default()

	items := []*item.Item{
		{ID: "beans-m1", Type: "milestone", Title: "v1.0", Status: "todo"},
		{ID: "beans-e1", Type: "epic", Title: "Cross-cutting alignment", Status: "todo", Parent: "beans-m1", Order: "2"},
		{ID: "beans-et1", Type: "task", Title: "Epic task", Status: "todo", Parent: "beans-e1"},
		{ID: "beans-f1", Type: "feature", Title: "Customer-facing feature", Status: "todo", Parent: "beans-m1", Order: "1"},
		{ID: "beans-ft1", Type: "task", Title: "Feature task", Status: "todo", Parent: "beans-f1"},
	}

	data := Build(cfg, items, false, nil, nil)

	if len(data.Milestones) != 1 {
		t.Fatalf("got %d milestone groups, want 1", len(data.Milestones))
	}
	containers := data.Milestones[0].Containers
	if len(containers) != 2 {
		t.Fatalf("got %d containers, want 2", len(containers))
	}
	if containers[0].Feature == nil || containers[0].Feature.Feature.ID != "beans-f1" {
		t.Fatalf("containers[0] = %+v, want feature beans-f1 (lower order)", containers[0])
	}
	if containers[1].Epic == nil || containers[1].Epic.Epic.ID != "beans-e1" {
		t.Fatalf("containers[1] = %+v, want epic beans-e1 (higher order)", containers[1])
	}
}

func TestBuildScopedRoadmapMilestoneRoot(t *testing.T) {
	cfg := config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
		{ID: "t1", Type: "task", Title: "Login", Status: "todo", Parent: "e1"},
		{ID: "m2", Type: "milestone", Title: "v2.0", Status: "todo", CreatedAt: &now},
		{ID: "t2", Type: "task", Title: "Other milestone task", Status: "todo", Parent: "m2"},
		{ID: "e3", Type: "epic", Title: "Unscheduled", Status: "todo"},
	}

	root := items[0] // m1
	result := BuildScoped(cfg, items, false, root)

	if len(result.Milestones) != 1 {
		t.Fatalf("got %d milestones, want 1", len(result.Milestones))
	}
	if result.Milestones[0].Milestone.ID != "m1" {
		t.Errorf("got milestone %s, want m1", result.Milestones[0].Milestone.ID)
	}
	if result.Unscheduled != nil {
		t.Errorf("expected Unscheduled to be nil when scoped to a milestone, got %+v", result.Unscheduled)
	}
	if result.Root != nil {
		t.Errorf("expected Root to be nil for milestone scope, got %+v", result.Root)
	}
}

func TestBuildScopedRoadmapMilestoneRootWithNoVisibleChildren(t *testing.T) {
	cfg := config.Default()

	now := time.Now()
	root := &item.Item{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now}
	items := []*item.Item{root}

	result := BuildScoped(cfg, items, false, root)

	if len(result.Milestones) != 1 {
		t.Fatalf("got %d milestones, want 1 (empty container still rendered)", len(result.Milestones))
	}
	if result.Milestones[0].Milestone.ID != "m1" {
		t.Errorf("got milestone %s, want m1", result.Milestones[0].Milestone.ID)
	}
	if len(result.Milestones[0].Containers) != 0 || len(result.Milestones[0].Other) != 0 {
		t.Errorf("expected empty milestone group, got %+v", result.Milestones[0])
	}
}

func TestBuildScopedRoadmapEpicRoot(t *testing.T) {
	cfg := config.Default()

	root := &item.Item{ID: "e1", Type: "epic", Title: "Auth", Status: "todo"}
	items := []*item.Item{
		root,
		{ID: "t1", Type: "task", Title: "Login", Status: "todo", Parent: "e1"},
		{ID: "f1", Type: "feature", Title: "SSO", Status: "todo", Parent: "e1"},
		{ID: "t2", Type: "task", Title: "OIDC", Status: "todo", Parent: "f1"},
		{ID: "m1", Type: "milestone", Title: "Other", Status: "todo"},
	}

	result := BuildScoped(cfg, items, false, root)

	if result.Root == nil || result.Root.Epic == nil {
		t.Fatalf("expected Root.Epic to be set, got %+v", result.Root)
	}
	if result.Root.Epic.Epic.ID != "e1" {
		t.Errorf("got epic %s, want e1", result.Root.Epic.Epic.ID)
	}
	if len(result.Root.Epic.Items) != 1 || result.Root.Epic.Items[0].ID != "t1" {
		t.Errorf("Root.Epic.Items = %v, want [t1]", result.Root.Epic.Items)
	}
	if len(result.Root.Epic.Features) != 1 || result.Root.Epic.Features[0].Feature.ID != "f1" {
		t.Fatalf("Root.Epic.Features = %+v, want feature f1", result.Root.Epic.Features)
	}
	if len(result.Milestones) != 0 {
		t.Errorf("expected no Milestones for epic scope, got %d", len(result.Milestones))
	}
}

func TestBuildScopedRoadmapFeatureRoot(t *testing.T) {
	cfg := config.Default()

	root := &item.Item{ID: "f1", Type: "feature", Title: "SSO", Status: "todo"}
	items := []*item.Item{
		root,
		{ID: "t1", Type: "task", Title: "OIDC", Status: "todo", Parent: "f1"},
	}

	result := BuildScoped(cfg, items, false, root)

	if result.Root == nil || result.Root.Feature == nil {
		t.Fatalf("expected Root.Feature to be set, got %+v", result.Root)
	}
	if result.Root.Feature.Feature.ID != "f1" {
		t.Errorf("got feature %s, want f1", result.Root.Feature.Feature.ID)
	}
	if len(result.Root.Feature.Items) != 1 || result.Root.Feature.Items[0].ID != "t1" {
		t.Errorf("Root.Feature.Items = %v, want [t1]", result.Root.Feature.Items)
	}
}

func TestBuildScopedRoadmapEpicRootWithNoVisibleChildren(t *testing.T) {
	cfg := config.Default()

	root := &item.Item{ID: "e1", Type: "epic", Title: "Empty epic", Status: "todo"}
	items := []*item.Item{root}

	result := BuildScoped(cfg, items, false, root)

	if result.Root == nil || result.Root.Epic == nil {
		t.Fatalf("expected Root.Epic to still be set for an empty epic, got %+v", result.Root)
	}
	if result.Root.Epic.Epic.ID != "e1" {
		t.Errorf("got epic %s, want e1", result.Root.Epic.Epic.ID)
	}
	if len(result.Root.Epic.Items) != 0 || len(result.Root.Epic.Features) != 0 {
		t.Errorf("expected an empty epic group, got %+v", result.Root.Epic)
	}
}

func TestBuildScopedRoadmapFeatureRootWithNoVisibleChildren(t *testing.T) {
	cfg := config.Default()

	root := &item.Item{ID: "f1", Type: "feature", Title: "Empty feature", Status: "todo"}
	items := []*item.Item{root}

	result := BuildScoped(cfg, items, false, root)

	if result.Root == nil || result.Root.Feature == nil {
		t.Fatalf("expected Root.Feature to still be set for an empty feature, got %+v", result.Root)
	}
	if result.Root.Feature.Feature.ID != "f1" {
		t.Errorf("got feature %s, want f1", result.Root.Feature.Feature.ID)
	}
	if len(result.Root.Feature.Items) != 0 {
		t.Errorf("expected an empty feature group, got %+v", result.Root.Feature)
	}
}

func TestValidateRoadmapRootType(t *testing.T) {
	cfg := config.Default()
	tests := []struct {
		itemType string
		wantErr  bool
	}{
		{"milestone", false},
		{"epic", false},
		{"feature", false},
		{"task", true},
		{"bug", true},
	}
	for _, tt := range tests {
		t.Run(tt.itemType, func(t *testing.T) {
			b := &item.Item{ID: "x1", Type: tt.itemType}
			err := ValidateRootType(cfg, b)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRoadmapRootType(%s) error = %v, wantErr %v", tt.itemType, err, tt.wantErr)
			}
		})
	}
}

// TestValidateRoadmapRootTypeWithNoContainerTypes covers a profile like
// "todo" that defines no container types at all: the old message rendered
// as "roadmap root must be one of , got task (x-1)" -- an empty list before
// "got" instead of a sensible explanation. The error must instead say the
// project has no container types, without touching the normal-case message
// TestRoadmapCmdRejectsNonContainerRootType depends on.
func TestValidateRoadmapRootTypeWithNoContainerTypes(t *testing.T) {
	leafRank := config.LeafRank
	cfg := &config.Config{TypesExclusive: true, Types: []config.TypeOverride{
		{Name: "task", Rank: &leafRank},
	}}

	err := ValidateRootType(cfg, &item.Item{ID: "x-1", Type: "task"})
	if err == nil {
		t.Fatal("expected an error when the project defines no container types")
	}
	if !strings.Contains(err.Error(), "no container types") {
		t.Errorf("expected error to say the project defines no container types, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "must be one of ,") {
		t.Errorf("error still renders the degenerate empty list, got %q", err.Error())
	}
}

func TestRoadmapPlacesAConfiguredRank2TypeInTheEpicSlot(t *testing.T) {
	rank := 2
	cfg := &config.Config{Types: []config.TypeOverride{{Name: "package", Rank: &rank}}}

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "p1", Type: "package", Status: "todo", Title: "A package", Parent: "m1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "A task", Parent: "p1"},
	}

	data := Build(cfg, items, false, nil, nil)

	if len(data.Milestones) != 1 {
		t.Fatalf("got %d milestone groups, want 1", len(data.Milestones))
	}
	group := data.Milestones[0]
	groupEpics := epicGroupsOf(group.Containers)
	if len(groupEpics) != 1 {
		t.Fatalf("got %d rank-2 groups, want 1 — a configured rank-2 type belongs in the epic slot", len(groupEpics))
	}
	if groupEpics[0].Epic.ID != "p1" {
		t.Errorf("rank-2 slot holds %q, want \"p1\"", groupEpics[0].Epic.ID)
	}
	if len(groupEpics[0].Items) != 1 {
		t.Errorf("got %d leaves under the package, want 1", len(groupEpics[0].Items))
	}
}

func TestRoadmapAcceptsAConfiguredRank1TypeAsScopeRoot(t *testing.T) {
	rank := 1
	cfg := &config.Config{Types: []config.TypeOverride{{Name: "release", Rank: &rank}}}

	root := &item.Item{ID: "r1", Type: "release", Status: "todo", Title: "v1"}
	if err := ValidateRootType(cfg, root); err != nil {
		t.Errorf("validateRoadmapRootType(release) = %v, want nil", err)
	}
}

func TestRoadmapRejectsALeafAsScopeRoot(t *testing.T) {
	cfg := &config.Config{}

	if err := ValidateRootType(cfg, &item.Item{ID: "t1", Type: "task"}); err == nil {
		t.Error("a leaf type must not be accepted as a roadmap root")
	}
}

func TestHiddenContainerAndItsSubtreeStayOutOfTheRoadmap(t *testing.T) {
	rank := 1
	visible := false
	cfg := &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank, Roadmap: &visible},
	}}

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Planned", Parent: "m1"},
		{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot"},
		{ID: "t2", Type: "task", Status: "todo", Title: "Someday", Parent: "b1"},
	}

	data := Build(cfg, items, false, nil, nil)

	for _, g := range data.Milestones {
		if g.Milestone.ID == "b1" {
			t.Fatal("a hidden container must not render as its own section")
		}
	}
	if data.Unscheduled != nil {
		for _, b := range data.Unscheduled.Other {
			if b.ID == "t2" {
				t.Fatal("a leaf under a hidden container must not resurface as unscheduled")
			}
		}
	}
	if len(data.Milestones) != 1 || data.Milestones[0].Milestone.ID != "m1" {
		t.Errorf("the visible milestone must still render")
	}
}

// TestHiddenContainerHidesEveryRankBeneathIt covers the full container depth
// the rank scheme allows: a hidden rank-1 bucket with a rank-2 child that
// itself has a rank-3 child with a leaf, plus a rank-3 child parented
// directly under the hidden rank-1 (skipping rank 2). A visible-typed
// container nested under a hidden one must vanish too -- markSubtree walks
// every descendant regardless of its own type -- and none of it may resurface
// in any of the unscheduled buckets.
func TestHiddenContainerHidesEveryRankBeneathIt(t *testing.T) {
	rank := 1
	visible := false
	cfg := &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank, Roadmap: &visible},
	}}

	items := []*item.Item{
		{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot"},
		{ID: "e1", Type: "epic", Status: "todo", Title: "Nested epic", Parent: "b1"},
		{ID: "f1", Type: "feature", Status: "todo", Title: "Nested feature", Parent: "e1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Deep leaf", Parent: "f1"},
		{ID: "f2", Type: "feature", Status: "todo", Title: "Direct feature", Parent: "b1"},
		{ID: "t2", Type: "task", Status: "todo", Title: "Leaf under direct feature", Parent: "f2"},
	}

	data := Build(cfg, items, false, nil, nil)

	if data.Unscheduled != nil {
		for _, eg := range epicGroupsOf(data.Unscheduled.Containers) {
			if eg.Epic.ID == "e1" {
				t.Fatal("a rank-2 container nested under a hidden rank-1 container must not resurface as unscheduled")
			}
		}
		for _, fg := range featureGroupsOf(data.Unscheduled.Containers) {
			if fg.Feature.ID == "f1" || fg.Feature.ID == "f2" {
				t.Fatalf("a rank-3 container under a hidden rank-1 container must not resurface as unscheduled, got %q", fg.Feature.ID)
			}
		}
		for _, b := range data.Unscheduled.Other {
			if b.ID == "t1" || b.ID == "t2" {
				t.Fatalf("a leaf at any depth under a hidden rank-1 container must not resurface as unscheduled, got %q", b.ID)
			}
		}
	}
	if len(data.Milestones) != 0 {
		t.Errorf("got %d milestone groups, want 0 — the hidden bucket is the only rank-1 bean", len(data.Milestones))
	}
}

// TestHiddenContainerNestedUnderVisibleParentVanishesEntirely covers Ruling 1
// of the fix round (D15): hiding removes, it does not reclassify. A
// hidden-type rank-2 container underneath a *visible* rank-1 milestone must
// vanish together with its whole subtree -- it must not render as one of the
// milestone's Epics, and its leaf must not fold up into the milestone's flat
// "Other" list either (that fold-up is exactly the "unassigned items"
// failure mode D15 names).
func TestHiddenContainerNestedUnderVisibleParentVanishesEntirely(t *testing.T) {
	rank := 2
	visible := false
	cfg := &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank, Roadmap: &visible},
	}}

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "t0", Type: "task", Status: "todo", Title: "Planned", Parent: "m1"},
		{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot", Parent: "m1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "b1"},
	}

	data := Build(cfg, items, false, nil, nil)

	if len(data.Milestones) != 1 || data.Milestones[0].Milestone.ID != "m1" {
		t.Fatalf("the visible milestone must still render, got %+v", data.Milestones)
	}
	group := data.Milestones[0]
	for _, eg := range epicGroupsOf(group.Containers) {
		if eg.Epic.ID == "b1" {
			t.Fatal("a hidden rank-2 container under a visible milestone must not render as one of its Epics")
		}
	}
	for _, o := range group.Other {
		if o.ID == "t1" || o.ID == "b1" {
			t.Fatalf("a hidden container's leaf must not fold up into the parent's flat Other list, got %q", o.ID)
		}
	}
	if data.Unscheduled != nil {
		for _, o := range data.Unscheduled.Other {
			if o.ID == "t1" || o.ID == "b1" {
				t.Fatalf("a hidden container's leaf must not resurface as unscheduled either, got %q", o.ID)
			}
		}
	}
}

// TestScopedRoadmapHidingRespectsScopeBoundary pins Ruling 2 (fix round 2)
// plus its round-3 correction: the scoped-by-ID bypass exempts exactly the
// named root, nothing more and nothing less.
//
//   - Naming a hidden container directly renders its full subtree (the
//     bypass): true for a rank-1 root (restores the coverage
//     TestScopedRoadmapBypassesHidingForTheNamedRoot carried before it was
//     narrowed to rank 2 only) and for a rank-2 root.
//   - Naming a visible container that sits BELOW a hidden ancestor also
//     renders its full subtree: a direct-by-ID lookup is not the aggregate
//     view the visibility flag governs, and an ancestor's opt-out must not
//     reach down past the scoped root. This is the fix-round-4 bug: seeding
//     hidden from an item whose ID merely differs from root reaches ancestors
//     of root too, and marks everything root and its descendants ancestrally
//     shared.
//   - Naming a visible container that CONTAINS a hidden container still
//     suppresses that inner hidden container and its subtree -- the bypass
//     is not total, D15 still applies to anything nested inside the scope.
//
// One fixture carries all four cases: v1 (hidden rank-1 vault) -> e2
// (visible epic) -> t2 (leaf), and m1 (visible milestone) -> b1 (hidden
// rank-2 bucket) -> t1 (leaf).
func TestScopedRoadmapHidingRespectsScopeBoundary(t *testing.T) {
	rank1, rank2 := 1, 2
	hidden := false
	cfg := &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank2, Roadmap: &hidden},
		{Name: "vault", Rank: &rank1, Roadmap: &hidden},
	}}

	m1 := &item.Item{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"}
	b1 := &item.Item{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot", Parent: "m1"}
	t1 := &item.Item{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "b1"}
	v1 := &item.Item{ID: "v1", Type: "vault", Status: "todo", Title: "Attic"}
	e2 := &item.Item{ID: "e2", Type: "epic", Status: "todo", Title: "Someday feature", Parent: "v1"}
	t2 := &item.Item{ID: "t2", Type: "task", Status: "todo", Title: "Someday task", Parent: "e2"}
	items := []*item.Item{m1, b1, t1, v1, e2, t2}

	t.Run("scoped to a visible container that contains a hidden one, the hidden subtree is suppressed", func(t *testing.T) {
		data := BuildScoped(cfg, items, false, m1)
		if len(data.Milestones) != 1 || data.Milestones[0].Milestone.ID != "m1" {
			t.Fatalf("expected the named root m1 to render, got %+v", data.Milestones)
		}
		group := data.Milestones[0]
		for _, eg := range epicGroupsOf(group.Containers) {
			if eg.Epic.ID == "b1" {
				t.Fatal("a hidden rank-2 container below the scoped root must not render as one of its Epics")
			}
		}
		for _, o := range group.Other {
			if o.ID == "b1" || o.ID == "t1" {
				t.Fatalf("a hidden container's subtree below the scoped root must not fold into Other, got %q", o.ID)
			}
		}
	})

	t.Run("scoped directly to a hidden rank-2 container, it renders in full", func(t *testing.T) {
		data := BuildScoped(cfg, items, false, b1)
		if data.Root == nil || data.Root.Epic == nil || data.Root.Epic.Epic.ID != "b1" {
			t.Fatalf("a scoped hidden root must still render as its own container, got %+v", data.Root)
		}
		found := false
		for _, o := range data.Root.Epic.Items {
			if o.ID == "t1" {
				found = true
			}
		}
		if !found {
			t.Error("a scoped hidden root must render its own full content -- the bypass covers the named root")
		}
	})

	t.Run("scoped directly to a hidden rank-1 container, it renders in full", func(t *testing.T) {
		data := BuildScoped(cfg, items, false, v1)
		if len(data.Milestones) != 1 || data.Milestones[0].Milestone.ID != "v1" {
			t.Fatalf("a scoped hidden rank-1 root must still render as its own container, got %+v", data.Milestones)
		}
		group := data.Milestones[0]
		milestoneEpics := epicGroupsOf(group.Containers)
		if len(milestoneEpics) != 1 || milestoneEpics[0].Epic.ID != "e2" {
			t.Fatalf("a scoped hidden rank-1 root must render its own visible child, got %+v", milestoneEpics)
		}
		found := false
		for _, item := range milestoneEpics[0].Items {
			if item.ID == "t2" {
				found = true
			}
		}
		if !found {
			t.Error("a scoped hidden rank-1 root must render its whole subtree -- the bypass covers the named root")
		}
	})

	t.Run("scoped to a visible container below a hidden ancestor, it renders in full", func(t *testing.T) {
		data := BuildScoped(cfg, items, false, e2)
		if data.Root == nil || data.Root.Epic == nil || data.Root.Epic.Epic.ID != "e2" {
			t.Fatalf("expected the named root e2 to render, got %+v", data.Root)
		}
		found := false
		for _, item := range data.Root.Epic.Items {
			if item.ID == "t2" {
				found = true
			}
		}
		if !found {
			t.Error("a visible root below a hidden ancestor must render its full subtree -- the ancestor's opt-out must not reach past the scoped root")
		}
	})
}

// TestHiddenRank3ContainerVanishesWithItsLeaf closes a coverage gap the
// reviewer flagged: rank 3, directly opting out (not via an ancestor), had
// no test of its own alongside the existing rank-1/rank-2 cases.
func TestHiddenRank3ContainerVanishesWithItsLeaf(t *testing.T) {
	rank := 3
	visible := false
	cfg := &config.Config{Types: []config.TypeOverride{
		{Name: "drawer", Rank: &rank, Roadmap: &visible},
	}}

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "e1", Type: "epic", Status: "todo", Title: "Auth", Parent: "m1"},
		{ID: "t2", Type: "task", Status: "todo", Title: "Planned", Parent: "e1"},
		{ID: "d1", Type: "drawer", Status: "todo", Title: "Hidden drawer", Parent: "e1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "d1"},
	}

	data := Build(cfg, items, false, nil, nil)

	if len(data.Milestones) != 1 {
		t.Fatalf("got %d milestone groups, want 1", len(data.Milestones))
	}
	group := data.Milestones[0]
	milestoneEpics := epicGroupsOf(group.Containers)
	if len(milestoneEpics) != 1 || milestoneEpics[0].Epic.ID != "e1" {
		t.Fatalf("expected epic e1 to still render (it has a visible sibling task), got %+v", milestoneEpics)
	}
	eg := milestoneEpics[0]
	for _, fg := range eg.Features {
		if fg.Feature.ID == "d1" {
			t.Fatal("a hidden rank-3 container must not render as one of its epic's Features")
		}
	}
	foundT1, foundT2 := false, false
	for _, item := range eg.Items {
		if item.ID == "t1" {
			foundT1 = true
		}
		if item.ID == "t2" {
			foundT2 = true
		}
	}
	if foundT1 {
		t.Error("a leaf under a hidden rank-3 container must not fold into the epic's Items")
	}
	if !foundT2 {
		t.Error("the epic's own visible leaf must still render")
	}
}

// TestScopedRoadmapRank3RootBypassesItsOwnHiddenType mirrors the rank-1/
// rank-2 scoped-bypass coverage for rank 3, per the reviewer's note.
func TestScopedRoadmapRank3RootBypassesItsOwnHiddenType(t *testing.T) {
	rank := 3
	visible := false
	cfg := &config.Config{Types: []config.TypeOverride{
		{Name: "drawer", Rank: &rank, Roadmap: &visible},
	}}

	root := &item.Item{ID: "d1", Type: "drawer", Status: "todo", Title: "Hidden drawer"}
	items := []*item.Item{
		root,
		{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "d1"},
	}

	data := BuildScoped(cfg, items, false, root)

	if data.Root == nil || data.Root.Feature == nil || data.Root.Feature.Feature.ID != "d1" {
		t.Fatalf("a scoped hidden rank-3 root must still render as its own container, got %+v", data.Root)
	}
	found := false
	for _, item := range data.Root.Feature.Items {
		if item.ID == "t1" {
			found = true
		}
	}
	if !found {
		t.Error("a scoped hidden rank-3 root must render its own full content")
	}
}

// TestChildrenIndexEmptyTree verifies that an empty input yields an
// empty index, and that looking up any ID on it returns nil (not a panic).
func TestChildrenIndexEmptyTree(t *testing.T) {
	idx := ChildrenIndex(nil)
	if len(idx) != 0 {
		t.Fatalf("expected empty index, got %d entries", len(idx))
	}
	if got := idx["missing"]; got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}
}

// TestChildrenIndexSingleLevel verifies that direct parent -> children
// relationships are indexed, and items without a parent are not indexed
// under any key.
func TestChildrenIndexSingleLevel(t *testing.T) {
	parent := &item.Item{ID: "beans-p1"}
	child1 := &item.Item{ID: "beans-c1", Parent: "beans-p1"}
	child2 := &item.Item{ID: "beans-c2", Parent: "beans-p1"}
	orphan := &item.Item{ID: "beans-o1"}

	idx := ChildrenIndex([]*item.Item{parent, child1, child2, orphan})

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
