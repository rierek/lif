package itemcore

import (
	"bytes"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"os"
	"path/filepath"

	"github.com/rierek/lif/pkg/config"
)

// indexMarkerFileName is the sidecar file, next to a persisted index
// directory's own Bleve/etag files, recording the absolute store root
// path that produced that directory's hash. pruneOrphanIndexDirs reads it
// back to decide whether a sibling directory's store still exists.
const indexMarkerFileName = "store-root.txt"

// storeRootAbs returns this store's absolute root path, falling back to
// the configured (possibly relative) root if it cannot be resolved -- the
// same fallback indexDir already applied inline before this was factored
// out, so the hash derivation below is unchanged.
func (c *Core) storeRootAbs() string {
	root, err := filepath.Abs(c.root)
	if err != nil {
		return c.root
	}
	return root
}

// indexDir returns the directory used to persist this store's search index.
// It is always outside the store root (AC-04): writeGitignore only excludes
// .conversations/, so an index file inside c.root would be git-tracked and
// end up committed to the repo or the hull. The directory name is a hash of
// the store's absolute root path rather than the configured project name,
// so two different stores never collide even if they share (or lack) a
// project name -- ~/.lif/worktrees/<project>/ (the config-driven sibling
// convention for worktrees) already accepts that collision risk, but the
// index has no such existing contract to match and a hash is exact instead
// of merely conventional.
func (c *Core) indexDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	h := fnv.New64a()
	h.Write([]byte(c.storeRootAbs()))
	id := hex.EncodeToString(h.Sum(nil))
	return filepath.Join(home, config.HomeDir, "index", id), nil
}

// maintainIndexDir keeps the persisted index directory tree at
// ~/.lif/index/ bounded (beans-dpnf): it records dir's owning store root
// in a marker file, then prunes sibling directories whose marker points at
// a store root that no longer exists. Both steps degrade to a logged
// warning rather than fail the caller, matching the discipline
// ensureSearchIndexLocked already applies to indexDir/search.Open errors
// (AC-05, AC-07 of beans-6y60).
func (c *Core) maintainIndexDir(dir string) {
	if err := writeIndexMarker(dir, c.storeRootAbs()); err != nil {
		c.logWarn("writing search index marker: %v", err)
	}
	if err := pruneOrphanIndexDirs(filepath.Dir(dir)); err != nil {
		c.logWarn("pruning orphaned search index directories: %v", err)
	}
}

// writeIndexMarker records root inside dir's marker file, creating dir if
// needed. It is a no-op once the marker already records root (AC-02):
// re-opening the same store's index must leave it unchanged rather than
// rewrite identical content on every open.
func writeIndexMarker(dir, root string) error {
	markerPath := filepath.Join(dir, indexMarkerFileName)
	if existing, err := os.ReadFile(markerPath); err == nil && string(existing) == root {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := markerPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(root), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, markerPath)
}

// pruneOrphanIndexDirs removes sibling directories under indexRoot (the
// ~/.lif/index/ parent of every per-store index directory) whose marker
// records a store root path that no longer exists on disk. A sibling with
// no marker, one that cannot be read, or one whose content is not an
// absolute path is left in place (AC-04): absence of evidence -- including
// a truncated or empty marker, which os.Stat("") would otherwise report as
// "not exist" and misread as a confirmed-gone store -- is not evidence of
// an orphan, only a marker naming a path that is verifiably gone is.
func pruneOrphanIndexDirs(indexRoot string) error {
	entries, err := os.ReadDir(indexRoot)
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		siblingDir := filepath.Join(indexRoot, entry.Name())
		raw, err := os.ReadFile(filepath.Join(siblingDir, indexMarkerFileName))
		if err != nil {
			continue
		}
		root := string(bytes.TrimSpace(raw))
		if !filepath.IsAbs(root) {
			continue
		}
		if _, statErr := os.Stat(root); os.IsNotExist(statErr) {
			if err := os.RemoveAll(siblingDir); err != nil {
				errs = append(errs, err)
				continue
			}
		}
	}
	return errors.Join(errs...)
}
