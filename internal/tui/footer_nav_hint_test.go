package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const navHintText = "[ ] sibling · zo zc open/close · zR zM all open/close"

func navHintModel(t *testing.T, w int) model {
	t.Helper()
	m := fixtureModel(t, fixtureBeans())
	return step(t, m, tea.WindowSizeMsg{Width: w, Height: 40})
}

func footerLineOf(m model) string {
	for _, l := range screenLines(m) {
		if strings.Contains(l, "next layout") || strings.Contains(l, "sibling") {
			return l
		}
	}
	return ""
}

func TestRoadmapNavHintShownRightInWideTree(t *testing.T) {
	m := navHintModel(t, 300)
	l := strings.NewReplacer("\u00a0", " ").Replace(footerLineOf(m))
	if !strings.Contains(l, navHintText+" │") && !strings.Contains(l, navHintText) {
		t.Fatalf("footer %q lacks %q", l, navHintText)
	}
	if !strings.HasSuffix(strings.TrimRight(l, " │"), "all open/close") {
		t.Errorf("hint not right-aligned: %q", l)
	}
	if !strings.Contains(l, "tab next focus") {
		t.Errorf("left list lost: %q", l)
	}
}

func TestRoadmapNavHintLabelsComeFromKeymap(t *testing.T) {
	got := stripHint(m0().roadmapNavHint())
	for _, b := range []string{keys.PrevSibling.Help().Key, keys.NextSibling.Help().Key,
		keys.ExpandNode.Help().Key, keys.CollapseNode.Help().Key,
		keys.ExpandAll.Help().Key, keys.CollapseAll.Help().Key} {
		if !strings.Contains(got, b) {
			t.Errorf("hint %q lacks key %q", got, b)
		}
	}
}

func m0() model { return newModel(nil, "/tmp/bt-fixture-repo") }

func TestRoadmapNavHintAbsentWhereNotApplicable(t *testing.T) {
	flat := navHintModel(t, 300)
	flat.flatView = true
	ready := navHintModel(t, 300)
	ready.view = viewReady
	overlay := navHintModel(t, 300)
	overlay.overlay = overlayNone + 1
	narrow := navHintModel(t, 60)
	for name, m := range map[string]model{"flat": flat, "ready": ready, "overlay": overlay, "narrow": narrow} {
		if strings.Contains(ansi.Strip(m.View()), "sibling") {
			t.Errorf("%s: nav hint must be absent", name)
		}
	}
}

func TestRoadmapNavHintDroppedWhenGapTooSmall(t *testing.T) {
	m := navHintModel(t, 300)
	_, left := m.browseRepoChrome(500)
	lw := ansi.StringWidth(left)
	rw := ansi.StringWidth(m.roadmapNavHint())
	fit := m.withRoadmapNavHint(left, lw+rw+roadmapNavMinGap)
	if !strings.Contains(stripHint(fit), "sibling") {
		t.Error("hint missing at exact fit")
	}
	if m.withRoadmapNavHint(left, lw+rw+roadmapNavMinGap-1) != left {
		t.Error("hint must drop when gap < min, left unchanged")
	}
}

func TestRoadmapNavHintKeepsChromeLeftAndHeight(t *testing.T) {
	m := navHintModel(t, 300)
	_, a := m.browseRepoChrome(298)
	if strings.Contains(stripHint(a), "sibling") {
		t.Error("browseRepoChrome must stay byte-equal (left only)")
	}
	if strings.Count(m.withRoadmapNavHint(a, 298), "\n") != strings.Count(a, "\n") {
		t.Error("footer height changed")
	}
}

// A long left list wraps; the hint then sits on the last line, height equal.
func TestRoadmapNavHintOnLastLineOfWrappedFooter(t *testing.T) {
	m := navHintModel(t, 140)
	_, left := m.browseRepoChrome(138)
	if !strings.Contains(left, "\n") {
		t.Fatal("fixture: left footer should wrap at 140")
	}
	got := m.withRoadmapNavHint(left, 138)
	if !strings.HasPrefix(got, left) || strings.Count(got, "\n") != strings.Count(left, "\n") {
		t.Errorf("left text or height changed: %q", got)
	}
	if !strings.HasSuffix(stripHint(got), "all open/close") {
		t.Errorf("hint missing at 140: %q", stripHint(got))
	}
}
