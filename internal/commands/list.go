package commands

import (
	"context"
	"fmt"
	"sort"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/spf13/cobra"
)

var (
	listJSON        bool
	listSearch      string
	listStatus      []string
	listNoStatus    []string
	listType        []string
	listNoType      []string
	listPriority    []string
	listNoPriority  []string
	listTag         []string
	listNoTag       []string
	listHasParent   bool
	listNoParent    bool
	listParentID    string
	listHasBlocking bool
	listNoBlocking  bool
	listIsBlocked   bool
	listUnblocked   bool
	listReady       bool
	listQuiet       bool
	listSort        string
	listDesc        bool
	listFull        bool
	listWhere       []string
	listView        string
	listMaxWidth    int
	listTags        bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Args:    cobra.NoArgs,
	Aliases: []string{"ls"},
	Short:   "List all items",
	Long: `Lists all items in the .lif directory (or in the legacy .beans directory of a project that still uses it).

Search Syntax (--search/-S):
  The search flag supports Bleve query string syntax. There are no AND/OR/NOT
  keywords and no parentheses; use +term and -term:

  login          Exact term match
  login~         Fuzzy match (1 edit distance, finds "loggin", "logins")
  login~2        Fuzzy match (2 edit distance)
  log*           Wildcard prefix match
  "user login"   Exact phrase match
  user login     Either term matches (a list of terms is an OR)
  +user +login   Both terms required
  login -user    "login" without "user"
  slug:auth      Search only in slug field
  title:login    Search only in title field
  body:auth      Search only in body field`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Flag validation comes before any data access and before every
		// early return: an empty result set or --json must not swallow a
		// typo in --view (beans-cfky).
		form, ok := ui.ParseForm(listView)
		if !ok {
			return cmdError(listJSON, output.ErrValidation,
				"unknown --view %q: expected table or tree", listView)
		}
		// Validate --where up front (usage errors, reserved keys) before
		// running the query, analogous to validateExtraKeys in create.go/update.go.
		if err := validateWhereKeys(listWhere); err != nil {
			return err
		}

		// Build GraphQL filter from CLI flags
		filter := &model.ItemFilter{
			Status:          listStatus,
			ExcludeStatus:   listNoStatus,
			Type:            listType,
			ExcludeType:     listNoType,
			Priority:        listPriority,
			ExcludePriority: listNoPriority,
			Tags:            listTag,
			ExcludeTags:     listNoTag,
		}

		// Add search filter if provided
		if listSearch != "" {
			filter.Search = &listSearch
		}

		// Add parent/blocks filters
		if listHasParent {
			filter.HasParent = &listHasParent
		}
		if listNoParent {
			filter.NoParent = &listNoParent
		}
		if listParentID != "" {
			filter.ParentID = &listParentID
		}
		if listHasBlocking {
			filter.HasBlocking = &listHasBlocking
		}
		if listNoBlocking {
			filter.NoBlocking = &listNoBlocking
		}
		// --ready and --is-blocked are mutually exclusive
		if listReady && listIsBlocked {
			return fmt.Errorf("--ready and --is-blocked are mutually exclusive")
		}
		// --unblocked is the exact inverse of --is-blocked, so the two
		// can never be satisfied together.
		if listUnblocked && listIsBlocked {
			return fmt.Errorf("--unblocked and --is-blocked are mutually exclusive")
		}

		if listIsBlocked {
			filter.IsBlocked = &listIsBlocked
		}

		// --unblocked keeps only items with no unresolved blocker, direct or
		// inherited from an ancestor. Unlike --ready it says nothing about
		// status, so drafts and in-progress items survive it. It is applied
		// after --is-blocked, which the check above has already ruled out,
		// and before --ready, which asks for the same value.
		if listUnblocked {
			unblocked := false
			filter.IsBlocked = &unblocked
		}

		// --ready: leaf items available to start (not blocked, excludes in-progress/completed/scrapped/draft
		// and container types, and excludes items with implicit terminal status from a scrapped/completed ancestor)
		if listReady {
			applyReadyFilter(filter, cfg)
		}

		// Execute query via core resolver
		resolver := newResolver()
		items, err := resolver.Items(context.Background(), filter)
		if err != nil {
			return fmt.Errorf("querying items: %w", err)
		}

		// --where filters on extra front matter keys after the graph query,
		// since ItemFilter (a generated GraphQL type) does not carry them (AC1/AC2).
		items = filterByWhere(items, listWhere)

		// Sort items
		sortItems(items, listSort, listDesc, cfg)

		// JSON output (flat list)
		if listJSON {
			if !listFull {
				for _, b := range items {
					b.Body = ""
				}
			}
			return output.SuccessMultiple(items)
		}

		// Quiet mode: just IDs (flat)
		if listQuiet {
			for _, b := range items {
				fmt.Println(b.ID)
			}
			return nil
		}

		// Both forms need every item: the tree form to resolve ancestors for
		// context, the table form to compute implicit statuses.
		allItems, err := resolver.Items(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("querying all items for tree: %w", err)
		}

		// Pre-compute implicit statuses for all items
		implicitStatuses := make(map[string]string, len(allItems))
		for _, b := range allItems {
			if status, _ := core.ImplicitStatus(b.ID); status != "" {
				implicitStatuses[b.ID] = status
			}
		}

		// Create sort function for tree building
		sortFn := func(b []*item.Item) {
			sortItems(b, listSort, listDesc, cfg)
		}

		// Build tree
		tree := ui.BuildTree(items, allItems, sortFn, implicitStatuses)

		if len(tree) == 0 {
			fmt.Println(ui.Muted.Render("No items found. Create one with: lif create <title>"))
			return nil
		}

		width := resolveWidth(listMaxWidth, cmd.Flags().Changed("max-width"), cfg)

		// The row source differs by form, not just by what Render does with
		// it afterwards: BuildTree deliberately widens `tree` beyond `lif`
		// with unmatched ancestors for context (ui.TreeNode.Matched), which
		// is correct for --view tree but wrong for --view table — a filtered
		// table promising comparable peers must not grow rows the filter
		// rejected. RowsFromFlatItems(FlattenTree(tree)) would carry that
		// leak through even though Render's table path re-flattens depth to
		// 0, because flattening depth does not drop the extran items. FlatRows
		// on the already-filtered `lif` avoids both the leak and computing
		// tree depth Render would then discard.
		var rows []ui.Row
		if form == ui.FormTree {
			rows = ui.RowsFromFlatItems(ui.FlattenTree(tree))
		} else {
			rows = ui.FlatRows(items)
		}
		fmt.Print(ui.Render(rows, form, "Items", width, listTags, cfg))
		return nil
	},
}

// applyReadyFilter mutates filter in place to express "--ready": not
// blocked, excludes in-progress/draft plus every configured archive status
// (default: completed/scrapped, but any custom status marked `archive:
// true` is picked up too -- a hardcoded literal list would silently treat
// a future archive-flagged status, e.g. a project-defined "po-review", as
// still ready), excludes items with implicit terminal status from a
// scrapped/completed ancestor, and excludes every container type (a type on
// rank 1 through config.MaxContainerRank: milestone, epic and feature by
// default, whatever the configuration puts there otherwise), because ready
// work is leaf work. It mutates an already partially-built
// *model.ItemFilter (e.g. one that already carries --type/--status flags)
// rather than constructing a fresh one, so callers keep whatever filters
// they set beforehand.
func applyReadyFilter(filter *model.ItemFilter, cfg *config.Config) {
	isBlocked := false
	excludeImplicitTerminal := true
	filter.IsBlocked = &isBlocked
	filter.ExcludeStatus = append(filter.ExcludeStatus, "in-progress", "draft")
	for _, s := range cfg.StatusList() {
		if s.Archive {
			filter.ExcludeStatus = append(filter.ExcludeStatus, s.Name)
		}
	}
	filter.ExcludeType = append(filter.ExcludeType, cfg.ContainerTypeNames()...)
	filter.ExcludeImplicitTerminal = &excludeImplicitTerminal
}

// sortItems orders items in place. desc reverses the finished ordering, so
// every sort field gains a direction without each branch having to know
// about one. With sortBy == "order" the reversal also flips the parent
// groups the order branch builds, not just the items inside them.
func sortItems(items []*item.Item, sortBy string, desc bool, cfg *config.Config) {
	statusNames := cfg.StatusNames()
	priorityNames := cfg.PriorityNames()
	typeNames := cfg.TypeNames()

	switch sortBy {
	case "created":
		sort.Slice(items, func(i, j int) bool {
			if items[i].CreatedAt == nil && items[j].CreatedAt == nil {
				return items[i].ID < items[j].ID
			}
			if items[i].CreatedAt == nil {
				return false
			}
			if items[j].CreatedAt == nil {
				return true
			}
			return items[i].CreatedAt.After(*items[j].CreatedAt)
		})
	case "updated":
		sort.Slice(items, func(i, j int) bool {
			if items[i].UpdatedAt == nil && items[j].UpdatedAt == nil {
				return items[i].ID < items[j].ID
			}
			if items[i].UpdatedAt == nil {
				return false
			}
			if items[j].UpdatedAt == nil {
				return true
			}
			return items[i].UpdatedAt.After(*items[j].UpdatedAt)
		})
	case "status":
		// Build status order from configured statuses
		statusOrder := make(map[string]int)
		for i, s := range statusNames {
			statusOrder[s] = i
		}
		sort.Slice(items, func(i, j int) bool {
			oi, oj := statusOrder[items[i].Status], statusOrder[items[j].Status]
			if oi != oj {
				return oi < oj
			}
			return items[i].ID < items[j].ID
		})
	case "priority":
		// Build priority order from configured priorities
		priorityOrder := make(map[string]int)
		for i, p := range priorityNames {
			priorityOrder[p] = i
		}
		// Find normal priority index for items without priority
		normalIdx := len(priorityNames)
		for i, p := range priorityNames {
			if p == "normal" {
				normalIdx = i
				break
			}
		}
		sort.Slice(items, func(i, j int) bool {
			pi := normalIdx
			if items[i].Priority != "" {
				if order, ok := priorityOrder[items[i].Priority]; ok {
					pi = order
				}
			}
			pj := normalIdx
			if items[j].Priority != "" {
				if order, ok := priorityOrder[items[j].Priority]; ok {
					pj = order
				}
			}
			if pi != pj {
				return pi < pj
			}
			return items[i].ID < items[j].ID
		})
	case "id":
		sort.Slice(items, func(i, j int) bool {
			return items[i].ID < items[j].ID
		})
	case "order":
		// Order is a fractional index scoped per parent (R-12): two
		// siblings under different parents can carry the same Order value,
		// so sorting the flat list by Order alone would interleave
		// unrelated groups. Group items by Parent first — groups keep the
		// order in which their first member appears in the input, for
		// determinism — then sort each group with item.SortByOrder, which
		// places items without an Order value after every sibling that has
		// one (AC1/AC2).
		groupOrder := make([]string, 0)
		groups := make(map[string][]*item.Item)
		for _, b := range items {
			if _, ok := groups[b.Parent]; !ok {
				groupOrder = append(groupOrder, b.Parent)
			}
			groups[b.Parent] = append(groups[b.Parent], b)
		}
		result := make([]*item.Item, 0, len(items))
		for _, p := range groupOrder {
			group := groups[p]
			item.SortByOrder(group)
			result = append(result, group...)
		}
		copy(items, result)
	default:
		// Default: sort by status order, then priority, then type order, then title (same as TUI)
		item.SortByStatusPriorityAndType(items, statusNames, priorityNames, typeNames)
	}

	if desc {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
}

// validateWhereKeys checks every --where argument up front: each entry must
// carry "=" (a usage error otherwise, same shape as --set) and must not name
// a reserved schema field (AC3).
func validateWhereKeys(wheres []string) error {
	for _, w := range wheres {
		key, _, err := parseSetPair(w, "--where")
		if err != nil {
			return err
		}
		if err := checkReservedKey(key); err != nil {
			return err
		}
	}
	return nil
}

// filterByWhere returns the subset of items whose Extra map satisfies every
// key=value pair in wheres (AND semantics, AC1/AC2). A key carried by no item
// simply yields an empty result (AC4). Callers must run validateWhereKeys
// first; a malformed pair here is skipped rather than erroring, since
// filtering runs after the query and has no error path back to the caller.
func filterByWhere(items []*item.Item, wheres []string) []*item.Item {
	if len(wheres) == 0 {
		return items
	}

	type pair struct{ key, value string }
	pairs := make([]pair, 0, len(wheres))
	for _, w := range wheres {
		key, value, err := parseSetPair(w, "--where")
		if err != nil {
			continue
		}
		pairs = append(pairs, pair{key, value})
	}

	result := make([]*item.Item, 0, len(items))
	for _, b := range items {
		matches := true
		for _, p := range pairs {
			v, ok := b.Extra[p.key]
			if !ok || fmt.Sprintf("%v", v) != p.value {
				matches = false
				break
			}
		}
		if matches {
			result = append(result, b)
		}
	}
	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// RegisterListCmd adds the list command to root. Flag registration is
// idempotent (guarded by a Lookup check) because listCmd is a package-level
// singleton and tests that want a real cobra.Execute() (so --view's parsing
// and cmd.Flags().Changed("max-width") behave as they do for the CLI) need
// to register it into a throwaway root without panicking on a second flag
// definition — the same pattern RegisterOrderCmd uses.
func RegisterListCmd(root *cobra.Command) {
	if listCmd.Flags().Lookup("json") == nil {
		listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
		listCmd.Flags().StringVarP(&listSearch, "search", "S", "", "Full-text search in title and body")
		listCmd.Flags().StringSliceVarP(&listStatus, "status", "s", nil, "Filter by status (comma-separated or repeated)")
		listCmd.Flags().StringSliceVar(&listNoStatus, "no-status", nil, "Exclude by status (comma-separated or repeated)")
		listCmd.Flags().StringSliceVarP(&listType, "type", "t", nil, "Filter by type (comma-separated or repeated)")
		listCmd.Flags().StringSliceVar(&listNoType, "no-type", nil, "Exclude by type (comma-separated or repeated)")
		listCmd.Flags().StringSliceVarP(&listPriority, "priority", "p", nil, "Filter by priority (comma-separated or repeated)")
		listCmd.Flags().StringSliceVar(&listNoPriority, "no-priority", nil, "Exclude by priority (comma-separated or repeated)")
		listCmd.Flags().StringSliceVar(&listTag, "tag", nil, "Filter by tag (comma-separated or repeated, OR logic)")
		listCmd.Flags().StringSliceVar(&listNoTag, "no-tag", nil, "Exclude items with tag (comma-separated or repeated)")
		listCmd.Flags().BoolVar(&listHasParent, "has-parent", false, "Filter items with a parent")
		listCmd.Flags().BoolVar(&listNoParent, "no-parent", false, "Filter items without a parent")
		listCmd.Flags().StringVar(&listParentID, "parent", "", "Filter by parent ID")
		listCmd.Flags().BoolVar(&listHasBlocking, "has-blocking", false, "Filter items that are blocking others")
		listCmd.Flags().BoolVar(&listNoBlocking, "no-blocking", false, "Filter items that aren't blocking others")
		listCmd.Flags().BoolVar(&listIsBlocked, "is-blocked", false, "Filter items that are blocked by others")
		listCmd.Flags().BoolVar(&listUnblocked, "unblocked", false, "Filter items with no unresolved blocker")
		listCmd.Flags().BoolVar(&listReady, "ready", false, "Filter leaf items available to start (not blocked, excludes in-progress/completed/scrapped/draft and container types)")
		listCmd.Flags().BoolVarP(&listQuiet, "quiet", "q", false, "Only output IDs (one per line)")
		listCmd.Flags().StringVar(&listSort, "sort", "", "Sort by: created, updated, status, priority, id, order (order is scoped per parent) (default: status, manual order, priority, type, title)")
		listCmd.Flags().BoolVar(&listDesc, "desc", false, "Reverse the sort order")
		listCmd.Flags().BoolVar(&listFull, "full", false, "Include item body in JSON output")
		listCmd.Flags().StringArrayVar(&listWhere, "where", nil, "Filter by extra front matter key=value (can be repeated, AND logic)")
		listCmd.Flags().StringVar(&listView, "view", "table",
			"Arrangement: table (flat, sortable) or tree (nested)")
		listCmd.Flags().IntVar(&listMaxWidth, "max-width", 0,
			"Cap the rendered width; 0 disables the cap (default: display.max_width, else 110)")
		listCmd.Flags().BoolVar(&listTags, "tags", false, "Render each item's tags")
		// AC-01/AC-02/AC-03/AC-04: only the affirmative enum-valued filter
		// flags gain completion (beans-pkq3 Non-Goals) -- the boolean and
		// --no-* exclusion flags take no enum value. --parent uses
		// itemIDCandidates (completion.go), not candidates.ParentCandidates:
		// list --parent matches any existing item ID, not a type-eligible
		// new-parent set (beans-pkq3 Risks).
		_ = listCmd.RegisterFlagCompletionFunc("status", statusFlagCompletion)
		_ = listCmd.RegisterFlagCompletionFunc("type", typeFlagCompletion)
		_ = listCmd.RegisterFlagCompletionFunc("priority", priorityFlagCompletion)
		_ = listCmd.RegisterFlagCompletionFunc("tag", tagFlagCompletion)
		_ = listCmd.RegisterFlagCompletionFunc("parent", listParentFlagCompletion)
	}
	root.AddCommand(listCmd)
}

// listParentFlagCompletion offers list --parent candidates: every item ID
// in the store (completion.go's itemIDCandidates), because list --parent
// filters by matching an existing item's ID, unlike create/update --parent
// which choose among type-eligible new-parent candidates (beans-pkq3
// Risks).
func listParentFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return itemIDCandidates(), completionDirective
}
