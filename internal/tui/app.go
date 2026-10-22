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
	pkgconfig "github.com/rierek/lif/pkg/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errNilStore is Q01's surfaced load error: a nil data.Store is a caller-side
// construction bug in every case EXCEPT the deliberate Lobby-first start
// (design decision d, bean bt-zhwl) -- Init() checks m.view == viewLobby
// FIRST specifically to keep those two nil-store cases distinct.
var errNilStore = errors.New("lif tui: nil store (Run() must open a data.Store before newModel)")

// Run starts the TUI. store/repoDir describe an already-opened lif store in
// the common case. store == nil starts DIRECTLY into the Lobby instead
// (design decision d, bean bt-zhwl): repoDir is then ignored, there is
// deliberately no repo to watch yet -- the first switchRepoCmd dispatch
// (keyLobby, view_lobby.go) supplies the first real store. settings is loaded
// by the CALLER (commands/tui.go), whose startup decision already needs
// settings.Repos.
//
// Run owns the store from here on: it closes whichever store is live when the
// program ends (the initial one, or the latest repo switch's).
//
// AltScreen + mouse (wheel/click) are enabled from the start.
func Run(store *data.Store, repoDir string, settings config.Settings) error {
	lipgloss.SetHasDarkBackground(true)

	configuredEditor = settings.Editor     // design decision c: Settings > $VISUAL > $EDITOR > vi (editor.go)
	theme.SetAccent(settings.Theme.Accent) // No-Op on empty/invalid (theme.go's own guard) -- built-in Mauve stays by default

	if store != nil {
		applyRepoConfig(store.Config()) // the project's value lists and colours, before the first index is built
	}
	m := newModel(store, repoDir)
	m.settings = settings
	if store == nil {
		// Lobby-first (design decision d): no repo resolved. repoList is
		// primed here so the Lobby's very first frame already shows every
		// configured repo.
		m.view = viewLobby
		m.repoList.setLen(len(settings.Repos))
	} else {
		var err error
		m.storeEvents, err = store.Watch()
		m.watchUnavailable = err != nil // degrade visibly, never silently (I04)
	}

	finalModel, runErr := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if fm, ok := finalModel.(model); ok && fm.store != nil {
		fm.store.Close()
	}
	return runErr
}

// Init kicks off the initial async load and, with a store, the wait for its
// first change event.
func (m model) Init() tea.Cmd {
	if m.view == viewLobby {
		// Lobby-first start: m.store is nil BY DESIGN here (Run() above), not
		// the Q01 caller bug the guard below exists for -- kick off the repo
		// metrics batch instead of an item load.
		return repoMetricsBatchCmd(m.settings.Repos, m.settings.RepoStorePaths)
	}
	if m.store == nil { // Q01: turn a would-be nil-deref panic into a status-line load error
		return func() tea.Msg { return beansLoadedMsg{err: errNilStore} }
	}
	return tea.Batch(loadCmd(m.store), waitForStoreEvents(m.storeEvents))
}

// applyRepoConfig makes cfg the TUI's source for status/type/priority lists,
// type ranks and explicitly configured colours. display.theme is NOT applied
// yet: the TUI palette is fixed (dark, Catppuccin Macchiato).
func applyRepoConfig(cfg *pkgconfig.Config) {
	data.SetConfig(cfg)
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
