package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestFocusTabShiftTabToggleBothWays pins the P4 focus model: tab and
// shift+tab both switch between the Tree and the Detail region, whichever
// region has focus.
func TestFocusTabShiftTabToggleBothWays(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")

	steps := []struct {
		key        tea.KeyType
		wantDetail bool
	}{
		{tea.KeyTab, true},
		{tea.KeyShiftTab, false},
		{tea.KeyShiftTab, true},
		{tea.KeyTab, false},
	}
	for i, s := range steps {
		m = step(t, m, keyMsg(s.key))
		if m.detailFocus != s.wantDetail {
			t.Fatalf("step %d (%v): detailFocus = %v, want %v", i, s.key, m.detailFocus, s.wantDetail)
		}
	}
}

// TestDetailUpDownMovesFieldCursor: in the Detail region down walks the
// box-form field cursor (boxFormCursor 0 -> 1), up walks it back.
func TestDetailUpDownMovesFieldCursor(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyDown))
	if m.boxFormCursor != 1 {
		t.Fatalf("boxFormCursor after down = %d, want 1", m.boxFormCursor)
	}
	m = step(t, m, keyMsg(tea.KeyUp))
	if m.boxFormCursor != 0 {
		t.Fatalf("boxFormCursor after up = %d, want 0", m.boxFormCursor)
	}
}

// TestDetailEscLeavesDetailRegion: esc drops Detail focus.
func TestDetailEscLeavesDetailRegion(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.detailFocus {
		t.Fatal("esc must leave the Detail region")
	}
}

// TestDetailHiddenAtStartAndToggle: a new model renders no Detail pane, `D`
// shows it, and tab on a hidden Detail shows and focuses it.
func TestDetailHiddenAtStartAndToggle(t *testing.T) {
	m := step(t, newModel(nil, "/tmp/bt-fixture-repo"), beansLoadedMsg{beans: fixtureBeans()})
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = focusBean(m, "tk-2")
	const detailFrame = "─ Title" // top border of the box-form Title box
	if !m.detailHidden {
		t.Fatal("a new model must start with the Detail pane hidden")
	}
	if strings.Contains(m.View(), detailFrame) {
		t.Fatal("hidden Detail pane still renders the Title box")
	}

	m = step(t, m, runeMsg('D'))
	if m.detailHidden || !strings.Contains(m.View(), detailFrame) {
		t.Fatal("D must show the Detail pane")
	}
	m = step(t, m, runeMsg('D'))
	if !m.detailHidden || strings.Contains(m.View(), detailFrame) {
		t.Fatal("second D must hide the Detail pane again")
	}

	m = step(t, m, keyMsg(tea.KeyTab))
	if m.detailHidden || !m.detailFocus {
		t.Fatalf("tab on a hidden Detail: detailHidden=%v detailFocus=%v, want false/true", m.detailHidden, m.detailFocus)
	}
}
