package tui

// view_browse_roadmap.go — the roadmap list rendered by the same row builder
// step (ui.RowsFromFlatItems) and row renderer (ui.TreeBlocks) as lif roadmap.
// Beside the detail pane the list keeps the compact one-line rows
// (treeRowLines).

import (
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/ui"
)

// roadmapBlocks renders every visible node as its block of lines, index-aligned
// with nodes. Item nodes go through the CLI renderer at width; the synthetic
// section and placeholder rows have no item and keep their own one-line form.
// Columns are measured over the visible rows, so collapsing a node can narrow
// a column -- the CLI does the same for a scoped roadmap.
func (m model) roadmapBlocks(nodes []treeNode, width int) [][]string {
	last := lastSiblings(nodes)
	var flat []ui.FlatItem
	var itemAt []int // index into nodes of each flat item
	shift := 0       // items under a section root sit one level up in the CLI, where the section is a heading
	for i, n := range nodes {
		switch {
		case n.section != "":
			shift = 1
		case n.depth == 0:
			shift = 0
		}
		if n.bean == nil || n.placeholder {
			continue
		}
		flat = append(flat, ui.FlatItem{Item: n.bean, Depth: n.depth - shift, IsLast: last[i]})
		itemAt = append(itemAt, i)
	}

	rows := ui.RowsFromFlatItems(flat)
	for r, i := range itemAt {
		switch n := nodes[i]; {
		case !n.hasKids:
		case n.open:
			rows[r].Marker = "▾"
		default:
			rows[r].Marker = "▸"
		}
	}
	itemBlocks := ui.TreeBlocks(rows, width, false, data.ActiveConfig())

	slug := m.beanIDPrefix()
	blocks := make([][]string, len(nodes))
	r := 0
	for i, n := range nodes {
		if r < len(itemAt) && itemAt[r] == i {
			blocks[i] = itemBlocks[r]
			r++
			continue
		}
		blocks[i] = treeRowLines(n, slug, width)
	}
	return blocks
}

// lastSiblings reports, per node, whether no later node shares its parent --
// the IsLast of the row model, which picks └─ over ├─.
func lastSiblings(nodes []treeNode) []bool {
	last := make([]bool, len(nodes))
	var seen []bool // seen[d]: a later node at depth d under the current parent
	for i := len(nodes) - 1; i >= 0; i-- {
		d := nodes[i].depth
		for len(seen) <= d {
			seen = append(seen, false)
		}
		last[i] = !seen[d]
		seen[d] = true
		for j := d + 1; j < len(seen); j++ {
			seen[j] = false
		}
	}
	return last
}
