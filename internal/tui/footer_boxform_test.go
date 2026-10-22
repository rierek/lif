package tui

// footer_boxform_test.go — bean bt-fy5d (Nebenbefund N2, epic bt-vy1q):
// the Detail pane already shows every scalar field's
// hotkey salient INSIDE its own box frame ((e) (s) (o) (u) (a) (t),
// box_detail_form.go). Repeating those keys in the footer is pure redundancy
// and costs a whole footer line at 80 columns. The footer therefore drops
// exactly the inline-visible ones -- and ONLY while the flag is on.

import (
	"strings"
	"testing"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/x/ansi"
)

// bindingKeys returns each binding's Help().Key -- the literal token
// renderBindings puts in the footer.
func bindingKeys(bs []keybind.Binding) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		if k := b.Help().Key; k != "" {
			out = append(out, k)
		}
	}
	return out
}

// boxFormInlineFooterKeys are the keys bt-fy5d removes from the footer while
// the box form is on: every one of them is rendered as an inline (x) badge by
// detailBoxForm/panelBox.
//
// bean bt-6nuz (PO finding #6): `r` USED to be in this list. It was the one
// entry admitted on a looser test -- "its target IS the Relations panel the
// same detail render shows" -- rather than on the actual criterion, an
// inline badge. The Relations panel carries no (r) badge, so dropping `r`
// from the footer removed the only place the key was advertised at all. The
// rule is now literal, and TestBoxFormInlineKeysAllHaveAnInlineBadge below
// enforces it structurally instead of by hand.
var boxFormInlineFooterKeys = []string{"s", "e", "t", "a"}

func TestBrowseRepoLocalBindingsDropInlineKeysWhileBoxForm(t *testing.T) {

	got := bindingKeys(browseRepoLocalBindings())
	for _, k := range boxFormInlineFooterKeys {
		for _, have := range got {
			if have == k {
				t.Fatalf("footer still advertises %q although the box form shows it inline: %v", k, got)
			}
		}
	}
	// Everything that is NOT inline-visible must survive -- the point is to
	// de-duplicate, not to strip the footer. `r` (bean bt-6nuz) is in this
	// list precisely because the Relations panel gives it no badge.
	for _, k := range []string{"tab", "shift+tab", "/", "f", "c", "d", "b", "y", "r", "v"} {
		found := false
		for _, have := range got {
			if have == k {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("footer lost non-inline key %q: %v", k, got)
		}
	}
}

// TestReadyFooterFollowsBrowseRepo: the Ready renders the SAME box-form
// detail pane, so its footer must thin out identically (readyLocalBindings
// delegates -- this pins that it keeps doing so).
func TestReadyFooterFollowsBrowseRepo(t *testing.T) {
	if a, b := bindingKeys(readyLocalBindings()), bindingKeys(browseRepoLocalBindings()); strings.Join(a, "|") != strings.Join(b, "|") {
		t.Fatalf("Ready footer diverged from Browse: %v vs %v", a, b)
	}
}

// TestBoxFormInlineKeysAllHaveAnInlineBadge is bean bt-6nuz's structural
// answer to the selection error itself, not to its two symptoms. bt-fy5d
// thinned the footer on the premise "the box form already shows this key
// inline" -- but nothing checked that premise, so `r` was dropped on a
// judgement call and simply vanished from the UI. This renders the real
// box-form Detail pane and requires every key boxFormInlineKeys hides to
// actually appear as a literal (x) badge in that render. A future key
// removed from the footer without a badge to justify it fails here.
func TestBoxFormInlineKeysAllHaveAnInlineBadge(t *testing.T) {

	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 120, 40
	m = focusBeanFull(m, "tk-2")
	b := m.focusedBean()
	if b == nil {
		t.Fatal("setup: no focused bean")
	}
	rendered := ansi.Strip(detailBoxForm(m.idx, b, 80, -1))

	for k := range boxFormInlineKeys {
		if !strings.Contains(rendered, "("+k+")") {
			t.Errorf("boxFormInlineKeys hides %q from the footer, but the box-form detail pane renders no (%s) badge -- the key would be advertised nowhere:\n%s", k, k, rendered)
		}
	}
}

// TestBoxFormFooterKeysStayFunctional guards the Grounding's drift-guard
// pitfall from the other side: the keys only vanish from the FOOTER DISPLAY.
// They must still be registered in the keymap (keymap.go is the single
// source, TestHelpGroupsCoverEveryBindingExactlyOnce reflects over it), so
// the Help overlay keeps documenting them and the handlers keep working.
func TestBoxFormFooterKeysStayFunctional(t *testing.T) {

	registered := map[string]bool{}
	for _, g := range keys.helpGroups() {
		for _, k := range bindingKeys(g.bindings) {
			registered[k] = true
		}
	}
	for _, k := range boxFormInlineFooterKeys {
		if !registered[k] {
			t.Fatalf("key %q disappeared from the keymap/help groups -- bt-fy5d may only thin the FOOTER, never the registration", k)
		}
	}
}

// --- bean bt-8d35: the footer names the REGION-local meaning of tab ---
