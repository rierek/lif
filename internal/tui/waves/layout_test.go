package waves

import (
	"reflect"
	"testing"

	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
)

// mk makes a todo item of type feature under parent. Its title is "Title <id>".
func mk(id, parent string, blockedBy ...string) *item.Item {
	return &item.Item{ID: id, Parent: parent, Status: "todo", Type: "feature", Title: "Title " + id, BlockedBy: blockedBy}
}

// board builds the board of root "root" over the items plus the root itself.
func board(items ...*item.Item) *Board {
	all := append([]*item.Item{{ID: "root", Status: "todo", Type: "milestone", Title: "Root"}}, items...)
	return Build(data.NewIndex(all), "root")
}

func (b *Board) edge(from, to string) (Edge, bool) {
	for _, e := range b.edges {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return Edge{}, false
}

// criticalEdges lists the edges marked critical as "from>to".
func criticalEdges(b *Board) []string {
	var out []string
	for i, e := range b.edges {
		if b.critEdge[i] {
			out = append(out, e.From+">"+e.To)
		}
	}
	return out
}

func pos(t *testing.T, b *Board, id string) (int, int) {
	t.Helper()
	col, row, ok := b.Pos(id)
	if !ok {
		t.Fatalf("Pos(%q): not on the board", id)
	}
	return col, row
}

func TestColumnsAndStairRows(t *testing.T) {
	// a -> b -> c, and a -> d branching off.
	b := board(mk("a", "root"), mk("b", "root", "a"), mk("c", "root", "b"), mk("d", "root", "a"))
	want := [][]string{{"a"}, {"b", "d"}, {"c"}}
	if !reflect.DeepEqual(b.Columns, want) {
		t.Fatalf("Columns = %v, want %v", b.Columns, want)
	}
	if b.Loose {
		t.Error("Loose = true, want false")
	}
	for id, w := range map[string][2]int{"a": {0, 0}, "b": {1, 0}, "d": {1, 1}, "c": {2, 0}} {
		if col, row := pos(t, b, id); col != w[0] || row != w[1] {
			t.Errorf("Pos(%s) = %d,%d want %d,%d", id, col, row, w[0], w[1])
		}
	}
	if _, _, ok := b.Pos("nope"); ok {
		t.Error("Pos of unknown ID reports ok")
	}
}

func TestStatusNeverDecidesColumn(t *testing.T) {
	a, bb := mk("a", "root"), mk("b", "root", "a")
	a.Status = "completed"
	got := board(a, bb)
	if col, _ := pos(t, got, "b"); col != 1 {
		t.Errorf("column of b = %d, want 1", col)
	}
}

func TestOrderSortsColumnAndPlacesIsolatedCard(t *testing.T) {
	p := mk("p", "root")
	p.Order = "0"
	x, y, z := mk("x", "root", "p"), mk("y", "root", "p"), mk("z", "root", "p")
	x.Order, y.Order, z.Order = "c", "a", "b"
	// no edges, but an order key between y and z: stands in the column of y.
	iso := mk("iso", "root")
	iso.Order = "ab"
	b := board(x, y, z, iso, p)
	want := [][]string{{"p"}, {"y", "iso", "z", "x"}}
	if !reflect.DeepEqual(b.Columns, want) {
		t.Fatalf("Columns = %v, want %v", b.Columns, want)
	}
}

func TestScrappedChildIsNotACard(t *testing.T) {
	s := mk("s", "root")
	s.Status = "scrapped"
	b := board(mk("a", "root"), mk("b", "root", "a"), s, mk("c", "root", "s"))
	if _, _, ok := b.Pos("s"); ok {
		t.Error("scrapped child is on the board")
	}
	// c only waited on the scrapped card: no edge, no hint, loose column.
	want := [][]string{{"c"}, {"a"}, {"b"}}
	if !reflect.DeepEqual(b.Columns, want) || !b.Loose {
		t.Fatalf("Columns = %v loose=%v, want %v loose", b.Columns, b.Loose, want)
	}
	if got := b.cards["c"]; got.extIn != 0 || got.extOut != 0 {
		t.Errorf("hints on c = %d/%d, want none", got.extIn, got.extOut)
	}
}

func TestEdgeFromBlockingAndBlockedByCountsOnce(t *testing.T) {
	a, bb := mk("a", "root"), mk("b", "root", "a")
	a.Blocking = []string{"b"}
	c := mk("c", "root")
	c.Blocking = []string{"a"} // only the blocking side: c -> a
	b := board(a, bb, c)
	if n := len(b.Edges()); n != 2 {
		t.Fatalf("edges = %v, want 2", b.Edges())
	}
	if _, ok := b.edge("a", "b"); !ok {
		t.Error("edge a->b missing")
	}
	if _, ok := b.edge("c", "a"); !ok {
		t.Error("edge c->a (from blocking) missing")
	}
	if col, _ := pos(t, b, "b"); col != 2 {
		t.Errorf("column of b = %d, want 2", col)
	}
}

func TestLiftedEdgeAndExternalHints(t *testing.T) {
	a, bb := mk("A", "root"), mk("B", "root")
	a1 := mk("a1", "A")
	b1 := mk("b1", "B", "a1", "ext1") // lifted edge A->B, ext1 is outside the root
	ext1, ext2 := mk("ext1", ""), mk("ext2", "", "a1")
	b := board(a, bb, a1, b1, ext1, ext2)

	e, ok := b.edge("A", "B")
	if !ok || !e.Lifted || e.Hidden {
		t.Fatalf("edge A->B = %+v ok=%v, want lifted and drawn", e, ok)
	}
	if colA, _ := pos(t, b, "A"); colA != 0 {
		t.Errorf("column of A = %d", colA)
	}
	if colB, _ := pos(t, b, "B"); colB != 1 {
		t.Errorf("column of B = %d, want 1", colB)
	}
	if got := b.cards["B"].extIn; got != 1 {
		t.Errorf("B waits on %d outside blockers, want 1", got)
	}
	if got := b.cards["A"].extOut; got != 1 {
		t.Errorf("A blocks %d outside items, want 1", got)
	}
	if len(b.Edges()) != 1 {
		t.Errorf("edges = %v, want only the lifted one", b.Edges())
	}
}

func TestDirectEdgeBeatsLiftedForSamePair(t *testing.T) {
	a, bb := mk("A", "root"), mk("B", "root", "A")
	a1, b1 := mk("a1", "A"), mk("b1", "B", "a1")
	b := board(a, bb, a1, b1)
	if e, _ := b.edge("A", "B"); e.Lifted {
		t.Error("edge with a direct item pair is marked lifted")
	}
}

func TestRedundantEdgeHiddenColumnsUnchanged(t *testing.T) {
	b := board(mk("a", "root"), mk("b", "root", "a"), mk("c", "root", "b", "a"))
	if e, ok := b.edge("a", "c"); !ok || !e.Hidden {
		t.Fatalf("edge a->c = %+v ok=%v, want hidden", e, ok)
	}
	if e, _ := b.edge("a", "b"); e.Hidden {
		t.Error("edge a->b is hidden")
	}
	want := [][]string{{"a"}, {"b"}, {"c"}}
	if !reflect.DeepEqual(b.Columns, want) {
		t.Errorf("Columns = %v, want %v", b.Columns, want)
	}
	for id, n := range map[string]int{"a": 1, "b": 0, "c": 1} {
		if got := b.Redundant(id); got != n {
			t.Errorf("Redundant(%s) = %d, want %d", id, got, n)
		}
	}
	if n := len(b.Edges()); n != 3 {
		t.Errorf("Edges() has %d edges, want all 3 including the hidden one", n)
	}
}

func TestCardLevelCycleTerminates(t *testing.T) {
	// The items are acyclic: a2 waits on b2, b1 waits on a1. The cards wait on each other.
	a, bb := mk("A", "root"), mk("B", "root")
	b := board(a, bb, mk("a1", "A"), mk("a2", "A", "b2"), mk("b1", "B", "a1"), mk("b2", "B"))
	for _, id := range []string{"A", "B"} {
		if _, _, ok := b.Pos(id); !ok {
			t.Fatalf("card %s not placed", id)
		}
	}
	drawn := 0
	for _, e := range b.Edges() {
		if !e.Hidden {
			drawn++
		}
	}
	if drawn != 1 {
		t.Errorf("drawn edges = %d, want 1: the edge against the column direction is hidden", drawn)
	}
	if out := b.Render(RenderOpts{Width: 80, Height: 20, Focus: "A"}); out == "" {
		t.Error("cycle board renders nothing")
	}
}

func TestCriticalPath(t *testing.T) {
	chain := func(ids ...string) []*item.Item {
		var out []*item.Item
		for i, id := range ids {
			if i == 0 {
				out = append(out, mk(id, "root"))
				continue
			}
			out = append(out, mk(id, "root", ids[i-1]))
		}
		return out
	}
	long, short := chain("a", "b", "c", "d"), chain("x", "y", "z")

	got := criticalEdges(board(append(append([]*item.Item{}, long...), short...)...))
	if want := []string{"a>b", "b>c", "c>d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("critical edges = %v, want %v", got, want)
	}

	// b and c are done: the long chain has 2 cards left, the short one 3.
	long[1].Status, long[2].Status = "completed", "completed"
	got = criticalEdges(board(append(append([]*item.Item{}, long...), short...)...))
	if want := []string{"x>y", "y>z"}; !reflect.DeepEqual(got, want) {
		t.Errorf("critical edges after completing = %v, want %v", got, want)
	}

	// A tie marks both chains.
	got = criticalEdges(board(append(chain("a", "b"), chain("x", "y")...)...))
	if want := []string{"a>b", "x>y"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tie: critical edges = %v, want %v", got, want)
	}
}

func TestCriticalPathCountsLiftedAndIgnoresHidden(t *testing.T) {
	a, bb, c := mk("A", "root"), mk("B", "root"), mk("C", "root", "B", "A")
	b1 := mk("b1", "B", "a1")
	b := board(a, bb, c, mk("a1", "A"), b1)
	// A -> B is lifted, B -> C direct; A -> C is redundant and hidden.
	if got, want := criticalEdges(b), []string{"A>B", "B>C"}; !reflect.DeepEqual(got, want) {
		t.Errorf("critical edges = %v, want %v", got, want)
	}
}

func TestLooseColumnOnlyForCardsWithoutEdgesAndOrder(t *testing.T) {
	a, bb := mk("a", "root"), mk("b", "root", "a")
	b := board(a, bb, mk("loose1", "root"), mk("loose2", "root"))
	want := [][]string{{"loose1", "loose2"}, {"a"}, {"b"}}
	if !reflect.DeepEqual(b.Columns, want) || !b.Loose {
		t.Errorf("Columns = %v loose=%v, want %v loose", b.Columns, b.Loose, want)
	}
}

func TestEmptyAndUnknownRoot(t *testing.T) {
	b := Build(data.NewIndex(nil), "nope")
	if len(b.Columns) != 0 || b.Render(RenderOpts{Width: 40, Height: 10}) != "" {
		t.Error("unknown root should give an empty board")
	}
	if Build(nil, "x").Render(RenderOpts{Width: 40, Height: 10}) != "" {
		t.Error("nil index should give an empty board")
	}
}

// viaOf returns the pass-through rows of the edge from -> to.
func viaOf(t *testing.T, b *Board, from, to string) []int {
	t.Helper()
	for i, e := range b.edges {
		if e.From == from && e.To == to {
			return b.via[i]
		}
	}
	t.Fatalf("edge %s->%s not found", from, to)
	return nil
}

func TestViaRowAtHeightOfSourceWhenFree(t *testing.T) {
	// x (col 0, row 1) -> t (col 2) crosses col 1, which has a card in row 0 only.
	b := board(mk("p", "root"), mk("x", "root"), mk("q", "root", "p"), mk("t", "root", "q", "x"))
	if got := viaOf(t, b, "x", "t"); !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("via rows = %v, want [1]", got)
	}
}

func TestViaRowBelowCardsWhenHeightIsTaken(t *testing.T) {
	// Same, but col 1 has cards in rows 0 and 1: the pass-through goes below them.
	b := board(mk("p", "root"), mk("x", "root"), mk("q", "root", "p"), mk("r", "root", "p"), mk("t", "root", "q", "x"))
	if got := viaOf(t, b, "x", "t"); !reflect.DeepEqual(got, []int{2}) {
		t.Errorf("via rows = %v, want [2]", got)
	}
}
