package tui

// footer_binding_source_test.go — bean bt-z4w7 (B7, epic bt-vy1q).
//
// TWO symptoms, ONE cause: a Footer Zone 3 label was written out by hand
// next to the handler instead of being READ OFF the binding the handler
// actually matches, so the two drifted apart the moment either side moved.
//
//   - Value-Menu: keyValueMenu closed on keys.Status only, so a menu opened
//     with `o`/`u` (Type/Priority) still answered to `s` -- and the footer
//     said "s" no matter which group was open.
//   - Blocking-Picker: the footer advertised keys.Toggle ("space/x Toggle
//     facet") although bt-a3a8 narrowed that picker's toggle to space-only
//     ("x" had to stay a typeable character inside the new search field).
//
// The fix is structural, not two corrected strings: each overlay exposes the
// binding for its context-dependent key through ONE accessor
// (valueMenuGroupKey / blockingPickerToggleHint), and BOTH the key handler
// and the footer consume that same accessor. Divergence is then impossible
// by construction. The tests below lock the property, not the strings.

import (
	"strings"
	"testing"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Value-Menu: close key AND footer follow the OPEN group ---

// valueMenuGroupCases maps each group to the key that opens it (keymap.go's
// Status/Type/Priority, design decision D07 "klein = Feld-Aktion").
var valueMenuGroupCases = []struct {
	group   string
	openKey rune
	label   string
}{
	{"status", 's', "s Status"},
	{"type", 't', "t Type"},
	{"priority", 'p', "p Priority"},
}

// TestValueMenuFooterFollowsOpenGroup: the footer must name the key that
// really closes THIS menu -- showing "s" over a `u`-opened priority menu is
// exactly the mislead the bean reports.
func TestValueMenuFooterFollowsOpenGroup(t *testing.T) {
	for _, c := range valueMenuGroupCases {
		t.Run(c.group, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m = focusBean(m, "tk-1")
			m = step(t, m, runeMsg(c.openKey))
			if m.overlay != overlayValueMenu {
				t.Fatalf("%c did not open the value menu (overlay = %v)", c.openKey, m.overlay)
			}
			got := stripHint(m.contextualLocalHint(viewLocalStub()))
			if !strings.Contains(got, c.label) {
				t.Errorf("group %q: footer = %q, want it to name its own key (%q)", c.group, got, c.label)
			}
			for _, other := range valueMenuGroupCases {
				if other.group == c.group {
					continue
				}
				if strings.Contains(got, other.label) {
					t.Errorf("group %q: footer = %q, must NOT advertise another group's key (%q)", c.group, got, other.label)
				}
			}
		})
	}
}

// TestValueMenuClosesOnItsOwnGroupKey is the behavioural half: the key the
// footer names must really close the menu.
func TestValueMenuClosesOnItsOwnGroupKey(t *testing.T) {
	for _, c := range valueMenuGroupCases {
		t.Run(c.group, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m = focusBean(m, "tk-1")
			m = step(t, m, runeMsg(c.openKey))
			if m.overlay != overlayValueMenu {
				t.Fatalf("%c did not open the value menu", c.openKey)
			}
			m = step(t, m, runeMsg(c.openKey))
			if m.overlay != overlayNone {
				t.Fatalf("group %q: %c did not close its own menu (overlay = %v)", c.group, c.openKey, m.overlay)
			}
		})
	}
}

// TestValueMenuDoesNotCloseOnAnotherGroupKey is the regression guard for the
// reported symptom: an `o`/`u`-opened menu used to close on `s` because the
// handler matched keys.Status unconditionally.
func TestValueMenuDoesNotCloseOnAnotherGroupKey(t *testing.T) {
	for _, c := range valueMenuGroupCases {
		for _, other := range valueMenuGroupCases {
			if other.group == c.group {
				continue
			}
			t.Run(c.group+"_not_"+other.group, func(t *testing.T) {
				m := fixtureModel(t, fixtureBeans())
				m = focusBean(m, "tk-1")
				m = step(t, m, runeMsg(c.openKey))
				m = step(t, m, runeMsg(other.openKey))
				if m.overlay != overlayValueMenu {
					t.Fatalf("group %q closed on a FOREIGN key %c -- only its own key and esc may close it", c.group, other.openKey)
				}
			})
		}
	}
}

// TestValueMenuAlwaysClosesOnEsc: esc stays the universal escape hatch in
// every group (the bean's own "nichts ist funktional kaputt" baseline).
func TestValueMenuAlwaysClosesOnEsc(t *testing.T) {
	for _, c := range valueMenuGroupCases {
		t.Run(c.group, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m = focusBean(m, "tk-1")
			m = step(t, m, runeMsg(c.openKey))
			m = step(t, m, keyMsg(tea.KeyEsc))
			if m.overlay != overlayNone {
				t.Fatalf("group %q did not close on esc (overlay = %v)", c.group, m.overlay)
			}
		})
	}
}

// TestValueMenuInlineHintMatchesFooter: the modal's OWN hint line
// (valueMenuBox) is a second surface that used to hardcode "esc/s:cancel".
// It must be derived from the same accessor, or the two surfaces disagree
// with each other while both claim to describe the same menu.
func TestValueMenuInlineHintMatchesFooter(t *testing.T) {
	for _, c := range valueMenuGroupCases {
		t.Run(c.group, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m = focusBean(m, "tk-1")
			m = step(t, m, runeMsg(c.openKey))
			got := stripHint(m.valueMenuBox())
			want := "esc/" + string(c.openKey) + ":cancel"
			if !strings.Contains(got, want) {
				t.Errorf("group %q: inline hint = %q, want it to contain %q", c.group, got, want)
			}
		})
	}
}

// --- Blocking-Picker: footer must not advertise a typeable character ---

// TestBlockingPickerFooterIsSpaceOnly guards the second symptom. "x" is a
// literal search character in this picker since bt-a3a8 (D6) -- advertising
// "space/x" tells the PO to press a key that types instead of toggles.
func TestBlockingPickerFooterIsSpaceOnly(t *testing.T) {
	m := model{overlay: overlayBlockingPicker}
	got := stripHint(m.contextualLocalHint(viewLocalStub()))
	if strings.Contains(got, "space/x") {
		t.Errorf("blocking picker footer = %q, must NOT show the space/x alias -- x is typeable text here (bt-a3a8 D6)", got)
	}
	if !strings.Contains(got, "space") {
		t.Errorf("blocking picker footer = %q, want the space-only toggle hint", got)
	}
}

// TestPickerFooterKeysAreReservedNotTyped is the CLASS guard, the reason
// this file exists: for every overlay that hosts an always-focused search
// field, each single-rune key its footer advertises must be RESERVED by the
// handler -- never fall through and get typed into the query. This is what
// catches "space/x" generically, and it will catch the next such label too
// without anyone remembering this bean.
func TestPickerFooterKeysAreReservedNotTyped(t *testing.T) {
	cases := []struct {
		name     string
		open     rune
		bindings []keybind.Binding
		query    func(model) string
	}{
		{"blocking", 'b', blockingPickerLocalBindings(), func(m model) string { return m.blockFilter.input.Value() }},
		{"parent", 'a', parentPickerLocalBindings(), func(m model) string { return m.parentFilter.input.Value() }},
		{"tag", 'g', tagPickerLocalBindings(), func(m model) string { return m.tagInput.Value() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, b := range c.bindings {
				for _, k := range b.Keys() {
					r := []rune(k)
					// Only single printable runes can be swallowed by a
					// text input; "up"/"enter"/"esc" cannot.
					if len(r) != 1 || r[0] == ' ' {
						continue
					}
					m := fixtureModel(t, fixtureBeans())
					m = focusBean(m, "tk-1")
					m = step(t, m, runeMsg(c.open))
					m = step(t, m, runeMsg(r[0]))
					if got := c.query(m); strings.ContainsRune(got, r[0]) {
						t.Errorf("%s picker advertises %q in its footer, but pressing it TYPED into the search query (%q) instead of acting -- the footer names a binding the handler does not reserve", c.name, k, got)
					}
				}
			}
		})
	}
}

// TestBlockingToggleStillWorksOnSpace: narrowing the advertised label must
// not have narrowed the real behaviour.
func TestBlockingToggleStillWorksOnSpace(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "ep-1")
	m = step(t, m, runeMsg('b'))
	if m.overlay != overlayBlockingPicker {
		t.Fatalf("r did not open the blocking picker (overlay = %v)", m.overlay)
	}
	before := len(m.blockPending)
	m = step(t, m, runeMsg(' '))
	if len(m.blockPending) == before {
		t.Fatalf("space no longer toggles the cursored blocking candidate (pending stayed %d)", before)
	}
	if got := m.blockFilter.input.Value(); got != "" {
		t.Fatalf("space leaked into the search query: %q", got)
	}
}
