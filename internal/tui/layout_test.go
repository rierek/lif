package tui

// layout_test.go — the v / shift+v layout cycle (layout.go): order, wrap,
// what each layout renders, focus on layout change, Browse and Ready.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	detailMarker = "─ Title" // top border of the box-form Title box (Detail pane)
	layoutW      = 100
	layoutH      = 30
)

var cycleOrder = []layoutMode{layoutRoadmap, layoutFilter, layoutFilterDetail, layoutDetail}

// layoutModels returns a Browse model and a Ready model, both sized and
// focused on a bean, in layout l.
func layoutModels(t *testing.T, l layoutMode) map[string]model {
	t.Helper()
	browse := fixtureModel(t, fixtureBeans())
	browse = step(t, browse, tea.WindowSizeMsg{Width: layoutW, Height: layoutH})
	browse = focusBean(browse, "tk-2")

	ready := readyBeansModel(t)
	ready.view = viewReady
	ready = step(t, ready, tea.WindowSizeMsg{Width: layoutW, Height: layoutH})
	ready.readyList.setLen(len(ready.readyVisible()))

	out := map[string]model{"browse": browse.setLayout(l), "ready": ready.setLayout(l)}
	return out
}

func TestLayoutStartsRoadmapAndVChangesIt(t *testing.T) {
	m := step(t, newModel(nil, "/tmp/bt-fixture-repo"), beansLoadedMsg{beans: fixtureBeans()})
	if m.layout != layoutRoadmap {
		t.Fatalf("start layout = %v, want layoutRoadmap", m.layout)
	}
	before := m.View()
	m = step(t, m, runeMsg('v'))
	if m.layout == layoutRoadmap || m.View() == before {
		t.Fatal("v in the start layout must change the layout and the screen")
	}
}

func TestLayoutCycleForwardAndBackwardWrap(t *testing.T) {
	for name, m := range layoutModels(t, layoutRoadmap) {
		t.Run(name, func(t *testing.T) {
			f := m
			for i := 1; i <= len(cycleOrder); i++ {
				f = step(t, f, runeMsg('v'))
				if want := cycleOrder[i%len(cycleOrder)]; f.layout != want {
					t.Fatalf("v #%d: layout = %v, want %v", i, f.layout, want)
				}
			}
			b := m
			for i := 1; i <= len(cycleOrder); i++ {
				b = step(t, b, runeMsg('V'))
				if want := cycleOrder[(len(cycleOrder)-i)%len(cycleOrder)]; b.layout != want {
					t.Fatalf("shift+v #%d: layout = %v, want %v", i, b.layout, want)
				}
			}
		})
	}
}

func TestLayoutRendersFilterBarAndDetailPerLayout(t *testing.T) {
	cases := []struct {
		l              layoutMode
		filter, detail bool
	}{
		{layoutRoadmap, false, false},
		{layoutFilter, true, false},
		{layoutFilterDetail, true, true},
		{layoutDetail, false, true},
	}
	for _, c := range cases {
		for name, m := range layoutModels(t, c.l) {
			out := ansi.Strip(m.View())
			bar := strings.SplitN(ansi.Strip(m.filterBar(layoutW-2)), "\n", 2)[0]
			if got := strings.Contains(out, bar); got != c.filter {
				t.Errorf("%s layout %v: filter bar shown = %v, want %v", name, c.l, got, c.filter)
			}
			if got := strings.Contains(out, detailMarker); got != c.detail {
				t.Errorf("%s layout %v: detail shown = %v, want %v", name, c.l, got, c.detail)
			}
		}
	}
}

func TestLayoutFitsOuterFrame(t *testing.T) {
	for _, l := range cycleOrder {
		for name, m := range layoutModels(t, l) {
			lines := strings.Split(m.View(), "\n")
			if len(lines) > layoutH {
				t.Fatalf("%s layout %v: View() has %d lines, want <= %d", name, l, len(lines), layoutH)
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > layoutW {
					t.Fatalf("%s layout %v: line %d width = %d, want <= %d", name, l, i, w, layoutW)
				}
			}
		}
	}
}

func TestLayoutChangeMovesFocusIntoShownRegion(t *testing.T) {
	cases := []struct {
		name            string
		from            layoutMode
		focusDetail     bool
		key             rune
		to              layoutMode
		wantDetailFocus bool
	}{
		{"detail-pane focus, next to detail layout", layoutFilterDetail, true, 'v', layoutDetail, true},
		{"list focus, next to detail layout", layoutFilterDetail, false, 'v', layoutDetail, true},
		{"detail layout wraps to roadmap", layoutDetail, true, 'v', layoutRoadmap, false},
		{"back from split with detail focus", layoutFilterDetail, true, 'V', layoutFilter, false},
		{"roadmap back to detail layout", layoutRoadmap, false, 'V', layoutDetail, true},
		{"split keeps list focus", layoutFilter, false, 'v', layoutFilterDetail, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := layoutModels(t, c.from)["browse"]
			m.detailFocus = c.focusDetail
			m = step(t, m, runeMsg(c.key))
			if m.layout != c.to || m.detailFocus != c.wantDetailFocus {
				t.Fatalf("layout=%v detailFocus=%v, want %v/%v", m.layout, m.detailFocus, c.to, c.wantDetailFocus)
			}
		})
	}
}

func TestTabAndShiftTabNeverChangeLayout(t *testing.T) {
	for _, l := range cycleOrder {
		m := layoutModels(t, l)["browse"]
		for _, k := range []tea.KeyType{tea.KeyTab, tea.KeyShiftTab} {
			if got := step(t, m, keyMsg(k)); got.layout != l {
				t.Fatalf("layout %v: %v changed the layout to %v", l, k, got.layout)
			}
		}
	}
}

func TestKeyDIsUnbound(t *testing.T) {
	for _, g := range keys.helpGroups() {
		for _, b := range g.bindings {
			for _, k := range b.Keys() {
				if k == "D" {
					t.Fatalf("help group %q still binds D", g.title)
				}
			}
		}
	}
	m := layoutModels(t, layoutRoadmap)["browse"]
	if got := step(t, m, runeMsg('D')); got.layout != layoutRoadmap {
		t.Fatalf("D changed the layout to %v", got.layout)
	}
}

func TestLayoutKeysIgnoredInLobby(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.view = viewLobby
	want := m.layout
	if got := step(t, m, runeMsg('v')); got.layout != want {
		t.Fatalf("v in the Lobby changed the layout to %v", got.layout)
	}
}

func TestLayoutDetailHoldsFocusOnEsc(t *testing.T) {
	m := layoutModels(t, layoutDetail)["browse"]
	m = step(t, m, keyMsg(tea.KeyEsc))
	if !m.detailFocus {
		t.Fatal("esc in layoutDetail dropped Detail focus: keys would reach the hidden list")
	}
}
