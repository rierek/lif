// internal/ui/render.go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// cellStyle is the colour and weight a row carries. Type word, id and title
// all take it, so a row reads as one unit rather than three fragments.
type cellStyle struct {
	tint     lipgloss.Color
	bold     bool
	hasColor bool
}

// styleFor resolves an item's type into a colour and a weight. It is a thin
// wrapper over typeStyle so cell rendering and the Legend derive the exact
// same colour rule from the exact same function — see typeStyle.
func styleFor(b *item.Item, cfg *config.Config) cellStyle {
	return typeStyle(cfg.GetType(b.Type))
}

// typeStyle resolves a type config into a colour and a weight. A type with
// no configured colour — task, by default — keeps the terminal's own text
// colour; it is the commonest row and earns no ink. Both styleFor (cell
// rendering) and Legend (columns.go) call through this one function so the
// legend's swatch for the commonest row can never disagree with what the
// cell actually renders.
func typeStyle(tc *config.TypeConfig) cellStyle {
	if tc == nil || tc.Color == "" {
		emphasis := tc != nil && tc.Emphasis
		return cellStyle{bold: emphasis}
	}
	return cellStyle{tint: ResolveColor(tc.Color), bold: tc.Emphasis, hasColor: true}
}

func (s cellStyle) render(text string) string {
	if text == "" {
		return text
	}
	st := lipgloss.NewStyle().Bold(s.bold)
	if s.hasColor {
		st = st.Foreground(s.tint)
	}
	return st.Render(text)
}

// statusCell renders the status in the decided form. Archive statuses are not
// bold: they are done, and done recedes.
func statusCell(b *item.Item, c Columns, cfg *config.Config) string {
	text := PadRight(c.StatusText(b), c.Status)
	colour := "overlay1"
	archive := true
	if sc := cfg.GetStatus(b.Status); sc != nil {
		colour = sc.Color
		archive = sc.Archive
	}
	return lipgloss.NewStyle().Foreground(ResolveColor(colour)).Bold(!archive).Render(text)
}

// prioCell renders the priority, or blank cells for normal.
func prioCell(b *item.Item, c Columns, cfg *config.Config) string {
	text := c.PrioText(b)
	if text == "" {
		return strings.Repeat(" ", c.Prio)
	}
	colour := ""
	if pc := cfg.GetPriority(b.Priority); pc != nil {
		colour = pc.Color
	}
	bold := b.Priority == "critical" || b.Priority == "high"
	return lipgloss.NewStyle().Foreground(ResolveColor(colour)).Bold(bold).
		Render(PadRight(text, c.Prio))
}

// progressCell renders a milestone's descendant completion.
func progressCell(p *Progress, c Columns) string {
	if p == nil || c.ProgressWidth == 0 {
		return strings.Repeat(" ", c.ProgressWidth)
	}
	filled := 0
	if p.Total > 0 {
		filled = (progressBarWidth*p.Done + p.Total/2) / p.Total
	}
	if filled > progressBarWidth {
		filled = progressBarWidth
	}
	bar := lipgloss.NewStyle().Foreground(ResolveColor("green")).
		Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(ResolveColor("surface2")).
			Render(strings.Repeat("░", progressBarWidth-filled))
	return bar + strings.Repeat(" ", c.Gap) + Muted.Render(PadRight(fmt.Sprintf("%d/%d", p.Done, p.Total), c.Counter))
}

// tagCell renders tags on exactly one line.
//
// Tags never hard-break: a word wrap once split #slug-tailwind-upgrade across
// three lines mid-word. The title is what must never be cut; tags are metadata
// and may be. The first tag is always shown, elided if it must be, because a
// column holding nothing but "+3" tells the reader nothing.
func tagCell(tags []string, width int) string {
	if width <= 0 || len(tags) == 0 {
		return ""
	}

	cell := func(tag string, room int) string {
		c := "#" + tag
		if DisplayWidth(c) <= room {
			return c
		}
		return Truncate(c, room)
	}

	marker := ""
	if len(tags) > 1 {
		marker = fmt.Sprintf("+%d", len(tags)-1)
	}
	room := width
	if marker != "" {
		room = width - DisplayWidth(marker) - 1
	}
	if room < 1 {
		room = 1
	}

	shown := []string{cell(tags[0], room)}
	used := DisplayWidth(shown[0])

	for i := 1; i < len(tags); i++ {
		remaining := len(tags) - i
		mark := fmt.Sprintf("+%d", remaining)
		reserve := DisplayWidth(mark) + 1
		next := cell(tags[i], width)
		if used+1+DisplayWidth(next) > width-reserve {
			// Nothing more fits. The marker must itself fit within what is
			// left after the tags already shown — if it does not, the
			// marker is dropped rather than let the cell overflow its
			// width, since staying within budget outranks reporting the
			// overflow count.
			plain := strings.Join(shown, " ")
			if used+1+DisplayWidth(mark) <= width {
				plain += " " + mark
			}
			return Muted.Render(plain)
		}
		shown = append(shown, next)
		used += 1 + DisplayWidth(next)
	}
	return Muted.Render(strings.Join(shown, " "))
}

// Form is how rows are arranged. It is the caller's choice, not a property of
// the command: a table claims its rows are peers, which is what columns promise
// and what makes sorting meaningful; a tree claims the opposite. Doing both at
// once produces a table nobody may sort.
type Form string

const (
	FormTable Form = "table"
	FormTree  Form = "tree"
)

// ParseForm validates a --view value.
func ParseForm(s string) (Form, bool) {
	switch Form(s) {
	case FormTable:
		return FormTable, true
	case FormTree:
		return FormTree, true
	}
	return "", false
}

// Render lays out rows in the requested form.
func Render(rows []Row, form Form, title string, width int, showTags bool, cfg *config.Config) string {
	if form == FormTable {
		return renderTable(rows, title, width, showTags, cfg)
	}
	return renderTree(rows, title, width, showTags, cfg)
}

func renderTable(rows []Row, title string, width int, showTags bool, cfg *config.Config) string {
	// Flat: the tree is dropped, not hidden, so the columns mean what they say.
	flat := make([]Row, 0, len(rows))
	for _, r := range rows {
		flat = append(flat, Row{Item: r.Item, Depth: 0, IsLast: true, Section: r.Section, Progress: r.Progress})
	}

	c := NewColumns(flat, width, showTags, cfg)
	c.Rebalance(flat)

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(title))
	sb.WriteString("\n")
	sb.WriteString(TreeLine.Render(strings.Repeat("─", width)))
	sb.WriteString("\n")
	sb.WriteString(c.Header())
	sb.WriteString("\n")

	gap := strings.Repeat(" ", c.Gap)
	for _, r := range flat {
		if r.Section != "" {
			sb.WriteString("\n" + Muted.Render(r.Section) + "\n")
		}
		st := styleFor(r.Item, cfg)
		parts := WrapText(r.Item.Title, c.Title)

		cells := []string{
			st.render(PadRight(c.TypeText(r.Item), c.Type)),
			st.render(PadRight(r.Item.ID, c.ID)),
			st.render(parts[0]) + strings.Repeat(" ", max(0, c.Title-DisplayWidth(parts[0]))),
			statusCell(r.Item, c, cfg),
			prioCell(r.Item, c, cfg),
		}
		if c.ProgressWidth > 0 {
			cells = append(cells, progressCell(r.Progress, c))
		}
		line := strings.Join(cells, gap)
		if c.Tags > 0 {
			if tc := tagCell(r.Item.Tags, c.Tags); tc != "" {
				line += gap + tc
			}
		}
		sb.WriteString(strings.TrimRight(line, " ") + "\n")

		lead := strings.Repeat(" ", c.Type+c.Gap+c.ID+c.Gap)
		for _, part := range parts[1:] {
			sb.WriteString(strings.TrimRight(lead+st.render(part), " ") + "\n")
		}
	}

	for _, l := range c.Legend(cfg) {
		sb.WriteString(l + "\n")
	}
	return sb.String()
}

func renderTree(rows []Row, title string, width int, showTags bool, cfg *config.Config) string {
	blocks, c := treeLayout(rows, rows, 0, width, showTags, cfg)

	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render(title))
	sb.WriteString("\n")
	sb.WriteString(TreeLine.Render(strings.Repeat("─", width)))
	sb.WriteString("\n")

	for i, r := range rows {
		if r.Section != "" {
			sb.WriteString("\n" + Muted.Render(r.Section) + "\n")
		}
		for _, l := range blocks[i] {
			sb.WriteString(l + "\n")
		}
	}

	for _, l := range c.Legend(cfg) {
		sb.WriteString(l + "\n")
	}
	return sb.String()
}

// TreeBlocks lays out rows exactly as the tree form of Render does and returns
// the lines of each row, index-aligned with rows: no title, rule, section
// heading or legend. An item can use several lines (wrapped title), so a
// caller that needs a cursor or a hit test works on whole blocks. This is the
// one row renderer of lif roadmap and the TUI list.
func TreeBlocks(rows []Row, width int, showTags bool, cfg *config.Config) [][]string {
	blocks, _ := treeLayout(rows, rows, 0, width, showTags, cfg)
	return blocks
}

// markerWidth is the marker column of an interactive list: the glyph and a gap.
const markerWidth = 2

// TreeBlocksMeasured is TreeBlocks for an interactive list. The column widths
// are measured over measure instead of rows: a list passes every row it could
// show, so the columns do not move when a node opens or closes. Every line
// also starts with markerWidth cells for Row.Marker and a gap (blank without
// a marker), left of the tree lines, so a marker never moves a tree line or a
// column. rows must be a subset of measure.
func TreeBlocksMeasured(rows, measure []Row, width int, showTags bool, cfg *config.Config) [][]string {
	blocks, _ := treeLayout(rows, measure, markerWidth, width, showTags, cfg)
	return blocks
}

// treeLayout renders every row of the tree form into its block of lines and
// returns the column layout it used (renderTree needs it for the legend).
// Column widths come from measure. markerCells is the width of the leading
// marker column (0, or markerWidth for the interactive list), taken from width.
func treeLayout(rows, measure []Row, markerCells, width int, showTags bool, cfg *config.Config) ([][]string, Columns) {
	c := NewColumns(measure, width-markerCells, showTags, cfg)

	leadWidth := c.Indent + c.Type + c.Gap
	rightWidth := func(tags int) int {
		r := c.Status + c.Gap + c.Prio + c.ID + c.Gap*2
		if tags > 0 {
			r += c.Gap + tags
		}
		if c.ProgressWidth > 0 {
			r += c.Gap + c.ProgressWidth
		}
		return r
	}
	// clampToMinRenderableTitle, not an invented tree-only floor: the table's
	// last resort (columns.go) and the tree's must be the same number and
	// the same story, or the two forms overflow their terminal by different
	// amounts for the same width. Below it the title cannot hold even a
	// truncation ellipsis meaningfully; this is the one place this layout
	// accepts overflow rather than guarding against it.
	body := clampToMinRenderableTitle(width - markerCells - leadWidth - rightWidth(c.Tags))

	// Same redistribution as the table: unclaimed title width goes to the tags.
	tmp := Columns{Title: body, Tags: c.Tags, Gap: c.Gap}
	tmp.Rebalance(measure)
	body, c.Tags = tmp.Title, tmp.Tags

	blocks := make([][]string, len(rows))

	gap := strings.Repeat(" ", c.Gap)
	for i, r := range rows {
		st := styleFor(r.Item, cfg)
		conn := r.Connector()
		word := c.TypeText(r.Item)

		// The marker cell sits left of the tree lines; the lead keeps its
		// CLI width, so every other column stays where the CLI puts it.
		markerCell := strings.Repeat(" ", markerCells)
		if markerCells > 0 && r.Marker != "" {
			markerCell = TreeLine.Render(r.Marker) + strings.Repeat(" ", markerCells-1)
		}
		padding := strings.Repeat(" ", max(0, leadWidth-DisplayWidth(conn)-DisplayWidth(word)))
		lead := markerCell + TreeLine.Render(conn) + st.render(word) + padding

		parts := WrapText(r.Item.Title, body)
		// The ID is padded to c.ID, the width rightWidth() already charges
		// the title for. Rendering it unpadded spent the title's cells and
		// left everything after a short ID ragged (renderTable pads it).
		// The line is TrimRight'ed below, so a trailing pad costs nothing.
		right := statusCell(r.Item, c, cfg) + gap + prioCell(r.Item, c, cfg) + gap +
			st.render(PadRight(r.Item.ID, c.ID))
		if c.ProgressWidth > 0 {
			right += gap + progressCell(r.Progress, c)
		}

		line := lead + st.render(parts[0]) +
			strings.Repeat(" ", max(0, body-DisplayWidth(parts[0]))) + gap + right
		if c.Tags > 0 {
			if tc := tagCell(r.Item.Tags, c.Tags); tc != "" {
				line += gap + tc
			}
		}
		lines := []string{strings.TrimRight(line, " ")}

		stem := r.Stem()
		hang := strings.Repeat(" ", markerCells) + TreeLine.Render(stem) + strings.Repeat(" ", max(0, leadWidth-DisplayWidth(stem)))
		for _, part := range parts[1:] {
			lines = append(lines, strings.TrimRight(hang+st.render(part), " "))
		}
		blocks[i] = lines
	}
	return blocks, c
}
