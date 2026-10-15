package tui

// overlay.go — ANSI-safe overlay compositing (painter's algorithm), ported
// unchanged from devd (~/dev/DeveloperDashboard/apps/cli-go/
// internal/tui/overlay.go). No devd-API coupling to strip — pure string/int
// primitives already. This logic is subtle (byte-faithful line splicing on
// ANSI-escaped strings); ported verbatim, only the theme import path changed.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/theme"
)

// overlayPad is the background fill for narrow modal lines. Every modal box
// uses theme.Base as background — the padding carries the same color so no
// terminal-default stripe appears (otherwise a form footer would look
// perforated).
var overlayPad = lipgloss.NewStyle().Background(theme.Base)

// canvasLines normalizes bg into a gapless tw×th rectangle with theme.Base
// background (devd DD2-216): every cell the overlay doesn't cover itself —
// stripes left/right of the box, rows below the frame — is then Base instead
// of the terminal default (usually black). spliceLine subsequently cuts from
// a fully Base-filled bg line, so no more plain-space padding leaks through.
// Root-fix at the canvas rather than patched per consumer.
func canvasLines(bg string, tw, th int) []string {
	lines := strings.Split(bg, "\n")
	if tw <= 0 {
		return lines // width unknown (init/tests) → leave bg unchanged
	}
	n := len(lines)
	if th > n {
		n = th
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var l string
		if i < len(lines) {
			l = lines[i]
		}
		if pad := tw - ansi.StringWidth(l); pad > 0 {
			l += overlayPad.Render(strings.Repeat(" ", pad))
		}
		out = append(out, l)
	}
	return out
}

// placeOverlay places fg centered over bg (painter's algorithm, ANSI-safe).
// bg remains visible all around — only the cells the modal occupies get
// overwritten. There's no z-index in Bubble Tea, hence line-compositing.
func placeOverlay(bg, fg string, tw, th int) string {
	bgLines := canvasLines(bg, tw, th) // devd DD2-216: fully Base-filled canvas
	fgLines := strings.Split(fg, "\n")

	fgW := 0
	for _, l := range fgLines {
		if w := ansi.StringWidth(l); w > fgW {
			fgW = w
		}
	}
	fgH := len(fgLines)

	bgW := tw
	if bgW <= 0 {
		for _, l := range bgLines {
			if w := ansi.StringWidth(l); w > bgW {
				bgW = w
			}
		}
	}

	x := (bgW - fgW) / 2
	if x < 0 {
		x = 0
	}
	y := (len(bgLines) - fgH) / 2
	if y < 0 {
		y = 0
	}

	return placeCompose(bgLines, fgLines, x, y)
}

// placeOverlayAt places fg at the given absolute (x, y) over bg (Slice B,
// bt-f0y9 "feld-verankertes Inline-Dropdown", D09 revidiert) — the
// anchor-positioned sibling of placeOverlay (centered, above): SAME
// ANSI-safe painter's-algorithm compositing core (canvasLines/placeCompose/
// spliceLine), only WHERE fg lands differs — one render path shared between
// the two variants (placeCompose, below) so they can never independently
// drift on how a line actually gets spliced (mirrors panelBox/
// panelBoxTopHotkey sharing panelBoxWith, box_panel.go's own precedent).
// NOT wired into composeOverlays (view_browse_repo.go) yet — Slice C's job;
// this is the primitive plus its own unit tests only (overlay_test.go).
//
// Overflow (S6/D09 grounding Q3, deliberately UNRESOLVED here — see bt-f0y9's
// own "## Notes for Slice C"): an fg row that would land outside bg's own
// [0, height) is silently DROPPED by placeCompose below — the EXACT same
// silent-overflow behavior placeOverlay itself has always had (this file's
// top doc comment, "There's no z-index in Bubble Tea"). No upward-flip
// fallback exists anywhere in this codebase today (checked, not assumed,
// during the S6 grounding) — a popup anchored too close to the bottom of the
// pane will lose its lower rows rather than flip above its anchor; adding
// that reflow is explicitly out of this slice's scope, left to Slice C.
// x < 0 clamps to 0 (a negative anchor column is a caller bug, not a valid
// "off the left edge" request this primitive tries to make sense of) —
// placeCompose itself applies the same clamp defensively.
func placeOverlayAt(bg, fg string, tw, th, x, y int) string {
	bgLines := canvasLines(bg, tw, th)
	fgLines := strings.Split(fg, "\n")
	return placeCompose(bgLines, fgLines, x, y)
}

// placeCompose is the ONE splice loop both placeOverlay and placeOverlayAt
// (above) drive: overlays fgLines onto bgLines at (x, y), padding every fg
// line to the uniform width the widest fg line needs (otherwise a narrower
// line — e.g. a form's helper/blank line — wouldn't fully cover the
// background, and text behind it would bleed through: the overlay must be a
// gapless rectangle). fg rows whose target row falls outside
// [0, len(bgLines)) are silently skipped — placeOverlay's pre-existing
// overflow behavior, now shared verbatim by placeOverlayAt too. Mutates and
// returns bgLines' own backing via strings.Join, mirroring placeOverlay's
// prior inline behavior exactly (byte-identical extraction, Basis-Goldens-
// Gegenbeleg).
func placeCompose(bgLines, fgLines []string, x, y int) string {
	if x < 0 {
		x = 0
	}
	fgW := 0
	for _, l := range fgLines {
		if w := ansi.StringWidth(l); w > fgW {
			fgW = w
		}
	}
	for i, fl := range fgLines {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		if pad := fgW - ansi.StringWidth(fl); pad > 0 {
			fl += overlayPad.Render(strings.Repeat(" ", pad))
		}
		bgLines[row] = spliceLine(bgLines[row], fl, x, fgW)
	}
	return strings.Join(bgLines, "\n")
}

// spliceLine replaces cells [x, x+fgW) of bg with fg (ANSI-safe).
func spliceLine(bg, fg string, x, fgW int) string {
	left := ansi.Truncate(bg, x, "")
	if lw := ansi.StringWidth(left); lw < x {
		left += strings.Repeat(" ", x-lw)
	}
	right := ansi.TruncateLeft(bg, x+fgW, "")
	return left + "\x1b[0m" + fg + "\x1b[0m" + right
}
