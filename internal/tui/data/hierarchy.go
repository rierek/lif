package data

import (
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// hierarchy.go — parent-assignment eligibility rules (E3 Task 3, bean
// bt-p1uz, design decision f, epic-E3-plan.md »Task 3«): validParentTypes
// mirrors beancore.ValidParentTypes (beans-src pkg/beancore/links.go:
// 446-459) -- the beans CLI enforces this server-side, the Parent-Picker
// (internal/tui/box_picker_parent.go) pre-filters so the PO is never offered
// a doomed choice. CollectDescendants/EligibleParents back the Picker's
// cycle exclusion: a new parent from the bean's own subtree would corrupt
// the Tree's depth-first flatten/render logic (view_browse_repo.go), so it
// (and the bean itself) are pre-filtered here rather than left to the CLI to
// reject after the fact.
//
// Scope note (design decision g, epic-E3-plan.md): this file is
// DELIBERATELY Parent-only. The Blocking-Picker has NO cycle exclusion at
// all (port-parity with beans-src blockingpicker.go, which also has none --
// a blocking cycle is a logical PO mistake, not a render hazard) and needs
// none of this machinery.

// validParentTypes returns the type names an item of beanType may hang under:
// every type on a strictly lower rank of the project's configured type table
// (the same rule the CLI enforces, itemcore.ValidParentTypes). nil means "cannot
// have a parent". An unknown type sits at the leaf rank.
func validParentTypes(beanType string) []string {
	return itemcore.ValidParentTypes(cur.Load().cfg, beanType)
}

// CollectDescendants returns every bean ID reachable below id, walking
// idx.Children (the parent->children map Index already maintains --
// beans-src parentpicker.go:233-257 rebuilds an equivalent map ad hoc from a
// flat bean list on every open; here it already exists as index.go's own
// Children field, so no second map is built) breadth-first with a visited
// set. A hand-edited frontmatter Parent cycle (bean A's parent is B, B's
// parent is A) must never hang this walk -- the visited guard mirrors
// beans-src's own queue de-dupe (`if !descendants[childID]`,
// parentpicker.go:250).
func CollectDescendants(idx *Index, id string) map[string]bool {
	descendants := map[string]bool{}
	if idx == nil {
		return descendants
	}
	queue := append([]*item.Item(nil), idx.Children[id]...)
	for len(queue) > 0 {
		b := queue[0]
		queue = queue[1:]
		if descendants[b.ID] {
			continue
		}
		descendants[b.ID] = true
		queue = append(queue, idx.Children[b.ID]...)
	}
	return descendants
}

// EligibleParents is the Parent-Picker's exported facade (design decision f,
// epic-E3-plan.md »Task 3«: "die TUI konsumiert nur die Fassade, Test der
// Regeln über sie"): every bean in idx that could legally become b's new
// parent -- EXCLUDING b itself, every descendant of b (CollectDescendants,
// the cycle guard), and every bean whose type is not in
// validParentTypes(b.Type). A nil validParentTypes (b.Type == "milestone")
// short-circuits to no candidates at all -- a milestone can never take a
// parent, so there is nothing to filter idx.beans against. Sorted via the
// single-source SortBeans (I03) -- callers must never re-sort.
func EligibleParents(idx *Index, b *item.Item) []*item.Item {
	if idx == nil || b == nil {
		return nil
	}
	allowed := validParentTypes(b.Type)
	if len(allowed) == 0 {
		return nil
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, t := range allowed {
		allowedSet[t] = true
	}
	descendants := CollectDescendants(idx, b.ID)

	var out []*item.Item
	for _, cand := range idx.beans {
		if cand.ID == b.ID {
			continue
		}
		if descendants[cand.ID] {
			continue
		}
		if !allowedSet[cand.Type] {
			continue
		}
		out = append(out, cand)
	}
	SortBeans(out)
	return out
}
