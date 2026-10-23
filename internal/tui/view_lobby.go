package tui

// view_lobby.go — E5 Task 6 (bean bt-zhwl, epic bt-5h4d, design-spec.md §6
// V1): the Lobby/Repo-Picker. Port devd view_home.go
// (~/dev/DeveloperDashboard/apps/cli-go/internal/tui/
// view_home.go) STRUCTURALLY -- centerInto/pickerRowFill are VERBATIM string
// utilities (no devd-API coupling to strip), homeLogoBlock/repoPickerBody/
// viewLobby are beans-tui-native (a "lif" banner instead of "DevDashboard",
// beans-tui's own config.Settings.Repos instead of devd's api.Project list,
// an Open/Total bean-count metric instead of devd's Sprint/Ready count).
//
// KEIN Overlay: viewLobby is a full top-level view (design decision, bean's
// own body text) -- a repo switch needs a complete client/watcher restart,
// which is not something a floating modal over the PREVIOUS repo's state
// should represent. handleKey's capture order (update.go) reflects this:
// m.view == viewLobby is checked as its OWN full-capture state, positioned
// AHEAD of the keys.Picker bare-match check (see that check's own doc-stamp
// for the full ERRATUM-vs-bt-0l8c-note rationale) so the Lobby's own
// repoQuery filter can accept "p" as an ordinary typed character.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// homeLogoLines is a pure-ASCII "lif" banner (figlet "Standard"-style),
// deliberately narrow EAW-neutral glyphs only (no East-Asian-Ambiguous
// drift, same rationale as devd's own homeLogoLines doc-stamp) -- terminal-
// independent, stable width.
var homeLogoLines = []string{
	` _ _  __ `,
	`| (_)/ _|`,
	`| | | |_ `,
	`| | |  _|`,
	`|_|_|_|  `,
}

// centerInto sets s exactly centered in a field of width w (left+right
// padded -> the resulting line is exactly w cells wide). ANSI-safe width via
// lipgloss.Width. Port devd view_home.go's centerInto VERBATIM.
func centerInto(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	left := (w - sw) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-sw-left)
}

// pickerRowFill right-aligns the right column to w cells (left is never
// truncated here -- beans-tui's own repo rows stay short enough that this
// never matters in practice, unlike devd's Project-name column). Port devd
// view_home.go's pickerRowFill VERBATIM.
func pickerRowFill(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// homeLogoBlock renders the Mauve "lif" banner as UNIFORM-width lines
// (each padded to the banner's own max width, so the block stays internally
// flush -- Port devd homeLogoBlock's own padding rationale). A narrow
// terminal falls back to a plain Header-styled "lif tui" title instead of
// the full banner.
func homeLogoBlock(width int) []string {
	lw := 0
	for _, l := range homeLogoLines {
		if x := lipgloss.Width(l); x > lw {
			lw = x
		}
	}
	if width < lw+2 {
		return []string{theme.Header.Render("lif tui")}
	}
	style := lipgloss.NewStyle().Foreground(theme.Mauve)
	out := make([]string, len(homeLogoLines))
	for i, l := range homeLogoLines {
		padded := l + strings.Repeat(" ", lw-lipgloss.Width(l))
		out[i] = style.Render(padded)
	}
	return out
}

// filteredRepos returns m.settings.Repos narrowed by a case-insensitive
// substring match against m.repoQuery -- an empty query returns the full
// list unchanged (the Lobby's whole PURPOSE is showing the configured list,
// so an empty filter must show everything, not nothing -- unlike the
// Command-Center's own empty-query contract, overlay_palette.go's
// palFiltered, which returns every ACTION but never dumps a bean list at
// all, B13).
func (m model) filteredRepos() []string {
	q := strings.ToLower(strings.TrimSpace(m.repoQuery))
	if q == "" {
		return m.settings.Repos
	}
	var out []string
	for _, r := range m.settings.Repos {
		if strings.Contains(strings.ToLower(r), q) {
			out = append(out, r)
		}
	}
	return out
}

// repoMetricLabel renders repo's Open/Total column: "…" while
// repoMetricsMsg hasn't arrived yet (repoMetricsBatchCmd, messages.go, is
// always async -- design note on cost/latency), "err" on a per-repo load
// failure (a single broken repo must not blank the whole column, see
// repoMetric's own doc-stamp), "open/total" otherwise.
func (m model) repoMetricLabel(repo string) string {
	rm, ok := m.repoMetrics[repo]
	if !ok || !rm.loaded {
		return "…"
	}
	if rm.err != nil {
		return "err"
	}
	return fmt.Sprintf("%d/%d", rm.open, rm.total)
}

// repoPickerWidth caps the Lobby's own table width -- wide, but capped, so
// the centered block never sprawls across a very wide terminal (Port devd
// pickerBodyWidth's own rationale, view_home.go) -- but NEVER wider than
// width itself (B01, same doc-stamp as viewLobby's own -- found alongside
// it during the tmux smoke test): devd's own floor (30) assumes a terminal
// wide enough to always afford it, which does not hold once beans-tui's
// caller passes the ALREADY border-budgeted innerW (viewLobby, below) at a
// genuinely narrow terminal. Handing repoPickerBody a table width WIDER
// than the outer frame's own innerW made every row overflow it -- and
// outerBorder's lipgloss.Width()-styled Render silently WORD-WRAPS any
// line wider than its own Width() instead of erroring, turning a width bug
// into extra, unbudgeted LINES (the actual symptom that was visible live:
// a frame taller than m.height). The floor is still tried first (nicer
// layout on most terminals), but width is now the hard ceiling.
func repoPickerWidth(width int) int {
	w := width - 8
	if w > 72 {
		w = 72
	}
	if w < 30 {
		w = 30
	}
	if w > width {
		w = width
	}
	if w < 1 {
		w = 1
	}
	return w
}

// repoPickerBody renders the search line + header + filtered repo rows as
// an aligned table (Port devd projectPickerBody's own structure,
// view_home.go) -- w is the table's content width.
func (m model) repoPickerBody(w int) string {
	ti := m.repoSearch
	if m.repoQuery != "" {
		ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Red)
	} else {
		ti.TextStyle = lipgloss.NewStyle().Foreground(theme.Text)
	}
	var b strings.Builder
	b.WriteString(theme.Muted.Render("⌕ ") + ti.View() + "\n\n")
	b.WriteString(pickerRowFill(theme.Dim.Render("  ◦ Repo"), theme.Dim.Render("open/total"), w) + "\n")
	b.WriteString(theme.Dim.Render(strings.Repeat("─", w)) + "\n")

	filtered := m.filteredRepos()
	for i, r := range filtered {
		cursor := "  "
		nameStyle := lipgloss.NewStyle().Foreground(theme.Text)
		if i == m.repoList.cursor {
			cursor = theme.Accent.Render("▸ ")
			nameStyle = theme.Header
		}
		metric := theme.Muted.Render(m.repoMetricLabel(r))
		nameW := w - lipgloss.Width(cursor) - lipgloss.Width(metric) - 1
		if nameW < 8 {
			nameW = 8
		}
		// bt-d3ps (epic-E13-plan.md Item 4, PO-Redefinition Grilling
		// 2026-07-17): label is "slug — path", not the raw path alone --
		// data.RepoSlug reads .beans.yml's beans.prefix (Dir-Basename
		// fallback). ONE combined truncate call (not slug/path budgeted
		// separately) -- ansi.Truncate cuts from the right, so on a narrow
		// terminal the SLUG (short, always first) stays visible and the
		// PATH (long, always last) is what gets clipped -- the more useful
		// priority, since the slug alone already disambiguates the row.
		label := data.RepoSlug(r) + " — " + r
		left := cursor + nameStyle.Render(truncate(label, nameW))
		b.WriteString(pickerRowFill(left, metric, w) + "\n")
	}
	if len(filtered) == 0 && len(m.settings.Repos) > 0 {
		b.WriteString(theme.Muted.Render("  (no matches)") + "\n")
	} else if len(m.settings.Repos) == 0 {
		b.WriteString(theme.Muted.Render("  (no repos in config.yaml -- ctrl+k -> settings)") + "\n")
	}
	return b.String()
}

// viewLobby renders the Lobby: banner + subtitle + repo table as ONE
// centered block (Port devd viewHome's own centering algebra, view_home.go)
// -- every line centered into the block's own content width (the widest of
// banner/table), placed via lipgloss.Place, wrapped in the app's outer
// frame. composeOverlays still runs at the end (same precedent as every
// other view -- confirmQuit must still be able to float over the Lobby,
// e.g. q/esc from a Lobby with no live repo yet).
func (m model) viewLobby() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	// B01 (found during this task's own tmux smoke test, step (e)):
	// content must be budgeted to innerW x innerH BEFORE outerBorder wraps
	// it -- outerBorder's own doc comment (view.go) is explicit that its
	// width param is "the CONTENT width the border wraps around ... pass
	// innerW ... or the frame ends up o.Width+2 wide instead of o.Width",
	// and the SAME +2 arithmetic applies to height (RoundedBorder adds a
	// top+bottom row). The first cut of this function passed the FULL w/h
	// straight into both lipgloss.Place and outerBorder -- the rendered
	// frame ended up h+2 rows tall / w+2 cols wide, silently overflowing
	// the terminal's real m.height/m.width by 2 in each dimension. Nothing
	// caught this in unit tests (View() output is just a string, never
	// diffed against m.height/m.width there) -- only became visible live,
	// in tmux, as renderToast's top-right Toast (which DOES compose
	// directly against the real m.width/m.height, this file's own
	// overlay_show_toast.go doc-stamp) appearing pushed almost entirely
	// off the top of the screen, just its bottom border edge visible.
	// Every OTHER view (viewBrowseRepo/viewReady) already gets this right
	// via the identical innerW := w - 2 pattern -- this now matches that
	// established convention exactly, no new arithmetic invented.
	innerW := w - 2
	innerH := h - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	lines := append([]string{}, homeLogoBlock(innerW)...)
	lines = append(lines, "", theme.Muted.Render("Repo-Picker"), "")
	lines = append(lines, strings.Split(m.repoPickerBody(repoPickerWidth(innerW)), "\n")...)

	cw := 0
	for _, l := range lines {
		if x := lipgloss.Width(l); x > cw {
			cw = x
		}
	}
	if cw > innerW {
		cw = innerW
	}
	for i := range lines {
		lines[i] = centerInto(lines[i], cw)
	}
	body := strings.Join(lines, "\n")

	// truncate BEFORE centerInto (never after -- B01 doc-stamp above): the
	// hint's own reserved budget is exactly ONE line (bodyH below already
	// subtracts it), and unlike the table rows (already width-controlled
	// via repoPickerWidth's own fix), this literal string has no natural
	// truncation point of its own -- a narrow terminal must cut it, not
	// silently word-wrap it into a second, unbudgeted line.
	hintText := truncate("↑↓:move  enter:open  type:filter  "+lobbyExitHint(m), innerW)
	hint := theme.Muted.Render(centerInto(hintText, innerW))
	bodyH := innerH - 1 // innerH total inner lines, minus the 1 reserved for hint below
	if bodyH < 1 {
		bodyH = 1
	}
	placed := lipgloss.Place(innerW, bodyH, lipgloss.Center, lipgloss.Center, body)
	out := outerBorder(placed+"\n"+hint, innerW, true)

	return m.composeOverlays(out, w, h)
}

// lobbyExitHint renders the exit-key segment of the Lobby's footer hint.
// Since the B01 fix (bt-1u0t Fix-Runde 1) decoupled keyLobby's exit keys,
// esc and q DIVERGE once a live repo exists (esc goes back, q quit-confirms)
// -- the pre-fix combined "esc/q:back" label would promise that q takes you
// back too, exactly the surprise-copy problem quitBox's own context-
// sensitive hint (B08 Planner add-on) exists to prevent. With no client
// both keys still quit-confirm (first-screen Lobby, design decision d), so
// the combined label stays there.
//
// D06 OPTIK-ANGLEICHUNG (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc,
// Notes-für-bt-d8kc in bean bt-1u0t): the ":" format is replaced with the
// same Teal-key/Subtext-desc color split renderBindings uses (theme.
// BindingKey/BindingDesc directly -- there is no keybind.Binding to hand
// renderBindings here, esc/q/esc-q are ad hoc label pairs, not real
// bindings). WHICH text shows (esc:back+q:quit vs. the combined esc/q:quit)
// is UNCHANGED -- only the optic. Scoped to lobbyExitHint's own segment
// only: the surrounding hint line ("↑↓:move  enter:open  type:filter  ",
// viewLobby) keeps its own colon format, out of this task's scope.
func lobbyExitHint(m model) string {
	if m.store != nil {
		return theme.BindingKey.Render("esc") + " " + theme.BindingDesc.Render("back") + "  " +
			theme.BindingKey.Render("q") + " " + theme.BindingDesc.Render("quit")
	}
	return theme.BindingKey.Render("esc/q") + " " + theme.BindingDesc.Render("quit")
}

// openLobby transitions into the Lobby (E5 Task 6, design decision h):
// reachable both as the Lobby's OWN startup entry (app.go Run/Init, design
// decision d) and via keys.Picker from any live view (update.go handleKey).
// Reloads Settings from disk FIRST (bean bt-zhwl's own acceptance wording:
// "lädt Settings.Repos neu, falls seit Start geändert") -- Run() only ever
// loads config.yaml ONCE at process start, so without this reload here, a
// PO who edits config.yaml's repos list (by hand, or via the Settings-Form,
// E5 Task 5) mid-session would keep seeing the STALE list every time they
// reopen the Lobby for the rest of the process lifetime (caught live during
// this task's own tmux smoke test, step (e) -- not a hypothetical). Mirrors
// Run()'s own "corrupt/missing config.yaml -> Defaults, never crash"
// fallback (app.go) rather than introducing a second, divergent error
// policy. Resets the filter/cursor (mirrors devd's openProjPick/keyHome-
// into-fresh-state precedent) and (re)dispatches the async metrics batch --
// a stale metrics map entry for a since-removed repo is harmless
// (repoPickerBody only ever reads entries for repos CURRENTLY in
// filteredRepos()) and a newly added repo simply shows "…" until its own
// repoMetricsMsg arrives.
func (m model) openLobby() (tea.Model, tea.Cmd) {
	m.view = viewLobby
	if fresh, err := config.LoadSettings(); err == nil {
		m.settings = fresh
	} else {
		m.settings = config.DefaultSettings()
	}
	m.repoQuery = ""
	m.repoSearch.SetValue("")
	m.repoSearch.Focus() // Port devd openProjPick/keyHome's own "always focused" Lobby-search precedent
	m.repoList = listState{}
	m.repoList.setLen(len(m.settings.Repos))
	return m, repoMetricsBatchCmd(m.settings.Repos, m.settings.RepoStorePaths)
}

// keyLobby drives the Lobby (Port devd keyHome's own structure,
// view_home.go): nav + selection dispatches switchRepoCmd (the watcher-
// lifecycle switch, messages.go); the three exit keys are DECOUPLED since
// the B01 fix (bt-1u0t Fix-Runde 1): esc returns to the live repo if one
// exists or quit-confirms otherwise (US-01 parity doc-stamp, lobbyExitHint
// above), q ALWAYS quit-confirms (B08 stage 2 must stay reachable), ctrl+c
// ALWAYS quits immediately (bt-7jr8 kill-switch contract); any other key
// filters repoQuery live.
func (m model) keyLobby(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// bt-l8e7 fix (E12 Item 3): intercept the RAW tea.KeyUp/tea.KeyDown
	// KeyType here, NOT navKey()'s letter-alias table (keys.Up binds "i",
	// keys.Down binds "k", vim-style) -- mirrors keyTagPicker's own raw-
	// KeyType intercept (box_picker_tag.go, bt-9ipw), which exists for the
	// exact same reason: a repoQuery starting with "i"/"k" (e.g. "ide") must
	// reach the textinput below untouched, not be swallowed as navigation.
	switch msg.Type {
	case tea.KeyUp:
		m.repoList.move(-1)
		return m, nil
	case tea.KeyDown:
		m.repoList.setLen(len(m.filteredRepos()))
		m.repoList.move(1)
		return m, nil
	}
	switch msg.String() {
	case "enter":
		filtered := m.filteredRepos()
		if m.repoList.cursor < 0 || m.repoList.cursor >= len(filtered) {
			return m, nil
		}
		target := filtered[m.repoList.cursor]
		return m, switchRepoCmd(m.store, target, m.settings.RepoStorePaths[target])
	case "esc":
		if m.store != nil {
			// A live repo already exists (p from a running session) -- go
			// back to it instead of force-quitting a working session
			// (design decision, bean bt-zhwl Step 9: devd's own keyHome has
			// no equivalent case since devd's Home is its ONLY entry point;
			// beans-tui's Lobby is reachable a SECOND way). D03: esc goes
			// exactly ONE level back -- the Lobby was a side trip.
			m.view = viewBrowseRepo
			return m, nil
		}
		return m.requestQuit()
	case "q":
		// B01 fix (bt-1u0t Fix-Runde 1): q quit-confirms REGARDLESS of
		// m.store -- the pre-fix uniform esc/q/ctrl+c case made stage 2 of
		// the B08 quit cascade unreachable once a repo had ever been opened
		// (q always bounced back to Browse), contradicting the PO wording
		// (bt-ntoz B08: "aus der Lobby q→enter beendet die TUI").
		// keyConfirmQuit's quitBoxWillGoToLobby() is already false here
		// (m.view == viewLobby), so the confirm's hint reads "enter: quit"
		// and enter resolves to tea.Quit -- stage 2 complete.
		return m.requestQuit()
	}

	// Port devd projFilterType's own Focus()-before-Update() precedent
	// (view_home.go): bubbles' textinput.Model.Update ignores key input
	// entirely while unfocused, and the Lobby -- unlike the Tree's `/`
	// search -- has no separate "not yet typing" mode (every non-nav/
	// non-enter/non-esc key IS the filter, mirrors devd's own keyHome/
	// projFilterType structure). Idempotent, safe to call every keystroke.
	m.repoSearch.Focus()
	var cmd tea.Cmd
	m.repoSearch, cmd = m.repoSearch.Update(msg)
	prev := m.repoQuery
	m.repoQuery = strings.TrimSpace(m.repoSearch.Value())
	if m.repoQuery != prev {
		m.repoList.cursor = 0
		m.repoList.setLen(len(m.filteredRepos()))
	}
	return m, cmd
}
