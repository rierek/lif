package tui

// view_waves.go — the Waves view (bean beans-wrj0, epic beans-1p2g): the
// direct children of one root item as the board the waves package lays out,
// with spatial focus movement (navKey), a breadcrumb bar with level picker,
// the detail panel, and edge mode (`b`) that writes one blocked_by edit.
//
// The view is a full-capture view like Lobby and Tag management: handleKey
// routes every key here except the global ones wavesPassthrough lists. It
// writes item data only on the second `b` (R-19); everything else only moves
// state in m.waves. The board is rebuilt after every reload (wavesReload).

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
	"github.com/rierek/lif/internal/tui/waves"
	"github.com/rierek/lif/pkg/item"
)

// wavesState is the Waves view's state. The zero value of critOff means the
// critical path is shown, as R-29 requires when the view opens.
type wavesState struct {
	root, focus, mark string
	board             *waves.Board
	critOff           bool
	detail            bool // Enter: detail panel of the focus item instead of the board

	picking bool // Tab: level picker open
	pickIDs []string
	pick    listState

	// origin is the view (and flat flag) the view was opened from; esc and a
	// vanished root return there.
	origin     viewID
	originFlat bool
}

// openWaves opens the Waves view with root as the root item and the focus on
// the first card of the leftmost column. From inside the view it re-roots and
// keeps the origin.
func (m model) openWaves(root string) model {
	st := wavesState{origin: m.view, originFlat: m.flatView}
	if m.view == viewWaves {
		st.origin, st.originFlat = m.waves.origin, m.waves.originFlat
		st.critOff = m.waves.critOff
	}
	m.waves = st
	m.view = viewWaves
	return m.wavesSetRoot(root, "")
}

// wavesSetRoot builds the board of root and focuses focus, or the first card
// of the leftmost column when focus is not on the board. It clears the mark.
func (m model) wavesSetRoot(root, focus string) model {
	m.waves.root, m.waves.mark = root, ""
	m.waves.board = waves.Build(m.idx, root)
	return m.wavesClampFocus(focus)
}

func (m model) wavesClampFocus(focus string) model {
	if _, _, ok := m.waves.board.Pos(focus); !ok {
		focus = wavesFirstCard(m.waves.board)
	}
	m.waves.focus = focus
	return m
}

func wavesFirstCard(b *waves.Board) string {
	if len(b.Columns) > 0 && len(b.Columns[0]) > 0 {
		return b.Columns[0][0]
	}
	return ""
}

// wavesReload rebuilds the board after items were reloaded (applyLoaded): root
// and focus ID stay; a mark that left the board is dropped; a vanished root
// returns to the origin view.
func (m model) wavesReload() model {
	if _, ok := m.idx.ByID[m.waves.root]; !ok {
		return m.wavesExit()
	}
	mark := m.waves.mark
	m = m.wavesSetRoot(m.waves.root, m.waves.focus)
	if _, _, ok := m.waves.board.Pos(mark); ok {
		m.waves.mark = mark
	}
	return m
}

func (m model) wavesExit() model {
	origin, flat := m.waves.origin, m.waves.originFlat
	m.waves = wavesState{}
	return m.showView(origin, flat)
}

// wavesFocusItem is the item of the focus card, nil when the board is empty.
func (m model) wavesFocusItem() *item.Item {
	if m.idx == nil {
		return nil
	}
	return m.idx.ByID[m.waves.focus]
}

// wavesPassthrough lists the keys that keep their global meaning inside the
// Waves view (R-29, R-15): help, command palette, quit, lobby, reload and the
// three view switches. The level picker captures everything.
func (m model) wavesPassthrough(msg tea.KeyMsg) bool {
	if m.waves.picking {
		return false
	}
	for _, b := range []keybind.Binding{keys.Palette, keys.Help, keys.Quit, keys.Picker, keys.Refresh, keys.Roadmap, keys.FlatList, keys.Ready} {
		if keybind.Matches(msg, b) {
			return true
		}
	}
	return false
}

// keyWaves handles every key inside the Waves view that is not passed
// through. Every unknown key is a handled no-op, so `c`, `b`, `W` and the
// other global keys have only their Waves meaning here.
func (m model) keyWaves(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.waves.picking:
		return m.keyWavesPicker(msg)
	case m.waves.detail:
		return m.keyWavesDetail(msg)
	}
	b := m.waves.board
	focus := m.wavesFocusItem()
	if dir := navKey(msg.String()); dir == "up" || dir == "down" || dir == "left" || dir == "right" {
		m.waves.focus = wavesMove(b, m.waves.focus, dir)
		return m, nil
	}
	switch {
	case keybind.Matches(msg, keys.Back):
		switch {
		case m.waves.mark != "":
			m.waves.mark = ""
		case m.toast != nil && m.toast.sticky:
			m.toast = nil
		default:
			m = m.wavesExit()
		}
		return m, nil
	case keybind.Matches(msg, keys.Enter):
		if focus != nil {
			m.waves.detail = true
			m.boxFormCursor, m.boxFormCursorBean = 0, ""
		}
		return m, nil
	case msg.String() == "o":
		return m.wavesGoInto(focus)
	case msg.Type == tea.KeyBackspace:
		return m.wavesGoUp(), nil
	case msg.Type == tea.KeyTab:
		return m.wavesOpenPicker(), nil
	case msg.String() == "c":
		m.waves.critOff = !m.waves.critOff
		return m, nil
	case msg.String() == "b":
		return m.wavesEdge(focus)
	}
	return m, nil
}

// wavesMove returns the focus after one step in direction dir: up/down walk
// the focus column, left/right jump to the card of the adjacent column whose
// row is nearest to the focus card's row. It keeps focus when no card lies
// that way.
func wavesMove(b *waves.Board, focus, dir string) string {
	col, row, ok := b.Pos(focus)
	if !ok {
		return focus
	}
	switch dir {
	case "up", "down":
		ids := b.Columns[col]
		i := slices.Index(ids, focus) + map[string]int{"up": -1, "down": 1}[dir]
		if i >= 0 && i < len(ids) {
			return ids[i]
		}
	case "left", "right":
		c := col + map[string]int{"left": -1, "right": 1}[dir]
		if c < 0 || c >= len(b.Columns) {
			return focus
		}
		best, bestD := "", 0
		for _, id := range b.Columns[c] {
			_, r, _ := b.Pos(id)
			if d := max(r-row, row-r); best == "" || d < bestD {
				best, bestD = id, d
			}
		}
		if best != "" {
			return best
		}
	}
	return focus
}

func (m model) hasVisibleChildren(id string) bool {
	for _, c := range m.idx.Children[id] {
		if c.Status != "scrapped" {
			return true
		}
	}
	return false
}

// wavesGoInto makes the focus card the root (`o`).
func (m model) wavesGoInto(focus *item.Item) (tea.Model, tea.Cmd) {
	if focus == nil || !m.hasVisibleChildren(focus.ID) {
		return m.showToast(toastInfo, "No children to go into", "", nil, false)
	}
	return m.wavesSetRoot(focus.ID, ""), nil
}

// wavesGoUp makes the root's parent the root and focuses the former root
// (`Backspace`); a root without a parent stays.
func (m model) wavesGoUp() model {
	root := m.idx.ByID[m.waves.root]
	if root == nil || root.Parent == "" {
		return m
	}
	if _, ok := m.idx.ByID[root.Parent]; !ok {
		return m
	}
	return m.wavesSetRoot(root.Parent, root.ID)
}

// wavesLevelIDs lists the entries of the level picker: the root and its
// non-scrapped siblings that have visible children.
func (m model) wavesLevelIDs() []string {
	root := m.idx.ByID[m.waves.root]
	if root == nil {
		return nil
	}
	var sibs []*item.Item
	if root.Parent != "" {
		sibs = m.idx.Children[root.Parent]
	} else {
		for _, it := range m.idx.ByID {
			if it.Parent == "" {
				sibs = append(sibs, it)
			}
		}
		sort.Slice(sibs, func(i, j int) bool { return sibs[i].ID < sibs[j].ID })
	}
	var ids []string
	for _, it := range sibs {
		if it.ID == root.ID || (it.Status != "scrapped" && m.hasVisibleChildren(it.ID)) {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

func (m model) wavesOpenPicker() model {
	m.waves.pickIDs = m.wavesLevelIDs()
	m.waves.pick = listState{}
	m.waves.pick.setLen(len(m.waves.pickIDs))
	m.waves.pick.cursor = max(slices.Index(m.waves.pickIDs, m.waves.root), 0)
	m.waves.picking = len(m.waves.pickIDs) > 0
	return m
}

func (m model) keyWavesPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keybind.Matches(msg, keys.Back):
		m.waves.picking = false
	case keybind.Matches(msg, keys.Enter):
		m.waves.picking = false
		if id := m.waves.pickIDs[m.waves.pick.cursor]; m.idx.ByID[id] != nil {
			m = m.wavesSetRoot(id, "")
		}
	default:
		switch navKey(msg.String()) {
		case "up":
			m.waves.pick.move(-1)
		case "down":
			m.waves.pick.move(1)
		}
	}
	return m, nil
}

// keyWavesDetail drives the detail panel opened with Enter: esc returns to
// the board with root and focus unchanged, everything else is the existing
// detail handling (item actions and the box form of the focus item).
func (m model) keyWavesDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keybind.Matches(msg, keys.Back) {
		m.waves.detail = false
		return m, nil
	}
	if handled, nm, cmd := m.keyNodeAction(msg); handled {
		return nm, cmd
	}
	return m.keyDetailFocus(msg)
}

// wavesNonSelectable is the set of cards a target must not be while mark is
// set: the mark and every card the mark depends on along the board's
// card-level edges, drawn or hidden. Marking such a card as waiting on the
// mark would close a cycle.
func wavesNonSelectable(b *waves.Board, mark string) map[string]bool {
	out := map[string]bool{mark: true}
	edges := b.Edges()
	for grew := true; grew; {
		grew = false
		for _, e := range edges {
			if out[e.To] && !out[e.From] {
				out[e.From], grew = true, true
			}
		}
	}
	return out
}

// wavesEdge is `b`: set the mark, cancel on the mark, or write the edge
// "focus waits on mark" (removing it when it exists) with the focus item's
// ETag as of now.
func (m model) wavesEdge(focus *item.Item) (tea.Model, tea.Cmd) {
	if focus == nil {
		return m, nil
	}
	mark := m.waves.mark
	switch {
	case mark == "":
		m.waves.mark = focus.ID
		return m, nil
	case focus.ID == mark:
		m.waves.mark = ""
		return m, nil
	case wavesNonSelectable(m.waves.board, mark)[focus.ID]:
		return m.showToast(toastInfo, "Not selectable: would form a cycle", "", nil, false)
	}
	etag, ok := m.beanETag(focus.ID)
	if !ok {
		m.err = "Item no longer exists — selection discarded"
		return m.showToast(toastError, m.err, "", nil, false)
	}
	var diff data.WholeEditDiff
	if focus.IsBlockedBy(mark) {
		diff.BlockedByRemove = []string{mark}
	} else {
		diff.BlockedByAdd = []string{mark}
	}
	m.waves.mark = ""
	store, id := m.store, focus.ID
	return m, mutateCmd(store, func() error { return store.UpdateWhole(id, diff, etag) })
}

// wavesCrumbs is the breadcrumb bar: one dropdown box per item from the
// top-level ancestor down to the root, root last and focused. Leftmost
// segments are dropped, with an ellipsis, when the chain does not fit.
func (m model) wavesCrumbs(width int) string {
	var chain []*item.Item
	for id := m.waves.root; len(chain) <= len(m.idx.ByID); {
		it, ok := m.idx.ByID[id]
		if !ok {
			break
		}
		chain = append([]*item.Item{it}, chain...)
		id = it.Parent
	}
	if len(chain) == 0 {
		return ""
	}
	const sepW = 3
	widths := make([]int, len(chain))
	total := 0
	for i, it := range chain {
		widths[i] = min(max(lipgloss.Width(it.Title)+6, 12), 28)
		total += widths[i] + sepW
	}
	total -= sepW
	cut := 0
	for total > width && cut < len(chain)-1 {
		total -= widths[cut] + sepW
		cut++
	}
	if cut > 0 {
		total += sepW // room for the ellipsis segment
	}
	last := len(chain) - 1
	widths[last] = max(8, widths[last]-max(total-width, 0))

	sep := "   \n" + theme.Chevron.Render(" › ") + "\n   "
	var parts []string
	if cut > 0 {
		parts = append(parts, "   \n"+theme.Chevron.Render(" … ")+"\n   ", sep)
	}
	for i := cut; i <= last; i++ {
		hotkey := ""
		if i == last {
			hotkey = "tab"
		}
		parts = append(parts, dropdownBox(chain[i].Type, chain[i].Title, hotkey, widths[i], i == last))
		if i < last {
			parts = append(parts, sep)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// wavesInfo is the footer status: `column n/m` for the focus column and
// `n redundant` for the focus card.
func (m model) wavesInfo() string {
	b := m.waves.board
	col, _, ok := b.Pos(m.waves.focus)
	if !ok {
		return ""
	}
	info := "column " + strconv.Itoa(col+1) + "/" + strconv.Itoa(len(b.Columns))
	if n := b.Redundant(m.waves.focus); n > 0 {
		info += " · " + strconv.Itoa(n) + " redundant"
	}
	return info
}

func wavesHint(key, desc string) keybind.Binding {
	return keybind.NewBinding(keybind.WithKeys(key), keybind.WithHelp(key, desc))
}

// wavesLocalBindings is the footer's local key set for the current mode.
func (m model) wavesLocalBindings() []keybind.Binding {
	switch {
	case m.waves.picking:
		return []keybind.Binding{keys.Up, keys.Down, wavesHint("enter", "open level"), keys.Back}
	case m.waves.detail:
		return []keybind.Binding{keys.Up, keys.Down, keys.Enter, wavesHint("esc", "back to board")}
	case m.waves.mark != "":
		return []keybind.Binding{wavesHint("b", "set/remove edge on this card"), wavesHint("b", "on mark: cancel"), wavesHint("esc", "cancel")}
	}
	return []keybind.Binding{
		wavesHint("↑↓←→", "move"), wavesHint("enter", "details"), wavesHint("o", "open"),
		wavesHint("backspace", "up"), wavesHint("tab", "level"), wavesHint("b", "edge"),
		wavesHint("c", "critical path"), wavesHint("esc", "back"),
	}
}

// wavesChrome builds the header (breadcrumb line plus dropdown bar) and the
// footer (status, then local keys).
func (m model) wavesChrome(innerW int) (head, localKeys string) {
	head = breadcrumb(m.repoLabel(), "Waves", renderBindings(globalBindings()), innerW) + "\n" + m.wavesCrumbs(innerW)
	hint := renderBindings(m.wavesLocalBindings())
	if info := m.wavesInfo(); info != "" && !m.waves.picking {
		hint = theme.Muted.Render(info) + " · " + hint
	}
	localKeys = footer(hint, innerW)
	return
}

func (m model) viewWaves() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2
	head, localKeys := m.wavesChrome(innerW)
	div := theme.Dim.Render(strings.Repeat("─", innerW))
	status := m.statusLine(innerW)
	bodyH, _, _, _, _ := clickPaneGeometry(w, h, head, localKeys, status, m.settings.Layout.TreeWidth, layoutRoadmap)
	paneW := innerW - 2

	var body string
	if f := m.wavesFocusItem(); m.waves.detail && f != nil {
		body = m.renderBeanDetailBox(f, paneW, bodyH, true)
	} else {
		body = renderPane(pane{rows: m.wavesRows(paneW-2, bodyH)}, paneW, bodyH, true)
	}
	content := appendStatusLine(head+"\n"+div+"\n"+body+"\n"+div+"\n"+localKeys, status)
	out := outerBorder(content, innerW, true)
	if m.waves.picking {
		out = placeOverlay(out, m.wavesPickerBox(), w, h)
	}
	return m.composeOverlays(out, w, h)
}

// wavesRows is the board cut, or the hint of an empty root.
func (m model) wavesRows(w, h int) []string {
	b := m.waves.board
	if len(b.Columns) == 0 {
		return []string{
			theme.Muted.Render("No visible children."),
			theme.Muted.Render("Backspace: up one level · Tab: pick another level"),
		}
	}
	var dimmed map[string]bool
	if m.waves.mark != "" {
		dimmed = wavesNonSelectable(b, m.waves.mark)
	}
	return strings.Split(b.Render(waves.RenderOpts{
		Width: w, Height: h, Focus: m.waves.focus, Mark: m.waves.mark,
		Dimmed: dimmed, Critical: !m.waves.critOff,
	}), "\n")
}

func (m model) wavesPickerBox() string {
	body := menuList(len(m.waves.pickIDs), m.waves.pick.cursor, func(i int, sel bool) string {
		it := m.idx.ByID[m.waves.pickIDs[i]]
		if it == nil {
			return m.waves.pickIDs[i]
		}
		return truncate(it.Title, 44) + " " + theme.Muted.Render(it.ID)
	})
	return modalPanel("Level", strings.TrimRight(body, "\n"), "enter: open · esc: cancel", 56, theme.Mauve)
}
