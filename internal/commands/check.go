package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/internal/worktree"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

// unknownTypeItems returns every item whose type the current configuration
// does not carry. A config edit or a profile switch can leave items behind
// that no longer match any known type.
func unknownTypeItems(allItems []*item.Item) []*item.Item {
	var out []*item.Item
	for _, b := range allItems {
		if b.Type == "" {
			continue
		}
		if !cfg.IsValidType(b.Type) {
			out = append(out, b)
		}
	}
	return out
}

// itemFieldIssues reports what a hand-edited Markdown file can get wrong and
// no command would have accepted: a parent of the wrong rank, an unknown
// status or priority, a tag that breaks the tag rules. A missing parent is
// left to the link check.
func itemFieldIssues(allItems []*item.Item) []string {
	byID := make(map[string]*item.Item, len(allItems))
	for _, b := range allItems {
		byID[b.ID] = b
	}
	var out []string
	for _, b := range allItems {
		if b.Status != "" && !cfg.IsValidStatus(b.Status) {
			out = append(out, fmt.Sprintf("item %s carries unknown status '%s'", b.ID, b.Status))
		}
		if !cfg.IsValidPriority(b.Priority) {
			out = append(out, fmt.Sprintf("item %s carries unknown priority '%s'", b.ID, b.Priority))
		}
		for _, tag := range b.Tags {
			if err := item.ValidateTag(tag); err != nil {
				out = append(out, fmt.Sprintf("item %s carries invalid tag '%s': %v", b.ID, tag, err))
			}
		}
		if p, ok := byID[b.Parent]; ok && cfg.IsValidType(b.Type) && cfg.IsValidType(p.Type) &&
			!itemcore.IsValidParentRank(cfg, b.Type, p.Type) {
			out = append(out, fmt.Sprintf("item %s (%s) cannot sit under parent %s (%s): parent must rank higher", b.ID, b.Type, p.ID, p.Type))
		}
	}
	return out
}

var (
	checkJSON   bool
	checkFix    bool
	checkStrict bool
)

type checkResult struct {
	Success           bool                      `json:"success"`
	ConfigErrors      []string                  `json:"config_errors"`
	ItemIssues        *itemcore.LinkCheckResult `json:"item_issues,omitempty"`
	Fixed             int                       `json:"fixed,omitempty"`
	PolicyWarnings    []string                  `json:"policy_warnings,omitempty"`
	AttachmentOrphans []string                  `json:"attachment_orphans,omitempty"`
	LegacyNames       []string                  `json:"legacy_names,omitempty"`
}

// legacyNameWarnings returns one warning per legacy store name in use, each
// naming the legacy name, the new name and the step that ends it. It reads
// the outcome of the name levels from res and never from the environment.
func legacyNameWarnings(res storeResolution, c *config.Config, storeRoot string) []string {
	var out []string
	file := filepath.Base(c.ConfigFile())
	if res.LegacyConfigFile {
		out = append(out, fmt.Sprintf("config file %s is a legacy name: run `git mv %s %s` in %s",
			file, config.LegacyConfigFileName, config.ConfigFileName, c.ConfigDir()))
	}
	if res.LegacySectionKey {
		out = append(out, fmt.Sprintf("section key %s: in %s is a legacy name: rename the key to %s:",
			config.LegacySectionKey, file, config.SectionKey))
	}
	if res.LegacyStoreDir {
		move := fmt.Sprintf("run `git mv %s %s` in %s", config.LegacyDefaultStoreDir, config.DefaultStoreDir, filepath.Dir(storeRoot))
		msg := fmt.Sprintf("store directory %s is a legacy name: %s", config.LegacyDefaultStoreDir, move)
		switch res.StoreDirSource {
		case storeSourceConfig:
			msg += fmt.Sprintf(", then change path: %s to path: %s in %s",
				c.Items.Path, filepath.Join(filepath.Dir(c.Items.Path), config.DefaultStoreDir), file)
			if res.LegacyConfigFile {
				msg += fmt.Sprintf(", and run `git mv %s %s`", config.LegacyConfigFileName, config.ConfigFileName)
			}
		case storeSourceFlag:
			msg += fmt.Sprintf(", then pass the new directory to --%s", storePathFlag)
		case storeSourceEnv:
			msg += fmt.Sprintf(", then point %s at the new directory", config.StoreEnvVar)
		}
		out = append(out, msg)
	}
	if res.LegacyEnv {
		out = append(out, fmt.Sprintf("environment variable %s is a legacy name: export %s instead",
			config.LegacyStoreEnvVar, config.StoreEnvVar))
	}
	return out
}

// legacyWorktreeWarnings returns the warnings for the legacy worktree root and
// for each worktree on a legacy branch of the repository at repoDir. It is
// read-only. Outside a git repository the branch scan finds nothing.
func legacyWorktreeWarnings(c *config.Config, repoDir string) []string {
	var out []string
	project := c.GetProjectName()
	if project == "" && c.ConfigDir() != "" {
		project = filepath.Base(c.ConfigDir())
	}
	if root, legacy, err := c.ResolveWorktreePath(project); err == nil && legacy {
		out = append(out, fmt.Sprintf("worktree root %s is a legacy name: remove the remaining worktrees, then delete %s; new worktrees go to ~/%s/worktrees/%s",
			root, root, config.HomeDir, filepath.Base(root)))
	}
	if wts, err := worktree.LegacyBranchWorktrees(repoDir); err == nil {
		for _, wt := range wts {
			id := strings.TrimPrefix(wt.Branch, config.LegacyWorktreeBranchPrefix)
			out = append(out, fmt.Sprintf("worktree %s is on legacy branch %s: finish or remove it, or run `git branch -m %s %s%s`",
				wt.Path, wt.Branch, wt.Branch, config.WorktreeBranchPrefix, id))
		}
	}
	return out
}

// attachmentOrphanIssues reports one line per attachment directory whose
// item no longer resolves. It exists because the carry-along invariant of
// an ID rename is otherwise only asserted: the measured failure was a green
// check over a directory left behind by a prefix rebrand. A store without
// attachments yields nothing, and a read error is reported rather than
// swallowed — a directory that cannot be read is exactly the case where a
// silent nil would hide the drift again.
func attachmentOrphanIssues(c *itemcore.Core) []string {
	orphans, err := c.OrphanAttachments()
	if err != nil {
		return []string{fmt.Sprintf("cannot read %s: %v", itemcore.AttachmentsDir, err)}
	}
	issues := make([]string, 0, len(orphans))
	for _, id := range orphans {
		issues = append(issues, fmt.Sprintf("%s/%s has no item %s", itemcore.AttachmentsDir, id, id))
	}
	return issues
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Args:  cobra.NoArgs,
	Short: "Validate configuration and item integrity",
	Long: `Checks configuration and item integrity, including:
- Configuration settings (colors, default status, type, priority and tags, ID prefix)
- Item types that the configuration still defines
- Item fields: unknown status or priority, invalid tags, parent of the wrong type rank
- Broken links (links to non-existent items)
- Self-references (items linking to themselves)
- Circular dependencies (cycles in blocks/parent relationships)

Use --fix to automatically remove broken links and self-references.
Note: Cycles cannot be auto-fixed and require manual intervention.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var configErrors []string
		var fixed int
		allItems := core.All()

		// === Configuration checks ===
		if !checkJSON {
			fmt.Println(ui.Bold.Render("Configuration"))
		}

		// 1. Check statuses are defined (always true since hardcoded)
		if !checkJSON {
			fmt.Printf("  %s Statuses defined (%d hardcoded)\n", ui.Success.Render("✓"), len(config.DefaultStatuses))
		}

		// 2. Check default_status exists in statuses (always true since hardcoded)
		if !checkJSON {
			fmt.Printf("  %s Default status '%s' exists\n", ui.Success.Render("✓"), cfg.GetDefaultStatus())
		}

		// 2b. Check default_type is a valid hardcoded type
		if cfg.GetDefaultType() != "" && !cfg.IsValidType(cfg.GetDefaultType()) {
			configErrors = append(configErrors, fmt.Sprintf("default_type '%s' is not a valid type", cfg.GetDefaultType()))
		} else if cfg.GetDefaultType() != "" {
			if !checkJSON {
				fmt.Printf("  %s Default type '%s' is valid\n", ui.Success.Render("✓"), cfg.GetDefaultType())
			}
		}

		// 2b-2. Check every item's type against the current configuration
		if stray := unknownTypeItems(allItems); len(stray) > 0 {
			for _, b := range stray {
				configErrors = append(configErrors,
					fmt.Sprintf("item %s carries type '%s', which the configuration does not define", b.ID, b.Type))
			}
		} else if !checkJSON {
			fmt.Printf("  %s Every item carries a known type\n", ui.Success.Render("✓"))
		}

		// 2b-3. Check status, priority, tags and parent rank of every item
		if issues := itemFieldIssues(allItems); len(issues) > 0 {
			configErrors = append(configErrors, issues...)
		} else if !checkJSON {
			fmt.Printf("  %s Status, priority, tags and hierarchy are valid\n", ui.Success.Render("✓"))
		}

		// 2c. Check agent.default_effort is a valid effort level
		if effort := cfg.GetDefaultEffort(); effort != "" && !config.IsValidEffortLevel(effort) {
			configErrors = append(configErrors, fmt.Sprintf("agent.default_effort '%s' is not valid (use low, medium, high, or max)", effort))
		} else if effort != "" {
			if !checkJSON {
				fmt.Printf("  %s Default effort '%s' is valid\n", ui.Success.Render("✓"), effort)
			}
		}

		// 2b. Check the configured theme names a bundled flavour. The
		// renderer deliberately falls back to the default on an unknown
		// name rather than stripping colour (see ui.SetTheme), so this is
		// the only place a typo in display.theme becomes visible.
		if cfg != nil && !ui.IsValidTheme(cfg.GetTheme()) {
			configErrors = append(configErrors,
				fmt.Sprintf("unknown display.theme '%s': expected latte, frappe, macchiato or mocha", cfg.GetTheme()))
		}

		// 2d. Check lif.default_priority and lif.default_tags (pre-fill for create -i)
		if p := cfg.GetDefaultPriority(); !cfg.IsValidPriority(p) {
			configErrors = append(configErrors, fmt.Sprintf("default_priority '%s' is not a known priority", p))
		}
		for _, tag := range cfg.GetDefaultTags() {
			if err := item.ValidateTag(tag); err != nil {
				configErrors = append(configErrors, fmt.Sprintf("default_tags entry '%s' is not valid: %v", tag, err))
			}
		}

		// 3. Check all status colours are valid. This must iterate the
		// MERGED list, not config.DefaultStatuses: the defaults are
		// compile-time constants this repository writes itself, so
		// validating them can only ever report a defect we shipped, never
		// one a user configured -- a guard that cannot fail. An empty Color
		// is not an invalid one; it means "no explicit colour, fall back to
		// the muted default" (see ui.ResolveColor), the same meaning it
		// carries in a StatusOverride. Only a non-empty value is checked
		// against the theme's tone names.
		for _, s := range cfg.StatusList() {
			if s.Color != "" && !ui.IsValidColor(s.Color) {
				configErrors = append(configErrors, fmt.Sprintf("invalid color '%s' for status '%s'", s.Color, s.Name))
			}
		}
		if !checkJSON {
			colorErrors := 0
			for _, e := range configErrors {
				if len(e) > 13 && e[:13] == "invalid color" {
					colorErrors++
				}
			}
			if colorErrors == 0 {
				fmt.Printf("  %s All status colors valid\n", ui.Success.Render("✓"))
			}
		}

		// 4. Check all type colours are valid, again over the merged list.
		// Same reasoning as the status loop above: "task" deliberately
		// carries an empty Color in the ranked type scale, and an empty
		// colour is "uncoloured", not "invalid".
		for _, t := range cfg.TypeList() {
			if t.Color != "" && !ui.IsValidColor(t.Color) {
				configErrors = append(configErrors, fmt.Sprintf("invalid color '%s' for type '%s'", t.Color, t.Name))
			}
		}
		if !checkJSON {
			typeColorErrors := 0
			for _, e := range configErrors {
				if len(e) > 13 && e[:13] == "invalid color" {
					typeColorErrors++
				}
			}
			if typeColorErrors == 0 {
				fmt.Printf("  %s All type colors valid\n", ui.Success.Render("✓"))
			}
		}

		// 4. Check prefix consistency (configuration vs on-disk)
		prefixError := core.ValidatePrefixConsistency()
		if prefixError != "" {
			configErrors = append(configErrors, "prefix consistency: "+prefixError)
		} else if !checkJSON {
			fmt.Printf("  %s Prefix consistency valid\n", ui.Success.Render("✓"))
		}

		// Print config errors in human-readable mode
		if !checkJSON {
			for _, e := range configErrors {
				fmt.Printf("  %s %s\n", ui.Danger.Render("✗"), e)
			}
		}

		// === Item link checks ===
		if !checkJSON {
			fmt.Println()
			fmt.Println(ui.Bold.Render("Item Links"))
		}

		linkResult := core.CheckAllLinks()

		// Handle --fix mode
		if checkFix && (len(linkResult.BrokenLinks) > 0 || len(linkResult.SelfLinks) > 0) {
			fixedCount, err := core.FixBrokenLinks()
			if err != nil {
				return fmt.Errorf("fixing broken links: %w", err)
			}
			fixed = fixedCount

			if !checkJSON {
				for _, bl := range linkResult.BrokenLinks {
					fmt.Printf("  %s %s: removed broken link %s:%s\n", ui.Success.Render("✓"), bl.ItemID, bl.LinkType, bl.Target)
				}
				for _, sl := range linkResult.SelfLinks {
					fmt.Printf("  %s %s: removed self-reference in %s link\n", ui.Success.Render("✓"), sl.ItemID, sl.LinkType)
				}
			}

			// Clear the fixed issues from the result
			linkResult.BrokenLinks = []itemcore.BrokenLink{}
			linkResult.SelfLinks = []itemcore.SelfLink{}
		} else if !checkJSON {
			// Report issues without fixing
			for _, bl := range linkResult.BrokenLinks {
				fmt.Printf("  %s %s: broken link %s:%s\n", ui.Danger.Render("✗"), bl.ItemID, bl.LinkType, bl.Target)
			}
			for _, sl := range linkResult.SelfLinks {
				fmt.Printf("  %s %s: self-reference in %s link\n", ui.Danger.Render("✗"), sl.ItemID, sl.LinkType)
			}
		}

		// Cycles cannot be auto-fixed
		if !checkJSON {
			for _, c := range linkResult.Cycles {
				if checkFix {
					fmt.Printf("  %s Cannot auto-fix cycle: %s (via %s)\n", ui.Warning.Render("!"), formatCycle(c.Path), c.LinkType)
				} else {
					fmt.Printf("  %s Circular dependency: %s (via %s)\n", ui.Danger.Render("✗"), formatCycle(c.Path), c.LinkType)
				}
			}
		}

		// Show success if no issues
		if !checkJSON && !linkResult.HasIssues() && fixed == 0 {
			fmt.Printf("  %s No link issues found\n", ui.Success.Render("✓"))
		}

		// === Attachments ===
		// Reported next to the link check because it is the same class of
		// defect: a reference that no longer resolves. Only emitted when a
		// store actually uses attachments, so every existing store's output
		// is unchanged.
		attachmentOrphans := attachmentOrphanIssues(core)
		if !checkJSON && len(attachmentOrphans) > 0 {
			fmt.Println()
			fmt.Println(ui.Bold.Render("Attachments"))
			for _, issue := range attachmentOrphans {
				fmt.Printf("  %s %s\n", ui.Danger.Render("✗"), issue)
			}
		}

		// === Policy checks ===
		var policyWarnings []string
		if len(cfg.Items.RequireFieldsOn) > 0 {
			if !checkJSON {
				fmt.Println()
				fmt.Println(ui.Bold.Render("Policy"))
			}

			commitField := cfg.GetCommitField()
			type commitRef struct {
				itemID string
				sha    string
			}
			var refs []commitRef
			var shas []string
			seenSha := make(map[string]bool)

			for _, b := range allItems {
				if fields := cfg.RequiredFieldsFor(b.Status); len(fields) > 0 {
					missing := itemcore.MissingRequiredFields(b, fields)
					if len(missing) > 0 {
						policyWarnings = append(policyWarnings, fmt.Sprintf("%s: status %q missing required field(s): %s", b.ID, b.Status, strings.Join(missing, ", ")))
					}
				}
				if v, ok := b.Extra[commitField]; ok {
					if sha, ok := v.(string); ok && sha != "" {
						refs = append(refs, commitRef{itemID: b.ID, sha: sha})
						if !seenSha[sha] {
							seenSha[sha] = true
							shas = append(shas, sha)
						}
					}
				}
			}

			if len(shas) > 0 {
				var exist map[string]bool
				ok := false
				if cwd, err := os.Getwd(); err == nil {
					exist, ok = gitutil.CommitsExist(cwd, shas)
				}
				if !ok {
					policyWarnings = append(policyWarnings, "commit verification skipped (not a git repository)")
				} else {
					for _, ref := range refs {
						if !exist[ref.sha] {
							policyWarnings = append(policyWarnings, fmt.Sprintf("%s: commit %s not found in this repository", ref.itemID, ref.sha))
						}
					}
				}
			}

			if !checkJSON {
				if len(policyWarnings) == 0 {
					fmt.Printf("  %s Field policy satisfied\n", ui.Success.Render("✓"))
				} else {
					for _, w := range policyWarnings {
						fmt.Printf("  %s %s\n", ui.Warning.Render("!"), w)
					}
				}
			}
		}

		// === Legacy names ===
		// Reported by check only (D06). They never count as issues: not in
		// totalIssues, with or without --strict.
		legacyNames := legacyNameWarnings(resolution, cfg, core.Root())
		legacyNames = append(legacyNames, legacyWorktreeWarnings(cfg, cfg.ConfigDir())...)
		if !checkJSON && len(legacyNames) > 0 {
			fmt.Println()
			fmt.Println(ui.Bold.Render("Legacy names"))
			for _, w := range legacyNames {
				fmt.Printf("  %s %s\n", ui.Warning.Render("!"), w)
			}
		}

		// === Summary ===
		totalIssues := len(configErrors) + linkResult.TotalIssues() + len(attachmentOrphans)
		if checkStrict {
			totalIssues += len(policyWarnings)
		}

		if checkJSON {
			result := checkResult{
				Success:           totalIssues == 0,
				ConfigErrors:      configErrors,
				ItemIssues:        linkResult,
				Fixed:             fixed,
				PolicyWarnings:    policyWarnings,
				AttachmentOrphans: attachmentOrphans,
				LegacyNames:       legacyNames,
			}
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Println()
			if totalIssues == 0 && fixed == 0 {
				fmt.Println(ui.Success.Render("All checks passed"))
			} else if totalIssues == 0 && fixed > 0 {
				fmt.Println(ui.Success.Render(fmt.Sprintf("Fixed %d issue(s)", fixed)))
			} else if fixed > 0 {
				// Some issues fixed, some remain (cycles)
				fmt.Println(ui.Warning.Render(fmt.Sprintf("Fixed %d issue(s), %d require manual intervention", fixed, totalIssues)))
			} else if totalIssues == 1 {
				fmt.Println(ui.Danger.Render("1 issue found"))
			} else {
				fmt.Println(ui.Danger.Render(fmt.Sprintf("%d issues found", totalIssues)))
			}
		}

		// Exit with error code if validation failed
		if totalIssues > 0 {
			os.Exit(1)
		}

		return nil
	},
}

func RegisterCheckCmd(root *cobra.Command) {
	checkCmd.Flags().BoolVar(&checkJSON, "json", false, "Output as JSON")
	checkCmd.Flags().BoolVar(&checkFix, "fix", false, "Automatically fix broken links and self-references")
	checkCmd.Flags().BoolVar(&checkStrict, "strict", false, "Count policy warnings as issues (exit 1)")
	root.AddCommand(checkCmd)
}
