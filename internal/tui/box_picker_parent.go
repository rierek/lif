package tui

// box_picker_parent.go — the Parent-Picker (`a`, E3 Task 3, bean bt-p1uz):
// single-select over data.EligibleParents(idx, b) (self + descendants +
// invalid types pre-filtered server-side rule mirror, design decision f)
// plus a "(No parent)" row pinned first (port beans-src parentpicker.go's
// clearParentItem). Enter applies IMMEDIATELY (SetParent/RemoveParent) and
// closes -- the SAME immediate-apply Enter semantics as the combined value
// menu (box_menu_value.go, design decision a3), NOT the Pending-Diff pattern
// the Tag-/Blocking-Picker use (box_picker_tag.go, box_picker_blocking.go):
// a bean has exactly ONE parent, there is nothing to diff.

import (
	"github.com/rierek/lif/pkg/item"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// pickerItem is one row shared by the Parent-Picker (this file) and the
// Blocking-Picker (box_picker_blocking.go). id "" is the Parent-Picker's own
// "(No parent)" clear row (RemoveParent) -- the Blocking-Picker never
// produces an empty id, every one of its rows is a real bean.
//
// T5 (bean bt-4mo9, B06): prefix/title replace the old single pre-rendered
// `label` field (relationRow(cand, "", relationRowNoWrap) -- a fully
// concatenated, NEVER-wrapped string baked at open-time). Splitting them
// lets blockingPickerBox/parentPickerBox compose their OWN leading element
// (the D08 cursor bar and, for Blocking, the pending dot) onto prefix and
// hand BOTH to hangingIndentWrap at RENDER time, against the LIVE picker
// width (wideModalWidth(m.width)) -- the actual B06 fix (PO: overlay IDs
// broke mid-word because the old single-line label was never wrap-aware).
type pickerItem struct {
	id string
	// prefix is the themed glyph+ID half of a relation row
	// (relationRowPrefix, view_detail_bean.go: status icon, type icon,
	// Key-styled ID, trailing space) -- "" for the Parent-Picker's own
	// synthetic "(No parent)" clear row, which carries no relation glyphs.
	prefix string
	// title is the wrappable text: a real bean's Title, or the clear row's
	// plain "(No parent)" string (themed at render time, mirroring the
	// pre-T5 contract).
	title string
}

// parentPickerRowBudget caps the Parent-Picker's/Blocking-Picker's row
// window (plan Step 3/5: "windowAround bei Überlänge"). Unlike the Tree/
// Ready panes, these floating overlays are not handed an explicit body
// height by their caller (modalPanel sizes to its content, not the other way
// around) -- there is no existing "clampModalHeight" helper to mirror
// (clampModalWidth, box_filter_facets.go, is width-only), so a fixed,
// generous cap keeps a repo with many eligible/blockable beans from growing
// the modal past the terminal instead of computing one from m.height.
const parentPickerRowBudget = 14

// buildParentItems assembles the Parent-Picker's row list: "(No parent)"
// pinned first (id "", port clearParentItem), then data.EligibleParents(idx,
// b) (self/descendants/invalid-types pre-filtered, sorted) rendered via the
// existing relationRow helper (view_detail_bean.go:66-68) -- same status-
// icon+type-icon+ID+title glyph order as every other bean row in the app.
// slug (bean bt-pt1r) is the caller's m.beanIDPrefix() -- the rendered
// prefix drops the current repo's own ID prefix exactly like the left pane's
// rows do. pickerItem.id deliberately keeps the FULL id: it is the mutation
// target handed to SetParent, not display text.
func buildParentItems(idx *data.Index, b *item.Item, slug string) []pickerItem {
	items := []pickerItem{{id: "", title: "(No parent)"}}
	for _, cand := range data.EligibleParents(idx, b) {
		items = append(items, pickerItem{id: cand.ID, prefix: relationRowPrefix(cand, slug), title: cand.Title})
	}
	return items
}

// openParentPicker opens the Parent-Picker on the focused bean (the
// focusedBean()!=nil guard lives in the caller, keyNodeAction). Cursor
// starts on the bean's CURRENT parent row, falling back to index 0 (the
// "(No parent)" row) for an unparented bean, or one whose parent fell out
// of eligibility since (port beans-src parentpicker.go's selectedIndex seed,
// newParentPickerModel:175-182). mutTarget captures the bean ID only, never
// the etag (design decision d) -- a watch-reload between open and the
// eventual enter is honored automatically via beanETag.
//
// bean bt-a3a8: opening also installs a FRESH pickerFilter and seeds
// parentFiltered from it (the full list, since the query starts empty), so
// the current-parent cursor seed below still runs against everything that
// is actually on screen. Returns textinput.Blink for the now-always-focused
// search field (openTagPicker's own convention) -- hence the new tea.Cmd
// return where this used to hand back a bare model.
func (m model) openParentPicker() (model, tea.Cmd) {
	b := m.focusedBean()
	if b == nil {
		return m, nil
	}
	m.mutTarget = b.ID
	m.parentItems = buildParentItems(m.idx, b, m.beanIDPrefix())
	m.parentFilter = newPickerFilter()
	m.parentFiltered = filterPickerItems(m.idx, m.parentItems, m.parentFilter, true)

	m.menu = listState{}
	m.menu.setLen(len(m.parentFiltered))
	for i, it := range m.parentFiltered {
		if it.id == b.Parent {
			m.menu.cursor = i
			break
		}
	}

	m.overlay = overlayParentPicker
	return m, textinput.Blink
}

// parentPickerCursorItem resolves the cursored row of the FILTERED list --
// the single place the Parent-Picker turns a cursor into a selection, shared
// by applyParentPickerSelection and its tests (bean bt-a3a8: before
// filtering existed, the cursor indexed parentItems directly at every call
// site, which would now silently select the wrong bean).
func (m model) parentPickerCursorItem() (pickerItem, bool) {
	if m.menu.cursor < 0 || m.menu.cursor >= len(m.parentFiltered) {
		return pickerItem{}, false
	}
	return m.parentFiltered[m.menu.cursor], true
}

// keyParentPicker drives the open Parent-Picker. Since bean bt-a3a8 the
// overlay hosts an always-focused search field (box_picker_filter.go), so
// only a small reserved set is intercepted ahead of it and everything else
// belongs to the input -- see keyBlockingPicker's doc comment for the full
// rationale, which applies here verbatim. Two differences from that picker,
// both structural: there is no space-toggle (single-select, so space is just
// another typeable character in a title search), and enter applies the
// cursored row IMMEDIATELY (SetParent/RemoveParent) rather than diffing.
//
// Raw tea.KeyUp/tea.KeyDown deliberately, NOT navKey: "i"/"k" must stay
// literal typeable characters inside the search field.
func (m model) keyParentPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		m.menu.move(-1)
		return m, nil
	case tea.KeyDown:
		m.menu.move(1)
		return m, nil
	}
	switch {
	case keybind.Matches(msg, keys.Back):
		m.overlay = overlayNone
		m.parentFilter.input.Blur()
		return m, nil
	case keybind.Matches(msg, keys.Enter):
		return m.applyParentPickerSelection()
	}

	f, cmd, changed := m.keyPickerFilter(m.parentFilter, msg)
	m.parentFilter = f
	if changed {
		m.parentFiltered = filterPickerItems(m.idx, m.parentItems, m.parentFilter, true)
		m.menu = listState{}
		m.menu.setLen(len(m.parentFiltered))
	}
	return m, cmd
}

// applyParentPickerSelection dispatches SetParent (a real row) or
// RemoveParent (the "(No parent)" row, id "") against a FRESH etag read
// from the live index (m.beanETag, update.go, design decision d -- never a
// captured copy). Always fires, regardless of whether the cursored row
// already matches the bean's current parent -- mirrors
// applyValueMenuSelection's own unconditional-dispatch Enter semantics
// (design decision a3), not a diff. ok==false means mutTarget vanished from
// the index between open and this enter -- the overlay closes with a
// status-line note instead of firing a doomed mutation (same vanished-target
// guard as applyValueMenuSelection/applyTagPickerDiff).
func (m model) applyParentPickerSelection() (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	it, ok0 := m.parentPickerCursorItem()
	if !ok0 {
		return m, nil
	}
	id := m.mutTarget
	etag, ok := m.beanETag(id)
	if !ok {
		m.err = "Item no longer exists — selection discarded"
		// bt-81f0: see box_confirm_delete.go's identical guard comment.
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
		return m, toastCmd
	}
	store := m.store
	if it.id == "" {
		return m, mutateCmd(store, func() error { return store.RemoveParent(id, etag) })
	}
	parentID := it.id
	return m, mutateCmd(store, func() error { return store.SetParent(id, parentID, etag) })
}

// parentPickerBox renders the floating Parent-Picker overlay -- D08 cursor
// treatment (ansi.Strip + Accent-wrap) for the cursored row, same convention
// as treeRows/readyRows (view_browse_repo.go/view_browse_ready.go)
// rather than box_menu_value.go's/box_picker_tag.go's theme.Header.Render:
// unlike those two overlays' PLAIN value/tag strings, a real row's prefix
// here is ALREADY themed (relationRowPrefix -- status-icon/type-icon
// colors, Key-styled ID), so re-wrapping it in a second style would clash;
// stripping to plain text and re-tinting Accent is the same technique the
// Tree/Ready panes already use for exactly this reason -- T5 (bean
// bt-4mo9, B06) narrows that treatment to the PREFIX only (cursor bar +
// glyph+ID), matching hangingIndentWrap's own contract (prefix carries
// styling, text/title stays plain unless separately styled): the pending
// dot's SHAPE (blockingDot, box_picker_blocking.go) still reads under an
// Accent override, exactly mirroring the ▷/▶ marker-only convention T4
// introduced for the RELATIONS section (view_detail_bean.go). windowAround
// (parentPickerRowBudget) keeps a long eligible-parent list from overflowing
// the modal (plan Step 3) -- ACHTUNG (B06 Architektur-Vorgabe, documented
// judgment call): rows can now be MULTI-LINE (a wrapped long title), so
// "parentPickerRowBudget=14" caps 14 windowAround SLICE ELEMENTS (i.e. 14
// eligible-parent rows), not 14 visible terminal lines -- an unusually long
// run of unusually long titles in the visible window can still grow the
// modal past 14 screen lines. Accepted per the bean's own note: most bean
// titles stay single-line even at 85% width, height (parentPickerRowBudget)
// deliberately stays untouched (PO: "Höhe passt").
//
// bt-a3a8: the search/chip chrome now sits above the rows and the window
// shrinks to pickerFilteredRowBudget to pay for it (see blockingPickerBox's
// twin note). The "(No parent)" clear row stays pinned at index 0 no matter
// what is typed (filterPickerItems' keepFirstSynthetic) -- it is an ACTION,
// not a candidate.
func (m model) parentPickerBox() string {
	var b strings.Builder

	// bean bt-6nuz: width first, the hint wraps to it (see blockingPickerBox).
	w := wideModalWidth(m.width)
	contW := w - 2 // modalBox's own Padding(0,1) overhead -- border adds no further inner-width cost (empirically verified: Width() already absorbs padding, Border() only adds outside it)
	if contW < 8 {
		contW = 8
	}

	b.WriteString(pickerFilterHint(parentPickerLocalBindings(), contW) + "\n")
	b.WriteString(m.parentFilter.chrome(contW))

	rows := make([]string, len(m.parentFiltered))
	for i, it := range m.parentFiltered {
		title := it.title
		if it.id == "" {
			title = theme.Dim.Render(title)
		}
		var prefix string
		if i == m.menu.cursor {
			plain := ansi.Strip(it.prefix)
			prefix = theme.Accent.Render("▌" + plain)
			title = theme.Accent.Render(ansi.Strip(title))
		} else {
			prefix = " " + it.prefix
		}
		rows[i] = hangingIndentWrap(prefix, title, contW)
	}
	rows = pickerRowWindow(rows, m.menu.cursor, m.height)
	b.WriteString(strings.Join(rows, "\n"))
	if len(rows) > 0 {
		b.WriteString("\n")
	}
	if len(m.parentFiltered) <= 1 {
		if len(m.parentItems) == 1 {
			b.WriteString(theme.Muted.Render("(no eligible parent types)") + "\n")
		} else {
			b.WriteString(theme.Muted.Render("(no match)") + "\n")
		}
	}
	return modalPanel("Assign parent", b.String(), "", w, theme.Mauve)
}
