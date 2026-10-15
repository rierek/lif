package tui

// footer_status_line_test.go — bean bt-oqsv (Nebenbefund N6, epic bt-vy1q):
// the footer used to reserve a permanently blank status line for
// notifications. Toast (overlay_show_toast.go) is the ONE notification
// channel since bt-81f0, so the reservation was dead pixels -- one screen
// line that now goes back to the panes. The line is rendered ONLY when it
// actually carries the watch-unavailable indicator.
//
// The Grounding's own headline pitfall is guarded here too: clickPaneGeometry
// (mouse.go) derives footH from the SAME status string the view composes, so
// a click can never land one row off (the "+2 vergessen"-Falle).

import (
	"fmt"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tui/data"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// frameLines renders m.View() ANSI-stripped, one entry per screen row.
func frameLines(m model) []string {
	return strings.Split(stripHint(m.View()), "\n")
}

// longTreeModel builds a flat 60-bean tree at 100x30 -- far taller than the
// pane, so treeRows/treeClickRow both go through windowStart(bodyH) and a
// wrong bodyH actually changes which node a screen row maps to. Cursor parked
// deep in the list so the window is scrolled, not pinned at the top.
func longTreeModel(t *testing.T) model {
	t.Helper()
	beans := make([]data.Bean, 60)
	for i := range beans {
		beans[i] = data.Bean{ID: fmt.Sprintf("gld-r%03d", i), Title: fmt.Sprintf("Row %03d", i), Status: "todo", Type: "task"}
	}
	m := newModel(nil, "/tmp/bt-golden-repo")
	m = step(t, m, beansLoadedMsg{beans: beans})
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.cursorID = "gld-r045"
	return m
}

// TestBrowseRepoFooterHasNoReservedBlankLine: the row directly above the
// outer bottom border must be the footer's own last line, not a blank
// placeholder.
func TestBrowseRepoFooterHasNoReservedBlankLine(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	lines := frameLines(goldenTreeModel(t))
	last := lines[len(lines)-2] // -1 is the bottom border row
	inner := strings.Trim(last, "│")
	if strings.TrimSpace(inner) == "" {
		t.Fatalf("footer still reserves a blank line above the bottom border: %q", last)
	}
}

// TestBrowseRepoWatchIndicatorKeepsItsOwnLine: the status line is not gone,
// only unreserved -- when the watcher fails it must still show up, and the
// frame must still fill exactly the terminal height (Golden Rule #1).
func TestBrowseRepoWatchIndicatorKeepsItsOwnLine(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenTreeModel(t)
	m.watchUnavailable = true
	out := m.View()
	if h := lipgloss.Height(out); h != 30 {
		t.Fatalf("frame height = %d, want 30 even with the status line present", h)
	}
	if !strings.Contains(stripHint(out), "watch unavailable") {
		t.Fatalf("watch-unavailable indicator vanished with the reserved line:\n%s", stripHint(out))
	}
}

// TestBrowseRepoPanesGainTheFreedLine: removing the reservation must widen
// the pane budget by exactly one row, not shrink the frame.
func TestBrowseRepoPanesGainTheFreedLine(t *testing.T) {
	m := goldenTreeModel(t)
	head, localKeys := m.browseRepoChrome(m.width - 2)

	withStatus, _, _, _, _ := clickPaneGeometry(m.width, m.height, head, localKeys, "indicator", m.settings.Layout.TreeWidth)
	withoutStatus, _, _, _, _ := clickPaneGeometry(m.width, m.height, head, localKeys, "", m.settings.Layout.TreeWidth)

	if withoutStatus != withStatus+1 {
		t.Fatalf("bodyH without status line = %d, want %d (exactly one row more than with it: %d)", withoutStatus, withStatus+1, withStatus)
	}
}

// TestClickPaneGeometryBodyHMatchesRenderedPane pins clickPaneGeometry's
// bodyH to the pane height the view ACTUALLY renders -- the direct guard for
// the Grounding's headline pitfall. A stale footH constant (e.g. keeping the
// pre-bt-oqsv "+2" after the reserved status line was dropped) makes the
// hit-test believe the pane is one row shorter than it is drawn, which
// silently offsets every windowed click. Counting the rows between the left
// pane's own top and bottom border in the rendered frame catches exactly
// that, independent of how many beans happen to be on screen.
func TestClickPaneGeometryBodyHMatchesRenderedPane(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	for _, watchDown := range []bool{false, true} {
		m := goldenTreeModel(t)
		m.watchUnavailable = watchDown

		head, localKeys := m.browseRepoChrome(m.width - 2)
		bodyH, _, _, _, originY := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth)

		lines := frameLines(m)
		// The frame must still fill the terminal exactly. This is where a
		// stale footH bites: the view reserves footH rows for the footer but
		// emits only as many as it composes, so a "+2" that no longer matches
		// the (now optional) status line shortens the whole frame by a row.
		if len(lines) != m.height {
			t.Fatalf("watchUnavailable=%v: frame height = %d, want %d -- footH no longer matches the composed footer", watchDown, len(lines), m.height)
		}
		rendered := 0
		for y := originY; y < len(lines); y++ {
			if strings.Contains(lines[y], "╰") {
				break
			}
			rendered++
		}
		if rendered != bodyH {
			t.Fatalf("watchUnavailable=%v: rendered left-pane content rows = %d, clickPaneGeometry bodyH = %d -- hit-test and render disagree", watchDown, rendered, bodyH)
		}
	}
}

// TestTreeClickRowSurvivesStatusLineRemoval is the Grounding's explicit
// regression demand: a click on a tree row's ACTUAL screen position must
// still resolve to that row -- with the status line absent (normal) AND
// present (watch unavailable), which shift the pane by one row relative to
// each other.
//
// It works on a list LONGER than the pane on purpose: only then does bodyH
// feed windowStart, so a wrong footH shifts which node a given screen row
// belongs to (a short list maps correctly even with a broken pane height,
// which is exactly why the first draft of this test passed a mutated footH).
// Nodes outside the current window simply have no screen row -- they are
// skipped, and the assertion at the end pins that a meaningful number of
// rows was actually checked.
func TestTreeClickRowSurvivesStatusLineRemoval(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	for _, watchDown := range []bool{false, true} {
		m := longTreeModel(t)
		m.watchUnavailable = watchDown
		nodes := m.visibleNodes()
		lines := frameLines(m)

		// Rows are matched inside the LEFT pane's own columns only -- the
		// Detail pane repeats the selected bean's ID in its header, which
		// would otherwise match on the wrong screen row.
		head, localKeys := m.browseRepoChrome(m.width - 2)
		_, lw, _, originX, _ := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth)

		checked := 0
		for want, n := range nodes {
			if n.bean == nil {
				continue
			}
			screenY := -1
			for y, ln := range lines {
				r := []rune(ln)
				if len(r) < originX+lw {
					continue
				}
				if strings.Contains(string(r[originX:originX+lw]), n.bean.ID) {
					screenY = y
					break
				}
			}
			if screenY < 0 {
				continue // outside the current window -- no screen row to click
			}
			checked++
			msg := tea.MouseMsg{X: 3, Y: screenY, Type: tea.MouseLeft}
			got, ok := treeClickRow(m, nodes, msg)
			if !ok || got != want {
				t.Fatalf("watchUnavailable=%v: click on screen row %d (%q) -> idx=%d ok=%v, want idx=%d",
					watchDown, screenY, n.bean.ID, got, ok, want)
			}
		}
		if checked < 15 {
			t.Fatalf("watchUnavailable=%v: only %d rows were on screen -- fixture no longer exercises the windowed path", watchDown, checked)
		}
	}
}
