// Package storeroot resolves the store directory of a lif project. The CLI
// and the TUI share it, so both answer "where is the store" the same way.
package storeroot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
)

// The levels that can decide the store directory, as recorded in
// Resolution.StoreDirSource.
const (
	SourceFlag    = "flag"
	SourceEnv     = "env"
	SourceConfig  = "config"  // an explicit path: in the config file
	SourceDefault = "default" // the default store directory
)

// Resolution is the outcome of the four name levels (config file,
// section key, store directory, env var) for the command that just resolved
// its store. `lif check` reads it to report names still in legacy form.
type Resolution struct {
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

// Resolve determines the items data directory path and records which
// name levels it went through.
// Precedence: --lif-path flag > config file (found upward or named via
// --config) > store-path env var > default.
//
// A config file is the repository's own declaration of where its store lives,
// so it outranks the env var: the variable is commonly exported by direnv and
// inherited into unrelated repositories, where honouring it would silently
// redirect every call to a foreign store. Without such a declaration the env
// var is still the next-best answer. The anchor is part of that declaration
// and is applied while resolving it (see resolveAnchored).
//
// The env level is resolved -- and can fail on both variables being set --
// only when it decides, that is without a flag and without a found config.
//
// In worktrees, the CLI uses the worktree's local store directory.
// lif-serve watches worktree store dirs and merges changes into
// runtime state, so the UI stays up-to-date without writing to main.
// A repository that wants one shared store instead opts out of that with
// `anchor: repo-root` in its config file (see resolveAnchored).
func Resolve(flagPath string, c *config.Config) (string, Resolution, error) {
	envPath, legacyEnv, envErr := config.ResolveStoreEnv()
	// A found config file, and only a found one, demotes the env var. A
	// defaulted config carries no declaration to honour.
	configFound := c != nil && c.ConfigFile() != ""
	envDecides := flagPath == "" && !configFound
	if envDecides && envErr != nil {
		return "", Resolution{}, envErr
	}
	useEnv := envDecides && envPath != ""
	explicitOverride := flagPath != "" || useEnv

	res := Resolution{LegacyEnv: legacyEnv}
	var root string
	switch {
	case flagPath != "":
		root = flagPath
		res.StoreDirSource = SourceFlag
	case useEnv:
		root = envPath
		res.StoreDirSource = SourceEnv
	default:
		anchored, err := resolveAnchored(c)
		if err != nil {
			return "", Resolution{}, err
		}
		root = anchored
		res.StoreDirSource = SourceDefault
		if c.Items.Path != "" {
			res.StoreDirSource = SourceConfig
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
			return "", Resolution{}, &MissingError{fmt.Sprintf("store path does not exist or is not a directory: %s", root)}
		}
		return "", Resolution{}, &MissingError{fmt.Sprintf("no %s directory found at %s (run 'lif init' to create one)", filepath.Base(root), root)}
	}

	return root, res, nil
}

// resolveAnchored applies the config's anchor setting.
//
// With AnchorRepoRoot, a call from a secondary worktree resolves to the main
// worktree's store, so worktrees of one repository share it. Outside a git repo
// — and in the main worktree, where there is nothing to redirect — the config
// file's own directory stays the anchor. The default store directory, where no
// explicit path applies, is resolved in that anchor.
func resolveAnchored(c *config.Config) (string, error) {
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

// MissingError marks the failure of a store directory that does not
// exist, so the root hook can report it as NO_LIF_DIR under --json.
type MissingError struct{ msg string }

func (e *MissingError) Error() string { return e.msg }
