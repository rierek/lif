package tui

// footer_context.go — the context-sensitive Footer Zone 3 (PF-11
// Q04-Antwort, design-spec.md §15, epic-E7-plan.md Task 7, bean bt-m6at):
// while a Filter-Menu/node-action-Overlay/Search/Command-Center/Help fully
// captures input (handleKey's own capture-order precedent, update.go), the
// view-local bindings underneath (browseRepoLocalBindings/
// readyLocalBindings) are non-functional -- showing them in the footer
// would actively mislead. contextualLocalHint swaps the footer to the
// ACTIVE capture state's own bindings instead.
//
// Two full-capture states are deliberately NOT cases below: an open huh Form
// (m.form != nil, forms_shared.go's formChrome) and m.confirmQuit (quitBox,
// box_confirm_quit.go) each already bake their own COMPLETE hint straight
// into their own modalPanel footer argument ("enter next/save · esc cancel"
// / "enter: quit   esc: cancel") -- there is no base-view fallback to build
// for either. The overlays covered below (Filter-Menu/Value-Menu/Tag-/
// Parent-/Blocking-Picker) also already render their OWN inline hint line
// at the top of their body (e.g. treeFilterBox's "space/x:toggle X:clear
// enter/esc/f:done") -- that is a separate, pre-existing surface; this
// file's job is strictly the OUTER footer (Zone 3, visible around/below the
// centered modal), which epic-E7-plan.md Task 7 Step 6 explicitly wants
// synced to the same active context too (Q04's underlying complaint: the
// outer footer kept showing STALE Tree/Ready hints while a totally
// different overlay had full input capture).
import keybind "github.com/charmbracelet/bubbles/key"

// filterMenuCategoryHint is the Filter-Menu's OWN local tab/shift+tab
// binding (bt-nxuk, Reviewer-Finding B04 aus bt-2p9m-Review 2026-07-17):
// inside the open filter menu, tab/shift+tab switch the active facet
// category (keyFilterMenu, box_filter_facets.go) -- a DIFFERENT,
// filter-menu-local meaning from keys.FocusIn/keys.FocusOut's global
// Tree<->Detail focus-swap (keymap.go). filterMenuLocalBindings USED TO
// reuse keys.FocusIn/keys.FocusOut directly here, which meant Footer Zone 3
// rendered the stale global label ("tab focus in · shift+tab focus out")
// even though the Filter-Menu's own inline hint (treeFilterBox,
// box_filter_facets.go) already said "tab/shift+tab:category". This is a
// deliberately standalone keybind.Binding (NOT a keyMap struct field --
// see keymap_test.go's TestHelpGroupsCoverEveryBindingExactlyOnce, which
// reflects ONLY over keyMap fields and would flag an unreferenced field;
// staying local here keeps that guard, and
// TestNoDuplicateBindingBetweenGlobalAndAnyLocalHintList (scoped to
// browseRepoLocalBindings/readyLocalBindings only), both untouched)
// mirroring treeFilterBox's own wording exactly.
var filterMenuCategoryHint = keybind.NewBinding(keybind.WithKeys("tab", "shift+tab"), keybind.WithHelp("tab/shift+tab", "category"))

// filterStripApplyHint is enter's Filter-Strip-local binding under bean
// bt-8d35's Fokus-Modell: enter APPLIES the cursored
// value and keeps the focus in the region instead of closing it, so the
// global keys.Enter label ("open") would be a lie there. Same standalone-
// binding construction as filterMenuCategoryHint above -- and here the
// handler (keyFilterMenu, box_filter_facets.go) matches THIS value, so
// bt-z4w7's "the label IS the binding" holds literally.
var filterStripApplyHint = keybind.NewBinding(keybind.WithKeys("enter"), keybind.WithHelp("enter", "apply"))

// --- bean bt-z4w7 (B7): footer labels DERIVED from the active binding ---
//
// The bug this section closes is a CLASS, not two strings: a Footer Zone 3
// entry was picked by hand from the global keymap next to the handler that
// implements the key, so the label and the real binding drifted apart the
// moment either side moved. Three instances existed at once --
//
//  1. the Value-Menu advertised "s" over an `o`/`u`-opened menu,
//  2. the Blocking-Picker advertised keys.Toggle's "space/x" although
//     bt-a3a8 (D6) narrowed its toggle to space-only,
//  3. all three search-field pickers advertised keys.Up/keys.Down's "↑/i"
//     and "↓/k" although "i"/"k" are literal, typeable characters there
//     (keyParentPicker's own doc comment says so explicitly).
//
// The remedy is that every context-dependent footer key now comes from ONE
// accessor that the KEY HANDLER matches against too -- the label cannot
// describe a binding the handler does not have, because it IS that binding.
// TestPickerFooterKeysAreReservedNotTyped (footer_binding_source_test.go)
// holds the line generically: any advertised single-rune key that merely
// gets typed into a picker's search query fails the build.
//
// These are deliberately standalone keybind.Bindings, NOT keyMap fields --
// the same reasoning filterMenuCategoryHint documents above: they are
// overlay-LOCAL relabelings of an existing global key, and adding them as
// keyMap fields would trip TestHelpGroupsCoverEveryBindingExactlyOnce.

// pickerNavUpHint/pickerNavDownHint are the ARROW-ONLY nav labels for the
// three pickers that host an always-focused search field (Tag-/Parent-/
// Blocking-Picker). Those handlers switch on raw tea.KeyUp/tea.KeyDown
// precisely so "i"/"k" stay typeable inside the query -- so the global
// keys.Up/keys.Down labels ("↑/i", "↓/k") name an alias that does not exist
// in this context and would send the PO's cursor into the search box.
var (
	pickerNavUpHint   = keybind.NewBinding(keybind.WithKeys("up"), keybind.WithHelp("↑", "up"))
	pickerNavDownHint = keybind.NewBinding(keybind.WithKeys("down"), keybind.WithHelp("↓", "down"))
)

// blockingPickerToggleHint is the Blocking-Picker's space-only membership
// toggle -- matched by keyBlockingPicker AND rendered by
// blockingPickerLocalBindings, so the two cannot disagree. Replaces the
// shared keys.Toggle ("space/x Toggle facet"), whose "x" half stopped being
// true for this picker when bt-a3a8 gave it a search field (D6 there), and
// whose "facet" wording never described a blocking relation anyway.
var blockingPickerToggleHint = keybind.NewBinding(keybind.WithKeys(" "), keybind.WithHelp("space", "Toggle blocking"))

// pickerApplyHint/pickerSetHint/pickerDiscardHint are the picker-local
// relabelings of keys.Enter and keys.Back (bean bt-6nuz, PO finding #9).
//
// Same reasoning as blockingPickerToggleHint above, one rung further: the
// KEYS are unchanged (the handlers still match keys.Enter/keys.Back, so a
// rebind stays correct), only the words are. keys.Enter's global "open/
// confirm" and keys.Back's "back" are accurate for a tree row and vague
// here -- in a picker, enter commits a pending multi-select diff and esc
// throws it away, which is precisely the distinction the PO needs before
// pressing either. The pre-bt-6nuz hand-written hint strings already said
// "enter:save"/"enter:set"/"esc:discard"; those verbs were right, they were
// just written next to the render instead of derived, so the OUTER footer
// never got them. Now both surfaces read these.
//
// They are also materially shorter, which is what keeps the hint line on
// ONE row at 80 columns inside the modal (verified by the tmux smoke this
// bean requires).
var (
	pickerApplyHint   = keybind.NewBinding(keybind.WithKeys("enter"), keybind.WithHelp("enter", "save"))
	pickerSetHint     = keybind.NewBinding(keybind.WithKeys("enter"), keybind.WithHelp("enter", "set"))
	pickerDiscardHint = keybind.NewBinding(keybind.WithKeys("esc"), keybind.WithHelp("esc", "discard"))
)

// valueMenuGroupKey returns the binding that OPENS -- and therefore also
// closes -- the value menu for the given group (design decision a3's
// "esc/<key> schliesst", now group-aware; see the a3-Nachtrag in
// ../docs/topics/jira-style-experiment/design-spec.md). keyValueMenu matches
// this, valueMenuLocalBindings renders it, and valueMenuBox's own inline
// hint reads its Help().Key -- three surfaces, one source.
//
// An unrecognized/empty group falls back to keys.Status, mirroring
// valueMenuTitle's own "Set value" defensive fallback: a zero-value model in
// a render-only test must not panic, and `s` is the historical default.
func valueMenuGroupKey(group string) keybind.Binding {
	switch group {
	case "type":
		return keys.Type
	case "priority":
		return keys.Priority
	}
	return keys.Status
}

// valueMenuGroup reports which single group the open value menu is showing
// (B11/B12 made the menu single-group, box_menu_value.go) -- the context
// valueMenuGroupKey needs. "" for a closed/empty menu.
func (m model) valueMenuGroup() string {
	if len(m.menuItems) == 0 {
		return ""
	}
	return m.menuItems[0].group
}

// filterMenuLocalBindings is the Facet-Filter-Menu's own footer set
// (epic-E7-plan.md Task 7 Step 6, literal): keys.Toggle is exactly the
// "space: select/toggle" hint Q04 asked for, at the concrete overlay (the
// Filter-Menu) whose absence the PO actually noticed.
//
// bean bt-8d35: enter is the Strip-local "apply and
// hold the focus" key (filterStripApplyHint), not the global close -- the
// label follows the handler's own branch rather than restating keys.Enter.
func filterMenuLocalBindings() []keybind.Binding {
	return []keybind.Binding{keys.Up, keys.Down, filterMenuCategoryHint, keys.Toggle, keys.FilterClear, filterStripApplyHint, keys.Back}
}

// valueMenuLocalBindings is the Value-Menu overlay's own footer set
// (epic-E7-plan.md Task 7 Step 6). The group's OWN key doubles as a close
// alias here (keyValueMenu, box_menu_value.go: opened by s/o/u, ALSO closes
// on a second press of that same key, like Back) -- a genuine local binding
// of this overlay, not a stray global leaking through.
//
// bean bt-z4w7: `group` is what makes the label true. This used to hardcode
// keys.Status, so a Type-/Priority-Menu opened with o/u still told the PO to
// press `s` -- and, worse, keyValueMenu really did accept `s` there, because
// it matched the same hardcoded binding. Both sides now read
// valueMenuGroupKey(group).
func valueMenuLocalBindings(group string) []keybind.Binding {
	return []keybind.Binding{keys.Up, keys.Down, keys.Enter, valueMenuGroupKey(group), keys.Back}
}

// tagPickerLocalBindings is the Tag-Picker overlay's own footer set.
//
// DEVIATION from epic-E7-plan.md Task 7 Step 6's literal text (which lumps
// Tag-/Parent-/Blocking-Picker into ONE Toggle-free {Up,Down,Enter,Back}
// set): keyTagPicker (box_picker_tag.go) actually wires a toggle. Omitting
// it here would silently hide a real, working key and leave Q04's own
// general wording ("wenn ein Form/Overlay aktiv ist ... inkl. 'space:
// select/toggle'") only half addressed -- Q04's PO example (the Filter-Menu)
// was illustrative, not exhaustive.
//
// keys.TagToggle, NOT keys.Toggle (ERRATUM/D01-Nachtrag, bean bt-9ipw
// Review-R1 B01): inside the picker's always-focused search field, "x" is a
// literal, typeable character -- only space toggles there. Advertising the
// shared "space/x" Toggle label here would mislead exactly the way the
// stale NewTag hint (below) would have.
//
// keys.NewTag REMOVED (bean bt-9ipw, US-07-Reopen 2026-07-17, D01): the
// former separate `n`-gated free-text new-tag sub-mode this hint used to
// point at is GONE -- D01 consolidated the Tag-Picker into ONE always-
// focused search field, so "n" is now just a literal, typeable character
// (e.g. filtering for a tag containing "n") rather than a picker command.
// Advertising it here would be actively misleading post-consolidation.
//
// bean bt-z4w7: nav is pickerNavUpHint/pickerNavDownHint, not keys.Up/
// keys.Down -- "i"/"k" are typeable search characters here for exactly the
// same reason "x" is (keyTagPicker switches on raw tea.KeyUp/tea.KeyDown).
func tagPickerLocalBindings() []keybind.Binding {
	return []keybind.Binding{pickerNavUpHint, pickerNavDownHint, keys.TagToggle, keys.Enter, keys.Back}
}

// parentPickerLocalBindings is the Parent-Picker overlay's own footer set
// (epic-E7-plan.md Task 7 Step 6, literal) -- genuinely Toggle-free:
// keyParentPicker (box_picker_parent.go) is a single-select list, no
// space/x case at all.
//
// bean bt-z4w7: arrow-only nav labels, see tagPickerLocalBindings.
// bean bt-6nuz: enter/esc carry the picker-local verbs (pickerSetHint/
// pickerDiscardHint) -- the Parent-Picker SETS a parent, it does not
// "open/confirm" one.
func parentPickerLocalBindings() []keybind.Binding {
	return []keybind.Binding{pickerNavUpHint, pickerNavDownHint, pickerSetHint, pickerDiscardHint}
}

// blockingPickerLocalBindings mirrors tagPickerLocalBindings' own Toggle
// deviation -- keyBlockingPicker (box_picker_blocking.go) also wires a
// multi-select membership toggle (toggleBlockPending).
//
// bean bt-z4w7: that toggle is blockingPickerToggleHint (space-only), NOT
// the shared keys.Toggle this list used to advertise. bt-a3a8 (D6) gave
// this picker a search field and dropped "x" from its toggle so the letter
// stays typeable -- the footer kept saying "space/x Toggle facet" for a key
// combination that no longer existed. Nav is arrow-only for the same
// reason (see tagPickerLocalBindings).
// bean bt-6nuz: enter/esc carry the picker-local verbs (pickerApplyHint/
// pickerDiscardHint) -- enter commits the pending diff, esc discards it.
func blockingPickerLocalBindings() []keybind.Binding {
	return []keybind.Binding{pickerNavUpHint, pickerNavDownHint, blockingPickerToggleHint, pickerApplyHint, pickerDiscardHint}
}

// confirmGateLocalBindings is the shared footer set for the two Confirm-Gate
// overlays (Create/Delete).
//
// GAP-FILL, not literally named by epic-E7-plan.md Task 7 Step 6: that
// step's overlay-specific-set enumeration only names Value-Menu and
// Tag-/Parent-/Blocking-Picker, leaving overlayCreateConfirm/
// overlayDeleteConfirm -- two more real `m.overlay != overlayNone` values
// (types.go) -- without an assigned set. Both keyCreateConfirm
// (box_confirm_create.go) and keyDeleteConfirm (box_confirm_delete.go)
// really only answer to Enter/Back (their own modal body already bakes in
// the destructive/constructive verb, e.g. "enter: delete permanently" --
// this base-view fallback intentionally stays generic).
func confirmGateLocalBindings() []keybind.Binding {
	return []keybind.Binding{keys.Enter, keys.Back}
}

// searchLocalBindings is the inline Tree/Ready search input's own footer
// set while active (m.searchActive, keySearchInput, update.go) --
// epic-E7-plan.md Task 7 Step 6, literal.
func searchLocalBindings() []keybind.Binding {
	return []keybind.Binding{keys.Enter, keys.Back}
}

// paletteLocalBindings/helpLocalBindings are deliberately minimal base-view
// fallbacks (epic-E7-plan.md Task 7 Step 6, literal): paletteBox/helpBox
// (overlay_palette.go/overlay_shortcuts.go) already bake their own complete
// hint into their OWN modalPanel footer argument (non-empty, unlike the
// overlays above) -- the plan still names a base-view fallback for both, so
// one exists here too, kept intentionally short since the modal's own hint
// is the primary surface.
func paletteLocalBindings() []keybind.Binding { return []keybind.Binding{keys.Enter, keys.Back} }
func helpLocalBindings() []keybind.Binding    { return []keybind.Binding{keys.Back} }

// fullscreenDetailLocalBindings is the Detail-Vollbild's own Footer Zone 3
// set: keys.Back's Vollbild-exit meaning here is a non-obvious D03 rung
// (keyDetailFocus's Back-case, update.go) worth reinforcing at the point
// the PO is actually looking.
func fullscreenDetailLocalBindings() []keybind.Binding {
	return []keybind.Binding{keys.Back}
}

// fullscreenListLocalBindings is the Listen-Vollbild's own Footer Zone 3
// set. enter is the meaningful LOCAL key
// this mode adds (Listen-Vollbild -> Detail-Vollbild jump, keyFullscreen).
func fullscreenListLocalBindings() []keybind.Binding {
	return []keybind.Binding{keys.Back, keys.Enter}
}

// overlayLocalBindings dispatches m.overlay to its own footer set --
// extracted helper for contextualLocalHint's overlay case, below. A method
// on model (bean bt-z4w7) rather than a free function taking overlayID: the
// Value-Menu's set now depends on WHICH group is open, which only the model
// knows (m.valueMenuGroup()).
func (m model) overlayLocalBindings() []keybind.Binding {
	switch m.overlay {
	case overlayValueMenu:
		return valueMenuLocalBindings(m.valueMenuGroup())
	case overlayTagPicker:
		return tagPickerLocalBindings()
	case overlayParentPicker:
		return parentPickerLocalBindings()
	case overlayBlockingPicker:
		return blockingPickerLocalBindings()
	case overlayCreateConfirm, overlayDeleteConfirm:
		return confirmGateLocalBindings()
	}
	return nil
}

// contextualLocalHint is Footer Zone 3's single source for BOTH
// Chrome-calling views (browseRepoChrome/view_browse_repo.go,
// readyChrome/view_browse_ready.go): view-local (viewLocal) in the
// normal state, but swaps to the active capture state's OWN bindings the
// instant one fully captures input (Q04-Antwort, PO-Nachtrag 5). Priority
// order (epic-E7-plan.md Task 7 Step 6): Filter-Menü > Overlay > Suche >
// Palette > Help > view-local default -- this is its OWN independently
// chosen order for footer display, NOT a mirror of handleKey's own
// full-capture dispatch order (update.go), which checks m.searchActive
// BEFORE m.filterOpen (the reverse of the order here). Harmless in
// practice since the two states are mutually exclusive (only one capture
// state is ever active at a time), but the two orderings must not be
// conflated (T7-Review I03, bean bt-dsog).
func (m model) contextualLocalHint(viewLocal []keybind.Binding) string {
	switch {
	case m.filterOpen:
		return renderBindings(filterMenuLocalBindings())
	case m.overlay != overlayNone:
		return renderBindings(m.overlayLocalBindings())
	case m.searchActive:
		return renderBindings(searchLocalBindings())
	case m.paletteOpen:
		return renderBindings(paletteLocalBindings())
	case m.helpOpen:
		return renderBindings(helpLocalBindings())
	case m.fullscreen == fullscreenDetail:
		// F01 (E9 Task 8, bean bt-1vbp): a further Capture-artiger Zustand,
		// slotted in AFTER Help/Palette/Suche/Overlay/Filter (none of those
		// can be active WHILE fullscreen != fullscreenNone anyway -- every
		// full-capture state routes handleKey's dispatch back to Split-Modus
		// first) but BEFORE the final viewLocal fallback, whose Tree/Ready-
		// specific hints (tab/shift+tab/search/…) are non-functional here.
		return renderBindings(fullscreenDetailLocalBindings())
	case m.fullscreen == fullscreenList:
		return renderBindings(fullscreenListLocalBindings())
	}
	return renderBindings(viewLocal)
}
