package waves

import (
	"cmp"
	"slices"
	"sort"

	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

// cmpOrder sorts cards by their order key. Items without an order key come
// after all items with one; the ID breaks ties so the order is total.
func cmpOrder(a, b *item.Item) int {
	return cmp.Or(
		cmp.Compare(min(len(b.Order), 1), min(len(a.Order), 1)), // keyed first
		cmp.Compare(a.Order, b.Order),
		cmp.Compare(a.ID, b.ID),
	)
}

// collectCards makes the direct children of the root that are not scrapped
// the cards of the board, in order sort. Status never decides placement.
func (b *Board) collectCards(idx *data.Index, rootID string) {
	var kids []*item.Item
	for _, it := range idx.Children[rootID] {
		if it.Status != "scrapped" {
			kids = append(kids, it)
		}
	}
	slices.SortFunc(kids, cmpOrder)
	for i, it := range kids {
		c := &card{id: it.ID, it: it, rank: i}
		b.cards[it.ID] = c
		b.byRank = append(b.byRank, c)
	}
}

type pair struct{ from, to string }

// owner finds the card an item belongs to: the item itself or its ancestor
// directly below the root. inside is true for every item of the root's
// subtree, also for scrapped children and their descendants (no card) and the
// root itself.
func (b *Board) owner(idx *data.Index, rootID, id string) (cardID string, inside bool) {
	for n := 0; n <= len(idx.ByID); n++ {
		it, ok := idx.ByID[id]
		if !ok {
			return "", false
		}
		if id == rootID {
			return "", true
		}
		if it.Parent == rootID {
			if _, ok := b.cards[id]; ok {
				return id, true
			}
			return "", true
		}
		id = it.Parent
	}
	return "", false // parent cycle
}

// collectEdges derives the card-level edges. An item is blocked by X when its
// own blocked_by lists X or when X lists it in blocking; each pair counts
// once. Edges between descendants of two cards are lifted to the cards. Edges
// to items outside the root only count as hints on the card.
func (b *Board) collectEdges(idx *data.Index, rootID string) {
	ids := make([]string, 0, len(idx.ByID))
	for id := range idx.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	seen := map[pair]bool{}
	var pairs []pair
	add := func(from, to string) {
		p := pair{from, to}
		_, okFrom := idx.ByID[from]
		_, okTo := idx.ByID[to]
		if from == to || !okFrom || !okTo || seen[p] {
			return
		}
		seen[p] = true
		pairs = append(pairs, p)
	}
	for _, id := range ids {
		it := idx.ByID[id]
		for _, blocker := range it.BlockedBy {
			add(blocker, id)
		}
		for _, waiting := range it.Blocking {
			add(id, waiting)
		}
	}

	at := map[pair]int{}
	extIn := map[string]map[string]bool{}
	extOut := map[string]map[string]bool{}
	note := func(m map[string]map[string]bool, cardID, other string) {
		if m[cardID] == nil {
			m[cardID] = map[string]bool{}
		}
		m[cardID][other] = true
	}
	for _, p := range pairs {
		from, fromIn := b.owner(idx, rootID, p.from)
		to, toIn := b.owner(idx, rootID, p.to)
		switch {
		case from != "" && to != "" && from != to:
			k := pair{from, to}
			i, ok := at[k]
			if !ok {
				i = len(b.edges)
				at[k] = i
				b.edges = append(b.edges, Edge{From: from, To: to, Lifted: true})
			}
			if p.from == from && p.to == to {
				b.edges[i].Lifted = false
			}
		case from != "" && !toIn:
			note(extOut, from, p.to)
		case to != "" && !fromIn:
			note(extIn, to, p.from)
		}
	}
	for id, c := range b.cards {
		c.extIn, c.extOut = len(extIn[id]), len(extOut[id])
	}
	sort.Slice(b.edges, func(i, j int) bool {
		ei, ej := b.edges[i], b.edges[j]
		if fi, fj := b.cards[ei.From].rank, b.cards[ej.From].rank; fi != fj {
			return fi < fj
		}
		return b.cards[ei.To].rank < b.cards[ej.To].rank
	})
}

// hideRedundant hides every edge the reduction helper reports and counts it
// for both of its cards. Columns never change: a redundant edge adds no
// longer path.
func (b *Board) hideRedundant() {
	list := make([]itemgraph.Edge, len(b.edges))
	for i, e := range b.edges {
		list[i] = itemgraph.Edge{From: e.From, To: e.To}
	}
	red := map[itemgraph.Edge]bool{}
	for _, r := range itemgraph.RedundantEdges(list) {
		red[r.Edge] = true
		b.redundant[r.From]++
		b.redundant[r.To]++
	}
	for i, e := range b.edges {
		b.edges[i].Hidden = red[itemgraph.Edge{From: e.From, To: e.To}]
	}
}

// placeColumns sets the column of every card and builds Columns. A card with
// predecessors stands one column right of its rightmost predecessor. A card
// without edges stands in the column of its order predecessor, or in the loose
// column when it has no order either. Inside a card-level cycle the column
// computation cuts the edge that closes the cycle, so every card still gets a
// column; the cut edges, and any edge that does not run left to right, become
// hidden.
func (b *Board) placeColumns() {
	preds := map[string][]string{}
	linked := map[string]bool{}
	for _, e := range b.edges {
		preds[e.To] = append(preds[e.To], e.From)
		linked[e.From], linked[e.To] = true, true
	}

	col := map[string]int{}
	state := map[string]int{} // 1 visiting, 2 done
	var colOf func(id string) int
	colOf = func(id string) int {
		switch state[id] {
		case 2:
			return col[id]
		case 1:
			return -1 // closes a cycle: contributes nothing
		}
		state[id] = 1
		c := 0
		for _, p := range preds[id] {
			c = max(c, colOf(p)+1)
		}
		state[id], col[id] = 2, c
		return c
	}

	prev := -1 // column of the previous card that has an order key
	var loose []*card
	for _, c := range b.byRank {
		switch {
		case !linked[c.id] && c.it.Order == "":
			loose = append(loose, c)
			continue
		case !linked[c.id] && prev >= 0:
			c.col = prev
		default:
			c.col = colOf(c.id)
		}
		if c.it.Order != "" {
			prev = c.col
		}
	}
	if len(loose) > 0 {
		b.Loose = true
		for _, c := range b.byRank {
			c.col++
		}
		for _, c := range loose {
			c.col = 0
		}
	}

	for i, e := range b.edges {
		if b.cards[e.To].col <= b.cards[e.From].col {
			b.edges[i].Hidden = true
		}
	}
}

// placeRows sets the stair row of every card, column by column from the left:
// the row of its first drawn predecessor in order, else the next free row; a
// taken row moves the card to the next free row below it.
func (b *Board) placeRows() {
	first := map[string]*card{} // first drawn predecessor by order
	for i, e := range b.edges {
		if b.drawn(i) && first[e.To] == nil {
			first[e.To] = b.cards[e.From]
		}
	}
	want := func(c *card) int {
		if p := first[c.id]; p != nil {
			return p.row
		}
		return 0
	}

	cols := make([][]*card, 0)
	for _, c := range b.byRank {
		for len(cols) <= c.col {
			cols = append(cols, nil)
		}
		cols[c.col] = append(cols[c.col], c)
	}
	b.Columns = make([][]string, len(cols))
	for ci, cs := range cols {
		wants := map[string]int{}
		for _, c := range cs {
			wants[c.id] = want(c)
		}
		sort.SliceStable(cs, func(i, j int) bool { return wants[cs[i].id] < wants[cs[j].id] })
		used := map[int]bool{}
		for _, c := range cs {
			r := wants[c.id]
			for used[r] {
				r++
			}
			used[r], c.row = true, r
		}
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].row < cs[j].row })
		for _, c := range cs {
			b.Columns[ci] = append(b.Columns[ci], c.id)
		}
	}
}

// findCritical marks the critical path: the longest chain of drawn edges,
// counting only cards that are not completed (remaining effort). A chain
// starts and ends at a card that is not completed and has at least one edge.
// Every chain of the maximal length is marked.
func (b *Board) findCritical() {
	b.critEdge = make([]bool, len(b.edges))
	preds := map[string][]string{}
	succs := map[string][]string{}
	for i, e := range b.edges {
		if b.drawn(i) {
			preds[e.To] = append(preds[e.To], e.From)
			succs[e.From] = append(succs[e.From], e.To)
		}
	}
	// chain[id]: best weight of a chain from a card that is not completed up
	// to id along next, visiting the cards in dependency order; -1 means
	// there is none.
	chain := func(order []*card, next map[string][]string) map[string]int {
		best := map[string]int{}
		for _, c := range order {
			w := 1
			if c.it.Status == "completed" {
				w = 0
			}
			top := -1
			if w == 1 {
				top = 1
			}
			for _, n := range next[c.id] {
				if best[n] >= 0 {
					top = max(top, best[n]+w)
				}
			}
			best[c.id] = top
		}
		return best
	}
	// Drawn edges run to a higher column, so column order is topological.
	order := append([]*card(nil), b.byRank...)
	sort.SliceStable(order, func(i, j int) bool { return order[i].col < order[j].col })
	ending := chain(order, preds)
	slices.Reverse(order)
	starting := chain(order, succs)

	longest := 0
	for i, e := range b.edges {
		if b.drawn(i) && ending[e.From] >= 0 && starting[e.To] >= 0 {
			longest = max(longest, ending[e.From]+starting[e.To])
		}
	}
	for i, e := range b.edges {
		if b.drawn(i) && ending[e.From] >= 0 && starting[e.To] >= 0 && ending[e.From]+starting[e.To] == longest {
			b.critEdge[i] = true
			b.critCard[e.From], b.critCard[e.To] = true, true
		}
	}
}

// assignVias gives every drawn edge that spans more than one gap a
// pass-through row in each crossed column. The row is the height of the
// previous station when that row is free in the column, else a row below the
// cards of the column. Edges of the same source at the same height share a
// pass-through.
func (b *Board) assignVias() {
	b.via = make([][]int, len(b.edges))
	cardRows := make([]map[int]bool, len(b.Columns))
	lastCard := make([]int, len(b.Columns))
	for i := range cardRows {
		cardRows[i] = map[int]bool{}
	}
	for _, c := range b.byRank {
		cardRows[c.col][c.row] = true
		lastCard[c.col] = max(lastCard[c.col], c.row)
	}

	type key struct {
		from      string
		col, prev int
	}
	shared := map[key]int{}
	for i, e := range b.edges {
		if !b.drawn(i) {
			continue
		}
		from, to := b.cards[e.From], b.cards[e.To]
		prev := from.row
		for col := from.col + 1; col < to.col; col++ {
			k := key{e.From, col, prev}
			r, ok := shared[k]
			if !ok {
				r = prev
				if cardRows[col][r] {
					r = lastCard[col] + 1
					for cardRows[col][r] {
						r++
					}
				}
				cardRows[col][r] = true
				shared[k] = r
			}
			b.via[i] = append(b.via[i], r)
			prev = r
		}
	}
}
