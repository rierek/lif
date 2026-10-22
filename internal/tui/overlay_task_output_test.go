package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
)

const taskTestCfg = "project:\n    name: tui-test\nlif:\n    path: .lif\n"

// taskModel is a model over a real store whose config is taskTestCfg plus
// extra (tasks:, tui:). The detail pane is shown and tk-1 (in-progress) is
// focused; tk-2 is todo.
func taskModel(t *testing.T, extra string) model {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // no global tasks
	base := newTestStore(t, fixtureBeans()...)
	if err := os.WriteFile(filepath.Join(base.RepoDir, ".lif.yml"), []byte(taskTestCfg+extra), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := data.Open(base.RepoDir, "", "")
	if err != nil {
		t.Fatalf("data.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	m := newModel(s, s.RepoDir)
	m = step(t, m, tea.WindowSizeMsg{Width: 160, Height: 45})
	m = step(t, m, beansLoadedMsg{beans: s.Items()})
	m.detailHidden = false
	return focusBean(m, "tk-1")
}

func paletteHas(m model, label string) bool {
	for _, it := range paletteActions(m) {
		if it.label == label {
			return true
		}
	}
	return false
}

const helloTasks = `
tasks:
  hello:
    desc: say hello
    args: [id]
    steps:
      - 'echo "hello $id"'
`

func TestTaskActionOfferedOnlyForMatchingStatus(t *testing.T) {
	m := taskModel(t, helloTasks+`
tui:
  actions:
    - task: hello
      key: A
      statuses: [in-progress]
`)
	if len(m.taskActions) != 1 || m.taskActions[0].Label != "say hello" {
		t.Fatalf("taskActions = %+v, want hello with label from desc", m.taskActions)
	}
	if !paletteHas(m, "run: say hello") {
		t.Error("palette lacks the action for an in-progress item")
	}
	if handled, _, cmd := m.keyTaskAction(runeMsg('A')); !handled || cmd == nil {
		t.Error("key A did not start the task for an in-progress item")
	}

	m = focusBean(m, "tk-2") // todo
	if paletteHas(m, "run: say hello") {
		t.Error("palette offers the action for a todo item")
	}
	if handled, _, _ := m.keyTaskAction(runeMsg('A')); handled {
		t.Error("key A fired for a todo item")
	}
}

func TestTaskActionWithoutStatusesNeedsNoFocus(t *testing.T) {
	m := taskModel(t, `
tasks:
  build:
    steps:
      - 'true'
tui:
  actions:
    - task: build
`)
	m.view = viewReady
	m.readyIDs = nil // an empty Ready list focuses no item
	if m.focusedBean() != nil {
		t.Fatal("test setup: expected no focused item")
	}
	if !paletteHas(m, "run: build") {
		t.Error("palette lacks a status-free action without focus")
	}
}

func TestTaskActionKeyCollisionIsDropped(t *testing.T) {
	m := taskModel(t, helloTasks+`
tui:
  actions:
    - task: hello
      key: s
`)
	if len(m.taskActions) != 0 {
		t.Errorf("taskActions = %+v, want the colliding action dropped", m.taskActions)
	}
	if m.toast == nil || m.toast.kind != toastWarn || !strings.Contains(m.toast.title, "tui action hello: key s already bound") {
		t.Fatalf("toast = %+v, want warn %q", m.toast, "tui action hello: key s already bound")
	}
	// The built-in binding is untouched: s still opens the status menu.
	m = step(t, m, runeMsg('s'))
	if m.overlay != overlayValueMenu {
		t.Errorf("overlay = %v after s, want the status menu", m.overlay)
	}
}

func TestTaskActionDropsUnknownTaskDuplicateAndLongKeys(t *testing.T) {
	m := taskModel(t, helloTasks+`
tui:
  actions:
    - task: nope
      key: N
    - task: hello
      key: ab
    - task: hello
      key: H
    - task: hello
      key: H
`)
	if len(m.taskActions) != 1 || m.taskActions[0].Key != "H" {
		t.Fatalf("taskActions = %+v, want exactly the first H action", m.taskActions)
	}
	for _, want := range []string{"tui action: unknown task nope", "tui action hello: key ab already bound", "tui action hello: key H already bound"} {
		if m.toast == nil || !strings.Contains(m.toast.title, want) {
			t.Errorf("toast %+v lacks %q", m.toast, want)
		}
	}
}

func TestTaskActionKeyRunsTaskThroughUpdate(t *testing.T) {
	m := taskModel(t, helloTasks+`
tui:
  actions:
    - task: hello
      key: A
`)
	nm, cmd := m.Update(runeMsg('A'))
	m = nm.(model)
	if m.toast == nil || m.toast.title != "running hello…" {
		t.Fatalf("toast = %+v, want %q", m.toast, "running hello…")
	}
	if cmd == nil {
		t.Fatal("no command returned")
	}
	done := runTaskCmd(m.store, "hello", "tk-1")().(taskDoneMsg)
	if done.output != "hello tk-1\n" || done.err != nil {
		t.Fatalf("taskDoneMsg = %+v", done)
	}
	m = step(t, m, done)
	if m.overlay != overlayTaskOutput || m.taskOutputTitle != "hello — exit 0" {
		t.Fatalf("overlay = %v title = %q", m.overlay, m.taskOutputTitle)
	}
	if v := m.View(); !strings.Contains(v, "hello tk-1") {
		t.Errorf("view lacks the output:\n%s", v)
	}
}

func TestTaskOutputOverlayShowsExitCode(t *testing.T) {
	m := taskModel(t, `
tasks:
  fail:
    steps:
      - 'echo boom; exit 3'
`)
	done := runTaskCmd(m.store, "fail", "")().(taskDoneMsg)
	m = step(t, m, done)
	if m.overlay != overlayTaskOutput {
		t.Fatalf("overlay = %v, want the task output", m.overlay)
	}
	if want := "fail — exit 3"; m.taskOutputTitle != want {
		t.Errorf("title = %q, want %q", m.taskOutputTitle, want)
	}
	v := m.View()
	for _, want := range []string{"fail — exit 3", "boom", "esc close"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q", want)
		}
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.overlay != overlayNone {
		t.Errorf("overlay = %v after esc, want none", m.overlay)
	}
}

func TestTaskOutputOverlayShowsLifError(t *testing.T) {
	m := taskModel(t, `
tasks:
  two:
    args: [a, b]
    steps:
      - 'true'
`)
	m = step(t, m, runTaskCmd(m.store, "two", "tk-1")().(taskDoneMsg))
	if m.taskOutputTitle != "two — exit 1" {
		t.Errorf("title = %q, want %q", m.taskOutputTitle, "two — exit 1")
	}
	if v := m.View(); !strings.Contains(v, "needs 2 args") {
		t.Errorf("view lacks the error text:\n%s", v)
	}
}

func TestTaskOutputOverlayScrolls(t *testing.T) {
	m := taskModel(t, `
tasks:
  many:
    steps:
      - 'seq 1 100'
`)
	m = step(t, m, runTaskCmd(m.store, "many", "")().(taskDoneMsg))
	if v := m.View(); !strings.Contains(v, "\n") || strings.Contains(v, " 100 ") {
		t.Fatal("setup: first page must not show the last line")
	}
	m = step(t, m, keyMsg(tea.KeyDown))
	if m.taskOutputScroll != 1 {
		t.Errorf("scroll after down = %d, want 1", m.taskOutputScroll)
	}
	m = step(t, m, keyMsg(tea.KeyUp))
	m = step(t, m, keyMsg(tea.KeyUp))
	if m.taskOutputScroll != 0 {
		t.Errorf("scroll after up past the top = %d, want 0", m.taskOutputScroll)
	}
	m = step(t, m, keyMsg(tea.KeyPgDown))
	if m.taskOutputScroll <= 1 {
		t.Errorf("scroll after pgdn = %d, want a page", m.taskOutputScroll)
	}
	for range 20 {
		m = step(t, m, keyMsg(tea.KeyPgDown))
	}
	lines, page, _ := m.taskOutputView()
	if want := len(lines) - page; m.taskOutputScroll != want {
		t.Errorf("scroll at the end = %d, want %d", m.taskOutputScroll, want)
	}
	if !strings.Contains(m.View(), fmt.Sprintf("%d-%d/%d", len(lines)-page+1, len(lines), len(lines))) {
		t.Error("view lacks the scroll position at the end")
	}
	// Other keys leave the overlay open and do nothing.
	m = step(t, m, runeMsg('q'))
	if m.overlay != overlayTaskOutput {
		t.Error("q closed the overlay")
	}
}

func TestTaskDoneWithoutOutputShowsToast(t *testing.T) {
	m := taskModel(t, `
tasks:
  quiet:
    steps:
      - 'true'
`)
	nm, cmd := m.Update(runTaskCmd(m.store, "quiet", "")())
	m = nm.(model)
	if m.overlay != overlayNone {
		t.Errorf("overlay = %v, want none for a silent success", m.overlay)
	}
	if m.toast == nil || m.toast.kind != toastInfo || m.toast.title != "quiet done" {
		t.Errorf("toast = %+v, want info %q", m.toast, "quiet done")
	}
	if cmd == nil {
		t.Error("no reload command")
	}
}

func TestPaletteDispatchStartsTask(t *testing.T) {
	m := taskModel(t, helloTasks+`
tui:
  actions:
    - task: hello
      label: greet
`)
	var item paletteItem
	for _, it := range paletteActions(m) {
		if it.kind == paletteKindTask {
			item = it
		}
	}
	if item.label != "run: greet" || item.taskName != "hello" {
		t.Fatalf("palette task item = %+v", item)
	}
	nm, cmd := m.dispatchPalette(item)
	m = nm.(model)
	if cmd == nil || m.toast == nil || m.toast.title != "running hello…" {
		t.Errorf("cmd = %v toast = %+v", cmd, m.toast)
	}
}

func TestHelpShowsTasksGroup(t *testing.T) {
	m := taskModel(t, helloTasks+`
tui:
  actions:
    - task: hello
      key: A
      label: greet
`)
	box := m.helpBox()
	if !strings.Contains(box, "Tasks") || !strings.Contains(box, "greet") {
		t.Errorf("help lacks the Tasks group:\n%s", box)
	}
	if plain := taskModel(t, ""); strings.Contains(plain.helpBox(), "Tasks") {
		t.Error("help shows a Tasks group without actions")
	}
}

func TestBoundKeysCoversKeymap(t *testing.T) {
	b := keys.boundKeys()
	for _, k := range []string{"s", "q", "D", "ctrl+r", "tab"} {
		if !b[k] {
			t.Errorf("boundKeys lacks %q", k)
		}
	}
	if b["A"] {
		t.Error(`boundKeys claims "A"`)
	}
}
