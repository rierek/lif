package tui

// render_shared.go — shared render primitives used by multiple future screens
// (Tree/Ready/Detail, T8+). Ported from devd (~/dev/Developer
// Dashboard/apps/cli-go/internal/tui/render_shared.go), stripped to the
// view-model-free primitives named in the port scope (design-spec.md,
// implementation-plan »E1 Task 7«): pane/renderPane/borderedPane/tagsInline.
// devd's issueFields/issueMetaPairs/entityMetaLine are NOT ported here — they
// take devd's api.Issue/api.Tag directly, which would pull a data-layer
// import into the chrome package (forbidden per task scope); their beans-
// shaped equivalents belong to the view that needs them (T8+), not this
// infrastructure layer.

import (
	"hash/fnv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// pane is a bordered column (master lists, previews). PF-10 (design-spec.md
// §15, epic-E7-plan.md »Task 5«, bean bt-uyzf) removed the title field: a
// pane-internal title + underline-separator duplicated the Breadcrumb's own
// view identity (PO-Nachtrag 4, PO wörtlich: "Es genügt, wenn es in den
// Breadcrumbs ... angezeigt wird. Dann die Suche - sonst ist es obsolet.") --
// the Breadcrumb (Chrome-Zeile 1) is now the SOLE carrier of view identity,
// renderPane starts straight into rows.
type pane struct {
	rows   []string
	cursor int
	isList bool
}

// renderPane renders a bordered pane. Golden Rule #1 (devd DD2-54): no
// Height() on the bordered style — instead the content is padded to the
// inner height (h lines) and the border grows around it naturally (total
// height h+2). That way the alignment never tips over if a future row isn't
// truncated: the line count is explicitly capped, not dependent on Height().
func renderPane(p pane, w, h int, focused bool) string {
	lines := make([]string, 0, h)
	for i := 0; i < len(p.rows) && len(lines) < h; i++ { // max h lines, no title/separator budget anymore (PF-10)
		row := truncate(p.rows[i], w-2)
		if p.isList && i == p.cursor && focused {
			row = theme.Accent.Render("▸ ") + row
		} else if p.isList {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	for len(lines) < h { // pad to inner height instead of forcing Height()
		lines = append(lines, "")
	}
	border := lipgloss.RoundedBorder()
	col := theme.Overlay
	if focused {
		col = theme.Mauve
	}
	return lipgloss.NewStyle().
		Width(w).
		Border(border).BorderForeground(col).
		Render(strings.Join(lines, "\n"))
}

// borderedPane pads/caps content to h inner lines and wraps it in a
// RoundedBorder (Golden Rule #1: no Height() on a bordered style). Total
// height = h+2.
func borderedPane(lines []string, w, h int, border lipgloss.Color) string {
	out := append([]string{}, lines...)
	if len(out) > h {
		out = out[:h]
	}
	for len(out) < h {
		out = append(out, "")
	}
	return lipgloss.NewStyle().Width(w).
		Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Render(strings.Join(out, "\n"))
}

// tagChipPalette is a small fixed accent palette (design-spec.md §8: "Tag-
// Picker mit Nutzungszählern; Farbe per Hash aus fester Palette") — beans
// tags carry no color of their own (internal/item.Item.Tags is []string,
// unlike devd's api.Tag{Color}), so the color is derived deterministically
// from the tag name instead of being stored.
func tagChipPalette() []lipgloss.Color {
	return []lipgloss.Color{
		theme.Mauve, theme.Peach, theme.Green, theme.Blue,
		theme.Teal, theme.Yellow, theme.Pink, theme.Sapphire,
	}
}

// tagSwatch renders "● name" in a color hashed from the tag name (stable
// across renders/process runs, since fnv is deterministic).
func tagSwatch(name string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	pal := tagChipPalette()
	col := pal[h.Sum32()%uint32(len(pal))]
	return lipgloss.NewStyle().Foreground(col).Render("● " + name)
}

// tagsInline renders tag swatches space-separated (empty string for no
// tags). Port-adaptation of devd's tagsInline([]api.Tag): takes plain tag
// names since beans has no separate Tag entity/color field.
func tagsInline(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	chips := make([]string, len(tags))
	for i, t := range tags {
		chips[i] = tagSwatch(t)
	}
	return strings.Join(chips, " ")
}
