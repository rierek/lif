package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// focusAt names the focus target: "list", "filter:<field>" or "detail:<box>".
func focusAt(m model) string {
	switch {
	case m.filterFocus:
		return "filter:" + filterFields[m.filterField]
	case m.detailFocus:
		return "detail:" + boxFormFieldOrder[boxFormEffectiveCursor(m, m.focusedBean())].name
	}
	return "list"
}

// ringTargets is the expected ring of layout l, built from the production
// field tables.
func ringTargets(l layoutMode) []string {
	var out []string
	if l.hasList() {
		out = append(out, "list")
	}
	if l.hasFilter() {
		for _, f := range filterFields {
			out = append(out, "filter:"+f)
		}
	}
	if l.hasDetail() {
		for _, f := range boxFormFieldOrder {
			out = append(out, "detail:"+f.name)
		}
	}
	return out
}

// tabIntoDetail presses tab until a detail field holds the focus, the way a
// user reaches the detail from the list (past the filter fields, when the
// layout has them). It fails when the ring has no detail field.
func tabIntoDetail(t *testing.T, m model) model {
	t.Helper()
	for range 20 {
		if m.detailFocus {
			return m
		}
		m = step(t, m, keyMsg(tea.KeyTab))
	}
	t.Fatal("tab never reached a detail field")
	return m
}

// TestFocusRingTabVisitsEveryTargetAndWraps pins AC1-AC7 and SC-01: tab walks
// the ring in order and wraps, shift+tab walks it backward and wraps, in
// every layout with a ring, for Browse and Ready, and no key changes the
// layout (AC8).
func TestFocusRingTabVisitsEveryTargetAndWraps(t *testing.T) {
	for _, l := range []layoutMode{layoutFilter, layoutFilterDetail, layoutDetail} {
		ring := ringTargets(l)
		for name, m := range layoutModels(t, l) {
			t.Run(layoutName(l)+"/"+name, func(t *testing.T) {
				start := focusAt(m)
				at := -1
				for i, target := range ring {
					if target == start {
						at = i
					}
				}
				if at < 0 {
					t.Fatalf("start focus %q is not in the ring %v", start, ring)
				}
				f := m
				for i := 1; i <= len(ring)+1; i++ {
					f = step(t, f, keyMsg(tea.KeyTab))
					if want := ring[(at+i)%len(ring)]; focusAt(f) != want || f.layout != l {
						t.Fatalf("tab #%d: focus %q layout %v, want %q layout %v", i, focusAt(f), f.layout, want, l)
					}
				}
				b := m
				for i := 1; i <= len(ring)+1; i++ {
					b = step(t, b, keyMsg(tea.KeyShiftTab))
					if want := ring[((at-i)%len(ring)+len(ring))%len(ring)]; focusAt(b) != want || b.layout != l {
						t.Fatalf("shift+tab #%d: focus %q layout %v, want %q layout %v", i, focusAt(b), b.layout, want, l)
					}
				}
			})
		}
	}
}

// TestFocusRingRoadmapIgnoresTab pins AC10: with no other target tab and
// shift+tab change neither focus nor layout.
func TestFocusRingRoadmapIgnoresTab(t *testing.T) {
	for name, m := range layoutModels(t, layoutRoadmap) {
		for _, k := range []tea.KeyType{tea.KeyTab, tea.KeyShiftTab} {
			got := step(t, m, keyMsg(k))
			if got.layout != layoutRoadmap || focusAt(got) != "list" || got.filterFocus || got.detailFocus {
				t.Fatalf("%s %v: layout=%v focus=%q, want roadmap/list", name, k, got.layout, focusAt(got))
			}
		}
	}
}

// TestFocusRingDetailFieldIsVisible pins AC9: tab and shift+tab onto a
// detail field put the field cursor there and scroll it into the viewport.
func TestFocusRingDetailFieldIsVisible(t *testing.T) {
	m := layoutModels(t, layoutFilterDetail)["browse"]
	m = step(t, m, tea.WindowSizeMsg{Width: layoutW, Height: 18})
	for _, k := range []tea.KeyType{tea.KeyShiftTab, tea.KeyShiftTab, tea.KeyShiftTab, tea.KeyTab} {
		m = step(t, m, keyMsg(k))
		if !m.detailFocus {
			t.Fatalf("%v: focus %q, want a detail field", k, focusAt(m))
		}
		b := m.focusedBean()
		accW, height := boxFormPaneMetrics(m, b)
		start, h := boxFormFieldSpan(boxFormBlocks(m.idx, b, accW, -1), boxFormEffectiveCursor(m, b))
		off := boxFormEffectiveScroll(m, b)
		if start+min(h, height) <= off || start >= off+height {
			t.Fatalf("%v: field %q rows [%d,%d) outside viewport [%d,%d)", k, focusAt(m), start, start+h, off, off+height)
		}
	}
}

// TestFocusRingClickMovesTabStart: tab continues from the focus a click or
// key set, because the ring position is derived from the model state.
func TestFocusRingClickMovesTabStart(t *testing.T) {
	m := layoutModels(t, layoutFilterDetail)["browse"].focusFilterField("priority")
	m = step(t, m, keyMsg(tea.KeyTab))
	if got := focusAt(m); got != "filter:tag" {
		t.Fatalf("tab from the priority field = %q, want filter:tag", got)
	}
}

// TestFocusTabDoesNotReachOtherViews: tab outside Browse and Ready never
// moves ring state.
func TestFocusTabDoesNotReachOtherViews(t *testing.T) {
	m := layoutModels(t, layoutFilterDetail)["browse"]
	m.view = viewLobby
	if got := m.stepFocus(1); focusAt(got) != focusAt(m) {
		t.Fatalf("stepFocus in the Lobby moved focus to %q", focusAt(got))
	}
}

// TestDetailUpDownMovesFieldCursor: in the Detail region down walks the
// box-form field cursor (boxFormCursor 0 -> 1), up walks it back.
func TestDetailUpDownMovesFieldCursor(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = tabIntoDetail(t, m)
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
	m = tabIntoDetail(t, m)
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
