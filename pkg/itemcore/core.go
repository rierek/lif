// Package itemcore provides a thread-safe in-memory store for items with filesystem persistence
// and optional file watching for long-running processes.
package itemcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rierek/lif/internal/search"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// worktreeItemsDir returns where this store lives inside the worktree at
// worktreePath: the directory carries the same name as the main store's
// directory, so a project with a legacy store and one with a new store both
// find their items in a worktree checkout.
func (c *Core) worktreeItemsDir(worktreePath string) string {
	return filepath.Join(worktreePath, filepath.Base(c.root))
}

const ArchiveDir = "archive"

// AttachmentsDir holds machine evidence artifacts belonging to an item, one
// subdirectory per item ID (D14/D16: findings documents, diffs, guard
// drafts). It is deliberately NOT dot-prefixed, because item bodies cite
// paths inside it and a hidden path is a poor citation target -- which is
// exactly why the item walk has to skip it by name: the walk loads every
// .md it meets under any visible subdirectory, so a guard draft parked here
// would otherwise be parsed into an item of its own (D24/R-31).
const AttachmentsDir = "attachments"

var ErrNotFound = errors.New("item not found")

// ETagMismatchError is returned when an ETag validation fails.
// This allows callers to distinguish concurrency conflicts from other errors.
type ETagMismatchError struct {
	Provided string
	Current  string
}

func (e *ETagMismatchError) Error() string {
	return fmt.Sprintf("etag mismatch: provided %s, current is %s", e.Provided, e.Current)
}

// ETagRequiredError is returned when require_if_match is enabled and no ETag is provided.
type ETagRequiredError struct{}

func (e *ETagRequiredError) Error() string {
	return "if-match etag is required (set require_if_match: false in config to disable)"
}

// PolicyViolationError is returned when a write would put an item into a status
// whose configured required front matter fields are missing.
type PolicyViolationError struct {
	ItemID  string
	Status  string
	Missing []string
}

func (e *PolicyViolationError) Error() string {
	return fmt.Sprintf("status %q requires front matter field(s) %s: supply them in the same write (e.g. `lif complete %s --commit HEAD`)",
		e.Status, strings.Join(e.Missing, ", "), e.ItemID)
}

// Core provides thread-safe in-memory storage for items with filesystem persistence.
type Core struct {
	root   string         // absolute path to .beans directory
	config *config.Config // project configuration

	// In-memory state
	mu            sync.RWMutex
	items         map[string]*item.Item // ID -> Item
	dirty         map[string]bool       // IDs of items modified in runtime but not yet persisted to disk
	worktreeLinks map[string]string     // item ID -> worktree path (items linked to a worktree)

	// incoming is the reverse link index: target item ID -> the links pointing
	// at it. Derived state, rebuilt lazily from items — every mutation of items
	// has to clear incomingValid, so go through setItemLocked and
	// removeItemLocked, or call invalidateIncomingLocked for in-place edits.
	incoming      map[string][]IncomingLink
	incomingValid bool

	// mainPaths maps an item ID to its file path relative to root, for items
	// backed by a file in the main store. A worktree overlay replaces the entry
	// in items but never touches this map, so it still answers "which file in
	// the main store holds this item" while a worktree version is active.
	mainPaths map[string]string

	// Search index (optional, lazy-initialized)
	searchIndex *search.Index

	// File watching (optional)
	watching               bool
	done                   chan struct{}
	onChange               func()                      // callback when items change (legacy API)
	worktreeWatchers       map[string]*worktreeWatcher // worktree path -> watcher
	onWorktreeItemsChanged func()                      // called when worktree item files change

	// Event subscribers (for channel-based API)
	subscribers map[uint64]*subscription
	subMu       sync.RWMutex
	nextSubID   uint64

	// Warning logger for non-fatal errors (defaults to stderr)
	warnWriter io.Writer
}

// New creates a new Core with the given root path and configuration.
func New(root string, cfg *config.Config) *Core {
	return &Core{
		root:          root,
		config:        cfg,
		items:         make(map[string]*item.Item),
		dirty:         make(map[string]bool),
		worktreeLinks: make(map[string]string),
		mainPaths:     make(map[string]string),
		subscribers:   make(map[uint64]*subscription),
		warnWriter:    os.Stderr,
	}
}

// SetOnWorktreeItemsChanged sets a callback that fires whenever item files
// change in a watched worktree. This is used to trigger worktree subscription
// refreshes so the frontend sees updated item lists.
func (c *Core) SetOnWorktreeItemsChanged(fn func()) {
	c.onWorktreeItemsChanged = fn
}

// SetWarnWriter sets the writer for warning messages.
// Pass nil to disable warnings.
func (c *Core) SetWarnWriter(w io.Writer) {
	c.warnWriter = w
}

// logWarn logs a warning message if a warn writer is configured.
func (c *Core) logWarn(format string, args ...any) {
	if c.warnWriter != nil {
		fmt.Fprintf(c.warnWriter, "warning: "+format+"\n", args...)
	}
}

// setItemLocked stores b under id and invalidates the derived indexes.
// Must be called with c.mu held for writing.
func (c *Core) setItemLocked(id string, b *item.Item) {
	c.items[id] = b
	c.incomingValid = false
}

// removeItemLocked drops the item under id and invalidates the derived indexes.
// Must be called with c.mu held for writing.
func (c *Core) removeItemLocked(id string) {
	delete(c.items, id)
	c.incomingValid = false
}

// invalidateIncomingLocked marks the reverse link index stale. Needed wherever
// item link fields are edited in place, without a store write.
// Must be called with c.mu held for writing.
func (c *Core) invalidateIncomingLocked() {
	c.incomingValid = false
}

// Root returns the absolute path to the .beans directory.
func (c *Core) Root() string {
	return c.root
}

// Config returns the configuration.
func (c *Core) Config() *config.Config {
	return c.config
}

// orphanBackups returns the paths of the non-empty crash-backup directories in
// repo: every directory named .lif.bak-* or .beans.bak-*. The two prefixes are
// treated alike; there is no ambiguity error between them. The error is the
// failure to read repo.
func orphanBackups(repo string) ([]string, error) {
	entries, err := os.ReadDir(repo)
	if err != nil {
		return nil, err
	}
	var backups []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !(strings.HasPrefix(name, config.BackupDirPrefix) || strings.HasPrefix(name, config.LegacyBackupDirPrefix)) {
			continue
		}
		backupPath := filepath.Join(repo, name)
		// Check if backup has content
		subentries, err := os.ReadDir(backupPath)
		if err == nil && len(subentries) > 0 {
			backups = append(backups, backupPath)
		}
	}
	return backups, nil
}

// HasOrphanBackup reports whether itemsRoot (a store directory path that may
// not exist) has a sibling .lif.bak-* or .beans.bak-* directory with content --
// the signal that a missing or empty live directory is not "no store here" but
// an interrupted stageAndSwap, and that Core.Load (whose detectOrphanBackup
// below performs the actual repair) should be given the chance to run
// instead of the caller failing outright. Read-only; does not repair.
func HasOrphanBackup(itemsRoot string) bool {
	backups, err := orphanBackups(filepath.Dir(itemsRoot))
	return err == nil && len(backups) > 0
}

// detectOrphanBackup checks for .lif.bak-* and .beans.bak-* orphans left by an
// interrupted stageAndSwap.
// If the live store directory is missing or empty AND exactly one backup exists with content,
// it automatically repairs by renaming the backup back to the store directory.
// If the store directory is missing/empty but multiple or zero backups exist, it logs a warning
// with the exact path and manual recovery command and ensures it exists (empty).
// Backups of both prefixes count together: one of each is two backups.
// Returns true if repair was performed, false otherwise.
func (c *Core) detectOrphanBackup() bool {
	repo := c.repoRoot()

	// Check if the store directory exists and has content
	entries, err := os.ReadDir(c.root)
	if err == nil && len(entries) > 0 {
		// store directory exists and has content; no orphan
		return false
	}

	// The store directory is missing or empty. Look for backups.
	backups, err := orphanBackups(repo)
	if err != nil {
		return false // can't read repo dir, nothing to do
	}

	if len(backups) == 0 {
		// No backups found: ensure the store dir exists (even if empty)
		if err := os.MkdirAll(c.root, 0755); err != nil {
			c.logWarn("failed to create %s directory: %v", filepath.Base(c.root), err)
		}
		return false
	}

	if len(backups) == 1 {
		// Exactly one backup exists: repair by renaming back
		// First remove the empty store dir if it exists
		if err := os.RemoveAll(c.root); err != nil && !os.IsNotExist(err) {
			c.logWarn("failed to clear empty %s directory before repair: %v", filepath.Base(c.root), err)
			// Continue anyway, os.Rename will fail with a clear message
		}
		if err := os.Rename(backups[0], c.root); err != nil {
			c.logWarn("failed to repair %s from backup %s: %v", filepath.Base(c.root), backups[0], err)
			// Ensure the store dir exists even after failed repair
			os.MkdirAll(c.root, 0755)
			return false
		}
		c.logWarn("repaired %s from backup %s (stageAndSwap crash detected and recovered)", filepath.Base(c.root), backups[0])
		return true
	}

	// Multiple backups exist: emit warning with each path and recovery command
	c.logWarn("detected multiple backups of the %s directory (stageAndSwap may have crashed multiple times):", filepath.Base(c.root))
	for _, backup := range backups {
		c.logWarn("  - %s", backup)
		c.logWarn("    To recover: rm -rf %s && mv %s %s", c.root, backup, c.root)
	}
	// Ensure the store dir exists (empty) so Load() can continue
	if err := os.MkdirAll(c.root, 0755); err != nil {
		c.logWarn("failed to create %s directory: %v", filepath.Base(c.root), err)
	}
	return false
}

// Load reads all items from disk into memory.
func (c *Core) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.loadFromDisk()
}

// loadFromDisk reads all items from disk (must be called with lock held).
// Loads all .md files from the root directory and any subdirectories.
func (c *Core) loadFromDisk() error {
	// Migrate legacy directory names (worktrees/ → .worktrees/, conversations/ → .conversations/)
	c.migrateLegacyDirs()

	// Detect and repair .lif.bak-* / .beans.bak-* orphans left by interrupted stageAndSwap
	_ = c.detectOrphanBackup() // best-effort; continue loading even if repair failed

	// Clear existing items and dirty state
	c.items = make(map[string]*item.Item)
	c.dirty = make(map[string]bool)
	c.mainPaths = make(map[string]string)
	c.invalidateIncomingLocked()
	err := filepath.WalkDir(c.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip dot-prefixed subdirectories (e.g. .worktrees/, .conversations/)
		// — these contain non-item data and should never be walked. The
		// attachments dir is the one visible directory with the same
		// property: it carries an item's evidence artifacts, which include
		// markdown, and only its name keeps it out of the store (D24).
		// archive/ is NOT skipped, deliberately — an archived item stays
		// loaded and addressable.
		if d.IsDir() && path != c.root {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if path == filepath.Join(c.root, AttachmentsDir) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip non-.md files
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		b, loadErr := c.loadItem(path)
		if loadErr != nil {
			return fmt.Errorf("loading %s: %w", path, loadErr)
		}

		c.setItemLocked(b.ID, b)
		c.mainPaths[b.ID] = b.Path
		return nil
	})
	if err != nil {
		return err
	}

	// Resync the search index if it was active: diff against the freshly
	// reloaded items instead of closing and rebuilding from scratch, so a
	// reload only reindexes what actually changed (AC-02). A cached
	// shared, read-only handle is upgraded to writable first (beans-4t2m):
	// Sync itself would otherwise hang forever on one (see
	// search.OpenRead's doc comment).
	if c.searchIndex != nil {
		if err := c.ensureWritableSearchIndexLocked(); err != nil {
			c.logWarn("failed to upgrade search index before resync: %v", err)
		} else if err := c.searchIndex.Sync(c.allItemsLocked()); err != nil {
			c.logWarn("failed to resync search index after reload: %v", err)
		}
	}

	return nil
}

// loadItem reads and parses a single item file.
func (c *Core) loadItem(path string) (*item.Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := item.Parse(f)
	if err != nil {
		return nil, err
	}

	// Set metadata from path
	relPath, err := filepath.Rel(c.root, path)
	if err != nil {
		return nil, err
	}
	b.Path = relPath

	// Extract ID and slug from filename
	filename := filepath.Base(path)
	b.ID, b.Slug = item.ParseFilename(filename)

	// Apply defaults for GraphQL non-nullable fields
	if b.Type == "" {
		b.Type = "task"
	}
	if b.Priority == "" {
		b.Priority = "normal"
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	if b.Blocking == nil {
		b.Blocking = []string{}
	}
	if b.CreatedAt == nil {
		if b.UpdatedAt != nil {
			b.CreatedAt = b.UpdatedAt
		} else {
			// Use file modification time as fallback
			info, statErr := os.Stat(path)
			if statErr == nil {
				modTime := info.ModTime().UTC().Truncate(time.Second)
				b.CreatedAt = &modTime
			}
		}
	}
	if b.UpdatedAt == nil {
		b.UpdatedAt = b.CreatedAt
	}

	return b, nil
}

// allItemsLocked returns a snapshot slice of every in-memory item. Callers
// must already hold c.mu; the slice aliases the stored *item.Item pointers
// (the same items Sync's callers have always passed) rather than copying
// them.
func (c *Core) allItemsLocked() []*item.Item {
	allItems := make([]*item.Item, 0, len(c.items))
	for _, b := range c.items {
		allItems = append(allItems, b)
	}
	return allItems
}

// ensureSearchIndexLocked initializes the search index if not already
// created, or upgrades an already-cached shared, read-only handle to a
// writable one when write is true. Must be called with lock held or from a
// method that holds the lock. It prefers a persisted, cross-process index
// at c.indexDir() (AC-01, AC-04); on any error resolving that location,
// opening it, or acquiring its lock, it falls back to a private in-memory
// index rather than fail (AC-05, AC-07).
//
// write distinguishes the caller's intent (beans-dfdw, beans-4t2m): false
// opens the persisted index in shared mode on first use (search.OpenRead),
// letting concurrent readers each get the warm on-disk index instead of
// degrading (AC-03); true opens it exclusively on first use (search.Open,
// beans-6y60 AC-07), and upgrades an already-cached shared handle in place
// (see upgradeSearchIndexLocked) rather than ever writing to it directly.
//
// A shared-mode index is never Sync'd here unconditionally: Sync on a
// Bleve index opened read-only hangs forever rather than erroring, because
// its background persister never starts (see search.OpenRead's doc
// comment), and a just-opened persisted index is already populated, so
// there is normally nothing to sync anyway. Instead this function asks the
// freshly opened shared index itself whether the current in-memory items
// have moved on since its etags were last saved (search.Index.NeedsSync);
// only then does it upgrade to a writable handle and sync that. An
// in-memory fallback (from either open mode) is always writable, so it is
// still synced unconditionally, exactly as before beans-4t2m.
//
// Search, the read-only production caller, now safely passes false:
// c.searchIndex is cached for the rest of this Core's lifetime and later
// reused by Create/Update/Delete and the file watcher for real writes, but
// every one of those write paths calls ensureWritableSearchIndexLocked
// first, which routes back through upgradeSearchIndexLocked whenever the
// cached handle is still read-only. The cache can therefore start life as
// a shared handle and only pay the exclusive-reopen cost on the first
// actual write, instead of every Core paying it up front regardless of
// whether it ever writes.
func (c *Core) ensureSearchIndexLocked(write bool) error {
	if c.searchIndex != nil {
		if write && c.searchIndex.ReadOnly() {
			return c.upgradeSearchIndexLocked()
		}
		return nil
	}

	openPersisted := search.OpenRead
	if write {
		openPersisted = search.Open
	}

	var idx *search.Index
	if dir, err := c.indexDir(); err != nil {
		c.logWarn("resolving persisted search index location: %v", err)
	} else if pidx, err := openPersisted(dir); err != nil {
		c.logWarn("opening persisted search index: %v", err)
	} else {
		idx = pidx
		c.maintainIndexDir(dir)
	}

	if idx == nil {
		memIdx, err := search.NewIndex()
		if err != nil {
			return fmt.Errorf("initializing search index: %w", err)
		}
		idx = memIdx
	}

	c.searchIndex = idx

	if idx.ReadOnly() {
		if idx.NeedsSync(c.allItemsLocked()) {
			return c.upgradeSearchIndexLocked()
		}
		return nil
	}

	if err := idx.Sync(c.allItemsLocked()); err != nil {
		idx.Close()
		c.searchIndex = nil
		return fmt.Errorf("populating search index: %w", err)
	}

	return nil
}

// ensureWritableSearchIndexLocked upgrades c.searchIndex in place when it
// is currently a shared, read-only handle (search.Index.ReadOnly), so a
// caller about to write to it -- IndexItem, DeleteItem, or Sync -- never
// touches a read-only-opened Bleve index, which hangs on its first write
// (see search.OpenRead's doc comment) rather than erroring. Every write
// path (Create, Update, Delete, the file watcher's incremental
// maintenance, and loadFromDisk's resync) calls this before touching
// c.searchIndex, so a future write path can't forget the upgrade by
// construction. A nil index (never lazily initialized -- a write never
// forces that) is left nil: there is nothing to upgrade.
//
// Must be called with c.mu already held; it never itself locks or unlocks.
func (c *Core) ensureWritableSearchIndexLocked() error {
	if c.searchIndex == nil || !c.searchIndex.ReadOnly() {
		return nil
	}
	return c.upgradeSearchIndexLocked()
}

// upgradeSearchIndexLocked closes the current shared, read-only
// c.searchIndex -- releasing the shared lock -- and reopens the same
// directory exclusively via search.Open, syncs it to the current
// in-memory items, and caches the result back onto c.searchIndex. The
// shared lock MUST be released before the exclusive open is attempted, or
// this process would be contending against its own lock rather than
// another process's (see search.Open's doc comment); Close does exactly
// that. If the exclusive open degrades to an in-memory index because
// another process holds the lock, that is the existing, accepted AC-07
// behaviour: this function syncs the fallback and carries on rather than
// erroring.
//
// Must be called with c.mu already held; it never itself locks or unlocks.
func (c *Core) upgradeSearchIndexLocked() error {
	dir, err := c.indexDir()
	if err != nil {
		return fmt.Errorf("resolving persisted search index location for upgrade: %w", err)
	}

	if closeErr := c.searchIndex.Close(); closeErr != nil {
		c.logWarn("closing shared search index before upgrade: %v", closeErr)
	}

	idx, err := search.Open(dir)
	if err != nil {
		c.searchIndex = nil
		return fmt.Errorf("reopening search index for write: %w", err)
	}
	c.maintainIndexDir(dir)
	c.searchIndex = idx

	if err := idx.Sync(c.allItemsLocked()); err != nil {
		idx.Close()
		c.searchIndex = nil
		return fmt.Errorf("syncing upgraded search index: %w", err)
	}

	return nil
}

// Search performs full-text search and returns matching items. The search
// index is lazily initialized on first use, in shared read-only mode
// (write=false): that is finally safe (beans-4t2m), because every later
// write to this same cached c.searchIndex -- Create, Update, Delete, the
// file watcher, and loadFromDisk's resync -- upgrades it to a writable
// handle itself through ensureWritableSearchIndexLocked before ever
// calling IndexItem/DeleteItem/Sync on it, instead of Search having to
// pre-emptively open exclusively just in case this Core is later asked to
// write.
func (c *Core) Search(query string) ([]*item.Item, error) {
	// Ensure index is initialized (needs write lock for lazy init)
	c.mu.Lock()
	if err := c.ensureSearchIndexLocked(false); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	// Capture searchIndex reference while holding lock
	idx := c.searchIndex
	c.mu.Unlock()

	// Perform search outside the lock (Bleve is thread-safe)
	ids, err := idx.Search(query, search.DefaultSearchLimit)
	if err != nil {
		return nil, err
	}

	// Read from items map (needs read lock only)
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*item.Item, 0, len(ids))
	for _, id := range ids {
		if b, ok := c.items[id]; ok {
			result = append(result, b.Clone())
		}
	}
	return result, nil
}

// All returns a snapshot of all items, copied under the read lock.
//
// The copies are what makes the result safe to use: callers read the slice
// long after the lock is gone — the GraphQL query resolver and the
// subscription snapshot both serialize it outside the lock — while writers
// mutate the stored structs in place under the write lock (RemoveLinksTo,
// FixBrokenLinks, applyRenameSlug). Handing out the live pointers let a
// reader serialize an item mid-write.
func (c *Core) All() []*item.Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*item.Item, 0, len(c.items))
	for _, b := range c.items {
		result = append(result, b.Clone())
	}
	return result
}

// Get finds an item by exact ID match and returns a copy, safe to use after
// the lock is gone. If a prefix is configured and the query doesn't include
// it, the prefix is automatically prepended. For example, with prefix
// "beans-", Get("abc") will match "beans-abc" but Get("ab") will not.
//
// The copy is the contract: writers mutate the stored structs in place
// under the write lock (RemoveLinksTo, FixBrokenLinks, applyRenameSlug), so
// handing out the live pointer would let a caller observe an item mid-write
// or silently write through it into the store.
func (c *Core) Get(id string) (*item.Item, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Try exact match
	if b, ok := c.items[id]; ok {
		return b.Clone(), nil
	}

	// If not found and we have a configured prefix that isn't already in the query,
	// try with the prefix prepended (allows short IDs like "abc" to match "beans-abc")
	if c.config != nil && c.config.Items.Prefix != "" && !strings.HasPrefix(id, c.config.Items.Prefix) {
		if b, ok := c.items[c.config.Items.Prefix+id]; ok {
			return b.Clone(), nil
		}
	}

	return nil, ErrNotFound
}

// NormalizeID resolves a potentially short ID to its full form.
// If a prefix is configured and the query doesn't include it, the prefix is automatically prepended.
// Returns the full ID and true if found, or the original ID and false if not found.
func (c *Core) NormalizeID(id string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Try exact match
	if _, ok := c.items[id]; ok {
		return id, true
	}

	// If not found and we have a configured prefix that isn't already in the query,
	// try with the prefix prepended (allows short IDs like "abc" to match "beans-abc")
	if c.config != nil && c.config.Items.Prefix != "" && !strings.HasPrefix(id, c.config.Items.Prefix) {
		fullID := c.config.Items.Prefix + id
		if _, ok := c.items[fullID]; ok {
			return fullID, true
		}
	}

	return id, false
}

// UpdateOption configures optional behavior for Create/Update operations.
type UpdateOption func(*updateOptions)

type updateOptions struct {
	persist      bool   // whether to write to disk (default: true)
	worktreePath string // if set, write to this worktree's store dir instead of main
	extraSet     map[string]any
	extraUnset   []string
}

func defaultUpdateOptions() updateOptions {
	return updateOptions{persist: true}
}

// WithPersist controls whether the operation writes to disk.
// When false, the item is only updated in memory and marked as dirty.
func WithPersist(persist bool) UpdateOption {
	return func(o *updateOptions) {
		o.persist = persist
	}
}

// WithWorktreePath writes the item to a worktree's store directory instead of main.
// The item is kept dirty in runtime (not persisted to main).
func WithWorktreePath(path string) UpdateOption {
	return func(o *updateOptions) {
		o.worktreePath = path
	}
}

// WithExtraOps applies extra front matter writes as part of this persist: sets
// first, then unsets, matching the CLI's --set/--unset precedence. Required so
// a status change and the fields its status policy demands land in one file
// write and under one etag.
func WithExtraOps(set map[string]any, unset []string) UpdateOption {
	return func(o *updateOptions) {
		o.extraSet = set
		o.extraUnset = unset
	}
}

// IsDirty returns true if the item with the given ID has been modified in
// runtime but not yet persisted to disk.
func (c *Core) IsDirty(id string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dirty[id]
}

// DirtyIDs returns the IDs of all items that have unsaved changes.
func (c *Core) DirtyIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.dirty))
	for id := range c.dirty {
		ids = append(ids, id)
	}
	return ids
}

// HasDirty returns true if there are any unsaved item changes.
func (c *Core) HasDirty() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.dirty) > 0
}

// WorktreeForItem returns the worktree path linked to the given item ID,
// or empty string if the item is not linked to any worktree.
func (c *Core) WorktreeForItem(id string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.worktreeLinks[id]
}

// ItemsForWorktree returns the IDs of all items linked to the given worktree path.
func (c *Core) ItemsForWorktree(worktreePath string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var ids []string
	for id, wt := range c.worktreeLinks {
		if wt == worktreePath {
			ids = append(ids, id)
		}
	}
	return ids
}

// SaveDirty persists all dirty items to disk and clears their dirty flags.
// Returns the number of items saved.
func (c *Core) SaveDirty() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	saved := 0
	for id := range c.dirty {
		b, ok := c.items[id]
		if !ok {
			// Item was deleted after being dirtied, just clear flag
			delete(c.dirty, id)
			continue
		}

		if err := c.saveToDisk(b); err != nil {
			return saved, fmt.Errorf("saving item %s: %w", id, err)
		}

		delete(c.dirty, id)
		saved++
	}

	return saved, nil
}

// SaveItem persists a specific dirty item to disk and clears its dirty flag.
func (c *Core) SaveItem(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty[id] {
		return nil // Not dirty, nothing to do
	}

	b, ok := c.items[id]
	if !ok {
		delete(c.dirty, id)
		return ErrNotFound
	}

	if err := c.saveToDisk(b); err != nil {
		return fmt.Errorf("saving item %s: %w", id, err)
	}

	delete(c.dirty, id)
	return nil
}

// applyExtraOptions writes o.extraSet then removes o.extraUnset on b.Extra,
// allocating Extra on demand. A key named by both ends up unset.
func applyExtraOptions(b *item.Item, set map[string]any, unset []string) {
	if len(set) == 0 && len(unset) == 0 {
		return
	}
	if b.Extra == nil {
		b.Extra = make(map[string]any)
	}
	for k, v := range set {
		b.Extra[k] = v
	}
	for _, k := range unset {
		delete(b.Extra, k)
	}
}

// MissingRequiredFields returns the subset of fields that b.Extra does not
// carry with a non-blank value. Exported so that read-only callers — the
// check command and the batch preflight of the status verbs — can ask the
// same question the write path asks, instead of each keeping a copy of the
// rule that then drifts.
func MissingRequiredFields(b *item.Item, fields []string) []string {
	var missing []string
	for _, field := range fields {
		v, ok := b.Extra[field]
		if !ok || v == nil || strings.TrimSpace(fmt.Sprint(v)) == "" {
			missing = append(missing, field)
		}
	}
	return missing
}

// statusOnDisk reads the status of the file this item would be written to.
// ok is false when the file is missing, unreadable, or unparseable.
func (c *Core) statusOnDisk(relPath, worktreePath string) (status string, ok bool) {
	if relPath == "" {
		return "", false
	}
	var path string
	if worktreePath != "" {
		path = filepath.Join(c.worktreeItemsDir(worktreePath), relPath)
	} else {
		path = filepath.Join(c.root, relPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return statusOfContent(data)
}

// statusOfContent parses the status out of an item file's bytes.
// ok is false when the content is unparseable.
func statusOfContent(data []byte) (status string, ok bool) {
	b, err := item.Parse(bytes.NewReader(data))
	if err != nil {
		return "", false
	}
	return b.Status, true
}

// requiredFieldsFor is a nil-safe wrapper around c.config's status policy.
func (c *Core) requiredFieldsFor(status string) []string {
	if c.config == nil {
		return nil
	}
	return c.config.RequiredFieldsFor(status)
}

// Create adds a new item, generating an ID if needed.
// By default, persists to disk. Use WithPersist(false) to only update runtime state.
func (c *Core) Create(b *item.Item, opts ...UpdateOption) error {
	o := defaultUpdateOptions()
	for _, opt := range opts {
		opt(&o)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Generate ID if not provided
	if b.ID == "" {
		prefix := ""
		length := 4
		if c.config != nil {
			prefix = c.config.Items.Prefix
			if c.config.Items.IDLength > 0 {
				length = c.config.Items.IDLength
			}
		}
		id, err := item.NewID(prefix, length)
		if err != nil {
			return fmt.Errorf("generating item ID: %w", err)
		}
		b.ID = id
	}

	// Set timestamps
	now := time.Now().UTC().Truncate(time.Second)
	b.CreatedAt = &now
	b.UpdatedAt = &now

	applyExtraOptions(b, o.extraSet, o.extraUnset)

	if fields := c.requiredFieldsFor(b.Status); len(fields) > 0 {
		if missing := MissingRequiredFields(b, fields); len(missing) > 0 {
			return &PolicyViolationError{ItemID: b.ID, Status: b.Status, Missing: missing}
		}
	}

	if o.persist {
		// Write to disk
		if err := c.saveToDisk(b); err != nil {
			return err
		}
	} else {
		// Mark as dirty (runtime-only change)
		c.dirty[b.ID] = true
	}

	// Add to in-memory map
	c.setItemLocked(b.ID, b.Clone())

	// Update search index if active (best-effort, don't fail create).
	// ensureWritableSearchIndexLocked upgrades a cached shared, read-only
	// handle before this ever calls IndexItem, which would otherwise hang
	// forever on one (beans-4t2m).
	if c.searchIndex != nil {
		if err := c.ensureWritableSearchIndexLocked(); err != nil {
			c.logWarn("failed to upgrade search index for item %s: %v", b.ID, err)
		} else if err := c.searchIndex.IndexItem(b); err != nil {
			c.logWarn("failed to index item %s: %v", b.ID, err)
		}
	}

	return nil
}

// Update modifies an existing item.
// If ifMatch is provided, validates the current version's etag matches before updating.
// By default, persists to disk. Use WithPersist(false) to only update runtime state.
func (c *Core) Update(b *item.Item, ifMatch *string, opts ...UpdateOption) error {
	o := defaultUpdateOptions()
	for _, opt := range opts {
		opt(&o)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Verify item exists in memory
	storedItem, ok := c.items[b.ID]
	if !ok {
		return ErrNotFound
	}

	// Validate etag if provided or required
	requireIfMatch := c.config != nil && c.config.Items.RequireIfMatch

	if requireIfMatch && (ifMatch == nil || *ifMatch == "") {
		return &ETagRequiredError{}
	}

	// Auto-route to linked worktree if no explicit path given
	wtPath := o.worktreePath
	if wtPath == "" {
		wtPath = c.worktreeLinks[b.ID]
	}

	// The main-store file backs both checks below: the etag If-Match is
	// validated against, and the previous status the field policy is measured
	// from. On the common path — a write to main — both want the same bytes, so
	// read them once. Read them here rather than before taking the lock: a read
	// outside the lock leaves a window in which another writer replaces the
	// file, which would turn a stale If-Match into a silent accept — the exact
	// lost update the etag exists to prevent.
	//
	// The If-Match etag always comes from the main store, worktree write or
	// not; the status policy measures against whichever file is being written,
	// so under a worktree path it keeps reading the worktree copy below.
	var mainContent []byte
	var haveMainContent bool
	readsMainFile := storedItem.Path != "" && !c.dirty[b.ID]
	wantsETagFromDisk := ifMatch != nil && *ifMatch != ""
	wantsStatusFromDisk := wtPath == "" && len(c.requiredFieldsFor(b.Status)) > 0
	if readsMainFile && (wantsETagFromDisk || wantsStatusFromDisk) {
		if content, err := os.ReadFile(filepath.Join(c.root, storedItem.Path)); err == nil {
			mainContent, haveMainContent = content, true
		}
	}

	if wantsETagFromDisk {
		// storedItem is the pre-update stored copy: Get()/Create()/Update() now
		// clone on both ingress and egress, so the caller's mutations on b never
		// land in c.items[id] until the setItemLocked call below. storedItem.ETag()
		// therefore already describes the state being replaced — the definition
		// If-Match is checked against — without needing to re-read the file,
		// except where mainContent was read above for a different reason.
		currentETag := storedItem.ETag()
		if haveMainContent {
			// Same definition item.ETag uses.
			currentETag = item.ETagOf(mainContent)
		}

		if currentETag != *ifMatch {
			return &ETagMismatchError{
				Provided: *ifMatch,
				Current:  currentETag,
			}
		}
	}

	applyExtraOptions(b, o.extraSet, o.extraUnset)

	if fields := c.requiredFieldsFor(b.Status); len(fields) > 0 {
		var prev string
		var ok bool
		if haveMainContent && wtPath == "" {
			prev, ok = statusOfContent(mainContent)
		} else {
			prev, ok = c.statusOnDisk(storedItem.Path, wtPath)
		}
		if !ok || prev != b.Status {
			if missing := MissingRequiredFields(b, fields); len(missing) > 0 {
				return &PolicyViolationError{ItemID: b.ID, Status: b.Status, Missing: missing}
			}
		}
	}

	// Update timestamp
	now := time.Now().UTC().Truncate(time.Second)
	b.UpdatedAt = &now

	if wtPath != "" {
		// Write to the worktree's store dir; keep dirty in main
		if err := c.saveToWorktree(b, wtPath); err != nil {
			return err
		}
		c.dirty[b.ID] = true
	} else if o.persist {
		// Write to disk
		if err := c.saveToDisk(b); err != nil {
			return err
		}
		// Clear dirty flag since we just persisted
		delete(c.dirty, b.ID)
	} else {
		// Mark as dirty (runtime-only change)
		c.dirty[b.ID] = true
	}

	// Update in-memory map
	c.setItemLocked(b.ID, b.Clone())

	// Update search index if active (best-effort, don't fail update).
	// ensureWritableSearchIndexLocked upgrades a cached shared, read-only
	// handle before this ever calls IndexItem, which would otherwise hang
	// forever on one (beans-4t2m).
	if c.searchIndex != nil {
		if err := c.ensureWritableSearchIndexLocked(); err != nil {
			c.logWarn("failed to upgrade search index for item %s: %v", b.ID, err)
		} else if err := c.searchIndex.IndexItem(b); err != nil {
			c.logWarn("failed to update item %s in search index: %v", b.ID, err)
		}
	}

	return nil
}

// saveToDisk writes an item to the filesystem.
func (c *Core) saveToDisk(b *item.Item) error {
	// Determine the file path
	var path string
	if b.Path != "" {
		path = filepath.Join(c.root, b.Path)
	} else {
		filename := item.BuildFilename(b.ID, b.Slug)
		path = filepath.Join(c.root, filename)
		b.Path = filename
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Render and write
	content, err := b.Render()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	b.SetContentETag(content)
	c.mainPaths[b.ID] = b.Path

	return nil
}

// saveToWorktree writes an item to the store directory inside a worktree.
func (c *Core) saveToWorktree(b *item.Item, worktreePath string) error {
	itemsDir := c.worktreeItemsDir(worktreePath)

	// Ensure the store directory exists in the worktree
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		return fmt.Errorf("creating worktree items dir: %w", err)
	}

	filename := item.BuildFilename(b.ID, b.Slug)
	path := filepath.Join(itemsDir, filename)

	content, err := b.Render()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("writing worktree item: %w", err)
	}
	b.SetContentETag(content)

	return nil
}

// Delete removes an item by exact ID match.
// Supports short IDs (without prefix) if a prefix is configured.
func (c *Core) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the item by exact match
	targetID := id
	targetItem, ok := c.items[id]

	// If not found and we have a configured prefix, try with prefix prepended
	if !ok && c.config != nil && c.config.Items.Prefix != "" && !strings.HasPrefix(id, c.config.Items.Prefix) {
		fullID := c.config.Items.Prefix + id
		if b, found := c.items[fullID]; found {
			targetID = fullID
			targetItem = b
			ok = true
		}
	}

	if !ok {
		return ErrNotFound
	}

	// Remove from disk
	path := filepath.Join(c.root, targetItem.Path)
	if err := os.Remove(path); err != nil {
		return err
	}

	// The attachment directory goes with the item. Leaving it behind makes
	// every deletion a `lif check` orphan finding, and under the current
	// sink that directory carries the container's design documents rather
	// than a review JSON. Git holds the history for whoever needs it.
	if err := os.RemoveAll(filepath.Join(c.root, AttachmentsDir, targetID)); err != nil {
		return err
	}

	// Remove from in-memory map
	c.removeItemLocked(targetID)
	delete(c.mainPaths, targetID)

	// Update search index if active (best-effort, don't fail delete).
	// ensureWritableSearchIndexLocked upgrades a cached shared, read-only
	// handle before this ever calls DeleteItem, which would otherwise hang
	// forever on one (beans-4t2m).
	if c.searchIndex != nil {
		if err := c.ensureWritableSearchIndexLocked(); err != nil {
			c.logWarn("failed to upgrade search index for item %s: %v", targetID, err)
		} else if err := c.searchIndex.DeleteItem(targetID); err != nil {
			c.logWarn("failed to remove item %s from search index: %v", targetID, err)
		}
	}

	return nil
}

// Archive moves an item to the archive directory.
// Supports short IDs (without prefix) if a prefix is configured.
func (c *Core) Archive(id string) error {
	c.mu.Lock()

	// Find the item
	targetItem, targetID, err := c.findItemLocked(id)
	if err != nil {
		c.mu.Unlock()
		return err
	}

	// Check if already archived
	if c.isArchivedPath(targetItem.Path) {
		c.mu.Unlock()
		return nil // Already archived, nothing to do
	}

	// Ensure archive directory exists
	archivePath := filepath.Join(c.root, ArchiveDir)
	if err := os.MkdirAll(archivePath, 0755); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("creating archive directory: %w", err)
	}

	// Move the file
	oldPath := filepath.Join(c.root, targetItem.Path)
	newRelPath := filepath.Join(ArchiveDir, filepath.Base(targetItem.Path))
	newPath := filepath.Join(c.root, newRelPath)

	if err := os.Rename(oldPath, newPath); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("moving item to archive: %w", err)
	}

	// Update item's path in store and notify subscribers
	targetItem.Path = newRelPath
	c.setItemLocked(targetID, targetItem)
	c.mainPaths[targetID] = newRelPath
	// Copy while the write lock is still held: fanOut runs unlocked, so a
	// copy taken there would race the writers it exists to protect.
	eventItem := targetItem.Clone()
	c.mu.Unlock()

	c.fanOut([]ItemEvent{{
		Type:   EventUpdated,
		Item:   eventItem,
		ItemID: targetID,
	}})

	return nil
}

// Unarchive moves an item from the archive directory back to the main directory.
// Supports short IDs (without prefix) if a prefix is configured.
func (c *Core) Unarchive(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the item
	targetItem, targetID, err := c.findItemLocked(id)
	if err != nil {
		return err
	}

	// Check if not archived
	if !c.isArchivedPath(targetItem.Path) {
		return nil // Not archived, nothing to do
	}

	// Move the file back to main directory
	oldPath := filepath.Join(c.root, targetItem.Path)
	newRelPath := filepath.Base(targetItem.Path)
	newPath := filepath.Join(c.root, newRelPath)

	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("moving item from archive: %w", err)
	}

	// Update item's path
	targetItem.Path = newRelPath
	c.setItemLocked(targetID, targetItem)
	c.mainPaths[targetID] = newRelPath

	return nil
}

// IsArchived returns true if the item with the given ID is in the archive.
// Supports short IDs (without prefix) if a prefix is configured.
func (c *Core) IsArchived(id string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	b, _, err := c.findItemLocked(id)
	if err != nil {
		return false
	}

	return c.isArchivedPath(b.Path)
}

// isArchivedPath returns true if the path indicates an archived item.
func (c *Core) isArchivedPath(path string) bool {
	return strings.HasPrefix(path, ArchiveDir+string(filepath.Separator)) ||
		strings.HasPrefix(path, ArchiveDir+"/")
}

// normalizeID returns the full ID with prefix if a prefix is configured
// and the ID doesn't already have it.
func (c *Core) normalizeID(id string) string {
	if c.config != nil && c.config.Items.Prefix != "" && !strings.HasPrefix(id, c.config.Items.Prefix) {
		return c.config.Items.Prefix + id
	}
	return id
}

// findItemLocked finds an item by ID, supporting short IDs.
// Must be called with lock held.
func (c *Core) findItemLocked(id string) (*item.Item, string, error) {
	// Try exact match
	if b, ok := c.items[id]; ok {
		return b, id, nil
	}

	// Try with prefix prepended
	fullID := c.normalizeID(id)
	if fullID != id {
		if b, ok := c.items[fullID]; ok {
			return b, fullID, nil
		}
	}

	return nil, "", ErrNotFound
}

// GetFromArchive loads an item directly from the archive directory.
// This is used when an item isn't in the main loaded set but might be archived.
// Returns nil, nil if the archive directory doesn't exist or item not found.
func (c *Core) GetFromArchive(id string) (*item.Item, error) {
	fullID := c.normalizeID(id)

	archiveDir := filepath.Join(c.root, ArchiveDir)
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		return nil, nil
	}

	// Look for the item file in the archive
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		fileID, _ := item.ParseFilename(entry.Name())
		if fileID == fullID {
			path := filepath.Join(archiveDir, entry.Name())
			return c.loadItem(path)
		}
	}

	return nil, nil
}

// LoadAndUnarchive finds an item in the archive, loads it, unarchives it,
// and adds it to the in-memory store. Returns the item or ErrNotFound.
func (c *Core) LoadAndUnarchive(id string) (*item.Item, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the item (always loaded since we now include archived items)
	b, targetID, err := c.findItemLocked(id)
	if err != nil {
		return nil, ErrNotFound
	}

	// If already in main directory, just return it
	if !c.isArchivedPath(b.Path) {
		return b.Clone(), nil
	}

	// Move file from archive to main directory
	oldPath := filepath.Join(c.root, b.Path)
	newRelPath := filepath.Base(b.Path)
	newPath := filepath.Join(c.root, newRelPath)

	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, fmt.Errorf("moving item from archive: %w", err)
	}

	// Update item's path
	b.Path = newRelPath
	c.mainPaths[targetID] = newRelPath
	c.setItemLocked(targetID, b)

	return b.Clone(), nil
}

// Init creates the .beans directory if it doesn't exist,
// and writes a .gitignore to exclude generated content.
func (c *Core) Init() error {
	if err := os.MkdirAll(c.root, 0755); err != nil {
		return err
	}
	return writeGitignore(c.root)
}

// FullPath returns the absolute path to an item file.
func (c *Core) FullPath(b *item.Item) string {
	return filepath.Join(c.root, b.Path)
}

// Close stops any active file watcher and cleans up resources.
func (c *Core) Close() error {
	// Stop worktree watchers first (outside main lock to avoid deadlock)
	c.UnwatchAllWorktrees()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Close search index if open
	if c.searchIndex != nil {
		if err := c.searchIndex.Close(); err != nil {
			return err
		}
		c.searchIndex = nil
	}

	return c.unwatchLocked()
}

// ValidatePrefixConsistency checks if the configured prefix matches the actual
// prefixes present in loaded items. Returns an error message if a mismatch is found.
// If config prefix is empty, all loaded items must also have no prefix (or be unprefixed).
// If config prefix is set, all loaded items should start with that prefix.
func (c *Core) ValidatePrefixConsistency() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	configPrefix := ""
	configKey := config.SectionKey
	if c.config != nil {
		configPrefix = c.config.Items.Prefix
		configKey = c.config.SectionKey()
	}

	// Extract, for each loaded, non-archived item, the prefix it should be
	// grouped under. Archived items are historical/frozen (project memory,
	// cf. Archive()) and may carry a prefix that predates a rename or,
	// occasionally, belong to an item that was physically copied in from
	// another store; neither case is a live consistency defect of this
	// store, so archived items never contribute to the detected prefix set.
	//
	// An item is consistent with the configured prefix iff configPrefix is a
	// literal string prefix of its ID (item.IDSuffix/RebrandID answer the
	// same question for `rename --prefix`); everything after that prefix,
	// hyphens included, is the suffix. So an item matching configPrefix is
	// always grouped under configPrefix itself, regardless of how many
	// hyphens its hand-authored semantic suffix contains (e.g.
	// "SEL-login-errors" under prefix "SEL-"). Only items that do NOT match
	// configPrefix fall back to a naive split at the last hyphen, purely to
	// produce a readable label for a foreign/legacy ID scheme we don't
	// otherwise understand.
	prefixesSeen := make(map[string]bool)
	for itemID, b := range c.items {
		if c.isArchivedPath(b.Path) {
			continue
		}
		if configPrefix != "" && strings.HasPrefix(itemID, configPrefix) {
			prefixesSeen[configPrefix] = true
			continue
		}
		parts := strings.Split(itemID, "-")
		if len(parts) > 1 {
			// Take everything except the last part as the prefix
			detectedPrefix := strings.Join(parts[:len(parts)-1], "-")
			if detectedPrefix != "" {
				prefixesSeen[detectedPrefix+"-"] = true
			}
		} else {
			// No dash, so no prefix
			prefixesSeen[""] = true
		}
	}

	// If no items loaded, no inconsistency
	if len(prefixesSeen) == 0 {
		return ""
	}

	// Check for mixed-prefix state
	if len(prefixesSeen) > 1 {
		var prefixes []string
		for p := range prefixesSeen {
			prefixes = append(prefixes, p)
		}
		return fmt.Sprintf("mixed-prefix state detected: items have prefixes %v but config.%s.prefix is %q", prefixes, configKey, configPrefix)
	}

	// Check if the single prefix matches the config
	var actualPrefix string
	for p := range prefixesSeen {
		actualPrefix = p
		break
	}

	if actualPrefix != configPrefix {
		return fmt.Sprintf("prefix mismatch: items have prefix %q but config.%s.prefix is %q", actualPrefix, configKey, configPrefix)
	}

	return ""
}

// writeGitignore creates or overwrites a .gitignore in the items directory
// to exclude conversation logs from version control.
// Note: worktrees are stored outside the repo (in ~/.lif/worktrees/<project>/,
// or ~/.beans/worktrees/<project>/ for a project that already has that root).
func writeGitignore(itemsDir string) error {
	content := "# Generated by lif init\n.conversations/\n"
	return os.WriteFile(filepath.Join(itemsDir, ".gitignore"), []byte(content), 0644)
}

// migrateLegacyDirs renames old-style directories (worktrees/, conversations/)
// to their dot-prefixed equivalents (.worktrees/, .conversations/). Best-effort.
func (c *Core) migrateLegacyDirs() {
	for _, name := range []string{"worktrees", "conversations"} {
		oldPath := filepath.Join(c.root, name)
		newPath := filepath.Join(c.root, "."+name)
		if _, err := os.Stat(oldPath); err == nil {
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				_ = os.Rename(oldPath, newPath)
			}
		}
	}
}
