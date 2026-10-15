package itemcore

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// RenameChange describes a single item's rename (own file move and/or ID
// change). OldPath/NewPath are relative to the .beans dir, matching
// Item.Path (e.g. "tp-aaaa--slug.md" or "epic-auth/tp-aaaa--slug.md").
type RenameChange struct {
	OldID, NewID     string
	OldPath, NewPath string
}

// AttachmentMove is a directory move under AttachmentsDir that an
// ID-changing rename entails. Paths are .beans-relative, matching
// RenameChange.
type AttachmentMove struct {
	OldPath, NewPath string
}

// RenamePlan is the dry-run result of a rename computation. Mode is one of
// "slug", "id", "prefix". AttachmentMoves is empty for "slug", which does
// not touch the ID an attachment directory is keyed by.
type RenamePlan struct {
	Mode            string
	Changes         []RenameChange
	RefUpdates      map[string]int // itemID -> number of ref fields rewritten
	AttachmentMoves []AttachmentMove
	NewPrefix       string
	ConfigWrite     bool
}

// repoRoot returns the directory containing the .beans dir (used for the
// staging sibling dir and worktree/config lookups, not for item paths).
func (c *Core) repoRoot() string { return filepath.Dir(c.root) }

// newItemPath returns the .beans-relative path an item should have after a
// rename, preserving any subdirectory it currently lives in (Item.Path may
// be nested, e.g. "epic-auth/id--slug.md").
func newItemPath(oldPath, newID, newSlug string) string {
	dir := filepath.Dir(oldPath) // "." for a flat file
	name := item.BuildFilename(newID, newSlug)
	if dir == "." {
		return name
	}
	return filepath.Join(dir, name)
}

// PlanRenameSlug computes a dry-run plan to change an item's slug. Exactly
// one of newSlug (explicit, "" clears it) or reslug (regenerate from Title)
// must be used — the caller enforces mutual exclusivity.
func (c *Core) PlanRenameSlug(id string, newSlug *string, reslug bool) (*RenamePlan, error) {
	c.mu.RLock()
	b, ok := c.items[id]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("item %q not found", id)
	}
	var slug string
	switch {
	case reslug:
		slug = item.Slugify(b.Title)
	case newSlug != nil:
		slug = *newSlug
	default:
		return nil, fmt.Errorf("no slug source given")
	}
	return &RenamePlan{
		Mode: "slug",
		Changes: []RenameChange{{
			OldID: b.ID, NewID: b.ID,
			OldPath: b.Path, NewPath: newItemPath(b.Path, b.ID, slug),
		}},
		RefUpdates: map[string]int{},
	}, nil
}

// ApplyRename executes a previously computed RenamePlan.
func (c *Core) ApplyRename(plan *RenamePlan) error {
	switch plan.Mode {
	case "slug":
		return c.applyRenameSlug(plan)
	case "id":
		return c.applyRenameCascade(plan)
	case "prefix":
		if err := c.applyRenameCascade(plan); err != nil {
			return err
		}
		return c.writeRebrandConfig(plan.NewPrefix)
	default:
		return fmt.Errorf("unknown rename mode %q", plan.Mode)
	}
}

func (c *Core) applyRenameSlug(plan *RenamePlan) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := plan.Changes[0]
	oldAbs := filepath.Join(c.root, ch.OldPath) // OldPath/NewPath are .beans-relative
	newAbs := filepath.Join(c.root, ch.NewPath)
	if oldAbs == newAbs {
		return nil
	}
	if err := os.Rename(oldAbs, newAbs); err != nil {
		return fmt.Errorf("renaming slug file: %w", err)
	}
	// A slug rename touches no link field, so the incoming index stays valid.
	// The mainPaths entry belongs inside the guard: registering a path for an
	// ID the store does not have would make findMainItemFile answer for an
	// unknown item.
	if b, ok := c.items[ch.OldID]; ok {
		b.Path = ch.NewPath
		_, b.Slug = item.ParseFilename(filepath.Base(newAbs))
		c.mainPaths[ch.OldID] = ch.NewPath
	}
	return nil
}

// rewriteRefs replaces mapped old IDs with their new IDs in b's relationship
// fields (Parent/Blocking/BlockedBy). It mutates b in place and returns the
// number of individual ref fields changed. A ref value not present in m is
// left unchanged; an empty Parent is never considered a match.
func rewriteRefs(b *item.Item, m map[string]string) int {
	changed := 0
	if nv, ok := m[b.Parent]; ok && b.Parent != "" {
		b.Parent = nv
		changed++
	}
	for i, id := range b.Blocking {
		if nv, ok := m[id]; ok {
			b.Blocking[i] = nv
			changed++
		}
	}
	for i, id := range b.BlockedBy {
		if nv, ok := m[id]; ok {
			b.BlockedBy[i] = nv
			changed++
		}
	}
	return changed
}

// PlanRenameID computes a dry-run plan to rename a single item's ID,
// cascading the change into every referencing item's Parent/Blocking/
// BlockedBy fields. Refuses (no mutation) if newID collides with an existing
// item.
func (c *Core) PlanRenameID(oldID, newID string) (*RenamePlan, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if oldID == newID {
		return nil, fmt.Errorf("new ID equals old ID")
	}
	b, ok := c.items[oldID]
	if !ok {
		return nil, fmt.Errorf("item %q not found", oldID)
	}
	if _, exists := c.items[newID]; exists {
		return nil, fmt.Errorf("ID collision: %q already exists", newID)
	}
	return c.planCascade("id", map[string]string{oldID: newID}, map[string]*item.Item{oldID: b})
}

// planCascade builds a RenamePlan for an ID map (oldID -> newID). renamed
// holds only the items whose own ID changes (their filename is recomputed
// via newItemPath, preserving any subdir); every item in c.items is scanned
// for ref-field hits against idMap. Must be called with at least a read lock
// held.
func (c *Core) planCascade(mode string, idMap map[string]string, renamed map[string]*item.Item) (*RenamePlan, error) {
	plan := &RenamePlan{Mode: mode, RefUpdates: map[string]int{}}
	for oldID, b := range renamed {
		newID := idMap[oldID]
		plan.Changes = append(plan.Changes, RenameChange{
			OldID: oldID, NewID: newID,
			OldPath: b.Path,
			NewPath: newItemPath(b.Path, newID, b.Slug),
		})
		// An attachment directory is keyed by item ID, so an ID change
		// strands it unless it moves too. Only plan a move for a directory
		// that is actually there: most items have no attachments, and a
		// move of a missing directory would fail the apply for every one of
		// them.
		if move, ok := c.planAttachmentMove(oldID, newID); ok {
			plan.AttachmentMoves = append(plan.AttachmentMoves, move)
		}
	}
	for _, b := range c.items {
		if n := countRefHits(b, idMap); n > 0 {
			plan.RefUpdates[b.ID] = n
		}
	}
	return plan, nil
}

// planAttachmentMove reports the directory move an ID change entails, and
// whether there is one at all. Reported false when the item has no
// attachment directory, which is the common case.
func (c *Core) planAttachmentMove(oldID, newID string) (AttachmentMove, bool) {
	oldRel := filepath.Join(AttachmentsDir, oldID)
	info, err := os.Stat(filepath.Join(c.root, oldRel))
	if err != nil || !info.IsDir() {
		return AttachmentMove{}, false
	}
	return AttachmentMove{OldPath: oldRel, NewPath: filepath.Join(AttachmentsDir, newID)}, true
}

// OrphanAttachments lists attachment directories whose item ID no longer
// resolves, sorted for stable output. Without this the carry-along
// invariant of an ID rename is merely asserted: the measured failure mode
// was a green `lif check` over a directory pointing at an ID that had
// been rebranded away. Archived items stay loaded, so their attachments are
// not orphans.
func (c *Core) OrphanAttachments() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(c.root, AttachmentsDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	var orphans []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := c.items[e.Name()]; !ok {
			orphans = append(orphans, e.Name())
		}
	}
	sort.Strings(orphans)
	return orphans, nil
}

// AttachmentNames lists the file names in attachments/<id>/, sorted. It is
// the read path D16 recorded as missing ("no verb, no show section, no
// check") and the reason a front matter key listing attachments is not
// needed: the directory is already keyed by the item ID, so the list is
// derived rather than asserted and cannot drift from the files.
//
// Subdirectories are skipped -- a reader opens a file, and a name that
// resolves to a directory is a citation that fails at the editor. An absent
// directory is the normal case for most items and yields nothing.
func (c *Core) AttachmentNames(id string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(c.root, AttachmentsDir, id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// countRefHits counts how many of b's ref fields (Parent/Blocking/BlockedBy)
// reference an old ID present in m, without mutating b.
func countRefHits(b *item.Item, m map[string]string) int {
	n := 0
	if _, ok := m[b.Parent]; ok && b.Parent != "" {
		n++
	}
	for _, id := range b.Blocking {
		if _, ok := m[id]; ok {
			n++
		}
	}
	for _, id := range b.BlockedBy {
		if _, ok := m[id]; ok {
			n++
		}
	}
	return n
}

// checkNoActiveWorktrees refuses (returns a non-nil error) if the resolved
// worktree directory for this project contains any *.meta.json marker
// (an active worktree — see the Worktree State Architecture section of
// CLAUDE.md). Project-name resolution mirrors serve.go:115-117 exactly:
// GetProjectName() first, basename(ConfigDir()) only as a fallback. Using
// only the basename would resolve the wrong worktree dir when project.name
// is configured, silently letting a rebrand through and undermining D05.
// If the worktree path cannot be resolved, or the directory does not exist,
// this is treated as "no active worktrees" (nothing to guard against).
func (c *Core) checkNoActiveWorktrees() error {
	if c.config == nil {
		return nil
	}
	projectName := c.config.GetProjectName()
	if projectName == "" {
		projectName = filepath.Base(c.config.ConfigDir())
	}
	wtPath, _, err := c.config.ResolveWorktreePath(projectName)
	if err != nil {
		return nil // cannot resolve -> assume none
	}
	entries, err := os.ReadDir(wtPath)
	if err != nil {
		return nil // dir absent -> none
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".meta.json") {
			return fmt.Errorf("active worktrees present under %s; merge or remove them before a prefix rebrand", wtPath)
		}
	}
	return nil
}

// checkServerNotRunning refuses (returns a non-nil error) if a TCP dial to
// 127.0.0.1:<GetServerPort()> succeeds — a heuristic proxy for "an items
// serve process is running against this project" (D05). This cannot detect
// a server bound to a different interface/host, or a rebrand check racing
// a server that starts immediately after the probe; it is a best-effort
// fail-fast, not a lock. Documented limitation, matches the plan (Task 7).
func (c *Core) checkServerNotRunning() error {
	if c.config == nil {
		return nil
	}
	port := c.config.GetServerPort()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return fmt.Errorf("lif-serve appears to be running on port %d; stop it before a prefix rebrand", port)
	}
	return nil
}

// PlanRebrand computes a dry-run plan that maps every item's ID from the
// project's current prefix (c.config.Items.Prefix) to newPrefix, preserving
// each ID's suffix (item.RebrandID). Mode is "prefix", NewPrefix and
// ConfigWrite are set so ApplyRename also persists the new prefix to
// .beans.yml after the cascade swap.
//
// B04 guard: refuses outright (no partial plan) if any item ID does not
// start with the current prefix — RebrandID would otherwise double-prefix
// it (newPrefix + the untouched full old ID), corrupting that ID.
//
// D05 guards (checkServerNotRunning, checkNoActiveWorktrees) run first,
// fail-fast, before any lock is taken or staging happens — a prefix rebrand
// is refused outright while a lif server is running or active worktrees
// exist, since either could produce a divergent/half-migrated state.
func (c *Core) PlanRebrand(newPrefix string) (*RenamePlan, error) {
	if err := c.checkServerNotRunning(); err != nil {
		return nil, err
	}
	if err := c.checkNoActiveWorktrees(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	oldPrefix := ""
	if c.config != nil {
		oldPrefix = c.config.Items.Prefix
	}
	if newPrefix == oldPrefix {
		return nil, fmt.Errorf("new prefix equals current prefix %q", oldPrefix)
	}

	idMap := map[string]string{}
	renamed := map[string]*item.Item{}
	for id, b := range c.items {
		if oldPrefix != "" && !strings.HasPrefix(id, oldPrefix) {
			return nil, fmt.Errorf("item %q does not start with the current prefix %q; refusing rebrand to avoid ID corruption", id, oldPrefix)
		}
		nid := item.RebrandID(id, oldPrefix, newPrefix)
		if nid == id {
			continue
		}
		if _, clash := c.items[nid]; clash {
			return nil, fmt.Errorf("rebrand collision: %q already exists", nid)
		}
		idMap[id] = nid
		renamed[id] = b
	}
	if len(idMap) == 0 {
		return nil, fmt.Errorf("no items matched prefix %q", oldPrefix)
	}

	plan, err := c.planCascade("prefix", idMap, renamed)
	if err != nil {
		return nil, err
	}
	plan.NewPrefix = newPrefix
	plan.ConfigWrite = true
	return plan, nil
}

// applyRenameCascade is the shared apply path for Mode "id" and "prefix": it
// re-renders every item touched by the plan's ID map — the renamed item(s)
// themselves (new filename, corrected "# id" comment via Render) plus every
// referencing item (rewritten ref fields) — and drives the whole change
// through stageAndSwap for atomicity. After a successful swap it refreshes
// in-memory state from disk so the same Core resolves the new ID(s)
// immediately (Get(newID) works without a separate Load()).
func (c *Core) applyRenameCascade(plan *RenamePlan) error {
	idMap := map[string]string{}
	renamedIDs := map[string]bool{}
	for _, ch := range plan.Changes {
		idMap[ch.OldID] = ch.NewID
		renamedIDs[ch.OldID] = true
	}

	c.mu.RLock()
	writes := map[string][]byte{}
	removes := []string{}
	for _, b := range c.items {
		// Work on a shallow clone so live state is never mutated pre-swap —
		// if rendering or staging fails partway through, c.items is untouched.
		clone := *b
		clone.Blocking = append([]string(nil), b.Blocking...)
		clone.BlockedBy = append([]string(nil), b.BlockedBy...)

		idChanged := renamedIDs[b.ID]
		refChanged := rewriteRefs(&clone, idMap) > 0
		if !idChanged && !refChanged {
			continue
		}
		if idChanged {
			clone.ID = idMap[b.ID]
		}
		content, err := clone.Render()
		if err != nil {
			c.mu.RUnlock()
			return fmt.Errorf("rendering %s: %w", b.ID, err)
		}
		if idChanged {
			// remove old-named file, write new-named file (subdir preserved)
			removes = append(removes, b.Path)
			writes[newItemPath(b.Path, clone.ID, clone.Slug)] = content
		} else {
			// same filename, new content (ref-only change)
			writes[b.Path] = content
		}
	}
	c.mu.RUnlock()

	if err := c.stageAndSwap(writes, removes, plan.AttachmentMoves...); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadFromDisk()
}

// writeRebrandConfig persists newPrefix to the config file after a successful
// prefix-rebrand cascade swap. Runs after (not inside) stageAndSwap's
// atomic swap: the item-file cascade is the primary invariant (an
// interrupted config write leaves items self-consistent — Item IDs are read
// from filenames, not from the config prefix — while an interrupted cascade
// mid-config-write would be far harder to reason about).
func (c *Core) writeRebrandConfig(newPrefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.config == nil {
		return fmt.Errorf("no config to update")
	}
	c.config.Items.Prefix = newPrefix
	if err := c.config.Save(c.config.ConfigDir()); err != nil {
		return fmt.Errorf("writing %s: %w", c.config.TargetFileName(c.config.ConfigDir()), err)
	}
	return nil
}

// copyTree recursively copies src into dst, skipping any src-relative path
// present in skip (a directory match skips the whole subtree).
func copyTree(src, dst string, skip map[string]bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relToSrc, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if skip[relToSrc] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, relToSrc)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	})
}

// swapRename performs the second (staging-into-c.root) rename in
// stageAndSwap. It is a package-level variable — rather than a bare
// os.Rename call — purely so tests can inject a deterministic failure here
// and assert the rollback branch, without depending on an OS-level race
// between the two renames. Production code always uses the real os.Rename.
var swapRename = os.Rename

// stageAndSwap builds a new store tree in a temp sibling dir (.lif-staging-*,
// a full clone of the current tree with removes/writes applied), then
// atomically swaps it in for c.root. writes/removes keys are store-relative
// (matching Item.Path, e.g. "x.md" or "epic/x.md"). Any error before the swap leaves
// the original .beans tree untouched and removes the staging dir.
//
// moves are directory renames applied inside the staging tree after the
// clone, which is what makes an attachment directory follow an ID change
// under the same atomic swap as the item files themselves — there is no
// window in which the item carries its new ID and the attachment still the
// old one.
func (c *Core) stageAndSwap(writes map[string][]byte, removes []string, moves ...AttachmentMove) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	repo := c.repoRoot()
	staging, err := os.MkdirTemp(repo, config.StagingDirPrefix+"*")
	if err != nil {
		return fmt.Errorf("creating staging dir: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(staging)
		}
	}()

	// skip set: removed files, keyed .beans-relative (== copyTree's walk keys)
	skip := map[string]bool{}
	for _, r := range removes {
		skip[filepath.Clean(r)] = true
	}
	if err := copyTree(c.root, staging, skip); err != nil {
		return fmt.Errorf("cloning tree: %w", err)
	}
	for relPath, content := range writes {
		target := filepath.Join(staging, relPath)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("staging write dir: %w", err)
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			return fmt.Errorf("staging write: %w", err)
		}
	}

	for _, m := range moves {
		from := filepath.Join(staging, m.OldPath)
		to := filepath.Join(staging, m.NewPath)
		if _, err := os.Stat(to); err == nil {
			return fmt.Errorf("staging move: %s already exists", m.NewPath)
		}
		if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
			return fmt.Errorf("staging move dir: %w", err)
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("staging move %s -> %s: %w", m.OldPath, m.NewPath, err)
		}
	}

	// atomic swap
	backup := filepath.Join(repo, fmt.Sprintf("%s%d", config.BackupDirPrefix, time.Now().UnixNano()))
	if err := os.Rename(c.root, backup); err != nil {
		return fmt.Errorf("backing up %s: %w", filepath.Base(c.root), err)
	}
	if err := swapRename(staging, c.root); err != nil {
		os.Rename(backup, c.root) // best-effort rollback
		return fmt.Errorf("swapping in new tree: %w", err)
	}
	cleanup = false
	os.RemoveAll(backup)
	return nil
}
