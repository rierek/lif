package tui

// overlay_task_output.go — tui.actions (.lif.yml): tasks run from the
// Command-Center or a key, and the read-only overlay that shows their output.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
	pkgconfig "github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// taskDoneMsg is the outcome of a started task action.
type taskDoneMsg struct {
	name   string
	output string
	err    error
}

// runTaskCmd runs the task off the UI goroutine. itemID is "" without a
// focused item.
func runTaskCmd(s *data.Store, name, itemID string) tea.Cmd {
	return func() tea.Msg {
		out, err := s.RunTask(context.Background(), name, itemID)
		return taskDoneMsg{name: name, output: out, err: err}
	}
}

// taskActionApplies reports whether a is offered for the focused item b.
// Without Statuses it always is; with Statuses it needs a focused item in
// one of them.
func taskActionApplies(a pkgconfig.TUIAction, b *item.Item) bool {
	if len(a.Statuses) == 0 {
		return true
	}
	if b == nil {
		return false
	}
	for _, s := range a.Statuses {
		if s == b.Status {
			return true
		}
	}
	return false
}

// buildTaskActions validates the config's tui.actions: an unknown task, a key
// with more than one rune and a key that bound already holds are dropped with
// a warning. A kept action gets its Label resolved (label, else task desc,
// else task name) and its key is added to bound, so two actions cannot share
// one.
func buildTaskActions(set tasks.Set, actions []pkgconfig.TUIAction, bound map[string]bool) (kept []pkgconfig.TUIAction, warnings []string) {
	for _, a := range actions {
		a.Label = tasks.Printable(a.Label)
		p, ok := set.Lookup(a.Task, false)
		if !ok {
			warnings = append(warnings, "tui action: unknown task "+a.Task)
			continue
		}
		if a.Key != "" && (utf8.RuneCountInString(a.Key) > 1 || bound[a.Key]) {
			warnings = append(warnings, fmt.Sprintf("tui action %s: key %s already bound", a.Task, a.Key))
			continue
		}
		if a.Key != "" {
			bound[a.Key] = true
		}
		if a.Label == "" {
			a.Label = tasks.Printable(p.Task.Desc)
		}
		if a.Label == "" {
			a.Label = a.Task
		}
		kept = append(kept, a)
	}
	return kept, warnings
}

// setTaskActions rebuilds m.taskActions from the open store's config and
// shows the validation warnings as one sticky warn toast.
func (m model) setTaskActions() model {
	m.taskActions = nil
	if m.store == nil || m.store.Config() == nil || len(m.store.Config().TUI.Actions) == 0 {
		return m
	}
	set, err := m.store.TaskSet()
	if err != nil {
		m, _ = m.showToast(toastWarn, "tui actions: "+err.Error(), "", nil, true)
		return m
	}
	var warnings []string
	m.taskActions, warnings = buildTaskActions(set, m.store.Config().TUI.Actions, keys.boundKeys())
	if len(warnings) > 0 {
		m, _ = m.showToast(toastWarn, strings.Join(warnings, "; "), "", nil, true)
	}
	return m
}

// startTask runs the named task against the focused item and reports that it
// started. The result arrives as taskDoneMsg.
func (m model) startTask(name string) (model, tea.Cmd) {
	if m.store == nil {
		return m, nil
	}
	id := ""
	if b := m.focusedBean(); b != nil {
		id = b.ID
	}
	var toastCmd tea.Cmd
	m, toastCmd = m.showToast(toastInfo, "running "+name+"…", "", nil, false)
	return m, tea.Batch(toastCmd, runTaskCmd(m.store, name, id))
}

// keyTaskAction runs the tui.actions action bound to msg, when its Statuses
// match the focused item.
func (m model) keyTaskAction(msg tea.KeyMsg) (bool, model, tea.Cmd) {
	for _, a := range m.taskActions {
		if a.Key == "" || a.Key != msg.String() || !taskActionApplies(a, m.focusedBean()) {
			continue
		}
		nm, cmd := m.startTask(a.Task)
		return true, nm, cmd
	}
	return false, m, nil
}

// taskExitCode is the code the overlay title shows: 0 without error, the
// step's exit code (128 plus the signal for a killed step), else 1.
func taskExitCode(err error) int {
	if err == nil {
		return 0
	}
	var se *tasks.StepError
	if errors.As(err, &se) {
		if se.Code > 0 {
			return se.Code
		}
		if se.Signal > 0 {
			return 128 + int(se.Signal)
		}
	}
	return 1
}

// applyTaskDone shows a finished task: a short toast when it printed nothing
// and succeeded, else the output overlay. Either way the items reload, since
// the task may have changed the store.
func (m model) applyTaskDone(msg taskDoneMsg) (tea.Model, tea.Cmd) {
	reload := loadCmd(m.store)
	if msg.output == "" && msg.err == nil {
		var toastCmd tea.Cmd
		m, toastCmd = m.showToast(toastInfo, msg.name+" done", "", nil, false)
		return m, tea.Batch(toastCmd, reload)
	}
	text := msg.output
	if msg.err != nil {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += msg.err.Error()
	}
	text = tasks.Printable(ansi.Strip(text))
	m.overlay = overlayTaskOutput
	m.taskOutputTitle = fmt.Sprintf("%s — exit %d", tasks.Printable(msg.name), taskExitCode(msg.err))
	m.taskOutputLines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	m.taskOutputScroll = 0
	return m, reload
}

const taskOutputWidth = 100

// taskOutputView returns the wrapped lines and how many fit the modal.
func (m model) taskOutputView() (lines []string, page, width int) {
	width = clampModalWidth(taskOutputWidth, m.width)
	for _, l := range m.taskOutputLines {
		l = strings.ReplaceAll(l, "\t", "    ")
		lines = append(lines, strings.Split(ansi.Hardwrap(l, width-4, true), "\n")...)
	}
	page = 20
	if m.height > 0 && m.height-8 < page {
		page = max(m.height-8, 3)
	}
	return lines, page, width
}

// taskOutputBox renders the overlay: the title, one page of lines, the footer.
func (m model) taskOutputBox() string {
	lines, page, width := m.taskOutputView()
	start := min(m.taskOutputScroll, max(len(lines)-page, 0))
	end := min(start+page, len(lines))
	var b strings.Builder
	for _, l := range lines[start:end] {
		b.WriteString(l + "\n")
	}
	footer := "esc close"
	if len(lines) > page {
		footer = fmt.Sprintf("%d-%d/%d  ·  %s", start+1, end, len(lines), footer)
	}
	return modalPanel(m.taskOutputTitle, b.String(), footer, width, theme.Mauve)
}

// keyTaskOutput handles input while the output overlay is open: up/down and
// pgup/pgdn scroll, esc closes, every other key does nothing.
func (m model) keyTaskOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines, page, _ := m.taskOutputView()
	maxScroll := max(len(lines)-page, 0)
	switch {
	case keybind.Matches(msg, keys.Back):
		m.overlay = overlayNone
		m.taskOutputLines = nil
		m.taskOutputScroll = 0
	case keybind.Matches(msg, keys.Up):
		m.taskOutputScroll = max(min(m.taskOutputScroll, maxScroll)-1, 0)
	case keybind.Matches(msg, keys.Down):
		m.taskOutputScroll = min(m.taskOutputScroll+1, maxScroll)
	case keybind.Matches(msg, boxFormPageUp):
		m.taskOutputScroll = max(min(m.taskOutputScroll, maxScroll)-page, 0)
	case keybind.Matches(msg, boxFormPageDown):
		m.taskOutputScroll = min(m.taskOutputScroll+page, maxScroll)
	}
	return m, nil
}

// taskOutputLocalBindings is the footer set while the output overlay is open.
func taskOutputLocalBindings() []keybind.Binding { return []keybind.Binding{keys.Back} }
