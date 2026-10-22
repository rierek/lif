package commands

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
)

// epicGroupsOf extracts the epic-slot groups from a mixed container list, in
// order, for tests written against milestoneGroup/unscheduledGroup's former
// dedicated Epics field (beans-swy1).
func epicGroupsOf(containers []roadmap.Container) []roadmap.EpicGroup {
	var out []roadmap.EpicGroup
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
func featureGroupsOf(containers []roadmap.Container) []roadmap.FeatureGroup {
	var out []roadmap.FeatureGroup
	for _, c := range containers {
		if c.Feature != nil {
			out = append(out, *c.Feature)
		}
	}
	return out
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

// beans-n8zw D01: a Feature is a container IFF it has >=1 leaf descendant
// (respecting includeDone). Childless features must render as flat leaf
// lines instead of vanishing from the roadmap.

// --- T5: roadmapOutput TTY switch -------------------------------------------
//
// roadmapOutputFixture is a standalone roadmap.Data literal (T4 pattern) --
// NOT prettyFixture(), whose want-literal in TestRenderRoadmapPrettyAt80 is
// the frozen DESIGN.md block and must not be perturbed by an unrelated test.

func roadmapOutputFixture() *roadmap.Data {
	now := time.Now()
	m := &item.Item{ID: "beans-m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now, Path: "m1--v10.md"}
	e := &item.Item{ID: "beans-e1", Type: "epic", Title: "Auth", Status: "todo", Path: "e1--auth.md"}
	t1 := &item.Item{ID: "beans-t1", Type: "task", Title: "Login", Status: "todo", Priority: "high", Path: "t1--login.md"}
	return &roadmap.Data{
		Milestones: []roadmap.MilestoneGroup{
			{
				Milestone: m,
				Containers: []roadmap.Container{
					{Epic: &roadmap.EpicGroup{Epic: e, Items: []*item.Item{t1}}},
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
	data := &roadmap.Data{
		Milestones: []roadmap.MilestoneGroup{
			{
				Milestone: m,
				Containers: []roadmap.Container{
					{Epic: &roadmap.EpicGroup{Epic: e, Features: []roadmap.FeatureGroup{{Feature: f, Items: []*item.Item{task}}}}},
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
	data := &roadmap.Data{
		Root: &roadmap.RootGroup{
			Epic: &roadmap.EpicGroup{Epic: epic, Items: []*item.Item{epicTask}},
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

func TestRenderRoadmapMarkdownRootEpic(t *testing.T) {
	e := &item.Item{ID: "beans-e1", Type: "epic", Title: "Auth", Status: "todo", Path: "e1--auth.md"}
	item1 := &item.Item{ID: "beans-t1", Type: "task", Title: "Login", Status: "todo", Path: "t1--login.md"}
	data := &roadmap.Data{
		Root: &roadmap.RootGroup{
			Epic: &roadmap.EpicGroup{Epic: e, Items: []*item.Item{item1}},
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
	data := &roadmap.Data{
		Root: &roadmap.RootGroup{
			Feature: &roadmap.FeatureGroup{Feature: f, Items: []*item.Item{item1}},
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
	data := roadmap.Build(cfg, items, false, nil, nil)
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
// an empty or nonsensical heading. This bypasses roadmap.Build (an
// unknown-typed item is never classified onto a container rank by RankOf)
// and drives renderRoadmapMarkdown directly with a hand-built rootGroup, the
// same way TestRenderRoadmapMarkdownRootEpic does, since the heading
// renderer itself must tolerate whatever string reaches it.
func TestRoadmapMarkdownHeadingForUnknownType(t *testing.T) {
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{}}
	defer func() { cfg = prev }()

	e := &item.Item{ID: "beans-legacy1", Type: "legacy", Title: "Old container", Status: "todo"}
	data := &roadmap.Data{Root: &roadmap.RootGroup{Epic: &roadmap.EpicGroup{Epic: e}}}

	got := renderRoadmapMarkdown(data, false, "", false)

	if !strings.Contains(got, "### Legacy: Old container") {
		t.Errorf("expected the heading to capitalise the bean's own unknown type, got %q", got)
	}
}

// TestRoadmapMarkdownHeadingForEmptyType covers an empty type string
// reaching the heading renderer directly (defensive: RankOf("") resolves to
// LeafRank via cfg.GetType, so roadmap.Build itself never classifies an
// empty-typed item onto a container rank -- but a hand-built roadmap.Data, or
// a future caller, could still hand the renderer one). "Untyped" must
// appear rather than an empty "### : Title" heading.
func TestRoadmapMarkdownHeadingForEmptyType(t *testing.T) {
	f := &item.Item{ID: "beans-notype1", Type: "", Title: "Mystery container", Status: "todo"}
	data := &roadmap.Data{Root: &roadmap.RootGroup{Feature: &roadmap.FeatureGroup{Feature: f}}}

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
	data := &roadmap.Data{
		Milestones:  []roadmap.MilestoneGroup{{Milestone: m, Other: []*item.Item{}}},
		Unscheduled: &roadmap.UnscheduledGroup{Other: []*item.Item{orphanTask}},
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
	data := roadmap.Build(cfg, items, false, nil, nil)
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

// A feature with a lower order renders before an epic with a higher order
// under the same milestone: the Markdown follows the builder's one
// order-comparable container sequence (beans-swy1).
func TestRoadmapMarkdownInterleavesEpicAndFeatureByOrder(t *testing.T) {
	items := []*item.Item{
		{ID: "beans-m1", Type: "milestone", Title: "v1.0", Status: "todo"},
		{ID: "beans-e1", Type: "epic", Title: "Cross-cutting alignment", Status: "todo", Parent: "beans-m1", Order: "2"},
		{ID: "beans-et1", Type: "task", Title: "Epic task", Status: "todo", Parent: "beans-e1"},
		{ID: "beans-f1", Type: "feature", Title: "Customer-facing feature", Status: "todo", Parent: "beans-m1", Order: "1"},
		{ID: "beans-ft1", Type: "task", Title: "Feature task", Status: "todo", Parent: "beans-f1"},
	}

	got := renderRoadmapMarkdown(roadmap.Build(cfg, items, false, nil, nil), false, "", false)
	f1Pos := strings.Index(got, "(beans-f1)")
	e1Pos := strings.Index(got, "(beans-e1)")
	if f1Pos == -1 || e1Pos == -1 {
		t.Fatalf("expected both beans-f1 and beans-e1 to render, got %q", got)
	}
	if f1Pos > e1Pos {
		t.Errorf("beans-f1 (order 1) rendered after beans-e1 (order 2): f1 at %d, e1 at %d\n%s", f1Pos, e1Pos, got)
	}
}
