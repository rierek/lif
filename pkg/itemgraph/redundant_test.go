package itemgraph

import (
	"reflect"
	"testing"
)

func e(from, to string) Edge { return Edge{From: from, To: to} }

func red(from, to string, via ...string) RedundantEdge {
	return RedundantEdge{Edge: e(from, to), Via: via}
}

func TestRedundantEdges(t *testing.T) {
	tests := []struct {
		name  string
		edges []Edge
		want  []RedundantEdge
	}{
		{
			name:  "no edges",
			edges: nil,
			want:  nil,
		},
		{
			name:  "chain has no redundant edge",
			edges: []Edge{e("a", "b"), e("b", "c")},
			want:  nil,
		},
		{
			name:  "triangle",
			edges: []Edge{e("a", "b"), e("b", "c"), e("a", "c")},
			want:  []RedundantEdge{red("a", "c", "b")},
		},
		{
			// b->d holds through c, and a->d and a->c hold through unreported edges only.
			name:  "alternative path avoids reported edges",
			edges: []Edge{e("a", "b"), e("b", "c"), e("c", "d"), e("a", "c"), e("a", "d"), e("b", "d")},
			want: []RedundantEdge{
				red("a", "c", "b"),
				red("a", "d", "b", "c"),
				red("b", "d", "c"),
			},
		},
		{
			// No shortcut: a→e holds only through the three-hop chain.
			name:  "long chain without shortcut",
			edges: []Edge{e("a", "b"), e("b", "c"), e("c", "d"), e("d", "e"), e("a", "e")},
			want:  []RedundantEdge{red("a", "e", "b", "c", "d")},
		},
		{
			name:  "diamond keeps both arms",
			edges: []Edge{e("a", "b"), e("a", "c"), e("b", "d"), e("c", "d")},
			want:  nil,
		},
		{
			name:  "repeated edges count once and do not make an edge redundant",
			edges: []Edge{e("a", "b"), e("a", "b"), e("a", "b")},
			want:  nil,
		},
		{
			name:  "self edge is ignored and never part of a path",
			edges: []Edge{e("a", "a"), e("a", "b"), e("b", "b")},
			want:  nil,
		},
		{
			name:  "repeated edge in a triangle is reported once",
			edges: []Edge{e("a", "b"), e("b", "c"), e("a", "c"), e("a", "c")},
			want:  []RedundantEdge{red("a", "c", "b")},
		},
		{
			name:  "edges inside a cycle are never reported",
			edges: []Edge{e("a", "b"), e("b", "a"), e("a", "c"), e("b", "c")},
			want:  nil,
		},
		{
			// x->c holds through the cycle {a, b}; the parallel exits a->c and b->c both stay.
			name:  "cycle on the alternative path",
			edges: []Edge{e("x", "a"), e("a", "b"), e("b", "a"), e("a", "c"), e("x", "c")},
			want:  []RedundantEdge{red("x", "c", "a")},
		},
		{
			// Each of a->c and b->d reaches its target through the other one and the
			// cycles; reporting both would cut {a, b} off from {c, d}.
			name:  "parallel edges between two cycles stay",
			edges: []Edge{e("a", "b"), e("b", "a"), e("c", "d"), e("d", "c"), e("a", "c"), e("b", "d")},
			want:  nil,
		},
		{
			name: "parallel edges between two cycles with a third path are all reported",
			edges: []Edge{
				e("a", "b"), e("b", "a"), e("c", "d"), e("d", "c"),
				e("a", "c"), e("b", "d"), e("a", "m"), e("m", "c"),
			},
			want: []RedundantEdge{
				red("a", "c", "m"),
				red("b", "d", "a", "m", "c"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedundantEdges(tt.edges)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RedundantEdges(%v) = %v, want %v", tt.edges, got, tt.want)
			}
		})
	}
}

func TestRedundantEdgesIgnoresInputOrder(t *testing.T) {
	forward := []Edge{e("a", "b"), e("b", "c"), e("c", "d"), e("a", "c"), e("a", "d"), e("b", "d")}
	shuffled := []Edge{e("b", "d"), e("a", "d"), e("a", "c"), e("c", "d"), e("a", "b"), e("b", "c")}
	want := []RedundantEdge{
		red("a", "c", "b"),
		red("a", "d", "b", "c"),
		red("b", "d", "c"),
	}
	for name, edges := range map[string][]Edge{"forward": forward, "shuffled": shuffled} {
		if got := RedundantEdges(edges); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: RedundantEdges() = %v, want %v", name, got, want)
		}
	}
}

func TestRedundantEdgesDoesNotChangeItsInput(t *testing.T) {
	edges := []Edge{e("a", "b"), e("b", "c"), e("a", "c")}
	RedundantEdges(edges)
	if want := []Edge{e("a", "b"), e("b", "c"), e("a", "c")}; !reflect.DeepEqual(edges, want) {
		t.Errorf("input changed to %v", edges)
	}
}
