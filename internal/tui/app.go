package tui

// app.go — model constructor + program runner (T8 App-Shell keystone), ported
// architecture-wise from devd (~/dev/DeveloperDashboard/apps/
// cli-go/internal/tui/app.go): AltScreen + mouse cell-motion, dark-background
// forced (huh/lipgloss adaptive-color detection over ssh/tmux is unreliable —
// this app is always dark, see devd DD2-24).

import (
	"errors"

	"github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
	"github.com/rierek/lif/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errNilClient is Q01's (bean bt-7jr8, T8-review) surfaced load error: a nil
// data.Client is a caller-side construction bug (Run() must build one before
// newModel) in every case EXCEPT the deliberate E5 Task 6 Lobby-first start
// (design decision d, bean bt-zhwl) -- Init() checks m.view == viewLobby
// FIRST specifically to keep those two nil-client cases distinct, see
// Init() below.
var errNilClient = errors.New("lif tui: nil client (Run() must construct a data.Client before newModel)")

// activeProgram is the currently running *tea.Program (E5 Task 6, bean
// bt-zhwl) -- set in Run() immediately after tea.NewProgram, strictly
// BEFORE p.Run() is ever called. Exists so switchRepoCmd's notify closure
// (built in keyLobby/view_lobby.go, at a point in time where a keypress has
// already reached the running program -- i.e. long after this assignment)
// can reach the SAME *tea.Program the initial watchMsg closure already
// captures directly by value, without threading p itself through
// model/Update. A package-level var is safe here specifically because
// nothing else that could invoke it (switchRepoCmd's notify is only ever
// invoked from a watcher goroutine started by a repo switch, itself only
// reachable from a keypress) can race the assignment -- unlike the
// INITIAL watch below, which is deliberately NOT wired through
// activeProgram (see initialWatchMsg's own doc-stamp, messages.go, for the
// startup race this sidesteps by keeping the original p.Send(watchMsg{})
// closure capture instead).
var activeProgram *tea.Program

// Run starts the bt TUI. client/repoDir describe an already-resolved beans
// repo (data.FindRepo) in the common case. client == nil starts DIRECTLY
// into the Lobby instead (E5 Task 6, design decision d, bean bt-zhwl):
// repoDir is then ignored, there is deliberately no repo to watch yet -- the
// PO's first switchRepoCmd dispatch (keyLobby, view_lobby.go) supplies the
// first real client/watcher. settings is loaded by the CALLER (cmd/tui.go
// RunE) since design decision d's own trigger logic already needs
// settings.Repos to decide whether Run is even called with client == nil in
// the first place -- Run no longer loads it a second time itself (a T5-era
// ERRATUM this task closes: Run and cmd/tui.go would otherwise each call
// config.LoadSettings() independently for the SAME startup, redundant and a
// second place the "corrupt config.yaml -> Defaults, never crash" fallback
// would need to stay in lockstep).
//
// AltScreen + mouse (wheel/click) are enabled from the start (design-
// spec.md §9: "Maus (Wheel, Klick-Cursor)" is in v1 scope, T8 only reserved
// the flag). E5 Task 4 (bean bt-mne6) wires the actual handling --
// tea.WithMouseCellMotion() itself is UNCHANGED since T8, just no longer a
// reservation: every tea.MouseMsg it now emits reaches Update()'s own case
// (update.go) -> handleMouse (mouse.go).
func Run(client *data.Client, repoDir string, settings config.Settings) error {
	lipgloss.SetHasDarkBackground(true)

	configuredEditor = settings.Editor     // design decision c: Settings > $VISUAL > $EDITOR > vi (editor.go)
	theme.SetAccent(settings.Theme.Accent) // No-Op on empty/invalid (theme.go's own guard) -- built-in Mauve stays by default

	if client != nil {
		applyRepoConfig(repoDir) // the project's value lists and colours, before the first index is built
	}
	m := newModel(client, repoDir)
	m.settings = settings
	if client == nil {
		// E5 Task 6 (design decision d): no repo resolved -- start directly
		// in the Lobby instead of the (would-be nil-client) Browse view.
		// repoList primed here (not left to Init/the first render) so the
		// Lobby's very first frame already shows every configured repo,
		// same "open transition primes the cursor" precedent as
		// openLobby/openPalette (view_lobby.go/overlay_palette.go).
		m.view = viewLobby
		m.repoList.setLen(len(settings.Repos))
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	// Set BEFORE any watch (initial or switch-driven) can possibly fire a
	// notify closure that reads it -- see activeProgram's own doc-stamp
	// above for why this ordering is the whole safety argument.
	activeProgram = p

	if client != nil {
		// B05 (MANDATORY, bean bt-7jr8): data.Watch's onChange callback runs
		// on the watcher's own goroutine and must NEVER call stop()
		// synchronously from inside it (that would deadlock -- stop() blocks
		// until the watcher goroutine exits, which is the very goroutine
		// invoking onChange). This consumer only ever hands the running
		// program an async tea.Msg via p.Send -- documented safe to call
		// from any goroutine -- and never touches stop from onChange.
		storeDir, watchErr := client.StoreDir()
		var stop func()
		if watchErr == nil {
			stop, watchErr = data.Watch(storeDir, func() {
				p.Send(watchMsg{})
			})
		}
		if watchErr == nil {
			// E5 Task 6 (bean bt-zhwl): hand this stop func to the MODEL
			// (not just a local Run() variable, T8-era behavior) so the
			// PO's first repo switch can retire it (initialWatchMsg's own
			// doc-stamp, messages.go, has the full leak-prevention
			// rationale). Sent asynchronously via a goroutine for the exact
			// same reason watchUnavailableMsg below already does -- p.Send
			// blocks until p.Run()'s event loop starts reading, which only
			// happens once this function reaches its own p.Run() call.
			go p.Send(initialWatchMsg{stop: stop})
		} else {
			// I04 (T8 Opus quality review): don't just swallow the error --
			// tell the model so it can surface it in the status line.
			go p.Send(watchUnavailableMsg{})
		}
	}

	// E5 Task 6 (bean bt-zhwl): capture the FINAL model p.Run() hands back
	// (T8-era code discarded it) so whichever watcher is CURRENTLY live at
	// quit time -- the initial one, or the latest repo switch's -- gets
	// retired exactly once. This REPLACES the T8-era `defer stop()` local-
	// variable pattern entirely: that pattern only ever knew about the
	// INITIAL watcher, so a session that switched repos before quitting
	// would have leaked whatever watcher was actually running by then.
	finalModel, runErr := p.Run()
	if fm, ok := finalModel.(model); ok && fm.watchStop != nil {
		fm.watchStop()
	}
	return runErr
}

// Init kicks off the initial async load (spinnerless, per plan scope).
func (m model) Init() tea.Cmd {
	if m.view == viewLobby {
		// E5 Task 6 (design decision d): starting directly into the Lobby
		// means m.client is nil BY DESIGN here (Run() above), not the Q01
		// caller-bug the guard below exists for -- kick off the repo
		// metrics batch instead of a bean-list load (there is no repo to
		// list yet).
		return repoMetricsBatchCmd(m.settings.Repos, m.settings.RepoStorePaths)
	}
	if m.client == nil { // Q01 (bean bt-7jr8 T8-review): the nil-client invariant is
		// otherwise only enforced by convention at Run()'s call site -- this guard
		// turns a would-be nil-deref panic (inside loadCmd -> Client.List -> run)
		// into a normal, status-line-surfaced load error instead.
		return func() tea.Msg { return beansLoadedMsg{err: errNilClient} }
	}
	return loadCmd(m.client)
}

// applyRepoConfig makes repoDir's project config the TUI's source for
// status/type/priority lists, type ranks and explicitly configured colours.
// display.theme is NOT applied: the TUI palette is fixed (dark, Catppuccin
// Macchiato), see wiki settings-and-themes.
func applyRepoConfig(repoDir string) {
	data.ApplyRepoConfig(repoDir)
	resolve := func(specs map[string]string) map[string]lipgloss.Color {
		out := make(map[string]lipgloss.Color, len(specs))
		for name, spec := range specs {
			out[name] = ui.ResolveColor(spec)
		}
		return out
	}
	s, t, p := data.ConfiguredColors()
	theme.SetConfiguredColors(resolve(s), resolve(t), resolve(p))
}
