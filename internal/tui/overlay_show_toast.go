package tui

// overlay_show_toast.go — E5 Task 1 (bean bt-6dts, epic bt-5h4d): color-coded
// corner Toast overlay. Port devd overlay_show_toast.go
// (~/dev/DeveloperDashboard/apps/cli-go/internal/tui/
// overlay_show_toast.go) structurally VERBATIM (epic-E5-plan.md design
// decision a) -- ONE slot (no stack, no stapeln): a new toast replaces the
// old one immediately. Auto-dismiss after a kind-specific duration
// (toastTimeout, messages.go) -- except sticky=true, which survives reload
// cycles AND has NO auto-dismiss timer, until a newer toast replaces it or a
// click closes it.
//
// Deviation vs. devd (design decision a): toastTarget carries ONLY `view
// viewID` -- devd's milestoneID/sprintID/issueID triad has no beans-tui
// equivalent (ONE entity, item.Item, design-spec §4); a future
// deeper-focus-restore producer can extend this struct when it needs to, no
// producer currently does.
//
// Rendering: no new compositing primitive -- canvasLines()/spliceLine() (the
// same primitives placeOverlay() itself uses, overlay.go) place the box in
// the top-right corner instead of placeOverlay()'s centering. modalBox
// (modal.go) stays the box-chrome source.
//
// toastGeometry/renderToast compose directly against m.width/m.height,
// UNCHANGED vs. devd's own (already B01-fixed) source: beans-tui's View
// functions (viewBrowseRepo/viewReady/viewLobby) each build their own full
// frame (w, h := m.width, m.height, outerBorder(..., true)) exactly like
// devd's fully-composited View()/viewComposite() split does -- no separate
// termWidth()-style inner-width helper exists here to mis-target against
// (design decision a, point 1).
//
// Input: NOT modal -- never blocks the keyboard. Only a left-click on the
// Toast hit area is intercepted ahead of the regular mouse dispatch (Task 4,
// mouse.go: handleMouse) -- dismiss, or (with a set target) a view switch
// plus dismiss.

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/theme"
)

type toastKind int

const (
	toastInfo  toastKind = iota // Blue/Mauve — Hinweis/Erfolg
	toastWarn                   // Yellow — Warnung, nicht blockierend
	toastError                  // Red — Fehler
)

// toastTarget addresses, minimally, where a click on the toast jumps to
// (devd DD2-272 AC4 parity). Only view -- beans-tui has ONE focus resolver
// (focusedBean(), view-agnostic across Tree/Ready), so a view switch alone
// is enough to land the click near the right context; no producer needs a
// deeper bean/section jump yet.
type toastTarget struct {
	view viewID
}

// toastState is the ONE toast slot (no stack). seq = generation, drawn from
// model.toastSeqCounter (types.go), a counter that is NEVER reset -- a
// toastExpiredMsg only clears the toast when seq still matches the current
// generation (otherwise a newer toast has already replaced it, or the slot
// was reset and refilled since, T6b-Review Prelude I01, bean bt-ggt2/T7).
type toastState struct {
	kind    toastKind
	title   string
	context string // second, dimmed line (optional)
	target  *toastTarget
	seq     int
	sticky  bool
	setAt   time.Time // debounce window (AC5 parity)
}

// toastDebounceWindow: a second showToast call within this span of the last
// toast replaces its content in place instead of starting a new generation +
// a new auto-dismiss tick (AC5 parity) -- ONE timer, never stacked.
const toastDebounceWindow = 300 * time.Millisecond

// toastDuration returns the kind-specific auto-dismiss duration.
func toastDuration(kind toastKind) time.Duration {
	switch kind {
	case toastError:
		return 8 * time.Second
	case toastWarn:
		return 3 * time.Second
	default:
		return 5 * time.Second
	}
}

// showToast sets the toast slot. Within the debounce window (AC5 parity) the
// existing slot is updated in place (content/kind/target), WITHOUT a new
// generation/timer -- the already-running auto-dismiss tick stays the single
// source of truth. Sticky toasts (AC2 parity) get NO auto-dismiss timer.
//
// seq is drawn from m.toastSeqCounter (T6b-Review Prelude I01 fix, bean
// bt-ggt2/T7, types.go's own doc-stamp) -- a model-wide counter that is NEVER
// reset by any m.toast=nil site, unlike the old `m.toast.seq + 1` scheme
// (which restarted at 1 every time the slot was nil'd, letting a stale,
// still-in-flight tick from a PRE-reset toast alias a POST-reset toast's
// seq and dismiss it prematurely).
func (m model) showToast(kind toastKind, title, context string, target *toastTarget, sticky bool) (model, tea.Cmd) {
	now := time.Now()
	if m.toast != nil && now.Sub(m.toast.setAt) < toastDebounceWindow {
		t := *m.toast
		t.kind, t.title, t.context, t.target, t.sticky, t.setAt = kind, title, context, target, sticky, now
		m.toast = &t
		return m, nil // debounce: no new tick, the running one stays valid
	}
	m.toastSeqCounter++
	seq := m.toastSeqCounter
	m.toast = &toastState{kind: kind, title: title, context: context, target: target, seq: seq, sticky: sticky, setAt: now}
	if sticky {
		return m, nil // AC2 parity: sticky = no auto-dismiss, until replace/click
	}
	return m, toastTimeout(seq, kind)
}

// clearToastUnlessSticky clears the toast ONLY when it is not sticky --
// reload handlers must never clobber a sticky toast (E3-I01 PFLICHT: the
// conflict + recovery-tempfile path must stay readable until the PO acts).
func (m model) clearToastUnlessSticky() model {
	if m.toast != nil && !m.toast.sticky {
		m.toast = nil
	}
	return m
}

// toastKindColor maps kind -> border/accent color.
func toastKindColor(k toastKind) lipgloss.Color {
	switch k {
	case toastError:
		return theme.Red
	case toastWarn:
		return theme.Yellow
	default:
		return theme.Blue
	}
}

const (
	toastBoxMinWidth = 32 // floor (devd DD2-272 parity, unchanged by bt-0xrb)
	toastBoxMaxWidth = 70 // cap (Planner-Entscheidung bt-0xrb, epic-E13-plan.md Item 1 -- PO
	// named no number; oriented on clampModalWidth's own convention,
	// box_filter_facets.go)
)

// toastBoxWidth returns the toast box's OUTER width (border+padding
// included), content-driven and clamped to [toastBoxMinWidth,
// min(termW-4, toastBoxMaxWidth)] (bt-0xrb: replaces the old fixed 36 --
// PO-Anforderung Grilling 2026-07-17 "Toast muss dynamisch größer werden
// [...] bis die Meldung VOLLSTÄNDIG lesbar ist", gilt für ALLE
// toastKind-Severities, epic-E13-plan.md Item 1). contentW is the widest
// UNWRAPPED content line (dot+" "+title, or context) in cells
// (ansi.StringWidth) -- the +4 below reserves the same border(2)+padding(2)
// budget the old fixed constant already accounted for (see toastBox's own
// doc comment on the modalBox Width-vs-border relationship). Mirrors
// clampModalWidth's own shrink-then-floor pattern (box_filter_facets.go).
func toastBoxWidth(termW, contentW int) int {
	capW := toastBoxMaxWidth
	if termW > 4 && termW-4 < capW {
		capW = termW - 4
	}
	w := contentW + 4
	if w > capW {
		w = capW
	}
	if w < toastBoxMinWidth {
		w = toastBoxMinWidth
	}
	return w
}

// toastBox renders the toast box: line 1 = colored dot + title (kind-tinted),
// line 2 = context, dimmed. Width is content-driven (toastBoxWidth) and
// NEITHER line is ansi.Truncate'd anymore (bt-0xrb, D04) -- content that
// still overflows the clamped width wraps via wrapText (view.go's own
// ansi.Wordwrap/ansi.Hardwrap wrapper, already used for the footer) instead
// of relying on lipgloss's own auto-wrap (LESSONS-LEARNED E12/1: TrueColor
// terminals can split lipgloss auto-wrap mid-word/mid-ANSI-span -- explicit
// wordwrap avoids that class of bug). The border follows the same kind
// color.
func (m model) toastBox() string {
	t := m.toast
	col := toastKindColor(t.kind)

	// Content-drive the width off the UNWRAPPED lines first (before any
	// wrapping decision) -- toastBoxWidth clamps that to [32, min(m.width-4,
	// 70)], then wrapText below wraps whatever still overflows the clamped
	// budget onto more lines (toastGeometry's h = len(lines) already adapts
	// automatically, no separate height wiring needed).
	contentW := ansi.StringWidth("● " + t.title)
	if t.context != "" {
		if cw := ansi.StringWidth(t.context); cw > contentW {
			contentW = cw
		}
	}
	outerW := toastBoxWidth(m.width, contentW)
	innerW := outerW - 4 // modalBox: Border(2) + Padding(0,1)*2 = 4

	dot := lipgloss.NewStyle().Foreground(col).Render("●")
	titleStyle := lipgloss.NewStyle().Foreground(col).Bold(true)
	titleLines := strings.Split(wrapText(t.title, innerW-2), "\n") // -2: dot + space
	line1 := dot + " " + titleStyle.Render(titleLines[0])
	for _, extra := range titleLines[1:] {
		line1 += "\n  " + titleStyle.Render(extra) // 2-space indent aligns under "● "
	}
	body := line1
	if t.context != "" {
		body += "\n" + theme.Dim.Render(wrapText(t.context, innerW))
	}
	// modalBox sets lipgloss.Width(width) on the style BEFORE the border --
	// the border then adds another +2 (left/right, 1 each) to the total
	// width. outerW-2 compensates so the rendered box is exactly outerW
	// wide.
	return modalBox(body, outerW-2, col)
}

// toastGeometry returns the toast box's placement (top-right corner) on the
// canvas -- shared geometry between rendering (renderToast) and the
// click-hit-test (mouse.go: handleMouse, Task 4), so they can never drift
// apart. Computed directly against m.width/m.height -- see this file's own
// doc comment (top) for why no termWidth()-style adaptation is needed here
// (design decision a, point 1): every one of beans-tui's View functions
// already builds its own full, bordered frame on m.width/m.height before
// composeOverlays runs.
func (m model) toastGeometry() (x, y, w, h int) {
	box := m.toastBox()
	lines := strings.Split(box, "\n")
	w = 0
	for _, l := range lines {
		if lw := ansi.StringWidth(l); lw > w {
			w = lw
		}
	}
	h = len(lines)
	tw := m.width
	x = tw - w
	if x < 0 {
		x = 0
	}
	y = 0
	return x, y, w, h
}

// renderToast lays the toast box over the finished base view (top-right).
// Reuses canvasLines()/spliceLine() (the same primitives placeOverlay() uses
// internally, overlay.go) instead of placeOverlay()'s centering -- no new
// compositing primitive, just different coordinates. base is already the
// full, finished frame (including the outer border) -- composed against
// m.width/m.height (see toastGeometry's doc comment).
func (m model) renderToast(base string) string {
	if m.toast == nil {
		return base
	}
	x, y, w, _ := m.toastGeometry()
	tw, th := m.width, m.height
	bgLines := canvasLines(base, tw, th)
	fgLines := strings.Split(m.toastBox(), "\n")
	for i, fl := range fgLines {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		if pad := w - ansi.StringWidth(fl); pad > 0 {
			fl += overlayPad().Render(strings.Repeat(" ", pad))
		}
		bgLines[row] = spliceLine(bgLines[row], fl, x, w)
	}
	return strings.Join(bgLines, "\n")
}

// toastHit reports whether (mx, my) lies within the currently rendered toast
// box.
func (m model) toastHit(mx, my int) bool {
	if m.toast == nil {
		return false
	}
	x, y, w, h := m.toastGeometry()
	return mx >= x && mx < x+w && my >= y && my < y+h
}

// dismissToast closes the toast (click, devd DD2-272 AC4 parity). With a set
// target it first switches to that view -- minimal addressing (target only
// carries what a future producer needs for deeper focus-restore; no current
// producer sets more than the view).
func (m model) dismissToast() (tea.Model, tea.Cmd) {
	if m.toast != nil && m.toast.target != nil {
		m.view = m.toast.target.view
	}
	m.toast = nil
	return m, nil
}

// escDismissesToast reports whether esc closes the toast: a sticky toast is
// visible and esc has no other job in the list views. Overlays, forms, the
// palette and the help already captured esc before this is asked.
func (m model) escDismissesToast() bool {
	return m.toast != nil && m.toast.sticky &&
		!m.detailFocus && !m.filterFocus && !m.treeActive() && !m.flatView && m.view != viewReady
}
