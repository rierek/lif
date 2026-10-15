package commands

import (
	"os"

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

--lif-path names the store of the opened repository.`,
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
		return runTUI(path, storePath)
	},
}

// RegisterTuiCmd registers the tui command on root.
func RegisterTuiCmd(root *cobra.Command) {
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

// runTUI starts the TUI: repo discovery upward from path (default cwd), a
// Client, then tui.Run. storePathFlag, when set, is handed only to the client
// of this process's single-repo launch -- never to a Lobby repo the user
// switches to later, which resolves its own store.
func runTUI(path, storePathFlag string) error {
	// The TUI drives the same build that runs it.
	if exe, err := os.Executable(); err == nil {
		data.Binary = exe
	}

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
		return tui.Run(&data.Client{RepoDir: repoDir, StorePath: storePathFlag}, repoDir, settings)
	}

	cwdRepoDir, cwdErr := data.FindRepo(".")
	switch decideStartup("", cwdErr, len(settings.Repos)) {
	case startupUseCwdRepo:
		return tui.Run(&data.Client{RepoDir: cwdRepoDir, StorePath: storePathFlag}, cwdRepoDir, settings)
	case startupLobby:
		return tui.Run(nil, "", settings) // Lobby-first: client == nil is tui.Run's contract
	default: // startupError
		return cwdErr
	}
}
