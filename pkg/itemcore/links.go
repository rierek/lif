package itemcore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// IncomingLink represents a link from another item to a target item.
type IncomingLink struct {
	FromItem *item.Item
	LinkType string
}

// BrokenLink represents a link to a non-existent item.
type BrokenLink struct {
	ItemID   string `json:"item_id"`
	LinkType string `json:"link_type"`
	Target   string `json:"target"`
}

// SelfLink represents an item linking to itself.
type SelfLink struct {
	ItemID   string `json:"item_id"`
	LinkType string `json:"link_type"`
}

// Cycle represents a circular dependency in links.
type Cycle struct {
	LinkType string   `json:"link_type"`
	Path     []string `json:"path"`
}

// LinkCheckResult contains all link validation issues found.
type LinkCheckResult struct {
	BrokenLinks []BrokenLink `json:"broken_links"`
	SelfLinks   []SelfLink   `json:"self_links"`
	Cycles      []Cycle      `json:"cycles"`
}

// HasIssues returns true if any link issues were found.
func (r *LinkCheckResult) HasIssues() bool {
	return len(r.BrokenLinks) > 0 || len(r.SelfLinks) > 0 || len(r.Cycles) > 0
}

// TotalIssues returns the total count of all issues.
func (r *LinkCheckResult) TotalIssues() int {
	return len(r.BrokenLinks) + len(r.SelfLinks) + len(r.Cycles)
}

// cloneIncomingLinks copies the index entries and the items behind them. The
// reverse index stores the live pointers on purpose — it is rebuilt from the
// map — so the copy has to happen where the lock is still held.
func cloneIncomingLinks(links []IncomingLink) []IncomingLink {
	if links == nil {
		return nil
	}
	out := make([]IncomingLink, len(links))
	for i, l := range links {
		out[i] = IncomingLink{FromItem: l.FromItem.Clone(), LinkType: l.LinkType}
	}
	return out
}

// cloneItems copies a slice of items taken from the store.
func cloneItems(items []*item.Item) []*item.Item {
	if items == nil {
		return nil
	}
	out := make([]*item.Item, len(items))
	for i, b := range items {
		out[i] = b.Clone()
	}
	return out
}

// FindIncomingLinks returns all items that link TO the given item ID.
// It answers from a reverse index built once per mutation, so a caller walking
// many items (children, blocked-by and blocking resolvers do exactly that)
// no longer scans the whole store per item.
func (c *Core) FindIncomingLinks(targetID string) []IncomingLink {
	c.mu.RLock()
	if c.incomingValid {
		links := cloneIncomingLinks(c.incoming[targetID])
		c.mu.RUnlock()
		return links
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.rebuildIncomingLocked()
	return cloneIncomingLinks(c.incoming[targetID])
}

// rebuildIncomingLocked recomputes the reverse link index from items.
// Must be called with c.mu held for writing.
func (c *Core) rebuildIncomingLocked() {
	if c.incomingValid {
		return
	}

	index := make(map[string][]IncomingLink, len(c.items))

	// Sorted so the answer does not change between runs — map iteration alone
	// would reorder the links of a target on every rebuild.
	ids := make([]string, 0, len(c.items))
	for id := range c.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		b := c.items[id]
		if b.Parent != "" {
			index[b.Parent] = append(index[b.Parent], IncomingLink{FromItem: b, LinkType: "parent"})
		}
		for _, blocked := range b.Blocking {
			index[blocked] = append(index[blocked], IncomingLink{FromItem: b, LinkType: "blocking"})
		}
		// Inverse: if A has blocked_by B, then B links to A
		for _, blocker := range b.BlockedBy {
			index[blocker] = append(index[blocker], IncomingLink{FromItem: b, LinkType: "blocked_by"})
		}
	}

	c.incoming = index
	c.incomingValid = true
}

// DetectCycle checks if adding a link from fromID to toID would create a cycle.
// Checks for blocking, blocked_by, and parent link types.
// Returns the cycle path if a cycle would be created, nil otherwise.
func (c *Core) DetectCycle(fromID, linkType, toID string) []string {
	// Only check hierarchical link types
	if linkType != "blocking" && linkType != "blocked_by" && linkType != "parent" {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Build adjacency list for the specific link type
	// Adding edge: fromID -> toID
	// Check if there's already a path from toID back to fromID
	visited := make(map[string]bool)
	path := []string{fromID, toID}

	return c.findPathToTarget(toID, fromID, linkType, visited, path)
}

// findPathToTarget uses DFS to find if there's a path from current to target.
// Returns the path if found, nil otherwise.
func (c *Core) findPathToTarget(current, target, linkType string, visited map[string]bool, path []string) []string {
	if current == target {
		return path
	}

	if visited[current] {
		return nil
	}
	visited[current] = true

	b, ok := c.items[current]
	if !ok {
		return nil
	}

	// Get targets based on link type
	var targets []string
	switch linkType {
	case "parent":
		if b.Parent != "" {
			targets = []string{b.Parent}
		}
	case "blocking":
		targets = b.Blocking
	case "blocked_by":
		targets = b.BlockedBy
	}

	for _, t := range targets {
		newPath := append(path, t)
		if result := c.findPathToTarget(t, target, linkType, visited, newPath); result != nil {
			return result
		}
	}

	return nil
}

// CheckAllLinks validates all links across all items.
func (c *Core) CheckAllLinks() *LinkCheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := &LinkCheckResult{
		BrokenLinks: []BrokenLink{},
		SelfLinks:   []SelfLink{},
		Cycles:      []Cycle{},
	}

	// Check for broken links and self-references
	for _, b := range c.items {
		// Check parent link
		if b.Parent != "" {
			if b.Parent == b.ID {
				result.SelfLinks = append(result.SelfLinks, SelfLink{
					ItemID:   b.ID,
					LinkType: "parent",
				})
			} else if _, ok := c.items[b.Parent]; !ok {
				result.BrokenLinks = append(result.BrokenLinks, BrokenLink{
					ItemID:   b.ID,
					LinkType: "parent",
					Target:   b.Parent,
				})
			}
		}

		// Check blocking links
		for _, blocked := range b.Blocking {
			if blocked == b.ID {
				result.SelfLinks = append(result.SelfLinks, SelfLink{
					ItemID:   b.ID,
					LinkType: "blocking",
				})
			} else if _, ok := c.items[blocked]; !ok {
				result.BrokenLinks = append(result.BrokenLinks, BrokenLink{
					ItemID:   b.ID,
					LinkType: "blocking",
					Target:   blocked,
				})
			}
		}

		// Check blocked_by links
		for _, blocker := range b.BlockedBy {
			if blocker == b.ID {
				result.SelfLinks = append(result.SelfLinks, SelfLink{
					ItemID:   b.ID,
					LinkType: "blocked_by",
				})
			} else if _, ok := c.items[blocker]; !ok {
				result.BrokenLinks = append(result.BrokenLinks, BrokenLink{
					ItemID:   b.ID,
					LinkType: "blocked_by",
					Target:   blocker,
				})
			}
		}
	}

	// Check for cycles in blocking, blocked_by, and parent links
	for _, linkType := range []string{"blocking", "blocked_by", "parent"} {
		cycles := c.findCycles(linkType)
		result.Cycles = append(result.Cycles, cycles...)
	}

	return result
}

// findCycles detects all cycles for a specific link type using DFS.
func (c *Core) findCycles(linkType string) []Cycle {
	var cycles []Cycle
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	seenCycles := make(map[string]bool) // To avoid duplicate cycle reports

	var dfs func(id string, path []string)
	dfs = func(id string, path []string) {
		if inStack[id] {
			// Found a cycle - find where the cycle starts
			cycleStart := -1
			for i, p := range path {
				if p == id {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cyclePath := append(path[cycleStart:], id)
				// Create a canonical key to avoid duplicate cycles
				key := canonicalCycleKey(cyclePath)
				if !seenCycles[key] {
					seenCycles[key] = true
					cycles = append(cycles, Cycle{
						LinkType: linkType,
						Path:     cyclePath,
					})
				}
			}
			return
		}

		if visited[id] {
			return
		}

		visited[id] = true
		inStack[id] = true

		b, ok := c.items[id]
		if ok {
			// Get targets based on link type
			var targets []string
			switch linkType {
			case "parent":
				if b.Parent != "" {
					targets = []string{b.Parent}
				}
			case "blocking":
				targets = b.Blocking
			case "blocked_by":
				targets = b.BlockedBy
			}

			for _, target := range targets {
				// Skip self-references (they're tracked separately as SelfLinks)
				if target == id {
					continue
				}
				dfs(target, append(path, id))
			}
		}

		inStack[id] = false
	}

	for id := range c.items {
		if !visited[id] {
			dfs(id, nil)
		}
	}

	return cycles
}

// canonicalCycleKey creates a unique key for a cycle to detect duplicates.
// It normalizes the cycle by starting from the smallest ID.
func canonicalCycleKey(path []string) string {
	if len(path) <= 1 {
		return ""
	}

	// Remove the duplicate end element (cycle closes back)
	cycle := path[:len(path)-1]

	// Find the minimum element to use as start
	minIdx := 0
	for i, id := range cycle {
		if id < cycle[minIdx] {
			minIdx = i
		}
	}

	// Rotate to start from minimum
	key := ""
	for i := 0; i < len(cycle); i++ {
		idx := (minIdx + i) % len(cycle)
		if i > 0 {
			key += "->"
		}
		key += cycle[idx]
	}

	return key
}

// RemoveLinksTo removes all links pointing to the given target ID from all items.
// Returns the number of links removed.
func (c *Core) RemoveLinksTo(targetID string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	for _, b := range c.items {
		changed := false

		// Remove parent link
		if b.Parent == targetID {
			b.Parent = ""
			changed = true
			removed++
		}

		// Remove blocking links
		originalBlockingLen := len(b.Blocking)
		b.RemoveBlocking(targetID)
		if len(b.Blocking) < originalBlockingLen {
			changed = true
			removed += originalBlockingLen - len(b.Blocking)
		}

		// Remove blocked_by links
		originalBlockedByLen := len(b.BlockedBy)
		b.RemoveBlockedBy(targetID)
		if len(b.BlockedBy) < originalBlockedByLen {
			changed = true
			removed += originalBlockedByLen - len(b.BlockedBy)
		}

		if changed {
			c.invalidateIncomingLocked()
			if err := c.saveToDisk(b); err != nil {
				return removed, err
			}
		}
	}

	return removed, nil
}

// FixBrokenLinks removes all broken links (links to non-existent items) and self-references.
// Returns the number of issues fixed.
func (c *Core) FixBrokenLinks() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fixed := 0
	for _, b := range c.items {
		changed := false

		// Fix parent link
		if b.Parent != "" {
			// Remove self-reference or broken link
			if b.Parent == b.ID {
				b.Parent = ""
				changed = true
				fixed++
			} else if _, ok := c.items[b.Parent]; !ok {
				b.Parent = ""
				changed = true
				fixed++
			}
		}

		// Fix blocking links
		originalBlockingLen := len(b.Blocking)
		var newBlocking []string
		for _, blocked := range b.Blocking {
			// Skip self-references
			if blocked == b.ID {
				continue
			}
			// Skip broken links (target doesn't exist)
			if _, ok := c.items[blocked]; !ok {
				continue
			}
			newBlocking = append(newBlocking, blocked)
		}
		if len(newBlocking) < originalBlockingLen {
			b.Blocking = newBlocking
			changed = true
			fixed += originalBlockingLen - len(newBlocking)
		}

		// Fix blocked_by links
		originalBlockedByLen := len(b.BlockedBy)
		var newBlockedBy []string
		for _, blocker := range b.BlockedBy {
			// Skip self-references
			if blocker == b.ID {
				continue
			}
			// Skip broken links (target doesn't exist)
			if _, ok := c.items[blocker]; !ok {
				continue
			}
			newBlockedBy = append(newBlockedBy, blocker)
		}
		if len(newBlockedBy) < originalBlockedByLen {
			b.BlockedBy = newBlockedBy
			changed = true
			fixed += originalBlockedByLen - len(newBlockedBy)
		}

		if changed {
			c.invalidateIncomingLocked()
			if err := c.saveToDisk(b); err != nil {
				return fixed, err
			}
		}
	}

	return fixed, nil
}

// ValidParentTypes returns the type names an item of itemType may hang under:
// every type on a strictly lower rank, in rank order. Returns nil when no such
// type exists, which is how "cannot have a parent" is expressed.
func ValidParentTypes(cfg *config.Config, itemType string) []string {
	childRank := cfg.RankOf(itemType)
	var out []string
	for rank := 1; rank < childRank; rank++ {
		out = append(out, cfg.TypesAtRank(rank)...)
	}
	return out
}

// IsValidParentRank reports whether parentType may hold childType.
func IsValidParentRank(cfg *config.Config, childType, parentType string) bool {
	return cfg.RankOf(childType) > cfg.RankOf(parentType)
}

// ValidateParent checks if a parent is valid for the given item.
// Returns nil if valid, error otherwise.
func (c *Core) ValidateParent(b *item.Item, parentID string) error {
	if parentID == "" {
		return nil
	}

	parent, err := c.Get(parentID)
	if err != nil {
		return fmt.Errorf("parent item not found: %s", parentID)
	}

	if IsValidParentRank(c.config, b.Type, parent.Type) {
		return nil
	}

	validTypes := ValidParentTypes(c.config, b.Type)
	if len(validTypes) == 0 {
		return fmt.Errorf("%s items cannot have a parent", b.Type)
	}
	return fmt.Errorf("%s items can only have %s as parent, not %s",
		b.Type, joinWithOr(validTypes), parent.Type)
}

// joinWithOr joins strings with commas and "or" for the last element.
func joinWithOr(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
	}
}

// isResolvedStatus returns true if the status means the item is "done"
// (either completed or scrapped).
func isResolvedStatus(status string) bool {
	return status == "completed" || status == "scrapped"
}

// IsBlocked returns true if the item is blocked, either explicitly (direct
// blockers) or implicitly (an ancestor in the parent chain is blocked).
func (c *Core) IsBlocked(itemID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.findAllBlockersLocked(itemID)) > 0
}

// IsExplicitlyBlocked returns true if the item has direct active blockers
// (via blocked_by field or incoming blocking links).
func (c *Core) IsExplicitlyBlocked(itemID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.findActiveBlockersLocked(itemID, nil)) > 0
}

// IsImplicitlyBlocked returns true if an ancestor of the item (via the parent
// chain) has active blockers, making this item implicitly blocked.
func (c *Core) IsImplicitlyBlocked(itemID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	b, ok := c.items[itemID]
	if !ok || b.Parent == "" {
		return false
	}

	found := false
	c.walkParentChain(b.Parent, func(ancestor *item.Item) {
		if !found && len(c.findActiveBlockersLocked(ancestor.ID, nil)) > 0 {
			found = true
		}
	})
	return found
}

// FindActiveBlockers returns all items that are explicitly (directly) blocking
// the given item. A blocker is "active" if its status is NOT completed/scrapped.
// This includes blockers from both the blocked_by field and incoming blocking links.
func (c *Core) FindActiveBlockers(itemID string) []*item.Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return cloneItems(c.findActiveBlockersLocked(itemID, nil))
}

// FindAllBlockers returns all items blocking the given item, both explicitly
// (direct blockers) and implicitly (blockers on ancestors in the parent chain).
func (c *Core) FindAllBlockers(itemID string) []*item.Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return cloneItems(c.findAllBlockersLocked(itemID))
}

// findAllBlockersLocked finds all items blocking itemID, both explicitly
// (direct blockers) and implicitly (blockers on ancestors in the parent
// chain). Must be called with c.mu held.
func (c *Core) findAllBlockersLocked(itemID string) []*item.Item {
	seen := make(map[string]bool)
	var allBlockers []*item.Item

	c.walkParentChain(itemID, func(b *item.Item) {
		blockers := c.findActiveBlockersLocked(b.ID, seen)
		allBlockers = append(allBlockers, blockers...)
	})

	return allBlockers
}

// findActiveBlockersLocked finds direct active blockers without acquiring the lock.
// seen deduplicates results across repeated calls; pass nil to create a fresh map.
// Must be called with c.mu held.
func (c *Core) findActiveBlockersLocked(itemID string, seen map[string]bool) []*item.Item {
	b, ok := c.items[itemID]
	if !ok {
		return nil
	}

	if seen == nil {
		seen = make(map[string]bool)
	}
	var blockers []*item.Item

	// Check direct blocked_by field
	for _, blockerID := range b.BlockedBy {
		if blocker, ok := c.items[blockerID]; ok {
			if !isResolvedStatus(blocker.Status) && !seen[blockerID] {
				seen[blockerID] = true
				blockers = append(blockers, blocker)
			}
		}
	}

	// Check incoming blocking links (other items that have this item in their Blocking list)
	for _, other := range c.items {
		for _, blocked := range other.Blocking {
			if blocked == itemID && !isResolvedStatus(other.Status) && !seen[other.ID] {
				seen[other.ID] = true
				blockers = append(blockers, other)
			}
		}
	}

	return blockers
}

// walkParentChain calls visitor for the item and each ancestor in the parent
// chain, stopping at cycles or missing items. Must be called with c.mu held.
func (c *Core) walkParentChain(itemID string, visitor func(b *item.Item)) {
	seen := make(map[string]bool)
	current := itemID
	for current != "" && !seen[current] {
		seen[current] = true
		b, ok := c.items[current]
		if !ok {
			break
		}
		visitor(b)
		current = b.Parent
	}
}

// ImplicitStatus walks the parent chain and returns the terminal status
// (scrapped or completed) of the nearest terminal ancestor, along with
// that ancestor's ID. Returns empty strings if no terminal ancestor exists.
// The item's own status is not considered.
func (c *Core) ImplicitStatus(itemID string) (status, fromID string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.implicitStatusLocked(itemID)
}

// implicitStatusLocked walks the parent chain without acquiring the lock.
// Must be called with c.mu held (at least for reading).
func (c *Core) implicitStatusLocked(itemID string) (status, fromID string) {
	b, ok := c.items[itemID]
	if !ok {
		return "", ""
	}

	// Skip the item itself; walk ancestors only
	if b.Parent == "" {
		return "", ""
	}

	var result struct{ status, fromID string }
	c.walkParentChain(b.Parent, func(ancestor *item.Item) {
		if result.status == "" && isResolvedStatus(ancestor.Status) {
			result.status = ancestor.Status
			result.fromID = ancestor.ID
		}
	})

	return result.status, result.fromID
}
