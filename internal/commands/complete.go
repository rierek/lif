package commands

import (
	"context"
	"os"
	"strings"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/spf13/cobra"
)

var (
	completeSummary string
	completeCommit  string
	completeSet     []string
	completeJSON    bool
)

var completeCmd = &cobra.Command{
	Use:   "complete <id> [id...]",
	Short: "Mark one or more items as completed",
	Long: `Marks one or more existing items as completed.

--summary, --commit and --set apply to every item named in the call. Every ID
is resolved and checked against the status policy before the first item is
written, so an unknown ID or a policy violation leaves the whole batch alone.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := newResolver()

		targets, code, err := resolveBatchTargets(ctx, resolver, args)
		if err != nil {
			return cmdError(completeJSON, code, "%s", err)
		}

		// Validate the status
		if !cfg.IsValidStatus("completed") {
			return cmdError(completeJSON, output.ErrValidation, "invalid status: completed (must be %s)", strings.Join(cfg.StatusNames(), ", "))
		}

		if err := validateExtraKeys(completeSet, nil); err != nil {
			return cmdError(completeJSON, output.ErrValidation, "%s", err)
		}
		normalizedSet, err := normalizeCommitSets(completeSet)
		if err != nil {
			return cmdError(completeJSON, output.ErrValidation, "%s", err)
		}
		setMap, err := extraSetMap(normalizedSet)
		if err != nil {
			return cmdError(completeJSON, output.ErrValidation, "%s", err)
		}

		// Resolve --commit once for the whole batch: ResolveCommit shells
		// out and reads the working directory, and a ref like HEAD could
		// otherwise resolve differently for items later in the loop.
		if completeCommit != "" {
			dir, err := os.Getwd()
			if err != nil {
				return cmdError(completeJSON, output.ErrValidation, "%s", err)
			}
			sha, err := gitutil.ResolveCommit(dir, completeCommit)
			if err != nil {
				return cmdError(completeJSON, output.ErrValidation, "%s", err)
			}
			// --commit and --set on the same field both yield the same
			// normalized SHA type; --commit wins as the more explicit flag.
			setMap[cfg.GetCommitField()] = sha
		}

		// Build the update input
		status := "completed"
		input := model.UpdateItemInput{
			Status: &status,
		}

		// Handle optional summary. Resolved once, before the loop: with "-"
		// this reads stdin, and stdin can only be drained once — inside the
		// loop every item after the first would get an empty section.
		if completeSummary != "" {
			appendText, err := resolveAppendContent(completeSummary)
			if err != nil {
				return cmdError(completeJSON, output.ErrValidation, "failed to resolve summary: %v", err)
			}
			summarySection := "## Summary of Changes\n\n" + appendText
			input.BodyMod = &model.BodyModification{
				Append: &summarySection,
			}
		}

		if err := preflightStatusPolicy(targets, status, setMap); err != nil {
			return mutationError(completeJSON, err)
		}

		// Apply the update
		done := make([]*item.Item, 0, len(targets))
		for _, target := range targets {
			b, err := resolver.UpdateItem(ctx, target.ID, input, itemcore.WithExtraOps(setMap, nil))
			if err != nil {
				return emitBatchFailure(completeJSON, done, err)
			}
			done = append(done, b)
		}

		return emitBatchSuccess(completeJSON, done,
			func(b *item.Item) error { return output.SuccessSingle(b) },
			func(b *item.Item) string {
				return ui.Success.Render("Completed ") + ui.ID.Render(b.ID) + " " + b.Title
			})
	},
}

func RegisterCompleteCmd(root *cobra.Command) {
	completeCmd.Flags().StringVar(&completeSummary, "summary", "", "Optional summary of changes, applied to every item in the call")
	completeCmd.Flags().StringVar(&completeCommit, "commit", "", "Git ref (HEAD, branch, tag, SHA) recorded in the configured commit field of every item in the call")
	completeCmd.Flags().StringArrayVar(&completeSet, "set", nil, "Set an extra front matter key as key=value on every item in the call (can be repeated)")
	completeCmd.Flags().BoolVar(&completeJSON, "json", false, "Output as JSON")
	// complete excludes archived items (already completed/scrapped) --
	// completing either again is not a valid action (beans-j5so).
	completeCmd.ValidArgsFunction = completionUnboundedFiltered(func(b *item.Item) bool {
		return !isArchivedStatus(b.Status)
	})
	root.AddCommand(completeCmd)
}
