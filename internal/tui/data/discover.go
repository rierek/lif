package data

import (
	"fmt"
	"path/filepath"

	"github.com/rierek/lif/pkg/config"
)

// FindRepo walks upward from start until it finds a directory containing the
// lif config (.lif.yml, or the legacy .beans.yml) and returns that directory's
// absolute path. It returns an error if none exists between start and the
// filesystem root, or if both config names exist in one directory.
func FindRepo(start string) (string, error) {
	resolved, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve start dir %q: %w", start, err)
	}
	path, err := config.FindConfig(resolved)
	if err != nil {
		return "", err
	}
	if path == "" {
		// Report the resolved (absolute) path: that is what got walked.
		return "", fmt.Errorf("no %s found above %s", config.ConfigFileName, resolved)
	}
	return filepath.Dir(path), nil
}

// loadRepoConfig loads the project config of repoDir; nil (built-in defaults)
// when it is missing or unreadable, like RepoSlug: the TUI must still open
// the repo.
func loadRepoConfig(repoDir string) *config.Config {
	if path, found, err := config.ResolveConfigFile(repoDir); err == nil && found {
		if c, err := config.Load(path); err == nil {
			return c
		}
	}
	return nil
}

// ApplyRepoConfig makes the project config of repoDir the TUI's source for
// status/type/priority lists, type ranks and configured colours (SetConfig).
func ApplyRepoConfig(repoDir string) { SetConfig(loadRepoConfig(repoDir)) }

// OpenStatusFunc returns "is this status open" for repoDir's own config, which
// may differ from the active repo's (Lobby metrics of other repos).
func OpenStatusFunc(repoDir string) func(string) bool {
	cfg := loadRepoConfig(repoDir)
	return func(s string) bool { return !cfg.IsArchiveStatus(s) }
}
