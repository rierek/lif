package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/candidates"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/spf13/cobra"
)

var (
	updateStatus          string
	updateType            string
	updatePriority        string
	updateTitle           string
	updateBody            string
	updateBodyFile        string
	updateBodyReplaceOld  string
	updateBodyReplaceNew  string
	updateBodyAppend      string
	updateParent          string
	updateRemoveParent    bool
	updateBlocking        []string
	updateRemoveBlocking  []string
	updateBlockedBy       []string
	updateRemoveBlockedBy []string
	updateTag             []string
	updateRemoveTag       []string
	updateSet             []string
	updateUnset           []string
	updateIfMatch         string
	updateJSON            bool
)

var updateCmd = &cobra.Command{
	Use:     "update <id> [id...]",
	Aliases: []string{"u"},
	Short:   "Update the properties of one or more items",
	Long: `Updates one or more properties of one or more existing items.

Every flag applies to every item named in the call. Every ID is resolved, and
--status's required fields, --parent's type and cycle rules, and the
--blocking/--blocked-by rules (no link to the item itself or to an unknown
item, no cycle) are checked for every item, before the first item is written.
--if-match carries one item's etag, so it accepts exactly one ID.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := newResolver()

		if err := validateExtraKeys(updateSet, updateUnset); err != nil {
			return cmdError(updateJSON, output.ErrValidation, "%s", err)
		}
		normalizedSet, err := normalizeCommitSets(updateSet)
		if err != nil {
			return cmdError(updateJSON, output.ErrValidation, "%s", err)
		}
		updateSet = normalizedSet

		if updateIfMatch != "" && len(args) > 1 {
			return cmdError(updateJSON, output.ErrValidation, "--if-match takes one item's etag, got %d ids", len(args))
		}

		targets, code, err := resolveBatchTargets(ctx, resolver, args)
		if err != nil {
			return cmdError(updateJSON, code, "%s", err)
		}

		input, changes, err := buildUpdateInput(cmd)
		if err != nil {
			return cmdError(updateJSON, output.ErrValidation, "%s", err)
		}

		// Extra front matter keys aren't part of buildUpdateInput's generated
		// UpdateBeanInput, but they still count as a change.
		if len(updateSet) > 0 || len(updateUnset) > 0 {
			changes = append(changes, "extra")
		}
		if len(changes) == 0 {
			return cmdError(updateJSON, output.ErrValidation,
				"no changes specified (use --status, --type, --priority, --title, --body, --parent, --blocking, --blocked-by, --tag, or their --remove-* variants)")
		}
		if updateIfMatch != "" {
			input.IfMatch = &updateIfMatch
		}

		setMap, err := extraSetMap(updateSet)
		if err != nil {
			return cmdError(updateJSON, output.ErrValidation, "%s", err)
		}

		// Rules that depend on the item are checked for every target before
		// the first write, so a violation of them leaves the store untouched.
		// A gate (hooks) answers per write: it can stop the loop below at any
		// target, and the targets before it stay written.
		if input.Status != nil {
			if err := preflightStatusPolicy(targets, *input.Status, setMap); err != nil {
				return mutationError(updateJSON, err)
			}
		}
		if err := preflightParent(resolver, targets, input); err != nil {
			return mutationError(updateJSON, err)
		}
		if err := preflightType(resolver, targets, input); err != nil {
			return mutationError(updateJSON, err)
		}
		if err := preflightLinks(resolver, targets, input); err != nil {
			return mutationError(updateJSON, err)
		}

		// Each item lands in a single UpdateItem mutation -- field updates,
		// body modifications, relationship changes, and extra front matter
		// keys all under one etag, so a status change and the extra fields a
		// status policy demands (e.g. commit) can't be split across writes.
		done := make([]*item.Item, 0, len(targets))
		for _, target := range targets {
			b, err := resolver.UpdateItem(ctx, target.ID, input, itemcore.WithExtraOps(setMap, updateUnset))
			if err != nil {
				return emitBatchFailure(updateJSON, done, err)
			}
			done = append(done, b)
		}

		// --json returns the resulting item directly (same schema as
		// `lif show --json`), not a {success,item,message} envelope
		// (beans-13ae); several IDs give an array.
		return emitBatchSuccess(updateJSON, done, output.SuccessSingle, func(b *item.Item) string {
			return ui.Success.Render("Updated ") + ui.ID.Render(b.ID) + " " + ui.Muted.Render(b.Path)
		})
	},
}

// buildUpdateInput constructs the GraphQL input from flags and returns which fields changed.
func buildUpdateInput(cmd *cobra.Command) (model.UpdateItemInput, []string, error) {
	var input model.UpdateItemInput
	var changes []string

	if cmd.Flags().Changed("status") {
		if !cfg.IsValidStatus(updateStatus) {
			return input, nil, fmt.Errorf("invalid status: %s (must be %s)", updateStatus, strings.Join(cfg.StatusNames(), ", "))
		}
		input.Status = &updateStatus
		changes = append(changes, "status")
	}

	if cmd.Flags().Changed("type") {
		if !cfg.IsValidType(updateType) {
			return input, nil, fmt.Errorf("invalid type: %s (must be %s)", updateType, strings.Join(cfg.TypeNames(), ", "))
		}
		input.Type = &updateType
		changes = append(changes, "type")
	}

	if cmd.Flags().Changed("priority") {
		if !cfg.IsValidPriority(updatePriority) {
			return input, nil, fmt.Errorf("invalid priority: %s (must be %s)", updatePriority, strings.Join(cfg.PriorityNames(), ", "))
		}
		input.Priority = &updatePriority
		changes = append(changes, "priority")
	}

	if cmd.Flags().Changed("title") {
		input.Title = &updateTitle
		changes = append(changes, "title")
	}

	// Handle body modifications
	if cmd.Flags().Changed("body") || cmd.Flags().Changed("body-file") {
		// Full body replacement
		body, err := resolveContent(updateBody, updateBodyFile)
		if err != nil {
			return input, nil, err
		}
		input.Body = &body
		changes = append(changes, "body")
	} else if cmd.Flags().Changed("body-replace-old") || cmd.Flags().Changed("body-append") {
		// Body modifications via bodyMod
		bodyMod := &model.BodyModification{}

		if cmd.Flags().Changed("body-replace-old") {
			// --body-replace-old requires --body-replace-new (enforced by MarkFlagsRequiredTogether)
			bodyMod.Replace = []*model.ReplaceOperation{
				{
					Old: item.UnescapeBody(updateBodyReplaceOld),
					New: item.UnescapeBody(updateBodyReplaceNew),
				},
			}
		}

		if cmd.Flags().Changed("body-append") {
			appendText, err := resolveAppendContent(updateBodyAppend)
			if err != nil {
				return input, nil, err
			}
			bodyMod.Append = &appendText
		}

		input.BodyMod = bodyMod
		changes = append(changes, "body")
	}

	// Handle tags using granular add/remove (consistent with relationships)
	if len(updateTag) > 0 {
		input.AddTags = updateTag
		changes = append(changes, "tags")
	}
	if len(updateRemoveTag) > 0 {
		input.RemoveTags = updateRemoveTag
		changes = append(changes, "tags")
	}

	// Handle parent relationship
	if cmd.Flags().Changed("parent") {
		input.Parent = &updateParent
		changes = append(changes, "parent")
	} else if updateRemoveParent {
		emptyParent := ""
		input.Parent = &emptyParent
		changes = append(changes, "parent")
	}

	// Handle blocking relationships
	if len(updateBlocking) > 0 {
		input.AddBlocking = updateBlocking
		changes = append(changes, "blocking")
	}
	if len(updateRemoveBlocking) > 0 {
		input.RemoveBlocking = updateRemoveBlocking
		changes = append(changes, "blocking")
	}

	// Handle blocked-by relationships
	if len(updateBlockedBy) > 0 {
		input.AddBlockedBy = updateBlockedBy
		changes = append(changes, "blocked-by")
	}
	if len(updateRemoveBlockedBy) > 0 {
		input.RemoveBlockedBy = updateRemoveBlockedBy
		changes = append(changes, "blocked-by")
	}

	return input, changes, nil
}

// preflightParent runs, for every target and before the first write, the
// parent check UpdateItem runs per item: a batch whose items differ in type
// can pass for one and fail for the next.
func preflightParent(resolver *itemgraph.CoreResolver, targets []*item.Item, input model.UpdateItemInput) error {
	if input.Parent == nil {
		return nil
	}
	for _, target := range targets {
		probe := *target
		if input.Type != nil {
			probe.Type = *input.Type
		}
		if err := resolver.ValidateAndSetParent(&probe, *input.Parent); err != nil {
			return fmt.Errorf("%s: %w", target.ID, err)
		}
	}
	return nil
}

// preflightLinks runs, for every target and before the first write, the checks
// UpdateItem runs on --blocking and --blocked-by: the link must not point at
// the item itself or at an item that does not exist, and must not close a
// cycle (beans-o8bs). What closes a cycle depends on the item, so a batch can
// pass for one target and fail for the next one, after the first was written.
func preflightLinks(resolver *itemgraph.CoreResolver, targets []*item.Item, input model.UpdateItemInput) error {
	if input.AddBlocking == nil && input.AddBlockedBy == nil {
		return nil
	}
	for _, target := range targets {
		// The probe is the item as UpdateItem has it when it checks the links:
		// with the new type and parent, which the cycle check counts.
		probe := target.Clone()
		if input.Type != nil {
			probe.Type = *input.Type
		}
		if input.Parent != nil {
			if err := resolver.ValidateAndSetParent(probe, *input.Parent); err != nil {
				return fmt.Errorf("%s: %w", target.ID, err)
			}
		}
		if err := resolver.ValidateAndAddBlocking(probe, input.AddBlocking); err != nil {
			return fmt.Errorf("%s: %w", target.ID, err)
		}
		resolver.RemoveBlockingRelationships(probe, input.RemoveBlocking)
		if err := resolver.ValidateAndAddBlockedBy(probe, input.AddBlockedBy); err != nil {
			return fmt.Errorf("%s: %w", target.ID, err)
		}
	}
	return nil
}

// preflightType runs, for every target and before the first write, the
// hierarchy check UpdateItem runs on a type change, so a batch cannot be
// left half-written by a violation on a later item.
func preflightType(resolver *itemgraph.CoreResolver, targets []*item.Item, input model.UpdateItemInput) error {
	if input.Type == nil {
		return nil
	}
	for _, target := range targets {
		if target.Type == *input.Type {
			continue
		}
		probe := *target
		probe.Type = *input.Type
		if input.Parent != nil {
			if err := resolver.ValidateAndSetParent(&probe, *input.Parent); err != nil {
				return fmt.Errorf("%s: %w", target.ID, err)
			}
		}
		if err := resolver.ValidateTypeChange(&probe); err != nil {
			return err
		}
	}
	return nil
}

// isConflictError returns true if the error is an ETag-related conflict error.
func isConflictError(err error) bool {
	var mismatchErr *itemcore.ETagMismatchError
	var requiredErr *itemcore.ETagRequiredError
	return errors.As(err, &mismatchErr) || errors.As(err, &requiredErr)
}

// mutationErrorCode maps a write-path error to the output error code that
// describes it. Split out of mutationError so that the batch verbs, which
// have to report a failure alongside the items already written, classify it
// exactly the same way. A gate that blocked the change is a policy violation,
// like a status policy that the item does not meet.
func mutationErrorCode(err error) string {
	if isConflictError(err) {
		return output.ErrConflict
	}
	var policyErr *itemcore.PolicyViolationError
	var blocked *hooks.BlockedError
	if errors.As(err, &policyErr) || errors.As(err, &blocked) {
		return output.ErrPolicy
	}
	var invalid *itemcore.ValidationError
	if errors.As(err, &invalid) {
		return output.ErrValidation
	}
	return output.ErrFileError
}

// mutationError returns a cmdError with the appropriate error code based on the error type.
func mutationError(jsonOutput bool, err error) error {
	return cmdError(jsonOutput, mutationErrorCode(err), "%s", err)
}

// refreshUpdateVocabularyHelp renders the status, type and priority flag texts
// from c; a nil c yields the built-in names.
func refreshUpdateVocabularyHelp(c *config.Config) {
	f := updateCmd.Flags()
	f.Lookup("status").Usage = "New status (" + strings.Join(c.StatusNames(), ", ") + ")"
	f.Lookup("type").Usage = "New type (" + strings.Join(c.TypeNames(), ", ") + ")"
	f.Lookup("priority").Usage = "New priority (" + strings.Join(c.PriorityNames(), ", ") + ", or empty to clear)"
}

func RegisterUpdateCmd(root *cobra.Command) {
	// Help text sources its allowed values from cfg's canonical accessors
	// (pkg/config/config.go StatusNames/TypeNames/PriorityNames), the same
	// read path the completion funcs below use -- no local copy of the name
	// lists (beans-pkq3 AC-02/AC-08, SC-01). cfg is nil here (Register* runs
	// before PersistentPreRunE loads it); StatusList/TypeList/PriorityList
	// are nil-receiver safe and fall back to DefaultStatuses/Types/Priorities.
	updateCmd.Flags().StringVarP(&updateStatus, "status", "s", "", "")
	updateCmd.Flags().StringVarP(&updateType, "type", "t", "", "")
	updateCmd.Flags().StringVarP(&updatePriority, "priority", "p", "", "")
	refreshUpdateVocabularyHelp(cfg)
	withConfiguredVocabulary(updateCmd, refreshUpdateVocabularyHelp)
	updateCmd.Flags().StringVar(&updateTitle, "title", "", "New title")
	updateCmd.Flags().StringVarP(&updateBody, "body", "d", "", "New body (use '-' to read from stdin)")
	updateCmd.Flags().StringVar(&updateBodyFile, "body-file", "", "Read body from file")
	updateCmd.Flags().StringVar(&updateBodyReplaceOld, "body-replace-old", "", "Text to find and replace (requires --body-replace-new)")
	updateCmd.Flags().StringVar(&updateBodyReplaceNew, "body-replace-new", "", "Replacement text (requires --body-replace-old)")
	updateCmd.Flags().StringVar(&updateBodyAppend, "body-append", "", "Text to append to body (use '-' for stdin)")
	updateCmd.Flags().StringVar(&updateParent, "parent", "", "Set parent item ID")
	updateCmd.Flags().BoolVar(&updateRemoveParent, "remove-parent", false, "Remove parent")
	updateCmd.Flags().StringArrayVar(&updateBlocking, "blocking", nil, "ID of item this blocks (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateRemoveBlocking, "remove-blocking", nil, "ID of item to unblock (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateBlockedBy, "blocked-by", nil, "ID of item that blocks this one (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateRemoveBlockedBy, "remove-blocked-by", nil, "ID of blocker item to remove (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateTag, "tag", nil, "Add tag (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateRemoveTag, "remove-tag", nil, "Remove tag (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateSet, "set", nil, "Set an extra front matter key as key=value (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateUnset, "unset", nil, "Remove an extra front matter key (can be repeated)")
	updateCmd.Flags().StringVar(&updateIfMatch, "if-match", "", "Only update if etag matches (optimistic locking); the item file is locked from the check to the write, so of two simultaneous updates with the same etag only one succeeds")
	updateCmd.MarkFlagsMutuallyExclusive("parent", "remove-parent")
	updateCmd.Flags().BoolVar(&updateJSON, "json", false, "Output as JSON")
	// body and body-file are mutually exclusive with body modifications
	updateCmd.MarkFlagsMutuallyExclusive("body", "body-file", "body-replace-old")
	updateCmd.MarkFlagsMutuallyExclusive("body", "body-file", "body-append")
	// body-replace-old and body-append can now be used together!
	updateCmd.MarkFlagsRequiredTogether("body-replace-old", "body-replace-new")
	updateCmd.ValidArgsFunction = completionUnbounded
	_ = updateCmd.RegisterFlagCompletionFunc("status", statusFlagCompletion)
	_ = updateCmd.RegisterFlagCompletionFunc("type", typeFlagCompletion)
	_ = updateCmd.RegisterFlagCompletionFunc("priority", priorityFlagCompletion)
	_ = updateCmd.RegisterFlagCompletionFunc("tag", tagFlagCompletion)
	_ = updateCmd.RegisterFlagCompletionFunc("parent", updateParentFlagCompletion)
	_ = updateCmd.RegisterFlagCompletionFunc("blocked-by", blockedByFlagCompletion)
	_ = updateCmd.RegisterFlagCompletionFunc("blocking", blockingFlagCompletion)
	root.AddCommand(updateCmd)
}

// statusFlagCompletion offers --status candidates for create/update/list:
// the merged status names from *config.Config's canonical accessor, never
// a second, independently maintained list (beans-pkq3 AC-02, SC-01).
func statusFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return cfg.StatusNames(), completionDirective
}

// typeFlagCompletion is statusFlagCompletion's --type counterpart.
func typeFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return cfg.TypeNames(), completionDirective
}

// priorityFlagCompletion is statusFlagCompletion's --priority counterpart.
func priorityFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return cfg.PriorityNames(), completionDirective
}

// tagFlagCompletion offers every tag already in use across the store,
// annotated with its usage count, sourced from pkg/candidates.TagCandidates
// (beans-v725) -- beans-pkq3 AC-03. Shared by create/update/list/tag's
// --tag.
func tagFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	resolver := newResolver()
	tags, err := candidates.TagCandidates(context.Background(), resolver)
	if err != nil {
		return nil, completionDirective
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, fmt.Sprintf("%s\t%d item(s)", t.Tag, t.Count))
	}
	return out, completionDirective
}

// itemFlagCandidates formats an already-filtered item list as
// shell-completion candidates ("id\ttype title"), mirroring completion.go's
// itemIDCandidates shape for a caller that holds its own pkg/candidates
// result rather than every item in the store.
func itemFlagCandidates(items []*item.Item) []string {
	out := make([]string, 0, len(items))
	for _, b := range items {
		out = append(out, b.ID+"\t"+b.Type+" "+b.Title)
	}
	return out
}

// updateParentFlagCompletion offers update --parent candidates: items whose
// type is a valid parent for every item update's positional arguments name,
// excluding them and their descendants (candidates.ParentCandidates,
// beans-v725) -- beans-pkq3 AC-04.
func updateParentFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 || core == nil {
		return nil, completionDirective
	}
	ids := make([]string, 0, len(args))
	types := make([]string, 0, len(args))
	for _, arg := range args {
		b, err := core.Get(arg)
		if err != nil {
			return nil, completionDirective
		}
		ids = append(ids, b.ID)
		types = append(types, b.Type)
	}
	resolver := newResolver()
	eligible, err := candidates.ParentCandidates(context.Background(), resolver, cfg, ids, types)
	if err != nil {
		return nil, completionDirective
	}
	return itemFlagCandidates(eligible), completionDirective
}

// blockedByFlagCompletion and blockingFlagCompletion both offer the same
// items: every other item in the store except the ones in the edited item's
// own subtree and on its parent chain (candidates.BlockingCandidates,
// beans-v725, beans-o8bs), because a link to those is refused.
func blockedByFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return blockingLikeFlagCompletion(args)
}

func blockingFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return blockingLikeFlagCompletion(args)
}

// blockingLikeFlagCompletion is blockedByFlagCompletion's and
// blockingFlagCompletion's shared body: args[0], when present, is the item
// being updated, excluded from its own candidate list.
func blockingLikeFlagCompletion(args []string) ([]string, cobra.ShellCompDirective) {
	var itemID string
	if len(args) > 0 {
		itemID = args[0]
	}
	resolver := newResolver()
	eligible, err := candidates.BlockingCandidates(context.Background(), resolver, cfg, itemID)
	if err != nil {
		return nil, completionDirective
	}
	return itemFlagCandidates(eligible), completionDirective
}
