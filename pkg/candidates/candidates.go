// Package candidates produces the option sets shown by the lif TUI's
// pickers (parent, blocking, status, type, priority, tag). It is
// independent of any rendering framework: every producer here returns
// plain data, never a bubbletea/lipgloss view.
package candidates

import (
	"context"
	"sort"
	"strings"

	"github.com/xRiErOS/lif/pkg/config"
	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemcore"
	"github.com/xRiErOS/lif/pkg/itemgraph"
)

// ParentCandidates returns the items eligible to become the parent of the
// items identified by itemIDs (whose types are itemTypes), sorted by type
// order then title (case-insensitive). An item is eligible only if its type
// is a valid parent type for every selected item, and it is neither one of
// the selected items nor a descendant of any of them (which would create a
// cycle) nor in an archive status.
func ParentCandidates(ctx context.Context, resolver *itemgraph.CoreResolver, cfg *config.Config, itemIDs, itemTypes []string) ([]*item.Item, error) {
	var validParentTypes []string
	for i, itemType := range itemTypes {
		typeParents := itemcore.ValidParentTypes(cfg, itemType)
		if i == 0 {
			validParentTypes = typeParents
		} else {
			validParentTypes = intersectStrings(validParentTypes, typeParents)
		}
	}

	allItems, err := resolver.Items(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Collect all descendants of all selected items (to prevent cycles).
	allDescendants := make(map[string]bool)
	for _, itemID := range itemIDs {
		for descID := range collectDescendants(itemID, allItems) {
			allDescendants[descID] = true
		}
	}

	selectedSet := make(map[string]bool)
	for _, id := range itemIDs {
		selectedSet[id] = true
	}

	// Filter to eligible parents:
	// 1. Must be of a valid parent type for ALL selected items
	// 2. Must not be any of the selected items
	// 3. Must not be a descendant of any selected item (to prevent cycles)
	// 4. Must not have an archive status (Config.IsArchiveStatus); only the
	//    item's own status counts, not its ancestors'
	var eligibleItems []*item.Item
	for _, b := range allItems {
		if selectedSet[b.ID] {
			continue
		}
		if allDescendants[b.ID] || cfg.IsArchiveStatus(b.Status) {
			continue
		}
		isValidType := false
		for _, validType := range validParentTypes {
			if b.Type == validType {
				isValidType = true
				break
			}
		}
		if !isValidType {
			continue
		}
		eligibleItems = append(eligibleItems, b)
	}

	sortByTypeThenTitle(eligibleItems, cfg)
	return eligibleItems, nil
}

// BlockingCandidates returns every item other than itemID, sorted by type
// order then title (case-insensitive). Like ParentCandidates, it excludes
// itemID's own descendants: an item may not block or be blocked by an item
// in its own subtree.
func BlockingCandidates(ctx context.Context, resolver *itemgraph.CoreResolver, cfg *config.Config, itemID string) ([]*item.Item, error) {
	allItems, err := resolver.Items(ctx, nil)
	if err != nil {
		return nil, err
	}

	descendants := collectDescendants(itemID, allItems)

	var eligibleItems []*item.Item
	for _, b := range allItems {
		if b.ID == itemID {
			continue
		}
		if descendants[b.ID] {
			continue
		}
		eligibleItems = append(eligibleItems, b)
	}

	sortByTypeThenTitle(eligibleItems, cfg)
	return eligibleItems, nil
}

// sortByTypeThenTitle sorts items by cfg's type order, then by
// case-insensitive title.
func sortByTypeThenTitle(items []*item.Item, cfg *config.Config) {
	typeNames := cfg.TypeNames()
	typeOrder := make(map[string]int)
	for i, t := range typeNames {
		typeOrder[t] = i
	}
	sort.Slice(items, func(i, j int) bool {
		ti, tj := typeOrder[items[i].Type], typeOrder[items[j].Type]
		if ti != tj {
			return ti < tj
		}
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
}

// StatusCandidates returns the available item statuses, merged from cfg's
// configured overrides onto the built-in defaults (*config.Config's
// StatusList, beans-b2v8). A nil cfg is not an error: StatusList tolerates
// it and falls back to the built-in defaults, so callers that have no
// config in hand yet (e.g. a picker constructed before one loads) still
// get a usable, if unconfigured, candidate list rather than a panic.
func StatusCandidates(cfg *config.Config) []config.StatusConfig {
	return cfg.StatusList()
}

// TypeCandidates is StatusCandidates' --type counterpart.
func TypeCandidates(cfg *config.Config) []config.TypeConfig {
	return cfg.TypeList()
}

// PriorityCandidates is StatusCandidates' --priority counterpart.
func PriorityCandidates(cfg *config.Config) []config.PriorityConfig {
	return cfg.PriorityList()
}

// TagCount holds a tag and its usage count across all items.
type TagCount struct {
	Tag   string
	Count int
}

// TagCandidates returns every tag in use across all items with its usage
// count, sorted by count descending, then by tag ascending.
func TagCandidates(ctx context.Context, resolver *itemgraph.CoreResolver) ([]TagCount, error) {
	items, err := resolver.Items(ctx, nil)
	if err != nil {
		return nil, err
	}

	tagCounts := make(map[string]int)
	for _, b := range items {
		for _, tag := range b.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]TagCount, 0, len(tagCounts))
	for tag, count := range tagCounts {
		tags = append(tags, TagCount{Tag: tag, Count: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Tag < tags[j].Tag
	})

	return tags, nil
}

// intersectStrings returns the intersection of two string slices.
func intersectStrings(a, b []string) []string {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}
	var result []string
	for _, s := range b {
		if set[s] {
			result = append(result, s)
		}
	}
	return result
}

// collectDescendants returns a set of all item IDs that are descendants of
// the given item.
func collectDescendants(itemID string, allItems []*item.Item) map[string]bool {
	descendants := make(map[string]bool)

	// Build parent->children map
	children := make(map[string][]string)
	for _, b := range allItems {
		if b.Parent != "" {
			children[b.Parent] = append(children[b.Parent], b.ID)
		}
	}

	// BFS to collect all descendants
	queue := children[itemID]
	for len(queue) > 0 {
		childID := queue[0]
		queue = queue[1:]
		if !descendants[childID] {
			descendants[childID] = true
			queue = append(queue, children[childID]...)
		}
	}

	return descendants
}
