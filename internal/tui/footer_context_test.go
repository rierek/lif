package tui

// footer_context_test.go — TDD coverage for PF-11's context-sensitive
// footer (Q04-Antwort, design-spec.md §15, epic-E7-plan.md Task 7 Step 6,
// bean bt-m6at): footer_context.go's contextualLocalHint swaps the base
// view's own footer to the ACTIVE capture state's bindings the instant one
// fully captures input (handleKey's own precedent, update.go) -- the
// view-local bindings underneath are then non-functional and would mislead.

import (
	"strings"
	"testing"

	keybind "github.com/charmbracelet/bubbles/key"
)

// viewLocalStub is a deliberately distinct sentinel binding list --
// contextualLocalHint's tests assert EITHER this sentinel shows (default
// case) OR it does NOT show (every context-capture case), so a bug that
// falls through to the wrong branch is caught either way.
func viewLocalStub() []keybind.Binding {
	return []keybind.Binding{keybind.NewBinding(keybind.WithKeys("z"), keybind.WithHelp("z", "stub-view-local"))}
}

func TestContextualLocalHintDefaultUsesViewLocal(t *testing.T) {
	m := model{}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if !strings.Contains(got, "z stub-view-local") {
		t.Errorf("default state: contextualLocalHint = %q, want it to contain the viewLocal stub hint", got)
	}
}

func TestContextualLocalHintOverlayFilterMenu(t *testing.T) {
	m := model{overlay: overlayFilterMenu}
	got := stripHint(m.contextualLocalHint(viewLocalStub()))
	if strings.Contains(got, "stub-view-local") {
		t.Errorf("overlayFilterMenu: contextualLocalHint = %q, must NOT contain the (irrelevant) viewLocal stub", got)
	}
	if !strings.Contains(got, "space/x Toggle facet") || !strings.Contains(got, "enter apply") {
		t.Errorf("overlayFilterMenu: contextualLocalHint = %q, want the toggle and apply hints (Q04)", got)
	}
	if strings.Contains(got, "category") {
		t.Errorf("overlayFilterMenu: contextualLocalHint = %q, the menu has no category tabs any more", got)
	}
}

// TestContextualLocalHintFilterFieldFocus: AC8 -- while the focus is in a
// filter field the footer shows that field's keys, not the list's.
func TestContextualLocalHintFilterFieldFocus(t *testing.T) {
	m := model{filterFocus: true}
	got := stripHint(m.contextualLocalHint(viewLocalStub()))
	if strings.Contains(got, "stub-view-local") {
		t.Errorf("filterFocus: contextualLocalHint = %q, must NOT contain the viewLocal stub", got)
	}
	for _, want := range []string{"←/→ field", "enter values", "X Clear filters", "esc back"} {
		if !strings.Contains(got, want) {
			t.Errorf("filterFocus: contextualLocalHint = %q, want it to contain %q", got, want)
		}
	}
}

func TestContextualLocalHintOverlayValueMenu(t *testing.T) {
	m := model{overlay: overlayValueMenu}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if strings.Contains(got, "stub-view-local") {
		t.Errorf("overlayValueMenu: contextualLocalHint = %q, must not contain the viewLocal stub", got)
	}
	if !strings.Contains(got, "s Status") {
		t.Errorf("overlayValueMenu: contextualLocalHint = %q, want it to contain keys.Status (real close-alias, keyValueMenu)", got)
	}
}

// TestContextualLocalHintOverlayTagPickerShowsToggle guards a deliberate
// DEVIATION from epic-E7-plan.md Task 7 Step 6's literal text (which lumps
// Tag-/Parent-/Blocking-Picker into one Toggle-free {Up,Down,Enter,Back}
// set): keyTagPicker (box_picker_tag.go) really wires a toggle -- omitting
// it here would hide a real, working key AND leave Q04's own general
// wording ("wenn ein Form/Overlay aktiv ist ... inkl. space:
// select/toggle") only half addressed. Post-Review-R1 B01 (bean bt-9ipw,
// ERRATUM/D01-Nachtrag) the displayed binding is the SPACE-ONLY
// keys.TagToggle -- the shared "space/x Toggle facet" label would mislead,
// since "x" is a literal, typeable search character inside this picker.
func TestContextualLocalHintOverlayTagPickerShowsToggle(t *testing.T) {
	m := model{overlay: overlayTagPicker}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if !strings.Contains(got, "space Toggle tag") {
		t.Errorf("overlayTagPicker: contextualLocalHint = %q, want the space-only TagToggle hint", got)
	}
	if strings.Contains(got, "space/x") {
		t.Errorf("overlayTagPicker: contextualLocalHint = %q, must NOT show the space/x alias label -- x is typeable text here (B01)", got)
	}
}

// TestTagPickerLocalBindingsOmitsNewTag guards the D01 consolidation (bean
// bt-9ipw, US-07-Reopen 2026-07-17, epic-E12-plan.md »Item 1«): the former
// separate `n`-gated free-text new-tag sub-mode is GONE -- the Tag-Picker is
// now ONE always-focused search field, so "n" is just a literal typeable
// character, not a picker command. Advertising keys.NewTag in this outer
// Footer Zone 3 set would now be actively misleading.
func TestTagPickerLocalBindingsOmitsNewTag(t *testing.T) {
	for _, b := range tagPickerLocalBindings() {
		if strings.Join(b.Keys(), ",") == strings.Join(keys.NewTag.Keys(), ",") {
			t.Fatalf("tagPickerLocalBindings() = %v, want it to OMIT keys.NewTag post-D01 consolidation (bean bt-9ipw)", tagPickerLocalBindings())
		}
	}
}

// TestContextualLocalHintOverlayTagPickerOmitsNewTag is the same guard at
// the Footer Zone 3 rendering layer (contextualLocalHint) -- the outer
// footer must not show a "n New tag" hint that no longer means anything
// post-D01 (bean bt-9ipw).
func TestContextualLocalHintOverlayTagPickerOmitsNewTag(t *testing.T) {
	m := model{overlay: overlayTagPicker}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if strings.Contains(got, "n New tag") {
		t.Errorf("overlayTagPicker: contextualLocalHint = %q, must NOT contain the stale New-Tag hint post-D01", got)
	}
}

func TestContextualLocalHintOverlayParentPickerOmitsToggle(t *testing.T) {
	m := model{overlay: overlayParentPicker}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if strings.Contains(got, "Toggle") {
		t.Errorf("overlayParentPicker: contextualLocalHint = %q, must NOT contain Toggle (keyParentPicker is single-select, no space/x case)", got)
	}
}

// TestContextualLocalHintOverlayBlockingPickerShowsToggle mirrors the
// Tag-Picker deviation above -- keyBlockingPicker (box_picker_blocking.go)
// also wires a multi-select blocking-relation membership toggle.
//
// REVISED (bean bt-z4w7, B7): this test used to assert "space/x Toggle
// facet" and was itself part of the bug -- it locked in the SHARED
// keys.Toggle label after bt-a3a8 (D6) had already narrowed this picker's
// toggle to space-only, so the footer confidently advertised an "x" that
// merely typed into the new search field. The assertion now names
// blockingPickerToggleHint, the one binding keyBlockingPicker actually
// matches. The general form of this guard lives in
// footer_binding_source_test.go's TestPickerFooterKeysAreReservedNotTyped.
func TestContextualLocalHintOverlayBlockingPickerShowsToggle(t *testing.T) {
	m := model{overlay: overlayBlockingPicker}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if !strings.Contains(got, "space Toggle blocking") {
		t.Errorf("overlayBlockingPicker: contextualLocalHint = %q, want the space-only toggle hint", got)
	}
	if strings.Contains(got, "space/x") {
		t.Errorf("overlayBlockingPicker: contextualLocalHint = %q, must NOT show the space/x alias -- x is typeable text here (bt-a3a8 D6)", got)
	}
}

// TestContextualLocalHintOverlayCreateConfirm/DeleteConfirm guard a plan
// GAP-FILL: epic-E7-plan.md Task 7 Step 6 only names Value-Menu and
// Tag-/Parent-/Blocking-Picker's overlay-specific sets -- overlayCreateConfirm/
// overlayDeleteConfirm are two more `m.overlay != overlayNone` cases the
// plan's own switch-priority text never assigns a set to. Both confirm
// gates (box_confirm_create.go/box_confirm_delete.go) really only answer to
// Enter/Back, so that is the fallback here too.
//
// D05 VERIFICATION (design-spec.md §15 PF-16, bean bt-ntoz, cited in
// bt-d8kc's own PO-Wortlaut): "Overlay-Footer zeigen enter/esc" -- these two
// tests are the concrete Sign-off evidence that D04's Header-Global-Kürzung
// (which degrades keys.Enter/keys.Back out of globalBindings()) is a No-Op
// for the Create-/Delete-Confirm-Gate's OWN footer: confirmGateLocalBindings
// (footer_context.go) was never derived FROM globalBindings() -- it is its
// own independent {Enter, Back} literal -- so removing Enter/Back from the
// header cannot silently remove them here too. Only the render OPTIC
// changed (no more ":" -- D06), never the binding SET.
func TestContextualLocalHintOverlayCreateConfirm(t *testing.T) {
	m := model{overlay: overlayCreateConfirm}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if strings.Contains(got, "stub-view-local") {
		t.Errorf("overlayCreateConfirm: contextualLocalHint = %q, must not contain the viewLocal stub", got)
	}
	if !strings.Contains(got, "y confirm dialog") || !strings.Contains(got, "n cancel dialog") {
		t.Errorf("overlayCreateConfirm: contextualLocalHint = %q, want confirm+cancel", got)
	}
}

func TestContextualLocalHintOverlayDeleteConfirm(t *testing.T) {
	m := model{overlay: overlayDeleteConfirm}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if strings.Contains(got, "stub-view-local") {
		t.Errorf("overlayDeleteConfirm: contextualLocalHint = %q, must not contain the viewLocal stub", got)
	}
	if !strings.Contains(got, "y confirm dialog") || !strings.Contains(got, "n cancel dialog") {
		t.Errorf("overlayDeleteConfirm: contextualLocalHint = %q, want confirm+cancel", got)
	}
}

func TestContextualLocalHintSearchActive(t *testing.T) {
	m := model{searchActive: true}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if strings.Contains(got, "stub-view-local") {
		t.Errorf("searchActive: contextualLocalHint = %q, must not contain the viewLocal stub", got)
	}
}

func TestContextualLocalHintPaletteOpen(t *testing.T) {
	m := model{paletteOpen: true}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if strings.Contains(got, "stub-view-local") {
		t.Errorf("paletteOpen: contextualLocalHint = %q, must not contain the viewLocal stub", got)
	}
}

func TestContextualLocalHintHelpOpen(t *testing.T) {
	m := model{helpOpen: true}
	got := m.contextualLocalHint(viewLocalStub())
	got = stripHint(got)
	if got != "esc back" {
		t.Errorf("helpOpen: contextualLocalHint = %q, want exactly %q", got, "esc back")
	}
}

// TestContextualLocalHintPriority guards the priority chain design-spec.md
// §15 PF-11 spells out: Overlay > Suche > Palette > Help > filter field >
// View-Normalzustand. Only adjacent pairs whose rendered hints actually
// DIFFER are usable as a discriminator here (searchActive and paletteOpen
// both render the identical {Enter,Back} pair by design, so that specific
// adjacency isn't separately re-verifiable via string content -- covered
// instead by the individual single-state tests above).
func TestContextualLocalHintPriority(t *testing.T) {
	cases := []struct {
		name string
		m    model
		want string // a substring only the winning branch's hint contains
	}{
		{"overlay beats filter field", model{filterFocus: true, overlay: overlayValueMenu}, "s Status"},
		{"search beats filter field", model{filterFocus: true, searchActive: true}, "enter open/confirm"},
		{"overlay beats search", model{overlay: overlayValueMenu, searchActive: true}, "s Status"},
		{"palette beats help", model{paletteOpen: true, helpOpen: true}, "enter open/confirm"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripHint(c.m.contextualLocalHint(viewLocalStub()))
			if !strings.Contains(got, c.want) {
				t.Errorf("%s: contextualLocalHint = %q, want it to contain %q", c.name, got, c.want)
			}
		})
	}
}
