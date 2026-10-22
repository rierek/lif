package data

import (
	"github.com/rierek/lif/pkg/item"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/rierek/lif/pkg/config"
)

// Index is an in-memory snapshot over a set of beans, built once per
// load/reload from Store.Items() output -- there is no incremental update
// path (YAGNI): a reload just builds a fresh Index.
//
// ByID and Children are exported for cheap read access. Consumers MUST NOT
// mutate the maps, their slices, or the underlying *Bean values -- this is
// not enforced by the type system, only by convention. All ordering must go
// through sortBeans; do not re-sort or append to Children entries directly.
type Index struct {
	// ByID looks up a bean by its short ID.
	ByID map[string]*item.Item
	// Children maps a parent ID to its direct children, sorted
	// Status -> Priority -> Type -> Title (see sortBeans).
	Children map[string][]*item.Item

	beans []*item.Item
}

// NewIndex builds an Index over beans. Callers must not mutate the slice
// (or its elements) afterward -- Index holds pointers into it.
func NewIndex(beans []*item.Item) *Index {
	idx := &Index{
		ByID:     make(map[string]*item.Item, len(beans)),
		Children: make(map[string][]*item.Item),
		beans:    make([]*item.Item, 0, len(beans)),
	}

	for _, b := range beans {
		idx.ByID[b.ID] = b
		idx.beans = append(idx.beans, b)

		if b.Parent != "" {
			idx.Children[b.Parent] = append(idx.Children[b.Parent], b)
		}
	}

	for parent := range idx.Children {
		sortBeans(idx.Children[parent])
	}

	return idx
}

// Roots returns top-level beans (no parent), optionally restricted to the
// given types, sorted Status -> Priority -> Type -> Title. Used by the tree
// view to seed milestones plus parentless epics/rest.
func (idx *Index) Roots(types ...string) []*item.Item {
	var allowed map[string]bool
	if len(types) > 0 {
		allowed = make(map[string]bool, len(types))
		for _, t := range types {
			allowed[t] = true
		}
	}

	var roots []*item.Item
	for _, b := range idx.beans {
		if b.Parent != "" {
			continue
		}
		if allowed != nil && !allowed[b.Type] {
			continue
		}
		roots = append(roots, b)
	}

	sortBeans(roots)
	return roots
}

// Backlog returns parentless beans that are neither milestone nor epic,
// with status todo or draft -- the "unscheduled work" view.
func (idx *Index) Backlog() []*item.Item {
	var backlog []*item.Item
	for _, b := range idx.beans {
		if b.Parent != "" {
			continue
		}
		if b.Type == "milestone" || b.Type == "epic" {
			continue
		}
		if b.Status != "todo" && b.Status != "draft" {
			continue
		}
		backlog = append(backlog, b)
	}

	sortBeans(backlog)
	return backlog
}

// WithTag returns all beans carrying tag, sorted Status -> Priority -> Type
// -> Title.
func (idx *Index) WithTag(tag string) []*item.Item {
	var tagged []*item.Item
	for _, b := range idx.beans {
		for _, t := range b.Tags {
			if t == tag {
				tagged = append(tagged, b)
				break
			}
		}
	}

	sortBeans(tagged)
	return tagged
}

// enums is the ORDERED status/type/priority single source: the project's
// configured lists (SetConfig), or the built-in lif defaults before any
// config is applied. The rank maps are DERIVED from the slices (position ==
// rank), not maintained as a second, independently authored order.
// box_filter_facets.go's buildFilterItems and the combined value menu
// (box_menu_value.go) consume the exported accessors (StatusValues/
// TypeValues/PriorityValues) instead of hard-coding values.
type enums struct {
	cfg                                 *config.Config
	statuses, types, priorities         []string
	statusOrder, typeOrder, priorityOrd map[string]int
}

func newEnums(cfg *config.Config, statuses, types, priorities []string) *enums {
	return &enums{cfg, statuses, types, priorities, rankMap(statuses), rankMap(types), rankMap(priorities)}
}

// cur is swapped atomically: SetConfig runs on the UI goroutine, sorting may
// run on command goroutines.
var cur atomic.Pointer[enums]

var (
	builtinStatuses   = []string{"in-progress", "todo", "draft", "completed", "scrapped"}
	builtinTypes      = []string{"milestone", "epic", "bug", "feature", "task"}
	builtinPriorities = []string{"critical", "high", "normal", "low", "deferred"}
)

func init() { cur.Store(newEnums(nil, builtinStatuses, builtinTypes, builtinPriorities)) }

// SetConfig makes the project's configured status, type and priority lists
// (and type ranks) the single source for menus, forms, sorting and parent
// candidates. nil restores the built-in defaults.
func SetConfig(cfg *config.Config) {
	if cfg == nil {
		cur.Store(newEnums(nil, builtinStatuses, builtinTypes, builtinPriorities))
		return
	}
	cur.Store(newEnums(cfg, cfg.StatusNames(), cfg.TypeNames(), cfg.PriorityNames()))
}

// ConfiguredColors returns the colour specs (tone name or hex) the project
// config sets explicitly, keyed by status/type/priority name: an entry counts
// when its colour differs from the built-in one. Nothing is returned for a
// repo without a config.
func ConfiguredColors() (status, typ, priority map[string]string) {
	cfg := cur.Load().cfg
	if cfg == nil {
		return nil, nil, nil
	}
	status, typ, priority = map[string]string{}, map[string]string{}, map[string]string{}
	builtin := map[string]string{}
	set := func(dst map[string]string, name, color string) {
		if color != "" && color != builtin[name] {
			dst[name] = color
		}
	}
	for _, v := range config.DefaultStatuses {
		builtin[v.Name] = v.Color
	}
	for _, v := range cfg.StatusList() {
		set(status, v.Name, v.Color)
	}
	clear(builtin)
	for _, v := range config.DefaultTypes {
		builtin[v.Name] = v.Color
	}
	for _, v := range cfg.TypeList() {
		set(typ, v.Name, v.Color)
	}
	clear(builtin)
	for _, v := range config.DefaultPriorities {
		builtin[v.Name] = v.Color
	}
	for _, v := range cfg.PriorityList() {
		set(priority, v.Name, v.Color)
	}
	return
}

// IsOpenStatus reports whether s counts as "open": every status that is not an
// archive status of the project config (completed and scrapped by default).
func IsOpenStatus(s string) bool { return !cur.Load().cfg.IsArchiveStatus(s) }

// DefaultStatus, DefaultType and DefaultPriority return the value the create
// form preselects: the project's configured default, else lif's usual
// default (todo, task, normal), else the first entry of the list.
func DefaultStatus() string {
	return pick(cur.Load().statuses, cur.Load().cfg.GetDefaultStatus(), "todo")
}
func DefaultType() string { return pick(cur.Load().types, cur.Load().cfg.GetDefaultType(), "task") }
func DefaultPriority() string {
	return pick(cur.Load().priorities, cur.Load().cfg.GetDefaultPriority(), "normal")
}

// pick returns the first of want, fallback that is in list, else list[0].
func pick(list []string, want, fallback string) string {
	for _, v := range []string{want, fallback} {
		for _, l := range list {
			if v != "" && l == v {
				return v
			}
		}
	}
	return list[0]
}

// rankMap builds a value->position lookup from an ordered slice (position ==
// rank).
func rankMap(vals []string) map[string]int {
	m := make(map[string]int, len(vals))
	for i, v := range vals {
		m[v] = i
	}
	return m
}

// StatusValues returns the status enum in tier order. A defensive copy --
// callers must not be able to mutate the shared slice through the returned
// value.
func StatusValues() []string { return append([]string(nil), cur.Load().statuses...) }

// TypeValues returns the type enum in tier order (defensive copy, see
// StatusValues).
func TypeValues() []string { return append([]string(nil), cur.Load().types...) }

// PriorityValues returns the priority enum in tier order (defensive copy,
// see StatusValues).
func PriorityValues() []string { return append([]string(nil), cur.Load().priorities...) }

// sortBeans sorts in place by Status -> Priority -> Type -> Title
// (case-insensitive), matching beans upstream ordering. This is the single
// place sort order is defined; every Index method reuses it. Empty
// priority is treated as "normal" (beans 0.4.2 default).
//
// Deliberate deviation: sort.SliceStable is used for a deterministic order
// on full ties (same status/priority/type/title), rather than sort.Slice.
// Upstream beans sorts with a non-stable sort -- this is not guaranteed to
// produce byte-for-byte parity with upstream ordering on ties.
func sortBeans(beans []*item.Item) {
	e := cur.Load()
	sort.SliceStable(beans, func(i, j int) bool {
		a, b := beans[i], beans[j]

		if ra, rb := rank(e.statusOrder, a.Status), rank(e.statusOrder, b.Status); ra != rb {
			return ra < rb
		}

		pa, pb := a.Priority, b.Priority
		if pa == "" {
			pa = "normal"
		}
		if pb == "" {
			pb = "normal"
		}
		if ra, rb := rank(e.priorityOrd, pa), rank(e.priorityOrd, pb); ra != rb {
			return ra < rb
		}

		if ra, rb := rank(e.typeOrder, a.Type), rank(e.typeOrder, b.Type); ra != rb {
			return ra < rb
		}

		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	})
}

// rank returns the sort position of val in order, or len(order) (sorts
// last) if val is not a recognized value.
func rank(order map[string]int, val string) int {
	if r, ok := order[val]; ok {
		return r
	}
	return len(order)
}

// SortBeans is the exported single-source sort (Status -> Priority -> Type ->
// Title) every consumer OUTSIDE this package must use for bean lists it
// itself assembles (e.g. resolved Blocking/BlockedBy IDs) -- I03 (bean
// bt-7jr8 T8-review): a second, ad-hoc tie-break in the tui package would
// violate the "single place sort order is defined" contract this file
// already asserts for its own callers.
func SortBeans(beans []*item.Item) { sortBeans(beans) }

// StatusRank exposes the status tier position (see statusOrder) for callers
// that need to compare two beans' status without a full SortBeans call
// (e.g. the Backlog sort-toggle, E2 Task 5).
func StatusRank(status string) int { return rank(cur.Load().statusOrder, status) }

// PriorityRank exposes the priority tier position, empty treated as "normal"
// (mirrors sortBeans' own empty-priority handling).
func PriorityRank(priority string) int {
	if priority == "" {
		priority = "normal"
	}
	return rank(cur.Load().priorityOrd, priority)
}
