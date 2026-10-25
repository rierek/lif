package data

// tagdefs.go — the Tag-Registry persistence layer (E10 Task 1, bean
// bt-49hh, epic bt-362n D01-D04): a repo-local ".beans-tags.yml" (Sibling
// to .beans.yml, resolved via Store.RepoDir -- no new discovery logic)
// that records which tag names are "defined" (as opposed to "free" tags
// that merely sit on some Bean but were never registered here, D09).
//
// D01 -- format is a flat, sorted `tags: [...]` list. No color/description
// field (YAGNI -- tag color is hash-derived from the name, design-spec.md
// §4/§8, never stored).
//
// D02 -- Load is tolerant-missing/tolerant-corrupt, mirroring
// internal/config/settings.go's LoadSettings contract: a missing or
// unreadable/malformed file is NEVER an error, an empty registry applies
// instead. Read is synchronous (os.ReadFile, no tea.Cmd) -- a local file
// read is fast enough for a direct call in the update path, same
// convention LoadSettings itself uses.
//
// D04 -- lives in package data (Store-scoped, repo I/O), NOT
// internal/config (that package is user-global scope,
// ~/.config/beans-tui/, a different authority than repo-local, team-shared
// data).

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// tagDefsFileName is the Tag-Registry's file name, resolved against
// Store.RepoDir (Sibling to .beans.yml, D01). Deliberately NOT inside
// .beans/ -- beans' own scanner tolerates foreign files there (verified,
// epic bt-362n body), but the repo root is the cleaner, fully-decoupled
// location, mirroring .beans.yml itself.
const tagDefsFileName = ".lif-tags.yml"

// legacyTagDefsFileName is the former name. It is used in place while the new
// name does not exist.
const legacyTagDefsFileName = ".beans-tags.yml"

// tagDefsPath returns the legacy file only when it exists and the new one does
// not; in every other case the new name.
func tagDefsPath(repoDir string) string {
	newPath := filepath.Join(repoDir, tagDefsFileName)
	legacy := filepath.Join(repoDir, legacyTagDefsFileName)
	if _, err := os.Stat(newPath); err != nil {
		if _, lerr := os.Stat(legacy); lerr == nil {
			return legacy
		}
	}
	return newPath
}

// tagDefsFile is the on-disk YAML shape of the Tag-Registry.
type tagDefsFile struct {
	Tags []string `yaml:"tags,omitempty"`
}

// LoadTagDefs reads the repo-local Tag-Registry (D01/D02). A missing file
// returns (nil, nil) -- an empty registry, not an error. Corrupt YAML also
// returns (nil, nil): this NEVER surfaces an error to the caller, mirroring
// config.LoadSettings' "never crash" contract. Names that fail
// ValidTagName are silently dropped (defensive filtering against a
// hand-edited, partially-broken file -- an invalid line is skipped, not
// escalated, same philosophy as the rest of beans-tui's loading code). The
// returned slice is sorted and deduplicated.
func (c *Store) LoadTagDefs() ([]string, error) {
	path := tagDefsPath(c.RepoDir)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // missing -> empty registry
	}
	var f tagDefsFile
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, nil // corrupt YAML -> empty registry, never crash
	}
	valid := make([]string, 0, len(f.Tags))
	for _, name := range f.Tags {
		if ValidTagName(name) {
			valid = append(valid, name)
		}
	}
	return sortDedupTagNames(valid), nil
}

// SaveTagDefs writes the Tag-Registry, sorted and deduplicated before
// writing (deterministic diffs). No directory create -- the repo root
// exists by definition (unlike ~/.config/beans-tui, which SaveUserSettings
// must MkdirAll).
func (c *Store) SaveTagDefs(tags []string) error {
	path := tagDefsPath(c.RepoDir)
	f := tagDefsFile{Tags: sortDedupTagNames(tags)}
	out, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// sortDedupTagNames returns a new, sorted, deduplicated copy of names --
// never mutates its input (Save/Load both rely on this for a
// deterministic, side-effect-free result).
func sortDedupTagNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	out := sorted[:0:0]
	var prev string
	for i, name := range sorted {
		if i == 0 || name != prev {
			out = append(out, name)
		}
		prev = name
	}
	return out
}

// AddTagDefName returns a new slice with name inserted, sorted and
// deduplicated. No-op (returns an equivalent sorted-deduped copy) if name
// is already present. Pure -- never mutates defs.
func AddTagDefName(defs []string, name string) []string {
	return sortDedupTagNames(append(append([]string(nil), defs...), name))
}

// RemoveTagDefName returns a new slice with name removed, if present.
// No-op otherwise. Pure -- never mutates defs.
func RemoveTagDefName(defs []string, name string) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		if d != name {
			out = append(out, d)
		}
	}
	return sortDedupTagNames(out)
}

// RenameTagDefName returns a new slice with old removed and new inserted
// (deduped). A no-op on old if it isn't present -- Rename onto an
// unregistered name is allowed (e.g. promoting a free tag during the
// rename flow), degrading to a plain Add of new. Pure -- never mutates
// defs.
func RenameTagDefName(defs []string, old, new string) []string {
	return AddTagDefName(RemoveTagDefName(defs, old), new)
}
