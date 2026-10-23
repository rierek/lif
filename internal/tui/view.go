package tui

// view.go — the shared chrome layer: breadcrumb/footer/status-line rendering,
// masterDetailWidths(), the outer RoundedBorder frame and the four-zone hull
// (header row `> repo: Titel` + global shortcuts right; scrollable body;
// footer hints; status line). Ported from devd (~/dev/Developer
// Dashboard/apps/cli-go/internal/tui/view.go).
//
// Decoupling from devd: every devd chrome function is a `model` method (m
// model) and reaches into m.project/m.cfg/m.width/... . beans-tui has no
// model yet (T8 builds the App-Shell) and the chrome layer must not import
// the data layer, so every function here takes plain strings/ints instead —
// Chrome/ChromeOpts bundle the render inputs a future model will supply.
// devd's metaGrid/metaStrip/issueMetaPairs/detailTitle (Detail-view header
// slots, api.Issue-coupled) and screenTitle (project-prefix titles) are NOT
// ported: out of the "four-zone hull" this task scopes (no optional info-
// grid zone), and/or view-specific — they land with the view that needs them.

import (
	"fmt"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/theme"
)

// breadcrumb renders header Zone 1 (devd wireframe): left `> repo: Title`
// (chevron+repo Peach, title Mauve bold), right = pre-rendered global
// shortcuts hint. title="" → only `> repo`. Port-adaptation: devd used the
// project slug ("> dd: Title"); beans-tui uses the repo name instead
// (design-spec.md §7/implementation-plan Task 7: "Breadcrumb-Format `> repo:
// Titel`").
func breadcrumb(repo, title, globalHint string, width int) string {
	left := theme.Chevron.Render("> " + repo)
	if title != "" {
		left += theme.Chevron.Render(":") + " " + theme.Header.Render(title)
	}
	// D06 (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc): globalHint
	// arrives PRE-STYLED from renderBindings (Teal key / Subtext desc, no
	// more ":") -- no outer Muted wash anymore. ANSI styles do not nest: an
	// outer Style.Render's own trailing reset only wraps the string once,
	// so any inner Reset (renderBindings emits one per key/desc span) would
	// already have clobbered it well before the end, silently reverting
	// every span AFTER the first one back to the terminal's raw default
	// foreground instead of staying Muted (see footer()'s own doc comment,
	// same underlying reasoning).
	right := globalHint
	if lipgloss.Width(left)+lipgloss.Width(right)+1 > width { // narrow: stack instead of overflow
		return ansi.Truncate(left, width, "…") + "\n" + ansi.Truncate(right, width, "…")
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// wrapText ANSI-safely wraps s to w cells: word-wrap first, then hard breaks
// for over-long tokens (URLs/paths) that would otherwise overflow (devd
// DD2-60 review finding). Preserves existing line breaks.
func wrapText(s string, w int) string {
	if w < 1 {
		w = 1
	}
	return ansi.Hardwrap(ansi.Wordwrap(s, w, ""), w, true)
}

// statusBar renders footer Zone 4 (devd wireframe): the scroll indicator
// (Accent), right-aligned. Empty → empty line.
//
// bt-81f0 (Notifications vereinheitlichen, Q1-Annahme): this used to ALSO
// render a critical error (Red) via a second errNote param -- Toast
// (overlay_show_toast.go) is now the ONE visible notification channel, so
// that param is gone. The status line itself stays (Q1: cap the Fehler-
// Anbindung only, no dynamic footer height / layout rework) -- it now
// carries ONLY the scroll indicator.
// statusLine is the ONE source for the footer's optional status row across
// viewBrowseRepo/viewReady/viewTagManagement AND their click-geometry
// reconstructions (clickPaneGeometry, mouse.go) -- bean bt-oqsv: the row is
// no longer RESERVED, it is rendered only when it carries something, so
// view and hit-test must agree on "is it there?" from the same expression.
// I04: a failed data.Watch start (no live reload) must never degrade
// silently -- it is the only tenant of this slot since bt-81f0 moved real
// load failures to Toast.
func (m model) statusLine(width int) string {
	indicator := ""
	if m.watchUnavailable {
		indicator = "watch unavailable — ctrl+r for manual reload"
	}
	return statusBar(indicator, width)
}

// statusLineHeight is the number of screen rows statusLine's result occupies
// (bean bt-oqsv): 0 for the empty string. Before bt-oqsv the row was
// unconditionally appended, so an empty status still cost a permanently
// blank line -- footH's own "+2" in clickPaneGeometry encoded exactly that
// reservation. Both sides now use THIS function instead of a constant.
func statusLineHeight(status string) int {
	if status == "" {
		return 0
	}
	return lipgloss.Height(status)
}

// appendStatusLine appends status as its own row to content, or returns
// content untouched when there is nothing to show (bean bt-oqsv).
func appendStatusLine(content, status string) string {
	if status == "" {
		return content
	}
	return content + "\n" + status
}

func statusBar(indicator string, width int) string {
	right := ""
	if indicator != "" {
		right = theme.Accent.Render(indicator)
	}
	if right == "" {
		return ""
	}
	if lipgloss.Width(right)+1 > width {
		return ansi.Truncate(right, width, "…")
	}
	gap := width - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return strings.Repeat(" ", gap) + right
}

// nbsp is U+00A0 NO-BREAK SPACE. x/ansi's own Wordwrap (wrap.go) explicitly
// excludes it from its whitespace-breakpoint set (`unicode.IsSpace(r) && r
// != nbsp`) -- used below to glue a rendered binding's key+desc into one
// wrap-atomic unit (see renderBindings' own doc comment for why).
const nbsp = "\u00a0"

// renderBindings renders a binding list as a colored "key desc" hint, joined
// by " · " (D06, design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc Grilling-
// Nachtrag, PO-Wortlaut "Taste in TEAL, Aktions-Wort grau (subtext), KEIN
// ':' mehr — Farbtrennung ersetzt den Doppelpunkt"): key rendered
// theme.BindingKey (Teal), description theme.BindingDesc (Subtext) --
// replaces the previous plain "key:desc" colon-joined, blanket-Dim-washed
// rendering. This is the ONE shared render path for BOTH Header Zone 1
// (globalBindings()) AND Footer Zone 3 (browseRepoLocalBindings/
// readyLocalBindings, plus every overlay-local set in footer_context.go)
// -- one function, one optic, "gilt einheitlich" (PO). Bindings without a
// Help().Key are skipped (devd DD2-175: footer hints are derived from the
// keymap, single source, can never drift from the real bindings).
//
// REGRESSION FIX (found live, this task's own tmux smoke test): the old
// "key:desc" rendering glued the key to its desc's first word via ":" (no
// internal space at all -- footer()'s wordwrap, view.go, could never split
// them). Replacing ":" with a plain " " reintroduced a real breakable
// space, so a narrow-terminal wrap could orphan a key alone on one line
// with its own desc starting the next (observed: Ready footer at 80
// columns split "S" from "Sort"). Every space WITHIN one binding's own
// "key desc..." -- between key and desc, AND between a multi-word desc's
// own words (e.g. FocusIn's "next focus") -- is therefore nbsp, not " ":
// the whole entry becomes one wrap-atomic unit. Only the " · " SEPARATOR
// BETWEEN entries stays a real, breakable space -- that IS where wrapping
// should be free to happen.
func renderBindings(bs []keybind.Binding) string {
	parts := make([]string, 0, len(bs))
	for _, b := range bs {
		h := b.Help()
		if h.Key != "" {
			desc := strings.ReplaceAll(h.Desc, " ", nbsp)
			parts = append(parts, theme.BindingKey.Render(h.Key)+nbsp+theme.BindingDesc.Render(desc))
		}
	}
	return strings.Join(parts, " · ")
}

// footer renders footer Zone 3 (local + global hints, already joined into
// hint by renderBindings) wrapped to width — narrow terminals wrap instead
// of overflowing into the pane columns (mirrors breadcrumb/chrome). No
// longer blanket-Dim-washes the string (pre-D06 behavior, design-spec.md §15
// PF-16, bean bt-ntoz/bt-d8kc): hint arrives PRE-STYLED (Teal key / Subtext
// desc, renderBindings' own doc comment) — an outer Style.Render's own
// trailing reset only wraps the whole string ONCE, so it would get
// immediately clobbered by the first inner Reset renderBindings' per-key/
// -desc spans already emit (ANSI styles do not nest), silently reverting
// every span AFTER the first back to the terminal's raw default foreground
// instead of staying Dim. wrapText itself is ANSI-aware (x/ansi Wordwrap/
// Hardwrap), so pre-styled input still wraps correctly across lines.
func footer(hint string, width int) string {
	return wrapText(hint, width)
}

// truncate ANSI-safely truncates s to w cells (never cuts an escape
// sequence) — critical since lines carry color; rune-slicing would destroy
// sequences.
func truncate(s string, w int) string {
	if w < 1 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// masterDetailWidths splits the master-detail pane width as a true 1fr:2fr:
// the list on the left gets a third (w/3), the detail on the right gets the
// rest (rw = w - lw - 4, 2 border columns per pane). treeWidthFloor (devd:
// cfg.Layout.TreeWidth) is only a minimum for readability on narrow
// terminals, capped at w*2/5 — not a fixed width on wide terminals (devd
// DD2-91 rework).
func masterDetailWidths(w, treeWidthFloor int) (lw, rw int) {
	lw = w / 3 // 1fr
	if treeWidthFloor > 0 && lw < treeWidthFloor {
		lw = treeWidthFloor
	}
	if lw < 24 {
		lw = 24
	}
	if cap := w * 2 / 5; lw > cap {
		lw = cap
	}
	rw = w - lw - 4
	if rw < 20 {
		rw = 20
	}
	return
}

// outerBorder wraps content in the app's outer frame (RoundedBorder,
// Overlay-colored) — a no-op when bordered is false. width is the full
// (outer) width; content must already fill the inner width/height, i.e. NO
// Height() here (Golden Rule #1) — the frame grows naturally around it.
func outerBorder(content string, width int, bordered bool) string {
	if !bordered {
		return content
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Overlay).
		Width(width).
		Render(content)
}

// scrollView windows content to height lines from offset (clamped) and pads
// with blank lines to height so the footer stays glued to the bottom.
// Returns a scroll indicator too (empty when everything fits).
func scrollView(content string, height, offset int) (string, string) {
	if height < 1 {
		height = 1
	}
	lines := strings.Split(content, "\n")
	total := len(lines)
	maxOff := total - height
	if maxOff < 0 {
		maxOff = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + height
	if end > total {
		end = total
	}
	win := append([]string{}, lines[offset:end]...)
	for len(win) < height {
		win = append(win, "")
	}
	ind := ""
	if total > height {
		ind = fmt.Sprintf("L %d–%d/%d", offset+1, end, total)
		switch {
		case offset == 0:
			ind += " ↓"
		case end >= total:
			ind = "↑ " + ind
		default:
			ind = "↑ " + ind + " ↓"
		}
	}
	return strings.Join(win, "\n"), ind
}

// ChromeOpts bundles the render inputs for Chrome — the plain-data
// equivalent of the fields a future model (T8) will hold.
type ChromeOpts struct {
	Width, Height int
	Bordered      bool
	Repo          string // breadcrumb "> repo"
	Title         string // breadcrumb ": Title" (skipped when empty)
	GlobalHint    string // pre-rendered "key:desc  key:desc  ..." right of breadcrumb
	Body          string // unwrapped body text; wrapped+scrolled internally
	Scroll        int    // scroll offset into the wrapped body
	FooterHint    string // pre-rendered local+global "key:desc  ..." hint
	fallbackAvail int    // test hook: override the body-height fallback (0 = default 18)
}

// Chrome is the shared screen chrome (devd DD2-48): breadcrumb header,
// height-filling scroll body, footer hints + status line — wrapped in the
// outer RoundedBorder frame when Bordered is set.
func Chrome(o ChromeOpts) string {
	innerW := o.Width
	innerH := o.Height
	if o.Bordered {
		innerW -= 2
		innerH -= 2
	}
	head := breadcrumb(o.Repo, o.Title, o.GlobalHint, innerW)
	wrapped := lipgloss.NewStyle().Padding(0, 1).Render(wrapText(o.Body, innerW-2))
	localKeys := footer(o.FooterHint, innerW)
	div := theme.Dim.Render(strings.Repeat("─", innerW))
	// bean bt-oqsv: the status line is no longer RESERVED, so its height is
	// not a constant here either. Chrome's own indicator is a chicken-and-egg
	// case the two-pane views do not have: scrollView produces `ind`, but it
	// needs `avail`, which needs to know whether `ind` will occupy a line.
	// Resolved with a monotone two-pass -- pass 1 assumes no status line;
	// if one IS needed, pass 2 re-windows one line shorter. Shrinking the
	// window can only ever make MORE content overflow, never less, so a
	// non-empty indicator can never become empty on the second pass: the
	// loop provably settles after at most one correction.
	availFor := func(statusH int) int {
		footH := lipgloss.Height(localKeys) + 1 + statusH   // + divider above the footer
		avail := innerH - lipgloss.Height(head) - footH - 1 // - divider under the top bar
		if avail < 4 {
			if o.fallbackAvail > 0 {
				return o.fallbackAvail
			}
			return 18 // height unknown (init/tests) → generous fallback
		}
		return avail
	}
	win, ind := scrollView(wrapped, availFor(0), o.Scroll)
	status := statusBar(ind, innerW)
	if statusH := statusLineHeight(status); statusH > 0 {
		win, ind = scrollView(wrapped, availFor(statusH), o.Scroll)
		status = statusBar(ind, innerW)
	}
	content := appendStatusLine(head+"\n"+div+"\n"+win+"\n"+div+"\n"+localKeys, status)
	// outerBorder's width param is the CONTENT width the border wraps around
	// (lipgloss's Border() adds 2 columns on top of Width()) — pass innerW
	// (already reduced above), not o.Width, or the frame ends up o.Width+2
	// wide instead of o.Width.
	return outerBorder(content, innerW, o.Bordered)
}
