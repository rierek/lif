package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

// setupGraphTest installs a throwaway core and default config into the
// package globals graphCmd.RunE reads, mirroring setupTagTest.
func setupGraphTest(t *testing.T) {
	t.Helper()
	itemsDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	testCfg := config.Default()
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })
}

// mkGraphItem creates and persists an item with the given relationships.
func mkGraphItem(t *testing.T, id, title, itemType, parent string, blocking, blockedBy []string) *item.Item {
	t.Helper()
	b := &item.Item{
		ID:        id,
		Slug:      item.Slugify(title),
		Title:     title,
		Status:    "todo",
		Type:      itemType,
		Parent:    parent,
		Blocking:  blocking,
		BlockedBy: blockedBy,
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create(%s) error = %v", id, err)
	}
	return b
}

// seedGraphFixtures creates the standard fixture set used by most tests:
//
//	beans-aaaa  Parent epic   (epic)
//	beans-bbbb  Child task    (task)  Parent: aaaa, Blocking: [cccc]
//	beans-cccc  Blocked task  (task)  BlockedBy: [bbbb]
//	beans-dddd  Lonely task   (task)  Parent: beans-zzzz (dangling)
func seedGraphFixtures(t *testing.T) {
	t.Helper()
	mkGraphItem(t, "beans-aaaa", "Parent epic", "epic", "", nil, nil)
	mkGraphItem(t, "beans-bbbb", "Child task", "task", "beans-aaaa", []string{"beans-cccc"}, nil)
	mkGraphItem(t, "beans-cccc", "Blocked task", "task", "", nil, []string{"beans-bbbb"})
	mkGraphItem(t, "beans-dddd", "Lonely task", "task", "beans-zzzz", nil, nil)
}

// resetGraphFlags clears the graph* package globals to their registered
// defaults and restores the previous values afterwards.
func resetGraphFlags(t *testing.T) {
	t.Helper()
	oldFormat, oldRelation, oldDepth := graphFormat, graphRelation, graphDepth
	graphFormat, graphRelation, graphDepth = "dot", nil, 1
	t.Cleanup(func() {
		graphFormat, graphRelation, graphDepth = oldFormat, oldRelation, oldDepth
	})
}

// graphCmdWithFlags builds a throwaway command carrying the same flags as
// graphCmd, mirroring createCmdWithOrderFlag/milestonesCmdWithFlags: the
// package-level graphCmd singleton must not be registered twice, but RunE
// needs a *cobra.Command whose Flags().Changed("depth") behaves correctly
// under real flag parsing.
func graphCmdWithFlags() *cobra.Command {
	c := &cobra.Command{Use: "graph"}
	c.Flags().StringVar(&graphFormat, "format", "dot", "")
	c.Flags().StringArrayVar(&graphRelation, "relation", nil, "")
	c.Flags().IntVar(&graphDepth, "depth", 1, "")
	return c
}

// runGraph parses flags then args as a real cobra invocation would, and
// returns graphCmd.RunE's captured stdout.
func runGraph(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := graphCmdWithFlags()
	var buf bytes.Buffer
	c.SetOut(&buf)
	if err := c.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v) error = %v", args, err)
	}
	err := graphCmd.RunE(c, c.Flags().Args())
	return buf.String(), err
}

func TestGraphDotDeduplicatesMirroredBlockEdge(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t)
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	if got := strings.Count(out, `"beans-bbbb" -> "beans-cccc" [label="blocks"]`); got != 1 {
		t.Errorf("blocks edge count = %d, want 1; output:\n%s", got, out)
	}
	if !strings.Contains(out, `"beans-aaaa" -> "beans-bbbb" [label="parent"]`) {
		t.Errorf("missing parent edge; output:\n%s", out)
	}
}

func TestGraphSkipsDanglingAndSelfLinks(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t)
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	if strings.Contains(out, "beans-zzzz") {
		t.Errorf("output mentions dangling target beans-zzzz; output:\n%s", out)
	}
	if !strings.Contains(out, `"beans-dddd"`) {
		t.Errorf("beans-dddd node missing despite its own dangling parent link; output:\n%s", out)
	}
}

func TestGraphASCIIEdgeList(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "--format", "ascii")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	want := "beans-aaaa ──parent──> beans-bbbb\nbeans-bbbb ──blocks──> beans-cccc\n"
	if out != want {
		t.Errorf("ascii output = %q, want %q", out, want)
	}
}

func TestGraphRelationFilter(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "--relation", "parent")
	if err != nil {
		t.Fatalf("runGraph(--relation parent) error = %v", err)
	}
	if strings.Contains(out, `label="blocks"`) {
		t.Errorf("--relation parent leaked a blocks edge; output:\n%s", out)
	}

	out, err = runGraph(t, "--relation", "blocks")
	if err != nil {
		t.Fatalf("runGraph(--relation blocks) error = %v", err)
	}
	if strings.Contains(out, `label="parent"`) {
		t.Errorf("--relation blocks leaked a parent edge; output:\n%s", out)
	}

	_, err = runGraph(t, "--relation", "bogus")
	if err == nil || !strings.Contains(err.Error(), "invalid --relation") {
		t.Errorf("runGraph(--relation bogus) error = %v, want message containing %q", err, "invalid --relation")
	}
}

func TestGraphScopeDepth(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "beans-aaaa")
	if err != nil {
		t.Fatalf("runGraph(beans-aaaa) error = %v", err)
	}
	if !strings.Contains(out, `label="parent"`) {
		t.Errorf("depth-1 scope missing the parent edge; output:\n%s", out)
	}
	if strings.Contains(out, `"beans-bbbb" -> "beans-cccc"`) {
		t.Errorf("depth-1 scope leaked the 2-hop blocks edge; output:\n%s", out)
	}

	out, err = runGraph(t, "beans-aaaa", "--depth", "2")
	if err != nil {
		t.Fatalf("runGraph(beans-aaaa --depth 2) error = %v", err)
	}
	if !strings.Contains(out, `"beans-bbbb" -> "beans-cccc"`) {
		t.Errorf("depth-2 scope missing the 2-hop blocks edge; output:\n%s", out)
	}

	out, err = runGraph(t, "beans-aaaa", "--depth", "0")
	if err != nil {
		t.Fatalf("runGraph(beans-aaaa --depth 0) error = %v", err)
	}
	if !strings.Contains(out, `"beans-bbbb" -> "beans-cccc"`) {
		t.Errorf("depth-0 scope missing the 2-hop blocks edge; output:\n%s", out)
	}
}

func TestGraphJSONIsolatedNode(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "beans-dddd", "--format", "json")
	if err != nil {
		t.Fatalf("runGraph(beans-dddd --format json) error = %v", err)
	}
	var model graphModel
	if err := json.Unmarshal([]byte(out), &model); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", out, err)
	}
	if len(model.Nodes) != 1 || model.Nodes[0].ID != "beans-dddd" {
		t.Errorf("nodes = %+v, want exactly one node beans-dddd", model.Nodes)
	}
	if len(model.Edges) != 0 {
		t.Errorf("edges = %+v, want none", model.Edges)
	}
}

func TestGraphNodeFillcolorComesFromConfig(t *testing.T) {
	withTrueColor(t)
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t)
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	sc := cfg.GetStatus("todo")
	if sc == nil || sc.Color == "" {
		t.Fatalf("default config has no color for status todo")
	}
	want := string(ui.ResolveColor(sc.Color))
	if !strings.Contains(out, `fillcolor="`+want+`"`) {
		t.Errorf("output missing fillcolor=%q derived from config; output:\n%s", want, out)
	}
}

func TestGraphDotEscapesTitleQuotes(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	mkGraphItem(t, "beans-quot", `He said "hi"`, "task", "", nil, nil)

	out, err := runGraph(t)
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	if !strings.Contains(out, `He said \"hi\"`) {
		t.Errorf("output missing escaped title; output:\n%s", out)
	}
	if strings.Contains(out, `He said "hi"`) {
		t.Errorf("output contains unescaped title quotes; output:\n%s", out)
	}
}

func TestGraphRejectsDepthWithoutID(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	_, err := runGraph(t, "--depth", "2")
	if err == nil || !strings.Contains(err.Error(), "--depth requires an item id") {
		t.Errorf("error = %v, want message containing %q", err, "--depth requires an item id")
	}
}

func TestGraphRejectsUnknownFormat(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	_, err := runGraph(t, "--format", "yaml")
	if err == nil || !strings.Contains(err.Error(), "invalid --format") {
		t.Errorf("error = %v, want message containing %q", err, "invalid --format")
	}
}

func TestGraphRejectsUnknownItemID(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	_, err := runGraph(t, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "item not found: nonexistent") {
		t.Errorf("error = %v, want message containing %q", err, "item not found: nonexistent")
	}
}

func TestGraphEmptyStoreIsNotAnError(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)

	out, err := runGraph(t)
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	if !strings.Contains(out, "digraph lif {") || !strings.Contains(out, "}") {
		t.Errorf("dot output on empty store = %q, want a valid empty digraph", out)
	}

	out, err = runGraph(t, "--format", "ascii")
	if err != nil {
		t.Fatalf("runGraph(--format ascii) error = %v", err)
	}
	if out != "no relationships\n" {
		t.Errorf("ascii output on empty store = %q, want %q", out, "no relationships\n")
	}
}

// TestGraphMermaidRendersEveryNodeAndEdge is the feature: the blocked-by
// chain as a Mermaid flowchart, so it pastes into a Markdown document
// instead of being derived by hand from --format json.
func TestGraphMermaidRendersEveryNodeAndEdge(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "--format", "mermaid", "--relation", "blocks", "--depth", "0", "beans-bbbb")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}

	if !strings.HasPrefix(out, "flowchart LR\n") {
		t.Errorf("output does not open with a flowchart header:\n%s", out)
	}
	for _, want := range []string{"beans-bbbb", "beans-cccc", "Child task", "Blocked task"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "beans-aaaa") {
		t.Errorf("--relation blocks leaked the parent edge:\n%s", out)
	}
	if n := strings.Count(out, "-->"); n != 1 {
		t.Errorf("found %d edge arrows, want exactly the one block edge:\n%s", n, out)
	}
}

// TestGraphMermaidUsesTheItemIDVerbatim keeps the handle mappable back to a
// item. Mermaid 11 accepts a hyphenated id, in `beans-a --> beans-b` as well
// as in a class name, so rewriting the hyphen would buy nothing and cost
// two things: a handle a reader cannot look up, and a collision between two
// ids that differ only in `-` versus `_`.
func TestGraphMermaidUsesTheItemIDVerbatim(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "--format", "mermaid", "--relation", "blocks", "beans-bbbb")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}

	var edgeLines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "-->") {
			edgeLines = append(edgeLines, strings.TrimSpace(line))
		}
	}
	if len(edgeLines) != 1 {
		t.Fatalf("want one edge line, got %v", edgeLines)
	}
	if want := "beans-bbbb -->|blocks| beans-cccc"; edgeLines[0] != want {
		t.Errorf("edge line = %q, want %q", edgeLines[0], want)
	}

	// The node handle is the id itself, not a rewritten form of it.
	if !strings.Contains(out, `beans-bbbb["`) {
		t.Errorf("node handle is not the verbatim id:\n%s", out)
	}
	if strings.Contains(out, "beans_bbbb") || strings.Contains(out, "nbeans") {
		t.Errorf("output still carries a sanitised handle:\n%s", out)
	}
}

// TestGraphMermaidClassNamesCarryTheStatus pairs with it for the second
// user of the id escaping: a status like in-progress becomes a class name,
// which Mermaid also accepts with its hyphen.
func TestGraphMermaidClassNamesCarryTheStatus(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	running := &item.Item{ID: "beans-iiii", Slug: "running", Title: "Running", Status: "in-progress", Type: "task"}
	if err := core.Create(running); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}

	out, err := runGraph(t, "--format", "mermaid", "beans-iiii")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	if !strings.Contains(out, "status-in-progress") {
		t.Errorf("class name does not name the status verbatim:\n%s", out)
	}
}

// TestGraphMermaidEscapesLabelSyntax stops a title from ending the node: a
// quote or a bracket in a title would otherwise close the label early and
// produce a diagram that does not render at all.
func TestGraphMermaidEscapesLabelSyntax(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	mkGraphItem(t, "beans-eeee", `Fix "q" [b] (p) {c} &quot; #91; title`, "task", "", []string{"beans-ffff"}, nil)
	mkGraphItem(t, "beans-ffff", "Plain", "task", "", nil, nil)

	out, err := runGraph(t, "--format", "mermaid", "--depth", "0", "beans-eeee")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}

	label := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "beans-eeee") && strings.Contains(line, "[") && !strings.Contains(line, "-->") {
			label = line
		}
	}
	if label == "" {
		t.Fatalf("no node line for beans-eeee:\n%s", out)
	}
	// The label frame is made of the same characters as a title may carry, so
	// the text is read out from between the quotes before anything is
	// asserted about it.
	openIdx, closeIdx := strings.Index(label, `["`), strings.LastIndex(label, `"]`)
	if openIdx < 0 || closeIdx <= openIdx {
		t.Fatalf("node line %q has no quoted label", label)
	}
	text := label[openIdx+2 : closeIdx]
	if strings.Contains(text, `"`) {
		t.Errorf("a raw double quote survives and closes the label: %q", text)
	}

	// The whole label is pinned, not substrings of it: an escaping pass that
	// runs its rules in the wrong order mangles its own entities into
	// visible text -- &#91; becoming &amp;#35;91; -- and every substring
	// assertion still passes on that. The literal &quot; and #91; in the
	// title are here for the same reason: Mermaid resolves both spellings,
	// so both have to survive as text.
	// Brackets stay verbatim on purpose: Mermaid renders them as written
	// inside a quoted label, and the HTML entity form came out as "&[".
	wantText := "beans-eeee<br/>Fix &quot;q&quot; [b] (p) {c} &amp;quot; #35;91; title"
	if text != wantText {
		t.Errorf("label text  = %q\nwant          %q", text, wantText)
	}
	if !strings.Contains(label, "Fix") || !strings.Contains(label, "title") {
		t.Errorf("escaping dropped the words themselves: %q", label)
	}
}

// TestGraphMermaidDistinguishesRelations keeps the two edge kinds apart:
// a parent edge and a block edge mean different things and a reader of the
// diagram must not have to guess which is which.
func TestGraphMermaidDistinguishesRelations(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "--format", "mermaid", "--depth", "0", "beans-bbbb")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	if !strings.Contains(out, relBlocks) {
		t.Errorf("block edge is unlabelled:\n%s", out)
	}
	if !strings.Contains(out, relParent) {
		t.Errorf("parent edge is unlabelled:\n%s", out)
	}
}

// TestGraphMermaidCarriesStatusColour holds the parity with --format dot,
// which is the reason a native renderer beats a jq one-liner: the status is
// visible in the picture rather than only in the label.
func TestGraphMermaidCarriesStatusColour(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	out, err := runGraph(t, "--format", "mermaid", "--relation", "blocks", "beans-bbbb")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}

	sc := cfg.GetStatus("todo")
	if sc == nil || sc.Color == "" {
		t.Skip("default config has no colour for status todo")
	}
	colour := string(ui.ResolveColor(sc.Color))
	if !strings.HasPrefix(colour, "#") {
		t.Skipf("status colour %q is not a hex value", colour)
	}
	if !strings.Contains(out, "classDef") {
		t.Fatalf("no classDef in the output:\n%s", out)
	}
	if !strings.Contains(out, colour) {
		t.Errorf("status colour %s is missing:\n%s", colour, out)
	}
	if !strings.Contains(out, "class ") && !strings.Contains(out, ":::") {
		t.Errorf("nodes are never assigned to a status class:\n%s", out)
	}
}

// TestGraphMermaidIsAValidFormat pins the flag surface: mermaid must pass
// validation and a typo must still be rejected with the full list.
func TestGraphMermaidIsAValidFormat(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	seedGraphFixtures(t)

	if _, err := runGraph(t, "--format", "mermaid"); err != nil {
		t.Errorf("runGraph(--format mermaid) error = %v, want nil", err)
	}
	_, err := runGraph(t, "--format", "mermaidd")
	if err == nil {
		t.Fatal("runGraph(--format mermaidd) error = nil, want a validation error")
	}
	if !strings.Contains(err.Error(), "mermaid") {
		t.Errorf("error %q does not offer mermaid as a choice", err)
	}
}

// TestGraphMermaidNeutralisesHTMLInLabels closes the gap the escaping had:
// Mermaid renders node labels as HTML, so a title carrying markup would be
// drawn as markup rather than as the title a user wrote. Angle brackets are
// escaped for that reason, and the <br/> the renderer inserts itself has to
// survive that escaping -- which fixes the order: escape the text first,
// then join the parts.
func TestGraphMermaidNeutralisesHTMLInLabels(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)
	mkGraphItem(t, "beans-hhhh", "A <b>bold</b> and <img src=x> title", "task", "", nil, nil)

	out, err := runGraph(t, "--format", "mermaid", "beans-hhhh")
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	for _, markup := range []string{"<b>", "</b>", "<img"} {
		if strings.Contains(out, markup) {
			t.Errorf("markup %q reaches the label unescaped:\n%s", markup, out)
		}
	}
	if !strings.Contains(out, "bold") || !strings.Contains(out, "title") {
		t.Errorf("escaping dropped the words themselves:\n%s", out)
	}
	if !strings.Contains(out, "<br/>") {
		t.Errorf("the renderer's own line break was escaped away:\n%s", out)
	}
}

// TestGraphMermaidHandlesAConfiguredPrefix covers the reason the handle is
// not simply the id: beans.prefix is free-form configuration, so an id can
// carry a space or a quote, and either one splits a handle and its class
// statement into fragments Mermaid cannot read. The hyphen is exempt -- it
// parses -- so the guard has to be narrow rather than a blanket rewrite.
func TestGraphMermaidHandlesAConfiguredPrefix(t *testing.T) {
	setupGraphTest(t)
	resetGraphFlags(t)

	// Status names are configuration as well, and they reach the output as
	// class names, so the same hole exists on that side: a status with a
	// space would split the class and classDef lines apart.
	cfg.Items.Prefix = `my bean"s `
	cfg.Statuses = append(cfg.Statuses, config.StatusOverride{Name: `needs "review" now`, Color: "#a6e3a1"})
	hostile := &item.Item{ID: `my bean"s vm76`, Slug: "first", Title: "First", Status: `needs "review" now`, Type: "task"}
	plain := &item.Item{ID: `my bean"s wtwd`, Slug: "second", Title: "Second", Status: `needs "review" now`, Type: "task",
		BlockedBy: []string{`my bean"s vm76`}}
	for _, b := range []*item.Item{hostile, plain} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create() error = %v", err)
		}
	}

	out, err := runGraph(t, "--format", "mermaid", "--relation", "blocks", `my bean"s vm76`)
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "flowchart") || strings.HasPrefix(trimmed, "classDef") {
			continue
		}
		// Every handle on a line is a whitespace-delimited token: the node
		// definition's `id["..."]`, the two ends of an arrow, and the two
		// names in a class statement. A handle may carry neither a space --
		// which is why they are tokens at all -- nor a quote.
		var handles []string
		switch {
		case strings.HasPrefix(trimmed, "class "):
			handles = strings.Fields(strings.TrimSuffix(trimmed, ";"))[1:]
		case strings.Contains(trimmed, "-->"):
			for _, side := range strings.Split(trimmed, "-->") {
				side = strings.TrimSpace(side)
				if i := strings.LastIndex(side, "|"); i >= 0 {
					side = strings.TrimSpace(side[i+1:])
				}
				handles = append(handles, side)
			}
		default:
			handles = []string{strings.Split(trimmed, `["`)[0]}
		}
		for _, h := range handles {
			if h == "" {
				t.Errorf("line %q yielded an empty handle", trimmed)
			}
			if strings.ContainsAny(h, ` "`) {
				t.Errorf("handle %q in line %q carries a space or a quote", h, trimmed)
			}
		}
	}

	// The id itself still reaches the reader, in the label.
	if !strings.Contains(out, `my bean&quot;s vm76<br/>`) {
		t.Errorf("the real id is missing from the label:\n%s", out)
	}
	// The class name went through the same guard, and the classDef that
	// declares it agrees with the class statements that reference it -- a
	// mismatch would leave every node unstyled.
	declared := ""
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) > 1 && fields[0] == "classDef" {
			declared = fields[1]
		}
	}
	if declared == "" {
		t.Fatalf("no classDef in the output:\n%s", out)
	}
	if strings.ContainsAny(declared, ` "`) {
		t.Errorf("class name %q carries a space or a quote", declared)
	}
	if !strings.Contains(out, "class "+strings.Fields(out[strings.Index(out, "class ")+6:])[0]+" "+declared+";") {
		t.Errorf("class statements do not reference the declared %q:\n%s", declared, out)
	}
}
