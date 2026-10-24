package tui

// view_waves_test.go — Update-step tests of the Waves view (bean beans-wrj0):
// entry, movement, level walking, detail, edge mode, reload, footer and the
// critical-path toggle. They drive the model with step/runeMsg/keyMsg and use
// the production waves.Build for every board fact.

import (
	"slices"
	"strings"
	"testing"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/rierek/lif/internal/tui/theme"
	"github.com/rierek/lif/pkg/item"
)

// wavesFixture: top > ms-w > cards. Chain k-a -> k-b -> k-c, k-d waits on
// k-a, k-c also waits on k-a directly (a redundant edge), k-l has no edges
// (loose column). k-a has a child. ms-s is a sibling level with children,
// ms-e a sibling without children, ms-x a scrapped sibling with children.
func wavesFixture() []*item.Item {
	task := func(id, parent string, blockedBy ...string) *item.Item {
		return &item.Item{ID: id, Title: "Card " + id, Status: "todo", Type: "task", Priority: "normal", Parent: parent, BlockedBy: blockedBy}
	}
	a := task("k-a", "ms-w")
	a.Tags = []string{"note-wavetag"}
	return []*item.Item{
		{ID: "top", Title: "Top Level", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ms-w", Title: "Milestone W", Status: "todo", Type: "epic", Priority: "normal", Parent: "top"},
		{ID: "ms-s", Title: "Milestone S", Status: "todo", Type: "epic", Priority: "normal", Parent: "top"},
		{ID: "ms-e", Title: "Milestone E", Status: "todo", Type: "epic", Priority: "normal", Parent: "top"},
		{ID: "ms-x", Title: "Milestone X", Status: "scrapped", Type: "epic", Priority: "normal", Parent: "top"},
		a,
		task("k-b", "ms-w", "k-a"),
		task("k-c", "ms-w", "k-b", "k-a"),
		task("k-d", "ms-w", "k-a"),
		task("k-l", "ms-w"),
		task("k-a1", "k-a"),
		task("s-1", "ms-s"),
		task("x-1", "ms-x"),
	}
}

func wavesModel(t *testing.T) model {
	t.Helper()
	m := fixtureModel(t, wavesFixture())
	m.width, m.height = 120, 40
	return m
}

// openWavesOn presses W with id focused in the Roadmap.
func openWavesOn(t *testing.T, m model, id string) model {
	t.Helper()
	m = focusBeanFull(m, id)
	m = step(t, m, runeMsg('W'))
	if m.view != viewWaves || m.waves.root != id {
		t.Fatalf("after W on %s: view=%v root=%q", id, m.view, m.waves.root)
	}
	return m
}

func wavesKeys(t *testing.T, m model, ks ...string) model {
	t.Helper()
	for _, k := range ks {
		switch k {
		case "up":
			m = step(t, m, keyMsg(tea.KeyUp))
		case "down":
			m = step(t, m, keyMsg(tea.KeyDown))
		case "left":
			m = step(t, m, keyMsg(tea.KeyLeft))
		case "right":
			m = step(t, m, keyMsg(tea.KeyRight))
		case "enter":
			m = step(t, m, keyMsg(tea.KeyEnter))
		case "esc":
			m = step(t, m, keyMsg(tea.KeyEsc))
		case "tab":
			m = step(t, m, keyMsg(tea.KeyTab))
		case "backspace":
			m = step(t, m, keyMsg(tea.KeyBackspace))
		default:
			m = step(t, m, runeMsg([]rune(k)[0]))
		}
	}
	return m
}

// focusOn moves the Waves focus to id by state, so tests that are not about
// movement do not depend on the movement keys.
func focusOn(t *testing.T, m model, id string) model {
	t.Helper()
	if _, _, ok := m.waves.board.Pos(id); !ok {
		t.Fatalf("%s is not on the board: %v", id, m.waves.board.Columns)
	}
	m.waves.focus = id
	return m
}

func TestWavesOpensFromRoadmapFlatListAndReady(t *testing.T) {
	m := wavesModel(t)
	if got := openWavesOn(t, m, "ms-w"); got.waves.focus != wavesFirstCard(got.waves.board) || got.waves.focus != "k-l" {
		t.Fatalf("focus on open = %q, want the first card of the leftmost (loose) column k-l", got.waves.focus)
	}

	flat := m.showView(viewBrowseRepo, true)
	flat.flatList.cursor = slices.IndexFunc(flat.flatVisible(), func(it *item.Item) bool { return it.ID == "ms-s" })
	flat = step(t, flat, runeMsg('W'))
	if flat.view != viewWaves || flat.waves.root != "ms-s" {
		t.Fatalf("W in flat list: view=%v root=%q", flat.view, flat.waves.root)
	}
	if flat = step(t, flat, keyMsg(tea.KeyEsc)); flat.view != viewBrowseRepo || !flat.flatView {
		t.Fatalf("esc returns to flat list, got view=%v flat=%v", flat.view, flat.flatView)
	}

	ready := step(t, m, beansLoadedMsg{beans: wavesFixture(), ready: map[string]bool{"k-b": true}}).showView(viewReady, false)
	vis := ready.readyVisible()
	if len(vis) == 0 {
		t.Fatal("fixture has no ready item")
	}
	ready.readyList.cursor = 0
	want := vis[0].ID
	ready = step(t, ready, runeMsg('W'))
	if ready.view != viewWaves || ready.waves.root != want {
		t.Fatalf("W in Ready: view=%v root=%q, want %q", ready.view, ready.waves.root, want)
	}
	if ready = step(t, ready, keyMsg(tea.KeyEsc)); ready.view != viewReady {
		t.Fatalf("esc returns to Ready, got %v", ready.view)
	}
}

func TestWavesPaletteEntry(t *testing.T) {
	m := focusBeanFull(wavesModel(t), "ms-w")
	m = step(t, m, runeMsg('K'))
	for _, q := range []string{"waves", "graph"} {
		m.palQuery = q
		var found bool
		for _, it := range m.palFiltered() {
			if it.actionID == "go_waves" {
				found = it.label == "view: waves (graph)"
			}
		}
		if !found {
			t.Fatalf("query %q does not list `view: waves (graph)`", q)
		}
	}
	m.palQuery = "waves"
	m.palList.setLen(len(m.palFiltered()))
	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.view != viewWaves || m.waves.root != "ms-w" || m.paletteOpen {
		t.Fatalf("palette dispatch: view=%v root=%q paletteOpen=%v", m.view, m.waves.root, m.paletteOpen)
	}
}

func TestWavesMovementFollowsNavKey(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	cols := m.waves.board.Columns
	if len(cols) != 4 || !m.waves.board.Loose {
		t.Fatalf("fixture board = %v, want loose column plus 3 columns", cols)
	}
	// Column of k-a, then right to the next column and back.
	m = wavesKeys(t, m, "l")
	if m.waves.focus != "k-a" {
		t.Fatalf("l from the loose column: focus = %q, want k-a", m.waves.focus)
	}
	for _, right := range []string{"l", "right"} {
		n := focusOn(t, m, "k-a")
		n = wavesKeys(t, n, right)
		if col, _, _ := n.waves.board.Pos(n.waves.focus); col != 2 {
			t.Fatalf("%s from k-a: focus %q in column %d, want column 2", right, n.waves.focus, col)
		}
		for _, left := range []string{"j", "left"} {
			if b := wavesKeys(t, n, left); b.waves.focus != "k-a" {
				t.Fatalf("%s back: focus = %q, want k-a", left, b.waves.focus)
			}
		}
	}
	// Column 2 holds k-b and k-d: k down / i up walk it.
	n := focusOn(t, m, cols[2][0])
	for _, down := range []string{"k", "down"} {
		got := wavesKeys(t, n, down)
		if got.waves.focus != cols[2][1] {
			t.Fatalf("%s: focus = %q, want %q", down, got.waves.focus, cols[2][1])
		}
		for _, up := range []string{"i", "up"} {
			if back := wavesKeys(t, got, up); back.waves.focus != cols[2][0] {
				t.Fatalf("%s: focus = %q, want %q", up, back.waves.focus, cols[2][0])
			}
		}
	}
}

func TestWavesMovementStopsAtTheEdges(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w") // focus k-l: leftmost, only card of its column
	for _, k := range []string{"j", "left", "i", "up", "k", "down", "[", "]"} {
		if got := wavesKeys(t, m, k); got.waves.focus != "k-l" {
			t.Fatalf("%s moved the focus to %q, want k-l", k, got.waves.focus)
		}
	}
	last := m.waves.board.Columns[len(m.waves.board.Columns)-1]
	n := focusOn(t, m, last[len(last)-1])
	for _, k := range []string{"l", "right", "k", "down"} {
		if got := wavesKeys(t, n, k); got.waves.focus != n.waves.focus {
			t.Fatalf("%s at the last card moved the focus to %q", k, got.waves.focus)
		}
	}
}

func TestWavesMovementTakesDirectionsFromNavKey(t *testing.T) {
	old := keys.Up
	keys.Up = keybind.NewBinding(keybind.WithKeys("up", "i", "y"), keybind.WithHelp("↑/i", "up"))
	t.Cleanup(func() { keys.Up = old })
	m := openWavesOn(t, wavesModel(t), "ms-w")
	cols := m.waves.board.Columns
	m = focusOn(t, m, cols[2][1])
	if got := wavesKeys(t, m, "y"); got.waves.focus != cols[2][0] {
		t.Fatalf("a key added to the Up binding moved the focus to %q, want %q", got.waves.focus, cols[2][0])
	}
}

func TestWavesViewportFollowsFocusOnNarrowTerminal(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	m.width = 64
	m = focusOn(t, m, "k-c")
	view := m.View()
	if !strings.Contains(view, "k-c") || !strings.Contains(view, "k-b") {
		t.Fatalf("focus column and its left neighbour must be visible:\n%s", view)
	}
	if strings.Contains(view, "k-l") || strings.Contains(view, "k-a ") {
		t.Fatalf("columns left of the neighbour must be scrolled out:\n%s", view)
	}
}

func TestWavesBreadcrumbsGoIntoAndBackUp(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	if v := m.View(); !strings.Contains(v, "Top Level") || !strings.Contains(v, "Milestone W") {
		t.Fatalf("breadcrumb bar must show the ancestor chain:\n%s", v)
	}
	m = focusOn(t, m, "k-a")
	in := wavesKeys(t, m, "o")
	if in.waves.root != "k-a" || in.waves.focus != "k-a1" {
		t.Fatalf("o: root=%q focus=%q, want root k-a, focus k-a1", in.waves.root, in.waves.focus)
	}
	up := wavesKeys(t, in, "backspace")
	if up.waves.root != "ms-w" || up.waves.focus != "k-a" {
		t.Fatalf("backspace: root=%q focus=%q, want ms-w / k-a", up.waves.root, up.waves.focus)
	}

	// o on a card without children keeps root and focus and shows a notice.
	m = focusOn(t, m, "k-b")
	nm, _ := m.Update(runeMsg('o'))
	got := nm.(model)
	if got.waves.root != "ms-w" || got.waves.focus != "k-b" || got.toast == nil {
		t.Fatalf("o without children: root=%q focus=%q toast=%v", got.waves.root, got.waves.focus, got.toast)
	}

	// Backspace on a root without parent keeps everything.
	top := openWavesOn(t, wavesModel(t), "top")
	if got := wavesKeys(t, top, "backspace"); got.waves.root != "top" || got.waves.focus != top.waves.focus {
		t.Fatalf("backspace at the top: root=%q focus=%q", got.waves.root, got.waves.focus)
	}
}

func TestWavesLevelPicker(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	m = wavesKeys(t, m, "tab")
	if !m.waves.picking || !slices.Equal(m.waves.pickIDs, wavesFixtureLevelIDs(t, m)) {
		t.Fatalf("picker = %v picking=%v", m.waves.pickIDs, m.waves.picking)
	}
	if slices.Contains(m.waves.pickIDs, "ms-e") || slices.Contains(m.waves.pickIDs, "ms-x") {
		t.Fatalf("picker must hold only the root and siblings with visible children: %v", m.waves.pickIDs)
	}
	if !slices.Contains(m.waves.pickIDs, "ms-s") || !slices.Contains(m.waves.pickIDs, "ms-w") {
		t.Fatalf("picker lacks root or sibling ms-s: %v", m.waves.pickIDs)
	}
	if v := m.View(); !strings.Contains(v, "Milestone S") {
		t.Fatalf("open picker must list ms-s:\n%s", v)
	}
	cancel := wavesKeys(t, m, "esc")
	if cancel.waves.picking || cancel.waves.root != "ms-w" || cancel.view != viewWaves {
		t.Fatalf("esc closes only the picker: %+v", cancel.waves)
	}
	m.waves.pick.cursor = slices.Index(m.waves.pickIDs, "ms-s")
	m = wavesKeys(t, m, "enter")
	if m.waves.picking || m.waves.root != "ms-s" || m.waves.focus != "s-1" {
		t.Fatalf("enter: picking=%v root=%q focus=%q", m.waves.picking, m.waves.root, m.waves.focus)
	}
}

// wavesFixtureLevelIDs reads the expected entries from the model's own index
// order for the root's parent: the production list must follow it.
func wavesFixtureLevelIDs(t *testing.T, m model) []string {
	t.Helper()
	var ids []string
	for _, it := range m.idx.Children["top"] {
		if it.ID == "ms-w" || (it.Status != "scrapped" && len(m.idx.Children[it.ID]) > 0) {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

func TestWavesEnterShowsDetailAndEscReturns(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	m = focusOn(t, m, "k-a")
	d := wavesKeys(t, m, "enter")
	if !d.waves.detail || d.focusedBean() == nil || d.focusedBean().ID != "k-a" {
		t.Fatalf("enter: detail=%v focused=%v", d.waves.detail, d.focusedBean())
	}
	if v := d.View(); !strings.Contains(v, "note-wavetag") || !strings.Contains(v, "Card k-a") {
		t.Fatalf("detail panel must show the item and its tags:\n%s", v)
	}
	back := wavesKeys(t, d, "esc")
	if back.waves.detail || back.view != viewWaves || back.waves.root != "ms-w" || back.waves.focus != "k-a" {
		t.Fatalf("esc from detail: %+v view=%v", back.waves, back.view)
	}
}

func TestWavesEmptyRootShowsHint(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-e")
	if m.waves.focus != "" {
		t.Fatalf("focus on an empty board = %q", m.waves.focus)
	}
	if v := m.View(); !strings.Contains(v, "Backspace") || !strings.Contains(v, "Tab") {
		t.Fatalf("empty root must name Backspace and Tab:\n%s", v)
	}
	if got := wavesKeys(t, m, "b", "enter", "o", "l"); got.waves.mark != "" || got.waves.detail {
		t.Fatalf("keys on an empty board must do nothing: %+v", got.waves)
	}
}

func TestWavesEscWithoutMarkReturnsToOrigin(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	if got := wavesKeys(t, m, "esc"); got.view != viewBrowseRepo || got.flatView {
		t.Fatalf("esc: view=%v flat=%v, want the roadmap", got.view, got.flatView)
	}
}

func TestWavesNonSelectableAreMarkAndItsDependencies(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	got := wavesNonSelectable(m.waves.board, "k-c")
	for _, id := range []string{"k-c", "k-b", "k-a"} {
		if !got[id] {
			t.Fatalf("%s must be non-selectable for mark k-c: %v", id, got)
		}
	}
	if got["k-d"] || got["k-l"] || len(got) != 3 {
		t.Fatalf("only the mark and its dependencies, got %v", got)
	}
}

func TestWavesEdgeModeMarkCancelAndRefusal(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	m = focusOn(t, m, "k-c")
	m = wavesKeys(t, m, "b")
	if m.waves.mark != "k-c" {
		t.Fatalf("first b: mark = %q", m.waves.mark)
	}
	if v := strings.ReplaceAll(m.View(), nbsp, " "); !strings.Contains(v, "set/remove edge") {
		t.Fatalf("edge mode shows its key hints in the footer:\n%s", v)
	}
	// non-selectable target: no write, mark kept, notice shown.
	n := focusOn(t, m, "k-a")
	tm, cmd := n.Update(runeMsg('b'))
	nn := tm.(model)
	if cmd != nil && nn.toast == nil {
		t.Fatal("refusal must not dispatch a write")
	}
	if nn.waves.mark != "k-c" || nn.toast == nil {
		t.Fatalf("non-selectable b: mark=%q toast=%v", nn.waves.mark, nn.toast)
	}
	// b on the mark and esc cancel without a write.
	for _, k := range []string{"b", "esc"} {
		tm, cmd := m.Update(wavesKeyMsg(k))
		got := tm.(model)
		if got.waves.mark != "" || cmd != nil || got.view != viewWaves {
			t.Fatalf("%s in edge mode: mark=%q cmd=%v view=%v", k, got.waves.mark, cmd != nil, got.view)
		}
	}
}

func wavesKeyMsg(k string) tea.KeyMsg {
	if k == "esc" {
		return keyMsg(tea.KeyEsc)
	}
	return runeMsg([]rune(k)[0])
}

// wavesStoreModel is the Waves model over a real store with the same items.
func wavesStoreModel(t *testing.T) model {
	t.Helper()
	m := wavesModel(t)
	m.store = newTestStore(t, wavesFixture()...)
	return openWavesOn(t, m, "ms-w")
}

// settle runs a mutation Cmd and feeds its result and the reload it asks for
// back into the model, the way the program would.
func settle(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a mutation Cmd")
	}
	done, ok := cmd().(mutationDoneMsg)
	if !ok {
		t.Fatal("expected a mutationDoneMsg")
	}
	m = step(t, m, done)
	return step(t, m, loadCmd(m.store)())
}

func TestWavesSecondBSetsAndRemovesTheEdge(t *testing.T) {
	m := wavesStoreModel(t)
	colBefore, _, _ := m.waves.board.Pos("k-d")

	m = focusOn(t, m, "k-b")
	m = wavesKeys(t, m, "b")
	m = focusOn(t, m, "k-d")
	tm, cmd := m.Update(runeMsg('b'))
	m = tm.(model)
	if m.waves.mark != "" {
		t.Fatalf("the mark is cleared by the second b, got %q", m.waves.mark)
	}
	m = settle(t, m, cmd)
	if got := storeItem(t, m.store, "k-d"); !got.IsBlockedBy("k-b") {
		t.Fatalf("k-d blocked_by = %v, want it to hold k-b", got.BlockedBy)
	}
	if m.waves.focus != "k-d" {
		t.Fatalf("focus after the reload = %q, want k-d", m.waves.focus)
	}
	if col, _, _ := m.waves.board.Pos("k-d"); col != colBefore+1 {
		t.Fatalf("k-d column after the edge = %d, want %d (one column right)", col, colBefore+1)
	}

	// The same pair again removes the edge.
	m = focusOn(t, m, "k-b")
	m = wavesKeys(t, m, "b")
	m = focusOn(t, m, "k-d")
	tm, cmd = m.Update(runeMsg('b'))
	m = settle(t, tm.(model), cmd)
	if got := storeItem(t, m.store, "k-d"); got.IsBlockedBy("k-b") || !got.IsBlockedBy("k-a") {
		t.Fatalf("k-d blocked_by = %v, want only k-a", got.BlockedBy)
	}
}

func TestWavesNoWriteWithoutSecondB(t *testing.T) {
	m := wavesStoreModel(t)
	before := storeItem(t, m.store, "k-d").ETag()
	m = focusOn(t, m, "k-b")
	m = wavesKeys(t, m, "b", "l", "k", "c", "enter", "esc", "esc", "tab", "esc", "o", "backspace")
	if after := storeItem(t, m.store, "k-d").ETag(); after != before {
		t.Fatal("navigation, mode keys and a cancelled mark must not write item data")
	}
}

func TestWavesEdgeWriteReadsETagAtSecondB(t *testing.T) {
	m := wavesStoreModel(t)
	m = focusOn(t, m, "k-b")
	m = wavesKeys(t, m, "b")
	// Another writer changes the target between the two b presses; the reload
	// delivers the new ETag.
	d := storeItem(t, m.store, "k-d")
	if err := m.store.SetTitle("k-d", "Renamed elsewhere", d.ETag()); err != nil {
		t.Fatal(err)
	}
	m = step(t, m, loadCmd(m.store)())
	m = focusOn(t, m, "k-d")
	tm, cmd := m.Update(runeMsg('b'))
	if cmd == nil {
		t.Fatal("second b must dispatch a write")
	}
	if done := cmd().(mutationDoneMsg); done.err != nil {
		t.Fatalf("write with the ETag read at the second b failed: %v", done.err)
	}
	_ = tm
	if got := storeItem(t, m.store, "k-d"); !got.IsBlockedBy("k-b") {
		t.Fatalf("k-d blocked_by = %v", got.BlockedBy)
	}
}

func TestWavesStaleETagReportsConflictAndReloads(t *testing.T) {
	m := wavesStoreModel(t)
	m = focusOn(t, m, "k-b")
	m = wavesKeys(t, m, "b")
	// The store changes, the model is not reloaded: its ETag is stale.
	d := storeItem(t, m.store, "k-d")
	if err := m.store.SetTitle("k-d", "Renamed elsewhere", d.ETag()); err != nil {
		t.Fatal(err)
	}
	m = focusOn(t, m, "k-d")
	_, cmd := m.Update(runeMsg('b'))
	done := cmd().(mutationDoneMsg)
	if done.err == nil {
		t.Fatal("stale ETag must fail")
	}
	nm, reload := m.applyMutationResult(done.err, "")
	if reload == nil || nm.(model).toast == nil {
		t.Fatal("a failed edge write goes through applyMutationResult: toast plus reload")
	}
	if storeItem(t, m.store, "k-d").IsBlockedBy("k-b") {
		t.Fatal("a failed write must leave the file unchanged")
	}
}

func TestWavesReloadKeepsRootAndFocus(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	m = focusOn(t, m, "k-c")
	items := wavesFixture()
	items = append(items, &item.Item{ID: "k-n", Title: "New", Status: "todo", Type: "task", Priority: "normal", Parent: "ms-w"})
	m = step(t, m, beansLoadedMsg{beans: items})
	if m.waves.root != "ms-w" || m.waves.focus != "k-c" {
		t.Fatalf("after reload root=%q focus=%q", m.waves.root, m.waves.focus)
	}
	if _, _, ok := m.waves.board.Pos("k-n"); !ok {
		t.Fatal("the board must be rebuilt from the reloaded items")
	}
}

func TestWavesReloadFocusGoneFocusesFirstCard(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	m = focusOn(t, m, "k-d")
	var items []*item.Item
	for _, it := range wavesFixture() {
		if it.ID != "k-d" {
			items = append(items, it)
		}
	}
	m = step(t, m, beansLoadedMsg{beans: items})
	if m.waves.focus != wavesFirstCard(m.waves.board) {
		t.Fatalf("focus = %q, want the first card %q", m.waves.focus, wavesFirstCard(m.waves.board))
	}
}

func TestWavesReloadRootGoneReturnsToOrigin(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	var items []*item.Item
	for _, it := range wavesFixture() {
		if it.ID != "ms-w" && it.Parent != "ms-w" {
			items = append(items, it)
		}
	}
	m = step(t, m, beansLoadedMsg{beans: items})
	if m.view != viewBrowseRepo {
		t.Fatalf("view after the root vanished = %v, want the origin view", m.view)
	}
}

func TestWavesFooterShowsColumnAndRedundant(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	m = focusOn(t, m, "k-a")
	col, _, _ := m.waves.board.Pos("k-a")
	want := "column " + string(rune('0'+col+1)) + "/" + string(rune('0'+len(m.waves.board.Columns)))
	v := m.View()
	if !strings.Contains(v, want) {
		t.Fatalf("footer lacks %q:\n%s", want, v)
	}
	if !strings.Contains(v, "1 redundant") {
		t.Fatalf("k-a carries the redundant edge a->c, footer lacks `1 redundant`:\n%s", v)
	}
	if v := focusOn(t, m, "k-b").View(); strings.Contains(v, "redundant") {
		t.Fatalf("k-b has no redundant edge:\n%s", v)
	}
}

func TestWavesCriticalPathShownOnOpenAndToggledByC(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })

	red, _, _ := strings.Cut(lipgloss.NewStyle().Foreground(theme.Red).Render("x"), "x")
	m := openWavesOn(t, wavesModel(t), "ms-w")
	if !strings.Contains(m.View(), red) {
		t.Fatal("the critical path must be shown right after opening")
	}
	m = wavesKeys(t, m, "c")
	if strings.Contains(m.View(), red) {
		t.Fatal("c must hide the critical path")
	}
	m = wavesKeys(t, m, "c")
	if !strings.Contains(m.View(), red) {
		t.Fatal("c again must show the critical path")
	}
}

func TestWavesGlobalKeysKeepTheirMeaning(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	if got := step(t, m, runeMsg('?')); !got.helpOpen || got.view != viewWaves {
		t.Fatal("? opens the help")
	}
	for _, k := range []rune{'K', ':'} {
		if got := step(t, m, runeMsg(k)); !got.paletteOpen {
			t.Fatalf("%c opens the Command Center", k)
		}
	}
	if got := step(t, m, runeMsg('q')); !got.confirmQuit {
		t.Fatal("q starts the quit flow")
	}
	// Create and the node-action keys have no meaning on the board.
	for _, k := range []rune{'W', 'a', 's', 'd', 'x', 'f', '/'} {
		got := step(t, m, runeMsg(k))
		if got.view != viewWaves || got.overlay != overlayNone || got.form != nil || got.searchActive || got.waves.mark != "" {
			t.Fatalf("%c must be a no-op in the Waves view", k)
		}
	}
}

func TestWavesIgnoresMouse(t *testing.T) {
	m := openWavesOn(t, wavesModel(t), "ms-w")
	for _, msg := range []tea.MouseMsg{
		{X: 10, Y: 10, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		{X: 10, Y: 10, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
	} {
		got := step(t, m, msg)
		if got.view != viewWaves || got.waves.focus != m.waves.focus || got.waves.root != m.waves.root {
			t.Fatalf("mouse %v changed the Waves view", msg)
		}
	}
}
