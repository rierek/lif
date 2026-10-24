package waves

import (
	"sort"
	"strconv"
)

// Routing breaks every drawn edge into hops, one per gap between two
// columns. A hop leaves its station (the source card or a pass-through) at
// row y1 and reaches the next station at row y2. Hops with y1 != y2 need a
// vertical lane in the gap; hops with y1 == y2 run straight.
//
// Lanes follow three rules: an own lane for every edge, one lane shared by the
// edges that leave the same station, and one collector lane for edges that
// enter the same target card from the same side. The lanes of a gap are
// ordered so that few lines cross.

// hop is one jump over the gap between column gap and gap+1.
type hop struct {
	gap    int
	y1, y2 int // canvas rows at the left and at the right end of the hop
	lane   int // -1 for a straight hop
}

// via is a pass-through: a horizontal line through a column without a card.
type via struct{ col, y int }

type route struct {
	hops []hop
	vias []via
}

// laneGroup is the set of hops that share one lane.
type laneGroup struct {
	key        string
	y1, y2     int   // vertical span of the lane
	srcs, dsts []int // rows where hops join the lane from the left and leave it to the right
	hops       []*hop
	lane       int
}

// segment is an axis-aligned line, ends included.
type segment struct{ x1, y1, x2, y2 int }

func (s segment) vertical() bool { return s.x1 == s.x2 && s.y1 != s.y2 }

// route builds the hops of every drawn edge, assigns the lanes of every gap
// and fixes the left cell of every column.
func (b *Board) route() {
	b.routes = make([]*route, len(b.edges))
	byGap := map[int][]hopRef{}
	for i := range b.edges {
		if !b.drawn(i) {
			continue
		}
		r := b.hopsOf(i)
		b.routes[i] = r
		for k := range r.hops {
			h := &r.hops[k]
			byGap[h.gap] = append(byGap[h.gap], hopRef{i, h, k == len(r.hops)-1})
		}
	}
	gaps := max(len(b.Columns)-1, 0)
	b.gaps = make([][]*laneGroup, gaps)
	b.colX = make([]int, len(b.Columns))
	for g := range gaps {
		b.gaps[g] = b.assignLanes(byGap[g])
		b.colX[g+1] = b.colX[g] + cardW + gapWidth(b.gaps[g])
	}
}

// gapWidth is the width of a gap in cells: room for the lanes, a free cell
// before the arrowhead, and at least 6 so edges always have space to turn.
func gapWidth(groups []*laneGroup) int {
	lanes := 0
	for _, g := range groups {
		lanes = max(lanes, g.lane+1)
	}
	return max(6, 2*lanes+4)
}

// hopRef points to one hop of edge i.
type hopRef struct {
	edge  int
	hop   *hop
	final bool // the hop that enters the target card
}

// hopsOf builds the stations and hops of drawn edge i. A hop that enters the
// target card ends in inner row 1 when it comes from above, inner row 3 when
// it comes from below and inner row 2 when it runs straight in.
func (b *Board) hopsOf(i int) *route {
	e := b.edges[i]
	from, to := b.cards[e.From], b.cards[e.To]
	ys := []int{midY(from.row)}
	for _, r := range b.via[i] {
		ys = append(ys, midY(r))
	}
	target := midY(to.row)
	entry := target
	switch last := ys[len(ys)-1]; {
	case last < target:
		entry = target - 1
	case last > target:
		entry = target + 1
	}
	r := &route{}
	for k, y := range ys {
		y2 := entry
		if k+1 < len(ys) {
			y2 = ys[k+1]
		}
		r.hops = append(r.hops, hop{gap: from.col + k, y1: y, y2: y2, lane: -1})
		if k > 0 {
			r.vias = append(r.vias, via{col: from.col + k, y: y})
		}
	}
	return r
}

// assignLanes groups the laned hops of one gap, orders the groups and gives
// them lane numbers. It returns the groups in lane order.
func (b *Board) assignLanes(refs []hopRef) []*laneGroup {
	type station struct {
		from string
		y    int
	}
	straight := map[station]bool{}
	for _, ref := range refs {
		if ref.hop.y1 == ref.hop.y2 {
			straight[station{b.edges[ref.edge].From, ref.hop.y1}] = true
		}
	}
	// A collector needs two or more hops from the same side into one target.
	collect := map[string]int{}
	collectKey := func(ref hopRef) string {
		return "t" + b.edges[ref.edge].To + "@" + strconv.Itoa(ref.hop.y2)
	}
	for _, ref := range refs {
		if ref.final && ref.hop.y1 != ref.hop.y2 {
			collect[collectKey(ref)]++
		}
	}

	groups := map[string]*laneGroup{}
	var list []*laneGroup
	for _, ref := range refs {
		h := ref.hop
		if h.y1 == h.y2 {
			continue
		}
		key := collectKey(ref)
		if !ref.final || collect[key] < 2 {
			from := b.edges[ref.edge].From
			key = "s" + from + "@" + strconv.Itoa(h.y1)
			if straight[station{from, h.y1}] { // a 4-way junction would look like a crossing
				dir := "d"
				if h.y2 < h.y1 {
					dir = "u"
				}
				key += dir
			}
		}
		g := groups[key]
		if g == nil {
			g = &laneGroup{key: key, y1: min(h.y1, h.y2), y2: max(h.y1, h.y2)}
			groups[key] = g
			list = append(list, g)
		}
		g.y1, g.y2 = min(g.y1, h.y1, h.y2), max(g.y2, h.y1, h.y2)
		g.srcs, g.dsts = append(g.srcs, h.y1), append(g.dsts, h.y2)
		g.hops = append(g.hops, h)
	}

	sort.Slice(list, func(i, j int) bool {
		a, c := list[i], list[j]
		if a.y1 != c.y1 {
			return a.y1 < c.y1
		}
		if a.y2 != c.y2 {
			return a.y2 < c.y2
		}
		return a.key < c.key
	})
	// ponytail: adjacent swaps reach a local minimum of crossings, not the
	// global one; upgrade to a permutation search if full levels need it.
	for changed := true; changed; {
		changed = false
		for i := 0; i+1 < len(list); i++ {
			if crossing(list[i+1], list[i]) < crossing(list[i], list[i+1]) {
				list[i], list[i+1], changed = list[i+1], list[i], true
			}
		}
	}
	// A lane is one right of every earlier group it overlaps, so the order stays.
	for i, g := range list {
		for _, o := range list[:i] {
			if o.y1 <= g.y2 && g.y1 <= o.y2 {
				g.lane = max(g.lane, o.lane+1)
			}
		}
		for _, h := range g.hops {
			h.lane = g.lane
		}
	}
	return list
}

// crossing counts the crossings if lane group a runs left of lane group b: the
// hops that leave a to the right cross b's vertical, and the hops that join b
// from the left cross a's vertical. Meeting a lane at its end is a junction,
// not a crossing.
func crossing(a, b *laneGroup) int {
	n := 0
	for _, y := range a.dsts {
		if b.y1 < y && y < b.y2 {
			n++
		}
	}
	for _, y := range b.srcs {
		if a.y1 < y && y < a.y2 {
			n++
		}
	}
	return n
}

// laneX is the cell of lane l in gap g.
func (b *Board) laneX(g, l int) int { return b.colX[g] + cardW + 2 + 2*l }

// segments lists the lines of drawn edge i: per hop the way out of the
// station, the vertical lane and the way into the next station, and the
// horizontal line through every pass-through column.
func (b *Board) segments(i int) []segment {
	r := b.routes[i]
	var out []segment
	for _, h := range r.hops {
		sx, tx := b.colX[h.gap]+cardW, b.colX[h.gap+1]-1
		if h.lane < 0 {
			out = append(out, segment{sx, h.y1, tx, h.y1})
			continue
		}
		lx := b.laneX(h.gap, h.lane)
		out = append(out,
			segment{sx, h.y1, lx, h.y1},
			segment{lx, h.y1, lx, h.y2},
			segment{lx, h.y2, tx, h.y2})
	}
	for _, v := range r.vias {
		out = append(out, segment{b.colX[v.col] - 1, v.y, b.colX[v.col] + cardW, v.y})
	}
	return out
}

// arrow is the cell of the arrowhead of drawn edge i, directly in front of
// the target card.
func (b *Board) arrow(i int) (x, y int) {
	last := b.routes[i].hops[len(b.routes[i].hops)-1]
	return b.colX[last.gap+1] - 1, last.y2
}

// height is the number of canvas rows: the lowest card row or, when a
// pass-through runs below the cards, the row under it.
func (b *Board) height() int {
	h := 0
	for _, c := range b.byRank {
		h = max(h, y0(c.row)+cardH)
	}
	for i := range b.edges {
		if b.drawn(i) {
			for _, s := range b.segments(i) {
				h = max(h, s.y1+1, s.y2+1)
			}
		}
	}
	return h
}
