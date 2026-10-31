// Package waves lays out, routes and renders the Waves board: the direct
// children of one root item as fixed-size cards in columns that follow the
// blocked_by edges between them. The package is pure: it reads only the
// in-memory data.Index it is given and returns text, so the same input always
// yields byte-identical output.
//
// Three stages build on each other. Layout (layout.go) derives cards, edges,
// columns, rows, pass-through rows and the critical path. Routing (route.go)
// gives every drawn edge its lanes. Render (render.go) paints a Board for one
// view state (focus, mark, dimmed cards, viewport).
package waves

import (
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
)

// Card geometry. Cards are cardH rows high and cardW cells wide regardless of
// the terminal; consecutive card rows are pitch rows apart.
const (
	cardW = 26
	cardH = 5
	pitch = cardH + 1
)

// Edge is one card-level edge: To waits for From.
type Edge struct {
	From, To string
	// Lifted is true when the edge was lifted from items below the cards.
	Lifted bool
	// Hidden is true when the edge is not drawn: it is redundant (its target
	// already waits through other edges) or, inside a card-level cycle, it
	// runs against the column direction.
	Hidden bool
}

// card is one visible direct child of the root.
type card struct {
	id    string
	it    *item.Item
	rank  int // position in order sort, the tie-breaker everywhere
	col   int // display column
	row   int // stair row
	extIn int // distinct blockers outside the root
	// extOut counts distinct items outside the root that wait on the card.
	extOut int
}

// Board is the computed Waves board. It does not depend on focus, mark or
// viewport: Render takes those as input.
type Board struct {
	// Columns lists the card IDs of every display column from left to right,
	// each in row order. The loose column comes first when Loose is set.
	Columns [][]string
	// Loose reports whether Columns[0] is the loose column.
	Loose bool

	cards     map[string]*card
	byRank    []*card
	edges     []Edge // sorted by rank of From, then of To
	redundant map[string]int

	critEdge []bool // aligned with edges
	critCard map[string]bool

	via    [][]int // aligned with edges: row of the pass-through in every crossed column
	routes []*route
	colX   []int // left cell of every display column
	gaps   [][]*laneGroup
}

// Build lays out and routes the board of rootID's direct children. An unknown
// root yields an empty board.
func Build(idx *data.Index, rootID string) *Board {
	b := &Board{cards: map[string]*card{}, redundant: map[string]int{}, critCard: map[string]bool{}}
	if idx == nil {
		return b
	}
	b.collectCards(idx, rootID)
	b.collectEdges(idx, rootID)
	b.hideRedundant()
	b.placeColumns()
	b.placeRows()
	b.findCritical()
	b.assignVias()
	b.route()
	return b
}

// Pos returns the display column and the stair row of a card. ok is false for
// IDs that are not on the board.
func (b *Board) Pos(id string) (col, row int, ok bool) {
	c, ok := b.cards[id]
	if !ok {
		return 0, 0, false
	}
	return c.col, c.row, true
}

// Redundant returns the number of hidden redundant card-level edges that have
// the card as source or target.
func (b *Board) Redundant(id string) int { return b.redundant[id] }

// Edges returns every card-level edge, drawn and hidden.
func (b *Board) Edges() []Edge { return append([]Edge(nil), b.edges...) }

// drawn reports whether edge i is drawn.
func (b *Board) drawn(i int) bool { return !b.edges[i].Hidden }

// y0 is the top canvas row of card row r.
func y0(r int) int { return r * pitch }

// midY is the canvas row where edges leave a card in card row r (inner row 2).
func midY(r int) int { return y0(r) + 2 }
