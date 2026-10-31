package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rierek/lif/pkg/itemgraph"
)

// blockingEdges returns the blocks edges of edges as ordering edges: To waits
// for From. Parent edges never count.
func blockingEdges(edges []graphEdge) []itemgraph.Edge {
	var out []itemgraph.Edge
	for _, e := range edges {
		if e.Relation == relBlocks {
			out = append(out, itemgraph.Edge{From: e.From, To: e.To})
		}
	}
	return out
}

// formatRedundantEdge names a redundant edge and the path through which it
// still holds: "A → B (holds via X)", with the intermediates joined by " → ".
// lif check and its --fix share this form.
func formatRedundantEdge(r itemgraph.RedundantEdge) string {
	return fmt.Sprintf("%s → %s (holds via %s)", r.From, r.To, strings.Join(r.Via, " → "))
}

// withoutRedundantBlocks drops from edges every blocks edge that the helper
// reports for fullEdges, the blocks edges of the whole store. Judging on the
// full store keeps the verdict independent of scope and --relation.
func withoutRedundantBlocks(edges, fullEdges []graphEdge) []graphEdge {
	redundant := make(map[graphEdge]bool)
	for _, r := range itemgraph.RedundantEdges(blockingEdges(fullEdges)) {
		redundant[graphEdge{From: r.From, To: r.To, Relation: relBlocks}] = true
	}
	return slices.DeleteFunc(slices.Clone(edges), func(e graphEdge) bool { return redundant[e] })
}
