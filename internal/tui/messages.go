package tui

// messages.go — tea.Msg types + tea.Cmd producers ONLY (port convention from
// devd messages.go: no dispatch/rendering lives here, see update.go/
// view_browse_repo.go).

import (
	"time"

	"github.com/rierek/lif/internal/tui/data"

	tea "github.com/charmbracelet/bubbletea"
)

// toastExpiredMsg clears the corner Toast (E5 Task 1, bean bt-6dts, Port devd
// overlay_show_toast.go's identically-named message) after its kind-specific
// duration elapses -- but only when seq still matches the toast's current
// generation (otherwise a newer toast has already replaced it,
// handleToastExpired, update.go).
type toastExpiredMsg struct{ seq int }

// toastTimeout fires a toastExpiredMsg for the given generation after
// toastDuration(kind) (Port devd overlay_show_toast.go's toastTimeout
// VERBATIM).
func toastTimeout(seq int, kind toastKind) tea.Cmd {
	return tea.Tick(toastDuration(kind), func(time.Time) tea.Msg {
		return toastExpiredMsg{seq}
	})
}

// beansLoadedMsg carries the result of an (initial or reload) data.Client.List
// call. err is non-nil on failure -- Update renders it into the status line
// rather than treating it as fatal: the App-Shell must survive a transient
// beans-CLI failure (e.g. a mid-edit malformed frontmatter file) without
// crashing, per the load error handling called out in the task brief.
type beansLoadedMsg struct {
	beans []data.Bean
	err   error
}

// watchMsg signals that data.Watch's debounced onChange fired. Update reacts
// with a full async reload (loadCmd), never a partial/incremental update
// (design decision D02, mirrored in data.Watch's doc comment) -- and NEVER
// synchronously, since onChange itself must not block (see app.go/watcher.go
// B05 doc contract).
type watchMsg struct{}

// initialWatchMsg hands app.go Run()'s VERY FIRST data.Watch stop func to
// the model (E5 Task 6, bean bt-zhwl) -- without this, m.watchStop would
// start (and stay) nil until the PO's first repo switch, so THAT switch's
// switchRepoCmd(oldStop=nil, ...) would never retire this initial watcher: a
// live fsnotify goroutine on the abandoned first repo, leaking for the rest
// of the process (every one of ITS file changes would still fire a reload
// of whatever repo is CURRENT by then, via m.client -- not wrong-repo data,
// but a real resource leak and a spurious extra reload on every old-repo
// change). Sent ASYNCHRONOUSLY via a goroutine (go p.Send(...), never an
// inline p.Send) for the exact reason watchUnavailableMsg below already
// documents: p.Send blocks until p.Run()'s event loop starts reading, which
// only happens once Run() reaches its own p.Run() call -- an inline send
// here would deadlock Run() before it ever gets there. Delivered well before
// any keypress can plausibly race it (human reaction time vastly exceeds an
// in-process goroutine send), same practical-safety argument this codebase
// already relies on elsewhere (e.g. searchBleveResultMsg's staleness guard
// tolerates async arrival order without a hard ordering guarantee).
type initialWatchMsg struct{ stop func() }

// watchUnavailableMsg signals that data.Watch failed to start at all (app.go
// Run) -- distinct from watchMsg (a live watcher firing). Sent exactly once,
// asynchronously (app.go: a goroutine, since the unbuffered tea.Program.msgs
// channel would otherwise deadlock the caller if sent before p.Run() starts
// consuming it -- same B05-style constraint as watchMsg, just for the
// startup-failure path instead of the steady-state one). I04 (T8 Opus
// quality review): must surface in the status line, never a silent degrade.
type watchUnavailableMsg struct{}

// loadCmd (re)loads all beans via the CLI client, async -- the sole read path
// for both the initial Init() load and every subsequent reload (ctrl+r,
// watchMsg).
func loadCmd(c *data.Client) tea.Cmd {
	return func() tea.Msg {
		beans, err := c.List()
		return beansLoadedMsg{beans: beans, err: err}
	}
}

// repoSwitchedMsg carries the outcome of switchRepoCmd below (E5 Task 6,
// bean bt-zhwl): a repo switch triggered from the Lobby (keyLobby,
// view_lobby.go). err != nil means the TARGET repo failed to validate
// (client/repoDir/beans/watchStop are then zero values and must NOT be
// applied -- applyRepoSwitched, update.go, leaves the CURRENT session
// untouched on this path, the core "Fehlerpfad darf die laufende Session
// NICHT zerstören" constraint, bean bt-zhwl).
type repoSwitchedMsg struct {
	client    *data.Client
	repoDir   string
	beans     []data.Bean
	watchStop func()
	err       error
}

// switchRepoCmd is the Kernschwierigkeit of E5 Task 6 (bean bt-zhwl): a
// tea.Program-DECOUPLED repo switch -- oldStop/notify are both injected
// (never a package-level activeProgram reference INSIDE this function
// itself), so this is fully testable with two plain temp directories and a
// fake notify func(), no real tea.Program required
// (switch_repo_test.go's TestSwitchRepoCmdStopsOldWatcherStartsNew).
// Production wires notify as func(){ activeProgram.Send(watchMsg{}) }
// (app.go/view_lobby.go's keyLobby) -- this function never imports or
// touches activeProgram.
//
// Ordering decision (design note, bean bt-zhwl's own "Fehlerpfad"
// constraint): the NEW repo is validated FIRST (client.List() must
// succeed) -- oldStop is only ever called AFTER that validation passes.
// This is a DELIBERATE deviation from epic-E5-plan.md's own Task 6 Step 3
// pseudocode sketch (which calls oldStop() unconditionally, first thing) --
// that ordering would retire a perfectly healthy OLD watcher for a switch
// attempt that then turns out to target a broken/empty directory, leaving
// the PO's live session without ANY watcher at all (an unnecessary,
// avoidable degradation the bean's own "ACHTUNG" callout asks to decide
// consciously). Validating first means a failed switch leaves the OLD
// watcher completely untouched -- still running, still live -- exactly the
// "session must not be destroyed" guarantee the bean demands.
// TestSwitchRepoCmdKeepsOldWatcherAliveOnValidationFailure is the regression
// guard for this exact ordering.
//
// B05 (data/watcher.go's own MANDATORY doc-stamp): calling oldStop()
// synchronously HERE is safe -- this whole function runs as a tea.Cmd, i.e.
// on a goroutine the bubbletea runtime spawns to evaluate the returned
// func() tea.Msg, which is NOT the watcher's own onChange goroutine B05
// forbids a synchronous stop from. oldStop()'s "blocks until the watcher
// goroutine has fully exited" contract (Watch's own doc comment) is exactly
// what guarantees NO event from the old repo can leak into the new watch
// (data.StartWatch below) that starts syncronously right after it on this
// same goroutine -- the whole point of doing this INSIDE a tea.Cmd (its own
// goroutine, decoupled from Update()'s single-threaded dispatch) rather than
// inline in a key handler, where a blocking oldStop() would freeze the
// entire UI for however long the watcher goroutine takes to unwind.
func switchRepoCmd(oldStop func(), newRepoDir string, storePath string, notify func()) tea.Cmd {
	return func() tea.Msg {
		client := &data.Client{RepoDir: newRepoDir, StorePath: storePath}
		beans, err := client.List()
		if err != nil {
			return repoSwitchedMsg{repoDir: newRepoDir, err: err}
		}

		// Validated -- safe to retire the old watcher now (design note
		// above).
		if oldStop != nil {
			oldStop()
		}

		var stop func()
		storeDir, watchErr := client.StoreDir()
		if watchErr == nil {
			stop, watchErr = data.StartWatch(storeDir, notify)
		}
		if watchErr != nil {
			// The switch itself still SUCCEEDED (client+beans are valid) --
			// degrade like app.go's own watchUnavailableMsg path instead of
			// failing the whole switch over a live-reload nicety.
			// applyRepoSwitched (update.go) surfaces this via the existing
			// m.watchUnavailable flag (I04 precedent), watchStop stays nil.
			stop = nil
		}
		return repoSwitchedMsg{client: client, repoDir: newRepoDir, beans: beans, watchStop: stop}
	}
}

// repoMetric is the Lobby's per-repo "Open/Total" figure (E5 Task 6, bean
// bt-zhwl design note: "Kosten/Latenz-Abwägung dokumentieren"). loaded=false
// (the zero value) means "no repoMetricsMsg has arrived for this repo yet"
// -- repoPickerBody (view_lobby.go) renders "…" for it; err != nil means
// THIS repo's own `beans list` call failed (e.g. a moved/deleted directory
// still listed in config.yaml) without blanking out every OTHER repo's
// already-loaded metric.
type repoMetric struct {
	open, total int
	err         error
	loaded      bool
}

// repoMetricsMsg carries ONE repo's metric result, tagged by repo path so
// applyRepoMetrics (update.go) can update just that ONE map entry --
// N independent messages (one per configured repo), not a single batched
// result, so a slow/broken repo never blocks the others from appearing as
// soon as they're ready (design note below, repoMetricsBatchCmd).
type repoMetricsMsg struct {
	repo        string
	open, total int
	err         error
}

// repoMetricsCmd loads ONE repo's full bean list and reduces it to an
// open/total count, tagged as repoMetricsMsg -- design note (bean bt-zhwl,
// "Kosten/Latenz-Abwägung"): this is genuinely a full `beans list --json
// --full` subprocess call per configured repo (same cost as loadCmd itself),
// so it is NEVER run synchronously in a loop (that would block the Lobby's
// own open transition for N subprocess round-trips, "Latenz-Gift" per the
// bean's own wording) -- always dispatched as its own independent tea.Cmd,
// batched via repoMetricsBatchCmd below.
func repoMetricsCmd(repo string, storePath string) tea.Cmd {
	return func() tea.Msg {
		c := &data.Client{RepoDir: repo, StorePath: storePath}
		beans, err := c.List()
		if err != nil {
			return repoMetricsMsg{repo: repo, err: err}
		}
		open := 0
		isOpen := data.OpenStatusFunc(repo)
		for _, b := range beans {
			if isOpen(b.Status) {
				open++
			}
		}
		return repoMetricsMsg{repo: repo, open: open, total: len(beans)}
	}
}

// repoMetricsBatchCmd dispatches repoMetricsCmd for EVERY configured repo at
// once, via tea.Batch (design note, bean bt-zhwl): bubbletea runs every
// Cmd in a batch concurrently on its own goroutine, so N configured repos
// cost one round-trip's worth of WALL-CLOCK latency, not N sequential
// round-trips -- "bei 2-5 konfigurierten Repos unkritisch" per the bean's
// own sizing note. Called both at Lobby-open time (openLobby,
// view_lobby.go) and from Init() when bt starts DIRECTLY into the Lobby
// (design decision d, app.go).
func repoMetricsBatchCmd(repos []string, storePaths map[string]string) tea.Cmd {
	cmds := make([]tea.Cmd, len(repos))
	for i, r := range repos {
		cmds[i] = repoMetricsCmd(r, storePaths[r])
	}
	return tea.Batch(cmds...)
}

// beanRawLoadedMsg carries data.Client.ShowRaw's async result (D01, design-
// spec.md §15 PF-17, bean bt-z4b1) -- openBeanEditor's FIRST Cmd-hop
// (editor.go): the raw markdown read must run as its own tea.Cmd BEFORE the
// tea.ExecProcess suspend can fire (a subprocess call, ~20-50ms, must never
// run synchronously inside Update). Update()'s beanRawLoadedMsg case
// (applyBeanRawLoaded, update.go) fires the ACTUAL editInEditor suspend on
// err==nil (the SECOND Cmd-hop) -- err != nil surfaces a toast and resets
// the editor-open state instead of ever suspending into $EDITOR on a read
// that never succeeded.
type beanRawLoadedMsg struct {
	id  string
	raw string
	err error
}

// showRawCmd runs data.Client.ShowRaw async, tagging the result as
// beanRawLoadedMsg (D01) -- id rides along so applyBeanRawLoaded can confirm
// it still matches m.editorTarget before suspending (defensive: nothing
// else can legitimately touch editorTarget between openBeanEditor and this
// Cmd resolving, since Update() only ever processes one Msg at a time, but
// the check costs nothing and guards against a stale load ever suspending
// into the WRONG bean's editor).
func showRawCmd(c *data.Client, id string) tea.Cmd {
	return func() tea.Msg {
		raw, err := c.ShowRaw(id)
		return beanRawLoadedMsg{id: id, raw: raw, err: err}
	}
}

// mutationDoneMsg carries any mutation's outcome (E3, bean bt-dlgk: the
// SHARED tail every Set*/Add*/Remove*/Delete mutation goes through -- no
// per-mutation Msg types). Success and failure BOTH trigger an unconditional
// reload (applyMutationResult, update.go): success must show the new state,
// an ErrConflict must resolve the now-stale index (design decision d).
type mutationDoneMsg struct{ err error }

// mutateCmd wraps a single mutation call (a data.Client Set*/Add*/Remove*/
// Delete method, already bound to its args via a closure) into the shared
// mutationDoneMsg Cmd -- every E3 overlay (Value-Menü T1, Tag-/Parent-/
// Blocking-Picker T2/T3, Delete T6) dispatches through this ONE producer.
func mutateCmd(fn func() error) tea.Cmd {
	return func() tea.Msg { return mutationDoneMsg{err: fn()} }
}

// createDoneMsg is the ONE exception to mutationDoneMsg (E3 Task 4, bean
// bt-y4ly): Create needs the newly minted bean back (for the post-create
// cursor jump), not just a bare error. Defined here in Task 1 alongside the
// rest of the shared mutation infra (plan »Task 1« Files list) -- Task 4
// wires the Update-dispatch case and the cursor-jump behavior once the
// Create form exists.
type createDoneMsg struct {
	bean data.Bean
	err  error
}

// createCmd runs data.Client.Create async, tagging the result as
// createDoneMsg.
func createCmd(c *data.Client, opts data.CreateOpts) tea.Cmd {
	return func() tea.Msg {
		b, err := c.Create(opts)
		return createDoneMsg{bean: b, err: err}
	}
}

// searchBleveResultMsg carries the result of an async data.Client.Search
// call (E2 Task 3, bean bt-4ep2), tagged with the query it answers. Update
// (applyBleveResult, update.go) discards it if m.searchPrefixRest (bt-2kfl
// D03: the query MINUS any typed `st:`/`ty:`/`pr:`/`tag:` prefix tokens,
// search_prefix.go) has moved on in the meantime -- staleness guard chosen
// over a debounce timer (E2 Task 3 commit rationale, keySearchInput/
// dispatchBleveIfDue doc comments): every qualifying (>=3 char) keystroke
// dispatches its own beans-CLI subprocess, but only the response matching
// the CURRENT rest text is ever applied.
type searchBleveResultMsg struct {
	query string
	ids   []string
	err   error
}

// searchCmd runs an async Bleve full-text search via data.Client.Search,
// tagging the result with query (design-spec.md §6 V2: "-S-Bleve-Modus ab 3
// Zeichen"). Only the resolved bean IDs are kept -- beanMatchesSearch
// (view_browse_repo.go) only ever needs ID membership, not the full Bean.
func searchCmd(c *data.Client, query string) tea.Cmd {
	return func() tea.Msg {
		beans, err := c.Search(query)
		if err != nil {
			return searchBleveResultMsg{query: query, err: err}
		}
		ids := make([]string, len(beans))
		for i, b := range beans {
			ids[i] = b.ID
		}
		return searchBleveResultMsg{query: query, ids: ids}
	}
}

// paletteBleveResultMsg/paletteSearchCmd (E4 Task 2, bean bt-yo60) -- the
// Command-Center's own Bleve staleness-guard for its former bean-search half
// -- were removed by B13 (design-spec.md §15 PF-16/"US-04-Revision", bean
// bt-ntoz, E8 Task 7, bean bt-yqdy): the Command-Center shows ONLY commands
// now, bean search is exclusively `/`'s job (searchBleveResultMsg/searchCmd
// above, UNTOUCHED).

// tagDefsSavedMsg carries a Tag-Registry SaveTagDefs write's outcome (E10
// Task 3, bean bt-604w, epic bt-362n D11/D14 Create -- also T4/T5's shared
// Delete/Rename write path). Deliberately its OWN Msg type, not the shared
// mutationDoneMsg (bean bt-604w's own wording: "hier gibt es kein m.idx zu
// invalidieren, nur die Registry") -- applyTagDefsSaved (update.go) does NOT
// run applyMutationResult's unconditional loadCmd reload, since a Tag-
// Registry write never touches any Bean (D11/D12) and therefore never stales
// m.idx. This is the SAME kind of deliberate exception createDoneMsg already
// is to mutationDoneMsg (messages.go doc-stamp above), one layer further:
// each mutation family gets its own Msg exactly when its OWN completion tail
// genuinely diverges from the shared one.
//
// refindName (E10 Task 4 Fix-Runde 1, bean bt-1lsu B01): the row name
// applyTagDefsSaved re-finds the cursor on after the rows rebuild -- passed
// EXPLICITLY by every dispatch site (Create: the new name; Delete: the
// deleted target, so the cursor follows a still-used tag into the Free
// group) instead of implicitly read from m.tagMgmtInput.Value() at apply
// time. The implicit read was safe while Create was the ONLY caller, but
// T4's Delete never touches the input field AND T3's esc-abort deliberately
// leaves the typed text in place -- an aborted Create followed by an
// unrelated Delete re-found the cursor on the STALE typed text (reviewer-
// verified repro, bean bt-1lsu Review-Findings Runde 1). A refindName with
// no matching row leaves the cursor where it was (same miss semantics the
// old name-search already had).
// successToast (E11 Item 6, bean bt-idm1): a non-empty string here has
// applyTagDefsSaved (update.go) fire an EXTRA toastInfo success Toast on a
// clean write -- Create/Rename/Delete all dispatch with this field left at
// its zero value "" (unchanged: those three stay silent on success, exactly
// TestApplyTagDefsSavedSuccessRefreshesRowsAndMovesCursor's own "no unconditional
// reload"/no-Cmd assertion still guards). Adopt (openTagMgmtAdopt) is the ONE
// dispatch site that sets it -- bt-ct3k's own Toast-Konsistenz wording: since
// that task just replaced a silent no-op with a Toast, a new silent success
// path here would be a regression in the other direction.
type tagDefsSavedMsg struct {
	err          error
	refindName   string
	successToast string
}

// saveTagDefsCmd wraps a single Tag-Registry SaveTagDefs write (a local,
// synchronous file write, data/tagdefs.go) into an async Cmd -- CONSISTENCY
// with every other state-changing call in this codebase demands a Cmd, never
// a direct call inside the Update path, even though the underlying I/O is
// fast enough it would not technically need one (mirrors mutateCmd's own
// build shape, messages.go above). refindName rides along untouched into
// tagDefsSavedMsg (B01, doc-stamp there); successToast likewise (E11 Item 6,
// doc-stamp on the struct above) -- every call site names its own, most
// callers passing "" (unchanged silent-success contract).
func saveTagDefsCmd(c *data.Client, defs []string, refindName, successToast string) tea.Cmd {
	return func() tea.Msg {
		return tagDefsSavedMsg{err: c.SaveTagDefs(defs), refindName: refindName, successToast: successToast}
	}
}

// tagRenameFailure records one bean's failed SetTags call during a Rename
// sweep (renameTagCmd below, E10 Task 5, bean bt-y9my, epic bt-362n D13) --
// id + the raw error, enough for a Toast's "first: <err>" summary
// (applyTagRenameDone, update.go) without a second lookup back into m.idx.
type tagRenameFailure struct {
	id  string
	err error
}

// tagRenameDoneMsg carries a Rename-sweep's outcome (D13) -- the SECOND
// deliberate exception to the shared mutationDoneMsg tail (tagDefsSavedMsg
// above was the first): a bulk, continue-on-error sweep across N beans needs
// richer feedback than a single error, the same "needs more than a bare
// error back" rationale createDoneMsg's own doc-stamp already establishes,
// one layer further. renamed/failed are populated by renameTagCmd's own
// continue-on-error loop.
//
// Ordering note (D13): the Registry rename (saveTagDefsCmd) and this Bean
// sweep (renameTagCmd) are dispatched as TWO INDEPENDENT Cmds in the SAME
// tea.Batch (keyTagMgmtInput's "rename" case, view_tag_management.go) --
// tea.Batch makes NO ordering guarantee between them, which is deliberately
// harmless: the two Cmds write disjoint state (applyTagDefsSaved only
// touches tagMgmtRows/the input sub-mode; applyTagRenameDone only touches
// m.idx/the Toast), so there is no Write-Write conflict regardless of which
// one lands first.
type tagRenameDoneMsg struct {
	oldTag, newTag string
	renamed        int
	failed         []tagRenameFailure
}

// renameTagCmd sweeps every bean CURRENTLY carrying oldTag (idx.WithTag,
// index.go -- an IN-MEMORY snapshot, so each bean's etag is read directly
// off idx's own Bean pointers, no m.beanETag redirect needed: this sweep
// operates on the exact idx handed to it at dispatch time) and fires ONE
// combined data.Client.SetTags(id, add=[newTag], remove=[oldTag], etag) call
// per bean (mutations.go's existing single-etag-no-cascade convention,
// reused verbatim -- no new Client method). CONTINUE-ON-ERROR (D13): a
// failure on one bean is collected into failed and the loop moves on to the
// NEXT bean, no early return -- beans has no cross-bean transaction
// (verified, epic bt-362n body's own empirical check against `beans update
// --help`), so a stale etag on bean K must never abort K+1..N.
//
// idx == nil degrades to a zero-value, no-op sweep (mirrors
// collectTagCounts' own "if idx != nil" defensive guard, box_picker_tag.go)
// -- a pre-load/test-fixture model (m.idx unset) must never panic here.
//
// oldTag == newTag guard: a resubmitted, UNCHANGED name (D14's dedupe
// exclusion of the rename's own old name, keyTagMgmtInput, deliberately lets
// this happen -- "eigener alter Name im Dedupe-Check durchgelassen") is
// treated as an ALREADY-COMPLETE no-op sweep: every bean currently carrying
// the tag counts as renamed WITHOUT ever calling SetTags. This is not just
// an optimization -- SetTags' own documented "the SAME tag in both add and
// remove -> remove wins" resolver (mutations.go, I2) would otherwise
// silently STRIP the tag from every one of these beans on a harmless
// re-confirm keystroke, a real, avoidable data-loss bug for this chain's
// riskiest (real-bean-mutating) task.
func renameTagCmd(c *data.Client, idx *data.Index, oldTag, newTag string) tea.Cmd {
	return func() tea.Msg {
		msg := tagRenameDoneMsg{oldTag: oldTag, newTag: newTag}
		if idx == nil {
			return msg
		}
		tagged := idx.WithTag(oldTag)
		if oldTag == newTag {
			msg.renamed = len(tagged)
			return msg
		}
		for _, b := range tagged {
			if err := c.SetTags(b.ID, []string{newTag}, []string{oldTag}, b.ETag); err != nil {
				msg.failed = append(msg.failed, tagRenameFailure{id: b.ID, err: err})
				continue
			}
			msg.renamed++
		}
		return msg
	}
}
