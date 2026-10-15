package tui

// box_picker_tag.go — the Tag-Picker (`t`, E3 Task 2, bean bt-8v69):
// usage-counted toggle-multi-select over every tag currently in use, plus
// (bean bt-9ipw, US-07-Reopen 2026-07-17, epic-E12-plan.md »Item 1«, D01) an
// ALWAYS-visible, always-focused search field that live-filters the same
// row list. Port pattern (design decision, plan »Task 2«): counter + sort
// semantics from beans-src tagpicker.go:64-96 (sort count desc, then
// alpha); the Pending-Diff/Enter-confirms/Esc-discards semantics from
// beans-src blockingpicker.go:197-245 (space toggles pending, enter diffs
// pending against original, esc discards) -- this is the FIRST beans-tui
// overlay carrying real mutation state across multiple keystrokes before a
// single confirm, so it is deliberately NOT box_filter_facets.go's
// keyFilterMenu pattern (whose enter only closes, since facets act live and
// there is nothing to "confirm"). The search field mirrors keySearchInput's
// "one persistent textinput.Model, reset+focused on open, every key but
// enter/esc belongs to the input" convention (update.go:520-549) -- EXCEPT
// that here space (toggle) and Up/Down (navigation) ALSO stay intercepted
// ahead of the input, since multi-select must keep working while the field
// is focused (D01, the very acceptance criterion this bean was reopened
// for). Toggle is space ONLY -- NOT keys.Toggle's "x" alias (ERRATUM/
// D01-Nachtrag, Review-R1 B01: "x" must stay typeable; see keyTagPicker).
//
// ERRATUM vs. the plan's own EARLIER Step-1 test sketch ("+1 Tag, -1 Tag ->
// 2 Mutationen (tea.Batch)"): superseded by the plan's own "Design-Nachtrag"
// immediately below it (verbindlich für die Implementierung) -- N sequential
// AddTag/RemoveTag mutateCmds against ONE etag would be a conflict cascade
// (the first call rotates the etag on disk, every subsequent call then sees
// a stale etag and fails ErrConflict). applyTagPickerDiff below fires
// exactly ONE mutateCmd wrapping data.SetTags (mutations.go), which builds
// ONE `beans update` invocation carrying every added/removed tag as
// repeated --tag/--remove-tag flags -- one etag, no cascade.
//
// Consolidation history (bean bt-9ipw, D01): the picker used to be TWO
// separate overlay states -- this Haupt-Picker (pure toggle, no textinput
// at all, "Tippen tut nichts") and a SEPARATE `n`-gated free-text sub-mode
// (tagInputActive/openTagInput/keyTagInput/tagInputBox) that carried the
// only visible search field. PO-Review 2026-07-17 (US-07) rejected that
// split: typing directly in the Haupt-Picker (the entry point the PO
// actually used) gave zero visual feedback. D01 merges both into ONE mode:
// this file's search field/filter/cursor are now the Haupt-Picker's own,
// always on, no second gate -- the former sub-mode's functions are gone,
// their logic lives in keyTagPicker/tagPickerBox below.

import (
	"fmt"
	"sort"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// tagCount is one usage-counted row of the tag picker's menu. defined marks
// a row whose tag is registered in the repo-local Tag-Registry (T6, bean
// bt-pqq3, epic bt-362n D10 Suggest-Mode) -- as opposed to a "free" tag that
// merely sits on some bean but was never defined there. Suggest-Mode is
// display/sort-only: a free row stays exactly as togglable/savable as
// before (kein strict mode, PO-Vorgabe wörtlich).
type tagCount struct {
	tag     string
	count   int
	defined bool
}

// collectTagCounts counts each tag's usage across every bean in idx and
// marks every tag present in defined as a registry-defined row (T6, D10).
// The returned set is the UNION of every tag currently in use AND every
// registry-defined tag -- a defined tag with ZERO current usage still
// appears (count 0, defined=true), mirroring D09's "auch mit Count 0"
// wording one layer up (view_tag_management.go's tagRegistryRows): the
// Suggest-Mode picker offers every defined tag as a candidate even before it
// has ever been applied to a bean (this task's own tmux-Smoke acceptance
// wording "unbenutzt-definierte sichtbar"). defined may be nil (e.g. no
// client yet, or a missing/corrupt registry file, D02 tolerant-missing) --
// a nil map read always returns false, so every row is simply "free" in
// that case, identical to this function's pre-D10 behavior.
//
// Sort (sortTagCountsDefinedFirst): "defined" is the NEW PRIMARY key (D10 --
// defined tags always sort before free tags, regardless of count); the
// pre-existing count-desc/alpha tie-break stays SECONDARY/TERTIARY,
// unchanged WITHIN each of the two groups.
//
// Determinism note (the same ERRATUM lesson box_filter_facets.go's
// tagFilterOptions documents, but resolved more simply here): idx.ByID is a
// Go map with no defined iteration order, so the intermediate counts map
// built by ranging over it is itself order-dependent -- but the FINAL sort
// key (defined, then count desc, then tag name alpha) fully determines row
// order on its own, because tag names are unique within the counts map (no
// two rows can tie on all three). Unlike tagFilterOptions (which sorts whole
// *Bean values that CAN tie completely on every sort field), no baseline
// pre-sort is needed here -- the map-walk order that built the counts never
// leaks into the returned slice's order.
func collectTagCounts(idx *data.Index, defined map[string]bool) []tagCount {
	counts := map[string]int{}
	if idx != nil {
		for _, b := range idx.ByID {
			for _, t := range b.Tags {
				if t == "" {
					continue
				}
				counts[t]++
			}
		}
	}

	out := make([]tagCount, 0, len(counts)+len(defined))
	seen := make(map[string]bool, len(counts))
	for tag, n := range counts {
		out = append(out, tagCount{tag: tag, count: n, defined: defined[tag]})
		seen[tag] = true
	}
	for tag := range defined {
		if seen[tag] {
			continue
		}
		out = append(out, tagCount{tag: tag, count: 0, defined: true})
	}
	sortTagCountsDefinedFirst(out)
	return out
}

// sortTagCountsDefinedFirst sorts items in place: defined rows first (D10
// NEW primary key), then count descending, then tag name alpha -- the SAME
// comparator collectTagCounts and keyTagInput's new-tag insert path (both
// build/maintain a []tagCount slice that must stay consistently ordered)
// share, so the two call sites can never drift apart on what "sorted" means.
func sortTagCountsDefinedFirst(items []tagCount) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].defined != items[j].defined {
			return items[i].defined
		}
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].tag < items[j].tag
	})
}

// tagItemIndex returns items' index for tag, or -1 if not present -- shared
// by the toggle handler and the new-tag submit's dedupe check.
func tagItemIndex(items []tagCount, tag string) int {
	for i, it := range items {
		if it.tag == tag {
			return i
		}
	}
	return -1
}

// openTagPicker opens the Tag-Picker on the focused bean (the
// focusedBean()!=nil guard lives in the caller, keyNodeAction -- same
// convention as openValueMenu). tagOriginal/tagPending are seeded as two
// INDEPENDENT maps (design decision, plan »Task 2«) from the focused bean's
// CURRENT tags -- wholesale-replace at open time, mirroring searchBleveIDs'
// "always a brand-new map value" convention (types.go doc-stamp). mutTarget
// captures the bean ID only, never the etag (design decision d) -- a
// watch-reload between open and the eventual enter is honored automatically
// via beanETag.
//
// D03/D10 (T6, bean bt-pqq3, epic bt-362n): the Tag-Registry is loaded
// FRESH from disk on EVERY open (mirrors openTagManagementPage's own
// "reload from disk on every open" convention, view_tag_management.go) -- a
// nil m.client (pre-load/test fixture) degrades to an empty registry rather
// than panicking, same D02 tolerant-missing philosophy one layer up.
// tagItems is then built via collectTagCounts' Suggest-Mode union: defined
// tags sort first, but every free tag stays fully togglable regardless
// (kein strict mode, PO-Vorgabe wörtlich).
//
// Consolidated Search (bean bt-9ipw, D01): the search field is focused
// IMMEDIATELY on open, no second `n` gate -- tagInputFiltered seeds from an
// EMPTY query (filterTagItems treats "" as "match everything", mirrors
// filteredRepos()'s own contract, view_lobby.go), so every row is visible
// and togglable from the very first frame, tagInputSuggestCursor at the top
// row. This REPLACES the old m.menu-driven cursor for this overlay (D01's
// own acceptance: Pfeiltasten navigate the -- now live-filterable -- list).
// Returns textinput.Blink (mirrors openSearchInput's own convention,
// update.go) so the now-always-focused field's cursor actually blinks.
func (m model) openTagPicker() (tea.Model, tea.Cmd) {
	b := m.focusedBean()
	if b == nil {
		return m, nil
	}
	m.mutTarget = b.ID

	var defs []string
	if m.client != nil {
		defs, _ = m.client.LoadTagDefs() // D02: LoadTagDefs never returns a non-nil error
	}
	defined := make(map[string]bool, len(defs))
	for _, name := range defs {
		defined[name] = true
	}
	m.tagItems = collectTagCounts(m.idx, defined)

	orig := make(map[string]bool, len(b.Tags))
	for _, t := range b.Tags {
		orig[t] = true
	}
	pending := make(map[string]bool, len(orig))
	for t := range orig {
		pending[t] = true
	}
	m.tagOriginal = orig
	m.tagPending = pending

	m.tagInputErr = ""
	m.tagInput.SetValue("")
	m.tagInput.Focus()
	m.tagInputFiltered = filterTagItems(m.tagItems, "")
	m.tagInputSuggestCursor = 0

	m.overlay = overlayTagPicker
	return m, textinput.Blink
}

// filterTagItems returns the subset of items whose tag contains query as a
// case-insensitive substring (strings.Contains, deliberately NO fuzzy
// scoring/Bleve -- YAGNI, bean bt-9ipw's own "Nicht jetzt" section). An
// empty (or whitespace-only) query matches every row, preserving items' own
// order -- items is already sorted via sortTagCountsDefinedFirst at every
// call site (collectTagCounts/the create-path's own insert in
// tagPickerEnter below), so the filtered list's order stays
// defined-first/count-desc/alpha throughout.
func filterTagItems(items []tagCount, query string) []tagCount {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return append([]tagCount(nil), items...)
	}
	out := make([]tagCount, 0, len(items))
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.tag), q) {
			out = append(out, it)
		}
	}
	return out
}

// keyTagPicker drives the open Tag-Picker's now-consolidated single mode
// (bean bt-9ipw, D01): Up/Down navigate tagInputSuggestCursor over the
// live-filtered tagInputFiltered, clamped to its bounds -- intercepted via
// the RAW tea.KeyUp/tea.KeyDown KeyType, NEVER via navKey's letter-alias
// table ("i"/"k" must stay literal, typeable characters here, e.g. a tag
// named "risk"; keyLobby swallowing them in its repoQuery filter is an
// EXISTING BUG, bean bt-l8e7, not a precedent -- full rationale in
// types.go's own doc-stamp). Esc/Enter are matched via keybind so a future
// rebind of either stays correct here automatically; toggle is the literal
// SPACE character only (ERRATUM/D01-Nachtrag, Review-R1 B01 -- inline
// rationale at the case below). All three are intercepted AHEAD of the
// textinput for the same reason Up/Down are (multi-select must keep working
// while the field is focused, the acceptance criterion this bean was
// reopened for -- D01 explicitly keeps this "unverändert"). Every other key
// falls through to
// m.tagInput.Update(msg); tagInputFiltered/tagInputSuggestCursor are
// recomputed ONLY when that Update actually changed the input's value
// (mirrors keyLobby's own prev/current repoQuery-changed guard,
// view_lobby.go) -- a value-preserving keystroke (e.g. left/right cursor
// movement inside the textinput) must not reset the cursor the user just
// navigated to.
func (m model) keyTagPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if len(m.tagInputFiltered) > 0 {
			m.tagInputSuggestCursor--
			if m.tagInputSuggestCursor < 0 {
				m.tagInputSuggestCursor = 0
			}
		}
		return m, nil
	case tea.KeyDown:
		if len(m.tagInputFiltered) > 0 {
			m.tagInputSuggestCursor++
			if m.tagInputSuggestCursor >= len(m.tagInputFiltered) {
				m.tagInputSuggestCursor = len(m.tagInputFiltered) - 1
			}
		}
		return m, nil
	}

	switch {
	case keybind.Matches(msg, keys.Back):
		m.overlay = overlayNone
		m.tagInput.Blur()
		return m, nil
	case keybind.Matches(msg, keys.TagToggle):
		// ERRATUM/D01-Nachtrag (Review-R1 B01, Supervisor-Entscheid): toggle
		// is deliberately the space-only keys.TagToggle here -- NOT
		// keys.Toggle, because keys.Toggle also binds "x" (keymap.go), and
		// intercepting "x" ahead of the textinput made it a NEVER-typeable
		// character (tags like nginx/linux/unix/box were silently
		// unfilterable and uncreatable). Space is safe to reserve:
		// data.ValidTagName never admits a space, so no tag name is lost.
		// Same "typeability of a common letter beats alias redundancy"
		// rationale as the raw-KeyType i/k intercept above. The plan's
		// "space/x togglet" wording is thereby consciously narrowed to space
		// for THIS overlay (filter menu/blocking picker keep both) -- full
		// rationale at keys.TagToggle's own keymap.go doc-stamp.
		return m.toggleTagPending(), nil
	case keybind.Matches(msg, keys.Enter):
		return m.tagPickerEnter()
	}

	prev := m.tagInput.Value()
	var cmd tea.Cmd
	m.tagInput, cmd = m.tagInput.Update(msg)
	if v := m.tagInput.Value(); v != prev {
		m.tagInputFiltered = filterTagItems(m.tagItems, v)
		m.tagInputSuggestCursor = 0
	}
	return m, cmd
}

// toggleTagPending flips the row AT tagInputSuggestCursor's membership in
// m.tagPending -- since D01, the cursor moves over the live-filtered
// tagInputFiltered (formerly the unfiltered tagItems via m.menu.cursor), so
// toggling a row narrowed down by typing "toggles what's ON SCREEN", not a
// now-stale full-list position. I01 (types.go doc-stamp, bean bt-7jr8):
// clones via cloneBoolMap before writing -- tagPending is COPY-ON-WRITE for
// every toggle during the picker's (potentially multi-keystroke) open
// session, same convention as toggleFacet (box_filter_facets.go), even
// though the map itself is wholesale-replaced fresh on each OPEN
// (openTagPicker's doc-stamp).
func (m model) toggleTagPending() model {
	if m.tagInputSuggestCursor < 0 || m.tagInputSuggestCursor >= len(m.tagInputFiltered) {
		return m
	}
	tag := m.tagInputFiltered[m.tagInputSuggestCursor].tag
	out := cloneBoolMap(m.tagPending)
	if out[tag] {
		delete(out, tag)
	} else {
		out[tag] = true
	}
	m.tagPending = out
	return m
}

// tagPickerEnter is keyTagPicker's enter dispatch (bean bt-9ipw, D01):
// UNCHANGED from before Typeahead ever existed whenever tagInputFiltered has
// at least one row -- INCLUDING the empty-query "every tag" state -- it
// simply applies the pending diff and closes (applyTagPickerDiff, below).
// The NEW branch only fires when the typed query has NO substring match at
// all (tagInputFiltered empty) AND something was actually typed: that is
// D11's create-path, unchanged in spirit from the pre-bt-9ipw free-text
// sub-mode -- data.ValidTagName gates the submit (invalid -> tagInputErr
// set, picker STAYS open for a retry), a valid name is appended to tagItems
// (deduped) and set pending=true. Unlike the old sub-mode, this does NOT
// close the picker -- it clears the query back to the full (now one-larger)
// list so the PO can keep toggling/creating more tags in the same session,
// consistent with space-toggle's own "keep going" semantics.
func (m model) tagPickerEnter() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.tagInput.Value())
	if len(m.tagInputFiltered) == 0 && name != "" {
		if !data.ValidTagName(name) {
			m.tagInputErr = "invalid tag name (a-z0-9, hyphen-separated, lowercase)"
			return m, nil
		}
		m.tagInputErr = ""

		if tagItemIndex(m.tagItems, name) < 0 {
			items := append([]tagCount(nil), m.tagItems...)
			// defined: false (zero value) -- B14's free-text new-tag path stays
			// UNCHANGED (Q03 in the epic body, bt-362n: NOT part of this task,
			// no Registry-write from here).
			items = append(items, tagCount{tag: name, count: 0})
			sortTagCountsDefinedFirst(items)
			m.tagItems = items
		}
		out := cloneBoolMap(m.tagPending)
		out[name] = true
		m.tagPending = out

		m.tagInput.SetValue("")
		m.tagInputFiltered = filterTagItems(m.tagItems, "")
		if idx := tagItemIndex(m.tagItems, name); idx >= 0 {
			m.tagInputSuggestCursor = idx
		} else {
			m.tagInputSuggestCursor = 0
		}
		return m, nil
	}

	return m.applyTagPickerDiff()
}

// applyTagPickerDiff computes tagPending's diff against tagOriginal and
// fires it as ONE combined data.SetTags mutateCmd (design decision, see
// this file's doc comment) against a FRESH etag read from the live index
// (m.beanETag, design decision d -- never a captured copy). No pending
// changes at all -> no Cmd (mirrors applyValueMenuSelection's vanished-
// target guard: ok==false means mutTarget disappeared from the index
// between open and this enter -- the overlay still closes, with a
// status-line note, but no doomed mutation fires). add/remove are sorted
// before dispatch so the underlying CLI invocation's flag order (and thus
// any test/log asserting on it) is deterministic despite the map-iteration
// order the diff loops below walk in.
func (m model) applyTagPickerDiff() (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	id := m.mutTarget

	var add, remove []string
	for tag := range m.tagPending {
		if !m.tagOriginal[tag] {
			add = append(add, tag)
		}
	}
	for tag := range m.tagOriginal {
		if !m.tagPending[tag] {
			remove = append(remove, tag)
		}
	}
	if len(add) == 0 && len(remove) == 0 {
		return m, nil
	}
	sort.Strings(add)
	sort.Strings(remove)

	etag, ok := m.beanETag(id)
	if !ok {
		m.err = "Item no longer exists — selection discarded"
		// bt-81f0: see box_confirm_delete.go's identical guard comment.
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
		return m, toastCmd
	}
	client := m.client
	return m, mutateCmd(func() error { return client.SetTags(id, add, remove, etag) })
}

// tagPickerBox renders the floating Tag-Picker overlay (bean bt-9ipw, D01):
// an ALWAYS-visible, always-focused search field (m.tagInput.View()) sits
// above the checkbox + usage-count row list (same modalPanel + [x]/[ ]
// convention as treeFilterBox, box_filter_facets.go) -- the two former
// separate overlay renderings (this function + the old tagInputBox) are now
// ONE. The row list is driven by tagInputFiltered (the live-filtered
// subset), NOT the unfiltered tagItems, so typing visibly narrows exactly
// what's on screen -- the PO-Review 2026-07-17 (US-07) complaint this bean
// reopened over ("kein visueller Hinweis WAS ich getippt habe"). The hint
// line is dynamic: the normal toggle/save/discard hint while there is at
// least one row to show (including the empty-query "every tag" state), or a
// create-specific hint once typed text has NO substring match at all
// (mirrors tagPickerEnter's own branch one layer up). Each row's marker
// column (T6, bean bt-pqq3, D10 Suggest-Mode, PF-12 design-spec.md §15) is
// ALWAYS reserved -- a defined tag renders tagManagementMarkerGlyph (reused
// verbatim from view_tag_management.go/T2, bean bt-r92i's own explicit "T6
// kann diesen Glyph ... wiederverwenden" hand-off -- PF-12-Konsistenz, one
// "defined" glyph across the whole app, instead of a second bespoke glyph),
// a free tag renders the SAME-WIDTH blank -- NEVER a conditional omission of
// the column itself, so neither group's checkbox/name column ever shifts
// relative to the other.
func (m model) tagPickerBox() string {
	var b strings.Builder

	hint := "type:filter  ↑/↓:nav  space:toggle  enter:save  esc:discard"
	if len(m.tagInputFiltered) == 0 && strings.TrimSpace(m.tagInput.Value()) != "" {
		hint = "no match — enter:create new tag  esc:discard"
	}
	b.WriteString(theme.Muted.Render(hint) + "\n")
	b.WriteString(m.tagInput.View() + "\n")
	if m.tagInputErr != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Red).Render(m.tagInputErr) + "\n")
	}
	b.WriteString("\n")

	for i, it := range m.tagInputFiltered {
		marker := " "
		if it.defined {
			marker = tagManagementMarkerStyle.Render(tagManagementMarkerGlyph)
		}
		box := theme.Dim.Render("[ ]")
		if m.tagPending[it.tag] {
			box = theme.Accent.Render("[x]")
		}
		cursor := "  "
		label := it.tag
		if i == m.tagInputSuggestCursor {
			cursor = theme.Accent.Render("▸ ")
			label = theme.Header.Render(label)
		}
		count := theme.Muted.Render(fmt.Sprintf(" (%d)", it.count))
		b.WriteString(cursor + marker + " " + box + " " + label + count + "\n")
	}
	if len(m.tagInputFiltered) == 0 {
		if len(m.tagItems) == 0 {
			b.WriteString(theme.Muted.Render("(no tags in repo)") + "\n")
		} else {
			b.WriteString(theme.Muted.Render("(no match)") + "\n")
		}
	}
	return modalPanel("Tags", b.String(), "", clampModalWidth(40, m.width), theme.Mauve)
}
