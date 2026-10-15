package itemcore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rierek/lif/pkg/item"
)

// worktreeWatcher tracks a single worktree's store directory.
type worktreeWatcher struct {
	worktreePath string // root of the worktree (e.g., ~/.lif/worktrees/<project>/<id>)
	itemsDir     string // store dir inside the worktree
	done         chan struct{}
}

// WatchWorktreeItems starts watching a worktree's store directory for item changes.
// When items change in the worktree, they are merged into the runtime state as dirty
// (not persisted to the main repo's disk).
// Returns nil if the worktree's store directory doesn't exist.
func (c *Core) WatchWorktreeItems(worktreePath string) error {
	itemsDir := c.worktreeItemsDir(worktreePath)

	// Check if the worktree has a store directory
	if _, err := os.Stat(itemsDir); os.IsNotExist(err) {
		return nil // No store dir in this worktree, nothing to watch
	}

	c.mu.Lock()
	// Check if already watching this worktree
	if c.worktreeWatchers == nil {
		c.worktreeWatchers = make(map[string]*worktreeWatcher)
	}
	if _, exists := c.worktreeWatchers[worktreePath]; exists {
		c.mu.Unlock()
		return nil // Already watching
	}

	wt := &worktreeWatcher{
		worktreePath: worktreePath,
		itemsDir:     itemsDir,
		done:         make(chan struct{}),
	}
	c.worktreeWatchers[worktreePath] = wt
	c.mu.Unlock()

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.mu.Lock()
		delete(c.worktreeWatchers, worktreePath)
		c.mu.Unlock()
		return err
	}

	if err := watcher.Add(itemsDir); err != nil {
		watcher.Close()
		c.mu.Lock()
		delete(c.worktreeWatchers, worktreePath)
		c.mu.Unlock()
		return err
	}

	// Also watch subdirectories (like archive/)
	_ = filepath.WalkDir(itemsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == itemsDir {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		_ = watcher.Add(path)
		return nil
	})

	// Initial scan: load existing item files from the worktree into runtime state.
	// Without this, items created in worktrees won't be resolvable via Get()
	// until the watcher picks up a change event.
	c.loadWorktreeItemsInitial(wt)

	// Start the watcher goroutine
	go c.worktreeWatchLoop(wt, watcher)

	c.logWarn("watching worktree items: %s", itemsDir)
	return nil
}

// loadWorktreeItemsInitial scans a worktree's store directory and loads
// item files that differ from the main version into runtime state.
// Only items that are new or modified compared to main are loaded and linked.
func (c *Core) loadWorktreeItemsInitial(wt *worktreeWatcher) {
	entries, err := os.ReadDir(wt.itemsDir)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(wt.itemsDir, entry.Name())
		newItem, err := c.loadItemFrom(path, wt.itemsDir)
		if err != nil {
			continue
		}

		// Only load and link if the item differs from the main version.
		// Worktrees branch from main, so most items are identical copies.
		if existing, exists := c.items[newItem.ID]; exists {
			if existing.ETag() == newItem.ETag() {
				continue // Same content as main — skip
			}
		}

		c.setItemLocked(newItem.ID, newItem)
		c.dirty[newItem.ID] = true
		c.worktreeLinks[newItem.ID] = wt.worktreePath

		if c.searchIndex != nil {
			_ = c.searchIndex.IndexItem(newItem)
		}
	}
}

// UnwatchWorktreeItems stops watching a worktree's store directory.
func (c *Core) UnwatchWorktreeItems(worktreePath string) {
	c.mu.Lock()
	wt, exists := c.worktreeWatchers[worktreePath]
	if exists {
		delete(c.worktreeWatchers, worktreePath)
		// Clear all worktree links for this worktree
		for id, wtp := range c.worktreeLinks {
			if wtp == worktreePath {
				delete(c.worktreeLinks, id)
			}
		}
	}
	c.mu.Unlock()

	if exists {
		close(wt.done)
		c.logWarn("stopped watching worktree items: %s", wt.itemsDir)
	}
}

// UnwatchAllWorktrees stops watching all worktree store directories.
func (c *Core) UnwatchAllWorktrees() {
	c.mu.Lock()
	watchers := c.worktreeWatchers
	c.worktreeWatchers = make(map[string]*worktreeWatcher)
	c.worktreeLinks = make(map[string]string)
	c.mu.Unlock()

	for _, wt := range watchers {
		close(wt.done)
	}
}

// worktreeWatchLoop processes filesystem events from a worktree with debouncing.
func (c *Core) worktreeWatchLoop(wt *worktreeWatcher, watcher *fsnotify.Watcher) {
	defer watcher.Close()

	var debounceTimer *time.Timer
	var pendingMu sync.Mutex
	pendingChanges := make(map[string]fsnotify.Op)

	for {
		select {
		case <-wt.done:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only care about .md files
			if !strings.HasSuffix(event.Name, ".md") {
				continue
			}

			// Verify the file is within the worktree's store directory
			relPath, err := filepath.Rel(wt.itemsDir, event.Name)
			if err != nil || strings.HasPrefix(relPath, "..") {
				continue
			}

			// Skip events from dot-prefixed subdirectories
			if topDir, _, ok := strings.Cut(relPath, string(filepath.Separator)); ok && strings.HasPrefix(topDir, ".") {
				continue
			}

			// Check if this is a relevant event
			relevant := event.Op&fsnotify.Create != 0 ||
				event.Op&fsnotify.Write != 0 ||
				event.Op&fsnotify.Remove != 0 ||
				event.Op&fsnotify.Rename != 0

			if !relevant {
				continue
			}

			pendingMu.Lock()
			pendingChanges[event.Name] |= event.Op
			pendingMu.Unlock()

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				pendingMu.Lock()
				changes := pendingChanges
				pendingChanges = make(map[string]fsnotify.Op)
				pendingMu.Unlock()

				c.handleWorktreeChanges(wt, changes)
			})

		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// handleWorktreeChanges processes item file changes from a worktree.
// Creates/writes are merged into runtime state as dirty.
// Deletes either revert to the main-repo version or remove the item from runtime.
func (c *Core) handleWorktreeChanges(wt *worktreeWatcher, changes map[string]fsnotify.Op) {
	if len(changes) == 0 {
		return
	}

	c.mu.Lock()

	var events []ItemEvent

	for path, op := range changes {
		isDelete := op&fsnotify.Remove != 0 || op&fsnotify.Rename != 0
		isCreateOrWrite := op&fsnotify.Create != 0 || op&fsnotify.Write != 0

		if isDelete && !isCreateOrWrite {
			// File was deleted in the worktree. Either revert to the
			// main-repo version or remove the item from runtime.
			filename := filepath.Base(path)
			id, _ := item.ParseFilename(filename)
			if id == "" {
				continue
			}

			// Try to load from the main store directory.
			// The filename may differ (different slug), so search by ID.
			mainPath := c.findMainItemFile(id)
			if mainPath != "" {
				mainItem, err := c.loadItemFrom(mainPath, c.root)
				if err != nil {
					continue
				}
				// Item exists in main — revert to that version
				c.setItemLocked(id, mainItem)
				delete(c.dirty, id)
				delete(c.worktreeLinks, id)

				if c.searchIndex != nil {
					_ = c.searchIndex.IndexItem(mainItem)
				}

				events = append(events, ItemEvent{
					Type:   EventUpdated,
					Item:   mainItem.Clone(),
					ItemID: id,
				})
			} else if _, existed := c.items[id]; existed {
				// Item was worktree-only — remove from runtime
				c.removeItemLocked(id)
				delete(c.dirty, id)
				delete(c.worktreeLinks, id)

				if c.searchIndex != nil {
					_ = c.searchIndex.DeleteItem(id)
				}

				events = append(events, ItemEvent{
					Type:   EventDeleted,
					ItemID: id,
				})
			}
			continue
		}

		if !isCreateOrWrite {
			continue
		}

		// Check the file still exists (handles rapid create+delete)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		newItem, err := c.loadItemFrom(path, wt.itemsDir)
		if err != nil {
			c.logWarn("failed to load worktree item from %s: %v", path, err)
			continue
		}

		// Compare against the main repo's version on disk.
		// If they match (e.g. after a rebase), don't link — this item
		// isn't actually modified in the worktree.
		mainPath := c.findMainItemFile(newItem.ID)
		if mainPath != "" {
			mainItem, mainErr := c.loadItemFrom(mainPath, c.root)
			if mainErr == nil && mainItem.ETag() == newItem.ETag() {
				// Worktree version matches main — clear any stale link
				if _, wasLinked := c.worktreeLinks[newItem.ID]; wasLinked {
					c.setItemLocked(newItem.ID, mainItem)
					delete(c.dirty, newItem.ID)
					delete(c.worktreeLinks, newItem.ID)
					if c.searchIndex != nil {
						_ = c.searchIndex.IndexItem(mainItem)
					}
					events = append(events, ItemEvent{
						Type:   EventUpdated,
						Item:   mainItem.Clone(),
						ItemID: newItem.ID,
					})
				}
				continue
			}
		}

		_, existed := c.items[newItem.ID]
		c.setItemLocked(newItem.ID, newItem)
		c.dirty[newItem.ID] = true // Mark as dirty — came from worktree, not persisted to main
		c.worktreeLinks[newItem.ID] = wt.worktreePath

		if c.searchIndex != nil {
			if err := c.searchIndex.IndexItem(newItem); err != nil {
				c.logWarn("failed to index worktree item %s: %v", newItem.ID, err)
			}
		}

		if existed {
			events = append(events, ItemEvent{
				Type:   EventUpdated,
				Item:   newItem.Clone(),
				ItemID: newItem.ID,
			})
		} else {
			events = append(events, ItemEvent{
				Type:   EventCreated,
				Item:   newItem.Clone(),
				ItemID: newItem.ID,
			})
		}
	}

	c.mu.Unlock()

	// Fan out events to subscribers
	c.fanOut(events)

	// Notify worktree manager so the worktree subscription re-emits
	// with updated detected item IDs
	if len(events) > 0 && c.onWorktreeItemsChanged != nil {
		c.onWorktreeItemsChanged()
	}
}

// findMainItemFile returns the full path of the main store's file for the given
// item ID, or an empty string if the main store does not hold it. The answer
// comes from the in-memory mainPaths index — a worktree overlay leaves that
// index alone, so this stays correct while a worktree version is active, and it
// covers items in subdirectories, which a flat directory scan missed.
// Must be called with c.mu held.
func (c *Core) findMainItemFile(id string) string {
	relPath, ok := c.mainPaths[id]
	if !ok {
		return ""
	}
	path := filepath.Join(c.root, relPath)
	if _, err := os.Stat(path); err != nil {
		// The index outlived the file (deleted outside the watcher's view).
		delete(c.mainPaths, id)
		return ""
	}
	return path
}

// loadItemFrom reads and parses an item file, calculating its relative path from the given root.
func (c *Core) loadItemFrom(path string, root string) (*item.Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := item.Parse(f)
	if err != nil {
		return nil, err
	}

	// Set metadata from path (relative to the root, not the worktree)
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	b.Path = relPath

	// Extract ID and slug from filename
	filename := filepath.Base(path)
	b.ID, b.Slug = item.ParseFilename(filename)

	// Apply defaults
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
