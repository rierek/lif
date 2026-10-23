package tui

// box_panel_test.go — S2c: unit tests for panelBox (box_panel.go), mirroring
// box_dropdown_test.go's own dropdownBox coverage pattern.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TestPanelBoxMultilineContent asserts multi-line content is framed one
// content-line per │ … │ row, every rendered line's width equals the
// requested box width, and the label appears in the top border.
func TestPanelBoxMultilineContent(t *testing.T) {
	const w = 30
	out := panelBox("Body", "first line\nsecond line\nthird", "", w, false)
	lines := strings.Split(out, "\n")
	if len(lines) != 5 { // top + 3 content + bottom
		t.Fatalf("want 5 lines (top+3 content+bottom), got %d: %q", len(lines), out)
	}
	for i, ln := range lines {
		if got := lipgloss.Width(ln); got != w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, w, ln)
		}
	}
	top := ansi.Strip(lines[0])
	if !strings.HasPrefix(top, "╭─ Body ") {
		t.Errorf("top border missing label: %q", top)
	}
	for i, want := range []string{"first line", "second line", "third"} {
		got := ansi.Strip(lines[i+1])
		if !strings.Contains(got, want) {
			t.Errorf("content line %d missing %q: %q", i, want, got)
		}
	}
	bot := ansi.Strip(lines[4])
	if !strings.HasPrefix(bot, "╰") {
		t.Errorf("bottom border malformed: %q", bot)
	}
}

// TestPanelBoxEmptyContent asserts empty content still renders exactly one
// blank inner line (never a zero-height panel).
func TestPanelBoxEmptyContent(t *testing.T) {
	const w = 20
	out := panelBox("Relations", "", "", w, false)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 { // top + 1 blank content + bottom
		t.Fatalf("want 3 lines (top+1 blank+bottom), got %d: %q", len(lines), out)
	}
	for i, ln := range lines {
		if got := lipgloss.Width(ln); got != w {
			t.Errorf("line %d width = %d, want %d: %q", i, got, w, ln)
		}
	}
	mid := ansi.Strip(lines[1])
	if !strings.HasPrefix(mid, "│") || !strings.HasSuffix(mid, "│") {
		t.Errorf("blank content line not framed: %q", mid)
	}
}

// TestPanelBoxHotkeyInBottomBorder asserts a non-empty hotkey renders as a
// "(x)" badge in the bottom border, and its absence renders no parens.
func TestPanelBoxHotkeyInBottomBorder(t *testing.T) {
	const w = 24
	withKey := panelBox("Body", "content", "e", w, false)
	lines := strings.Split(withKey, "\n")
	bot := ansi.Strip(lines[len(lines)-1])
	if !strings.Contains(bot, "(e)") || !strings.HasPrefix(bot, "╰") {
		t.Errorf("bottom border missing hotkey badge: %q", bot)
	}

	noKey := panelBox("Body", "content", "", w, false)
	lines2 := strings.Split(noKey, "\n")
	bot2 := ansi.Strip(lines2[len(lines2)-1])
	if strings.Contains(bot2, "(") {
		t.Errorf("empty hotkey must not render parens: %q", bot2)
	}
}

// TestPanelBoxHotkeyOverflowClamp asserts B2/B3: at a narrow width with a
// long hotkey, the bottom border never overflows `width` -- if the badge
// can't fit, it drops to a plain dash line instead of exceeding width.
func TestPanelBoxHotkeyOverflowClamp(t *testing.T) {
	for _, w := range []int{8, 9, 10, 12} {
		out := panelBox("Body", "content", "shift+tab", w, false)
		for i, ln := range strings.Split(out, "\n") {
			if got := lipgloss.Width(ln); got != w {
				t.Errorf("width %d: line %d width = %d, want %d (must never overflow): %q", w, i, got, w, ln)
			}
		}
	}
}

// --- bean bt-oox1 (#4): hotkey in the TOP border ---

// TestPanelBoxTopHotkeyRendersInTopBorder guards the Body panel's exception
// (PO finding #4). Every other box advertises its hotkey in the BOTTOM
// border, which is fine for a fixed-height field -- but the Body panel is
// as tall as the bean's body text, so on a long body the bottom border is
// scrolled off the pane and the (e) badge with it: the PO could not see how
// to edit precisely the beans that most needed editing. panelBoxTopHotkey
// puts the badge in the top border instead, which is always on screen.
func TestPanelBoxTopHotkeyRendersInTopBorder(t *testing.T) {
	const w = 30
	out := panelBoxTopHotkey("Body", "first\nsecond", "e", w, false)
	lines := strings.Split(out, "\n")

	top := ansi.Strip(lines[0])
	bot := ansi.Strip(lines[len(lines)-1])
	if !strings.Contains(top, "(e)") {
		t.Errorf("top border carries no hotkey badge: %q", top)
	}
	if strings.Contains(bot, "(e)") {
		t.Errorf("hotkey must move, not be duplicated in the bottom border: %q", bot)
	}
	if !strings.HasPrefix(top, "╭") || !strings.HasPrefix(bot, "╰") {
		t.Errorf("frame corners wrong: top %q bot %q", top, bot)
	}
	if !strings.Contains(top, "Body") {
		t.Errorf("top border lost its label: %q", top)
	}
}

// TestPanelBoxTopHotkeyWidthExact holds the frame invariant every box in
// this package shares: each rendered line is exactly `width` cells, at any
// width, including ones too narrow for label+badge together.
func TestPanelBoxTopHotkeyWidthExact(t *testing.T) {
	for _, w := range []int{8, 10, 14, 20, 40, 80} {
		out := panelBoxTopHotkey("Body", "content", "e", w, false)
		for i, ln := range strings.Split(out, "\n") {
			if got := lipgloss.Width(ln); got != w {
				t.Errorf("width %d: line %d width = %d, want %d: %q", w, i, got, w, ln)
			}
		}
	}
}

// TestDetailBoxFormBodyHotkeyIsInTopBorder is the same guarantee at the
// call site the PO actually sees: the Body panel inside the box-form Detail
// pane, not the primitive in isolation.
func TestDetailBoxFormBodyHotkeyIsInTopBorder(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBeanFull(m, "tk-2")
	b := m.focusedBean()
	if b == nil {
		t.Fatal("setup: no focused bean")
	}
	out := ansi.Strip(detailBoxForm(m.idx, b, 60, -1))

	var bodyTop string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "╭─ Body ") {
			bodyTop = ln
			break
		}
	}
	if bodyTop == "" {
		t.Fatalf("no Body panel top border found:\n%s", out)
	}
	if !strings.Contains(bodyTop, "(e)") {
		t.Errorf("Body panel top border carries no (e) badge -- unreachable on a long body: %q", bodyTop)
	}
}

// TestDetailBoxFormRelationsHotkeyIsInBottomBorder guards bean bt-bszz: the
// Relations panel is a fixed-purpose panel (not content-height-driven like
// Body, bt-oox1 #4), so it keeps the DEFAULT bottom-border badge placement
// every other panelBox call uses -- it must show the "r" activation keybind
// (keymap.go's WithKeys("r"), "Relations") as a Box-Badge in ITS bottom
// border, same as Status/Type/Priority/Parent/Tags do for their own keys.
// History deliberately stays exempt (no activation keybind exists there;
// [/] are stack-nav, not a panel hotkey) -- this test does not touch it.
func TestDetailBoxFormRelationsHotkeyIsInBottomBorder(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBeanFull(m, "tk-2")
	b := m.focusedBean()
	if b == nil {
		t.Fatal("setup: no focused bean")
	}
	out := ansi.Strip(detailBoxForm(m.idx, b, 60, -1))
	lines := strings.Split(out, "\n")

	var relationsBot string
	for i, ln := range lines {
		if strings.HasPrefix(ln, "╭─ Relations ") {
			for _, l2 := range lines[i+1:] {
				if strings.HasPrefix(l2, "╰") {
					relationsBot = l2
					break
				}
			}
			break
		}
	}
	if relationsBot == "" {
		t.Fatalf("no Relations panel bottom border found:\n%s", out)
	}
	if !strings.Contains(relationsBot, "(b)") {
		t.Errorf("Relations panel bottom border carries no (b) badge: %q", relationsBot)
	}
}
