package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// audienceAnnotationKey carries the R-07 audience marker in a command's
// Annotations map: whether a verb is meant for direct human/interactive use
// (audienceUserFacing) or is plumbing consumed by scripts, shell completion,
// or the CLI's own bootstrap (audiencePlumbing). It is the single source of
// truth an interactive caller (e.g. `lif pick`) queries through
// IsUserFacing — no second, driftable verb-name list may exist anywhere
// else in the codebase (SC-03).
const (
	audienceAnnotationKey = "beans.audience"
	audienceUserFacing    = "user-facing"
	audiencePlumbing      = "plumbing"
)

// bootstrapSkipAnnotationKey carries a distinct, independent marker from
// audienceAnnotationKey: whether a verb may run before PersistentPreRunE
// has loaded a core (bootstrapSkip), as opposed to every other verb, which
// requires a loaded store. It answers a different question than the
// audience marker -- "can this run without a store" vs. "should a human
// see this in a selection view" -- and the two sets differ: prime is
// bootstrap-skip but audience-user-facing, while path is
// audience-plumbing but requires a store. Folding one into the other would
// silently change either prime's or path's runtime behaviour
// (beans-bv90).
const (
	bootstrapSkipAnnotationKey = "beans.bootstrap-skip"
	bootstrapSkip              = "skip"
)

func markBootstrapSkip(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[bootstrapSkipAnnotationKey] = bootstrapSkip
}

// markBootstrapSkipTree marks cmd and all its sub-commands bootstrap-skip.
func markBootstrapSkipTree(cmd *cobra.Command) {
	markBootstrapSkip(cmd)
	for _, sub := range cmd.Commands() {
		markBootstrapSkipTree(sub)
	}
}

// IsBootstrapSkip reports whether cmd was declared to run without a loaded
// core, replacing the literal cmd.Name() == "init" || … comparison that
// used to live in PersistentPreRunE (beans-bv90).
func IsBootstrapSkip(cmd *cobra.Command) bool {
	return cmd.Annotations[bootstrapSkipAnnotationKey] == bootstrapSkip
}

func markPlumbing(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[audienceAnnotationKey] = audiencePlumbing
}

func markUserFacing(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[audienceAnnotationKey] = audienceUserFacing
}

// IsUserFacing reports whether cmd was classified as a verb meant for a
// human to type directly, as opposed to plumbing (AC1/AC4). Every command
// under the root carries the marker once RegisterCoreCommands returns, so a
// caller needs no verb-name literal of its own to filter by audience.
func IsUserFacing(cmd *cobra.Command) bool {
	return cmd.Annotations[audienceAnnotationKey] == audienceUserFacing
}

// RegisterCoreCommands adds all core CLI commands to the root command.
func RegisterCoreCommands(root *cobra.Command) {
	RegisterArchiveCmd(root)
	RegisterCheckCmd(root)
	RegisterCompleteCmd(root)
	RegisterCreateCmd(root)
	RegisterDeleteCmd(root)
	RegisterGraphCmd(root)
	RegisterGraphqlCmd(root)
	RegisterHooksCmd(root)
	markBootstrapSkipTree(hooksCmd)
	RegisterInitCmd(root)
	markPlumbing(initCmd)
	markBootstrapSkip(initCmd)
	RegisterListCmd(root)
	RegisterMilestonesCmd(root)
	RegisterNextCmd(root)
	RegisterOrderCmd(root)
	RegisterPickCmd(root)
	RegisterPathCmd(root)
	markPlumbing(pathCmd)
	RegisterPrimeCmd(root)
	markBootstrapSkip(primeCmd)
	RegisterProgressCmd(root)
	RegisterPromoteCmd(root)
	RegisterRenameCmd(root)
	RegisterRoadmapCmd(root)
	RegisterRunCmd(root)
	markBootstrapSkip(runCmd)
	RegisterScrapCmd(root)
	RegisterShowCmd(root)
	RegisterStartCmd(root)
	RegisterTagCmd(root)
	RegisterUpdateCmd(root)
	RegisterVersionCmd(root)
	markPlumbing(versionCmd)
	markBootstrapSkip(versionCmd)

	// Deprecated placeholders for commands that moved to separate binaries
	registerDeprecatedCmd(root, "serve", "lif-serve")

	// tui is a real command; like init it must run without a loaded store
	// (the Lobby outside a repo has none), so it skips the bootstrap.
	RegisterTuiCmd(root)
	markBootstrapSkip(tuiCmd)

	// Cobra's own help and completion commands are plumbing exactly like
	// init/path/version (AC2), but classifying them by name would itself
	// be the curated verb-name list SC-03 forbids — just spelled as two
	// `cmd.Name() ==` comparisons instead of a slice. Classify them by
	// origin instead: snapshot every command already on the root (the 24
	// core verbs plus the serve stub), materialise cobra's built-ins, then
	// mark plumbing whatever is new. "Added by cobra's own bootstrap,
	// not by an explicit Register*Cmd call above" is a structural fact,
	// not a name literal, and survives cobra renaming or adding a third
	// built-in.
	preBuiltin := make(map[*cobra.Command]bool, len(root.Commands()))
	for _, cmd := range root.Commands() {
		preBuiltin[cmd] = true
	}
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	for _, cmd := range root.Commands() {
		if !preBuiltin[cmd] {
			markPlumbing(cmd)
			// help and completion render text only; they need no store, and
			// the completion scripts' sub-commands inherit that (beans-mgx8).
			markBootstrapSkipTree(cmd)
		}
	}

	// Every verb this loop has not already marked plumbing above —
	// including the 24 real commands and the serve stub — is
	// user-facing per AC2's "every other registered verb" default.
	for _, cmd := range root.Commands() {
		if cmd.Annotations[audienceAnnotationKey] == "" {
			markUserFacing(cmd)
		}
	}
}

func registerDeprecatedCmd(root *cobra.Command, name, binary string) {
	root.AddCommand(&cobra.Command{
		Use:    name,
		Args:   cobra.NoArgs,
		Short:  fmt.Sprintf("(moved to %s)", binary),
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stderr, "The %q command has moved to a separate binary: %s\n", name, binary)
			fmt.Fprintf(os.Stderr, "Please install and use %q instead.\n", binary)
			os.Exit(1)
		},
	})
}
