package commands

import (
	"strings"
	"testing"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
)

// tagsFixture is a one-milestone roadmap where the milestone and one leaf
// carry tags and a second leaf carries none, so a single render exercises
// both the tagged and the untagged path.
func tagsFixture() *roadmap.Data {
	ms := &item.Item{ID: "beans-ms01", Title: "Payments", Type: "milestone", Status: "todo", Tags: []string{"planning", "cli"}}
	tagged := &item.Item{ID: "beans-it01", Title: "Wire up sheet", Type: "task", Status: "todo", Parent: ms.ID, Tags: []string{"ux"}}
	untagged := &item.Item{ID: "beans-it02", Title: "Rotate signing key", Type: "task", Status: "todo", Parent: ms.ID}

	return &roadmap.Data{
		Milestones: []roadmap.MilestoneGroup{{
			Milestone: ms,
			Other:     []*item.Item{tagged, untagged},
		}},
	}
}

// -- TTY tag properties, migrated onto roadmapOutput (beans-dbph Step B
// review, F2): the bespoke roadmap-only renderer became unreachable from any
// command once the TTY branch went through ui.Render, so the six tests that
// used to call it directly pinned a renderer nothing can reach any more. The value kept
// here is the tag *properties* the ruling named -- tags appear only when
// --tags is set, they occupy exactly one line, the first tag stays visible
// even when it must be elided, and overflow becomes "+N" -- not the old
// renderer's hanging-indent-row glyph shape, which ui/columns.go's tagCell
// replaces with an inline cell on the item's own line. Dropped entirely
// (properties that no longer exist under the new layout, not migrated):
// TestRoadmapTagLineRendersAtHangingIndent (pinned the old row's column-17
// hanging indent -- tags are an inline cell on the item's own line now, no
// hanging row exists), TestRoadmapTagLineWrapsAtTitleWidth (pinned that a
// long tag list wraps over several lines -- inverted by design, tagCell
// documents itself as never hard-breaking; overflow elides into "+N"
// instead, see TestRoadmapOutputTagOverflowKeepsFirstTagVisible),
// TestRoadmapTagLineFollowsWrappedTitle (pinned the ordering of a separate
// tag row after a wrapped title's continuation lines -- there is no
// separate tag row to order any more, see
// TestRoadmapOutputTagsStayOnTheItemsOwnLine), TestRoadmapTagRowFollowedByBlankLine
// and TestRoadmapUntaggedItemKeepsNoBlankLine (both pinned a blank line
// after a separate tag row -- moot once tags stopped being a separate row).

// TestRoadmapOutputTagsAppearOnlyWhenFlagSet replaces
// TestRoadmapPrettyWithoutTagsFlagUnchanged: --tags is opt-in, in both
// directions -- no tag text without the flag, and the same fixture does
// show tag text with it.
func TestRoadmapOutputTagsAppearOnlyWhenFlagSet(t *testing.T) {
	data := tagsFixture()

	without := roadmapOutput(data, true, roadmapFormatTTY, 90, true, "", false, ui.FormTree, config.Default())
	if strings.Contains(without, "#planning") || strings.Contains(without, "#ux") {
		t.Errorf("tags leaked into the render without --tags:\n%s", without)
	}

	with := roadmapOutput(data, true, roadmapFormatTTY, 90, true, "", true, ui.FormTree, config.Default())
	if !strings.Contains(with, "#planning") {
		t.Errorf("expected #planning in the render with --tags, got:\n%s", with)
	}
}

// TestRoadmapOutputTagsOnlyForTaggedItems replaces
// TestRoadmapTagLineOnlyForTaggedItems: an item's tags render on its own
// line (tagCell is an inline cell, not a following row any more), and an
// untagged item's line carries no "#" at all.
func TestRoadmapOutputTagsOnlyForTaggedItems(t *testing.T) {
	out := roadmapOutput(tagsFixture(), true, roadmapFormatTTY, 90, true, "", true, ui.FormTree, config.Default())

	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "Wire up sheet"):
			if !strings.Contains(line, "#ux") {
				t.Errorf("tagged bean's own line missing #ux: %q", line)
			}
		case strings.Contains(line, "Rotate signing key"):
			if strings.Contains(line, "#") {
				t.Errorf("untagged bean's line carries a tag: %q", line)
			}
		}
	}
}

// TestRoadmapOutputTagsStayOnTheItemsOwnLine replaces
// TestRoadmapTagLineWrapsAtTitleWidth and TestRoadmapTagLineFollowsWrappedTitle:
// tags never get their own row, even when the title wraps over several
// lines -- they render once, on the item's first (title-starting) line, and
// none of the title's continuation lines carry a "#".
func TestRoadmapOutputTagsStayOnTheItemsOwnLine(t *testing.T) {
	long := "Refactor payment reconciliation ledger to support multi-currency settlement across regions"
	ms := &item.Item{ID: "beans-ms01", Title: long, Type: "milestone", Status: "todo", Tags: []string{"ledger"}}
	data := &roadmap.Data{Milestones: []roadmap.MilestoneGroup{{Milestone: ms}}}

	out := roadmapOutput(data, true, roadmapFormatTTY, 80, true, "", true, ui.FormTree, config.Default())
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	var tagLines []string
	for _, l := range lines {
		if strings.Contains(l, "#ledger") {
			tagLines = append(tagLines, l)
		}
	}
	if len(tagLines) != 1 {
		t.Fatalf("expected #ledger on exactly one line, got %d:\n%s", len(tagLines), out)
	}
	if !strings.Contains(tagLines[0], "beans-ms01") {
		t.Errorf("expected the tag to share the bean's own line (carrying its ID), got %q", tagLines[0])
	}
}

// TestRoadmapOutputTagOverflowKeepsFirstTagVisible replaces the overflow
// half of TestRoadmapTagLineWrapsAtTitleWidth under the new model: a tag
// list too long for the column elides rather than wraps -- the first tag
// stays visible and the rest collapse into a "+N" marker, per
// ui/columns.go's tagCell.
func TestRoadmapOutputTagOverflowKeepsFirstTagVisible(t *testing.T) {
	many := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliett", "kilo", "lima"}
	ms := &item.Item{ID: "beans-ms01", Title: "Payments", Type: "milestone", Status: "todo", Tags: many}
	data := &roadmap.Data{Milestones: []roadmap.MilestoneGroup{{Milestone: ms}}}

	out := roadmapOutput(data, true, roadmapFormatTTY, 80, true, "", true, ui.FormTree, config.Default())

	if !strings.Contains(out, "#alpha") {
		t.Errorf("expected the first tag (#alpha) to stay visible, got:\n%s", out)
	}
	if !strings.Contains(out, "+") {
		t.Errorf("expected an elision marker (+N) for the remaining tags, got:\n%s", out)
	}
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.Contains(l, "#lima") {
			t.Errorf("expected the last tag to be elided behind +N, not rendered directly: %q", l)
		}
	}
}

// TestRoadmapMarkdownTagLine pins the Markdown mirror: tags on their own
// line, as inline code, indented under a list item so they stay part of it.
// renderRoadmapMarkdown is unaffected by the Step B swap -- still reachable
// directly from the command, still tested directly here.
func TestRoadmapMarkdownTagLine(t *testing.T) {
	out := renderRoadmapMarkdown(tagsFixture(), false, "", true)

	if !strings.Contains(out, "\n`#planning` `#cli`\n") {
		t.Errorf("milestone tag line missing from:\n%s", out)
	}
	if !strings.Contains(out, "\n  `#ux`\n") {
		t.Errorf("leaf tag line missing or not indented in:\n%s", out)
	}
	if strings.Contains(out, "`#`") {
		t.Errorf("untagged bean produced an empty tag line:\n%s", out)
	}
}

// TestRoadmapMarkdownWithoutTagsFlagUnchanged pins that the Markdown path is
// also opt-in.
func TestRoadmapMarkdownWithoutTagsFlagUnchanged(t *testing.T) {
	withFlag := renderRoadmapMarkdown(tagsFixture(), false, "", false)

	if strings.Contains(withFlag, "#planning") || strings.Contains(withFlag, "#ux") {
		t.Errorf("tags leaked into the default Markdown render:\n%s", withFlag)
	}
}
