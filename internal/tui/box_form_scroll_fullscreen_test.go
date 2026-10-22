package tui

// box_form_scroll_fullscreen_test.go — TDD coverage for the SECOND half of
// the box-form scroll story (bean bt-s90e, epic bt-vy1q): F1 (bean bt-ze10,
// box_form_scroll_test.go) deliberately scoped its wiring to the SPLIT Detail
// pane and guarded the Vollbild path OFF (keyDetailFocus's `m.fullscreen !=
// fullscreenDetail` condition, update.go; renderFullscreenBody's literal 0
// boxScroll argument, view_fullscreen.go), because the Vollbild-Detail's
// single full-width pane is a DIFFERENT accW budget than boxFormScrollBounds
// computed from the split's own clickPaneGeometry call. These tests pin the
// closed gap: the SAME adjustBoxFormScroll/boxFormEffectiveScroll machinery
// now serves both geometries, with boxFormScrollBounds branching on
// m.fullscreen for the width half only (the height half -- bodyH, incl. the
// filter-bar reclaim -- is identical in both, viewBrowseRepo computes it
// BEFORE its own fullscreen branch).
//
// Fixture/harness pattern mirrors box_form_scroll_test.go verbatim
// (boxFormScrollModel/requireOverflow/step/keyMsg).

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// boxFormWrapSensitiveBeans is boxFormLongBodyBeans (box_form_scroll_test.go)
// plus ONE deliberately very long prose paragraph on tk-2. The 80-item
// markdown list alone renders one item per line at ANY sane width, so its
// total line count is width-INDEPENDENT -- which would make
// TestBoxFormScrollBoundsUsesFullscreenWidth below vacuously pass even against
// the old split-only geometry. The long paragraph wraps to a different line
// count at the split's accW (~54 cols at a 100-wide frame) than at the
// Vollbild's (96 cols), so the two geometries provably disagree and the test
// actually exercises the branch under test.
func boxFormWrapSensitiveBeans() []*item.Item {
	var body strings.Builder
	body.WriteString(strings.TrimSpace(strings.Repeat("wrapword ", 60)) + "\n\n")
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&body, "- BODYLINE%02d\n", i)
	}
	beans := fixtureBeans()
	for i := range beans {
		if beans[i].ID == "tk-1" || beans[i].ID == "tk-2" {
			beans[i].Body = body.String()
		}
	}
	return beans
}

// fullscreenBoxFormModel builds boxFormScrollModel's model (
// 100x30, tk-2 focused) already inside fullscreenDetail, entered through the
// REAL `v` keypress from a Detail-focused split view (keyFullscreen's own
// detailFocus branch, view_fullscreen.go) rather than by setting m.fullscreen
// by hand -- same round-trip discipline the rest of this package's update
// tests use.
func fullscreenBoxFormModel(t *testing.T, beans []*item.Item) model {
	t.Helper()
	m := fixtureModel(t, beans)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = focusBean(m, "tk-2")
	m.detailFocus = true
	m = step(t, m, runeMsg('v'))
	if m.fullscreen != fullscreenDetail {
		t.Fatalf("setup: fullscreen = %v after `v` from a Detail-focused split, want fullscreenDetail", m.fullscreen)
	}
	return m
}

// --- Keyboard drive inside the Vollbild ---

// TestBoxFormScrollDownUpInFullscreenDetail is the direct inverse of F1's own
// guard: up/down inside fullscreenDetail now move model.boxFormScroll by ±1
// through the SAME adjustBoxFormScroll the split path uses, instead of
// falling through to the (box-form-inert) Accordion nav.
func TestBoxFormScrollDownUpInFullscreenDetail(t *testing.T) {
	m := fullscreenBoxFormModel(t, boxFormLongBodyBeans())
	b := m.focusedBean()
	requireOverflow(t, m, b)

	m = step(t, m, keyMsg(tea.KeyDown))
	if m.boxFormScroll != 1 {
		t.Fatalf("boxFormScroll after one fullscreen-detail down = %d, want 1", m.boxFormScroll)
	}
	if m.boxFormScrollBean != b.ID {
		t.Fatalf("boxFormScrollBean = %q, want %q (offset must record its owning bean in the Vollbild too)", m.boxFormScrollBean, b.ID)
	}

	m = step(t, m, keyMsg(tea.KeyDown))
	if m.boxFormScroll != 2 {
		t.Fatalf("boxFormScroll after two fullscreen-detail downs = %d, want 2", m.boxFormScroll)
	}

	m = step(t, m, keyMsg(tea.KeyUp))
	if m.boxFormScroll != 1 {
		t.Fatalf("boxFormScroll after down/down/up in the Vollbild = %d, want 1", m.boxFormScroll)
	}
}

// --- Geometry: the clamp must match the Vollbild's OWN render width ---

// TestBoxFormScrollBoundsUsesFullscreenWidth guards the actual reason F1
// punted: boxFormScrollBounds must reconstruct the width the Vollbild
// ACTUALLY renders at (paneW-2 == innerW-4, view_browse_repo.go's fullscreen
// branch) rather than the split's rw-2 -- otherwise the stored offset clamps
// against a line total the render never produces, and the last lines stay
// unreachable (or the view over-scrolls into padding).
func TestBoxFormScrollBoundsUsesFullscreenWidth(t *testing.T) {
	beans := boxFormWrapSensitiveBeans()
	mSplit := boxFormScrollModel(t)
	mSplit = step(t, mSplit, beansLoadedMsg{beans: beans})
	mSplit = focusBean(mSplit, "tk-2")
	b := mSplit.focusedBean()
	if b == nil {
		t.Fatal("setup: focusedBean() == nil")
	}

	splitTotal, splitHeight := boxFormScrollBounds(mSplit, b)

	mFull := fullscreenBoxFormModel(t, beans)
	fb := mFull.focusedBean()
	fullTotal, fullHeight := boxFormScrollBounds(mFull, fb)

	// Height needs NO fullscreen branch of its own: boxFormScrollBounds
	// already derives bodyH from the CURRENT model's own chrome
	// (browseRepoChrome is footer-context-aware, so the Vollbild's localKeys
	// line legitimately differs from the split's -- hence splitHeight and
	// fullHeight need not be equal) via the SAME clickPaneGeometry call
	// viewBrowseRepo makes before its fullscreen branch, filter-bar reclaim
	// included. Only sanity-checked here; the end-to-end keyboard test below
	// is what actually proves the height half lands right.
	if fullHeight <= 0 {
		t.Fatalf("fullscreen height budget = %d, want > 0", fullHeight)
	}
	_ = splitHeight

	// The fixture is wrap-sensitive on purpose: if these agree, the test is
	// not exercising the width branch at all.
	if splitTotal == fullTotal {
		t.Fatalf("split and fullscreen totals both %d -- fixture is not wrap-sensitive, test would pass vacuously", splitTotal)
	}

	innerW := mFull.width - 2
	wantAccW := innerW - 4 // paneW(innerW-2) handed to renderDetailBox, which takes w-2 as accW
	// cursor -1: the Vollbild renders no field cursor (bt-1o4g) -- there,
	// up/down scroll the viewport instead of walking fields, so a cursor would
	// be a Mauve frame nothing can move. Line COUNT is cursor-independent
	// anyway; -1 keeps this height assertion honest about what fullscreen does.
	want := len(strings.Split(detailBoxForm(mFull.idx, fb, wantAccW, -1), "\n"))
	if fullTotal != want {
		t.Fatalf("boxFormScrollBounds total in fullscreenDetail = %d, want %d (detailBoxForm at the Vollbild's own accW %d)", fullTotal, want, wantAccW)
	}
}

// --- End-to-end through the REAL render pipeline ---

// TestBoxFormScrollFullscreenRevealsContentPreviouslyCutOff is the headline
// criterion, mirroring box_form_scroll_test.go's split-pane counterpart: the
// Body's tail is unreachable in the Vollbild before this bean and reachable
// after, exercised through m.View() rather than the helpers.
func TestBoxFormScrollFullscreenRevealsContentPreviouslyCutOff(t *testing.T) {
	m := fullscreenBoxFormModel(t, boxFormLongBodyBeans())
	b := m.focusedBean()
	total, _ := requireOverflow(t, m, b)

	before := ansi.Strip(m.View())
	if !strings.Contains(before, "BODYLINE01") {
		t.Fatal("BODYLINE01 must be visible in the unscrolled Vollbild")
	}
	if strings.Contains(before, "BODYLINE80") {
		t.Fatal("BODYLINE80 must NOT already be visible -- fixture doesn't overflow the Vollbild pane, test is not meaningful")
	}

	scrolled := m.adjustBoxFormScroll(b, total) // oversized on purpose -- clampBoxFormScroll bounds it
	after := ansi.Strip(scrolled.View())
	if !strings.Contains(after, "BODYLINE80") {
		t.Fatal("BODYLINE80 must become visible once the Vollbild is scrolled to its end")
	}
}

// TestBoxFormScrollFullscreenDrivenByKeyboardReachesEnd closes the loop
// between the two halves above: repeated `down` KEYPRESSES (not a direct
// adjustBoxFormScroll call) must actually walk the Vollbild render to its
// last line. This is what a wrong (split-derived) clamp would break --
// the offset would stop early against a foreign total.
func TestBoxFormScrollFullscreenDrivenByKeyboardReachesEnd(t *testing.T) {
	m := fullscreenBoxFormModel(t, boxFormLongBodyBeans())
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)

	for i := 0; i < total+5; i++ {
		m = step(t, m, keyMsg(tea.KeyDown))
	}
	if want := total - height; m.boxFormScroll != want {
		t.Fatalf("boxFormScroll after driving `down` past the end = %d, want the Vollbild's own ceiling %d", m.boxFormScroll, want)
	}
	if !strings.Contains(ansi.Strip(m.View()), "BODYLINE80") {
		t.Fatal("keyboard-driven scroll to the ceiling must reveal the Body's last line in the Vollbild")
	}
}

// --- Flag off: nothing about the Vollbild changes ---

// --- bean bt-8d35: the Vollbild has no field cursor to walk ---

// TestBoxFormFullscreenTabDoesNotMoveTheFieldCursor pins the deliberate
// exclusion in bt-8d35's Fokus-Modell: renderFullscreenBody passes cursor
// -1 (view_fullscreen.go), so there is no cursor rendered in the Vollbild --
// moving an invisible one would be a focus model without a visible focus.
// tab therefore keeps its GLOBAL meaning there (handleKey routes it to the
// region-local field nav only for the split geometry).
func TestBoxFormFullscreenTabDoesNotMoveTheFieldCursor(t *testing.T) {
	m := fullscreenBoxFormModel(t, boxFormLongBodyBeans())
	before := m.boxFormCursor

	m = step(t, m, keyMsg(tea.KeyTab))
	if m.fullscreen != fullscreenDetail {
		t.Fatalf("tab left the Vollbild: fullscreen = %v", m.fullscreen)
	}
	if m.boxFormCursor != before {
		t.Fatalf("tab moved the Vollbild's field cursor to %q -- the Vollbild renders none (bt-8d35)", boxFormFieldOrder[m.boxFormCursor].name)
	}
}
