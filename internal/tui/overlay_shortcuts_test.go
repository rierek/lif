package tui

// overlay_shortcuts_test.go — E5 Task 2 (bean bt-wpn9): State/render tests
// for the `?` Help-Overlay, generated from keys.helpGroups() (Single-Source,
// keymap.go, T7/T8 Drift-Guard). Port devd overlay_shortcuts_test.go's
// coverage shape (~/dev/DeveloperDashboard/apps/cli-go/internal/
// tui/overlay_shortcuts_test.go), ADAPTED: devd's Help closes on ANY key
// ("beliebige Taste sollte Hilfe schließen") -- beans-tui deliberately does
// NOT port that (bean bt-wpn9 Akzeptanz + design decision h precedent):
// Help is a FULL-CAPTURE floating overlay like filterOpen/paletteOpen/
// m.form/m.overlay (handleKey's own doc comments), so only esc/?/q close it
// and every OTHER key is swallowed as a no-op, not routed anywhere else.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHelpBoxRendersEveryGroup guards the Single-Source contract (bean
// Akzeptanz): helpBox() must render every keys.helpGroups() title -- a
// hand-maintained parallel list could drift from the real keymap, this
// can't (same generation the Drift-Guard test TestHelpGroupsCoverEvery
// BindingExactlyOnce, keymap_test.go, already protects upstream of it).
func TestHelpBoxRendersEveryGroup(t *testing.T) {
	m := model{width: 90, height: 24}
	out := m.helpBox()
	if !strings.Contains(out, "Keyboard shortcuts") {
		t.Errorf("helpBox() missing title %q", "Keyboard shortcuts")
	}
	for _, g := range keys.helpGroups() {
		if !strings.Contains(out, g.title) {
			t.Errorf("helpBox() missing group title %q (Single-Source drift vs. keys.helpGroups())", g.title)
		}
		for _, b := range g.bindings {
			h := b.Help()
			if !strings.Contains(out, h.Key) {
				t.Errorf("helpBox() missing key label %q (group %q)", h.Key, g.title)
			}
			if !strings.Contains(out, h.Desc) {
				t.Errorf("helpBox() missing description %q (group %q)", h.Desc, g.title)
			}
		}
	}
}

// TestKeyHelpOpensFromAnyView guards design decision h (ctrl+k/Palette
// precedent, E4 Task 1): `?` opens Help from ANY view, not just
// viewBrowseRepo -- checked here against viewReady.
func TestKeyHelpOpensFromAnyView(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.view = viewReady
	nm := step(t, m, runeMsg('?'))
	if !nm.helpOpen {
		t.Fatal("`?` must open Help from any view (design decision h)")
	}
}

// TestKeyHelpEscQCloseHelp guards the devd footer hint "esc/?/q: close"
// (Port VERBATIM, plan Step 3) and the three keys that actually close Help.
func TestKeyHelpEscQCloseHelp(t *testing.T) {
	m := model{width: 90, height: 24}
	out := m.helpBox()
	if !strings.Contains(out, "esc/?/q: close") {
		t.Fatalf("helpBox() footer missing literal devd hint %q, got: %s", "esc/?/q: close", out)
	}

	for _, msg := range []tea.KeyMsg{keyMsg(tea.KeyEsc), runeMsg('?'), runeMsg('q')} {
		hm := model{helpOpen: true}
		tm, cmd := hm.keyHelp(msg)
		nm, ok := tm.(model)
		if !ok {
			t.Fatalf("keyHelp(%v) did not return a model, got %T", msg, tm)
		}
		if nm.helpOpen {
			t.Errorf("keyHelp(%v) left helpOpen=true, want it closed", msg)
		}
		if cmd != nil {
			t.Errorf("keyHelp(%v) returned a non-nil Cmd, want nil (closing Help fires no async work)", msg)
		}
	}
}

// TestHelpCapturesSingleKeysWhileOpen guards full-capture semantics (same
// precedent as filterOpen/paletteOpen/m.form/m.overlay, handleKey's own doc
// comments): `q` while Help is open must NOT ALSO fall through to the
// quit-confirm switch, and any other key (not esc/?/q) is a no-op that
// leaves Help open and leaks through to NOTHING else (no overlay, no
// palette, no quit-confirm) -- a deliberate deviation from devd, which
// closes Help on any key (see file doc comment above).
func TestHelpCapturesSingleKeysWhileOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.helpOpen = true

	nm := step(t, m, runeMsg('q'))
	if nm.helpOpen {
		t.Fatal("`q` must close Help")
	}
	if nm.confirmQuit {
		t.Fatal("`q` while Help is open must not ALSO trigger quit-confirm (full capture)")
	}

	// bt-mx4k: the palette probe is `K`, not the retired ctrl+k -- an
	// unbound key can no longer prove that Help captures it.
	for _, msg := range []tea.KeyMsg{runeMsg('c'), runeMsg('s'), runeMsg('K')} {
		nm2 := step(t, m, msg) // m.helpOpen is still true here
		if !nm2.helpOpen {
			t.Errorf("%v closed Help, want it to stay open (only esc/?/q close it)", msg)
		}
		if nm2.overlay != overlayNone {
			t.Errorf("%v leaked through to keyNodeAction while Help open (overlay = %v)", msg, nm2.overlay)
		}
		if nm2.paletteOpen {
			t.Errorf("%v leaked through to openPalette while Help open", msg)
		}
		if nm2.confirmQuit {
			t.Errorf("%v leaked through to quit-confirm while Help open", msg)
		}
	}
}

// TestKeyHelpUnreachableWhilePaletteOpen guards the ERRATUM fix vs. the
// epic-plan's "Geteilte Infrastruktur" capture-order sketch (which listed
// m.helpOpen/keys.Help BEFORE m.paletteOpen -- see bean bt-wpn9 Deviations):
// that ordering would let `?`, typed as a literal query character into an
// open Command-Center filter, hijack input and open Help instead. The
// correct order (this task's own Step 3, verified here) keeps
// m.paletteOpen's existing full capture ahead of the new Help checks, same
// as every other full-capture state in handleKey.
func TestKeyHelpUnreachableWhilePaletteOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	nm, _ := m.openPalette()
	m, ok := nm.(model)
	if !ok || !m.paletteOpen {
		t.Fatal("setup: openPalette() must set paletteOpen")
	}

	m = step(t, m, runeMsg('?'))
	if m.helpOpen {
		t.Fatal("`?` opened Help while the palette was open -- capture order violated (ERRATUM fix, see bt-wpn9 Deviations)")
	}
	if !m.paletteOpen {
		t.Fatal("paletteOpen was cleared -- `?` must have been swallowed by keyPalette as a query character, not routed to Help")
	}
	if m.palQuery != "?" {
		t.Fatalf("palQuery = %q, want %q (`?` must reach the palette's query, not open Help)", m.palQuery, "?")
	}
}
