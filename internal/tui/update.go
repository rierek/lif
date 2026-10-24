package tui

// update.go — the Update dispatcher (Elm architecture): routes tea.Msg to
// state transitions only. No rendering here (view_browse_repo.go), no
// Cmd-only definitions here (messages.go) -- port convention from devd
// update.go.

import (
	"errors"
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"os"
	"strings"
	"time"

	"github.com/rierek/lif/internal/tui/clip"
	"github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/data"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// doubleClickInterval is the window within which two clicks on the SAME
// Tree node count as a Doppelklick (E5 Task 4, bean bt-mne6, design decision
// f -- Port devd doubleClickInterval verbatim, update.go). bubbletea
// delivers no click-count, so mouse.go's mouseTreeClick detects it itself
// via now()/m.lastClickAt/m.lastClickIdx.
const doubleClickInterval = 500 * time.Millisecond

// now returns the current time via the (test-injectable) clock -- nil falls
// back to time.Now() (Port devd's own now() verbatim). Testable via
// m.clock's injection (mouse_test.go) for a deterministic double-click
// window.
func (m model) now() time.Time {
	if m.clock != nil {
		return m.clock()
	}
	return time.Now()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case beansLoadedMsg:
		return m.applyLoaded(msg)

	case storeEventsMsg:
		// The store's watcher reported a change batch -- always a full
		// reload, never partial (design decision D02), then wait for the
		// next batch.
		return m, tea.Batch(loadCmd(m.store), waitForStoreEvents(m.storeEvents))

	case searchBleveResultMsg:
		return m.applyBleveResult(msg)

	case taskDoneMsg:
		return m.applyTaskDone(msg)

	case mutationDoneMsg:
		// E3 (bean bt-dlgk): every Set*/Add*/Remove*/Delete mutation goes
		// through this ONE case -- applyMutationResult below is the shared
		// status-line + unconditional-reload tail (design decision d).
		return m.applyMutationResult(msg.err, msg.warn)

	case createDoneMsg:
		// E3 Task 4 (bean bt-y4ly): the ONE exception to mutationDoneMsg --
		// a success additionally jumps the cursor (applyCreateDone below), a
		// failure routes through the same applyMutationResult tail as every
		// other mutation.
		return m.applyCreateDone(msg)

	case tagDefsSavedMsg:
		// E10 Task 3 (bean bt-604w, D11/D14): a Tag-Registry SaveTagDefs
		// write's outcome -- applyTagDefsSaved below is its OWN tail, NOT
		// applyMutationResult (no Bean was touched, so no unconditional
		// loadCmd reload is needed, messages.go's own tagDefsSavedMsg
		// doc-stamp).
		return m.applyTagDefsSaved(msg)

	case tagRenameDoneMsg:
		// E10 Task 5 (bean bt-y9my, D13): renameTagCmd's own completion
		// tail -- applyTagRenameDone below is the SECOND deliberate
		// exception to mutationDoneMsg (tagDefsSavedMsg above was the
		// first). UNLIKE tagDefsSavedMsg (a pure Registry write, no Bean
		// ever touched), this Msg's whole point IS a real Bean-tag
		// mutation across N beans, so it always reloads (mirrors
		// applyMutationResult's own unconditional-reload convention).
		return m.applyTagRenameDone(msg)

	case beanRawLoadedMsg:
		// D01 (design-spec.md §15 PF-17, bean bt-z4b1): openBeanEditor's
		// FIRST Cmd-hop result (the async ShowRaw read) -- fires the ACTUAL
		// $EDITOR suspend on success, or surfaces a toast + resets the
		// editor-open state on failure (applyBeanRawLoaded below).
		return m.applyBeanRawLoaded(msg)

	case editorFinishedMsg:
		// D01 (design-spec.md §15 PF-17, bean bt-z4b1, supersedes E3 Task 5/
		// E8 B10): the e/ctrl+e whole-bean $EDITOR-Suspend's result -- err ->
		// status line only; unchanged -> silent no-op (no CLI call for a
		// no-op edit); otherwise parsed + diffed against m.editorSnapshot and
		// dispatched as ONE combined UpdateWhole call (applyEditorFinished
		// below).
		return m.applyEditorFinished(msg)

	case toastExpiredMsg:
		// E5 Task 1 (bean bt-6dts): the corner Toast's auto-dismiss tick
		// (toastTimeout, messages.go) -- handleToastExpired below only
		// clears m.toast if seq still matches the CURRENT generation (a
		// newer showToast call may have already replaced it).
		return m.handleToastExpired(msg)

	case repoSwitchedMsg:
		// E5 Task 6 (bean bt-zhwl): switchRepoCmd's outcome (Lobby enter,
		// keyLobby/view_lobby.go) -- applyRepoSwitched below is the shared
		// success/failure tail.
		return m.applyRepoSwitched(msg)

	case repoMetricsMsg:
		// E5 Task 6 (bean bt-zhwl): ONE repo's async Open/Total figure
		// (repoMetricsBatchCmd, messages.go) -- applyRepoMetrics below
		// updates just that one map entry.
		return m.applyRepoMetrics(msg)

	case tea.MouseMsg:
		// E5 Task 4 (Maus, bean bt-mne6): HERE, ahead of tea.KeyMsg below --
		// handleMouse's (mouse.go) very first check is the Toast hit-test
		// (Port devd update.go:466, design decision a "Klick-Dismiss vs.
		// Maus"), independent of any overlay/form guard, so a Toast click
		// must dismiss it even while a form/overlay is open (Cross-Feature-
		// Fix parity, devd DD2-272/273). This switch-case placement (inside
		// Update()'s own top-level switch, BEFORE the post-switch `if
		// m.form != nil { return m.updateForm(msg) }` fallback below) is
		// itself the whole fix -- unlike devd, which needs an explicit `if
		// m.form != nil` guard AHEAD of its own switch (its Update()
		// otherwise routes everything to updateForm first), beans-tui's
		// switch already runs unconditionally on every Msg BEFORE that
		// fallback ever triggers, so a matched case here is automatically
		// safe -- verified as its own regression test,
		// TestToastClickDismissesEvenWithFormOpen (mouse_test.go), not just
		// documented.
		return m.handleMouse(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	// E3 Task 4 (bean bt-y4ly): huh-internal Msgs (cursor blink, viewport
	// ticks, its own nextField/nextGroup chain) that matched none of the
	// cases above -- the form's own Init()/Update() Cmds keep firing these
	// for as long as it's open. Key-Msgs never reach here: tea.KeyMsg has
	// its own case above, routed through handleKey -> keyForm when
	// m.form != nil (capture order doc-stamp, handleKey below).
	if m.form != nil {
		return m.updateForm(msg)
	}
	return m, nil
}

// handleToastExpired applies a toastTimeout tick (E5 Task 1, bean bt-6dts,
// Port devd overlay_show_toast.go's own inline Update-case): clears m.toast
// ONLY when seq still matches the CURRENT generation -- a newer showToast
// call (design decision a's debounce/replace semantics) may have already
// bumped m.toast.seq past this stale tick's generation, in which case the
// tick is a no-op (the newer toast's OWN tick, or its sticky=true, is what
// governs dismissal from here on).
func (m model) handleToastExpired(msg toastExpiredMsg) (tea.Model, tea.Cmd) {
	if m.toast != nil && msg.seq == m.toast.seq {
		m.toast = nil
	}
	return m, nil
}

// applyRepoSwitched is switchRepoCmd's (messages.go) shared success/failure
// tail (E5 Task 6, bean bt-zhwl). Failure: the CURRENT session (m.store/
// m.idx/m.watchStop) is left COMPLETELY untouched -- switchRepoCmd already
// validated the target repo BEFORE ever touching the old watcher (design
// note, messages.go), so there is nothing stale to clean up here, only a
// status-line/Toast note to surface (the "Fehlerpfad darf die laufende
// Session NICHT zerstören" constraint, bean bt-zhwl).
//
// Success: rebuilds the Index directly from msg.beans (no separate loadCmd
// round-trip needed -- switchRepoCmd already ran client.List() once as part
// of its own validation, re-running it here would be a redundant second
// subprocess call for the exact same data) and resets EVERY navigation/
// search/filter/detail-focus field a stale value from the OLD repo could
// otherwise leak through as (Port devd selectProject's own full reset
// breadth, view_home.go -- deliberately wider than the bean's own minimal
// "expanded/cursorID/detailFocus" acceptance-checklist sketch: a stale
// searchQuery/facet filter from the old repo could silently show an EMPTY
// tree for the new one with no visible reason, which is a genuine
// usability bug, not just cosmetic drift -- same class of finding as this
// file's own m.searchActive capture-order precedent).
func (m model) applyRepoSwitched(msg repoSwitchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, "Repo switch failed: "+msg.err.Error(), "", nil, false)
		return m, toastCmd
	}

	m.store = msg.store
	m.repoDir = msg.repoDir
	m.storeEvents = msg.events
	m.watchUnavailable = msg.events == nil // I04 precedent: degrade visibly, never silently
	applyRepoConfig(msg.store.Config())    // the new repo's value lists, BEFORE its index is sorted
	m = m.setItems(msg.beans, msg.ready)
	m.view = viewBrowseRepo
	m.err = ""

	// T6-Review Prelude I01 (T6b, bean bt-pd22): a Toast from the OLD repo
	// (e.g. a sticky ErrConflict warning, applyMutationResult) must not
	// survive a repo switch -- unlike applyLoaded's clearToastUnlessSticky
	// (which protects a sticky toast across a SAME-repo background reload),
	// a repo switch is a full session discontinuity, so this clears
	// UNCONDITIONALLY, sticky or not (Port devd selectProject's own full
	// reset breadth, same rationale as every other field reset below). nil
	// is the whole toastState -- there is no separate generation/sticky
	// field elsewhere on model to also reset (toastState's own seq/sticky
	// live INSIDE the struct this pointer addresses, overlay_show_toast.go).
	m.toast = nil
	m = m.setTaskActions() // the new repo's tui.actions; may raise a warning toast

	// Port devd selectProject's reset-on-switch breadth (view_home.go).
	m.expanded = map[string]bool{}
	m.cursorID = ""
	m.detailFocus = m.layout == layoutDetail // layoutDetail always holds Detail focus
	m.boxFormCursor, m.boxFormCursorBean = 0, ""
	m.searchActive = false
	m.searchInput.SetValue("")
	m.searchQuery = ""
	m.searchPrefixFacets = nil // bt-2kfl: a stale typed prefix from the OLD repo must not leak through either
	m.searchPrefixRest = ""
	m.searchBleveIDs = nil
	m.searchBleveFor = ""
	m.searchBleveLoading = false
	m.filterFocus = false
	m = m.clearFacets()
	m.showArchived = false // E5 Task 7 (bean bt-ggt2, T6-Note bug class, bt-zhwl "Notes for T7"): the archive-default toggle must not leak across a repo switch, same reset breadth as every other search/filter field here.
	m.readyList = listState{}
	m.flatList = listState{} // S5: same stale-cursor hygiene as readyList just above -- flatView itself (a display-mode preference, not per-repo state) deliberately survives the switch

	_ = config.SetLastRepo(msg.repoDir) // best-effort persistence (Port devd DD2-273 RMW pattern, state.go) -- a failed write must not block the switch itself
	return m, waitForStoreEvents(m.storeEvents)
}

// applyRepoMetrics applies ONE repoMetricsMsg (E5 Task 6, bean bt-zhwl) --
// I01 copy-on-write: clones m.repoMetrics before adding the new entry, same
// convention as every other model map field (types.go's own I01 doc-stamp).
func (m model) applyRepoMetrics(msg repoMetricsMsg) (tea.Model, tea.Cmd) {
	metrics := make(map[string]repoMetric, len(m.repoMetrics)+1)
	for k, v := range m.repoMetrics {
		metrics[k] = v
	}
	metrics[msg.repo] = repoMetric{open: msg.open, total: msg.total, err: msg.err, loaded: true}
	m.repoMetrics = metrics
	return m, nil
}

// applyMutationResult is the shared tail every E3 mutation's mutationDoneMsg
// runs through (design decision d, bean bt-dlgk): ALWAYS reloads
// (loadCmd), success or failure alike -- a success must show the new state,
// a failure (including a genuine ETag conflict) must resolve the now-stale
// index rather than leave the UI showing a value that silently failed to
// apply. errors.Is(err, data.ErrConflict) gets its own status-line wording;
// Toast is E5 scope, m.err/the status line's Red slot (view.go statusBar)
// is the interim feedback channel.
func (m model) applyMutationResult(err error, warn string) (tea.Model, tea.Cmd) {
	m.err = ""
	var toastCmd tea.Cmd
	if err != nil {
		if errors.Is(err, data.ErrConflict) {
			m.err = "Conflict: item changed externally — reloaded"
			// F2 (Review-Runde 2) nicety: applyEditorFinished's mutateCmd
			// closure wraps a genuine conflict in *conflictWithRecovery when
			// it managed to persist the PO's just-edited body to a kept
			// tempfile first (otherwise this reload would silently discard
			// it) -- surface that path alongside the generic conflict text
			// so the PO can recover it manually. Every OTHER mutation site's
			// plain ErrConflict-wrapped error simply doesn't match here
			// (errors.As returns false) -- bt-0xrb (D04, PO Grilling
			// 2026-07-17): those plain-conflict sites (e.g. the t-Picker
			// Tag-Konflikt repro, box_picker_tag.go) used to get toastCtx=""
			// here, dropping the bean-ID + beans-CLI detail err already
			// carries (internal/data/mutations.go:63/75, classifyError) --
			// toastCtx now pre-fills from err.Error() so that detail survives
			// into the Toast; the errors.As match below still OVERRIDES it
			// with "Version saved: …" for the Editor-recovery path,
			// UNCHANGED (D04: recovery-path wording stays exactly as-is).
			toastCtx := err.Error()
			var cr *conflictWithRecovery
			if errors.As(err, &cr) {
				m.err += " — your version: " + cr.path
				toastCtx = "Version saved: " + cr.path
			}
			// E5 Task 1 (bean bt-6dts, E3-I01 PFLICHT): sticky=true is the
			// ONLY sticky Toast in the whole app -- a genuine ETag conflict
			// (+ its recovery-tempfile path, when present) must stay
			// readable until the PO explicitly dismisses it (click) or a
			// newer toast replaces it, surviving every reload cycle in
			// between (applyLoaded never touches m.toast, so there is
			// nothing here that would clobber it).
			m, toastCmd = m.showToast(toastError, "Conflict: item changed externally", toastCtx, nil, true)
		} else {
			m.err = err.Error()
			// F01 (bt-z4b1 Review-Runde 1): applyEditorFinished's mutateCmd
			// closure now wraps EVERY UpdateWhole failure (not just genuine
			// conflicts) in *conflictWithRecovery -- a CLI-side
			// VALIDATION_ERROR from a whole-bean $EDITOR submit lands HERE
			// (it is not ErrConflict), and its recovery-tempfile path must
			// surface exactly like the conflict branch above does, or the
			// PO's whole $EDITOR session is silently lost to this
			// function's unconditional reload. Every OTHER mutation site's
			// plain error simply doesn't match (errors.As returns false),
			// leaving their existing behavior untouched. Toast title stays
			// the bare error (short); the path rides in ctx, mirroring the
			// conflict branch's own title/ctx split.
			toastCtx := ""
			var cr *conflictWithRecovery
			if errors.As(err, &cr) {
				m.err += " — your version: " + cr.path
				toastCtx = "Version saved: " + cr.path
			}
			// A gate block carries the hook name and its output: keep it
			// readable until dismissed, like a conflict.
			m, toastCmd = m.showToast(toastError, err.Error(), toastCtx, nil, strings.Contains(err.Error(), `hook "`))
		}
	} else {
		m, toastCmd = m.showWarn(warn)
	}
	return m, tea.Batch(toastCmd, loadCmd(m.store))
}

// showWarn shows the core warnings and hook output of a load or mutation as a
// sticky warning toast; empty warn shows nothing. Stdout/stderr of the core
// never reach the terminal (the alt screen would break), so this is the one
// place they surface.
func (m model) showWarn(warn string) (model, tea.Cmd) {
	if warn == "" {
		return m, nil
	}
	return m.showToast(toastWarn, "Warning", warn, nil, true)
}

// conflictWithRecovery wraps a mutation error with the path of a KEPT
// tempfile holding content the mutation was about to silently lose to
// applyMutationResult's unconditional reload (F2, Review-Runde 2 -- widened
// by F01, bt-z4b1 Review-Runde 1, from ErrConflict-only to EVERY whole-bean
// $EDITOR UpdateWhole failure, CLI VALIDATION_ERROR included: the name kept
// its historical "conflict" prefix, the mechanism is generic recovery).
// Unwrap preserves errors.Is(_, data.ErrConflict) for genuinely
// conflict-classified inner errors, so applyMutationResult's conflict branch
// still fires unmodified for those; BOTH of its branches errors.As-recover
// path to append to the status-line text.
type conflictWithRecovery struct {
	err  error
	path string
}

func (c *conflictWithRecovery) Error() string { return c.err.Error() }
func (c *conflictWithRecovery) Unwrap() error { return c.err }

// writeConflictTempFile persists body to a NEW, deliberately KEPT tempfile
// (unlike readEditorResult's own scratch file, editor.go, which os.Remove's
// itself once read) -- a PO whose whole-bean $EDITOR submit just got
// rejected (ETag conflict, CLI VALIDATION_ERROR -- F01, bt-z4b1
// Review-Runde 1 -- or a parse failure before any CLI call) can recover the
// edited content manually instead of losing the whole $EDITOR session to
// applyMutationResult's unconditional reload. Best-effort: any failure here
// must not mask the underlying error, so the caller (applyEditorFinished)
// falls back to the bare unwrapped error.
func writeConflictTempFile(body string) (path string, err error) {
	f, err := os.CreateTemp("", "lif-tui-conflict-*.md")
	if err != nil {
		return "", err
	}
	path = f.Name()
	if _, err = f.WriteString(body); err != nil {
		f.Close()
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return path, nil
}

// applyCreateDone handles a completed Create (E3 Task 4, bean bt-y4ly): a
// failure routes through the shared applyMutationResult tail exactly like
// every other mutation (status line + reload). A success additionally jumps
// the cursor onto the freshly minted bean and expands its parent's ancestor
// path BEFORE the reload fires: m.idx here is still the OLD index (the new
// bean does not exist in it yet), so expandAncestorsOf must walk UP FROM
// msg.bean.Parent (an EXISTING bean) rather than from msg.bean.ID itself (a
// lookup that would silently no-op against the old index) -- and since
// expandAncestorsOf only marks an id's ANCESTORS, not the id itself, the
// immediate parent needs its own explicit exp[...]=true so the new bean
// (its child) is revealed once flattenTree runs against it. applyLoaded's
// existing "exact bean still present" cursor-restore path (above) then finds
// msg.bean.ID unchanged after the beansLoadedMsg arrives (US-10-parity, no
// new cursor logic). A parentless new bean (msg.bean.Parent == "") needs no
// expansion at all -- a parentless bean sits at the roadmap root, or under a synthetic root.
//
// F1 (Review-Runde 2, Async-Gap-Clobbering, Finding 1): between the Confirm-
// Gate's enter (keyCreateConfirm, box_confirm_create.go: overlay -> None,
// form -> nil) and THIS createDoneMsg arriving, the PO can open something
// else entirely (a new form, a picker/menu overlay) during that async gap --
// pendingCreate's in-flight guard (types.go doc-stamp) stops a SECOND `c`
// from starting a second create, but does nothing to stop the PO from
// opening a DIFFERENT overlay/form in the meantime. Reopening the stale
// Create-Form here unconditionally (the original T4/T5 behavior) would
// clobber whatever the PO is now doing and/or cross-contaminate the single
// createDraft slot with an unrelated form's state. The busy guard below
// treats that as the PO having actively abandoned the failed create by
// starting something new: drop the stale draft, route the error through the
// draft-agnostic tail instead (status line + reload), and leave the
// currently open form/overlay untouched. pendingCreate always clears here --
// the in-flight window is over either way, success or failure.
func (m model) applyCreateDone(msg createDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.pendingCreate = nil // in-flight window over (F1), regardless of outcome
		busy := m.form != nil || m.overlay != overlayNone
		if m.createDraft != nil && !busy {
			// B01 (E3-T4-Review PFLICHT, closed in T5, bean bt-sl45): a
			// CLI-rejected create (e.g. VALIDATION_ERROR) must not lose the
			// PO's filled-in draft -- reopen the Create-Form FILLED from it
			// instead of routing through the draft-agnostic
			// applyMutationResult tail (which would just show the error and
			// reload, discarding the work). Only reachable when the PO is
			// NOT busy with something else (F1 above).
			d := *m.createDraft
			m.createDraft = nil
			m.err = msg.err.Error()
			// B01 (E5-T1-Review Prelude, PFLICHT): this reopen-branch was the
			// one showToast site E5 Task 1's dual-write audit missed (it
			// returns through openCreateFormWithDraft rather than
			// applyMutationResult's shared tail, so the grep-driven audit of
			// that tail's call sites never saw it) -- same convention as
			// every other hard-error site (toastError, non-sticky, title
			// mirrors m.err).
			var toastCmd tea.Cmd
			m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
			formModel, formCmd := m.openCreateFormWithDraft(d)
			return formModel, tea.Batch(toastCmd, formCmd)
		}
		m.createDraft = nil // busy (F1) or no draft to reopen from -- drop it either way
		return m.applyMutationResult(msg.err, msg.warn)
	}
	m.createDraft = nil   // success: draft consumed, no reopen needed (B01's other half)
	m.pendingCreate = nil // in-flight window over (F1)
	if msg.bean.Parent != "" {
		m.expanded = expandAncestorsOf(m.treeParent, m.expanded, msg.bean.Parent)
		exp := cloneBoolMap(m.expanded) // I01: copy-on-write
		exp[msg.bean.Parent] = true     // expandAncestorsOf expands ancestors OF id, not id itself
		m.expanded = exp
	}
	m.cursorID = msg.bean.ID
	m.err = ""
	return m, loadCmd(m.store)
}

// applyTagDefsSaved handles a completed Tag-Registry write (E10 Task 3, bean
// bt-604w, D11/D14 Create -- also T4/T5's shared Delete/Rename tail):
// mirrors applyMutationResult's own error branch (non-conflict half -- a
// Tag-Registry write can never produce data.ErrConflict, it carries no ETag)
// WITHOUT that function's unconditional loadCmd reload, since a Registry
// write never touches any Bean (D11/D12) and therefore never stales m.idx --
// "hier gibt es kein m.idx zu invalidieren, nur die Registry" (bean bt-604w
// wording). On failure the input sub-mode stays OPEN (tagMgmtInputActive
// untouched) so the PO can retry the same typed name without retyping it,
// alongside a Toast surfacing the error.
//
// On success: the input sub-mode closes, and tagMgmtRows is rebuilt from a
// FRESH LoadTagDefs (D02/D03, mirrors openTagManagementPage's own
// "reload from disk on every open" convention one layer up) rather than from
// whatever defs slice keyTagMgmtInput happened to compute at dispatch time --
// the on-disk file is the single source of truth this function trusts, same
// D02 tolerant-missing philosophy as everywhere else this registry is read.
// The cursor then lands on msg.refindName's row by NAME (mirrors
// applyLoaded's own cursor-refinding-by-ID pattern one layer up) -- the name
// is passed EXPLICITLY by every dispatch site (Create: the new name; Delete:
// the deleted target, T4 Fix-Runde 1 B01, bean bt-1lsu), NEVER read
// implicitly from m.tagMgmtInput.Value() here: T3's esc-abort deliberately
// leaves the typed text in the input, so an implicit read let an aborted
// Create redirect the cursor after a completely unrelated Delete
// (reviewer-verified repro, tagDefsSavedMsg's own doc-stamp, messages.go).
func (m model) applyTagDefsSaved(msg tagDefsSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, msg.err.Error(), "", nil, false)
		return m, toastCmd
	}

	m.tagMgmtInputActive = false
	m.tagMgmtInput.Blur()
	m.tagMgmtInput.SetValue("")
	m.tagMgmtInputErr = ""
	m.tagMgmtInputTarget = ""

	var defs []string
	if m.store != nil {
		defs, _ = m.store.LoadTagDefs() // D02: LoadTagDefs never returns a non-nil error
	}
	m.tagMgmtRows = tagRegistryRows(m.idx, defs)
	m.tagMgmtCursor.setLen(len(m.tagMgmtRows))
	for i, r := range m.tagMgmtRows {
		if r.name == msg.refindName {
			m.tagMgmtCursor.cursor = i
			break
		}
	}
	// E11 Item 6 (bean bt-idm1): Adopt is the ONE dispatch site that sets
	// successToast (messages.go doc-stamp on the field) -- Create/Rename/
	// Delete all leave it "" and stay silent on success, unchanged.
	if msg.successToast != "" {
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastInfo, msg.successToast, "", nil, false)
		return m, toastCmd
	}
	return m, nil
}

// applyTagRenameDone is renameTagCmd's own completion tail (E10 Task 5, bean
// bt-y9my, D13): ALWAYS reloads (loadCmd, mirrors applyMutationResult's own
// unconditional-reload convention) so m.idx reflects the swept beans' new
// tag before the Page next rebuilds tagMgmtRows from it -- UNLIKE
// applyTagDefsSaved above, which deliberately never reloads (no Bean is ever
// touched by a pure Registry write). A non-empty failed slice degrades the
// Toast to toastError (continue-on-error, D13: SOME beans may have missed
// the rename), otherwise a plain success Toast -- toastInfo is the
// "Hinweis/Erfolg" kind (overlay_show_toast.go's own doc-stamp), the SAME
// kind e.g. the Yank "Copied: "+b.ID toast above already uses for a bare
// success.
func (m model) applyTagRenameDone(msg tagRenameDoneMsg) (tea.Model, tea.Cmd) {
	title := fmt.Sprintf("Renamed %s → %s: %d item(s) updated", msg.oldTag, msg.newTag, msg.renamed)
	kind := toastInfo
	if len(msg.failed) > 0 {
		title += fmt.Sprintf(", %d failed (first: %v)", len(msg.failed), msg.failed[0].err)
		kind = toastError
	}
	var toastCmd tea.Cmd
	m, toastCmd = m.showToast(kind, title, "", nil, false)
	return m, tea.Batch(toastCmd, loadCmd(m.store))
}

// applyBeanRawLoaded handles openBeanEditor's FIRST Cmd-hop result (D01,
// design-spec.md §15 PF-17, bean bt-z4b1): a ShowRaw failure surfaces a
// toast and resets the FULL editor-open state (target/etag/snapshot,
// mirrors applyEditorFinished's own err path below) -- a success fires the
// ACTUAL tea.ExecProcess suspend (editInEditor, editor.go), the SECOND
// Cmd-hop the design-spec calls for ("zwei Cmd-Hops statt einem", since a
// subprocess read must never run synchronously inside Update). A msg.id
// mismatch against m.editorTarget (defensive -- should be unreachable since
// nothing else can touch editorTarget between openBeanEditor and this Msg
// resolving) discards the stale load silently rather than suspending into
// the WRONG bean's editor.
func (m model) applyBeanRawLoaded(msg beanRawLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.editorTarget {
		return m, nil // stale -- editorTarget moved on since this Cmd was dispatched
	}
	if msg.err != nil {
		m.err = msg.err.Error()
		m.editorTarget = ""
		m.editorETag = ""
		m.editorSnapshot = nil
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
		return m, toastCmd
	}
	return m, editInEditor(msg.raw, ".md")
}

// applyEditorFinished handles the e/ctrl+e whole-bean $EDITOR-Suspend's
// result (D01, design-spec.md §15 PF-17, bean bt-z4b1, supersedes E3 Task 5/
// E8 B10's Body-only SetBody dispatch): a process-level err surfaces in the
// status line with no mutation; unchanged content is a silent no-op (no CLI
// call for a no-op edit, mirrors the plan's own Akzeptanz wording);
// otherwise the content is PARSED (parseRawBean, editor.go), DIFFED against
// the snapshot frozen at $EDITOR-open time (buildWholeEditDiff), and
// dispatched as ONE combined data.Store.UpdateWhole call against the etag
// CAPTURED AT $EDITOR-OPEN TIME (m.editorETag, F2 Review-Runde 2 fix) --
// NOT a fresh m.beanETag(id) read here. Design decision d's "always read
// fresh at submit" is correct for every OTHER E3 overlay, whose
// open<->submit window is a single keystroke, but wrong for a potentially
// long-lived $EDITOR session: re-reading fresh at submit would let a
// watch-reload that landed WHILE the PO was still typing in $EDITOR
// silently win, discarding their edit with no conflict ever raised (a
// genuine lost update). Freezing the etag (AND the full snapshot, D01's
// extension of F2's original etag-only freeze) at open turns that same
// race into a genuine, surfaced ErrConflict instead -- correct
// optimistic-lock semantics over the WHOLE session.
//
// Two distinct failure modes, both RECOVERABLE (design-spec §15 PF-17
// "Fehlerfall" -- the whole-bean editor is bewusst UNconstrained Freitext-
// YAML, unlike every constrained Overlay, so a rejection must never lose
// the PO's edits):
//   - parseRawBean fails (malformed frontmatter, invalid YAML): the FULL
//     raw text is persisted to a kept recovery tempfile immediately, no CLI
//     call ever fires (there is nothing valid to diff).
//   - UpdateWhole fails -- ANY failure (F01, bt-z4b1 Review-Runde 1: a
//     genuine ErrConflict from a parallel external write AND equally a CLI
//     VALIDATION_ERROR from a hand-typed invalid field value; the original
//     conflict-only wrap lost the whole session on the latter, live
//     reproduced by the reviewer): the FULL raw text is persisted the SAME
//     way, wrapped as *conflictWithRecovery so its path rides along in the
//     surfaced status-line text (applyMutationResult above, BOTH branches)
//     -- otherwise applyMutationResult's unconditional reload would
//     silently discard the PO's just-edited content.
//
// m.beanETag(id) is still consulted for the vanished-target guard below,
// but ONLY for its ok bool (bean presence) -- the etag VALUE it would
// return is discarded in favor of the captured one, same guard shape as
// applyValueMenuSelection.
//
// Bekannte Grenze (Dokumentationspflicht, kein Bug, design-spec.md §15
// PF-17): created_at/updated_at/the "# <id>" header line are all VISIBLE in
// the $EDITOR text (part of ShowRaw's own seed) but NOT editable -- they
// are deliberately not even parsed into rawBeanFrontmatter (that type's own
// doc-stamp, editor.go), since `beans update` has no flag for any of them.
// A PO edit to one of these is silently dropped when the WholeEditDiff is
// built, never surfaced as a rejected change -- a CLI-side feature gap, not
// an implementation task for this bean.
func (m model) applyEditorFinished(msg editorFinishedMsg) (tea.Model, tea.Cmd) {
	id := m.editorTarget
	etag := m.editorETag
	snapshot := m.editorSnapshot
	m.editorTarget = ""
	m.editorETag = ""
	m.editorSnapshot = nil
	if msg.err != nil {
		m.err = msg.err.Error()
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
		return m, toastCmd
	}
	if !msg.changed {
		return m, nil
	}
	if _, ok := m.beanETag(id); !ok {
		// F04 (bean bt-6bgn, T2-Re-Review-Fund -- same data-loss class as
		// F01): an externally deleted target used to discard the FULL
		// $EDITOR text with no recovery -- pikant, WITHOUT this guard the
		// CLI call would fail and F01's UpdateWhole-wrap below would
		// recover; the guard actively prevented that. Same recovery
		// convention as F01 now: kept tempfile, path appended to the
		// status text, toast ctx carries it. Warn semantics (toastWarn,
		// not an error) stay unchanged -- only the recovery half is new.
		const note = "Item no longer exists — editor edit discarded"
		m.err = note
		toastCtx := ""
		if path, werr := writeConflictTempFile(msg.content); werr == nil {
			m.err += " — your version: " + path
			toastCtx = "Version saved: " + path
		}
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastWarn, note, toastCtx, nil, false)
		return m, toastCmd
	}

	content := msg.content
	fm, body, perr := parseRawBean(content)
	if perr != nil {
		m.err = "Invalid item format: " + perr.Error()
		if path, werr := writeConflictTempFile(content); werr == nil {
			m.err += " — your version: " + path
		}
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
		return m, toastCmd
	}

	diff := buildWholeEditDiff(snapshot, fm, body)
	store := m.store
	return m, mutateCmd(store, func() error {
		err := store.UpdateWhole(id, diff, etag)
		if err != nil {
			// F01 (bt-z4b1 Review-Runde 1, critical, live reproduced):
			// EVERY UpdateWhole failure -- not just errors.Is(_,
			// data.ErrConflict) -- must be recoverable. A CLI-side
			// VALIDATION_ERROR after a successful parse (hand-typed
			// `status: banana`, deleted title: line, ...) used to fall
			// through here bare, and applyMutationResult's else branch
			// discarded the PO's whole $EDITOR session to its
			// unconditional reload. conflictWithRecovery's Unwrap passes
			// ErrConflict through, so the conflict branch's errors.Is
			// still fires for genuine conflicts -- no separate conflict
			// special-case needed anymore.
			if path, werr := writeConflictTempFile(content); werr == nil {
				return &conflictWithRecovery{err: err, path: path}
			}
		}
		return err
	})
}

// beanETag reads id's CURRENT etag straight from the live index (design
// decision d, bean bt-dlgk) -- NEVER a copy captured when an overlay opened,
// so a watch-reload between open and submit is automatically honored and a
// "real" ETag conflict is only ever the narrow Submit<->Disk race window.
// ok=false means id is no longer in the index (deleted by another agent, or
// this reload's own applyLoaded already dropped it) -- callers must close
// their overlay and surface a status-line note instead of firing a doomed
// mutation against a bean that no longer exists.
func (m model) beanETag(id string) (etag string, ok bool) {
	if m.idx == nil {
		return "", false
	}
	b, ok := m.idx.ByID[id]
	if !ok {
		return "", false
	}
	return b.ETag(), true
}

// createInFlightNote is the brief status-line note shown when the PO tries
// to start a SECOND create while one is still parked-or-in-flight (F1,
// Review-Runde 2, Finding 1b) -- one shared string so keyNodeAction's Create
// case and submitForm's "create" case (box_confirm_create.go) can't drift.
const createInFlightNote = "Creation already in progress — please wait"

// keyNodeAction routes the node-focused mutation keys (design-spec §7:
// s/t/a/B/c/d/e -- Status/TagAssign/Assign/Blocking/Create/Delete/Editor;
// plus jira-style-ui experiment S4's o/u -- Type/Priority, wired onto the
// SAME value-menu machinery as Status).
// Placed in handleKey directly after the Refresh check and BEFORE the
// m.detailFocus dispatch, since node actions must act on m.focusedBean()
// regardless of which pane currently has focus (focusedBean() already
// covers Tree, Ready AND detail focus, the same view-agnostic dispatcher
// E2 Task 2 built) -- routing through keyDetailFocus/keyTree/keyReady
// first would either shadow these keys entirely or require duplicating the
// dispatch in three places. Create is the ONE key that works without a
// focused bean (an empty repo can still be seeded); every other key here is
// a handled-but-silent no-op with no focused bean (design decision, plan
// »Task 1« Step 10: a user-facing "not yet" text would be throwaway work,
// the remaining keys land in the immediately following tasks T2-T6).
// Returns handled=false for every other key so callers fall through to the
// view-specific dispatch.
func (m model) keyNodeAction(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	switch {
	case keybind.Matches(msg, keys.Create):
		// F1 (Review-Runde 2, Async-Gap-Clobbering, Finding 1b): a create is
		// already parked-or-in-flight (pendingCreate's dual meaning, types.go
		// doc-stamp) -- refuse to open a SECOND Create-Form on top of it
		// rather than clobbering the single createDraft/pendingCreate slots.
		// handleKey's capture order (m.form != nil / m.overlay != overlayNone
		// checked BEFORE keyNodeAction is ever reached) already rules out
		// "form or Confirm-Gate currently open"; this guard closes the
		// remaining gap -- the async window AFTER the Confirm-Gate's enter
		// fires the createCmd but BEFORE createDoneMsg resolves it, where
		// overlay/form are both back to their "nothing open" state.
		if m.pendingCreate != nil {
			m.err = createInFlightNote
			// E5 Task 1 (bean bt-6dts): Toast additiv neben m.err (design
			// decision a) -- toastWarn, nicht toastError: dies ist ein
			// Hinweis ("bitte warten"), kein harter Fehler.
			var toastCmd tea.Cmd
			m, toastCmd = m.showToast(toastWarn, createInFlightNote, "", nil, false)
			return true, m, toastCmd
		}
		// E3 Task 4 (bean bt-y4ly): Create works WITHOUT a focused bean (an
		// empty repo can still be seeded) -- the ONLY key here that skips
		// the focusedBean() guard below.
		nm, cmd := m.openCreateForm()
		return true, nm, cmd
	case keybind.Matches(msg, keys.Status),
		keybind.Matches(msg, keys.Type),
		keybind.Matches(msg, keys.Priority),
		keybind.Matches(msg, keys.TagAssign),
		keybind.Matches(msg, keys.Assign),
		keybind.Matches(msg, keys.Blocking),
		keybind.Matches(msg, keys.Delete),
		keybind.Matches(msg, keys.Editor):
		b := m.focusedBean()
		if b == nil {
			return true, m, nil // handled, silent no-op -- no node to act on
		}
		if keybind.Matches(msg, keys.Status) {
			return true, m.openValueMenu("status"), nil
		}
		// Type (o) / Priority (u) — jira-style-ui experiment S4: same
		// value-menu machinery as Status, only the seeded group differs
		// (openValueMenu/applyValueMenuSelection already fully support
		// "type"/"priority", box_menu_value.go -- pre-existing via the
		// Command-Center and the Meta field-level enter cascade).
		if keybind.Matches(msg, keys.Type) {
			return true, m.openValueMenu("type"), nil
		}
		if keybind.Matches(msg, keys.Priority) {
			return true, m.openValueMenu("priority"), nil
		}
		if keybind.Matches(msg, keys.TagAssign) {
			nm, cmd := m.openTagPicker()
			return true, nm, cmd
		}
		if keybind.Matches(msg, keys.Assign) {
			nm, cmd := m.openParentPicker()
			return true, nm, cmd
		}
		if keybind.Matches(msg, keys.Blocking) {
			nm, cmd := m.openBlockingPicker()
			return true, nm, cmd
		}
		if keybind.Matches(msg, keys.Editor) {
			// D01 (design-spec.md §15 PF-17, bean bt-z4b1, supersedet E8-
			// B10): "e"/"ctrl+e" share ONE keys.Editor binding (design-spec
			// §7) and now ALWAYS open the SAME whole-bean $EDITOR path,
			// unconditionally -- egal welche Sektion/Feld-Ebene, auch ohne
			// aktiven Detail-Fokus, aus Tree/Ready/Detail (PO verbatim:
			// "egal an welcher Stelle"). The former msg.String()=="ctrl+e"
			// vs. section-context-sensitive "e" branching (B10) is GONE --
			// there is only ONE editor path now, not two convergent ones
			// ("ctrl+e-Sonderpfad vereinheitlichen", PO/D01-Wortlaut). e/
			// ctrl+e NEVER open the Title-Edit-Form anymore -- openEditTitleForm
			// stays reachable ONLY via enter on the title: field
			// (activateDetailField, below). F2 (Review-Runde 2, Finding 2,
			// ETag-Lost-Update) still holds, extended to the full bean
			// snapshot by D01: the etag AND snapshot are captured HERE, at
			// open time -- never a fresh m.beanETag(id)/m.idx read in
			// applyEditorFinished at submit time (see that func's doc-stamp
			// for the full lost-update rationale).
			nm, cmd := m.openBeanEditor(b)
			return true, nm, cmd
		}
		if keybind.Matches(msg, keys.Delete) {
			// E3 Task 6 (bean bt-ppzb): opens the Delete-Confirm overlay
			// (box_confirm_delete.go) -- Kinder-Count-Warnung, no mutation
			// fires until keyDeleteConfirm's own enter.
			return true, m.openDeleteConfirm(), nil
		}
		return true, m, nil // unreachable: msg matched one of the eight keys in this case's condition above
	case keybind.Matches(msg, keys.Yank):
		// E5 Task 3 (bean bt-e4a6, design decision b): `y` always acts on
		// m.focusedBean() -- Tree/Ready identical (focusedBean is already
		// the view-agnostic dispatcher E2 Task 2 built).
		b := m.focusedBean()
		if b == nil {
			return true, m, nil // orphan-root cursor: handled, silent no-op (Plan Step 6)
		}
		if err := clip.Copy(beanContext(m.idx, b)); err != nil {
			nm, cmd := m.showToast(toastWarn, "Yank failed", "", nil, false)
			return true, nm, cmd
		}
		nm, cmd := m.showToast(toastInfo, "Copied: "+b.ID, "", nil, false)
		return true, nm, cmd
	}
	return false, m, nil
}

// keyOverlay routes to the currently open node-action overlay's own key
// handler (design decision a2: exactly one overlayID is active at a time).
// Checked in handleKey ahead of the global ctrl+c/q/tab switch (same
// full-capture precedent as m.searchActive just above it) so
// e.g. "q" cannot leak through to a quit-request while a menu/picker is
// open. T1 wires overlayValueMenu; T2/T3/T4/T6 add their cases as their
// overlays land.
func (m model) keyOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.overlay {
	case overlayValueMenu:
		return m.keyValueMenu(msg)
	case overlayTagPicker:
		return m.keyTagPicker(msg)
	case overlayParentPicker:
		return m.keyParentPicker(msg)
	case overlayBlockingPicker:
		return m.keyBlockingPicker(msg)
	case overlayCreateConfirm:
		return m.keyCreateConfirm(msg)
	case overlayDeleteConfirm:
		return m.keyDeleteConfirm(msg)
	case overlayTaskOutput:
		return m.keyTaskOutput(msg)
	case overlayFilterMenu:
		return m.keyFilterMenu(msg)
	}
	return m, nil
}

// applyBleveResult applies an async Bleve search result (E2 Task 3, bean
// bt-4ep2) -- discarded if the query has moved on since the request was
// dispatched (staleness guard, chosen over a debounce timer: every
// qualifying keystroke dispatches its own beans-CLI subprocess, but only the
// response tagged for the CURRENT search rest text is ever applied; see
// messages.go's searchBleveResultMsg doc comment for the full rationale).
// bt-2kfl D03: compared against m.searchPrefixRest, not the raw
// m.searchQuery -- msg.query was itself dispatched against rest
// (maybeBleveCmd), so staleness must be judged against the same value.
func (m model) applyBleveResult(msg searchBleveResultMsg) (tea.Model, tea.Cmd) {
	if msg.query != m.searchPrefixRest {
		return m, nil // stale -- rest has moved on since this request was sent
	}
	m.searchBleveLoading = false
	if msg.err != nil {
		m.err = msg.err.Error()
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
		return m, toastCmd
	}
	ids := make(map[string]bool, len(msg.ids))
	for _, id := range msg.ids {
		ids[id] = true
	}
	m.searchBleveIDs = ids
	m.searchBleveFor = msg.query
	return m, nil
}

// applyLoaded rebuilds the Index from a (initial or reload) beansLoadedMsg
// and restores the cursor by bean ID: the same bean keeps the same ID across
// a reload, so the cursor stays on it; a bean that vanished (deleted/
// renamed) clamps to roughly where it used to sit rather than jumping back
// to the top (US-10: reload must never lose the PO's place arbitrarily).
func (m model) applyLoaded(msg beansLoadedMsg) (model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err.Error()
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
		return m, toastCmd
	}
	m.err = ""

	// Capture the cursor's position in the OLD tree before swapping idx, so
	// a vanished bean can clamp near its old spot instead of resetting to 0.
	oldPos := 0
	if m.idx != nil {
		oldPos = m.cursorPos(flattenTree(m.roadmapScheduled, m.roadmapUnscheduled, m.notOnRoadmap, m.expanded))
	}

	m = m.setItems(msg.beans, msg.ready)
	nodes := m.visibleNodes()

	if len(nodes) == 0 {
		m.cursorID = ""
	} else {
		// B01 (bt-39cl Review R1): both restore paths must be placeholder-
		// safe. (a) The id-match loop requires n.id != "" -- a placeholder's
		// id IS "" (view_browse_repo.go treeNode doc), so a stale
		// m.cursorID == "" (e.g. a previous empty load) would otherwise
		// "find" the first placeholder and leave the cursor parked on a
		// non-selectable row. (b) The positional oldPos fallback runs
		// through skipPlaceholder so a clamp landing on a placeholder index
		// in the NEW filtered tree slides onto the nearest real row instead.
		cursorFound := false
		for _, n := range nodes {
			if n.id != "" && n.id == m.cursorID {
				cursorFound = true
				break
			}
		}
		if !cursorFound {
			if oldPos >= len(nodes) {
				oldPos = len(nodes) - 1
			}
			m.cursorID = nodes[skipPlaceholder(nodes, oldPos, 1)].id
		}
	}

	return m.showWarn(msg.warn)
}

// handleKey is the top-level key dispatch (devd keys.go port pattern):
// the quit-confirm modal fully captures input first; then a couple of
// view-global keys (ctrl+c hard-quit, tab focus-swap -- Q01, deliberately
// NOT part of keymap.go's Right binding); then the tree/detail handlers.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c quits at once from every state (bean beans-zata): checked before
	// every capture (confirm, search, filter, form, overlay, lobby). tui.Run
	// closes the store after the program ends.
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	// A buffered fold prefix lives for exactly one key press, whatever that key does.
	foldPending := m.foldPrefix
	m.foldPrefix = false
	if m.confirmQuit {
		return m.keyConfirmQuit(msg)
	}

	// E2 Task 3 (bean bt-4ep2, design-spec.md §7: "Single-Keys inaktiv bei
	// fokussierter Texteingabe"): the search input captures EVERY key except
	// enter/esc while focused -- typing "q" or "tab" must produce that
	// character in the search box, not quit/focus-swap. Checked before the
	// ctrl+c/q/tab switch below, mirroring how m.confirmQuit's modal already
	// captures input ahead of that same switch (keyConfirmQuit does not
	// special-case ctrl+c either -- same existing precedent, applied here for
	// consistency rather than introducing a second, partial-capture pattern).
	if m.searchActive {
		return m.keySearchInput(msg)
	}

	// E3 Task 4 (bean bt-y4ly): an open huh Form fully captures input too,
	// same precedent as searchActive/confirmQuit above -- checked
	// BEFORE the node-action overlay switch below since m.form != nil is its
	// own separate capture state, not one of the overlayID cases (design
	// decision a2: "huh forms are not a menu overlay").
	if m.form != nil {
		return m.keyForm(msg)
	}

	// E3 (bean bt-dlgk): the open node-action overlay (Value-Menü T1, Tag-/
	// Parent-/Blocking-Picker T2/T3, Create-/Delete-Confirm T4/T6) fully
	// captures input, same precedent as m.searchActive just
	// above -- checked ahead of the ctrl+c/q/tab switch so those single keys
	// cannot leak through while a menu/picker is open.
	if m.overlay != overlayNone {
		return m.keyOverlay(msg)
	}

	// E5 Task 6 (bean bt-zhwl, design decision h): the Lobby fully captures
	// input, same full-capture precedent as m.overlay above --
	// but positioned HERE, AHEAD of the ctrl+k/`?`/keys.Picker bare MATCH
	// checks below (an ERRATUM vs. bt-0l8c's own forward-looking Notes-für-
	// T6 sketch, which listed keys.Picker's match check BEFORE the
	// m.view==viewLobby state check). This mirrors the SAME "every
	// full-capture STATE check precedes every bare keybind MATCH check"
	// rule this file's own m.helpOpen doc-comment already established for a
	// structurally identical bug (an open Command-Center filter racing a
	// later-added `?` match check for the same key). Concretely: repoQuery
	// -- the Lobby's own live filter over Settings.Repos -- must accept "p"
	// as an ordinary filter character like any other (a real repo path is
	// highly likely to contain the letter, e.g. this very repo's own
	// "beans-tui-repository" via "rePository"); with keys.Picker checked
	// first instead, every keystroke containing "p" would re-open
	// (reset) the Lobby instead of ever reaching keyLobby's own textinput
	// handling. ctrl+k/`?` are DELIBERATELY not special-cased to still
	// reach the Lobby either (Port devd's own keyHome, view_home.go, which
	// never lets ctrl+k or any other global shortcut leak into viewHome's
	// typing state) -- once inside the Lobby, every key is the Lobby's own
	// to interpret; keyLobby provides its own esc/q path (US-01 parity,
	// that function's own doc-stamp).
	if m.view == viewLobby {
		return m.keyLobby(msg)
	}

	// E10 Task 2 (bean bt-r92i, epic bt-362n D06): the Tag-Management page is
	// ALSO a full-capture state, checked at the SAME checkpoint as
	// m.view==viewLobby just above (identical rationale: every full-capture
	// STATE check precedes every bare keybind MATCH check below). Without
	// this, focusedBean()'s default branch would fall back to the Tree
	// cursor while the PO is looking at the tag registry, letting global
	// node-action keys (s/t/a/r/c/d/e) fire against a STALE, unrelated bean
	// (verified against this file's own focusedBean(), lines ~1019-1029 --
	// exactly the failure class LESSONS-LEARNED already documents twice,
	// "Exit-Pfad-Inventur"/"Lobby-Exit im Hauptfall unerreichbar", epic
	// bt-362n body's own D06 citation).
	if m.view == viewTagManagement {
		return m.keyTagManagement(msg)
	}

	// E4 Task 1 (bean bt-jpgn, design decision h): the Command-Center's own
	// open state fully captures, same precedent as m.overlay
	// above.
	if m.paletteOpen {
		return m.keyPalette(msg)
	}
	// E5 Task 2 (bean bt-wpn9, design decision h): the Help-Overlay's own
	// open state fully captures, same precedent as m.paletteOpen just
	// above -- checked HERE (after m.paletteOpen, before the bare
	// keys.Palette match below) rather than the epic-plan's original
	// "Geteilte Infrastruktur" sketch (which listed helpOpen/keys.Help
	// BEFORE m.paletteOpen): that ordering is an ERRATUM (see bean
	// Deviations) -- it would let `?`, typed as a literal query character
	// into an open Command-Center filter, hijack input and open Help
	// instead of reaching keyPalette. Keeping every full-capture STATE
	// check (form/overlay/paletteOpen/helpOpen) ahead of every
	// bare keybind MATCH check (keys.Palette/keys.Help) preserves the
	// existing precedent this file already establishes end to end.
	if m.helpOpen {
		return m.keyHelp(msg)
	}
	// Roadmap fold keys (zo/zc/zR/zM): intercepted AHEAD of every bare key
	// below, whose letters (c, R, M, ...) would otherwise swallow the second
	// key of the sequence.
	if m.view == viewBrowseRepo && !m.flatView && !m.detailFocus && !m.filterFocus {
		switch s := msg.String(); {
		case foldPending:
			return m.keyFold(foldPrefixKey + s), nil
		case s == foldPrefixKey:
			m.foldPrefix = true
			return m, nil
		case len(s) == 2 && s[:1] == foldPrefixKey && msg.Type == tea.KeyRunes:
			// both keys arrived in one terminal read (fast typing, paste): bubbletea delivers them as one KeyRunes message
			return m.keyFold(s), nil
		}
	}
	// E4 Task 1 (design decision h): ctrl+k/K opens the palette from ANY
	// view (design-spec §7: "von überall").
	if keybind.Matches(msg, keys.Palette) {
		return m.openPalette()
	}
	// E5 Task 2 (bean bt-wpn9, design decision h): `?` opens Help from ANY
	// view, same precedent as ctrl+k/keys.Palette just above.
	if keybind.Matches(msg, keys.Help) {
		m.helpOpen = true
		return m, nil
	}

	// E5 Task 6 (bean bt-zhwl, design decision h): `p` opens the Lobby from
	// any OTHER (non-Lobby) view.
	if keybind.Matches(msg, keys.Picker) {
		return m.openLobby()
	}

	switch msg.String() {
	case "q":
		return m.requestQuit()
	}

	// tab and shift+tab (keys.FocusIn/FocusOut) step the focus along the ring
	// of the active layout (stepFocus, layout.go). They sit ahead of
	// keyNodeAction/keyDetailFocus/keyReady/keyTree below, and ahead of the
	// focused filter field, so every region reacts the same.
	if keybind.Matches(msg, keys.FocusIn) {
		return m.stepFocus(1), nil
	}
	if keybind.Matches(msg, keys.FocusOut) {
		return m.stepFocus(-1), nil
	}

	// v / shift+v cycle the layout (keyLayout, layout.go).
	if handled, nm, cmd := m.keyLayout(msg); handled {
		return nm, cmd
	}

	// f jumps to the filter bar's Type field (switching to a layout with a
	// filter bar when needed); while the bar has focus it owns the keys.
	if keybind.Matches(msg, keys.Filter) && (m.view == viewBrowseRepo || m.view == viewReady) {
		return m.focusFilter(), nil
	}
	if m.filterFocus {
		return m.keyFilterField(msg)
	}

	// M / L / R switch between the Roadmap, the flat list and Ready (keyView,
	// layout.go); reachable from every list view, but not while the filter bar
	// owns the keys.
	if handled, nm := m.keyView(msg); handled {
		return nm, nil
	}

	if keybind.Matches(msg, keys.Refresh) {
		return m, loadCmd(m.store)
	}

	// bean bt-adkn Rework (B1, PO-Reject 2026-07-21 "PageDown/PageUp scrollt das
	// gesamte linke pane"): pgup/pgdn page the Detail-Body by one FULL screen,
	// FOCUS-INDEPENDENTLY -- exactly like the mouse wheel (mouse.go's
	// boxFormWheelHit), which pages the Body no matter which pane has focus. The
	// original bt-adkn wired this into keyDetailFocus, so it only fired AFTER a
	// tab into the Detail region; at the default Tree focus pgdn fell through to
	// keyTree (no pgup/pgdn binding there) and was a silent no-op -- the very
	// symptom the PO reported. Lifting it to this global checkpoint (ahead of
	// keyNodeAction/detailFocus/keyTree, the same position the node-action keys
	// occupy for the same focus-agnostic reason) makes pgup/pgdn act on
	// focusedBean() regardless of focus, through the SAME adjustBoxFormScroll
	// mutation point line-wise up/down and the wheel use (one clamp/reset path,
	// no drift -- SSTD load-bearing constraint). One page == the pane's own
	// visible line budget (boxFormScrollBounds' height).
	//
	// A layout without a Detail pane has no Body to page.
	if m.layout.hasDetail() {
		if b := m.focusedBean(); b != nil {
			switch {
			case keybind.Matches(msg, boxFormPageUp):
				_, page := boxFormScrollBounds(m, b)
				return m.adjustBoxFormScroll(b, -page), nil
			case keybind.Matches(msg, boxFormPageDown):
				_, page := boxFormScrollBounds(m, b)
				return m.adjustBoxFormScroll(b, page), nil
			}
		}
	}

	// E3 (bean bt-dlgk): node-focused mutation keys (s/t/a/B/c/d/e) --
	// checked BEFORE the detailFocus/keyReady/keyTree dispatch below so
	// they act on m.focusedBean() regardless of which pane has focus (see
	// keyNodeAction's own doc comment for the full rationale).
	if handled, nm, cmd := m.keyTaskAction(msg); handled {
		return nm, cmd
	}
	if handled, nm, cmd := m.keyNodeAction(msg); handled {
		return nm, cmd
	}

	if keybind.Matches(msg, keys.Back) && m.escDismissesToast() {
		return m.dismissToast()
	}
	if m.detailFocus {
		return m.keyDetailFocus(msg)
	}
	if m.view == viewReady {
		return m.keyReady(msg)
	}
	return m.keyTree(msg)
}

// focusedBean returns the bean the Detail-Accordion currently targets,
// independent of which view is active (devd port focusedIssue,
// view_detail_issue.go:20-35) -- view-agnostic so Task 5's Ready view
// (viewReady case below, bean bt-gzu6) reuses keyDetailFocus verbatim.
func (m model) focusedBean() *item.Item {
	switch m.view {
	case viewReady: // E2 Task 5: the Ready list's own selection, NOT the (possibly stale/irrelevant) tree cursor
		return m.readySelected()
	default: // viewBrowseRepo (T8)
		// S5 (jira-style-ui experiment, Nested/Flat Browse toggle): in flat
		// mode the flat list's OWN cursor (flatList) is the selection, not
		// the (frozen, untouched-while-flat) Tree cursorID -- mirrors the
		// viewReady case just above (a view/mode-local cursor overrides
		// the Tree's own).
		if m.flatView {
			return m.flatSelected()
		}
		nodes := m.visibleNodes()
		pos := m.cursorPos(nodes)
		if pos < 0 || pos >= len(nodes) || nodes[pos].section != "" || nodes[pos].placeholder {
			return nil
		}
		return nodes[pos].bean
	}
}

// keyDetailFocus drives the Detail region. pgup/pgdn page the Body at
// handleKey's global checkpoint, never here.
//
// up/down walk the box-form field cursor (boxFormNav, box_nav_field.go,
// linear with wrap, dragging the scroll offset along so the cursored field
// is always in the viewport); enter opens the cursored field's editor
// (boxFormActivateCursor, the same dispatch a click takes); left/right do
// nothing; esc leaves the region (in layoutDetail, which has no other, it is
// the list views' esc: escBack).
func (m model) keyDetailFocus(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keybind.Matches(msg, keys.Back) {
		if m.layout == layoutDetail {
			return m.escBack(), nil
		}
		m.detailFocus = false
		return m, nil
	}
	b := m.focusedBean()
	if b == nil { // defensive guard: orphan-root cursor, no focusable bean
		if m.layout != layoutDetail { // layoutDetail keeps focus: it shows no other region
			m.detailFocus = false
		}
		return m, nil
	}

	switch navKey(msg.String()) {
	case "up":
		return m.boxFormNav(b, "prev"), nil
	case "down":
		return m.boxFormNav(b, "next"), nil
	case "left", "right":
		return m, nil
	}
	if keybind.Matches(msg, keys.Enter) {
		return m.boxFormActivateCursor(b)
	}

	return m, nil
}

// expandAncestorsOf returns a NEW expanded map (I01 copy-on-write) with every
// tree ancestor of id (walking treeParent up to a root, synthetic roots
// included) marked expanded, so a relation-jump target is guaranteed visible
// in the next visibleNodes() call. visited guards the walk against a cyclic
// treeParent map (defensive; the roadmap tree itself is acyclic).
func expandAncestorsOf(treeParent map[string]string, expanded map[string]bool, id string) map[string]bool {
	out := cloneBoolMap(expanded)
	visited := map[string]bool{id: true}
	for p := treeParent[id]; p != "" && !visited[p]; p = treeParent[p] {
		visited[p] = true
		out[p] = true
	}
	return out
}

// openSearchInput enters the search-input capture state (E2 Task 3, bean
// bt-4ep2): pre-loads the input with the CURRENT query (re-opening a
// committed search resumes editing it, mirrors devd's SetValue(m.treeQuery)).
// Factored out of keyTree (E2 Task 5, bean bt-gzu6): keyReady
// (view_browse_ready.go) shares this verbatim -- the search head/live
// filter is ONE piece of shared model state, not a per-view concept.
func (m model) openSearchInput() (tea.Model, tea.Cmd) {
	m.searchActive = true
	m.searchInput.SetValue(m.searchQuery)
	m.searchInput.CursorEnd()
	m.searchInput.Focus()
	return m, textinput.Blink
}

// keyTree drives the tree: up/down move the cursor, right/left expand/
// collapse, enter toggles expand (no-op on a leaf, per task scope). `/`, `X`,
// and the esc-cascade's search/filter-clearing rung (E2 Task 3+4) are
// checked FIRST, ahead of the len(nodes)==0 short-circuit below -- opening
// the search box or clearing an active query/facet must work
// even on an empty/pre-load tree.
func (m model) keyTree(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keybind.Matches(msg, keys.Search):
		// Port devd keyTree's Search case (view_browse_project.go:689-699),
		// minus loadAllIssues -- beans-tui already holds the full Index in
		// memory (no separate "load everything for search" step needed).
		return m.openSearchInput()

	case keybind.Matches(msg, keys.FilterClear):
		// X as a direct top-level reset (design-
		// spec.md, mirrors devd's esc-cascade also clearing filters below) --
		// wired to the SAME clearFacets() helper keyFilterField's and
		// keyFilterMenu's X-cases use.
		m = m.clearFacets()
		return m.resetCursorToFirstVisible(), nil
	case keybind.Matches(msg, keys.Back):
		// esc-cascade: clear a committed query and active facets, else leave the
		// flat list for the Roadmap (escBack, layout.go); no-op in the Roadmap.
		return m.escBack(), nil
	}

	// Flat list mode: once the shared search/filter/FilterClear/Back handling
	// above has run (identical in both modes -- flat mode narrows/widens the
	// SAME m.beanMatches-filtered set the Tree does), flat mode's own
	// up/down-only cursor takes over instead of the Tree's node walk below.
	if m.flatView {
		return m.keyFlat(msg)
	}

	nodes := m.visibleNodes()
	if len(nodes) == 0 {
		return m, nil
	}
	pos := m.cursorPos(nodes)

	switch {
	case keybind.Matches(msg, keys.NextSibling), keybind.Matches(msg, keys.PrevSibling):
		dir := 1
		if keybind.Matches(msg, keys.PrevSibling) {
			dir = -1
		}
		if i := siblingPos(nodes, pos, dir); i >= 0 {
			m.cursorID = nodes[i].id
		}
		return m, nil
	}

	switch navKey(msg.String()) {
	case "up":
		// E5 Task 4 (bean bt-mne6): factored into treeCursorMove
		// (view_browse_repo.go) so the keyboard AND the wheel dispatch
		// (mouse.go handleMouse) share the exact same clamp logic instead
		// of two independent copies.
		return m.treeCursorMove(nodes, -1), nil
	case "down":
		return m.treeCursorMove(nodes, 1), nil
	case "right":
		// I01 (bt-39cl Review R1): explicit placeholder guard -- a
		// placeholder happens to no-op through setExpanded anyway
		// (hasKids=false), but that is incidental, not a contract; the
		// guard makes "never a legitimate target" explicit (same doctrine
		// as skipPlaceholder/mouseTreeClick).
		if nodes[pos].placeholder {
			return m, nil
		}
		return m.setExpanded(nodes[pos], true), nil
	case "left":
		if nodes[pos].placeholder { // I01, see "right" above
			return m, nil
		}
		return m.setExpanded(nodes[pos], false), nil
	}

	if keybind.Matches(msg, keys.Enter) {
		n := nodes[pos]
		if n.placeholder || !n.hasKids { // I01: placeholder explicit, not incidental via hasKids
			return m, nil // leaf: no-op for now (E2 opens detail focus instead)
		}
		return m.setExpanded(n, !n.open), nil
	}
	return m, nil
}

// keySearchInput drives the active search textinput (E2 Task 3, bean
// bt-4ep2, port devd keyTreeSearch view_browse_project.go:1073-1097): every
// keystroke updates m.searchQuery LIVE (immediate local filter, not just on
// commit) and resets the cursor to the freshly filtered list's first row
// (mirrors devd's unconditional treeCursor=0). enter commits (blurs the
// input; the filter/query itself is NOT cleared). esc cancels AND clears the
// query entirely (esc-cascade Rung 1 -- distinct from enter). Both also
// dispatch an async Bleve search once the query is due (maybeBleveCmd).
func (m model) keySearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.searchActive = false
		m.searchInput.Blur()
		m.searchQuery = strings.TrimSpace(m.searchInput.Value())
		m = m.applySearchPrefixes() // bt-2kfl D03: reparse on every commit too
		m = m.resetCursorToFirstVisible()
		return m.dispatchBleveIfDue(nil)
	case tea.KeyEsc:
		m.searchActive = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.searchQuery = ""
		m = m.applySearchPrefixes() // bt-2kfl D02: clearing the query drops typed prefixes too
		return m.resetCursorToFirstVisible(), nil
	}

	var inputCmd tea.Cmd
	m.searchInput, inputCmd = m.searchInput.Update(msg)
	m.searchQuery = strings.TrimSpace(m.searchInput.Value())
	m = m.applySearchPrefixes() // bt-2kfl D03: parser runs on EVERY keystroke
	m = m.resetCursorToFirstVisible()
	return m.dispatchBleveIfDue(inputCmd)
}

// resetCursorToFirstVisible parks the cursor on the (newly filtered) node
// list's first row -- called on every search keystroke/clear, mirrors devd's
// unconditional `m.treeCursor = 0` (view_browse_project.go:1073-1097): a
// bean ID, not an index, is beans-tui's own cursor representation (T8), so
// the equivalent reset is "first visible node's ID" rather than a bare 0.
func (m model) resetCursorToFirstVisible() model {
	nodes := m.visibleNodes()
	if len(nodes) == 0 {
		m.cursorID = ""
		return m
	}
	m.cursorID = nodes[skipPlaceholder(nodes, 0, 1)].id
	return m
}

// dispatchBleveIfDue is keySearchInput's shared tail: batches an extra Cmd
// (the textinput's own Update cmd, or nil) together with maybeBleveCmd()'s
// dispatch when one is due, flagging searchBleveLoading so the search head
// (treeSearchLine) can surface it.
func (m model) dispatchBleveIfDue(extra tea.Cmd) (tea.Model, tea.Cmd) {
	bleve := m.maybeBleveCmd()
	if bleve == nil {
		return m, extra
	}
	m.searchBleveLoading = true
	return m, tea.Batch(extra, bleve)
}

// maybeBleveCmd returns a searchCmd dispatch when the current search rest
// text (m.searchPrefixRest -- bt-2kfl D03, typed `st:`/`ty:`/`pr:`/`tag:`
// tokens stripped out) has reached the Bleve threshold (>=3 chars,
// design-spec.md §6 V2) and differs from the query the current
// searchBleveIDs answer (searchBleveFor) -- nil below the threshold, or when
// nothing has changed since the last dispatch. Deliberately NOT debounced
// (E2 Task 3 commit rationale, bean bt-4ep2): the plan's own design (and the
// bean's Akzeptanz criteria) choose a staleness guard on the RESPONSE
// (applyBleveResult) over delaying the REQUEST with a timer -- every
// qualifying keystroke may dispatch its own beans-CLI subprocess, but only
// the freshest response is ever applied. bt-2kfl D03: prefix tokens
// themselves never reach Bleve -- only rest is ever passed to searchCmd.
func (m model) maybeBleveCmd() tea.Cmd {
	q := m.searchPrefixRest
	if len(q) < 3 || q == m.searchBleveFor {
		return nil
	}
	return searchCmd(m.store, q)
}

// setExpanded sets n's expand state in m.expanded; a no-op for leaves. I01
// (bean bt-7jr8 T8-review): clones m.expanded via cloneBoolMap before writing
// -- expanded is COPY-ON-WRITE (types.go doc-stamp), never mutated in place.
func (m model) setExpanded(n treeNode, open bool) model {
	if !n.hasKids {
		return m
	}
	m.expanded = cloneBoolMap(m.expanded)
	if open {
		m.expanded[n.id] = true
	} else {
		delete(m.expanded, n.id)
	}
	return m
}

// foldPrefixKey starts the roadmap fold sequences (zo/zc/zR/zM, keys.ExpandNode ...).
const foldPrefixKey = "z"

// siblingPos returns the index of the previous (dir<0) or next (dir>0) visible
// sibling of nodes[pos]: same depth under the same parent, expanded children
// skipped. -1 at the edge, where the cursor stays.
func siblingPos(nodes []treeNode, pos, dir int) int {
	d := nodes[pos].depth
	for i := pos + dir; i >= 0 && i < len(nodes); i += dir {
		if nodes[i].depth < d {
			return -1 // left the parent: no further sibling
		}
		if nodes[i].depth == d && !nodes[i].unselectable() {
			return i
		}
	}
	return -1
}

// foldTargets returns every node with children that is id or lies below id in
// the tree ("" = every node), the set zo/zc/zR/zM expand or collapse.
func (m model) foldTargets(id string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range m.treeParent {
		if seen[p] {
			continue
		}
		seen[p] = true
		for x, n := p, 0; x != "" && n <= len(m.treeParent); x, n = m.treeParent[x], n+1 {
			if id == "" || x == id {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// keyFold runs a completed fold sequence on the roadmap tree. A collapse can
// hide the cursor row; it then moves to the nearest visible ancestor.
func (m model) keyFold(seq string) model {
	nodes := m.visibleNodes()
	if len(nodes) == 0 {
		return m
	}
	var id string
	open := true
	switch {
	case bindHas(keys.ExpandAll, seq):
	case bindHas(keys.CollapseAll, seq):
		open = false
	case bindHas(keys.ExpandNode, seq), bindHas(keys.CollapseNode, seq):
		n := nodes[m.cursorPos(nodes)]
		if n.placeholder {
			return m
		}
		id, open = n.id, bindHas(keys.ExpandNode, seq)
	default:
		return m
	}
	m.expanded = cloneBoolMap(m.expanded)
	for _, t := range m.foldTargets(id) {
		if open {
			m.expanded[t] = true
		} else {
			delete(m.expanded, t)
		}
	}
	visible := map[string]bool{}
	for _, n := range m.visibleNodes() {
		visible[n.id] = true
	}
	for x, n := m.cursorID, 0; x != "" && n <= len(m.treeParent); x, n = m.treeParent[x], n+1 {
		if visible[x] {
			m.cursorID = x
			break
		}
	}
	return m
}
