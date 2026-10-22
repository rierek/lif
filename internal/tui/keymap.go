package tui

// keymap.go — the central, typed single source of every bt keybinding, ported
// from the devd-TUI (~/dev/DeveloperDashboard/apps/cli-go/internal/
// tui/keymap.go). Scope for this port (design-spec.md §7, implementation-plan
// »E1 Task 7«): the global + node-focused bindings needed by the chrome layer
// and the App-Shell (T8). devd also carries Sort/Tags(-Manager)/Rename/Review-
// verdict/User-Story bindings — out of scope here per design-spec §9 (Tag-
// Manager-CRUD and Docs/Notes views don't exist in beans-tui) or belong to a
// later epic (Review-Cockpit, E4); they get added to this single source when
// their view lands, not speculatively now (YAGNI).
//
// Port-adaptation vs. devd: the Blocking-Picker (now `r`, Q06 remap, design-
// spec.md §15 PF-16, bean bt-ntoz/bt-d8kc — originally `B`) is new — beans
// has no devd equivalent (blocking/blocked_by relation, design-spec §4).
// `Editor` gains a bare `e` alongside `ctrl+e` (design-spec §7: "e/ctrl+e
// Editor").

import keybind "github.com/charmbracelet/bubbles/key"

// keyMap is the central, typed single source of every bt keybinding (mirrors
// devd DD2-47): navKey() derives the direction-cross normalization from
// Up/Down/Left/Right, the in-app help (?) and any future external doc
// generate from Help(), and re-binding a key changes exactly one place.
type keyMap struct {
	// Direction cross (evaluated by navKey — the up/down/left/right truth
	// lives here). Arrows are always an alias, per every binding below.
	Up    keybind.Binding
	Down  keybind.Binding
	Left  keybind.Binding
	Right keybind.Binding

	// Detail-Focus pair (PF-13, design-spec.md §15, E7 T6): tab and
	// shift+tab both toggle between the two regions, Tree and Detail, in
	// both directions (toggleRegion, update.go). A hidden Detail pane is
	// shown by the toggle.
	FocusIn  keybind.Binding // tab — toggle focus Tree<->Detail
	FocusOut keybind.Binding // shift+tab — toggle focus Tree<->Detail

	// Activation / return / global.
	Enter        keybind.Binding // enter — open/confirm
	Back         keybind.Binding // esc — back
	Quit         keybind.Binding // q — quit (confirm); ctrl+c quits at once
	Help         keybind.Binding // ? — help overlay
	Palette      keybind.Binding // K — Command-Center
	Picker       keybind.Binding // p — repo-picker
	Ready        keybind.Binding // b — Ready
	Search       keybind.Binding // / — search
	Filter       keybind.Binding // f — facet filter
	Yank         keybind.Binding // y — copy context (OSC52 + native)
	Refresh      keybind.Binding // ctrl+r — manual data reload
	DetailToggle keybind.Binding // D — show/hide the Detail pane

	FilterClear keybind.Binding // X — reset filters
	Toggle      keybind.Binding // space/x — toggle facet checkbox (E2 Task 4, bean bt-9ldr)
	Sort        keybind.Binding // S — cycle Ready sort mode (E2 Task 5, bean bt-gzu6)
	FlatView    keybind.Binding // G — Nested/Flat Browse toggle (jira-style-ui experiment S5, D07 uppercase = view/global)

	// Node-focused (act on the focused tree/list node).
	Status    keybind.Binding // s — status menu (all node types)
	Type      keybind.Binding // o — type menu (jira-style-ui experiment S4, D07 lowercase field action)
	Priority  keybind.Binding // u — priority menu (jira-style-ui experiment S4, D07 lowercase field action)
	Assign    keybind.Binding // a — parent assignment
	Blocking  keybind.Binding // r — blocking picker (Q06 remap: was B, PF-14 freed r)
	Create    keybind.Binding // c — create
	Delete    keybind.Binding // d — delete (confirm, no cascade -- E3 Task 6, bean bt-ppzb)
	TagAssign keybind.Binding // t — tag picker
	Editor    keybind.Binding // e / ctrl+e — edit body in $EDITOR

	// NewTag (B14, design-spec.md §15 PF-16, bean bt-ntoz, E8 Task 7, bean
	// bt-yqdy) is a Tag-Picker-LOCAL sub-mode binding, not a global
	// checkpoint key like the ones above -- keyTagPicker (box_picker_tag.go)
	// already wired the raw key ("n") correctly; this typed Binding exists
	// SOLELY so renderBindings() (view.go) can display it in the outer
	// Footer Zone 3 (tagPickerLocalBindings, footer_context.go) -- a raw
	// msg.String() comparison cannot be rendered.
	NewTag keybind.Binding // n — new tag (Tag-Picker free-text sub-mode)

	// TagToggle (ERRATUM/D01-Nachtrag, bean bt-9ipw Review-R1 B01) is the
	// Tag-Picker's OWN space-only toggle -- deliberately NOT a reuse of
	// Toggle (which binds " " AND "x"): inside the picker's always-focused
	// search field (bt-9ipw D01 consolidation), "x" must stay a literal,
	// typeable character (nginx/linux/unix/box), so ONLY space toggles
	// there -- space is safe to reserve because data.ValidTagName never
	// admits one. Filter menu / Blocking picker keep the full space/x
	// Toggle. A SEPARATE keybind.Binding (not a narrowed Toggle) mirrors
	// the RenameTag precedent below: TestHelpGroupsCoverEveryBindingExactly
	// Once identifies bindings by their OWN key set (" " alone vs " ,x"),
	// so the two coexist without tripping that drift guard.
	TagToggle keybind.Binding // space — toggle tag (Tag-Picker, space-only)

	// PickerFacet* (bean bt-a3a8, PO-Nebenbefund N7) are the Blocking-/
	// Parent-Picker's OWN chip-strip cycles (box_picker_filter.go). They are
	// deliberately CTRL-CHORDS, not the mnemonic bare letters t/s/p: both
	// pickers now host an always-focused search field, and a bare letter
	// intercepted ahead of a textinput becomes permanently untypeable there
	// -- the exact bug Review-R1 B01 found in the Tag-Picker (bean bt-9ipw,
	// see TagToggle's doc-stamp above). A ctrl-chord can never be consumed
	// as a literal character, so the control layer and the typing layer are
	// structurally incapable of colliding. Tags takes ctrl+g because "t" was
	// already spent on Type here; Status takes ctrl+n because design-spec.md
	// §7 FORBIDS ctrl+s outright (XOFF/XON flow control -- on many terminal/
	// tmux configs the sequence never reaches the app at all, guarded by
	// TestKeymapNoCtrlSQ). Both non-mnemonic choices are self-documenting on
	// screen: each chip renders its own hotkey badge (dropdownBox's hotkey
	// slot, pickerFilter.strip), so nothing depends on the PO guessing them.
	//
	// Like NewTag/TagToggle these are OVERLAY-LOCAL, not global checkpoint
	// keys: keyPickerFilter (box_picker_filter.go) dispatches on the raw
	// tea.KeyType, and these typed Bindings exist so the Help-Overlay
	// (helpGroups) documents them at all -- a raw msg.Type switch cannot be
	// rendered.
	PickerFacetType     keybind.Binding // ctrl+t — cycle the picker's Type chip
	PickerFacetStatus   keybind.Binding // ctrl+n — cycle the picker's Status chip
	PickerFacetPriority keybind.Binding // ctrl+p — cycle the picker's Priority chip
	PickerFacetTag      keybind.Binding // ctrl+g — cycle the picker's Tags chip

	// RenameTag (E10 Task 5, bean bt-y9my, epic bt-362n D13/D14) is a
	// Tag-Management-page-LOCAL binding on "e" -- the SAME raw key as the
	// GLOBAL Editor binding ("e"/"ctrl+e"), but a disjoint, mutually
	// exclusive display/dispatch context: viewTagManagement is a
	// FULL-CAPTURE page (D06, view_tag_management.go), so keyNodeAction's
	// own "e" case (which fires Editor) is structurally unreachable while
	// this page is open -- mirrors the established Ready "S" precedent
	// (same physical key, different view-local meaning, no functional
	// collision). A SEPARATE keybind.Binding (not a reuse of Editor)
	// because TestHelpGroupsCoverEveryBindingExactlyOnce identifies
	// bindings by their OWN key set ("e" alone vs "e,ctrl+e"), so the two
	// coexist without tripping that drift guard.
	RenameTag keybind.Binding // e — rename tag (Tag-Management page)

	// Fullscreen (F01 Kernmechanik, E9 Task 7, bean bt-13l7, design-spec.md
	// §15): `v` (view) -- verified free against every OTHER binding above
	// before wiring (no collision). Dispatch: keyFullscreen
	// (view_fullscreen.go), handleKey's checkpoint NACH FocusIn/FocusOut, VOR
	// keyNodeAction (update.go).
	Fullscreen keybind.Binding // v — fullscreen

}

// newKeyMap returns the currently active keybinding set. The direction cross
// uses the jkli layout ported 1:1 from devd (DD2-34: i=up, j=left, k=down,
// l=right — inverted-T around the right index finger); arrow keys remain a
// second binding in every direction so arrow-only users are never broken.
// Every handler that routes through navKey() therefore supports both
// automatically.
func newKeyMap() keyMap {
	return keyMap{
		Up:    keybind.NewBinding(keybind.WithKeys("up", "i"), keybind.WithHelp("↑/i", "up")),
		Down:  keybind.NewBinding(keybind.WithKeys("down", "k"), keybind.WithHelp("↓/k", "down")),
		Left:  keybind.NewBinding(keybind.WithKeys("left", "j"), keybind.WithHelp("←/j", "back/out")),
		Right: keybind.NewBinding(keybind.WithKeys("right", "l"), keybind.WithHelp("→/l", "in/expand")),

		FocusIn:  keybind.NewBinding(keybind.WithKeys("tab"), keybind.WithHelp("tab", "focus in")),
		FocusOut: keybind.NewBinding(keybind.WithKeys("shift+tab"), keybind.WithHelp("shift+tab", "focus out")),

		Enter: keybind.NewBinding(keybind.WithKeys("enter"), keybind.WithHelp("enter", "open/confirm")),
		Back:  keybind.NewBinding(keybind.WithKeys("esc"), keybind.WithHelp("esc", "back")),
		Quit:  keybind.NewBinding(keybind.WithKeys("q", "ctrl+c"), keybind.WithHelp("q", "quit")),
		Help:  keybind.NewBinding(keybind.WithKeys("?"), keybind.WithHelp("?", "help")),
		// bt-mx4k: ctrl+k retired -- K was long since bound and matches the
		// D07 case convention (uppercase = view/global). One key, one
		// function; the header gets six characters shorter, which counts at
		// 80 columns.
		Palette: keybind.NewBinding(keybind.WithKeys("K"), keybind.WithHelp("K", "commands")),
		Picker:  keybind.NewBinding(keybind.WithKeys("p"), keybind.WithHelp("p", "repos")),
		Ready:   keybind.NewBinding(keybind.WithKeys("b"), keybind.WithHelp("b", "Ready")),
		Search:  keybind.NewBinding(keybind.WithKeys("/"), keybind.WithHelp("/", "search")),
		Filter:  keybind.NewBinding(keybind.WithKeys("f"), keybind.WithHelp("f", "Filter")),
		Yank:    keybind.NewBinding(keybind.WithKeys("y"), keybind.WithHelp("y", "Yank")),
		Refresh: keybind.NewBinding(keybind.WithKeys("ctrl+r"), keybind.WithHelp("ctrl+r", "reload")),

		FilterClear: keybind.NewBinding(keybind.WithKeys("X"), keybind.WithHelp("X", "Clear filters")),
		Toggle:      keybind.NewBinding(keybind.WithKeys(" ", "x"), keybind.WithHelp("space/x", "Toggle facet")),
		Sort:        keybind.NewBinding(keybind.WithKeys("S"), keybind.WithHelp("S", "Sort")),
		FlatView:    keybind.NewBinding(keybind.WithKeys("G"), keybind.WithHelp("G", "Nested/Flat")),

		Status:   keybind.NewBinding(keybind.WithKeys("s"), keybind.WithHelp("s", "Status")),
		Type:     keybind.NewBinding(keybind.WithKeys("o"), keybind.WithHelp("o", "Type")),
		Priority: keybind.NewBinding(keybind.WithKeys("u"), keybind.WithHelp("u", "Priority")),
		Assign:   keybind.NewBinding(keybind.WithKeys("a"), keybind.WithHelp("a", "Parent")),
		// Label "Relations", nicht "Blocking" (PO-Befund #7): das Overlay heisst
		// Relations und die Detail-Pane ueberschreibt ihr Panel ebenso. Der
		// Bezeichner `Blocking` und `data.SetBlocking` bleiben, weil sie die
		// konkrete Beziehungsart meinen -- ebenso appendGroup("Blocking", ...)
		// in view_detail_bean.go, das eine Gruppe INNERHALB des Relations-Panels
		// benennt. Nur das ANZEIGE-Label folgt der Ueberschrift.
		Blocking:  keybind.NewBinding(keybind.WithKeys("r"), keybind.WithHelp("r", "Relations")),
		Create:    keybind.NewBinding(keybind.WithKeys("c"), keybind.WithHelp("c", "Create")),
		Delete:    keybind.NewBinding(keybind.WithKeys("d"), keybind.WithHelp("d", "Delete")),
		TagAssign: keybind.NewBinding(keybind.WithKeys("t"), keybind.WithHelp("t", "Tags")),
		Editor:    keybind.NewBinding(keybind.WithKeys("e", "ctrl+e"), keybind.WithHelp("e", "Edit")),

		NewTag: keybind.NewBinding(keybind.WithKeys("n"), keybind.WithHelp("n", "New tag")),

		TagToggle: keybind.NewBinding(keybind.WithKeys(" "), keybind.WithHelp("space", "Toggle tag")),

		PickerFacetType:     keybind.NewBinding(keybind.WithKeys("ctrl+t"), keybind.WithHelp("^t", "picker: type filter")),
		PickerFacetStatus:   keybind.NewBinding(keybind.WithKeys("ctrl+n"), keybind.WithHelp("^n", "picker: status filter")),
		PickerFacetPriority: keybind.NewBinding(keybind.WithKeys("ctrl+p"), keybind.WithHelp("^p", "picker: priority filter")),
		PickerFacetTag:      keybind.NewBinding(keybind.WithKeys("ctrl+g"), keybind.WithHelp("^g", "picker: tag filter")),

		RenameTag: keybind.NewBinding(keybind.WithKeys("e"), keybind.WithHelp("e", "Rename")),

		Fullscreen:   keybind.NewBinding(keybind.WithKeys("v"), keybind.WithHelp("v", "fullscreen")),
		DetailToggle: keybind.NewBinding(keybind.WithKeys("D"), keybind.WithHelp("D", "toggle detail")),
	}
}

// keys is the process-wide active keymap (single source).
var keys = newKeyMap()

// helpGroup is a named block of the shortcut overview (devd DD2-31/DD2-5
// pattern): the in-app help (?) and any external shortcut doc generate from
// this, so display and doc can never drift from the real bindings.
type helpGroup struct {
	title    string
	bindings []keybind.Binding
}

// helpGroups returns the keymap grouped for the in-app help overlay (added in
// a later task — the help view itself is out of scope here).
func (k keyMap) helpGroups() []helpGroup {
	return []helpGroup{
		{"Navigation", []keybind.Binding{k.Up, k.Down, k.Left, k.Right, k.Enter, k.Back, k.FocusIn, k.FocusOut, k.Fullscreen}},
		{"Views & Global", []keybind.Binding{k.Ready, k.Picker, k.Search, k.Filter, k.FilterClear, k.Refresh, k.Palette, k.Help, k.Quit, k.FlatView, k.DetailToggle}},
		{"Actions", []keybind.Binding{k.Status, k.Type, k.Priority, k.Assign, k.TagAssign, k.Blocking, k.Create, k.Delete, k.Editor, k.Yank, k.Toggle, k.TagToggle, k.Sort, k.NewTag, k.RenameTag}},
		// bean bt-a3a8: the Blocking-/Parent-Picker's own chip-strip cycles.
		// Their own group rather than an append to "Actions" -- they are
		// overlay-local controls that only exist while a picker is open, and
		// burying four ctrl-chords at the tail of a 15-item global action
		// list would read as if they worked anywhere.
		{"Picker filter", []keybind.Binding{k.PickerFacetType, k.PickerFacetStatus, k.PickerFacetPriority, k.PickerFacetTag}},
	}
}

// globalBindings is Header Zone 1's single source (D04, design-spec.md §15
// PF-16, bean bt-ntoz Grilling-Nachtrag/bt-d8kc -- SUPERSEDES PF-11's
// original 7-item set below): EXACTLY 4 globally-reachable bindings get
// their own dedicated header slot -- `Palette, Picker, Help, Quit`, in this
// exact order (rendered by renderBindings, D06's shared Teal-key/
// Subtext-desc optic). ctrl+r (Refresh)/esc (Back)/enter (Enter) fly OUT of
// the header entirely (PO: too much noise for a 4-slot budget that must fit
// 80 columns without wrap/truncation, TestGlobalBindingsFitIn80Columns) --
// they stay documented in the Help-Overlay only (helpGroups() is
// UNCHANGED, all three keys are already listed there) and do NOT resurface
// in either view-local footer list either (Q06's verbatim list omits all
// three too -- TestGlobalBindingsOmitsRefreshBackEnter guards the header
// side of that).
//
// FocusIn/FocusOut (PF-13) are deliberately NOT here despite being
// dispatched at the same global handleKey checkpoint as Palette/Picker/
// Quit -- dispatch POSITION and display BUCKET are two different axes (see
// bt-t1uy's own T7 notes): FocusIn/FocusOut stay footer/view-local
// (browseRepoLocalBindings/readyLocalBindings, now FIRST per Q06's
// ordering). Every other global-checkpoint key (Create/Status/TagAssign/
// Assign/Blocking/Delete/Editor/Yank/Ready/Search/Filter/FilterClear/
// Section) is the same -- reachable from anywhere, but shown per-view in
// the footer, not here (TestNoDuplicateBindingBetweenGlobalAndAnyLocalHint
// List guards the two footer lists against ever re-adding one of THESE 4).
//
// PF-11 precedent (superseded, kept for history): the original 7-item set
// was `{Refresh, Palette, Picker, Help, Back, Enter, Quit}`, rendered
// `ctrl+r:reload · ctrl+k:commands · p:repos · ?:help · esc:back ·
// enter:open/confirm · q:quit` -- the PO's own Grilling feedback found this
// too noisy; D04 is the direct fast-follow.
func globalBindings() []keybind.Binding {
	return []keybind.Binding{keys.Palette, keys.Picker, keys.Help, keys.Quit}
}

// bindHas reports whether key k is part of binding b (exact match).
func bindHas(b keybind.Binding, k string) bool {
	for _, v := range b.Keys() {
		if v == k {
			return true
		}
	}
	return false
}

// navKey normalizes a raw key to a canonical direction ("up"/"down"/"left"/
// "right") using the central keymap. Keys with no direction meaning pass
// through unchanged. Every handler that routes through navKey therefore
// supports arrow keys and the jkli layout equally, and follows any future
// re-binding (DD2-34-style) without an edit at the call site.
func navKey(k string) string {
	switch {
	case bindHas(keys.Up, k):
		return "up"
	case bindHas(keys.Down, k):
		return "down"
	case bindHas(keys.Left, k):
		return "left"
	case bindHas(keys.Right, k):
		return "right"
	}
	return k
}
