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
