package roadmap

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// Data holds the structured roadmap for JSON output.
type Data struct {
	// Root is set instead of Milestones/Unscheduled when the roadmap is
	// scoped to a single epic or feature (BuildScoped). A
	// milestone-scoped roadmap reuses Milestones with a single entry
	// instead, since that requires no new rendering path.
	Root        *RootGroup        `json:"root,omitempty"`
	Milestones  []MilestoneGroup  `json:"milestones"`
	Unscheduled *UnscheduledGroup `json:"unscheduled,omitempty"`
}

// RootGroup holds the scoped roadmap when rooted at an epic or feature.
// Exactly one of Epic/Feature is set.
type RootGroup struct {
	Epic    *EpicGroup    `json:"epic,omitempty"`
	Feature *FeatureGroup `json:"feature,omitempty"`
}

// Container holds one order-comparable container child of a milestone
// or of the unscheduled bucket: exactly one of Epic/Feature is set. The slot
// exists so an epic and a feature under the same parent sort into one
// sequence (beans-swy1) instead of two rank-segregated blocks.
type Container struct {
	Epic    *EpicGroup    `json:"epic,omitempty"`
	Feature *FeatureGroup `json:"feature,omitempty"`
}

// UnscheduledGroup represents items not assigned to any milestone.
type UnscheduledGroup struct {
	Containers []Container  `json:"containers,omitempty"`
	Other      []*item.Item `json:"other,omitempty"`
}

// MilestoneGroup represents a rank-1 container and its contents. The JSON
// key is a slot name, not a type name: each entry in "containers" carries
// either a rank-2 ("epic") or a rank-3 ("feature") group, whatever they are
// called, sorted together into one sequence (beans-swy1).
type MilestoneGroup struct {
	Milestone  *item.Item   `json:"milestone"`
	Containers []Container  `json:"containers,omitempty"`
	Other      []*item.Item `json:"other,omitempty"`
}

// EpicGroup represents a rank-2 container and its child items. The "epic"
// JSON key is a slot name, not a type name: it holds whatever type is
// configured at rank 2.
type EpicGroup struct {
	Epic     *item.Item     `json:"epic"`
	Items    []*item.Item   `json:"items,omitempty"`
	Features []FeatureGroup `json:"features,omitempty"`
}

// FeatureGroup represents a rank-3 container and the leaf items found
// anywhere beneath it (leafs below nested rank-3 containers are flattened
// into this list). The "feature" JSON key is a slot name, not a type name:
// it holds whatever type is configured at rank 3.
type FeatureGroup struct {
	Feature *item.Item   `json:"feature"`
	Items   []*item.Item `json:"items,omitempty"`
}

// Build constructs the roadmap data structure from items.
func Build(cfg *config.Config, allItems []*item.Item, includeDone bool, statusFilter, noStatusFilter []string) *Data {
	// Index all items by ID for lookups
	byID := make(map[string]*item.Item)
	for _, b := range allItems {
		byID[b.ID] = b
	}

	children := ChildrenIndex(allItems)
	hidden := HiddenSubtrees(cfg, allItems, children)

	// Find milestones, applying status filters
	var milestones []*item.Item
	for _, b := range allItems {
		if !isRank(cfg, b, 1) {
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
	SortSiblings(cfg, milestones)

	// Build milestone groups
	var milestoneGroups []MilestoneGroup
	for _, m := range milestones {
		group := buildMilestoneGroup(cfg, m, children, includeDone, hidden)
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
			if isRank(cfg, child, 2) {
				for _, epicChild := range children[child.ID] {
					underMilestone[epicChild.ID] = true
				}
			}
		}
	}

	// Find unscheduled epics (epics not under a milestone)
	var unscheduledContainers []Container
	for _, b := range allItems {
		if !isRank(cfg, b, 2) {
			continue
		}
		if hidden[b.ID] {
			continue
		}
		if underMilestone[b.ID] {
			continue
		}
		eg := buildEpicGroup(cfg, b, children, includeDone, hidden)
		if len(eg.Items) > 0 || len(eg.Features) > 0 {
			unscheduledContainers = append(unscheduledContainers, Container{Epic: &eg})
		}
	}

	// Find unscheduled features: feature-typed items that are not under a
	// milestone or epic (orphan features, e.g. created without --parent).
	for _, b := range allItems {
		if !isRank(cfg, b, 3) {
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
			if parent, ok := byID[b.Parent]; ok && (isRank(cfg, parent, 2) || isRank(cfg, parent, 3)) {
				continue
			}
		}
		fg := buildFeatureGroup(cfg, b, children, includeDone, hidden)
		if len(fg.Items) > 0 {
			unscheduledContainers = append(unscheduledContainers, Container{Feature: &fg})
		}
	}

	// Sort unscheduled epics and features together
	sortRoadmapContainers(cfg, unscheduledContainers)

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
		if isRank(cfg, b, 1) {
			continue
		}
		if isRank(cfg, b, 2) {
			// Epics with >=1 leaf descendant or feature are rendered via the
			// unscheduledEpics loop above as an EpicGroup; skip them
			// here to avoid double-rendering. Childless epics (beans-36fa)
			// are not containers -- fall through and treat them as a flat
			// leaf like any other orphan item below.
			if eg, _ := classifyEpicChild(cfg, b, children, includeDone, hidden); eg != nil {
				continue
			}
		}
		if isRank(cfg, b, 3) {
			// Features with >=1 leaf descendant are rendered via the
			// unscheduledFeatures loop above as a FeatureGroup; skip them
			// here to avoid double-rendering. Childless features (D01,
			// beans-n8zw) are not containers -- fall through and treat
			// them as a flat leaf like any other orphan item below.
			if fg, _ := classifyFeatureChild(cfg, b, children, includeDone, hidden); fg != nil {
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
	var unscheduled *UnscheduledGroup
	if len(unscheduledContainers) > 0 || len(orphanItems) > 0 {
		unscheduled = &UnscheduledGroup{
			Containers: unscheduledContainers,
			Other:      orphanItems,
		}
	}

	return &Data{
		Milestones:  milestoneGroups,
		Unscheduled: unscheduled,
	}
}

// isRank reports whether an item sits on the given hierarchy rank.
func isRank(cfg *config.Config, b *item.Item, rank int) bool {
	return cfg.RankOf(b.Type) == rank
}

// IsContainerRank reports whether an item sits on a container rank. Leaves
// (rank config.LeafRank) are rendered inside a container, never as one.
func IsContainerRank(cfg *config.Config, b *item.Item) bool {
	r := cfg.RankOf(b.Type)
	return r >= 1 && r <= config.MaxContainerRank
}

// ChildrenIndex maps each item ID to the items that have it as a parent.
func ChildrenIndex(allItems []*item.Item) map[string][]*item.Item {
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

// HiddenSubtrees returns the set of item IDs that must vanish from an
// aggregate view (D15): every container-ranked item whose type opted out
// via cfg.IsRoadmapType, plus everything beneath it, at any depth --
// markSubtree walks every descendant regardless of its own type, so a
// visible-typed epic under a hidden milestone is hidden too.
func HiddenSubtrees(cfg *config.Config, allItems []*item.Item, children map[string][]*item.Item) map[string]bool {
	hidden := make(map[string]bool)
	for _, b := range allItems {
		if IsContainerRank(cfg, b) && !cfg.IsRoadmapType(b.Type) {
			markSubtree(b.ID, children, hidden)
		}
	}
	return hidden
}

// hiddenSubtreesWithinScope is HiddenSubtrees restricted to a scoped-by-ID
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
func hiddenSubtreesWithinScope(cfg *config.Config, allItems []*item.Item, children map[string][]*item.Item, rootID string) map[string]bool {
	descendants := make(map[string]bool)
	markSubtree(rootID, children, descendants)

	hidden := make(map[string]bool)
	for _, b := range allItems {
		if b.ID == rootID || !descendants[b.ID] {
			continue
		}
		if IsContainerRank(cfg, b) && !cfg.IsRoadmapType(b.Type) {
			markSubtree(b.ID, children, hidden)
		}
	}
	return hidden
}

// BuildScoped builds a Data scoped to a single milestone,
// epic, or feature root. Callers must have already validated root's type
// via ValidateRootType; any other type panics, since that would be a
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
func BuildScoped(cfg *config.Config, allItems []*item.Item, includeDone bool, root *item.Item) *Data {
	children := ChildrenIndex(allItems)
	hidden := hiddenSubtreesWithinScope(cfg, allItems, children, root.ID)
	switch cfg.RankOf(root.Type) {
	case 1:
		group := buildMilestoneGroup(cfg, root, children, includeDone, hidden)
		return &Data{Milestones: []MilestoneGroup{group}}
	case 2:
		eg := buildEpicGroup(cfg, root, children, includeDone, hidden)
		return &Data{Root: &RootGroup{Epic: &eg}}
	case 3:
		fg := buildFeatureGroup(cfg, root, children, includeDone, hidden)
		return &Data{Root: &RootGroup{Feature: &fg}}
	default:
		// ValidateRootType is checked by every caller before this
		// runs, so reaching a non-container rank here would be a caller
		// bug, not user input.
		panic("BuildScoped: unsupported root type " + root.Type)
	}
}

// ValidateRootType returns an error if b does not sit on a
// container rank (rank 1 through config.MaxContainerRank).
func ValidateRootType(cfg *config.Config, b *item.Item) error {
	if IsContainerRank(cfg, b) {
		return nil
	}
	containers := cfg.ContainerTypeNames()
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
// unfiltered (BuildScoped's direct-by-ID lookup bypasses hiding).
func buildMilestoneGroup(cfg *config.Config, m *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) MilestoneGroup {
	group := MilestoneGroup{Milestone: m}

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
		if isRank(cfg, child, 2) {
			epics = append(epics, child)
		} else {
			rest = append(rest, child)
		}
	}

	// Build epic groups
	for _, epic := range epics {
		eg := buildEpicGroup(cfg, epic, children, includeDone, hidden)
		if len(eg.Items) > 0 || len(eg.Features) > 0 {
			group.Containers = append(group.Containers, Container{Epic: &eg})
		}
	}

	// Split the milestone's non-epic direct children into leaf items and
	// feature-typed children (which need their own recursive resolution).
	other, featureChildren := splitByContainerType(cfg, rest)

	for _, feature := range featureChildren {
		fg, leaf := classifyFeatureChild(cfg, feature, children, includeDone, hidden)
		if fg != nil {
			group.Containers = append(group.Containers, Container{Feature: fg})
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
	sortRoadmapContainers(cfg, group.Containers)

	// Sort other items
	item.SortRoadmapLeaves(filteredOther, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	group.Other = filteredOther

	return group
}

// buildEpicGroup builds an epic group: its direct leaf children plus a
// recursively-resolved FeatureGroup for each direct feature child. hidden is
// as in buildMilestoneGroup: a hidden child's whole subtree is dropped, not
// folded into Items. Pass nil to render unfiltered (BuildScoped's
// direct-by-ID lookup bypasses hiding).
func buildEpicGroup(cfg *config.Config, epic *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) EpicGroup {
	var visibleChildren []*item.Item
	for _, child := range children[epic.ID] {
		if hidden[child.ID] {
			continue
		}
		visibleChildren = append(visibleChildren, child)
	}
	leafs, featureChildren := splitByContainerType(cfg, visibleChildren)

	eg := EpicGroup{Epic: epic}
	for _, feature := range featureChildren {
		fg, leaf := classifyFeatureChild(cfg, feature, children, includeDone, hidden)
		if fg != nil {
			eg.Features = append(eg.Features, *fg)
		}
		if leaf != nil {
			// Childless feature (D01, beans-n8zw): not a container, render
			// as a flat leaf alongside the epic's other direct items.
			leafs = append(leafs, leaf)
		}
	}

	leafItems := filterChildren(cfg, leafs, includeDone)
	item.SortRoadmapLeaves(leafItems, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	eg.Items = leafItems

	sortFeatureGroups(cfg, eg.Features)
	return eg
}

// classifyFeatureChild resolves a direct feature-typed child item per D01
// (beans-n8zw): a feature is a container IFF it has >=1 leaf descendant
// (collectLeafDescendants, respecting includeDone). If it has descendants,
// the resolved FeatureGroup is returned for container rendering (existing
// behavior, unchanged). If it has none, the feature item itself is returned
// as leaf so the caller can fold it into its own flat-leaf list -- and go
// through the exact same archive-status filtering every other leaf in that
// list goes through, instead of being silently dropped.
func classifyFeatureChild(cfg *config.Config, feature *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) (fg *FeatureGroup, leaf *item.Item) {
	built := buildFeatureGroup(cfg, feature, children, includeDone, hidden)
	if len(built.Items) > 0 {
		return &built, nil
	}
	return nil, feature
}

// classifyEpicChild resolves a direct epic-typed child item per beans-36fa:
// an epic is a container IFF it has >=1 leaf descendant or any feature.
// (buildEpicGroup, respecting includeDone). If it has descendants/features,
// the resolved EpicGroup is kept for container rendering. If it has none,
// the epic item itself is returned as leaf so the caller can fold it into
// its own flat-leaf list -- and go through the exact same archive-status
// filtering every other leaf in that list goes through, instead of being
// silently dropped.
func classifyEpicChild(cfg *config.Config, epic *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) (eg *EpicGroup, leaf *item.Item) {
	built := buildEpicGroup(cfg, epic, children, includeDone, hidden)
	if len(built.Items) > 0 || len(built.Features) > 0 {
		return &built, nil
	}
	return nil, epic
}

// buildFeatureGroup builds a feature group: all leaf descendants found
// anywhere beneath the feature, flattened and sorted. hidden is as in
// buildMilestoneGroup: a hidden descendant, and everything beneath it, is
// dropped rather than flattened into Items. Pass nil to render unfiltered
// (BuildScoped's direct-by-ID lookup bypasses hiding).
func buildFeatureGroup(cfg *config.Config, feature *item.Item, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) FeatureGroup {
	items := collectLeafDescendants(cfg, feature.ID, children, includeDone, hidden)
	item.SortRoadmapLeaves(items, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	return FeatureGroup{Feature: feature, Items: items}
}

// filterChildren filters children based on done status.
func filterChildren(cfg *config.Config, children []*item.Item, includeDone bool) []*item.Item {
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
func splitByContainerType(cfg *config.Config, items []*item.Item) (leafs []*item.Item, features []*item.Item) {
	for _, b := range items {
		if isRank(cfg, b, 3) {
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
// (BuildScoped's direct-by-ID lookup bypasses hiding).
// beans.yml's ValidateParent forbids feature-under-feature via the CLI, so
// this only recurses more than one level on hand-edited data -- the
// visited guard exists purely so a hand-authored parent cycle can't crash
// roadmap with a stack overflow (the old, non-recursive code was immune).
func collectLeafDescendants(cfg *config.Config, parentID string, children map[string][]*item.Item, includeDone bool, hidden map[string]bool) []*item.Item {
	return collectLeafDescendantsVisited(cfg, parentID, children, includeDone, hidden, map[string]bool{})
}

func collectLeafDescendantsVisited(cfg *config.Config, parentID string, children map[string][]*item.Item, includeDone bool, hidden map[string]bool, visited map[string]bool) []*item.Item {
	if visited[parentID] {
		return nil
	}
	visited[parentID] = true

	var leafs []*item.Item
	for _, child := range children[parentID] {
		if hidden[child.ID] {
			continue
		}
		if isRank(cfg, child, 3) {
			leafs = append(leafs, collectLeafDescendantsVisited(cfg, child.ID, children, includeDone, hidden, visited)...)
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
// own item, following the shared roadmap order (roadmap.SortSiblings) --
// across the epic/feature
// rank boundary, not within one rank (beans-swy1).
func sortRoadmapContainers(cfg *config.Config, containers []Container) {
	items := make([]*item.Item, len(containers))
	for i, c := range containers {
		items[i] = containerItem(c)
	}
	SortSiblings(cfg, items)
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
func containerItem(c Container) *item.Item {
	if c.Epic != nil {
		return c.Epic.Epic
	}
	return c.Feature.Feature
}

// sortFeatureGroups sorts FeatureGroups by their Feature item, following the
// same status → dependency → order → priority → created_at chain as
// roadmap.SortSiblings.
func sortFeatureGroups(cfg *config.Config, groups []FeatureGroup) {
	items := make([]*item.Item, len(groups))
	for i, g := range groups {
		items[i] = g.Feature
	}
	SortSiblings(cfg, items)
	rank := make(map[string]int, len(items))
	for i, b := range items {
		rank[b.ID] = i
	}
	sort.SliceStable(groups, func(i, j int) bool { return rank[groups[i].Feature.ID] < rank[groups[j].Feature.ID] })
}
