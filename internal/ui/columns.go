// internal/ui/columns.go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// Progress is a milestone's descendant completion, shown as a bar plus n/m.
type Progress struct {
	Done  int
	Total int
}

// Row is one line of output: an item plus the tree shape needed to draw it.
//
// AncestorsLast records, for each ancestor, whether that ancestor was the last
// child at its level. That is what decides whether a vertical line continues
// past this row or stops. Index 0 corresponds to the root and is never
// consulted by Connector or Stem — roots draw nothing, so the stem starts one
// level in.
//
// Invariant: a Row with Depth > 0 must carry AncestorsLast of length Depth —
// Connector and Stem index AncestorsLast[1:] and panic on a shorter slice.
type Row struct {
	Item          *item.Item
	Depth         int
	AncestorsLast []bool
	IsLast        bool
	// Section, when set, prints as a heading above this row — the roadmap's
	// "Unscheduled" bucket is the only current use.
	Section  string
	Progress *Progress
	// Marker, when set, is the one-cell expand/collapse glyph of an
	// interactive list (TreeBlocksMeasured); the tree form draws it in the
	// first cell of the line, left of the tree lines. lif roadmap leaves it empty.
	Marker string
}

// Connector is the tree prefix that leads into this row's type word.
// Roots draw nothing, so the stem starts one level in.
func (r Row) Connector() string {
	if r.Depth == 0 {
		return ""
	}
	var sb strings.Builder
	for _, last := range r.AncestorsLast[1:] {
		if last {
			sb.WriteString("   ")
		} else {
			sb.WriteString("│  ")
		}
	}
	if r.IsLast {
		sb.WriteString("└─ ")
	} else {
		sb.WriteString("├─ ")
	}
	return sb.String()
}

// Stem is what a wrapped row's continuation lines carry in place of the
// connector: the ancestors' verticals, plus this row's own vertical while
// siblings still follow it. Same width as Connector by construction.
func (r Row) Stem() string {
	if r.Depth == 0 {
		return ""
	}
	var sb strings.Builder
	for _, last := range r.AncestorsLast[1:] {
		if last {
			sb.WriteString("   ")
		} else {
			sb.WriteString("│  ")
		}
	}
	if r.IsLast {
		sb.WriteString("   ")
	} else {
		sb.WriteString("│  ")
	}
	return sb.String()
}

// RowsFromFlatItems adapts the existing tree flattening to the row model.
//
// ancestry is truncated back to the current item's depth before use: a flat
// list returning to a shallower depth (e.g. after a grandchild, back to a
// second-level sibling) must not carry forward an ancestor from the deeper
// branch it just left.
func RowsFromFlatItems(items []FlatItem) []Row {
	rows := make([]Row, 0, len(items))
	ancestry := []bool{}
	for _, it := range items {
		if it.Depth < len(ancestry) {
			ancestry = ancestry[:it.Depth]
		}
		for len(ancestry) < it.Depth {
			ancestry = append(ancestry, true)
		}
		anc := make([]bool, len(ancestry))
		copy(anc, ancestry)
		rows = append(rows, Row{
			Item:          it.Item,
			Depth:         it.Depth,
			AncestorsLast: anc,
			IsLast:        it.IsLast,
		})
		ancestry = append(ancestry, it.IsLast)
	}
	return rows
}

// FlatRows puts every item at depth 0, which is what the table form needs:
// dropping the tree rather than hiding it is what makes the rows sortable.
func FlatRows(items []*item.Item) []Row {
	rows := make([]Row, 0, len(items))
	for _, b := range items {
		rows = append(rows, Row{Item: b, Depth: 0, IsLast: true})
	}
	return rows
}

// minTitleWidth gates the long-form upgrades: an upgrade is only bought while
// the title keeps at least this many cells. It is a purchase criterion, never
// an actual width — using it as one is how a row grew past its terminal.
const minTitleWidth = 45

// progressBarWidth is the filled/empty bar in the milestone progress column.
const progressBarWidth = 6

// tagsCrushWidth is the floor below which the tags column is dropped rather
// than left to crush the title. It is well below minTitleWidth on purpose:
// giving up tags costs the reader far less than giving up a long-form column.
const tagsCrushWidth = 25

// minRenderableTitle is the absolute floor the title column is clamped up to
// after every other adjustment. It is the one place this layout can still
// overflow its terminal width — an ID wide enough to eat the whole budget
// forces the title back up past what was actually available.
const minRenderableTitle = 12

// headerTagsLabel is the TAGS column's header word — see Header. Rebalance
// floors the tags column at this label's width so a shrink can never leave
// the column narrower than its own header, which is why Header and
// Rebalance both read this constant instead of each carrying "TAGS".
const headerTagsLabel = "TAGS"

// clampToMinRenderableTitle floors a title-column width at minRenderableTitle.
// The table (NewColumns) and the tree/detail form (renderTree in render.go)
// must apply the exact same floor to the exact same constant, or the two
// forms overflow their terminal by different amounts at the same width —
// this is the one place both call through instead of each computing it.
func clampToMinRenderableTitle(w int) int {
	if w < minRenderableTitle {
		return minRenderableTitle
	}
	return w
}

// Columns holds the resolved widths for one rendering.
//
// Short and long forms are decided here and nowhere else. Cell renderers ask
// this type for their text, so the header can never promise a form the cells
// do not deliver.
type Columns struct {
	Width  int
	Gap    int
	Indent int // tree depth times three; folded into the type cell, not its own column

	Type   int
	ID     int
	Title  int
	Status int
	Prio   int
	Tags   int

	// ProgressWidth is 0 unless rows carry Progress. Named for the width, not
	// the thing, so it does not read like the Progress type beside it.
	ProgressWidth int
	Counter       int // width of "n/m", measured from the data

	LongType   bool
	LongStatus bool
	LongPrio   bool
}

// NewColumns resolves every width for the given rows at the given terminal
// width. It starts compact and buys long forms in a fixed order — status,
// then type, then priority — while the title can afford them.
func NewColumns(rows []Row, width int, showTags bool, cfg *config.Config) Columns {
	// Long forms cost what the longest row needs, never a fixed guess.
	// Floors are the header words, so a table header never overflows.
	wantType, wantStatus, wantPrio := len("TYPE"), len("STATUS"), 0
	for _, r := range rows {
		wantType = max(wantType, DisplayWidth(r.Item.Type))
		wantStatus = max(wantStatus, DisplayWidth(r.Item.Status))
		if p := (Columns{LongPrio: true}).PrioText(r.Item); p != "" {
			wantPrio = max(wantPrio, DisplayWidth(p), len("PRIORITY"))
		}
	}
	c := Columns{Width: width, Gap: 2, Type: 1, Status: 1, Prio: 1}

	maxDepth := 0
	idWidth := 2
	counter := 0
	hasProgress := false
	for _, r := range rows {
		if r.Depth > maxDepth {
			maxDepth = r.Depth
		}
		if w := DisplayWidth(r.Item.ID); w > idWidth {
			idWidth = w
		}
		if r.Progress != nil {
			hasProgress = true
			if w := DisplayWidth(fmt.Sprintf("%d/%d", r.Progress.Done, r.Progress.Total)); w > counter {
				counter = w
			}
		}
	}
	c.Indent = 3 * maxDepth
	c.ID = idWidth
	if hasProgress {
		c.Counter = counter
		c.ProgressWidth = progressBarWidth + c.Gap + counter
	}
	if showTags {
		if width >= 120 {
			c.Tags = 24
		} else {
			c.Tags = 18
		}
	}

	budget := func() int {
		fixed := c.Indent + c.Type + c.Gap + c.ID + c.Gap + c.Gap + c.Status + c.Gap + c.Prio
		if c.Tags > 0 {
			fixed += c.Gap + c.Tags
		}
		if c.ProgressWidth > 0 {
			fixed += c.Gap + c.ProgressWidth
		}
		return width - fixed
	}

	// Buying order: status first, because T and D and C are the codes a reader
	// stumbles over most; then type; then priority, whose symbols read fine.
	upgrades := []struct {
		field *int
		flag  *bool
		want  int
	}{
		{&c.Status, &c.LongStatus, wantStatus},
		{&c.Type, &c.LongType, wantType},
		{&c.Prio, &c.LongPrio, wantPrio},
	}
	for _, u := range upgrades {
		if u.want == 0 {
			continue // nothing to show on this axis: keep the 1-cell short form
		}
		cost := u.want - *u.field
		if budget()-cost < minTitleWidth {
			// The order is fixed: once one upgrade cannot be afforded, a
			// later, cheaper upgrade must not be bought either — otherwise
			// priority could go long while type and status stay short.
			break
		}
		*u.field = u.want
		*u.flag = true
	}

	c.Title = budget()
	if c.Title < tagsCrushWidth && c.Tags > 0 {
		// Tags outrank the long forms, not the other way round: the purchase
		// loop above already ran with tags still occupying their share of the
		// budget, so a narrow terminal denies status/type/prio their long
		// form before it ever touches tags. This check only fires afterwards,
		// when the title is still crushed even with every upgrade declined —
		// which is why tagsCrushWidth sits below minTitleWidth: by the time
		// tags are on the table, no upgrade ever was. Dropping tags is the
		// last resort here, not the first.
		c.Tags = 0
		c.Title = budget()
	}
	c.Title = clampToMinRenderableTitle(c.Title)
	return c
}

// Rebalance hands width the titles do not need over to the tags.
//
// The title column otherwise swallows everything left over, even when no
// title comes near filling it, while the tags beside it are elided. Width is
// information; unclaimed width belongs to whoever still has something to say.
//
// Two moves happen, and only one of them can fire for a given row set:
//
//   - Tags want more than they have: title gives up its spare, capped by
//     what tags actually need and by what title can give up without going
//     below its own floor. This move only ever transfers width — the total
//     held by the two columns together is unchanged.
//   - Tags hold more than their content needs: that excess is released.
//     It is not handed to the title — the title only ever gives, it never
//     receives back — so this move can shrink the two columns' total, but
//     never grow it and never grow the title.
//
// Either way, Rebalance never enlarges what NewColumns already decided; it
// only redistributes or releases width within that budget.
func (c *Columns) Rebalance(rows []Row) {
	if c.Tags <= 0 || len(rows) == 0 {
		return
	}

	neededTitle := 0
	neededTags := 0
	for _, r := range rows {
		if w := DisplayWidth(r.Item.Title); w > neededTitle {
			neededTitle = w
		}
		if w := DisplayWidth(joinTags(r.Item.Tags)); w > neededTags {
			neededTags = w
		}
	}

	// The title floor is minRenderableTitle, not neededTitle alone: Rebalance
	// must not undo the readability guarantee NewColumns already enforces,
	// even when the actual title content would ask for less than that.
	titleFloor := neededTitle
	if titleFloor < minRenderableTitle {
		titleFloor = minRenderableTitle
	}

	switch {
	case c.Tags < neededTags:
		spare := c.Title - titleFloor
		if spare <= 0 {
			return
		}
		give := neededTags - c.Tags
		if give > spare {
			give = spare
		}
		c.Title -= give
		c.Tags += give
	case c.Tags > neededTags:
		// Released tags width goes to Title, but only up to what the longest
		// title needs: a title wrapped for lack of room gets it back, a
		// title column is never padded past any title.
		give := c.Tags - max(neededTags, DisplayWidth(headerTagsLabel))
		if need := neededTitle - c.Title; need > 0 {
			c.Title += min(give, need)
		}
		//
		// Floored at headerTagsLabel's width, not just neededTags: Header()
		// is computed after Rebalance runs, so if this shrink were left
		// free to go below "TAGS"'s own width the column would end up
		// narrower than the word it is titled with.
		c.Tags = neededTags
		if tagsFloor := DisplayWidth(headerTagsLabel); c.Tags < tagsFloor {
			c.Tags = tagsFloor
		}
	}
}

// joinTags renders an item's tags the way they appear in the tag column.
func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = "#" + t
	}
	return strings.Join(parts, " ")
}

// TypeText renders the type in the form this layout decided on.
func (c Columns) TypeText(b *item.Item) string {
	if c.LongType {
		return b.Type
	}
	return ShortType(b.Type)
}

// StatusText renders the status in the form this layout decided on.
func (c Columns) StatusText(b *item.Item) string {
	if c.LongStatus {
		return b.Status
	}
	return ShortStatus(b.Status)
}

// PrioText renders the priority, or "" for normal — an unremarkable priority
// earns no ink.
func (c Columns) PrioText(b *item.Item) string {
	if b.Priority == "" || b.Priority == "normal" {
		return ""
	}
	if c.LongPrio {
		return b.Priority
	}
	return GetPrioritySymbol(b.Priority)
}

// Header labels the columns. Short columns get short labels, so the header
// never promises a form the cells do not deliver. Padding is computed on the
// plain label text and only the finished, padded line is coloured — colouring
// each cell first would let its ANSI escapes count toward PadRight's width.
func (c Columns) Header() string {
	cells := []string{
		PadRight(headerLabel(c.LongType, "TYPE", "T"), c.Indent+c.Type),
		PadRight("ID", c.ID),
		PadRight("TITLE", c.Title),
		PadRight(headerLabel(c.LongStatus, "STATUS", "S"), c.Status),
		PadRight(headerLabel(c.LongPrio, "PRIORITY", "P"), c.Prio),
	}
	if c.ProgressWidth > 0 {
		cells = append(cells, PadRight("PROGRESS", c.ProgressWidth))
	}
	if c.Tags > 0 {
		cells = append(cells, PadRight(headerTagsLabel, c.Tags))
	}
	gap := strings.Repeat(" ", c.Gap)
	// TrimRight, not just Join: the last cell pads to its full width like any
	// other, and a header ending in spaces is invisible in a file but
	// glaring next to a flat raster in a terminal.
	return Muted.Render(strings.TrimRight(strings.Join(cells, gap), " "))
}

// headerLabel picks the header word to match the form NewColumns decided on
// for this axis — the long word when it bought the long form, the same
// single-letter code the cells fall back to otherwise.
func headerLabel(long bool, wide, narrow string) string {
	if long {
		return wide
	}
	return narrow
}

// legendLead is the column an entry sits in: the axis label itself is padded
// to this many cells (see PadRight calls below), so a continuation line
// carries the same number of leading spaces to keep entries aligned under
// entries rather than sliding left under the label.
const legendLead = 9

// legendSep joins entries on one legend line. It is a plain-text constant
// because wrapLegendLine measures against it before any colour is applied —
// see wrapLegendLine's own comment for why measuring must happen first.
const legendSep = " · "

// Legend names every abbreviation still on screen, in the same colours the
// cells use. An axis written out in full explains itself and is left out —
// no abbreviation, no legend entry for it.
func (c Columns) Legend(cfg *config.Config) []string {
	var lines []string

	if !c.LongType {
		var plain, styled []string
		for _, tc := range cfg.TypeList() {
			tc := tc
			style := typeStyle(&tc)
			code := ShortType(tc.Name)
			plain = append(plain, code+" "+tc.Name)
			styled = append(styled, style.render(code)+Muted.Render(" "+tc.Name))
		}
		lines = append(lines, wrapLegendLine("type", plain, styled, c.Width)...)
	}

	if !c.LongStatus {
		var plain, styled []string
		for _, sc := range cfg.StatusList() {
			style := lipgloss.NewStyle().Foreground(ResolveColor(sc.Color)).Bold(!sc.Archive)
			code := ShortStatus(sc.Name)
			plain = append(plain, code+" "+sc.Name)
			styled = append(styled, style.Render(code)+Muted.Render(" "+sc.Name))
		}
		lines = append(lines, wrapLegendLine("status", plain, styled, c.Width)...)
	}

	if !c.LongPrio {
		var plain, styled []string
		for _, pc := range cfg.PriorityList() {
			sym := GetPrioritySymbol(pc.Name)
			if sym == "" {
				continue // normal has no symbol and needs no explanation
			}
			style := lipgloss.NewStyle().Foreground(ResolveColor(pc.Color)).
				Bold(pc.Name == "critical" || pc.Name == "high")
			plain = append(plain, sym+" "+pc.Name)
			styled = append(styled, style.Render(sym)+Muted.Render(" "+pc.Name))
		}
		lines = append(lines, wrapLegendLine("priority", plain, styled, c.Width)...)
	}

	if len(lines) == 0 {
		return nil
	}
	return append([]string{""}, lines...) // blank line sets the legend off from the table
}

// wrapLegendLine reflows one axis's entries into lines that fit width cells,
// instead of joining every entry onto a single line regardless of how wide
// it grows — which is the defect this function replaces. Truncating with an
// ellipsis was rejected: the legend's whole purpose is to name every
// abbreviation on screen, and cutting the list short would silently drop
// mappings for whichever entries did not fit.
//
// A continuation line carries legendLead worth of blank space instead of the
// axis label, so entries keep lining up under entries. Measurement happens
// on plain, an entry always joins the line it started even if that alone
// already exceeds width — a single entry is never split, only the join
// between entries wraps — so the loop always makes progress and a line is
// never emitted empty. That mirrors the tradeoff WrapText documents for an
// unbreakable word: at a pathologically narrow width, a lone entry can still
// leave its line wider than requested, which is preferable to truncating the
// mapping away or breaking a status/type name mid-character.
//
// plain and styled must be the same length and index-aligned: plain decides
// where the line breaks fall, styled supplies what actually gets rendered at
// those same positions. Deciding on plain and rendering from styled — rather
// than measuring styled directly — is what keeps the wrap correct: an ANSI
// colour escape counts as printable width to runewidth, so measuring a
// styled string reports it wider than it renders and would wrap too early
// (or, once no tty is attached and lipgloss emits no colour at all, hide the
// question entirely).
func wrapLegendLine(label string, plain, styled []string, width int) []string {
	var lines []string
	linePlainWidth := 0
	lineStyled := ""
	entriesOnLine := 0

	flush := func() {
		if len(lines) == 0 {
			lines = append(lines, Muted.Render(PadRight(label, legendLead))+lineStyled)
		} else {
			lines = append(lines, strings.Repeat(" ", legendLead)+lineStyled)
		}
	}

	for i, p := range plain {
		entryWidth := DisplayWidth(p)
		grown := legendLead + linePlainWidth
		if entriesOnLine > 0 {
			grown += DisplayWidth(legendSep) + entryWidth
		} else {
			grown += entryWidth
		}
		if entriesOnLine > 0 && grown > width {
			flush()
			linePlainWidth = 0
			lineStyled = ""
			entriesOnLine = 0
		}
		if entriesOnLine > 0 {
			lineStyled += Muted.Render(legendSep)
			linePlainWidth += DisplayWidth(legendSep)
		}
		lineStyled += styled[i]
		linePlainWidth += entryWidth
		entriesOnLine++
	}
	if entriesOnLine > 0 {
		flush()
	}
	return lines
}
