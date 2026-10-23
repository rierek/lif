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
// Columns are measured over every node the tree could show, open or not, so
// opening or closing a node never moves a column (lif roadmap measures only
// what it prints).
func (m model) roadmapBlocks(nodes []treeNode, width int) [][]string {
	last := lastSiblings(nodes)
	flat, itemAt := roadmapFlat(nodes, last)
	all, _ := roadmapFlat(m.nodesFor(m.allExpanded()), nil)

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
	itemBlocks := ui.TreeBlocksMeasured(rows, ui.RowsFromFlatItems(all), width, false, data.ActiveConfig())

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

// allExpanded is an expand state with every node open.
func (m model) allExpanded() map[string]bool {
	open := map[string]bool{unscheduledRootID: true, notOnRoadmapRootID: true}
	if m.idx != nil {
		for id := range m.idx.ByID {
			open[id] = true
		}
	}
	return open
}

// roadmapFlat maps the item nodes to the row model's input, and returns the
// index into nodes of each. last may be nil when only depths matter.
func roadmapFlat(nodes []treeNode, last []bool) (flat []ui.FlatItem, itemAt []int) {
	shift := 0 // items under a section root sit one level up in the CLI, where the section is a heading
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
		flat = append(flat, ui.FlatItem{Item: n.bean, Depth: n.depth - shift, IsLast: last != nil && last[i]})
		itemAt = append(itemAt, i)
	}
	return flat, itemAt
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
