// Package worktree manages git worktrees associated with items.
package worktree

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// DefaultFetchTimeout is the default timeout for git fetch operations during
// worktree creation. Set below the HTTP server's WriteTimeout (15s) to prevent
// the HTTP connection from dying before the fetch completes.
const DefaultFetchTimeout = 10 * time.Second

// SetupStatus represents the state of a worktree's post-creation setup.
type SetupStatus string

const (
	SetupNone    SetupStatus = ""        // no setup configured or already done
	SetupRunning SetupStatus = "running" // setup command is executing
	SetupDone    SetupStatus = "done"    // setup completed successfully
	SetupFailed  SetupStatus = "failed"  // setup command failed
)

// Worktree represents a git worktree.
type Worktree struct {
	ID           string
	Branch       string
	Path         string
	Name         string      // Human-readable name
	Description  string      // Auto-generated summary of what this workspace is doing
	ItemIDs      []string    // Item IDs detected from changes vs base branch
	Setup        SetupStatus // post-creation setup status (runtime only)
	SetupError   string      // error message if setup failed
	LastActiveAt time.Time   // When an agent last completed a turn in this worktree
}

// SetupDoneFunc is called when a worktree's setup command finishes.
// Receives the worktree ID, success status, and error message (if failed).
type SetupDoneFunc func(worktreeID string, success bool, errMsg string)

// Manager handles git worktree operations for a repository.
type Manager struct {
	repoRoot     string
	worktreeRoot string // directory where worktrees are created (e.g. ~/.lif/worktrees/<project>/)
	baseRef      string
	setupCommand string        // shell command to run after worktree creation
	fetchTimeout time.Duration // timeout for git fetch during worktree creation (0 = skip fetch)
	storeDir     string        // name of the store directory inside a worktree checkout
	mu           sync.RWMutex

	// setupStatuses tracks runtime setup status for worktrees (not persisted)
	setupStatuses map[string]setupState

	// onSetupDone is called when a worktree's setup command finishes
	onSetupDone SetupDoneFunc

	// subscribers for worktree change events
	subMu       sync.Mutex
	subscribers []chan struct{}
}

// setupState tracks the runtime setup status and error for a worktree.
type setupState struct {
	status SetupStatus
	err    string
}

// NewManager creates a new worktree manager for the given repository root.
// worktreeRoot is the directory where worktrees are created (e.g. ~/.lif/worktrees/<project>/).
// baseRef is the git ref to use as the starting point for new branches (e.g. "main").
// setupCommand is an optional shell command to run inside new worktrees after creation.
// ManagerOption is a functional option for configuring a Manager.
type ManagerOption func(*Manager)

// WithFetchTimeout sets the timeout for git fetch operations during worktree creation.
// A value of 0 disables the fetch entirely. Negative values use the default timeout.
func WithFetchTimeout(d time.Duration) ManagerOption {
	return func(m *Manager) {
		m.fetchTimeout = d
	}
}

// WithStoreDir sets the name of the store directory inside a worktree checkout,
// the same name as the main project's store directory (default: the new
// default store directory). DetectItemIDs reads item changes under it.
func WithStoreDir(name string) ManagerOption {
	return func(m *Manager) {
		m.storeDir = name
	}
}

func NewManager(repoRoot, worktreeRoot, baseRef, setupCommand string, opts ...ManagerOption) *Manager {
	m := &Manager{
		repoRoot:      repoRoot,
		worktreeRoot:  worktreeRoot,
		baseRef:       baseRef,
		setupCommand:  setupCommand,
		fetchTimeout:  DefaultFetchTimeout,
		storeDir:      config.DefaultStoreDir,
		setupStatuses: make(map[string]setupState),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// RepoRoot returns the path to the main repository root.
func (m *Manager) RepoRoot() string {
	return m.repoRoot
}

// BaseRef returns the configured base ref for worktree branches.
func (m *Manager) BaseRef() string {
	return m.baseRef
}

// StoreDir returns the name of the store directory inside a worktree checkout.
func (m *Manager) StoreDir() string {
	return m.storeDir
}

// fetchBaseRef fetches the base ref from the remote so that worktrees branch
// from the latest code. Handles both plain refs ("main") and remote-tracking
// refs ("origin/main"). Logs warnings on failure but does not return an error —
// a stale ref is better than refusing to create a worktree.
//
// The fetch is bounded by m.fetchTimeout. If the timeout is 0, the fetch is
// skipped entirely (useful for airgapped environments or when the remote is
// known to be unavailable).
func (m *Manager) fetchBaseRef() {
	if m.baseRef == "" {
		return
	}

	if m.fetchTimeout == 0 {
		log.Printf("[worktree] fetch timeout is 0, skipping remote fetch")
		return
	}

	// Determine which remote and ref to fetch.
	// "origin/main" → remote="origin", ref="main"
	// "main"        → remote="origin", ref="main"
	remote := "origin"
	ref := m.baseRef
	if strings.Contains(m.baseRef, "/") {
		parts := strings.SplitN(m.baseRef, "/", 2)
		remote = parts[0]
		ref = parts[1]
	}

	// Skip fetch if the remote doesn't exist (e.g., local-only test repos)
	checkCmd := exec.Command("git", "remote", "get-url", remote)
	checkCmd.Dir = m.repoRoot
	if err := checkCmd.Run(); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.fetchTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "fetch", remote, ref)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[worktree] warning: git fetch %s %s timed out after %s", remote, ref, m.fetchTimeout)
		} else {
			log.Printf("[worktree] warning: git fetch %s %s failed: %s", remote, ref, strings.TrimSpace(string(out)))
		}
	}
}

// SetOnSetupDone registers a callback that fires when a worktree's setup command finishes.
func (m *Manager) SetOnSetupDone(fn SetupDoneFunc) {
	m.onSetupDone = fn
}

// Subscribe returns a channel that receives a signal whenever worktrees change.
// The caller should call Unsubscribe when done.
func (m *Manager) Subscribe() chan struct{} {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	ch := make(chan struct{}, 1)
	m.subscribers = append(m.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscription channel.
func (m *Manager) Unsubscribe(ch chan struct{}) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for i, sub := range m.subscribers {
		if sub == ch {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Notify sends a signal to all subscribers. Exported so external watchers
// (e.g., the worktree item file watcher) can trigger a refresh.
func (m *Manager) Notify() {
	m.notify()
}

// notify sends a signal to all subscribers.
func (m *Manager) notify() {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for _, ch := range m.subscribers {
		select {
		case ch <- struct{}{}:
		default:
			// Non-blocking: if the channel already has a pending signal, skip
		}
	}
}

// listPorcelain runs `git worktree list --porcelain` in repoRoot.
func listPorcelain(repoRoot string) (string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git worktree list: %w", err)
	}
	return string(out), nil
}

// branchExists reports whether the local branch exists in the repository.
func (m *Manager) branchExists(branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = m.repoRoot
	return cmd.Run() == nil
}

// parseWorktrees lists the worktrees of the repository (see parsePorcelain).
func (m *Manager) parseWorktrees() ([]Worktree, error) {
	out, err := listPorcelain(m.repoRoot)
	if err != nil {
		return nil, err
	}
	return parsePorcelain(out, m.worktreeRoot, m.branchExists), nil
}

// List returns all active worktrees that were created by lif, on a lif/ or a
// legacy beans/ branch, each with its actual branch.
func (m *Manager) List() ([]Worktree, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	worktrees, err := m.parseWorktrees()
	if err != nil {
		return nil, err
	}

	// Enrich with metadata (name, description for standalone worktrees)
	for i := range worktrees {
		if meta := m.loadMeta(worktrees[i].ID); meta != nil {
			worktrees[i].Name = meta.Name
			worktrees[i].Description = meta.Description
			if meta.LastActiveAt != nil {
				worktrees[i].LastActiveAt = *meta.LastActiveAt
			}
		}
		worktrees[i].ItemIDs = m.DetectItemIDs(worktrees[i].Path)
		// Attach runtime setup status
		if st, ok := m.setupStatuses[worktrees[i].ID]; ok {
			worktrees[i].Setup = st.status
			worktrees[i].SetupError = st.err
		}
	}

	// Keep worktrees in git's output order (creation order, oldest first).
	return worktrees, nil
}

// worktreeBranchPrefixes are the branch prefixes of a worktree, the prefix of
// new worktrees first. Every lookup by prefix goes through this list.
var worktreeBranchPrefixes = []string{config.WorktreeBranchPrefix, config.LegacyWorktreeBranchPrefix}

// worktreeBranchID returns the worktree ID of a worktree branch: the branch
// name without its lif/ or legacy beans/ prefix. ok is false for any other
// branch.
func worktreeBranchID(branch string) (id string, ok bool) {
	for _, prefix := range worktreeBranchPrefixes {
		if rest, found := strings.CutPrefix(branch, prefix); found {
			return rest, true
		}
	}
	return "", false
}

// parsePorcelain parses `git worktree list --porcelain` output and returns
// worktrees whose branch starts with the lif/ or the legacy beans/ prefix, each
// with its actual branch.
// Entries marked as "prunable" (stale/missing directory) are skipped.
// worktreesDir is the path to the items worktrees directory (e.g. "~/.lif/worktrees/<project>/"),
// used to identify lif-managed worktrees that are temporarily detached (e.g. during rebase).
// A detached worktree carries no branch in the output, so branchExists decides
// which branch it reports: lif/<id> if that exists, else beans/<id>, else
// none (empty Branch) -- a branch name is never made up. A nil branchExists
// finds none.
func parsePorcelain(output string, worktreesDir string, branchExists func(branch string) bool) []Worktree {
	// git prints resolved paths; a symlinked root (macOS /var -> /private/var)
	// matches only in its resolved form.
	resolvedDir := worktreesDir
	if worktreesDir != "" {
		if r, err := filepath.EvalSymlinks(worktreesDir); err == nil {
			resolvedDir = r
		}
	}
	var worktrees []Worktree
	var currentPath, currentBranch string
	var prunable, detached bool

	emit := func() {
		if prunable || currentPath == "" {
			currentPath = ""
			currentBranch = ""
			prunable = false
			detached = false
			return
		}

		if id, ok := worktreeBranchID(currentBranch); ok {
			// Normal case: branch is on a lif/ or legacy beans/ branch
			worktrees = append(worktrees, Worktree{
				ID:     id,
				Branch: currentBranch,
				Path:   currentPath,
			})
		} else if detached && worktreesDir != "" && strings.HasPrefix(currentPath, resolvedDir) {
			// Detached HEAD (e.g. during rebase) — identify by path
			id := filepath.Base(currentPath)
			branch := ""
			if branchExists != nil {
				for _, prefix := range worktreeBranchPrefixes {
					if branchExists(prefix + id) {
						branch = prefix + id
						break
					}
				}
			}
			worktrees = append(worktrees, Worktree{
				ID:     id,
				Branch: branch,
				Path:   currentPath,
			})
		}

		currentPath = ""
		currentBranch = ""
		prunable = false
		detached = false
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
			currentBranch = ""
			prunable = false
			detached = false
		} else if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			// ref is like "refs/heads/lif/item-abc1" (or "refs/heads/beans/beans-abc1")
			currentBranch = strings.TrimPrefix(ref, "refs/heads/")
		} else if line == "detached" {
			detached = true
		} else if strings.HasPrefix(line, "prunable ") {
			prunable = true
		} else if line == "" {
			emit()
		}
	}

	// Handle last entry (porcelain output may not end with blank line)
	emit()

	return worktrees
}

// DetectItemIDs returns item IDs found in the worktree's diff vs the base branch.
// It filters for <store dir>/*.md files, excluding dot-prefixed subdirs (like .worktrees/,
// .conversations/) and the archive/ directory.
func (m *Manager) DetectItemIDs(worktreePath string) []string {
	changes, err := gitutil.AllChangesVsUpstream(worktreePath, m.baseRef)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var ids []string

	for _, change := range changes {
		// Must be directly under the store directory and end with .md
		rest, ok := strings.CutPrefix(change.Path, m.storeDir+"/")
		if !ok {
			continue
		}

		// Exclude subdirectories: anything with a / is nested
		if strings.Contains(rest, "/") {
			continue
		}

		// Must be a .md file
		if !strings.HasSuffix(rest, ".md") {
			continue
		}

		// Skip if the file was deleted in the worktree
		fullPath := filepath.Join(worktreePath, m.storeDir, rest)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			continue
		}

		id, _ := item.ParseFilename(rest)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)
	return ids
}

// validName keeps the name a single safe path segment (no separators, no leading dash or dot).
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Create creates a new git worktree with the given name.
// It stores the human-readable name as metadata.
// The worktree is placed in the configured worktree root directory.
func (m *Manager) Create(name string) (*Worktree, error) {
	if name == "" {
		return nil, fmt.Errorf("worktree name must not be empty")
	}
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("invalid worktree name %q: use letters, digits, '.', '_', '-'", name)
	}

	// Use the name as the worktree ID so branch and directory match
	id := name

	m.mu.Lock()
	defer m.mu.Unlock()

	branch := config.WorktreeBranchPrefix + name
	worktreePath := m.WorktreePath(id)

	// Check if the worktree path already exists
	if _, err := os.Stat(worktreePath); err == nil {
		return nil, fmt.Errorf("worktree path already exists: %s", worktreePath)
	}

	// Fetch the latest base ref from origin so worktrees branch from up-to-date code.
	// This is especially important for PR-based workflows using origin/<branch> as base_ref.
	m.fetchBaseRef()

	// Create the worktree with a new branch
	args := []string{"worktree", "add", worktreePath, "-b", branch}
	if m.baseRef != "" {
		args = append(args, m.baseRef)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[worktree] failed to create worktree %s at %s: %s: %v", id, worktreePath, strings.TrimSpace(string(out)), err)
		return nil, fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Save the name metadata with initial LastActiveAt so new worktrees
	// sort to the top (most recently created first)
	now := time.Now().UTC()
	if err := m.saveMeta(id, &worktreeMeta{Name: name, LastActiveAt: &now}); err != nil {
		log.Printf("[worktree] warning: failed to save metadata for %s: %v", id, err)
	}

	wt := &Worktree{
		ID:     id,
		Branch: branch,
		Path:   worktreePath,
		Name:   name,
	}

	// Run setup command asynchronously if configured
	if m.setupCommand != "" {
		m.setupStatuses[id] = setupState{status: SetupRunning}
		wt.Setup = SetupRunning

		go func() {
			log.Printf("[worktree] running setup command in %s: %s", worktreePath, m.setupCommand)
			setupCmd := exec.Command("sh", "-c", m.setupCommand)
			setupCmd.Dir = worktreePath
			out, err := setupCmd.CombinedOutput()

			m.mu.Lock()
			if err != nil {
				errMsg := strings.TrimSpace(string(out))
				log.Printf("[worktree] setup command failed in %s: %s: %v", worktreePath, errMsg, err)
				m.setupStatuses[id] = setupState{status: SetupFailed, err: errMsg}
			} else {
				log.Printf("[worktree] setup command completed in %s", worktreePath)
				m.setupStatuses[id] = setupState{status: SetupDone}
			}
			m.mu.Unlock()

			m.notify()

			if m.onSetupDone != nil {
				m.onSetupDone(id, err == nil, strings.TrimSpace(string(out)))
			}
		}()
	}

	log.Printf("[worktree] created worktree %s (name=%s, branch=%s, path=%s)", id, name, branch, worktreePath)
	m.notify()
	return wt, nil
}

// worktreeMeta is the metadata stored alongside standalone worktrees.
type worktreeMeta struct {
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Port         int        `json:"port,omitempty"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
}

// metaPath returns the path to the metadata file for a worktree ID.
func (m *Manager) metaPath(id string) string {
	return filepath.Join(m.worktreeRoot, id+".meta.json")
}

// loadMeta loads the metadata for a worktree, if it exists.
func (m *Manager) loadMeta(id string) *worktreeMeta {
	data, err := os.ReadFile(m.metaPath(id))
	if err != nil {
		return nil
	}
	var meta worktreeMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

// saveMeta saves metadata for a worktree.
func (m *Manager) saveMeta(id string, meta *worktreeMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(m.metaPath(id), data, 0644)
}

// removeMeta removes the metadata file for a worktree.
func (m *Manager) removeMeta(id string) {
	os.Remove(m.metaPath(id))
}

// SavePort persists the allocated port for a worktree into its metadata file.
func (m *Manager) SavePort(id string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta := m.loadMeta(id)
	if meta == nil {
		meta = &worktreeMeta{}
	}
	meta.Port = port
	return m.saveMeta(id, meta)
}

// GetPort returns the persisted port for a worktree, or 0 if none is stored.
func (m *Manager) GetPort(id string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if meta := m.loadMeta(id); meta != nil {
		return meta.Port
	}
	return 0
}

// TouchLastActive updates the LastActiveAt timestamp for a worktree to now
// and notifies subscribers. Called when an agent completes a turn.
func (m *Manager) TouchLastActive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta := m.loadMeta(id)
	if meta == nil {
		meta = &worktreeMeta{}
	}
	now := time.Now().UTC()
	meta.LastActiveAt = &now
	if err := m.saveMeta(id, meta); err != nil {
		return fmt.Errorf("save last_active_at: %w", err)
	}
	m.notify()
	return nil
}

// UpdateDescription updates the description for a worktree and notifies subscribers.
func (m *Manager) UpdateDescription(id, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta := m.loadMeta(id)
	if meta == nil {
		meta = &worktreeMeta{}
	}
	meta.Description = description
	if err := m.saveMeta(id, meta); err != nil {
		return fmt.Errorf("save description: %w", err)
	}
	m.notify()
	return nil
}

// Remove removes the worktree with the given ID.
// The actual worktree path is looked up from git (not computed), so this works
// even when the worktree was created from a different repo root/workspace.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Look up the actual path from git rather than computing it,
	// since the worktree may have been created from a different workspace.
	worktreePath, err := m.findWorktreePathByID(id)
	if err != nil {
		return fmt.Errorf("worktree %s not found: %w", id, err)
	}

	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = m.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		outStr := strings.TrimSpace(string(out))
		log.Printf("[worktree] failed to remove worktree %s at %s: %s: %v", id, worktreePath, outStr, err)
		return fmt.Errorf("git worktree remove: %s: %w", outStr, err)
	}

	log.Printf("[worktree] removed worktree %s (path=%s)", id, worktreePath)
	m.removeMeta(id)
	m.notify()
	return nil
}

// findWorktreePathByID looks up the actual filesystem path for a worktree
// by parsing git worktree list output.
// Must be called with m.mu held.
func (m *Manager) findWorktreePathByID(id string) (string, error) {
	worktrees, err := m.parseWorktrees()
	if err != nil {
		return "", err
	}

	for _, wt := range worktrees {
		if wt.ID == id {
			return wt.Path, nil
		}
	}
	return "", fmt.Errorf("no worktree with id %s", id)
}

// LegacyBranchWorktrees returns the worktrees of the repository at repoRoot
// whose branch carries the legacy beans/ prefix, for `lif check`. It makes one
// `git worktree list --porcelain` call and nothing else: no metadata, no diff
// scan, and no detached worktree (it has no branch in that output). It returns
// an error outside a git repository.
func LegacyBranchWorktrees(repoRoot string) ([]Worktree, error) {
	out, err := listPorcelain(repoRoot)
	if err != nil {
		return nil, err
	}
	var legacy []Worktree
	for _, wt := range parsePorcelain(out, "", nil) {
		if strings.HasPrefix(wt.Branch, config.LegacyWorktreeBranchPrefix) {
			legacy = append(legacy, wt)
		}
	}
	return legacy, nil
}

// WorktreePath returns the filesystem path for a worktree with the given ID.
// Worktrees are stored outside the main repo, in the configured worktree root
// (default: ~/.lif/worktrees/<project>/, or ~/.beans/worktrees/<project>/ for
// a project that already has that root).
func (m *Manager) WorktreePath(id string) string {
	return filepath.Join(m.worktreeRoot, id)
}
