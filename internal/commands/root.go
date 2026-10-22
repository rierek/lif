package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/storeroot"
	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/internal/uiconfig"
	"github.com/rierek/lif/internal/version"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var core *itemcore.Core
var cfg *config.Config

// hookDispatch runs the hooks and gates of the project. The store bootstrap
// sets it; a nil dispatcher does nothing (its Gate and Fire are nil-safe).
var hookDispatch *hooks.Dispatcher

// newResolver returns the resolver that every command uses, so that every
// write path fires the hooks and runs the gates. Its hook functions are method
// values of hookDispatch: they exist even when hookDispatch is nil, so only
// the ran result of a call tells whether a hook ran, never the field being set.
func newResolver() *itemgraph.CoreResolver {
	return &itemgraph.CoreResolver{Core: core, BeforeMutation: hookDispatch.Gate, AfterMutation: hookDispatch.Fire}
}

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
			uiconfig.Apply(cfg)

			root, res, err := storeroot.Resolve(effectiveStorePath, cfg)
			if err != nil {
				var missing *storeroot.MissingError
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

			// The warnings and the output of after hooks go to this
			// command's stderr, so --json keeps stdout a single document.
			hookDispatch = hooks.New(cfg, core.Root(), cmd.ErrOrStderr())

			return nil
		},
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")
	rootCmd.PersistentFlags().StringVar(&storePath, storePathFlag, "", "Path to data directory (overrides config and LIF_PATH env var; a legacy BEANS_PATH is read when LIF_PATH is unset)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: searches upward for .lif.yml; a legacy .beans.yml is read when no .lif.yml exists)")

	return rootCmd
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

// resolution is set by the root PersistentPreRunE next to core and cfg.
var resolution storeroot.Resolution

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
	var blocked *hooks.BlockedError
	var mismatch *itemcore.ETagMismatchError
	var required *itemcore.ETagRequiredError
	if errors.As(err, &se) || errors.As(err, &blocked) || errors.As(err, &mismatch) || errors.As(err, &required) {
		// A failed task step, a gate that blocked the change or an etag
		// error is not a usage error: no pointer to the manual.
		cmd.PrintErrln(cmd.ErrPrefix(), err.Error())
		return
	}
	cmd.PrintErrln(cmd.ErrPrefix(), err.Error())
	cmd.PrintErrf("Run '%v --help' for usage.\n", cmd.CommandPath())
}

// exitCode is the process exit code for err. For `lif run`: the failing step's
// own code, 128+n for a step killed by signal n, 2 for an error of lif itself
// (runLifError, or a step that could not start). 1 for everything else, also
// for a gate that blocked a change: hooks.BlockedError has no Unwrap, so the
// step code of the gate's task does not reach this function.
func exitCode(err error) int {
	var se *tasks.StepError
	if errors.As(err, &se) {
		switch {
		case se.Code > 0:
			return se.Code
		case se.Signal > 0:
			return 128 + int(se.Signal)
		case !errors.Is(se.Err, tasks.ErrInterrupted):
			return 2
		}
	}
	var le *runLifError
	if errors.As(err, &le) {
		return 2
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
