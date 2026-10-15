package commands

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/template"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

//go:embed roadmap.tmpl
var roadmapTemplateContent string

var (
	roadmapJSON        bool
	roadmapIncludeDone bool
	roadmapStatus      []string
	roadmapNoStatus    []string
	roadmapNoLinks     bool
	roadmapLinkPrefix  string
	roadmapDepth       int
	roadmapTags        bool
	roadmapView        string
	roadmapFormat      string
	roadmapWidthFlag   int
)

// roadmapData holds the structured roadmap for JSON output.
type roadmapData struct {
	// Root is set instead of Milestones/Unscheduled when the roadmap is
	// scoped to a single epic or feature (buildScopedRoadmap). A
	// milestone-scoped roadmap reuses Milestones with a single entry
	// instead, since that requires no new rendering path.
	Root        *rootGroup        `json:"root,omitempty"`
	Milestones  []milestoneGroup  `json:"milestones"`
	Unscheduled *unscheduledGroup `json:"unscheduled,omitempty"`
}

// rootGroup holds the scoped roadmap when rooted at an epic or feature.
// Exactly one of Epic/Feature is set.
type rootGroup struct {
	Epic    *epicGroup    `json:"epic,omitempty"`
	Feature *featureGroup `json:"feature,omitempty"`
}

// roadmapContainer holds one order-comparable container child of a milestone
// or of the unscheduled bucket: exactly one of Epic/Feature is set. The slot
// exists so an epic and a feature under the same parent sort into one
// sequence (beans-swy1) instead of two rank-segregated blocks.
type roadmapContainer struct {
	Epic    *epicGroup    `json:"epic,omitempty"`
	Feature *featureGroup `json:"feature,omitempty"`
}

// unscheduledGroup represents items not assigned to any milestone.
type unscheduledGroup struct {
	Containers []roadmapContainer `json:"containers,omitempty"`
	Other      []*item.Item       `json:"other,omitempty"`
}

// milestoneGroup represents a rank-1 container and its contents. The JSON
// key is a slot name, not a type name: each entry in "containers" carries
// either a rank-2 ("epic") or a rank-3 ("feature") group, whatever they are
// called, sorted together into one sequence (beans-swy1).
type milestoneGroup struct {
	Milestone  *item.Item         `json:"milestone"`
	Containers []roadmapContainer `json:"containers,omitempty"`
	Other      []*item.Item       `json:"other,omitempty"`
}

// epicGroup represents a rank-2 container and its child items. The "epic"
// JSON key is a slot name, not a type name: it holds whatever type is
// configured at rank 2.
type epicGroup struct {
	Epic     *item.Item     `json:"epic"`
	Items    []*item.Item   `json:"items,omitempty"`
	Features []featureGroup `json:"features,omitempty"`
}

// featureGroup represents a rank-3 container and the leaf items found
// anywhere beneath it (leafs below nested rank-3 containers are flattened
// into this list). The "feature" JSON key is a slot name, not a type name:
// it holds whatever type is configured at rank 3.
type featureGroup struct {
	Feature *item.Item   `json:"feature"`
	Items   []*item.Item `json:"items,omitempty"`
}

var roadmapCmd = &cobra.Command{
	Use:   "roadmap [id]",
	Short: "Generate a Markdown roadmap from milestones and epics",
	Long: `Displays a roadmap of milestones, epics, and their child items.

With no argument, renders the entire roadmap. With an ID argument (a milestone,
epic, or feature), scopes the output to that item's subtree only.

The --status and --no-status flags cannot be combined with an ID argument.

--depth limits how many levels below the roadmap root are rendered, following
the tree -L n convention: the root itself never counts. Without an ID argument the root is
the roadmap as a whole, so --depth 1 lists milestones only. With an ID
argument the root is that item, so --depth 1 lists its direct children.

--tags renders each item's tags on a line of their own beneath its title.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Query all items via GraphQL resolver
		resolver := &itemgraph.CoreResolver{Core: core}
		allItems, err := resolver.Items(context.Background(), nil)
		if err != nil {
			return cmdError(roadmapJSON, output.ErrValidation, "querying items: %v", err)
		}

		if err := validateRoadmapDepth(roadmapDepth, cmd.Flags().Changed("depth")); err != nil {
			return cmdError(roadmapJSON, output.ErrValidation, "%s", err)
		}

		form, ok := ui.ParseForm(roadmapView)
		if !ok {
			return cmdError(roadmapJSON, output.ErrValidation, "invalid --view %q: must be one of \"tree\", \"table\"", roadmapView)
		}

		var formatOverride roadmapFormatOverride
		switch roadmapFormat {
		case "":
			formatOverride = roadmapFormatAuto
		case "tty":
			formatOverride = roadmapFormatTTY
		case "markdown":
			formatOverride = roadmapFormatMarkdown
		default:
			return cmdError(roadmapJSON, output.ErrValidation, "invalid --format %q: must be one of \"tty\", \"markdown\"", roadmapFormat)
		}

		// Build the roadmap
		var data *roadmapData
		scoped := len(args) == 1
		if scoped {
			if len(roadmapStatus) > 0 || len(roadmapNoStatus) > 0 {
				return cmdError(roadmapJSON, output.ErrValidation, "--status/--no-status cannot be combined with a roadmap root ID")
			}
			root, err := core.Get(args[0])
			if err != nil {
				return cmdError(roadmapJSON, output.ErrNotFound, "unknown item: %s", args[0])
			}
			if err := validateRoadmapRootType(root); err != nil {
				return cmdError(roadmapJSON, output.ErrValidation, "%s", err)
			}
			data = buildScopedRoadmap(allItems, roadmapIncludeDone, root)
		} else {
			data = buildRoadmap(allItems, roadmapIncludeDone, roadmapStatus, roadmapNoStatus)
		}
		pruneRoadmapDepth(data, roadmapDepth, scoped)

		// JSON output
		if roadmapJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(data)
		}

		// TTY-aware rendering: gerendert am Terminal, Markdown bei Pipe/Redirect
		// (D02). The width is only queried when stdout is a terminal (EARS-6) --
		// a non-TTY caller never pays for term.GetSize, and its result would be
		// discarded by roadmapOutput's markdown branch anyway.
		links := !roadmapNoLinks
		linkPrefix := roadmapLinkPrefix
		if links && linkPrefix == "" {
			// Default: relative path from cwd to .beans directory
			linkPrefix = defaultLinkPrefix()
		}

		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		cols := resolveWidth(roadmapWidthFlag, cmd.Flags().Changed("max-width"), cfg)

		fmt.Print(roadmapOutput(data, isTTY, formatOverride, cols, links, linkPrefix, roadmapTags, form, cfg))
		return nil
	},
}

// roadmapFormatOverride lets --format force which branch roadmapOutput takes,
// independent of isTTY detection. roadmapFormatAuto preserves today's
// detection-only behaviour exactly -- this step does not change what either
// branch renders, only how the branch is chosen.
type roadmapFormatOverride int

const (
	roadmapFormatAuto roadmapFormatOverride = iota
	roadmapFormatTTY
	roadmapFormatMarkdown
)

// roadmapOutput is the testable TTY switch (EARS-1/EARS-2/EARS-5): TTY gets
// the shared layout engine (ui.Render) in the requested form, everything else
// (pipe, redirect, non-terminal) gets renderRoadmapMarkdown unchanged --
// byte-identical to calling renderRoadmapMarkdown directly (Q07/D02). cols is
// passed through to ui.Render as-is -- resolveWidth (roadmap.go's RunE) is
// the one place the width is decided (floor 80, cap from --max-width/config/
// terminal, or uncapped for --max-width 0); a second clamp here would make
// roadmap silently disagree with every other command about what --max-width
// means (beans-dbph). format overrides the isTTY-derived choice when
// it is not roadmapFormatAuto, so a caller can force either branch (e.g. for
// tests, or a user explicitly asking for --format markdown/tty).
func roadmapOutput(data *roadmapData, isTTY bool, format roadmapFormatOverride, cols int, links bool, linkPrefix string, showTags bool, form ui.Form, cfg *config.Config) string {
	renderTTY := isTTY
	switch format {
	case roadmapFormatTTY:
		renderTTY = true
	case roadmapFormatMarkdown:
		renderTTY = false
	}
	if renderTTY {
		return ui.Render(roadmapRows(data), form, "Roadmap", cols, showTags, cfg)
	}
	return renderRoadmapMarkdown(data, links, linkPrefix, showTags)
}

// roadmapRows bridges the grouped roadmapData produced by buildRoadmap /
// buildScopedRoadmap into the flat row list ui.Render expects. It mirrors the
// walk roadmap.tmpl renders (milestone -> epics -> features -> leafs,
// then epic/feature/other for the unscheduled bucket), but builds
// ui.FlatItems instead of writing text directly, and does no sorting of its
// own -- order comes entirely from the builder's slices.
//
// Depth counts tree levels, not the old renderer's indent-in-spaces: each
// milestone (or, for the unscheduled bucket, each of its top-level epics/
// features/other) is its own depth-0 root. ui.RowsFromFlatItems resets its
// ancestry stack whenever depth drops back to 0, so consecutive roots never
// bleed tree lines into each other -- which is also why a milestone's own
// IsLast is irrelevant to what gets drawn (Connector/Stem never consult
// AncestorsLast[0]) and is set to true throughout rather than tracked.
//
// Only the unscheduled bucket gets a Section heading ("Unscheduled"): each
// rank-1 container is itself a Item row, which is heading enough on its own,
// matching the one documented use of Row.Section (ui/columns.go). The wording
// is "Unscheduled" rather than "No Milestone" because a project's rank-1 type
// need not be called "milestone", and a profile may define none at all.
func roadmapRows(data *roadmapData) []ui.Row {
	var items []ui.FlatItem

	if data.Root != nil {
		if data.Root.Epic != nil {
			appendRoadmapEpicGroup(&items, *data.Root.Epic, 0, true)
		}
		if data.Root.Feature != nil {
			appendRoadmapFeatureGroup(&items, *data.Root.Feature, 0, true)
		}
		return ui.RowsFromFlatItems(items)
	}

	for _, mg := range data.Milestones {
		appendRoadmapMilestoneGroup(&items, mg)
	}

	unscheduledAt := -1
	if data.Unscheduled != nil {
		unscheduledAt = len(items)
		appendRoadmapUnscheduledGroup(&items, *data.Unscheduled)
	}

	rows := ui.RowsFromFlatItems(items)
	if unscheduledAt >= 0 && unscheduledAt < len(rows) {
		rows[unscheduledAt].Section = "Unscheduled"
	}
	return rows
}

// appendRoadmapMilestoneGroup appends a milestone, then its container
// children and other leaf items in mg.Containers / mg.Other order, the
// order roadmap.tmpl renders.
func appendRoadmapMilestoneGroup(items *[]ui.FlatItem, mg milestoneGroup) {
	// Unreachable in production -- every construction site sets Milestone --
	// but a hand-built roadmapData literal in a test would otherwise panic
	// inside ui.Render instead of producing an empty render (beans-dbph
	// review F5).
	if mg.Milestone == nil {
		return
	}
	*items = append(*items, ui.FlatItem{Item: mg.Milestone, Depth: 0, IsLast: true})

	total := len(mg.Containers) + len(mg.Other)
	i := 0
	for _, c := range mg.Containers {
		i++
		if c.Epic != nil {
			appendRoadmapEpicGroup(items, *c.Epic, 1, i == total)
		} else {
			appendRoadmapFeatureGroup(items, *c.Feature, 1, i == total)
		}
	}
	for _, leaf := range mg.Other {
		i++
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: 1, IsLast: i == total})
	}
}

// appendRoadmapUnscheduledGroup appends the unscheduled bucket's containers
// and other leaf items at depth 0 -- each is its own root, exactly like a
// milestone-less scoped root, since there is no milestone item to hang them
// under.
func appendRoadmapUnscheduledGroup(items *[]ui.FlatItem, ug unscheduledGroup) {
	total := len(ug.Containers) + len(ug.Other)
	i := 0
	for _, c := range ug.Containers {
		i++
		if c.Epic != nil {
			appendRoadmapEpicGroup(items, *c.Epic, 0, i == total)
		} else {
			appendRoadmapFeatureGroup(items, *c.Feature, 0, i == total)
		}
	}
	for _, leaf := range ug.Other {
		i++
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: 0, IsLast: i == total})
	}
}

// appendRoadmapEpicGroup appends an epic at depth, followed by its direct
// leaf items and nested feature groups at depth+1 -- items before features,
// per roadmap.tmpl.
func appendRoadmapEpicGroup(items *[]ui.FlatItem, eg epicGroup, depth int, isLast bool) {
	// Unreachable in production -- every construction site sets Epic -- but
	// a hand-built epicGroup literal in a test would otherwise panic inside
	// ui.Render instead of producing an empty render (beans-dbph review F5).
	if eg.Epic == nil {
		return
	}
	*items = append(*items, ui.FlatItem{Item: eg.Epic, Depth: depth, IsLast: isLast})

	childDepth := depth + 1
	total := len(eg.Items) + len(eg.Features)
	i := 0
	for _, leaf := range eg.Items {
		i++
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: childDepth, IsLast: i == total})
	}
	for _, fg := range eg.Features {
		i++
		appendRoadmapFeatureGroup(items, fg, childDepth, i == total)
	}
}

// appendRoadmapFeatureGroup appends a feature at depth, followed by its
// flattened leaf items at depth+1.
func appendRoadmapFeatureGroup(items *[]ui.FlatItem, fg featureGroup, depth int, isLast bool) {
	// Unreachable in production -- every construction site sets Feature --
	// but a hand-built featureGroup literal in a test would otherwise panic
	// inside ui.Render instead of producing an empty render (beans-dbph
	// review F5).
	if fg.Feature == nil {
		return
	}
	*items = append(*items, ui.FlatItem{Item: fg.Feature, Depth: depth, IsLast: isLast})

	childDepth := depth + 1
	n := len(fg.Items)
	for i, leaf := range fg.Items {
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: childDepth, IsLast: i == n-1})
	}
}

// buildRoadmap constructs the roadmap data structure from items.
func buildRoadmap(allItems []*item.Item, includeDone bool, statusFilter, noStatusFilter []string) *roadmapData {
	// Index all items by ID for lookups
	byID := make(map[string]*item.Item)
	for _, b := range allItems {
		byID[b.ID] = b
	}

	children := childrenIndex(allItems)
	hidden := hiddenSubtrees(allItems, children)

	// Find milestones, applying status filters
	var milestones []*item.Item
	for _, b := range allItems {
		if !isRank(b, 1) {
			continue
		}
		if hidden[b.ID] {
			continue
		}
		// Apply status filters to milestones
		if len(statusFilter) > 0 && !containsStatus(statusFilter, b.Status) {
			continue
		}
		if len(noStatusFilter) > 0 && containsStatus(noStatusFilter, b.Status) {
			continue
		}
		milestones = append(milestones, b)
	}

	// Sort milestones by status, then dependency, then manual order, then priority, then created date
	item.SortRoadmapContainers(milestones, cfg.StatusNames(), cfg.PriorityNames())

	// Build milestone groups
	var milestoneGroups []milestoneGroup
	for _, m := range milestones {
		group := buildMilestoneGroup(m, children, includeDone, hidden)
		// Only include milestones that have visible content
		if len(group.Containers) > 0 || len(group.Other) > 0 {
			milestoneGroups = append(milestoneGroups, group)
		}
	}

	// Build unscheduled group: items not under any milestone
	// Track which items are under a milestone (directly or via epic)
	underMilestone := make(map[string]bool)
	for _, m := range milestones {
		underMilestone[m.ID] = true
		for _, child := range children[m.ID] {
			underMilestone[child.ID] = true
			// Also mark children of epics under this milestone
			if isRank(child, 2) {
				for _, epicChild := range children[child.ID] {
					underMilestone[epicChild.ID] = true
				}
			}
		}
	}

	// Find unscheduled epics (epics not under a milestone)
	var unscheduledContainers []roadmapContainer
	for _, b := range allItems {
		if !isRank(b, 2) {
			continue
		}
		if hidden[b.ID] {
			continue
		}
		if underMilestone[b.ID] {
			continue
		}
		eg := buildEpicGroup(b, children, includeDone, hidden)
		if len(eg.Items) > 0 || len(eg.Features) > 0 {
			unscheduledContainers = append(unscheduledContainers, roadmapContainer{Epic: &eg})
		}
	}

	// Find unscheduled features: feature-typed items that are not under a
	// milestone or epic (orphan features, e.g. created without --parent).
	for _, b := range allItems {
		if !isRank(b, 3) {
			continue
		}
		if hidden[b.ID] {
			continue
		}
		if underMilestone[b.ID] {
			continue
		}
		// Skip features that are themselves children of an unscheduled epic
		// or of another feature above -- those are already rendered as part
		// of that ancestor's Features list / flattened into its Items via
		// collectLeafDescendants. (feature-under-feature is rejected by
		// ValidateParent via the CLI, but items are hand-editable markdown --
		// this guard keeps hand-edited data from double-rendering.)
		if b.Parent != "" {
			if parent, ok := byID[b.Parent]; ok && (isRank(parent, 2) || isRank(parent, 3)) {
				continue
			}
		}
		fg := buildFeatureGroup(b, children, includeDone, hidden)
		if len(fg.Items) > 0 {
			unscheduledContainers = append(unscheduledContainers, roadmapContainer{Feature: &fg})
		}
	}

	// Sort unscheduled epics and features together
	sortRoadmapContainers(unscheduledContainers, cfg.StatusNames(), cfg.PriorityNames())

	// Find orphan items (not milestone, not epic, no parent or parent is not milestone/epic)
	var orphanItems []*item.Item
	for _, b := range allItems {
		// A hidden container's whole subtree stays out of the roadmap --
		// including here, where its orphaned children would otherwise
		// resurface as unscheduled items.
		if hidden[b.ID] {
			continue
		}
		// Skip milestones and epics -- always containers, never flat leaves.
		if isRank(b, 1) {
			continue
		}
		if isRank(b, 2) {
			// Epics with >=1 leaf descendant or feature are rendered via the
			// unscheduledEpics loop above as an epicGroup; skip them
			// here to avoid double-rendering. Childless epics (beans-36fa)
			// are not containers -- fall through and treat them as a flat
			// leaf like any other orphan item below.
			if eg, _ := classifyEpicChild(b, children, includeDone, hidden); eg != nil {
				continue
			}
		}
		if isRank(b, 3) {
			// Features with >=1 leaf descendant are rendered via the
			// unscheduledFeatures loop above as a featureGroup; skip them
			// here to avoid double-rendering. Childless features (D01,
			// beans-n8zw) are not containers -- fall through and treat
			// them as a flat leaf like any other orphan item below.
			if fg, _ := classifyFeatureChild(b, children, includeDone, hidden); fg != nil {
				continue
			}
		}
		// Skip if already under a milestone
		if underMilestone[b.ID] {
			continue
		}
		// Skip if has a parent (it's under an unscheduled epic, handled above)
		if b.Parent != "" {
			continue
		}
		// Apply done filter
		if !includeDone && cfg.IsArchiveStatus(b.Status) {
			continue
		}
		orphanItems = append(orphanItems, b)
	}

	// Sort orphan items
	item.SortRoadmapLeaves(orphanItems, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())

	// Build unscheduled group if there's content
	var unscheduled *unscheduledGroup
	if len(unscheduledContainers) > 0 || len(orphanItems) > 0 {
		unscheduled = &unscheduledGroup{
			Containers: unscheduledContainers,
			Other:      orphanItems,
		}
	}

	return &roadmapData{
		Milestones:  milestoneGroups,
		Unscheduled: unscheduled,
	}
}

// isRank reports whether an item sits on the given hierarchy rank.
func isRank(b *item.Item, rank int) bool {
	return cfg.RankOf(b.Type) == rank
}

// containerTypeNames returns the names of every type occupying a
// container rank (1 through config.MaxContainerRank), in TypesAtRank's
// per-rank order. isContainerRank and pick.go's roadmapScopeTypes both
// route through this one loop instead of each looping 1..MaxContainerRank
// on its own (beans-v2ox).
func containerTypeNames() []string {
	var names []string
	for rank := 1; rank <= config.MaxContainerRank; rank++ {
		names = append(names, cfg.TypesAtRank(rank)...)
	}
	return names
}

// isContainerRank reports whether an item sits on a container rank. Leaves
// (rank config.LeafRank) are rendered inside a container, never as one.
func isContainerRank(b *item.Item) bool {
	r := cfg.RankOf(b.Type)
	return r >= 1 && r <= config.MaxContainerRank
}

// childrenIndex maps each item ID to the items that have it as a parent.
func childrenIndex(allItems []*item.Item) map[string][]*item.Item {
	children := make(map[string][]*item.Item)
	for _, b := range allItems {
		if b.Parent != "" {
			children[b.Parent] = append(children[b.Parent], b)
		}
	}
	return children
}

// markSubtree records id and every descendant of it in seen.
func markSubtree(id string, children map[string][]*item.Item, seen map[string]bool) {
	if seen[id] {
		return
	}
	seen[id] = true
	for _, child := range children[id] {
		markSubtree(child.ID, children, seen)
	}
}

// hiddenSubtrees returns the set of item IDs that must vanish from an
// aggregate view (D15): every container-ranked item whose type opted out
// via cfg.IsRoadmapType, plus everything beneath it, at any depth --
// markSubtree walks every descendant regardless of its own type, so a
// visible-typed epic under a hidden milestone is hidden too.
func hiddenSubtrees(allItems []*item.Item, children map[string][]*item.Item) map[string]bool {
	hidden := make(map[string]bool)
	for _, b := range allItems {
		if isContainerRank(b) && !cfg.IsRoadmapType(b.Type) {
			markSubtree(b.ID, children, hidden)
		}
	}
	return hidden
}

// hiddenSubtreesWithinScope is hiddenSubtrees restricted to a scoped-by-ID
// root: only a hidden container that is a strict descendant of root can
// seed hiding. This is deliberately narrower than "every item except root":
// an ancestor of root -- however deeply hidden -- is never a strict
// descendant of root, so it can never seed hiding here, even though it
// would have seeded it (and marked root's whole subtree) in an
// allItems-wide scan. root itself is also never a seed, even if its own
// type opted out -- naming a container by ID is a direct lookup, not the
// aggregate view the visibility flag governs, and the bypass is for
// exactly that named node. A hidden container nested anywhere beneath root
// still seeds normally and takes its own subtree down with it, exactly as
// in the unscoped roadmap.
func hiddenSubtreesWithinScope(allItems []*item.Item, children map[string][]*item.Item, rootID string) map[string]bool {
	descendants := make(map[string]bool)
	markSubtree(rootID, children, descendants)

	hidden := make(map[string]bool)
	for _, b := range allItems {
		if b.ID == rootID || !descendants[b.ID] {
			continue
		}
		if isContainerRank(b) && !cfg.IsRoadmapType(b.Type) {
			markSubtree(b.ID, children, hidden)
		}
	}
	return hidden
}

// buildScopedRoadmap builds a roadmapData scoped to a single milestone,
// epic, or feature root. Callers must have already validated root's type
// via validateRoadmapRootType; any other type panics, since that would be a
// caller bug, not user input.
//
// The roadmap-visibility flag (cfg.IsRoadmapType, D15) governs the aggregate
// views -- the unscoped roadmap and lif milestones -- but a direct-by-ID
// lookup of a container the user named here bypasses hiding for the named
// root only: hiddenSubtreesWithinScope seeds hiding only from root's strict
// descendants, so no ancestor of root (hidden or not) can mark anything
// inside the scope, while a hidden container nested anywhere beneath root
// still seeds normally and is suppressed exactly as in the unscoped
// roadmap.
func buildScopedRoadmap(allItems []*item.Item, includeDone bool, root *item.Item) *roadmapData {
	children := childrenIndex(allItems)
	hidden := hiddenSubtreesWithinScope(allItems, children, root.ID)
	switch cfg.RankOf(root.Type) {
	case 1:
		group := buildMilestoneGroup(root, children, includeDone, hidden)
		return &roadmapData{Milestones: []milestoneGroup{group}}
	case 2:
		eg := buildEpicGroup(root, children, includeDone, hidden)
		return &roadmapData{Root: &rootGroup{Epic: &eg}}
	case 3:
		fg := buildFeatureGroup(root, children, includeDone, hidden)
		return &roadmapData{Root: &rootGroup{Feature: &fg}}
	default:
		// validateRoadmapRootType is checked by every caller before this
		// runs, so reaching a non-container rank here would be a caller
		// bug, not user input.
		panic("buildScopedRoadmap: unsupported root type " + root.Type)
	}
}

// validateRoadmapRootType returns an error if b does not sit on a
// container rank (rank 1 through config.MaxContainerRank).
func validateRoadmapRootType(b *item.Item) error {
	if isContainerRank(b) {
		return nil
	}
	containers := containerTypeNames()
	if len(containers) == 0 {
		return fmt.Errorf("this project defines no container types (ranks 1-3), so %s (%s) cannot be a roadmap root",
			b.Type, b.ID)
	}
	return fmt.Errorf("roadmap root must be one of %s, got %s (%s)",
		strings.Join(containers, ", "), b.Type, b.ID)
}

// buildMilestoneGroup builds a milestone group with its epics and other
// items. hidden marks every item whose whole subtree opted out of the
// aggregate views (D15): such a child is dropped outright, not folded into
// Other -- hiding removes, it does not reclassify. Pass nil to render
// unfiltered (buildScopedRoadmap's direct-by-ID lookup bypasses hiding).
func buildMilestoneGroup(m *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) milestoneGroup {
	group := milestoneGroup{Milestone: m}

	// Get direct children of this milestone, dropping any whose subtree is
	// hidden before they can be classified as an epic or folded into Other.
	var directChildren []*item.Item
	for _, child := range children[m.ID] {
		if hidden[child.ID] {
			continue
		}
		directChildren = append(directChildren, child)
	}

	// Separate epics from other items
	var epics []*item.Item
	var rest []*item.Item
	for _, child := range directChildren {
		if isRank(child, 2) {
			epics = append(epics, child)
		} else {
			rest = append(rest, child)
		}
	}

	// Build epic groups
	for _, epic := range epics {
		eg := buildEpicGroup(epic, children, includeDone, hidden)
		if len(eg.Items) > 0 || len(eg.Features) > 0 {
			group.Containers = append(group.Containers, roadmapContainer{Epic: &eg})
		}
	}

	// Split the milestone's non-epic direct children into leaf items and
	// feature-typed children (which need their own recursive resolution).
	other, featureChildren := splitByContainerType(rest)

	for _, feature := range featureChildren {
		fg, leaf := classifyFeatureChild(feature, children, includeDone, hidden)
		if fg != nil {
			group.Containers = append(group.Containers, roadmapContainer{Feature: fg})
		}
		if leaf != nil {
			// Childless feature (D01, beans-n8zw): not a container, render
			// as a flat leaf alongside the milestone's other direct items.
			other = append(other, leaf)
		}
	}

	// Filter the remaining flat "Other" items by done status.
	var filteredOther []*item.Item
	for _, child := range other {
		if includeDone || !cfg.IsArchiveStatus(child.Status) {
			filteredOther = append(filteredOther, child)
		}
	}

	// Sort epics and features together
	sortRoadmapContainers(group.Containers, cfg.StatusNames(), cfg.PriorityNames())

	// Sort other items
	item.SortRoadmapLeaves(filteredOther, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	group.Other = filteredOther

	return group
}

// buildEpicGroup builds an epic group: its direct leaf children plus a
// recursively-resolved featureGroup for each direct feature child. hidden is
// as in buildMilestoneGroup: a hidden child's whole subtree is dropped, not
// folded into Items. Pass nil to render unfiltered (buildScopedRoadmap's
// direct-by-ID lookup bypasses hiding).
func buildEpicGroup(epic *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) epicGroup {
	var visibleChildren []*item.Item
	for _, child := range children[epic.ID] {
		if hidden[child.ID] {
			continue
		}
		visibleChildren = append(visibleChildren, child)
	}
	leafs, featureChildren := splitByContainerType(visibleChildren)

	eg := epicGroup{Epic: epic}
	for _, feature := range featureChildren {
		fg, leaf := classifyFeatureChild(feature, children, includeDone, hidden)
		if fg != nil {
			eg.Features = append(eg.Features, *fg)
		}
		if leaf != nil {
			// Childless feature (D01, beans-n8zw): not a container, render
			// as a flat leaf alongside the epic's other direct items.
			leafs = append(leafs, leaf)
		}
	}

	leafItems := filterChildren(leafs, includeDone)
	item.SortRoadmapLeaves(leafItems, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	eg.Items = leafItems

	sortFeatureGroups(eg.Features, cfg.StatusNames(), cfg.PriorityNames())
	return eg
}

// classifyFeatureChild resolves a direct feature-typed child item per D01
// (beans-n8zw): a feature is a container IFF it has >=1 leaf descendant
// (collectLeafDescendants, respecting includeDone). If it has descendants,
// the resolved featureGroup is returned for container rendering (existing
// behavior, unchanged). If it has none, the feature item itself is returned
// as leaf so the caller can fold it into its own flat-leaf list -- and go
// through the exact same archive-status filtering every other leaf in that
// list goes through, instead of being silently dropped.
func classifyFeatureChild(feature *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) (fg *featureGroup, leaf *item.Item) {
	built := buildFeatureGroup(feature, children, includeDone, hidden)
	if len(built.Items) > 0 {
		return &built, nil
	}
	return nil, feature
}

// classifyEpicChild resolves a direct epic-typed child item per beans-36fa:
// an epic is a container IFF it has >=1 leaf descendant or any feature.
// (buildEpicGroup, respecting includeDone). If it has descendants/features,
// the resolved epicGroup is kept for container rendering. If it has none,
// the epic item itself is returned as leaf so the caller can fold it into
// its own flat-leaf list -- and go through the exact same archive-status
// filtering every other leaf in that list goes through, instead of being
// silently dropped.
func classifyEpicChild(epic *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) (eg *epicGroup, leaf *item.Item) {
	built := buildEpicGroup(epic, children, includeDone, hidden)
	if len(built.Items) > 0 || len(built.Features) > 0 {
		return &built, nil
	}
	return nil, epic
}

// buildFeatureGroup builds a feature group: all leaf descendants found
// anywhere beneath the feature, flattened and sorted. hidden is as in
// buildMilestoneGroup: a hidden descendant, and everything beneath it, is
// dropped rather than flattened into Items. Pass nil to render unfiltered
// (buildScopedRoadmap's direct-by-ID lookup bypasses hiding).
func buildFeatureGroup(feature *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) featureGroup {
	items := collectLeafDescendants(feature.ID, children, includeDone, hidden)
	item.SortRoadmapLeaves(items, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	return featureGroup{Feature: feature, Items: items}
}

// filterChildren filters children based on done status.
func filterChildren(children []*item.Item, includeDone bool) []*item.Item {
	if includeDone {
		// Return a copy to avoid modifying the original
		result := make([]*item.Item, len(children))
		copy(result, children)
		return result
	}

	var filtered []*item.Item
	for _, b := range children {
		if !cfg.IsArchiveStatus(b.Status) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// splitByContainerType separates an item's direct children into leafs
// (anything that isn't a feature) and feature-typed children.
func splitByContainerType(items []*item.Item) (leafs []*item.Item, features []*item.Item) {
	for _, b := range items {
		if isRank(b, 3) {
			features = append(features, b)
		} else {
			leafs = append(leafs, b)
		}
	}
	return leafs, features
}

// collectLeafDescendants recursively walks everything below parentID and
// returns the leaf items found at any depth, flattened. Feature-typed
// descendants are transparent containers: their own children are walked
// too, but the feature item itself is never included in the result. hidden
// is as in buildMilestoneGroup: a hidden descendant, and everything beneath
// it, is dropped rather than flattened in. Pass nil to render unfiltered
// (buildScopedRoadmap's direct-by-ID lookup bypasses hiding).
// beans.yml's ValidateParent forbids feature-under-feature via the CLI, so
// this only recurses more than one level on hand-edited data -- the
// visited guard exists purely so a hand-authored parent cycle can't crash
// roadmap with a stack overflow (the old, non-recursive code was immune).
func collectLeafDescendants(parentID string, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) []*item.Item {
	return collectLeafDescendantsVisited(parentID, children, includeDone, hidden, map[string]bool{})
}

func collectLeafDescendantsVisited(parentID string, children map[string][]*item.Item, includeDone bool, hidden map[string]bool, visited map[string]bool) []*item.Item {
	if visited[parentID] {
		return nil
	}
	visited[parentID] = true

	var leafs []*item.Item
	for _, child := range children[parentID] {
		if hidden[child.ID] {
			continue
		}
		if isRank(child, 3) {
			leafs = append(leafs, collectLeafDescendantsVisited(child.ID, children, includeDone, hidden, visited)...)
			continue
		}
		if !includeDone && cfg.IsArchiveStatus(child.Status) {
			continue
		}
		leafs = append(leafs, child)
	}
	return leafs
}

// containsStatus checks if a status is in the list.
func containsStatus(statuses []string, status string) bool {
	return slices.Contains(statuses, status)
}

// sortRoadmapContainers sorts mixed epic/feature container slots by their
// own item, following the same status → dependency → order → priority →
// created_at chain as item.SortRoadmapContainers -- across the epic/feature
// rank boundary, not within one rank (beans-swy1).
func sortRoadmapContainers(containers []roadmapContainer, statusNames, priorityNames []string) {
	items := make([]*item.Item, len(containers))
	for i, c := range containers {
		items[i] = containerItem(c)
	}
	item.SortRoadmapContainers(items, statusNames, priorityNames)
	rank := make(map[string]int, len(items))
	for i, b := range items {
		rank[b.ID] = i
	}
	sort.SliceStable(containers, func(i, j int) bool {
		return rank[containerItem(containers[i]).ID] < rank[containerItem(containers[j]).ID]
	})
}

// containerItem returns the slot's own item: the epic for an epic slot, the
// feature for a feature slot.
func containerItem(c roadmapContainer) *item.Item {
	if c.Epic != nil {
		return c.Epic.Epic
	}
	return c.Feature.Feature
}

// sortFeatureGroups sorts featureGroups by their Feature item, following the
// same status → dependency → order → priority → created_at chain as
// item.SortRoadmapContainers.
func sortFeatureGroups(groups []featureGroup, statusNames, priorityNames []string) {
	items := make([]*item.Item, len(groups))
	for i, g := range groups {
		items[i] = g.Feature
	}
	item.SortRoadmapContainers(items, statusNames, priorityNames)
	rank := make(map[string]int, len(items))
	for i, b := range items {
		rank[b.ID] = i
	}
	sort.SliceStable(groups, func(i, j int) bool { return rank[groups[i].Feature.ID] < rank[groups[j].Feature.ID] })
}

// renderRoadmapMarkdown renders the roadmap as Markdown using the template.
func renderRoadmapMarkdown(data *roadmapData, links bool, linkPrefix string, showTags bool) string {
	// Create template with closures that capture link and tag settings
	tmpl := template.Must(
		template.New("roadmap").Funcs(template.FuncMap{
			"firstParagraph": firstParagraph,
			"typeBadge":      typeBadge,
			"typeLabel":      typeHeadingLabel,
			"beanRef": func(b *item.Item) string {
				return renderItemRef(b, links, linkPrefix)
			},
			"tagLine": func(b *item.Item, indent string) string {
				return renderTagLine(b, indent, showTags)
			},
		}).Parse(roadmapTemplateContent),
	)

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		panic(err)
	}
	// The template's trailing text emits a final newline after the last
	// block; normalise so the document always ends in exactly one.
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// renderItemRef renders an item ID, optionally as a markdown link.
func renderItemRef(b *item.Item, asLink bool, linkPrefix string) string {
	if !asLink {
		return "(" + b.ID + ")"
	}
	if linkPrefix == "" {
		return fmt.Sprintf("([%s](%s))", b.ID, b.Path)
	}
	// Ensure prefix ends with / for clean concatenation
	if !strings.HasSuffix(linkPrefix, "/") {
		linkPrefix += "/"
	}
	return fmt.Sprintf("([%s](%s%s))", b.ID, linkPrefix, b.Path)
}

// typeHeadingLabel renders an item's own type name for a roadmap heading: the
// name (identity, D03/D05) with its first letter upper-cased, purely a
// display transform on the string itself -- not a config lookup, so it
// renders the same whether or not cfg still knows the type (lif check's
// unknownTypeItems). This mirrors the same first-letter-uppercase idiom
// Config.ShortOf already uses for the one-character type code, rather than
// progressStatusLabel's per-word title-casing, which exists to turn a
// hyphenated status like "in-progress" into "In Progress" and does not fit a
// single-word type name. An empty type -- RankOf("") resolves to LeafRank,
// so buildRoadmap itself never hands an empty-typed item to a heading, but a
// hand-built roadmapData could -- falls back to "Untyped" rather than
// rendering an empty label.
func typeHeadingLabel(typeName string) string {
	if typeName == "" {
		return "Untyped"
	}
	return strings.ToUpper(typeName[:1]) + typeName[1:]
}

// typeBadge returns a shields.io badge markdown for the item type.
func typeBadge(b *item.Item) string {
	if b.Type == "" {
		return ""
	}
	// One colour source: the badge shows the same tone the terminal renders,
	// resolved through the active theme. shields.io wants the hex without the
	// leading hash.
	color := "gray"
	if t := cfg.GetType(b.Type); t != nil && t.Color != "" {
		color = strings.TrimPrefix(string(ui.ResolveColor(t.Color)), "#")
	}
	return fmt.Sprintf("![%s](https://img.shields.io/badge/%s-%s?style=flat-square)", b.Type, b.Type, color)
}

// renderTagLine renders an item's tags as their own Markdown line, mirroring
// the TTY tag row: each tag as inline code, prefixed with "#". The returned
// string starts with a newline so a template can append it directly to the
// item's line; indent keeps a leaf's tags inside its list item. Returns ""
// when tags are off or the item has none.
func renderTagLine(b *item.Item, indent string, showTags bool) string {
	if !showTags || len(b.Tags) == 0 {
		return ""
	}
	tags := make([]string, len(b.Tags))
	for i, t := range b.Tags {
		tags[i] = "`#" + t + "`"
	}
	return "\n" + indent + strings.Join(tags, " ")
}

// defaultLinkPrefix returns the relative path from cwd to the .beans directory.
func defaultLinkPrefix() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(cwd, core.Root())
	if err != nil {
		return ""
	}
	// Convert to forward slashes for URL compatibility
	return filepath.ToSlash(rel)
}

// firstParagraph extracts the first paragraph from a body text.
func firstParagraph(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	// Find the first blank line (paragraph separator)
	lines := strings.Split(body, "\n")
	var para []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			break
		}
		// Skip markdown headers
		if strings.HasPrefix(line, "#") {
			continue
		}
		para = append(para, strings.TrimSpace(line))
	}

	result := strings.Join(para, " ")
	// Truncate if too long
	if len(result) > 200 {
		result = result[:197] + "..."
	}
	return result
}

func RegisterRoadmapCmd(root *cobra.Command) {
	roadmapCmd.ValidArgsFunction = completionUpTo(1)
	// Registration is idempotent: the command is a package-level singleton,
	// so a second Register call in the same process (tests build several
	// roots) would panic in pflag with "flag redefined". list.go carries the
	// same guard; without it here, adding a test that registers this command
	// alongside another one panics rather than failing.
	if roadmapCmd.Flags().Lookup("json") != nil {
		root.AddCommand(roadmapCmd)
		return
	}
	roadmapCmd.Flags().BoolVar(&roadmapJSON, "json", false, "Output as JSON")
	roadmapCmd.Flags().BoolVar(&roadmapIncludeDone, "include-done", false, "Include completed items")
	roadmapCmd.Flags().StringSliceVar(&roadmapStatus, "status", nil, "Filter milestones by status (comma-separated or repeated)")
	roadmapCmd.Flags().StringSliceVar(&roadmapNoStatus, "no-status", nil, "Exclude milestones by status (comma-separated or repeated)")
	_ = roadmapCmd.RegisterFlagCompletionFunc("status", statusFlagCompletion)
	_ = roadmapCmd.RegisterFlagCompletionFunc("no-status", statusFlagCompletion)
	roadmapCmd.Flags().BoolVar(&roadmapNoLinks, "no-links", false, "Don't render item IDs as markdown links")
	roadmapCmd.Flags().StringVar(&roadmapLinkPrefix, "link-prefix", "", "URL prefix for links")
	roadmapCmd.Flags().IntVar(&roadmapDepth, "depth", 0, "Limit output to n levels below the roadmap root (default: no limit)")
	roadmapCmd.Flags().BoolVar(&roadmapTags, "tags", false, "Render each item's tags")
	roadmapCmd.Flags().StringVar(&roadmapView, "view", "tree",
		"Arrangement: table (flat, sortable) or tree (nested)")
	roadmapCmd.Flags().StringVar(&roadmapFormat, "format", "", `Output format: "tty" or "markdown" (default: detect)`)
	roadmapCmd.Flags().IntVar(&roadmapWidthFlag, "max-width", 0,
		"Cap the rendered width; 0 disables the cap (default: display.max_width, else 110)")
	root.AddCommand(roadmapCmd)
}
