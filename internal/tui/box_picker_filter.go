package tui

// box_picker_filter.go — the shared search+chip-strip component both
// candidate pickers wear (bean bt-a3a8, PO-Nebenbefund N7 2026-07-20): the
// Blocking-Picker (`r`, box_picker_blocking.go) and the Parent-Picker (`a`,
// box_picker_parent.go) used to list EVERY candidate bean unfiltered, which
// at realistic repo sizes (sproutling 114 beans, beans-tui 123) means blind
// scrolling. This file is the ONE place their new filtering lives, so the
// two pickers can never drift apart on how search or a chip behaves.
//
// PORT, NOT NEUBAU (the bean's own framing): the text half is the
// Tag-Picker's already-proven pattern (box_picker_tag.go, bean bt-9ipw D01)
// -- one persistent textinput.Model, focused immediately on open, every key
// but the explicitly reserved ones belongs to the input, and a case-
// insensitive substring filter recomputed only when the input's value
// actually changed. The chip half reuses box_detail_form.go's scalarCell/
// gridRow/dropdownBox primitives verbatim, the SAME widgets box_filter_bar.go
// paints the Browse view's filter row with -- so the picker's strip and the
// Browse strip read as one widget language (design-spec.md D01/D08).
//
// DESIGN DECISIONS (bean bt-a3a8):
//
// D1 -- PICKER-LOCAL STATE, never the global facets. pickerFilter is a plain
// value hung off the model per picker (blockFilter/parentFilter, types.go);
// nothing here ever reads or writes m.filterStatus/Type/Priority/Tag or
// m.searchQuery. Opening a picker must not silently re-filter the Browse
// view behind it (the bean's explicit "(Test!)" criterion, guarded by
// TestBlockingPickerDoesNotMutateGlobalFilter).
//
// D2 -- SINGLE-VALUE CYCLING CHIPS, not multi-select maps. The global filter
// menu (box_filter_facets.go) is a multi-select checkbox list because it is
// the PO's persistent working set; a picker's filter is a throwaway "get me
// to that one bean" narrowing that dies with the overlay. One value per
// facet cycles with a single keystroke, needs no sub-overlay of its own
// (a picker cannot host a second modal), and fits dropdownBox's one-value
// slot exactly. "" means unset/(any).
//
// D3 -- CTRL-CHORD HOTKEYS (ctrl+t/n/p/g), never bare letters. This is the
// bt-9ipw/D01 lesson applied preemptively: while a search field is focused,
// every plain letter MUST stay typeable, so a chip cannot claim `t`/`s`/`p`
// (all three are live global action keys, keymap.go) without making them
// permanently untypeable here -- exactly the bug that made tags like
// nginx/linux unfilterable in the Tag-Picker before B01. A textinput can
// never consume a ctrl-chord as a literal character, so the two layers are
// structurally incapable of colliding. Tags takes ctrl+g because ctrl+t is
// already Type's ("t" is the global TagAssign key, so the mnemonic letter
// was taken); Status takes ctrl+g's sibling ctrl+n rather than the mnemonic
// ctrl+s, which design-spec.md §7 forbids outright (XOFF/XON flow control,
// guarded by TestKeymapNoCtrlSQ). Every chip renders its own hotkey badge,
// so neither non-mnemonic choice depends on the PO guessing it.
//
// D4 -- TEXT MATCHES TITLE OR ID, case-insensitive substring, no Bleve.
// Same contract as beanMatchesSearch's own sub-3-char local fallback and
// filterTagItems' deliberate "NO fuzzy scoring" (YAGNI): a picker filter is
// dispatched against an in-memory candidate list of at most a few hundred
// rows, where an async subprocess round-trip would only add latency and a
// staleness guard for no gain.

import (
	"github.com/rierek/lif/pkg/item"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// pickerFilterPlaceholder is the search field's prompt text. Named (not an
// inline literal) because TestPickerBoxesRenderFilterChrome asserts on it --
// the assertion can never drift from what is actually rendered.
const pickerFilterPlaceholder = "type to filter (title or id)"

// pickerFilterChromeLines is how many rendered lines the new chrome costs a
// picker overlay: filterBarHeight (3, box_filter_bar.go's own const -- a
// gridRow of dropdownBoxes is always exactly 3 lines) + 3 search field lines
// + 1 blank separator. The bean's "Overlay-Höhe: Strip + Suchzeile kosten
// Zeilen — Kandidatenliste entsprechend kürzen" requirement is honored by
// subtracting this from the row budget (pickerRowWindow below)
// rather than by letting the modal grow past the terminal.
//
// bean bt-6nuz (#8): the search field went from ONE bare line to a framed
// three-line box (searchBox), so this grew by 2 and the row window pays for
// it -- TestPickerBoxesFitIn24Lines proves the overlay still fits 80x24.
const pickerFilterChromeLines = filterBarHeight + 4

// pickerFilterFrameLines is what a picker overlay spends OUTSIDE its row
// list and outside the filter chrome: modalPanel's own top/bottom border,
// title and the blank line after the rows (4), plus the hint line, which
// wraps to 2 at narrow widths (2). Empirically pinned by
// TestPickerBoxesFitIn24Lines rather than derived -- that test fails loudly
// if a future chrome change makes this estimate wrong.
const pickerFilterFrameLines = 6

// pickerMinRows is the floor pickerRowWindow never goes below -- on a
// terminal too short to honor it the overlay does overflow, but showing ONE
// candidate is still strictly more useful than showing none.
const pickerMinRows = 3

// pickerRowWindow windows rows around cursor so the overlay FITS a terminal
// of termH lines.
//
// ERRATUM vs. parentPickerRowBudget's own documented contract ("caps 14
// windowAround SLICE ELEMENTS, i.e. 14 rows, not 14 visible terminal
// lines"): that judgment call was accepted while the overlay had no chrome
// and most titles stayed single-line. bean bt-a3a8 adds 5 lines of chrome
// (strip + search field) on top, and the 80-column tmux smoke this bean
// requires showed the result running off a standard 24-line terminal --
// rows really can be multi-line (hangingIndentWrap), so a fixed element cap
// cannot bound the rendered height at all.
//
// So this counts LINES, not elements: it walks outward from the cursor row,
// admitting neighbours only while their real rendered height still fits the
// budget. That makes the fit a property of what was actually rendered
// rather than an estimate, and it degrades gracefully -- a window of long
// wrapped titles simply shows fewer of them.
// termH <= 0 means "not sized yet" -- a model that has never seen a
// tea.WindowSizeMsg (the pre-first-frame state, and what several render
// tests construct deliberately). That falls back to the pre-bt-a3a8 fixed
// element cap rather than clamping to pickerMinRows: an unknown terminal
// height is NOT a tiny one, and silently showing a single row there would
// be a worse lie than the old estimate. Mirrors windowAround's own
// "height <= 0 -> impose nothing" contract (view_browse_repo.go).
func pickerRowWindow(rows []string, cursor, termH int) []string {
	if len(rows) == 0 {
		return rows
	}
	if termH <= 0 {
		return windowAround(rows, parentPickerRowBudget, cursor)
	}

	budget := termH - pickerFilterChromeLines - pickerFilterFrameLines
	if budget < pickerMinRows {
		budget = pickerMinRows
	}
	if cursor < 0 || cursor >= len(rows) {
		cursor = 0
	}

	height := func(s string) int { return strings.Count(s, "\n") + 1 }

	used := height(rows[cursor])
	lo, hi := cursor, cursor // inclusive
	for {
		grew := false
		if hi+1 < len(rows) {
			if h := height(rows[hi+1]); used+h <= budget {
				used += h
				hi++
				grew = true
			}
		}
		if lo-1 >= 0 {
			if h := height(rows[lo-1]); used+h <= budget {
				used += h
				lo--
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	return rows[lo : hi+1]
}

// pickerFilter is one picker's throwaway narrowing state (D1): a focused
// search field plus four single-value facet chips ("" = unset/(any), D2).
type pickerFilter struct {
	input textinput.Model

	facetType     string
	facetStatus   string
	facetPriority string
	facetTag      string
}

// newPickerFilter builds a fresh, focused, empty filter -- called on EVERY
// picker open (wholesale-replace convention, openTagPicker's own doc-stamp),
// so a picker can never reopen carrying the previous session's query or
// chips (TestPickerFilterResetsOnReopen).
func newPickerFilter() pickerFilter {
	ti := textinput.New()
	ti.Placeholder = pickerFilterPlaceholder
	ti.Prompt = ""
	ti.CharLimit = 60
	ti.Focus()
	return pickerFilter{input: ti, facetStatus: statusOpen}
}

// statusOpen is the Status chip's default value: every status that is not an archive status.
// Cycle ^n past it to reach single statuses, then "(any)".
const statusOpen = "open"

func statusFacetOptions() []string { return append([]string{statusOpen}, data.StatusValues()...) }

// matchesBean is the AND-combined predicate: every SET chip must match, and
// the typed query must substring-match the title or the ID (D4). An unset
// chip and an empty query each impose no constraint, mirroring
// beanMatchesFacets' own "empty facet map matches everything" contract.
func (f pickerFilter) matchesBean(b *item.Item) bool {
	if b == nil {
		return false
	}
	if f.facetType != "" && b.Type != f.facetType {
		return false
	}
	if f.facetStatus == statusOpen {
		if !data.IsOpenStatus(b.Status) {
			return false
		}
	} else if f.facetStatus != "" && b.Status != f.facetStatus {
		return false
	}
	if f.facetPriority != "" && b.Priority != f.facetPriority {
		return false
	}
	if f.facetTag != "" {
		hit := false
		for _, t := range b.Tags {
			if t == f.facetTag {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	q := strings.ToLower(strings.TrimSpace(f.input.Value()))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(b.Title), q) ||
		strings.Contains(strings.ToLower(b.ID), q)
}

// cycleFilterValue steps cur one position through options, treating "" as
// the head of the ring: "" -> options[0] -> ... -> options[n-1] -> "" (D2).
// A value that has fallen out of options (e.g. the last bean carrying a tag
// was deleted while the picker sat open) resets to "" rather than sticking.
func cycleFilterValue(cur string, options []string) string {
	if len(options) == 0 {
		return ""
	}
	if cur == "" {
		return options[0]
	}
	for i, v := range options {
		if v == cur {
			if i+1 >= len(options) {
				return ""
			}
			return options[i+1]
		}
	}
	return ""
}

// filterChipValue is one chip's rendered value: the set value in the shared
// D08 "gesetzter Filter = Peach" salience style (filterBarActive,
// box_filter_bar.go -- reused rather than re-declared, so the picker strip
// and the Browse strip can never diverge on what "active" looks like), or a
// muted "(any)" when unset.
func filterChipValue(v string) string {
	if v == "" {
		return theme.Muted.Render("(any)")
	}
	return filterBarActive.Render(v)
}

// pickerFilterStrip renders the four-chip Type/Status/Priority/Tags row at
// exactly `width` cells -- structurally filterBar's twin (box_filter_bar.go),
// but reading the PICKER's own chips (D1) and carrying the ctrl-chord hotkey
// badges filterBar deliberately omits (its own D07: the Browse bar is a
// read-only status strip, whereas these chips ARE the control surface).
func (f pickerFilter) strip(width int) string {
	cells := []scalarCell{
		{"Type", filterChipValue(f.facetType), "^t"},
		{"Status", filterChipValue(f.facetStatus), "^n"},
		{"Priority", filterChipValue(f.facetPriority), "^p"},
		{"Tags", filterChipValue(f.facetTag), "^g"},
	}
	return gridRow(cells, width, -1) // -1: the picker filter strip has no field cursor (bt-1o4g)
}

// searchBoxLabel is the label boxTopBorder puts in the search field's own
// frame -- named because TestPickerBoxesFrameTheSearchField and the layout
// constants below both depend on the field being a real, framed box.
const searchBoxLabel = "Search"

// searchBox frames the query input exactly like every other field in the
// overlay (bean bt-6nuz, PO finding #8). Before this the input was rendered
// bare, a single unframed row directly beneath a strip of four framed
// dropdownBoxes -- the one element that fell out of the box language the
// rest of the picker (and the whole box-form Detail pane) speaks. It reuses
// boxTopBorder/boxBottomBorder rather than re-deriving a frame, so a change
// to the border vocabulary reaches this field too.
//
// No hotkey badge in the bottom border: the field is ALWAYS focused while a
// picker is open (newPickerFilter), so there is no key that would focus it
// and nothing truthful to advertise.
func (f pickerFilter) searchBox(width int) string {
	if width < 8 {
		width = 8
	}
	frame := lipgloss.NewStyle().Foreground(theme.Overlay)

	inner := width - 4
	val := clampVisible(f.input.View(), inner)
	pad := inner - lipgloss.Width(val)
	if pad < 0 {
		pad = 0
	}
	mid := frame.Render("│") + " " + val + strings.Repeat(" ", pad) + " " + frame.Render("│")

	return boxTopBorder(searchBoxLabel, width, frame) + "\n" + mid + "\n" + boxBottomBorder("", width, frame)
}

// chrome is the strip + search field block every picker box prefixes onto
// its row list -- one function so the two overlays cannot drift on layout.
func (f pickerFilter) chrome(width int) string {
	return f.strip(width) + "\n" + f.searchBox(width) + "\n\n"
}

// keyPickerFilter is the shared key step both pickers delegate their
// non-reserved keys to. It handles the four ctrl-chord chip cycles (D3) and
// otherwise feeds the key to the textinput. changed reports whether the
// visible candidate set may have moved (a chip cycled, or the input's value
// really changed) -- the caller recomputes its filtered slice and resets its
// cursor ONLY then, mirroring keyTagPicker's own prev/current value-changed
// guard: a value-preserving keystroke (left/right inside the field) must not
// throw away the cursor position the user just navigated to.
//
// The caller keeps ownership of esc/enter/space/up/down: those must be
// intercepted AHEAD of this, exactly as keyTagPicker intercepts them ahead
// of its own input, so navigation and multi-select keep working while the
// field is focused (the bt-9ipw/D01 acceptance criterion).
func (m model) keyPickerFilter(f pickerFilter, msg tea.KeyMsg) (out pickerFilter, cmd tea.Cmd, changed bool) {
	switch msg.Type {
	case tea.KeyCtrlT:
		f.facetType = cycleFilterValue(f.facetType, data.TypeValues())
		return f, nil, true
	case tea.KeyCtrlN:
		f.facetStatus = cycleFilterValue(f.facetStatus, statusFacetOptions())
		return f, nil, true
	case tea.KeyCtrlP:
		f.facetPriority = cycleFilterValue(f.facetPriority, data.PriorityValues())
		return f, nil, true
	case tea.KeyCtrlG:
		f.facetTag = cycleFilterValue(f.facetTag, m.tagFilterOptions())
		return f, nil, true
	}

	prev := f.input.Value()
	f.input, cmd = f.input.Update(msg)
	return f, cmd, f.input.Value() != prev
}

// pickerFilterHint is the shared hint line both picker boxes render above
// their chrome. bean bt-6nuz (PO finding #9) changed BOTH what it says and
// where it gets it from:
//
// STYLING -- it was one flat theme.Muted string, so an overlay footer looked
// nothing like the app's every other footer, which renders Key in Teal and
// action in Subtext (renderBindings, view.go). It now goes through
// renderBindings like all of them.
//
// SOURCE -- the text was hand-written next to the render ("space:toggle
// enter:save"), the exact two-sources shape bean bt-z4w7 eliminated
// everywhere else. It now takes the picker's OWN local binding set, the
// same accessor Footer Zone 3 renders for this overlay
// (blockingPickerLocalBindings/parentPickerLocalBindings,
// footer_context.go), so the inner hint and the outer footer are one string
// built once and cannot disagree.
//
// The four facet chords are deliberately NOT in that set: the chip strip
// badges ^t/^n/^p/^g in its own frames, and repeating an inline-badged key
// in the footer is precisely what bt-fy5d removed from the main view.
// "type:filter" went for the same reason -- the search field now carries a
// framed "Search" label of its own (#8).
// It also WRAPS at `width`, through the same ANSI-aware wrapText the main
// footer uses (footer, view.go). The old flat string was written straight
// into the modal body, where nothing wrapped it -- the 80-column tmux smoke
// for this bean caught the result splitting "esc back" mid-word across two
// lines ("esc b" / "ack"). renderBindings makes each entry wrap-atomic via
// nbsp and leaves only the " · " separators breakable, so a word-aware wrap
// breaks BETWEEN bindings or not at all (LESSONS-LEARNED Eintrag 4, the
// NBSP-Wordwrap-Falle).
func pickerFilterHint(bs []keybind.Binding, width int) string {
	return wrapText(renderBindings(bs), width)
}

// filterPickerItems returns the subset of items whose bean satisfies f.
// keepFirstSynthetic pins index 0 through unconditionally when it carries no
// bean id -- the Parent-Picker's "(No parent)" clear row (buildParentItems),
// which is an ACTION rather than a candidate: clearing a parent must never
// require emptying the search box first (documented design decision,
// TestParentPickerSearchNarrows). The Blocking-Picker passes false; every
// one of its rows is a real bean.
func filterPickerItems(idx *data.Index, items []pickerItem, f pickerFilter, keepFirstSynthetic bool) []pickerItem {
	out := make([]pickerItem, 0, len(items))
	for i, it := range items {
		if i == 0 && keepFirstSynthetic && it.id == "" {
			out = append(out, it)
			continue
		}
		if idx == nil {
			continue
		}
		if b, ok := idx.ByID[it.id]; ok && f.matchesBean(b) {
			out = append(out, it)
		}
	}
	return out
}

// pickerFilterReserved reports whether msg is one of the keys a picker must
// intercept BEFORE handing anything to keyPickerFilter -- kept next to the
// component it guards so a future rebind cannot leave one picker behind.
// Raw KeyUp/KeyDown deliberately, NEVER navKey's letter aliases ("i"/"k"
// must stay literal typeable characters, keyTagPicker's own rationale).
func pickerFilterReserved(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown:
		return true
	}
	return keybind.Matches(msg, keys.Back) || keybind.Matches(msg, keys.Enter)
}
