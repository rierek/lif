package commands

import (
	"context"
	"strings"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/spf13/cobra"
)

var (
	startJSON bool
)

// startTargetStatus is the status start writes on success (RunE below) and
// the status startCmd's completion narrowing excludes as already-there, so
// the write path and the completion predicate can never drift onto two
// different literals (beans-j5so).
const startTargetStatus = "in-progress"

var startCmd = &cobra.Command{
	Use:   "start <id> [id...]",
	Short: "Mark one or more items as in-progress",
	Long: `Marks one or more existing items as in-progress.

A single item is displayed in full, as lif show would. Several items give one
confirmation line each, because n full bodies on one screen help nobody. Every
ID is resolved before the first item is written.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := newResolver()

		targets, code, err := resolveBatchTargets(ctx, resolver, args)
		if err != nil {
			return cmdError(startJSON, code, "%s", err)
		}

		// Validate the status
		if !cfg.IsValidStatus(startTargetStatus) {
			return cmdError(startJSON, output.ErrValidation, "invalid status: %s (must be %s)", startTargetStatus, strings.Join(cfg.StatusNames(), ", "))
		}

		// Build the update input
		status := startTargetStatus
		input := model.UpdateItemInput{
			Status: &status,
		}

		if err := preflightStatusPolicy(targets, status, nil); err != nil {
			return mutationError(startJSON, err)
		}

		// Apply the update
		done := make([]*item.Item, 0, len(targets))
		for _, target := range targets {
			b, err := resolver.UpdateItem(ctx, target.ID, input)
			if err != nil {
				return emitBatchFailure(startJSON, done, err)
			}
			done = append(done, b)
		}

		if len(done) == 1 {
			// Delegate to show command for display
			showJSON = startJSON
			return showCmd.RunE(showCmd, []string{done[0].ID})
		}

		return emitBatchSuccess(startJSON, done,
			func(b *item.Item) error { return output.SuccessSingle(b) },
			func(b *item.Item) string {
				return ui.Success.Render("Started ") + ui.ID.Render(b.ID) + " " + b.Title
			})
	},
}

func RegisterStartCmd(root *cobra.Command) {
	startCmd.Flags().BoolVar(&startJSON, "json", false, "Output as JSON")
	// start excludes archived items (already completed/scrapped) and items
	// already in-progress -- starting either again is not a valid action
	// (beans-j5so).
	startCmd.ValidArgsFunction = completionUnboundedFiltered(func(b *item.Item) bool {
		return !isArchivedStatus(b.Status) && b.Status != startTargetStatus
	})
	root.AddCommand(startCmd)
}
