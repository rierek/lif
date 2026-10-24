package waves

import "testing"

// hopsOfEdge returns the hops of the drawn edge from -> to.
func hopsOfEdge(t *testing.T, b *Board, from, to string) []hop {
	t.Helper()
	for i, e := range b.edges {
		if e.From == from && e.To == to && b.routes[i] != nil {
			return b.routes[i].hops
		}
	}
	t.Fatalf("no drawn edge %s->%s", from, to)
	return nil
}

func firstLane(t *testing.T, b *Board, from, to string) int {
	t.Helper()
	return hopsOfEdge(t, b, from, to)[0].lane
}

// assertNoEdgeInsideCards checks that no cell of any drawn edge, arrowhead
// included, lies in a card rectangle.
func assertNoEdgeInsideCards(t *testing.T, b *Board) {
	t.Helper()
	inside := func(x, y int) string {
		for _, c := range b.byRank {
			cx, cy := b.colX[c.col], y0(c.row)
			if x >= cx && x < cx+cardW && y >= cy && y < cy+cardH {
				return c.id
			}
		}
		return ""
	}
	for i := range b.edges {
		if !b.drawn(i) {
			continue
		}
		for _, s := range b.segments(i) {
			for x := min(s.x1, s.x2); x <= max(s.x1, s.x2); x++ {
				for y := min(s.y1, s.y2); y <= max(s.y1, s.y2); y++ {
					if id := inside(x, y); id != "" {
						t.Errorf("edge %s->%s runs through card %s at %d,%d", b.edges[i].From, b.edges[i].To, id, x, y)
					}
				}
			}
		}
		if x, y := b.arrow(i); inside(x, y) != "" {
			t.Errorf("arrowhead of %s->%s lies inside a card", b.edges[i].From, b.edges[i].To)
		}
	}
}

func TestFanSharesOneLane(t *testing.T) {
	b := board(mk("p", "root"), mk("x", "root", "p"), mk("y", "root", "p"), mk("z", "root", "p"))
	if got := firstLane(t, b, "p", "x"); got != -1 {
		t.Errorf("straight edge p->x has lane %d, want -1", got)
	}
	ly, lz := firstLane(t, b, "p", "y"), firstLane(t, b, "p", "z")
	if ly < 0 || ly != lz {
		t.Errorf("lanes of p->y and p->z = %d,%d, want one shared lane", ly, lz)
	}
	if n := len(b.gaps[0]); n != 1 {
		t.Errorf("gap 0 has %d lanes, want 1", n)
	}
	assertNoEdgeInsideCards(t, b)
}

func TestCollectorSharesOneLane(t *testing.T) {
	// p, q, r in column 0 (rows 0, 1, 2) all feed t in row 0: q and r enter from below.
	b := board(mk("p", "root"), mk("q", "root"), mk("r", "root"), mk("t", "root", "p", "q", "r"))
	if got := firstLane(t, b, "p", "t"); got != -1 {
		t.Errorf("straight edge p->t has lane %d, want -1", got)
	}
	lq, lr := firstLane(t, b, "q", "t"), firstLane(t, b, "r", "t")
	if lq < 0 || lq != lr {
		t.Errorf("lanes of q->t and r->t = %d,%d, want one collector lane", lq, lr)
	}
	if n := len(b.gaps[0]); n != 1 {
		t.Errorf("gap 0 has %d lanes, want 1", n)
	}
	assertNoEdgeInsideCards(t, b)
}

// crossingFixture: s1 (row 0) feeds fb and ta (rows 1 and 2), s2 (row 1) feeds
// tb (row 3). Sorted by start row the two lanes would cross twice; swapped they
// do not cross.
func crossingFixture() *Board {
	return board(
		mk("s1", "root"), mk("s2", "root"),
		mk("fa", "root", "s1"), mk("fb", "root", "s1"), mk("ta", "root", "s1"), mk("tb", "root", "s2"),
	)
}

func TestLaneOrderMinimisesCrossings(t *testing.T) {
	b := crossingFixture()
	if col, row := pos(t, b, "tb"); col != 1 || row != 3 {
		t.Fatalf("fixture drifted: tb at %d,%d, want 1,3", col, row)
	}
	g := b.gaps[0]
	if len(g) != 2 {
		t.Fatalf("gap 0 has %d lane groups, want 2", len(g))
	}
	if crossing(g[1], g[0]) == 0 {
		t.Fatal("fixture is no test: the start-row order would not cross")
	}
	if n := b.crossings(); n != 0 {
		t.Errorf("crossings = %d, want 0", n)
	}
	if lf, lo := firstLane(t, b, "s1", "fb"), firstLane(t, b, "s2", "tb"); lf == lo {
		t.Errorf("edges of different sources share lane %d", lf)
	}
	assertNoEdgeInsideCards(t, b)
	// The painted canvas has no crossing cell either.
	cv := b.paint(RenderOpts{})
	for y := range cv.cells {
		for x := range cv.cells[y] {
			if cv.cells[y][x].crossing() {
				t.Errorf("crossing cell at %d,%d", x, y)
			}
		}
	}
}

func TestEntryRowByDirection(t *testing.T) {
	// q: straight from p (row 0) and from below from s (row 1); r sits below p: from above.
	b := board(mk("p", "root"), mk("s", "root"), mk("q", "root", "p", "s"), mk("r", "root", "p"))
	entry := func(from, to string) int {
		h := hopsOfEdge(t, b, from, to)
		_, row := pos(t, b, to)
		return h[len(h)-1].y2 - y0(row)
	}
	for _, c := range []struct {
		from, to string
		inner    int
	}{{"p", "q", 2}, {"s", "q", 3}, {"p", "r", 1}} {
		if got := entry(c.from, c.to); got != c.inner {
			t.Errorf("%s->%s enters in inner row %d, want %d", c.from, c.to, got, c.inner)
		}
	}
	assertNoEdgeInsideCards(t, b)
}

func TestLongEdgesAvoidCards(t *testing.T) {
	free := board(mk("p", "root"), mk("x", "root"), mk("q", "root", "p"), mk("t", "root", "q", "x"))
	below := board(mk("p", "root"), mk("x", "root"), mk("q", "root", "p"), mk("r", "root", "p"), mk("t", "root", "q", "x"))
	for name, b := range map[string]*Board{"free row": free, "below the cards": below} {
		t.Run(name, func(t *testing.T) {
			assertNoEdgeInsideCards(t, b)
			if len(hopsOfEdge(t, b, "x", "t")) != 2 {
				t.Error("long edge should consist of two hops")
			}
		})
	}
	// Below the cards: the pass-through line runs under the last card of column 1.
	for i, e := range below.edges {
		if e.From == "x" && e.To == "t" {
			v := below.routes[i].vias
			if len(v) != 1 || v[0].y != midY(2) {
				t.Errorf("pass-through = %+v, want one at row y=%d", v, midY(2))
			}
		}
	}
}
