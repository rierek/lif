package tui

// view_detail_bean.go — the 4 fixed accordion sections for a bean (design-
// spec §6 V4, §15 PF-1/PF-3/PF-4/PF-7): META / BODY (glamour) / RELATIONS /
// HISTORY. Always all 4 (unlike devd's content-gated issueSections) -- there
// is no reason to hide an empty section; digit jump 1..4 stays meaningful for
// every bean. Section titles are uppercase English (PF-7, E7 T3 deliberately
// left these 4 to this task -- bt-w9o8 "Notes for T4"). META additionally
// carries a 5-entry field list (PF-4 + PF-15/D01) instead of its old 4-line
// status/type/priority/tags summary -- tags rejoined Meta as its own field
// (E8 Task 1, bean bt-e6q9, directly after priority). Shrunk from 7 to 5
// entries by bt-lg68 (PO-Nebenbefund, US-Review Runde 3): created_at/
// updated_at were doubly rendered in META AND HISTORY -- removed from META,
// HISTORY (historieSectionBody) stays the sole source.
//
// META's section-level open/closed behavior: originally PF-1 made META
// non-collapsible (always rendered its body). PF-18 REVISED this (design-
// spec.md §15, PO-Feedback 2026-07-16, bean bt-98cb) -- META now behaves
// exclusive-open like every other section (accordion.go's renderAccordion),
// default-closed until actively selected, since the relevant summary info
// already lives in the always-visible Meta-Strip header (detailHeaderBlock).

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// beanSectionCount is the Single Source for the fixed section count (T6,
// design-spec.md §15 PF-2: replaces two independently hardcoded '4' literals
// in update.go::keyDetailFocus and the former view_review_cockpit.go --
// removed by PF-14 -- with one shared constant).
const beanSectionCount = 4

// Section index constants (E8 Task 1, bean bt-e6q9, PF-16/B04 "Kleine
// Zusatzarbeit"): named indices into the 4 fixed sections beanSections
// returns, replacing Magic-Number section indices at call sites. Prepared
// for T6/B10 (bean bt-ntoz), which needs bodySectionIdx instead of a bare 1.
const (
	metaSectionIdx      = 0
	bodySectionIdx      = 1
	relationsSectionIdx = 2
	historySectionIdx   = 3
)

// Section titles (PF-7, uppercase English).
const (
	sectionTitleMeta      = "META"
	sectionTitleBody      = "BODY"
	sectionTitleRelations = "RELATIONS"
	sectionTitleHistory   = "HISTORY"
)

// beanSections builds the 4 fixed sections of the Detail-Accordion for b.
// bodyW is the inner body width (renderAccordion's box, already accounting
// for its own PaddingLeft/border -- callers pass the same width they'd hand
// renderAccordion). focused/activeIdx/fieldIdx are the SAME render-state
// triple renderAccordion itself takes (E7 T4, bean bt-kyj5): META's body
// needs to know whether IT is the currently active section to place its own
// ▶ field marker (PF-4) -- previously META's body was static text, agnostic
// of focus state. detailLevel (E8 Task 1, bean bt-e6q9, PF-16/B04) is a 4th
// gate: the ▶ marker must not appear merely because Meta IS the active
// section (activeIdx==0) -- it appears only once the field level has
// actually been entered (detailLevel==1), otherwise a bare `tab` already
// shows title: pre-selected before the user did anything at field level.
func beanSections(idx *data.Index, b *data.Bean, bodyW int, focused bool, activeIdx, fieldIdx, detailLevel int) []accordionSection {
	var secs []accordionSection
	secs = append(secs, accordionSection{
		title:  sectionTitleMeta,
		body:   metaSectionBody(b, bodyW, focused && activeIdx == 0 && detailLevel == 1, fieldIdx),
		fields: metaFields(b),
	})
	secs = append(secs, accordionSection{title: sectionTitleBody, body: bodySectionBody(b, bodyW)})
	// B04 (design-spec.md §15 PF-17, bean bt-b0w0): relationsSectionBody
	// grows the SAME (active, fieldIdx) pair metaSectionBody's own call
	// above takes -- mirrors accordion.go's own activeSec gate (`active &&
	// activeIdx == i`) verbatim, deliberately WITHOUT a detailLevel gate
	// (unlike Meta's `&& detailLevel == 1`): the removed fieldStrip showed
	// its active field as soon as the SECTION itself was active, regardless
	// of detailLevel -- this preserves that exact visibility timing, only
	// relocating WHERE the marker renders (per-row instead of a separate
	// strip), per this task's own "Pfeiltasten-Navigation ändert KEINEN
	// Code, nur die Visualisierung wechselt" scope.
	rel, fields, relActiveLine := relationsSectionBody(idx, b, bodyW, focused && activeIdx == relationsSectionIdx, fieldIdx)
	secs = append(secs, accordionSection{title: sectionTitleRelations, body: rel, fields: fields, activeLine: relActiveLine})
	secs = append(secs, accordionSection{title: sectionTitleHistory, body: historieSectionBody(b, bodyW)})
	return secs
}

// metaFieldLabels are the 5 fixed row labels of the Meta field list (PF-4 +
// PF-15/D01, E8 Task 1, bean bt-e6q9; shrunk from 7 to 5 by bt-lg68, PO-
// Nebenbefund US-Review Runde 3: created_at/updated_at were doubly rendered
// here AND in HISTORY -- removed from META, HISTORY stays the sole source).
// metaFieldLabelWidth (below) computes the shared padding width FROM this
// slice, not a hand-typed literal -- bt-lg68's own removal is exactly the
// case the plan flagged: "created_at:" used to be the longest label (12
// cells incl. padding), and hard-coding that width would have silently gone
// stale the moment it was removed.
var metaFieldLabels = [...]string{"title:", "status:", "type:", "priority:", "tags:"}

// metaFieldLabelWidth is metaFieldLabels' common left-pad width: one cell
// past the longest label ("priority:", 9 chars) -- computed here so a future
// change to metaFieldLabels can never let a hard-coded width literal drift
// out of sync again (bt-lg68 ripple-check).
var metaFieldLabelWidth = func() int {
	w := 0
	for _, l := range metaFieldLabels {
		if len(l) > w {
			w = len(l)
		}
	}
	return w + 1
}()

// metaFields builds the 5 kind-tagged Meta field entries (PF-4 + PF-15/D01,
// design-spec.md §15; shrunk from 7 to 5 by bt-lg68): title / status / type
// / priority / tags, in that fixed order. status/type/priority reuse T2's
// GLYPH-producing theme helpers (StatusIcon/TypeIcon/Priority) -- NOT the
// word, unlike the Kopfblock (detailHeaderBlock, below), which uses the word
// for type/status. tags (PF-15/D01, E8 Task 1) reuses the previously
// caller-less tagsInline/tagSwatch helper (render_shared.go) -- Hash-colored
// "● tag" swatches, or a theme.Dim "(none)" placeholder for an empty Tags
// slice. beanID is always "" (Meta fields are never Beziehungen-style jump
// targets); kind drives the enter-dispatch (status/type/priority/tags ->
// seeded Value-Menu / Tag-Picker, title -> Title-Edit-Form). created_at/
// updated_at (formerly two trailing "readonly" No-Op entries here) are
// REMOVED by bt-lg68 -- HISTORY (historieSectionBody, below) is now the
// sole renderer of Created/Updated; fmtTime stays shared between the two so
// their formatting can never drift apart again.
func metaFields(b *data.Bean) []relationField {
	priority := b.Priority
	if priority == "" {
		priority = "normal"
	}
	tags := tagsInline(b.Tags)
	if tags == "" {
		tags = theme.Dim.Render("(none)")
	}
	return []relationField{
		{kind: "title", label: b.Title},
		{kind: "status", label: theme.StatusIcon(b.Status)},
		{kind: "type", label: theme.TypeIcon(b.Type)},
		{kind: "priority", label: theme.Priority(priority)},
		{kind: "tags", label: tags},
	}
}

// metaSectionBody renders the 5-entry Meta field list (PF-4 + PF-15/D01;
// shrunk from 7 to 5 by bt-lg68): a ▷/▶ cursor column (PF-12: ALWAYS one or
// the other, never omitted -- the gutter is inherently stable by
// construction here, unlike the two accordion.go call sites PF-12 had to
// retrofit) followed by the field's label (padded to metaFieldLabelWidth)
// and its pre-rendered value (metaFields). ▶ (Accent) marks the row only
// when active is true AND fieldIdx matches that row's index -- mirrors
// render_shared.go's renderPane list-cursor convention (only the marker
// glyph is accent-tinted, not the whole row). Wrapped via wrapText like the
// section body always has been (B01, E2-T1-quality-review, MANDATORY
// carry-over) -- an overlong title must wrap instead of overflowing the
// Detail pane's bordered width.
func metaSectionBody(b *data.Bean, bodyW int, active bool, fieldIdx int) string {
	fields := metaFields(b)
	var lines []string
	for i, f := range fields {
		marker := theme.Muted.Render("▷ ") // B09 (bean bt-ntoz, PF-16): was unstilisiert (terminal-default white)
		if active && fieldIdx == i {
			marker = theme.Accent.Render("▶ ")
		}
		label := theme.Muted.Render(fmt.Sprintf("%-*s", metaFieldLabelWidth, metaFieldLabels[i]))
		lines = append(lines, marker+label+f.label)
	}
	return wrapText(strings.Join(lines, "\n"), bodyW)
}

// detailHeaderBlock renders the NEW Detail-Pane Kopfblock (PF-3+PF-4,
// verschmolzen laut PO-Antwort Q01, design-spec.md §15; PF-17/B05 (bean
// bt-mtig): bean-id / NAME / blank / "type: X    status: Y    prio: Z
// tags: <swatches>" / trailing blank, above the Accordion. type/status
// render as the colored WORD (theme.TypeStyle/StatusStyle applied to
// b.Type/b.Status) -- prio renders as the colored GLYPH (theme.Priority;
// PF-6 removed the word-producing counterpart, there is no PriorityStyle to
// apply to a word). tags is a 4th, appended column (PO-Mockup exact: "type:
// epic  status: in-progress  prio: !  tags: to-review") -- variable width is
// uncritical since it is the row's last column (no trailing padded field
// depends on it, unlike type/status which DO have a following column and
// therefore keep their fixed E8-B02 padding unchanged). Not the App-Chrome
// header (PO-Antwort Q01 explicit: "Kein separater App-Header-Umbau") --
// Detail-Pane only.
func detailHeaderBlock(b *data.Bean, w int) string {
	priority := b.Priority
	if priority == "" {
		priority = "normal"
	}
	// B02 (bean bt-ntoz, design-spec.md §15 PF-16): type/status are padded to
	// the longest legal word BEFORE the TypeStyle/StatusStyle color is
	// applied (9 = len("milestone"), 11 = len("in-progress")) -- otherwise
	// the row's total width (and thus "status:"/"prio:"'s column start)
	// shifts on every bean-change depending on word length. prio stays a
	// GLYPH (theme.Priority), never a variable-length word, so it needs no
	// padding of its own.
	typeWord := fmt.Sprintf("%-9s", b.Type)
	statusWord := fmt.Sprintf("%-11s", b.Status)
	// tags mirrors metaFields' own tagsInline/"(none)" fallback (render_
	// shared.go/view_detail_bean.go) verbatim -- kept as a small duplicate
	// here (not extracted into a shared helper) because the two call sites
	// have unrelated surrounding logic; if either changes, update both (see
	// metaFields above).
	tags := tagsInline(b.Tags)
	if tags == "" {
		tags = theme.Dim.Render("(none)")
	}
	typeStatusPrio := theme.Muted.Render("type: ") + theme.TypeStyle(b.Type).Render(typeWord) +
		theme.Muted.Render("    status: ") + theme.StatusStyle(b.Status).Render(statusWord) +
		theme.Muted.Render("    prio: ") + theme.Priority(priority) +
		theme.Muted.Render("    tags: ") + tags
	lines := []string{
		truncate(theme.Key.Render(b.ID), w),
		truncate(theme.Header.Render(b.Title), w),
		"",
		truncate(typeStatusPrio, w),
		"",
	}
	return strings.Join(lines, "\n")
}

// bodySectionBody renders b.Body via glowRender (glamour), with a muted
// placeholder for an empty body -- digit-jump 2 must always show something.
func bodySectionBody(b *data.Bean, bodyW int) string {
	if strings.TrimSpace(b.Body) == "" {
		return theme.Dim.Render("(no body)")
	}
	return glowRender(b.Body, bodyW)
}

// hangingIndentWrap wraps text to width w, with the FIRST line prefixed by
// prefix (already-styled; its VISIBLE width via lipgloss.Width(ansi.Strip(
// prefix)) becomes the indent) and every CONTINUATION line left-padded with
// that many spaces instead of the wrap restarting at column 0 (B04.3,
// design-spec.md §15 PF-17) -- indent is computed PER ROW (not a shared
// table column across all rows: bean IDs have variable prefix length,
// "<prefix>-<n-chars>"). Fixes the actual PO-Mockup bug: relationRow's
// former caller wrapped the ENTIRE joined Relations block with one blanket
// wrapText() call, so a long title's continuation line had no notion of its
// own row's prefix and fell back to column 0, "unterwandering" the
// following row's Meta-Spalten (glyph/status/ID). Reused verbatim by Task 5/
// bt-4mo9's Relations-Picker (structurally identical glyph+ID+title row) --
// kept as an independent, unit-tested helper (view_detail_bean_test.go) for
// that reason, not folded into relationRow's own body.
func hangingIndentWrap(prefix, text string, w int) string {
	indentW := lipgloss.Width(ansi.Strip(prefix))
	contW := w - indentW
	if contW < 8 {
		contW = 8 // never collapse to nothing on narrow terminals
	}
	// Two-pass wrap, mirrors wrapText (view.go) exactly (F01, bt-b0w0
	// Fix-Runde 1): Wordwrap alone only breaks at whitespace -- a single
	// spaceless token (URL, German compound, long ID, CJK prose) longer
	// than contW would overflow the target width unbounded. Hardwrap
	// force-breaks such over-long tokens at the cell boundary (true =
	// preserveSpace, wie wrapText), same as the pre-B04 blanket wrapText did.
	wrapped := ansi.Hardwrap(ansi.Wordwrap(text, contW, ""), contW, true)
	lines := strings.Split(wrapped, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i == 0 {
			b.WriteString(prefix + line)
		} else {
			b.WriteString("\n" + strings.Repeat(" ", indentW) + line)
		}
	}
	return b.String()
}

// relationRowMarker renders a row's ▷/▶ cursor prefix (B04, design-spec.md
// §15 PF-17): mirrors metaSectionBody's own per-row convention (PF-4) --
// ▶ (Accent) only when active is true AND fieldIdx equals this row's
// position in the GLOBAL field order relationsSectionBody accumulates
// (Parent=0, Children=1..n, Blocking=n+1..m, Blocked By=m+1..k, the SAME
// order fields[] already carried before this task), ▷ (Muted) otherwise.
func relationRowMarker(active bool, fieldIdx, rowIdx int) string {
	if active && fieldIdx == rowIdx {
		return theme.Accent.Render("▶ ")
	}
	return theme.Muted.Render("▷ ")
}

// relationRowPrefix returns the glyph+ID half of a relation row (status
// icon, type icon, Key-styled ID, trailing space) WITHOUT a leading marker
// and WITHOUT wrapping -- extracted out of relationRow (T5, bean bt-4mo9,
// B06) so the Blocking-/Parent-Picker (box_picker_blocking.go/box_picker_
// parent.go) can compose their OWN leading element (the D08 cursor bar and,
// for Blocking, the pending dot) in front of it and hand the combined
// string to hangingIndentWrap as ITS prefix argument, against the REAL
// picker width (wideModalWidth(m.width)) instead of the old relationRowNoWrap
// sentinel (removed by this task -- no caller left after this refactor).
// relationRow (below) is now a thin wrapper around this plus its own
// `marker` parameter, byte-identical to its pre-extraction behavior.
//
// slug (bean bt-pt1r) is the same repo prefix treeRowText/backlogRowText
// already strip via shortBeanID (view_browse_repo.go, bean bt-pl5p) --
// threaded in as a PARAMETER rather than resolved here, because the two
// callers want opposite things and the difference is a deliberate design
// decision, not an accident:
//
//   - the Blocking-/Parent-Picker pass the live m.beanIDPrefix(): their rows
//     wrap at wideModalWidth, so the redundant "<slug>-" costs exactly the
//     columns the title needs -- and leaving it long there made the app
//     contradict ITSELF (short in the left pane, long in the picker).
//   - relationRow (the DETAIL pane) passes "" and keeps the full ID:
//     bt-pl5p's decision that the complete, copyable ID must remain readable
//     somewhere. Do not "unify" this to the live slug.
//
// A foreign/dangling ID (no such prefix) is untouched either way -- that is
// shortBeanID's own contract.
func relationRowPrefix(rel *data.Bean, slug string) string {
	return theme.StatusIcon(rel.Status) + " " + theme.TypeIcon(rel.Type) + " " + theme.Key.Render(shortBeanID(rel.ID, slug)) + " "
}

// relationRow renders one resolved relation as a marker-prefixed status-
// icon+type-icon+ID+title row (mirrors treeRowText's glyph order, view_
// browse_repo.go), wrapped via hangingIndentWrap (B04.3) so a long title's
// continuation lines align under the title's own start on THIS row instead
// of falling back to column 0. Passes slug "" on purpose -- see
// relationRowPrefix's own note: the Detail pane keeps the FULL ID (bt-pl5p).
func relationRow(rel *data.Bean, marker string, w int) string {
	return hangingIndentWrap(marker+relationRowPrefix(rel, ""), rel.Title, w)
}

// resolveSorted resolves a slice of bean IDs against idx.ByID, canonically
// sorting the resolved beans (data.SortBeans -- I03) and appending unresolved
// IDs (dangling references, beans-legal) as their own relationField with
// beanID == "" (not jumpable). Returns the rendered body lines plus the
// relationFields in the same order (parallel to the rendered rows).
// startIdx/active/fieldIdx (B04, design-spec.md §15 PF-17) thread the ONE
// GLOBAL cursor position (relationsSectionBody's running row counter) through
// this group's rows, mirrored into relationRowMarker per row; w is the body
// width handed to relationRow's hangingIndentWrap. nextIdx is startIdx plus
// the number of rows this call renders, letting the caller chain it into the
// NEXT group's startIdx.
// activeOffset (bt-se4q, bt-b0w0-Review Follow-up B01) is the DISPLAY-LINE
// offset of the active row within this call's own `lines` slice (0-based,
// counting each row's own hangingIndentWrap-produced line count -- NOT a
// row/relation count), or -1 if none of this group's rows is the active one.
// Threaded up through relationsSectionBody (below) into its own `activeLine`
// return value -- replaces the removed activeRelationLine's post-render
// "▶"-glyph rescan (view_browse_repo.go) with an index known exactly at
// construction time, so a relation TITLE that happens to contain "▶"/"▷"
// can never be mistaken for the real cursor.
func resolveSorted(idx *data.Index, ids []string, startIdx int, active bool, fieldIdx, w int) (lines []string, fields []relationField, nextIdx, activeOffset int) {
	var resolved []*data.Bean
	var unresolved []string
	for _, id := range ids {
		if rel, ok := idx.ByID[id]; ok {
			resolved = append(resolved, rel)
		} else {
			unresolved = append(unresolved, id)
		}
	}
	data.SortBeans(resolved)
	rowIdx := startIdx
	activeOffset = -1
	lineOffset := 0
	for _, rel := range resolved {
		line := relationRow(rel, relationRowMarker(active, fieldIdx, rowIdx), w)
		if active && fieldIdx == rowIdx {
			activeOffset = lineOffset
		}
		lines = append(lines, line)
		fields = append(fields, relationField{beanID: rel.ID, label: theme.Key.Render(rel.ID) + " " + rel.Title})
		lineOffset += strings.Count(line, "\n") + 1
		rowIdx++
	}
	for _, id := range unresolved {
		// text (unlike relationRow's resolved rows above) carries no
		// hangingIndentWrap of its own -- unresolved rows are a short fixed
		// "(unresolved: <id>)" string, never realistically long enough to
		// wrap, mirrors the pre-B04 behavior verbatim. The marker is
		// prefixed onto the RENDERED line only, never onto fields[].label
		// (label stays marker-free -- unused since fieldStrip's removal, but
		// kept byte-identical to its pre-B04 contract for any future reader/
		// reuse, e.g. TestBeanSectionsRelationsDanglingReferenceShowsUnresolvedNotJumpable).
		text := theme.Dim.Render("(unresolved: " + id + ")")
		line := relationRowMarker(active, fieldIdx, rowIdx) + text
		if active && fieldIdx == rowIdx {
			activeOffset = lineOffset
		}
		lines = append(lines, line)
		fields = append(fields, relationField{beanID: "", label: text})
		lineOffset++ // unresolved rows never wrap (fixed short text, see comment above)
		rowIdx++
	}
	return lines, fields, rowIdx, activeOffset
}

// beanListRow renders an already-resolved bean list (e.g. idx.Children,
// pre-sorted -- no dangling references possible there) the same way
// resolveSorted renders its resolved half. startIdx/active/fieldIdx/w mirror
// resolveSorted's own B04 parameters; activeOffset mirrors resolveSorted's
// own bt-se4q addition (display-line offset of the active row within this
// call's `lines`, -1 if not present here).
func beanListRow(beans []*data.Bean, startIdx int, active bool, fieldIdx, w int) (lines []string, fields []relationField, nextIdx, activeOffset int) {
	rowIdx := startIdx
	activeOffset = -1
	lineOffset := 0
	for _, rel := range beans {
		line := relationRow(rel, relationRowMarker(active, fieldIdx, rowIdx), w)
		if active && fieldIdx == rowIdx {
			activeOffset = lineOffset
		}
		lines = append(lines, line)
		fields = append(fields, relationField{beanID: rel.ID, label: theme.Key.Render(rel.ID) + " " + rel.Title})
		lineOffset += strings.Count(line, "\n") + 1
		rowIdx++
	}
	return lines, fields, rowIdx, activeOffset
}

// relationsSectionBody builds the Beziehungen section body + its jump-only
// relationFields: Parent (0 or 1, via idx.ByID) / Children (idx.Children[b.ID],
// already sorted) / Blocking / Blocked By (each resolved via idx.ByID, then
// data.SortBeans -- I03), grouped under muted sub-headers. Groups with no
// entries are omitted entirely.
//
// active/fieldIdx (B04, design-spec.md §15 PF-17, bean bt-b0w0) mirror
// metaSectionBody's own (active bool, fieldIdx int) signature (PF-4): every
// rendered row now carries its own ▷/▶ cursor marker (relationRowMarker)
// instead of the removed fieldStrip. gi is the ONE running row counter
// threaded across ALL four groups (Parent=0, Children=1..n, Blocking=
// n+1..m, Blocked By=m+1..k) -- the SAME order fields[] already accumulated
// in before this task (appendGroup hangs lines/fields on in lockstep,
// UNCHANGED), so a row's position in the returned fields slice and its
// on-screen ▶ marker always agree (keyDetailFocus's existing fieldCursor
// navigation, update.go, is not touched by this task).
//
// Each row is wrapped via hangingIndentWrap (B04.3) BEFORE the groups are
// joined -- the former blanket wrapText(strings.Join(groups, "\n\n"), bodyW)
// at the very end is GONE: that call was the actual B04.3 bug (a long title
// wrapped back to column 0, "unterwandering" the Meta-Spalten of whatever
// followed) since it re-wrapped already-composed rows with no notion of
// their own per-row prefix width. wrapText is no longer needed for the
// short, fixed subheader strings either (Parent/Children/Blocking/Blocked
// By never realistically overflow bodyW).
//
// activeLine (3rd return value, bt-se4q, bt-b0w0-Review Follow-up B01)
// carries the DISPLAY-LINE index (0-based, into strings.Split(body, "\n"))
// of the active row's own marker line within the returned body -- accumulated
// as the groups are built (subheader lines + each row's own hangingIndentWrap
// line count + the blank-line separator strings.Join(groups, "\n\n") inserts
// between groups all count), Default 0 when no row is active (mirrors the
// removed activeRelationLine's own "no active marker -> top" default).
// Replaces windowRelationsSection's post-render activeRelationLine glyph
// rescan (view_browse_repo.go) -- the caller now has the exact index instead
// of re-deriving it from already-rendered text, which could be fooled by a
// "▶"/"▷" character living inside a relation's own TITLE (Reviewer-Beleg,
// bt-b0w0-Review).
func relationsSectionBody(idx *data.Index, b *data.Bean, bodyW int, active bool, fieldIdx int) (string, []relationField, int) {
	var groups []string
	var fields []relationField
	gi := 0
	displayLine := 0
	activeLine := 0

	appendGroup := func(title string, lines []string, fs []relationField, activeOffset int) {
		if len(lines) == 0 {
			return
		}
		if len(groups) > 0 {
			displayLine++ // strings.Join(groups, "\n\n")'s own blank-line separator
		}
		displayLine++ // this group's subheader line ("Parent"/"Children"/...)
		if activeOffset >= 0 {
			activeLine = displayLine + activeOffset
		}
		for _, l := range lines {
			displayLine += strings.Count(l, "\n") + 1
		}
		groups = append(groups, theme.Muted.Render(title)+"\n"+strings.Join(lines, "\n"))
		fields = append(fields, fs...)
	}

	if b.Parent != "" {
		parentActiveOffset := -1
		if active && fieldIdx == gi {
			parentActiveOffset = 0
		}
		if parent, ok := idx.ByID[b.Parent]; ok {
			line := relationRow(parent, relationRowMarker(active, fieldIdx, gi), bodyW)
			appendGroup("Parent", []string{line}, []relationField{{beanID: parent.ID, label: theme.Key.Render(parent.ID) + " " + parent.Title}}, parentActiveOffset)
		} else {
			text := theme.Dim.Render("(unresolved: " + b.Parent + ")")
			appendGroup("Parent", []string{relationRowMarker(active, fieldIdx, gi) + text}, []relationField{{beanID: "", label: text}}, parentActiveOffset)
		}
		gi++
	}

	if children := idx.Children[b.ID]; len(children) > 0 {
		lines, fs, next, activeOffset := beanListRow(children, gi, active, fieldIdx, bodyW)
		appendGroup("Children", lines, fs, activeOffset)
		gi = next
	}

	if len(b.Blocking) > 0 {
		lines, fs, next, activeOffset := resolveSorted(idx, b.Blocking, gi, active, fieldIdx, bodyW)
		appendGroup("Blocking", lines, fs, activeOffset)
		gi = next
	}

	if len(b.BlockedBy) > 0 {
		lines, fs, next, activeOffset := resolveSorted(idx, b.BlockedBy, gi, active, fieldIdx, bodyW)
		appendGroup("Blocked By", lines, fs, activeOffset)
		gi = next
	}

	if len(groups) == 0 {
		return theme.Dim.Render("(no relations)"), nil, 0
	}
	return strings.Join(groups, "\n\n"), fields, activeLine
}

// fmtTime formats a nullable timestamp for display: a muted placeholder for
// nil instead of Go's zero-time string, "2006-01-02 15:04" otherwise. Shared
// by historieSectionBody (Created/Updated) and metaFields (created_at/
// updated_at, PF-4, E7 T4) -- extracted from historieSectionBody's former
// local closure so both call sites stay byte-identical, not two
// re-implementations that could drift.
func fmtTime(t *time.Time) string {
	if t == nil {
		return theme.Dim.Render("(unknown)")
	}
	return t.Format("2006-01-02 15:04")
}

// historieSectionBody renders CreatedAt/UpdatedAt/ETag. nil timestamps show a
// muted placeholder instead of Go's zero-time string. B01 (E2-T1-quality-
// review, MANDATORY carry-over into Task 2): wrapped via wrapText -- a long
// ETag must wrap instead of overflowing the Detail pane's bordered width.
func historieSectionBody(b *data.Bean, bodyW int) string {
	var lines []string
	lines = append(lines, theme.Muted.Render("Created: ")+fmtTime(b.CreatedAt))
	lines = append(lines, theme.Muted.Render("Updated: ")+fmtTime(b.UpdatedAt))
	etag := b.ETag
	if etag == "" {
		etag = theme.Dim.Render("(none)")
	}
	lines = append(lines, theme.Muted.Render("ETag: ")+etag)
	return wrapText(strings.Join(lines, "\n"), bodyW)
}
