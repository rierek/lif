package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
	"github.com/spf13/cobra"
)

// Relation kinds a graph edge can carry. "parent" points from the parent
// item to its child; "blocks" points from the blocking item to the item it
// blocks. Both directions in the front matter (Blocking and BlockedBy)
// collapse to the same "blocks" kind -- see buildGraphEdges.
const (
	relParent = "parent"
	relBlocks = "blocks"
)

// graphEdge is one relationship between two items. It is also the
// deduplication key: Blocking and BlockedBy are not mirrored on disk (see
// buildGraphEdges), so the same logical edge can be declared from either
// end and must collapse to one entry here.
type graphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

// graphNode is one item as it appears in --format json output.
type graphNode struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Type   string `json:"type,omitempty"`
}

// graphModel is the --format json payload: every node in scope, and every
// edge between two nodes in scope.
type graphModel struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

// buildGraphEdges derives every relationship edge from the given items.
//
// Blocking and BlockedBy are not mirrored on disk and not validated for
// symmetry: the mutation resolvers write only the item being mutated
// (pkg/itemgraph/mutations.go), and itemcore only unions the two directions
// transiently for GraphQL reads (pkg/itemgraph/item_fields.go). So
// A.blocking=[B] and B.blocked_by=[A] may both be present for one logical
// edge, or only one of them may be -- both must produce the same edge here.
//
// A link to an item that is not in the given set, or a link from an item to
// itself, is skipped rather than reported: that is lif check's job
// (pkg/itemcore.CheckAllLinks reports these as BrokenLink and SelfLink), and
// a second, differently worded report here would be a rival policy.
func buildGraphEdges(all []*item.Item) []graphEdge {
	byID := make(map[string]*item.Item, len(all))
	for _, b := range all {
		byID[b.ID] = b
	}

	seen := make(map[graphEdge]bool)
	var edges []graphEdge
	add := func(from, to, relation string) {
		if from == to {
			return
		}
		if byID[from] == nil || byID[to] == nil {
			return
		}
		e := graphEdge{From: from, To: to, Relation: relation}
		if seen[e] {
			return
		}
		seen[e] = true
		edges = append(edges, e)
	}

	for _, b := range all {
		if b.Parent != "" {
			add(b.Parent, b.ID, relParent)
		}
		for _, target := range b.Blocking {
			add(b.ID, target, relBlocks)
		}
		for _, blocker := range b.BlockedBy {
			add(blocker, b.ID, relBlocks)
		}
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].Relation != edges[j].Relation {
			return edges[i].Relation < edges[j].Relation
		}
		return edges[i].To < edges[j].To
	})
	return edges
}

// filterGraphEdges keeps only edges whose Relation is in kinds. A nil or
// empty kinds leaves edges unchanged.
func filterGraphEdges(edges []graphEdge, kinds []string) []graphEdge {
	if len(kinds) == 0 {
		return edges
	}
	keep := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		keep[k] = true
	}
	var out []graphEdge
	for _, e := range edges {
		if keep[e.Relation] {
			out = append(out, e)
		}
	}
	return out
}

// scopeGraph limits edges to the neighbourhood of rootID: every item
// reachable within depth hops over the undirected view of edges (From->To
// and To->From both traversable), plus every edge with at least one
// endpoint closer than depth. depth == 0 means the whole connected
// component. The returned map always contains rootID, even when it has no
// edges, so an unlinked item still yields a one-node scope.
func scopeGraph(edges []graphEdge, rootID string, depth int) (map[string]int, []graphEdge) {
	adj := make(map[string][]graphEdge)
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e)
		adj[e.To] = append(adj[e.To], e)
	}

	dist := map[string]int{rootID: 0}
	queue := []string{rootID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		d := dist[id]
		if depth > 0 && d >= depth {
			continue
		}
		for _, e := range adj[id] {
			other := e.To
			if other == id {
				other = e.From
			}
			if _, ok := dist[other]; ok {
				continue
			}
			dist[other] = d + 1
			queue = append(queue, other)
		}
	}

	var kept []graphEdge
	for _, e := range edges {
		distFrom, okFrom := dist[e.From]
		distTo, okTo := dist[e.To]
		if !okFrom || !okTo {
			continue
		}
		if depth == 0 {
			kept = append(kept, e)
			continue
		}
		if distFrom < depth || distTo < depth {
			kept = append(kept, e)
		}
	}
	return dist, kept
}

var (
	graphFormat   string
	graphRelation []string
	graphDepth    int
)

var graphCmd = &cobra.Command{
	Use:   "graph [id]",
	Short: "Print the item relationship graph",
	Long: `Prints the parent and blocking relationships between items.

The default output is Graphviz DOT, which pipes into ` + "`dot -Tpng`" + ` for an
image; --format ascii prints a plain edge list for reading in a terminal,
--format mermaid a Mermaid flowchart that pastes into a Markdown document,
and --format json the same graph as data. Nodes are coloured by the status colour
from the configuration.

Naming an item scopes the output to that item's neighbourhood: --depth 1 (the
default) shows only relationships the item itself takes part in, a higher
depth widens it by that many hops, and --depth 0 walks the whole connected
component.

Links pointing at an item that does not exist, and self-references, are left
out here; ` + "`lif check`" + ` is what reports them.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if graphFormat != "dot" && graphFormat != "ascii" && graphFormat != "json" && graphFormat != "mermaid" {
			return fmt.Errorf("invalid --format %q (must be dot, ascii, mermaid or json)", graphFormat)
		}
		jsonMode := graphFormat == "json"

		if graphDepth < 0 {
			return cmdError(jsonMode, output.ErrValidation, "--depth must be >= 0, got %d", graphDepth)
		}
		if cmd.Flags().Changed("depth") && len(args) == 0 {
			return cmdError(jsonMode, output.ErrValidation, "--depth requires an item id")
		}
		for _, r := range graphRelation {
			if r != relParent && r != relBlocks {
				return cmdError(jsonMode, output.ErrValidation, "invalid --relation %q (must be %s or %s)", r, relParent, relBlocks)
			}
		}

		all := core.All()
		item.SortByStatusPriorityAndType(all, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())

		edges := filterGraphEdges(buildGraphEdges(all), graphRelation)

		var rootID string
		if len(args) == 1 {
			resolved, err := core.Get(args[0])
			if err != nil {
				return cmdError(jsonMode, output.ErrNotFound, "item not found: %s", args[0])
			}
			rootID = resolved.ID

			var nodeIDs map[string]int
			nodeIDs, edges = scopeGraph(edges, rootID, graphDepth)
			filtered := make([]*item.Item, 0, len(nodeIDs))
			for _, b := range all {
				if _, ok := nodeIDs[b.ID]; ok {
					filtered = append(filtered, b)
				}
			}
			all = filtered
		}

		switch graphFormat {
		case "ascii":
			return renderGraphASCII(cmd, edges, rootID)
		case "mermaid":
			return renderGraphMermaid(cmd, all, edges)
		case "json":
			return renderGraphJSON(cmd, all, edges)
		default:
			return renderGraphDot(cmd, all, edges)
		}
	},
}

// dotQuote escapes a string for use inside a DOT double-quoted identifier or
// attribute value.
func dotQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\r\n", `\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\n`)
	return s
}

func renderGraphDot(cmd *cobra.Command, items []*item.Item, edges []graphEdge) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "digraph lif {")
	fmt.Fprintln(w, "  rankdir=LR;")
	fmt.Fprintln(w, `  node [shape=box style=filled fontname="Helvetica"];`)
	for _, b := range items {
		label := dotQuote(b.ID) + `\n` + dotQuote(ui.Truncate(b.Title, 40))
		attrs := fmt.Sprintf(`label="%s"`, label)
		if sc := cfg.GetStatus(b.Status); sc != nil && sc.Color != "" {
			color := string(ui.ResolveColor(sc.Color))
			if strings.HasPrefix(color, "#") {
				attrs += fmt.Sprintf(`, fillcolor="%s"`, color)
			}
		}
		fmt.Fprintf(w, "  \"%s\" [%s];\n", dotQuote(b.ID), attrs)
	}
	for _, e := range edges {
		fmt.Fprintf(w, "  \"%s\" -> \"%s\" [label=\"%s\"];\n", dotQuote(e.From), dotQuote(e.To), dotQuote(e.Relation))
	}
	fmt.Fprintln(w, "}")
	return nil
}

// mermaidHandle turns an item id or status name into a Mermaid node handle or
// class name.
//
// A hyphen is kept: mermaid 11 accepts it in `beans-a --> beans-b` and in a
// class named `status-in-progress`, so a handle stays something a reader can
// look up in the store, and two ids differing only in `-` versus `_` cannot
// collapse onto one node.
//
// Everything outside [A-Za-z0-9_-] becomes an underscore, because the id
// prefix is free-form configuration: `prefix: "my item\"s "` in .beans.yml
// yields the real id `my item"s vm76`, whose space alone splits the handle
// and the class statement into fragments Mermaid cannot read. The label
// still carries the id as written, which is where a reader looks for it.
func mermaidHandle(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// mermaidLabel escapes a string for a quoted Mermaid node label.
//
// The set is smaller than it looks, and it was measured by rendering with
// mermaid-cli rather than derived from the grammar:
//
//   - " would close the label, so it becomes &quot;.
//   - < becomes an entity because Mermaid draws labels as HTML, so markup in
//     a title would otherwise be rendered as markup. A lone > needs no
//     escaping: without an opening < it is text, and it is left alone rather
//     than escaped for symmetry.
//   - # opens a Mermaid entity of the form #91;, which a title may spell out
//     literally, so it becomes #35;.
//   - & opens an HTML entity for the same reason.
//   - Newlines become <br/>, as they become \n in DOT.
//
// Brackets, parentheses and braces are deliberately absent: inside a quoted
// label Mermaid takes them verbatim, so escaping them only made titles
// harder to read. The HTML forms were worse than useless -- Mermaid resolves
// the #91; inside &#91; and leaves the ampersand behind, so a bracket came
// out as "&[".
//
// & and # are escaped before every other rule, because those rules insert
// entities built from both characters and escaping them afterwards would
// mangle this function's own output into visible text.
func mermaidLabel(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "#", "#35;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, "\r\n", "<br/>")
	s = strings.ReplaceAll(s, "\n", "<br/>")
	s = strings.ReplaceAll(s, "\r", "<br/>")
	return s
}

// renderGraphMermaid prints the graph as a Mermaid flowchart, which pastes
// straight into a Markdown document. It keeps --format dot's shape: LR
// direction, id and truncated title in the label, and the status colour
// from the configuration -- as a classDef per status, since Mermaid has no
// per-node fill attribute.
func renderGraphMermaid(cmd *cobra.Command, items []*item.Item, edges []graphEdge) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "flowchart LR")

	classes := make(map[string]string)
	var classOrder []string
	for _, b := range items {
		fmt.Fprintf(w, "  %s[\"%s<br/>%s\"]\n",
			mermaidHandle(b.ID), mermaidLabel(b.ID), mermaidLabel(ui.Truncate(b.Title, 40)))

		sc := cfg.GetStatus(b.Status)
		if sc == nil || sc.Color == "" {
			continue
		}
		colour := string(ui.ResolveColor(sc.Color))
		if !strings.HasPrefix(colour, "#") {
			continue
		}
		name := "status-" + mermaidHandle(b.Status)
		if _, ok := classes[name]; !ok {
			classes[name] = colour
			classOrder = append(classOrder, name)
		}
		fmt.Fprintf(w, "  class %s %s;\n", mermaidHandle(b.ID), name)
	}

	for _, e := range edges {
		fmt.Fprintf(w, "  %s -->|%s| %s\n",
			mermaidHandle(e.From), mermaidLabel(e.Relation), mermaidHandle(e.To))
	}

	for _, name := range classOrder {
		fmt.Fprintf(w, "  classDef %s fill:%s,stroke:#333;\n", name, classes[name])
	}
	return nil
}

func renderGraphASCII(cmd *cobra.Command, edges []graphEdge, rootID string) error {
	w := cmd.OutOrStdout()
	if len(edges) == 0 {
		if rootID != "" {
			fmt.Fprintf(w, "no relationships for %s\n", rootID)
		} else {
			fmt.Fprintln(w, "no relationships")
		}
		return nil
	}

	widest := 0
	for _, e := range edges {
		if w := len([]rune(e.From)); w > widest {
			widest = w
		}
	}
	for _, e := range edges {
		fmt.Fprintf(w, "%s ──%s──> %s\n", ui.PadRight(e.From, widest), e.Relation, e.To)
	}
	return nil
}

func renderGraphJSON(cmd *cobra.Command, items []*item.Item, edges []graphEdge) error {
	model := graphModel{Edges: edges}
	for _, b := range items {
		model.Nodes = append(model.Nodes, graphNode{ID: b.ID, Title: b.Title, Status: b.Status, Type: b.Type})
	}
	if model.Edges == nil {
		model.Edges = []graphEdge{}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(model)
}

func RegisterGraphCmd(root *cobra.Command) {
	// Flags are bound once: these are package-level vars and pflag panics on
	// a second definition, while tests register into a throwaway root.
	if graphCmd.Flags().Lookup("format") == nil {
		graphCmd.Flags().StringVar(&graphFormat, "format", "dot", `Output format: "dot", "ascii", "mermaid" or "json"`)
		graphCmd.Flags().StringArrayVar(&graphRelation, "relation", nil, `Only this relation kind: "parent" or "blocks" (can be repeated)`)
		graphCmd.Flags().IntVar(&graphDepth, "depth", 1, "Hops from the named item; 0 walks the whole connected component (requires an item id)")
	}
	graphCmd.ValidArgsFunction = completionUpTo(1)
	root.AddCommand(graphCmd)
}
