package tui

// app.go — model constructor + program runner (T8 App-Shell keystone), ported
// architecture-wise from devd (~/dev/DeveloperDashboard/apps/
// cli-go/internal/tui/app.go): AltScreen + mouse cell-motion, background
// darkness forced from display.theme (huh/lipgloss adaptive-color detection
// over ssh/tmux is unreliable, see devd DD2-24; applyRepoConfig).

import (
	"errors"

	"github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/internal/uiconfig"
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
	configuredEditor = settings.Editor // design decision c: Settings > $VISUAL > $EDITOR > vi (editor.go)
	configuredAccent = settings.Theme.Accent

	var cfg *pkgconfig.Config // nil in the Lobby: built-in defaults
	if store != nil {
		cfg = store.Config()
	}
	applyRepoConfig(cfg) // theme, value lists and colours, before the first index is built
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

// configuredAccent is Settings.Theme.Accent; applyRepoConfig re-applies it
// after every theme change, since theme.Apply resets the accent styles.
var configuredAccent string

// applyRepoConfig makes cfg the TUI's source for status/type/priority lists,
// type ranks, glyphs, colours and the display.theme palette. cfg == nil means
// the built-in defaults (Lobby).
func applyRepoConfig(cfg *pkgconfig.Config) {
	data.SetConfig(cfg)
	uiconfig.Apply(cfg)
	theme.Apply(ui.ActiveTheme())
	lipgloss.SetHasDarkBackground(ui.ActiveTheme().Name != "latte")
	if configuredAccent != "" {
		theme.SetAccent(configuredAccent)
	}
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
