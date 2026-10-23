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

// TestStartLayoutShowsNoDetailAndTabNeverChangesLayout: a new model starts in
// layoutRoadmap with no Detail pane; tab and shift+tab only move focus, so
// with no Detail pane they change neither layout nor focus.
func TestStartLayoutShowsNoDetailAndTabNeverChangesLayout(t *testing.T) {
	m := step(t, newModel(nil, "/tmp/bt-fixture-repo"), beansLoadedMsg{beans: fixtureBeans()})
	m = step(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m = focusBean(m, "tk-2")
	const detailFrame = "─ Title" // top border of the box-form Title box
	if m.layout != layoutRoadmap {
		t.Fatalf("a new model must start in layoutRoadmap, got %v", m.layout)
	}
	if strings.Contains(m.View(), detailFrame) {
		t.Fatal("start layout still renders the Title box of the Detail pane")
	}

	for _, k := range []tea.KeyType{tea.KeyTab, tea.KeyShiftTab} {
		m = step(t, m, keyMsg(k))
		if m.layout != layoutRoadmap || m.detailFocus {
			t.Fatalf("%v: layout=%v detailFocus=%v, want layoutRoadmap/false", k, m.layout, m.detailFocus)
		}
	}
}
