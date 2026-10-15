package commands

import (
	"context"
	"strings"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/candidates"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	createStatus    string
	createType      string
	createPriority  string
	createBody      string
	createBodyFile  string
	createTag       []string
	createParent    string
	createBlocking  []string
	createBlockedBy []string
	createPrefix    string
	createSet       []string
	createUnset     []string
	createOrder     string
	createJSON      bool
)

var createCmd = &cobra.Command{
	Use:     "create [title]",
	Args:    cobra.ArbitraryArgs,
	Aliases: []string{"c", "new"},
	Short:   "Create a new item",
	Long: `Creates a new item (issue) with a generated ID and optional title.

With -i/--interactive a guided form asks for title, type, priority and parent
on the terminal, then offers an action menu (body, tags, blocking links,
status, create next). Flags and the lif.default_* settings pre-fill the form.
Without a terminal on stdin the command fails at once and never waits.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate inputs
		if createStatus != "" && !cfg.IsValidStatus(createStatus) {
			return cmdError(createJSON, output.ErrInvalidStatus, "invalid status: %s (must be %s)", createStatus, strings.Join(cfg.StatusNames(), ", "))
		}
		if createType != "" && !cfg.IsValidType(createType) {
			return cmdError(createJSON, output.ErrValidation, "invalid type: %s (must be %s)", createType, strings.Join(cfg.TypeNames(), ", "))
		}
		if createPriority != "" && !cfg.IsValidPriority(createPriority) {
			return cmdError(createJSON, output.ErrValidation, "invalid priority: %s (must be %s)", createPriority, strings.Join(cfg.PriorityNames(), ", "))
		}
		if err := validateExtraKeys(createSet, createUnset); err != nil {
			return cmdError(createJSON, output.ErrValidation, "%s", err)
		}
		normalizedSet, err := normalizeCommitSets(createSet)
		if err != nil {
			return cmdError(createJSON, output.ErrValidation, "%s", err)
		}
		createSet = normalizedSet
		if cmd.Flags().Changed("order") && !item.IsValidOrderKey(createOrder) {
			return cmdError(createJSON, output.ErrValidation, "invalid order value: %q (must be a non-empty base62 fractional index)", createOrder)
		}

		// Terminal guard: before anything reads stdin (--body - included).
		if createInteractive {
			if err := requireCreateTerminal(); err != nil {
				return cmdError(createJSON, output.ErrValidation, "%s", err)
			}
		}

		body, err := resolveContent(createBody, createBodyFile)
		if err != nil {
			return cmdError(createJSON, output.ErrFileError, "%s", err)
		}

		if createInteractive {
			// The form pre-fills Body from createBody; --body - and
			// --body-file are already resolved to content here.
			createBody, createBodyFile = body, ""
			return runCreateInteractive(cmd, args)
		}

		b, err := writeCreateDraft(cmd, createDraft{
			Title:     strings.Join(args, " "),
			Type:      createType,
			Priority:  createPriority,
			Status:    createStatus,
			Parent:    createParent,
			Body:      body,
			Tags:      createTag,
			Blocking:  createBlocking,
			BlockedBy: createBlockedBy,
		})
		if err != nil {
			return err
		}
		return printCreated(b)
	},
}

func RegisterCreateCmd(root *cobra.Command) {
	// Help text sources its allowed values from cfg's canonical accessors --
	// see RegisterUpdateCmd's identical comment (update.go) for why cfg may
	// be nil here and why that is safe (beans-pkq3 AC-02/AC-08, SC-01).
	statusNames := cfg.StatusNames()
	typeNames := cfg.TypeNames()
	priorityNames := cfg.PriorityNames()

	createCmd.Flags().StringVarP(&createStatus, "status", "s", "", "Initial status ("+strings.Join(statusNames, ", ")+")")
	createCmd.Flags().StringVarP(&createType, "type", "t", "", "Item type ("+strings.Join(typeNames, ", ")+")")
	createCmd.Flags().StringVarP(&createPriority, "priority", "p", "", "Priority level ("+strings.Join(priorityNames, ", ")+")")
	createCmd.Flags().StringVarP(&createBody, "body", "d", "", "Body content (use '-' to read from stdin)")
	createCmd.Flags().StringVar(&createBodyFile, "body-file", "", "Read body from file")
	createCmd.Flags().StringArrayVar(&createTag, "tag", nil, "Add tag (can be repeated)")
	createCmd.Flags().StringVar(&createParent, "parent", "", "Parent item ID")
	createCmd.Flags().StringArrayVar(&createBlocking, "blocking", nil, "ID of item this blocks (can be repeated)")
	createCmd.Flags().StringArrayVar(&createBlockedBy, "blocked-by", nil, "ID of item that blocks this one (can be repeated)")
	createCmd.Flags().StringVar(&createPrefix, "prefix", "", "Custom ID prefix (overrides config prefix)")
	createCmd.Flags().StringArrayVar(&createSet, "set", nil, "Set an extra front matter key as key=value (can be repeated)")
	createCmd.Flags().StringArrayVar(&createUnset, "unset", nil, "Remove an extra front matter key (can be repeated)")
	createCmd.Flags().StringVar(&createOrder, "order", "", "Explicit fractional-index order value")
	createCmd.Flags().BoolVar(&createJSON, "json", false, "Output as JSON")
	createCmd.Flags().BoolVarP(&createInteractive, "interactive", "i", false, "Open a guided form for title, type, priority and parent (needs a terminal)")
	createCmd.MarkFlagsMutuallyExclusive("body", "body-file")
	createCmd.ValidArgsFunction = createValidArgs
	_ = createCmd.RegisterFlagCompletionFunc("status", statusFlagCompletion)
	_ = createCmd.RegisterFlagCompletionFunc("type", typeFlagCompletion)
	_ = createCmd.RegisterFlagCompletionFunc("priority", priorityFlagCompletion)
	_ = createCmd.RegisterFlagCompletionFunc("tag", tagFlagCompletion)
	_ = createCmd.RegisterFlagCompletionFunc("parent", createParentFlagCompletion)
	_ = createCmd.RegisterFlagCompletionFunc("blocked-by", createBlockingLikeFlagCompletion)
	_ = createCmd.RegisterFlagCompletionFunc("blocking", createBlockingLikeFlagCompletion)
	createFlagNames = nil
	createCmd.Flags().VisitAll(func(f *pflag.Flag) {
		createFlagNames = append(createFlagNames, "--"+f.Name)
	})
	root.AddCommand(createCmd)
}

// createParentFlagCompletion offers create --parent candidates: items whose
// type is a valid parent for the type the new item will have (--type if
// already given, else cfg's configured default -- itemcore.ValidParentTypes
// via candidates.ParentCandidates, beans-v725). The item being created has
// no ID yet, so there is no self/descendant exclusion (itemIDs is nil,
// unlike updateParentFlagCompletion in update.go) -- beans-pkq3 AC-04.
func createParentFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	itemType := createType
	if itemType == "" {
		itemType = cfg.GetDefaultType()
	}
	resolver := &itemgraph.CoreResolver{Core: core}
	eligible, err := candidates.ParentCandidates(context.Background(), resolver, cfg, nil, []string{itemType})
	if err != nil {
		return nil, completionDirective
	}
	return itemFlagCandidates(eligible), completionDirective
}

// createBlockingLikeFlagCompletion serves create's --blocked-by and
// --blocking: every item in the store (candidates.BlockingCandidates,
// beans-v725), since the item being created has no ID yet to exclude --
// beans-pkq3 AC-04.
func createBlockingLikeFlagCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return blockingLikeFlagCompletion(nil)
}

// createFlagNames holds the names of create's own registered flags,
// captured once by RegisterCreateCmd right after it defines them (before
// cobra ever merges root's persistent flags -- --config, --lif-path,
// --help -- into createCmd's flag set). createHint reads this slice
// instead of re-deriving it at completion time, which is what keeps
// those unrelated global/help flags out of the hint (beans-u93j
// Integration point 1's "primary flags" are create's own, not root's).
var createFlagNames []string

// createHint names create's expected title and its primary flags for
// display as cobra ActiveHelp when the user presses TAB after
// `lif create ` with no argument yet (beans-u93j AC-01). It is derived
// from createFlagNames -- itself pulled live from createCmd's flag
// definitions via VisitAll, not hand-copied -- so a newly added flag
// shows up here automatically and a removed one can't linger (AC-03).
func createHint() string {
	return `provide a title, e.g. lif create "Fix the login bug" -- flags: ` + strings.Join(createFlagNames, ", ")
}

// createValidArgs is create's ValidArgsFunction. create's sole positional
// argument is free-form title text (completionNoFileComp's rationale
// applies: no item-ID or file-path meaning), so once any word of the title
// is already typed there is nothing further to suggest. With zero args --
// the `lif create ` + TAB case the PO asked for -- it surfaces createHint
// as ActiveHelp instead of staying silent. cobra.AppendActiveHelp encodes
// the hint as a "_activeHelp_ "-prefixed pseudo-candidate; the shipped zsh
// script (zsh_completions.go) renders such lines via `compadd -x`, zsh's
// display-only/non-inserting form, so pressing TAB shows the text but
// leaves the command line as typed (AC-02) -- unlike a candidate with an
// empty value, which that same script drops outright (only non-empty
// comps reach compadd). ShellCompDirectiveNoFileComp still blocks the
// filename fallback beans-12cb fixed.
func createValidArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return cobra.AppendActiveHelp(nil, createHint()), cobra.ShellCompDirectiveNoFileComp
}
