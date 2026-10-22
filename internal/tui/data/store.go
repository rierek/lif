package data

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/internal/storeroot"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// ErrConflict is wrapped by every mutation when the caller's ETag no longer
// matches the item on disk (optimistic locking). Callers errors.Is against it
// and reload rather than retry blindly.
var ErrConflict = errors.New("lif: stale etag (conflict)")

// warnBuffer collects the warnings and after-hook output of the core and the
// hooks. Writing them to os.Stderr would corrupt the alt screen, so the TUI
// drains them into toasts instead.
type warnBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *warnBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// Drain returns the collected text, trimmed, and empties the buffer.
func (w *warnBuffer) Drain() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := strings.TrimSpace(w.buf.String())
	w.buf.Reset()
	return s
}

// Store is the TUI's in-process view of one lif store. It reads and writes
// through itemcore and the CLI's CoreResolver, so hooks and gates run exactly
// as they do for the CLI.
type Store struct {
	RepoDir  string
	Core     *itemcore.Core
	Resolver *itemgraph.CoreResolver

	cfg         *config.Config
	warn        *warnBuffer
	unsubscribe func()
	closeOnce   sync.Once
	closeErr    error
}

// Open loads the config (configPath, else the one found from repoDir), resolves
// the store root and loads all items. Core warnings and hook output go to the
// Store's warning buffer (DrainWarnings), never to os.Stderr.
func Open(repoDir, storePath, configPath string) (*Store, error) {
	var cfg *config.Config
	var err error
	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil {
			return nil, fmt.Errorf("loading config from %s: %w", configPath, err)
		}
	} else {
		cfg, err = config.LoadFromDirectory(repoDir)
		if err != nil {
			return nil, fmt.Errorf("loading config: %w", err)
		}
	}

	root, _, err := storeroot.Resolve(storePath, cfg)
	if err != nil {
		return nil, err
	}

	warn := &warnBuffer{}
	core := itemcore.New(root, cfg)
	core.SetWarnWriter(warn)
	if err := core.Load(); err != nil {
		return nil, fmt.Errorf("loading items: %w", err)
	}

	d := hooks.New(cfg, core.Root(), warn)
	return &Store{
		RepoDir:  repoDir,
		Core:     core,
		Resolver: &itemgraph.CoreResolver{Core: core, BeforeMutation: d.Gate, AfterMutation: d.Fire},
		cfg:      cfg,
		warn:     warn,
	}, nil
}

// Config returns the project config the store was opened with.
func (s *Store) Config() *config.Config { return s.cfg }

// Items returns every loaded item.
func (s *Store) Items() []*item.Item { return s.Core.All() }

// Search runs a full-text query over title and body.
func (s *Store) Search(q string) ([]*item.Item, error) { return s.Core.Search(q) }

// Raw returns the item's file exactly as it lies on disk.
func (s *Store) Raw(id string) (string, error) {
	it, err := s.Core.Get(id)
	if err != nil {
		return "", fmt.Errorf("item %s: %w", id, err)
	}
	b, err := os.ReadFile(filepath.Join(s.Core.Root(), it.Path))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Watch starts file watching and returns the change-event channel. Close
// closes the channel.
func (s *Store) Watch() (<-chan []itemcore.ItemEvent, error) {
	if err := s.Core.StartWatching(); err != nil {
		return nil, err
	}
	ch, unsub := s.Core.Subscribe()
	s.unsubscribe = unsub
	return ch, nil
}

// Close stops watching and releases the core. It is safe to call twice.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		if s.unsubscribe != nil {
			s.unsubscribe()
		}
		s.closeErr = s.Core.Close()
	})
	return s.closeErr
}

// DrainWarnings returns the warnings and hook output collected since the last
// call.
func (s *Store) DrainWarnings() string { return s.warn.Drain() }

// wrapErr words a core error. An ETag error wraps ErrConflict; every other
// error keeps its text, so a hook block keeps hook name, message and output.
func wrapErr(id string, err error) error {
	var mismatch *itemcore.ETagMismatchError
	var required *itemcore.ETagRequiredError
	if errors.As(err, &mismatch) || errors.As(err, &required) {
		return fmt.Errorf("%w: item %s: %v", ErrConflict, id, err)
	}
	return fmt.Errorf("item %s: %w", id, err)
}

// CreateOpts are the fields accepted by Create. Title is required; every
// other field is optional and left to the core's default when zero.
type CreateOpts struct {
	Title     string
	Type      string
	Status    string
	Priority  string
	Parent    string
	Tags      []string
	BlockedBy []string
	Body      string
}

// Create creates an item and returns it with its fresh ID and ETag.
func (s *Store) Create(opts CreateOpts) (*item.Item, error) {
	in := model.CreateItemInput{Title: opts.Title, Tags: opts.Tags, BlockedBy: opts.BlockedBy}
	if opts.Type != "" {
		in.Type = &opts.Type
	}
	if opts.Status != "" {
		in.Status = &opts.Status
	}
	if opts.Priority != "" {
		in.Priority = &opts.Priority
	}
	if opts.Parent != "" {
		in.Parent = &opts.Parent
	}
	if opts.Body != "" {
		in.Body = &opts.Body
	}
	it, err := s.Resolver.CreateItem(context.Background(), in)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	return it, nil
}

func (s *Store) update(id, etag string, in model.UpdateItemInput) error {
	in.IfMatch = &etag
	if _, err := s.Resolver.UpdateItem(context.Background(), id, in); err != nil {
		return wrapErr(id, err)
	}
	return nil
}

// SetStatus sets an item's status.
func (s *Store) SetStatus(id, status, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{Status: &status})
}

// SetPriority sets an item's priority.
func (s *Store) SetPriority(id, priority, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{Priority: &priority})
}

// SetType sets an item's type.
func (s *Store) SetType(id, typ, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{Type: &typ})
}

// SetTitle sets an item's title.
func (s *Store) SetTitle(id, title, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{Title: &title})
}

// SetTags applies a combined tag diff against one ETag. If a tag is in both
// add and remove, the core decides; the tag picker never produces that.
func (s *Store) SetTags(id string, add, remove []string, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{AddTags: add, RemoveTags: remove})
}

// SetParent sets an item's parent.
func (s *Store) SetParent(id, parent, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{Parent: &parent})
}

// RemoveParent clears an item's parent.
func (s *Store) RemoveParent(id, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{Parent: new("")})
}

// SetBlocking applies a combined blocking diff against one ETag.
func (s *Store) SetBlocking(id string, add, remove []string, etag string) error {
	return s.update(id, etag, model.UpdateItemInput{AddBlocking: add, RemoveBlocking: remove})
}

// WholeEditDiff is the field-level diff between a whole-item $EDITOR
// session's original snapshot and its edited return. nil or empty means
// unchanged. ParentChanged is a separate bool because "" is a valid target
// (remove the parent).
type WholeEditDiff struct {
	Title, Status, Type, Priority *string
	TagsAdd, TagsRemove           []string
	BlockingAdd, BlockingRemove   []string
	BlockedByAdd, BlockedByRemove []string
	ParentChanged                 bool
	Parent                        string // valid only if ParentChanged; "" removes the parent
	Body                          *string
}

// UpdateWhole applies every changed field of a whole-item $EDITOR round-trip
// in one update against one ETag. An empty diff does nothing and returns nil.
func (s *Store) UpdateWhole(id string, diff WholeEditDiff, etag string) error {
	in := model.UpdateItemInput{
		Title:           diff.Title,
		Status:          diff.Status,
		Type:            diff.Type,
		Priority:        diff.Priority,
		AddTags:         diff.TagsAdd,
		RemoveTags:      diff.TagsRemove,
		AddBlocking:     diff.BlockingAdd,
		RemoveBlocking:  diff.BlockingRemove,
		AddBlockedBy:    diff.BlockedByAdd,
		RemoveBlockedBy: diff.BlockedByRemove,
		Body:            diff.Body,
	}
	if diff.ParentChanged {
		in.Parent = &diff.Parent
	}
	if isEmptyUpdate(in) {
		return nil
	}
	return s.update(id, etag, in)
}

func isEmptyUpdate(in model.UpdateItemInput) bool {
	return in.Title == nil && in.Status == nil && in.Type == nil && in.Priority == nil &&
		in.Body == nil && in.Parent == nil &&
		len(in.AddTags) == 0 && len(in.RemoveTags) == 0 &&
		len(in.AddBlocking) == 0 && len(in.RemoveBlocking) == 0 &&
		len(in.AddBlockedBy) == 0 && len(in.RemoveBlockedBy) == 0
}

// Delete removes an item and the links that point at it.
func (s *Store) Delete(id string) error {
	if _, err := s.Resolver.DeleteItem(context.Background(), id); err != nil {
		return wrapErr(id, err)
	}
	return nil
}
