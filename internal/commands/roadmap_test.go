package commands

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// epicGroupsOf extracts the epic-slot groups from a mixed container list, in
// order, for tests written against milestoneGroup/unscheduledGroup's former
// dedicated Epics field (beans-swy1).
func epicGroupsOf(containers []roadmapContainer) []epicGroup {
	var out []epicGroup
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
func featureGroupsOf(containers []roadmapContainer) []featureGroup {
	var out []featureGroup
	for _, c := range containers {
		if c.Feature != nil {
			out = append(out, *c.Feature)
		}
	}
	return out
}

func TestBuildRoadmap(t *testing.T) {
	// Save and restore global cfg
	oldCfg := cfg
	defer func() { cfg = oldCfg }()

	cfg = config.Default()

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
			result := buildRoadmap(tt.items, tt.includeDone, nil, nil)

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

func TestFirstParagraph(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "empty body",
			body: "",
			want: "",
		},
		{
			name: "single line",
			body: "This is a description.",
			want: "This is a description.",
		},
		{
			name: "multiple paragraphs",
			body: "First paragraph.\n\nSecond paragraph.",
			want: "First paragraph.",
		},
		{
			name: "multiline first paragraph",
			body: "Line one\nLine two\n\nSecond para.",
			want: "Line one Line two",
		},
		{
			name: "skips headers at start",
			body: "## Checklist\n- item one",
			want: "- item one",
		},
		{
			name: "truncates long text",
			body: "This is a very long paragraph that exceeds two hundred characters and needs to be truncated so it does not take up too much space in the roadmap output. Lorem ipsum dolor sit amet consectetur adipiscing elit.",
			want: "This is a very long paragraph that exceeds two hundred characters and needs to be truncated so it does not take up too much space in the roadmap output. Lorem ipsum dolor sit amet consectetur adipi...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstParagraph(tt.body)
			if got != tt.want {
				t.Errorf("firstParagraph() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderItemRef(t *testing.T) {
	tests := []struct {
		name       string
		item       *item.Item
		asLink     bool
		linkPrefix string
		want       string
	}{
		{
			name:   "no link - just ID",
			item:   &item.Item{ID: "abc", Path: "abc--milestone.md"},
			asLink: false,
			want:   "(abc)",
		},
		{
			name:       "link without prefix",
			item:       &item.Item{ID: "abc", Path: "abc--milestone.md"},
			asLink:     true,
			linkPrefix: "",
			want:       "([abc](abc--milestone.md))",
		},
		{
			name:       "link with prefix",
			item:       &item.Item{ID: "abc", Path: "abc--milestone.md"},
			asLink:     true,
			linkPrefix: "https://example.com/beans/",
			want:       "([abc](https://example.com/beans/abc--milestone.md))",
		},
		{
			name:       "link with prefix without trailing slash",
			item:       &item.Item{ID: "abc", Path: "abc--milestone.md"},
			asLink:     true,
			linkPrefix: ".beans",
			want:       "([abc](.beans/abc--milestone.md))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderItemRef(tt.item, tt.asLink, tt.linkPrefix)
			if got != tt.want {
				t.Errorf("renderBeanRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusFiltering(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()

	cfg = config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "Todo Milestone", Status: "todo", CreatedAt: &now},
		{ID: "m2", Type: "milestone", Title: "In Progress Milestone", Status: "in-progress", CreatedAt: &now},
		{ID: "t1", Type: "task", Title: "Task 1", Status: "todo", Parent: "m1"},
		{ID: "t2", Type: "task", Title: "Task 2", Status: "todo", Parent: "m2"},
	}

	t.Run("filter by status", func(t *testing.T) {
		result := buildRoadmap(items, false, []string{"todo"}, nil)
		if len(result.Milestones) != 1 {
			t.Errorf("expected 1 milestone, got %d", len(result.Milestones))
		}
		if result.Milestones[0].Milestone.Status != "todo" {
			t.Errorf("expected todo milestone, got %s", result.Milestones[0].Milestone.Status)
		}
	})

	t.Run("exclude by status", func(t *testing.T) {
		result := buildRoadmap(items, false, nil, []string{"in-progress"})
		if len(result.Milestones) != 1 {
			t.Errorf("expected 1 milestone, got %d", len(result.Milestones))
		}
		if result.Milestones[0].Milestone.Status != "todo" {
			t.Errorf("expected todo milestone, got %s", result.Milestones[0].Milestone.Status)
		}
	})
}

func TestMilestoneOrderRespectsManualOrderKey(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()

	cfg = config.Default()

	older := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	items := []*item.Item{
		// Created later but manually ordered first via the "order" key.
		{ID: "m2", Type: "milestone", Title: "v2.0", Status: "todo", Order: "a", CreatedAt: &newer},
		{ID: "t2", Type: "task", Title: "Task 2", Status: "todo", Parent: "m2"},
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", Order: "z", CreatedAt: &older},
		{ID: "t1", Type: "task", Title: "Task 1", Status: "todo", Parent: "m1"},
	}

	result := buildRoadmap(items, false, nil, nil)

	if len(result.Milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(result.Milestones))
	}
	if result.Milestones[0].Milestone.ID != "m2" || result.Milestones[1].Milestone.ID != "m1" {
		t.Errorf("milestone order = [%q, %q], want [m2, m1] (manual order overrides created_at)",
			result.Milestones[0].Milestone.ID, result.Milestones[1].Milestone.ID)
	}
}

func TestMilestoneOrderRespectsBlockingDependency(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()

	cfg = config.Default()

	// IDs deliberately sort the "wrong" way (aa- before zz-) so the ID
	// fallback alone cannot make this test pass -- only Blocking/BlockedBy
	// awareness can put the blocker first.
	items := []*item.Item{
		{ID: "aa-blocked", Type: "milestone", Title: "Blocked", Status: "todo", BlockedBy: []string{"zz-blocker"}},
		{ID: "t2", Type: "task", Title: "Task 2", Status: "todo", Parent: "aa-blocked"},
		{ID: "zz-blocker", Type: "milestone", Title: "Blocker", Status: "todo", Blocking: []string{"aa-blocked"}},
		{ID: "t1", Type: "task", Title: "Task 1", Status: "todo", Parent: "zz-blocker"},
	}

	result := buildRoadmap(items, false, nil, nil)

	if len(result.Milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(result.Milestones))
	}
	if result.Milestones[0].Milestone.ID != "zz-blocker" || result.Milestones[1].Milestone.ID != "aa-blocked" {
		t.Errorf("milestone order = [%q, %q], want [zz-blocker, aa-blocked] (blocker sorts first)",
			result.Milestones[0].Milestone.ID, result.Milestones[1].Milestone.ID)
	}
}

func TestSplitByContainerType(t *testing.T) {
	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "F1"},
		{ID: "t1", Type: "task", Title: "T1"},
		{ID: "b1", Type: "bug", Title: "B1"},
	}

	leafs, features := splitByContainerType(items)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

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
		got := collectLeafDescendants("feat1", children, false, nil)
		if len(got) != 2 {
			t.Fatalf("got %d leafs, want 2 (t1, t2)", len(got))
		}
		ids := map[string]bool{got[0].ID: true, got[1].ID: true}
		if !ids["t1"] || !ids["t2"] {
			t.Errorf("got ids %v, want t1 and t2", ids)
		}
	})

	t.Run("includes done when requested", func(t *testing.T) {
		got := collectLeafDescendants("feat1", children, true, nil)
		if len(got) != 3 {
			t.Fatalf("got %d leafs, want 3 (t1, t2, t3)", len(got))
		}
	})

	t.Run("no children returns empty, not nil panic", func(t *testing.T) {
		got := collectLeafDescendants("nonexistent", children, false, nil)
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
		got := collectLeafDescendants("featA", cyclic, false, nil)
		if len(got) != 1 || got[0].ID != "t1" {
			t.Errorf("got %v, want exactly [t1]", got)
		}
	})
}

func TestBuildMilestoneGroupResolvesFeatureNesting(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
		{ID: "f1", Type: "feature", Title: "SSO", Status: "todo", Parent: "e1"},
		{ID: "t1", Type: "task", Title: "OIDC login", Status: "todo", Parent: "f1"},
		{ID: "b1", Type: "bug", Title: "Direct epic bug", Status: "todo", Parent: "e1"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Standalone feature", Status: "todo"},
		{ID: "t1", Type: "task", Title: "Leaf under orphan feature", Status: "todo", Parent: "f1"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "e1", Type: "epic", Title: "Unscheduled epic", Status: "todo"},
		{ID: "f1", Type: "feature", Title: "Feature under unscheduled epic", Status: "todo", Parent: "e1"},
		{ID: "t1", Type: "task", Title: "Leaf", Status: "todo", Parent: "f1"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Top feature", Status: "todo"},
		{ID: "f2", Type: "feature", Title: "Nested feature", Status: "todo", Parent: "f1"},
		{ID: "t1", Type: "task", Title: "Leaf", Status: "todo", Parent: "f2"},
	}

	result := buildRoadmap(items, false, nil, nil)

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

// beans-n8zw D01: a Feature is a container IFF it has >=1 leaf descendant
// (respecting includeDone). Childless features must render as flat leaf
// lines instead of vanishing from the roadmap.

// Ta: orphan feature, 0 children, status todo -> flat leaf in
// Unscheduled.Other, never as a featureGroup, never dropped.
func TestOrphanChildlessFeatureAppearsAsFlatLeaf(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Lonely", Status: "todo"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
		{ID: "f1", Type: "feature", Title: "Lonely", Status: "todo", Parent: "e1"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "f1", Type: "feature", Title: "Lonely", Status: "todo", Parent: "m1"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	now := time.Now()
	items := []*item.Item{
		{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now},
		{ID: "e1", Type: "epic", Title: "Auth", Status: "todo", Parent: "m1"},
		{ID: "f1", Type: "feature", Title: "Childless", Status: "todo", Parent: "e1"},
		{ID: "f2", Type: "feature", Title: "Has a child", Status: "todo", Parent: "e1"},
		{ID: "t1", Type: "task", Title: "OIDC login", Status: "todo", Parent: "f2"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "f1", Type: "feature", Title: "Done and lonely", Status: "completed"},
	}

	result := buildRoadmap(items, false, nil, nil)

	if result.Unscheduled != nil {
		t.Errorf("expected Unscheduled to be nil (completed childless feature dropped), got %+v", result.Unscheduled)
	}
}

// beans-36fa: a childless orphan epic (type epic, status open, no parent)
// must render as a flat leaf row in the Unscheduled group, analogous to
// childless orphan features (beans-n8zw).
func TestOrphanChildlessEpicAppearsAsFlatLeaf(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "e1", Type: "epic", Title: "Lonely Epic", Status: "todo"},
	}

	result := buildRoadmap(items, false, nil, nil)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	items := []*item.Item{
		{ID: "beans-m1", Type: "milestone", Title: "v1.0", Status: "todo"},
		{ID: "beans-e1", Type: "epic", Title: "Cross-cutting alignment", Status: "todo", Parent: "beans-m1", Order: "2"},
		{ID: "beans-et1", Type: "task", Title: "Epic task", Status: "todo", Parent: "beans-e1"},
		{ID: "beans-f1", Type: "feature", Title: "Customer-facing feature", Status: "todo", Parent: "beans-m1", Order: "1"},
		{ID: "beans-ft1", Type: "task", Title: "Feature task", Status: "todo", Parent: "beans-f1"},
	}

	data := buildRoadmap(items, false, nil, nil)

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

	got := renderRoadmapMarkdown(data, false, "", false)
	f1Pos := strings.Index(got, "(beans-f1)")
	e1Pos := strings.Index(got, "(beans-e1)")
	if f1Pos == -1 || e1Pos == -1 {
		t.Fatalf("expected both beans-f1 and beans-e1 to render, got %q", got)
	}
	if f1Pos > e1Pos {
		t.Errorf("beans-f1 (order 1) rendered after beans-e1 (order 2): f1 at %d, e1 at %d\n%s", f1Pos, e1Pos, got)
	}
}

// --- T5: roadmapOutput TTY switch -------------------------------------------
//
// roadmapOutputFixture is a standalone roadmapData literal (T4 pattern) --
// NOT prettyFixture(), whose want-literal in TestRenderRoadmapPrettyAt80 is
// the frozen DESIGN.md block and must not be perturbed by an unrelated test.

func roadmapOutputFixture() *roadmapData {
	now := time.Now()
	m := &item.Item{ID: "beans-m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now, Path: "m1--v10.md"}
	e := &item.Item{ID: "beans-e1", Type: "epic", Title: "Auth", Status: "todo", Path: "e1--auth.md"}
	t1 := &item.Item{ID: "beans-t1", Type: "task", Title: "Login", Status: "todo", Priority: "high", Path: "t1--login.md"}
	return &roadmapData{
		Milestones: []milestoneGroup{
			{
				Milestone: m,
				Containers: []roadmapContainer{
					{Epic: &epicGroup{Epic: e, Items: []*item.Item{t1}}},
				},
			},
		},
	}
}

// TestRoadmapRowsNestItemsUnderTheirEpic asserts roadmapRows' Depth mapping
// directly, on a four-level fixture (milestone -> epic -> feature -> task).
// TestRoadmapCmdTreeViewRendersConnectors only checks that a "└─" connector
// appears somewhere in rendered output, which stays green even if nesting
// past the first level collapses -- e.g. the mutation
// `childDepth := depth + 1` -> `childDepth := depth` in
// appendRoadmapEpicGroup/appendRoadmapFeatureGroup flattens feature and task
// onto depth 1/2 instead of 2/3, and a connector is still drawn. Asserting
// Depth on roadmapRows' own output is the only thing that catches that.
func TestRoadmapRowsNestItemsUnderTheirEpic(t *testing.T) {
	now := time.Now()
	m := &item.Item{ID: "beans-m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now, Path: "m1--v10.md"}
	e := &item.Item{ID: "beans-e1", Type: "epic", Title: "Auth", Status: "todo", Path: "e1--auth.md"}
	f := &item.Item{ID: "beans-f1", Type: "feature", Title: "Login flow", Status: "todo", Path: "f1--login-flow.md"}
	task := &item.Item{ID: "beans-t1", Type: "task", Title: "Login", Status: "todo", Priority: "high", Path: "t1--login.md"}
	data := &roadmapData{
		Milestones: []milestoneGroup{
			{
				Milestone: m,
				Containers: []roadmapContainer{
					{Epic: &epicGroup{Epic: e, Features: []featureGroup{{Feature: f, Items: []*item.Item{task}}}}},
				},
			},
		},
	}

	rows := roadmapRows(data)

	want := []struct {
		id    string
		depth int
	}{
		{"beans-m1", 0},
		{"beans-e1", 1},
		{"beans-f1", 2},
		{"beans-t1", 3},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i, w := range want {
		if rows[i].Item == nil || rows[i].Item.ID != w.id {
			t.Errorf("row %d: bean = %v, want id %s", i, rows[i].Item, w.id)
		}
		if rows[i].Depth != w.depth {
			t.Errorf("row %d (%s): Depth = %d, want %d", i, w.id, rows[i].Depth, w.depth)
		}
	}
}

// SC-501: the TTY flag alone must decide which renderer roadmapOutput picks
// -- pipe gets the markdown artifact (badges + links), TTY gets the plain
// table (no badges, no markdown link syntax, glyph tree).
func TestRoadmapOutputSwitchesOnTTY(t *testing.T) {
	data := roadmapOutputFixture()

	t.Run("pipe (non-tty) renders markdown", func(t *testing.T) {
		got := roadmapOutput(data, false, roadmapFormatAuto, 0, true, "", false, ui.FormTree, config.Default())
		if !strings.HasPrefix(got, "# Roadmap") {
			t.Errorf("got prefix %q, want %q", got[:min(len(got), 20)], "# Roadmap")
		}
		if !strings.Contains(got, "img.shields.io") {
			t.Error("expected markdown output to contain an img.shields.io badge")
		}
	})

	t.Run("tty renders plain-text table", func(t *testing.T) {
		got := roadmapOutput(data, true, roadmapFormatAuto, 80, true, "", false, ui.FormTree, config.Default())
		if !strings.HasPrefix(got, "Roadmap") {
			t.Errorf("got prefix %q, want %q", got[:min(len(got), 20)], "Roadmap")
		}
		if strings.Contains(got, "img.shields.io") {
			t.Error("TTY output must not contain shields.io badges (EARS-4)")
		}
		if strings.Contains(got, "](") {
			t.Error("TTY output must not contain markdown link syntax (EARS-4)")
		}
		// Since beans-dbph Step B the milestone item itself is the heading
		// (a Row of type "milestone"), not a custom "■ Milestone" prefix --
		// its ID showing up proves the milestone row rendered.
		if !strings.Contains(got, "beans-m1") {
			t.Error("expected TTY output to contain the milestone row (beans-m1)")
		}
	})
}

// TestRoadmapOutputSwitchesOnTTYWithRoot verifies that roadmapOutput correctly
// dispatches Root-scoped data (from buildScopedRoadmap) to both renderers,
// not just the Milestones-based data tested in TestRoadmapOutputSwitchesOnTTY.
// This proves the dispatcher itself handles the Root case, alongside the
// renderer-specific tests (TestRenderRoadmapMarkdownRootEpic, etc.) that test
// each renderer in isolation.
func TestRoadmapOutputSwitchesOnTTYWithRoot(t *testing.T) {
	epic := &item.Item{ID: "beans-epic1", Type: "epic", Title: "Auth", Status: "todo", Path: "epic1--auth.md"}
	epicTask := &item.Item{ID: "beans-task1", Type: "task", Title: "Login", Status: "todo", Path: "task1--login.md"}
	data := &roadmapData{
		Root: &rootGroup{
			Epic: &epicGroup{Epic: epic, Items: []*item.Item{epicTask}},
		},
	}

	t.Run("pipe (non-tty) renders markdown with Root", func(t *testing.T) {
		got := roadmapOutput(data, false, roadmapFormatAuto, 0, true, "", false, ui.FormTree, config.Default())
		if !strings.HasPrefix(got, "# Roadmap") {
			t.Errorf("got prefix %q, want %q", got[:min(len(got), 20)], "# Roadmap")
		}
		if !strings.Contains(got, "### Epic:") {
			t.Errorf("expected epic heading (### Epic:), got %q", got)
		}
		if !strings.Contains(got, "img.shields.io") {
			t.Error("expected markdown output to contain an img.shields.io badge")
		}
		if !strings.Contains(got, "Auth") {
			t.Errorf("expected epic title Auth in output, got %q", got)
		}
		if !strings.Contains(got, "Login") {
			t.Errorf("expected item title Login in output, got %q", got)
		}
	})

	t.Run("tty renders plain-text table with Root", func(t *testing.T) {
		got := roadmapOutput(data, true, roadmapFormatAuto, 80, true, "", false, ui.FormTree, config.Default())
		if !strings.HasPrefix(got, "Roadmap") {
			t.Errorf("got prefix %q, want %q", got[:min(len(got), 20)], "Roadmap")
		}
		// Since beans-dbph Step B the epic item itself is the heading (a Row
		// of type "epic"), not a custom "▸ Epic" prefix -- its ID showing up
		// proves the epic row rendered.
		if !strings.Contains(got, "beans-epic1") {
			t.Errorf("expected epic row (beans-epic1), got %q", got)
		}
		if strings.Contains(got, "img.shields.io") {
			t.Error("TTY output must not contain shields.io badges")
		}
		if strings.Contains(got, "](") {
			t.Error("TTY output must not contain markdown link syntax")
		}
		if strings.Contains(got, "No Milestone") {
			t.Error("root-scoped output must not contain the unscheduled-bucket heading")
		}
		if !strings.Contains(got, "Auth") {
			t.Errorf("expected epic title Auth in output, got %q", got)
		}
		if !strings.Contains(got, "Login") {
			t.Errorf("expected item title Login in output, got %q", got)
		}
	})
}

// SC-502 / EARS-5: the non-TTY path of roadmapOutput must be a pure pass
// -through to renderRoadmapMarkdown -- character-for-character, with the
// exact same arguments. This is the regression guard for the Q07/D02
// byte-identity constraint at the function level.
// Table-driven over both `links` values (reviewer B01): a prior version of
// this test only ever passed links=true, so a mutation hardcoding the
// markdown branch's `links` argument to `true` left the whole suite green --
// the `false` direction was never exercised. `--no-links` is a real user
// flag (roadmapNoLinks, roadmap.go); if roadmapOutput ever stopped
// forwarding it, the piped/redirected markdown output would silently ignore
// it. Both directions must be observed for the wiring itself to be pinned.
func TestRoadmapMarkdownByteIdentical(t *testing.T) {
	data := roadmapOutputFixture()

	tests := []struct {
		name  string
		links bool
	}{
		{name: "links enabled", links: true},
		{name: "links disabled (--no-links)", links: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := roadmapOutput(data, false, roadmapFormatAuto, 0, tt.links, "some/prefix", false, ui.FormTree, config.Default())
			want := renderRoadmapMarkdown(data, tt.links, "some/prefix", false)
			if got != want {
				t.Errorf("roadmapOutput(non-tty, links=%v, false) diverged from renderRoadmapMarkdown:\ngot:  %q\nwant: %q", tt.links, got, want)
			}
			// links=false must actually change the rendered output relative
			// to links=true for this fixture (has a milestone with an item
			// ref) -- otherwise the two subtests could both pass vacuously
			// against a `links` argument that was never wired through at all.
			withLinks := renderRoadmapMarkdown(data, true, "some/prefix", false)
			withoutLinks := renderRoadmapMarkdown(data, false, "some/prefix", false)
			if withLinks == withoutLinks {
				t.Fatal("fixture invariant broken: renderRoadmapMarkdown output identical for links=true/false -- test cannot distinguish the two branches")
			}
		})
	}
}

func TestBuildScopedRoadmapMilestoneRoot(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

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
	result := buildScopedRoadmap(items, false, root)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	now := time.Now()
	root := &item.Item{ID: "m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now}
	items := []*item.Item{root}

	result := buildScopedRoadmap(items, false, root)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	root := &item.Item{ID: "e1", Type: "epic", Title: "Auth", Status: "todo"}
	items := []*item.Item{
		root,
		{ID: "t1", Type: "task", Title: "Login", Status: "todo", Parent: "e1"},
		{ID: "f1", Type: "feature", Title: "SSO", Status: "todo", Parent: "e1"},
		{ID: "t2", Type: "task", Title: "OIDC", Status: "todo", Parent: "f1"},
		{ID: "m1", Type: "milestone", Title: "Other", Status: "todo"},
	}

	result := buildScopedRoadmap(items, false, root)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	root := &item.Item{ID: "f1", Type: "feature", Title: "SSO", Status: "todo"}
	items := []*item.Item{
		root,
		{ID: "t1", Type: "task", Title: "OIDC", Status: "todo", Parent: "f1"},
	}

	result := buildScopedRoadmap(items, false, root)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	root := &item.Item{ID: "e1", Type: "epic", Title: "Empty epic", Status: "todo"}
	items := []*item.Item{root}

	result := buildScopedRoadmap(items, false, root)

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
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	root := &item.Item{ID: "f1", Type: "feature", Title: "Empty feature", Status: "todo"}
	items := []*item.Item{root}

	result := buildScopedRoadmap(items, false, root)

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
			err := validateRoadmapRootType(b)
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
	prev := cfg
	cfg = &config.Config{TypesExclusive: true, Types: []config.TypeOverride{
		{Name: "task", Rank: &leafRank},
	}}
	defer func() { cfg = prev }()

	err := validateRoadmapRootType(&item.Item{ID: "x-1", Type: "task"})
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

func TestRenderRoadmapMarkdownRootEpic(t *testing.T) {
	e := &item.Item{ID: "beans-e1", Type: "epic", Title: "Auth", Status: "todo", Path: "e1--auth.md"}
	item1 := &item.Item{ID: "beans-t1", Type: "task", Title: "Login", Status: "todo", Path: "t1--login.md"}
	data := &roadmapData{
		Root: &rootGroup{
			Epic: &epicGroup{Epic: e, Items: []*item.Item{item1}},
		},
	}

	got := renderRoadmapMarkdown(data, true, "", false)

	if !strings.HasPrefix(got, "# Roadmap") {
		t.Errorf("got prefix %q, want %q", got[:min(len(got), 20)], "# Roadmap")
	}
	if !strings.Contains(got, "### Epic: Auth") {
		t.Errorf("expected epic heading, got %q", got)
	}
	if !strings.Contains(got, "Login") {
		t.Errorf("expected item Login to be rendered, got %q", got)
	}
	if strings.Contains(got, "## Milestone") {
		t.Errorf("root-scoped output must not contain a Milestone heading, got %q", got)
	}
	if strings.Contains(got, "Unscheduled") {
		t.Errorf("root-scoped output must not contain the Unscheduled heading, got %q", got)
	}
}

func TestRenderRoadmapMarkdownRootFeature(t *testing.T) {
	f := &item.Item{ID: "beans-f1", Type: "feature", Title: "SSO", Status: "todo", Path: "f1--sso.md"}
	item1 := &item.Item{ID: "beans-t1", Type: "task", Title: "OIDC", Status: "todo", Path: "t1--oidc.md"}
	data := &roadmapData{
		Root: &rootGroup{
			Feature: &featureGroup{Feature: f, Items: []*item.Item{item1}},
		},
	}

	got := renderRoadmapMarkdown(data, true, "", false)

	if !strings.Contains(got, "#### Feature: SSO") {
		t.Errorf("expected feature heading, got %q", got)
	}
	if !strings.Contains(got, "OIDC") {
		t.Errorf("expected item OIDC to be rendered, got %q", got)
	}
}

func TestRoadmapPlacesAConfiguredRank2TypeInTheEpicSlot(t *testing.T) {
	rank := 2
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{{Name: "package", Rank: &rank}}}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "p1", Type: "package", Status: "todo", Title: "A package", Parent: "m1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "A task", Parent: "p1"},
	}

	data := buildRoadmap(items, false, nil, nil)

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
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{{Name: "release", Rank: &rank}}}
	defer func() { cfg = prev }()

	root := &item.Item{ID: "r1", Type: "release", Status: "todo", Title: "v1"}
	if err := validateRoadmapRootType(root); err != nil {
		t.Errorf("validateRoadmapRootType(release) = %v, want nil", err)
	}
}

func TestRoadmapRejectsALeafAsScopeRoot(t *testing.T) {
	prev := cfg
	cfg = &config.Config{}
	defer func() { cfg = prev }()

	if err := validateRoadmapRootType(&item.Item{ID: "t1", Type: "task"}); err == nil {
		t.Error("a leaf type must not be accepted as a roadmap root")
	}
}

func TestHiddenContainerAndItsSubtreeStayOutOfTheRoadmap(t *testing.T) {
	rank := 1
	visible := false
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank, Roadmap: &visible},
	}}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Planned", Parent: "m1"},
		{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot"},
		{ID: "t2", Type: "task", Status: "todo", Title: "Someday", Parent: "b1"},
	}

	data := buildRoadmap(items, false, nil, nil)

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
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank, Roadmap: &visible},
	}}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot"},
		{ID: "e1", Type: "epic", Status: "todo", Title: "Nested epic", Parent: "b1"},
		{ID: "f1", Type: "feature", Status: "todo", Title: "Nested feature", Parent: "e1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Deep leaf", Parent: "f1"},
		{ID: "f2", Type: "feature", Status: "todo", Title: "Direct feature", Parent: "b1"},
		{ID: "t2", Type: "task", Status: "todo", Title: "Leaf under direct feature", Parent: "f2"},
	}

	data := buildRoadmap(items, false, nil, nil)

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
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank, Roadmap: &visible},
	}}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "t0", Type: "task", Status: "todo", Title: "Planned", Parent: "m1"},
		{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot", Parent: "m1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "b1"},
	}

	data := buildRoadmap(items, false, nil, nil)

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
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "bucket", Rank: &rank2, Roadmap: &hidden},
		{Name: "vault", Rank: &rank1, Roadmap: &hidden},
	}}
	defer func() { cfg = prev }()

	m1 := &item.Item{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"}
	b1 := &item.Item{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot", Parent: "m1"}
	t1 := &item.Item{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "b1"}
	v1 := &item.Item{ID: "v1", Type: "vault", Status: "todo", Title: "Attic"}
	e2 := &item.Item{ID: "e2", Type: "epic", Status: "todo", Title: "Someday feature", Parent: "v1"}
	t2 := &item.Item{ID: "t2", Type: "task", Status: "todo", Title: "Someday task", Parent: "e2"}
	items := []*item.Item{m1, b1, t1, v1, e2, t2}

	t.Run("scoped to a visible container that contains a hidden one, the hidden subtree is suppressed", func(t *testing.T) {
		data := buildScopedRoadmap(items, false, m1)
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
		data := buildScopedRoadmap(items, false, b1)
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
		data := buildScopedRoadmap(items, false, v1)
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
		data := buildScopedRoadmap(items, false, e2)
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
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "drawer", Rank: &rank, Roadmap: &visible},
	}}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo", Title: "Release"},
		{ID: "e1", Type: "epic", Status: "todo", Title: "Auth", Parent: "m1"},
		{ID: "t2", Type: "task", Status: "todo", Title: "Planned", Parent: "e1"},
		{ID: "d1", Type: "drawer", Status: "todo", Title: "Hidden drawer", Parent: "e1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "d1"},
	}

	data := buildRoadmap(items, false, nil, nil)

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
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "drawer", Rank: &rank, Roadmap: &visible},
	}}
	defer func() { cfg = prev }()

	root := &item.Item{ID: "d1", Type: "drawer", Status: "todo", Title: "Hidden drawer"}
	items := []*item.Item{
		root,
		{ID: "t1", Type: "task", Status: "todo", Title: "Someday", Parent: "d1"},
	}

	data := buildScopedRoadmap(items, false, root)

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

func TestTypeBadgeUsesTheConfiguredColour(t *testing.T) {
	rank := 2
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "chore", Rank: &rank, Color: "peach"},
	}}
	defer func() { cfg = prev }()

	got := typeBadge(&item.Item{ID: "c1", Type: "chore"})

	if strings.Contains(got, "gray") {
		t.Errorf("badge fell back to gray: %s", got)
	}
	if !strings.Contains(got, "chore-") {
		t.Errorf("badge does not name the type: %s", got)
	}
}

func TestTypeBadgeStaysEmptyForATypelessItem(t *testing.T) {
	if got := typeBadge(&item.Item{ID: "x1"}); got != "" {
		t.Errorf("typeBadge() = %q, want empty", got)
	}
}

// badgeURLPattern pins the exact shape shields.io needs: a bare 6-digit hex
// (no leading '#') or the literal fallback "gray" after the type-dash. A
// stray '#' would make the URL fail to match and thus fail the test, where
// the badge itself would just silently not render.
var badgeURLPattern = regexp.MustCompile(`^!\[([a-z]+)\]\(https://img\.shields\.io/badge/([a-z]+)-([0-9a-f]{6}|gray)\?style=flat-square\)$`)

func TestTypeBadgeURLOmitsTheLeadingHash(t *testing.T) {
	rank := 2
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "chore", Rank: &rank, Color: "peach"},
	}}
	defer func() { cfg = prev }()

	got := typeBadge(&item.Item{ID: "c1", Type: "chore"})

	m := badgeURLPattern.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("typeBadge() = %q, does not match expected badge URL shape %s", got, badgeURLPattern)
	}
	if m[3] == "gray" {
		t.Errorf("typeBadge() = %q, resolved colour fell back to gray", got)
	}
	if strings.Contains(got, "#") {
		t.Errorf("typeBadge() = %q, badge URL carries a stray '#'", got)
	}
}

func TestTypeBadgeFallsBackToGrayForATypeWithNoColour(t *testing.T) {
	rank := 4
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "task", Rank: &rank},
	}}
	defer func() { cfg = prev }()

	got := typeBadge(&item.Item{ID: "t1", Type: "task"})

	want := "![task](https://img.shields.io/badge/task-gray?style=flat-square)"
	if got != want {
		t.Errorf("typeBadge() = %q, want %q", got, want)
	}
}

func TestTypeBadgeFallsBackToGrayForAnUnknownType(t *testing.T) {
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{}}
	defer func() { cfg = prev }()

	got := typeBadge(&item.Item{ID: "u1", Type: "mystery"})

	want := "![mystery](https://img.shields.io/badge/mystery-gray?style=flat-square)"
	if got != want {
		t.Errorf("typeBadge() = %q, want %q", got, want)
	}
}

func TestTypeBadgeAcceptsARawHexColour(t *testing.T) {
	rank := 2
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "chore", Rank: &rank, Color: "#123abc"},
	}}
	defer func() { cfg = prev }()

	got := typeBadge(&item.Item{ID: "c1", Type: "chore"})

	want := "![chore](https://img.shields.io/badge/chore-123abc?style=flat-square)"
	if got != want {
		t.Errorf("typeBadge() = %q, want %q", got, want)
	}
}

// --- Heading labels follow the item's own type (beans-0mvv fix wave 2) -----
//
// Task 4 moved the roadmap's container *selection* to rank but left the
// *labels* hardcoded to "Milestone"/"Epic"/"Feature" in both renderers. On a
// non-classic profile every rank-1 container rendered as "Milestone" and
// every rank-2 container as "Epic", regardless of its configured type name.

// TestTypeHeadingLabel pins typeHeadingLabel in isolation: first-letter
// upper-case as a pure display transform (D05 keeps the stored/configured
// name lowercase), plus the empty-type fallback. A mutation that returns the
// name unchanged fails the "chore"/"a" cases; a mutation that drops the
// empty-string fallback fails the "" case.
func TestTypeHeadingLabel(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"milestone", "Milestone"},
		{"chore", "Chore"},
		{"a", "A"},
		{"", "Untyped"},
	}
	for _, tt := range tests {
		t.Run("\""+tt.name+"\"", func(t *testing.T) {
			if got := typeHeadingLabel(tt.name); got != tt.want {
				t.Errorf("typeHeadingLabel(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestRoadmapMarkdownHeadingsUseTheItemsOwnType is the direct regression
// test for the defect: on a project whose rank-1 type is "release" and
// whose rank-2 type is "chore" (the "complex" profile shape), the roadmap
// heading must say "Release"/"Chore", not the classic-profile words. A
// reversion to the hardcoded "## Milestone:"/"### Epic:" text keeps the
// item data and structure identical but changes only the literal template
// text, so it would leave every other test in this file green while
// failing the "contains Release/Chore" and "does not contain Milestone/
// Epic" assertions below -- exactly the defect reported against a real
// store ("Parking lot" rendering as "## Milestone: Parking lot").
func TestRoadmapMarkdownHeadingsUseTheItemsOwnType(t *testing.T) {
	rank1, rank2 := 1, 2
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "release", Rank: &rank1},
		{Name: "chore", Rank: &rank2},
	}}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "r1", Type: "release", Status: "todo", Title: "v2.0"},
		{ID: "c1", Type: "chore", Status: "todo", Title: "Tidy the store", Parent: "r1"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Sweep", Parent: "c1"},
	}
	data := buildRoadmap(items, false, nil, nil)
	got := renderRoadmapMarkdown(data, false, "", false)

	if !strings.Contains(got, "## Release: v2.0") {
		t.Errorf("expected the rank-1 heading to name the bean's own type (Release), got %q", got)
	}
	if !strings.Contains(got, "### Chore: Tidy the store") {
		t.Errorf("expected the rank-2 heading to name the bean's own type (Chore), got %q", got)
	}
	if strings.Contains(got, "Milestone") || strings.Contains(got, "Epic") {
		t.Errorf("heading must not fall back to the classic-profile words when they are not configured, got %q", got)
	}
}

// TestRoadmapMarkdownHeadingForUnknownType covers an item whose type the
// config no longer knows (surfaced by `lif check`'s unknownTypeItems): the
// heading must still show the item's own type name, capitalised, rather than
// an empty or nonsensical heading. This bypasses buildRoadmap (an
// unknown-typed item is never classified onto a container rank by RankOf)
// and drives renderRoadmapMarkdown directly with a hand-built rootGroup, the
// same way TestRenderRoadmapMarkdownRootEpic does, since the heading
// renderer itself must tolerate whatever string reaches it.
func TestRoadmapMarkdownHeadingForUnknownType(t *testing.T) {
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{}}
	defer func() { cfg = prev }()

	e := &item.Item{ID: "beans-legacy1", Type: "legacy", Title: "Old container", Status: "todo"}
	data := &roadmapData{Root: &rootGroup{Epic: &epicGroup{Epic: e}}}

	got := renderRoadmapMarkdown(data, false, "", false)

	if !strings.Contains(got, "### Legacy: Old container") {
		t.Errorf("expected the heading to capitalise the bean's own unknown type, got %q", got)
	}
}

// TestRoadmapMarkdownHeadingForEmptyType covers an empty type string
// reaching the heading renderer directly (defensive: RankOf("") resolves to
// LeafRank via cfg.GetType, so buildRoadmap itself never classifies an
// empty-typed item onto a container rank -- but a hand-built roadmapData, or
// a future caller, could still hand the renderer one). "Untyped" must
// appear rather than an empty "### : Title" heading.
func TestRoadmapMarkdownHeadingForEmptyType(t *testing.T) {
	f := &item.Item{ID: "beans-notype1", Type: "", Title: "Mystery container", Status: "todo"}
	data := &roadmapData{Root: &rootGroup{Feature: &featureGroup{Feature: f}}}

	got := renderRoadmapMarkdown(data, false, "", false)

	if !strings.Contains(got, "#### Untyped: Mystery container") {
		t.Errorf("expected the empty-type fallback heading, got %q", got)
	}
	if strings.Contains(got, "#### : ") {
		t.Errorf("must not render an empty type label, got %q", got)
	}
}

// TestRoadmapMarkdownUnscheduledHeadingSaysUnscheduled pins the wording
// chosen for the orphan section (formerly the classic-profile-specific
// "No Milestone"): "Unscheduled" says something true whether the project's
// rank-1 type is named "milestone", is one of several rank-1 types, or does
// not exist at all (the "todo" profile). A reversion to the literal
// "No Milestone" string would fail this on any of those projects.
func TestRoadmapMarkdownUnscheduledHeadingSaysUnscheduled(t *testing.T) {
	m := &item.Item{ID: "m1", Type: "milestone", Status: "todo", Title: "v1"}
	orphanTask := &item.Item{ID: "t1", Type: "task", Status: "todo", Title: "Loose end"}
	data := &roadmapData{
		Milestones:  []milestoneGroup{{Milestone: m, Other: []*item.Item{}}},
		Unscheduled: &unscheduledGroup{Other: []*item.Item{orphanTask}},
	}
	got := renderRoadmapMarkdown(data, false, "", false)

	if !strings.Contains(got, "## Unscheduled") {
		t.Errorf("expected the orphan section heading to say Unscheduled, got %q", got)
	}
	if strings.Contains(got, "No Milestone") {
		t.Errorf("orphan section heading must not hardcode the classic-profile word, got %q", got)
	}
}

// TestRoadmapMarkdownBlockSeparation pins the exact blank-line structure of
// the rendered Markdown: every heading/body/item block is separated from
// its neighbours by exactly one blank line, and the document ends in
// exactly one trailing newline. All other Markdown assertions in this file
// use strings.Contains/HasPrefix and would silently pass through a
// whitespace regression (the missing-blank-line-before-"#### Feature:" and
// missing-blank-line-before-"## Unscheduled" defects this test guards
// against were both invisible to those assertions).
func TestRoadmapMarkdownBlockSeparation(t *testing.T) {
	prev := cfg
	cfg = config.Default()
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "t-m1", Type: "milestone", Status: "todo", Title: "Milestone One", Body: "M body"},
		{ID: "t-e1", Type: "epic", Status: "todo", Title: "Epic One", Body: "E body", Parent: "t-m1"},
		{ID: "t-l1", Type: "task", Status: "todo", Title: "Direct leaf", Parent: "t-e1"},
		{ID: "t-f1", Type: "feature", Status: "todo", Title: "Feature One", Body: "F body", Parent: "t-e1"},
		{ID: "t-l2", Type: "task", Status: "todo", Title: "Nested leaf", Parent: "t-f1"},
		{ID: "t-of", Type: "feature", Status: "todo", Title: "Orphan Feature", Parent: ""},
		{ID: "t-l3", Type: "task", Status: "todo", Title: "Orphan nested leaf", Parent: "t-of"},
	}
	data := buildRoadmap(items, false, nil, nil)
	got := renderRoadmapMarkdown(data, false, "", false)

	const taskBadge = "![task](https://img.shields.io/badge/task-gray?style=flat-square)"
	want := "# Roadmap\n" +
		"\n" +
		"## Milestone: Milestone One (t-m1)\n" +
		"\n" +
		"> M body\n" +
		"\n" +
		"### Epic: Epic One (t-e1)\n" +
		"\n" +
		"> E body\n" +
		"\n" +
		"- " + taskBadge + " Direct leaf (t-l1)\n" +
		"\n" +
		"#### Feature: Feature One (t-f1)\n" +
		"\n" +
		"> F body\n" +
		"\n" +
		"- " + taskBadge + " Nested leaf (t-l2)\n" +
		"\n" +
		"## Unscheduled\n" +
		"\n" +
		"#### Feature: Orphan Feature (t-of)\n" +
		"\n" +
		"- " + taskBadge + " Orphan nested leaf (t-l3)\n"

	if got != want {
		t.Errorf("roadmap markdown block separation mismatch:\ngot:  %q\nwant: %q", got, want)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("expected no run of 2+ blank lines, got %q", got)
	}
}
