package tui

// box_menu_value.go — the combined Status/Type/Priority value menu (`s`, E3
// Task 1, bean bt-dlgk, design decision a3): design-spec.md §7 reserves
// exactly ONE key for this cluster (no separate Type/Priority key), so all
// three beans enums render as ONE modalPanel with three group headers (the
// SAME facetHead render pattern box_filter_facets.go's treeFilterBox already
// established) instead of three separate picker overlays -- beans-src's
// statuspicker.go/typepicker.go/prioritypicker.go are structurally
// identical, consolidated here per design decision a3. Enter applies the
// cursored value IMMEDIATELY and closes (port beans-src statuspicker.go
// Enter-semantics, statuspicker.go:158-172) -- unlike the filter menu, whose
// enter only closes without mutating anything (filter facets act live).

import (
	"github.com/rierek/lif/pkg/item"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// valueMenuItem is one row of the combined s-menu. group is "status",
// "type", or "priority" -- value is one of that group's canonical enum
// members (data.StatusValues/TypeValues/PriorityValues, the E3 enum
// single-source, design decision b).
type valueMenuItem struct{ group, value string }

// valueMenuGroupHead labels each group's sub-header -- same facetHead-style
// render pattern as box_filter_facets.go's treeFilterBox.
var valueMenuGroupHead = map[string]string{
	"status":   "Status",
	"type":     "Type",
	"priority": "Priority",
}

// buildValueMenuItems assembles ONE group's 5 rows (B11/B12, design-spec.md
// §15 PF-16, bean bt-ntoz): the menu USED to always return all 15 rows (5
// status + 5 type + 5 priority) regardless of which group the caller asked
// for -- confusing both for the `s` key (Status-only by design decision a3)
// and for the PF-5 Meta field-level enter cascade (which reaches this menu
// already correctly group-scoped via f.kind, but the combined render then
// showed the other two groups anyway). group selects exactly one of
// data.StatusValues/TypeValues/PriorityValues (design decision b -- single
// source; box_filter_facets.go's buildFilterItems consumes the exact same
// accessors); an unrecognized group returns nil (openValueMenu's caller
// contract only ever passes "status"/"type"/"priority", mirrors
// currentValueForGroup's own unrecognized-group fallback).
func buildValueMenuItems(group string) []valueMenuItem {
	var items []valueMenuItem
	switch group {
	case "status":
		for _, v := range data.StatusValues() {
			items = append(items, valueMenuItem{group: "status", value: v})
		}
	case "type":
		for _, v := range data.TypeValues() {
			items = append(items, valueMenuItem{group: "type", value: v})
		}
	case "priority":
		for _, v := range data.PriorityValues() {
			items = append(items, valueMenuItem{group: "priority", value: v})
		}
	}
	return items
}

// valueMenuCursorFor finds group/value's index in items -- openValueMenu
// uses this to seed the menu's initial cursor on the focused bean's CURRENT
// status (port beans-src statuspicker.go newStatusPickerModel: selectedIndex
// seeded from the current value, statuspicker.go:85-139). Falls back to 0
// for an unrecognized value (never panics on stale/foreign data).
func valueMenuCursorFor(items []valueMenuItem, group, value string) int {
	for i, it := range items {
		if it.group == group && it.value == value {
			return i
		}
	}
	return 0
}

// valueMenuIsCurrent reports whether it is b's current value in its own
// group -- drives the "(current)" marker (valueMenuBox). Priority mirrors
// data.PriorityRank's own empty-defaults-to-normal convention.
func valueMenuIsCurrent(b *item.Item, it valueMenuItem) bool {
	switch it.group {
	case "status":
		return b.Status == it.value
	case "type":
		return b.Type == it.value
	case "priority":
		p := b.Priority
		if p == "" {
			p = "normal"
		}
		return p == it.value
	}
	return false
}

// currentValueForGroup returns b's current value in the given group
// (status/type/priority) -- the same empty-defaults-to-normal convention as
// valueMenuIsCurrent. Used by openValueMenu(group) to seed the cursor on
// whichever group the caller opens (T6, design-spec.md §15 PF-5: the Meta
// field-level enter cascade seeds "type"/"priority" too, not only the
// `s`-key's hardcoded "status"). Returns "" for an unrecognized group (falls
// back to valueMenuCursorFor's own index-0 default, never panics).
func currentValueForGroup(b *item.Item, group string) string {
	switch group {
	case "status":
		return b.Status
	case "type":
		return b.Type
	case "priority":
		p := b.Priority
		if p == "" {
			p = "normal"
		}
		return p
	}
	return ""
}

// openValueMenu opens the value menu on the focused bean, filtered to and
// seeded on group's current value (the focusedBean()!=nil guard lives in the
// caller, keyNodeAction/keyDetailFocus/dispatchPalette). group is "status"/
// "type"/"priority" (T6, design-spec.md §15 PF-5 -- was hardcoded to "status"
// before the Meta field-level enter cascade needed to seed the other two
// groups as well). B11/B12 (E8 Task 6, bean bt-ntoz): group now ALSO filters
// buildValueMenuItems, not just the cursor seed -- the menu shows ONLY the
// requested group's 5 rows. mutTarget captures WHICH bean the open overlay
// acts on; the ETag is deliberately NOT captured here (design decision d, see
// beanETag in update.go) -- only the ID, so a watch-reload between open and
// submit is automatically honored.
func (m model) openValueMenu(group string) model {
	b := m.focusedBean()
	if b == nil {
		return m
	}
	m.mutTarget = b.ID
	m.menuItems = buildValueMenuItems(group)
	m.menu = listState{}
	m.menu.setLen(len(m.menuItems))
	m.menu.cursor = valueMenuCursorFor(m.menuItems, group, currentValueForGroup(b, group))
	// Slice C (bt-f0y9, D09 revidiert): seed the anchor field HERE, the ONE
	// place all four trigger paths (keyboard s/o/u, field-cursor Enter, mouse
	// click, Palette) funnel through (S6 grounding §4) -- so this is the only
	// call site that needs touching, not four. Read by composeOverlays'
	// placeValueMenuOverlay (view_browse_repo.go) ONLY.
	m.valueMenuAnchorField = boxFormFieldIndexForGroup(group)
	m.overlay = overlayValueMenu
	return m
}

// boxFormFieldIndexForGroup maps a value-menu group ("status"/"type"/
// "priority") to its boxFormFieldOrder (box_nav_field.go) index -- a STATIC
// lookup (the group<->field position never varies per-bean or per-render,
// unlike boxFormEffectiveCursor's per-session state), used by openValueMenu
// (above) to seed m.valueMenuAnchorField regardless of which of the four
// trigger paths opened the menu. -1 for an unrecognized group (defensive
// only -- openValueMenu's own callers never pass one); placeValueMenuOverlay
// falls back to the pre-existing centered placeOverlay whenever
// boxFormFieldRect can't resolve the resulting index (same as a nil focused
// bean).
func boxFormFieldIndexForGroup(group string) int {
	var target string
	switch group {
	case "status":
		target = boxFormTargetStatus
	case "type":
		target = boxFormTargetType
	case "priority":
		target = boxFormTargetPriority
	default:
		return -1
	}
	for i, f := range boxFormFieldOrder {
		if f.target == target {
			return i
		}
	}
	return -1
}

// keyValueMenu drives the open value menu: up/down move the cursor
// (navKey), enter applies the cursored value and closes (immediate-apply
// Enter semantics, design decision a3), esc and the group's OWN key close
// WITHOUT mutating anything (mirrors keyFilterMenu's esc/enter/f
// close-without-clearing precedent, box_filter_facets.go).
//
// a3-NACHTRAG (bean bt-z4w7, B7): decision a3 was written as "esc/s
// schliesst" back when `s` was the only key that could open this menu. S4
// added o=Type and u=Priority, and the literal `s` became wrong twice over
// -- an o-opened menu answered to a key its own footer had no business
// showing. The close alias is now the key that OPENED the menu, resolved
// through valueMenuGroupKey (footer_context.go) -- the SAME accessor the
// footer and the inline hint read, so the three can no longer disagree.
// This deliberately revises a3 rather than quietly patching a label.
func (m model) keyValueMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch navKey(msg.String()) {
	case "up":
		m.menu.move(-1)
		return m, nil
	case "down":
		m.menu.move(1)
		return m, nil
	}
	switch {
	case keybind.Matches(msg, keys.Back), keybind.Matches(msg, valueMenuGroupKey(m.valueMenuGroup())):
		m.overlay = overlayNone
		return m, nil
	case keybind.Matches(msg, keys.Enter):
		return m.applyValueMenuSelection()
	}
	return m, nil
}

// applyValueMenuSelection dispatches the cursored row's Set* mutation
// (SetStatus/SetType/SetPriority, keyed by group) against a FRESH etag read
// from the live index (m.beanETag, update.go, design decision d -- never a
// captured copy). ok==false means mutTarget vanished from the index between
// open and this enter (e.g. a watch-reload after an external delete) -- the
// overlay closes with a status-line note instead of firing a doomed
// mutation; no reload Cmd is fired either, since the vanished-bean fact was
// itself already learned from the CURRENT (already reloaded) index.
func (m model) applyValueMenuSelection() (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	if m.menu.cursor < 0 || m.menu.cursor >= len(m.menuItems) {
		return m, nil
	}
	it := m.menuItems[m.menu.cursor]
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
	switch it.group {
	case "status":
		return m, mutateCmd(store, func() error { return store.SetStatus(id, it.value, etag) })
	case "type":
		return m, mutateCmd(store, func() error { return store.SetType(id, it.value, etag) })
	case "priority":
		return m, mutateCmd(store, func() error { return store.SetPriority(id, it.value, etag) })
	}
	return m, nil
}

// valueMenuTitle picks modalPanel's title from the open menu's (now single,
// B11/B12) group -- "Set status"/"Set type"/"Set priority" instead of the old
// hard-coded "Set value", so it's unambiguous WHICH group is currently open
// (Planner addition beyond the bare B12 wording, bean bt-y2iw: "welches Menü
// ist gerade offen" must not become the new source of confusion once the
// combined 15-row menu splits into three single-group ones). Falls back to
// "Set value" for an empty menuItems slice (defensive -- openValueMenu never
// produces one for a recognized group, but valueMenuBox must not panic on a
// zero-value model either, e.g. in a render-only test).
func valueMenuTitle(items []valueMenuItem) string {
	if len(items) == 0 {
		return "Set value"
	}
	switch items[0].group {
	case "status":
		return "Set status"
	case "type":
		return "Set type"
	case "priority":
		return "Set priority"
	}
	return "Set value"
}

// valueMenuBodyAndItemRows builds the value-menu's body content (everything
// modalPanel wraps below its own title line) IDENTICALLY to before this
// slice, while ALSO recording the ABSOLUTE fg-row (0 = the popup's own top
// border, matching m.valueMenuBox()'s own rendered lines) each item lands
// at. Factored out so valueMenuItemRow (below, Slice D's click hit-test,
// bt-f0y9 "feld-verankertes Inline-Dropdown") and valueMenuBox (render) share
// the ONE builder -- the two can never independently drift on where an item
// actually renders (Golden-Rule-Drift-Schutz). itemRows[i] = 2 (border-top +
// title, both fixed 1-line) + the hint line (1) + however many lines this
// loop has ALREADY written by the time item i's own turn comes up
// (strings.Count of "\n" in the builder so far) -- exactly one blank+
// group-header block ever fires (B11/B12: every open menu is single-group),
// so this naturally matches valueMenuBox's OWN loop without hardcoding a
// row-count constant that could silently desync from a future change here.
func (m model) valueMenuBodyAndItemRows() (body string, itemRows []int) {
	var b strings.Builder
	// bean bt-z4w7: the cancel key is READ OFF the binding keyValueMenu
	// actually matches (valueMenuGroupKey, footer_context.go) instead of the
	// hardcoded "s" this line used to carry -- a Priority menu opened with
	// `u` said "esc/s:cancel" while the outer Footer Zone 3 said the same
	// wrong thing, both describing a group they were not showing.
	b.WriteString(theme.Muted.Render("enter:apply  esc/"+valueMenuGroupKey(m.valueMenuGroup()).Help().Key+":cancel") + "\n")

	var cur *item.Item
	if m.idx != nil {
		cur = m.idx.ByID[m.mutTarget]
	}

	itemRows = make([]int, len(m.menuItems))
	lastGroup := ""
	for i, it := range m.menuItems {
		if it.group != lastGroup {
			b.WriteString("\n" + theme.Dim.Render(valueMenuGroupHead[it.group]) + "\n")
			lastGroup = it.group
		}
		itemRows[i] = 2 + strings.Count(b.String(), "\n") // border-top(1) + title(1) + lines written so far
		cursor := "  "
		label := it.value
		if i == m.menu.cursor {
			cursor = theme.Accent.Render("▸ ")
			label = theme.Header.Render(label)
		}
		if cur != nil && valueMenuIsCurrent(cur, it) {
			label += " " + theme.Muted.Render("(current)")
		}
		b.WriteString(cursor + label + "\n")
	}
	return b.String(), itemRows
}

// valueMenuItemRow returns the fg-row (0 = the popup's own top border) item
// index i renders at (Slice D click hit-test), or -1 for an out-of-range
// index. Read box_menu_value.go's valueMenuBodyAndItemRows doc comment for
// why this can never drift from the actual render.
func valueMenuItemRow(m model, i int) int {
	if i < 0 || i >= len(m.menuItems) {
		return -1
	}
	_, itemRows := m.valueMenuBodyAndItemRows()
	return itemRows[i]
}

// valueMenuBox renders the floating (now single-group, B11/B12) value menu --
// same modalPanel + facetHead-group-header render pattern as
// box_filter_facets.go's treeFilterBox, "(current)" suffix (theme.Muted) on
// the group's current value (port beans-src statuspicker.go isCurrent). The
// per-row group-header line still renders (lastGroup bookkeeping) even though
// exactly one group is present now -- harmless, keeps the render loop
// unchanged/simple, and the header text still adds value confirming which
// group is open.
func (m model) valueMenuBox() string {
	body, _ := m.valueMenuBodyAndItemRows()
	title := valueMenuTitle(m.menuItems)
	// Slice C B02 fix (bt-f0y9, PO-Entscheidung 2026-07-20): width now sizes
	// to CONTENT (valueMenuContentWidth), not a fixed 40 -- a fixed-width
	// popup is unrelated to the ~15-cell field it anchors to and, being far
	// wider than any of its own lines actually need, forced
	// placeValueMenuOverlay's old right-edge clamp to slam DIFFERENT fields
	// (Type/Priority) onto the SAME clamped column (B02). clampModalWidth
	// still applies on top -- an ultra-narrow terminal still shrinks/floors
	// at 24, same safety net every other modal already has.
	width := clampModalWidth(valueMenuContentWidth(title, body), m.width)
	return modalPanel(title, body, "", width, theme.Mauve)
}

// valueMenuContentWidth sizes the value-menu popup to its own longest line
// (Slice C B02 fix) instead of a fixed preference -- title, the "enter:
// apply .../esc:..." hint, the group header, and every item row (the
// cursored one is often the longest thanks to its own " (current)" suffix)
// are all real candidates; +2 accounts for modalBox's own Padding(0,1) (1
// cell reserved on each side within the width budget passed to lipgloss's
// Width(), Border() adds ON TOP of that -- see overlay.go's own placeOverlay
// doc-stamp for the general "Width() is content+padding, border is extra"
// contract this reuses verbatim).
func valueMenuContentWidth(title, body string) int {
	maxW := ansi.StringWidth(title)
	for _, l := range strings.Split(body, "\n") {
		if w := ansi.StringWidth(l); w > maxW {
			maxW = w
		}
	}
	return maxW + 2
}

// valueMenuPopupY picks the value-menu popup's Y position (Slice C, bt-f0y9
// "feld-verankertes Inline-Dropdown", D09 revidiert) given its triggering
// field's own rect (anchorY, anchorH -- boxFormFieldRect, box_nav_field.go),
// the popup's own height (fgH), the canvas height (canvasH), and minY -- the
// LOWEST row the popup may ever occupy (boxFormPopupChromeFloor,
// box_nav_field.go -- one row past the app's own outer border/breadcrumb/
// divider). Conventional dropdown behavior: BELOW the field by default
// (PO-Vorgabe).
//
// B01 FIX (Reviewer, 2026-07-20): the popup flips ABOVE the field ONLY when
// it fits there WITHOUT crossing minY -- the previous version flipped
// whenever there was "any room at all" above the field and let the result go
// negative, which spliced the popup's own MIDDLE rows straight over the
// app's title/border at 80x24 (Status sits at y=10, a 12-line popup flipped
// to y=-2). Since Status/Type/Priority sit only ~10 rows down in every real
// render (box_nav_field.go's own ERRATUM) and minY is typically 3-7, a popup
// taller than roughly (anchorY-minY) can NEVER fully fit above them -- this
// is now working as intended: it stays BELOW instead, clipped at the BOTTOM
// (placeOverlayAt's pre-existing silent-overflow-drop, Slice B) rather than
// corrupting chrome above. The popup's own TOP EDGE therefore always stays
// visible (Reviewer's own wording) -- a bottom-clip is cosmetically fine, an
// app-title-clobber is not.
func valueMenuPopupY(anchorY, anchorH, fgH, canvasH, minY int) int {
	below := anchorY + anchorH
	if below+fgH <= canvasH {
		return below
	}
	if above := anchorY - fgH; above >= minY {
		return above
	}
	return below
}

// placeValueMenuOverlay composes the value-menu overlay onto out (Slice C):
// while the anchor field's rect resolves
// (boxFormFieldRect, box_nav_field.go, seeded via m.mutTarget/
// m.valueMenuAnchorField at open time, openValueMenu above), the menu is
// placed LEFT-ALIGNED directly BELOW its triggering boxed field (or flipped
// above it, valueMenuPopupY) via placeOverlayAt (Slice B) instead of the
// pre-existing CENTERED placeOverlay. Falls back to placeOverlay in every
// other case: the Detail pane is hidden (no boxed fields on screen to
// anchor to, except in the Vollbild-Detail), or a rect that fails to
// resolve (nil mutTarget bean, out-of-range
// anchor field -- defensive; should not happen in practice since
// openValueMenu always seeds m.valueMenuAnchorField from the static
// boxFormFieldIndexForGroup lookup in the SAME call that sets m.overlay).
//
// x (Slice C B02 fix, PO-Entscheidung 2026-07-20): left-aligned to rx
// (the field's own x) by default -- ONLY shifted LEFT when the (now
// content-width-sized, valueMenuContentWidth) popup would run past the
// canvas' own RIGHT edge, and NEVER shifted past boxFormPaneContentLeft(m)
// (the pane's own left content edge, box_nav_field.go) -- so Type and
// Priority, anchored at visibly different field columns, now render at
// visibly different popup columns too (the B02 bug: a fixed-width-40 popup
// forced BOTH onto the SAME clamped column on an 80-column canvas).
func (m model) placeValueMenuOverlay(out string, w, h int) string {
	fg := m.valueMenuBox()
	x, y, _, _, ok := m.valueMenuPopupRect()
	if !ok {
		return placeOverlay(out, fg, w, h)
	}
	return placeOverlayAt(out, fg, w, h, x, y)
}

// valueMenuPopupRect returns the value-menu popup's absolute screen rect
// (x, y, w, h) EXACTLY as placeValueMenuOverlay (above) draws it -- Slice D's
// click hit-test (mouseValueMenuClick, mouse.go, bt-f0y9 "feld-verankertes
// Inline-Dropdown") needs the SAME geometry the render uses, so both now
// call this ONE function rather than maintaining the x/y math twice
// (Golden-Rule-Drift-Schutz). ok=false whenever the Detail pane is hidden
// (the menu is then rendered CENTERED via the pre-existing placeOverlay) or
// the anchor field's rect fails to resolve (same defensive fallback
// placeValueMenuOverlay itself has, e.g. a vanished mutTarget bean).
func (m model) valueMenuPopupRect() (x, y, w, h int, ok bool) {
	if m.detailHidden && m.fullscreen != fullscreenDetail {
		return 0, 0, 0, 0, false
	}
	b := m.idx.ByID[m.mutTarget] // nil-safe: a nil map / missing key both yield nil
	rx, ry, _, rh, rectOK := boxFormFieldRect(m, b, m.valueMenuAnchorField)
	if !rectOK {
		return 0, 0, 0, 0, false
	}

	// Same <=0 fallback every geometry helper in this file/box_nav_field.go
	// applies (clickPaneGeometry's own defaults) -- m.width/m.height are
	// otherwise 0 in a zero-value model (render-only tests, init), which
	// would make the clamps below meaningless.
	ww, hh := m.width, m.height
	if ww <= 0 {
		ww = 80
	}
	if hh <= 0 {
		hh = 24
	}

	fg := m.valueMenuBox()
	fgLines := strings.Split(fg, "\n")
	fgW, fgH := 0, len(fgLines)
	for _, l := range fgLines {
		if lw := ansi.StringWidth(l); lw > fgW {
			fgW = lw
		}
	}

	x = rx
	if fgW < ww && x+fgW > ww {
		x = ww - fgW // right-edge overflow: shift left, never past the field's own line
	}
	if paneLeft := boxFormPaneContentLeft(m); x < paneLeft {
		x = paneLeft // never cross into the Tree/Ready/Flat pane on the left
	}
	y = valueMenuPopupY(ry, rh, fgH, hh, boxFormPopupChromeFloor(m))

	return x, y, fgW, fgH, true
}

// mouseValueMenuClick dispatches a left-click while the value menu is open
// (Slice D, bt-f0y9 "feld-verankertes Inline-Dropdown", D09 revidiert -- the
// actual mouse-native payoff D09 was written for): a click on one of the
// popup's own item rows applies that value IMMEDIATELY via
// applyValueMenuSelection (box_menu_value.go -- the SAME enter-applies-and-
// closes semantics keyValueMenu already has, UNTOUCHED by this slice), a
// click anywhere else CLOSES the menu WITHOUT mutating (mirrors esc's own
// keyValueMenu branch). Geometry comes exclusively from valueMenuPopupRect/
// valueMenuItemRow (above) -- the SAME functions placeValueMenuOverlay
// renders with -- NEVER detailBoxFormClickRow's own X (Slice A/B's ERRATUM:
// that boundary is ~3 columns short of where content actually draws,
// harmless for its own coarse pane gate, but would misplace this hit-test).
//
// The centered mode (Detail pane hidden) has no anchored popup:
// valueMenuPopupRect's own ok=false there routes every click straight to
// "close without mutation".
//
// A click INSIDE the popup's own rect but not on any item row (the title/
// hint/group-header/blank/border rows) is a deliberate no-op -- neither
// selects nor closes, mirroring "clicking a modal's own chrome does
// nothing" precedent elsewhere in this codebase; only OUTSIDE the rect
// closes.
func (m model) mouseValueMenuClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	x, y, w, h, ok := m.valueMenuPopupRect()
	if !ok || msg.X < x || msg.X >= x+w || msg.Y < y || msg.Y >= y+h {
		m.overlay = overlayNone // outside the popup (or unresolved geometry): close, no mutation
		return m, nil
	}
	row := msg.Y - y
	for i := range m.menuItems {
		if valueMenuItemRow(m, i) == row {
			m.menu.cursor = i
			return m.applyValueMenuSelection()
		}
	}
	return m, nil // inside the popup, but not on an item row -- no-op
}
