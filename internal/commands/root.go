package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/internal/version"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var core *itemcore.Core
var cfg *config.Config

// storePathFlag is the persistent root flag that names the store directory.
const storePathFlag = "lif-path"

var storePath string
var configPath string

// NewRootCmd creates the root cobra command with shared persistent flags
// and core initialization logic.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "lif",
		Short: "A file-based issue tracker for AI-first workflows",
		// `lif --version` prints exactly what `lif version` prints (beans-3slb).
		Version: version.String(),
		// Reporting a failure is done once, in reportExecutionError. Cobra
		// otherwise prints the error itself and follows it with the whole
		// usage block — measured at 33 lines behind a one-line failure, and
		// identical for a runtime outcome and a typo'd flag. Both switches
		// are conjunctions over command and root, so setting them here
		// covers every subcommand of all three binaries.
		SilenceErrors: true,
		SilenceUsage:  true,
		Long: `Lif is a lightweight issue tracker that stores issues as markdown files.
Track your work alongside your code and supercharge your coding agent with
a full view of your project.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip core initialization for commands declared bootstrap-skip
			// (init, prime, version) via a structural per-command marker,
			// not a cmd.Name() literal comparison (beans-bv90).
			if IsBootstrapSkip(cmd) {
				return nil
			}

			var err error

			effectiveStorePath, effectiveConfigPath := completionAwareRootFlags(cmd, args)

			// Load configuration
			if effectiveConfigPath != "" {
				cfg, err = config.Load(effectiveConfigPath)
				if err != nil {
					return fmt.Errorf("loading config from %s: %w", effectiveConfigPath, err)
				}
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("getting current directory: %w", err)
				}
				cfg, err = config.LoadFromDirectory(cwd)
				if err != nil {
					return fmt.Errorf("loading config: %w", err)
				}
			}

			// The theme is process wide and must be in force before anything
			// renders, so this is the single place that applies it.
			ui.SetTheme(cfg.GetTheme())

			feedTypeTables(cfg)
			feedStatusPriorityTables(cfg)

			root, res, err := resolveStorePath(effectiveStorePath, cfg)
			if err != nil {
				var missing *missingStoreError
				// Completion runs in any directory: without a store it offers
				// the candidates that need none (beans-mgx8). An explicit
				// --lif-path that does not resolve still fails, so completion
				// never silently answers for another store.
				if errors.As(err, &missing) && cmd.Name() == cobra.ShellCompRequestCmd && effectiveStorePath == "" {
					core = nil
					return nil
				}
				if f := cmd.Flags().Lookup("json"); f != nil && f.Value.String() == "true" && errors.As(err, &missing) {
					return output.ErrorFrom(output.ErrNoLifDir, err)
				}
				return err
			}
			resolution = res

			core = itemcore.New(root, cfg)
			if err := core.Load(); err != nil {
				return fmt.Errorf("loading items: %w", err)
			}

			return nil
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.PersistentFlags().StringVar(&storePath, storePathFlag, "", "Path to data directory (overrides config and LIF_PATH env var; a legacy BEANS_PATH is read when LIF_PATH is unset)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: searches upward for .lif.yml; a legacy .beans.yml is read when no .lif.yml exists)")

	return rootCmd
}

// feedTypeTables fills internal/ui's process-wide type-short and
// type-column-width tables from the merged type list. internal/ui must not
// import pkg/config, so this is the one place - analogous to a SetTheme
// call - where the config-derived values cross into it.
func feedTypeTables(c *config.Config) {
	shorts := make(map[string]string)
	longest := 0
	for _, t := range c.TypeList() {
		shorts[t.Name] = c.ShortOf(t.Name)
		// Cells, not bytes: this value becomes a column width, and a type
		// named "Änderung" is 8 cells in 9 bytes.
		if w := ui.DisplayWidth(t.Name); w > longest {
			longest = w
		}
	}
	ui.SetTypeShorts(shorts)
	// One trailing space keeps the column from touching the next one, which is
	// what the old literal 10 did for "milestone" at nine characters.
	ui.SetTypeColumnWidths(3, longest+1)
}

// feedStatusPriorityTables fills internal/ui's process-wide status-short and
// priority-symbol tables from the merged status/priority lists. Same
// reasoning as feedTypeTables: internal/ui must not import pkg/config, so
// this is where the config-derived values cross into it.
func feedStatusPriorityTables(c *config.Config) {
	shorts := make(map[string]string, len(c.StatusList()))
	for _, s := range c.StatusList() {
		shorts[s.Name] = c.ShortOfStatus(s.Name)
	}
	ui.SetStatusShorts(shorts)

	symbols := make(map[string]string, len(c.PriorityList()))
	for _, p := range c.PriorityList() {
		symbols[p.Name] = p.Symbol
	}
	ui.SetPrioritySymbols(symbols)
}

// completionAwareRootFlags returns the --lif-path and --config values
// PersistentPreRunE should resolve against, accounting for cobra's
// __complete request.
//
// completeCmd (cobra's hidden "__complete" command) sets
// DisableFlagParsing: true, because it must forward the untouched
// command-line words to its own getCompletions logic rather than have
// cobra's normal Command.execute consume them. That means Command.execute
// skips ParseFlags for it (command.go's execute: "if c.DisableFlagParsing {
// return nil }"), so neither the storePath nor the configPath package var
// -- both bound to their flags via pflag.StringVar -- is populated by the
// time PersistentPreRunE runs here, even though the raw args this hook
// receives still contain "--lif-path X" / "--config Y" verbatim (cobra's
// Find only strips flags for command-name matching; it never discards them
// from the args it hands back). Left alone, both the store-path resolution
// and the config lookup fall back to directory discovery, silently
// resolving another store's/config's candidates (beans-9jtq).
//
// Outside a __complete request cobra has already parsed both flags
// normally, so storePath/configPath are authoritative and are returned
// unchanged.
func completionAwareRootFlags(cmd *cobra.Command, args []string) (storePathVal, configPathVal string) {
	if cmd.Name() != cobra.ShellCompRequestCmd {
		return storePath, configPath
	}
	fs := pflag.NewFlagSet("completion-root-flags", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist = pflag.ParseErrorsAllowlist{UnknownFlags: true}
	sp := fs.String(storePathFlag, "", "")
	cp := fs.String("config", "", "")
	_ = fs.Parse(args)
	return *sp, *cp
}

// The levels that can decide the store directory, as recorded in
// storeResolution.StoreDirSource.
const (
	storeSourceFlag    = "flag"
	storeSourceEnv     = "env"
	storeSourceConfig  = "config"  // an explicit path: in the config file
	storeSourceDefault = "default" // the default store directory
)

// storeResolution is the outcome of the four name levels (config file,
// section key, store directory, env var) for the command that just resolved
// its store. `lif check` reads it to report names still in legacy form.
type storeResolution struct {
	// LegacyConfigFile: the loaded config file is the legacy file.
	LegacyConfigFile bool
	// LegacySectionKey: that file uses the legacy section key.
	LegacySectionKey bool
	// LegacyStoreDir: the resolved store directory carries the legacy name.
	LegacyStoreDir bool
	// StoreDirSource: flag, env, config (an explicit path:) or default.
	StoreDirSource string
	// LegacyEnv: the legacy variable is set and the new one is not.
	LegacyEnv bool
}

// resolution is set by the root PersistentPreRunE next to core and cfg.
var resolution storeResolution

// resolveStorePath determines the items data directory path and records which
// name levels it went through.
// Precedence: --lif-path flag > config file (found upward or named via
// --config) > store-path env var > default.
//
// A config file is the repository's own declaration of where its store lives,
// so it outranks the env var: the variable is commonly exported by direnv and
// inherited into unrelated repositories, where honouring it would silently
// redirect every call to a foreign store. Without such a declaration the env
// var is still the next-best answer. The anchor is part of that declaration
// and is applied while resolving it (see resolveAnchoredPath).
//
// The env level is resolved -- and can fail on both variables being set --
// only when it decides, that is without a flag and without a found config.
//
// In worktrees, the CLI uses the worktree's local store directory.
// lif-serve watches worktree store dirs and merges changes into
// runtime state, so the UI stays up-to-date without writing to main.
// A repository that wants one shared store instead opts out of that with
// `anchor: repo-root` in its config file (see resolveAnchoredPath).
func resolveStorePath(flagPath string, c *config.Config) (string, storeResolution, error) {
	envPath, legacyEnv, envErr := config.ResolveStoreEnv()
	// A found config file, and only a found one, demotes the env var. A
	// defaulted config carries no declaration to honour.
	configFound := c != nil && c.ConfigFile() != ""
	envDecides := flagPath == "" && !configFound
	if envDecides && envErr != nil {
		return "", storeResolution{}, envErr
	}
	useEnv := envDecides && envPath != ""
	explicitOverride := flagPath != "" || useEnv

	res := storeResolution{LegacyEnv: legacyEnv}
	var root string
	switch {
	case flagPath != "":
		root = flagPath
		res.StoreDirSource = storeSourceFlag
	case useEnv:
		root = envPath
		res.StoreDirSource = storeSourceEnv
	default:
		anchored, err := resolveAnchoredPath(c)
		if err != nil {
			return "", storeResolution{}, err
		}
		root = anchored
		res.StoreDirSource = storeSourceDefault
		if c.Items.Path != "" {
			res.StoreDirSource = storeSourceConfig
		}
	}
	if configFound {
		res.LegacyConfigFile = config.IsLegacyConfigFile(c.ConfigFile())
		res.LegacySectionKey = config.IsLegacySectionKey(c.SectionKey())
	}
	res.LegacyStoreDir = config.IsLegacyStoreDir(root)

	if info, statErr := os.Stat(root); statErr != nil || !info.IsDir() {
		// A missing/empty live directory next to a non-empty .lif.bak-* or .beans.bak-*
		// sibling is not "no store here" -- it's an interrupted stageAndSwap
		// (rename.go: the swap is two os.Rename calls with the store dir absent in
		// between). Core.Load's own detectOrphanBackup repairs or warns
		// about exactly that case, but only gets the chance to run if this
		// precheck doesn't fail first.
		if itemcore.HasOrphanBackup(root) {
			return root, res, nil
		}
		if explicitOverride {
			return "", storeResolution{}, &missingStoreError{fmt.Sprintf("store path does not exist or is not a directory: %s", root)}
		}
		return "", storeResolution{}, &missingStoreError{fmt.Sprintf("no %s directory found at %s (run 'lif init' to create one)", filepath.Base(root), root)}
	}

	return root, res, nil
}

// resolveAnchoredPath applies the config's anchor setting.
//
// With AnchorRepoRoot, a call from a secondary worktree resolves to the main
// worktree's store, so worktrees of one repository share it. Outside a git repo
// — and in the main worktree, where there is nothing to redirect — the config
// file's own directory stays the anchor. The default store directory, where no
// explicit path applies, is resolved in that anchor.
func resolveAnchoredPath(c *config.Config) (string, error) {
	switch c.Items.Anchor {
	case "":
		return c.ResolveItemsPath()
	case config.AnchorRepoRoot:
		mainRoot, isSecondary := gitutil.MainWorktreeRoot(c.ConfigDir())
		if !isSecondary {
			return c.ResolveItemsPath()
		}
		return c.ResolveItemsPathIn(mainRoot)
	default:
		return "", config.ValidateAnchor(c.Items.Anchor, c.SectionKey())
	}
}

// missingStoreError marks the failure of a store directory that does not
// exist, so the root hook can report it as NO_LIF_DIR under --json.
type missingStoreError struct{ msg string }

func (e *missingStoreError) Error() string { return e.msg }

// reportExecutionError writes the one shape a failure takes: the error line
// and a pointer to the manual, both on stderr. It is cobra's own idiom for an
// unknown command (command.go:1124-1133), adopted here for every error class
// rather than inventing a second policy.
//
// The suppression is keyed on the error already carrying a machine-readable
// document, not on --json being present. That way a --json consumer gets
// exactly one artifact, while an error raised before a command reached the
// output layer — a broken config, a missing store — still reaches the user.
func reportExecutionError(cmd *cobra.Command, err error) {
	if output.Emitted(err) || output.IsSilent(err) {
		return
	}
	var se *tasks.StepError
	if errors.As(err, &se) {
		// A failed task step is not a usage error: no pointer to the manual.
		cmd.PrintErrln(cmd.ErrPrefix(), err.Error())
		return
	}
	cmd.PrintErrln(cmd.ErrPrefix(), err.Error())
	cmd.PrintErrf("Run '%v --help' for usage.\n", cmd.CommandPath())
}

// exitCode is the process exit code for err: the failing step's own code for
// a task step, 1 for everything else.
func exitCode(err error) int {
	var se *tasks.StepError
	if errors.As(err, &se) && se.Code > 0 {
		return se.Code
	}
	return 1
}

// Execute runs the given root command and exits on error.
func Execute(rootCmd *cobra.Command) {
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		reportExecutionError(cmd, err)
		os.Exit(exitCode(err))
	}
}
