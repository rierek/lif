package itemcore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rierek/lif/pkg/item"
)

const debounceDelay = 100 * time.Millisecond

// EventType represents the type of change that occurred to an item.
type EventType int

const (
	// EventCreated indicates a new item was created.
	EventCreated EventType = iota
	// EventUpdated indicates an existing item was modified.
	EventUpdated
	// EventDeleted indicates an item was deleted.
	EventDeleted
	// EventResync tells a subscriber that events were dropped because its
	// channel was full, and that it has to reload the full state instead of
	// trusting the increments it has seen so far.
	EventResync
)

// String returns a human-readable representation of the event type.
func (e EventType) String() string {
	switch e {
	case EventCreated:
		return "created"
	case EventUpdated:
		return "updated"
	case EventDeleted:
		return "deleted"
	case EventResync:
		return "resync"
	default:
		return "unknown"
	}
}

// ItemEvent represents a change to an item.
type ItemEvent struct {
	Type   EventType  // The type of change
	Item   *item.Item // The item (nil for Deleted events)
	ItemID string     // Always set, useful for Deleted when Item is nil
}

// subscription represents a subscriber to item events.
type subscription struct {
	ch chan []ItemEvent
	id uint64
	// needsResync is set when a fan-out had to drop this subscriber's events.
	// The next fan-out that finds room leads with an EventResync so the
	// subscriber knows its incremental state is no longer trustworthy.
	needsResync atomic.Bool
}

// Subscribe creates a new subscription to item change events.
// Returns the event channel and an unsubscribe function.
// The channel receives batches of events after debouncing.
// Callers should use defer to call the unsubscribe function.
func (c *Core) Subscribe() (<-chan []ItemEvent, func()) {
	c.subMu.Lock()
	defer c.subMu.Unlock()

	id := atomic.AddUint64(&c.nextSubID, 1)
	ch := make(chan []ItemEvent, 16)

	sub := &subscription{ch: ch, id: id}
	c.subscribers[id] = sub

	unsubscribe := func() {
		c.subMu.Lock()
		defer c.subMu.Unlock()
		if _, ok := c.subscribers[id]; ok {
			close(ch)
			delete(c.subscribers, id)
		}
	}

	return ch, unsubscribe
}

// fanOut sends events to all subscribers (non-blocking).
// A subscriber whose channel is full has its events dropped rather than
// blocking the others, and is marked for resync: the next fan-out that finds
// room leads with an EventResync, so a client that missed events reloads its
// state instead of drifting on top of an incomplete history.
func (c *Core) fanOut(events []ItemEvent) {
	if len(events) == 0 {
		return
	}

	c.subMu.RLock()
	defer c.subMu.RUnlock()

	for _, sub := range c.subscribers {
		if sub.needsResync.Load() {
			select {
			case sub.ch <- []ItemEvent{{Type: EventResync}}:
				sub.needsResync.Store(false)
			default:
				// Still full — keep the mark and skip this batch, which the
				// subscriber will recover from the resync snapshot anyway.
				continue
			}
		}

		select {
		case sub.ch <- events:
			// Sent successfully
		default:
			// Subscriber is slow: drop this batch and get a resync marker into
			// its buffer, evicting one queued batch to make room. Waiting for
			// the next fan-out would strand a client that fell behind on the
			// last burst of a busy period, and the evicted batch costs nothing
			// — the client replaces its state from the snapshot anyway.
			sub.needsResync.Store(true)
			select {
			case <-sub.ch:
			default:
			}
			select {
			case sub.ch <- []ItemEvent{{Type: EventResync}}:
				sub.needsResync.Store(false)
			default:
			}
			c.logWarn("subscriber %d: event channel full, dropped %d events, resync sent", sub.id, len(events))
		}
	}
}

// StartWatching begins filesystem monitoring.
// Use Subscribe() to receive item change events via a channel.
// This is the preferred API for new code; Watch() is kept for backward compatibility.
func (c *Core) StartWatching() error {
	return c.Watch(nil)
}

// Watch starts watching the .beans directory for changes.
// The onChange callback is invoked (after debouncing) whenever items are created, modified, or deleted.
// The internal state is automatically reloaded before the callback is invoked.
// Deprecated: Use StartWatching() + Subscribe() for new code.
func (c *Core) Watch(onChange func()) error {
	c.mu.Lock()
	if c.watching {
		c.mu.Unlock()
		return nil // Already watching
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.mu.Unlock()
		return err
	}

	if err := watcher.Add(c.root); err != nil {
		watcher.Close()
		c.mu.Unlock()
		return err
	}

	// Watch all subdirectories (best effort - don't fail if any can't be watched)
	// Skip dot-prefixed subdirectories (e.g. .worktrees/, .conversations/)
	_ = filepath.WalkDir(c.root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == c.root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		_ = watcher.Add(path)
		return nil
	})

	c.watching = true
	c.done = make(chan struct{})
	c.onChange = onChange
	c.mu.Unlock()

	// Start the watcher goroutine
	go c.watchLoop(watcher)

	return nil
}

// Unwatch stops watching the .beans directory.
func (c *Core) Unwatch() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.unwatchLocked()
}

// unwatchLocked stops watching (must be called with lock held).
func (c *Core) unwatchLocked() error {
	if !c.watching {
		return nil
	}

	close(c.done)
	c.watching = false
	c.onChange = nil

	// Close all subscriber channels
	c.subMu.Lock()
	for id, sub := range c.subscribers {
		close(sub.ch)
		delete(c.subscribers, id)
	}
	c.subMu.Unlock()

	return nil
}

// watchLoop processes filesystem events with debouncing.
func (c *Core) watchLoop(watcher *fsnotify.Watcher) {
	defer watcher.Close()

	var debounceTimer *time.Timer
	var pendingMu sync.Mutex
	pendingChanges := make(map[string]fsnotify.Op)

	for {
		select {
		case <-c.done:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only care about .md files within the .beans directory tree
			if !strings.HasSuffix(event.Name, ".md") {
				continue
			}

			// Verify the file is within the .beans directory
			relPath, err := filepath.Rel(c.root, event.Name)
			if err != nil || strings.HasPrefix(relPath, "..") {
				continue
			}

			// Skip events from dot-prefixed subdirectories (e.g. .worktrees/, .conversations/)
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

			// Accumulate changes during debounce window
			pendingMu.Lock()
			pendingChanges[event.Name] |= event.Op
			pendingMu.Unlock()

			// Start/reset debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				// Swap out pending changes atomically
				pendingMu.Lock()
				changes := pendingChanges
				pendingChanges = make(map[string]fsnotify.Op)
				pendingMu.Unlock()

				c.handleChanges(changes)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			// Log errors but continue watching
			_ = err // In production, you might want to log this
		}
	}
}

// handleChanges processes only the files that changed, updating state incrementally.
func (c *Core) handleChanges(changes map[string]fsnotify.Op) {
	if len(changes) == 0 {
		return
	}

	c.mu.Lock()

	// Check if we're still watching
	if !c.watching {
		c.mu.Unlock()
		return
	}

	var events []ItemEvent

	for path, op := range changes {
		filename := filepath.Base(path)
		id, _ := item.ParseFilename(filename)

		// Handle removes/renames (file is gone)
		if op&fsnotify.Remove != 0 || op&fsnotify.Rename != 0 {
			if _, exists := c.items[id]; exists {
				// Only delete if it was in our map and file is actually gone
				if !c.fileExists(path) {
					c.removeItemLocked(id)
					delete(c.mainPaths, id)

					// on one.
					if c.searchIndex != nil {
						if err := c.searchIndex.DeleteItem(id); err != nil {
							c.logWarn("failed to remove item %s from search index: %v", id, err)
						}
					}

					events = append(events, ItemEvent{
						Type:   EventDeleted,
						Item:   nil,
						ItemID: id,
					})

					// File is truly gone, skip Write/Create handling
					continue
				}
			}
			// File still exists (e.g., Remove+Write in same batch), fall through
			// to Write/Create handler below
		}

		// Handle creates/writes (file exists or was created)
		if op&fsnotify.Create != 0 || op&fsnotify.Write != 0 {
			newItem, err := c.loadItem(path)
			if err != nil {
				c.logWarn("failed to load item from %s: %v", path, err)
				continue
			}

			_, existed := c.items[newItem.ID]
			c.setItemLocked(newItem.ID, newItem)
			c.mainPaths[newItem.ID] = newItem.Path
			delete(c.dirty, newItem.ID) // Disk is now up-to-date

			if c.searchIndex != nil {
				if err := c.searchIndex.IndexItem(newItem); err != nil {
					c.logWarn("failed to index item %s: %v", newItem.ID, err)
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
	}

	callback := c.onChange
	c.mu.Unlock()

	// Fan out to subscribers (outside lock)
	c.fanOut(events)

	// Invoke legacy callback
	if callback != nil {
		callback()
	}
}

// fileExists checks if a file exists at the given path.
func (c *Core) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
