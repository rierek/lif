package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xRiErOS/lif/internal/output"
	"github.com/xRiErOS/lif/internal/ui"
	"github.com/xRiErOS/lif/pkg/itemgraph"
	"github.com/xRiErOS/lif/pkg/itemgraph/model"
)

var (
	nextType   []string
	nextTag    []string
	nextParent string
	nextSort   string
	nextDesc   bool
	nextJSON   bool
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Show the highest-priority ready item",
	Long: `Finds the highest-priority item available to start (not blocked, not in-progress/completed/scrapped/draft) and displays it.

The --type, --tag, --parent and --sort flags mean the same as in ` + "`lif list`" + `, so a narrowed
query can be moved between the two commands unchanged.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := &itemgraph.CoreResolver{Core: core}

		// Same flag names and semantics as list, so that narrowing a query
		// does not require translating it between the two commands.
		filter := &model.ItemFilter{
			Type: nextType,
			Tags: nextTag,
		}
		if nextParent != "" {
			filter.ParentID = &nextParent
		}
		applyReadyFilter(filter, cfg)

		items, err := resolver.Items(ctx, filter)
		if err != nil {
			return cmdError(nextJSON, output.ErrValidation, "querying items: %v", err)
		}

		sortItems(items, nextSort, nextDesc, cfg)

		if len(items) == 0 {
			return reportNoReadyItems(describeNextFilters())
		}

		// Delegate to show command for display
		showJSON = nextJSON
		return showCmd.RunE(showCmd, []string{items[0].ID})
	},
}

// describeNextFilters renders the active narrowing flags as a human-readable
// list. It returns an empty string when next was called unfiltered, which is
// what keeps the unfiltered message byte-identical to earlier releases.
func describeNextFilters() string {
	var parts []string
	if len(nextType) > 0 {
		parts = append(parts, "--type "+strings.Join(nextType, ","))
	}
	if len(nextTag) > 0 {
		parts = append(parts, "--tag "+strings.Join(nextTag, ","))
	}
	if nextParent != "" {
		parts = append(parts, "--parent "+nextParent)
	}
	return strings.Join(parts, " ")
}

// reportNoReadyItems prints the empty-result message. With filters active it
// names them, so that "nothing is ready" stays distinguishable from "the
// filter was too narrow".
func reportNoReadyItems(filters string) error {
	if filters == "" {
		if nextJSON {
			return output.SuccessMessage("no ready items found")
		}
		fmt.Println(ui.Muted.Render("No ready items found. Check " + ui.ID.Render("lif list --is-blocked") + " or " + ui.ID.Render("lif list") + " for what's outstanding."))
		return nil
	}

	if nextJSON {
		return output.SuccessMessage("no ready items found matching " + filters)
	}
	fmt.Println(ui.Muted.Render("No ready items found matching "+ui.ID.Render(filters)+". Widen the filter, or check ") + ui.ID.Render("lif list "+filters) + ui.Muted.Render(" for what's outstanding."))
	return nil
}

func RegisterNextCmd(root *cobra.Command) {
	nextCmd.Flags().StringSliceVarP(&nextType, "type", "t", nil, "Filter by type (comma-separated or repeated, OR logic)")
	nextCmd.Flags().StringSliceVar(&nextTag, "tag", nil, "Filter by tag (comma-separated or repeated, OR logic)")
	nextCmd.Flags().StringVar(&nextParent, "parent", "", "Filter by parent ID")
	nextCmd.Flags().StringVar(&nextSort, "sort", "", "Sort by: created, updated, status, priority, id, order (order is scoped per parent, so pair it with --parent) (default: status, manual order, priority, type, title)")
	nextCmd.Flags().BoolVar(&nextDesc, "desc", false, "Reverse the sort order")
	nextCmd.Flags().BoolVar(&nextJSON, "json", false, "Output as JSON")
	// next's doc comment (:29) states --type/--tag/--parent "mean the same
	// as in `lif list`" -- --parent is therefore list's single-value
	// filter-by-existing-ID semantics (listParentFlagCompletion,
	// itemIDCandidates), not update's type-eligible-new-parent chooser
	// (candidates.ParentCandidates) -- beans-pkq3 Auflage 1.
	_ = nextCmd.RegisterFlagCompletionFunc("type", typeFlagCompletion)
	_ = nextCmd.RegisterFlagCompletionFunc("tag", tagFlagCompletion)
	_ = nextCmd.RegisterFlagCompletionFunc("parent", listParentFlagCompletion)
	root.AddCommand(nextCmd)
}
