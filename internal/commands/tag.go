package commands

import (
	"context"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/spf13/cobra"
)

var (
	tagAdd    []string
	tagRemove []string
	tagJSON   bool
)

var tagCmd = &cobra.Command{
	Use:   "tag <id> [id...]",
	Short: "Add or remove tags on one or more items",
	Long: `Adds and removes tags on one or more items in a single call.

Tags are merged, not replaced: --tag adds, --remove-tag takes away, and every
tag not named is left alone. At least one of the two is required. Every ID is
resolved before the first item is written.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := newResolver()

		if len(tagAdd) == 0 && len(tagRemove) == 0 {
			return cmdError(tagJSON, output.ErrValidation, "no tags given (use --tag or --remove-tag)")
		}

		targets, code, err := resolveBatchTargets(ctx, resolver, args)
		if err != nil {
			return cmdError(tagJSON, code, "%s", err)
		}

		// Built here rather than through buildUpdateInput: that helper reads
		// the update* globals, which would let a previous update in the same
		// process leak into a tag call.
		input := model.UpdateItemInput{
			AddTags:    tagAdd,
			RemoveTags: tagRemove,
		}

		done := make([]*item.Item, 0, len(targets))
		for _, target := range targets {
			b, err := resolver.UpdateItem(ctx, target.ID, input)
			if err != nil {
				return emitBatchFailure(tagJSON, done, err)
			}
			done = append(done, b)
		}

		return emitBatchSuccess(tagJSON, done,
			output.SuccessSingle,
			func(b *item.Item) string {
				return ui.Success.Render("Tagged ") + ui.ID.Render(b.ID) + " " + b.Title
			})
	},
}

func RegisterTagCmd(root *cobra.Command) {
	tagCmd.Flags().StringArrayVar(&tagAdd, "tag", nil, "Add tag (can be repeated)")
	tagCmd.Flags().StringArrayVar(&tagRemove, "remove-tag", nil, "Remove tag (can be repeated)")
	tagCmd.Flags().BoolVar(&tagJSON, "json", false, "Output as JSON")
	tagCmd.ValidArgsFunction = completionUnbounded
	_ = tagCmd.RegisterFlagCompletionFunc("tag", tagFlagCompletion)
	root.AddCommand(tagCmd)
}
