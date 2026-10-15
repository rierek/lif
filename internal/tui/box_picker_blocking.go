package tui

// box_picker_blocking.go — the Blocking-Picker (`B`, E3 Task 3, bean
// bt-p1uz): toggle-multi-select over EVERY bean except the focused one
// itself (design decision g/f: deliberately NO cycle exclusion, port-parity
// with beans-src blockingpicker.go, which has none either -- a blocking
// cycle is a logical PO mistake, not a render hazard, unlike the
// Parent-Picker's tree-flatten concern, box_picker_parent.go/data/
// hierarchy.go). Pending-Diff pattern ported verbatim from box_picker_tag.go
// (E3 Task 2): space toggles pending, enter diffs pending against original
// and fires ONE combined data.SetBlocking mutateCmd (mirrors SetTags' single-
// etag-no-cascade rationale), esc discards.

import (
	"sort"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// buildBlockingItems lists every bean except selfID, rendered via
// relationRow (view_detail_bean.go:66-68, same glyph order as every other
// bean row in the app). idx.ByID is a Go map, so a baseline ID sort runs
// first before the single-source data.SortBeans pass -- same two-pass
// determinism fix box_filter_facets.go's tagFilterOptions documents (ties on
// Status/Priority/Type/Title alike would otherwise be map-walk-order-
// dependent).
// slug (bean bt-pt1r) is the caller's m.beanIDPrefix(), stripped from the
// RENDERED prefix only -- pickerItem.id stays the full mutation target.
func buildBlockingItems(idx *data.Index, selfID, slug string) []pickerItem {
	if idx == nil {
		return nil
	}
	all := make([]*data.Bean, 0, len(idx.ByID))
	for _, cand := range idx.ByID {
		if cand.ID == selfID {
			continue
		}
		all = append(all, cand)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	data.SortBeans(all)

	items := make([]pickerItem, len(all))
	for i, cand := range all {
		items[i] = pickerItem{id: cand.ID, prefix: relationRowPrefix(cand, slug), title: cand.Title}
	}
	return items
}

// openBlockingPicker opens the Blocking-Picker on the focused bean (the
// focusedBean()!=nil guard lives in the caller, keyNodeAction).
// blockOriginal/blockPending are seeded as two INDEPENDENT maps (same
// wholesale-replace convention as openTagPicker) from the focused bean's
// CURRENT Blocking field. mutTarget captures the bean ID only, never the
// etag (design decision d).
// bean bt-a3a8: opening also installs a FRESH pickerFilter (newPickerFilter
// -- focused, empty query, every chip unset) and seeds blockFiltered from
// it, so the first frame shows the complete candidate list and the search
// field is typeable immediately, with no second gate. Returns
// textinput.Blink so the now-always-focused field's cursor actually blinks
// (openTagPicker's own convention) -- which is why this constructor grew a
// tea.Cmd return where it used to return a bare model.
func (m model) openBlockingPicker() (model, tea.Cmd) {
	b := m.focusedBean()
	if b == nil {
		return m, nil
	}
	m.mutTarget = b.ID
	m.blockItems = buildBlockingItems(m.idx, b.ID, m.beanIDPrefix())
	m.blockFilter = newPickerFilter()
	m.blockFiltered = filterPickerItems(m.idx, m.blockItems, m.blockFilter, false)

	orig := make(map[string]bool, len(b.Blocking))
	for _, id := range b.Blocking {
		orig[id] = true
	}
	pending := make(map[string]bool, len(orig))
	for id := range orig {
		pending[id] = true
	}
	m.blockOriginal = orig
	m.blockPending = pending

	m.menu = listState{}
	m.menu.setLen(len(m.blockFiltered))

	m.overlay = overlayBlockingPicker
	return m, textinput.Blink
}

// keyBlockingPicker drives the open Blocking-Picker. Since bean bt-a3a8 the
// overlay hosts an always-focused search field (box_picker_filter.go), so
// the key contract mirrors keyTagPicker's exactly: a SMALL reserved set is
// intercepted ahead of the input, EVERYTHING else belongs to it.
//
// Reserved: raw tea.KeyUp/tea.KeyDown for cursor movement -- deliberately
// NOT navKey, whose "i"/"k" letter aliases would become permanently
// untypeable here (keyTagPicker's own rationale, applied verbatim); esc
// (discard) and enter (apply the pending diff) via keybind so a rebind stays
// correct; and space via blockingPickerToggleHint.
//
// ERRATUM vs. this picker's pre-bt-a3a8 behavior: toggle was keys.Toggle,
// which binds BOTH " " and "x". Intercepting "x" ahead of a focused
// textinput makes it a never-typeable character -- the exact bug Review-R1
// B01 found in the Tag-Picker (bean bt-9ipw, keymap.go's TagToggle
// doc-stamp: "Filter menu / Blocking picker keep the full space/x Toggle"
// was written when this picker had NO search field). Now that it does, the
// same narrowing applies for the same reason: bean titles/IDs routinely
// contain "x", and space is safe to reserve because it can never be a
// meaningful leading search character. The filter menu, which still has no
// input field, keeps the full space/x Toggle unchanged.
//
// bean bt-z4w7 (B7): the narrowing above reached the HANDLER in bt-a3a8 but
// never the FOOTER, which kept advertising keys.Toggle's "space/x Toggle
// facet" for a combination that no longer existed. Both sides now match/
// render the one blockingPickerToggleHint (footer_context.go) -- borrowing
// keys.TagToggle worked key-wise but described the wrong domain ("Toggle
// tag" in a blocking picker) and left the same two-sources gap open.
//
// The cursor and the toggle both address m.blockFiltered, not m.blockItems
// -- "toggles what's ON SCREEN", so a row narrowed down by typing is the row
// that actually flips (TestBlockingPickerSelectionAfterFilterHitsRightBean).
func (m model) keyBlockingPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.blockFilter.input.Blur()
		return m, nil
	case keybind.Matches(msg, blockingPickerToggleHint):
		return m.toggleBlockPending(), nil
	case keybind.Matches(msg, keys.Enter):
		return m.applyBlockingPickerDiff()
	}

	f, cmd, changed := m.keyPickerFilter(m.blockFilter, msg)
	m.blockFilter = f
	if changed {
		m.blockFiltered = filterPickerItems(m.idx, m.blockItems, m.blockFilter, false)
		m.menu = listState{}
		m.menu.setLen(len(m.blockFiltered))
	}
	return m, cmd
}

// toggleBlockPending flips the cursored row's membership in m.blockPending.
// I01 (types.go doc-stamp): clones via cloneBoolMap before writing -- same
// convention as toggleTagPending. bt-a3a8: the cursor addresses
// m.blockFiltered (what is on screen), never the unfiltered m.blockItems.
func (m model) toggleBlockPending() model {
	if m.menu.cursor < 0 || m.menu.cursor >= len(m.blockFiltered) {
		return m
	}
	id := m.blockFiltered[m.menu.cursor].id
	out := cloneBoolMap(m.blockPending)
	if out[id] {
		delete(out, id)
	} else {
		out[id] = true
	}
	m.blockPending = out
	return m
}

// applyBlockingPickerDiff computes blockPending's diff against
// blockOriginal and fires it as ONE combined data.SetBlocking mutateCmd
// (mirrors applyTagPickerDiff's rationale verbatim -- N sequential
// AddBlocking/RemoveBlocking calls against ONE etag would be a conflict
// cascade: the first call rotates the etag on disk, every subsequent call
// then sees a stale etag and fails ErrConflict) against a FRESH etag
// (m.beanETag, design decision d). No pending changes at all -> no Cmd. add/
// remove are sorted before dispatch for a deterministic CLI-invocation
// order.
func (m model) applyBlockingPickerDiff() (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	id := m.mutTarget

	var add, remove []string
	for target := range m.blockPending {
		if !m.blockOriginal[target] {
			add = append(add, target)
		}
	}
	for target := range m.blockOriginal {
		if !m.blockPending[target] {
			remove = append(remove, target)
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
	return m, mutateCmd(func() error { return client.SetBlocking(id, add, remove, etag) })
}

// blockingDot is the ●/○ pending indicator (port beans-src
// blockingpicker.go's blockingIndicator, Red/Muted) -- rendered separately
// from the row's own relationRow theming, prefixed onto the row text before
// the shared D08 cursor treatment strips/re-tints the whole line.
func blockingDot(pending bool) string {
	if pending {
		return lipgloss.NewStyle().Foreground(theme.Red).Render("● ")
	}
	return theme.Muted.Render("○ ")
}

// blockingPickerBox renders the floating Blocking-Picker overlay -- ●/○
// indicator + relationRow-styled row, D08 cursor treatment (ansi.Strip +
// Accent-wrap, see parentPickerBox's doc comment for the full rationale)
// since each row already carries its own theme colors. windowAround
// (parentPickerRowBudget, box_picker_parent.go) keeps a repo with many beans
// from overflowing the modal.
//
// T5 (bean bt-4mo9, B06): the old single-line `blockingDot(...) + it.label`
// concatenation (label pre-rendered via relationRow(cand, "", relationRow
// NoWrap) at open-time, NEVER wrap-aware) is replaced by hangingIndentWrap
// against the REAL, live picker width (wideModalWidth(m.width)) -- the dot
// now lives INSIDE the composed `prefix` argument (alongside the cursor bar
// and the glyph+ID part), so hangingIndentWrap's own indent math accounts
// for its width too and continuation lines align correctly regardless of
// pending/cursor state. See parentPickerBox's doc comment for why only
// prefix (not title) carries the Accent cursor-recolor.
//
// bt-a3a8: the hint line + the search/chip chrome (pickerFilter.chrome) now
// sit above the rows, and the row window shrinks to pickerFilteredRowBudget
// to pay for them -- the bean's "Kandidatenliste entsprechend kürzen"
// requirement, so the overlay still fits an 80x24 terminal
// (TestPickerBoxesFitIn80Columns).
func (m model) blockingPickerBox() string {
	var b strings.Builder

	// bean bt-6nuz: the width is computed BEFORE the hint now, because the
	// hint wraps to it (pickerFilterHint) instead of running off the modal.
	w := wideModalWidth(m.width)
	contW := w - 2 // modalBox's Padding(0,1) overhead only -- see parentPickerBox's doc comment
	if contW < 8 {
		contW = 8
	}

	b.WriteString(pickerFilterHint(blockingPickerLocalBindings(), contW) + "\n")
	b.WriteString(m.blockFilter.chrome(contW))

	rows := make([]string, len(m.blockFiltered))
	for i, it := range m.blockFiltered {
		dot := blockingDot(m.blockPending[it.id])
		title := it.title
		var prefix string
		if i == m.menu.cursor {
			plain := ansi.Strip(dot + it.prefix)
			prefix = theme.Accent.Render("▌" + plain)
			title = theme.Accent.Render(title)
		} else {
			prefix = " " + dot + it.prefix
		}
		rows[i] = hangingIndentWrap(prefix, title, contW)
	}
	rows = pickerRowWindow(rows, m.menu.cursor, m.height)
	b.WriteString(strings.Join(rows, "\n"))
	if len(rows) > 0 {
		b.WriteString("\n")
	}
	if len(m.blockFiltered) == 0 {
		if len(m.blockItems) == 0 {
			b.WriteString(theme.Muted.Render("(no other items in repo)") + "\n")
		} else {
			b.WriteString(theme.Muted.Render("(no match)") + "\n")
		}
	}
	// bean bt-6nuz (#7): the heading reads "Relations", matching the
	// Detail-View's own RELATIONS panel -- the PO arrives here FROM that
	// panel, and two names for one thing read as two features. This is the
	// DISPLAY label only: blockItems/blockPending/blockFiltered,
	// data.SetBlocking and the bean field itself all keep "blocking", which
	// is the domain word the data model and the beans CLI actually use.
	return modalPanel("Relations", b.String(), "", w, theme.Mauve)
}
