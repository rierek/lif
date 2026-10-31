package commands

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// seedReduceFixtures creates an epic with three children that form the
// triangle a→b, b→c, a→c (a→c declared on both ends) and a task d that waits
// for a. The only redundant edge is a→c, held via b.
//
//	beans-eeee  epic, parent of aaa1, bbb2, ccc3
//	beans-aaa1  Blocking: [bbb2, ccc3]
//	beans-bbb2  Blocking: [ccc3]
//	beans-ccc3  BlockedBy: [aaa1]
//	beans-ddd4  BlockedBy: [aaa1]
func seedReduceFixtures(t *testing.T) {
	t.Helper()
	mkGraphItem(t, "beans-eeee", "Epic", "epic", "", nil, nil)
	mkGraphItem(t, "beans-aaa1", "A", "task", "beans-eeee", []string{"beans-bbb2", "beans-ccc3"}, nil)
	mkGraphItem(t, "beans-bbb2", "B", "task", "beans-eeee", []string{"beans-ccc3"}, nil)
	mkGraphItem(t, "beans-ccc3", "C", "task", "beans-eeee", nil, []string{"beans-aaa1"})
	mkGraphItem(t, "beans-ddd4", "D", "task", "", nil, []string{"beans-aaa1"})
}

func graphJSON(t *testing.T, args ...string) graphModel {
	t.Helper()
	out, err := runGraph(t, append([]string{"--format", "json"}, args...)...)
	if err != nil {
		t.Fatalf("runGraph(%v) error = %v", args, err)
	}
	var m graphModel
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	return m
}

func nodeIDsOf(m graphModel) []string {
	var ids []string
	for _, n := range m.Nodes {
		ids = append(ids, n.ID)
	}
	slices.Sort(ids)
	return ids
}

// SC-03: in every format, --reduce drops exactly the redundant edge and the
// store stays untouched.
func TestGraphReduceOmitsTheRedundantEdgeInEveryFormat(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedReduceFixtures(t)
	before := storeFiles(t, core.Root())

	for _, format := range []string{"dot", "mermaid", "ascii", "json"} {
		t.Run(format, func(t *testing.T) {
			full, err := runGraph(t, "--format", format)
			if err != nil {
				t.Fatalf("runGraph() error = %v", err)
			}
			reduced, err := runGraph(t, "--format", format, "--reduce")
			if err != nil {
				t.Fatalf("runGraph(--reduce) error = %v", err)
			}
			if format == "json" {
				var f, r graphModel
				if err := json.Unmarshal([]byte(full), &f); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(reduced), &r); err != nil {
					t.Fatal(err)
				}
				if len(r.Edges) != len(f.Edges)-1 {
					t.Errorf("reduced edges = %d, want %d", len(r.Edges), len(f.Edges)-1)
				}
				if slices.Contains(r.Edges, graphEdge{From: "beans-aaa1", To: "beans-ccc3", Relation: relBlocks}) {
					t.Errorf("reduced edges still hold aaa1 → ccc3: %v", r.Edges)
				}
				return
			}
			fullLines := strings.Split(full, "\n")
			reducedLines := strings.Split(reduced, "\n")
			var dropped []string
			for _, l := range fullLines {
				if !slices.Contains(reducedLines, l) {
					dropped = append(dropped, l)
				}
			}
			if len(dropped) != 1 || !strings.Contains(dropped[0], "aaa1") || !strings.Contains(dropped[0], "ccc3") {
				t.Errorf("--reduce dropped %q, want exactly the aaa1 → ccc3 line\nfull:\n%s\nreduced:\n%s", dropped, full, reduced)
			}
			if len(reducedLines) != len(fullLines)-1 {
				t.Errorf("reduced output has %d lines, want %d", len(reducedLines), len(fullLines)-1)
			}
		})
	}

	if after := storeFiles(t, core.Root()); !reflect.DeepEqual(before, after) {
		t.Errorf("lif graph changed the store")
	}
}

// AC 16: the node set and the parent edges do not depend on --reduce, with and
// without an item id and --depth.
func TestGraphReduceKeepsNodesAndParentEdgesInEveryScope(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedReduceFixtures(t)

	tests := []struct {
		name string
		args []string
	}{
		{"whole store", nil},
		{"item, depth 1", []string{"beans-ccc3"}},
		{"item, depth 2", []string{"--depth", "2", "beans-ccc3"}},
		{"item, whole component", []string{"--depth", "0", "beans-eeee"}},
		{"lonely end of the redundant edge", []string{"beans-ddd4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			full := graphJSON(t, tt.args...)
			reduced := graphJSON(t, append([]string{"--reduce"}, tt.args...)...)
			if !reflect.DeepEqual(nodeIDsOf(full), nodeIDsOf(reduced)) {
				t.Errorf("nodes differ: full %v, reduced %v", nodeIDsOf(full), nodeIDsOf(reduced))
			}
			parentEdges := func(m graphModel) []graphEdge {
				var out []graphEdge
				for _, e := range m.Edges {
					if e.Relation == relParent {
						out = append(out, e)
					}
				}
				return out
			}
			if !reflect.DeepEqual(parentEdges(full), parentEdges(reduced)) {
				t.Errorf("parent edges differ: full %v, reduced %v", parentEdges(full), parentEdges(reduced))
			}
		})
	}
}

// The verdict comes from the whole store, so a scoped run omits the edge too.
func TestGraphReduceJudgesTheEdgeOnTheFullStore(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedReduceFixtures(t)

	if m := graphJSON(t, "beans-ccc3"); !slices.Contains(m.Edges, graphEdge{From: "beans-aaa1", To: "beans-ccc3", Relation: relBlocks}) {
		t.Fatalf("fixture: the unreduced scope lacks aaa1 → ccc3: %v", m.Edges)
	}
	m := graphJSON(t, "--reduce", "beans-ccc3")
	if slices.Contains(m.Edges, graphEdge{From: "beans-aaa1", To: "beans-ccc3", Relation: relBlocks}) {
		t.Errorf("--reduce kept aaa1 → ccc3 in the scope of ccc3: %v", m.Edges)
	}
	if !slices.Contains(m.Edges, graphEdge{From: "beans-bbb2", To: "beans-ccc3", Relation: relBlocks}) {
		t.Errorf("--reduce lost bbb2 → ccc3: %v", m.Edges)
	}
}

func TestGraphWithoutRedundantEdgesIsUnchangedByReduce(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)
	for _, format := range []string{"dot", "mermaid", "ascii", "json"} {
		full, _ := runGraph(t, "--format", format)
		reduced, _ := runGraph(t, "--format", format, "--reduce")
		if full != reduced {
			t.Errorf("%s: --reduce changed the output of a store without redundant edges\nfull:\n%s\nreduced:\n%s", format, full, reduced)
		}
	}
}
