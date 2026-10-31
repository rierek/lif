package itemgraph

import (
	"maps"
	"slices"
)

// Edge is one directed ordering edge between two IDs: To waits for From.
type Edge struct {
	From, To string
}

// RedundantEdge is an edge that already holds through other edges. Via lists
// the intermediate IDs of an alternative path from Edge.From to Edge.To in
// path order, both ends excluded. Via is never empty.
type RedundantEdge struct {
	Edge
	Via []string
}

// RedundantEdges returns every edge of edges that is redundant, each with an
// alternative path, ordered by From and then To. The result does not depend
// on the order of edges. A repeated edge counts once and an edge from an ID
// to itself is ignored.
//
// An edge is redundant when its target stays reachable from its source after
// all reported edges are removed together. Edges inside a cycle (a strongly
// connected component) are never reported: the parallel paths of a cycle
// would otherwise mark each other redundant and removal would cut the cycle.
// Between two components, an edge is reported when the components are
// connected through a third component; the edges of the transitive reduction
// of the components stay. The alternative path runs only over unreported
// edges, so removing any subset of the result keeps every reachability.
func RedundantEdges(edges []Edge) []RedundantEdge {
	// Distinct non-self edges as sorted adjacency lists over dense indexes.
	idSet := make(map[string]struct{})
	uniq := make(map[Edge]struct{})
	for _, e := range edges {
		if e.From == e.To {
			continue
		}
		uniq[e] = struct{}{}
		idSet[e.From] = struct{}{}
		idSet[e.To] = struct{}{}
	}
	if len(uniq) == 0 {
		return nil
	}
	ids := slices.Sorted(maps.Keys(idSet))
	index := make(map[string]int, len(ids))
	for i, id := range ids {
		index[id] = i
	}
	adj := make([][]int, len(ids))
	for e := range uniq {
		adj[index[e.From]] = append(adj[index[e.From]], index[e.To])
	}
	for _, out := range adj {
		slices.Sort(out) // indexes follow the ID order
	}

	comp, nComp := stronglyConnected(adj)

	// Tarjan numbers components in reverse topological order: every edge
	// between components runs from a higher number to a lower one. reach[c]
	// holds every component reachable from c over one or more edges.
	succ := make([]map[int]struct{}, nComp)
	for u, out := range adj {
		for _, v := range out {
			if comp[u] != comp[v] {
				if succ[comp[u]] == nil {
					succ[comp[u]] = make(map[int]struct{})
				}
				succ[comp[u]][comp[v]] = struct{}{}
			}
		}
	}
	reach := make([]map[int]struct{}, nComp)
	for c := range nComp {
		reach[c] = make(map[int]struct{})
		for d := range succ[c] {
			reach[c][d] = struct{}{}
			for r := range reach[d] {
				reach[c][r] = struct{}{}
			}
		}
	}

	// A component edge c->d is redundant when another successor of c reaches d.
	redundantComp := func(c, d int) bool {
		for z := range succ[c] {
			if z == d {
				continue
			}
			if _, ok := reach[z][d]; ok {
				return true
			}
		}
		return false
	}

	// Kept graph: every edge that is not reported. Alternative paths run in it.
	kept := make([][]int, len(adj))
	type pair struct{ u, v int }
	var reported []pair
	for u, out := range adj {
		for _, v := range out {
			if comp[u] != comp[v] && redundantComp(comp[u], comp[v]) {
				reported = append(reported, pair{u, v})
				continue
			}
			kept[u] = append(kept[u], v)
		}
	}

	var result []RedundantEdge
	for _, p := range reported { // u ascending, then v ascending: the required order
		path := shortestPath(kept, p.u, p.v)
		via := make([]string, 0, len(path)-2)
		for _, n := range path[1 : len(path)-1] {
			via = append(via, ids[n])
		}
		result = append(result, RedundantEdge{Edge: Edge{From: ids[p.u], To: ids[p.v]}, Via: via})
	}
	return result
}

// stronglyConnected returns the Tarjan component number of every node and the
// number of components. Components are numbered in reverse topological order.
func stronglyConnected(adj [][]int) ([]int, int) {
	n := len(adj)
	const unvisited = -1
	order := make([]int, n)
	low := make([]int, n)
	comp := make([]int, n)
	onStack := make([]bool, n)
	for i := range order {
		order[i] = unvisited
		comp[i] = unvisited
	}
	var stack []int
	next, nComp := 0, 0

	var visit func(u int)
	visit = func(u int) {
		order[u], low[u] = next, next
		next++
		stack = append(stack, u)
		onStack[u] = true
		for _, v := range adj[u] {
			switch {
			case order[v] == unvisited:
				visit(v)
				low[u] = min(low[u], low[v])
			case onStack[v]:
				low[u] = min(low[u], order[v])
			}
		}
		if low[u] != order[u] {
			return
		}
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			comp[w] = nComp
			if w == u {
				break
			}
		}
		nComp++
	}
	for u := range n {
		if order[u] == unvisited {
			visit(u)
		}
	}
	return comp, nComp
}

// shortestPath returns the node chain from src to dst in adj (both ends
// included), breadth first with neighbours in ascending order. dst is
// reachable by construction.
func shortestPath(adj [][]int, src, dst int) []int {
	prev := make([]int, len(adj))
	for i := range prev {
		prev[i] = -1
	}
	prev[src] = src
	queue := []int{src}
	for len(queue) > 0 && prev[dst] == -1 {
		u := queue[0]
		queue = queue[1:]
		for _, v := range adj[u] {
			if prev[v] == -1 {
				prev[v] = u
				queue = append(queue, v)
			}
		}
	}
	var path []int
	for n := dst; n != src; n = prev[n] {
		path = append(path, n)
	}
	path = append(path, src)
	slices.Reverse(path)
	return path
}
