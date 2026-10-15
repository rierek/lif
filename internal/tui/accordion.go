package tui

// accordion.go — exclusive-open, digit-addressed detail sections (design-spec
// §6 V4). Port of devd accordion.go's render algebra (renderAccordion,
// originally also fieldStrip), stripped of every edit-field/detailField-
// editor concept (E3 scope) -- E2's "fields" are pure navigation targets
// inside the Beziehungen section (relationField), never edit targets.
//
// fieldStrip (a separate "Fields: ..." row shown above an active section's
// body) was REMOVED entirely by B04 (design-spec.md §15 PF-17, bean
// bt-b0w0, 2026-07-16): RELATIONS was its only remaining caller by then
// (Meta's own field cursor already lived inline via metaSectionBody's ▷/▶
// per-row markers, PF-4) -- relationsSectionBody (view_detail_bean.go) now
// renders that SAME cursor convention per-row instead, so every section's
// field cursor lives inline in its own body, never in a second strip.
//
// Port references (~/dev/DeveloperDashboard/apps/cli-go/internal/
// tui/): accordion.go:309-355 (renderAccordion), editor.go:102-126
// (glowRender).

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/theme"
)

// relationField is a navigation target inside a section's field list --
// EITHER a jump target (Relations section, kind == "") OR a Meta field (PF-4,
// E7 T4, bean bt-kyj5, design-spec.md §15): kind drives T6's enter-dispatch
// (status/type/priority open the seeded Value-Menu, tags opens the
// Tag-Picker, title opens the Title-Edit-Form). beanID == "" marks either an
// unresolved/dangling relation reference (not jumpable) or any Meta field
// (Meta fields are never jump targets, beanID is always "" there). The
// former "readonly" kind (created_at/updated_at, a No-Op) was REMOVED by
// bt-lg68 (PO-Nebenbefund, US-Review Runde 3): those two Meta entries
// doubled up with HISTORY, which is now the sole Created/Updated source --
// metaFields (view_detail_bean.go) no longer produces "readonly" entries.
type relationField struct {
	beanID string // target bean ID; "" for an unresolved/dangling reference (not jumpable) or any Meta field
	label  string // pre-rendered row text (theme colors already applied)
	kind   string // "" = jump (Relations, E2 behavior unchanged) | "status"|"type"|"priority" = Value-Menu seeded on group | "tags" = Tag-Picker (PF-15/D01, E8 Task 1, bean bt-e6q9) | "title" = Title-Edit-Form
}

// accordionSection mirrors devd's shape minus the editor-specific parts.
// fields == nil for every section except Beziehungen (Meta/Body/Historie are
// pure display, no field-level navigation in E2).
//
// activeLine (bt-se4q, bt-b0w0-Review Follow-up B01) is RELATIONS-only: the
// display-line index of the active row within `body`, as computed by
// relationsSectionBody (view_detail_bean.go) while it builds the section --
// zero-value (0) for every other section, unused by renderAccordion itself,
// only read by windowRelationsSection's caller (renderAccordionPane,
// view_browse_repo.go) so it no longer has to re-derive the cursor position
// by rescanning the ALREADY-RENDERED body text for a "▶" glyph.
type accordionSection struct {
	title      string
	body       string
	fields     []relationField
	activeLine int
}

// renderAccordion rendert die Sektions-Header (`> [n] Title`) und klappt die
// offene Sektion (1-basiert, exklusiv) als eingerückten Body darunter auf
// (port devd accordion.go:309-355). active/activeIdx/fieldIdx replace devd's
// detailFocusView{sec,field}.
//
// PF-1 (design-spec.md §15, E7 T4, bean bt-kyj5) originally made section 1
// (Meta) a forced-open exception -- its body rendered regardless of `open`,
// while sections 2-4 stayed exclusive-open. PF-18 REVISED PF-1 (design-
// spec.md §15, PO-Feedback 2026-07-16, bean bt-98cb): the PO wants Meta
// default-CLOSED, since the relevant info already lives in the Meta-Strip
// header (detailHeaderBlock) -- Meta only opens when actively selected. All
// four sections are now exclusive-open with NO special case, section 1
// included.
//
// fieldIdx is UNUSED inside this function since B04 removed fieldStrip (its
// only reader here) -- kept in the signature anyway (not worth rippling a
// removal through every call site, view_browse_repo.go/mouse.go/tests, for
// a parameter Go happily allows unused): every section's field cursor now
// renders INSIDE its own body string (metaSectionBody's ▷/▶ markers, PF-4;
// relationsSectionBody's own, B04), built by beanSections BEFORE
// renderAccordion ever sees s.body.
func renderAccordion(secs []accordionSection, open, w int, active bool, activeIdx, fieldIdx int) string {
	if len(secs) == 0 {
		return theme.Dim.Render("(no detail fields)")
	}
	headerStyle := lipgloss.NewStyle().Width(w)
	// DD2-186 (devd note, carried over): lipgloss.Width INCLUDES PaddingLeft
	// -> Width(w).PaddingLeft(2) keeps the effective content width == the
	// caller's own bodyW (w-2), indent stays 2.
	boxStyle := lipgloss.NewStyle().Width(w).PaddingLeft(2)

	var b strings.Builder
	for i, s := range secs {
		n := i + 1
		isOpen := n == open                   // PF-18: exclusive-open for every section, Meta (n==1) included (revises PF-1)
		activeSec := active && activeIdx == i // D08: focus is on this section
		marker := theme.Chevron.Render("> ") + theme.Key.Render(fmt.Sprintf("[%d]", n)) + " "
		// B05 (design-spec.md §15 PF-16, bean bt-ntoz/bt-czpf, 2026-07-16):
		// the former "  ▾"/"  ▸" hint suffix was removed -- redundant, the
		// section's open/closed state is already visible from whether its
		// body renders below the header (isOpen, above).
		var title string
		if isOpen {
			title = theme.Header.Render(s.title)
		} else {
			// B06 EXPERIMENT (same source): closed header title Teal
			// (theme.HeaderInactive) instead of Muted/Hint-grey -- PO-Sign-
			// off pending, one-line rollback to theme.Muted if rejected.
			title = theme.HeaderInactive.Render(s.title)
		}
		// PF-12 (design-spec.md §15, E7 T4): BOTH branches now reserve
		// exactly 1 gutter column (" " inactive, "▌" active) and truncate
		// content to w-1 -- previously only the active branch reserved a
		// column (truncate to w-1), the inactive branch used the FULL w with
		// no prefix, so a header's own title text shifted 1 column depending
		// on whether IT ITSELF was active (layout shift on selection).
		var header string
		if activeSec {
			// D08: active section = ▌ bar + whole line accent-tinted (own
			// per-cell colors stripped first, mirrors the tree cursor).
			header = theme.Accent.Render("▌" + truncate(ansi.Strip(marker+title), w-1))
		} else {
			header = " " + truncate(marker+title, w-1)
		}
		b.WriteString(headerStyle.Render(header) + "\n")
		if isOpen {
			// B04 (design-spec.md §15 PF-17, bean bt-b0w0): the former
			// fieldStrip row (a separate "Fields: ..." line shown above an
			// active section's body) is REMOVED -- RELATIONS was its only
			// remaining caller (Meta's own field cursor already lived
			// inline via metaSectionBody's ▷/▶ per-row markers, PF-4;
			// Body/History carry no .fields at all). relationsSectionBody
			// now renders that SAME ▷/▶ cursor convention per-row, inline
			// in s.body itself -- there is no longer a second, separate
			// representation of the field cursor for ANY section.
			b.WriteString(boxStyle.Render(s.body) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// glowRender rendert Markdown für die Body-Section via glamour. Fällt auf den
// rohen Text zurück, wenn glamour fehlschlägt. Im Ascii-Profil (Tests/kein
// TTY -> an dasselbe Profil gekoppelt wie die Goldens) wird der plain
// "notty"-Style genutzt, damit Golden-Snapshots ANSI-frei und stabil bleiben
// (port devd editor.go:102-126, verbatim).
//
// I02 (E2-T1-quality-review, decision made at Task 2 wiring time): this
// constructs a brand-new glamour.TermRenderer on EVERY call -- and, now that
// view_browse_repo.go's renderDetailPane wires beanSections into the live
// View() path, that means once per render tick while the Body section is
// open (every keypress, not just on bean-change). Decision: ACCEPT the cost
// rather than add a cache -- this mirrors devd's own upstream editor.go
// behavior 1:1 (same per-call construction there), bean bodies are typically
// a few KB of markdown (well under interactive-latency budgets), and E2 has
// no profiling evidence of this being a hot path. If profiling ever shows
// otherwise, the fix is a cache keyed by (bean.ID, bean.ETag, width) on
// model -- NOT implemented here (YAGNI until proven necessary).
//
// I03 (optional, E2-T1-quality-review): the two error-fallback paths below
// return the raw, UNWRAPPED markdown source verbatim. Accepted residual
// risk, not fixed here: both errors are effectively unreachable in practice
// (NewTermRenderer fails only on invalid style/option construction -- both
// hardcoded/valid here -- and Render() failing on well-formed markdown input
// is exceedingly rare), and the raw-text fallback already goes through
// bodySectionBody's caller-side wrapText-wrapped siblings (Meta/Historie) --
// only the Body section itself could, in the extremely unlikely error case,
// render an unwrapped line. Wrapping the fallback (wrapText(md, width)) would
// be the fix if this is ever observed in practice.
func glowRender(md string, width int) string {
	if strings.TrimSpace(md) == "" {
		return ""
	}
	w := width
	if w <= 0 {
		w = 80
	}
	style := "dark"
	if lipgloss.ColorProfile() == termenv.Ascii {
		style = "notty"
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return strings.TrimRight(out, "\n")
}
