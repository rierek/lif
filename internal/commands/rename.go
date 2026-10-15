package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

var (
	renameSlug   string
	renameNoSlug bool
	renameReslug bool
	renameSuffix string
	renamePrefix string
	renameDryRun bool
	renameYes    bool
	renameJSON   bool
)

var renameCmd = &cobra.Command{
	Use:   "rename [id] [new-id]",
	Short: "Rename an item slug, a single item ID (cascading refs), or the project-wide ID prefix",
	Long: `Renames items in one of three modes:

  lif rename <id> --slug "neuer-slug"   set the slug
  lif rename <id> --no-slug             clear the slug (filename becomes id.md)
  lif rename <id> --reslug              regenerate the slug from the title
  lif rename <id> <new-id>               full new ID (cascades refs)
  lif rename <id> --suffix k7x2          new suffix, keep the configured prefix
  lif rename --prefix "bew-"             project-wide ID-prefix rebrand (cascades refs)

--dry-run shows the plan without applying it (all modes). A prefix rebrand
additionally requires --yes (or an interactive y/N confirm) before applying,
and refuses outright while "lif-serve" is running or active worktrees
exist.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := renameFlags{
			slug: renameSlug, slugSet: cmd.Flags().Changed("slug"),
			noSlug: renameNoSlug, reslug: renameReslug,
			suffix: renameSuffix, suffixSet: cmd.Flags().Changed("suffix"),
			prefix: renamePrefix, prefixSet: cmd.Flags().Changed("prefix"),
		}

		plan, err := buildRenamePlan(core, args, flags)
		if err != nil {
			return cmdError(renameJSON, "VALIDATION_ERROR", "%v", err)
		}

		// With --json an applied rename prints only its result; the plan is
		// the document of --dry-run.
		if renameDryRun || !renameJSON {
			if err := renderRenamePlan(cmd, plan, renameJSON); err != nil {
				return err
			}
		}

		if renameDryRun {
			return nil
		}

		if plan.Mode == "prefix" && !renameYes {
			if !confirmRename(cmd, fmt.Sprintf("Rebrand %d item(s) to prefix %q?", len(plan.Changes), plan.NewPrefix)) {
				if !renameJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "aborted")
				}
				return nil
			}
		}

		if err := applyRenameWithGuards(core, plan); err != nil {
			return cmdError(renameJSON, "FILE_ERROR", "%v", err)
		}

		if renameJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(renameResult{
				Success: true,
				Mode:    plan.Mode,
				Changed: len(plan.Changes),
			})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Renamed %d item(s).\n", len(plan.Changes))
		return nil
	},
}

// renameResult is the --json shape of an applied rename.
type renameResult struct {
	// Success is always true: a failed rename prints the error envelope instead.
	Success bool `json:"success"`
	// Mode is "slug", "id" or "prefix".
	Mode string `json:"mode"`
	// Changed is the number of items the rename rewrote.
	Changed int `json:"changed"`
}

// RegisterRenameCmd registers the rename command.
func RegisterRenameCmd(root *cobra.Command) {
	renameCmd.Flags().StringVar(&renameSlug, "slug", "", "set the slug part of the filename")
	renameCmd.Flags().BoolVar(&renameNoSlug, "no-slug", false, "remove the slug (filename becomes id.md)")
	renameCmd.Flags().BoolVar(&renameReslug, "reslug", false, "regenerate the slug from the item title")
	renameCmd.Flags().StringVar(&renameSuffix, "suffix", "", "replace only the ID suffix, keeping the configured prefix")
	renameCmd.Flags().StringVar(&renamePrefix, "prefix", "", "project-wide: rebrand all item IDs to this prefix")
	renameCmd.Flags().BoolVar(&renameDryRun, "dry-run", false, "show the planned changes without applying them")
	renameCmd.Flags().BoolVar(&renameYes, "yes", false, "skip the confirmation prompt (prefix rebrand)")
	renameCmd.Flags().BoolVar(&renameJSON, "json", false, "output as JSON")
	renameCmd.ValidArgsFunction = completionUpTo(1)
	root.AddCommand(renameCmd)
}

// renameFlags is the parsed, cobra-independent representation of the
// rename command's flags. buildRenamePlan operates on this so mode
// dispatch is testable without a cobra execution harness.
type renameFlags struct {
	slug      string
	slugSet   bool
	noSlug    bool
	reslug    bool
	suffix    string
	suffixSet bool
	prefix    string
	prefixSet bool
}

// modeCount reports how many distinct rename modes the flags/args request.
// Exactly one is valid; buildRenamePlan rejects 0 and >1.
func (f renameFlags) modeCount(argCount int) int {
	n := 0
	if f.prefixSet {
		n++
	}
	if f.slugSet || f.noSlug || f.reslug {
		n++
	}
	if f.suffixSet {
		n++
	}
	if argCount == 2 { // <id> <new-id>
		n++
	}
	return n
}

// buildRenamePlan maps flags+args to exactly one itemcore.RenamePlan mode
// (slug/id/prefix), enforcing mutual exclusivity (I05) and refusing a
// --suffix on an ID that does not start with the configured prefix (I04)
// rather than fabricating a corrupt ID.
func buildRenamePlan(c *itemcore.Core, args []string, f renameFlags) (*itemcore.RenamePlan, error) {
	if f.modeCount(len(args)) > 1 {
		return nil, fmt.Errorf("conflicting options: choose exactly one of --slug/--no-slug/--reslug, <new-id>, --suffix, or --prefix")
	}

	if f.prefixSet {
		if len(args) > 0 {
			return nil, fmt.Errorf("--prefix takes no positional arguments")
		}
		return c.PlanRebrand(f.prefix)
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("provide an item id, or use --prefix")
	}
	id := args[0]

	switch {
	case f.noSlug:
		empty := ""
		return c.PlanRenameSlug(id, &empty, false)
	case f.reslug:
		return c.PlanRenameSlug(id, nil, true)
	case f.slugSet:
		s := f.slug
		return c.PlanRenameSlug(id, &s, false)
	}

	if f.suffixSet {
		prefix := c.Config().Items.Prefix
		if prefix != "" && !strings.HasPrefix(id, prefix) {
			return nil, fmt.Errorf("item %q does not start with the configured prefix %q; cannot apply --suffix", id, prefix)
		}
		return c.PlanRenameID(id, prefix+f.suffix)
	}

	if len(args) == 2 {
		return c.PlanRenameID(id, args[1])
	}

	return nil, fmt.Errorf("nothing to rename: pass a new id, --suffix, or a slug flag")
}

// applyRenameWithGuards executes plan via ApplyRename, but for Mode=="prefix"
// re-runs the D05 guards (checkServerNotRunning / checkNoActiveWorktrees)
// immediately beforehand.
//
// I01 (T07-review prelude): PlanRebrand already runs these guards, but only
// at plan time. Between building the plan and actually applying it — most
// notably while an interactive --yes confirm is waiting on the user — a
// lif-serve process or a worktree can appear, and without a re-check the
// cascade swap would proceed regardless (TOCTOU). Re-planning immediately
// before the apply call closes that window: PlanRebrand recomputes the plan
// (which re-invokes both guards as its first act) right before the mutating
// call, so any guard failure that appeared during the confirm-wait aborts
// the apply instead of racing past it. Slug/single-ID rename are not
// D05-guarded (the guards exist specifically for the prefix mode: a running
// server merges file-watcher updates for those non-cascading disk writes
// per the Worktree State Architecture, so no guard applies) and pass
// through unchanged.
func applyRenameWithGuards(c *itemcore.Core, plan *itemcore.RenamePlan) error {
	if plan.Mode == "prefix" {
		fresh, err := c.PlanRebrand(plan.NewPrefix)
		if err != nil {
			return fmt.Errorf("aborting rebrand: %w", err)
		}
		plan = fresh
	}
	return c.ApplyRename(plan)
}

// renderRenamePlan prints the plan as a human-readable summary, or as JSON
// with --json.
func renderRenamePlan(cmd *cobra.Command, plan *itemcore.RenamePlan, jsonOut bool) error {
	w := cmd.OutOrStdout()
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(plan)
	}
	fmt.Fprintf(w, "mode: %s\n", plan.Mode)
	for _, ch := range plan.Changes {
		if ch.OldID != ch.NewID {
			fmt.Fprintf(w, "  %s -> %s  (%s -> %s)\n", ch.OldID, ch.NewID, ch.OldPath, ch.NewPath)
		} else {
			fmt.Fprintf(w, "  %s  (%s -> %s)\n", ch.OldID, ch.OldPath, ch.NewPath)
		}
	}
	if len(plan.RefUpdates) > 0 {
		total := 0
		for _, n := range plan.RefUpdates {
			total += n
		}
		fmt.Fprintf(w, "ref updates: %d across %d item(s)\n", total, len(plan.RefUpdates))
	}
	for _, m := range plan.AttachmentMoves {
		fmt.Fprintf(w, "  attachments: %s -> %s\n", m.OldPath, m.NewPath)
	}
	if plan.ConfigWrite {
		fmt.Fprintf(w, "config: %s prefix -> %q\n", cfg.TargetFileName(cfg.ConfigDir()), plan.NewPrefix)
	}
	return nil
}

// confirmRename reads a y/N answer from the command's input stream.
func confirmRename(cmd *cobra.Command, prompt string) bool {
	in := cmd.InOrStdin()
	if in == nil {
		in = os.Stdin
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N] ", prompt)
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		return false
	}
	ans := strings.ToLower(strings.TrimSpace(sc.Text()))
	return ans == "y" || ans == "yes"
}
