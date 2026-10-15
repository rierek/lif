package tui

// view_detail_bean_test.go — TDD-first tests for beanSections (E2 Task 1,
// bean bt-ms0k): the 4 fixed accordion sections built from a *data.Bean + its
// *data.Index. E7 Task 4 (bean bt-kyj5, PF-1/PF-3/PF-4/PF-12) rebuilds this
// around the new Meta-Feldliste + Kopfblock: section titles go uppercase-
// English (META/BODY/RELATIONS/HISTORY, PF-7's remainder), beanSections
// grows 3 render-state params (focused, activeIdx, fieldIdx -- Meta's body
// now needs to know whether IT is the active section to place the ▶ marker),
// and Meta's body becomes a 6-entry cursor-navigable field list instead of a
// 4-line status/type/priority/tags summary (tags are no longer part of Meta
// at all -- PF-4 fixes the field set to title/status/type/priority/
// created_at/updated_at). E8 Task 1 (bean bt-e6q9, PF-15/PF-16, D01/B02/B04):
// tags rejoins Meta as its own 7th field (title/status/type/priority/tags/
// created_at/updated_at) and beanSections grows a 4th render-state param
// (detailLevel) so the ▶ marker gates on the field level actually having
// been entered, not merely on Meta being the active section (B04).

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/data"
)

// TestBeanSectionsAlwaysFourFixedSections guards E2's simpler digit-jump
// semantics vs. devd's content-gated sections: a minimal bean (no body, no
// tags, no relations) still yields exactly 4 sections, titled and ordered
// META/BODY/RELATIONS/HISTORY (PF-7, uppercase English -- T3 deliberately
// left these 4 titles to this task, see bt-w9o8 "Notes for T4").
func TestBeanSectionsAlwaysFourFixedSections(t *testing.T) {
	beans := []data.Bean{
		{ID: "min-1", Title: "Minimal", Status: "todo", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	secs := beanSections(idx, idx.ByID["min-1"], 60, false, 0, 0, 0)

	if len(secs) != 4 {
		t.Fatalf("beanSections returned %d sections, want 4", len(secs))
	}
	want := []string{"META", "BODY", "RELATIONS", "HISTORY"}
	for i, w := range want {
		if secs[i].title != w {
			t.Errorf("section %d title = %q, want %q", i, secs[i].title, w)
		}
	}
}

// TestMetaFieldsFiveEntriesWithKinds guards metaFields' structural contract
// (PF-4 + PF-15/D01, shrunk by bt-lg68): exactly 5 kind-tagged entries in a
// fixed order (title/status/type/priority/tags), every entry's beanID empty
// (Meta fields are never Beziehungen-style jump targets -- kind drives T6's
// enter-dispatch instead). Renamed from TestMetaFieldsSevenEntriesWithKinds
// (bt-lg68, PO-Nebenbefund US-Review Runde 3: created_at/updated_at were
// doubly rendered in META AND HISTORY -- the two readonly entries are
// removed here, HISTORY stays the sole source for Created/Updated).
func TestMetaFieldsFiveEntriesWithKinds(t *testing.T) {
	b := &data.Bean{ID: "meta-1", Title: "Meta Bean", Status: "in-progress", Type: "bug", Priority: "critical"}
	fields := metaFields(b)
	if len(fields) != 5 {
		t.Fatalf("metaFields returned %d entries, want 5 (bt-lg68: created_at/updated_at removed, HISTORY is now the sole source)", len(fields))
	}
	wantKinds := []string{"title", "status", "type", "priority", "tags"}
	for i, want := range wantKinds {
		if fields[i].kind != want {
			t.Errorf("fields[%d].kind = %q, want %q", i, fields[i].kind, want)
		}
		if fields[i].beanID != "" {
			t.Errorf("fields[%d].beanID = %q, want empty (Meta fields are never jump targets)", i, fields[i].beanID)
		}
	}
	if fields[0].label != "Meta Bean" {
		t.Errorf("fields[0] (title) label = %q, want %q", fields[0].label, "Meta Bean")
	}
	// status/type/priority reuse T2's GLYPH output (PF-6 StatusIcon/TypeIcon/
	// Priority), NOT the word -- design-spec.md §15 PF-4 explicit ("nutzt T2s
	// Glyph-Output"), unlike the Kopfblock's type/status (see
	// TestDetailHeaderBlockShowsIDTitleTypeStatusPrio, which uses the WORD).
	if got := ansi.Strip(fields[1].label); got != "i" {
		t.Errorf("fields[1] (status) label = %q, want glyph \"i\" (PF-6 StatusIcon(\"in-progress\"))", got)
	}
	if got := ansi.Strip(fields[2].label); got != "B" {
		t.Errorf("fields[2] (type) label = %q, want glyph \"B\" (PF-6 TypeIcon(\"bug\"))", got)
	}
	if got := ansi.Strip(fields[3].label); got != "‼" {
		t.Errorf("fields[3] (priority) label = %q, want glyph \"‼\" (PF-6 Priority(\"critical\"))", got)
	}
}

// TestMetaFieldsTagsEmptyShowsNonePlaceholder guards D01's placeholder path:
// a bean with no tags renders the tags field as a theme.Dim "(none)", not an
// empty string (design-spec.md §15 PF-15).
func TestMetaFieldsTagsEmptyShowsNonePlaceholder(t *testing.T) {
	b := &data.Bean{ID: "meta-notags", Title: "No Tags", Status: "todo", Type: "task", Priority: "normal"}
	fields := metaFields(b)
	if got := ansi.Strip(fields[4].label); got != "(none)" {
		t.Errorf("fields[4] (tags) label = %q, want the empty-tags placeholder \"(none)\"", got)
	}
}

// TestMetaFieldsTagsNonEmptyUsesTagsInline guards D01's value-rendering path:
// a bean WITH tags renders fields[4] via the revived tagsInline/tagSwatch
// helper (render_shared.go) -- Hash-colored "● tag" swatches, not a plain
// comma-joined list.
func TestMetaFieldsTagsNonEmptyUsesTagsInline(t *testing.T) {
	b := &data.Bean{ID: "meta-tags", Title: "Tagged", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"backend", "urgent"}}
	fields := metaFields(b)
	want := tagsInline(b.Tags)
	if fields[4].label != want {
		t.Errorf("fields[4] (tags) label = %q, want tagsInline(b.Tags) = %q", fields[4].label, want)
	}
	if got := ansi.Strip(fields[4].label); !strings.Contains(got, "backend") || !strings.Contains(got, "urgent") {
		t.Errorf("fields[4] (tags) label = %q, want both tag names present", got)
	}
}

// TestMetaFieldsPriorityDefaultsToNormalWhenEmpty guards the same "" ->
// "normal" default the old metaSectionBody used to apply (b.Priority is
// beans-legal empty on hand-edited frontmatter).
func TestMetaFieldsPriorityDefaultsToNormalWhenEmpty(t *testing.T) {
	b := &data.Bean{ID: "meta-noprio", Title: "No Priority", Status: "todo", Type: "task"}
	fields := metaFields(b)
	if got := ansi.Strip(fields[3].label); got != "·" {
		t.Errorf("fields[3] (priority) label = %q, want the \"normal\" glyph \"·\" for empty Priority", got)
	}
}

// TestMetaSectionBodyShowsSelectedFieldMarker guards PF-4's ▷/▶ cursor +
// PF-12's gutter contract at the same time: exactly one ▶ at the active
// fieldIdx's row when the Meta section itself is active, ▷ on all 4 other
// rows (never an omitted marker) when it is not. Field count shrunk 7->5 by
// bt-lg68 (created_at/updated_at removed from META, HISTORY stays sole
// source).
func TestMetaSectionBodyShowsSelectedFieldMarker(t *testing.T) {
	b := &data.Bean{ID: "meta-mark", Title: "Marked", Status: "todo", Type: "task", Priority: "high"}

	active := metaSectionBody(b, 80, true, 2) // type row (index 2)
	lines := strings.Split(active, "\n")
	if len(lines) != 5 {
		t.Fatalf("metaSectionBody active=true has %d lines, want 5 (bt-lg68: 5 Meta fields)", len(lines))
	}
	if n := strings.Count(ansi.Strip(active), "▶"); n != 1 {
		t.Fatalf("active metaSectionBody has %d ▶ markers, want exactly 1", n)
	}
	if !strings.Contains(ansi.Strip(lines[2]), "▶") {
		t.Errorf("▶ marker not on row index 2: %q", ansi.Strip(lines[2]))
	}
	if n := strings.Count(ansi.Strip(active), "▷"); n != 4 {
		t.Errorf("active metaSectionBody has %d ▷ markers, want exactly 4 (the other rows): %q", n, ansi.Strip(active))
	}

	inactive := metaSectionBody(b, 80, false, 2)
	if strings.Contains(ansi.Strip(inactive), "▶") {
		t.Error("inactive metaSectionBody must show no ▶ marker anywhere")
	}
	if n := strings.Count(ansi.Strip(inactive), "▷"); n != 5 {
		t.Errorf("inactive metaSectionBody has %d ▷ markers, want exactly 5 (every row): %q", n, ansi.Strip(inactive))
	}
}

// TestMetaSectionBodyShowsAllFiveLabelsAndValues guards the row content
// itself (label prefixes + values), not just the marker: PF-15/D01's mockup
// shows title/status/type/priority/tags in that exact order. Renamed from
// TestMetaSectionBodyShowsAllSevenLabelsAndValues (bt-lg68: created_at/
// updated_at removed from META, HISTORY stays the sole source).
func TestMetaSectionBodyShowsAllFiveLabelsAndValues(t *testing.T) {
	b := &data.Bean{ID: "meta-full", Title: "Full Bean", Status: "todo", Type: "task", Priority: "normal"}
	body := ansi.Strip(metaSectionBody(b, 80, false, 0))

	wantLabels := []string{"title:", "status:", "type:", "priority:", "tags:"}
	lastIdx := -1
	for _, label := range wantLabels {
		idx := strings.Index(body, label)
		if idx < 0 {
			t.Fatalf("metaSectionBody missing label %q: %q", label, body)
		}
		if idx <= lastIdx {
			t.Errorf("label %q out of order (idx %d, previous %d)", label, idx, lastIdx)
		}
		lastIdx = idx
	}
	if !strings.Contains(body, "Full Bean") {
		t.Errorf("metaSectionBody missing title value: %q", body)
	}
	if strings.Contains(body, "created_at:") || strings.Contains(body, "updated_at:") {
		t.Errorf("metaSectionBody must NOT show created_at/updated_at (bt-lg68: HISTORY is the sole source): %q", body)
	}
}

// TestBeanSectionsMetaMarkerHiddenUntilDetailLevelEntered guards B04 (bean
// bt-ntoz, design-spec.md §15 PF-16): the Meta field ▶ marker must NOT appear
// merely because Meta (activeIdx==0) is the active section after `tab` --
// it appears only once the field level has actually been entered
// (detailLevel==1). Meta's ▷/▶ marker lives inside metaSectionBody (via
// beanSections' `focused && activeIdx == 0` gate) -- detailLevel is the NEW
// third gate condition this test pins.
func TestBeanSectionsMetaMarkerHiddenUntilDetailLevelEntered(t *testing.T) {
	beans := []data.Bean{
		{ID: "b04-1", Title: "B04 Bean", Status: "todo", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)

	// focused=true, activeIdx=0 (Meta is active section), detailLevel=0 (field
	// level NOT yet entered) -- must show NO ▶ marker anywhere in Meta's body.
	secsLevel0 := beanSections(idx, idx.ByID["b04-1"], 60, true, 0, 0, 0)
	if strings.Contains(ansi.Strip(secsLevel0[0].body), "▶") {
		t.Errorf("Meta body shows ▶ at detailLevel==0 (field level not entered): %q", ansi.Strip(secsLevel0[0].body))
	}

	// Same focused/activeIdx, but detailLevel=1 (field level entered) -- ▶
	// must now appear at fieldIdx's row.
	secsLevel1 := beanSections(idx, idx.ByID["b04-1"], 60, true, 0, 0, 1)
	if !strings.Contains(ansi.Strip(secsLevel1[0].body), "▶") {
		t.Errorf("Meta body missing ▶ at detailLevel==1 (field level entered): %q", ansi.Strip(secsLevel1[0].body))
	}
}

// TestDetailHeaderBlockShowsIDTitleTypeStatusPrio guards the NEW Kopfblock
// (PF-3+PF-4, verschmolzen laut PO-Antwort Q01): bean-id / NAME / blank /
// "type: X    status: Y    prio: Z" / trailing blank. type/status render as
// the WORD (theme.TypeStyle/StatusStyle + the raw word, mirrors the OLD
// metaSectionBody's word rendering) -- prio renders as the GLYPH (theme.
// Priority has no word-producing counterpart since PF-6, epic-E7-plan.md
// Task 4 signature block is explicit: "...+Priority", not "+PriorityStyle").
func TestDetailHeaderBlockShowsIDTitleTypeStatusPrio(t *testing.T) {
	b := &data.Bean{ID: "hdr-1", Title: "Header Bean", Status: "in-progress", Type: "bug", Priority: "high"}
	out := detailHeaderBlock(b, 60)
	stripped := ansi.Strip(out)

	if !strings.Contains(stripped, "hdr-1") {
		t.Errorf("header block missing bean ID: %q", stripped)
	}
	if !strings.Contains(stripped, "Header Bean") {
		t.Errorf("header block missing title: %q", stripped)
	}
	if !strings.Contains(stripped, "type: bug") {
		t.Errorf("header block missing type as WORD (\"type: bug\"): %q", stripped)
	}
	if !strings.Contains(stripped, "status: in-progress") {
		t.Errorf("header block missing status as WORD (\"status: in-progress\"): %q", stripped)
	}
	if !strings.Contains(stripped, "prio: !") {
		t.Errorf("header block missing prio as GLYPH (\"prio: !\", high): %q", stripped)
	}
	// B02 (bean bt-ntoz, design-spec.md §15 PF-16): type/status are padded to
	// fixed column widths (9 = len("milestone"), 11 = len("in-progress")) so
	// the Kopfblock never jumps between beans of differing word length.
	// Expected spacing computed via the SAME fmt verb the implementation
	// uses, not hand-counted, to avoid a brittle literal.
	wantTypeCol := fmt.Sprintf("type: %-9s    status:", "bug")
	if !strings.Contains(stripped, wantTypeCol) {
		t.Errorf("header block type column not padded to 9: got %q, want to contain %q", stripped, wantTypeCol)
	}
	wantStatusCol := fmt.Sprintf("status: %-11s    prio:", "in-progress")
	if !strings.Contains(stripped, wantStatusCol) {
		t.Errorf("header block status column not padded to 11: got %q, want to contain %q", stripped, wantStatusCol)
	}

	lines := strings.Split(out, "\n")
	if len(lines) < 5 {
		t.Fatalf("header block has %d lines, want at least 5 (ID/Title/blank/type-status-prio/trailing blank)", len(lines))
	}
	if lines[len(lines)-1] != "" {
		t.Errorf("header block must end with a trailing blank line, got %q as last line", lines[len(lines)-1])
	}
}

// TestDetailHeaderBlockFixedColumnWidthsNoJumpAcrossBeans is B02's core
// regression guard (bean bt-ntoz, design-spec.md §15 PF-16): the "status:"
// and "prio:" column start positions must be IDENTICAL across beans with
// differing type/status word length (epic vs. milestone, todo vs.
// in-progress) -- previously type/status rendered unpadded, so the Kopfblock
// row shifted horizontally on every bean-change.
func TestDetailHeaderBlockFixedColumnWidthsNoJumpAcrossBeans(t *testing.T) {
	short := &data.Bean{ID: "hdr-short", Title: "Short", Status: "todo", Type: "epic", Priority: "normal"}
	long := &data.Bean{ID: "hdr-long2", Title: "Long", Status: "in-progress", Type: "milestone", Priority: "normal"}

	shortLine := ansi.Strip(strings.Split(detailHeaderBlock(short, 80), "\n")[3])
	longLine := ansi.Strip(strings.Split(detailHeaderBlock(long, 80), "\n")[3])

	shortStatusIdx := strings.Index(shortLine, "status:")
	longStatusIdx := strings.Index(longLine, "status:")
	if shortStatusIdx < 0 || longStatusIdx < 0 {
		t.Fatalf("missing \"status:\" in one of the two lines: %q / %q", shortLine, longLine)
	}
	if shortStatusIdx != longStatusIdx {
		t.Errorf("status: column shifted across beans (type word length varies): short=%d long=%d (%q vs %q)", shortStatusIdx, longStatusIdx, shortLine, longLine)
	}

	shortPrioIdx := strings.Index(shortLine, "prio:")
	longPrioIdx := strings.Index(longLine, "prio:")
	if shortPrioIdx < 0 || longPrioIdx < 0 {
		t.Fatalf("missing \"prio:\" in one of the two lines: %q / %q", shortLine, longLine)
	}
	if shortPrioIdx != longPrioIdx {
		t.Errorf("prio: column shifted across beans (status word length varies): short=%d long=%d (%q vs %q)", shortPrioIdx, longPrioIdx, shortLine, longLine)
	}
}

// TestDetailHeaderBlockTruncatesToWidth guards that every content line is
// clamped to w (no App-Chrome wrap concept here, single-line-per-field
// mirrors the ID/type-status-prio mockup) -- an overlong title must not blow
// out the accordion pane's bordered width.
func TestDetailHeaderBlockTruncatesToWidth(t *testing.T) {
	b := &data.Bean{ID: "hdr-long", Title: strings.Repeat("very-long-title-word ", 10), Status: "todo", Type: "task", Priority: "normal"}
	const w = 24
	for _, line := range strings.Split(ansi.Strip(detailHeaderBlock(b, w)), "\n") {
		if width := lipgloss.Width(line); width > w {
			t.Errorf("header block line exceeds w=%d (got %d): %q", w, width, line)
		}
	}
}

// TestDetailHeaderBlockShowsTagsColumn guards B05 (bean bt-mtig, design-
// spec.md §15 PF-17, bt-tct9 "B05 REDEFINIERT"): the Kopfblock's type/status/
// prio row (line 4) grows a 4th column "    tags: <tagsInline(b.Tags)>" --
// PO-Mockup exact: "type: epic  status: in-progress  prio: !  tags: to-
// review". Render-geerdet against tagsInline itself (the SAME helper
// metaFields already reuses, PF-15) -- no second, independently computed
// formula for the tag swatches.
func TestDetailHeaderBlockShowsTagsColumn(t *testing.T) {
	b := &data.Bean{ID: "hdr-tags", Title: "Tagged Header", Status: "in-progress", Type: "epic", Priority: "high", Tags: []string{"to-review"}}
	const w = 100
	out := detailHeaderBlock(b, w)
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Fatalf("header block has %d lines, want at least 4", len(lines))
	}
	line := lines[3]
	stripped := ansi.Strip(line)
	if !strings.Contains(stripped, "tags:") {
		t.Errorf("header block line 4 missing \"tags:\" label: %q", stripped)
	}
	if !strings.Contains(stripped, "to-review") {
		t.Errorf("header block line 4 missing tag value \"to-review\": %q", stripped)
	}
	wantTags := tagsInline(b.Tags)
	if !strings.Contains(line, wantTags) {
		t.Errorf("header block line 4 does not contain tagsInline(b.Tags) output %q verbatim: %q", wantTags, line)
	}
	// B02 (E8-B02) column widths for type/status/prio must stay untouched --
	// tags is a NEW, appended 4th column, not a rework of the first three.
	wantTypeCol := fmt.Sprintf("type: %-9s    status:", "epic")
	if !strings.Contains(stripped, wantTypeCol) {
		t.Errorf("header block type column not padded to 9 (B05 must not touch E8-B02 padding): got %q, want to contain %q", stripped, wantTypeCol)
	}
}

// TestDetailHeaderBlockShowsNoneForTaglessBean guards the taglos fallback:
// mirrors metaFields' own "(none)" (theme.Dim) convention, not a bare empty
// string or a missing "tags:" label.
func TestDetailHeaderBlockShowsNoneForTaglessBean(t *testing.T) {
	b := &data.Bean{ID: "hdr-notags", Title: "No Tags Header", Status: "todo", Type: "task", Priority: "normal"}
	out := detailHeaderBlock(b, 100)
	lines := strings.Split(out, "\n")
	stripped := ansi.Strip(lines[3])
	if !strings.Contains(stripped, "tags: (none)") {
		t.Errorf("header block line 4 missing taglos placeholder \"tags: (none)\": %q", stripped)
	}
}

// TestBeanSectionsBodyEmptyShowsPlaceholder guards the empty-body path: no
// Body field set -> a placeholder, not an empty string (digit-jump 2 must
// always show something).
func TestBeanSectionsBodyEmptyShowsPlaceholder(t *testing.T) {
	beans := []data.Bean{
		{ID: "nobody-1", Title: "No Body", Status: "todo", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	secs := beanSections(idx, idx.ByID["nobody-1"], 60, false, 0, 0, 0)

	if strings.TrimSpace(secs[1].body) == "" {
		t.Fatal("Body section must show a placeholder for an empty bean body, got empty string")
	}
}

// TestBeanSectionsBodyRendersMarkdown guards the glamour wiring: markdown
// heading/bold syntax survives as rendered ANSI under TrueColor.
func TestBeanSectionsBodyRendersMarkdown(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	beans := []data.Bean{
		{ID: "body-1", Title: "Has Body", Status: "todo", Type: "task", Priority: "normal", Body: "# Heading\n\nSome **bold** text."},
	}
	idx := data.NewIndex(beans)
	secs := beanSections(idx, idx.ByID["body-1"], 60, false, 0, 0, 0)

	if !strings.Contains(secs[1].body, "\x1b[") {
		t.Errorf("Body section under TrueColor must contain ANSI escapes (glamour-rendered), got: %q", secs[1].body)
	}
	if !strings.Contains(secs[1].body, "Heading") {
		t.Errorf("Body section missing rendered heading text: %q", secs[1].body)
	}
}

// TestBeanSectionsRelationsListsParentChildrenBlockingBlockedByInCanonicalOrder
// guards I03: Blocking/BlockedBy resolved beans must render in data.SortBeans
// canonical tier order (Status -> Priority -> Type -> Title), NOT insertion
// order. Children arrives pre-sorted via idx.Children; this test forces
// Blocking to disagree with insertion order (in-progress bean listed second
// in the raw slice, must still render first). Renamed from ...Beziehungen...
// (PF-7, section title RELATIONS).
func TestBeanSectionsRelationsListsParentChildrenBlockingBlockedByInCanonicalOrder(t *testing.T) {
	beans := []data.Bean{
		{ID: "rel-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "rel-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rel-parent",
			Blocking: []string{"rel-block-todo", "rel-block-inprog"}},
		{ID: "rel-child", Title: "Child Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rel-main"},
		// Blocking targets: listed todo-first in Blocking slice above, but
		// in-progress must sort first per data.SortBeans canonical order.
		{ID: "rel-block-todo", Title: "Blocked Todo", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "rel-block-inprog", Title: "Blocked InProgress", Status: "in-progress", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	secs := beanSections(idx, idx.ByID["rel-main"], 60, false, 0, 0, 0)

	rel := secs[2]
	if rel.title != "RELATIONS" {
		t.Fatalf("section 2 title = %q, want RELATIONS", rel.title)
	}
	if !strings.Contains(rel.body, "Parent Bean") {
		t.Errorf("Relations body missing Parent: %q", rel.body)
	}
	if !strings.Contains(rel.body, "Child Bean") {
		t.Errorf("Relations body missing Children: %q", rel.body)
	}

	// Find the relationField order for the two Blocking targets among
	// rel.fields (Parent is 1 field, Children 1 field, then Blocking x2).
	var blockingIDs []string
	for _, f := range rel.fields {
		if f.beanID == "rel-block-todo" || f.beanID == "rel-block-inprog" {
			blockingIDs = append(blockingIDs, f.beanID)
		}
	}
	if len(blockingIDs) != 2 {
		t.Fatalf("expected 2 Blocking relationFields, got %v", blockingIDs)
	}
	if blockingIDs[0] != "rel-block-inprog" {
		t.Errorf("Blocking field order = %v, want rel-block-inprog first (canonical status tier, not insertion order)", blockingIDs)
	}
}

// TestBeanSectionsRelationsDanglingReferenceShowsUnresolvedNotJumpable
// guards the dangling-BlockedBy path: an ID with no match in idx.ByID renders
// as a muted "(unresolved: <id>)" row with relationField.beanID == "" (Task
// 2's jump-guard checks this). Renamed from ...Beziehungen... (PF-7).
func TestBeanSectionsRelationsDanglingReferenceShowsUnresolvedNotJumpable(t *testing.T) {
	beans := []data.Bean{
		{ID: "dangling-1", Title: "Dangling Bean", Status: "todo", Type: "task", Priority: "normal",
			BlockedBy: []string{"does-not-exist"}},
	}
	idx := data.NewIndex(beans)
	secs := beanSections(idx, idx.ByID["dangling-1"], 60, false, 0, 0, 0)

	rel := secs[2]
	if !strings.Contains(rel.body, "does-not-exist") {
		t.Errorf("Relations body missing the unresolved reference id: %q", rel.body)
	}
	found := false
	for _, f := range rel.fields {
		if strings.Contains(f.label, "does-not-exist") {
			found = true
			if f.beanID != "" {
				t.Errorf("unresolved relationField.beanID = %q, want empty (not jumpable)", f.beanID)
			}
		}
	}
	if !found {
		t.Fatal("expected a relationField for the unresolved BlockedBy reference")
	}
}

// TestBeanSectionsMetaWrapsLongTitleToBodyWidth guards B01 (E2-T1-quality-
// review finding, MANDATORY carry-over)'s wrapping requirement against the
// PF-4 Meta field list: Tags is no longer part of Meta at all (the old test
// exercised long tag names), so the overflow risk now sits on the title row
// -- an arbitrarily long bean title must still wrap inside bodyW, not
// overflow the Detail pane's bordered width.
func TestBeanSectionsMetaWrapsLongTitleToBodyWidth(t *testing.T) {
	beans := []data.Bean{
		{ID: "wrap-meta", Title: strings.Repeat("a-very-long-title-word ", 6), Status: "todo", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	const bodyW = 20
	secs := beanSections(idx, idx.ByID["wrap-meta"], bodyW, false, 0, 0, 0)

	for _, line := range strings.Split(ansi.Strip(secs[0].body), "\n") {
		if w := lipgloss.Width(line); w > bodyW {
			t.Errorf("Meta line exceeds bodyW=%d (got %d): %q", bodyW, w, line)
		}
	}
}

// TestBeanSectionsHistorieShowsCreatedUpdatedETag guards the History
// section: nil CreatedAt/UpdatedAt render a placeholder, never a zero-time
// string ("0001-01-01..."). Section title assertion updated to HISTORY
// (PF-7).
func TestBeanSectionsHistorieShowsCreatedUpdatedETag(t *testing.T) {
	beans := []data.Bean{
		{ID: "hist-nil", Title: "No Timestamps", Status: "todo", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	secs := beanSections(idx, idx.ByID["hist-nil"], 60, false, 0, 0, 0)

	if secs[3].title != "HISTORY" {
		t.Fatalf("section 3 title = %q, want HISTORY", secs[3].title)
	}

	histNil := secs[3].body
	if strings.Contains(histNil, "0001-01-01") {
		t.Errorf("Historie must not render Go's zero-time string for nil timestamps: %q", histNil)
	}

	created := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 1, 3, 11, 0, 0, 0, time.UTC)
	beansSet := []data.Bean{
		{ID: "hist-set", Title: "With Timestamps", Status: "todo", Type: "task", Priority: "normal",
			CreatedAt: &created, UpdatedAt: &updated, ETag: "abc123"},
	}
	idx2 := data.NewIndex(beansSet)
	secs2 := beanSections(idx2, idx2.ByID["hist-set"], 60, false, 0, 0, 0)
	hist := secs2[3].body
	if !strings.Contains(hist, "2026-01-02") {
		t.Errorf("Historie missing CreatedAt: %q", hist)
	}
	if !strings.Contains(hist, "2026-01-03") {
		t.Errorf("Historie missing UpdatedAt: %q", hist)
	}
	if !strings.Contains(hist, "abc123") {
		t.Errorf("Historie missing ETag: %q", hist)
	}
}

// TestBeanSectionsHistorieWrapsLongETagToBodyWidth guards B01 (E2-T1-quality-
// review finding, MANDATORY carry-over into Task 2): historieSectionBody must
// wrap via wrapText -- otherwise a long ETag overflows the Detail pane's
// bordered width.
func TestBeanSectionsHistorieWrapsLongETagToBodyWidth(t *testing.T) {
	beans := []data.Bean{
		{ID: "wrap-hist", Title: "Wrap Hist", Status: "todo", Type: "task", Priority: "normal",
			ETag: strings.Repeat("etagchunk-", 5)},
	}
	idx := data.NewIndex(beans)
	const bodyW = 20
	secs := beanSections(idx, idx.ByID["wrap-hist"], bodyW, false, 0, 0, 0)

	for _, line := range strings.Split(ansi.Strip(secs[3].body), "\n") {
		if w := lipgloss.Width(line); w > bodyW {
			t.Errorf("Historie line exceeds bodyW=%d (got %d): %q", bodyW, w, line)
		}
	}
}

// --- hangingIndentWrap (B04.3, design-spec.md §15 PF-17, bean bt-b0w0) ---
//
// hangingIndentWrap wraps text to width w, with the FIRST line prefixed by
// prefix (already-styled) and every CONTINUATION line left-padded to match
// the prefix's own VISIBLE width instead of falling back to column 0 -- the
// actual PO-Mockup bug: relationsSectionBody's old blanket wrapText() over
// the whole joined block re-wrapped already-composed rows with no notion of
// their own per-row prefix width, so a long title's continuation wrapped
// back to column 0 and "unterwanderte" the following row's Meta-Spalten
// (glyph/status/ID). Reused verbatim by Task 5/bt-4mo9's Relations-Picker
// (same row shape) -- kept as an independent, well-tested helper per that
// task's own Notes-for-T5 requirement.

// TestHangingIndentWrapShortTitleNoWrap guards the trivial no-wrap path: a
// title that fits on the first line renders as prefix+text, single line, no
// continuation indent logic engaged at all.
func TestHangingIndentWrapShortTitleNoWrap(t *testing.T) {
	got := hangingIndentWrap("t bt-1 ", "Short title", 40)
	want := "t bt-1 Short title"
	if got != want {
		t.Errorf("hangingIndentWrap = %q, want %q", got, want)
	}
}

// TestHangingIndentWrapLongTitleAlignsContinuationUnderPrefixEnd guards the
// actual fix: a title long enough to wrap must have its continuation
// line(s) left-padded with exactly indentW spaces (indentW == the prefix's
// own visible width via lipgloss.Width), never column 0.
func TestHangingIndentWrapLongTitleAlignsContinuationUnderPrefixEnd(t *testing.T) {
	prefix := "t bt-apmy "
	indentW := lipgloss.Width(prefix)
	got := hangingIndentWrap(prefix, "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", 40)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the long title to wrap into >=2 lines, got %d: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], prefix) {
		t.Errorf("first line must start with the prefix, got: %q", lines[0])
	}
	for i, line := range lines[1:] {
		gotIndent := len(line) - len(strings.TrimLeft(line, " "))
		if gotIndent != indentW {
			t.Errorf("continuation line %d indent = %d spaces, want %d (prefix width): %q", i+1, gotIndent, indentW, line)
		}
	}
}

// TestHangingIndentWrapVaryingPrefixWidths guards the "PRO ZEILE
// individuell" requirement (design-spec.md §15 PF-17 B04.3): the indent is
// NOT a shared table column across rows -- bean IDs vary in length
// ("<prefix>-<n-chars>"), so a short-ID prefix and a long-ID prefix must
// each produce a continuation indent matching THEIR OWN prefix width.
func TestHangingIndentWrapVaryingPrefixWidths(t *testing.T) {
	shortPrefix := "t bt-1 "
	longPrefix := "t bt-abcdef12 "
	longTitle := "This title is intentionally long enough that it must wrap across more than one line for sure"

	shortOut := hangingIndentWrap(shortPrefix, longTitle, 30)
	longOut := hangingIndentWrap(longPrefix, longTitle, 30)

	shortLines := strings.Split(shortOut, "\n")
	longLines := strings.Split(longOut, "\n")
	if len(shortLines) < 2 || len(longLines) < 2 {
		t.Fatalf("setup: both outputs must wrap, got %d/%d lines", len(shortLines), len(longLines))
	}
	shortIndent := len(shortLines[1]) - len(strings.TrimLeft(shortLines[1], " "))
	longIndent := len(longLines[1]) - len(strings.TrimLeft(longLines[1], " "))
	if shortIndent != lipgloss.Width(shortPrefix) {
		t.Errorf("short-prefix continuation indent = %d, want %d", shortIndent, lipgloss.Width(shortPrefix))
	}
	if longIndent != lipgloss.Width(longPrefix) {
		t.Errorf("long-prefix continuation indent = %d, want %d", longIndent, lipgloss.Width(longPrefix))
	}
	if shortIndent == longIndent {
		t.Errorf("short and long prefix indents must differ (per-row, not a shared column): both = %d", shortIndent)
	}
}

// TestHangingIndentWrapNeverCollapsesBelowMinimumContinuationWidth guards
// the narrow-terminal floor (code sketch, bean bt-b0w0): when w is small
// enough that w-indentW would go below 8, the continuation width floors at
// 8 instead of collapsing to zero/negative -- indent itself still reflects
// the REAL (unclamped) prefix width, only the wrap width is floored.
func TestHangingIndentWrapNeverCollapsesBelowMinimumContinuationWidth(t *testing.T) {
	prefix := "t bt-averylongid1234 " // wider than w below
	indentW := lipgloss.Width(prefix)
	got := hangingIndentWrap(prefix, "some continuation text that needs room to wrap across lines", 10)

	if strings.TrimSpace(got) == "" {
		t.Fatal("hangingIndentWrap must not collapse to empty output on a narrow width")
	}
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapping to still occur at the floored width, got 1 line: %q", got)
	}
	for i, line := range lines[1:] {
		gotIndent := len(line) - len(strings.TrimLeft(line, " "))
		if gotIndent != indentW {
			t.Errorf("continuation line %d indent = %d, want %d (unclamped prefix width)", i+1, gotIndent, indentW)
		}
	}
}

// TestHangingIndentWrapHardBreaksSpacelessLongToken guards F01 (bt-b0w0
// Review-Findings, Fix-Runde 1, medium/blockierend): a single token with NO
// whitespace (URL, German compound, long ID) longer than the continuation
// width must be HARD-broken at the width boundary -- ansi.Wordwrap alone
// only breaks at whitespace and lets such a token overflow the target width
// unbounded (reviewer repro: a 103-char word at w=30 produced one 104-cell
// line). The fix mirrors wrapText's (view.go) exact two-pass pattern:
// ansi.Hardwrap(ansi.Wordwrap(text, contW, ""), contW, true).
func TestHangingIndentWrapHardBreaksSpacelessLongToken(t *testing.T) {
	prefix := "t bt-1 "
	const w = 30
	got := hangingIndentWrap(prefix, strings.Repeat("x", 103), w)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the 103-char spaceless token to hard-break into >=2 lines at w=%d, got 1 line (%d cells wide)", w, lipgloss.Width(got))
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("line %d is %d cells wide, want <= %d (spaceless token must hard-break, F01): %q", i, lw, w, line)
		}
	}
}

// TestHangingIndentWrapCJKDoubleWidthTitleStaysWithinWidth guards F02
// (bt-b0w0 Review-Findings, Fix-Runde 1): CJK/Emoji titles are double-width
// per CELL, not per rune -- the wrap math must budget by lipgloss.Width
// (terminal cells), so no rendered line may exceed w even though the RUNE
// count per line is roughly half a Latin title's. CJK prose also carries no
// spaces, so this doubles as the double-width arm of F01's hard-break pin.
func TestHangingIndentWrapCJKDoubleWidthTitleStaysWithinWidth(t *testing.T) {
	prefix := "t bt-1 "
	const w = 30
	title := "漢字のタイトルが長くて折り返しが必要になるほど長い場合の確認テスト🎉🚀"
	got := hangingIndentWrap(prefix, title, w)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the double-width title to wrap into >=2 lines at w=%d, got 1 line (%d cells wide)", w, lipgloss.Width(got))
	}
	indentW := lipgloss.Width(prefix)
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("line %d is %d cells wide, want <= %d (double-width cells must be budgeted per CELL, F02): %q", i, lw, w, line)
		}
		if i > 0 {
			if gotIndent := len(line) - len(strings.TrimLeft(line, " ")); gotIndent != indentW {
				t.Errorf("continuation line %d indent = %d spaces, want %d (prefix width)", i, gotIndent, indentW)
			}
		}
	}
}

// TestHangingIndentWrapIndentMatchesStyledPrefixUnderTrueColor guards F03
// (bt-b0w0 Review-Findings, Fix-Runde 1): the indent computation
// (lipgloss.Width(ansi.Strip(prefix))) must count VISIBLE cells of a REAL,
// ANSI-styled relationRow prefix -- every other hangingIndentWrap test runs
// under the Ascii ambient default (zero ANSI bytes in the prefix), so a
// regression that counted raw bytes instead of stripped cells would slip
// through them. Render-grounded through relationRow itself (the actual
// production prefix construction), under a forced TrueColor profile.
func TestHangingIndentWrapIndentMatchesStyledPrefixUnderTrueColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	rel := &data.Bean{ID: "tc-1", Title: "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", Status: "todo", Type: "task", Priority: "normal"}
	out := relationRow(rel, relationRowMarker(false, 0, 1), 40)

	if !strings.Contains(out, "\x1b[") {
		t.Fatal("setup: TrueColor relationRow output must contain ANSI escapes (the styled-prefix premise of this test)")
	}
	stripped := ansi.Strip(out)
	lines := strings.Split(stripped, "\n")
	if len(lines) < 2 {
		t.Fatalf("setup: expected the long title to wrap into >=2 lines, got 1: %q", stripped)
	}
	titleCol := cellCol(t, lines[0], "Hier")
	for i, line := range lines[1:] {
		gotIndent := len(line) - len(strings.TrimLeft(line, " "))
		if gotIndent != titleCol {
			t.Errorf("continuation line %d indent = %d, want %d (title-start column of the STYLED prefix, F03): %q", i+1, gotIndent, titleCol, line)
		}
	}
}

// --- relationsSectionBody cursor markers + fieldStrip removal (B04,
// design-spec.md §15 PF-17, bean bt-b0w0) ---

// TestRelationsSectionBodyShowsCursorMarkerOnActiveRow guards B04.2: with
// active=true and fieldIdx=1, exactly the row at GLOBAL index 1 (the Child,
// row order Parent=0/Children=1) carries ▶ -- every other row carries ▷.
// Mirrors TestMetaSectionBodyShowsSelectedFieldMarker's own contract for
// Meta (PF-4).
func TestRelationsSectionBodyShowsCursorMarkerOnActiveRow(t *testing.T) {
	beans := []data.Bean{
		{ID: "rsb-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "rsb-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rsb-parent"},
		{ID: "rsb-child", Title: "Child Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rsb-main"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["rsb-main"]

	body, fields, _ := relationsSectionBody(idx, main, 60, true, 1) // fieldIdx 1 -> the Child row
	if len(fields) != 2 {
		t.Fatalf("setup: expected 2 relationFields (Parent + Child), got %d", len(fields))
	}
	stripped := ansi.Strip(body)
	if n := strings.Count(stripped, "▶"); n != 1 {
		t.Fatalf("active relationsSectionBody has %d ▶ markers, want exactly 1: %q", n, stripped)
	}
	if n := strings.Count(stripped, "▷"); n != 1 {
		t.Fatalf("active relationsSectionBody has %d ▷ markers, want exactly 1 (the other row): %q", n, stripped)
	}
	var childLine string
	for _, l := range strings.Split(stripped, "\n") {
		if strings.Contains(l, "Child Bean") {
			childLine = l
		}
	}
	if !strings.Contains(childLine, "▶") {
		t.Errorf("▶ marker not on the Child row: %q", childLine)
	}

	inactive, _, _ := relationsSectionBody(idx, main, 60, false, 1)
	if strings.Contains(ansi.Strip(inactive), "▶") {
		t.Error("inactive relationsSectionBody must show no ▶ marker anywhere")
	}
}

// TestRelationsSectionBodyNoLongerRendersFieldsStripLine is the B04.1
// regression pin: the separate "Fields:" strip line is gone -- neither the
// active nor the inactive render path may ever emit that substring again.
func TestRelationsSectionBodyNoLongerRendersFieldsStripLine(t *testing.T) {
	beans := []data.Bean{
		{ID: "nofs-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "nofs-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "nofs-parent"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["nofs-main"]

	for _, active := range []bool{true, false} {
		body, _, _ := relationsSectionBody(idx, main, 60, active, 0)
		if strings.Contains(body, "Fields:") {
			t.Errorf("active=%v: relationsSectionBody must never render a 'Fields:' strip line (B04, removed): %q", active, body)
		}
	}
}

// TestRelationsSectionBodyLongTitleAlignsContinuationUnderTitleStartNotColumnZero
// guards B04.3, the actual PO-Mockup bug: a Children-row title long enough
// to wrap must have its continuation line(s) start under the TITLE's own
// column (not column 0, and not the whole-row indent) -- design-spec.md §15
// PF-17's literal mockup ("t M bt-apmy Hier steht ein langer Titel... / der
// so umbricht, dass die Uebersicht gewahrt ist").
func TestRelationsSectionBodyLongTitleAlignsContinuationUnderTitleStartNotColumnZero(t *testing.T) {
	longTitle := "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist"
	beans := []data.Bean{
		{ID: "wrp-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "wrp-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "wrp-parent"},
		{ID: "wrp-child", Title: longTitle, Status: "todo", Type: "task", Priority: "normal", Parent: "wrp-main"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["wrp-main"]

	const bodyW = 40
	body, _, _ := relationsSectionBody(idx, main, bodyW, false, 0)
	stripped := ansi.Strip(body)

	groups := strings.Split(stripped, "\n\n")
	children := groups[len(groups)-1]
	lines := strings.Split(children, "\n")
	if lines[0] != "Children" {
		t.Fatalf("setup: last group must be Children, got %q", lines[0])
	}
	if len(lines) < 3 {
		t.Fatalf("expected the long child title to wrap into >=1 continuation line at bodyW=%d, got %d lines: %q", bodyW, len(lines), children)
	}
	titleCol := cellCol(t, lines[1], "Hier")
	for i, cl := range lines[2:] {
		indent := len(cl) - len(strings.TrimLeft(cl, " "))
		if indent == 0 {
			t.Errorf("continuation line %d fell back to column 0 (PO-Mockup B04.3 bug): %q", i, cl)
		}
		if indent != titleCol {
			t.Errorf("continuation line %d indent = %d, want %d (aligned under the title's own start)", i, indent, titleCol)
		}
	}
}

// --- bt-se4q (bt-b0w0-Review Follow-up, B01): numeric active-line index
// instead of activeRelationLine's post-render "▶" glyph rescan ---

// TestRelationsSectionBodyActiveLineIgnoresGlyphEmbeddedInTitle is the RED-
// anchor: a relation whose TITLE itself contains the "▶" active-marker
// glyph, rendered BEFORE the actually active row, must not fool the
// cursor-centering. The OLD activeRelationLine (view_browse_repo.go)
// rescanned the ALREADY-RENDERED lines for the first "▶" substring -- a
// title containing that glyph would win over the real marker.
// relationsSectionBody now hands back the numeric display-line index of the
// active row directly (accumulated while the groups are built, never
// rediscovered by scanning rendered text), so a glyph anywhere inside a
// title can never be mistaken for the marker.
func TestRelationsSectionBodyActiveLineIgnoresGlyphEmbeddedInTitle(t *testing.T) {
	beans := []data.Bean{
		{ID: "gl-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "gl-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "gl-parent"},
		// Sorts BEFORE gl-child-b (Title ascending, data.SortBeans) -- its
		// title carries the "▶" glyph the old glyph-rescan keyed on, and it
		// renders as an INACTIVE (▷) row, one line above the real cursor.
		{ID: "gl-child-a", Title: "AAA Glyph ▶ Title", Status: "todo", Type: "task", Priority: "normal", Parent: "gl-main"},
		{ID: "gl-child-b", Title: "BBB Real Child", Status: "todo", Type: "task", Priority: "normal", Parent: "gl-main"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["gl-main"]

	// fieldIdx 2 -> GLOBAL row order Parent=0, Children=1(child-a)/2(child-b):
	// the REAL active row is child-b, not the glyph-titled child-a.
	body, fields, activeLine := relationsSectionBody(idx, main, 80, true, 2)
	if len(fields) != 3 {
		t.Fatalf("setup: expected 3 relationFields (Parent + 2 Children), got %d", len(fields))
	}

	lines := strings.Split(body, "\n")
	if activeLine < 0 || activeLine >= len(lines) {
		t.Fatalf("activeLine %d out of range for %d body lines:\n%s", activeLine, len(lines), ansi.Strip(body))
	}
	gotLine := ansi.Strip(lines[activeLine])
	if !strings.HasPrefix(gotLine, "▶ ") {
		t.Fatalf("activeLine %d does not carry the real active marker, got %q\nfull body:\n%s", activeLine, gotLine, ansi.Strip(body))
	}
	if !strings.Contains(gotLine, "BBB Real Child") {
		t.Fatalf("activeLine %d must be the REAL active row (child-b), got %q", activeLine, gotLine)
	}

	// Guard the trap itself: a DIFFERENT, EARLIER line must contain "▶" too
	// (embedded in child-a's title) WITHOUT being marked active -- this is
	// exactly the line the old strings.Contains(l, "▶") scan would have
	// (wrongly) returned first.
	trapLineIdx := -1
	for i, l := range lines {
		if i == activeLine {
			continue
		}
		if strings.Contains(ansi.Strip(l), "▶") {
			trapLineIdx = i
			break
		}
	}
	if trapLineIdx == -1 {
		t.Fatalf("setup: expected an EARLIER line containing the glyph inside a title (the old scan's trap), found none:\n%s", ansi.Strip(body))
	}
	if trapLineIdx >= activeLine {
		t.Fatalf("setup: the glyph-in-title trap line (%d) must render BEFORE the real active line (%d) to prove the old first-match scan would have failed", trapLineIdx, activeLine)
	}
	if strings.HasPrefix(ansi.Strip(lines[trapLineIdx]), "▶ ") {
		t.Fatalf("setup: trap line %q must NOT itself start with the active marker (must be the inactive ▷ row)", ansi.Strip(lines[trapLineIdx]))
	}
}
