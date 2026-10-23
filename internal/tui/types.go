package tui

// types.go — the App-Shell model + viewID enum (T8, implementation-plan.md
// »E1 Task 8«). Elm value-receiver model, ported architecture-wise from devd
// (~/dev/DeveloperDashboard/apps/cli-go/internal/tui/types.go):
// viewID enum + a single model struct, no devd-API coupling (data layer is
// beans-native from the start, design-spec.md §3.1/§13).

import (
	pkgconfig "github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/roadmap"
	"time"

	"github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/data"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// viewID enumerates the top-level screens. T8 ships exactly one (the Browse
// Primat-View, design-spec.md §6 V2); later epics add Ready/Detail/
// Command-Center/Review/Forms/Overlays as siblings — the switch in View()
// (view_browse_repo.go) grows one case per view, never a branch here.
type viewID int

const (
	viewBrowseRepo viewID = iota
	viewReady             // V3 Ready (E2 Task 5, bean bt-gzu6, design-spec.md §6 V3)
	viewLobby             // V1 Lobby/Repo-Picker (E5 Task 6, bean bt-zhwl, design-spec.md §6 V1)
	// viewTagManagement (E10 Task 2, bean bt-r92i, epic bt-362n D05): the
	// Tag-Management page -- ALWAYS appended LAST (never resort the values
	// above), see this const block's own doc comment above.
	viewTagManagement
)

// Synthetic root node IDs below the roadmap tree. "Unscheduled" holds the
// roadmap's unscheduled containers and leaves; "Not on roadmap" holds every
// item the roadmap places nowhere (childless milestones, types with
// `roadmap: false`, invalid parents, parent cycles) as flat rows -- dropped
// items must never be silently invisible. The leading NUL byte guarantees
// these can never collide with a real item ID.
const (
	unscheduledRootID  = "\x00unscheduled"
	notOnRoadmapRootID = "\x00notonroadmap"

	sectionUnscheduled  = "Unscheduled"
	sectionNotOnRoadmap = "Not on roadmap"
)

// overlayID enumerates E3's node-action overlays (Value-Menü, Tag-/Parent-/
// Blocking-Picker, Create-/Delete-Confirm) -- mutually exclusive by
// construction (design decision a2, bean bt-dlgk): ONE model.overlay field
// (iota-enum) instead of six more bool fields alongside
// confirmQuit. confirmQuit is NOT retrofitted into this enum
// (documented, deliberate coexistence) -- handleKey's capture order settles
// precedence between them. m.form != nil (T4) is a third, separate capture
// state (huh forms are not a menu overlay). T1 wires overlayValueMenu only;
// T2/T3/T4/T6 add their cases to keyOverlay/composeOverlays as their
// overlays land -- the full closed set is declared here upfront so a new
// overlay is always a new `case`, never a new bool.
type overlayID int

const (
	overlayNone overlayID = iota
	overlayValueMenu
	overlayTagPicker
	overlayParentPicker
	overlayBlockingPicker
	overlayCreateConfirm
	overlayDeleteConfirm
	overlayTaskOutput
	overlayFilterMenu
)

// Picker-Stil-Divergenz (E3-T3-Review PFLICHT, carried into bean bt-ppzb/E3
// Task 6, closes the review's "Doc-Note" finding): the overlays above render
// their cursored row in TWO different styles, both deliberate, neither a
// drift bug. box_menu_value.go (valueMenuBox) and box_picker_tag.go
// (tagPickerBox) use theme.Header.Render plus a leading "▸ " glyph on an
// otherwise PLAIN value/tag string. box_picker_parent.go (parentPickerBox)
// and box_picker_blocking.go (blockingPickerBox) instead use the D08 cursor
// treatment shared with the Tree/Ready panes (ansi.Strip + a whole-line
// theme.Accent.Render("▌"+plain) wrap) -- because THEIR rows are
// relationRow-rendered (already carrying their own status-icon/type-icon
// colors), so a second Header-wrap would clash with the existing per-cell
// theming instead of highlighting it. Full rationale lives on
// parentPickerBox's own doc comment (box_picker_parent.go); this note exists
// so a reader scanning the overlay set here doesn't mistake the split for an
// unnoticed inconsistency.

// I01 (bean bt-7jr8, T8-review): every map[string]bool field on model
// (expanded, and E2's new filter facet sets) is COPY-ON-WRITE, never mutated
// in place. Rationale: model is a value-receiver Elm architecture (design-
// spec.md §3.3) -- Go map values are reference types, so mutating one
// in-place silently aliases every other struct copy holding the same map
// header (e.g. an old model variable a test kept around, or a future
// undo/diff feature). Bubbletea's own single-active-model discipline
// (Update always returns a fresh value, the caller discards the old one)
// currently hides this hazard, but it is not guaranteed to stay hidden as
// more map fields are added (E2 Task 4's filter facets) -- so every setter
// clones via cloneBoolMap before writing, closing the hazard permanently at
// negligible cost (these maps are always small: expand state + a handful of
// filter selections).
func cloneBoolMap(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// model is the App-Shell state. T8 wires the read-only Tree (design-spec.md
// §6 V2 basis) + async load/reload/watch; mutation state (forms, pickers,
// menus) lands in E2/E3 as new fields on this same struct (devd port
// convention: one model, one viewID switch, no per-view sub-models).
type model struct {
	view viewID

	width, height int

	store   *data.Store
	repoDir string

	idx *data.Index
	err string // rendered in the status line; "" = no error to show

	// The tree is the roadmap (roadmap.Build, rebuilt by applyLoaded on every
	// load): roadmapScheduled = milestone roots, roadmapUnscheduled = the
	// "Unscheduled" root's nodes, notOnRoadmap = items placed nowhere (flat
	// rows under the "Not on roadmap" root). treeParent maps a node ID to its
	// tree parent's ID (synthetic roots included) for expandAncestorsOf.
	// readyIDs is the `lif list --ready` set the Ready view shows.
	roadmapScheduled, roadmapUnscheduled []*roadmap.Node
	notOnRoadmap                         []*item.Item
	treeParent                           map[string]string
	readyIDs                             map[string]bool

	expanded map[string]bool // node ID (item ID or a synthetic root ID) -> expanded
	cursorID string          // currently selected node's ID (item ID or a synthetic root ID)

	// detailFocus: the Detail pane holds the focus, one stop of the focus
	// ring (stepFocus, layout.go); tab/shift+tab are deliberately NOT routed
	// through keymap.Right (which stays the tree's expand key).
	detailFocus bool

	// taskActions are the tui.actions of the open repo's config that passed
	// validation (setTaskActions). Label is resolved.
	taskActions []pkgconfig.TUIAction

	// taskOutput* hold the open overlayTaskOutput: its title, the sanitized
	// output lines and the index of the first visible line.
	taskOutputTitle  string
	taskOutputLines  []string
	taskOutputScroll int

	// Suche `/` (E2 Task 3, bean bt-4ep2, design-spec.md §6 V2): searchActive
	// is the textinput-focused typing state (single-key shortcuts inactive
	// while true, design-spec §7); searchQuery is the committed-OR-live query
	// (updated on every keystroke, mirrors devd's treeQuery -- there is no
	// separate "live preview vs. committed" split, enter only blurs the
	// input). searchBleveIDs/searchBleveFor answer the async Bleve half: once
	// searchQuery reaches >=3 chars, searchBleveFor tags which query the
	// CURRENT searchBleveIDs answers -- a searchBleveResultMsg whose query no
	// longer matches m.searchQuery on arrival is discarded (staleness guard,
	// messages.go). searchBleveIDs is always REPLACED wholesale with a fresh
	// map (never mutated in place), so it does not need the I01
	// cloneBoolMap convention above -- there is no shared-backing-array
	// hazard when every writer assigns a brand-new map value.
	searchActive       bool
	searchInput        textinput.Model
	searchQuery        string
	searchBleveIDs     map[string]bool
	searchBleveFor     string
	searchBleveLoading bool

	// Such-Präfixe `st:`/`ty:`/`pr:`/`tag:` (bt-2kfl, D02/D03 final,
	// epic-E13-plan.md »Item 3«): searchPrefixFacets/searchPrefixRest are
	// applySearchPrefixes' (search_prefix.go) output, recomputed from
	// searchQuery on EVERY keystroke (keySearchInput, D03) -- a SEPARATE
	// additive layer from filterStatus/filterType/filterPriority/filterTag
	// below (D02: the f-menu is never written by typed prefixes).
	// searchPrefixRest is what beanMatchesSearch's text half AND the Bleve
	// dispatch (maybeBleveCmd/applyBleveResult) run against instead of the
	// raw searchQuery -- typed prefix tokens themselves never reach Bleve.
	// filterSummary (box_filter_facets.go) reads searchPrefixFacets to render
	// the UNION with the f-menu's own facet maps.
	searchPrefixFacets map[string][]string
	searchPrefixRest   string

	// Facet filter `f`/`X`: ONE shared filter state for Tree, flat and Ready.
	// filterStatus/filterType/filterPriority/filterTag are COPY-ON-WRITE
	// (I01 doc-stamp above) -- every toggle clones via cloneBoolMap before
	// writing.
	//
	// The filter bar's fields (filterFields, box_filter_facets.go) are focus
	// targets: filterFocus says the focus is in the bar, filterField is the
	// focused field's index into filterFields. Activating a field opens its
	// value menu (overlayFilterMenu); filterItems is the flattened row list
	// built fresh each time the menu opens (buildFilterItems) and filterMenu
	// is its cursor, a GLOBAL index into filterItems.
	filterStatus, filterType, filterPriority, filterTag map[string]bool
	filterFocus                                         bool
	filterField                                         int
	filterItems                                         []ffItem
	filterMenu                                          listState

	// Ready `b` (E2 Task 5, bean bt-gzu6, design-spec.md §6 V3): readyList
	// is the Ready master pane's cursor (index-based, listState -- unlike
	// the Tree's bean-ID cursorID, the Ready is a flat, non-hierarchical
	// list, so an index is sufficient and mirrors devd's own readyCursor
	// shape). Kept in sync (setLen) by keyReady on every keypress it
	// handles and by the `b`-open case in keyTree, since readyVisible()'s
	// LENGTH can change out from under it (search/filter routes through the
	// SHARED keySearchInput/keyFilterMenu handlers, which know nothing about
	// readyList -- view_browse_ready.go's doc comment has the full
	// rationale). readySort is the active Sort-Toggle `S` mode: ""
	// (default) leaves the canonical order in place;
	// "status"/"priority"/"created"/"updated" are the four cycle stops
	// (nextReadySort, view_browse_ready.go).
	readyList listState
	readySort string

	// Nested/Flat Browse toggle `G` (jira-style-ui experiment S5, D07
	// uppercase = view/global group): flatView switches the Browse view's
	// LEFT pane between the Tree (nested, default) and a flat, sorted list
	// (columns) -- the master-detail split and the Detail pane itself are
	// UNCHANGED (unlike the full-screen Ready view, view_browse_ready.go
	// -- flat mode here is still Browse, master-detail stays intact). flatList
	// is a minimal index-based cursor (listState, mirrors readyList's own
	// shape/rationale) over flatVisible()'s bean slice -- kept deliberately
	// SEPARATE from the Tree's cursorID rather than reconciled into one
	// shared cursor: the Tree's bean-ID cursor and a flat index cursor are
	// different shapes with no natural unification, and the Tree's own
	// cursorID/expanded state must survive a round-trip through flat mode
	// untouched (toggling G back resumes the Tree exactly where it was).
	// Default false -- existing tree goldens stay byte-identical.
	flatView bool
	flatList listState

	// Box-form Detail-Pane scroll (bean bt-ze10, epic bt-vy1q F1):
	// detailBoxForm (box_detail_form.go) renders Title + scalar-grid +
	// Body/Relations/History as ONE tall string with NO windowing of its
	// own. boxFormScroll is the offset renderDetailBox windows that string
	// to (scrollView, view.go), advanced/clamped by pgup/pgdn
	// (update.go), boxFormNav and wheelMove
	// (mouse.go, wheel over the Detail pane) via mouse.go's
	// adjustBoxFormScroll/boxFormScrollBounds. boxFormScrollBean is the bean
	// ID the offset was last computed for -- a SELECTION change is detected
	// lazily by comparing this against the currently focused bean's ID
	// (boxFormEffectiveScroll, mouse.go) wherever the offset is READ, rather
	// than hunted down at every one of the many cursor-move/click/jump call
	// sites that can change the selected bean (Golden-Rule-Drift-Schutz: one
	// derived check beats N independent, driftable reset sites). Both zero-
	// valued by default.
	boxFormScroll     int
	boxFormScrollBean string

	// Box-form Detail-Pane FIELD CURSOR (bean bt-1o4g, epic bt-vy1q, PO-
	// Nebenbefund N8 "keyboard-first"): the index into boxFormFieldOrder
	// (box_nav_field.go) the up/down keys move and enter activates while the
	// Detail pane is focused. boxFormCursorBean mirrors boxFormScrollBean's
	// derived-reset doctrine exactly (see above): the cursor is reset to the
	// first field lazily, where it is READ (boxFormEffectiveCursor), by
	// comparing this ID against the currently focused bean -- not at every
	// call site that can change the selection. Zero-valued by default, which
	// is ALSO the "cursor on Title" default, so no explicit init is needed;
	// an UNFOCUSED Detail pane renders with cursor -1 (no Mauve frame at
	// all).
	boxFormCursor     int
	boxFormCursorBean string

	confirmQuit bool

	// E3 (bean bt-dlgk): node-action overlays -- mutually exclusive by
	// construction (ONE enum field, not 6 bools; confirmQuit
	// predates this and is deliberately not retrofitted, see overlayID's
	// doc-stamp above). mutTarget is the bean ID the open overlay acts on,
	// captured at open time; the ETag is NEVER captured -- every submit
	// re-reads m.idx.ByID[mutTarget].ETag (beanETag, update.go) so
	// watch-reloads between open and submit are automatically honored
	// (design decision d). menu is the shared cursor for the value menu
	// (T1) / future pickers (T2/T3) -- one open at a time, so one field
	// suffices. menuItems is the value-menu's row list, built fresh at
	// open time (openValueMenu, box_menu_value.go). valueMenuAnchorField
	// (Slice C, bt-f0y9 "feld-verankertes Inline-Dropdown", D09 revidiert) is
	// the boxFormFieldOrder (box_nav_field.go) index the open value menu is
	// anchored to -- a STATIC group->field lookup (boxFormFieldIndexForGroup,
	// box_menu_value.go), seeded by openValueMenu itself in the SAME call
	// that flips overlay to overlayValueMenu, regardless of WHICH of the four
	// trigger paths called it (keyboard s/o/u, field-cursor Enter, mouse
	// click, Palette all funnel through openValueMenu(group) -- this is the
	// ONE place that needs setting, not four). Only ever READ while
	// m.overlay == overlayValueMenu (composeOverlays' placeValueMenuOverlay,
	// view_browse_repo.go), which openValueMenu never sets without also
	// freshly writing this field first -- so a stale value between opens can
	// never be observed. boxFormFieldIndexForGroup returns -1 for an
	// unrecognized group (defensive only, openValueMenu's callers never pass
	// one); placeValueMenuOverlay falls back to the pre-existing centered
	// placeOverlay whenever boxFormFieldRect can't resolve it (accordion mode
	// off entirely ignores this field, same as boxScroll/boxCursor above).
	overlay              overlayID
	mutTarget            string
	menu                 listState
	menuItems            []valueMenuItem
	valueMenuAnchorField int

	// Tag-Picker `t` (E3 Task 2, bean bt-8v69, box_picker_tag.go): tagItems
	// is the usage-counted, deterministically sorted row list (count desc,
	// then alpha -- collectTagCounts), built fresh at open time.
	// tagOriginal/tagPending are two INDEPENDENT maps seeded from the
	// focused bean's current tags (wholesale-replace convention, mirrors
	// searchBleveIDs -- NOT the I01 cloneBoolMap-on-every-write pattern for
	// SEEDING, since both are fresh at open). Every TOGGLE against
	// tagPending during the picker's lifetime still goes through
	// cloneBoolMap before writing, though (I01, same convention as
	// toggleFacet) -- the map is long-lived for the whole overlay session,
	// not a throwaway per-keystroke value, so the same aliasing hazard the
	// I01 doc-stamp describes applies.
	//
	// Consolidated Search (bean bt-9ipw, US-07-Reopen 2026-07-17, epic-E12-
	// plan.md »Item 1«, D01): tagInput/tagInputErr are now the Haupt-
	// Picker's OWN always-visible, always-focused search field -- the
	// former two-stage `t`->`n` entry (a separate tagInputActive sub-mode,
	// opened only via the `n` key) is GONE; openTagPicker focuses tagInput
	// directly on open, mirroring searchInput's "one persistent
	// textinput.Model, reset+focused on open" convention (openSearchInput),
	// just without a second gate in front of it. tagInputFiltered is
	// tagItems narrowed by a case-insensitive substring match against
	// tagInput's live value (filterTagItems, mirrors filteredRepos()'s
	// "empty query -> full list" contract, view_lobby.go) -- seeded full on
	// open, recomputed only when the input's value actually changes.
	// tagInputSuggestCursor is a plain int cursor over tagInputFiltered
	// (NOT a listState) -- THE Haupt-Picker's own row cursor now (space
	// toggles the row AT this cursor, replacing the old m.menu-driven
	// cursor over the unfiltered tagItems), reset to 0 whenever
	// tagInputFiltered is recomputed. Up/Down/space are deliberately
	// intercepted via RAW tea.KeyType/literal-space checks in keyTagPicker,
	// NEVER via navKey's letter-alias table (keys.Up/keys.Down also bind
	// "i"/"k") or keys.Toggle (which also binds "x" -- ERRATUM/D01-
	// Nachtrag, Review-R1 B01, rationale in keyTagPicker) -- this is a
	// free-text capture field, so "i"/"k"/"x" must stay
	// literal, typeable characters (e.g. a tag named "risk"). keyLobby's own
	// repoQuery filter routes navKey() BEFORE its textinput update and thus
	// SWALLOWS typed "i"/"k"/"j"/"l" -- that is an EXISTING BUG there (bean
	// bt-l8e7), NOT a precedent to follow; the raw-KeyType intercept here is
	// the correct pattern for any free-text field with nav keys. `enter`
	// branches on tagInputFiltered/the typed value (keyTagPicker's own
	// doc-stamp): a non-empty filtered set (including the empty-query "all
	// tags" state) applies the pending diff and closes (applyTagPickerDiff,
	// UNCHANGED); typed text with NO substring match at all falls through to
	// the create-path (D11, reassigns tagItems/tagPending, then clears the
	// query back to the full list WITHOUT closing the picker).
	tagItems              []tagCount
	tagOriginal           map[string]bool
	tagPending            map[string]bool
	tagInput              textinput.Model
	tagInputErr           string
	tagInputFiltered      []tagCount
	tagInputSuggestCursor int

	// Parent-Picker `a` (E3 Task 3, bean bt-p1uz, box_picker_parent.go):
	// parentItems is the row list built fresh at open time
	// (buildParentItems) -- index 0 is ALWAYS the "(No parent)" clear row
	// (id "", port beans-src parentpicker.go's clearParentItem), the rest is
	// data.EligibleParents(idx, b) (self/descendants/invalid-types
	// pre-filtered, design decision f). Single-select, immediate-apply Enter
	// semantics (like the value menu, design decision a3) -- no
	// pending/original diff state here, a bean has exactly one parent.
	parentItems []pickerItem

	// parentFilter/parentFiltered (bean bt-a3a8, PO-Nebenbefund N7): the
	// picker's own throwaway search+chip state and the subset of
	// parentItems it currently admits. m.menu's cursor runs over
	// parentFiltered, NOT parentItems -- "select what's on screen", the
	// same semantics toggleTagPending gained under bt-9ipw/D01. The state
	// is deliberately PICKER-LOCAL (box_picker_filter.go D1): it never
	// touches m.filterStatus/Type/Priority/Tag, so opening a picker cannot
	// re-filter the Browse view behind it.
	parentFilter   pickerFilter
	parentFiltered []pickerItem

	// Blocking-Picker `B` (E3 Task 3, bean bt-p1uz, box_picker_blocking.go):
	// blockItems lists every OTHER bean (self excluded, deliberately NO
	// descendant/cycle exclusion -- design decision g: port-parity with
	// beans-src blockingpicker.go, which has none either). blockOriginal/
	// blockPending are two INDEPENDENT maps seeded from the focused bean's
	// CURRENT Blocking field (wholesale-replace convention, mirrors
	// tagOriginal/tagPending) -- every TOGGLE against blockPending during the
	// picker's lifetime goes through cloneBoolMap (I01), same convention as
	// toggleTagPending.
	blockItems    []pickerItem
	blockOriginal map[string]bool
	blockPending  map[string]bool

	// blockFilter/blockFiltered (bean bt-a3a8): the Blocking-Picker's own
	// half of the same picker-local search+chip state parentFilter
	// documents above. blockFiltered is what the cursor and every toggle
	// address -- blockItems stays the untouched full candidate source, so
	// clearing the query restores the whole list without a rebuild.
	blockFilter   pickerFilter
	blockFiltered []pickerItem

	// huh-Form-Hosting (E3 Task 4, bean bt-y4ly, Port devd forms_shared.go):
	// form is the embedded huh sub-model (nil when no form is open, the
	// THIRD separate capture state alongside m.overlay, design
	// decision a2 -- huh forms are not a menu overlay); formKind is "create"
	// (T4; T5 adds "editTitle") and drives both submitForm's dispatch
	// (box_confirm_create.go) and formTitle's label (forms_shared.go).
	// pendingCreate/createLabel/createDraft back the Create-Form's
	// Confirm-Gate: pendingCreate is the ALREADY-BUILT createCmd, parked by
	// submitForm until the Confirm-Gate's enter fires it; createLabel is the
	// modal's preview text; createDraft survives an n/esc Confirm-Gate
	// bounce so the Create-Form reopens FILLED instead of losing the PO's
	// work (Draft-Erhalt, port DD2-190, box_confirm_create.go).
	//
	// F1 (Review-Runde 2, Async-Gap-Clobbering): pendingCreate ALSO now
	// doubles as the "a create is in flight" guard -- keyCreateConfirm's
	// enter (box_confirm_create.go) deliberately no longer nulls it when
	// firing the parked Cmd (a deviation from the field's original "parked,
	// not yet dispatched" meaning); it stays non-nil for the WHOLE async gap
	// until createDoneMsg resolves (applyCreateDone below clears it on
	// EITHER outcome). keyNodeAction's Create case, submitForm's "create"
	// case AND dispatchPalette's "create" case (overlay_palette.go, E4 Task
	// 1, bean bt-jpgn -- the Command-Center is a genuine second entry point
	// to the SAME handlers) -- exactly THREE guarded call sites -- all check
	// `pendingCreate != nil` and refuse to start a SECOND create while one
	// is still in flight -- without this, the gap between the Confirm-Gate's
	// enter (overlay -> None, form -> nil) and createDoneMsg arriving is a
	// window where a second `c` (or a palette "create: bean") would cross-
	// contaminate these very same single-slot fields.
	//
	// ERRATUM vs. epic-E3-plan.md's epic-level "Geteilte Infrastruktur"
	// sketch (which additionally lists a `createConfirm bool` field
	// alongside these): design decision a2 is unambiguous that the E3
	// overlays -- INCLUDING Create-Confirm -- are ONE model.overlay
	// overlayID enum, not "6 weitere Bools neben confirmQuit";
	// Task 4's own Step 4 pseudocode agrees verbatim (`overlay =
	// overlayCreateConfirm`, never `createConfirm = true`). No separate bool
	// is added here -- the Confirm-Gate's open/closed state IS
	// `m.overlay == overlayCreateConfirm` (overlayID enum above), the sketch
	// mention is a stale holdover from an earlier draft.
	form          *huh.Form
	formKind      string
	pendingCreate tea.Cmd
	createLabel   string
	createDraft   *beanDraft

	// Editor (E3 Task 5, bean bt-sl45, design decisions c/h): editorTarget
	// captures WHICH bean's body a ctrl+e $EDITOR-Suspend acts on, set
	// immediately before tea.ExecProcess fires (the cursor cannot move
	// DURING a suspend, but capturing explicitly avoids relying on that --
	// same rationale as mutTarget above). The Title-Edit-Form ("e") reuses
	// mutTarget instead, mirroring the value-menu/picker convention (one
	// node-action target at a time) -- forms and the editor suspend are
	// mutually exclusive capture states (m.form != nil vs. a fired
	// editInEditor Cmd), so there is no collision between the two fields.
	//
	// editorETag (F2, Review-Runde 2 fix): the target bean's ETag, captured
	// in the SAME keyNodeAction branch as editorTarget, at $EDITOR-OPEN time
	// -- a deliberate DEVIATION from design decision d's "always read fresh
	// at submit" rule (m.beanETag(id), update.go). That rule is correct for
	// every other E3 overlay (open<->submit is a single keystroke), but
	// wrong for a potentially long-lived $EDITOR session: a watch-reload
	// landing WHILE the PO is still typing would silently rotate m.idx's
	// in-memory ETag out from under them, and a fresh re-read at submit
	// would then let SetBody sail through against a DIFFERENT bean state
	// than the one the PO's edit is actually based on -- a silent lost
	// update, no conflict ever raised. Freezing the etag at open instead
	// means that same external change now surfaces as a genuine, visible
	// ErrConflict (applyEditorFinished, update.go) -- optimistic-lock
	// semantics over the WHOLE session, not just the Submit<->Disk instant.
	editorTarget string
	editorETag   string

	// editorSnapshot (D01, design-spec.md §15 PF-17, bean bt-z4b1): the FULL
	// *item.Item value at $EDITOR-OPEN time, frozen in the SAME
	// openBeanEditor call as editorTarget/editorETag above -- unlike the
	// old openBodyEditor (SetBody-only, E3 Task 5/E8 B10, both SUPERSEDED
	// by this task), the whole-bean $EDITOR round-trip diffs EVERY field
	// individually against this snapshot (buildWholeEditDiff, editor.go),
	// not just the body -- ID+ETag alone are not enough. Always non-nil
	// together with a non-empty editorTarget (the single openBeanEditor
	// call site sets all three at once); applyEditorFinished/
	// applyBeanRawLoaded's error paths (update.go) clear all three
	// together on every exit, mirroring editorTarget/editorETag's own
	// contract.
	editorSnapshot *item.Item

	// Delete-Confirm `d` (E3 Task 6, bean bt-ppzb, box_confirm_delete.go):
	// delTitle/delChildren are captured at OPEN time (openDeleteConfirm) for
	// the modal's own render -- mutTarget (shared with every other overlay,
	// above) already carries the bean ID enter acts on, so no separate delID
	// field is needed. delChildren is idx.Children[id]'s DIRECT count only
	// (idx.Children is already in memory -- Port devd box_confirm_delete.go
	// MINUS its loadDeletePreview, no async count-load) -- deliberately NOT
	// data.CollectDescendants's full recursive descendant walk
	// (hierarchy.go/box_picker_parent.go's cycle-exclusion machinery):
	// `beans delete` does not cascade (mutations.go Delete's own doc-stamp),
	// so only the bean's IMMEDIATE children are ever affected -- a
	// grandchild's own parent (the direct child) stays intact either way.
	// ERRATUM (empirically verified, box_confirm_delete.go's own doc-stamp
	// has the full story): the affected direct children do NOT become
	// "(orphaned)"-bucket orphans as originally assumed (epic-E3-plan.md) --
	// beans 0.4.2's `delete` clears their `parent:` field outright, so they
	// become ordinary parentless ROOTS instead. deleteBox's copy reflects
	// the CORRECTED outcome, never devd's own "will also be deleted" cascade
	// wording either way.
	delTitle    string
	delChildren int
	// delLinks (Q01, E3-T6-Review PFLICHT finding, bean bt-qzwt) is the
	// DISTINCT count of OTHER beans that reference the delete target via
	// Blocking or BlockedBy, captured synchronously at open time (same
	// idx-in-memory convention as delChildren, box_confirm_delete.go's
	// countLinkedBeans). Empirically verified (scratch-repo probe, mirrored
	// in internal/data/client_mut_test.go's
	// TestDeleteClearsOtherBeansBlockedByReference/...BlockingReference):
	// `beans delete` silently clears these references in the OTHER bean's
	// frontmatter too -- the exact same "actively rewrites the file, no CLI
	// warning" behavior the parent-field ERRATUM above already documents,
	// just for a different link family. deleteBox's copy warns about this
	// the same way it warns about children losing their parent.
	delLinks int

	// watchUnavailable is set once (I04, T8 Opus quality review) when
	// data.Watch failed to start in app.go's Run: the App-Shell still works
	// (ctrl+r reloads manually), it just never reacts to on-disk changes on
	// its own -- this must be surfaced in the status line, never silently
	// degraded.
	watchUnavailable bool

	// Command-Center (E4 Task 1, bean bt-jpgn, design decisions a/b/h):
	// paletteOpen is a full-capture floating-overlay state, same precedent
	// as m.overlay (handleKey capture order, design decision h). palQuery
	// drives the action candidate pool -- palList is the cursor over the
	// filtered result list (palFiltered, rebuilt every keystroke). E4 Task 2
	// (bean bt-yo60) had added a SECOND candidate pool (matching beans mixed
	// in below the actions, with its own palBleveIDs/palBleveFor/
	// palBleveLoading Bleve staleness-guard triplet) -- removed again by B13
	// (design-spec.md §15 PF-16/"US-04-Revision", bean bt-ntoz, E8 Task 7,
	// bean bt-yqdy): the Command-Center shows ONLY commands now, bean search
	// is exclusively `/`'s job.
	paletteOpen bool
	palQuery    string
	palList     listState

	// Toast (E5 Task 1, bean bt-6dts, epic bt-5h4d, Port devd
	// overlay_show_toast.go, design decision a): ONE slot (no stack) --
	// nil = no toast shown. ONLY ever written by showToast/dismissToast/
	// handleToastExpired (update.go) and applyRepoSwitched's own
	// unconditional `m.toast = nil` reset (update.go, T6-Prelude I02
	// closure, T6b-Review bean bt-pd22 -- a repo switch is a bigger session
	// discontinuity than a same-repo reload, so even a sticky toast must not
	// survive it, see applyRepoSwitched's own doc comment) -- no OTHER call
	// site may assign m.toast directly (Grep-Audit, Task 1 Step 4), which is
	// exactly what lets a sticky toast (data.ErrConflict, applyMutationResult)
	// survive a beansLoadedMsg/watchMsg reload automatically: applyLoaded
	// never touches this field, so there is nothing to clobber it. Dual-Write
	// with m.err (above): m.err stays the Chrome status-line's Red slot,
	// UNCHANGED in content/semantics (>20 E1-E4 tests assert against its
	// string content) -- Toast is purely additive, fired alongside every
	// existing m.err assignment in update.go (design decision a).
	toast *toastState

	// toastSeqCounter is the toast generation source (T6b-Review Prelude I01
	// fix, bean bt-ggt2/T7): a model-WIDE counter, incremented by showToast
	// on every genuinely NEW generation (never on a debounced in-place
	// update) and NEVER reset by any of the four m.toast=nil sites above.
	// Fixes a real bug: the OLD scheme derived seq from `m.toast.seq + 1`,
	// which restarts at 1 the moment m.toast is nil'd (dismiss/expire/repo-
	// switch) -- a still-in-flight, non-cancelable toastTimeout tick from
	// the toast that existed BEFORE the reset then carries a seq that
	// COLLIDES with the very first toast shown after it, and
	// handleToastExpired's `msg.seq == m.toast.seq` check would dismiss that
	// unrelated new toast prematurely (even a sticky one, since the check
	// runs before the sticky/non-sticky branch). A monotonic counter makes
	// every generation's seq globally unique for the lifetime of the model,
	// so a stale tick can never alias a later, unrelated toast again.
	toastSeqCounter int

	// Help-Overlay (E5 Task 2, bean bt-wpn9, Port devd overlay_shortcuts.go):
	// helpOpen is a full-capture floating-overlay state, same precedent as
	// paletteOpen above -- `?` (keys.Help) opens it from ANY
	// view (design decision h, ctrl+k/Palette precedent), esc/?/q close it,
	// every OTHER key is swallowed as a no-op while it stays open (a
	// deliberate deviation from devd, which closes Help on any key --
	// overlay_shortcuts_test.go's file doc comment). No new keyMap fields:
	// Help has existed since E1 Task 7 and is already covered by the
	// Drift-Guard test TestHelpGroupsCoverEveryBindingExactlyOnce.
	helpOpen bool

	// Maus (E5 Task 4, bean bt-mne6, design decision f): lastClickIdx/
	// lastClickAt back the Tree's Doppelklick detection (Port devd
	// doubleClickInterval verbatim, update.go's now()) -- lastClickIdx is
	// the LAST clicked node's index into visibleNodes() (an index, not a
	// bean ID: devd's own port compares index equality, kept verbatim
	// here). clock is the (test-injectable) time source now() reads
	// (update.go) -- nil in production (falls back to time.Now()), fixed
	// in tests for a deterministic double-click window (mouse_test.go).
	lastClickIdx int
	lastClickAt  time.Time
	clock        func() time.Time

	// Settings (E5 Task 5, bean bt-0l8c, design decision c): the WHOLE
	// config.Settings struct (mirrors devd app.go's own m.cfg precedent,
	// epic-E5-plan.md's model-fields sketch) -- loaded once at TUI-start
	// (app.go Run(), config.LoadSettings) and re-assigned wholesale on every
	// Settings-Form submit (box_form_settings.go's submitForm "settings"
	// case, box_confirm_create.go) so the form's next open always prefills
	// from whatever was last saved, LIVE, no restart. Zero value (a fresh
	// model in tests that never went through Run(), or newModel()'s own
	// return before Run() assigns it) is config.Settings{} -- an empty
	// repos list, editor "", accent "", tree_width 0 (NOT
	// validateSettings' clamped default 36; this field is a plain struct
	// value, not a lazy accessor -- code that needs the clamped/defaulted
	// shape calls config.LoadSettings()/config.DefaultSettings() directly,
	// same as Run() does). buildSettingsForm tolerates the 0 fine
	// (strconv.Itoa(0) == "0", no crash) -- Run() populates this BEFORE
	// tea.NewProgram in the real app, so a live TUI session never observes
	// the zero value in practice.
	settings config.Settings

	// Lobby / Repo-Picker (E5 Task 6, bean bt-zhwl, design decisions d/h):
	// repoQuery/repoSearch/repoList mirror the Tree search's own shape
	// (searchQuery/searchInput/a listState cursor) -- repoQuery is the live
	// filter text over m.settings.Repos, repoSearch the backing
	// textinput.Model (constructed once in newModel, like searchInput/
	// tagInput), repoList the cursor over the CURRENTLY filtered result
	// (filteredRepos(), view_lobby.go). storeEvents is the CURRENT store's
	// change-event channel (Store.Watch): Update() re-arms
	// waitForStoreEvents(m.storeEvents) after each batch, and a repo switch
	// replaces it (applyRepoSwitched). nil is a legitimate steady-state value
	// (Watch failed to start, m.watchUnavailable carries that, I04
	// precedent); waitForStoreEvents(nil) is a no-op.
	repoQuery   string
	repoSearch  textinput.Model
	repoList    listState
	storeEvents <-chan []itemcore.ItemEvent
	// repoFilterOpen: the Lobby filter takes typed text; it opens with / only
	// (beans-u4uu), so q and the other keys stay commands. lobbyEscArmed: the
	// first esc in the Lobby; the second leaves it.
	repoFilterOpen bool
	lobbyEscArmed  bool

	// Archiv-Sicht (E5 Task 7, bean bt-ggt2, design decision e): showArchived
	// is a PROGRAM DEFAULT (false = completed/scrapped hidden), toggled via
	// the f-menu's own "Show archived" row (box_filter_facets.go,
	// facet "archive", the SAME space/x toggle path every other facet uses,
	// no new key). Deliberately NOT one of the four filterStatus/filterType/
	// filterPriority/filterTag maps: filterActive() must stay unaffected by
	// it (this is not a PO-set facet, doc-stamp there) -- it lives as its
	// own bool field instead, consulted by beanMatchesArchive
	// (box_filter_facets.go) AND, separately, by visibleNodes()'s own
	// routing condition (view_browse_repo.go) -- the latter is what makes
	// the default actually apply with NO search/facet active (the fully-
	// unfiltered flattenTree path is only reached when m.showArchived is
	// true AND nothing else is narrowing; see visibleNodes()'s own doc
	// comment for why this is NOT folded into treeActive() itself). Reset to
	// false on every repo switch (applyRepoSwitched, update.go) -- same
	// leak-prevention class as every other search/filter field reset there
	// (T6-Note bug class: a toggle from repo A must not silently apply to
	// repo B).
	showArchived bool

	// layout is the Browse/Ready layout cycled by v / shift+v (layout.go).
	// The zero value is layoutRoadmap, the start layout.
	layout layoutMode

	// Tag-Management page (E10 Task 2, bean bt-r92i, epic bt-362n D05-D09):
	// tagMgmtRows is the D09 Union row list (definierte Tags alpha-sortiert
	// zuerst, dann freie/in-Verwendung-Tags Count-absteigend), built fresh at
	// EVERY page-open (openTagManagementPage, D03 -- mirrors openLobby's own
	// "reload from disk on open" convention). tagMgmtCursor is a plain
	// listState cursor over that SAME row list (reuse, mirrors readyList's
	// index-based shape -- the page is a flat, non-hierarchical list, same
	// rationale as Ready's own doc-stamp above).
	tagMgmtRows   []tagRegistryRow
	tagMgmtCursor listState

	// Tag-Management page's own shared free-text input sub-mode (E10 Task 3,
	// bean bt-604w, epic bt-362n D11/D14): mirrors tagInput/tagInputErr's
	// "one persistent textinput.Model, reset+focused on open" convention
	// (box_picker_tag.go's own doc-stamp, types.go above) --
	// tagMgmtInputActive fully captures input WITHIN the already-full-capture
	// Page (D06), never a second overlayID case. tagMgmtInputMode is
	// "create" (T3, this task) or "rename" (T5, D14: Rename reuses this SAME
	// sub-mode instead of inventing a second one). tagMgmtInputTarget is the
	// OLD name being renamed -- empty for Create (D11: a new Definition never
	// targets an existing name), populated by T5's own prefill. tagMgmtInputErr
	// carries an inline validation/dedupe failure (ValidTagName or a
	// duplicate against m.tagMgmtRows' current name set) -- the input stays
	// open for a retry when set, same contract as tagInputErr one layer up.
	tagMgmtInputActive bool
	tagMgmtInputMode   string
	tagMgmtInput       textinput.Model
	tagMgmtInputTarget string
	tagMgmtInputErr    string

	// Tag-Management page's own Delete-Confirm (E10 Task 4, bean bt-1lsu,
	// epic bt-362n D12/D15): a page-local Bool+Ziel-Paar mirroring
	// m.confirmQuit's own "bewusst NICHT ins overlayID-Enum zurückgeholt"
	// precedent (types.go doc-stamp above) -- Delete is REGISTRY-ONLY (D12:
	// removing a tag Definition never touches any Bean, the tag simply
	// becomes "free" again), so keyTagMgmtDeleteConfirm's own enter path
	// dispatches the SAME saveTagDefsCmd/tagDefsSavedMsg tail Create (T3)
	// already introduced -- no second save path, no SetTags/Bean-mutation
	// call anywhere in this pair's own dispatch. tagMgmtDeleteTarget is the
	// tag name the Confirm is asking about; the LIVE usage count shown in
	// the modal is resolved from m.tagMgmtRows at RENDER time
	// (tagMgmtDeleteConfirmBox), not captured here as a third field.
	tagMgmtDeleteConfirm bool
	tagMgmtDeleteTarget  string

	// repoMetrics is the Lobby's own "Open/Total" column per configured
	// repo (design note, bean bt-zhwl: "Kosten/Latenz-Abwägung dokumentieren"
	// -- resolved as N independent async tea.Cmd dispatches, batched via
	// tea.Batch at Lobby-open time, NOT a synchronous N-subprocess loop
	// blocking the Lobby's own render; see repoMetricsCmd/repoMetricsBatchCmd,
	// messages.go). Keyed by repo path (Settings.Repos entries are already
	// the natural, unique key -- no separate ID needed). A missing entry
	// means "still loading" (repoPickerBody renders "…" for it);
	// repoMetric.err != nil means that ONE repo's `beans list` call failed
	// (a single misconfigured/moved repo must not blank out the whole
	// Lobby's metric column, each entry is independent). COPY-ON-WRITE (I01
	// doc-stamp above) -- applyRepoMetrics (update.go) clones before adding
	// an entry, same convention as every other model map field.
	repoMetrics map[string]repoMetric
}

// newModel builds the initial (pre-load) App-Shell state.
func newModel(store *data.Store, repoDir string) model {
	ti := textinput.New() // E2 Task 3: Tree search box (port devd app.go treeSearch)
	ti.Placeholder = "Search (title/ID, 3+ chars also searches Bleve)"
	ti.Prompt = ""
	ti.CharLimit = 80

	tagIn := textinput.New() // E3 Task 2/bean bt-9ipw: Tag-Picker's always-visible search/new-tag field
	tagIn.Placeholder = "type to filter or create (a-z0-9, hyphen-separated)"
	tagIn.Prompt = ""
	tagIn.CharLimit = 40

	tagMgmtIn := textinput.New() // E10 Task 3: Tag-Management page's own shared create/rename input (D14)
	tagMgmtIn.Placeholder = "new tag (a-z0-9, hyphen-separated)"
	tagMgmtIn.Prompt = ""
	tagMgmtIn.CharLimit = 40

	repoIn := textinput.New() // E5 Task 6: Lobby's own repo-filter input
	repoIn.Placeholder = "Filter repos (path)"
	repoIn.Prompt = ""
	repoIn.CharLimit = 200

	m := model{
		view:         viewBrowseRepo,
		store:        store,
		repoDir:      repoDir,
		expanded:     map[string]bool{},
		searchInput:  ti,
		tagInput:     tagIn,
		tagMgmtInput: tagMgmtIn,
		repoSearch:   repoIn,
	}
	return m.setTaskActions()
}
