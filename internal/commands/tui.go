package commands

import (
	"fmt"
	"strings"

	"github.com/rierek/lif/internal/tui"
	tuiconfig "github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui [repo-path]",
	Short: "Open the interactive terminal UI",
	Long: `Opens the interactive terminal UI.

Without an argument, the TUI opens the repository found upward from the
current directory. Outside a repository, it shows the repository picker when
~/.config/lif/tui/config.yaml lists at least two repositories.

--lif-path names the store of the opened repository.

--item <id> opens that item's detail at startup. The ID is a full item ID or
the complete short ID without the configured prefix; partial IDs do not match.
The item must exist in the selected repository (repo-path, else the one found
from the current directory; the repository picker is never used). If it does
not, the command fails before the terminal UI starts. The request applies once
to the first load: later reloads keep your navigation.`,
	Args: cobra.MaximumNArgs(1),
	// repo-path is a directory: complete directories only, not any file.
	ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		itemID, err := cmd.Flags().GetString("item")
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("item") && strings.TrimSpace(itemID) == "" {
			return fmt.Errorf("--item requires a non-empty item ID")
		}
		return runTUI(path, storePath, configPath, itemID)
	},
}

// RegisterTuiCmd registers the tui command on root.
func RegisterTuiCmd(root *cobra.Command) {
	tuiCmd.Flags().String("item", "", "Open item detail at startup (full or short ID)")
	root.AddCommand(tuiCmd)
}

// startupDecision is the startup trigger, factored out as a pure function (no
// I/O) so it is unit-testable without launching tui.Run, which starts an
// interactive AltScreen program that a go test process cannot drive.
type startupDecision int

const (
	startupUseArgRepo startupDecision = iota // an explicit path arg resolved -- use it
	startupUseCwdRepo                        // no arg, but data.FindRepo(cwd) resolved -- use it
	startupLobby                             // no arg, cwd unresolved, >=2 configured repos -- Lobby
	startupError                             // no arg, cwd unresolved, 0/1 configured repos -- plain error
)

// decideStartup implements the priority order: an explicit path arg always
// wins; absent that, a resolving cwd wins next, regardless of how many repos
// the settings list; only when both are unresolved does the configured repo
// count decide: >=2 -> Lobby, 0/1 -> the plain FindRepo error (a Lobby over
// one or zero repos has no value over that error).
func decideStartup(argPath string, cwdFindErr error, configuredRepoCount int) startupDecision {
	if argPath != "" {
		return startupUseArgRepo
	}
	if cwdFindErr == nil {
		return startupUseCwdRepo
	}
	if configuredRepoCount >= 2 {
		return startupLobby
	}
	return startupError
}

// runTUI starts the TUI: repo discovery upward from path (default cwd), an
// in-process data.Store, then tui.Run. storePathFlag and configPathFlag (the
// persistent --lif-path and --config flags, parsed also for this bootstrap-skip
// command), when set, apply only to the store of this process's single-repo
// launch -- never to a Lobby repo the user switches to later, which resolves
// its own store and config.
func runTUI(path, storePathFlag, configPathFlag, initialItemID string) error {
	settings, err := tuiconfig.LoadSettings()
	if err != nil {
		settings = tuiconfig.DefaultSettings()
	}

	// An explicit path arg wins without ever walking up from cwd.
	if path != "" {
		repoDir, ferr := data.FindRepo(path)
		if ferr != nil {
			return ferr
		}
		return runTUIRepo(repoDir, storePathFlag, configPathFlag, settings, initialItemID)
	}

	cwdRepoDir, cwdErr := data.FindRepo(".")
	decision := decideStartup("", cwdErr, len(settings.Repos))
	if decision == startupLobby && initialItemID != "" {
		return cwdErr // --item names an item of one repository: never guess one in the Lobby
	}
	switch decision {
	case startupUseCwdRepo:
		return runTUIRepo(cwdRepoDir, storePathFlag, configPathFlag, settings, initialItemID)
	case startupLobby:
		return tui.Run(nil, "", settings, "") // Lobby-first: store == nil is tui.Run's contract
	default: // startupError
		return cwdErr
	}
}

// runTUIRepo opens repoDir's store and hands it to tui.Run, which owns it from
// then on and closes it on exit. A non-empty initialItemID must resolve in
// this store (full ID, else short ID with the configured prefix) before the
// terminal is entered.
func runTUIRepo(repoDir, storePathFlag, configPathFlag string, settings tuiconfig.Settings, initialItemID string) error {
	store, err := data.Open(repoDir, storePathFlag, configPathFlag)
	if err != nil {
		return err
	}
	if initialItemID != "" {
		if initialItemID, err = resolveStartupItem(store, initialItemID); err != nil {
			store.Close()
			return err
		}
	}
	return tui.Run(store, repoDir, settings, initialItemID)
}

// resolveStartupItem maps the --item value to the canonical ID of an existing
// item: an exact full ID first, else the complete short ID with the configured
// prefix (the CLI's resolution, itemcore.Core.NormalizeID). No fuzzy matching.
func resolveStartupItem(store *data.Store, id string) (string, error) {
	full, found := store.Core.NormalizeID(id)
	if !found {
		return "", fmt.Errorf("item not found: %s", id)
	}
	return full, nil
}
