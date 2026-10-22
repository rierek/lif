package tui

// notifications_toast_only_test.go — TDD coverage for bt-81f0
// (Notifications vereinheitlichen: Toast als einziger Kanal, epic-E12-plan.md
// »Item 2«). D02/Q1 (bean body Prelude): m.err stays as a FIELD (43
// pre-existing assertions elsewhere keep reading it), but every RENDER path
// that used to surface it (ChromeOpts.ErrNote/statusBar's errNote param,
// view.go) must stop reading it -- the status line survives only as the
// scroll-indicator slot (Q1-Annahme: no dynamic footer height, no layout
// rework). This file pins the render-side half of the Akzeptanz ("untere
// reservierte Zeile entfernt [Fehler-Anbindung], Toast ist der einzige
// sichtbare Kanal") across all three top-level views that call statusBar
// directly (viewBrowseRepo/viewReady/viewTagManagement) plus Chrome
// itself. The Update-side half (the seven ex-silent m.err-only sites now
// also firing a Toast) is covered per-site in each of their own owning
// _test.go files (box_confirm_delete_test.go et al.), not duplicated here.
import (
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// errMarker is a distinctive, never-otherwise-emitted string used to prove
// m.err's ABSENCE from a rendered frame without false-positiving on some
// unrelated substring collision.
const errMarker = "BT81F0-ERR-MARKER-must-not-render"

// TestChromeStatusBarNeverRendersErrNote guards Chrome/statusBar directly
// (view.go): ChromeOpts no longer carries anything that reaches errNote --
// a Chrome render with m.err-shaped content threaded through must not show
// it. Exercised via goldenChromeOpts (chrome_test.go) plus the errMarker.
func TestChromeStatusBarNeverRendersErrNote(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	o := goldenChromeOpts()
	out := Chrome(o)
	if strings.Contains(out, errMarker) {
		t.Fatalf("Chrome() output contains %q, want it never rendered (bt-81f0: ErrNote/errNote wiring removed)", errMarker)
	}
}

// TestViewBrowseRepoNeverRendersMErr guards viewBrowseRepo's own statusBar
// call (view_browse_repo.go): m.err set to errMarker must never reach the
// rendered frame, regardless of any legitimate scroll-indicator content
// sharing the same status line slot.
func TestViewBrowseRepoNeverRendersMErr(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	beans := []*item.Item{
		{ID: "gld-mlst", Title: "Golden Milestone", Status: "in-progress", Type: "milestone", Priority: "high"},
	}
	m := newModel(nil, "/tmp/bt-81f0-scratch-repo")
	m = step(t, m, beansLoadedMsg{beans: beans})
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.err = errMarker

	out := m.View()
	if strings.Contains(out, errMarker) {
		t.Fatalf("viewBrowseRepo's View() output contains m.err (%q), want it never rendered (bt-81f0)", errMarker)
	}
}

// TestViewReadyNeverRendersMErr mirrors TestViewBrowseRepoNeverRendersMErr
// for view_browse_ready.go's own statusBar call.
func TestViewReadyNeverRendersMErr(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	beans := []*item.Item{
		{ID: "gld-tsk1", Title: "Ready-eligible task", Status: "todo", Type: "task", Priority: "normal"},
	}
	m := newModel(nil, "/tmp/bt-81f0-scratch-repo")
	m = step(t, m, beansLoadedMsg{beans: beans})
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.view = viewReady
	m.err = errMarker

	out := m.View()
	if strings.Contains(out, errMarker) {
		t.Fatalf("viewReady's View() output contains m.err (%q), want it never rendered (bt-81f0)", errMarker)
	}
}

// TestViewTagManagementNeverRendersMErr mirrors the above for
// view_tag_management.go's own statusBar call.
func TestViewTagManagementNeverRendersMErr(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	beans := []*item.Item{
		{ID: "gld-tsk1", Title: "Some task", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"backend"}},
	}
	m := newModel(nil, "/tmp/bt-81f0-scratch-repo")
	m = step(t, m, beansLoadedMsg{beans: beans})
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.view = viewTagManagement
	m.err = errMarker

	out := m.View()
	if strings.Contains(out, errMarker) {
		t.Fatalf("viewTagManagement's View() output contains m.err (%q), want it never rendered (bt-81f0)", errMarker)
	}
}
