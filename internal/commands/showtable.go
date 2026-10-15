// internal/commands/showtable.go
package commands

import (
	"context"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

// The label column is sized from the label vocabulary below rather than from
// the item at hand: two items rendered by two separate calls must produce the
// same geometry, or reading down a column -- the entire point of this view --
// stops working. An extra front matter key longer than the widest fixed label
// is the one thing that can still widen the column, so it is measured too,
// per item, and that is a deliberate trade: an unusually long key is visible
// and rare, whereas a silently uneven grid is neither.
var tableLabelVocabulary = []string{
	"title:", "tags:", "parent:", "blocked by:", "blocking:",
}

// bandIDWidth budgets the id cell in the top band from the store's own id
// shape, so "type:" starts at the same cell for every item of a store
// without spending cells a store's ids can never use. A constant fitted to
// "beans-" wasted four cells on a "SPF-" store and would have been too
// narrow for a longer prefix; an id longer than its own configuration -- a
// item carried over from another store -- pushes the band right, which is
// visible and rare, the same trade the label column makes.
func bandIDWidth(cfg *config.Config) int {
	return ui.DisplayWidth(cfg.Items.Prefix) + cfg.Items.IDLength
}

// itemTableLabelWidth is the label column shared by every item of one call.
//
// Sizing it per item broke the view's only promise across several ids: a
// item carrying customer_value got a sixteen-cell column and its neighbour
// an eleven-cell one, so consecutive grids stepped and no column could be
// followed down the page. The width is therefore the widest label any of
// the items carries, never one item's own.
func itemTableLabelWidth(items []*item.Item, cfg *config.Config) int {
	width := 0
	for _, l := range tableLabelVocabulary {
		if w := ui.DisplayWidth(l); w > width {
			width = w
		}
	}
	for _, b := range items {
		for _, bl := range itemTableBlocks(b, cfg) {
			if w := ui.DisplayWidth(bl.label); w > width {
				width = w
			}
		}
	}
	return width
}

// tableBlock is one horizontally ruled section of the grid. A band spans the
// full width and carries several label/value pairs, left-packed with one
// right-aligned trailer; a field is the two-column label/value form whose
// value wraps.
type tableBlock struct {
	band  bool
	label string
	value string
	left  []string
	right string
	// bold marks a value to be emphasised. It is a flag rather than a
	// pre-styled value because the value wraps: styling the whole string
	// first puts the opening sequence on the first line and its reset on
	// the last, so every line and border between them inherits the
	// weight. The style is applied per wrapped line instead.
	bold bool
}

// renderItemTable lays out one item's whole front matter, capped at width
// cells including the borders.
//
// The arrangement is the PO's: a band across the top carrying what the item
// *is* (id, type, status, and priority against the right edge), a band across
// the bottom carrying the managed stamps, and between them one horizontally
// ruled field per line of front matter. The bands separate identity and
// bookkeeping from content, which is what a flat label column could not do --
// there, id and created sat in the same shape as the title.
//
// It is an arrangement of the same fields renderItemHeader prints, not a
// selection of them: dropping one here would reintroduce exactly the defect
// beans-p1d0 fixed, and TestTableCarriesEveryFrontMatterField pins that.
func renderItemTable(b *item.Item, cfg *config.Config, width, labelWidth int) string {
	blocks := itemTableBlocks(b, cfg)

	// A field row is "│ " + label + " │ " + value + " │": seven cells of
	// border and padding on top of the two text columns. A band row is
	// "│ " + content + " │": four.
	valueWidth := width - labelWidth - 7
	if valueWidth < 1 {
		valueWidth = 1
	}
	bandWidth := width - 4
	if bandWidth < 1 {
		bandWidth = 1
	}

	bar := ui.TreeLine.Render("│")

	// The column line exists only across the field rows, so each horizontal
	// rule needs the junction that matches what happens to that line at
	// that height: it begins (┬), continues (┼), ends (┴), or is absent.
	// Drawing every rule straight left visible gaps where the column line
	// arrived at a rule and no connector met it.
	column := labelWidth + 3
	rule := func(left, joint, right string) string {
		bar := strings.Repeat("─", column-1)
		rest := strings.Repeat("─", width-column-2)
		return ui.TreeLine.Render(left+bar+joint+rest+right) + "\n"
	}

	var sb strings.Builder
	sb.WriteString(rule("┌", "─", "┐"))
	for i, bl := range blocks {
		if i > 0 {
			sb.WriteString(rule("├", ruleJoint(blocks, i), "┤"))
		}
		if bl.band {
			sb.WriteString(bar + " " + padVisible(packBand(bl, bandWidth), bandWidth) + " " + bar + "\n")
			continue
		}
		for j, line := range fieldValueLines(bl, valueWidth) {
			if bl.bold {
				line = ui.Bold.Render(line)
			}
			label := ""
			switch {
			case j == 0:
				label = bl.label
			case j == 1 && bl.right != "":
				label = bl.right
			}
			// The label column is padded as plain text and styled
			// afterwards: ui.PadRight measures the string it is handed,
			// so padding an already-styled cell -- as the relation id
			// was -- counted its escape sequences as width and padded
			// by nothing, which stepped the border seven cells left on
			// exactly those rows.
			sb.WriteString(bar + " " + ui.Muted.Render(ui.PadRight(label, labelWidth)) +
				" " + bar + " " + padVisible(line, valueWidth) + " " + bar + "\n")
		}
	}
	sb.WriteString(rule("└", lastJoint(blocks), "┘"))

	return sb.String()
}

// ruleJoint is the connector for the rule above block i: the column line
// starts where the first field row does, continues between two field rows,
// and ends where the fields give way to the closing band.
func ruleJoint(blocks []tableBlock, i int) string {
	above, below := !blocks[i-1].band, !blocks[i].band
	switch {
	case above && below:
		return "┼"
	case below:
		return "┬"
	case above:
		return "┴"
	default:
		return "─"
	}
}

// lastJoint is the connector in the bottom border: a ┴ only when the final
// block is a field row, so the column line actually reaches it.
func lastJoint(blocks []tableBlock) string {
	if len(blocks) > 0 && !blocks[len(blocks)-1].band {
		return "┴"
	}
	return "─"
}

// packBand left-packs a band's pairs and pushes its trailer against the right
// edge, which is what makes priority and order findable without reading the
// line: they are always in the same corner.
func packBand(bl tableBlock, bandWidth int) string {
	left := strings.Join(bl.left, "    ")
	if bl.right == "" {
		return left
	}

	gap := bandWidth - visibleWidth(left) - visibleWidth(bl.right)
	if gap < 1 {
		// Too narrow to separate the two: cut the left group so the
		// trailer survives, since it is the shorter and the more
		// positional of the two.
		left = ui.Truncate(stripANSI(left), bandWidth-visibleWidth(bl.right)-1)
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + bl.right
}

// fieldValueLines folds a field's value, guaranteeing a second line when the
// row carries a trailer -- the related item's id, which is rendered in the
// label column beneath the label.
//
// The id belongs under its label rather than beside the text: it identifies
// the row, the label column is where a reader looks for what a row *is*, and
// putting it there leaves the value column undivided, so type and title get
// the full width instead of surrendering a dozen cells on every relation
// row. Right-aligning it inside the value column, which this replaced, made
// every relation cell narrower than every other cell in the grid.
func fieldValueLines(bl tableBlock, valueWidth int) []string {
	lines := wrapVisible(bl.value, valueWidth)
	if bl.right != "" && len(lines) < 2 {
		lines = append(lines, "")
	}
	return lines
}

// wrapVisible wraps s to width *visible* cells, leaving the ANSI sequences a
// styled value carries intact.
//
// ui.WrapText cannot be used directly here: it measures the string it is
// handed, and a styled value carries escape bytes that occupy no cells, so a
// coloured relation wrapped several cells early while the grid around it
// stayed aligned. Splitting on whitespace keeps each sequence inside the
// token that opened it, so no line can end mid-escape; a single token wider
// than the column is handed to ui.WrapText stripped, because there is no way
// to hard-break inside a coloured run without splitting its sequence.
func wrapVisible(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return []string{""}
	}

	var lines []string
	cur := ""
	for _, tok := range tokens {
		switch {
		case visibleWidth(tok) > width:
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, ui.WrapText(stripANSI(tok), width)...)
		case cur == "":
			cur = tok
		case visibleWidth(cur)+1+visibleWidth(tok) <= width:
			cur += " " + tok
		default:
			lines = append(lines, cur)
			cur = tok
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// padVisible pads s to width counting only visible cells, so a styled value
// keeps the grid aligned. ui.PadRight would count the ANSI sequences.
func padVisible(s string, width int) string {
	if pad := width - visibleWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func visibleWidth(s string) int {
	return ui.DisplayWidth(stripANSI(s))
}

// stripANSI removes the SGR sequences lipgloss emits. Width arithmetic must
// see the text a terminal shows, not the bytes it consumes.
func stripANSI(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
			break
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}

// itemTableBlocks turns one item into the sketch's sequence: identity band,
// then one ruled field per front matter entry, then the stamps band.
func itemTableBlocks(b *item.Item, cfg *config.Config) []tableBlock {
	tint := lipgloss.NewStyle()
	if tc := cfg.GetType(b.Type); tc != nil {
		tint = tint.Bold(tc.Emphasis)
		if tc.Color != "" {
			tint = tint.Foreground(ui.ResolveColor(tc.Color))
		}
	}

	statusColor, statusBold := "gray", true
	if sc := cfg.GetStatus(b.Status); sc != nil {
		statusColor, statusBold = sc.Color, !sc.Archive
	}
	statusCell := lipgloss.NewStyle().Foreground(ui.ResolveColor(statusColor)).
		Bold(statusBold).Render(b.Status)
	if implicit, from := core.ImplicitStatus(b.ID); implicit != "" {
		statusCell += " " + ui.Muted.Render("↑"+implicit+" ("+from+")")
	}

	identity := tableBlock{band: true, left: []string{
		ui.Muted.Render("id:") + " " + padVisible(tint.Render(b.ID), bandIDWidth(cfg)),
		ui.Muted.Render("type:") + " " + padVisible(tint.Render(b.Type), vocabularyWidth(cfg.TypeNames())),
		ui.Muted.Render("status:") + " " + statusCell,
	}}
	if b.Priority != "" {
		priorityColor := "gray"
		if pc := cfg.GetPriority(b.Priority); pc != nil {
			priorityColor = pc.Color
		}
		identity.right = ui.Muted.Render("priority:") + " " +
			lipgloss.NewStyle().Foreground(ui.ResolveColor(priorityColor)).Render(b.Priority)
	}
	// The title is the one field a reader looks for first, and bold is the
	// only weight available that does not spend a colour the type tint
	// already uses.
	blocks := []tableBlock{identity, {label: "title:", value: b.Title, bold: true}}

	if len(b.Tags) > 0 {
		parts := make([]string, len(b.Tags))
		for i, t := range b.Tags {
			parts[i] = "#" + t
		}
		blocks = append(blocks, tableBlock{label: "tags:", value: strings.Join(parts, " ")})
	}

	if b.Parent != "" {
		text, trailer := describeRelated(b.Parent, cfg)
		blocks = append(blocks, tableBlock{label: "parent:", value: text, right: trailer})
	}
	for _, id := range b.BlockedBy {
		text, trailer := describeRelated(id, cfg)
		blocks = append(blocks, tableBlock{label: "blocked by:", value: text, right: trailer})
	}
	for _, id := range b.Blocking {
		text, trailer := describeRelated(id, cfg)
		blocks = append(blocks, tableBlock{label: "blocking:", value: text, right: trailer})
	}

	// Extra keys are front matter the schema does not name -- policy fields
	// like branch, topic or release. Sorted, because a map has no order and
	// a reader needs a stable one.
	if len(b.Extra) > 0 {
		keys := make([]string, 0, len(b.Extra))
		for k := range b.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			blocks = append(blocks, tableBlock{label: k + ":", value: formatExtraValue(b.Extra[k])})
		}
	}

	// Same row the header carries, in the grid's shape: the two views are
	// one arrangement of the same fields, and a row present in only one of
	// them is the drift TestTableCarriesEveryFrontMatterField guards.
	if att := attachmentValue(b.ID); att != "" {
		blocks = append(blocks, tableBlock{label: "attachments:", value: att})
	}

	stamps := tableBlock{band: true}
	if b.CreatedAt != nil {
		stamps.left = append(stamps.left, ui.Muted.Render("created:")+" "+
			b.CreatedAt.Format("2006-01-02 15:04 UTC"))
	}
	if b.UpdatedAt != nil {
		stamps.left = append(stamps.left, ui.Muted.Render("updated:")+" "+
			b.UpdatedAt.Format("2006-01-02 15:04 UTC"))
	}
	if b.Order != "" {
		stamps.right = ui.Muted.Render("order:") + " " + b.Order
	}
	if len(stamps.left) > 0 || stamps.right != "" {
		blocks = append(blocks, stamps)
	}

	return blocks
}

// vocabularyWidth is the widest name in a config enum, which is what makes a
// band's columns a property of the configuration rather than of the item
// being rendered.
func vocabularyWidth(names []string) int {
	w := 0
	for _, n := range names {
		if d := ui.DisplayWidth(n); d > w {
			w = d
		}
	}
	return w
}

// describeRelated names a related item by type and title, falling back to the
// bare id. A parent from another store, or one that has been deleted, is
// information the reader still needs -- an empty cell or an error would be
// worse than the id.
func describeRelated(id string, cfg *config.Config) (string, string) {
	resolver := &itemgraph.CoreResolver{Core: core}
	related, err := resolver.Item(context.Background(), id)
	if err != nil || related == nil {
		// Nothing to describe: the id becomes the value itself rather
		// than a trailer beside an empty cell.
		return id, ""
	}

	tint := lipgloss.NewStyle()
	if tc := cfg.GetType(related.Type); tc != nil {
		tint = tint.Bold(tc.Emphasis)
		if tc.Color != "" {
			tint = tint.Foreground(ui.ResolveColor(tc.Color))
		}
	}

	// The separator is a middle dot rather than spacing because wrapVisible
	// splits on fields, so a two-space gap survives only until the value
	// wraps and "epic show --table: ..." then reads as one run-on phrase.
	parts := []string{}
	if related.Type != "" {
		parts = append(parts, tint.Render(related.Type))
	}
	if related.Title != "" {
		parts = append(parts, related.Title)
	}
	return strings.Join(parts, " · "), id
}
