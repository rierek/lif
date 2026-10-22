package tui

// view_tag_management.go — the Tag-Management page (E10 Task 2, bean
// bt-r92i, epic bt-362n D05-D09): a read-only, single-pane, FULL-CAPTURE top
// level view listing the Union of registry-defined tags (T1's
// internal/data/tagdefs.go, always shown even at count 0) and free/
// undefined tags currently in use on at least one bean. Create/Delete/
// Rename land in T3/T4/T5 -- this task ships ONLY the Grundgerüst: routing
// (D05, via the Command-Center "go to tags", mirrors the "go to settings"
// precedent -- no dedicated keybinding), the D06 full-capture dispatch, D07
// Chrome (empty GlobalHint), D08 single-pane layout (enter is a documented
// handled no-op, reserved for a future Master-Detail drilldown --
// idx.WithTag(tag) already exists), and D09's Union/sort algebra.
//
// Render structurally mirrors viewReady() (view_browse_ready.go): same
// innerW/innerH algebra, same clickPaneGeometry-sourced bodyH (Golden-Rule-
// Drift-Schutz, that helper's own doc-stamp), same renderPane wrapping --
// just ONE pane instead of a JoinHorizontal master-detail pair (D08). Each
// row carries a PF-12-style ALWAYS-reserved marker column (design-spec.md
// §15: "kein bedingtes Präfix, das das Layout verschiebt") distinguishing a
// defined row from a free row -- pre-borrowing the SAME reserved-gutter
// convention D10/T6 (bean bt-pqq3) will apply to the Tag-Picker's own
// suggest-mode rows, though this page has no Picker relationship of its own
// (bean bt-r92i's own "D10-Konvention vorgezogen ... hier noch ohne
// Picker-Bezug" wording).
//
// E10 Task 3 (bean bt-604w, D11/D14) adds Create: `n` (keys.NewTag, reused
// verbatim from the Tag-Picker) opens a page-LOCAL free-text input sub-mode
// that lives INSIDE this same full-capture page (D06) -- not a floating
// overlay/modalPanel like the Tag-Picker's own tagInputBox, since the Page
// itself already owns full input capture; a second, nested overlayID would
// only duplicate that capture, not add anything. The sub-mode is SHARED
// (D14) between Create (this task, tagMgmtInputMode "create") and Rename
// (T5, "rename") -- one textinput.Model, one open/close/validate path, two
// callers.
//
// E10 Task 4 (bean bt-1lsu, D12/D15) adds Delete: `d` (keys.Delete, reused
// verbatim -- same "delete" meaning as the Delete-Confirm elsewhere in the
// app, box_confirm_delete.go) on a DEFINED row opens a page-local
// Delete-Confirm (tagMgmtDeleteConfirm/tagMgmtDeleteTarget, D15 -- again NO
// new overlayID case, mirrors m.confirmQuit) showing the tag's LIVE usage
// count; `d` on a free row is a silent no-op (nothing to delete -- a free
// tag has no Definition). REGISTRY-ONLY (D12): confirming removes ONLY the
// Definition (data.RemoveTagDefName + the SAME saveTagDefsCmd/
// tagDefsSavedMsg tail T3 introduced) -- any Bean currently carrying the tag
// keeps it, the tag simply becomes "free" again on its next row-rebuild.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// tagRegistryRow is one row of the Tag-Management page's D09 Union list.
type tagRegistryRow struct {
	name    string
	count   int
	defined bool
}

// tagRegistryRows builds the D09 Union: every registry-defined tag in defs
// FIRST (alpha-sorted, deduped defensively -- LoadTagDefs already
// sorts/dedupes, T1, but this function must not crash or double-list if it
// is ever handed a raw, un-sanitized slice), carrying its REAL usage count
// (0 for an unused defined tag, T1's own "auch mit Count 0" wording) -- THEN
// every tag currently in use on at least one bean that is NOT in defs
// ("frei"/undefined), count-descending then alpha tie-break (mirrors
// collectTagCounts' own tie-break, box_picker_tag.go). A tag that is BOTH
// defined AND in use appears exactly ONCE, in the Defined group, with its
// real count -- never a duplicate row in the Free group.
//
// PRELUDE (T3, bean bt-604w, T6-Review F01): the COUNTING/UNION pass is now
// SHARED with collectTagCounts (box_picker_tag.go, T6, D10) instead of a
// second, separately-maintained loop over idx.ByID -- resolves the
// D10-ERRATUM-Notiz T2 (bean bt-r92i) itself forward-flagged ("once T6
// extends collectTagCounts with its own defined map[string]bool parameter,
// THIS function could call that extended version directly"). collectTagCounts
// already returns the identical defined+free Union, globally sorted (defined
// primary, count-desc/alpha secondary/tertiary, sortTagCountsDefinedFirst) --
// but D09's own grouping contract diverges from that global sort in exactly
// ONE place: the Defined group here sorts ALPHA ONLY (a predictable,
// index-like read), not count-desc like collectTagCounts' Picker-facing
// sort. So this function still owns its OWN re-split + re-sort of the
// Defined half (a stable partition of collectTagCounts' already-sorted
// output, re-sorted by name) -- the Free half's relative order coming out of
// that same partition is ALREADY exactly D09's own count-desc/alpha
// contract (collectTagCounts' secondary/tertiary keys), so it is reused
// AS-IS, no second sort. Page GROUPING (D09) stays Page logic; only the
// underlying COUNT/UNION pass is now shared (this task's own PRELUDE
// mandate: "Zählung geteilt, Page-Gruppierung bleibt Page-Logik").
func tagRegistryRows(idx *data.Index, defs []string) []tagRegistryRow {
	defSet := make(map[string]bool, len(defs))
	for _, name := range defs {
		defSet[name] = true
	}
	counts := collectTagCounts(idx, defSet)

	definedRows := make([]tagRegistryRow, 0, len(defSet))
	freeRows := make([]tagRegistryRow, 0, len(counts))
	for _, c := range counts {
		row := tagRegistryRow{name: c.tag, count: c.count, defined: c.defined}
		if c.defined {
			definedRows = append(definedRows, row)
		} else {
			freeRows = append(freeRows, row)
		}
	}
	sort.Slice(definedRows, func(i, j int) bool { return definedRows[i].name < definedRows[j].name })

	out := make([]tagRegistryRow, 0, len(definedRows)+len(freeRows))
	out = append(out, definedRows...)
	out = append(out, freeRows...)
	return out
}

// openTagManagementPage transitions into the Tag-Management page (D05
// entry): loads the registry FRESH from disk (D03, mirrors openLobby's own
// "reload from disk on every open" convention) -- synchronous, no tea.Cmd
// (D02: a local file read is fast enough for a direct call, same contract
// LoadTagDefs itself documents). A nil m.store (pre-load / test fixture)
// degrades to an empty registry rather than panicking -- mirrors D02's own
// tolerant-missing philosophy one layer up. Resets tagMgmtCursor (mirrors
// openLobby's repoList reset) so a stale cursor from a PREVIOUS, longer page
// visit never survives into a shorter re-open.
func (m model) openTagManagementPage() (tea.Model, tea.Cmd) {
	m.view = viewTagManagement
	var defs []string
	if m.store != nil {
		defs, _ = m.store.LoadTagDefs() // D02: LoadTagDefs never returns a non-nil error
	}
	m.tagMgmtRows = tagRegistryRows(m.idx, defs)
	m.tagMgmtCursor = listState{}
	m.tagMgmtCursor.setLen(len(m.tagMgmtRows))
	return m, nil
}

// tagManagementChrome builds viewTagManagement's own head/localKeys strings
// -- mirrors readyChrome's factoring-out rationale (view_browse_ready.go
// doc-stamp), even though this task wires no mouse-click geometry consumer
// for it yet (T3-T5 may add row-level mouse targets later, out of scope
// here). D07: GlobalHint is LEFT EMPTY (not renderBindings(globalBindings()))
// -- none of the 4 global keys (Palette/Picker/Help/Quit) function while
// this full-capture page holds input (D06), so showing them would be a
// misleading, non-functional promise (the exact bug class D06/PF-11 already
// fixed once for stale footer hints, epic bt-362n body's own D07 citation).
func (m model) tagManagementChrome(innerW int) (head, localKeys string) {
	head = breadcrumb(m.repoLabel(), "Tags", "", innerW)
	localKeys = footer(renderBindings(tagManagementLocalBindings()), innerW)
	return
}

// tagManagementLocalBindings is the page's own Footer Zone 3 local set.
// T2 shipped Up/Down/Back only (read-only Grundgerüst) -- T3 (bean bt-604w)
// appends keys.NewTag here (reuse of the EXISTING binding, same "n"/"new
// tag" meaning as the Tag-Picker's own, no new keybind.Binding introduced,
// keymap.go's helpGroups already lists it). T4 (Delete, bean bt-1lsu)
// appends keys.Delete here (same reuse precedent -- same "d"/"delete"
// meaning as the Delete-Confirm elsewhere in the app, no new
// keybind.Binding introduced). T5 (Rename) appends its own binding to this
// SAME shared function body as it lands (bean bt-r92i's own wording: "EIN
// gemeinsamer Funktionsrumpf, kein Duplikat je Task"), never a second,
// parallel bindings list. Ordering: NewTag/Delete sit BEFORE Back (mirrors
// tagPickerLocalBindings' own action-keys-before-Back convention,
// footer_context.go) -- an implementer decision, neither bean body
// specifies more than "hängt an".
func tagManagementLocalBindings() []keybind.Binding {
	return []keybind.Binding{keys.Up, keys.Down, keys.NewTag, keys.Delete, keys.RenameTag, keys.Back}
}

// tagManagementMarkerGlyph is the PF-12 reserved-gutter glyph (design-
// spec.md §15: "kein bedingtes Präfix, das das Layout verschiebt") marking a
// REGISTRY-DEFINED row -- a free/undefined row renders the SAME-WIDTH blank
// instead of omitting the column outright, so neither group's name column
// ever shifts relative to the other. U+2713 CHECK MARK is EAW=Neutral
// (Narrow, same class as theme.priorityGlyph's "‼"/"!" -- already
// column-safe without needing a LIF_TUI_ASCII_ICONS fallback variant, theme.go's
// own doc-stamp on that pair).
const tagManagementMarkerGlyph = "✓"

// tagManagementMarkerStyle colors the defined-marker glyph Green (a distinct
// hue from theme.Accent/theme.Dim, which the cursor-bar treatment below
// already claims for "this row has the cursor" -- reusing Accent here would
// conflate "defined" with "selected").
func tagManagementMarkerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Green) }

// tagManagementRowText renders one row's plain content to exactly w cells:
// the PF-12 marker column + a space + the (truncated if needed) name on the
// left, the usage count right-aligned via pickerRowFill (Port view_lobby.go
// repoPickerBody's own row-fill idiom for a right-aligned metric column) --
// same nameW-budgeting shape as repoPickerBody's own cursor+metric+nameW
// calc, so a narrow (80-column) terminal truncates the name with "…" instead
// of wrapping or overflowing (tmux-smoke acceptance wording).
func tagManagementRowText(r tagRegistryRow, w int) string {
	marker := " "
	if r.defined {
		marker = tagManagementMarkerStyle().Render(tagManagementMarkerGlyph)
	}
	count := theme.Muted.Render(strconv.Itoa(r.count))
	nameW := w - lipgloss.Width(marker) - 1 - lipgloss.Width(count) - 1
	if nameW < 1 {
		nameW = 1
	}
	left := marker + " " + truncate(r.name, nameW)
	return pickerRowFill(left, count, w)
}

// tagManagementRows renders every row using the SAME inline `▌`-cursor-bar
// convention as readyRows/treeRows (view_browse_ready.go/
// view_browse_repo.go) -- D08's own "structurally closer to Ready than to
// a floating overlay menu" call, NOT the Tag-Picker's ▸+Header style
// (box_picker_tag.go's Picker-Stil-Divergenz doc-stamp, types.go). paneW is
// renderPane's OWN outer width param (the caller passes the same value it
// hands to renderPane) -- content is budgeted to paneW-3 (paneW-2 is
// renderPane's own truncate budget, doc-stamp there; -1 more reserves this
// function's own 1-column cursor-bar prefix), so the fully-assembled row
// ("▌"+content or " "+content) fits renderPane's truncate call as a
// practical no-op instead of relying on it to silently cut a too-long line.
func (m model) tagManagementRows(rows []tagRegistryRow, focused bool, paneW, bodyH int) []string {
	contentW := paneW - 3
	if contentW < 1 {
		contentW = 1
	}
	pos := m.tagMgmtCursor.cursor
	out := make([]string, len(rows))
	for i, r := range rows {
		text := tagManagementRowText(r, contentW)
		if i == pos {
			plain := ansi.Strip(text)
			if focused {
				out[i] = theme.Accent.Render("▌" + plain)
			} else {
				out[i] = theme.Dim.Render("▌" + plain)
			}
		} else {
			out[i] = " " + text
		}
	}
	return windowAround(out, bodyH, pos)
}

// viewTagManagement renders the Tag-Management page -- mirrors viewReady's
// own algebra (view_browse_ready.go) exactly (innerW/innerH via the
// established w-2/h-2 convention, clickPaneGeometry-sourced bodyH, Golden
// Rule #1 via renderPane), just ONE renderPane call instead of a
// JoinHorizontal master-detail pair (D08 -- no lw/rw split).
func (m model) viewTagManagement() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	head, localKeys := m.tagManagementChrome(innerW)

	div := theme.Dim.Render(strings.Repeat("─", innerW))
	// bt-81f0 (Notifications vereinheitlichen, Q1-Annahme): m.err lost the
	// status line's rendering slot -- Toast is the ONE visible channel now.
	status := m.statusLine(innerW)

	// Same shared geometry source viewBrowseRepo/viewReady use (mouse.go
	// clickPaneGeometry) -- lw/rw are the SPLIT master-detail widths neither
	// of THIS page's single pane needs (D08), only bodyH is consumed.
	bodyH, _, _, _, _ := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, false)

	// paneW is the SINGLE pane's own CONTENT-width param passed to
	// renderPane/tagManagementRows -- mirrors viewBrowseRepo's own Vollbild
	// single-pane precedent (view_browse_repo.go, F01, bean bt-13l7's own
	// doc-stamp there): renderPane's `w` argument is a CONTENT width, its OWN
	// RoundedBorder adds +2 more on top (masterDetailWidths' lw+rw+4==innerW
	// is the identical accounting for the TWO Split panes; here there is
	// only ONE, so its content width is innerW-2, not innerW itself --
	// passing innerW unadjusted double-counts the border and overflows the
	// outer frame by 2 columns, exactly the class of bug view_browse_repo.go
	// line ~798 already documents "caught live via tmux smoke" -- caught the
	// SAME way here, this task's own tmux-smoke run).
	paneW := innerW - 2

	// E10 Task 3 (bean bt-604w, D14): while the shared input sub-mode is
	// active, the SAME pane's content swaps to the input's own hint/field/
	// error rows instead of the row list -- D06's "lebt INNERHALB der
	// Full-Capture-Page" wording taken literally: no separate overlay/
	// composeOverlays detour, just a different set of rows fed into the SAME
	// renderPane call below (which already truncates every row to paneW-2,
	// view.go's truncate helper, so an oversized error line degrades to a
	// "…"-suffixed cut instead of wrapping -- same budget the normal row
	// list already relies on, B01's own lesson one layer up).
	var listRows []string
	if m.tagMgmtInputActive {
		listRows = m.tagMgmtInputRows()
	} else {
		listRows = m.tagManagementRows(m.tagMgmtRows, true, paneW, bodyH)
		if len(m.tagMgmtRows) == 0 {
			listRows = []string{theme.Muted.Render("(no tags)")}
		}
	}
	listBox := renderPane(pane{rows: listRows}, paneW, bodyH, true)

	content := appendStatusLine(head+"\n"+div+"\n"+listBox+"\n"+div+"\n"+localKeys, status)
	out := outerBorder(content, innerW, true)

	return m.composeOverlays(out, w, h)
}

// keyTagManagement drives the open Tag-Management page (D06 full-capture,
// dispatched from handleKey at the SAME checkpoint as keyLobby -- see that
// check's own doc-stamp in update.go). up/down move tagMgmtCursor (reuse
// navKey, same convention as every other list cursor in this codebase);
// enter is a documented HANDLED no-op (D08 -- reserved for a future
// Master-Detail drilldown fast-follow, idx.WithTag(tag) already exists);
// esc (keys.Back) returns to Browse (D03/D06-Pendant: exactly ONE level
// back, mirrors keyLobby's own esc-with-a-live-client case); `n` (keys.NewTag,
// E10 Task 3, bean bt-604w, D11/D14) opens the shared Create/Rename input
// sub-mode; `d` (keys.Delete, E10 Task 4, bean bt-1lsu, D12/D15) opens the
// page-local Delete-Confirm on a defined row (no-op on a free one). Every
// OTHER key (including the global s/t/a/r/c/e node-action set AND
// `?`/ctrl+k/`p`) is silently swallowed -- the D06 regression this page
// exists to prevent.
func (m model) keyTagManagement(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// E10 Task 3 (bean bt-604w, D14): the shared input sub-mode is checked
	// FIRST and fully captures every key except enter/esc -- same precedent
	// as keyTagPicker's own `if m.tagInputActive` check (box_picker_tag.go).
	// Without this, typing e.g. "d" into a new tag name would fall through
	// to the switches below and either move a cursor or (were this page NOT
	// already itself full-capture) leak to a global action -- exactly the
	// Full-Capture-Disziplin the harness brief for this task calls out by
	// name.
	if m.tagMgmtInputActive {
		return m.keyTagMgmtInput(msg)
	}
	// E10 Task 4 (bean bt-1lsu, D12/D15): the Delete-Confirm sub-mode is
	// checked NEXT, after the Input sub-mode above -- Input and
	// Delete-Confirm are mutually exclusive by construction (Delete-Confirm
	// can only be OPENED from the base state below, `openTagMgmtDeleteConfirm`
	// is never called while `tagMgmtInputActive` is true), so this order is
	// never ambiguous, only a defensive precedence mirroring T3's own
	// checkpoint shape one case down.
	if m.tagMgmtDeleteConfirm {
		return m.keyTagMgmtDeleteConfirm(msg)
	}

	switch navKey(msg.String()) {
	case "up":
		m.tagMgmtCursor.move(-1)
		return m, nil
	case "down":
		m.tagMgmtCursor.move(1)
		return m, nil
	}

	switch {
	case keybind.Matches(msg, keys.Back):
		m.view = viewBrowseRepo
		return m, nil
	case keybind.Matches(msg, keys.Enter):
		// D08: handled no-op -- a future Master-Detail drilldown fast-follow
		// ("Beans zu diesem Tag") would land here, idx.WithTag(tag) already
		// exists (epic bt-362n body's own empirical-verification note).
		return m, nil
	case keybind.Matches(msg, keys.NewTag):
		// E11 Item 6 (bean bt-idm1, blocked_by bt-ct3k): a cursor resting on a
		// FREE row adopts that row's OWN name directly (Registrieren, no
		// input sub-mode) -- mirrors openTagMgmtRename's own Cursor-Row-Check
		// shape one function up. An out-of-range cursor (no rows) OR a row
		// that is ALREADY defined falls through to the unchanged Blank-Create
		// below (Registrieren ergibt hier keinen Sinn, Name existiert schon /
		// es gibt nichts zu adoptieren).
		if m.tagMgmtCursor.cursor >= 0 && m.tagMgmtCursor.cursor < len(m.tagMgmtRows) {
			if row := m.tagMgmtRows[m.tagMgmtCursor.cursor]; !row.defined {
				return m.openTagMgmtAdopt(row)
			}
		}
		return m.openTagMgmtInput("create", "")
	case keybind.Matches(msg, keys.Delete):
		return m.openTagMgmtDeleteConfirm()
	case keybind.Matches(msg, keys.RenameTag):
		return m.openTagMgmtRename()
	}
	return m, nil
}

// openTagMgmtInput enters the shared free-text input sub-mode (D14, mirrors
// openTagInput's own "reset value, focus, blink" convention,
// box_picker_tag.go): prefill seeds BOTH the input's visible text AND
// tagMgmtInputTarget -- for Create (T3, this task) prefill is always "",
// leaving tagMgmtInputTarget empty (D11: a new Definition never targets an
// existing name); T5's own Rename call site will pass the OLD name as
// prefill, pre-filling the field AND remembering it as the dedupe-exclusion/
// rename-source target in one shot.
func (m model) openTagMgmtInput(mode, prefill string) (tea.Model, tea.Cmd) {
	m.tagMgmtInputActive = true
	m.tagMgmtInputMode = mode
	m.tagMgmtInputTarget = prefill
	m.tagMgmtInputErr = ""
	m.tagMgmtInput.SetValue(prefill)
	m.tagMgmtInput.Focus()
	return m, textinput.Blink
}

// openTagMgmtRename enters the shared input sub-mode in "rename" mode (E10
// Task 5, bean bt-y9my, D13/D14): reads the CURRENTLY SELECTED row, No-Op
// (mirrors openTagMgmtDeleteConfirm's own no-op shape) when the cursor is
// out of range OR the row is a free/undefined one -- a free tag has no
// Registry entry to rename, the PO must `n`-define it first (bean body's
// own wording). A defined row's name seeds BOTH the input's visible text AND
// tagMgmtInputTarget in ONE openTagMgmtInput("rename", row.name) call (T3's
// own "Notes for T5" pointer) -- keyTagMgmtInput's existing dedupe loop
// already excludes tagMgmtInputTarget from the "name already in use" check
// (T3-introduced `name != m.tagMgmtInputTarget`), so a PO who re-confirms
// the unchanged old name is never rejected as a duplicate of themselves.
func (m model) openTagMgmtRename() (tea.Model, tea.Cmd) {
	if m.tagMgmtCursor.cursor < 0 || m.tagMgmtCursor.cursor >= len(m.tagMgmtRows) {
		return m, nil
	}
	row := m.tagMgmtRows[m.tagMgmtCursor.cursor]
	if !row.defined {
		// bt-ct3k (E11 Item 5, PO-Review Runde 7/8): the former silent no-op
		// read as a broken keybind -- a free row has no Registry Definition to
		// rename, but the PO must be TOLD that instead of nothing visibly
		// happening. toastWarn mirrors update.go's own existing warn-toast
		// precedent (e.g. the conflict/in-flight notes there); the Registry
		// itself stays untouched (no Cmd, no save).
		return m.showToast(toastWarn, "Unregistered tag — modification not possible", "n to define first", nil, false)
	}
	return m.openTagMgmtInput("rename", row.name)
}

// openTagMgmtAdopt directly registers a FREE row's own name as a new Registry
// Definition (E11 Item 6, bean bt-idm1, blocked_by bt-ct3k -- same file) --
// dispatched from keyTagManagement's keys.NewTag case ONLY when the cursor
// already sits on an undefined row (that check lives one layer up, this
// function trusts its caller and never re-checks row.defined itself).
// Planner-Entscheidung (final): DIREKTES Registrieren OHNE Zwischenschritt --
// no openTagMgmtInput detour, since the name is already fixed (the row's own
// name) and Registrieren is non-destructive (unlike Delete's own Confirm
// precedent, no confirm-step needed). saveTagDefsCmd's successToast carries a
// success message (bt-ct3k Toast-Konsistenz: that task just replaced a
// silent no-op with a Toast, so a new silent success path here would be a
// regression) -- applyTagDefsSaved (update.go) shows it, and its EXISTING
// cursor-refind-by-name tail lands the cursor on the now-defined row for
// free (mirrors Delete/Rename's own refindName convention), no separate
// handling needed here.
func (m model) openTagMgmtAdopt(row tagRegistryRow) (tea.Model, tea.Cmd) {
	// Defensive ValidTagName gate (Review-Finding, Fix-Runde): a free row's
	// name comes from a bean's ACTUAL on-disk tags -- a hand-edited bean file
	// can carry a name the tag grammar rejects. Mirror the Create path's own
	// pre-dispatch validation (keyTagMgmtInput, one function down): warn
	// Toast, NO Registry write -- and deliberately NO fallback to the
	// Blank-Create input either (the PO pressed n ON this row; silently
	// opening an empty input would misread that intent).
	if !data.ValidTagName(row.name) {
		return m.showToast(toastWarn, "invalid tag name (a-z0-9, hyphen-separated, lowercase)", "", nil, false)
	}
	defs := definedTagNames(m.tagMgmtRows)
	return m, saveTagDefsCmd(m.store, data.AddTagDefName(defs, row.name), row.name,
		fmt.Sprintf("tag '%s' registered", row.name))
}

// keyTagMgmtInput drives the open input sub-mode -- mirrors keyTagInput's
// structure 1:1 (box_picker_tag.go): esc discards ONLY the sub-mode (the
// Page underneath is untouched, D14); enter validates against
// data.ValidTagName, THEN dedupes against the CURRENT m.tagMgmtRows name set
// (defined AND free rows alike -- the bean body's own explicit wording,
// "Dedupe prüft also gegen ALLE vorhandenen Namen" for Create, where
// tagMgmtInputTarget is always "" -- T5/Rename will exclude its OWN old name
// via that field). A validation/dedupe failure sets tagMgmtInputErr and
// leaves the sub-mode open for a retry, same contract as keyTagInput. A
// valid Create submit dispatches saveTagDefsCmd with the registry's current
// defined names (extracted from tagMgmtRows, D11: Create is a pure Registry
// act, never touches m.idx) plus the new one (data.AddTagDefName) -- the
// sub-mode itself does NOT close here; applyTagDefsSaved (update.go) closes
// it only on a CONFIRMED successful write, so a failed disk write leaves the
// PO able to retry the same typed name without retyping it.
func (m model) keyTagMgmtInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.tagMgmtInputActive = false
		m.tagMgmtInput.Blur()
		m.tagMgmtInputErr = ""
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.tagMgmtInput.Value())
		if !data.ValidTagName(name) {
			m.tagMgmtInputErr = "invalid tag name (a-z0-9, hyphen-separated, lowercase)"
			return m, nil
		}
		for _, r := range m.tagMgmtRows {
			if r.name == name && name != m.tagMgmtInputTarget {
				// Neutral wording (T5-F01, bean bt-sohl): the dedupe set spans
				// defined AND free rows alike -- the former "tag already
				// defined" text was factually wrong when the collision hit a
				// free/undefined row. The rejection itself is unchanged
				// (rename-onto-free = merge semantics, PO question Q04 in
				// epic bt-362n).
				m.tagMgmtInputErr = "name already in use: " + name
				return m, nil
			}
		}
		m.tagMgmtInputErr = ""

		switch m.tagMgmtInputMode {
		case "create":
			defs := definedTagNames(m.tagMgmtRows)
			// refindName = the new name, explicitly (B01, Fix-Runde 1, bean
			// bt-1lsu): applyTagDefsSaved no longer reads the input's Value()
			// implicitly -- every dispatch site names its own cursor target.
			return m, saveTagDefsCmd(m.store, data.AddTagDefName(defs, name), name, "")
		case "rename":
			// E10 Task 5 (bean bt-y9my, D13): Registry-Rename and Bean-Sweep
			// dispatch as TWO INDEPENDENT Cmds in ONE tea.Batch -- neither
			// waits on the other (tagRenameDoneMsg's own doc-stamp,
			// messages.go, explains why that's harmless: disjoint state
			// writes). refindName = the NEW name (mirrors "create"'s own
			// convention above) -- applyTagDefsSaved's cursor re-find works
			// identically for Rename, it is mode-agnostic (T3's "Notes for
			// T5", bean bt-604w).
			old := m.tagMgmtInputTarget
			defs := definedTagNames(m.tagMgmtRows)
			return m, tea.Batch(
				saveTagDefsCmd(m.store, data.RenameTagDefName(defs, old, name), name, ""),
				renameTagCmd(m.store, m.idx, old, name),
			)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.tagMgmtInput, cmd = m.tagMgmtInput.Update(msg)
	return m, cmd
}

// definedTagNames extracts the currently-registry-defined subset of rows
// (defined==true) as a plain name slice -- the input to
// data.AddTagDefName/RemoveTagDefName/RenameTagDefName (T1's own pure
// helpers, internal/data/tagdefs.go), since m.tagMgmtRows itself never
// carried a separate "raw defs slice" field (T2's own "one Union row list,
// no manual patching" convention, bean bt-r92i's Notes-für-T3+T4).
func definedTagNames(rows []tagRegistryRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.defined {
			out = append(out, r.name)
		}
	}
	return out
}

// tagMgmtInputRows renders the input sub-mode's own content -- mirrors
// tagInputBox's hint+field+error layout (box_picker_tag.go) but INLINE,
// inside the SAME pane the row list normally occupies (D06/D14: no floating
// modalPanel/overlay for this page).
func (m model) tagMgmtInputRows() []string {
	hint := "enter:create  esc:cancel"
	if m.tagMgmtInputMode == "rename" {
		hint = "enter:rename  esc:cancel"
	}
	rows := []string{theme.Muted.Render(hint), "", m.tagMgmtInput.View()}
	if m.tagMgmtInputErr != "" {
		rows = append(rows, "", lipgloss.NewStyle().Foreground(theme.Red).Render(m.tagMgmtInputErr))
	}
	return rows
}

// openTagMgmtDeleteConfirm opens the page-local Delete-Confirm (E10 Task 4,
// bean bt-1lsu, D12/D15): reads the CURRENTLY SELECTED row
// (m.tagMgmtRows[m.tagMgmtCursor.cursor]) -- a free (undefined) row has no
// Registry Definition to remove, so this is a silent, HANDLED no-op
// (mirrors the focusedBean()==nil no-op convention quer durchs Repo, same
// guard shape for an out-of-range cursor on an empty/degenerate row list).
// A defined row sets tagMgmtDeleteConfirm=true and tagMgmtDeleteTarget to
// its name -- no Cmd fires here, the actual SaveTagDefs write only happens
// on a CONFIRMED enter (keyTagMgmtDeleteConfirm below).
func (m model) openTagMgmtDeleteConfirm() (tea.Model, tea.Cmd) {
	if m.tagMgmtCursor.cursor < 0 || m.tagMgmtCursor.cursor >= len(m.tagMgmtRows) {
		return m, nil
	}
	row := m.tagMgmtRows[m.tagMgmtCursor.cursor]
	if !row.defined {
		// bt-ct3k (E11 Item 5): mirrors openTagMgmtRename's own toast fix one
		// function up -- a free row has no Registry Definition to delete,
		// surfaced now instead of a silent no-op.
		return m.showToast(toastWarn, "Unregistered tag — modification not possible", "n to define first", nil, false)
	}
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = row.name
	return m, nil
}

// keyTagMgmtDeleteConfirm drives the open Delete-Confirm (D12/D15): enter
// dispatches saveTagDefsCmd (T3's own Save infra, reused verbatim -- no
// second save path) with the target removed from the registry's CURRENT
// defined names (data.RemoveTagDefName, T1); esc/n cancels WITHOUT saving
// (mirrors keyDeleteConfirm's own esc/n-cancel dual, box_confirm_delete.go).
// REGISTRY-ONLY (D12): this function never calls data.Store.SetTags or
// touches m.idx in any way -- only SaveTagDefs, exactly like Create (T3).
// Every OTHER key is a silent, HANDLED no-op (Full-Capture-Disziplin, the
// SAME "no fallthrough to any outer handler" contract keyTagMgmtInput
// already established one layer up).
func (m model) keyTagMgmtDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keybind.Matches(msg, keys.Enter):
		target := m.tagMgmtDeleteTarget
		m.tagMgmtDeleteConfirm = false
		m.tagMgmtDeleteTarget = ""
		defs := definedTagNames(m.tagMgmtRows)
		// refindName = the deleted target, explicitly (B01, Fix-Runde 1,
		// bean bt-1lsu): a still-USED tag falls into the Free group and the
		// cursor follows it there; an unused tag vanishes entirely, the
		// name-search misses, and the cursor stays put -- and stale text in
		// the (esc-aborted, never-cleared) Create input can no longer
		// redirect the cursor, since applyTagDefsSaved no longer reads it.
		return m, saveTagDefsCmd(m.store, data.RemoveTagDefName(defs, target), target, "")
	case keybind.Matches(msg, keys.Back), msg.String() == "n":
		m.tagMgmtDeleteConfirm = false
		m.tagMgmtDeleteTarget = ""
		return m, nil
	}
	return m, nil
}

// tagMgmtDeleteConfirmBox renders the floating Delete-Confirm modal (D12/
// D15) -- mirrors deleteBox's own Aufbau (box_confirm_delete.go: singular/
// plural count discipline, Red border for a "Delete"-named action) but via
// modalPanel (the bean body's own explicit choice, quitBox's own precedent,
// box_confirm_quit.go) rather than a hand-built modalBox call. count is
// resolved LIVE from m.tagMgmtRows at RENDER time (mirrors deleteBox's own
// "typ resolves from the LIVE index at render time... since it is only
// needed for display, not for dispatch" doc-stamp) -- not a separate
// captured-at-open-time field (the bean body deliberately lists ONLY
// tagMgmtDeleteConfirm/tagMgmtDeleteTarget as new model fields, no third
// count field). Count 0 gets the bean's own explicit shorter, non-
// contradictory text ("Not currently used by any bean") instead of "Still
// used by 0 bean(s)".
func (m model) tagMgmtDeleteConfirmBox() string {
	count := 0
	for _, r := range m.tagMgmtRows {
		if r.name == m.tagMgmtDeleteTarget {
			count = r.count
			break
		}
	}

	var body string
	switch count {
	case 0:
		body = "Not currently used by any item.\n"
	case 1:
		body = "Still used by 1 item — it keeps the tag, it just won't be prioritized anymore.\n"
	default:
		body = fmt.Sprintf("Still used by %d items — they keep the tag, it just won't be prioritized anymore.\n", count)
	}

	title := fmt.Sprintf("Delete tag definition '%s'?", m.tagMgmtDeleteTarget)
	footer := "enter: delete   esc/n: cancel"
	return modalPanel(title, body, footer, clampModalWidth(48, m.width), theme.Red)
}
