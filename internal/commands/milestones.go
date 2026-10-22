package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
	"github.com/spf13/cobra"
)

var (
	milestonesJSON     bool
	milestonesAll      bool
	milestonesView     string
	milestonesMaxWidth int
	milestonesTags     bool
)

// milestoneEntry pairs a milestone item with its descendant progress
// counts, per the descendantProgress percent-complete convention.
type milestoneEntry struct {
	Item      *item.Item `json:"item"`
	Completed int        `json:"completed"`
	Total     int        `json:"total"`
}

// collectMilestones returns every item on rank 1 whose type takes part in the
// aggregate views, filtered by archive status unless all is set.
func collectMilestones(allItems []*item.Item, all bool) []*item.Item {
	var out []*item.Item
	for _, b := range allItems {
		if cfg.RankOf(b.Type) != 1 {
			continue
		}
		if !cfg.IsRoadmapType(b.Type) {
			continue
		}
		if !all && cfg.IsArchiveStatus(b.Status) {
			continue
		}
		out = append(out, b)
	}
	return out
}

// visibleChildrenIndex returns children with every hidden subtree (D15)
// filtered out, so a caller walking it -- e.g. descendantProgress -- cannot
// see into a hidden container or anything beneath it: a hidden epic nested
// under a visible milestone must not contribute to that milestone's progress
// count.
func visibleChildrenIndex(children map[string][]*item.Item, hidden map[string]bool) map[string][]*item.Item {
	out := make(map[string][]*item.Item, len(children))
	for parent, kids := range children {
		var visible []*item.Item
		for _, k := range kids {
			if !hidden[k.ID] {
				visible = append(visible, k)
			}
		}
		if len(visible) > 0 {
			out[parent] = visible
		}
	}
	return out
}

var milestonesCmd = &cobra.Command{
	Use:   "milestones",
	Short: "List milestones with descendant progress",
	Long:  `Lists all items on the top container rank, each annotated with how many of its descendants (via any number of parent levels, e.g. epics and their tasks) are completed. Completed and scrapped milestones are hidden by default; use --all to include them.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Flag validation comes before any data access and before every
		// early return: an empty result set or --json must not swallow a
		// typo in --view (beans-cfky).
		form, ok := ui.ParseForm(milestonesView)
		if !ok {
			return cmdError(milestonesJSON, output.ErrValidation,
				"unknown --view %q: expected table or tree", milestonesView)
		}
		ctx := context.Background()
		resolver := newResolver()

		allItems, err := resolver.Items(ctx, nil)
		if err != nil {
			return cmdError(milestonesJSON, output.ErrValidation, "querying items: %v", err)
		}

		milestones := collectMilestones(allItems, milestonesAll)

		item.SortByStatusPriorityAndType(milestones, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())

		idx := roadmap.ChildrenIndex(allItems)
		hidden := roadmap.HiddenSubtrees(cfg, allItems, idx)
		visibleIdx := visibleChildrenIndex(idx, hidden)

		entries := make([]milestoneEntry, 0, len(milestones))
		for _, m := range milestones {
			completed, total := descendantProgress(m.ID, visibleIdx)
			entries = append(entries, milestoneEntry{Item: m, Completed: completed, Total: total})
		}

		if milestonesJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		}

		if len(entries) == 0 {
			fmt.Println(ui.Muted.Render("No milestones found."))
			return nil
		}

		// Every row carries a Progress, even at 0/0 — that is what buys the
		// PROGRESS column from the shared engine without a separate opt-in.
		rows := make([]ui.Row, 0, len(entries))
		for _, e := range entries {
			rows = append(rows, ui.Row{
				Item:     e.Item,
				Progress: &ui.Progress{Done: e.Completed, Total: e.Total},
			})
		}

		width := resolveWidth(milestonesMaxWidth, cmd.Flags().Changed("max-width"), cfg)
		fmt.Print(ui.Render(rows, form, "Milestones", width, milestonesTags, cfg))
		return nil
	},
}

func RegisterMilestonesCmd(root *cobra.Command) {
	// Registration is idempotent: the command is a package-level singleton,
	// so a second Register call in the same process (tests build several
	// roots) would panic in pflag with "flag redefined". list.go carries the
	// same guard; without it here, adding a test that registers this command
	// alongside another one panics rather than failing.
	if milestonesCmd.Flags().Lookup("json") != nil {
		root.AddCommand(milestonesCmd)
		return
	}
	milestonesCmd.Flags().BoolVar(&milestonesJSON, "json", false, "Output as JSON")
	milestonesCmd.Flags().BoolVar(&milestonesAll, "all", false, "Include completed and scrapped milestones")
	milestonesCmd.Flags().StringVar(&milestonesView, "view", "table",
		"Arrangement: table (flat, sortable) or tree (nested)")
	milestonesCmd.Flags().IntVar(&milestonesMaxWidth, "max-width", 0,
		"Cap the rendered width; 0 disables the cap (default: display.max_width, else 110)")
	milestonesCmd.Flags().BoolVar(&milestonesTags, "tags", false, "Render each milestone's tags")
	root.AddCommand(milestonesCmd)
}
