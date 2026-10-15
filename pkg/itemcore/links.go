package itemcore

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
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

// Cycle represents a circular dependency in links. LinkType names the links
// the path runs over: "blocking" (blocking links, whichever field stores
// them), "parent" (parent links) or "blocking+parent" (both, see Note). The
// path reads in the direction of the links and closes on its first item.
type Cycle struct {
	LinkType string   `json:"link_type"`
	Path     []string `json:"path"`
}

// cycleBlockingParent is the LinkType of a cycle that needs a parent link and
// a blocking link to close.
const cycleBlockingParent = "blocking+parent"

// Note explains a cycle that runs over blocking and parent links. The path of
// such a cycle looks acyclic to a reader who does not know how the hierarchy
// takes part, so it needs a sentence; every other cycle needs none.
func (c Cycle) Note() string {
	if c.LinkType != cycleBlockingParent {
		return ""
	}
	return "a child is blocked while its parent is blocked and counts as done once its parent is done"
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

// The cycle checks walk one dependency graph. Its nodes are events rather
// than items, because the links say two different things about an item: when
// it can become ready, and when it is over. Every item has three events:
//
//	ready   the item can become ready: no open blocker of its own or of an ancestor
//	sealed  everything below the item has had its chance to become ready
//	done    the item is completed or scrapped
//
// An edge says that its first event has to come before its second one:
//
//	ready(X)  -> done(X)    an item is worked on before it is done
//	sealed(X) -> done(X)    and it is done once its subtree has been seen to
//	done(Z)   -> ready(X)   Z blocks X, whichever field stores the link
//	ready(P)  -> ready(C)   a child is blocked while its parent is blocked
//	ready(C)  -> sealed(P)  a child counts as done once its parent is done, so it has
//	sealed(C) -> sealed(P)  to get ready first, and so does everything below it
//
// A cycle is a set of items that wait for one another for ever: none of them
// can become ready. Next to a ring of blocking links the graph finds a blocker
// below the item it blocks (the blocked parent blocks its children, the
// blocker among them), a blocker above it (the child counts as done with its
// parent) and every longer chain made of these.

// depEvent is one of the three events of an item.
type depEvent uint8

const (
	evReady depEvent = iota
	evSealed
	evDone
)

// depNode is the event ev of the item id.
type depNode struct {
	id string
	ev depEvent
}

// compare orders nodes by item ID, then by event, so that every walk over the
// graph visits them in the same order.
func (a depNode) compare(b depNode) int {
	return cmp.Or(strings.Compare(a.id, b.id), cmp.Compare(a.ev, b.ev))
}

// edgeKind records which links an edge comes from. The edges inside one item
// come from none.
type edgeKind uint8

const (
	viaBlocking edgeKind = 1 << iota
	viaParent
)

// depEdge says that from has to come before to.
type depEdge struct {
	from, to depNode
	kind     edgeKind
}

// depGraph maps each node to the nodes that wait for it.
type depGraph map[depNode]map[depNode]edgeKind

// newItemID is the graph key of an item that has no ID yet.
const newItemID = "<new item>"

// itemKey returns the graph key of b: its ID, or newItemID while it has none.
func itemKey(b *item.Item) string {
	if b.ID == "" {
		return newItemID
	}
	return b.ID
}

// add records that from has to come before to. Links of another kind between
// the same two events merge into the one edge.
func (g depGraph) add(from, to depNode, kind edgeKind) {
	if g[from] == nil {
		g[from] = map[depNode]edgeKind{}
	}
	g[from][to] |= kind
}

// next returns the nodes that wait for n, in order.
func (g depGraph) next(n depNode) []depNode {
	return slices.SortedFunc(maps.Keys(g[n]), depNode.compare)
}

// dependencyGraphLocked builds the graph of the stored items. When pending is
// not nil it takes the place of the stored item with its ID, or joins the
// graph as a new item. A link to an item that does not exist, or from an item
// to itself, adds no edge: lif check reports both on their own, and neither
// one makes anything wait. Must be called with c.mu held.
func (c *Core) dependencyGraphLocked(pending *item.Item) depGraph {
	items := maps.Clone(c.items)
	if pending != nil {
		items[itemKey(pending)] = pending
	}

	g := depGraph{}
	for id, b := range items {
		ready, sealed, done := depNode{id, evReady}, depNode{id, evSealed}, depNode{id, evDone}
		g.add(ready, done, 0)
		g.add(sealed, done, 0)
		for _, blocked := range b.Blocking {
			if _, ok := items[blocked]; ok && blocked != id {
				g.add(done, depNode{blocked, evReady}, viaBlocking)
			}
		}
		for _, blocker := range b.BlockedBy {
			if _, ok := items[blocker]; ok && blocker != id {
				g.add(depNode{blocker, evDone}, ready, viaBlocking)
			}
		}
		if parent := b.Parent; parent != "" && parent != id {
			if _, ok := items[parent]; ok {
				g.add(depNode{parent, evReady}, ready, viaParent)
				g.add(ready, depNode{parent, evSealed}, viaParent)
				g.add(sealed, depNode{parent, evSealed}, viaParent)
			}
		}
	}
	return g
}

// path returns the shortest path from one node to another, both ends
// included, or nil when there is none.
func (g depGraph) path(from, to depNode) []depNode {
	prev := map[depNode]depNode{}
	seen := map[depNode]bool{from: true}
	for queue := []depNode{from}; len(queue) > 0; queue = queue[1:] {
		n := queue[0]
		if n == to {
			path := []depNode{n}
			for n != from {
				n = prev[n]
				path = append(path, n)
			}
			slices.Reverse(path)
			return path
		}
		for _, m := range g.next(n) {
			if !seen[m] {
				seen[m] = true
				prev[m] = n
				queue = append(queue, m)
			}
		}
	}
	return nil
}

// ringItems lists the items of a ring in order. The events of one item that
// follow each other show as a single step, and a ring that closes on the item
// it began with does not repeat it.
func ringItems(ring []depNode) []string {
	var ids []string
	for _, n := range ring {
		if len(ids) == 0 || ids[len(ids)-1] != n.id {
			ids = append(ids, n.id)
		}
	}
	if len(ids) > 1 && ids[0] == ids[len(ids)-1] {
		ids = ids[:len(ids)-1]
	}
	return ids
}

// repeatsAnItem reports whether a ring passes through an item more than once.
func repeatsAnItem(ring []depNode) bool {
	ids := ringItems(ring)
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}

// cycle turns a ring of nodes, whose last node has an edge back to the first,
// into a Cycle: the items in order, rotated to begin at start (the smallest ID
// when start is empty) and closed by repeating it.
func (g depGraph) cycle(ring []depNode, start string) Cycle {
	var kinds edgeKind
	for i, n := range ring {
		kinds |= g[n][ring[(i+1)%len(ring)]]
	}
	ids := ringItems(ring)

	if kinds == viaParent && ring[0].ev == evReady {
		// A loop of parent links closes twice in the graph: over the ready
		// events, which run from the parent to the child, and over the sealed
		// events, which run from the child to the parent. It reads like the
		// parent links do, from the child to the parent, whichever ring found
		// it, so that both rings give one and the same path.
		slices.Reverse(ids)
	}

	first := slices.Index(ids, start)
	if start == "" {
		first = slices.Index(ids, slices.Min(ids))
	}
	first = max(first, 0)
	path := slices.Concat(ids[first:], ids[:first], ids[first:first+1])

	linkType := cycleBlockingParent
	switch kinds {
	case viaBlocking:
		linkType = "blocking"
	case viaParent:
		linkType = "parent"
	}
	return Cycle{LinkType: linkType, Path: path}
}

// cycles returns the cycles of the graph. A depth-first walk finds the back
// edges, and each one stands for the shortest ring through it. The walk takes
// the nodes and the edges in order, and the result is sorted again by link type
// and path, so the same graph always gives the same cycles in the same order,
// each starting at its smallest ID.
//
// The ring is the shortest way back and not the walk's own stack, because the
// stack can wind through one item twice. The children of a blocked container
// take part in its ring (ready(P) -> ready(C) -> sealed(P) -> done(P)), so a
// walk that entered the ring through a child would report that detour: a ring
// that names the container twice. A ring that still repeats an item is left
// out. The shortest ring of a deadlock never repeats one: two events of the
// same item that are not next to each other have an edge or a shorter way
// between them (ready -> done, sealed -> done, ready(C) -> sealed(P)). So every
// deadlock keeps a report, and it is one that names each item once.
func (g depGraph) cycles() []Cycle {
	cycles := []Cycle{}
	const onStack, finished = 1, 2
	state := map[depNode]int{}
	var visit func(n depNode)
	visit = func(n depNode) {
		state[n] = onStack
		for _, m := range g.next(n) {
			switch state[m] {
			case 0:
				visit(m)
			case onStack:
				if ring := g.path(m, n); !repeatsAnItem(ring) {
					cycles = append(cycles, g.cycle(ring, ""))
				}
			}
		}
		state[n] = finished
	}
	for _, n := range slices.SortedFunc(maps.Keys(g), depNode.compare) {
		if state[n] == 0 {
			visit(n)
		}
	}

	// A loop of parent links is found twice, over its ready and over its sealed
	// events. cycle gives both the same path, and the second one is dropped here.
	slices.SortFunc(cycles, func(a, b Cycle) int {
		return cmp.Or(strings.Compare(a.LinkType, b.LinkType), slices.Compare(a.Path, b.Path))
	})
	return slices.CompactFunc(cycles, func(a, b Cycle) bool {
		return a.LinkType == b.LinkType && slices.Equal(a.Path, b.Path)
	})
}

// DetectCycle reports the cycle that adding a link to item b would close, or
// nil when it closes none. linkType says what the link is: "blocking" (b
// blocks targetID), "blocked_by" (targetID blocks b) or "parent" (targetID
// becomes the parent of b, replacing the one b has). b is the item as the
// caller is about to write it: the other links it carries count, the links it
// dropped do not, and it need not be stored yet (a new item has no ID). The
// path of the cycle starts at b.
//
// Only a cycle through the new link counts, so a store that already holds a
// cycle does not refuse links that have nothing to do with it.
func (c *Core) DetectCycle(b *item.Item, linkType, targetID string) *Cycle {
	if linkType != "blocking" && linkType != "blocked_by" && linkType != "parent" {
		return nil
	}
	if linkType == "parent" {
		// The graph is built without a parent for b, so that neither the old
		// parent nor the new one is in it; the new links are tried against it.
		unparented := *b
		unparented.Parent = ""
		b = &unparented
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	g := c.dependencyGraphLocked(b)

	id := itemKey(b)
	ready, sealed, done := depNode{id, evReady}, depNode{id, evSealed}, depNode{id, evDone}
	var added []depEdge
	switch linkType {
	case "blocking":
		added = []depEdge{{done, depNode{targetID, evReady}, viaBlocking}}
	case "blocked_by":
		added = []depEdge{{depNode{targetID, evDone}, ready, viaBlocking}}
	case "parent":
		added = []depEdge{
			{depNode{targetID, evReady}, ready, viaParent},
			{ready, depNode{targetID, evSealed}, viaParent},
			{sealed, depNode{targetID, evSealed}, viaParent},
		}
	}
	for _, e := range added {
		// The link closes a cycle when its second event already leads to its first.
		if path := g.path(e.to, e.from); path != nil {
			g.add(e.from, e.to, e.kind)
			cycle := g.cycle(path, id)
			return &cycle
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

	// Check for cycles in blocking and parent links, in one walk over the graph
	result.Cycles = append(result.Cycles, c.dependencyGraphLocked(nil).cycles()...)

	return result
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
